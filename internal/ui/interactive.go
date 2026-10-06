// Package ui holds helpers shared by the terminal user interfaces.
package ui

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Interactive reports whether in and out are both terminals, so full-screen
// interfaces can run. Otherwise (pipes, cron, CI) callers should fall back to
// plain output.
func Interactive(in io.Reader, out io.Writer) bool {
	fin, ok := in.(*os.File)
	if !ok {
		return false
	}
	fout, ok := out.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(fin.Fd())) && term.IsTerminal(int(fout.Fd()))
}
