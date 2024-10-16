package build

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"os"
	"os/signal"

	"github.com/docker/docker/client"
	buildkit "github.com/moby/buildkit/client"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
)

type contextConfig string
type contextRootPath string
type contextDockerClient string
type contextRemoteClient string

const contextConfigField contextConfig = "projectConfig"
const contextRootPathField contextRootPath = "rootPath"
const contextDockerClientField contextDockerClient = "dockerClient"
const contextRemoteClientField contextRemoteClient = "remoteClient"

func BuildImage(ctx context.Context, config *config.ProjectConfig, root string) (string, error) {
	var dockerClient *client.Client
	var err error

	remoteClient, err := docker.CreateClientFromConfig(config)

	if err != nil {
		return "", err
	}

	info, err := remoteClient.Info(ctx)

	if err != nil {
		return "", err
	}

	defer remoteClient.Close()

	dockerClient, err = client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		return "", err
	}

	ctx = context.WithValue(ctx, contextConfigField, config)
	ctx = context.WithValue(ctx, contextRootPathField, root)
	ctx = context.WithValue(ctx, contextDockerClientField, dockerClient)
	ctx = context.WithValue(ctx, contextRemoteClientField, remoteClient)

	spinnerInfo, err := pterm.DefaultSpinner.Start("Starting buildkitd")

	if err != nil {
		return "", err
	}

	containerId, err := startBuildkitd(ctx, dockerClient)
	if err != nil {
		return "", err
	}

	spinnerInfo.Success("Started buildkitd")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		stopBuildkitd(dockerClient, ctx, containerId)
		os.Exit(1)
	}()

	defer stopBuildkitd(dockerClient, ctx, containerId)

	containerConfig, def, err := llbFromProject(ctx, info)
	if err != nil {
		return "", err
	}

	activeDockerClient = dockerClient

	builder, err := buildkit.New(ctx, fmt.Sprintf("tanjun://%s", containerId))

	if err != nil {
		return "", err
	}

	defer builder.Close()

	version, solveOpt, err := getSolveConfiguration(ctx, containerConfig)

	if err != nil {
		return "", err
	}

	_, err = builder.Solve(ctx, def, *solveOpt, createSolveChan(ctx))

	if err != nil {
		return "", err
	}

	return version, nil
}
