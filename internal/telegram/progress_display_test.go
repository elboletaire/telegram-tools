package telegram

import (
	"bytes"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUploadProgressModel_UpdateWindowSizeFillsTerminalWidth(t *testing.T) {
	model := newUploadProgressModel("Uploading", "video.mp4", "", "", false, 30)
	if model.progress.Width != 30 {
		t.Fatalf("initial width = %d, want 30", model.progress.Width)
	}

	_, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if model.progress.Width != 120 {
		t.Fatalf("width after resize = %d, want 120", model.progress.Width)
	}
}

func TestTerminalWidthFromWriter_NonTerminalWriter(t *testing.T) {
	w, ok := terminalWidthFromWriter(&bytes.Buffer{})
	if ok {
		t.Fatalf("ok = true, want false (width=%d)", w)
	}
	if w != 0 {
		t.Fatalf("width = %d, want 0", w)
	}
}
