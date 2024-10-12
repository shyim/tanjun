package server

import (
	"context"
	"fmt"
	"github.com/shyim/tanjun/internal/config"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func isDockerInstalled(ctx context.Context, server config.ProjectServer) string {
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", server.Username, server.Address), "-p", strconv.Itoa(server.Port), "docker", "version", "-f", "'{{ .Client.Version }}'")

	output, err := cmd.Output()

	if err != nil {
		return ""
	}

	return strings.TrimSuffix(string(output), "\n")
}

func installDocker(ctx context.Context, server config.ProjectServer) error {
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", server.Username, server.Address), "-p", strconv.Itoa(server.Port), "curl", "-fsSL", "https://get.docker.com", "|", "sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isLiveRestoreEnabled(ctx context.Context, server config.ProjectServer) bool {
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", server.Username, server.Address), "-p", strconv.Itoa(server.Port), "docker", "info", "--format", "'{{ .LiveRestoreEnabled }}'")

	output, err := cmd.Output()

	if err != nil {
		return false
	}

	return string(output) == "true"
}
