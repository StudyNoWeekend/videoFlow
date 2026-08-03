package component

import (
	"context"
	"strings"
)

func installLada(ctx context.Context, sessionID string, params InstallParams, events chan<- ProgressEvent) error {
	sendEvent(sessionID, events, "install.pulling", "Pulling ladaapp/lada:latest", "running")

	image := "ladaapp/lada:latest"
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
	sendEvent(sessionID, events, "install.completed", "Lada image pulled successfully", "completed")
	return nil
}
