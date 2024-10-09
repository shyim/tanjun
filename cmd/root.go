package cmd

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var configFile = ".tanjun.yml"
var projectRoot = ""

var rootCmd = &cobra.Command{
	Use:   "tanjun",
	Short: "Tanjun is a simple Docker based deployment solution for self-hosting",
}

func Execute(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Fatalln(err)
	}
}

func init() {
	rootCmd.SilenceErrors = true
	rootCmd.PersistentFlags().StringVar(&configFile, "config", configFile, "Path to the config file")
	rootCmd.PersistentFlags().StringVar(&projectRoot, "project-root", "", "Path to the project root, otherwise it will use the current directory")

	cobra.OnInitialize(func() {
		if projectRoot != "" {
			if err := os.Chdir(projectRoot); err != nil {
				log.Fatalln(err)
			}
		}
	})
}
