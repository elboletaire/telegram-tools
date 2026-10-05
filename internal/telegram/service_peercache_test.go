package telegram

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/elboletaire/telegram-tools/internal/config"
)

func TestResolveChatUsesCanonicalDiskCacheForNumericAliases(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(&config.Config{
		Session: config.SessionConfig{File: filepath.Join(dir, "session.json")},
	}, WithIO(nil, io.Discard, io.Discard))

	const channelID int64 = 1111111111
	const accessHash int64 = -8234567891234
	cached := &channelPeer{
		peer:    &tg.InputPeerChannel{ChannelID: channelID, AccessHash: accessHash},
		channel: &tg.InputChannel{ChannelID: channelID, AccessHash: accessHash},
		display: "-1001111111111",
	}
	svc.diskCache = newPeerDiskCache()
	svc.diskCache.set("chan:1111111111", cached)

	got, err := svc.resolveChat(context.Background(), nil, "-1001111111111")
	if err != nil {
		t.Fatalf("resolveChat: %v", err)
	}
	if got.channel.ChannelID != channelID || got.channel.AccessHash != accessHash {
		t.Fatalf("resolveChat returned channel_id=%d access_hash=%d", got.channel.ChannelID, got.channel.AccessHash)
	}
	if svc.cachedPeer("-1001111111111") == nil {
		t.Fatal("expected raw numeric alias to be promoted into memory cache")
	}
}
