package docker

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
)

var validMySQLVersions = []string{"mysql:8.0", "mysql:8.4"}
var supportedMySQLConfiguration = []string{
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

type MySQLService struct {
}

func (m MySQLService) Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *types.ContainerJSON) error {
	serviceConfig := deployCfg.ProjectConfig.Services[serviceName]

	containerName, containerCfg, networkConfig, hostCfg := getDefaultServiceContainers(deployCfg, serviceName)

	containerCfg.Env = append(containerCfg.Env, "MYSQL_ALLOW_EMPTY_PASSWORD=yes", "MYSQL_DATABASE=database")

	hostCfg.Mounts = []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_%s_data", deployCfg.ContainerPrefix(), serviceName),
			Target: "/var/lib/mysql",
			VolumeOptions: &mount.VolumeOptions{
				Labels: map[string]string{
					"tanjun":         "true",
					"tanjun.project": fmt.Sprintf("%s", deployCfg.Name),
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

		if err := client.ContainerStop(ctx, existingContainer.ID, container.StopOptions{Timeout: nil}); err != nil {
			return fmt.Errorf("failed to stop service %s (id: %s): %w", serviceName, existingContainer.ID, err)
		}

		if err := client.ContainerRemove(ctx, existingContainer.ID, container.RemoveOptions{}); err != nil {
			return fmt.Errorf("failed to delete service %s (id: %s): %w", serviceName, existingContainer.ID, err)
		}
	}

	return startService(ctx, client, serviceName, containerName, containerCfg, hostCfg, networkConfig)
}

func (m MySQLService) AttachEnvironmentVariables(serviceName string, serviceCfg config.ProjectService) (map[string]string, error) {
	urlKey := fmt.Sprintf("%s_URL", strings.ToUpper(serviceName))

	return map[string]string{
		urlKey: fmt.Sprintf("mysql://root@%s:3306/database", serviceName),
	}, nil
}

func (m MySQLService) Validate(serviceName string, serviceCfg config.ProjectService) error {
	if !slices.Contains(validMySQLVersions, serviceName) {
		return fmt.Errorf("not supported mysql version %s", serviceName)
	}

	for key := range serviceCfg.Settings {
		if !slices.Contains(supportedMySQLConfiguration, key) {
			return fmt.Errorf("unsupported mysql configuration key %s", key)
		}
	}

	return nil
}
