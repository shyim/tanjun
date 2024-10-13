package docker

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
)

type ValkeyService struct {
}

func (v ValkeyService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *types.ContainerJSON) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Healthcheck = &container.HealthConfig{
		Test: []string{"CMD", "valkey-cli", "ping"},
	}

	containerCfg.Image = "valkey/" + serviceConfig.Type

	containerCfg.Cmd = []string{"valkey-server"}

	for key, value := range serviceConfig.Settings {
		containerCfg.Cmd = append(containerCfg.Cmd, fmt.Sprintf("--%s=%s", key, value))
	}

	if existingContainer != nil {
		if slices.Compare(existingContainer.Config.Cmd, containerCfg.Cmd) == 0 {
			return nil
		}

		if err := client.ContainerStop(ctx, existingContainer.ID, container.StopOptions{Timeout: nil}); err != nil {
			return fmt.Errorf("failed to stop service %s (id: %s): %w", serviceName, existingContainer.ID, err)
		}

		if err := client.ContainerRemove(ctx, existingContainer.ID, container.RemoveOptions{}); err != nil {
			return fmt.Errorf("failed to delete service %s (id: %s): %w", serviceName, existingContainer.ID, err)
		}
	}

	return startService(ctx, client, serviceName, containerName, containerCfg, hostCfg, networkConfig)
}

func (v ValkeyService) AttachEnvironmentVariables(serviceName string, serviceConfig config.ProjectService) (map[string]string, error) {
	urlKey := fmt.Sprintf("%s_URL", strings.ToUpper(serviceName))

	return map[string]string{
		urlKey: fmt.Sprintf("redis://%s", serviceName),
	}, nil
}

func (v ValkeyService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	if serviceConfig.Type != "valkey:7.2" && serviceConfig.Type != "valkey:8.0" {
		return fmt.Errorf("service %s: invalid service type %s, must be valkey:7.2 or valkey:8.0", serviceName, serviceConfig.Type)
	}

	return nil
}
