package component

import (
	"context"
	"fmt"
)

func uninstallComponent(ctx context.Context, sessionID string, componentType ComponentType, events chan<- ProgressEvent) error {
	switch componentType {
	case ComponentWhisperASR:
		return uninstallWhisper(ctx, sessionID, events)
	case ComponentLada:
		return uninstallLada(ctx, sessionID, events)
	case ComponentFFmpeg:
		return uninstallFFmpeg(ctx, sessionID, events)
	default:
		sendError(sessionID, events, "uninstall.start", fmt.Errorf("unsupported component type: %s", componentType))
		return fmt.Errorf("unsupported component type: %s", componentType)
	}
}

func uninstallWhisper(ctx context.Context, sessionID string, events chan<- ProgressEvent) error {
	// Step 1: Stop container
	sendEvent(sessionID, events, "uninstall.stopping", "Stopping whisper-asr-webservice container...", "running")
	_, err := runCommand(ctx, "docker", "stop", "whisper-asr-webservice")
	if err != nil {
		sendEvent(sessionID, events, "uninstall.stopping", "Container may not exist, continuing...", "running")
	} else {
		sendEvent(sessionID, events, "uninstall.stopping", "Container stopped", "running")
	}

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "uninstall.failed", "Uninstall cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 2: Remove container
	sendEvent(sessionID, events, "uninstall.removing", "Removing container...", "running")
	_, err = runCommand(ctx, "docker", "rm", "whisper-asr-webservice")
	if err != nil {
		sendEvent(sessionID, events, "uninstall.removing", "Container may not exist, continuing...", "running")
	} else {
		sendEvent(sessionID, events, "uninstall.removing", "Container removed", "running")
	}

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "uninstall.failed", "Uninstall cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 3: Remove image
	sendEvent(sessionID, events, "uninstall.rmi", "Removing image...", "running")
	_, err = runCommand(ctx, "docker", "rmi", "onerahmet/openai-whisper-asr-webservice:latest")
	if err != nil {
		sendEvent(sessionID, events, "uninstall.rmi", "Image may be in use or not found, continuing...", "running")
	} else {
		sendEvent(sessionID, events, "uninstall.rmi", "Image removed", "running")
	}

	sendEvent(sessionID, events, "uninstall.completed", "Whisper ASR uninstalled successfully", "running")
	return nil
}

func uninstallLada(ctx context.Context, sessionID string, events chan<- ProgressEvent) error {
	// Lada uses disposable containers, so just remove the image
	sendEvent(sessionID, events, "uninstall.rmi", "Removing lada image...", "running")
	_, err := runCommand(ctx, "docker", "rmi", "ladaapp/lada:latest")
	if err != nil {
		sendEvent(sessionID, events, "uninstall.rmi", "Image may not exist or is in use, continuing...", "running")
	} else {
		sendEvent(sessionID, events, "uninstall.rmi", "Image removed", "running")
	}

	sendEvent(sessionID, events, "uninstall.completed", "Lada uninstalled successfully", "running")
	return nil
}

func uninstallFFmpeg(ctx context.Context, sessionID string, events chan<- ProgressEvent) error {
	sendEvent(sessionID, events, "uninstall.start", "FFmpeg is a local system binary. Please use your system package manager to uninstall it.", "running")

	// Provide instructions based on OS
	osCmd := ""
	_, err := runCommand(ctx, "which", "brew")
	if err == nil {
		osCmd = "brew uninstall ffmpeg"
	} else {
		_, err = runCommand(ctx, "which", "apt-get")
		if err == nil {
			osCmd = "sudo apt-get remove ffmpeg"
		} else {
			_, err = runCommand(ctx, "which", "dnf")
			if err == nil {
				osCmd = "sudo dnf remove ffmpeg"
			} else {
				_, err = runCommand(ctx, "which", "pacman")
				if err == nil {
					osCmd = "sudo pacman -R ffmpeg"
				}
			}
		}
	}

	if osCmd != "" {
		sendEvent(sessionID, events, "uninstall.start", fmt.Sprintf("Run: %s", osCmd), "running")
		sendEvent(sessionID, events, "uninstall.completed", "Please run the command above to uninstall FFmpeg", "running")
	} else {
		sendEvent(sessionID, events, "uninstall.completed", "Please use your system package manager to uninstall FFmpeg", "running")
	}

	return nil
}
