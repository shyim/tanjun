package docker

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
)

type DeployConfiguration struct {
	Name                 string
	ImageName            string
	NetworkName          string
	ProjectConfig        *config.ProjectConfig
	EnvironmentVariables map[string]string
	storage              *KvClient
}

func (c DeployConfiguration) ContainerPrefix() string {
	return fmt.Sprintf("tanjun_%s", c.Name)
}

func (c DeployConfiguration) GetEnvironmentVariables() []string {
	var env []string

	for key, value := range c.EnvironmentVariables {
		env = append(env, key+"="+value)
	}

	return env
}

func getEnvironmentContainers(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) ([]types.Container, error) {
	options := container.ListOptions{
		Filters: filters.NewArgs(),
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))
	options.Filters.Add("label", "tanjun.app=true")

	return client.ContainerList(ctx, options)
}

func getWorkerEnvironmentContainers(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) ([]types.Container, error) {
	options := container.ListOptions{
		Filters: filters.NewArgs(),
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))
	options.Filters.Add("label", "tanjun.worker")

	return client.ContainerList(ctx, options)
}

func getCronjobEnvironmentContainers(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) ([]types.Container, error) {
	options := container.ListOptions{
		Filters: filters.NewArgs(),
	}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))
	options.Filters.Add("label", "tanjun.cronjob")

	return client.ContainerList(ctx, options)
}

func getAppContainerConfiguration(deployCfg DeployConfiguration) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	//routerName := fmt.Sprintf("tanjun_%s_default", deployCfg.Name)

	containerCfg := &container.Config{
		Image: deployCfg.ImageName,
		Labels: map[string]string{
			"traefik.enable": "true",

			"com.docker.compose.project": deployCfg.ContainerPrefix(),
			"com.docker.compose.service": "web",

			//fmt.Sprintf("traefik.http.middlewares.%s_redirect.redirectscheme.scheme", routerName):    "https",
			//fmt.Sprintf("traefik.http.middlewares.%s_redirect.redirectscheme.permanent", routerName): "true",

			//fmt.Sprintf("traefik.http.routers.%s.rule", routerName):        fmt.Sprintf("Host(`%s`)", deployCfg.C),
			//fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName): "web",
			//fmt.Sprintf("traefik.http.routers.%s.middlewares", routerName): fmt.Sprintf("%s_redirect", routerName),
			//
			//fmt.Sprintf("traefik.http.routers.%s_ssl.rule", routerName):             fmt.Sprintf("Host(`%s`)", deployCfg.DefaultDomain),
			//fmt.Sprintf("traefik.http.routers.%s_ssl.entrypoints", routerName):      "websecure",
			//fmt.Sprintf("traefik.http.routers.%s_ssl.tls", routerName):              "true",
			//fmt.Sprintf("traefik.http.routers.%s_ssl.tls.certresolver", routerName): "letsencrypt",

			"tanjun":         "true",
			"tanjun.app":     "true",
			"tanjun.project": fmt.Sprintf("%s", deployCfg.Name),
		},
		Env: deployCfg.GetEnvironmentVariables(),
	}

	hostCfg := &container.HostConfig{}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"tanjun-public":       {},
			deployCfg.NetworkName: {},
		},
	}

	addAppServerVolumes(deployCfg, hostCfg)

	return containerCfg, hostCfg, networkCfg
}

