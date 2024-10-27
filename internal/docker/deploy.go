package docker

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"math/rand/v2"

	"github.com/charmbracelet/log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
)

func newDeployConfiguration(config *config.ProjectConfig, version string) DeployConfiguration {
	return DeployConfiguration{
		Name:                 config.Name,
		ImageName:            version,
		ProjectConfig:        config,
		environmentVariables: make(map[string]string),
		serviceConfig:        make(map[string]interface{}),
	}
}

type DeployConfiguration struct {
	Name                 string
	ImageName            string
	ProjectConfig        *config.ProjectConfig
	environmentVariables map[string]string
	storage              *KvClient
	serviceConfig        map[string]interface{}
	imageConfig          *container.Config
}

func (c DeployConfiguration) ContainerPrefix() string {
	return fmt.Sprintf("tanjun_%s", c.Name)
}

func (c DeployConfiguration) GetEnvironmentVariables() []string {
	var env []string

	for key, value := range c.environmentVariables {
		env = append(env, key+"="+value)
	}

	return env
}

func getEnvironmentContainers(ctx context.Context, client *client.Client, projectName string) ([]types.Container, error) {
	options := container.ListOptions{
		Filters: filters.NewArgs(),
		All:     true,
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", projectName))
	options.Filters.Add("label", "tanjun.app=true")

	return client.ContainerList(ctx, options)
}

func getWorkerEnvironmentContainers(ctx context.Context, client *client.Client, projectName string) ([]types.Container, error) {
	options := container.ListOptions{
		Filters: filters.NewArgs(),
		All:     true,
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", projectName))
	options.Filters.Add("label", "tanjun.worker")

	return client.ContainerList(ctx, options)
}

func getCronjobEnvironmentContainers(ctx context.Context, client *client.Client, projectName string) ([]types.Container, error) {
	options := container.ListOptions{
		Filters: filters.NewArgs(),
		All:     true,
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", projectName))
	options.Filters.Add("label", "tanjun.cronjob")

	return client.ContainerList(ctx, options)
}

func getAppContainerConfiguration(deployCfg DeployConfiguration) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	containerCfg := &container.Config{
		Image: deployCfg.ImageName,
		Labels: map[string]string{
			"traefik.enable": "true",

			"com.docker.compose.project": deployCfg.ContainerPrefix(),
			"com.docker.compose.service": "web",

			"tanjun":         "true",
			"tanjun.app":     "true",
			"tanjun.project": deployCfg.Name,
		},
		Env: deployCfg.GetEnvironmentVariables(),
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
	}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"tanjun-public": {},
			deployCfg.Name:  {},
		},
	}

	addAppServerVolumes(deployCfg, hostCfg)

	return containerCfg, hostCfg, networkCfg
}

