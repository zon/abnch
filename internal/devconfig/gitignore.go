package devconfig

import (
	"os"
	"strings"
)

// EnsureGitignore adds lines that are not already present, creating the file
// when it does not exist.
func EnsureGitignore(path string, lines ...string) error {
	var content string
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}

	present := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		present[strings.TrimSpace(line)] = true
	}

	var add []string
	for _, line := range lines {
		if !present[line] {
			add = append(add, line)
		}
	}
	if len(add) == 0 {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += strings.Join(add, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
