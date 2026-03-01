package telegram

import (
	"bytes"
	"testing"
	"time"

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

func TestEstimateRemainingDuration(t *testing.T) {
	remaining, ok := estimateRemainingDuration(40*time.Second, 0.5)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if remaining != 40*time.Second {
		t.Fatalf("remaining = %v, want %v", remaining, 40*time.Second)
	}
}

func TestBuildETALabel(t *testing.T) {
	label := buildETALabel(83*time.Second, true)
	if label != "ETA 01:23 " {
		t.Fatalf("label = %q, want %q", label, "ETA 01:23 ")
	}

	unknown := buildETALabel(0, false)
	if unknown != "ETA --:-- " {
		t.Fatalf("label = %q, want %q", unknown, "ETA --:-- ")
	}
}

func TestProgressContentWidth(t *testing.T) {
	width := progressContentWidth(120, "ETA 01:23 ")
	if width != 110 {
		t.Fatalf("width = %d, want %d", width, 110)
	}

	tooSmall := progressContentWidth(8, "ETA 01:23 ")
	if tooSmall != 1 {
		t.Fatalf("width = %d, want %d", tooSmall, 1)
	}
}
