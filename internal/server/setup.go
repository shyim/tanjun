package server

import (
	"context"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
)

func Setup(ctx context.Context, server config.ProjectServer) error {
	if isInKnownHost, err := isServerPartOfKnownHost(server); err != nil {
		return err
	} else if !isInKnownHost {
		if err := addServerToKnownHost(ctx, server); err != nil {
			return err
		}
	} else {
		log.Infof("Server %s is already part of known hosts", server.Address)
	}

	if dockerVersion := isDockerInstalled(ctx, server); dockerVersion == "" {
		log.Infof("Docker is not installed on %s. Installing Docker...", server.Address)
		return installDocker(ctx, server)
	} else {
		log.Infof("Docker is installed with version: %s", dockerVersion)
	}

	if !isLiveRestoreEnabled(ctx, server) {
		log.Warn("Live restore is not enabled. Take a look at https://docs.docker.com/engine/daemon/live-restore/ how to enable it")
	} else {
		log.Info("Live restore is already enabled")
	}

	return nil
}
