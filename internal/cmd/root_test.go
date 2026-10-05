package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_PrintsVersion(t *testing.T) {
	root := newRootCommand("1.2.3")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "1.2.3") {
		t.Errorf("expected version in output, got %q", out.String())
	}
}
