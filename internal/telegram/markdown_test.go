package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestParseMarkdownV2_BoldWrappedLink(t *testing.T) {
	input := "**[Sample Chapter 1](https://t.me/c/1111111111/8)**"
	expectedPlaintext := "Sample Chapter 1"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 2 {
		t.Fatalf("Expected 2 entities (bold + link), got %d", len(entities))
	}

	// Check for bold entity
	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityBold, got %T", entities[0])
	}
	if bold.Offset != 0 || bold.Length != 16 {
		t.Errorf("Bold entity: expected offset=0, length=16, got offset=%d, length=%d", bold.Offset, bold.Length)
	}

	// Check for link entity
	link, ok := entities[1].(*tg.MessageEntityTextURL)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityTextURL, got %T", entities[1])
	}
	if link.Offset != 0 || link.Length != 16 {
		t.Errorf("Link entity: expected offset=0, length=16, got offset=%d, length=%d", link.Offset, link.Length)
	}
	if link.URL != "https://t.me/c/1111111111/8" {
		t.Errorf("Link entity: expected URL %q, got %q", "https://t.me/c/1111111111/8", link.URL)
	}
}

func TestParseMarkdownV2_PlainLink(t *testing.T) {
	input := "[link](https://example.com)"
	expectedPlaintext := "link"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity (link), got %d", len(entities))
	}

	link, ok := entities[0].(*tg.MessageEntityTextURL)
	if !ok {
		t.Errorf("Expected entity to be MessageEntityTextURL, got %T", entities[0])
	}
	if link.URL != "https://example.com" {
		t.Errorf("Expected URL %q, got %q", "https://example.com", link.URL)
	}
}

func TestParseMarkdownV2_ItalicWrappedLink(t *testing.T) {
	input := "_[italic link](https://example.com)_"
	expectedPlaintext := "italic link"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 2 {
		t.Fatalf("Expected 2 entities (italic + link), got %d", len(entities))
	}

	// Check for italic entity
	italic, ok := entities[0].(*tg.MessageEntityItalic)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityItalic, got %T", entities[0])
	}
	if italic.Offset != 0 || italic.Length != 11 {
		t.Errorf("Italic entity: expected offset=0, length=11, got offset=%d, length=%d", italic.Offset, italic.Length)
	}

	// Check for link entity
	link, ok := entities[1].(*tg.MessageEntityTextURL)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityTextURL, got %T", entities[1])
	}
	if link.Offset != 0 || link.Length != 11 {
		t.Errorf("Link entity: expected offset=0, length=11, got offset=%d, length=%d", link.Offset, link.Length)
	}
	if link.URL != "https://example.com" {
		t.Errorf("Link entity: expected URL %q, got %q", "https://example.com", link.URL)
	}
}

func TestParseMarkdownV2_MixedFormatting(t *testing.T) {
	input := "This is **bold** test message with _italic_ text and `code`."
	expectedPlaintext := "This is bold test message with italic text and code."

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 3 {
		t.Fatalf("Expected 3 entities (bold, italic, code), got %d", len(entities))
	}

	// Check bold
	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityBold, got %T", entities[0])
	}
	if bold.Offset != 8 || bold.Length != 4 {
		t.Errorf("Bold entity: expected offset=8, length=4, got offset=%d, length=%d", bold.Offset, bold.Length)
	}

	// Check italic
	italic, ok := entities[1].(*tg.MessageEntityItalic)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityItalic, got %T", entities[1])
	}
	if italic.Offset != 31 || italic.Length != 6 {
		t.Errorf("Italic entity: expected offset=31, length=6, got offset=%d, length=%d", italic.Offset, italic.Length)
	}

	// Check code
	code, ok := entities[2].(*tg.MessageEntityCode)
	if !ok {
		t.Errorf("Expected third entity to be MessageEntityCode, got %T", entities[2])
	}
	if code.Offset != 47 || code.Length != 4 {
		t.Errorf("Code entity: expected offset=47, length=4, got offset=%d, length=%d", code.Offset, code.Length)
	}
}

