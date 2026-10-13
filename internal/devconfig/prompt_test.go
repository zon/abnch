package devconfig

import (
	"bytes"
	"strings"
	"testing"
)

func TestAskUsesTypedValue(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("typed\n"), &out)

	got, err := p.Ask("host", "db.example", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "typed" {
		t.Errorf("value = %q, want typed", got)
	}
}

func TestAskOffersStoredDefault(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("\n"), &out)

	got, err := p.Ask("host", "db.example", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "db.example" {
		t.Errorf("value = %q, want the stored default", got)
	}
	if !strings.Contains(out.String(), "db.example") {
		t.Errorf("prompt does not offer the stored default: %q", out.String())
	}
}

func TestAskHidesStoredSecret(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("\n"), &out)

	got, err := p.Ask("password", "s3cret", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "s3cret" {
		t.Errorf("value = %q, want the stored secret", got)
	}
	if strings.Contains(out.String(), "s3cret") {
		t.Errorf("prompt reveals the stored secret: %q", out.String())
	}
}

func TestAskHidesTypingOfSecret(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("s3cret\n"), &out)
	var off, on int
	p.echoOff = func() { off++ }
	p.echoOn = func() { on++ }

	if _, err := p.Ask("password", "", true); err != nil {
		t.Fatal(err)
	}
	if off != 1 {
		t.Errorf("echo off called %d times, want 1", off)
	}
	if on != off {
		t.Errorf("echo off/on = %d/%d, want them balanced", off, on)
	}
}

func TestAskKeepsEchoOnForPlainValue(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("db.example\n"), &out)
	called := 0
	p.echoOff = func() { called++ }
	p.echoOn = func() { called++ }

	if _, err := p.Ask("host", "", false); err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Errorf("echo control called %d times for a plain prompt", called)
	}
}
