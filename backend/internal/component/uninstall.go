package component

import (
	"context"
	"fmt"
)

func uninstallComponent(ctx context.Context, sessionID string, componentType ComponentType, events chan<- ProgressEvent) error {
	switch componentType {
	case ComponentLada:
		return uninstallLada(ctx, sessionID, events)
	case ComponentFFmpeg:
		return uninstallFFmpeg(ctx, sessionID, events)
	default:
		sendError(sessionID, events, "uninstall.start", fmt.Errorf("unsupported component type: %s", componentType))
		return fmt.Errorf("unsupported component type: %s", componentType)
	}
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
