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
	Get    getCmd    `cmd:"" help:"Read resources from the inferred repo."`
	Set    setCmd    `cmd:"" help:"Create or update resources in the inferred repo."`
	Add    addCmd    `cmd:"" help:"Add resources to the inferred repo."`
	Run    runCmd    `cmd:"" help:"Run a task's agent against a test's data."`
}

// runCmd runs a task's agent against the data in a test.
type runCmd struct {
	Path    string `arg:"" help:"Task, test, and optional datum ref as <task>/<test>[/<ref>]."`
	Model   string `help:"Override the task's default model." optional:""`
	Limit   int    `help:"Cap the number of data run." default:"-1"`
	Shuffle bool   `help:"Randomize the data order."`
	Local   bool   `help:"Select a local run, the only run mode."`
}

// Run runs the agent against the test's data and records a result for each.
func (c runCmd) Run(deps deps) error {
	return runTask(deps, c)
}

// addCmd groups the commands that add resources.
type addCmd struct {
	Data addDataCmd `cmd:"" help:"Add a datum to a test in the inferred repo."`
}

// addDataCmd adds a datum to a test under a task in the inferred repo.
type addDataCmd struct {
	Path string `arg:"" help:"Task and test stub as <task>/<stub>."`
	File string `help:"Path to a JSON file holding the datum." optional:""`
}

// Run adds the datum to the test in the inferred repo.
func (c addDataCmd) Run(deps deps) error {
	return addData(deps, c)
}

// getCmd groups the commands that read resources.
type getCmd struct {
	Tasks   getTasksCmd   `cmd:"" help:"List the tasks in the inferred repo."`
	Task    getTaskCmd    `cmd:"" help:"Print one task as YAML."`
	Tests   getTestsCmd   `cmd:"" help:"List a task's tests as YAML."`
	Test    getTestCmd    `cmd:"" help:"Print one test as YAML."`
	Data    getDataCmd    `cmd:"" help:"List a test's data refs or print one datum as JSON."`
	Models  getModelsCmd  `cmd:"" help:"List the models in the CLI config as YAML."`
	Model   getModelCmd   `cmd:"" help:"Print one model as YAML."`
	Agent   getAgentCmd   `cmd:"" help:"Print a task's agent as YAML."`
	Results getResultsCmd `cmd:"" help:"List the inferred repo's results as YAML."`
	Result  getResultCmd  `cmd:"" help:"Print one result as JSON."`
}

// getResultsCmd lists the inferred repo's results.
type getResultsCmd struct{}

// Run lists the inferred repo's results as YAML.
func (c getResultsCmd) Run(deps deps) error {
	return getResults(deps)
}

// getResultCmd prints one result as JSON.
type getResultCmd struct {
	Ref string `arg:"" help:"Result ref."`
}

// Run prints the result as JSON.
func (c getResultCmd) Run(deps deps) error {
	return getResult(deps, c.Ref)
}

// getAgentCmd prints one task's agent as YAML.
type getAgentCmd struct {
	Stub string `arg:"" help:"Task stub."`
}

// Run prints the task's agent as YAML.
func (c getAgentCmd) Run(deps deps) error {
	return getAgent(deps, c.Stub)
}

// getModelsCmd lists the models in the CLI config.
type getModelsCmd struct{}

// Run lists the models in the CLI config.
func (c getModelsCmd) Run(deps deps) error {
	return getModels(deps)
}

// getModelCmd prints one model as YAML.
type getModelCmd struct {
	Stub string `arg:"" help:"Model stub."`
}

// Run prints the model as YAML.
func (c getModelCmd) Run(deps deps) error {
	return getModel(deps, c.Stub)
}

// getTasksCmd lists the tasks in the inferred repo.
type getTasksCmd struct{}

// Run lists the tasks in the inferred repo.
func (c getTasksCmd) Run(deps deps) error {
	return getTasks(deps)
}

// getTaskCmd prints one task as YAML.
type getTaskCmd struct {
	Stub string `arg:"" help:"Task stub."`
}

// Run prints the task as YAML.
func (c getTaskCmd) Run(deps deps) error {
	return getTask(deps, c.Stub)
}

// getTestsCmd lists a task's tests.
type getTestsCmd struct {
	Task string `arg:"" help:"Task stub."`
}

// Run lists the task's tests.
func (c getTestsCmd) Run(deps deps) error {
	return getTests(deps, c.Task)
}

// getTestCmd prints one test as YAML.
type getTestCmd struct {
	Path string `arg:"" help:"Task and test stub as <task>/<stub>."`
}

