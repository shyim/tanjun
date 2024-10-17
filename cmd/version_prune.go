package cmd

import (
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var versionPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old versions of images",
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

		return docker.VersionDrain(cmd.Context(), client, cfg)
	},
}

func init() {
	versionCmd.AddCommand(versionPruneCmd)
}
