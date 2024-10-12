package cmd

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
	"strings"
)

var versionListCmd = &cobra.Command{
	Use:   "version:list",
	Short: "List all versions of an image",
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

		versions, err := docker.VersionList(cmd.Context(), client, cfg.Image)

		if err != nil {
			return err
		}

		t := table.New().
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
			Headers("Name", "Aliases", "Created at")

		for _, version := range versions {
			t.Row(version.Name, strings.Join(version.Aliases, ", "), formatRelativeDate(version.CreatedAt))
		}

		fmt.Println(t.Render())

		log.Infof("Found %d versions", len(versions))
		log.Infof("Aliases are versions that are similar to each other. They are groupped by the first version that was created.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionListCmd)
}
