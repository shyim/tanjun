package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets",
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

		kv, err := docker.CreateKVConnection(cmd.Context(), client)

		if err != nil {
			return err
		}

		secrets, err := docker.ListProjectSecrets(kv, cfg.Name)

		if err != nil {
			return err
		}

		t := table.New().
			Headers("Key", "Value")

		for key, value := range secrets {
			t.Row(key, value)
		}

		fmt.Println(t.Render())

		return nil
	},
}

func init() {
	secretCmd.AddCommand(secretListCmd)
}
