package telegram

import (
	"strings"
	"testing"

	"github.com/gotd/td/tg"
)

func chunkTexts(chunks []MessageChunk) []string {
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Text
	}
	return texts
}

func assertTexts(t *testing.T, got []MessageChunk, want []string) {
	t.Helper()
	texts := chunkTexts(got)
	if len(texts) != len(want) {
		t.Fatalf("expected %d chunks %q, got %d chunks %q", len(want), want, len(texts), texts)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("chunk %d: expected %q, got %q", i, want[i], texts[i])
		}
	}
}

func TestSplitMessage_ShortMessageIsUntouched(t *testing.T) {
	entities := []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: 5}}

	chunks := SplitMessage("hello world", entities, 4096)

	assertTexts(t, chunks, []string{"hello world"})
	if len(chunks[0].Entities) != 1 {
		t.Fatalf("expected entities to be kept, got %d", len(chunks[0].Entities))
	}
}

func TestSplitMessage_PrefersParagraphBreaks(t *testing.T) {
	chunks := SplitMessage("aaaa\n\nbbbb\n\ncccc", nil, 10)

	assertTexts(t, chunks, []string{"aaaa\n\nbbbb", "cccc"})
}

func TestSplitMessage_PrefersLineBreakOverLaterSpace(t *testing.T) {
	chunks := SplitMessage("aaaa\nbbbb cccc", nil, 10)

	assertTexts(t, chunks, []string{"aaaa", "bbbb cccc"})
}

func TestSplitMessage_AvoidsCuttingInsideEntities(t *testing.T) {
	// "bb cc" is a link (offsets 3..8); the space inside it must not be used.
	entities := []tg.MessageEntityClass{&tg.MessageEntityTextURL{Offset: 3, Length: 5, URL: "https://example.com"}}

	chunks := SplitMessage("aa bb cc dd", entities, 7)

	assertTexts(t, chunks, []string{"aa", "bb cc", "dd"})
	if len(chunks[0].Entities) != 0 {
		t.Errorf("chunk 0: expected no entities, got %d", len(chunks[0].Entities))
	}
	if len(chunks[1].Entities) != 1 {
		t.Fatalf("chunk 1: expected 1 entity, got %d", len(chunks[1].Entities))
	}
	link, ok := chunks[1].Entities[0].(*tg.MessageEntityTextURL)
	if !ok {
		t.Fatalf("chunk 1: expected MessageEntityTextURL, got %T", chunks[1].Entities[0])
	}
	if link.Offset != 0 || link.Length != 5 || link.URL != "https://example.com" {
		t.Errorf("chunk 1: expected link offset=0 length=5, got offset=%d length=%d url=%q", link.Offset, link.Length, link.URL)
	}
	if len(chunks[2].Entities) != 0 {
		t.Errorf("chunk 2: expected no entities, got %d", len(chunks[2].Entities))
	}
}

func TestSplitMessage_HardCutClipsEntitiesToEachChunk(t *testing.T) {
	entities := []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: 10}}

	chunks := SplitMessage("abcdefghij", entities, 4)

	assertTexts(t, chunks, []string{"abcd", "efgh", "ij"})
	wantLengths := []int{4, 4, 2}
	for i, c := range chunks {
		if len(c.Entities) != 1 {
			t.Fatalf("chunk %d: expected 1 entity, got %d", i, len(c.Entities))
		}
		bold, ok := c.Entities[0].(*tg.MessageEntityBold)
		if !ok {
			t.Fatalf("chunk %d: expected MessageEntityBold, got %T", i, c.Entities[0])
		}
		if bold.Offset != 0 || bold.Length != wantLengths[i] {
			t.Errorf("chunk %d: expected bold offset=0 length=%d, got offset=%d length=%d", i, wantLengths[i], bold.Offset, bold.Length)
		}
	}
	// The input entity must not be mutated.
	if b := entities[0].(*tg.MessageEntityBold); b.Offset != 0 || b.Length != 10 {
		t.Errorf("input entity was mutated: offset=%d length=%d", b.Offset, b.Length)
	}
}

func TestSplitMessage_NeverSplitsSurrogatePairs(t *testing.T) {
	// Each emoji is 2 UTF-16 code units; a limit of 3 would land mid-pair.
	chunks := SplitMessage("😀😀😀", nil, 3)

	assertTexts(t, chunks, []string{"😀", "😀", "😀"})
}

func TestSplitMessage_RebasesEntitiesAfterTrimmedSeparator(t *testing.T) {
	// "bbbb" is bold at offset 6 in the original; after splitting on "\n\n"
	// it must start at offset 0 of the second chunk.
	entities := []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 6, Length: 4}}

	chunks := SplitMessage("aaaa\n\nbbbb", entities, 5)

	assertTexts(t, chunks, []string{"aaaa", "bbbb"})
	if len(chunks[1].Entities) != 1 {
		t.Fatalf("chunk 1: expected 1 entity, got %d", len(chunks[1].Entities))
	}
	bold := chunks[1].Entities[0].(*tg.MessageEntityBold)
	if bold.Offset != 0 || bold.Length != 4 {
		t.Errorf("chunk 1: expected bold offset=0 length=4, got offset=%d length=%d", bold.Offset, bold.Length)
	}
}

func TestPrepareMessage_ParsesMarkdownThenSplitsAtTelegramLimit(t *testing.T) {
	// 3000 + 2 + 2000 = 5002 plaintext units, over the 4096 limit; the
	// paragraph break is the cut point and the bold must land in chunk 2.
	message := strings.Repeat("a", 3000) + "\n\n**" + strings.Repeat("b", 2000) + "**"

	chunks, err := PrepareMessage(message, "MarkdownV2")
	if err != nil {
		t.Fatalf("PrepareMessage failed: %v", err)
	}

	assertTexts(t, chunks, []string{strings.Repeat("a", 3000), strings.Repeat("b", 2000)})
	if len(chunks[1].Entities) != 1 {
		t.Fatalf("chunk 1: expected 1 entity, got %d", len(chunks[1].Entities))
	}
	bold, ok := chunks[1].Entities[0].(*tg.MessageEntityBold)
	if !ok || bold.Offset != 0 || bold.Length != 2000 {
		t.Errorf("chunk 1: expected bold offset=0 length=2000, got %#v", chunks[1].Entities[0])
	}
}

func TestPrepareMessage_PlainModeKeepsMarkdownSyntax(t *testing.T) {
	chunks, err := PrepareMessage("**x**", "")
	if err != nil {
		t.Fatalf("PrepareMessage failed: %v", err)
	}

	assertTexts(t, chunks, []string{"**x**"})
	if len(chunks[0].Entities) != 0 {
		t.Errorf("expected no entities in plain mode, got %d", len(chunks[0].Entities))
	}
}