func Deploy(ctx context.Context, client *client.Client, projectConfig *config.ProjectConfig, version string) error {
	deployCfg := newDeployConfiguration(projectConfig, fmt.Sprintf("%s:%s", projectConfig.Image, version))

	if err := PullImageIfNotThere(ctx, client, deployCfg.ImageName); err != nil {
		return err
	}

	image, _, err := client.ImageInspectWithRaw(ctx, deployCfg.ImageName)

	if err != nil {
		return err
	}

	deployCfg.imageConfig = image.Config

	deployCfg.storage, err = CreateKVConnection(ctx, client)

	if err != nil {
		return err
	}

	defer deployCfg.storage.Close()

	if err := createEnvironmentNetwork(ctx, client, deployCfg); err != nil {
		return err
	}

	beforeContainers, err := getEnvironmentContainers(ctx, client, deployCfg.Name)

	if err != nil {
		return err
	}

	if err := createAppServerVolumes(ctx, client, deployCfg); err != nil {
		return err
	}

	if err := startServices(ctx, client, deployCfg); err != nil {
		return err
	}

	if err := prepareEnvironmentVariables(ctx, deployCfg); err != nil {
		return err
	}

	beforeWorkers, err := getWorkerEnvironmentContainers(ctx, client, deployCfg.Name)

	if err != nil {
		return err
	}

	beforeCronjobs, err := getCronjobEnvironmentContainers(ctx, client, deployCfg.Name)

	if err != nil {
		return err
	}

	allSideContainers := append(beforeWorkers, beforeCronjobs...)

	if len(allSideContainers) > 0 {
		spinnerInfo, err := pterm.DefaultSpinner.Start("Draining old side containers like workers and cronjobs")

		if err != nil {
			return err
		}

		if err := stopContainers(ctx, client, allSideContainers); err != nil {
			return err
		}

		spinnerInfo.Success("Drained old side containers")
	}

	if len(deployCfg.ProjectConfig.App.Hooks.Deploy) > 0 {
		log.Infof("Running deploy hook")
		if err := runHookInContainer(ctx, client, deployCfg, deployCfg.ProjectConfig.App.Hooks.Deploy); err != nil {
			return err
		}
	}

	containerName := fmt.Sprintf("%s_app_%d", deployCfg.ContainerPrefix(), rand.IntN(1000000))

	containerCfg, hostCfg, networkCfg := getAppContainerConfiguration(deployCfg)

	resp, err := client.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, containerName)

	if err != nil {
		return err
	}

	if err := client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	containerInspect, err := client.ContainerInspect(ctx, resp.ID)

	if err != nil {
		return err
	}

	spinnerInfo, err := pterm.DefaultSpinner.Start("Routing new traffic to new container")

	if err != nil {
		return err
	}

	proxyHost := containerInspect.NetworkSettings.Networks["tanjun-public"].IPAddress
	proxyPort := findPortMapping(deployCfg, containerInspect)

	kamalCmd := []string{
		"kamal-proxy",
		"deploy",
		"--host", deployCfg.ProjectConfig.Proxy.Host,
		"--forward-headers",
		"--health-check-path", deployCfg.ProjectConfig.Proxy.HealthCheck.Path,
		"--health-check-interval", fmt.Sprintf("%ds", deployCfg.ProjectConfig.Proxy.HealthCheck.Interval),
		"--health-check-timeout", fmt.Sprintf("%ds", deployCfg.ProjectConfig.Proxy.HealthCheck.Timeout),
		"--target", fmt.Sprintf("%s:%s", proxyHost, proxyPort), deployCfg.Name,
		"--target-timeout", fmt.Sprintf("%ds", deployCfg.ProjectConfig.Proxy.ResponseTimeout),
	}

	if deployCfg.ProjectConfig.Proxy.SSL {
		kamalCmd = append(kamalCmd, "--tls")
	}

	if deployCfg.ProjectConfig.Proxy.Buffering.Requests {
		kamalCmd = append(kamalCmd, "--buffer-requests")
	}

	if deployCfg.ProjectConfig.Proxy.Buffering.Responses {
		kamalCmd = append(kamalCmd, "--buffer-responses")
	}

	if deployCfg.ProjectConfig.Proxy.Buffering.MaxRequestBody > 0 {
		kamalCmd = append(kamalCmd, "--max-request-body", fmt.Sprintf("%d", deployCfg.ProjectConfig.Proxy.Buffering.MaxRequestBody))
	}

	if deployCfg.ProjectConfig.Proxy.Buffering.MaxResponseBody > 0 {
		kamalCmd = append(kamalCmd, "--max-response-body", fmt.Sprintf("%d", deployCfg.ProjectConfig.Proxy.Buffering.MaxResponseBody))
	}

	if deployCfg.ProjectConfig.Proxy.Buffering.Memory > 0 {
		kamalCmd = append(kamalCmd, "--buffer-memory", fmt.Sprintf("%d", deployCfg.ProjectConfig.Proxy.Buffering.Memory))
	}

	removalContainers := append(beforeContainers, beforeWorkers...)

	if err := configureKamalService(ctx, client, kamalCmd); err != nil {
		spinnerInfo.Fail(err)

		if len(removalContainers) > 0 {
			// If we fail to configure kamal, we should stop the container and remove it
			if restoreErr := client.ContainerKill(ctx, resp.ID, "SIGKILL"); restoreErr != nil {
				return fmt.Errorf("kamal configure failed: %w and could not stop the new container: %s", err, restoreErr)
			}

			if restoreErr := client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); restoreErr != nil {
				return fmt.Errorf("kamal configure failed: %w and could not remove the new container: %s", err, restoreErr)
			}

			if restoreErr := startContainers(ctx, client, removalContainers); restoreErr != nil {
				return fmt.Errorf("kamal configure failed: %w and could not start the old workers / cronjobs: %s", err, restoreErr)
			}
		}

		return err
	}

	spinnerInfo.Success("Routing new traffic to new container successful")

	if err := removeContainers(ctx, client, append(removalContainers, beforeCronjobs...)); err != nil {
		return err
	}

	if err := startWorkers(ctx, client, deployCfg); err != nil {
		return err
	}

	if err := startCronjobs(ctx, client, deployCfg); err != nil {
		return err
	}

	if len(deployCfg.ProjectConfig.App.Hooks.PostDeploy) > 0 {
		log.Infof("Running post deploy hook")
		if err := runHookInContainer(ctx, client, deployCfg, deployCfg.ProjectConfig.App.Hooks.PostDeploy); err != nil {
			return err
		}
	}

	log.Infof("Deployed successful, website is reachable at %s", deployCfg.ProjectConfig.Proxy.GetURL())

	if len(beforeContainers) > 0 {
		log.Infof("You can rollback to the previous version with tanjun deploy --rollback")
	}

	return VersionDrain(ctx, client, deployCfg.ProjectConfig)
}

