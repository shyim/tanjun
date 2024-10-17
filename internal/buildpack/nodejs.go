package buildpack

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/shyim/go-version"
)

type PackageJSON struct {
	Main            string            `json:"main"`
	PackageManager  string            `json:"packageManager"`
	Engines         map[string]string `json:"engines"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Tanjun          struct {
		Runtime string `json:"runtime"`
	} `json:"tanjun"`
}

func (j PackageJSON) HasDependencies() bool {
	return len(j.Dependencies) > 0 || len(j.DevDependencies) > 0
}

func detectNodeVersion(packageJson PackageJSON) string {
	nodeConstraint, ok := packageJson.Engines["node"]

	if !ok {
		return "22"
	}

	constraint, err := version.NewConstraint(nodeConstraint)

	if err != nil {
		log.Printf("Error parsing node version constraint: %s", err)
		return "22"
	}

	if constraint.Check(version.Must(version.NewVersion("22"))) {
		return "22"
	}

	if constraint.Check(version.Must(version.NewVersion("20"))) {
		return "20"
	}

	return "18"
}

func detectNodeRuntime(packageJson PackageJSON) string {
	if packageJson.Tanjun.Runtime == "bun" {
		return "bun"
	}

	if _, ok := packageJson.DevDependencies["@types/bun"]; ok {
		return "bun"
	}

	return "node"
}

func generateByNodeJS(project string) (*GeneratedImageResult, error) {
	var packageJSON PackageJSON

	if err := readJSONFile(path.Join(project, "package.json"), &packageJSON); err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	nodeVersion := detectNodeVersion(packageJSON)
	runtime := detectNodeRuntime(packageJSON)

	result := &GeneratedImageResult{}

	packageManager := detectNodePackageManager(project)

	result.AddIgnoreLine("node_modules")

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest as builder")
	result.AddLine("ENV CI=true NODE_ENV=production NPM_CONFIG_PRODUCTION=false")

	if runtime == "node" {
		result.Add("RUN apk add --no-cache nodejs-%s npm", nodeVersion)
	}

	if packageManager == "bun" && runtime != "bun" {
		result.Add(" bun-bin")
	} else if runtime == "bun" {
		result.AddLine("RUN apk add --no-cache bun-bin")
	}

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

	if _, ok := packageJSON.Scripts["build"]; ok {
		result.AddLine("RUN npm run build")
	}

	result.NewLine()
	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest")
	result.AddLine("ENV NODE_ENV=production")
	result.AddLine("WORKDIR /app")

	if runtime == "node" {
		result.AddLine("RUN apk add --no-cache nodejs-%s", nodeVersion)
	} else {
		result.AddLine("RUN apk add --no-cache bun-bin")
	}

	result.AddLine("COPY --from=builder /app .")

	result.AddLine("EXPOSE 3000")

	if err := nodeJSAddStartupCommand(project, runtime, result, packageJSON); err != nil {
		return nil, err
	}

	return result, nil
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

func nodeJSAddStartupCommand(project string, runtime string, result *GeneratedImageResult, packageJSON PackageJSON) error {
	if _, ok := packageJSON.Scripts["start"]; ok {
		if runtime == "node" {
			result.AddLine("CMD npm run start")
		} else {
			result.AddLine("CMD bun run --bun start")
		}

		return nil
	}

	possibleFiles := []string{"index.ts", "index.mts", "index.mjs", "index.js"}

	if packageJSON.Main != "" {
		// prepend the main file to the list of possible files
		possibleFiles = append([]string{packageJSON.Main}, possibleFiles...)
	}

	for _, file := range possibleFiles {
		if _, err := os.Stat(path.Join(project, file)); err == nil {
			if runtime == "node" {
				if strings.HasSuffix(file, ".ts") {
					result.AddLine("CMD node --experimental-strip-types %s", file)
				} else {
					result.AddLine("CMD node %s", file)
				}
			} else {
				result.AddLine("CMD bun %s", file)
			}

			return nil
		}
	}

	return fmt.Errorf("could not detect how to start the application: provide a start script, a main file in package.json or an index.js file in the project root")
}
