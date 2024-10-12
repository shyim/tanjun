package buildpack

import (
	"fmt"
	"github.com/shyim/go-version"
	"log"
	"os"
	"path"
)

type PackageJSON struct {
	Main            string            `json:"main"`
	PackageManager  string            `json:"packageManager"`
	Engines         map[string]string `json:"engines"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
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

func generateByNodeJS(project string) (*GeneratedImageResult, error) {
	var packageJSON PackageJSON

	if err := readJSONFile(path.Join(project, "package.json"), &packageJSON); err != nil {
		return nil, err
	}

	nodeVersion := detectNodeVersion(packageJSON)

	result := &GeneratedImageResult{}

	packageManager := detectNodePackageManager(project)

	result.AddIgnoreLine("node_modules")

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/base:latest as builder")
	result.AddLine("ENV CI=true NODE_ENV=production NPM_CONFIG_PRODUCTION=false")
	result.Add("RUN apk add --no-cache nodejs-%s npm", nodeVersion)

	if packageManager == "bun" {
		result.Add(" bun-bin")
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
	result.AddLine("RUN apk add --no-cache nodejs-%s curl", nodeVersion)
	result.AddLine("COPY --from=builder /app .")

	result.AddLine("EXPOSE 3000")

	if err := nodeJSAddStartupCommand(project, result, packageJSON); err != nil {
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

	if _, err := os.Stat(path.Join(project, "bun.lockdb")); err == nil {
		return "bun"
	}

	return "npm"
}

func nodeJSAddStartupCommand(project string, result *GeneratedImageResult, packageJSON PackageJSON) error {
	if _, ok := packageJSON.Scripts["start"]; ok {
		result.AddLine("CMD npm start")

		return nil
	}

	if packageJSON.Main != "" {
		result.AddLine("CMD node %s", packageJSON.Main)
		return nil
	}

	if _, err := os.Stat(path.Join(project, "index.mjs")); err == nil {
		result.AddLine("CMD node index.mjs")
		return nil
	}

	if _, err := os.Stat(path.Join(project, "index.js")); err == nil {
		result.AddLine("CMD node index.js")
		return nil
	}

	return fmt.Errorf("could not detect how to start the application: provide a start script, a main file in package.json or an index.js file in the project root")
}
