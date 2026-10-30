package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/zon/abnch/internal/service"
	"github.com/zon/abnch/internal/version"
)

// cli is the abnchd command line.
type cli struct {
	Version versionFlag `name:"version" help:"Print version information and quit."`
	Migrate migrateCmd  `cmd:"" help:"Create or update the database tables and exit."`
	Serve   serveCmd    `cmd:"" default:"withargs" help:"Serve the service."`
}

// serveCmd serves the service from a config file.
type serveCmd struct {
	Config string `arg:"" help:"Service config file." type:"path"`
}

// Run starts the service.
func (c serveCmd) Run(kctx *kong.Context, ctx context.Context) error {
	return service.Run(ctx, service.Options{ConfigPath: c.Config, Out: kctx.Stdout})
}

// migrateCmd creates or updates the database tables and exits.
type migrateCmd struct {
	Config string `arg:"" help:"Service config file." type:"path"`
}

// Run migrates the database.
func (c migrateCmd) Run(ctx context.Context) error {
	return service.Migrate(ctx, service.Options{ConfigPath: c.Config})
}

// errVersion stops parsing after the version prints, so a version request is not
// reported as a missing config.
var errVersion = errors.New("version printed")

// versionFlag prints the app version and stops parsing when --version is set.
type versionFlag bool

func (versionFlag) Decode(*kong.DecodeContext) error { return nil }
func (versionFlag) IsBool() bool                     { return true }

func (versionFlag) BeforeApply(app *kong.Kong, vars kong.Vars) error {
	fmt.Fprintln(app.Stdout, vars["version"])
	return errVersion
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "abnchd:", err)
		os.Exit(1)
	}
}

// run parses an abnchd invocation and runs its command.
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
		kong.Vars{"version": version.Version},
		kong.BindTo(ctx, (*context.Context)(nil)),
	)
	if err != nil {
		return err
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		if errors.Is(err, errVersion) {
			return nil
		}
		return err
	}

	return kctx.Run()
}
