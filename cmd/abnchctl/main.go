package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"

	"github.com/zon/abnch/internal/devconfig"
)

// cli is the abnchctl command line.
type cli struct {
	Config string   `help:"Local service config path." type:"path" default:"${configPath}"`
	Set    setCmd   `cmd:"" help:"Create or update local development resources."`
	Check  checkCmd `cmd:"" help:"Report on local development resources."`
}

// setCmd holds the commands that write local development resources.
type setCmd struct {
	LocalAuth localAuthCmd `cmd:"" name:"local-auth" help:"Generate the local service config, dev key, and CLI credential."`
}

// checkCmd holds the commands that report on local development resources.
type checkCmd struct {
	Config checkConfigCmd `cmd:"" help:"Confirm the local service config is ready."`
}

// localAuthCmd generates local service and CLI auth config.
type localAuthCmd struct {
	Port int `help:"Local service port." default:"8080" short:"p"`
}

// Validate rejects a port outside the TCP range.
func (c localAuthCmd) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d", c.Port)
	}
	return nil
}

// Run writes the local service config, the dev key, and the local CLI credential.
func (c localAuthCmd) Run(deps deps) error {
	path, err := devconfig.SetLocalAuth(devconfig.Options{
		Port:       c.Port,
		RepoRoot:   deps.repoRoot,
		HomeDir:    deps.homeDir,
		ConfigPath: deps.configPath,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(deps.out, "Wrote %s\n", path)
	return nil
}

// checkConfigCmd confirms the local service config is ready.
type checkConfigCmd struct{}

// Run loads the local service config and confirms it is complete.
func (checkConfigCmd) Run(deps deps) error {
	path := deps.configPath

	cfg, err := devconfig.LoadConfig(path)
	if err != nil {
		return err
	}
	if err := devconfig.Check(cfg); err != nil {
		return err
	}

	fmt.Fprintf(deps.out, "Config %s is ready\n", path)
	return nil
}

// deps carries the directories and streams a command works with.
type deps struct {
	repoRoot   string
	homeDir    string
	out        io.Writer
	configPath string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "abnchctl:", err)
		os.Exit(1)
	}
}

// run parses an abnchctl invocation from the current repo.
func run(args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return runCLI(args, deps{repoRoot: root, homeDir: home, out: os.Stdout}, os.Exit)
}

// runIn parses a command with explicit directories and streams. It uses a
// non-terminating exit so a test observes help and parse failures as return
// values instead of a process exit.
func runIn(args []string, repoRoot, homeDir string, out io.Writer) error {
	return runCLI(args, deps{repoRoot: repoRoot, homeDir: homeDir, out: out}, func(int) {})
}

func runCLI(args []string, env deps, exit func(int)) error {
	if env.configPath == "" {
		env.configPath = devconfig.ConfigPath(env.repoRoot)
	}

	var cli cli
	parser, err := kong.New(&cli,
		kong.Name("abnchctl"),
		kong.Description("Local development tooling for the Agent Benchmark service."),
		kong.Writers(env.out, env.out),
		kong.Exit(exit),
		kong.Vars{"configPath": env.configPath},
	)
	if err != nil {
		return err
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	env.configPath = cli.Config
	return ctx.Run(env)
}
