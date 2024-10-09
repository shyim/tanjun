package docker

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"golang.org/x/sync/errgroup"
)

func startWorkers(ctx context.Context, client *client.Client, deployConfig DeployConfiguration) error {
	var errgroup errgroup.Group

	for workerName, worker := range deployConfig.ProjectConfig.App.Workers {
		worker := worker
		workerName := workerName
		errgroup.Go(func() error {
			return startWorker(ctx, client, deployConfig, worker, workerName)
		})
	}

	return errgroup.Wait()
}

func startWorker(ctx context.Context, client *client.Client, deployCfg DeployConfiguration, worker config.ProjectWorker, workerName string) error {
	if worker.Replicas == 0 {
		worker.Replicas = 1
	}

	var errgroup errgroup.Group

	for i := 0; i < worker.Replicas; i++ {
		i := i
		errgroup.Go(func() error {
			containerConfig, hostConfig, networkConfig := getAppContainerConfiguration(deployCfg)

			containerConfig.Entrypoint = []string{"sh", "-c"}
			containerConfig.Cmd = []string{worker.Command}

			containerConfig.Labels = map[string]string{
				"com.docker.compose.project": deployCfg.ContainerPrefix(),
				"com.docker.compose.service": workerName,
				"tanjun":                     "true",
				"tanjun.worker":              workerName,
				"tanjun.project":             fmt.Sprintf("%s", deployCfg.Name),
			}

			hostConfig.RestartPolicy = container.RestartPolicy{
				Name: container.RestartPolicyUnlessStopped,
			}

			c, err := client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, fmt.Sprintf("%s_%s_%d_%d", deployCfg.ContainerPrefix(), workerName, i, rand.IntN(1000000)))
			if err != nil {
				return err
			}

			if err := client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
				return err
			}

			return nil
		})
	}

	return errgroup.Wait()
}
