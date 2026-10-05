package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unreadable fails the test if the code under test reads from it, standing in
// for a stdin pipe that never closes (cron, CI runners, scripts).
type unreadable struct{ t *testing.T }

func (u unreadable) Read([]byte) (int, error) {
	u.t.Fatal("stdin must not be read when another message source is given")
	return 0, nil
}

func TestReadMessageInput_ExplicitSourcesWinOverStdin(t *testing.T) {
	file := filepath.Join(t.TempDir(), "msg.md")
	if err := os.WriteFile(file, []byte("from file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		in   messageInput
		want string
	}{
		{name: "file", in: messageInput{file: file, message: "flag", messageSet: true, args: []string{"arg"}}, want: "from file"},
		{name: "flag", in: messageInput{message: "flag", messageSet: true, args: []string{"arg"}}, want: "flag"},
		{name: "args", in: messageInput{args: []string{"hello", "world"}}, want: "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.stdin = unreadable{t}
			got, provided, err := readMessageInput(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !provided || got != tt.want {
				t.Errorf("expected (%q, true), got (%q, %v)", tt.want, got, provided)
			}
		})
	}
}

func TestReadMessageInput_ReadsPipedStdinAsLastResort(t *testing.T) {
	got, provided, err := readMessageInput(messageInput{stdin: strings.NewReader("  piped text\n")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !provided || got != "piped text" {
		t.Errorf("expected (%q, true), got (%q, %v)", "piped text", got, provided)
	}
}

func TestReadMessageInput_EmptyStdinIsNotAMessage(t *testing.T) {
	got, provided, err := readMessageInput(messageInput{stdin: strings.NewReader("  \n")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provided || got != "" {
		t.Errorf("expected no message, got (%q, %v)", got, provided)
	}
}

func TestReadMessageInput_IgnoresNonPipedStdin(t *testing.T) {
	devNull, err := os.Open(os.DevNull) // a character device, like a terminal
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	_, provided, err := readMessageInput(messageInput{stdin: devNull})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provided {
		t.Error("expected a non-piped stdin to provide no message")
	}
}

func TestReadMessageInput_ExplicitEmptyMessageCountsAsProvided(t *testing.T) {
	got, provided, err := readMessageInput(messageInput{messageSet: true, stdin: unreadable{t}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !provided || got != "" {
		t.Errorf("expected (\"\", true), got (%q, %v)", got, provided)
	}
}

func TestReadMessageInput_UnreadableFileIsAnError(t *testing.T) {
	_, _, err := readMessageInput(messageInput{file: filepath.Join(t.TempDir(), "missing.md"), stdin: unreadable{t}})
	if err == nil || !strings.Contains(err.Error(), "read message file") {
		t.Fatalf("expected a read message file error, got %v", err)
	}
}
