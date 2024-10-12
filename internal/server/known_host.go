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

func isServerPartOfKnownHost(server config.ProjectServer) (bool, error) {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return false, err
	}

	knownHostsFile := homeDir + "/.ssh/known_hosts"

	knownHosts, err := os.ReadFile(knownHostsFile)

	if err != nil {
		return false, nil
	}

	return strings.Contains(string(knownHosts), server.Address), nil
}

func addServerToKnownHost(ctx context.Context, server config.ProjectServer) error {
	keyscanCmd := exec.CommandContext(ctx, "ssh-keyscan", server.Address, "-p", strconv.Itoa(server.Port))
	keys, err := keyscanCmd.Output()

	if err != nil {
		return fmt.Errorf("failed to get server keys (ssh-keyscan %s -p %d): %w", server.Address, server.Port, err)
	}

	filteredKeys := ""
	for _, key := range strings.Split(string(keys), "\n") {
		if len(key) == 0 || key[0] == '#' {
			continue
		}

		filteredKeys += key + "\n"
	}

	homeDir, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	knownHostsFile := homeDir + "/.ssh/known_hosts"

	if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
		return os.WriteFile(knownHostsFile, keys, 0644)
	}

	knownHosts, err := os.ReadFile(knownHostsFile)

	if err != nil {
		return err
	}

	knownHosts = append(knownHosts, []byte(filteredKeys)...)

	return os.WriteFile(knownHostsFile, knownHosts, 0644)
}
