package telegram

import "testing"

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
