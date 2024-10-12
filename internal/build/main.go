package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/patternmatcher/ignorefile"
	"github.com/shyim/tanjun/internal/buildpack"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	dockerConfig "github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/namesgenerator"
	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/frontend/dockerfile/dockerfile2llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress/progressui"
	imageSpecsV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/tonistiigi/fsutil"
)

func BuildImage(ctx context.Context, config *config.ProjectConfig, root string) (string, error) {
	var dockerClient *client.Client
	var err error

	remoteClient, err := docker.CreateClientFromConfig(config)

	if err != nil {
		return "", err
	}

	info, err := remoteClient.Info(ctx)

	if err != nil {
		return "", err
	}

	if err := remoteClient.Close(); err != nil {
		return "", err
	}

	var c container.CreateResponse

	dockerClient, err = client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		return "", err
	}

	if err := docker.PullImageIfNotThere(ctx, dockerClient, "moby/buildkit:v0.16.0"); err != nil {
		return "", err
	}

	c, err = dockerClient.ContainerCreate(ctx, &container.Config{
		Image: "moby/buildkit:v0.16.0",
	}, &container.HostConfig{Privileged: true}, nil, nil, "")

	if err != nil {
		return "", err
	}

	time.Sleep(2 * time.Second)

	if err := dockerClient.ContainerStart(ctx, c.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	defer func() {
		if err := dockerClient.ContainerKill(ctx, c.ID, "SIGKILL"); err != nil {
			fmt.Println(err)
		}

		if err := dockerClient.ContainerRemove(ctx, c.ID, container.RemoveOptions{}); err != nil {
			fmt.Println(err)
		}

		if err := dockerClient.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	var dockerFile []byte

	if _, err := os.Stat(path.Join(root, config.App.Dockerfile)); os.IsNotExist(err) {
		build, err := buildpack.GenerateImageFile(root)

		if err != nil {
			return "", err
		}

		dockerFile = []byte(build.Dockerfile)
		if err := os.WriteFile(path.Join(root, ".dockerignore"), []byte(strings.Join(build.Dockerignore, "\n")), 0644); err != nil {
			return "", err
		}
	} else {
		dockerFile, err = os.ReadFile(path.Join(root, config.App.Dockerfile))

		if err != nil {
			return "", err
		}
	}

	var dockerIgnore []string

	if _, err := os.Stat(path.Join(root, ".dockerignore")); err == nil {
		dockerIgnoreFile, err := os.ReadFile(path.Join(root, ".dockerignore"))

		if err != nil {
			return "", err
		}

		dockerIgnore, err = ignorefile.ReadAll(bytes.NewBuffer(dockerIgnoreFile))

		if err != nil {
			return "", err
		}
	}

	if !slices.Contains(dockerIgnore, ".tanjun.yml") {
		dockerIgnore = append(dockerIgnore, ".tanjun.yml")
	}

	caps := pb.Caps.CapSet(pb.Caps.All())

	local := llb.Local("context", llb.ExcludePatterns(dockerIgnore))
	state, img, _, _, err := dockerfile2llb.Dockerfile2LLB(ctx, dockerFile, dockerfile2llb.ConvertOpt{
		MainContext:  &local,
		MetaResolver: imagemetaresolver.Default(),
		LLBCaps:      &caps,
		TargetPlatform: &imageSpecsV1.Platform{
			OS:           "linux",
			Architecture: info.Architecture,
		},
	})

	if err != nil {
		return "", err
	}

	def, err := state.Marshal(ctx)

	if err != nil {
		return "", err
	}

	activeDockerClient = dockerClient

	builder, err := buildkit.New(ctx, fmt.Sprintf("tanjun://%s", c.ID))

	if err != nil {
		return "", err
	}

	defer builder.Close()

	fsRoot, err := fsutil.NewFS(root)

	if err != nil {
		return "", err
	}

	ch := make(chan *buildkit.SolveStatus, 1)
	display, err := progressui.NewDisplay(os.Stdout, "auto")

	go func() {
		_, err := display.UpdateFrom(ctx, ch)

		if err != nil {
			fmt.Println(err)
		}

		// wait until end of ch
		for range ch {
		}
	}()

	if err != nil {
		return "", err
	}

	containerConfig, err := json.Marshal(img)

	if err != nil {
		return "", err
	}

	cacheDir, err := os.UserCacheDir()

	if err != nil {
		return "", err
	}

	cacheExports := []buildkit.CacheOptionsEntry{
		{
			Type: "local",
			Attrs: map[string]string{
				"dest":         path.Join(cacheDir, "tanjun", "buildkit", "cache"),
				"src":          path.Join(cacheDir, "tanjun", "buildkit", "cache"),
				"ignore-error": "true",
			},
		},
	}

	version := namesgenerator.GetRandomName(1)

	_, err = builder.Solve(ctx, def, buildkit.SolveOpt{
		Session: []session.Attachable{authprovider.NewDockerAuthProvider(dockerConfig.LoadDefaultConfigFile(os.Stderr), nil)},
		LocalMounts: map[string]fsutil.FS{
			"context":    fsRoot,
			"dockerfile": fsRoot,
		},
		CacheExports: cacheExports,
		CacheImports: cacheExports,
		Exports: []buildkit.ExportEntry{
			{
				Type: buildkit.ExporterImage,
				Attrs: map[string]string{
					"name":                  fmt.Sprintf("%s:%s", config.Image, version),
					"push":                  "true",
					"containerimage.config": string(containerConfig),
				},
			},
		},
	}, ch)

	if err != nil {
		return "", err
	}

	return version, nil
}
