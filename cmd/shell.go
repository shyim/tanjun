package cmd

import (
	"context"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	"github.com/shyim/tanjun/internal/streams"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"os"
	"strings"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Opens a shell to the app container or shell",
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

		serviceName, _ := cmd.PersistentFlags().GetString("service")

		containerId, err := docker.FindProjectContainer(cmd.Context(), client, cfg.Name, serviceName)

		if err != nil {
			return err
		}

		tty := term.IsTerminal(0)

		execConfig := container.ExecOptions{
			Tty:          tty,
			AttachStderr: true,
			AttachStdout: true,
			AttachStdin:  true,
			Cmd:          []string{"sh"},
		}

		if len(args) > 0 {
			execConfig.Cmd = []string{"sh", "-c", strings.Join(args, " ")}
		}

		exec, err := client.ContainerExecCreate(cmd.Context(), containerId, execConfig)

		if err != nil {
			return err
		}

		var consoleSizes *[2]uint

		out := streams.NewOut(os.Stdout)

		if term.IsTerminal(0) {
			width, height, err := term.GetSize(0)

			if err != nil {
				return err
			}

			consoleSizes = &[2]uint{uint(height), uint(width)}

			monitorTtySize(cmd.Context(), client, exec.ID, out)
		}

		resp, err := client.ContainerExecAttach(cmd.Context(), exec.ID, container.ExecStartOptions{
			Tty:         tty,
			ConsoleSize: consoleSizes,
		})

		if err != nil {
			return err
		}

		defer resp.Close()

		defer func() {
			if err := resp.CloseWrite(); err != nil {
				log.Warnf("error closing write: %s", err)
			}

			if err := resp.Conn.Close(); err != nil {
				fmt.Println("Error closing connection:", err)
			}
		}()

		streamer := hijackedIOStreamer{
			in:           streams.NewIn(os.Stdin),
			out:          out,
			inputStream:  os.Stdin,
			outputStream: os.Stdout,
			errorStream:  os.Stderr,
			resp:         resp,
			tty:          tty,
		}

		if err := streamer.stream(cmd.Context()); err != nil {
			if !strings.HasSuffix(err.Error(), " EOF") {
				return fmt.Errorf("stream error: %w", err)
			}
		}

		inspectResult, err := client.ContainerExecInspect(cmd.Context(), exec.ID)

		if err != nil {
			return err
		}

		if inspectResult.ExitCode != 0 {
			return fmt.Errorf("exit code %d", inspectResult.ExitCode)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
	shellCmd.PersistentFlags().String("service", "", "Specify service name to tail logs from, otherwise app is used")
}

func resizeTty(ctx context.Context, client *client.Client, id string, out *streams.Out) {
	h, w := out.GetTtySize()

	if err := client.ContainerExecResize(ctx, id, container.ResizeOptions{
		Height: h,
		Width:  w,
	}); err != nil {
		fmt.Println("Error resizing terminal:", err)
	}
}
