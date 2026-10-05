package main

import (
	"bytes"
	"errors"
	"testing"
)

func TestReportError_PrintsWithoutTimestamp(t *testing.T) {
	var out bytes.Buffer

	reportError(&out, errors.New("invalid --color value"))

	if got, want := out.String(), "Error: invalid --color value\n"; got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
