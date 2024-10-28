package buildpack

import (
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"os"
	"path"
)

type GoLang struct {
}

func (g GoLang) Default() ConfigSettings {
	return ConfigSettings{
		"port":     "3000",
		"packages": []any{},
		"env":      make(ConfigSettings),
	}
}

func (g GoLang) Name() string {
	return "go"
}

func (g GoLang) Supports(root string) bool {
	_, err := os.Stat(path.Join(root, "go.mod"))

	return err == nil
}

func (g GoLang) Generate(_ string, cfg *Config) (*GeneratedImageResult, error) {
	result := &GeneratedImageResult{}

	result.AddLine("FROM chainguard/wolfi-base:latest as builder")

	addPackagesFromSettings(result, cfg, "go")

	result.NewLine()
	result.AddLine("WORKDIR /code")
	result.AddLine("COPY . .")

	addEnvFromSettings(result, cfg)

	result.AddLine("RUN go build -o /application")
	result.NewLine()

	result.AddLine("FROM chainguard/wolfi-base:latest")

	addPackagesFromSettings(result, cfg, "")

	result.AddLine("COPY --from=builder /application /application")
	result.AddLine("WORKDIR /app")

	addEnvFromSettings(result, cfg)

	result.AddLine("CMD /application")
	result.AddLine("EXPOSE %s", cfg.Settings["port"])

	return result, nil
}

func (g GoLang) Schema() *jsonschema.Schema {
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

func init() {
	RegisterLanguage(&GoLang{})
}
