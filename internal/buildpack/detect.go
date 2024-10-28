package buildpack

import (
	"fmt"
	"github.com/invopop/jsonschema"
)

var supportedLanguages []Language

func RegisterLanguage(language Language) {
	supportedLanguages = append(supportedLanguages, language)
}

type Language interface {
	Name() string
	Generate(root string, cfg *Config) (*GeneratedImageResult, error)
	Schema() *jsonschema.Schema
	Default() ConfigSettings
	Supports(root string) bool
}

func DetectProjectType(root string) (string, error) {
	for _, lang := range supportedLanguages {
		if lang.Supports(root) {
			return lang.Name(), nil
		}
	}

	return "", fmt.Errorf("buildpack does not support this kind of project")
}
