package cmd

import (
	"fmt"
	"io"
	"os"
	"unicode/utf16"

	"golang.org/x/term"

	"github.com/elboletaire/ttools/internal/telegram"
)

// printDryRun shows what 'posts new' would send, one block per Telegram
// message (long messages are split exactly as they would be when sending).
func printDryRun(out io.Writer, chat string, messages []string, parseMode string, color bool) error {
	var chunks []telegram.MessageChunk
	for _, msg := range messages {
		prepared, err := telegram.PrepareMessage(msg, parseMode)
		if err != nil {
			return err
		}
		chunks = append(chunks, prepared...)
	}

	for i, chunk := range chunks {
		header := fmt.Sprintf("Message %d/%d · %d/%d chars · to %s", i+1, len(chunks), utf16Length(chunk.Text), telegram.MaxMessageLength, chat)
		printPreviewBlock(out, header, chunk, color)
	}

	noun := "messages"
	if len(chunks) == 1 {
		noun = "message"
	}
	fmt.Fprintf(out, "Dry run: %d %s would be sent to %s. Nothing was sent.\n", len(chunks), noun, chat)
	return nil
}

// printDryRunEdit shows what 'posts edit' would set as the new message text.
func printDryRunEdit(out io.Writer, chat string, postID int, message, parseMode string, color bool) error {
	chunks, err := telegram.PrepareMessage(message, parseMode)
	if err != nil {
		return err
	}
	if len(chunks) > 1 {
		return fmt.Errorf("message too long for an edit: it would need %d messages of up to %d characters", len(chunks), telegram.MaxMessageLength)
	}

	header := fmt.Sprintf("Edit of post #%d · %d/%d chars · in %s", postID, utf16Length(chunks[0].Text), telegram.MaxMessageLength, chat)
	printPreviewBlock(out, header, chunks[0], color)
	fmt.Fprintf(out, "Dry run: post #%d in %s would be edited. Nothing was sent.\n", postID, chat)
	return nil
}

func printPreviewBlock(out io.Writer, header string, chunk telegram.MessageChunk, color bool) {
	if color {
		fmt.Fprintf(out, "\x1b[90m── %s ──\x1b[39m\n", header)
	} else {
		fmt.Fprintf(out, "── %s ──\n", header)
	}
	fmt.Fprintf(out, "%s\n\n", telegram.RenderPreview(chunk, color))
}

// isTerminalWriter reports whether out is an interactive terminal, so previews
// only use ANSI styles when they will be rendered.
func isTerminalWriter(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func utf16Length(s string) int {
	return len(utf16.Encode([]rune(s)))
}
