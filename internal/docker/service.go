package docker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/pterm/pterm"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"golang.org/x/sync/errgroup"
)

type AppService interface {
	Deploy(ctx context.Context, client *client.Client, serviceName string, deployCfg DeployConfiguration, existingContainer *types.ContainerJSON) error
	AttachInfo(serviceName string, serviceConfig config.ProjectService) interface{}
	Validate(serviceName string, serviceConfig config.ProjectService) error
	SupportedTypes() []string
	ConfigSchema(serviceType string) *jsonschema.Schema
}

var allServices []AppService

func GetAllServices() []AppService {
	return allServices
}

func newService(serviceType string, serviceConfig config.ProjectService) (AppService, error) {
	var svc AppService

	for _, s := range allServices {
		for _, supportedType := range s.SupportedTypes() {
			if supportedType == serviceType {
				svc = s
				break
			}
		}
	}

	if svc == nil {
		return nil, fmt.Errorf("service type %s not supported", serviceType)
	}

	if err := svc.Validate(serviceType, serviceConfig); err != nil {
		return nil, err
	}

	return svc, nil

}

func validateServices(deployCfg DeployConfiguration) error {
	for serviceName, serviceConfig := range deployCfg.ProjectConfig.Services {
		svc, err := newService(deployCfg.ProjectConfig.Services[serviceName].Type, serviceConfig)

		if err != nil {
			return err
		}

		if err := svc.Validate(deployCfg.ProjectConfig.Services[serviceName].Type, serviceConfig); err != nil {
			return err
		}
	}

	return nil
}

func startServices(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) error {
	if err := validateServices(deployCfg); err != nil {
		return err
	}

	options := container.ListOptions{
		Filters: filters.NewArgs(),
		All:     true,
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))
	options.Filters.Add("label", "tanjun.service")

	containers, err := client.ContainerList(ctx, options)

	if err != nil {
		return err
	}

	var wg errgroup.Group
	var configLock sync.Mutex

	for serviceName, serviceConfig := range deployCfg.ProjectConfig.Services {
		wg.Go(func(serviceName string, serviceConfig config.ProjectService) func() error {
			return func() error {
				svc, err := newService(deployCfg.ProjectConfig.Services[serviceName].Type, serviceConfig)

				if err != nil {
					return err
				}

				configLock.Lock()
				deployCfg.serviceConfig[serviceName] = svc.AttachInfo(serviceName, serviceConfig)
				configLock.Unlock()

				var containerId string

				for _, c := range containers {
					if c.Labels["tanjun.service"] == serviceName {
						containerId = c.ID
						break
					}
				}

				var existingContainer *types.ContainerJSON

				if containerId != "" {
					c, err := client.ContainerInspect(ctx, containerId)
					if err != nil {
						return err
					}

					existingContainer = &c
				}

				if err := svc.Deploy(ctx, client, serviceName, deployCfg, existingContainer); err != nil {
					return err
				}

				return nil
			}
		}(serviceName, serviceConfig))
	}

	return wg.Wait()
}

func getDefaultServiceContainers(cfg DeployConfiguration, name string) (string, *container.Config, *network.NetworkingConfig, *container.HostConfig) {
	containerName := fmt.Sprintf("%s_%s", cfg.ContainerPrefix(), name)

	containerCfg := &container.Config{
		Image: cfg.ProjectConfig.Services[name].Type,
		Env:   []string{},
		Labels: map[string]string{
			"com.docker.compose.project": cfg.ContainerPrefix(),
			"com.docker.compose.service": name,
			"tanjun":                     "true",
			"tanjun.project":             cfg.Name,
			"tanjun.service":             name,
		},
	}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			cfg.Name: {
				Aliases: []string{name},
			},
		},
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	return containerName, containerCfg, networkCfg, hostCfg
}

