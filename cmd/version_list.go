package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var versionListCmd = &cobra.Command{
	Use:   "list",
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

		defer func() {
			if err := client.Close(); err != nil {
				log.Warnf("Failed to close docker client: %s", err)
			}
		}()

		versions, err := docker.VersionList(cmd.Context(), client, cfg)

		if err != nil {
			return err
		}

		t := table.New().
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
	versionCmd.AddCommand(versionListCmd)
}