// Run prints the test as YAML.
func (c getTestCmd) Run(deps deps) error {
	return getTest(deps, c.Path)
}

// getDataCmd lists a test's data or prints one datum as JSON.
type getDataCmd struct {
	Path string `arg:"" help:"Task, test, and optional ref as <task>/<test>[/<ref>]."`
}

// Run lists the test's data refs or prints one datum as JSON.
func (c getDataCmd) Run(deps deps) error {
	return getData(deps, c.Path)
}

// setCmd groups the commands that create or update resources.
type setCmd struct {
	Task   setTaskCmd   `cmd:"" help:"Create or update a task in the inferred repo."`
	Test   setTestCmd   `cmd:"" help:"Create or update a test in the inferred repo."`
	Model  setModelCmd  `cmd:"" help:"Create or update a model in the CLI config."`
	Agent  setAgentCmd  `cmd:"" help:"Create or update a task's agent in the CLI config."`
	Grader setGraderCmd `cmd:"" help:"Create or update a task's grader in the CLI config."`
}

// setAgentCmd creates or updates a task's agent in the CLI config.
type setAgentCmd struct {
	Stub       string `arg:"" help:"Task stub."`
	VersionCmd string `help:"Agent version command." optional:""`
	BuildCmd   string `help:"Agent build command." optional:""`
	RunCmd     string `help:"Agent run command." optional:""`
}

// Run creates or updates the task's agent in the CLI config.
func (c setAgentCmd) Run(deps deps) error {
	return setAgent(deps, c)
}

// setGraderCmd creates or updates a task's grader in the CLI config.
type setGraderCmd struct {
	Stub       string `arg:"" help:"Task stub."`
	VersionCmd string `help:"Grader version command." optional:""`
	BuildCmd   string `help:"Grader build command." optional:""`
	RunCmd     string `help:"Grader run command." optional:""`
}

// Run creates or updates the task's grader in the CLI config.
func (c setGraderCmd) Run(deps deps) error {
	return setGrader(deps, c)
}

// setModelCmd creates or updates a model in the CLI config.
type setModelCmd struct {
	Path    string `arg:"" help:"Model provider and name as <provider>/<name>."`
	Stub    string `help:"Override the auto generated stub." optional:""`
	Variant string `help:"Model variant." optional:""`
}

// Run creates or updates the model in the CLI config.
func (c setModelCmd) Run(deps deps) error {
	return setModel(deps, c)
}

// setTestCmd creates, updates, or renames a test under a task in the inferred
// repo.
type setTestCmd struct {
	Path        string `arg:"" help:"Task and test stub as <task>/<stub>."`
	Name        string `help:"Test name." optional:""`
	Description string `help:"Test description." optional:""`
	Version     string `help:"Test version." optional:""`
	Move        string `help:"Rename the test to this stub." optional:""`
}

// Run creates or updates the test in the inferred repo.
func (c setTestCmd) Run(deps deps) error {
	return setTest(deps, c)
}

// setTaskCmd creates, updates, or renames a task in the inferred repo.
type setTaskCmd struct {
	Stub        string `arg:"" help:"Task stub."`
	Name        string `help:"Task name." optional:""`
	Description string `help:"Task description." optional:""`
	Move        string `help:"Rename the task to this stub." optional:""`
	Model       string `help:"Set the task's default model in config." optional:""`
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
	errOut      io.Writer
	ctx         context.Context
	openBrowser func(string) error
}

func main() {
	env, err := processDeps(os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "abnch:", err)
		os.Exit(1)
	}
	runMain(os.Args[1:], env, os.Exit)
}

// processDeps builds the dependencies from the process's directories and the
// given output streams.
func processDeps(out, errOut io.Writer) (deps, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return deps{}, err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return deps{}, err
	}
	return deps{
		homeDir:     home,
		workDir:     workDir,
		in:          os.Stdin,
		out:         out,
		errOut:      errOut,
		ctx:         context.Background(),
		openBrowser: openBrowser,
	}, nil
}

// runMain runs one abnch invocation. A command that fails writes its error to
// the error stream and reports a non-zero exit code; a command that succeeds
// reports a zero exit code.
func runMain(args []string, env deps, exit func(int)) {
	if err := runCLI(args, env, exit); err != nil {
		fmt.Fprintln(env.errorWriter(), "abnch:", err)
		exit(1)
		return
	}
	exit(0)
}

// errorWriter returns the stream command errors are written to.
func (d deps) errorWriter() io.Writer {
	if d.errOut != nil {
		return d.errOut
	}
	return os.Stderr
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
