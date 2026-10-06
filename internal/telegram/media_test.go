package telegram

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/gotd/td/tg"
)

func TestUploadPartSize(t *testing.T) {
	const mb = 1024 * 1024

	tests := []struct {
		name string
		size int64
		want int
	}{
		{name: "small file keeps default part size", size: 5 * mb, want: 128 * 1024},
		{name: "just below the 128KB part limit", size: 500 * mb, want: 128 * 1024},
		{name: "above the 128KB part limit", size: 600 * mb, want: 256 * 1024},
		{name: "above the 256KB part limit", size: 1100 * mb, want: 524288},
		{name: "huge file is capped at the maximum part size", size: 8000 * mb, want: 524288},
		{name: "empty file keeps default part size", size: 0, want: 128 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uploadPartSize(tt.size)
			if got != tt.want {
				t.Fatalf("uploadPartSize(%d) = %d, want %d", tt.size, got, tt.want)
			}
			if maxUploadPartSize%got != 0 || got%1024 != 0 {
				t.Fatalf("uploadPartSize(%d) = %d is not a valid Telegram part size", tt.size, got)
			}
			if parts := uploadParts(tt.size, got); parts > maxUploadParts && got != maxUploadPartSize {
				t.Fatalf("uploadPartSize(%d) yields %d parts, above the %d limit", tt.size, parts, maxUploadParts)
			}
		})
	}
}

func TestUploadParts(t *testing.T) {
	tests := []struct {
		size     int64
		partSize int
		want     int64
	}{
		{size: 0, partSize: 1024, want: 0},
		{size: -1, partSize: 1024, want: 0},
		{size: 1024, partSize: 0, want: 0},
		{size: 1024, partSize: 1024, want: 1},
		{size: 1025, partSize: 1024, want: 2},
	}

	for _, tt := range tests {
		if got := uploadParts(tt.size, tt.partSize); got != tt.want {
			t.Errorf("uploadParts(%d, %d) = %d, want %d", tt.size, tt.partSize, got, tt.want)
		}
	}
}

func writeTestImage(t *testing.T, name string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 16), G: uint8(y * 16), B: 128, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if filepath.Ext(name) == ".png" {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// ffprobe reports still images as an mjpeg/png "video stream"; they must still
// be sent as photos, not as documents.
func TestAnalyzeMedia_StillImagesArePhotos(t *testing.T) {
	for _, name := range []string{"photo.jpg", "photo.png"} {
		t.Run(name, func(t *testing.T) {
			meta, _ := analyzeMedia(context.Background(), writeTestImage(t, name))
			if meta.Kind != mediaKindPhoto {
				t.Errorf("expected a photo, got kind %d (mime %s)", meta.Kind, meta.MIME)
			}
		})
	}
}

func TestBuildInputMedia(t *testing.T) {
	file := &tg.InputFile{ID: 1}

	if _, ok := buildInputMedia(file, nil, "a.jpg", mediaMetadata{Kind: mediaKindPhoto, MIME: "image/jpeg"}).(*tg.InputMediaUploadedPhoto); !ok {
		t.Error("photos must be sent as InputMediaUploadedPhoto")
	}

	tests := []struct {
		name        string
		meta        mediaMetadata
		wantNosound bool
	}{
		// Without nosound_video Telegram turns silent videos into GIF
		// animations, which also can't be part of albums.
		{name: "silent video", meta: mediaMetadata{Kind: mediaKindVideo, MIME: "video/mp4"}, wantNosound: true},
		{name: "video with audio", meta: mediaMetadata{Kind: mediaKindVideo, MIME: "video/mp4", HasAudio: true}},
		{name: "gif stays an animation", meta: mediaMetadata{Kind: mediaKindVideo, MIME: "image/gif"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, ok := buildInputMedia(file, nil, "v.mp4", tt.meta).(*tg.InputMediaUploadedDocument)
			if !ok {
				t.Fatal("videos must be sent as InputMediaUploadedDocument")
			}
			if doc.NosoundVideo != tt.wantNosound {
				t.Errorf("expected NosoundVideo=%v, got %v", tt.wantNosound, doc.NosoundVideo)
			}
		})
	}
}
