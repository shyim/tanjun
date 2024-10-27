package build

import (
	"context"
	"fmt"
	"github.com/charmbracelet/log"
	"io"
	"os"
	"os/signal"

	"github.com/pterm/pterm"

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

	log.Debugf("Connected to remote docker client: %s (%s)", info.ServerVersion, info.Architecture)

	defer remoteClient.Close()

	if config.Build.RemoteBuild {
		dockerClient = remoteClient
		log.Debugf("Using remote builder to build docker image")
	} else {
		dockerClient, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

		if err != nil {
			return "", err
		}
		log.Debugf("Connected to local docker daemon")
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

	log.Debugf("Building LLB from docker image")

	containerConfig, def, err := llbFromProject(ctx, info)
	if err != nil {
		return "", err
	}

	activeDockerClient = dockerClient

	log.Debugf("Connecting to buildkit running as container id %s", containerId)

	builder, err := buildkit.New(ctx, fmt.Sprintf("tanjun://%s", containerId))

	if err != nil {
		return "", err
	}

	defer builder.Close()

	buildkitInfo, err := builder.Info(ctx)

	if err != nil {
		return "", err
	}

	log.Debugf("Buildkit connected version: %s", buildkitInfo.BuildkitVersion)

	log.Debugf("Building solver")

	version, solveOpt, err := getSolveConfiguration(ctx, containerConfig)

	if err != nil {
		return "", err
	}

	log.Debugf("Next version will be %s", version)

	waitChain := make(chan error)

	if config.Build.RemoteBuild {
		pr, pw := io.Pipe()

		go func() {
			resp, err := dockerClient.ImageLoad(ctx, pr, false)

			if err != nil {
				waitChain <- err
				return
			}

			defer resp.Body.Close()

			_, err = io.ReadAll(resp.Body)

			if err != nil {
				waitChain <- err
				return
			}

			waitChain <- nil
		}()

		solveOpt.Exports = []buildkit.ExportEntry{
			{
				Type: buildkit.ExporterDocker,
				Output: func(m map[string]string) (io.WriteCloser, error) {
					return pw, nil
				},
				Attrs: map[string]string{
					"name":                  fmt.Sprintf("%s:%s", config.Image, version),
					"containerimage.config": containerConfig,
				},
			},
		}
	} else {
		waitChain <- nil
	}

	log.Debugf("Starting buildkit build process")

	_, err = builder.Solve(ctx, def, *solveOpt, createSolveChan(ctx))

	if err != nil {
		return "", err
	}

	if config.Build.RemoteBuild {
		log.Debugf("Loading image to local docker registry")
	}

	if <-waitChain != nil {
		return "", err
	}

	return version, nil
}
