package docker

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const kamalImage = "ghcr.io/shyim/tanjun/kamal-proxy:v0.8.7"
const kamalContainerName = "tanjun-proxy"
const kamalVolumeName = "tanjun-kamal-certs"
const kamalNetworkName = "tanjun-public"
const tanjunKVContainerName = "tanjun-kv"
const tanjunKVVolumeName = "tanjun-kv"
const tanjunKVImage = "ghcr.io/shyim/tanjun/kv-store:v1"

func ConfigureServer(ctx context.Context, client *client.Client) error {
	var sideGroup errgroup.Group

	sideGroup.Go(func() error {
		return createVolume(ctx, client, kamalVolumeName)
	})

	sideGroup.Go(func() error {
		return createVolume(ctx, client, tanjunKVVolumeName)
	})

	sideGroup.Go(func() error {
		return createPublicNetwork(ctx, client)
	})

	if err := sideGroup.Wait(); err != nil {
		return err
	}

	var containerGroup errgroup.Group

	containerGroup.Go(func() error {
		return createKamalContainer(ctx, client)
	})

	containerGroup.Go(func() error {
		return createKeyValueContainer(ctx, client)
	})

	containerGroup.Go(func() error {
		return createSysctlContainer(ctx, client)
	})

	return containerGroup.Wait()
}

func createKeyValueContainer(ctx context.Context, c *client.Client) error {
	if err := PullImageIfNotThere(ctx, c, tanjunKVImage); err != nil {
		return err
	}

	opts := container.ListOptions{Filters: filters.NewArgs()}
	opts.Filters.Add("name", tanjunKVContainerName)

	containers, err := c.ContainerList(ctx, opts)

	if err != nil {
		return err
	}

	if len(containers) == 1 {
		if containers[0].Image == tanjunKVImage {
			return nil
		}

		if err := removeContainers(ctx, c, containers); err != nil {
			return err
		}
	}

	cfg := &container.Config{
		Image: tanjunKVImage,
		Tty:   true,
		Labels: map[string]string{
			"tanjun":                              "true",
			"com.docker.compose.project":          "tanjun",
			"com.docker.compose.service":          "kv",
			"com.docker.compose.container-number": "1",
			"com.docker.compose.oneoff":           "False",
		},
	}

	hostCfg := &container.HostConfig{
		Mounts: []mount.Mount{{Type: "volume", Source: tanjunKVVolumeName, Target: "/data"}},
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	created, err := c.ContainerCreate(ctx, cfg, hostCfg, nil, nil, tanjunKVContainerName)

	if err != nil {
		return err
	}

	return c.ContainerStart(ctx, created.ID, container.StartOptions{})
}

func createKamalContainer(ctx context.Context, c *client.Client) error {
	if err := PullImageIfNotThere(ctx, c, kamalImage); err != nil {
		return err
	}

	opts := container.ListOptions{Filters: filters.NewArgs()}
	opts.Filters.Add("name", kamalContainerName)

	containers, err := c.ContainerList(ctx, opts)

	if err != nil {
		return err
	}

	if len(containers) == 1 {
		if containers[0].Image == kamalImage {
			return nil
		}

		if err := c.ContainerRemove(ctx, containers[0].ID, container.RemoveOptions{Force: true}); err != nil {
			return err
		}
	}

	cfg := &container.Config{
		Image: kamalImage,
		Labels: map[string]string{
			"tanjun":                              "true",
			"com.docker.compose.project":          "tanjun",
			"com.docker.compose.service":          "proxy",
			"com.docker.compose.container-number": "1",
			"com.docker.compose.oneoff":           "False",
		},
		ExposedPorts: map[nat.Port]struct{}{"80/tcp": {}, "443/tcp": {}, "443/udp": {}},
	}

	ports := make(nat.PortMap)
	ports["80/tcp"] = []nat.PortBinding{{HostPort: "80/tcp"}}
	ports["443/tcp"] = []nat.PortBinding{{HostPort: "443/tcp"}}
	ports["443/udp"] = []nat.PortBinding{{HostPort: "443/udp"}}

	hostCfg := &container.HostConfig{
		Mounts:       []mount.Mount{{Type: "volume", Source: kamalVolumeName, Target: "/home/kamal-proxy/.config/kamal-proxy/"}},
		PortBindings: ports,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			kamalNetworkName: {},
		},
	}

	created, err := c.ContainerCreate(ctx, cfg, hostCfg, networkCfg, nil, kamalContainerName)

	if err != nil {
		return err
	}

	return c.ContainerStart(ctx, created.ID, container.StartOptions{})
}

func createPublicNetwork(ctx context.Context, c *client.Client) error {
	opts := network.ListOptions{Filters: filters.NewArgs()}
	opts.Filters.Add("name", kamalNetworkName)

	networks, err := c.NetworkList(ctx, opts)

	if err != nil {
		return err
	}

	if len(networks) == 0 {
		if _, err := c.NetworkCreate(ctx, kamalNetworkName, network.CreateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func createVolume(ctx context.Context, c *client.Client, name string) error {
	opts := volume.ListOptions{Filters: filters.NewArgs()}
	opts.Filters.Add("name", name)

	volumes, err := c.VolumeList(ctx, opts)

	if err != nil {
		return err
	}

	if len(volumes.Volumes) == 0 {
		_, err := c.VolumeCreate(ctx, volume.CreateOptions{
			Name: name,
			Labels: map[string]string{
				"com.docker.compose.project": "tanjun",
				"com.docker.compose.version": "2.0.0",
				"com.docker.compose.volume":  name,
				"tanjun":                     "true",
			},
		})

		if err != nil {
			return err
		}

		return nil
	}

	return nil
}

func createSysctlContainer(ctx context.Context, client *client.Client) error {
	info, err := client.Info(ctx)

	if err != nil {
		return err
	}

	// OrbStack is a custom OS that doesn't need sysctl
	if info.OperatingSystem == "OrbStack" {
		return nil
	}

	opts := container.ListOptions{Filters: filters.NewArgs()}
	opts.Filters.Add("name", "tanjun-sysctl")

	containers, err := client.ContainerList(ctx, opts)

	if err != nil {
		return err
	}

	if len(containers) == 1 {
		return nil
	}

	if err := PullImageIfNotThere(ctx, client, "ghcr.io/shyim/tanjun/sysctl:v1"); err != nil {
		return err
	}

	cfg := &container.Config{
		Image: "ghcr.io/shyim/tanjun/sysctl:v1",
		Labels: map[string]string{
			"tanjun":                     "true",
			"com.docker.compose.project": "tanjun",
			"com.docker.compose.service": "sysctl",
		},
	}

	hostCfg := &container.HostConfig{
		Privileged: true,
		PidMode:    "host",
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	c, err := client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, "tanjun-sysctl")

	if err != nil {
		return err
	}

	return client.ContainerStart(ctx, c.ID, container.StartOptions{})
}
