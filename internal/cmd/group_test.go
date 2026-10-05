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
