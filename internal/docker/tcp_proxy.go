package docker

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/shyim/tanjun/internal/mtls"
)

type TCPProxy struct {
	ProxyContainerId string
	ListenPort       string
	Keys             *mtls.MTLSGenerated
}

func CreateTCPProxy(ctx context.Context, client *client.Client, externalHost string, containerId string, port string) (*TCPProxy, error) {
	keys, err := mtls.Generate(externalHost)

	if err != nil {
		return nil, err
	}

	if err := PullImageIfNotThere(ctx, client, "ghcr.io/shyim/tanjun/tcp-proxy:v1"); err != nil {
		return nil, err
	}

	inspect, err := client.ContainerInspect(ctx, containerId)
	if err != nil {
		return nil, err
	}

	networkName := ""

	for name, _ := range inspect.NetworkSettings.Networks {
		networkName = name
		break
	}

	containerCfg := container.Config{
		Image: "ghcr.io/shyim/tanjun/tcp-proxy:v1",
		Cmd:   []string{fmt.Sprintf("%s:%s", inspect.NetworkSettings.Networks[networkName].IPAddress, port), "6879"},
		Env: []string{
			"TLS_CA_CERT=" + base64.StdEncoding.EncodeToString(keys.CaCert),
			"TLS_SERVER_CERT=" + base64.StdEncoding.EncodeToString(keys.ServerCert),
			"TLS_SERVER_KEY=" + base64.StdEncoding.EncodeToString(keys.ServerKey),
		},
		ExposedPorts: map[nat.Port]struct{}{
			"6879/tcp": {},
		},
		Labels: map[string]string{
			"tanjun": "true",
		},
	}

	hostCfg := container.HostConfig{
		AutoRemove: true,
		PortBindings: map[nat.Port][]nat.PortBinding{
			"6879/tcp": {},
		},
	}

	networkCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	resp, err := client.ContainerCreate(ctx, &containerCfg, &hostCfg, networkCfg, nil, "")

	if err != nil {
		return nil, err
	}

	if err := client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, err
	}

	listenPort := ""

	proxyInspect, err := client.ContainerInspect(ctx, resp.ID)

	if err != nil {
		return nil, err
	}

	for _, p := range proxyInspect.NetworkSettings.Ports {
		for _, ipP := range p {
			listenPort = ipP.HostPort
		}
	}

	return &TCPProxy{
		ProxyContainerId: resp.ID,
		ListenPort:       listenPort,
		Keys:             keys,
	}, nil

}
