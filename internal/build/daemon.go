package build

import (
	"context"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/docker"
	"time"
)

func stopBuildkitd(dockerClient *client.Client, ctx context.Context, containerId string) {
	if err := dockerClient.ContainerKill(ctx, containerId, "SIGKILL"); err != nil {
		log.Warnf("Failed to kill container %s: %s", containerId, err)
	}

	if err := dockerClient.ContainerRemove(ctx, containerId, container.RemoveOptions{}); err != nil {
		log.Warnf("Failed to remove container %s: %s", containerId, err)
	}

	if err := dockerClient.Close(); err != nil {
		log.Warnf("Failed to close docker client: %s", err)
	}
}

func startBuildkitd(ctx context.Context, dockerClient *client.Client) (string, error) {
	if err := docker.PullImageIfNotThere(ctx, dockerClient, "moby/buildkit:v0.16.0"); err != nil {
		return "", err
	}

	c, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: "moby/buildkit:v0.16.0",
		Healthcheck: &container.HealthConfig{
			Test:          []string{"CMD", "buildctl", "debug", "workers"},
			StartInterval: time.Millisecond * 100,
			Interval:      time.Millisecond * 100,
		},
	}, &container.HostConfig{Privileged: true}, nil, nil, "")

	if err != nil {
		return "", err
	}

	if err := dockerClient.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	for {
		inspect, err := dockerClient.ContainerInspect(ctx, c.ID)

		if err != nil {
			return "", err
		}

		if inspect.State.Health.Status == types.Healthy {
			break
		}

		time.Sleep(time.Millisecond * 50)
	}

	return c.ID, nil
}
