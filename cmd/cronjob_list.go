package cmd

import (
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var cronjobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cronjobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		defer client.Close()

		return docker.RunCronjobCommand(cmd.Context(), client, cfg, []string{"list"})
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobListCmd)
}
