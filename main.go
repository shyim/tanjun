package main

import (
	"context"

	"github.com/shyim/tanjun/cmd"
)

func main() {
	ctx := context.Background()
	cmd.Execute(ctx)
}
