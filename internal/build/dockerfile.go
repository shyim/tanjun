package build

import (
	"bytes"
	"github.com/moby/patternmatcher/ignorefile"
	"github.com/shyim/tanjun/internal/buildpack"
	"github.com/shyim/tanjun/internal/config"
	"os"
	"path"
	"slices"
	"strings"
)

func getDockerFile(root string, config *config.ProjectConfig) ([]byte, error) {
	var dockerFile []byte

	if _, err := os.Stat(path.Join(root, config.App.Dockerfile)); os.IsNotExist(err) {
		build, err := buildpack.GenerateImageFile(root)

		if err != nil {
			return nil, err
		}

		dockerFile = []byte(build.Dockerfile)
		if err := os.WriteFile(path.Join(root, ".dockerignore"), []byte(strings.Join(build.Dockerignore, "\n")), 0644); err != nil {
			return nil, err
		}
	} else {
		dockerFile, err = os.ReadFile(path.Join(root, config.App.Dockerfile))

		if err != nil {
			return nil, err
		}
	}
	return dockerFile, nil
}

func getDockerIgnores(root string) ([]string, error) {
	var dockerIgnore []string

	if _, err := os.Stat(path.Join(root, ".dockerignore")); err == nil {
		dockerIgnoreFile, err := os.ReadFile(path.Join(root, ".dockerignore"))

		if err != nil {
			return nil, err
		}

		dockerIgnore, err = ignorefile.ReadAll(bytes.NewBuffer(dockerIgnoreFile))

		if err != nil {
			return nil, err
		}
	}

	if !slices.Contains(dockerIgnore, ".tanjun.yml") {
		dockerIgnore = append(dockerIgnore, ".tanjun.yml")
	}
	return dockerIgnore, nil
}
