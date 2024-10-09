package cmd

import (
	"fmt"
	"github.com/gosimple/slug"
	"github.com/manifoldco/promptui"
	"github.com/shyim/tanjun/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initializes a new project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(".tanjun.yml"); err == nil {
			return fmt.Errorf("project already initialized (found .tanjun.yml)")
		}

		cfg := config.ProjectConfig{}
		cfg.FillDefaults()

		cfg.App.Dockerfile = ""

		prompt := promptui.Prompt{
			Label: "Project Name",
		}

		cfg.Name, _ = prompt.Run()
		cfg.Name = slug.Make(cfg.Name)

		prompt = promptui.Prompt{
			Label: "Image Name (this will be used to push and pull the image, e.g ghcr.io/username/project)",
		}

		cfg.ImageName, _ = prompt.Run()

		prompt = promptui.Prompt{
			Label: "SSH Address (this will be used to connect to the server)",
		}

		cfg.Server.Address, _ = prompt.Run()

		prompt = promptui.Prompt{
			Label:   "SSH User (this will be used to connect to the server)",
			Default: "root",
		}

		cfg.Server.Username, _ = prompt.Run()

		prompt = promptui.Prompt{
			Label:   "SSH Port (this will be used to connect to the server)",
			Default: "22",
		}

		serverPort, _ := prompt.Run()

		cfg.Server.Port, _ = strconv.Atoi(serverPort)

		prompt = promptui.Prompt{
			Label: "Proxy Host (this will be where the app will be served)",
		}

		cfg.Proxy.Host, _ = prompt.Run()

		bytes, err := yaml.Marshal(cfg)

		if err != nil {
			return err
		}

		config := "# yaml-language-server: $schema=https://raw.githubusercontent.com/shyim/tanjun/refs/heads/main/schema.json\n" + string(bytes)

		if err := os.WriteFile(".tanjun.yml", []byte(config), 0644); err != nil {
			return err
		}

		fmt.Println("Created a .tanjun.yml. Run next tanjun setup to setup the server")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