func TestParseMarkdownV2_MultiLineWithEnDashBeforeEntity(t *testing.T) {
	// This tests the UTF-16 offset bug fix
	// En-dash (–) is 3 bytes in UTF-8 but 1 UTF-16 code unit
	input := "* **[Sample Chapter 1](https://t.me/c/1111111111/8)** (1–3)\n* **[Sample Part 2](https://t.me/c/1111111111/11)** (4–8)"
	expectedPlaintext := "* Sample Chapter 1 (1–3)\n* Sample Part 2 (4–8)"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	// Should have 4 entities: 2 bold + 2 links
	if len(entities) != 4 {
		t.Fatalf("Expected 4 entities (2 bold + 2 links), got %d", len(entities))
	}

	// First bold entity
	bold1, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityBold, got %T", entities[0])
	}
	if bold1.Offset != 2 || bold1.Length != 16 {
		t.Errorf("First bold entity: expected offset=2, length=16, got offset=%d, length=%d", bold1.Offset, bold1.Length)
	}

	// First link entity
	link1, ok := entities[1].(*tg.MessageEntityTextURL)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityTextURL, got %T", entities[1])
	}
	if link1.Offset != 2 || link1.Length != 16 {
		t.Errorf("First link entity: expected offset=2, length=16, got offset=%d, length=%d", link1.Offset, link1.Length)
	}

	// Second bold entity - this is where the UTF-16 bug would manifest
	// The string "* Sample Chapter 1 (1–3)\n* " has:
	// - Byte length: 29 (en-dash is 3 bytes)
	// - UTF-16 length: 27 (en-dash is 1 UTF-16 code unit)
	bold2, ok := entities[2].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected third entity to be MessageEntityBold, got %T", entities[2])
	}
	expectedOffset := 27 // UTF-16 code units, not bytes!
	expectedLength := 13 // "Sample Part 2"
	if bold2.Offset != expectedOffset {
		t.Errorf("Second bold entity: expected offset=%d (UTF-16), got %d", expectedOffset, bold2.Offset)
	}
	if bold2.Length != expectedLength {
		t.Errorf("Second bold entity: expected length=%d (UTF-16), got %d", expectedLength, bold2.Length)
	}

	// Second link entity
	link2, ok := entities[3].(*tg.MessageEntityTextURL)
	if !ok {
		t.Errorf("Expected fourth entity to be MessageEntityTextURL, got %T", entities[3])
	}
	if link2.Offset != expectedOffset || link2.Length != expectedLength {
		t.Errorf("Second link entity: expected offset=%d, length=%d, got offset=%d, length=%d",
			expectedOffset, expectedLength, link2.Offset, link2.Length)
	}
}

func TestParseMarkdownV2_EmojiBeforeBold(t *testing.T) {
	// This tests UTF-16 offset calculation with emoji
	// Wave emoji (👋) is 4 bytes in UTF-8 but 2 UTF-16 code units
	input := "👋 **bold text**"
	expectedPlaintext := "👋 bold text"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(entities))
	}

	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected entity to be MessageEntityBold, got %T", entities[0])
	}

	// "👋 " = 5 bytes (4 for emoji + 1 for space) but 3 UTF-16 units (2 for emoji + 1 for space)
	expectedOffset := 3  // UTF-16 code units
	expectedLength := 9  // "bold text" (ASCII, same in UTF-8 and UTF-16)

	if bold.Offset != expectedOffset {
		t.Errorf("Bold offset: expected %d (UTF-16), got %d", expectedOffset, bold.Offset)
	}
	if bold.Length != expectedLength {
		t.Errorf("Bold length: expected %d (UTF-16), got %d", expectedLength, bold.Length)
	}
}

func TestParseMarkdownV2_SingleLineBlockquote(t *testing.T) {
	input := "> This is a blockquote"
	expectedPlaintext := "This is a blockquote"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity (blockquote), got %d", len(entities))
	}

	blockquote, ok := entities[0].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Errorf("Expected entity to be MessageEntityBlockquote, got %T", entities[0])
	}

	if blockquote.Offset != 0 || blockquote.Length != 20 {
		t.Errorf("Blockquote entity: expected offset=0, length=20, got offset=%d, length=%d", blockquote.Offset, blockquote.Length)
	}

	if blockquote.Collapsed {
		t.Errorf("Expected Collapsed=false for regular blockquote, got true")
	}
}

func TestParseMarkdownV2_MultiLineBlockquote(t *testing.T) {
	input := "> First line\n> Second line\n> Third line"
	expectedPlaintext := "First line\nSecond line\nThird line"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity (multi-line blockquote), got %d", len(entities))
	}

	blockquote, ok := entities[0].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Errorf("Expected entity to be MessageEntityBlockquote, got %T", entities[0])
	}

	expectedLength := utf16Len("First line\nSecond line\nThird line")
	if blockquote.Offset != 0 || blockquote.Length != expectedLength {
		t.Errorf("Blockquote entity: expected offset=0, length=%d, got offset=%d, length=%d",
			expectedLength, blockquote.Offset, blockquote.Length)
	}

	if blockquote.Collapsed {
		t.Errorf("Expected Collapsed=false for regular blockquote, got true")
	}
}

