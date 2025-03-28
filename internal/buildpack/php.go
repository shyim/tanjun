package buildpack

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type PHP struct {
}

func (P PHP) Name() string {
	return "php"
}

func (P PHP) Generate(root string, cfg *Config) (*GeneratedImageResult, error) {
	result := &GeneratedImageResult{}

	result.AddIgnoreLine("vendor")

	var composerJson ComposerJson
	var composerLock ComposerLock

	if err := readJSONFile(path.Join(root, "composer.lock"), &composerLock); err != nil {
		return nil, err
	}

	if err := readJSONFile(path.Join(root, "composer.json"), &composerJson); err != nil {
		return nil, err
	}

	phpVersion := detectPHPVersion(composerLock)

	imageVersion := phpVersion

	if cfg.Settings["variant"].(string) == "" {
		cfg.Settings["variant"] = "frankenphp"
	}

	if cfg.Settings["variant"].(string) == "frankenphp" {
		phpVersion = fmt.Sprintf("frankenphp-%s", phpVersion)
	}

	phpPackages, err := getRequiredPHPPackages(phpVersion, composerJson, composerLock, cfg)

	if cfg.Settings["profiler"].(string) == "tideways" {
		phpPackages = append(phpPackages, fmt.Sprintf("php-%s-tideways", phpVersion))
	}

	if err != nil {
		return nil, err
	}

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/%s:%s as builder", cfg.Settings["variant"], imageVersion)
	installPackages := fmt.Sprintf("composer %s php-%s-phar php-%s-openssl php-%s-curl ", strings.Join(phpPackages, " "), phpVersion, phpVersion, phpVersion)
	addPackagesFromSettings(result, cfg, installPackages)
	addEnvFromSettings(result, cfg)

	result.NewLine()

	result.AddLine("WORKDIR /var/www/html")
	result.AddLine("COPY . /var/www/html")

	if len(cfg.Settings["ini"].(ConfigSettings)) > 0 {
		result.AddLine("COPY <<EOF /etc/php/conf.d/zz-custom.ini")

		for key, value := range cfg.Settings["ini"].(ConfigSettings) {
			result.AddLine("%s=%s", key, value)
		}

		result.AddLine("EOF")
		result.NewLine()
	}

	result.AddLine("RUN composer install --no-interaction --no-progress")

	result.NewLine()

	if _, err := os.Stat(path.Join(root, "package.json")); err == nil {
		packageManager := detectNodePackageManager(root)

		switch packageManager {
		case "pnpm":
			result.AddLine("RUN npm install -g pnpm")
		case "yarn":
			result.AddLine("RUN npm install -g yarn")
		}

		var packageJSON PackageJSON

		if err := readJSONFile(path.Join(root, "package.json"), &packageJSON); err != nil {
			return nil, fmt.Errorf("failed to read package.json: %w", err)
		}

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

		result.AddLine("RUN rm -rf node_modules")
	}

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/%s:%s", cfg.Settings["variant"], imageVersion)

	if cfg.Settings["variant"] == "frankenphp" {
		if composerLock.HasPackage("runtime/frankenphp-symfony") {
			result.AddLine("ENV APP_RUNTIME=Runtime\\\\FrankenPhpSymfony\\\\Runtime FRANKENPHP_CONFIG=\"worker ./public/index.php\"")
		}

		result.AddLine("ENV SERVER_NAME :80")
		result.AddLine("RUN \\")
		result.AddLine("    mkdir -p /data/caddy && mkdir -p /config/caddy; \\")
		result.AddLine("    apk add --no-cache libcap-utils; \\")
		result.AddLine("		adduser -u 82 -D www-data; \\")
		result.AddLine("    	setcap CAP_NET_BIND_SERVICE=+eip /usr/bin/frankenphp; \\")
		result.AddLine("    	chown -R www-data:www-data /data/caddy && chown -R www-data:www-data /config/caddy; \\")
		result.AddLine("    apk del libcap-utils")
	}

	addPackagesFromSettings(result, cfg, "curl "+strings.Join(phpPackages, " \\\n "))
	addEnvFromSettings(result, cfg)
	result.NewLine()

	if cfg.Settings["variant"] == "frankenphp" {
		result.AddLine("COPY --from=builder --chown=82:82 /var/www/html /app")
	} else {
		result.AddLine("COPY --from=builder --chown=82:82 /var/www/html /var/www/html")
	}

	if len(cfg.Settings["ini"].(ConfigSettings)) > 0 {
		result.AddLine("COPY <<EOF /etc/php/conf.d/zz-custom.ini")

		for key, value := range cfg.Settings["ini"].(ConfigSettings) {
			result.AddLine("%s=%s", key, value)
		}

		result.AddLine("EOF")
		result.NewLine()
	}

	result.Add("USER www-data")

	return result, nil
}

func (P PHP) Schema() *jsonschema.Schema {
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

	properties.Set("version", &jsonschema.Schema{
		Type:        "string",
		Enum:        []any{"8.1", "8.2", "8.3", "8.4"},
		Description: "PHP Version (default detect from composer.json)",
	})

	properties.Set("variant", &jsonschema.Schema{
		Type:        "string",
		Enum:        []any{"nginx", "caddy", "frankenphp"},
		Description: "Server Variant",
		Default:     "nginx",
	})

	properties.Set("ini", &jsonschema.Schema{
		Type:        "object",
		Description: "PHP Ini configurations",
		AdditionalProperties: &jsonschema.Schema{
			Type: "string",
		},
	})

	properties.Set("extensions", &jsonschema.Schema{
		Type:        "array",
		Items:       &jsonschema.Schema{Type: "string"},
		Description: "Additional PHP extensions to install",
	})

	properties.Set("profiler", &jsonschema.Schema{
		Type:    "string",
		Enum:    []any{"tideways", ""},
		Default: "",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func (P PHP) Default() ConfigSettings {
	return ConfigSettings{
		"packages":   []any{},
		"env":        make(ConfigSettings),
		"ini":        make(ConfigSettings),
		"version":    "",
		"variant":    "frankenphp",
		"extensions": []any{},
		"profiler":   "",
	}
}

func (P PHP) Supports(root string) bool {
	var composerJson ComposerJson

	if err := readJSONFile(path.Join(root, "composer.json"), &composerJson); err != nil {
		return false
	}

	return !composerJson.HasPackage("shopware/core") && !composerJson.HasPackage("shopware/platform")
}

func init() {
	RegisterLanguage(PHP{})
}
