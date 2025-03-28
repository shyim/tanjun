package cmd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/spf13/cobra"
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

		defer func() {
			err := client.Close()
			if err != nil {
				log.Warnf("Failed to close docker client: %s", err)
			}
		}()

		if strings.Contains(args[0], ":") {
			parts := strings.SplitN(args[0], ":", 2)
			serviceName := parts[0]

			if serviceName == "application" {
				serviceName = ""
			}

			containerID, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, serviceName)

			if err != nil {
				return err
			}

			return downloadFromContainer(cmd.Context(), client, containerID, parts[1], args[1])
		} else if strings.Contains(args[1], ":") {
			parts := strings.SplitN(args[1], ":", 2)
			serviceName := parts[0]

			if serviceName == "application" {
				serviceName = ""
			}

			containerID, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, serviceName)

			if err != nil {
				return err
			}

			return uploadToContainer(cmd.Context(), client, containerID, args[0], parts[1])
		}

		return fmt.Errorf("invalid arguments, please provide a source and destination examle: tanjun cp application:/path/to/file /local/path")
	},
}

func uploadToContainer(ctx context.Context, c *client.Client, containerID, local, remote string) error {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "tanjun-cp")

	if err != nil {
		return err
	}

	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Warnf("Failed to remove temporary directory %s", tmpDir)
		}
	}()

	tmpFile := filepath.Join(tmpDir, "file.tar")

	tarFile, err := os.Create(tmpFile)

	if err != nil {
		return err
	}

	defer func() {
		err := tarFile.Close()
		if err != nil {
			log.Warnf("Failed to close tar file")
		}
	}()

	tarWriter := tar.NewWriter(tarFile)

	defer func() {
		err := tarWriter.Close()
		if err != nil {
			log.Warnf("Failed to close tar writer: %s", err)
		}
	}()

	stat, err := os.Stat(local)

	if err != nil {
		return err
	}

	if !stat.IsDir() {
		file, err := os.Open(local)

		if err != nil {
			return err
		}

		defer func() {
			err := file.Close()
			if err != nil {
				log.Warnf("Failed to close file %s", local)
			}
		}()

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

				defer func() {
					err := file.Close()
					if err != nil {
						log.Warnf("Failed to close file %s", path)
					}
				}()

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

	defer func() {
		err := generatedTar.Close()
		if err != nil {
			log.Warnf("Failed to close generated tar file %s", tmpFile)
		}
	}()

	return c.CopyToContainer(ctx, containerID, remote, generatedTar, container.CopyToContainerOptions{})
}

func downloadFromContainer(ctx context.Context, c *client.Client, containerID, remote, local string) error {
	resp, _, err := c.CopyFromContainer(ctx, containerID, remote)

	if err != nil {
		return err
	}

	defer func() {
		err := resp.Close()
		if err != nil {
			log.Warnf("Failed to close response body: %s", err)
		}
	}()

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
				if closeErr := f.Close(); closeErr != nil {
					log.Warnf("Failed to close file %s: %s", targetPath, closeErr)
				}
				return fmt.Errorf("failed to write file contents: %w", err)
			}
			if err = f.Close(); err != nil {
				log.Warnf("Failed to close file %s: %s", targetPath, err)
			}
		case tar.TypeSymlink:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create directories for symlink: %w", err)
			}

			// Validate the symlink target to prevent directory traversal
			linkDest := header.Linkname

			// If the link is absolute, reject it
			if filepath.IsAbs(linkDest) {
				log.Warnf("skipping absolute symlink: %s -> %s", header.Name, linkDest)
				continue
			}

			// Construct the full path that the symlink would point to
			fullLinkPath := filepath.Join(filepath.Dir(targetPath), linkDest)

			// Clean the path to resolve any ".." components
			fullLinkPath = filepath.Clean(fullLinkPath)

			// Check if the link would escape the extraction directory
			localAbs, err := filepath.Abs(local)
			if err != nil {
				return fmt.Errorf("failed to get absolute path of extraction directory: %w", err)
			}

			// Ensure the symlink target is within the extraction directory
			if !strings.HasPrefix(fullLinkPath, localAbs) {
				log.Warnf("skipping symlink that escapes extraction directory: %s -> %s", header.Name, linkDest)
				continue
			}

			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symbolic link: %w", err)
			}
		default:
			log.Warnf("unknown type: %s in %s", string(header.Typeflag), header.Name)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(copyCmd)
}
