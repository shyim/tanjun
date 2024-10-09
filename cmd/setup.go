package cmd

import (
	"fmt"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setups a server for initial deployment",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		if err := docker.ConfigureServer(cmd.Context(), client); err != nil {
			return err
		}

		fmt.Println("Server setup complete")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
