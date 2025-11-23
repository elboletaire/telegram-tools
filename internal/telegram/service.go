package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gotd/td/crypto"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"github.com/elboletaire/ttools/internal/config"
)

// ServiceOption configures the Telegram service behavior.
type ServiceOption func(*Service)

// WithIO overrides the streams used for prompts and status messages.
func WithIO(in io.Reader, out, err io.Writer) ServiceOption {
	return func(s *Service) {
		if in != nil {
			s.io.in = in
		}
		if out != nil {
			s.io.out = out
		}
		if err != nil {
			s.io.err = err
		}
	}
}

// Service exposes higher level helpers to interact with Telegram channels.
type Service struct {
	cfg *config.Config
	io  ioStreams

	prompter *prompter

	peerMu    sync.RWMutex
	peerCache map[string]*channelPeer
}

type ioStreams struct {
	in  io.Reader
	out io.Writer
	err io.Writer
}

const (
	uploadActionLabel   = "↗️ Uploading"
	reuploadActionLabel = "🔄️ Reuploading"
)

// NewService builds a Service with the loaded configuration.
func NewService(cfg *config.Config, opts ...ServiceOption) *Service {
	s := &Service{
		cfg: cfg,
		io: ioStreams{
			in:  os.Stdin,
			out: os.Stdout,
			err: os.Stderr,
		},
		peerCache: make(map[string]*channelPeer),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.prompter = newPrompter(s.io.in, s.io.out, s.io.err)
	return s
}

// UploadRequest represents the data needed to push a new file.
type UploadRequest struct {
	ChatId     string
	FilePath   string
	ThumbPath  string
	Caption    string
	CaptionSet bool
	Silent     bool
}

// ReplaceRequest is used to edit the media content of a message.
type ReplaceRequest struct {
	ChatId       string
	PostId       int
	FilePath     string
	ThumbPath    string
	Caption      string
	CaptionSet   bool
	ClearCaption bool
	Silent       bool
}

// ListPostsRequest describes pagination/search filters.
type ListPostsRequest struct {
	ChatId string
	Limit  int
	Search string
	// OnBatch, if set, is called after each batch is processed with the total
	// number of messages fetched so far (before filtering).
	OnBatch func(total int)
	// OnFloodWait, if set, is called before waiting on Telegram FLOOD_WAIT.
	OnFloodWait func(delay time.Duration, total int)
}

// PostInfo contains the minimal data required for CLI rendering.
type PostInfo struct {
	ID        int
	Date      time.Time
	MediaType string
	Caption   string
}

// SendMessageRequest represents a text message to send.
type SendMessageRequest struct {
	ChatId    string
	Message   string
	ParseMode string // "MarkdownV2", "HTML", or "" for plain text
	Silent    bool
}

// ListChatsRequest describes pagination/search filters for chats.
type ListChatsRequest struct {
	Limit  int
	Search string
	// OnBatch, if set, is called after each batch is processed with the total
	// number of chats fetched so far (before filtering).
	OnBatch func(total int)
	// OnFloodWait, if set, is called before waiting on Telegram FLOOD_WAIT.
	OnFloodWait func(delay time.Duration, total int)
}

// ChatInfo contains the minimal data required for CLI rendering of chats.
type ChatInfo struct {
	ID           int64
	AccessHash   int64
	Title        string
	Username     string
	Type         string
	Participants int
	Unread       int
	LastDate     time.Time
}

// Upload uploads a new file to a channel.
func (s *Service) Upload(ctx context.Context, req UploadRequest) error {
	if strings.TrimSpace(req.FilePath) == "" {
		return fmt.Errorf("file path is required")
	}
	if strings.TrimSpace(req.ChatId) == "" {
		return fmt.Errorf("channel is required")
	}

	fileName := filepath.Base(req.FilePath)
	var thumbName string
	if strings.TrimSpace(req.ThumbPath) != "" {
		thumbName = filepath.Base(req.ThumbPath)
	}
	captionPreview := ""
	if req.CaptionSet {
		captionPreview = formatCaptionPreview(req.Caption)
	}
	progressUI := newUploadProgressDisplay(s.io.out, uploadActionLabel, fileName, thumbName, captionPreview, false)
	if progressUI != nil {
		defer progressUI.Wait()
	} else {
		logStaticStart(s.io.out, uploadActionLabel, fileName, thumbName, captionPreview, false)
	}

	var successMessage string
	err := s.run(ctx, func(ctx context.Context, api *tg.Client) error {
		channel, err := s.resolveChat(ctx, api, req.ChatId)
		if err != nil {
			return err
		}

		media, err := s.prepareMedia(ctx, api, mediaRequest{
			FilePath:  req.FilePath,
			ThumbPath: req.ThumbPath,
			Progress:  progressUI,
		})
		if err != nil {
			return err
		}

		randomID, err := crypto.RandInt64(crypto.DefaultRand())
		if err != nil {
			return fmt.Errorf("generate random id: %w", err)
		}

		send := &tg.MessagesSendMediaRequest{
			Peer:     channel.peer,
			Media:    media,
			Message:  req.Caption,
			RandomID: randomID,
		}
		send.SetSilent(req.Silent)

		var updates tg.UpdatesClass
		onFlood := s.buildFloodLogger(req.ChatId, fileName)
		err = callWithFloodRetry(ctx, func() error {
			u, err := api.MessagesSendMedia(ctx, send)
			if err != nil {
				return err
			}
			updates = u
			return nil
		}, onFlood)
		if err != nil {
			return err
		}

		if id, ok := extractMessageID(updates); ok {
			successMessage = fmt.Sprintf("%s uploaded to %s as message #%d!", fileName, channel.display, id)
		} else {
			successMessage = fmt.Sprintf("%s uploaded to %s!", fileName, channel.display)
		}
		return nil
	})

	if err != nil {
		if progressUI != nil {
			progressUI.Fail(err)
		} else {
			logStaticFailure(s.io.out, err)
		}
		return err
	}

	if progressUI != nil {
		progressUI.Success(successMessage)
		if s.io.out != nil {
			if line := renderStyledSuccess(successMessage, fileName); line != "" {
				fmt.Fprintln(s.io.out, line)
			}
		}
	} else {
		logStaticSuccess(s.io.out, successMessage)
	}

	return nil
}

// SendMessage sends a text message to a channel.
func (s *Service) SendMessage(ctx context.Context, req SendMessageRequest) (int, error) {
	if strings.TrimSpace(req.Message) == "" {
		return 0, fmt.Errorf("message is required")
	}
	if strings.TrimSpace(req.ChatId) == "" {
		return 0, fmt.Errorf("channel is required")
	}

	var messageID int
	err := s.run(ctx, func(ctx context.Context, api *tg.Client) error {
		channel, err := s.resolveChat(ctx, api, req.ChatId)
		if err != nil {
			return err
		}

		message := req.Message
		var entities []tg.MessageEntityClass

		// Parse markdown if requested
		if req.ParseMode == "MarkdownV2" {
			parsedText, parsedEntities, err := ParseMarkdownV2(message)
			if err != nil {
				return fmt.Errorf("parse markdown: %w", err)
			}
			message = parsedText
			entities = parsedEntities
		}
		// Note: HTML parsing would require a different parser
		// For now, HTML mode will send the raw HTML as plain text with no entities

		randomID, err := crypto.RandInt64(crypto.DefaultRand())
		if err != nil {
			return fmt.Errorf("generate random id: %w", err)
		}

		send := &tg.MessagesSendMessageRequest{
			Peer:     channel.peer,
			Message:  message,
			RandomID: randomID,
		}
		send.SetSilent(req.Silent)

		// Only set entities if we have any (for markdown mode)
		if len(entities) > 0 {
			send.Entities = entities
		}

		var updates tg.UpdatesClass
		onFlood := s.buildFloodLogger(req.ChatId, "message")
		err = callWithFloodRetry(ctx, func() error {
			u, err := api.MessagesSendMessage(ctx, send)
			if err != nil {
				return err
			}
			updates = u
			return nil
		}, onFlood)
		if err != nil {
			return err
		}

		if id, ok := extractMessageID(updates); ok {
			messageID = id
		}
		return nil
	})

	return messageID, err
}

// ReplaceMedia edits an existing message with new media.
func (s *Service) ReplaceMedia(ctx context.Context, req ReplaceRequest) error {
	if strings.TrimSpace(req.FilePath) == "" {
		return fmt.Errorf("file path is required")
	}
	if req.PostId == 0 {
		return fmt.Errorf("post id is required")
	}
	if strings.TrimSpace(req.ChatId) == "" {
		return fmt.Errorf("channel is required")
	}

	fileName := filepath.Base(req.FilePath)
	var thumbName string
	if strings.TrimSpace(req.ThumbPath) != "" {
		thumbName = filepath.Base(req.ThumbPath)
	}
	captionPreview := ""
	if req.CaptionSet && !req.ClearCaption {
		captionPreview = formatCaptionPreview(req.Caption)
	}
	progressUI := newUploadProgressDisplay(s.io.out, reuploadActionLabel, fileName, thumbName, captionPreview, req.ClearCaption)
	if progressUI != nil {
		defer progressUI.Wait()
	} else {
		logStaticStart(s.io.out, reuploadActionLabel, fileName, thumbName, captionPreview, req.ClearCaption)
	}

	var successMessage string
	err := s.run(ctx, func(ctx context.Context, api *tg.Client) error {
		channel, err := s.resolveChat(ctx, api, req.ChatId)
		if err != nil {
			return err
		}

		caption := req.Caption
		var entities []tg.MessageEntityClass
		switch {
		case req.ClearCaption:
			caption = ""
		case !req.CaptionSet:
			original, err := s.fetchMessage(ctx, api, channel, req.PostId)
			if err != nil {
				return err
			}
			caption = original.Message
			entities = original.Entities
		}

		media, err := s.prepareMedia(ctx, api, mediaRequest{
			FilePath:  req.FilePath,
			ThumbPath: req.ThumbPath,
			Progress:  progressUI,
		})
		if err != nil {
			return err
		}

		edit := &tg.MessagesEditMessageRequest{
			Peer: channel.peer,
			ID:   req.PostId,
		}
		edit.SetMedia(media)
		edit.SetMessage(caption) // always set message so empty captions clear properly
		if len(entities) > 0 {
			edit.SetEntities(entities)
		}

		onFlood := s.buildFloodLogger(req.ChatId, fileName)
		if err := callWithFloodRetry(ctx, func() error {
			_, err := api.MessagesEditMessage(ctx, edit)
			return err
		}, onFlood); err != nil {
			return err
		}

		successMessage = fmt.Sprintf("%s replaced message #%d in %s!", fileName, req.PostId, channel.display)
		return nil
	})

	if err != nil {
		if progressUI != nil {
			progressUI.Fail(err)
		} else {
			logStaticFailure(s.io.out, err)
		}
		return err
	}

	if progressUI != nil {
		progressUI.Success(successMessage)
	} else {
		logStaticSuccess(s.io.out, successMessage)
	}

	return nil
}

func logStaticStart(out io.Writer, actionLabel, fileName, thumbName, captionPreview string, removingCaption bool) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "%s %s\n", actionLabel, fileName)
	if strings.TrimSpace(thumbName) != "" {
		fmt.Fprintf(out, "🖼️ Will use custom thumbnail %s\n", thumbName)
	}
	logStaticCaption(out, captionPreview, removingCaption)
}

