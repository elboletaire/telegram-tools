package telegram

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/elboletaire/ttools/internal/config"
)

func TestCheckCaptionLength(t *testing.T) {
	tests := []struct {
		name        string
		caption     string
		bot         bool
		wantWarning bool
		wantErr     bool
	}{
		{name: "empty", caption: ""},
		{name: "at standard limit", caption: strings.Repeat("a", MaxCaptionLength)},
		{name: "premium only", caption: strings.Repeat("a", MaxCaptionLength+1), wantWarning: true},
		{name: "at premium limit", caption: strings.Repeat("a", MaxPremiumCaptionLength), wantWarning: true},
		{name: "over premium limit", caption: strings.Repeat("a", MaxPremiumCaptionLength+1), wantErr: true},
		{name: "bots cannot use premium limit", caption: strings.Repeat("a", MaxCaptionLength+1), bot: true, wantErr: true},
		// Length is counted in UTF-16 code units: each emoji counts as 2.
		{name: "emoji count double", caption: strings.Repeat("👋", MaxCaptionLength/2+1), bot: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning, err := CheckCaptionLength(tt.caption, tt.bot)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
			}
			if (warning != "") != tt.wantWarning {
				t.Errorf("expected warning=%v, got %q", tt.wantWarning, warning)
			}
			if err != nil && !strings.Contains(err.Error(), "caption too long") {
				t.Errorf("expected a clear error message, got %q", err.Error())
			}
		})
	}
}

func TestUpload_RejectsTooLongCaptionBeforeUploading(t *testing.T) {
	var stderr bytes.Buffer
	svc := NewService(&config.Config{}, WithIO(nil, &bytes.Buffer{}, &stderr))

	err := svc.Upload(context.Background(), UploadRequest{
		ChatId:     "@testchannel",
		FilePath:   "/nonexistent/video.mp4",
		Caption:    strings.Repeat("a", MaxPremiumCaptionLength+1),
		CaptionSet: true,
	})
	if err == nil || !strings.Contains(err.Error(), "caption too long") {
		t.Fatalf("expected caption too long error, got %v", err)
	}
}

func TestReplaceMedia_RejectsTooLongCaptionBeforeUploading(t *testing.T) {
	svc := NewService(&config.Config{}, WithIO(nil, &bytes.Buffer{}, &bytes.Buffer{}))

	err := svc.ReplaceMedia(context.Background(), ReplaceRequest{
		ChatId:     "@testchannel",
		PostId:     1,
		FilePath:   "/nonexistent/video.mp4",
		Caption:    strings.Repeat("a", MaxPremiumCaptionLength+1),
		CaptionSet: true,
	})
	if err == nil || !strings.Contains(err.Error(), "caption too long") {
		t.Fatalf("expected caption too long error, got %v", err)
	}
}
