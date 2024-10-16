package cmd

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var serviceRmCmd = &cobra.Command{
	Use:   "rm [name]",
	Short: "Remove a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		services, err := docker.ProjectListServices(cmd.Context(), client, cfg)

		if err != nil {
			return err
		}

		service, ok := services[args[0]]

		if !ok {
			return fmt.Errorf("Service %s not found", args[0])
		}

		force, _ := cmd.Flags().GetBool("force")

		if !force && !service.Dangling {
			return fmt.Errorf("Service %s is not dangling. Use --force to remove it", args[0])
		}

		if err := docker.ProjectDeleteService(cmd.Context(), client, cfg, args[0]); err != nil {
			return err
		}

		log.Infof("Service %s removed", args[0])

		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceRmCmd)
	serviceRmCmd.PersistentFlags().BoolP("force", "f", false, "Force remove the service")
}