func logStaticSuccess(out io.Writer, message string) {
	if out == nil || strings.TrimSpace(message) == "" {
		return
	}
	fmt.Fprintf(out, "✅ %s\n", message)
}

func logStaticFailure(out io.Writer, err error) {
	if out == nil || err == nil {
		return
	}
	fmt.Fprintf(out, "❌ %v\n", err)
}

func logStaticCaption(out io.Writer, caption string, removing bool) {
	if out == nil {
		return
	}
	if removing {
		fmt.Fprintln(out, "📝 Removing caption")
		return
	}
	preview := strings.TrimSpace(caption)
	if preview == "" {
		return
	}
	fmt.Fprintf(out, "📝 Setting caption to %s\n", preview)
}

func formatCaptionPreview(caption string) string {
	const maxRunes = 80
	trimmed := strings.TrimSpace(caption)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	return string(runes[:maxRunes]) + "..."
}

var successEmphasisStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("84")).Bold(true)

func renderStyledSuccess(message, fileName string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return ""
	}
	styled := msg
	if fileName != "" {
		styled = strings.ReplaceAll(styled, fileName, successEmphasisStyle.Render(fileName))
	}
	if strings.Contains(styled, "uploaded") {
		styled = strings.Replace(styled, "uploaded", successEmphasisStyle.Render("uploaded"), 1)
	}
	return "✅ " + styled
}

