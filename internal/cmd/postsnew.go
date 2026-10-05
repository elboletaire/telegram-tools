package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/telegram-tools/internal/config"
	"github.com/elboletaire/telegram-tools/internal/telegram"
)

// defaultAutoCaptionRegex captures everything after the first "-" in a file name.
const defaultAutoCaptionRegex = "^[^-]*-\\s*(.*)$"

type postsNewOptions struct {
	// Text/message options
	message     string
	messageFile string
	html        bool
	plain       bool
	delimiter   string
	noPreview   bool

	// Media upload options
	files            []string
	thumb            string
	autoCaption      bool
	autoCaptionRegex string
	group            bool // Upload multiple files as grouped post/album

	// Common options
	silent bool
	dryRun bool
	color  string
}

func newPostsNewCommand(sharedOpts *sharedPostsOptions) *cobra.Command {
	opts := &postsNewOptions{
		delimiter:        "[npost]",
		autoCaptionRegex: defaultAutoCaptionRegex,
	}

	cmd := &cobra.Command{
		Use:     "new [message...]",
		Aliases: []string{"post", "create"},
		Short:   "Send text messages or upload media to a channel or group",
		Long: `Send text messages or upload media files to a channel or group.

SMART DETECTION:
Arguments are automatically detected as files or text:
  - If ALL arguments are existing files → upload as media (separate posts)
  - If NO arguments are files → send as text messages
  - If MIXED (some files, some text) → error

TEXT MESSAGES:
Messages are formatted as MarkdownV2 by default. Use standard markdown syntax:
  **bold**, _italic_, ` + "`code`" + `, ` + "```code block```" + `, [link](url)

Messages longer than Telegram's 4096 character limit are split into several
messages, cutting at paragraph breaks, then line breaks, then spaces.

Message input sources (in priority order):
  1. --message-file: Read from file
  2. Stdin: Pipe content (text only)
  3. --message: Direct flag value
  4. Arguments: Direct message text or file paths

MEDIA UPLOADS:
Use --file flag or provide file paths as arguments:
  - Supports multiple files (uploaded as separate posts by default)
  - Use --group to upload them as an album (up to 10 files per album, more are
    split into balanced albums; photos and videos can be mixed, documents and
    audio only with their own kind). --message captions the album; with
    --autocaption every file gets its own caption
  - Auto-detects thumbnails (e.g., video-thumb.jpg)
  - Can set captions using --message or --autocaption

Examples:
  # Send text messages
  ttools posts new "Hello **world**"
  ttools posts new "Post 1" "Post 2"              # 2 separate posts

  # Upload media (smart detection)
  ttools posts new video.mp4                      # Upload 1 file
  ttools posts new video1.mp4 video2.mp4          # Upload 2 separate posts
  ttools posts new *.mp4                          # Upload all .mp4 as separate posts
  ttools posts new *.mp4 --group                  # Upload all as grouped album

  # Explicit --file flag
  ttools posts new --file video.mp4 --message "Caption"
  ttools posts new --file v1.mp4 --file v2.mp4    # 2 separate posts
  ttools posts new --file v1.mp4 --file v2.mp4 --group  # Grouped album

  # Other input methods
  ttools posts new --message-file message.md
  cat message.md | ttools posts new
  ttools posts new "<b>Bold</b> text" --html

  # Preview without sending (long messages show how they would be split)
  cat message.md | ttools posts new --dry-run`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostsNew(cmd.Context(), cmd, args, opts)
		},
	}

	// Message/text flags
	cmd.Flags().StringVar(&opts.message, "message", "", "Message text or caption")
	cmd.Flags().StringVar(&opts.messageFile, "message-file", "", "Read message from file")
	cmd.Flags().BoolVar(&opts.html, "html", false, "Use HTML format instead of MarkdownV2")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Send as plain text (no formatting)")
	cmd.Flags().StringVar(&opts.delimiter, "delimiter", opts.delimiter, "Message delimiter for batch sending")

	// Media upload flags
	cmd.Flags().StringArrayVar(&opts.files, "file", nil, "Media file(s) to upload")
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "Custom thumbnail for uploads")
	cmd.Flags().BoolVar(&opts.autoCaption, "autocaption", false, "Set caption from file name using regex")
	cmd.Flags().StringVar(&opts.autoCaptionRegex, "autocaption-regex", opts.autoCaptionRegex, "Regex to capture caption from file name (uses first group)")
	cmd.Flags().BoolVar(&opts.group, "group", false, "Upload files as albums of up to 10 files")

	// Common flags
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Send without notification")
	cmd.Flags().BoolVar(&opts.noPreview, "no-preview", false, "Disable link preview generation")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview text messages without sending them")
	cmd.Flags().StringVar(&opts.color, "color", "auto", "Color the --dry-run preview: auto, always or never")

	return cmd
}

