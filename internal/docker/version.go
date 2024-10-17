package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"slices"
	"sort"
	"strings"
	"time"
)

type Version struct {
	Name      string
	Aliases   []string
	CreatedAt time.Time
	Active    bool
}

func VersionList(ctx context.Context, client *client.Client, cfg *config.ProjectConfig) ([]Version, error) {
	opts := image.ListOptions{Filters: filters.NewArgs(), All: true}

	opts.Filters.Add("reference", cfg.Image)

	images, err := client.ImageList(ctx, opts)

	if err != nil {
		return nil, err
	}

	currentVersion, err := VersionCurrentlyActive(ctx, client, cfg)

	if err != nil {
		return nil, err
	}

	versions := make([]Version, 0, len(images))

	for _, img := range images {
		aliases := make([]string, 0, len(img.RepoTags)-1)

		for index, tag := range img.RepoTags {
			if index == 0 {
				continue
			}

			aliases = append(aliases, strings.TrimPrefix(tag, cfg.Image+":"))
		}

		imageName := strings.TrimPrefix(img.RepoTags[0], cfg.Image+":")
		activeVersion := false
		if imageName == currentVersion || slices.Contains(aliases, currentVersion) {
			activeVersion = true
		}

		versions = append(versions, Version{
			Name:      imageName,
			CreatedAt: time.Unix(img.Created, 0),
			Aliases:   aliases,
			Active:    activeVersion,
		})
	}

	// Sort versions by CreatedAt
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	return versions, nil
}

func VersionDrain(ctx context.Context, client *client.Client, cfg *config.ProjectConfig) error {
	versions, err := VersionList(ctx, client, cfg)

	if err != nil {
		return err
	}

	if len(versions) <= cfg.KeepVersions {
		return nil
	}

	for _, version := range versions[cfg.KeepVersions:] {
		// Skip active versions
		if version.Active {
			continue
		}

		for _, alias := range append(version.Aliases, version.Name) {
			_, err := client.ImageRemove(ctx, cfg.Image+":"+alias, image.RemoveOptions{PruneChildren: true})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func VersionCurrentlyActive(ctx context.Context, client *client.Client, cfg *config.ProjectConfig) (string, error) {
	c, err := getEnvironmentContainers(ctx, client, cfg.Name)

	if err != nil {
		return "", err
	}

	if len(c) == 0 {
		return "", fmt.Errorf("there is no deployment yet for project %s", cfg.Name)
	}

	imageSplit := strings.SplitN(c[0].Image, ":", 2)

	return imageSplit[1], nil
}
