package cmd

import "testing"

func TestAlbumItemCaption(t *testing.T) {
	tests := []struct {
		name        string
		index       int
		caption     string
		autoCaption bool
		want        string
	}{
		{name: "message goes on the first item", index: 0, caption: "hello", want: "hello"},
		{name: "message is not repeated", index: 1, caption: "hello", want: ""},
		{name: "autocaption on every item", index: 3, caption: "from name", autoCaption: true, want: "from name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := albumItemCaption(tt.index, tt.caption, tt.autoCaption); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestCaptionParseMode(t *testing.T) {
	tests := []struct {
		name        string
		html, plain bool
		autoCaption bool
		want        string
	}{
		{name: "markdown by default", want: "MarkdownV2"},
		{name: "html", html: true, want: "HTML"},
		{name: "plain", plain: true, want: ""},
		{name: "file names are never markup", html: true, autoCaption: true, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := captionParseMode(parseModeFor(tt.html, tt.plain), tt.autoCaption); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
