package buildpack

import (
	"fmt"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"os"
	"path"
)

type Bun struct {
}

func (b Bun) Name() string {
	return "bun"
}

func (b Bun) Generate(root string, cfg *Config) (*GeneratedImageResult, error) {
	result := &GeneratedImageResult{}

	result.AddIgnoreLine("node_modules")

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest as builder")
	result.AddLine("ENV CI=true NODE_ENV=production NPM_CONFIG_PRODUCTION=false")
	result.AddLine("WORKDIR /app")
	result.AddLine("ADD . .")

	addEnvFromSettings(result, cfg)
	addPackagesFromSettings(result, cfg, "bun-bin")

	var packageJSON PackageJSON

	if readJSONFile(path.Join(root, "package.json"), &packageJSON) == nil {
		if packageJSON.HasDependencies() {
			result.AddLine("RUN bun install")
		}

		if _, ok := packageJSON.Scripts["build"]; ok {
			result.AddLine("RUN bun run --bun build")
		}
	}

	result.NewLine()
	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest")
	result.AddLine("ENV NODE_ENV=production")
	result.AddLine("WORKDIR /app")

	addEnvFromSettings(result, cfg)
	addPackagesFromSettings(result, cfg, "bun-bin")

	result.AddLine("COPY --from=builder /app .")

	result.AddLine("EXPOSE %s", cfg.Settings["port"])

	if _, ok := packageJSON.Scripts["start"]; ok {
		result.AddLine("CMD bun run --bun start")

		return result, nil
	}

	possibleFiles := []string{"index.ts", "index.mts", "index.mjs", "index.js"}

	if packageJSON.Main != "" {
		// prepend the main file to the list of possible files
		possibleFiles = append([]string{packageJSON.Main}, possibleFiles...)
	}

	for _, file := range possibleFiles {
		if _, err := os.Stat(path.Join(root, file)); err == nil {
			result.AddLine("CMD bun %s", file)

			return result, nil
		}
	}

	return nil, fmt.Errorf("could not detect how to start the application: provide a start script, a main file in package.json or an index.js file in the project root")
}

func (b Bun) Schema() *jsonschema.Schema {
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

func (b Bun) Default() ConfigSettings {
	return ConfigSettings{
		"port":     "3000",
		"packages": []any{},
		"env":      make(ConfigSettings),
	}
}

func (b Bun) Supports(root string) bool {
	if _, err := os.Stat(path.Join(root, "bunfig.toml")); err == nil {
		return true
	}

	var packageJSON PackageJSON

	if err := readJSONFile(path.Join(root, "package.json"), &packageJSON); err != nil {
		return false
	}

	if _, ok := packageJSON.DevDependencies["@types/bun"]; ok {
		return true
	}

	return false
}

func init() {
	RegisterLanguage(&Bun{})
}
