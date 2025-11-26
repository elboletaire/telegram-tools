package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
	"github.com/elboletaire/ttools/internal/ui/postslist"
)

type postsEditOptions struct {
	// Post selection
	postID   int
	limit    int
	search   string
	pageSize int

	// Message/caption options
	message      string
	messageFile  string
	clearMessage bool
	html         bool
	plain        bool

	// Media options
	file  string
	thumb string

	// Common options
	silent bool
}

func newPostsEditCommand(sharedOpts *sharedPostsOptions) *cobra.Command {
	opts := &postsEditOptions{
		limit:    sharedOpts.limit,
		pageSize: sharedOpts.pageSize,
	}

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing channel post",
		Long: `Edit an existing channel post's message or media.

POST SELECTION:
If --post-id is not provided, an interactive selector will be shown.

MESSAGE/TEXT EDITING:
Update the message text using the same flexible input options as 'posts new':
  1. --message-file: Read from file
  2. Stdin: Pipe content (text only)
  3. --message: Direct flag value
  4. Arguments: Direct message text

Messages support MarkdownV2 (default), HTML (--html), or plain text (--plain).

MEDIA EDITING:
Replace the media file using --file flag. Can also update thumbnail with --thumb.

Examples:
  # Edit message text (interactive post selection)
  ttools posts edit "New message text"

  # Edit specific post by ID
  ttools posts edit --post-id 123 "Updated text"

  # Edit with piped input
  cat new-message.md | ttools posts edit --post-id 123

  # Replace media file
  ttools posts edit --post-id 123 --file new-video.mp4

  # Replace media and update caption
  ttools posts edit --post-id 123 --file new-video.mp4 --message "New caption"

  # Clear message/caption
  ttools posts edit --post-id 123 --clear-message

  # Search and select post interactively
  ttools posts edit --search "keyword" "New text"`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostsEdit(cmd.Context(), cmd, args, opts)
		},
	}

	// Post selection flags
	cmd.Flags().IntVar(&opts.postID, "post-id", 0, "Message identifier to edit (0 = interactive selector)")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", opts.limit, "Number of entries to fetch for selection (0 = all)")
	cmd.Flags().StringVar(&opts.search, "search", "", "Filter selectable posts containing this substring")
	cmd.Flags().IntVar(&opts.pageSize, "page-size", opts.pageSize, "Entries per page in selector")

	// Message/text flags
	cmd.Flags().StringVar(&opts.message, "message", "", "New message text or caption")
	cmd.Flags().StringVar(&opts.messageFile, "message-file", "", "Read message from file")
	cmd.Flags().BoolVar(&opts.clearMessage, "clear-message", false, "Remove the message/caption entirely")
	cmd.Flags().BoolVar(&opts.clearMessage, "remove-message", false, "Remove the message/caption entirely (alias)")
	cmd.Flags().BoolVar(&opts.html, "html", false, "Use HTML format instead of MarkdownV2")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Send as plain text (no formatting)")

	// Media flags
	cmd.Flags().StringVar(&opts.file, "file", "", "New media file to replace")
	cmd.Flags().StringVar(&opts.thumb, "thumb", "", "New thumbnail for media")

	// Common flags
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Edit message silently if possible")

	return cmd
}

func runPostsEdit(ctx context.Context, cmd *cobra.Command, args []string, opts *postsEditOptions) error {
	out := cmd.OutOrStdout()

	cfg, err := configFromContext(cmd)
	if err != nil {
		return err
	}
	if err := cfg.Requirements(); err != nil {
		return err
	}

	chat, err := cfg.ResolveChat("")
	if err != nil {
		return err
	}

	svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), out, cmd.ErrOrStderr()))

	// Select post if post-id not provided
	if opts.postID == 0 {
		req := fetchPostsRequest{
			ChatId: chat,
			Limit:  opts.limit,
			Search: opts.search,
		}

		posts, err := fetchPostsWithProgress(
			ctx,
			cfg,
			out,
			cmd.InOrStdin(),
			cmd.ErrOrStderr(),
			req,
		)
		if err != nil {
			return err
		}

		if len(posts) == 0 {
			return errors.New("no posts available to select")
		}

		fmt.Fprintf(out, "Loaded %d posts\n\n", len(posts))
		selected, ok, err := postslist.Select(cmd.InOrStdin(), out, chat, opts.search, posts, opts.pageSize)
		if err != nil {
			return fmt.Errorf("select post: %w", err)
		}
		if !ok {
			return errors.New("post selection cancelled")
		}
		opts.postID = selected.ID
	}

	// Check if this is media replacement or text edit
	if opts.file != "" {
		return runMediaReplace(ctx, cmd, args, opts, cfg, chat, svc)
	}

	return runTextEdit(ctx, cmd, args, opts, cfg, chat, svc)
}

func runMediaReplace(ctx context.Context, cmd *cobra.Command, args []string, opts *postsEditOptions, cfg *config.Config, chat string, svc *telegram.Service) error {
	// Expand file path
	filePath, err := config.ExpandPath(opts.file)
	if err != nil {
		return fmt.Errorf("expand file path: %w", err)
	}

	// Handle thumbnail
	thumb := opts.thumb
	if thumb == "" {
		thumb = cfg.Defaults.Thumb
	}
	if thumb != "" {
		thumb, err = config.ExpandPath(thumb)
		if err != nil {
			return fmt.Errorf("expand thumb: %w", err)
		}
	}

	// Gather caption/message
	message := gatherMessageEdit(cmd, args, opts)
	messageProvided := cmd.Flags().Changed("message") || opts.messageFile != "" || isPipedInput() || len(args) > 0

	// Check for conflicting flags
	if messageProvided && opts.clearMessage {
		return errors.New("--message and --clear-message may not be used together")
	}

	return svc.ReplaceMedia(ctx, telegram.ReplaceRequest{
		ChatId:       chat,
		PostId:       opts.postID,
		FilePath:     filePath,
		ThumbPath:    thumb,
		Caption:      message,
		CaptionSet:   messageProvided || opts.clearMessage,
		ClearCaption: opts.clearMessage,
		Silent:       opts.silent,
	})
}

func runTextEdit(ctx context.Context, cmd *cobra.Command, args []string, opts *postsEditOptions, cfg *config.Config, chat string, svc *telegram.Service) error {
	// For text-only edits, we need to use the EditMessage API (not yet implemented in service)
	// For now, return an error message
	return errors.New("text-only editing is not yet implemented; please use --file to replace media")
}

// gatherMessageEdit collects message/caption from various sources for edit command
func gatherMessageEdit(cmd *cobra.Command, args []string, opts *postsEditOptions) string {
	// Priority 1: message-file
	if opts.messageFile != "" {
		filePath, err := config.ExpandPath(opts.messageFile)
		if err == nil {
			if content, err := os.ReadFile(filePath); err == nil {
				return strings.TrimSpace(string(content))
			}
		}
	}

	// Priority 2: piped stdin (text only)
	if isPipedInput() {
		if content, err := io.ReadAll(cmd.InOrStdin()); err == nil {
			return strings.TrimSpace(string(content))
		}
	}

	// Priority 3: --message flag
	if opts.message != "" {
		return opts.message
	}

	// Priority 4: args
	if len(args) > 0 {
		return strings.Join(args, " ")
	}

	return ""
}
