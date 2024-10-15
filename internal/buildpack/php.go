package buildpack

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/shyim/go-version"
)

type ComposerLock struct {
	Platform map[string]string `json:"platform"`
	Packages []struct {
		Name    string            `json:"name"`
		Require map[string]string `json:"require"`
	} `json:"packages"`
}

func (l ComposerLock) HasPackage(packageName string) bool {
	for _, pkg := range l.Packages {
		if pkg.Name == packageName {
			return true
		}
	}

	return false
}

type ComposerJson struct {
	Require map[string]string `json:"require"`
	Replace map[string]string `json:"replace"`
	Extra   struct {
		Buildpack struct {
			Variant    string            `json:"variant"`
			Extensions []string          `json:"extensions"`
			Ini        map[string]string `json:"ini"`
			Env        map[string]string `json:"env"`
		} `json:"build-pack"`
	} `json:"extra"`
}

func (j ComposerJson) GetEnvironmentVariables() []string {
	mapped := make([]string, 0, len(j.Extra.Buildpack.Env))

	for key, value := range j.Extra.Buildpack.Env {
		mapped = append(mapped, fmt.Sprintf("%s=%s", key, value))
	}

	return mapped
}

func generateByPHP(project string) (*GeneratedImageResult, error) {
	var result GeneratedImageResult

	result.AddIgnoreLine("vendor")

	var composerJson ComposerJson
	var composerLock ComposerLock

	if err := readJSONFile(path.Join(project, "composer.lock"), &composerLock); err != nil {
		return nil, err
	}

	if err := readJSONFile(path.Join(project, "composer.json"), &composerJson); err != nil {
		return nil, err
	}

	phpVersion := detectPHPVersion(composerLock)

	imageVersion := phpVersion

	if composerJson.Extra.Buildpack.Variant == "" {
		composerJson.Extra.Buildpack.Variant = "nginx"
	}

	if composerJson.Extra.Buildpack.Variant == "frankenphp" {
		phpVersion = fmt.Sprintf("frankenphp-%s", phpVersion)
	}

	phpPackages, err := getRequiredPHPPackages(phpVersion, composerJson, composerLock)

	if err != nil {
		return nil, err
	}

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/%s:%s as builder", composerJson.Extra.Buildpack.Variant, imageVersion)
	result.AddLine("RUN apk add --no-cache composer %s php-%s-phar \\\n php-%s-openssl \\\n php-%s-curl \\\n unzip", strings.Join(phpPackages, " \\\n "), phpVersion, phpVersion, phpVersion)
	result.NewLine()

	result.AddLine("WORKDIR /var/www/html")
	result.AddLine("COPY . /var/www/html")
	result.AddLine("RUN composer install --no-interaction --no-progress")

	result.NewLine()

	result.AddLine("FROM ghcr.io/shyim/wolfi-php/%s:%s", composerJson.Extra.Buildpack.Variant, imageVersion)

	if composerJson.Extra.Buildpack.Variant == "frankenphp" {
		if composerLock.HasPackage("runtime/frankenphp-symfony") {
			result.AddLine("ENV APP_RUNTIME=Runtime\\\\FrankenPhpSymfony\\\\Runtime FRANKENPHP_CONFIG=\"worker ./public/index.php\"")
		}

		result.AddLine("ENV SERVER_NAME :8000")
		result.AddLine("RUN \\")
		result.AddLine("    mkdir -p /data/caddy && mkdir -p /config/caddy; \\")
		result.AddLine("    apk add --no-cache libcap-utils; \\")
		result.AddLine("		adduser -u 82 -D www-data; \\")
		result.AddLine("    	setcap CAP_NET_BIND_SERVICE=+eip /usr/bin/frankenphp; \\")
		result.AddLine("    	chown -R www-data:www-data /data/caddy && chown -R www-data:www-data /config/caddy; \\")
		result.AddLine("    apk del libcap-utils")
	}

	result.AddLine("RUN apk add --no-cache curl \\\n %s", strings.Join(phpPackages, " \\\n "))
	result.NewLine()

	if composerJson.Extra.Buildpack.Variant == "frankenphp" {
		result.AddLine("COPY --from=builder --chown=82:82 /var/www/html /app")
	} else {
		result.AddLine("COPY --from=builder --chown=82:82 /var/www/html /var/www/html")
	}

	if len(composerJson.Extra.Buildpack.Ini) > 0 {
		result.AddLine("COPY <<EOF /etc/php/conf.d/zz-custom.ini")

		for key, value := range composerJson.Extra.Buildpack.Ini {
			result.AddLine("%s=%s", key, value)
		}

		result.AddLine("EOF")
		result.NewLine()
	}

	if len(composerJson.Extra.Buildpack.Env) > 0 {
		result.AddLine("ENV %s", strings.Join(composerJson.GetEnvironmentVariables(), " \\\n "))
	}

	result.AddLine("EXPOSE 8000")
	result.AddLine("HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 CMD curl -f http://localhost:8000 || exit 1")
	result.Add("USER www-data")

	return &result, nil
}

