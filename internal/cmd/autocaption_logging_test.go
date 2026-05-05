package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogAutoCaption_UsedWithCaption(t *testing.T) {
	var out bytes.Buffer
	logAutoCaption(&out, true, "Episode 01")
	if !strings.Contains(out.String(), "🤖 Using auto-caption Episode 01") {
		t.Fatalf("output = %q, want auto-caption log line", out.String())
	}
}

func TestLogAutoCaption_NotUsed(t *testing.T) {
	var out bytes.Buffer
	logAutoCaption(&out, false, "Episode 01")
	if out.Len() != 0 {
		t.Fatalf("output = %q, want empty output", out.String())
	}
}

func TestLogAutoCaption_EmptyCaption(t *testing.T) {
	var out bytes.Buffer
	logAutoCaption(&out, true, "   ")
	if out.Len() != 0 {
		t.Fatalf("output = %q, want empty output", out.String())
	}
}
