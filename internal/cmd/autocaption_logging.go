package cmd

import (
	"fmt"
	"io"
	"strings"
)

func logAutoCaption(out io.Writer, usedAutoCaption bool, caption string) {
	if out == nil || !usedAutoCaption {
		return
	}
	preview := strings.TrimSpace(caption)
	if preview == "" {
		return
	}
	fmt.Fprintf(out, "🤖 Using auto-caption %s\n", preview)
}
