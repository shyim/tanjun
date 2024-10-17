package build

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/moby/patternmatcher/ignorefile"
	"github.com/shyim/tanjun/internal/buildpack"
	"github.com/shyim/tanjun/internal/config"
)

func getDockerFile(root string, config *config.ProjectConfig) ([]byte, []string, error) {
	var dockerFile []byte
	var dockerIgnore []string

	if _, err := os.Stat(path.Join(root, config.App.Dockerfile)); os.IsNotExist(err) {
		build, err := buildpack.GenerateImageFile(root)

		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate Dockerfile: %w", err)
		}

		dockerFile = []byte(build.Dockerfile)
		dockerIgnore = build.Dockerignore
	} else {
		dockerFile, err = os.ReadFile(path.Join(root, config.App.Dockerfile))

		if err != nil {
			return nil, nil, err
		}

		if _, err := os.Stat(path.Join(root, ".dockerignore")); err == nil {
			dockerIgnoreFile, err := os.ReadFile(path.Join(root, ".dockerignore"))

			if err != nil {
				return nil, nil, err
			}

			dockerIgnore, err = ignorefile.ReadAll(bytes.NewBuffer(dockerIgnoreFile))

			if err != nil {
				return nil, nil, err
			}
		}
	}

	if !slices.Contains(dockerIgnore, ".tanjun.yml") {
		dockerIgnore = append(dockerIgnore, ".tanjun.yml")
	}

	return dockerFile, dockerIgnore, nil
}