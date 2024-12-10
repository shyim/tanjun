package buildpack

import (
	"fmt"
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
}

func (j ComposerJson) HasPackage(packageName string) bool {
	_, ok := j.Require[packageName]

	return ok
}

func detectPHPVersion(lock ComposerLock) string {
	if php, ok := lock.Platform["php"]; ok {
		constraint, err := version.NewConstraint(php)

		if err != nil {
			return "8.2"
		}

		if constraint.Check(version.Must(version.NewVersion("8.4"))) {
			return "8.4"
		}

		if constraint.Check(version.Must(version.NewVersion("8.3"))) {
			return "8.3"
		}

		return "8.2"
	}

	return "8.2"
}

func getRequiredPHPPackages(phpVersion string, composerJson ComposerJson, lock ComposerLock, cfg *Config) ([]string, error) {
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

	for _, extName := range cfg.Settings["extensions"].([]any) {
		handlePHPExtension(phpVersion, extName.(string), packages)
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
