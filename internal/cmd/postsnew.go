package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elboletaire/ttools/internal/config"
	"github.com/elboletaire/ttools/internal/telegram"
)

type postsNewOptions struct {
	file      string
	html      bool
	plain     bool
	silent    bool
	delimiter string
}

func newPostsNewCommand(sharedOpts *sharedPostsOptions) *cobra.Command {
	opts := &postsNewOptions{
		delimiter: "[npost]",
	}

	cmd := &cobra.Command{
		Use:     "new [message...]",
		Aliases: []string{"post", "create"},
		Short:   "Send text messages to channel",
		Long: `Send text messages to the channel with MarkdownV2 formatting (default).

Messages are formatted as MarkdownV2 by default. Use standard markdown syntax:
  **bold**, _italic_, ` + "`code`" + `, ` + "```code block```" + `, [link](url)

Input sources (in priority order):
  1. --file flag: Read from file
  2. Stdin: Pipe content (e.g., cat file.md | ttools posts new)
  3. Arguments: Direct message text

Multiple messages:
  Separate messages with [npost] delimiter to send multiple messages.
  Example file:
    First message with **markdown**
    [npost]
    Second message with _formatting_

Examples:
  # Send a simple message
  ttools posts new "Hello **world**"

  # Send from file
  ttools posts new -f message.md

  # Pipe from stdin
  cat message.md | ttools posts new

  # Use HTML format instead
  ttools posts new "<b>Bold</b> text" --html

  # Send plain text (no formatting)
  ttools posts new "Plain text" --plain`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPostsNew(cmd.Context(), cmd, args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "Read messages from file")
	cmd.Flags().BoolVar(&opts.html, "html", false, "Use HTML format instead of MarkdownV2")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Send as plain text (no formatting)")
	cmd.Flags().BoolVar(&opts.silent, "silent", false, "Send without notification")
	cmd.Flags().StringVar(&opts.delimiter, "delimiter", opts.delimiter, "Message delimiter for batch sending")

	return cmd
}

func runPostsNew(ctx context.Context, cmd *cobra.Command, args []string, opts *postsNewOptions) error {
	out := cmd.OutOrStdout()

	cfg, err := configFromContext(cmd)
	if err != nil {
		return err
	}
	if err := cfg.RequireAPI(); err != nil {
		return err
	}

	chat, err := cfg.ResolveChat("")
	if err != nil {
		return err
	}

	// Determine parse mode
	parseMode := "MarkdownV2" // default
	if opts.html {
		parseMode = "HTML"
	} else if opts.plain {
		parseMode = ""
	}

	// Gather input from various sources
	var messages []string

	// Priority 1: Read from file
	if opts.file != "" {
		filePath, err := config.ExpandPath(opts.file)
		if err != nil {
			return fmt.Errorf("expand file path: %w", err)
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		messages = splitMessages(string(content), opts.delimiter)
	}

	// Priority 2: Read from stdin if piped
	if len(messages) == 0 && isPipedInput() {
		content, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		messages = splitMessages(string(content), opts.delimiter)
	}

	// Priority 3: Use positional arguments
	if len(messages) == 0 && len(args) > 0 {
		// Join all args as a single message, then split by delimiter
		combined := strings.Join(args, " ")
		messages = splitMessages(combined, opts.delimiter)
	}

	if len(messages) == 0 {
		return fmt.Errorf("no messages provided (use args, --file, or pipe from stdin)")
	}

	// Send messages with batch delay
	svc := telegram.NewService(cfg, telegram.WithIO(cmd.InOrStdin(), out, cmd.ErrOrStderr()))

	for i, msg := range messages {
		// Trim whitespace
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}

		req := telegram.SendMessageRequest{
			ChatId:    chat,
			Message:   msg,
			ParseMode: parseMode,
			Silent:    opts.silent,
		}

		messageID, err := svc.SendMessage(ctx, req)
		if err != nil {
			return fmt.Errorf("send message %d: %w", i+1, err)
		}

		fmt.Fprintf(out, "✓ Message sent (ID: %d)\n", messageID)

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

// isPipedInput checks if data is being piped into stdin
func isPipedInput() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}
