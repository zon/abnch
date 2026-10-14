package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/zon/abnch/internal/service"
)

// cli is the abnchd command line.
type cli struct {
	Config string `arg:"" help:"Service config file." type:"path"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "abnchd:", err)
		os.Exit(1)
	}
}

// run parses an abnchd invocation and starts the service.
func run(ctx context.Context, args []string, out io.Writer) error {
	return runCLI(ctx, args, out, os.Exit)
}

// runIn parses a command with explicit streams. It uses a non-terminating exit
// so a test observes help and parse failures as return values instead of a
// process exit.
func runIn(ctx context.Context, args []string, out io.Writer) error {
	return runCLI(ctx, args, out, func(int) {})
}

func runCLI(ctx context.Context, args []string, out io.Writer, exit func(int)) error {
	var c cli
	parser, err := kong.New(&c,
		kong.Name("abnchd"),
		kong.Description("The Agent Benchmark service."),
		kong.Writers(out, out),
		kong.Exit(exit),
	)
	if err != nil {
		return err
	}
	if _, err := parser.Parse(args); err != nil {
		return err
	}

	return service.Run(ctx, service.Options{ConfigPath: c.Config, Out: out})
}
