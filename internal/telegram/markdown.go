package telegram

import (
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/gotd/td/tg"
)

var (
	boldRegex       = regexp.MustCompile(`\*\*([^\*]+)\*\*`)
	italicRegex     = regexp.MustCompile(`_([^_]+)_`)
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	linkRegex       = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	// (?s) enables multiline code blocks; group 1 is language (optional), group 2 is the block content.
	codeBlockRegex = regexp.MustCompile("(?s)```([a-zA-Z0-9_+-]*)\\n?([\\s\\S]*?)```")
	// (?m) enables multiline mode; ^ matches line starts. Matches consecutive lines starting with >> or >
	expandableBlockquoteRegex = regexp.MustCompile(`(?m)^>>[ ]?([^\n]+(?:\n>>[ ]?[^\n]+)*)`)
	blockquoteRegex           = regexp.MustCompile(`(?m)^>[ ]?([^\n]+(?:\n>[ ]?[^\n]+)*)`)
)

// utf16Len returns the length of a string in UTF-16 code units.
// Telegram's Bot API requires entity offsets and lengths in UTF-16 code units, not bytes.
func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// processBlockquoteText removes > or >> prefixes from blockquote lines.
// Input: "> line 1\n> line 2" or ">> expandable\n>> quote"
// Output: "line 1\nline 2" or "expandable\nquote"
func processBlockquoteText(text string, expandable bool) string {
	prefix := "> "
	altPrefix := ">"
	if expandable {
		prefix = ">> "
		altPrefix = ">>"
	}

	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		// Try to remove prefix with space first, then without space
		trimmed := strings.TrimPrefix(line, prefix)
		if trimmed == line {
			trimmed = strings.TrimPrefix(line, altPrefix)
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

type markdownMatch struct {
	kind       string
	start      int
	end        int
	submatches []int
}

// ParseMarkdownV2 converts basic markdown into plaintext plus Telegram entities.
// Supported: **bold**, _italic_, `code`, ```code blocks```, [text](url), > blockquote, >> expandable quote.
// Supports nested formatting like **[bold link](url)**.
func ParseMarkdownV2(text string) (string, []tg.MessageEntityClass, error) {
	plaintext, entities := parseMarkdown(text, 0)
	return plaintext, entities, nil
}

// parseMarkdown is the recursive parser that handles nested entities.
func parseMarkdown(text string, baseOffset int) (string, []tg.MessageEntityClass) {
	var b strings.Builder
	var entities []tg.MessageEntityClass

	pos := 0
	for {
		m, ok := nextMatch(text, pos)
		if !ok {
			break
		}

		// Write text before the match as-is.
		b.WriteString(text[pos:m.start])

		switch m.kind {
		case "codeblock":
			langStart, langEnd := m.submatches[2], m.submatches[3]
			codeStart, codeEnd := m.submatches[4], m.submatches[5]
			lang := strings.TrimSpace(text[langStart:langEnd])
			code := text[codeStart:codeEnd]
			offset := baseOffset + utf16Len(b.String())

			entities = append(entities, &tg.MessageEntityPre{
				Offset:   offset,
				Length:   utf16Len(code),
				Language: lang,
			})
			b.WriteString(code)
		case "inlinecode":
			codeStart, codeEnd := m.submatches[2], m.submatches[3]
			code := text[codeStart:codeEnd]
			offset := baseOffset + utf16Len(b.String())

			entities = append(entities, &tg.MessageEntityCode{
				Offset: offset,
				Length: utf16Len(code),
			})
			b.WriteString(code)
		case "bold":
			textStart, textEnd := m.submatches[2], m.submatches[3]
			val := text[textStart:textEnd]
			offset := baseOffset + utf16Len(b.String())

			// Recursively parse content inside bold for nested formatting (e.g., links)
			innerPlaintext, innerEntities := parseMarkdown(val, offset)

			entities = append(entities, &tg.MessageEntityBold{
				Offset: offset,
				Length: utf16Len(innerPlaintext),
			})
			// Add any nested entities found inside the bold text
			entities = append(entities, innerEntities...)
			b.WriteString(innerPlaintext)
		case "italic":
			textStart, textEnd := m.submatches[2], m.submatches[3]
			val := text[textStart:textEnd]
			offset := baseOffset + utf16Len(b.String())

			// Recursively parse content inside italic for nested formatting
			innerPlaintext, innerEntities := parseMarkdown(val, offset)

			entities = append(entities, &tg.MessageEntityItalic{
				Offset: offset,
				Length: utf16Len(innerPlaintext),
			})
			// Add any nested entities found inside the italic text
			entities = append(entities, innerEntities...)
			b.WriteString(innerPlaintext)
		case "link":
			textStart, textEnd := m.submatches[2], m.submatches[3]
			urlStart, urlEnd := m.submatches[4], m.submatches[5]
			linkText := text[textStart:textEnd]
			url := text[urlStart:urlEnd]
			offset := baseOffset + utf16Len(b.String())

			entities = append(entities, &tg.MessageEntityTextURL{
				Offset: offset,
				Length: utf16Len(linkText),
				URL:    url,
			})
			b.WriteString(linkText)
		case "blockquote":
			// Extract the matched text and remove > prefixes
			quoteText := processBlockquoteText(text[m.start:m.end], false)
			offset := baseOffset + utf16Len(b.String())

			// Recursively parse content inside blockquote for nested formatting
			innerPlaintext, innerEntities := parseMarkdown(quoteText, offset)

			entities = append(entities, &tg.MessageEntityBlockquote{
				Offset:    offset,
				Length:    utf16Len(innerPlaintext),
				Collapsed: false,
			})
			// Add any nested entities found inside the blockquote
			entities = append(entities, innerEntities...)
			b.WriteString(innerPlaintext)
		case "expandablequote":
			// Extract the matched text and remove >> prefixes
			quoteText := processBlockquoteText(text[m.start:m.end], true)
			offset := baseOffset + utf16Len(b.String())

			// Recursively parse content inside expandable blockquote for nested formatting
			innerPlaintext, innerEntities := parseMarkdown(quoteText, offset)

			entities = append(entities, &tg.MessageEntityBlockquote{
				Offset:    offset,
				Length:    utf16Len(innerPlaintext),
				Collapsed: true,
			})
			// Add any nested entities found inside the expandable blockquote
			entities = append(entities, innerEntities...)
			b.WriteString(innerPlaintext)
		}

		pos = m.end
	}

	// Append any trailing text after the last match.
	b.WriteString(text[pos:])

	return b.String(), entities
}

// nextMatch finds the earliest markdown token after the provided position.
func nextMatch(text string, start int) (markdownMatch, bool) {
	candidates := []markdownMatch{
		findMatch("codeblock", codeBlockRegex, text, start),
		findMatch("expandablequote", expandableBlockquoteRegex, text, start),
		findMatch("blockquote", blockquoteRegex, text, start),
		findMatch("inlinecode", inlineCodeRegex, text, start),
		findMatch("link", linkRegex, text, start),
		findMatch("bold", boldRegex, text, start),
		findMatch("italic", italicRegex, text, start),
	}

	var best markdownMatch
	found := false

	for _, c := range candidates {
		if c.start == -1 {
			continue
		}
		if !found || c.start < best.start {
			best = c
			found = true
		}
	}

	return best, found
}

// findMatch executes a regex starting at the given position and returns the match metadata.
func findMatch(kind string, re *regexp.Regexp, text string, start int) markdownMatch {
	idx := re.FindStringSubmatchIndex(text[start:])
	if idx == nil {
		return markdownMatch{kind: kind, start: -1, end: -1}
	}

	adjusted := make([]int, len(idx))
	for i, v := range idx {
		adjusted[i] = v + start
	}

	return markdownMatch{
		kind:       kind,
		start:      adjusted[0],
		end:        adjusted[1],
		submatches: adjusted,
	}
}
