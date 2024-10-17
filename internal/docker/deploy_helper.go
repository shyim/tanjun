package docker

import (
	"context"
	"github.com/pterm/pterm"

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
	if len(containers) == 0 {
		return nil
	}

	spinnerInfo, spinnerErr := pterm.DefaultSpinner.Start("Removing old containers")

	if spinnerErr != nil {
		return spinnerErr
	}

	var err errgroup.Group

	for _, c := range containers {
		c := c
		err.Go(func() error {
			return client.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		})
	}

	if err := err.Wait(); err != nil {
		spinnerInfo.Fail("Failed to remove containers")
		return err
	}

	spinnerInfo.Success("Removed old containers")

	return nil
}

func findPortMapping(cfg DeployConfiguration, container types.ContainerJSON) string {
	if cfg.ProjectConfig.Proxy.AppPort != 0 {
		return string(rune(cfg.ProjectConfig.Proxy.AppPort))
	}

	for p := range container.NetworkSettings.Ports {
		if p.Proto() == "udp" {
			continue
		}

		if p.Port() != "9000" {
			return p.Port()
		}
	}

	return "8000"
}
