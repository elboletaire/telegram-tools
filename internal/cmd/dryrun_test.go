package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintDryRun_ShowsEachMessageRenderedWithoutSending(t *testing.T) {
	var out bytes.Buffer

	err := printDryRun(&out, "@chan", []string{"**Hi** [there](https://x.test)", "second"}, "MarkdownV2", false)
	if err != nil {
		t.Fatalf("printDryRun failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Message 1/2 · 8/4096 chars · to @chan",
		"Hi there <https://x.test>\n",
		"Message 2/2 · 6/4096 chars · to @chan",
		"second\n",
		"Dry run: 2 messages would be sent to @chan. Nothing was sent.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "**") {
		t.Errorf("markdown should be rendered, got raw syntax:\n%s", got)
	}
}

func TestPrintDryRun_SplitsMessagesOverTheTelegramLimit(t *testing.T) {
	var out bytes.Buffer
	long := strings.Repeat("a", 3000) + "\n\n" + strings.Repeat("b", 2000)

	if err := printDryRun(&out, "@chan", []string{long}, "MarkdownV2", false); err != nil {
		t.Fatalf("printDryRun failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Message 1/2 · 3000/4096 chars · to @chan",
		"Message 2/2 · 2000/4096 chars · to @chan",
		"Dry run: 2 messages would be sent to @chan. Nothing was sent.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintDryRunEdit_RejectsMessagesThatWouldNeedSplitting(t *testing.T) {
	var out bytes.Buffer
	long := strings.Repeat("a", 3000) + "\n\n" + strings.Repeat("b", 2000)

	err := printDryRunEdit(&out, "@chan", 42, long, "MarkdownV2", false)

	if err == nil {
		t.Fatalf("expected an error for an edit over the limit, got output:\n%s", out.String())
	}
}

func TestPrintDryRunEdit_ShowsTheEditTarget(t *testing.T) {
	var out bytes.Buffer

	if err := printDryRunEdit(&out, "@chan", 42, "new text", "MarkdownV2", false); err != nil {
		t.Fatalf("printDryRunEdit failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Edit of post #42 · 8/4096 chars · in @chan",
		"new text\n",
		"Dry run: post #42 in @chan would be edited. Nothing was sent.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
}

func TestResolveColor(t *testing.T) {
	var buf bytes.Buffer // never a terminal

	tests := []struct {
		mode    string
		noColor string
		want    bool
		wantErr bool
	}{
		{mode: "auto", want: false},
		{mode: "", want: false},
		{mode: "always", want: true},
		{mode: "always", noColor: "1", want: true},
		{mode: "never", want: false},
		{mode: "sometimes", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.mode+"/"+tt.noColor, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			got, err := resolveColor(tt.mode, &buf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
