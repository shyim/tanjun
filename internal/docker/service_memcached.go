package docker

import (
	"context"
	"fmt"
	"slices"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type MemcachedService struct {
}

func (m MemcachedService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *types.ContainerJSON) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Image = "memcached:alpine"

	for key, value := range serviceConfig.Settings {
		containerCfg.Cmd = append(containerCfg.Cmd, fmt.Sprintf("-%s %s", key, value))
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

func (m MemcachedService) AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{} {
	return map[string]interface{}{
		"host": serviceName,
		"port": "11211",
		"url":  fmt.Sprintf("memcached://%s:11211", serviceName),
	}
}

func (m MemcachedService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	if serviceConfig.Type != "memcached:latest" {
		return fmt.Errorf("service %s: invalid service type %s, must be memcached:latest", serviceName, serviceConfig.Type)
	}

	return nil
}

func (m MemcachedService) SupportedTypes() []string {
	return []string{"memcached:latest"}
}

func (m MemcachedService) ConfigSchema(serviceType string) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	properties.Set("memory-limit", &jsonschema.Schema{
		Type:        "string",
		Description: "The maximum amount of memory the server is allowed to use for item storage.",
	})

	properties.Set("max-connections", &jsonschema.Schema{
		Type:        "integer",
		Description: "The maximum number of simultaneous connections to the server.",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func init() {
	allServices = append(allServices, MemcachedService{})
}
