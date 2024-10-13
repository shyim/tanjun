package cmd

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/gosimple/slug"
	"github.com/shyim/tanjun/internal/build"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
	"os"
	"slices"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys local source to server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		rollback, _ := cmd.Flags().GetBool("rollback")

		version, _ := cmd.Flags().GetString("version")

		if rollback {
			version, err = docker.VersionCurrentlyActive(cmd.Context(), client, cfg.Name)

			if err != nil {
				return err
			}

			log.Infof("Current version is %s", version)

			versions, err := docker.VersionList(cmd.Context(), client, cfg.Image)

			if err != nil {
				return err
			}

			foundCurrent := false
			setOne := false

			for _, v := range versions {
				// This is the current version, skip
				if v.Name == version || slices.Contains(v.Aliases, version) {
					foundCurrent = true
					continue
				}

				if foundCurrent {
					version = v.Name
					setOne = true
					break
				}
			}

			if !setOne {
				return fmt.Errorf("no version to rollback to")
			}

			log.Infof("Rolling back to version %s", version)
		} else if version == "current" {
			version, err = docker.VersionCurrentlyActive(cmd.Context(), client, cfg.Name)

			if err != nil {
				return err
			}
		} else {
			if version == "" {
				currentDir, err := os.Getwd()

				if err != nil {
					return err
				}

				version, err = build.BuildImage(cmd.Context(), cfg, currentDir)

				if err != nil {
					return err
				}

				log.Infof("Built version %s", version)
			}

			return nil
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
	deployCmd.PersistentFlags().Bool("rollback", false, "Rollback to previous version")
}