func Deploy(ctx context.Context, client *client.Client, deployCfg DeployConfiguration) error {
	var err error
	deployCfg.storage, err = CreateKVConnection(ctx, client)

	if err != nil {
		return err
	}

	defer deployCfg.storage.Close()

	if err := prepareEnvironmentVariables(deployCfg); err != nil {
		return err
	}

	if err := createEnvironmentNetwork(ctx, client, deployCfg); err != nil {
		return err
	}

	beforeContainers, err := getEnvironmentContainers(ctx, client, deployCfg)

	if err != nil {
		return err
	}

	if err := createAppServerVolumes(ctx, client, deployCfg); err != nil {
		return err
	}

	if err := startServices(ctx, client, deployCfg); err != nil {
		return err
	}

	beforeWorkers, err := getWorkerEnvironmentContainers(ctx, client, deployCfg)

	if err != nil {
		return err
	}

	if len(beforeWorkers) > 0 {
		fmt.Println("Stopping old worker containers")

		if err := stopContainers(ctx, client, beforeWorkers); err != nil {
			return err
		}
	}

	beforeCronjobs, err := getCronjobEnvironmentContainers(ctx, client, deployCfg)

	if err != nil {
		return err
	}

	if len(beforeCronjobs) > 0 {
		fmt.Println("Stopping old cronjob containers")

		if err := stopContainers(ctx, client, beforeCronjobs); err != nil {
			return err
		}
	}

	if deployCfg.storage.Get(deployCfg.ContainerPrefix()+"_setup") == "" {
		fmt.Println("Environment is new, running setup hook")

		if len(deployCfg.ProjectConfig.App.Hooks.Setup) > 0 {
			if err := runHookInContainer(ctx, client, deployCfg, deployCfg.ProjectConfig.App.Hooks.Setup); err != nil {
				return err
			}
		}

		deployCfg.storage.Set(deployCfg.ContainerPrefix()+"_setup", "true")
	} else {
		if len(deployCfg.ProjectConfig.App.Hooks.Changed) > 0 {
			fmt.Printf("Environment %s is not new, running changed hook\n", deployCfg.Name)

			if err := runHookInContainer(ctx, client, deployCfg, deployCfg.ProjectConfig.App.Hooks.Changed); err != nil {
				return err
			}
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

	maxStartup := 60

	var containerInspect types.ContainerJSON

	for {
		containerInspect, err = client.ContainerInspect(ctx, resp.ID)

		if err != nil {
			return err
		}

		// container has no health check configured
		if containerInspect.State.Running && containerInspect.State.Health == nil {
			break
		}

		// container is running and healthy
		if containerInspect.State.Running && containerInspect.State.Health != nil && containerInspect.State.Health.Status == types.Healthy {
			break
		}

		maxStartup--

		if maxStartup == 0 {
			_ = client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

			if err := startContainers(ctx, client, append(beforeWorkers, beforeCronjobs...)); err != nil {
				return err
			}

			return fmt.Errorf("container did not start in time, using old running containers")
		}

		time.Sleep(time.Second)
	}

	fmt.Println("New container is healthy, waiting 5 seconds before removing old containers")
	time.Sleep(5 * time.Second)

	if err := startWorkers(ctx, client, deployCfg); err != nil {
		return err
	}

	if err := startCronjobs(ctx, client, deployCfg); err != nil {
		return err
	}

	fmt.Println("Starting to route new traffic to new container")

	proxyHost := containerInspect.NetworkSettings.Networks["tanjun-public"].IPAddress
	proxyPort := findPortMapping(deployCfg, containerInspect)

	if err := configureKamalService(ctx, client, deployCfg, fmt.Sprintf("%s:%s", proxyHost, proxyPort)); err != nil {
		return err
	}

	removalContainers := append(beforeContainers, beforeWorkers...)

	return removeContainers(ctx, client, append(removalContainers, beforeCronjobs...))
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
		fmt.Printf("Error determining UID of app container: %s\n", err)
		fmt.Printf("Using know 1000:1000 as fallback when creating volumes\n")
	}

	options := volume.ListOptions{Filters: filters.NewArgs()}

	options.Filters.Add("label", fmt.Sprintf("tanjun.project=%s", deployCfg.Name))

	volumes, err := client.VolumeList(context.Background(), options)

	if err != nil {
		return err
	}

	for _, appMount := range deployCfg.ProjectConfig.App.Mounts {
		expectedVolumeName := fmt.Sprintf("%s_app_%s", deployCfg.ContainerPrefix(), appMount.Name)

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
					"tanjun.project": fmt.Sprintf("%s", deployCfg.Name),
				},
			})

			if err != nil {
				return err
			}

			containerCfg := &container.Config{
				Image: "alpine:latest",
				Cmd:   []string{"sh", "-c", fmt.Sprintf("chown -R %s /volume", userId)},
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
	for _, appMount := range deployCfg.ProjectConfig.App.Mounts {
		if appMount.Path == "" {
			continue
		}

		if appMount.Path[0] != '/' {
			appMount.Path = fmt.Sprintf("/var/www/html/%s", appMount.Path)
		}

		hostCfg.Mounts = append(hostCfg.Mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: fmt.Sprintf("%s_app_%s", deployCfg.ContainerPrefix(), appMount.Name),
			Target: appMount.Path,
			VolumeOptions: &mount.VolumeOptions{

				Labels: map[string]string{
					"tanjun":         "true",
					"tanjun.project": fmt.Sprintf("%s", deployCfg.Name),
				},
			},
		})
	}
}

func createEnvironmentNetwork(ctx context.Context, c *client.Client, deployCfg DeployConfiguration) error {
	options := network.ListOptions{Filters: filters.NewArgs()}
	options.Filters.Add("name", deployCfg.NetworkName)

	networks, err := c.NetworkList(ctx, options)

	if err != nil {
		return err
	}

	if len(networks) > 0 {
		return nil
	}

	_, err = c.NetworkCreate(ctx, deployCfg.NetworkName, types.NetworkCreate{
		Labels: map[string]string{
			"tanjun":         "true",
			"tanjun.project": fmt.Sprintf("%s", deployCfg.Name),
		},
	})

	return err
}
