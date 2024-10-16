package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var serviceList = &cobra.Command{
	Use:   "list",
	Short: "List all services",
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

		t := table.New().
			Headers("Name", "Status", "Dangling")

		for name, service := range services {
			t.Row(name, service.Status, formatBoolToString(service.Dangling))
		}

		fmt.Println(t.Render())

		if services.HasDanlingServices() {
			log.Warnf("There are dangling services. Run 'tanjun service rm <name>' to remove them.")
		}

		if services.HasNotDeployedServices() {
			log.Warnf("There are services that are not deployed yet. Run 'tanjun deploy' to deploy them.")
		}

		return nil
	},
}

func init() {
	serviceCmd.AddCommand(serviceList)
}

func formatBoolToString(b bool) string {
	if b {
		return "yes"
	}

	return "no"
}
