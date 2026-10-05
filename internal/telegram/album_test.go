package telegram

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/elboletaire/telegram-tools/internal/config"
)

func TestAlbumGroupForMIME(t *testing.T) {
	tests := map[string]albumGroup{
		"image/jpeg":      albumGroupVisual,
		"image/png":       albumGroupVisual,
		"video/mp4":       albumGroupVisual,
		"audio/mpeg":      albumGroupAudio,
		"application/pdf": albumGroupDocument,
		"text/plain":      albumGroupDocument,
	}
	for mimeType, want := range tests {
		if got := albumGroupForMIME(mimeType); got != want {
			t.Errorf("%s: expected %v, got %v", mimeType, want, got)
		}
	}
}

func TestSplitAlbum_BalancesChunksOfAtMostTen(t *testing.T) {
	tests := []struct {
		n    int
		want []int
	}{
		{1, []int{1}},
		{2, []int{2}},
		{10, []int{10}},
		{11, []int{6, 5}},
		{20, []int{10, 10}},
		{21, []int{7, 7, 7}},
	}
	for _, tt := range tests {
		items := make([]AlbumItem, tt.n)
		for i := range items {
			items[i] = AlbumItem{FilePath: string(rune('a' + i))}
		}

		chunks := splitAlbum(items)

		var sizes []int
		var order []AlbumItem
		for _, c := range chunks {
			sizes = append(sizes, len(c))
			order = append(order, c...)
		}
		if len(sizes) != len(tt.want) {
			t.Fatalf("%d items: expected chunk sizes %v, got %v", tt.n, tt.want, sizes)
		}
		for i := range sizes {
			if sizes[i] != tt.want[i] {
				t.Errorf("%d items: expected chunk sizes %v, got %v", tt.n, tt.want, sizes)
				break
			}
		}
		for i := range items {
			if order[i] != items[i] {
				t.Fatalf("%d items: order not preserved", tt.n)
			}
		}
	}
}

func TestCheckAlbumGroups_RejectsMixingDocumentsWithMedia(t *testing.T) {
	err := checkAlbumGroups([]string{"a.jpg", "b.mp4", "c.pdf"}, []albumGroup{albumGroupVisual, albumGroupVisual, albumGroupDocument})
	if err == nil || !strings.Contains(err.Error(), "c.pdf") {
		t.Fatalf("expected an error naming c.pdf, got %v", err)
	}

	if err := checkAlbumGroups([]string{"a.jpg", "b.mp4"}, []albumGroup{albumGroupVisual, albumGroupVisual}); err != nil {
		t.Errorf("photos and videos can share an album, got %v", err)
	}
	if err := checkAlbumGroups([]string{"a.pdf", "b.zip"}, []albumGroup{albumGroupDocument, albumGroupDocument}); err != nil {
		t.Errorf("documents can share an album, got %v", err)
	}
}

func TestMessageMediaToInput(t *testing.T) {
	photo := &tg.MessageMediaPhoto{Photo: &tg.Photo{ID: 1, AccessHash: 2, FileReference: []byte("ref")}}
	got, err := messageMediaToInput(photo)
	if err != nil {
		t.Fatalf("photo: %v", err)
	}
	p, ok := got.(*tg.InputMediaPhoto)
	if !ok {
		t.Fatalf("photo: expected *tg.InputMediaPhoto, got %T", got)
	}
	if id, ok := p.ID.(*tg.InputPhoto); !ok || id.ID != 1 || id.AccessHash != 2 || string(id.FileReference) != "ref" {
		t.Errorf("photo: unexpected input %#v", p.ID)
	}

	doc := &tg.MessageMediaDocument{Document: &tg.Document{ID: 3, AccessHash: 4, FileReference: []byte("ref")}}
	got, err = messageMediaToInput(doc)
	if err != nil {
		t.Fatalf("document: %v", err)
	}
	d, ok := got.(*tg.InputMediaDocument)
	if !ok {
		t.Fatalf("document: expected *tg.InputMediaDocument, got %T", got)
	}
	if id, ok := d.ID.(*tg.InputDocument); !ok || id.ID != 3 || id.AccessHash != 4 {
		t.Errorf("document: unexpected input %#v", d.ID)
	}

	if _, err := messageMediaToInput(&tg.MessageMediaEmpty{}); err == nil {
		t.Error("expected an error for unsupported media")
	}
}

func TestExtractMessageIDs_ReturnsAllAlbumMessagesInOrder(t *testing.T) {
	updates := &tg.Updates{Updates: []tg.UpdateClass{
		&tg.UpdateMessageID{ID: 12},
		&tg.UpdateNewChannelMessage{Message: &tg.Message{ID: 12}},
		&tg.UpdateNewChannelMessage{Message: &tg.Message{ID: 11}},
		&tg.UpdateNewChannelMessage{Message: &tg.Message{ID: 13}},
	}}

	got := extractMessageIDs(updates)

	want := []int{11, 12, 13}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadAlbum_ValidatesBeforeConnecting(t *testing.T) {
	dir := t.TempDir()
	png := []byte("\x89PNG\r\n\x1a\n0000000000000000")
	photo1 := writeFile(t, dir, "a.png", png)
	photo2 := writeFile(t, dir, "b.png", png)
	pdf := writeFile(t, dir, "c.pdf", []byte("%PDF-1.4\n"))

	svc := NewService(&config.Config{
		Session: config.SessionConfig{File: filepath.Join(dir, "session.json")},
	}, WithIO(nil, &bytes.Buffer{}, &bytes.Buffer{}))

	tests := []struct {
		name    string
		req     UploadAlbumRequest
		wantErr string
	}{
		{name: "no chat", req: UploadAlbumRequest{Items: []AlbumItem{{FilePath: photo1}}}, wantErr: "channel is required"},
		{name: "no items", req: UploadAlbumRequest{ChatId: "@x"}, wantErr: "at least one file"},
		{name: "mixed kinds", req: UploadAlbumRequest{ChatId: "@x", Items: []AlbumItem{{FilePath: photo1}, {FilePath: pdf}}}, wantErr: "c.pdf"},
		{name: "caption too long", req: UploadAlbumRequest{ChatId: "@x", Items: []AlbumItem{
			{FilePath: photo1},
			{FilePath: photo2, Caption: strings.Repeat("a", MaxPremiumCaptionLength+1)},
		}}, wantErr: "caption too long"},
		{name: "missing file", req: UploadAlbumRequest{ChatId: "@x", Items: []AlbumItem{{FilePath: photo1}, {FilePath: filepath.Join(dir, "nope.png")}}}, wantErr: "nope.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UploadAlbum(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