// ListPosts returns posts in the channel, optionally limited, walking history
// from newest to oldest.
func (s *Service) ListPosts(ctx context.Context, req ListPostsRequest) ([]PostInfo, error) {
	var posts []PostInfo
	err := s.run(ctx, func(ctx context.Context, api *tg.Client) error {
		channel, err := s.resolveChat(ctx, api, req.ChatId)
		if err != nil {
			return err
		}

		limit := req.Limit
		chunkLimit := 100
		if limit > 0 && limit < chunkLimit {
			chunkLimit = limit
		}

		offsetID := 0
		prevOffsetID := -1
		filter := strings.ToLower(strings.TrimSpace(req.Search))
		totalFetched := 0

		throttle := newThrottle(750 * time.Millisecond)
		for {
			if err := throttle.Wait(ctx); err != nil {
				return err
			}

			history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:      channel.peer,
				Limit:     chunkLimit,
				OffsetID:  offsetID,
				AddOffset: 0,
			})
			if err != nil {
				if wait, ok := tgerr.AsFloodWait(err); ok {
					delay := wait + time.Second
					if req.OnFloodWait != nil {
						req.OnFloodWait(delay, totalFetched)
					}
					if err := sleepWithContext(ctx, delay); err != nil {
						return err
					}
					continue
				}
				return err
			}

			messages, err := collectMessages(history)
			if err != nil {
				return err
			}
			if len(messages) == 0 {
				break
			}

			lastID := messages[len(messages)-1].ID
			if lastID == offsetID || lastID == prevOffsetID {
				// Avoid infinite loops if the server returns the same page.
				break
			}
			prevOffsetID = offsetID

			totalFetched += len(messages)

			// Stop fetching if we've reached the API fetch limit
			if limit > 0 && totalFetched >= limit {
				break
			}

			for _, msg := range messages {
				caption := strings.TrimSpace(msg.Message)
				if filter != "" && !strings.Contains(strings.ToLower(caption), filter) {
					continue
				}

				posts = append(posts, PostInfo{
					ID:        msg.ID,
					Date:      time.Unix(int64(msg.Date), 0).UTC(),
					MediaType: describeMedia(msg.Media),
					Caption:   caption,
				})
			}

			if req.OnBatch != nil {
				req.OnBatch(totalFetched)
			}

			offsetID = lastID
		}

		return nil
	})
	return posts, err
}

