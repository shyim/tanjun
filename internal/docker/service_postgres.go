package docker

import (
	"context"
	"fmt"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

var supportedPostgresConfiguration = []string{
	"max_connections",
	"shared_buffers",
	"effective_cache_size",
	"maintenance_work_mem",
	"checkpoint_completion_target",
	"wal_buffers",
	"default_statistics_target",
	"random_page_cost",
	"effective_io_concurrency",
	"work_mem",
	"min_wal_size",
	"max_wal_size",
	"max_worker_processes",
	"max_parallel_workers_per_gather",
	"max_parallel_workers",
}

type PostgresService struct {
}

func (p PostgresService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *container.InspectResponse) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Image = "postgres:alpine"
	containerCfg.Env = append(containerCfg.Env, "POSTGRES_DB=database", "POSTGRES_USER=user", "POSTGRES_PASSWORD=password")
	containerCfg.Cmd = []string{"postgres"}

	hostCfg.Mounts = []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_%s_data", deployCfg.ContainerPrefix(), serviceName),
			Target: "/var/lib/postgresql/data",
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
		Test: []string{"CMD-SHELL", "pg_isready -U user"},
	}

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

func (p PostgresService) AttachInfo(serviceName string, serviceCfg config.ProjectService) interface{} {
	return map[string]interface{}{
		"host":     serviceName,
		"port":     "5432",
		"username": "user",
		"password": "password",
		"database": "database",
		"url":      fmt.Sprintf("postgres://user:password@%s:5432/database", serviceName),
		"go":       fmt.Sprintf("user:password@tcp(%s:5432)/database", serviceName),
	}
}

func (p PostgresService) Validate(serviceName string, serviceCfg config.ProjectService) error {
	for key := range serviceCfg.Settings {
		if !slices.Contains(supportedPostgresConfiguration, key) {
			return fmt.Errorf("unsupported postgres configuration key %s", key)
		}
	}

	return nil
}

func (p PostgresService) SupportedTypes() []string {
	return []string{"postgres:17", "postgres:16", "postgres:15", "postgres:14"}
}

func (p PostgresService) ConfigSchema(serviceType string) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	for _, key := range supportedPostgresConfiguration {
		properties.Set(key, &jsonschema.Schema{
			Type: "string",
		})
	}

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func init() {
	allServices = append(allServices, PostgresService{})
}
