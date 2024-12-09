package cmd

import "github.com/spf13/cobra"

var cronjobCmd = &cobra.Command{
	Use:   "cronjob",
	Short: "Manage cronjobs",
}

func init() {
	rootCmd.AddCommand(cronjobCmd)
}
