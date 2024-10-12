package cmd

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
	"strings"
)

var secretSetCmd = &cobra.Command{
	Use:   "secret:set [id] [key=value] ...",
	Short: "Set a secret",
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

		defer client.Close()

		kv, err := docker.CreateKVConnection(cmd.Context(), client)

		if err != nil {
			return err
		}

		secrets, err := docker.ListProjectSecrets(kv, cfg.Name)

		if err != nil {
			return err
		}

		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid key=value format: %s", arg)
			}

			secrets[parts[0]] = parts[1]
		}

		if err := docker.SetProjectSecrets(kv, cfg.Name, secrets); err != nil {
			return err
		}

		log.Print("Secrets set. You need to redeploy the project for the changes to take effect\n")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(secretSetCmd)
}
