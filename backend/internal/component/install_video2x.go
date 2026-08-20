package component

import (
	"context"
	"strings"
)

func installVideo2X(ctx context.Context, sessionID string, params InstallParams, events chan<- ProgressEvent) error {
	image := params.Image
	if image == "" {
		image = "ghcr.io/k4yt3x/video2x:latest"
	}

	sendEvent(sessionID, events, "install.pulling", "Pulling "+image, "running")

	err := runCommandWithCallback(ctx, "docker", []string{"pull", image}, func(line string) {
		for _, raw := range strings.Split(line, "\r") {
			raw = strings.SplitN(raw, "\n", 2)[0]
			trimmed := strings.TrimSpace(raw)
			if trimmed != "" {
				sendEvent(sessionID, events, "install.pulling", trimmed, "running")
			}
		}
	})
	if err != nil {
		sendEvent(sessionID, events, "install.pulling", "", "failed")
		return err
	}
	sendEvent(sessionID, events, "install.completed", "Video2X image pulled successfully", "completed")
	return nil
}
