package buildpack

import (
	"fmt"
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"os"
	"path"
	"strings"
)

type Node struct {
}

func (n Node) Name() string {
	return "node"
}

func (n Node) Generate(root string, cfg *Config) (*GeneratedImageResult, error) {
	var packageJSON PackageJSON

	if err := readJSONFile(path.Join(root, "package.json"), &packageJSON); err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	result := &GeneratedImageResult{}

	if cfg.Settings["version"].(string) == "" {
		cfg.Settings["version"] = detectNodeVersion(packageJSON)
	}

	nodePackage := fmt.Sprintf("nodejs-%s npm", cfg.Settings["version"])

	packageManager := detectNodePackageManager(root)

	result.AddIgnoreLine("node_modules")

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest as builder")
	result.AddLine("ENV CI=true NODE_ENV=production NPM_CONFIG_PRODUCTION=false")

	addPackagesFromSettings(result, cfg, nodePackage)
	addEnvFromSettings(result, cfg)

	result.NewLine()

	if packageManager == "pnpm" {
		result.AddLine("RUN npm install -g pnpm")
	} else if packageManager == "yarn" {
		result.AddLine("RUN npm install -g yarn")
	}

	result.AddLine("WORKDIR /app")
	result.AddLine("COPY . .")

	if packageJSON.HasDependencies() {
		switch packageManager {
		case "bun":
			result.AddLine("RUN bun install")
		case "yarn":
			result.AddLine("RUN yarn install")
		case "pnpm":
			result.AddLine("RUN pnpm install")
		default:
			result.AddLine("RUN npm ci")
		}
	}

	possibleScripts := []string{"build", "prod", "production"}

	for _, script := range possibleScripts {
		if _, ok := packageJSON.Scripts[script]; ok {
			result.AddLine("RUN npm run %s", script)
		}
	}

	result.NewLine()
	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest")
	result.AddLine("ENV NODE_ENV=production")
	result.AddLine("WORKDIR /app")

	addPackagesFromSettings(result, cfg, nodePackage)
	addEnvFromSettings(result, cfg)

	result.AddLine("COPY --from=builder /app .")

	result.AddLine("EXPOSE %s", cfg.Settings["port"])

	if err := nodeJSAddStartupCommand(root, result, packageJSON); err != nil {
		return nil, err
	}

	return result, nil
}

func (n Node) Schema() *jsonschema.Schema {
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

	properties.Set("version", &jsonschema.Schema{
		Type:        "string",
		Enum:        []any{"20", "22", "23"},
		Description: "Node version to use, when empty automatically detected by package.json",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func (n Node) Default() ConfigSettings {
	return ConfigSettings{
		"port":     "3000",
		"packages": []any{},
		"env":      make(ConfigSettings),
		"version":  "",
	}
}

func (n Node) Supports(root string) bool {
	var packageJSON PackageJSON

	if err := readJSONFile(path.Join(root, "package.json"), &packageJSON); err != nil {
		return false
	}

	if _, ok := packageJSON.DevDependencies["@types/bun"]; ok {
		return false
	}

	return true
}

func detectNodePackageManager(project string) string {
	if _, err := os.Stat(path.Join(project, "yarn.lock")); err == nil {
		return "yarn"
	}

	if _, err := os.Stat(path.Join(project, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}

	if _, err := os.Stat(path.Join(project, "bun.lockb")); err == nil {
		return "bun"
	}

	return "npm"
}

func nodeJSAddStartupCommand(project string, result *GeneratedImageResult, packageJSON PackageJSON) error {
	if _, ok := packageJSON.Scripts["start"]; ok {
		result.AddLine("CMD npm run start")

		return nil
	}

	possibleFiles := []string{"index.ts", "index.mts", "index.mjs", "index.js"}

	if packageJSON.Main != "" {
		// prepend the main file to the list of possible files
		possibleFiles = append([]string{packageJSON.Main}, possibleFiles...)
	}

	for _, file := range possibleFiles {
		if _, err := os.Stat(path.Join(project, file)); err == nil {
			if strings.HasSuffix(file, ".ts") {
				result.AddLine("CMD node --experimental-strip-types %s", file)
			} else {
				result.AddLine("CMD node %s", file)
			}

			return nil
		}
	}

	return fmt.Errorf("could not detect how to start the application: provide a start script, a main file in package.json or an index.js file in the project root")
}

func init() {
	RegisterLanguage(Node{})
}
