package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var cronjobRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a job out of schedule",
	Args:  cobra.MinimumNArgs(1),
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

		return docker.RunCronjobCommand(cmd.Context(), client, cfg, []string{"run", args[0]})
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobRunCmd)
}