func runPostsNew(ctx context.Context, cmd *cobra.Command, args []string, opts *postsNewOptions) error {
	cfg, err := configFromContext(cmd)
	if err != nil {
		return err
	}
	// A dry run never connects to Telegram, so it needs no credentials.
	if !opts.dryRun {
		if err := cfg.Requirements(); err != nil {
			return err
		}
	}

	chat, err := cfg.ResolveChat("")
	if err != nil {
		return err
	}

	// Check if --file flag was used explicitly
	if len(opts.files) > 0 {
		return runMediaUpload(ctx, cmd, args, opts, cfg, chat)
	}

	// Smart detection: check if args are files or text
	if len(args) > 0 {
		allFiles, hasFiles, hasNonFiles := detectArgsType(args)

		if hasFiles && hasNonFiles {
			return fmt.Errorf("cannot mix files and text in arguments; use --file for media or --message for text")
		}

		if allFiles {
			// All args are existing files, treat as media upload
			opts.files = args
			return runMediaUpload(ctx, cmd, nil, opts, cfg, chat)
		}
	}

	// Default to text message
	return runTextMessage(ctx, cmd, args, opts, cfg, chat)
}

func runMediaUpload(ctx context.Context, cmd *cobra.Command, args []string, opts *postsNewOptions, cfg *config.Config, chat string) error {
	if opts.dryRun {
		return fmt.Errorf("--dry-run is only supported for text messages")
	}
	out := cmd.OutOrStdout()

	// Gather message/caption
	message, messageProvided, err := readMessageInput(messageInput{
		file:       opts.messageFile,
		message:    opts.message,
		messageSet: cmd.Flags().Changed("message"),
		args:       args,
		stdin:      cmd.InOrStdin(),
	})
	if err != nil {
		return err
	}

	// Check for conflicting flags
	if messageProvided && opts.autoCaption {
		return fmt.Errorf("cannot use both --message (or message input) and --autocaption together")
	}

	// Expand and validate thumb path
	thumbProvided := cmd.Flags().Changed("thumb")
	providedThumb := opts.thumb
	if thumbProvided && providedThumb != "" {
		providedThumb, err := config.ExpandPath(providedThumb)
		if err != nil {
			return fmt.Errorf("expand provided thumb: %w", err)
		}
		opts.thumb = providedThumb
	}

	// Get default thumb from config
	defaultThumb := cfg.Defaults.Thumb
	if defaultThumb != "" {
		var err error
		defaultThumb, err = config.ExpandPath(defaultThumb)
		if err != nil {
			return fmt.Errorf("expand default thumb: %w", err)
		}
	}

	// Compile autocaption regex if needed
	var autoCaptionRe *regexp.Regexp
	if opts.autoCaption {
		re, err := regexp.Compile(opts.autoCaptionRegex)
		if err != nil {
			return fmt.Errorf("compile autocaption regex: %w", err)
		}
		autoCaptionRe = re
	}

	svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), out, cmd.ErrOrStderr()))

	var album []telegram.AlbumItem
	for i, filePath := range opts.files {
		filePath, err := config.ExpandPath(filePath)
		if err != nil {
			return fmt.Errorf("expand file path %q: %w", filePath, err)
		}

		// Determine thumbnail for this file
		thumb := providedThumb
		autoThumbFound := false
		if !thumbProvided {
			thumb, autoThumbFound, err = findSiblingThumbnail(filePath)
			if err != nil {
				return err
			}
			if thumb == "" {
				thumb = defaultThumb
			}
		} else if thumb == "" {
			thumb = defaultThumb
		}

		if autoThumbFound {
			fmt.Fprintf(out, "🖼️ Using detected thumbnail %s\n", filepath.Base(thumb))
		}

		// Determine caption for this file
		caption := message
		captionSet := messageProvided
		usedAutoCaption := false
		if opts.autoCaption && autoCaptionRe != nil {
			name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			rg := autoCaptionRe.FindStringSubmatch(name)
			if len(rg) > 1 {
				caption = strings.TrimSpace(rg[1])
			} else {
				caption = name
			}
			captionSet = true
			usedAutoCaption = true
		}
		logAutoCaption(out, usedAutoCaption, caption)

		if opts.group {
			album = append(album, telegram.AlbumItem{
				FilePath:  filePath,
				ThumbPath: thumb,
				Caption:   albumItemCaption(i, caption, usedAutoCaption),
				ParseMode: captionParseMode(parseModeFor(opts.html, opts.plain), usedAutoCaption),
			})
			continue
		}

		// Upload the file
		if err := svc.Upload(ctx, telegram.UploadRequest{
			ChatId:     chat,
			FilePath:   filePath,
			ThumbPath:  thumb,
			Caption:    caption,
			CaptionSet: captionSet,
			ParseMode:  captionParseMode(parseModeFor(opts.html, opts.plain), usedAutoCaption),
			Silent:     opts.silent,
		}); err != nil {
			return err
		}

		// Add delay between uploads (except after last)
		if i < len(opts.files)-1 {
			select {
			case <-time.After(1100 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if opts.group {
		_, err := svc.UploadAlbum(ctx, telegram.UploadAlbumRequest{
			ChatId: chat,
			Items:  album,
			Silent: opts.silent,
		})
		return err
	}
	return nil
}

// parseModeFor maps the --html and --plain flags to a Telegram parse mode;
// markdown is the default.
func parseModeFor(html, plain bool) string {
	switch {
	case html:
		return "HTML"
	case plain:
		return ""
	default:
		return "MarkdownV2"
	}
}

// captionParseMode returns how a caption is parsed. Captions generated from
// file names are always plain text: "_" or "*" in a name is not formatting.
func captionParseMode(parseMode string, autoCaption bool) string {
	if autoCaption {
		return ""
	}
	return parseMode
}

// albumItemCaption returns the caption of the index-th file of an album: a
// message caption goes only on the first file (Telegram shows it for the whole
// album), while auto-captions are set on every file.
func albumItemCaption(index int, caption string, autoCaption bool) string {
	if autoCaption || index == 0 {
		return caption
	}
	return ""
}

func runTextMessage(ctx context.Context, cmd *cobra.Command, args []string, opts *postsNewOptions, cfg *config.Config, chat string) error {
	out := cmd.OutOrStdout()

	// Determine parse mode
	parseMode := parseModeFor(opts.html, opts.plain)

	text, _, err := readMessageInput(messageInput{
		file:       opts.messageFile,
		message:    opts.message,
		messageSet: cmd.Flags().Changed("message"),
		args:       args,
		stdin:      cmd.InOrStdin(),
	})
	if err != nil {
		return err
	}
	messages := splitMessages(text, opts.delimiter)

	if len(messages) == 0 {
		return fmt.Errorf("no messages provided (use args, --message, --message-file, or pipe from stdin)")
	}

	if opts.dryRun {
		color, err := resolveColor(opts.color, out)
		if err != nil {
			return err
		}
		return printDryRun(out, chat, messages, parseMode, color)
	}

	// Send messages with batch delay
	svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), out, cmd.ErrOrStderr()))

	for i, msg := range messages {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}

		req := telegram.SendMessageRequest{
			ChatId:    chat,
			Message:   msg,
			ParseMode: parseMode,
			Silent:    opts.silent,
			NoWebpage: opts.noPreview,
		}

		messageIDs, err := svc.SendMessage(ctx, req)
		if err != nil {
			return fmt.Errorf("send message %d: %w", i+1, err)
		}

		for _, messageID := range messageIDs {
			fmt.Fprintf(out, "✓ Message sent (ID: %d)\n", messageID)
		}
		if len(messageIDs) > 1 {
			fmt.Fprintf(out, "  (split into %d messages to fit Telegram's length limit)\n", len(messageIDs))
		}

		// Add delay between messages (except after last)
		if i < len(messages)-1 {
			select {
			case <-time.After(1100 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if len(messages) > 1 {
		fmt.Fprintf(out, "\n✓ Sent %d messages\n", len(messages))
	}

	return nil
}

// splitMessages splits content by delimiter and returns non-empty messages
func splitMessages(content, delimiter string) []string {
	parts := strings.Split(content, delimiter)
	messages := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			messages = append(messages, trimmed)
		}
	}
	return messages
}

// detectArgsType checks if arguments are files, text, or mixed
// Returns: allFiles, hasFiles, hasNonFiles
func detectArgsType(args []string) (allFiles bool, hasFiles bool, hasNonFiles bool) {
	for _, arg := range args {
		// Try to expand the path first (handles ~ and env vars)
		expanded, err := config.ExpandPath(arg)
		if err != nil {
			// If expansion fails, treat as non-file (text)
			hasNonFiles = true
			continue
		}

		// Check if the expanded path exists as a file
		if _, err := os.Stat(expanded); err == nil {
			hasFiles = true
		} else {
			hasNonFiles = true
		}
	}
	allFiles = hasFiles && !hasNonFiles
	return
}
