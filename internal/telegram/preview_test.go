package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestRenderPreview_PlainShowsLinkTargets(t *testing.T) {
	chunk := MessageChunk{
		Text:     "see link now",
		Entities: []tg.MessageEntityClass{&tg.MessageEntityTextURL{Offset: 4, Length: 4, URL: "https://x.test"}},
	}

	got := RenderPreview(chunk, false)

	want := "see link <https://x.test> now"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderPreview_PlainPrefixesBlockquoteLines(t *testing.T) {
	chunk := MessageChunk{
		Text:     "intro\nq1\nq2\nend",
		Entities: []tg.MessageEntityClass{&tg.MessageEntityBlockquote{Offset: 6, Length: 5}},
	}

	got := RenderPreview(chunk, false)

	want := "intro\n▎ q1\n▎ q2\nend"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderPreview_ColorWrapsBold(t *testing.T) {
	chunk := MessageChunk{
		Text:     "a bold b",
		Entities: []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 2, Length: 4}},
	}

	got := RenderPreview(chunk, true)

	want := "a \x1b[1mbold\x1b[22m b"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderPreview_ColorClosesNestedEntitiesInReverseOrder(t *testing.T) {
	// **[x](u)** produces bold + link over the same range; the link must close
	// (and print its URL) before the bold closes.
	chunk := MessageChunk{
		Text: "x",
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: 1},
			&tg.MessageEntityTextURL{Offset: 0, Length: 1, URL: "u"},
		},
	}

	got := RenderPreview(chunk, true)

	want := "\x1b[1m\x1b[4mx\x1b[24m\x1b[90m <u>\x1b[39m\x1b[22m"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderPreview_OffsetsAreUTF16(t *testing.T) {
	// The emoji takes 2 UTF-16 units, so the link starts at offset 3, not 2.
	chunk := MessageChunk{
		Text:     "😀 go",
		Entities: []tg.MessageEntityClass{&tg.MessageEntityTextURL{Offset: 3, Length: 2, URL: "u"}},
	}

	got := RenderPreview(chunk, false)

	want := "😀 go <u>"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderPreview_ColorsHTMLOnlyEntities(t *testing.T) {
	chunk := MessageChunk{
		Text: "u s sp",
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityUnderline{Offset: 0, Length: 1},
			&tg.MessageEntityStrike{Offset: 2, Length: 1},
			&tg.MessageEntitySpoiler{Offset: 4, Length: 2},
		},
	}

	got := RenderPreview(chunk, true)
	want := "\x1b[4mu\x1b[24m \x1b[9ms\x1b[29m \x1b[7msp\x1b[27m"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
