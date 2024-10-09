//go:build !windows

package cmd

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/streams"
	"os"
	"os/signal"
	"syscall"
)

func monitorTtySize(ctx context.Context, client *client.Client, id string, out *streams.Out) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	go func() {
		for range sigchan {
			resizeTty(ctx, client, id, out)
		}
	}()
}
