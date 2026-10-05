package telegram

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gotd/td/crypto"
	"github.com/gotd/td/tg"
)

// MaxAlbumSize is the most items Telegram accepts in one album.
const MaxAlbumSize = 10

// AlbumItem is one file of a grouped upload.
type AlbumItem struct {
	FilePath  string
	ThumbPath string
	Caption   string
}

// UploadAlbumRequest uploads several files as grouped posts (albums).
type UploadAlbumRequest struct {
	ChatId string
	Items  []AlbumItem
	Silent bool
}

// albumGroup is the kind of items that can share an album: photos and videos
// can be mixed, but documents and audio files only go with their own kind.
type albumGroup int

const (
	albumGroupVisual albumGroup = iota
	albumGroupDocument
	albumGroupAudio
)

func (g albumGroup) String() string {
	switch g {
	case albumGroupVisual:
		return "a photo or video"
	case albumGroupAudio:
		return "an audio file"
	default:
		return "a document"
	}
}

func albumGroupForMIME(mimeType string) albumGroup {
	if strings.HasPrefix(strings.ToLower(mimeType), "audio/") {
		return albumGroupAudio
	}
	switch guessKindFromMIME(mimeType) {
	case mediaKindPhoto, mediaKindVideo:
		return albumGroupVisual
	default:
		return albumGroupDocument
	}
}

// checkAlbumGroups fails when the files can't share an album.
func checkAlbumGroups(paths []string, groups []albumGroup) error {
	for i := 1; i < len(groups); i++ {
		if groups[i] != groups[0] {
			return fmt.Errorf("can't group %s (%s) with %s (%s): albums can mix photos and videos, but documents and audio only go with their own kind",
				filepath.Base(paths[i]), groups[i], filepath.Base(paths[0]), groups[0])
		}
	}
	return nil
}

// splitAlbum splits items into albums of at most MaxAlbumSize, balancing their
// sizes so the last album isn't left with a single item (11 → 6 + 5).
func splitAlbum(items []AlbumItem) [][]AlbumItem {
	if len(items) == 0 {
		return nil
	}
	n := (len(items) + MaxAlbumSize - 1) / MaxAlbumSize
	base, extra := len(items)/n, len(items)%n
	chunks := make([][]AlbumItem, 0, n)
	start := 0
	for i := 0; i < n; i++ {
		size := base
		if i < extra {
			size++
		}
		chunks = append(chunks, items[start:start+size])
		start += size
	}
	return chunks
}

// messageMediaToInput turns media returned by messages.uploadMedia into the
// input form messages.sendMultiMedia expects.
func messageMediaToInput(m tg.MessageMediaClass) (tg.InputMediaClass, error) {
	switch v := m.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := v.Photo.AsNotEmpty()
		if !ok {
			return nil, fmt.Errorf("uploaded photo is empty")
		}
		return &tg.InputMediaPhoto{ID: photo.AsInput()}, nil
	case *tg.MessageMediaDocument:
		doc, ok := v.Document.AsNotEmpty()
		if !ok {
			return nil, fmt.Errorf("uploaded document is empty")
		}
		return &tg.InputMediaDocument{ID: doc.AsInput()}, nil
	default:
		return nil, fmt.Errorf("unexpected uploaded media %T", m)
	}
}

// extractMessageIDs returns the ids of all new messages in updates, sorted.
func extractMessageIDs(updates tg.UpdatesClass) []int {
	var list []tg.UpdateClass
	switch u := updates.(type) {
	case *tg.Updates:
		list = u.Updates
	case *tg.UpdatesCombined:
		list = u.Updates
	default:
		if id, ok := extractMessageID(updates); ok {
			return []int{id}
		}
		return nil
	}

	var ids []int
	for _, update := range list {
		var msg tg.MessageClass
		switch v := update.(type) {
		case *tg.UpdateNewChannelMessage:
			msg = v.Message
		case *tg.UpdateNewMessage:
			msg = v.Message
		}
		if m, ok := msg.(*tg.Message); ok {
			ids = append(ids, m.ID)
		}
	}
	sort.Ints(ids)
	return ids
}

