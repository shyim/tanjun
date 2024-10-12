package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"sort"
	"strings"
	"time"
)

type Version struct {
	Name      string
	Aliases   []string
	CreatedAt time.Time
}

func VersionList(ctx context.Context, client *client.Client, name string) ([]Version, error) {
	opts := image.ListOptions{Filters: filters.NewArgs(), All: true}

	opts.Filters.Add("reference", name)

	images, err := client.ImageList(ctx, opts)

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

			aliases = append(aliases, strings.TrimPrefix(tag, name+":"))
		}

		versions = append(versions, Version{
			Name:      strings.TrimPrefix(img.RepoTags[0], name+":"),
			CreatedAt: time.Unix(img.Created, 0),
			Aliases:   aliases,
		})
	}

	// Sort versions by CreatedAt
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	return versions, nil
}

func VersionDrain(ctx context.Context, client *client.Client, name string, keep int) error {
	versions, err := VersionList(ctx, client, name)

	if err != nil {
		return err
	}

	if len(versions) <= keep {
		return nil
	}

	for _, version := range versions[keep:] {
		for _, alias := range append(version.Aliases, version.Name) {
			_, err := client.ImageRemove(ctx, name+":"+alias, image.RemoveOptions{PruneChildren: true})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func VersionCurrentlyActive(ctx context.Context, client *client.Client, projectName string) (string, error) {
	c, err := getEnvironmentContainers(ctx, client, projectName)

	if err != nil {
		return "", err
	}

	if len(c) == 0 {
		return "", fmt.Errorf("there is no deployment yet for project %s", projectName)
	}

	imageSplit := strings.SplitN(c[0].Image, ":", 2)

	return imageSplit[1], nil
}
