package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestTTYPrompterReadsLines(t *testing.T) {
	var out bytes.Buffer
	p := &ttyPrompter{in: bufio.NewReader(strings.NewReader(" 12345 \nlast")), out: &out}
	if v, err := p.line("id: "); err != nil || v != "12345" {
		t.Fatalf("%q %v", v, err)
	}
	if v, err := p.line("x: "); err != nil || v != "last" {
		t.Fatalf("last line without newline: %q %v", v, err)
	}
	if _, err := p.line("x: "); err == nil {
		t.Fatal("EOF must be an error")
	}
	if strings.Contains(out.String(), "12345") {
		t.Fatal("answers must never be echoed back")
	}
}

func TestLoginRefusesNonTerminal(t *testing.T) {
	// go test runs with stdin not attached to a terminal.
	err := runLogin(options{dataDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "interactive") {
		t.Fatalf("got %v", err)
	}
}