// UploadAlbum uploads the items as albums of up to MaxAlbumSize files. All
// files are validated before anything is uploaded. It returns the ids of the
// sent messages.
func (s *Service) UploadAlbum(ctx context.Context, req UploadAlbumRequest) ([]int, error) {
	if strings.TrimSpace(req.ChatId) == "" {
		return nil, fmt.Errorf("channel is required")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("an album needs at least one file")
	}

	paths := make([]string, len(req.Items))
	groups := make([]albumGroup, len(req.Items))
	for i, item := range req.Items {
		mimeType, err := detectMimeType(item.FilePath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(item.FilePath), err)
		}
		paths[i] = item.FilePath
		groups[i] = albumGroupForMIME(mimeType)
		if err := s.checkCaption(item.Caption); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(item.FilePath), err)
		}
	}
	if err := checkAlbumGroups(paths, groups); err != nil {
		return nil, err
	}

	var messageIDs []int
	err := s.run(ctx, func(ctx context.Context, api *tg.Client) error {
		channel, err := s.resolveChat(ctx, api, req.ChatId)
		if err != nil {
			return err
		}

		done := 0
		for c, chunk := range splitAlbum(req.Items) {
			if c > 0 {
				// Keep the same pacing as batch sends to avoid flood waits.
				select {
				case <-time.After(1100 * time.Millisecond):
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			multi := make([]tg.InputSingleMedia, 0, len(chunk))
			for _, item := range chunk {
				done++
				media, err := s.uploadAlbumItem(ctx, api, channel, item, done, len(req.Items))
				if err != nil {
					return err
				}
				randomID, err := crypto.RandInt64(crypto.DefaultRand())
				if err != nil {
					return fmt.Errorf("generate random id: %w", err)
				}
				multi = append(multi, tg.InputSingleMedia{Media: media, RandomID: randomID, Message: item.Caption})
			}

			ids, err := s.sendAlbum(ctx, api, channel, req, multi)
			if err != nil {
				s.evictPeerIfStale(channel, err)
				return err
			}
			messageIDs = append(messageIDs, ids...)
			if s.io.out != nil {
				fmt.Fprintf(s.io.out, "✅ Album of %d files sent to %s%s\n", len(chunk), channel.display, formatMessageIDs(ids))
			}
		}
		return nil
	})
	return messageIDs, err
}

// uploadAlbumItem uploads one file with its own progress display and
// registers it with messages.uploadMedia, as albums require.
func (s *Service) uploadAlbumItem(ctx context.Context, api *tg.Client, channel *channelPeer, item AlbumItem, n, total int) (tg.InputMediaClass, error) {
	fileName := filepath.Base(item.FilePath)
	var thumbName string
	if strings.TrimSpace(item.ThumbPath) != "" {
		thumbName = filepath.Base(item.ThumbPath)
	}
	label := fmt.Sprintf("%s (%d/%d)", uploadActionLabel, n, total)
	captionPreview := ""
	if item.Caption != "" {
		captionPreview = formatCaptionPreview(item.Caption)
	}
	progressUI := newUploadProgressDisplay(s.io.out, label, fileName, thumbName, captionPreview, false)
	if progressUI == nil {
		logStaticStart(s.io.out, label, fileName, thumbName, captionPreview, false)
	}

	media, err := s.prepareAlbumMedia(ctx, api, channel, item, progressUI)
	if progressUI != nil {
		if err != nil {
			progressUI.Fail(err)
		} else {
			progressUI.Success(fmt.Sprintf("%s uploaded", fileName))
		}
		progressUI.Wait()
	} else if err != nil {
		logStaticFailure(s.io.out, err)
	}
	return media, err
}

func (s *Service) prepareAlbumMedia(ctx context.Context, api *tg.Client, channel *channelPeer, item AlbumItem, progressUI *uploadProgressDisplay) (tg.InputMediaClass, error) {
	req := mediaRequest{FilePath: item.FilePath, ThumbPath: item.ThumbPath}
	if progressUI != nil {
		req.Progress = progressUI
	}
	uploaded, err := s.prepareMedia(ctx, api, req)
	if err != nil {
		return nil, err
	}

	var media tg.MessageMediaClass
	onFlood := s.buildFloodLogger(channel.display, filepath.Base(item.FilePath))
	err = callWithFloodRetry(ctx, func() error {
		var callErr error
		media, callErr = api.MessagesUploadMedia(ctx, &tg.MessagesUploadMediaRequest{
			Peer:  channel.peer,
			Media: uploaded,
		})
		return callErr
	}, onFlood)
	if err != nil {
		return nil, fmt.Errorf("register %s: %w", filepath.Base(item.FilePath), err)
	}
	return messageMediaToInput(media)
}

// sendAlbum sends the uploaded media as one album, or as a regular post when
// there is a single item (Telegram albums need at least two).
func (s *Service) sendAlbum(ctx context.Context, api *tg.Client, channel *channelPeer, req UploadAlbumRequest, multi []tg.InputSingleMedia) ([]int, error) {
	var updates tg.UpdatesClass
	onFlood := s.buildFloodLogger(req.ChatId, "album")
	err := callWithFloodRetry(ctx, func() error {
		var callErr error
		if len(multi) == 1 {
			send := &tg.MessagesSendMediaRequest{
				Peer:     channel.peer,
				Media:    multi[0].Media,
				Message:  multi[0].Message,
				RandomID: multi[0].RandomID,
			}
			send.SetSilent(req.Silent)
			updates, callErr = api.MessagesSendMedia(ctx, send)
			return callErr
		}
		send := &tg.MessagesSendMultiMediaRequest{
			Peer:       channel.peer,
			MultiMedia: multi,
		}
		send.SetSilent(req.Silent)
		updates, callErr = api.MessagesSendMultiMedia(ctx, send)
		return callErr
	}, onFlood)
	if err != nil {
		return nil, err
	}
	return extractMessageIDs(updates), nil
}

func formatMessageIDs(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return " as messages " + strings.Join(parts, ", ")
}
