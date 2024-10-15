package cmd

import (
	"github.com/spf13/cobra"
)

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets",
}

func init() {
	rootCmd.AddCommand(secretCmd)
}
