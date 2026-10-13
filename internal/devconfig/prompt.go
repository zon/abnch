package devconfig

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Prompter reads the answers for the config questions.
type Prompter struct {
	reader  *bufio.Reader
	out     io.Writer
	echoOff func()
	echoOn  func()
}

// NewPrompter builds a Prompter that reads answers from in and writes prompts
// to out.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{reader: bufio.NewReader(in), out: out, echoOff: disableEcho, echoOn: enableEcho}
}

// Ask reads one value and returns def when the answer is empty. A secret answer
// is not echoed.
func (p *Prompter) Ask(label, def string, secret bool) (string, error) {
	fmt.Fprint(p.out, promptText(label, def, secret))
	if secret {
		p.echoOff()
	}

	line, err := p.reader.ReadString('\n')

	if secret {
		p.echoOn()
		fmt.Fprintln(p.out)
	}
	if err != nil && line == "" {
		return "", err
	}

	value := strings.TrimSpace(line)
	if value == "" {
		return def, nil
	}
	return value, nil
}

func promptText(label, def string, secret bool) string {
	if secret {
		if def != "" {
			return fmt.Sprintf("%s [stored]: ", label)
		}
		return fmt.Sprintf("%s: ", label)
	}
	if def != "" {
		return fmt.Sprintf("%s [%s]: ", label, def)
	}
	return fmt.Sprintf("%s: ", label)
}

func disableEcho() {
	exec.Command("stty", "-echo").Run()
}

func enableEcho() {
	exec.Command("stty", "echo").Run()
}
