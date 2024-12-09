package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/shyim/tanjun/internal/config"
	"os"
)

func RunCronjobCommand(ctx context.Context, client *client.Client, config *config.ProjectConfig, args []string) error {
	opts := container.ListOptions{
		Filters: filters.NewArgs(),
	}

	opts.Filters.Add("label", "tanjun.project="+config.Name)
	opts.Filters.Add("label", "tanjun.cronjob=scheduler")

	containers, err := client.ContainerList(ctx, opts)

	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("no scheduler container found for project %s, did you configured cronjobs", config.Name)
	}

	for _, c := range containers {
		exec, err := client.ContainerExecCreate(ctx, c.ID, container.ExecOptions{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          append([]string{"/scheduler"}, args...),
		})

		if err != nil {
			return err
		}

		resp, err := client.ContainerExecAttach(ctx, exec.ID, container.ExecStartOptions{})

		if err != nil {
			return err
		}

		defer resp.Close()

		if _, err = stdcopy.StdCopy(os.Stdout, os.Stderr, resp.Reader); err != nil {
			return err
		}

		inspect, err := client.ContainerExecInspect(ctx, exec.ID)

		if err != nil {
			return err
		}

		if inspect.ExitCode != 0 {
			return fmt.Errorf("command failed with exit code %d", inspect.ExitCode)
		}
	}

	return nil
}
