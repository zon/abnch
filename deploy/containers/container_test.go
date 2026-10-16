package containers

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// stage is one FROM stage of the Containerfile and the instructions after it.
type stage struct {
	name  string
	image string
	instr []string
}

// TestContainerfileIsMultiStage checks that the Containerfile builds the app in
// a build stage, then copies it into a runtime stage whose base carries no
// build tools.
func TestContainerfileIsMultiStage(t *testing.T) {
	stages := stagesUnderTest(t)
	if len(stages) != 2 {
		t.Fatalf("Containerfile has %d FROM stages, want one build stage and one runtime stage", len(stages))
	}

	build, run := stages[0], stages[1]
	if build.name == "" || run.name == "" {
		t.Fatalf("stages %q and %q are not both named, so the runtime cannot copy from the build", build.name, run.name)
	}
	if build.name == run.name {
		t.Fatalf("stage names %q collide", build.name)
	}
	if !isGoImage(build.image) {
		t.Errorf("build stage base %q is not a Go image", build.image)
	}
	if isGoImage(run.image) {
		t.Errorf("runtime stage base %q is a Go image, which carries build tools", run.image)
	}

	copied := copyFrom(run, build.name)
	if !copied.found {
		t.Errorf("runtime stage does not copy the app from the build stage %q:\n%s", build.name, run.joined())
	}
}

// TestBuildStageCompilesAbnchd checks that the build stage compiles the abnchd
// command from the module sources.
func TestBuildStageCompilesAbnchd(t *testing.T) {
	build := stagesUnderTest(t)[0]
	for _, want := range []string{"go build", "./cmd/abnchd"} {
		if !build.has(want) {
			t.Errorf("build stage does not run %q:\n%s", want, build.joined())
		}
	}
}

// TestRuntimeStageHoldsBinaryAndRuntimeDeps checks that the runtime stage holds
// the compiled binary and what the service needs to reach HTTPS endpoints, and
// none of the build tools.
func TestRuntimeStageHoldsBinaryAndRuntimeDeps(t *testing.T) {
	stages := stagesUnderTest(t)
	build, run := stages[0], stages[1]

	copied := copyFrom(run, build.name)
	if !copied.found {
		t.Fatalf("runtime stage does not copy the app from the build stage %q:\n%s", build.name, run.joined())
	}
	if !strings.Contains(copied.src, "abnchd") {
		t.Errorf("runtime stage copies %q, want the abnchd binary", copied.src)
	}
	if !run.has("ca-certificates") {
		t.Errorf("runtime stage installs no CA certificates for HTTPS calls:\n%s", run.joined())
	}
	for _, tool := range []string{"go build", "go mod", "go install"} {
		if run.has(tool) {
			t.Errorf("runtime stage runs %q, so build tools leaked into the image:\n%s", tool, run.joined())
		}
	}
}

// TestRuntimeStageRunsAbnchdFromCallerConfig checks that the runtime stage
// starts abnchd and passes it the config path the caller gives, so a deployment
// mounts a service config and names it on the command line.
func TestRuntimeStageRunsAbnchdFromCallerConfig(t *testing.T) {
	stages := stagesUnderTest(t)
	run := stages[1]

	entry, ok := run.instruction("ENTRYPOINT")
	if !ok {
		t.Fatalf("runtime stage sets no ENTRYPOINT, so the image does not run abnchd:\n%s", run.joined())
	}
	argv, err := execForm(entry)
	if err != nil {
		t.Fatalf("ENTRYPOINT %s is not exec form: %v", entry, err)
	}
	if len(argv) == 0 || filepath.Base(argv[0]) != "abnchd" {
		t.Errorf("ENTRYPOINT %s does not run the abnchd binary", entry)
	}
	if len(argv) != 1 {
		t.Errorf("ENTRYPOINT %s passes arguments, so the image fixes the config path instead of taking the caller's", entry)
	}

	copied := copyFrom(run, stages[0].name)
	if !copied.found {
		t.Fatalf("runtime stage does not copy the app from the build stage %q:\n%s", stages[0].name, run.joined())
	}
	if len(argv) > 0 && argv[0] != copied.dest {
		t.Errorf("ENTRYPOINT runs %q but the binary is copied to %q", argv[0], copied.dest)
	}
}

