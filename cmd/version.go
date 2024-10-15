package cmd

import (
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Manage versions",
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
