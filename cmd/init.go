package cmd

import (
	"fmt"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/gosimple/slug"
	"github.com/shyim/tanjun/internal/buildpack"
	"github.com/shyim/tanjun/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"strings"
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

		namePlaceHolder := namesgenerator.GetRandomName(0)

		sshPort := "22"

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Project Name").
					Description("Name of the project").
					Placeholder(namePlaceHolder).
					Value(&cfg.Name).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("project name is required")
						}

						if !slug.IsSlug(s) {
							return fmt.Errorf("project name should not contain whitespaces and special characters")
						}

						return nil
					}),
				huh.NewInput().
					Title("Image Name").
					Description("Image Name (this will be used to push and pull the image)").
					PlaceholderFunc(func() string {
						val := cfg.Name

						if val == "" {
							val = namePlaceHolder
						}
						return fmt.Sprintf("ghcr.io/%s/%s", os.Getenv("USER"), val)
					}, &cfg.Name).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("image name is required")
						}

						if strings.Contains(s, ":") {
							return fmt.Errorf("image name should not contain tag")
						}

						return nil
					}).
					Value(&cfg.Image),
			).Title("Project"),
			huh.NewGroup(
				huh.NewInput().Title("SSH Address").Description("SSH Address (this will be used to connect to the server)").Value(&cfg.Server.Address),
				huh.NewInput().Title("SSH User").Description("SSH User (this will be used to connect to the server)").Value(&cfg.Server.Username),
				huh.NewInput().Title("SSH Port").Description("SSH Port (this will be used to connect to the server)").Value(&sshPort).Validate(func(s string) error {
					if _, err := strconv.Atoi(s); err != nil {
						return fmt.Errorf("port should be a number")
					}

					return nil
				}),
			).Title("Server"),
			huh.NewGroup(
				huh.NewInput().Title("Host").Description("At which host the app will be served").Placeholder("example.com").Value(&cfg.Proxy.Host),
				huh.NewConfirm().Title("Use HTTPS").Description("Use HTTPS for the proxy").Value(&cfg.Proxy.SSL),
			).Title("Proxy"),
		)

		if err := form.Run(); err != nil {
			return err
		}

		cfg.Server.Port, _ = strconv.Atoi(sshPort)

		currentDir, _ := os.Getwd()

		language, err := buildpack.DetectProjectType(currentDir)

		if err == nil {
			cfg.Build.BuildPack = &buildpack.Config{
				Type: language,
			}
			log.Infof("Detected project uses %s and trying to build automatically the Dockerfile for you. Specify a Dockerfile in your config to disable this", language)
		} else {
			log.Infof("Buildpack cannot generate a Dockerfile automatically. Create a Dockerfile for your application container before you can deploy.")
			cfg.Build.Dockerfile = "Dockerfile"
		}

		bytes, err := yaml.Marshal(cfg)

		if err != nil {
			return err
		}

		config := "# yaml-language-server: $schema=https://raw.githubusercontent.com/shyim/tanjun/refs/heads/main/schema.json\n" + string(bytes)

		if err := os.WriteFile(".tanjun.yml", []byte(config), 0644); err != nil {
			return err
		}

		log.Print("Created a .tanjun.yml. Run next tanjun setup to setup the server\n")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
