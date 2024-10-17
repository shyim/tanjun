package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type OpenSearchService struct {
}

func (v OpenSearchService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *types.ContainerJSON) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Image = fmt.Sprintf("opensearchproject/%s", serviceConfig.Type)
	containerCfg.Env = append(
		containerCfg.Env,
		"discovery.type=single-node",
		"OPENSEARCH_INITIAL_ADMIN_PASSWORD=c3o_ZPHo!",
		"plugins.security.disabled=true",
	)

	hostCfg.Mounts = []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_%s_data", deployCfg.ContainerPrefix(), serviceName),
			Target: "/usr/share/opensearch/data",
			VolumeOptions: &mount.VolumeOptions{
				Labels: map[string]string{
					"tanjun":         "true",
					"tanjun.project": deployCfg.Name,
					"tanjun.service": serviceName,
				},
			},
		},
	}

	containerCfg.Healthcheck = &container.HealthConfig{
		Test: []string{"CMD", "curl", "-f", "localhost:9200"},
	}

	if existingContainer != nil {
		if existingContainer.Config.Image == containerCfg.Image {
			return nil
		}

		if err := stopAndRemoveContainer(ctx, client, existingContainer.ID); err != nil {
			return fmt.Errorf("failed to stop and remove service %s (id: %s): %w", serviceName, existingContainer.ID, err)
		}
	}

	return startService(ctx, client, serviceName, containerName, containerCfg, hostCfg, networkConfig)
}

func (v OpenSearchService) AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{} {
	return map[string]interface{}{
		"host": serviceName,
		"port": "9200",
		"url":  fmt.Sprintf("http://%s:9200", serviceName),
	}
}

func (v OpenSearchService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	return nil
}

func (v OpenSearchService) SupportedTypes() []string {
	return []string{"opensearch:2.17.1"}
}

func (v OpenSearchService) ConfigSchema(serviceType string) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func init() {
	allServices = append(allServices, OpenSearchService{})
}
