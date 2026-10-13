package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/zon/abnch/internal/devconfig"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "abnchctl:", err)
		os.Exit(1)
	}
}

// run dispatches an abnchctl invocation from the current repo.
func run(args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return runIn(args, root, home, os.Stdin, os.Stdout)
}

// runIn dispatches a command with explicit directories and streams.
func runIn(args []string, repoRoot, homeDir string, in io.Reader, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: abnchctl set local-auth [port] | abnchctl check config")
	}
	switch args[0] + " " + args[1] {
	case "set local-auth":
		return runSetLocalAuth(args[2:], repoRoot, homeDir, in, out)
	case "check config":
		return runCheckConfig(repoRoot, out)
	default:
		return fmt.Errorf("unknown command %q", strings.Join(args, " "))
	}
}

func runSetLocalAuth(args []string, repoRoot, homeDir string, in io.Reader, out io.Writer) error {
	port, err := parsePort(args)
	if err != nil {
		return err
	}

	path, err := devconfig.SetLocalAuth(devconfig.Options{
		Port:     port,
		RepoRoot: repoRoot,
		HomeDir:  homeDir,
		In:       in,
		Out:      out,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Wrote %s\n", path)
	return nil
}

func runCheckConfig(repoRoot string, out io.Writer) error {
	path := devconfig.ConfigPath(repoRoot)

	cfg, err := devconfig.LoadConfig(path)
	if err != nil {
		return err
	}
	if err := devconfig.Check(cfg); err != nil {
		return err
	}

	fmt.Fprintf(out, "Config %s is ready\n", path)
	return nil
}

// parsePort reads the optional port from positional, `--port`, or `port=`
// arguments and defaults to 8080.
func parsePort(args []string) (int, error) {
	value := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--port" || arg == "-p":
			if i+1 >= len(args) {
				return 0, fmt.Errorf("missing value for %s", arg)
			}
			i++
			value = args[i]
		case strings.HasPrefix(arg, "--port="):
			value = strings.TrimPrefix(arg, "--port=")
		case strings.HasPrefix(arg, "port="):
			value = strings.TrimPrefix(arg, "port=")
		default:
			value = arg
		}
	}

	if value == "" {
		return 8080, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q", value)
	}
	return port, nil
}