// ListChats returns chats/channels the user is part of, optionally limited and filtered.
func (s *Service) ListChats(ctx context.Context, req ListChatsRequest) ([]ChatInfo, error) {
	var chats []ChatInfo
	err := s.run(ctx, func(ctx context.Context, api *tg.Client) error {
		offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})
		offsetID := 0
		offsetDate := 0
		filter := strings.ToLower(strings.TrimSpace(req.Search))
		totalFetched := 0
		seen := make(map[int64]bool) // Track seen chat IDs to avoid duplicates

		// Calculate chunk limit based on requested limit
		chunkLimit := 100
		if req.Limit > 0 && req.Limit < chunkLimit {
			chunkLimit = req.Limit
		}

		throttle := newThrottle(750 * time.Millisecond)
		for {
			if err := throttle.Wait(ctx); err != nil {
				return err
			}

			resp, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
				OffsetDate: offsetDate,
				OffsetID:   offsetID,
				OffsetPeer: offsetPeer,
				Limit:      chunkLimit,
				Hash:       0,
			})
			if err != nil {
				if wait, ok := tgerr.AsFloodWait(err); ok {
					delay := wait + time.Second
					if req.OnFloodWait != nil {
						req.OnFloodWait(delay, totalFetched)
					}
					if err := sleepWithContext(ctx, delay); err != nil {
						return err
					}
					continue
				}
				return err
			}

			batch, err := newDialogBatch(resp)
			if err != nil {
				return err
			}

			if len(batch.dialogs) == 0 {
				break
			}

			totalFetched += len(batch.dialogs)

			// Stop fetching if we've reached the API fetch limit
			if req.Limit > 0 && totalFetched >= req.Limit {
				break
			}

			// Extract chat info from dialogs
			for _, d := range batch.dialogs {
				dialog, ok := d.(*tg.Dialog)
				if !ok {
					continue
				}

				chatInfo := extractChatInfo(dialog, batch.chats, batch.messages)
				if chatInfo == nil {
					continue
				}

				// Apply search filter
				if filter != "" {
					titleMatch := strings.Contains(strings.ToLower(chatInfo.Title), filter)
					usernameMatch := strings.Contains(strings.ToLower(chatInfo.Username), filter)
					if !titleMatch && !usernameMatch {
						continue
					}
				}

				// Skip if we've already seen this chat (avoid duplicates)
				if seen[chatInfo.ID] {
					continue
				}
				seen[chatInfo.ID] = true

				chats = append(chats, *chatInfo)
			}

			if req.OnBatch != nil {
				req.OnBatch(totalFetched)
			}

			// Set up pagination for next batch
			last, ok := batch.dialogs[len(batch.dialogs)-1].(*tg.Dialog)
			if !ok {
				break
			}
			offsetID = last.TopMessage
			offsetDate = findMessageDate(batch.messages, last.TopMessage)
			if offsetDate == 0 {
				offsetDate = int(time.Now().Unix())
			}
			offsetPeer = buildInputPeer(last.Peer, batch.chats)
			if offsetPeer == nil {
				break
			}
		}

		return nil
	})
	return chats, err
}

