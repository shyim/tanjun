package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
)

type BlackfireService struct {
}

func (t BlackfireService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *container.InspectResponse) error {
	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(ctx, deployCfg, serviceName)

	containerCfg.Image = "blackfire/blackfire:2"

	if existingContainer != nil {
		return nil
	}

	return startService(ctx, client, serviceName, containerName, containerCfg, hostCfg, networkConfig)
}

func (t BlackfireService) AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{} {
	return map[string]interface{}{
		"url": fmt.Sprintf("tcp://%s:8307", serviceName),
	}
}

func (t BlackfireService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	return nil
}

func (t BlackfireService) SupportedTypes() []string {
	return []string{"blackfire:latest"}
}

func (t BlackfireService) ConfigSchema(serviceType string) *jsonschema.Schema {
	return &jsonschema.Schema{}
}

func init() {
	allServices = append(allServices, BlackfireService{})
}
