package docker

import (
	"fmt"
	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
)

func CreateClientFromConfig(config *config.ProjectConfig) (*client.Client, error) {
	if config.Server.Address == "127.0.0.1" {
		c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

		if err != nil {
			return nil, err
		}

		return c, nil
	}

	hostScheme := fmt.Sprintf("ssh://%s@%s:%d", config.Server.Username, config.Server.Address, config.Server.Port)
	helper, err := connhelper.GetConnectionHelperWithSSHOpts(hostScheme, []string{"-o", "ServerAliveInterval=10"})
	if err != nil {
		return nil, err
	}

	c, err := client.NewClientWithOpts(
		client.WithHost(helper.Host),
		client.WithDialContext(helper.Dialer),
		client.WithAPIVersionNegotiation(),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}
