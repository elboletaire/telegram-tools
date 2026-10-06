package ui

import (
	"bytes"
	"os"
	"testing"
)

func TestInteractive_FalseWithoutTerminal(t *testing.T) {
	if Interactive(&bytes.Buffer{}, &bytes.Buffer{}) {
		t.Error("buffers are not a terminal")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if Interactive(devNull, devNull) {
		t.Error("/dev/null is not a terminal")
	}
}
