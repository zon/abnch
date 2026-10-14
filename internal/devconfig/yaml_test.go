package devconfig

import (
	"strings"
	"testing"
)

func TestMarshalYAMLIndentsWithTwoSpaces(t *testing.T) {
	data, err := marshalYAML(Config{Postgres: Postgres{Host: "db"}})
	if err != nil {
		t.Fatal(err)
	}

	got := string(data)
	if !strings.Contains(got, "\n  host: db\n") {
		t.Errorf("yaml is not indented with two spaces:\n%s", got)
	}
	if strings.Contains(got, "\n    host:") {
		t.Errorf("yaml still uses four-space indents:\n%s", got)
	}
}