// TestBaseImagesArePinned checks that each base image names a version rather
// than a moving tag such as latest.
func TestBaseImagesArePinned(t *testing.T) {
	for _, s := range stagesUnderTest(t) {
		tag := imageTag(s.image)
		if tag == "" {
			t.Errorf("base image %q names no version", s.image)
			continue
		}
		if tag == "latest" {
			t.Errorf("base image %q uses the moving tag latest", s.image)
		}
		if !versionTag.MatchString(tag) {
			t.Errorf("base image %q tag %q does not name a version", s.image, tag)
		}
	}
}

// TestGoDependenciesFollowGoMod checks that the build stage takes dependency
// versions from go.mod and go.sum instead of pinning them in the Containerfile.
func TestGoDependenciesFollowGoMod(t *testing.T) {
	build := stagesUnderTest(t)[0].joined()
	for _, want := range []string{"go.mod", "go.sum", "go mod download"} {
		if !strings.Contains(build, want) {
			t.Errorf("build stage does not use %q:\n%s", want, build)
		}
	}
	if strings.Contains(build, "go get") {
		t.Errorf("build stage runs go get instead of the pinned modules:\n%s", build)
	}
	if moduleVersion.MatchString(build) {
		t.Errorf("build stage pins a module version outside go.mod:\n%s", build)
	}
}

// TestBuilderGoMatchesGoMod checks that the build image's Go version follows
// the version the module pins, so the module builds with the toolchain it
// declares.
func TestBuilderGoMatchesGoMod(t *testing.T) {
	got := goVersionOf(stagesUnderTest(t)[0])
	goDirective, toolchain := moduleGo(t)

	if toolchain != "" && got != toolchain {
		t.Errorf("build image Go %q, want the go.mod toolchain %q", got, toolchain)
	}
	if goDirective != "" && !strings.HasPrefix(got, goDirective) {
		t.Errorf("build image Go %q does not satisfy the go.mod go directive %q", got, goDirective)
	}
}

// TestImageRunsAbnchdFromConfig builds the image and starts the app it holds
// the way a deployment does. It skips when no container engine is installed, so
// the suite runs without one.
func TestImageRunsAbnchdFromConfig(t *testing.T) {
	engine, ok := containerEngine()
	if !ok {
		t.Skip("no podman or docker installed; skipping the image build")
	}

	root := repoRoot(t)
	tag := "abnchd:test"

	build := exec.Command(engine, "build", "-f", filepath.Join(root, "deploy", "containers", "Containerfile"), "-t", tag, root)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%s build: %v\n%s", engine, err, out)
	}
	t.Cleanup(func() { exec.Command(engine, "rmi", "-f", tag).Run() })

	help := exec.Command(engine, "run", "--rm", tag, "--help")
	out, err := help.CombinedOutput()
	if err != nil {
		t.Fatalf("%s run --help: %v\n%s", engine, err, out)
	}
	if !strings.Contains(string(out), "abnchd") {
		t.Errorf("image output does not name abnchd:\n%s", out)
	}

	if out, err := exec.Command(engine, "run", "--rm", tag).CombinedOutput(); err == nil {
		t.Errorf("image started with no config path and did not fail:\n%s", out)
	}

	// A caller-supplied path reaches abnchd, so a missing file is reported by
	// name.
	path := "/etc/abnchd/missing.yaml"
	out, err = exec.Command(engine, "run", "--rm", tag, path).CombinedOutput()
	if err == nil {
		t.Fatalf("image started with a missing config and did not fail:\n%s", out)
	}
	if !strings.Contains(string(out), path) {
		t.Errorf("image output does not name the config path %q it was given:\n%s", path, out)
	}
}

// copyLine is a COPY --from line's source and destination paths.
type copyLine struct {
	found bool
	src   string
	dest  string
}

// copyFrom returns the first COPY in s that copies from the named stage.
func copyFrom(s stage, name string) copyLine {
	for _, line := range s.instr {
		cmd, rest := splitCommand(line)
		if !strings.EqualFold(cmd, "COPY") {
			continue
		}
		from := ""
		var args []string
		for _, field := range strings.Fields(rest) {
			if strings.HasPrefix(field, "--from=") {
				from = strings.TrimPrefix(field, "--from=")
				continue
			}
			args = append(args, field)
		}
		if strings.EqualFold(from, name) && len(args) >= 2 {
			return copyLine{found: true, src: args[0], dest: args[len(args)-1]}
		}
	}
	return copyLine{}
}

