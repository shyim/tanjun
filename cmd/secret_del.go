package cmd

import (
	"fmt"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
)

var secretDelCmd = &cobra.Command{
	Use:   "secret:del [id] ...",
	Short: "Delete an secret",
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
			if _, ok := secrets[arg]; ok {
				delete(secrets, arg)
			} else {
				fmt.Printf("Secret %s not found. Skipping..\n", arg)
			}
		}

		if err := docker.SetProjectSecrets(kv, cfg.Name, secrets); err != nil {
			return err
		}

		fmt.Println("Secrets set. You need to redeploy the project for the changes to take effect")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(secretDelCmd)
}
