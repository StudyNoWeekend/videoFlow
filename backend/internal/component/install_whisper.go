package component

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func installWhisper(ctx context.Context, sessionID string, params InstallParams, events chan<- ProgressEvent) error {

	// Step 1: Pull image
	sendEvent(sessionID, events, "install.pulling", "Pulling onerahmet/openai-whisper-asr-webservice:latest", "running")

	image := "onerahmet/openai-whisper-asr-webservice:latest"
	err := runCommandWithCallback(ctx, "docker", []string{"pull", image}, func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			sendEvent(sessionID, events, "install.pulling", trimmed, "running")
		}
	})
	if err != nil {
		sendEvent(sessionID, events, "install.pulling", "", "failed")
		return fmt.Errorf("failed to pull image: %w", err)
	}
	sendEvent(sessionID, events, "install.pulling", "Image pulled successfully", "running")

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "install.cancelled", "Installation cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 2: Remove old container if exists
	sendEvent(sessionID, events, "install.starting", "Removing old container if exists", "running")
	_, _ = runCommand(ctx, "docker", "rm", "-f", "whisper-asr-webservice")
	sendEvent(sessionID, events, "install.starting", "Old container removed", "running")

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "install.cancelled", "Installation cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 3: Start container with params
	sendEvent(sessionID, events, "install.starting", "Starting container", "running")

	args := []string{
		"run", "-d",
		"--name", "whisper-asr-webservice",
		"-p", "9000:9000",
	}

	// Add ASR engine parameter
	asrEngine := params.ASREngine
	if asrEngine == "" {
		asrEngine = "openai_whisper"
	}
	args = append(args, "-e", fmt.Sprintf("ASR_ENGINE=%s", asrEngine))

	// Add model parameter
	asrModel := params.ASRModel
	if asrModel == "" {
		asrModel = "base"
	}
	args = append(args, "-e", fmt.Sprintf("ASR_MODEL=%s", asrModel))

	// Add device parameter
	asrDevice := params.ASRDevice
	if asrDevice != "" {
		args = append(args, "-e", fmt.Sprintf("ASR_DEVICE=%s", asrDevice))
	}

	// Add HF token for whisperx
	if params.HFToken != "" {
		args = append(args, "-e", fmt.Sprintf("HF_TOKEN=%s", params.HFToken))
	}

	args = append(args, image)

	containerID, err := runCommand(ctx, "docker", args...)
	if err != nil {
		sendEvent(sessionID, events, "install.starting", "", "failed")
		return fmt.Errorf("failed to start container: %w", err)
	}
	containerID = strings.TrimSpace(containerID)
	sendEvent(sessionID, events, "install.starting", fmt.Sprintf("Container started: %s", containerID), "running")

	select {
	case <-ctx.Done():
		sendEvent(sessionID, events, "install.cancelled", "Installation cancelled", "failed")
		return ctx.Err()
	default:
	}

	// Step 4: Wait for service ready
	sendEvent(sessionID, events, "install.waiting", "Waiting for service to be ready...", "running")

	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			sendEvent(sessionID, events, "install.cancelled", "Installation cancelled", "failed")
			return ctx.Err()
		default:
		}

		// Check container logs for "Uvicorn running on"
		logs, _ := runCommand(ctx, "docker", "logs", "whisper-asr-webservice", "--tail", "5")
		if strings.Contains(logs, "Uvicorn running on") || strings.Contains(logs, "Application startup complete") {
			break
		}

		// Also try curling
		curlOut, _ := runCommand(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "http://localhost:9000/docs", "--connect-timeout", "2")
		if strings.TrimSpace(curlOut) == "200" {
			break
		}

		sendEvent(sessionID, events, "install.waiting", fmt.Sprintf("Waiting... (%d/60s)", i+1), "running")
		time.Sleep(1 * time.Second)
	}

	sendEvent(sessionID, events, "install.waiting", "Service is ready", "running")

	// Step 5: Verify
	sendEvent(sessionID, events, "install.verifying", "Verifying installation...", "running")
	curlOut, err := runCommand(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "http://localhost:9000/docs", "--connect-timeout", "5")
	if err != nil || strings.TrimSpace(curlOut) != "200" {
		sendEvent(sessionID, events, "install.verifying", "", "failed")
		return fmt.Errorf("service verification failed: HTTP %s", strings.TrimSpace(curlOut))
	}

	sendEvent(sessionID, events, "install.verifying", "Whisper ASR installed and verified successfully", "running")
	return nil
}
