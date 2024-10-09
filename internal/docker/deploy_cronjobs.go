package docker

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"golang.org/x/sync/errgroup"
)

func startCronjobs(ctx context.Context, client *client.Client, deployConfig DeployConfiguration) error {
	if len(deployConfig.ProjectConfig.App.Cronjobs) == 0 {
		return nil
	}

	var errgroup errgroup.Group

	errgroup.Go(func() error {
		return startOfelia(ctx, client, deployConfig)
	})

	containerCfg, hostCfg, networkCfg := getAppContainerConfiguration(deployConfig)

	containerCfg.Labels = map[string]string{
		"com.docker.compose.project": deployConfig.ContainerPrefix(),
		"tanjun":                     "true",
		"tanjun.project":             deployConfig.Name,
		"tanjun.cronjob":             "app",
		"ofelia.enabled":             "true",
	}

	containerCfg.Entrypoint = []string{"sh"}
	containerCfg.Cmd = []string{}
	containerCfg.Tty = true

	for i, cronjob := range deployConfig.ProjectConfig.App.Cronjobs {
		scheduleName := fmt.Sprintf("ofelia.job-exec.tanjun-cronjob-%d.schedule", i)
		commandName := fmt.Sprintf("ofelia.job-exec.tanjun-cronjob-%d.command", i)

		containerCfg.Labels[scheduleName] = cronjob.Schedule
		containerCfg.Labels[commandName] = cronjob.Command
		containerCfg.Labels["com.docker.compose.service"] = fmt.Sprintf("cronjob-%d", i)
	}

	errgroup.Go(func() error {
		c, err := client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, fmt.Sprintf("%s-ofelia-%d", deployConfig.ContainerPrefix(), rand.IntN(1000000)))

		if err != nil {
			return err
		}

		return client.ContainerStart(ctx, c.ID, container.StartOptions{})
	})

	return errgroup.Wait()
}

func startOfelia(ctx context.Context, client *client.Client, deployConfig DeployConfiguration) error {
	if err := PullImageIfNotThere(ctx, client, "mcuadros/ofelia:latest"); err != nil {
		return err
	}

	cfg := &container.Config{
		Image: "mcuadros/ofelia:latest",
		Cmd:   []string{"daemon", "--docker", "-f", fmt.Sprintf("label=tanjun.project=%s", deployConfig.Name)},
		Labels: map[string]string{
			"com.docker.compose.project": deployConfig.ContainerPrefix(),
			"com.docker.compose.service": "ofelia",
			"tanjun":                     "true",
			"tanjun.project":             deployConfig.Name,
			"tanjun.cronjob":             "ofelia",
		},
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/run/docker.sock",
				Target: "/var/run/docker.sock",
			},
		},
	}

	c, err := client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, fmt.Sprintf("%s-ofelia-%d", deployConfig.ContainerPrefix(), rand.IntN(1000000)))

	if err != nil {
		return err
	}

	return client.ContainerStart(ctx, c.ID, container.StartOptions{})
}
