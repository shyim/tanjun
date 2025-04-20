package docker

import (
	"context"
	"fmt"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type ValkeyService struct {
}

func (v ValkeyService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *container.InspectResponse) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(ctx, deployCfg, serviceName)

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

		if err := stopAndRemoveContainer(ctx, client, existingContainer.ID); err != nil {
			return fmt.Errorf("failed to stop and remove service %s (id: %s): %w", serviceName, existingContainer.ID, err)
		}
	}

	return startService(ctx, client, serviceName, containerName, containerCfg, hostCfg, networkConfig)
}

func (v ValkeyService) AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{} {
	return map[string]interface{}{
		"host": serviceName,
		"port": "6379",
		"url":  fmt.Sprintf("redis://%s:6379", serviceName),
	}
}

func (v ValkeyService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	return nil
}

func (v ValkeyService) SupportedTypes() []string {
	return []string{"valkey:7.2", "valkey:8.0"}
}

func (v ValkeyService) ConfigSchema(serviceType string) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()
	properties.Set("maxmemory", &jsonschema.Schema{
		Type:        "string",
		Description: "The value can be an absolute number (bytes), a percentage of the available memory, or a percentage of the memory limit.",
	})

	properties.Set("maxmemory-policy", &jsonschema.Schema{
		Type:        "string",
		Enum:        []interface{}{"allkeys-lru", "allkeys-lfu", "allkeys-random", "volatile-lru", "volatile-lfu", "volatile-random", "volatile-ttl", "noeviction"},
		Description: "How Valkey will select what to remove when maxmemory is reached. ",
	})

	properties.Set("appendonly", &jsonschema.Schema{
		Type:        "string",
		Enum:        []interface{}{"yes", "no"},
		Description: "By default, Valkey will not persist data to disk. ",
	})

	properties.Set("save", &jsonschema.Schema{
		Type:        "string",
		Description: "Save the DB to disk.",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func init() {
	allServices = append(allServices, ValkeyService{})
}
