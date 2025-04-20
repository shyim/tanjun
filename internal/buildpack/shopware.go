package buildpack

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type Shopware struct {
}

func (s Shopware) Name() string {
	return "shopware"
}

func (s Shopware) Generate(root string, cfg *Config) (*GeneratedImageResult, error) {
	result := &GeneratedImageResult{}

	addShopwareDefaults(cfg)

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

	switch cfg.Settings["profiler"].(string) {
	case "tideways":
		phpPackages = append(phpPackages, fmt.Sprintf("php-%s-tideways", phpVersion))
	case "blackfire":
		phpPackages = append(phpPackages, fmt.Sprintf("php-%s-blackfire", phpVersion))

		if cfg.Settings["variant"].(string) == "frankenphp" {
			return nil, fmt.Errorf("blackfire is not supported with frankenphp, set variant in buildpack config to nginx or caddy")
		}
	}

	if err != nil {
		return nil, err
	}

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/%s:%s AS builder", cfg.Settings["variant"], imageVersion)
	result.AddLine("COPY --from=shopware/shopware-cli:bin /shopware-cli /usr/local/bin/shopware-cli")
	installPackages := fmt.Sprintf("nodejs-22 composer %s php-%s-phar php-%s-openssl php-%s-curl ", strings.Join(phpPackages, " "), phpVersion, phpVersion, phpVersion)
	addPackagesFromSettings(result, cfg, installPackages)
	addEnvFromSettings(result, cfg)

	if len(cfg.Settings["ini"].(ConfigSettings)) > 0 {
		result.AddLine("COPY <<EOF /etc/php/conf.d/zz-custom.ini")

		for key, value := range cfg.Settings["ini"].(ConfigSettings) {
			result.AddLine("%s=%s", key, value)
		}

		result.AddLine("EOF")
		result.NewLine()
	}

	result.NewLine()

	result.AddLine("WORKDIR /var/www/html")
	result.AddLine("COPY . /var/www/html")
	result.NewLine()
	result.AddLine("RUN mkdir -p custom/plugins && mkdir -p custom/static-plugins")

	if _, err := os.Stat(path.Join(root, "symfony.lock")); os.IsNotExist(err) {
		result.AddLine("RUN composer install --no-scripts && composer recipes:install --force --reset && rm -rf vendor")
	}

	result.AddLine("RUN shopware-cli project ci /var/www/html")

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/%s:%s", cfg.Settings["variant"], imageVersion)

	if cfg.Settings["variant"] == "frankenphp" {
		result.AddLine("ENV SERVER_NAME :80")
		result.AddLine("RUN \\")
		result.AddLine("    mkdir -p /data/caddy && mkdir -p /config/caddy; \\")
		result.AddLine("    apk add --no-cache libcap-utils; \\")
		result.AddLine("		adduser -u 82 -D www-data; \\")
		result.AddLine("    	setcap CAP_NET_BIND_SERVICE=+eip /usr/bin/frankenphp; \\")
		result.AddLine("    	chown -R www-data:www-data /data/caddy && chown -R www-data:www-data /config/caddy; \\")
		result.AddLine("    apk del libcap-utils")
	}

	addPackagesFromSettings(result, cfg, strings.Join(phpPackages, " \\\n "))
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

func (s Shopware) Schema() *jsonschema.Schema {
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
		Enum:    []any{"tideways", "blackfire", ""},
		Default: "",
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
	}
}

func (s Shopware) Default() ConfigSettings {
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

func (s Shopware) Supports(root string) bool {
	var composerJson ComposerJson

	if err := readJSONFile(path.Join(root, "composer.json"), &composerJson); err != nil {
		return false
	}

	return composerJson.HasPackage("shopware/core") || composerJson.HasPackage("shopware/platform")
}

func init() {
	RegisterLanguage(Shopware{})
}

func addShopwareDefaults(cfg *Config) {
	defaultEnv := map[string]any{
		"APP_ENV":                      "prod",
		"APP_URL":                      "http://localhost",
		"APP_URL_CHECK_DISABLED":       "1",
		"LOCK_DSN":                     "flock",
		"MAILER_DSN":                   "null://localhost",
		"BLUE_GREEN_DEPLOYMENT":        "0",
		"SHOPWARE_ES_ENABLED":          "0",
		"SHOPWARE_ES_INDEXING_ENABLED": "0",
		"SHOPWARE_HTTP_CACHE_ENABLED":  "1",
		"SHOPWARE_HTTP_DEFAULT_TTL":    "7200",
		"SHOPWARE_CACHE_ID":            "docker",
		"SHOPWARE_SKIP_WEBINSTALLER":   "1",
		"COMPOSER_HOME":                "/tmp/composer",
		"COMPOSER_ROOT_VERSION":        "1.0.0",
		"INSTALL_LOCALE":               "en-GB",
		"INSTALL_CURRENCY":             "EUR",
		"INSTALL_ADMIN_USERNAME":       "admin",
		"INSTALL_ADMIN_PASSWORD":       "shopware",
	}

	for key, value := range defaultEnv {
		if _, ok := cfg.Settings["env"].(ConfigSettings)[key]; !ok {
			cfg.Settings["env"].(ConfigSettings)[key] = value
		}
	}

	defaultIni := map[string]any{
		"expose_php":                      "Off",
		"memory_limit":                    "512M",
		"display_errors":                  "Off",
		"error_reporting":                 "E_ALL",
		"upload_max_filesize":             "32M",
		"post_max_size":                   "32M",
		"max_execution_time":              "60",
		"opcache.enable_file_override":    "0",
		"opcache.interned_strings_buffer": "20",
		"opcache.max_accelerated_files":   "10000",
		"opcache.memory_consumption":      "128",
		"zend.assertions":                 "-1",
		"zend.detect_unicode":             "0",
	}

	for key, value := range defaultIni {
		if _, ok := cfg.Settings["ini"].(ConfigSettings)[key]; !ok {
			cfg.Settings["ini"].(ConfigSettings)[key] = value
		}
	}
}
