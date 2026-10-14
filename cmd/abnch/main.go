// Package main implements the abnch command line interface.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

// cli is the abnch command line.
type cli struct {
	Login  loginCmd  `cmd:"" help:"Sign in a human user and store the session."`
	Logout logoutCmd `cmd:"" help:"Remove the stored session."`
	Set    setCmd    `cmd:"" help:"Create or update resources in the inferred repo."`
}

// setCmd groups the commands that create or update resources.
type setCmd struct {
	Task setTaskCmd `cmd:"" help:"Create or update a task in the inferred repo."`
}

// setTaskCmd creates, updates, or renames a task in the inferred repo.
type setTaskCmd struct {
	Stub        string `arg:"" help:"Task stub."`
	Name        string `help:"Task name." optional:""`
	Description string `help:"Task description." optional:""`
	Move        string `help:"Rename the task to this stub." optional:""`
}

// Run creates or updates the task in the inferred repo.
func (c setTaskCmd) Run(deps deps) error {
	return setTask(deps, c)
}

// loginCmd signs a human user in through the service.
type loginCmd struct {
	URL string `help:"Service URL." optional:""`
}

// Run signs in and stores the session for the service URL.
func (c loginCmd) Run(deps deps) error {
	return login(deps, c.URL)
}

// logoutCmd removes the stored session for the service.
type logoutCmd struct {
	URL string `help:"Service URL." optional:""`
}

// Run removes the stored session for the service URL.
func (c logoutCmd) Run(deps deps) error {
	return logout(deps, c.URL)
}

// deps carries the directories, streams, and browser a command works with.
type deps struct {
	homeDir     string
	workDir     string
	in          io.Reader
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
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCLI(args, deps{
		homeDir:     home,
		workDir:     workDir,
		in:          os.Stdin,
		out:         os.Stdout,
		ctx:         context.Background(),
		openBrowser: openBrowser,
	}, os.Exit)
}

// runIn parses a command with explicit directories, a browser, and streams. It
// uses a non-terminating exit so a test observes help and parse failures as
// return values instead of a process exit.
func runIn(args []string, homeDir, workDir string, openBrowser func(string) error, ctx context.Context, out io.Writer) error {
	return runCLI(args, deps{homeDir: homeDir, workDir: workDir, in: strings.NewReader(""), out: out, ctx: ctx, openBrowser: openBrowser}, func(int) {})
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
