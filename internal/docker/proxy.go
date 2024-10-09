package docker

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func configureKamalService(ctx context.Context, client *client.Client, cmd []string) error {
	opts := container.ListOptions{
		Filters: filters.NewArgs(),
	}

	opts.Filters.Add("name", kamalContainerName)

	containers, err := client.ContainerList(ctx, opts)

	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("kamal proxy container not found")
	}

	exec, err := client.ContainerExecCreate(ctx, containers[0].ID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})

	if err != nil {
		return err
	}

	resp, err := client.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})

	if err != nil {
		return err
	}

	defer resp.Close()

	reader := bufio.NewScanner(resp.Reader)
	reader.Scan()

	if reader.Text() == "" {
		return nil
	}

	return fmt.Errorf("kamal proxy failed: %s", reader.Text())
}
