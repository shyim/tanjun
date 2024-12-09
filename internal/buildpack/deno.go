package buildpack

import (
	"fmt"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"os"
	"path"
)

type Deno struct {
}

func (b Deno) Generate(root string, cfg *Config) (*GeneratedImageResult, error) {
	result := &GeneratedImageResult{}

	result.AddIgnoreLine("node_modules")

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest as builder")
	result.AddLine("ENV CI=true NODE_ENV=production")
	result.AddLine("WORKDIR /app")
	result.AddLine("ADD . .")

	addEnvFromSettings(result, cfg)
	addPackagesFromSettings(result, cfg, "deno")

	result.AddLine("RUN deno install")

	result.NewLine()

	result.AddLine("EXPOSE %d", cfg.Settings["port"])

	result.NewLine()

	result.AddLine("ENTRYPOINT %s", getDenoStartCommand(root, cfg.Settings["port"].(int)))

	return result, nil
}

func (b Deno) Schema() *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	properties.Set("packages", &jsonschema.Schema{
		Type:        "array",
		Items:       &jsonschema.Schema{Type: "string"},
		Description: "Allows installation of additional packages",
	})

	properties.Set("env", &jsonschema.Schema{
		Type:        "object",
		Description: "Default environment variables",
		AdditionalProperties: &jsonschema.Schema{
			Type: "string",
		},
	})

	properties.Set("port", &jsonschema.Schema{
		Type:        "integer",
		Default:     "3000",
		Description: "Application Listing Port",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func (b Deno) Default() ConfigSettings {
	return ConfigSettings{
		"port":     3000,
		"packages": []any{},
		"env":      make(ConfigSettings),
	}
}

func (b Deno) Supports(root string) bool {
	_, err := os.Stat(path.Join(root, "deno.json"))

	return err == nil
}

func (b Deno) Name() string {
	return "deno"
}

func init() {
	RegisterLanguage(&Deno{})
}

type denoConfig struct {
	Tasks map[string]string `json:"tasks"`
}

func getDenoStartCommand(root string, port int) string {
	if _, err := os.Stat(path.Join(root, "deno.json")); os.IsNotExist(err) {
		return fmt.Sprintf("deno serve --port %d -A main.ts", port)
	}

	var config denoConfig

	if err := readJSONFile(path.Join(root, "deno.json"), &config); err != nil {
		return fmt.Sprintf("deno serve --port %d -A main.ts", port)
	}

	if task, ok := config.Tasks["start"]; ok {
		return task
	}

	if task, ok := config.Tasks["production"]; ok {
		return task
	}

	return fmt.Sprintf("deno serve --port %d -A main.ts", port)
}
