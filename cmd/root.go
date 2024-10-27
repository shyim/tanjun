package cmd

import (
	"context"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"os"
)

var configFile = ".tanjun.yml"
var projectRoot = ""
var verboseMode = false
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "tanjun",
	Short:   "Tanjun is a simple Docker based deployment solution for self-hosting",
	Version: version,
}

func Execute(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().StringVar(&configFile, "config", configFile, "Path to the config file")
	rootCmd.PersistentFlags().StringVar(&projectRoot, "project-root", "", "Path to the project root, otherwise it will use the current directory")
	rootCmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Show debug info")

	cobra.OnInitialize(func() {
		if projectRoot != "" {
			if err := os.Chdir(projectRoot); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		if verboseMode {
			log.SetLevel(log.DebugLevel)
		}
	})
}
