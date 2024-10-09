package cmd

import (
	"fmt"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy a project",
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

		if err := docker.DestroyProject(cmd.Context(), client, cfg.Name); err != nil {
			return err
		}

		fmt.Printf("Project %s destroyed\n", cfg.Name)
		fmt.Println("The docker image is still available, you need to delete it manually")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}