func createAppServerVolumes(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) error {
	if len(deployCfg.ProjectConfig.App.Mounts) == 0 {
		return nil
	}

	if err := PullImageIfNotThere(ctx, client, "alpine:latest"); err != nil {
		return err
	}

	userId, err := determineUidOfAppContainer(ctx, client, deployCfg.ImageName)

	if err != nil {
		log.Warnf("Error determining UID of app container: %s\n", err)
		log.Warnf("Using know 1000:1000 as fallback when creating volumes\n")
	}

	options := volume.ListOptions{Filters: filters.NewArgs()}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))

	volumes, err := client.VolumeList(context.Background(), options)

	if err != nil {
		return err
	}

	for mountName, _ := range deployCfg.ProjectConfig.App.Mounts {
		expectedVolumeName := fmt.Sprintf("%s_app_%s", deployCfg.ContainerPrefix(), mountName)

		found := false

		for _, dockerVolume := range volumes.Volumes {
			if dockerVolume.Name == expectedVolumeName {
				found = true
			}
		}

		if !found {
			_, err := client.VolumeCreate(ctx, volume.CreateOptions{
				Name: expectedVolumeName,
				Labels: map[string]string{
					"tanjun":         "true",
					"tanjun.project": deployCfg.Name,
				},
			})

			if err != nil {
				return err
			}

			containerCfg := &container.Config{
				Image: "alpine:latest",
				Cmd:   []string{"sh", "-c", fmt.Sprintf("chown -R %s:%s /volume", userId, userId)},
			}

			hostCfg := &container.HostConfig{
				AutoRemove: true,
				Mounts: []mount.Mount{
					{
						Type:   mount.TypeVolume,
						Source: expectedVolumeName,
						Target: "/volume",
					},
				},
			}

			c, err := client.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, fmt.Sprintf("%s_chown", expectedVolumeName))

			if err != nil {
				return err
			}

			if err := client.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}

func addAppServerVolumes(deployCfg DeployConfiguration, hostCfg *container.HostConfig) {
	for mountName, appMount := range deployCfg.ProjectConfig.App.Mounts {
		if appMount.Path == "" {
			continue
		}

		if appMount.Path[0] != '/' {
			appMount.Path = fmt.Sprintf("%s/%s", deployCfg.imageConfig.WorkingDir, appMount.Path)
		}

		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_app_%s", deployCfg.ContainerPrefix(), mountName),
			Target: appMount.Path,
			VolumeOptions: &mount.VolumeOptions{
				Labels: map[string]string{
					"tanjun":         "true",
					"tanjun.project": deployCfg.Name,
				},
			},
		})
	}
}

func createEnvironmentNetwork(ctx context.Context, c *client.Client, deployCfg DeployConfiguration) error {
	options := network.ListOptions{Filters: filters.NewArgs()}
	options.Filters.Add("name", deployCfg.Name)

	networks, err := c.NetworkList(ctx, options)

	if err != nil {
		return err
	}

	if len(networks) > 0 {
		return nil
	}

	_, err = c.NetworkCreate(ctx, deployCfg.Name, network.CreateOptions{
		Labels: map[string]string{
			"tanjun":         "true",
			"tanjun.project": deployCfg.Name,
		},
	})

	return err
}
