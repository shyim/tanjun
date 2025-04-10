package docker

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func runHookInContainer(ctx context.Context, client *client.Client, deployCfg DeployConfiguration, hook string) error {
	containerName := fmt.Sprintf("%s_%d_hook", deployCfg.ContainerPrefix(), rand.IntN(1000000))

	containerCfg := &container.Config{
		Image:      deployCfg.ImageName,
		Env:        deployCfg.GetEnvironmentVariables(),
		Entrypoint: []string{"sh", "-c", hook},
		Cmd:        []string{},
		Labels: map[string]string{
			"com.docker.compose.project": deployCfg.ContainerPrefix(),
			"com.docker.compose.service": "hook",
			"tanjun":                     "true",
			"tanjun.project":             deployCfg.Name,
		},
	}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"tanjun-public": {},
			deployCfg.Name:  {},
		},
	}

	hostCfg := &container.HostConfig{}

	addAppServerVolumes(deployCfg, hostCfg)

	hookContainer, err := client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, containerName)

	if err != nil {
		return err
	}

	if err := client.ContainerStart(ctx, hookContainer.ID, container.StartOptions{}); err != nil {
		return err
	}

	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Minute*5))

	defer cancel()

	stdout, err := client.ContainerLogs(ctx, hookContainer.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})

	if err != nil {
		if err := client.ContainerRemove(ctx, hookContainer.ID, container.RemoveOptions{Force: true}); err != nil {
			return err
		}

		return err
	}

	defer func() {
		if err := stdout.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close stdout: %v\n", err)
		}
	}()

	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, stdout)

	if err != nil {
		if err := client.ContainerRemove(ctx, hookContainer.ID, container.RemoveOptions{Force: true}); err != nil {
			return err
		}

		return err
	}

	inspectedContainer, err := client.ContainerInspect(ctx, hookContainer.ID)

	if err != nil {
		return err
	}

	if err := client.ContainerRemove(ctx, hookContainer.ID, container.RemoveOptions{Force: true}); err != nil {
		return err
	}

	if inspectedContainer.State.ExitCode != 0 {
		return fmt.Errorf("hook failed with status code: %d", inspectedContainer.State.ExitCode)
	}

	return nil
}
