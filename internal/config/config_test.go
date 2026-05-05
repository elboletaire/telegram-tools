package config

import (
	"path/filepath"
	"testing"
)

func TestPeerCacheFile(t *testing.T) {
	cfg := &Config{
		Session: SessionConfig{
			File: "/home/user/.local/share/ttools/session.json",
		},
	}
	got := cfg.PeerCacheFile()
	want := filepath.Join("/home/user/.local/share/ttools", "peers.json")
	if got != want {
		t.Errorf("PeerCacheFile() = %q, want %q", got, want)
	}
}
