package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
)

// exitCode runs one command with recording streams and returns the exit code,
// the stdout it wrote, and the stderr it wrote.
func exitCode(t *testing.T, args []string, d deps) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	d.out = &out
	d.errOut = &errOut
	code := -1
	runMain(args, d, func(c int) { code = c })
	return code, out.String(), errOut.String()
}

// commandDeps returns deps for a command run in an isolated home and work dir.
func commandDeps(t *testing.T, workDir string) deps {
	t.Helper()
	return deps{
		homeDir:     t.TempDir(),
		workDir:     workDir,
		in:          strings.NewReader(""),
		ctx:         context.Background(),
		openBrowser: unusedBrowser(t),
	}
}

func TestRunMainExitsZeroAndWritesOutputToStdout(t *testing.T) {
	d := commandDeps(t, t.TempDir())

	code, out, errOut := exitCode(t, []string{"logout", "--url", "https://bench.example.com"}, d)

	if code != 0 {
		t.Errorf("exit = %d, want 0 on success", code)
	}
	if !strings.Contains(out, "Signed out") {
		t.Errorf("stdout = %q, want the command output", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want nothing on success", errOut)
	}
}

func TestRunMainExitsNonZeroAndWritesErrorToStderr(t *testing.T) {
	d := commandDeps(t, t.TempDir())

	code, out, errOut := exitCode(t, []string{"login"}, d)

	if code == 0 {
		t.Error("exit = 0, want a non-zero exit on failure")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing on failure", out)
	}
	if !strings.Contains(errOut, "url") {
		t.Errorf("stderr = %q, want the error", errOut)
	}
}

func TestRunMainWritesParseFailureToStderr(t *testing.T) {
	d := commandDeps(t, t.TempDir())

	code, out, errOut := exitCode(t, []string{"bogus"}, d)

	if code == 0 {
		t.Error("exit = 0, want a non-zero exit for a parse failure")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing for a parse failure", out)
	}
	if !strings.Contains(errOut, "bogus") {
		t.Errorf("stderr = %q, want the parse error", errOut)
	}
}

func TestRunMainSurfacesServiceErrorOnStderr(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	service := forbiddenService(t)
	d := commandDeps(t, workDir)
	if err := devconfig.StoreSession(d.homeDir, service, "session-token"); err != nil {
		t.Fatal(err)
	}
	writeCLIConfig(t, workDir, "url: "+service+"\n")

	code, out, errOut := exitCode(t, []string{"set", "task", "fix-bug", "--name", "Fix bug", "--description", "Fix the bug"}, d)

	if code == 0 {
		t.Error("exit = 0, want a non-zero exit when the service rejects the write")
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing when the service rejects the write", out)
	}
	if !strings.Contains(errOut, "forbidden") {
		t.Errorf("stderr = %q, want the service error message", errOut)
	}
}