func (s *Service) run(ctx context.Context, fn func(context.Context, *tg.Client) error) error {
	cfg := s.cfg
	if cfg == nil {
		return errors.New("configuration is not available")
	}
	if err := ensureDir(filepath.Dir(cfg.SessionFile())); err != nil {
		return fmt.Errorf("prepare session directory: %w", err)
	}

	storage := &session.FileStorage{Path: cfg.SessionFile()}
	client := telegram.NewClient(cfg.API.ID, cfg.API.Hash, telegram.Options{
		SessionStorage: storage,
	})

	return client.Run(ctx, func(runCtx context.Context) error {
		flow := auth.NewFlow(s.authenticator(), auth.SendCodeOptions{})
		if err := client.Auth().IfNecessary(runCtx, flow); err != nil {
			return err
		}
		return fn(runCtx, client.API())
	})
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type throttle struct {
	interval time.Duration
	last     time.Time
}

func newThrottle(interval time.Duration) *throttle {
	if interval <= 0 {
		return &throttle{}
	}
	return &throttle{interval: interval}
}

func (t *throttle) Wait(ctx context.Context) error {
	if t == nil || t.interval <= 0 {
		return nil
	}
	now := time.Now()
	if t.last.IsZero() {
		t.last = now
		return nil
	}
	sleepFor := t.interval - now.Sub(t.last)
	if sleepFor > 0 {
		if err := sleepWithContext(ctx, sleepFor); err != nil {
			return err
		}
	}
	t.last = time.Now()
	return nil
}

func ensureDir(path string) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o700)
}

