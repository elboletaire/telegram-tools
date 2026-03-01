package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveReplacementThumbnail_AutoDetectSibling(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	thumbPath := filepath.Join(tmpDir, "video-thumb.jpg")
	mustWriteFile(t, thumbPath)

	gotThumb, autoDetected, err := resolveReplacementThumbnail(filePath, false, "", "")
	if err != nil {
		t.Fatalf("resolveReplacementThumbnail() error = %v", err)
	}
	if gotThumb != thumbPath {
		t.Fatalf("thumb = %q, want %q", gotThumb, thumbPath)
	}
	if !autoDetected {
		t.Fatalf("autoDetected = false, want true")
	}
}

func TestResolveReplacementThumbnail_FallsBackToDefault(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	defaultThumb := filepath.Join(tmpDir, "default.jpg")
	mustWriteFile(t, defaultThumb)

	gotThumb, autoDetected, err := resolveReplacementThumbnail(filePath, false, "", defaultThumb)
	if err != nil {
		t.Fatalf("resolveReplacementThumbnail() error = %v", err)
	}
	if gotThumb != defaultThumb {
		t.Fatalf("thumb = %q, want %q", gotThumb, defaultThumb)
	}
	if autoDetected {
		t.Fatalf("autoDetected = true, want false")
	}
}

func TestResolveReplacementThumbnail_PrefersSiblingOverDefault(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	siblingThumb := filepath.Join(tmpDir, "video-thumb.jpeg")
	defaultThumb := filepath.Join(tmpDir, "default.jpg")
	mustWriteFile(t, siblingThumb)
	mustWriteFile(t, defaultThumb)

	gotThumb, autoDetected, err := resolveReplacementThumbnail(filePath, false, "", defaultThumb)
	if err != nil {
		t.Fatalf("resolveReplacementThumbnail() error = %v", err)
	}
	if gotThumb != siblingThumb {
		t.Fatalf("thumb = %q, want %q", gotThumb, siblingThumb)
	}
	if !autoDetected {
		t.Fatalf("autoDetected = false, want true")
	}
}

func TestResolveReplacementThumbnail_UsesProvidedThumb(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	providedThumb := filepath.Join(tmpDir, "provided.jpg")
	defaultThumb := filepath.Join(tmpDir, "default.jpg")
	siblingThumb := filepath.Join(tmpDir, "video-thumb.jpg")
	mustWriteFile(t, siblingThumb)
	mustWriteFile(t, providedThumb)
	mustWriteFile(t, defaultThumb)

	gotThumb, autoDetected, err := resolveReplacementThumbnail(filePath, true, providedThumb, defaultThumb)
	if err != nil {
		t.Fatalf("resolveReplacementThumbnail() error = %v", err)
	}
	if gotThumb != providedThumb {
		t.Fatalf("thumb = %q, want %q", gotThumb, providedThumb)
	}
	if autoDetected {
		t.Fatalf("autoDetected = true, want false")
	}
}

func TestResolveReplacementThumbnail_EmptyProvidedUsesDefault(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	defaultThumb := filepath.Join(tmpDir, "default.jpg")
	siblingThumb := filepath.Join(tmpDir, "video-thumb.jpg")
	mustWriteFile(t, siblingThumb)
	mustWriteFile(t, defaultThumb)

	gotThumb, autoDetected, err := resolveReplacementThumbnail(filePath, true, "", defaultThumb)
	if err != nil {
		t.Fatalf("resolveReplacementThumbnail() error = %v", err)
	}
	if gotThumb != defaultThumb {
		t.Fatalf("thumb = %q, want %q", gotThumb, defaultThumb)
	}
	if autoDetected {
		t.Fatalf("autoDetected = true, want false")
	}
}

func TestResolveReplacementThumbnail_InvalidProvidedPath(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	_, _, err := resolveReplacementThumbnail(filePath, true, "~invalid-user/path.jpg", "")
	if err == nil {
		t.Fatal("resolveReplacementThumbnail() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "expand provided thumb") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "expand provided thumb")
	}
}

func TestResolveReplacementThumbnail_InvalidDefaultPath(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "video.mp4")
	_, _, err := resolveReplacementThumbnail(filePath, false, "", "~invalid-user/path.jpg")
	if err == nil {
		t.Fatal("resolveReplacementThumbnail() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "expand default thumb") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "expand default thumb")
	}
}

func TestResolveReplacementThumbnail_SiblingStatError(t *testing.T) {
	tmpDir := t.TempDir()
	lockedDir := filepath.Join(tmpDir, "locked")
	if err := os.Mkdir(lockedDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(lockedDir, 0); err != nil {
		t.Fatalf("chmod lock: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(lockedDir, 0o700)
	})

	filePath := filepath.Join(lockedDir, "video.mp4")
	_, _, err := resolveReplacementThumbnail(filePath, false, "", "")
	if err == nil {
		t.Fatal("resolveReplacementThumbnail() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "checking thumbnail") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "checking thumbnail")
	}
}

func TestPrintDetectedThumbnail(t *testing.T) {
	var buf bytes.Buffer
	printDetectedThumbnail(&buf, "/tmp/video-thumb.jpg", true)
	if !strings.Contains(buf.String(), "Using detected thumbnail video-thumb.jpg") {
		t.Fatalf("output = %q, want detected thumbnail message", buf.String())
	}

	buf.Reset()
	printDetectedThumbnail(&buf, "/tmp/video-thumb.jpg", false)
	if buf.Len() != 0 {
		t.Fatalf("output = %q, want empty output", buf.String())
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}
