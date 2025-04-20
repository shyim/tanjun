package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
)

type TidewaysService struct {
}

func (t TidewaysService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *container.InspectResponse) error {
	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(ctx, deployCfg, serviceName)

	containerCfg.Image = "ghcr.io/tideways/daemon"

	if existingContainer != nil {
		return nil
	}

	return startService(ctx, client, serviceName, containerName, containerCfg, hostCfg, networkConfig)
}

func (t TidewaysService) AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{} {
	return map[string]interface{}{
		"url": fmt.Sprintf("tcp://%s:9135", serviceName),
	}
}

func (t TidewaysService) Validate(serviceName string, serviceConfig config.ProjectService) error {
	return nil
}

func (t TidewaysService) SupportedTypes() []string {
	return []string{"tideways:latest"}
}

func (t TidewaysService) ConfigSchema(serviceType string) *jsonschema.Schema {
	return &jsonschema.Schema{}
}

func init() {
	allServices = append(allServices, TidewaysService{})
}
