// Command stackbroom inspects local development resource usage and suggests
// review opportunities. It never changes anything.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/Caseymccallum/StackBroom/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := cli.New(os.Stdout, os.Stderr)
	os.Exit(app.Run(ctx, os.Args[1:]))
}
