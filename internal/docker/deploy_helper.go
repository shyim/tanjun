package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
)

func startContainers(ctx context.Context, client *client.Client, containers []types.Container) error {
	var err errgroup.Group

	for _, c := range containers {
		c := c
		err.Go(func() error {
			return client.ContainerStart(ctx, c.ID, container.StartOptions{})
		})
	}

	return err.Wait()
}

func stopContainers(ctx context.Context, client *client.Client, containers []types.Container) error {
	var err errgroup.Group

	for _, c := range containers {
		c := c
		err.Go(func() error {
			return client.ContainerStop(ctx, c.ID, container.StopOptions{})
		})
	}

	return err.Wait()
}

func removeContainers(ctx context.Context, client *client.Client, containers []types.Container) error {
	var err errgroup.Group

	for _, c := range containers {
		c := c
		err.Go(func() error {
			timeout := 5
			if err := client.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
				return err
			}

			return client.ContainerRemove(ctx, c.ID, container.RemoveOptions{})
		})
	}

	return err.Wait()
}

func findPortMapping(cfg DeployConfiguration, container types.ContainerJSON) string {
	if cfg.ProjectConfig.Proxy.Port != 0 {
		return string(rune(cfg.ProjectConfig.Proxy.Port))
	}

	for p, _ := range container.NetworkSettings.Ports {
		if p.Proto() == "tcp" && p.Port() != "9000" {
			return p.Port()
		}
	}

	return "8000"
}