func startService(ctx context.Context, client *client.Client, name, containerName string, containerCfg *container.Config, hostCfg *container.HostConfig, networkCfg *network.NetworkingConfig) error {
	spinnerInfo, err := pterm.DefaultSpinner.Start(fmt.Sprintf("Starting service: %s", name))

	if err != nil {
		return err
	}

	if err := PullImageIfNotThere(ctx, client, containerCfg.Image); err != nil {
		return err
	}

	c, err := client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, containerName)

	if err != nil {
		return err
	}

	if err := client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return err
	}

	spinnerInfo.UpdateText(fmt.Sprintf("Started service: %s, waiting to be healhty", name))

	timeOut := 300

	for {
		containerInspect, err := client.ContainerInspect(ctx, c.ID)
		if err != nil {
			return err
		}

		if containerInspect.State.Health == nil {
			break
		}

		if containerInspect.State.Health != nil && containerInspect.State.Health.Status == types.Healthy {
			break
		}

		timeOut--

		time.Sleep(time.Second)

		if timeOut == 0 {
			spinnerInfo.Fail("Service did not start in time")
			return fmt.Errorf("service %s did not start in time", name)
		}
	}

	spinnerInfo.Success(fmt.Sprintf("Service %s is healthy", name))

	return nil
}

func stopAndRemoveContainer(ctx context.Context, client *client.Client, containerID string) error {
	if err := client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: nil}); err != nil {
		return fmt.Errorf("failed to stop container (id: %s): %w", containerID, err)
	}

	if err := client.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("failed to delete container (id: %s): %w", containerID, err)
	}

	return nil
}

type ProjectServiceList map[string]ProjectServiceInfo

func (p ProjectServiceList) HasDanlingServices() bool {
	for _, serviceInfo := range p {
		if serviceInfo.Dangling {
			return true
		}
	}

	return false
}

func (p ProjectServiceList) HasNotDeployedServices() bool {
	for _, serviceInfo := range p {
		if !serviceInfo.Existing {
			return true
		}
	}

	return false
}

type ProjectServiceInfo struct {
	Status   string
	Existing bool
	Dangling bool
}

func ProjectListServices(ctx context.Context, client *client.Client, cfg *config.ProjectConfig) (ProjectServiceList, error) {
	opts := container.ListOptions{Filters: filters.NewArgs(), All: true}

	opts.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", cfg.Name))
	opts.Filters.Add("label", "tanjun.service")

	containers, err := client.ContainerList(ctx, opts)

	if err != nil {
		return nil, err
	}

	serviceInfo := make(ProjectServiceList)

	for _, c := range containers {
		serviceName := c.Labels["tanjun.service"]

		_, shouldExists := cfg.Services[serviceName]

		serviceInfo[serviceName] = ProjectServiceInfo{
			Status:   c.State,
			Existing: true,
			Dangling: !shouldExists,
		}
	}

	for serviceName := range cfg.Services {
		if _, ok := serviceInfo[serviceName]; !ok {
			serviceInfo[serviceName] = ProjectServiceInfo{
				Status:   "missing, not deployed yet",
				Existing: false,
				Dangling: false,
			}
		}
	}

	return serviceInfo, nil
}

func ProjectDeleteService(ctx context.Context, client *client.Client, cfg *config.ProjectConfig, serviceName string) error {
	opts := container.ListOptions{Filters: filters.NewArgs(), All: true}

	opts.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", cfg.Name))
	opts.Filters.Add("label", fmt.Sprintf("tanjun.service=%s", serviceName))

	containers, err := client.ContainerList(ctx, opts)

	if err != nil {
		return err
	}

	for _, c := range containers {
		if err := client.ContainerKill(ctx, c.ID, "SIGKILL"); err != nil {
			return err
		}

		if err := client.ContainerRemove(ctx, c.ID, container.RemoveOptions{}); err != nil {
			return err
		}
	}

	volumeOpts := volume.ListOptions{Filters: filters.NewArgs()}
	volumeOpts.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", cfg.Name))
	volumeOpts.Filters.Add("label", fmt.Sprintf("tanjun.service=%s", serviceName))

	volumes, err := client.VolumeList(ctx, volumeOpts)

	if err != nil {
		return err
	}

	for _, v := range volumes.Volumes {
		if err := client.VolumeRemove(ctx, v.Name, true); err != nil {
			return err
		}
	}

	return nil
}
