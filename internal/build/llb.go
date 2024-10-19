package build

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/docker/api/types/system"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/frontend/dockerfile/dockerfile2llb"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/solver/pb"
	imageSpecsV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/shyim/tanjun/internal/config"
)

func llbFromProject(ctx context.Context, info system.Info) (string, *llb.Definition, error) {
	root := ctx.Value(contextRootPathField).(string)
	configFile := ctx.Value(contextConfigField).(*config.ProjectConfig)

	dockerFile, dockerIgnore, err := getDockerFile(root, configFile)
	if err != nil {
		return "", nil, err
	}

	architecture := info.Architecture

	if architecture == "aarch64" {
		architecture = "arm64"
	}

	caps := pb.Caps.CapSet(pb.Caps.All())

	local := llb.Local("context", llb.ExcludePatterns(dockerIgnore))
	state, img, _, _, err := dockerfile2llb.Dockerfile2LLB(ctx, dockerFile, dockerfile2llb.ConvertOpt{
		MainContext:  &local,
		MetaResolver: imagemetaresolver.Default(),
		LLBCaps:      &caps,
		TargetPlatform: &imageSpecsV1.Platform{
			OS:           "linux",
			Architecture: architecture,
		},
		Config: dockerui.Config{
			Labels:    configFile.Build.Labels,
			BuildArgs: configFile.Build.BuildArgs,
		},
	})

	if err != nil {
		return "", nil, fmt.Errorf("failed to convert Dockerfile to LLB: %w", err)
	}

	def, err := state.Marshal(ctx)

	if err != nil {
		return "", nil, err
	}

	containerConfig, err := json.Marshal(img)

	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal image: %w", err)
	}

	return string(containerConfig), def, nil
}
