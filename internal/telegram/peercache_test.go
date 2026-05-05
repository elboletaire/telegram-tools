package telegram

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gotd/td/tg"
)

func TestPeerDiskCache_Load_MissingFile(t *testing.T) {
	c := newPeerDiskCache()
	err := c.load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("load of missing file should not error: %v", err)
	}
	if len(c.entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(c.entries))
	}
}

func TestPeerDiskCache_Load_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")
	content := `{
  "version": 1,
  "peers": {
    "-1001111111111": {
      "channel_id": 1111111111,
      "access_hash": -8234567891234,
      "display": "-1001111111111"
    },
    "chan:1111111111": {
      "channel_id": 1111111111,
      "access_hash": -8234567891234,
      "display": "-1001111111111"
    },
    "@somechannel": {
      "channel_id": 1234567890,
      "access_hash": 5678901234567,
      "display": "@somechannel"
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	c := newPeerDiskCache()
	if err := c.load(path); err != nil {
		t.Fatalf("load of valid file should not error: %v", err)
	}

	// Check @somechannel
	peer, ok := c.get("@somechannel")
	if !ok {
		t.Fatal("expected @somechannel to be found")
	}
	if peer.channel.ChannelID != 1234567890 {
		t.Errorf("expected channel_id 1234567890, got %d", peer.channel.ChannelID)
	}
	if peer.channel.AccessHash != 5678901234567 {
		t.Errorf("expected access_hash 5678901234567, got %d", peer.channel.AccessHash)
	}
	if peer.display != "@somechannel" {
		t.Errorf("expected display @somechannel, got %s", peer.display)
	}

	// Check numeric key
	peer, ok = c.get("-1001111111111")
	if !ok {
		t.Fatal("expected -1001111111111 to be found")
	}
	if peer.channel.ChannelID != 1111111111 {
		t.Errorf("expected channel_id 1111111111, got %d", peer.channel.ChannelID)
	}

	// Check chan: key
	peer, ok = c.get("chan:1111111111")
	if !ok {
		t.Fatal("expected chan:1111111111 to be found")
	}
	if peer.channel.ChannelID != 1111111111 {
		t.Errorf("expected channel_id 1111111111, got %d", peer.channel.ChannelID)
	}
}

func TestPeerDiskCache_Load_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := newPeerDiskCache()
	err := c.load(path)
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

func TestPeerDiskCache_Load_WrongVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")
	content := `{"version": 2, "peers": {"key": {"channel_id": 1, "access_hash": 2, "display": "d"}}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	c := newPeerDiskCache()
	if err := c.load(path); err != nil {
		t.Fatalf("unknown version should not error: %v", err)
	}
	if len(c.entries) != 0 {
		t.Fatalf("unknown version should yield empty cache, got %d entries", len(c.entries))
	}
}

func TestPeerDiskCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.json")
	const channelID int64 = 9007199254740993
	const accessHash int64 = -9223372036854775807

	c := newPeerDiskCache()
	peer := &channelPeer{
		peer:    &tg.InputPeerChannel{ChannelID: channelID, AccessHash: accessHash},
		channel: &tg.InputChannel{ChannelID: channelID, AccessHash: accessHash},
		display: "@test",
	}
	c.set("@test", peer)

	// Second key for same channel (chan: prefix)
	peer2 := &channelPeer{
		peer:    &tg.InputPeerChannel{ChannelID: channelID, AccessHash: accessHash},
		channel: &tg.InputChannel{ChannelID: channelID, AccessHash: accessHash},
		display: "@test",
	}
	c.set("chan:9007199254740993", peer2)

	if err := c.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Verify file contents
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f peerCacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("the saved file is valid JSON: %v", err)
	}
	if f.Version != 1 {
		t.Errorf("expected version 1, got %d", f.Version)
	}
	if f.Peers["@test"].ChannelID != channelID {
		t.Errorf("expected channel_id %d, got %d", channelID, f.Peers["@test"].ChannelID)
	}
	if f.Peers["@test"].AccessHash != accessHash {
		t.Errorf("expected access_hash %d, got %d", accessHash, f.Peers["@test"].AccessHash)
	}

	// Reload and verify
	c2 := newPeerDiskCache()
	if err := c2.load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	if p, ok := c2.get("@test"); !ok || p.channel.ChannelID != channelID || p.channel.AccessHash != accessHash {
		t.Fatal("round-trip failed for @test")
	}
	if p, ok := c2.get("chan:9007199254740993"); !ok || p.channel.ChannelID != channelID || p.channel.AccessHash != accessHash {
		t.Fatal("round-trip failed for chan:9007199254740993")
	}
}

func TestPeerDiskCache_Evict(t *testing.T) {
	c := newPeerDiskCache()
	peer := &channelPeer{
		channel: &tg.InputChannel{ChannelID: 99, AccessHash: 111},
	}
	c.set("-10099", peer)
	c.set("chan:99", peer)

	// Different channel
	peer2 := &channelPeer{
		channel: &tg.InputChannel{ChannelID: 100, AccessHash: 222},
	}
	c.set("@other", peer2)

	c.evict(99)

	if _, ok := c.get("-10099"); ok {
		t.Error("expected -10099 to be evicted")
	}
	if _, ok := c.get("chan:99"); ok {
		t.Error("expected chan:99 to be evicted")
	}
	if _, ok := c.get("@other"); !ok {
		t.Error("expected @other to remain")
	}
}

func TestPeerDiskCache_Get_MissingKey(t *testing.T) {
	c := newPeerDiskCache()
	if _, ok := c.get("nonexistent"); ok {
		t.Error("expected nil for missing key")
	}
}

func TestPeerDiskCache_Set_NilPeer(t *testing.T) {
	c := newPeerDiskCache()
	c.set("key", nil)
	if _, ok := c.get("key"); ok {
		t.Error("expected nil peer not to be stored")
	}
}

func TestPeerDiskCache_Set_NilChannel(t *testing.T) {
	c := newPeerDiskCache()
	c.set("key", &channelPeer{})
	if _, ok := c.get("key"); ok {
		t.Error("expected nil channel not to be stored")
	}
}

func TestPeerDiskCache_Set_EmptyKey(t *testing.T) {
	c := newPeerDiskCache()
	peer := &channelPeer{
		channel: &tg.InputChannel{ChannelID: 1, AccessHash: 1},
	}
	c.set("", peer)
	if len(c.entries) != 0 {
		t.Error("expected empty key not to be stored")
	}
}
