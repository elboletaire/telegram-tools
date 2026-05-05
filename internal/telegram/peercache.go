package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gotd/td/tg"
)

// peerCacheEntry is a single serialisable peer record.
type peerCacheEntry struct {
	ChannelID  int64  `json:"channel_id"`
	AccessHash int64  `json:"access_hash"`
	Display    string `json:"display"`
}

// peerCacheFile represents the on-disk JSON structure.
type peerCacheFile struct {
	Version int                       `json:"version"`
	Peers   map[string]peerCacheEntry `json:"peers"`
}

// peerDiskCache holds an in-memory copy of the on-disk peer cache.
type peerDiskCache struct {
	entries map[string]peerCacheEntry
}

// newPeerDiskCache returns an empty disk cache.
func newPeerDiskCache() *peerDiskCache {
	return &peerDiskCache{
		entries: make(map[string]peerCacheEntry),
	}
}

// load reads and parses the peer cache from path.  If the file does not exist
// the cache is left empty (no error).  Corrupt JSON returns an error.
func (c *peerDiskCache) load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read peer cache: %w", err)
	}

	var f peerCacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse peer cache: %w", err)
	}

	// Accept version 1; ignore any other version (reset to empty).
	if f.Version != 1 {
		c.entries = make(map[string]peerCacheEntry)
		return nil
	}

	c.entries = f.Peers
	if c.entries == nil {
		c.entries = make(map[string]peerCacheEntry)
	}
	return nil
}

// save writes the current cache to path as JSON. It uses a temporary file
// and atomic rename to avoid corrupting the cache on partial writes.
func (c *peerDiskCache) save(path string) error {
	f := peerCacheFile{
		Version: 1,
		Peers:   c.entries,
	}
	if f.Peers == nil {
		f.Peers = make(map[string]peerCacheEntry)
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal peer cache: %w", err)
	}
	// Append newline for human readability.
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create peer cache dir: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write peer cache temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename peer cache: %w", err)
	}
	return nil
}

// get looks up a key in the disk cache and returns a channelPeer if found.
func (c *peerDiskCache) get(key string) (*channelPeer, bool) {
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	return &channelPeer{
		peer:    &tg.InputPeerChannel{ChannelID: entry.ChannelID, AccessHash: entry.AccessHash},
		channel: &tg.InputChannel{ChannelID: entry.ChannelID, AccessHash: entry.AccessHash},
		display: entry.Display,
	}, true
}

// set adds or overwrites a peer entry under the given key.
func (c *peerDiskCache) set(key string, peer *channelPeer) {
	if peer == nil || key == "" {
		return
	}
	c.entries[key] = peerCacheEntry{
		ChannelID:  peer.channel.ChannelID,
		AccessHash: peer.channel.AccessHash,
		Display:    peer.display,
	}
}

// evict removes every entry whose channel_id matches the given ID.
func (c *peerDiskCache) evict(channelID int64) {
	for key, entry := range c.entries {
		if entry.ChannelID == channelID {
			delete(c.entries, key)
		}
	}
}
