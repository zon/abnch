// Package main implements the abnch command line interface.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
)

// cli is the abnch command line.
type cli struct {
	Login loginCmd `cmd:"" help:"Sign in a human user and store the session."`
}

// loginCmd signs a human user in through the service.
type loginCmd struct {
	URL string `help:"Service URL." optional:""`
}

// Run signs in and stores the session for the service URL.
func (c loginCmd) Run(deps deps) error {
	return login(deps, c.URL)
}

// deps carries the directories, streams, and browser a command works with.
type deps struct {
	homeDir     string
	out         io.Writer
	ctx         context.Context
	openBrowser func(string) error
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "abnch:", err)
		os.Exit(1)
	}
}

// run parses an abnch invocation from the current directories.
func run(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return runCLI(args, deps{
		homeDir:     home,
		out:         os.Stdout,
		ctx:         context.Background(),
		openBrowser: openBrowser,
	}, os.Exit)
}

// runIn parses a command with explicit directories, a browser, and streams. It
// uses a non-terminating exit so a test observes help and parse failures as
// return values instead of a process exit.
func runIn(args []string, homeDir string, openBrowser func(string) error, ctx context.Context, out io.Writer) error {
	return runCLI(args, deps{homeDir: homeDir, out: out, ctx: ctx, openBrowser: openBrowser}, func(int) {})
}

func runCLI(args []string, env deps, exit func(int)) error {
	var cli cli
	parser, err := kong.New(&cli,
		kong.Name("abnch"),
		kong.Description("Agent Benchmark command line."),
		kong.Writers(env.out, env.out),
		kong.Exit(exit),
	)
	if err != nil {
		return err
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	return ctx.Run(env)
}
