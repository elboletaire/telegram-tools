package telegram

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/elboletaire/telegram-tools/internal/config"
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

func TestPrepareCaption_ParsesFormatting(t *testing.T) {
	svc := NewService(&config.Config{}, WithIO(nil, &bytes.Buffer{}, &bytes.Buffer{}))

	tests := []struct {
		name      string
		caption   string
		parseMode string
		wantText  string
		wantCount int
	}{
		{name: "markdown", caption: "**bold** and my_var", parseMode: "MarkdownV2", wantText: "bold and my_var", wantCount: 1},
		{name: "html", caption: "<b>bold</b> <u>u</u>", parseMode: "HTML", wantText: "bold u", wantCount: 2},
		{name: "plain", caption: "**bold**", parseMode: "", wantText: "**bold**", wantCount: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, entities, err := svc.prepareCaption(tt.caption, tt.parseMode)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if text != tt.wantText || len(entities) != tt.wantCount {
				t.Errorf("expected (%q, %d entities), got (%q, %d entities)", tt.wantText, tt.wantCount, text, len(entities))
			}
		})
	}
}

func TestPrepareCaption_LengthCountsParsedText(t *testing.T) {
	bot := NewService(&config.Config{Session: config.SessionConfig{BotToken: "123:fake"}}, WithIO(nil, &bytes.Buffer{}, &bytes.Buffer{}))

	// The markup pushes the raw text over 1024, but Telegram counts the result.
	fits := "**" + strings.Repeat("a", MaxCaptionLength) + "**"
	if _, _, err := bot.prepareCaption(fits, "MarkdownV2"); err != nil {
		t.Errorf("expected a %d-character parsed caption to fit, got %v", MaxCaptionLength, err)
	}

	tooLong := "**" + strings.Repeat("a", MaxCaptionLength+1) + "**"
	if _, _, err := bot.prepareCaption(tooLong, "MarkdownV2"); err == nil || !strings.Contains(err.Error(), "caption too long") {
		t.Errorf("expected caption too long, got %v", err)
	}
}
