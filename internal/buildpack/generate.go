package buildpack

import (
	"fmt"
)

type GeneratedImageResult struct {
	Dockerfile   string
	DockerIgnore []string
}

func (r *GeneratedImageResult) Add(text string, args ...any) {
	r.Dockerfile += fmt.Sprintf(text, args...)
}

func (r *GeneratedImageResult) NewLine() {
	r.Dockerfile += "\n"
}

func (r *GeneratedImageResult) AddLine(line string, args ...any) {
	r.Dockerfile += fmt.Sprintf(line, args...) + "\n"
}

func (r *GeneratedImageResult) AddIgnoreLine(line string) {
	r.DockerIgnore = append(r.DockerIgnore, line)
}

func GenerateImageFile(root string, cfg *Config) (*GeneratedImageResult, error) {
	for _, lang := range supportedLanguages {
		if lang.Name() == cfg.Type {
			for key, value := range lang.Default() {
				if _, ok := cfg.Settings[key]; !ok {
					cfg.Settings[key] = value
				}
			}

			return lang.Generate(root, cfg)
		}
	}

	return nil, fmt.Errorf("unsupported project type given")
}