func (s *Service) authenticator() auth.UserAuthenticator {
	return &interactiveAuth{
		phone:    strings.TrimSpace(s.cfg.Session.Phone),
		password: strings.TrimSpace(s.cfg.Session.Password),
		prompter: s.prompter,
	}
}

func (s *Service) buildFloodLogger(chatDisplay, fileName string) func(time.Duration) {
	return func(delay time.Duration) {
		out := s.io.out
		if out == nil {
			return
		}
		fmt.Fprintf(out, "\rRate limit hit while talking to %s (%s), waiting %s...", chatDisplay, fileName, delay.Round(time.Second))
	}
}

func callWithFloodRetry(ctx context.Context, fn func() error, onFlood func(time.Duration)) error {
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if delay, ok := tgerr.AsFloodWait(err); ok {
			delay += time.Second
			if onFlood != nil {
				onFlood(delay)
			}
			if err := sleepWithContext(ctx, delay); err != nil {
				return err
			}
			continue
		}
		return err
	}
}

func (s *Service) resolveChat(ctx context.Context, api *tg.Client, identifier string) (*channelPeer, error) {
	id := strings.TrimSpace(identifier)
	if id == "" {
		return nil, fmt.Errorf("chat is required")
	}
	if peer := s.cachedPeer(id); peer != nil {
		return peer, nil
	}

	if ident, ok := parseChatIdentifier(id); ok {
		peer, err := s.resolveByChatID(ctx, api, ident)
		if err != nil {
			return nil, err
		}
		s.cachePeer(peer, id, ident.cacheKey())
		return peer, nil
	}

	resolver := peer.DefaultResolver(api)
	promise := peer.Resolve(resolver, id)
	inputPeer, err := promise(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve channel %q: %w", id, err)
	}

	asChannel, ok := inputPeer.(*tg.InputPeerChannel)
	if !ok {
		return nil, fmt.Errorf("peer %q is not a channel", id)
	}
	if asChannel.AccessHash == 0 {
		return nil, fmt.Errorf("channel %q is missing access hash", id)
	}

	peerInfo := &channelPeer{
		peer:    asChannel,
		channel: &tg.InputChannel{ChannelID: asChannel.ChannelID, AccessHash: asChannel.AccessHash},
		display: id,
	}
	s.cachePeer(peerInfo, id)
	return peerInfo, nil
}

func (s *Service) fetchMessage(ctx context.Context, api *tg.Client, channel *channelPeer, id int) (*tg.Message, error) {
	resp, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: channel.channel,
		ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: id}},
	})
	if err != nil {
		return nil, err
	}

	messages, err := collectMessages(resp)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("message #%d not found", id)
	}
	return messages[0], nil
}

type channelPeer struct {
	peer    *tg.InputPeerChannel
	channel *tg.InputChannel
	display string
}

const botAPIChannelOffset int64 = 1000000000000

type chatKind int

const (
	chatKindUnknown chatKind = iota
	chatKindChannel
)

type chatIdentifier struct {
	raw  string
	kind chatKind
	id   int64
}

func (c chatIdentifier) cacheKey() string {
	if c.kind == chatKindChannel && c.id > 0 {
		return fmt.Sprintf("chan:%d", c.id)
	}
	return ""
}

func parseChatIdentifier(raw string) (chatIdentifier, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return chatIdentifier{}, false
	}
	n, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return chatIdentifier{}, false
	}
	ident := chatIdentifier{raw: trimmed}
	switch {
	case n <= -botAPIChannelOffset:
		ident.kind = chatKindChannel
		ident.id = -n - botAPIChannelOffset
		return ident, true
	case n > 0:
		ident.kind = chatKindChannel
		ident.id = n
		return ident, true
	default:
		return chatIdentifier{}, false
	}
}

func (s *Service) resolveByChatID(ctx context.Context, api *tg.Client, ident chatIdentifier) (*channelPeer, error) {
	if ident.kind != chatKindChannel {
		return nil, fmt.Errorf("chat id %s is not a supported channel identifier", ident.raw)
	}
	peer, err := s.lookupChannelDialog(ctx, api, ident.id)
	if err != nil {
		return nil, err
	}
	if peer == nil {
		return nil, fmt.Errorf("chat id %s not found among your dialogs; ensure this account joined the channel", ident.raw)
	}
	peer.display = ident.raw
	return peer, nil
}