// stagesUnderTest parses the Containerfile beside this test.
func stagesUnderTest(t *testing.T) []stage {
	t.Helper()
	return parseStages(readFile(t, filepath.Join(testDir(t), "Containerfile")))
}

// parseStages splits a Containerfile into its FROM stages.
func parseStages(content string) []stage {
	var stages []stage
	for _, line := range logicalLines(content) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmd, rest := splitCommand(line)
		if strings.EqualFold(cmd, "FROM") {
			stages = append(stages, parseFrom(rest))
			continue
		}
		if len(stages) > 0 {
			stages[len(stages)-1].instr = append(stages[len(stages)-1].instr, line)
		}
	}
	return stages
}

// parseFrom reads a FROM instruction's image and optional stage name.
func parseFrom(rest string) stage {
	fields := strings.Fields(rest)
	s := stage{image: fields[0]}
	for i := 1; i+1 < len(fields); i++ {
		if strings.EqualFold(fields[i], "AS") {
			s.name = fields[i+1]
		}
	}
	return s
}

// logicalLines joins Containerfile line continuations into single lines.
func logicalLines(text string) []string {
	var out []string
	var cur strings.Builder
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(strings.TrimRight(raw, "\r"), " \t")
		if strings.HasSuffix(line, "\\") {
			cur.WriteString(strings.TrimSuffix(line, "\\"))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(line)
		out = append(out, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// splitCommand splits an instruction into its command and arguments.
func splitCommand(line string) (string, string) {
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return line, ""
	}
	return line[:i], strings.TrimSpace(line[i+1:])
}

// has reports whether any instruction in s contains substr.
func (s stage) has(substr string) bool {
	return strings.Contains(s.joined(), substr)
}

// instruction returns the arguments of the last instruction in s whose command
// matches cmd, such as ENTRYPOINT or CMD.
func (s stage) instruction(cmd string) (string, bool) {
	for i := len(s.instr) - 1; i >= 0; i-- {
		c, rest := splitCommand(s.instr[i])
		if strings.EqualFold(c, cmd) {
			return rest, true
		}
	}
	return "", false
}

// execForm parses an instruction's exec-form JSON array of arguments.
func execForm(rest string) ([]string, error) {
	var argv []string
	if err := json.Unmarshal([]byte(rest), &argv); err != nil {
		return nil, err
	}
	return argv, nil
}

// joined returns the stage instructions as one block of text.
func (s stage) joined() string {
	return strings.Join(s.instr, "\n")
}

// isGoImage reports whether an image is a Go toolchain image.
func isGoImage(image string) bool {
	name := image
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.HasPrefix(name, "golang")
}

// goVersionOf reads the Go version from a build stage's image tag, such as
// 1.27.2 from golang:1.27.2-alpine3.24.
func goVersionOf(s stage) string {
	return strings.SplitN(imageTag(s.image), "-", 2)[0]
}

// imageTag returns the tag of a base image, or "" when it names none.
func imageTag(image string) string {
	name := image
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	i := strings.LastIndex(name, ":")
	if i < 0 {
		return ""
	}
	return name[i+1:]
}

// moduleGo reads the go directive and toolchain version from go.mod.
func moduleGo(t *testing.T) (directive, toolchain string) {
	t.Helper()
	for _, line := range strings.Split(readFile(t, filepath.Join(repoRoot(t), "go.mod")), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "go":
			directive = fields[1]
		case "toolchain":
			toolchain = strings.TrimPrefix(fields[1], "go")
		}
	}
	return directive, toolchain
}

// containerEngine returns the path of the installed container engine and
// whether one was found.
func containerEngine() (string, bool) {
	for _, engine := range []string{"podman", "docker"} {
		if path, err := exec.LookPath(engine); err == nil {
			return path, true
		}
	}
	return "", false
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// testDir is the directory holding this test and the Containerfile.
func testDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Dir(file)
}

// repoRoot is the module root above the test directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join(testDir(t), "..", ".."))
}

// versionTag matches a tag that names a version, such as 3.24.2.
var versionTag = regexp.MustCompile(`^[0-9]+(\.[0-9]+)+`)

// moduleVersion matches a module pinned to a version, such as example.com/m@v1.2.3.
var moduleVersion = regexp.MustCompile(`\S+@v[0-9]`)
