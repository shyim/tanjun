package cmd

import (
	"fmt"
	"github.com/gosimple/slug"
	"github.com/shyim/tanjun/internal/build"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
	"os"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys local source to server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		version, _ := cmd.Flags().GetString("version")

		if version == "" {
			currentDir, err := os.Getwd()

			if err != nil {
				return err
			}

			version, err = build.BuildImage(cmd.Context(), cfg, currentDir)

			if err != nil {
				return err
			}

			fmt.Println("Built version", version)
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		imageName := fmt.Sprintf("%s:%s", cfg.Image, version)

		if err := docker.PullImageIfNotThere(cmd.Context(), client, imageName); err != nil {
			return err
		}

		deployConfig := docker.DeployConfiguration{
			Name:                 slug.Make(cfg.Name),
			ImageName:            imageName,
			ProjectConfig:        cfg,
			EnvironmentVariables: make(map[string]string),
		}

		if err := docker.Deploy(cmd.Context(), client, deployConfig); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().String("version", "", "Use this version to deploy, instead of building a new one. Useful for rollbacks")
}