func (s *Service) lookupChannelDialog(ctx context.Context, api *tg.Client, channelID int64) (*channelPeer, error) {
	offsetPeer := tg.InputPeerClass(&tg.InputPeerEmpty{})
	offsetID := 0
	offsetDate := 0
	for {
		resp, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			OffsetPeer: offsetPeer,
			Limit:      100,
			Hash:       0,
		})
		if err != nil {
			return nil, fmt.Errorf("get dialogs: %w", err)
		}

		batch, err := newDialogBatch(resp)
		if err != nil {
			return nil, err
		}

		if peer := matchChannelInChats(batch.chats, channelID); peer != nil {
			return peer, nil
		}

		if len(batch.dialogs) == 0 {
			break
		}
		last, ok := batch.dialogs[len(batch.dialogs)-1].(*tg.Dialog)
		if !ok {
			break
		}
		offsetID = last.TopMessage
		offsetDate = findMessageDate(batch.messages, last.TopMessage)
		if offsetDate == 0 {
			offsetDate = int(time.Now().Unix())
		}
		offsetPeer = buildInputPeer(last.Peer, batch.chats)
		if offsetPeer == nil {
			offsetPeer = &tg.InputPeerEmpty{}
		}
	}
	return nil, nil
}

type dialogBatch struct {
	dialogs  []tg.DialogClass
	messages []tg.MessageClass
	chats    []tg.ChatClass
}

func newDialogBatch(resp tg.MessagesDialogsClass) (dialogBatch, error) {
	switch v := resp.(type) {
	case *tg.MessagesDialogs:
		return dialogBatch{
			dialogs:  v.Dialogs,
			messages: v.Messages,
			chats:    v.Chats,
		}, nil
	case *tg.MessagesDialogsSlice:
		return dialogBatch{
			dialogs:  v.Dialogs,
			messages: v.Messages,
			chats:    v.Chats,
		}, nil
	default:
		return dialogBatch{}, fmt.Errorf("unsupported dialogs response %T", resp)
	}
}

func matchChannelInChats(chats []tg.ChatClass, channelID int64) *channelPeer {
	ch := findChannel(chats, channelID)
	if ch == nil || ch.AccessHash == 0 {
		return nil
	}
	return &channelPeer{
		peer:    &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash},
		channel: &tg.InputChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash},
	}
}

func findChannel(chats []tg.ChatClass, channelID int64) *tg.Channel {
	for _, ch := range chats {
		channel, ok := ch.(*tg.Channel)
		if !ok {
			continue
		}
		if channel.ID == channelID {
			return channel
		}
	}
	return nil
}

func findMessageDate(messages []tg.MessageClass, id int) int {
	for _, m := range messages {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		if msg.ID == id {
			return msg.Date
		}
	}
	return 0
}

func buildInputPeer(peer tg.PeerClass, chats []tg.ChatClass) tg.InputPeerClass {
	switch p := peer.(type) {
	case *tg.PeerChannel:
		ch := findChannel(chats, p.ChannelID)
		if ch == nil || ch.AccessHash == 0 {
			return &tg.InputPeerEmpty{}
		}
		return &tg.InputPeerChannel{ChannelID: p.ChannelID, AccessHash: ch.AccessHash}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	default:
		return &tg.InputPeerEmpty{}
	}
}

func (s *Service) cachedPeer(key string) *channelPeer {
	s.peerMu.RLock()
	defer s.peerMu.RUnlock()
	return s.peerCache[key]
}

func (s *Service) cachePeer(peer *channelPeer, keys ...string) {
	if peer == nil {
		return
	}
	s.peerMu.Lock()
	defer s.peerMu.Unlock()
	for _, key := range keys {
		if key == "" {
			continue
		}
		s.peerCache[key] = peer
	}
}
