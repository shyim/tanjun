package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Shows logs of a service",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		defer func() {
			if err := client.Close(); err != nil {
				log.Warnf("Failed to close docker client: %s", err)
			}
		}()

		serviceName, _ := cmd.PersistentFlags().GetString("service")

		containerId, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, serviceName)

		if err != nil {
			return err
		}

		followLogs, _ := cmd.PersistentFlags().GetBool("follow")

		stream, err := client.ContainerLogs(cmd.Context(), containerId, container.LogsOptions{
			Follow:     followLogs,
			ShowStderr: true,
			ShowStdout: true,
		})

		if err != nil {
			return err
		}

		defer func() {
			if err := stream.Close(); err != nil {
				log.Warnf("Failed to close container log stream: %s", err)
			}
		}()

		_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, stream)

		return err
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.PersistentFlags().String("service", "", "Specify service name to tail logs from, otherwise app is used")
	logsCmd.PersistentFlags().BoolP("follow", "f", false, "Follow the logs")
}