func detectPHPVersion(lock ComposerLock) string {
	if php, ok := lock.Platform["php"]; ok {
		constraint, err := version.NewConstraint(php)

		if err != nil {
			return "8.2"
		}

		if constraint.Check(version.Must(version.NewVersion("8.3"))) {
			return "8.3"
		}

		return "8.2"
	}

	return "8.2"
}

func getRequiredPHPPackages(phpVersion string, composerJson ComposerJson, lock ComposerLock) ([]string, error) {
	var packages = make(map[string]string)

	packages[fmt.Sprintf("php-%s", phpVersion)] = fmt.Sprintf("php-%s", phpVersion)
	packages[fmt.Sprintf("php-%s-opcache", phpVersion)] = fmt.Sprintf("php-%s-opcache", phpVersion)

	for _, pkg := range lock.Packages {
		for name := range pkg.Require {
			if !strings.HasPrefix(name, "ext-") {
				continue
			}

			handlePHPExtension(phpVersion, strings.TrimPrefix(name, "ext-"), packages)
		}
	}

	for name := range composerJson.Replace {
		if !strings.HasPrefix(name, "symfony/polyfill-") {
			continue
		}

		extName := strings.TrimPrefix(name, "symfony/polyfill-")

		if extName == "iconv" || extName == "ctype" || extName == "mbstring" || extName == "apcu" {
			handlePHPExtension(phpVersion, extName, packages)
		}

		if strings.HasPrefix(extName, "intl") {
			handlePHPExtension(phpVersion, "intl", packages)
		}
	}

	for _, extName := range composerJson.Extra.Buildpack.Extensions {
		handlePHPExtension(phpVersion, extName, packages)
	}

	keys := make([]string, 0, len(packages))

	for _, v := range packages {
		keys = append(keys, v)
	}

	return keys, nil
}

var phpBuiltinExtensions = []string{
	"filter",
	"json",
	"pcre",
	"session",
	"zlib",
}

func handlePHPExtension(phpVersion string, extName string, packages map[string]string) {
	if slices.Contains(phpBuiltinExtensions, extName) {
		return
	}

	if extName == "pdo_mysql" || extName == "mysqli" {
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "mysqlnd")] = fmt.Sprintf("php-%s-%s", phpVersion, "mysqlnd")
	}

	packages[fmt.Sprintf("php-%s-%s", phpVersion, extName)] = fmt.Sprintf("php-%s-%s", phpVersion, extName)

	if strings.HasPrefix(extName, "pdo") {
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "pdo")] = fmt.Sprintf("php-%s-%s", phpVersion, "pdo")
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "curl")] = fmt.Sprintf("php-%s-%s", phpVersion, "curl")
	}

	if extName == "xml" {
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "xmlreader")] = fmt.Sprintf("php-%s-%s", phpVersion, "xmlreader")
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "xmlwriter")] = fmt.Sprintf("php-%s-%s", phpVersion, "xmlwriter")
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "dom")] = fmt.Sprintf("php-%s-%s", phpVersion, "dom")
		packages[fmt.Sprintf("php-%s-%s", phpVersion, "simplexml")] = fmt.Sprintf("php-%s-%s", phpVersion, "simplexml")
	}

	if extName == "openssl" {
		packages["openssl-config"] = "openssl-config"
	}
}
