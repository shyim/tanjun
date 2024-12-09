package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shyim/tanjun/internal/config"
	"math/rand/v2"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

func startCronjobs(ctx context.Context, client *client.Client, deployConfig DeployConfiguration) error {
	if len(deployConfig.ProjectConfig.App.Cronjobs) == 0 {
		return nil
	}

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

	c, err := client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, fmt.Sprintf("%s-scheduler-%d", deployConfig.ContainerPrefix(), rand.IntN(1000000)))

	if err != nil {
		return err
	}

	if err := client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return err
	}

	var schedulerConfig = struct {
		ContainerID string                  `json:"container_id"`
		Jobs        []config.ProjectCronjob `json:"jobs"`
	}{ContainerID: c.ID, Jobs: deployConfig.ProjectConfig.App.Cronjobs}

	schedulerConfigStr, err := json.Marshal(schedulerConfig)

	if err != nil {
		return err
	}

	return startScheduler(ctx, client, deployConfig, string(schedulerConfigStr))
}

func startScheduler(ctx context.Context, client *client.Client, deployConfig DeployConfiguration, schedulerConfig string) error {
	if err := PullImageIfNotThere(ctx, client, "ghcr.io/shyim/tanjun/scheduler:v1"); err != nil {
		return err
	}

	cfg := &container.Config{
		Image: "ghcr.io/shyim/tanjun/scheduler:v1",
		Labels: map[string]string{
			"com.docker.compose.project": deployConfig.ContainerPrefix(),
			"com.docker.compose.service": "scheduler",
			"tanjun":                     "true",
			"tanjun.project":             deployConfig.Name,
			"tanjun.cronjob":             "scheduler",
		},
		Env: []string{"SCHEDULER_CONFIG=" + schedulerConfig},
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
			{
				Type:   mount.TypeVolume,
				Source: deployConfig.ContainerPrefix() + "_scheduler_data",
				Target: "/data",
				VolumeOptions: &mount.VolumeOptions{
					Labels: map[string]string{
						"tanjun":         "true",
						"tanjun.project": deployConfig.Name,
					},
				},
			},
		},
	}

	c, err := client.ContainerCreate(ctx, cfg, hostCfg, nil, nil, fmt.Sprintf("%s-scheduler-%d", deployConfig.ContainerPrefix(), rand.IntN(1000000)))

	if err != nil {
		return err
	}

	return client.ContainerStart(ctx, c.ID, container.StartOptions{})
}
