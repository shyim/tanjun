package docker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	filters2 "github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"golang.org/x/sync/errgroup"
)

type AppService interface {
	Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *types.ContainerJSON) error
	AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{}
	Validate(serviceName string, serviceConfig config.ProjectService) error
}

func newService(serviceType string, serviceConfig config.ProjectService) (AppService, error) {
	var svc AppService

	if strings.HasPrefix(serviceType, "mysql:") {
		svc = &MySQLService{}
	} else if strings.HasPrefix(serviceType, "valkey:") {
		svc = &ValkeyService{}
	} else {
		return nil, fmt.Errorf("service type %s not supported", serviceType)
	}

	if err := svc.Validate(serviceType, serviceConfig); err != nil {
		return nil, err
	}

	return svc, nil

}

func validateServices(deployCfg DeployConfiguration) error {
	for serviceName, serviceConfig := range deployCfg.ProjectConfig.Services {
		svc, err := newService(deployCfg.ProjectConfig.Services[serviceName].Type, serviceConfig)

		if err != nil {
			return err
		}

		if err := svc.Validate(deployCfg.ProjectConfig.Services[serviceName].Type, serviceConfig); err != nil {
			return err
		}
	}

	return nil
}

func startServices(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) error {
	if err := validateServices(deployCfg); err != nil {
		return err
	}

	options := container.ListOptions{
		Filters: filters2.NewArgs(),
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))
	options.Filters.Add("label", "tanjun.service")

	containers, err := client.ContainerList(ctx, options)

	if err != nil {
		return err
	}

	var wg errgroup.Group
	var configLock sync.Mutex

	for serviceName, serviceConfig := range deployCfg.ProjectConfig.Services {
		wg.Go(func(serviceName string, serviceConfig config.ProjectService) func() error {
			return func() error {
				svc, err := newService(deployCfg.ProjectConfig.Services[serviceName].Type, serviceConfig)

				if err != nil {
					return err
				}

				configLock.Lock()
				deployCfg.serviceConfig[serviceName] = svc.AttachInfo(serviceName, serviceConfig)
				configLock.Unlock()

				var containerId string

				for _, c := range containers {
					if c.Labels["tanjun.service"] == serviceName {
						containerId = c.ID
						break
					}
				}

				var existingContainer *types.ContainerJSON

				if containerId != "" {
					c, err := client.ContainerInspect(ctx, containerId)
					if err != nil {
						return err
					}

					existingContainer = &c
				}

				if err := svc.Deploy(ctx, client, serviceName, deployCfg, existingContainer); err != nil {
					return err
				}

				return nil
			}
		}(serviceName, serviceConfig))
	}

	return wg.Wait()
}

func getDefaultServiceContainers(cfg DeployConfiguration, name string) (string, *container.Config, *network.NetworkingConfig, *container.HostConfig) {
	containerName := fmt.Sprintf("%s_%s", cfg.ContainerPrefix(), name)

	containerCfg := &container.Config{
		Image: cfg.ProjectConfig.Services[name].Type,
		Env:   []string{},
		Labels: map[string]string{
			"com.docker.compose.project": cfg.ContainerPrefix(),
			"com.docker.compose.service": name,
			"tanjun":                     "true",
			"tanjun.project":             cfg.Name,
			"tanjun.service":             name,
		},
	}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			cfg.Name: {
				Aliases: []string{name},
			},
		},
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	return containerName, containerCfg, networkCfg, hostCfg
}

func startService(ctx context.Context, client *client.Client, name, containerName string, containerCfg *container.Config, hostCfg *container.HostConfig, networkCfg *network.NetworkingConfig) error {
	if err := PullImageIfNotThere(ctx, client, containerCfg.Image); err != nil {
		return err
	}

	c, err := client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, containerName)

	if err != nil {
		return err
	}

	if err := client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return err
	}

	log.Infof("Started service: %s, waiting to be healthy", name)
	timeOut := 300

	for {
		containerInspect, err := client.ContainerInspect(ctx, c.ID)
		if err != nil {
			return err
		}

		if containerInspect.State.Health != nil && containerInspect.State.Health.Status == types.Healthy {
			break
		}

		timeOut--

		time.Sleep(time.Second)

		if timeOut == 0 {
			return fmt.Errorf("service %s did not start in time", name)
		}
	}

	log.Infof("Service %s is healthy", name)

	return nil
}
