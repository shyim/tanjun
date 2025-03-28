package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var cronjobHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "List last executions of an cronjob",
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

		return docker.RunCronjobCommand(cmd.Context(), client, cfg, []string{"history", args[0]})
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return []string{}, cobra.ShellCompDirectiveError
		}

		var completions []string

		for _, job := range cfg.App.Cronjobs {
			completions = append(completions, job.Name)
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	},
}

func init() {
	cronjobCmd.AddCommand(cronjobHistoryCmd)
}
