package component

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

func installFFmpeg(ctx context.Context, sessionID string, params InstallParams, events chan<- ProgressEvent) error {
	sendEvent(sessionID, events, "install.start", "Detecting operating system...", "running")

	osName := runtime.GOOS
	sendEvent(sessionID, events, "install.start", fmt.Sprintf("Detected OS: %s", osName), "running")

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "install.cancelled", "Installation cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 2: Install via package manager
	var installCmd string
	var installArgs []string

	switch osName {
	case "darwin":
		// Check if brew is available
		brewPath, err := runCommand(ctx, "which", "brew")
		if err != nil || strings.TrimSpace(brewPath) == "" {
			sendEvent(sessionID, events, "install.start", "", "failed")
			return fmt.Errorf("Homebrew is not installed. Please install ffmpeg manually: brew install ffmpeg")
		}
		installCmd = "brew"
		installArgs = []string{"install", "ffmpeg"}
	case "linux":
		// Detect package manager
		installCmd, installArgs = detectLinuxPackageManager(ctx)
		if installCmd == "" {
			sendEvent(sessionID, events, "install.start", "", "failed")
			return fmt.Errorf("unsupported Linux distribution. Please install ffmpeg manually")
		}
	case "windows":
		// Check if winget is available
		_, err := runCommand(ctx, "where", "winget")
		if err != nil {
			sendEvent(sessionID, events, "install.start", "", "failed")
			return fmt.Errorf("winget is not available. Please install ffmpeg manually from https://ffmpeg.org/download.html")
		}
		installCmd = "winget"
		installArgs = []string{"install", "--id", "FFmpeg.FFmpeg", "--accept-source-agreements", "--accept-package-agreements"}
	default:
		sendEvent(sessionID, events, "install.start", "", "failed")
		return fmt.Errorf("unsupported OS: %s. Please install ffmpeg manually", osName)
	}

	sendEvent(sessionID, events, "install.start", fmt.Sprintf("Installing ffmpeg via %s...", installCmd), "running")

	err := runCommandWithCallback(ctx, installCmd, installArgs, func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			sendEvent(sessionID, events, "install.start", trimmed, "running")
		}
	})
	if err != nil {
		sendEvent(sessionID, events, "install.start", "", "failed")
		return fmt.Errorf("failed to install ffmpeg: %w", err)
	}

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "install.cancelled", "Installation cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 3: Verify
	sendEvent(sessionID, events, "install.verifying", "Verifying installation...", "running")

	version, err := runCommand(ctx, "ffmpeg", "-version")
	if err != nil {
		sendEvent(sessionID, events, "install.verifying", "", "failed")
		return fmt.Errorf("verification failed: ffmpeg command not found after installation")
	}

	ver := parseFFmpegVersion(version)
	sendEvent(sessionID, events, "install.verifying", fmt.Sprintf("FFmpeg installed successfully: %s", ver), "running")
	return nil
}

// detectLinuxPackageManager 检测 Linux 包管理器
func detectLinuxPackageManager(ctx context.Context) (string, []string) {
	// Try apt-get (Debian/Ubuntu)
	_, err := runCommand(ctx, "which", "apt-get")
	if err == nil {
		return "apt-get", []string{"install", "-y", "ffmpeg"}
	}

	// Try dnf (Fedora/RHEL)
	_, err = runCommand(ctx, "which", "dnf")
	if err == nil {
		return "dnf", []string{"install", "-y", "ffmpeg"}
	}

	// Try yum (CentOS/RHEL)
	_, err = runCommand(ctx, "which", "yum")
	if err == nil {
		return "yum", []string{"install", "-y", "ffmpeg"}
	}

	// Try pacman (Arch)
	_, err = runCommand(ctx, "which", "pacman")
	if err == nil {
		return "pacman", []string{"-S", "--noconfirm", "ffmpeg"}
	}

	return "", nil
}
