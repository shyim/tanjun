package buildpack

import (
	"fmt"
	"os"
	"path"
)

type GeneratedImageResult struct {
	Dockerfile   string
	Dockerignore []string
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
	r.Dockerignore = append(r.Dockerignore, line)
}

func GenerateImageFile(project string) (*GeneratedImageResult, error) {
	if _, err := os.Stat(path.Join(project, "package.json")); err == nil {
		return generateByNodeJS(project)
	}

	if _, err := os.Stat(path.Join(project, "composer.json")); err == nil {
		return generateByPHP(project)
	}

	if _, err := os.Stat(path.Join(project, "go.mod")); err == nil {
		return generateByGolang()
	}

	return nil, fmt.Errorf("unknown project type")
}
