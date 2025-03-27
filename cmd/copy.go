package cmd

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strings"
)


var copyCmd = &cobra.Command{
	Use:   "cp",
	Short: "Copy out of containers or to containers",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.CreateConfig(configFile)

		if err != nil {
			return err
		}

		client, err := docker.CreateClientFromConfig(cfg)

		if err != nil {
			return err
		}

		defer client.Close()

		if strings.Contains(args[0], ":") {
			parts := strings.SplitN(args[0], ":", 2)
			serviceName := parts[0]

			if serviceName == "application" {
				serviceName = ""
			}

			containerId, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, serviceName)

			if err != nil {
				return err
			}

			return downloadFromContainer(cmd.Context(), client, containerId, parts[1], args[1])
		} else if strings.Contains(args[1], ":") {
			parts := strings.SplitN(args[1], ":", 2)
			serviceName := parts[0]

			if serviceName == "application" {
				serviceName = ""
			}

			containerId, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, serviceName)

			if err != nil {
				return err
			}

			return uploadToContainer(cmd.Context(), client, containerId, args[0], parts[1])
		}

		return fmt.Errorf("invalid arguments, please provide a source and destination examle: tanjun cp application:/path/to/file /local/path")
	},
}

func uploadToContainer(ctx context.Context, c *client.Client, containerId, local, remote string) error {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "tanjun-cp")

	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "file.tar")

	tarFile, err := os.Create(tmpFile)

	if err != nil {
		return err
	}

	defer tarFile.Close()

	tarWriter := tar.NewWriter(tarFile)

	defer tarWriter.Close()

	stat, err := os.Stat(local)

	if err != nil {
		return err
	}

	if !stat.IsDir() {
		file, err := os.Open(local)

		if err != nil {
			return err
		}

		defer file.Close()

		fileStat, err := file.Stat()

		if err != nil {
			return err
		}

		header := &tar.Header{
			Name:     file.Name(),
			Size:     fileStat.Size(),
			Mode:     int64(fileStat.Mode()),
			Typeflag: tar.TypeReg,
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			return err
		}
	} else {
		err = filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(info, info.Name())

			if err != nil {
				return err
			}

			header.Name = strings.TrimPrefix(path, local)

			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			if !info.IsDir() {
				file, err := os.Open(path)

				if err != nil {
					return err
				}

				defer file.Close()

				if _, err := io.Copy(tarWriter, file); err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return err
	}

	generatedTar, err := os.Open(tmpFile)

	if err != nil {
		return err
	}

	return c.CopyToContainer(ctx, containerId, remote, generatedTar, container.CopyToContainerOptions{})
}

func downloadFromContainer(ctx context.Context, c *client.Client, containerId, remote, local string) error {
	resp, _, err := c.CopyFromContainer(ctx, containerId, remote)

	if err != nil {
		return err
	}

	defer resp.Close()

	tarReader := tar.NewReader(resp)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("failed to read next tar header: %w", err)
		}

		// Check for directory traversal sequences
		if strings.Contains(header.Name, "..") {
			log.Warnf("skipping potentially unsafe file path: %s", header.Name)
			continue
		}

		targetPath := filepath.Join(local, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure directory hierarchy exists for file
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create directories for file path: %w", err)
			}

			// Create and copy the file contents
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to open file for writing: %w", err)
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file contents: %w", err)
			}
			f.Close()
		case tar.TypeSymlink:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create directories for symlink: %w", err)
			}

			// Create the symbolic link
			// Resolve symbolic links and validate the target path
			realTargetPath, err := filepath.EvalSymlinks(targetPath)
			if err != nil {
				return fmt.Errorf("failed to resolve symbolic link: %w", err)
			}
			if !isRel(realTargetPath, local) {
				log.Warnf("skipping potentially unsafe symlink: %s -> %s", header.Name, header.Linkname)
				continue
			}
			if err := os.Symlink(header.Linkname, realTargetPath); err != nil {
				return fmt.Errorf("failed to create symbolic link: %w", err)
			}
		default:
			log.Warnf("unknown type: %s in %s", string(header.Typeflag), header.Name)
		}
	}

	return err
}

func init() {
	rootCmd.AddCommand(copyCmd)
}
