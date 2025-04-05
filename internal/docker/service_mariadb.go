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

var supportedMariaDBConfiguration = []string{
	"sql_mode",
	"log_bin_trust_function_creators",
	"binlog_cache_size",
	"join_buffer_size",
	"innodb_log_file_size",
	"innodb_buffer_pool_size",
	"innodb_buffer_pool_instances",
	"group_concat_max_len",
	"max_connections",
	"max_allowed_packet",
	"max_binlog_size",
	"binlog_expire_logs_seconds",
}

type MariaDBService struct {
}

func (m MariaDBService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *container.InspectResponse) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Env = append(containerCfg.Env, "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD=yes", "MARIADB_DATABASE=database")

	hostCfg.Mounts = []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_%s_data", deployCfg.ContainerPrefix(), serviceName),
			Target: "/var/lib/mysql",
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
		Test: []string{"CMD", "mysqladmin", "ping", "-h", "localhost"},
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

func (m MariaDBService) AttachInfo(serviceName string, serviceCfg config.ProjectService) interface{} {
	return map[string]interface{}{
		"host":     serviceName,
		"port":     "3306",
		"username": "root",
		"password": "",
		"database": "database",
		"url":      fmt.Sprintf("mysql://root@%s:3306/database", serviceName),
		"go":       fmt.Sprintf("root:@tcp(%s:3306)/database", serviceName),
	}
}

func (m MariaDBService) Validate(serviceName string, serviceCfg config.ProjectService) error {
	for key := range serviceCfg.Settings {
		if !slices.Contains(supportedMariaDBConfiguration, key) {
			return fmt.Errorf("unsupported mariadb configuration key %s", key)
		}
	}

	return nil
}

func (m MariaDBService) SupportedTypes() []string {
	return []string{"mariadb:10.6", "mariadb:10.11", "mariadb:11.4"}
}

func (m MariaDBService) ConfigSchema(serviceType string) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	for _, key := range supportedMariaDBConfiguration {
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
	allServices = append(allServices, MariaDBService{})
}