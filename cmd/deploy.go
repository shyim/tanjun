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

		currentDir, err := os.Getwd()

		if err != nil {
			return err
		}

		image, err := build.BuildImage(cmd.Context(), cfg, currentDir)

		if err != nil {
			return err
		}

		fmt.Println("Built image", image)

		fmt.Println("Pulling the image on target server")

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		if err := docker.PullImageIfNotThere(cmd.Context(), client, image); err != nil {
			return err
		}

		deployConfig := docker.DeployConfiguration{
			Name:                 slug.Make(cfg.Name),
			ImageName:            image,
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
}