func TestParseMarkdownV2_ExpandableBlockquote(t *testing.T) {
	input := ">> This is an expandable quote"
	expectedPlaintext := "This is an expandable quote"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity (expandable blockquote), got %d", len(entities))
	}

	blockquote, ok := entities[0].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Errorf("Expected entity to be MessageEntityBlockquote, got %T", entities[0])
	}

	if blockquote.Offset != 0 || blockquote.Length != 27 {
		t.Errorf("Blockquote entity: expected offset=0, length=27, got offset=%d, length=%d", blockquote.Offset, blockquote.Length)
	}

	if !blockquote.Collapsed {
		t.Errorf("Expected Collapsed=true for expandable blockquote, got false")
	}
}

func TestParseMarkdownV2_BlockquoteWithOtherFormatting(t *testing.T) {
	// Blockquotes must start at the beginning of a line (standard Markdown)
	input := "Normal text **bold**\n> a quote"
	expectedPlaintext := "Normal text bold\na quote"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	// Should have 2 entities: bold + blockquote
	if len(entities) != 2 {
		t.Fatalf("Expected 2 entities (bold + blockquote), got %d", len(entities))
	}

	// Check bold
	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityBold, got %T", entities[0])
	}
	if bold.Offset != 12 || bold.Length != 4 {
		t.Errorf("Bold entity: expected offset=12, length=4, got offset=%d, length=%d", bold.Offset, bold.Length)
	}

	// Check blockquote
	blockquote, ok := entities[1].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityBlockquote, got %T", entities[1])
	}
	if blockquote.Offset != 17 || blockquote.Length != 7 {
		t.Errorf("Blockquote entity: expected offset=17, length=7, got offset=%d, length=%d", blockquote.Offset, blockquote.Length)
	}
}

func TestParseMarkdownV2_BlockquoteWithNestedBold(t *testing.T) {
	input := "> This is **bold** in a quote"
	expectedPlaintext := "This is bold in a quote"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	// Should have 2 entities: blockquote + bold (nested)
	if len(entities) != 2 {
		t.Fatalf("Expected 2 entities (blockquote + bold), got %d", len(entities))
	}

	// Check blockquote
	blockquote, ok := entities[0].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityBlockquote, got %T", entities[0])
	}
	if blockquote.Offset != 0 || blockquote.Length != 23 {
		t.Errorf("Blockquote entity: expected offset=0, length=23, got offset=%d, length=%d", blockquote.Offset, blockquote.Length)
	}

	// Check nested bold
	bold, ok := entities[1].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityBold, got %T", entities[1])
	}
	if bold.Offset != 8 || bold.Length != 4 {
		t.Errorf("Bold entity: expected offset=8, length=4, got offset=%d, length=%d", bold.Offset, bold.Length)
	}
}

func TestParseMarkdownV2_BlockquoteWithMultipleNestedFormats(t *testing.T) {
	input := "> Quotes **_should_ also** allow recursive markdown"
	expectedPlaintext := "Quotes should also allow recursive markdown"

	plaintext, entities, err := ParseMarkdownV2(input)
	if err != nil {
		t.Fatalf("ParseMarkdownV2 failed: %v", err)
	}

	if plaintext != expectedPlaintext {
		t.Errorf("Expected plaintext %q, got %q", expectedPlaintext, plaintext)
	}

	// Should have 3 entities: blockquote + bold + italic (nested inside bold)
	if len(entities) != 3 {
		t.Fatalf("Expected 3 entities (blockquote + bold + italic), got %d", len(entities))
	}

	// Check blockquote
	blockquote, ok := entities[0].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Errorf("Expected first entity to be MessageEntityBlockquote, got %T", entities[0])
	}
	if blockquote.Offset != 0 || blockquote.Length != 43 {
		t.Errorf("Blockquote entity: expected offset=0, length=43, got offset=%d, length=%d", blockquote.Offset, blockquote.Length)
	}

	// Check nested bold
	bold, ok := entities[1].(*tg.MessageEntityBold)
	if !ok {
		t.Errorf("Expected second entity to be MessageEntityBold, got %T", entities[1])
	}
	if bold.Offset != 7 || bold.Length != 11 {
		t.Errorf("Bold entity: expected offset=7, length=11, got offset=%d, length=%d", bold.Offset, bold.Length)
	}

	// Check nested italic (inside bold)
	italic, ok := entities[2].(*tg.MessageEntityItalic)
	if !ok {
		t.Errorf("Expected third entity to be MessageEntityItalic, got %T", entities[2])
	}
	if italic.Offset != 7 || italic.Length != 6 {
		t.Errorf("Italic entity: expected offset=7, length=6, got offset=%d, length=%d", italic.Offset, italic.Length)
	}
}
