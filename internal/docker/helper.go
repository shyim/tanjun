package docker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/charmbracelet/log"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"io"
	"os"
	"slices"
	"strings"
)

var configFile *configfile.ConfigFile

func init() {
	configFile = config.LoadDefaultConfigFile(os.Stderr)
}

func PullImageIfNotThere(ctx context.Context, client *client.Client, imageName string) error {
	images, err := client.ImageList(ctx, image.ListOptions{})

	if err != nil {
		return err
	}

	imageExists := false

	for _, i := range images {
		if slices.Contains(i.RepoTags, imageName) {
			imageExists = true
			break
		}
	}

	if !imageExists {
		log.Infof("Pulling image: %s\n", imageName)

		opts := image.PullOptions{}

		hasAuth, authStr := loadAuthInfo(imageName)

		if hasAuth {
			opts.RegistryAuth = authStr
		}

		reader, err := client.ImagePull(ctx, imageName, opts)

		if err != nil {
			return err
		}

		if err := logDockerResponse(reader); err != nil {
			return err
		}
	}

	return nil
}

func loadAuthInfo(image string) (bool, string) {
	if !strings.Contains(image, "/") {
		return false, ""
	}

	imagePrefix := strings.Split(image, "/")[0]

	authConfig, err := configFile.GetAuthConfig(imagePrefix)

	if err != nil {
		return false, ""
	}

	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return false, ""
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	return true, authStr
}

func determineUidOfAppContainer(ctx context.Context, client *client.Client, imageName string) (string, error) {
	created, err := client.ContainerCreate(ctx, &container.Config{
		Image:        imageName,
		AttachStdout: true,
		Entrypoint:   []string{"id", "-u"},
	}, &container.HostConfig{AutoRemove: true}, nil, nil, "")

	if err != nil {
		return "", err
	}

	attach, err := client.ContainerAttach(ctx, created.ID, container.AttachOptions{Stdout: true, Stream: true})

	if err != nil {
		return "", err
	}

	defer attach.Close()

	if err := client.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	statusCh, errCh := client.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}

	pr, pw := io.Pipe()

	go func() {
		_, _ = stdcopy.StdCopy(pw, io.Discard, attach.Reader)
	}()

	scanner := bufio.NewScanner(pr)
	scanner.Scan()

	return scanner.Text(), nil
}
