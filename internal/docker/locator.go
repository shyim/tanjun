package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

func FindProjectContainer(ctx context.Context, client *client.Client, projectName, service string) (string, error) {
	listOptions := container.ListOptions{Filters: filters.NewArgs()}

	listOptions.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", projectName))

	if service != "" {
		listOptions.Filters.Add("label", fmt.Sprintf("tanjun.service=%s", service))
	} else {
		listOptions.Filters.Add("label", "tanjun.app=true")
	}

	containers, err := client.ContainerList(ctx, listOptions)

	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "", fmt.Errorf("no containers found")
	}

	return containers[0].ID, nil
}
