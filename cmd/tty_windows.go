//go:build windows

package cmd

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/shyim/tanjun/internal/streams"
	"time"
)

func monitorTtySize(ctx context.Context, client *client.Client, id string, out *streams.Out) {
	go func() {
		prevH, prevW := out.GetTtySize()
		for {
			time.Sleep(time.Millisecond * 250)
			h, w := out.GetTtySize()

			if prevW != w || prevH != h {
				resizeTty(ctx, client, id, out)
			}
			prevH = h
			prevW = w
		}
	}()
}
