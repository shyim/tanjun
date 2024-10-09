package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/gosimple/slug"
	"strings"
)

func DestroyProject(ctx context.Context, client *client.Client, name string) error {
	containerOpts := container.ListOptions{Filters: filters.NewArgs()}

	containerOpts.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", slug.Make(name)))

	containers, err := client.ContainerList(ctx, containerOpts)

	if err != nil {
		return err
	}

	for _, c := range containers {
		if err := client.ContainerKill(ctx, c.ID, "SIGKILL"); err != nil {
			return err
		}

		if err := client.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			return err
		}
	}

	volumeOpts := volume.ListOptions{Filters: filters.NewArgs()}
	volumeOpts.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", slug.Make(name)))

	volumes, err := client.VolumeList(ctx, volumeOpts)

	if err != nil {
		return err
	}

	for _, v := range volumes.Volumes {
		if err := client.VolumeRemove(ctx, v.Name, true); err != nil {
			return err
		}
	}

	networkOpts := network.ListOptions{Filters: filters.NewArgs()}
	networkOpts.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", slug.Make(name)))

	networks, err := client.NetworkList(ctx, networkOpts)

	if err != nil {
		return err
	}

	for _, n := range networks {
		if err := client.NetworkRemove(ctx, n.ID); err != nil {
			return err
		}
	}

	kv, err := CreateKVConnection(ctx, client)

	if err != nil {
		return err
	}

	defer kv.Close()

	cfg := DeployConfiguration{Name: slug.Make(name)}

	kv.Delete(cfg.ContainerPrefix() + "_secrets")
	kv.Delete(cfg.ContainerPrefix() + "_setup")

	if err := configureKamalService(ctx, client, []string{"kamal-proxy", "remove", cfg.Name}); err != nil {
		if strings.Contains(err.Error(), "service not found") {
			return nil
		}

		return err
	}

	return nil
}
