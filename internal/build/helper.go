package build

import (
	"context"
	"github.com/charmbracelet/log"
	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"os"
)

func createSolveChan(ctx context.Context) chan *buildkit.SolveStatus {
	ch := make(chan *buildkit.SolveStatus, 1)
	display, _ := progressui.NewDisplay(os.Stdout, "auto")

	go func() {
		_, err := display.UpdateFrom(ctx, ch)

		if err != nil {
			log.Warnf("Failed to update display: %s", err)
		}

		// wait until end of ch
		for range ch {
		}
	}()

	return ch
}
