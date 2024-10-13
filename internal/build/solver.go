package build

import (
	"context"
	"fmt"
	"github.com/charmbracelet/log"
	dockerConfig "github.com/docker/cli/cli/config"
	"github.com/docker/docker/pkg/namesgenerator"
	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/shyim/tanjun/internal/config"
	"github.com/tonistiigi/fsutil"
	"os"
	"path"
)

func getSolveConfiguration(ctx context.Context, containerConfig string) (string, *buildkit.SolveOpt, error) {
	version := namesgenerator.GetRandomName(0)

	configFile := ctx.Value(contextConfigField).(*config.ProjectConfig)

	fsRoot, err := fsutil.NewFS(ctx.Value(contextRootPathField).(string))

	if err != nil {
		return "", nil, err
	}

	cacheDir, err := os.UserCacheDir()

	if err != nil {
		return "", nil, err
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

	attachables := []session.Attachable{
		authprovider.NewDockerAuthProvider(dockerConfig.LoadDefaultConfigFile(os.Stderr), nil),
	}

	if configFile.Build.PassThroughSSHSocket {
		if sshAgent, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{ID: "default"}}); err == nil {
			attachables = append(attachables, sshAgent)
		} else {
			log.Warnf("Failed to create SSH agent provider: %s", err)
		}
	}

	attachables = append(attachables, secretsprovider.NewSecretProvider(secretStore{config: configFile}))

	solveOpt := buildkit.SolveOpt{
		Session: attachables,
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
					"name":                  fmt.Sprintf("%s:%s", configFile.Image, version),
					"push":                  "true",
					"containerimage.config": containerConfig,
				},
			},
		},
	}

	return "", &solveOpt, nil
}
