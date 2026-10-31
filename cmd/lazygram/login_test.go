package main

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTTYPrompterReadsLines(t *testing.T) {
	var out bytes.Buffer
	p := &ttyPrompter{in: bufio.NewReader(strings.NewReader(" +1 555 010 0000 \n12345")), out: &out}
	ctx := context.Background()
	if v, err := p.Phone(ctx); err != nil || v != "+1 555 010 0000" {
		t.Fatalf("%q %v", v, err)
	}
	if v, err := p.Code(ctx); err != nil || v != "12345" {
		t.Fatalf("last line without newline: %q %v", v, err)
	}
	if _, err := p.Code(ctx); err == nil {
		t.Fatal("EOF must be an error")
	}
	if strings.Contains(out.String(), "5550100000") || strings.Contains(out.String(), "12345") {
		t.Fatal("answers must never be echoed back")
	}
}

func TestLoginRefusesNonTerminal(t *testing.T) {
	// go test runs with stdin not attached to a terminal.
	if err := runLogin(); err == nil || !strings.Contains(err.Error(), "interactive") {
		t.Fatalf("got %v", err)
	}
}
