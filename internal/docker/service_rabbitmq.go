package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type RabbitmqService struct {
}

func (v RabbitmqService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *container.InspectResponse) error {
	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Image = "rabbitmq:4-management-alpine"

	hostCfg.Mounts = []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_%s_data", deployCfg.ContainerPrefix(), serviceName),
			Target: "/var/lib/rabbitmq",
			VolumeOptions: &mount.VolumeOptions{
				Labels: map[string]string{
					"tanjun":         "true",
					"tanjun.project": deployCfg.Name,
					"tanjun.service": serviceName,
				},
			},
		},
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

func (v RabbitmqService) AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{} {
	return map[string]interface{}{
		"host":     serviceName,
		"port":     "5672",
		"username": "guest",
		"password": "guest",
		"url":      fmt.Sprintf("amqp://%s:5672", serviceName),
	}
}

func (v RabbitmqService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	return nil
}

func (v RabbitmqService) SupportedTypes() []string {
	return []string{"rabbitmq:4"}
}

func (v RabbitmqService) ConfigSchema(serviceType string) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func init() {
	allServices = append(allServices, RabbitmqService{})
}
