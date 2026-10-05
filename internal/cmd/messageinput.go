package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/elboletaire/telegram-tools/internal/config"
)

// messageInput holds every source a message (or caption) can come from.
type messageInput struct {
	file       string    // --message-file
	message    string    // --message
	messageSet bool      // --message was given, even if empty
	args       []string  // positional arguments
	stdin      io.Reader // read only when no other source is given
}

// readMessageInput returns the message and whether one was provided.
// Explicit sources win, in order: --message-file, --message, arguments. Stdin
// is the last resort and is only read when piped, so commands run from cron,
// CI or scripts never block waiting for input they don't need. Empty piped
// input counts as no message.
func readMessageInput(in messageInput) (string, bool, error) {
	if in.file != "" {
		path, err := config.ExpandPath(in.file)
		if err != nil {
			return "", false, fmt.Errorf("expand message file path: %w", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("read message file: %w", err)
		}
		return strings.TrimSpace(string(content)), true, nil
	}
	if in.messageSet || in.message != "" {
		return in.message, true, nil
	}
	if len(in.args) > 0 {
		return strings.Join(in.args, " "), true, nil
	}
	if in.stdin == nil || !isPipedInput(in.stdin) {
		return "", false, nil
	}
	content, err := io.ReadAll(in.stdin)
	if err != nil {
		return "", false, fmt.Errorf("read stdin: %w", err)
	}
	text := strings.TrimSpace(string(content))
	return text, text != "", nil
}

// isPipedInput reports whether r has data piped into it. Readers that aren't
// files (as in tests) are treated as piped.
func isPipedInput(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice == 0
}
