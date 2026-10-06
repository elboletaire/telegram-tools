package postslist

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/elboletaire/telegram-tools/internal/telegram"
)

func TestDisplay_PrintsPlainListWithoutTerminal(t *testing.T) {
	var out bytes.Buffer
	posts := []telegram.PostInfo{
		{ID: 42, Date: time.Date(2026, 10, 6, 19, 5, 0, 0, time.UTC), MediaType: "photo", Caption: "first line\nsecond line"},
		{ID: 41, Date: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC), MediaType: "text", Caption: "hello"},
	}

	if err := Display(strings.NewReader(""), &out, "@chan", "", posts, 10); err != nil {
		t.Fatalf("Display failed: %v", err)
	}

	want := "42\t2026-10-06 19:05\tphoto\tfirst line second line\n41\t2026-10-05 08:00\ttext\thello\n"
	if out.String() != want {
		t.Errorf("expected:\n%q\ngot:\n%q", want, out.String())
	}
}

func TestSelect_RequiresTerminal(t *testing.T) {
	_, _, err := Select(strings.NewReader(""), &bytes.Buffer{}, "@chan", "", []telegram.PostInfo{{ID: 1}}, 10)
	if err == nil || !strings.Contains(err.Error(), "--post-id") {
		t.Fatalf("expected an error suggesting --post-id, got %v", err)
	}
}
