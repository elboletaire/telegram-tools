package chatslist

import (
	"bytes"
	"strings"
	"testing"

	"github.com/elboletaire/telegram-tools/internal/telegram"
)

func TestDisplay_PrintsPlainListWithoutTerminal(t *testing.T) {
	var out bytes.Buffer
	chats := []telegram.ChatInfo{
		{ID: 1111111111, Title: "Test channel", Username: "testchan", Type: "broadcast", Participants: 12},
		{ID: 22, Title: "Old group", Type: "group"},
	}

	if err := Display(strings.NewReader(""), &out, "", chats, 10); err != nil {
		t.Fatalf("Display failed: %v", err)
	}

	want := "1111111111\tbroadcast\t@testchan\tTest channel\n22\tgroup\t\tOld group\n"
	if out.String() != want {
		t.Errorf("expected:\n%q\ngot:\n%q", want, out.String())
	}
}
