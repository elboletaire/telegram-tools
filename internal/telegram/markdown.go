package telegram

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/gotd/td/tg"
)

var (
	// Bold content must not start or end with whitespace, so "2 ** 3 ** 4" stays literal.
	boldRegex       = regexp.MustCompile(`\*\*([^\s*](?:[^*]*[^\s*])?)\*\*`)
	inlineCodeRegex = regexp.MustCompile("`([^`]+)`")
	linkRegex       = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	// (?s) enables multiline code blocks; group 1 is language (optional), group 2 is the block content.
	codeBlockRegex = regexp.MustCompile("(?s)```([a-zA-Z0-9_+-]*)\\n?([\\s\\S]*?)```")
	// (?m) enables multiline mode; ^ matches line starts. Matches consecutive lines starting with >> or >
	expandableBlockquoteRegex = regexp.MustCompile(`(?m)^>>[ ]?([^\n]+(?:\n>>[ ]?[^\n]+)*)`)
	blockquoteRegex           = regexp.MustCompile(`(?m)^>[ ]?([^\n]+(?:\n>[ ]?[^\n]+)*)`)
	// Bare URLs are passed through untouched. "*" and "`" are excluded so that
	// **https://example.com** and `https://example.com` keep working.
	bareURLRegex = regexp.MustCompile("https?://[^\\s<>*`]+")
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
		case "url":
			b.WriteString(text[m.start:m.end])
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
	urls := bareURLSpans(text)
	candidates := []markdownMatch{
		findURL(urls, start),
		findMatch("codeblock", codeBlockRegex, text, start),
		findMatch("expandablequote", expandableBlockquoteRegex, text, start),
		findMatch("blockquote", blockquoteRegex, text, start),
		findMatch("inlinecode", inlineCodeRegex, text, start),
		findMatch("link", linkRegex, text, start),
		findMatch("bold", boldRegex, text, start),
		findItalic(text, start, urls),
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

// bareURLSpans returns the byte ranges of bare URLs in text. Trailing
// punctuation (and an unbalanced closing parenthesis) is not part of the URL.
func bareURLSpans(text string) [][2]int {
	var spans [][2]int
	for _, loc := range bareURLRegex.FindAllStringIndex(text, -1) {
		end := loc[1]
		for end > loc[0] {
			last := text[end-1]
			if strings.IndexByte(".,;:!?'\"_", last) >= 0 ||
				(last == ')' && strings.Count(text[loc[0]:end], "(") < strings.Count(text[loc[0]:end], ")")) {
				end--
				continue
			}
			break
		}
		spans = append(spans, [2]int{loc[0], end})
	}
	return spans
}

// findURL returns the first bare URL starting at or after start.
func findURL(spans [][2]int, start int) markdownMatch {
	for _, s := range spans {
		if s[0] >= start {
			return markdownMatch{kind: "url", start: s[0], end: s[1]}
		}
	}
	return markdownMatch{kind: "url", start: -1, end: -1}
}

func inSpans(i int, spans [][2]int) bool {
	for _, s := range spans {
		if i >= s[0] && i < s[1] {
			return true
		}
	}
	return false
}

// isWordRune reports whether r continues a word, so an adjacent "_" is part of
// an identifier (my_var, __init__) rather than an italic delimiter.
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// findItalic finds the next _italic_ span. Delimiters only count at word
// boundaries and never inside bare URLs, so identifiers and links are kept.
func findItalic(text string, start int, urls [][2]int) markdownMatch {
	canOpen := func(i int) bool {
		if i > 0 {
			if prev, _ := utf8.DecodeLastRuneInString(text[:i]); isWordRune(prev) {
				return false
			}
		}
		next, size := utf8.DecodeRuneInString(text[i+1:])
		return size > 0 && next != '_' && !unicode.IsSpace(next)
	}
	canClose := func(j int) bool {
		prev, _ := utf8.DecodeLastRuneInString(text[:j])
		if prev == '_' || unicode.IsSpace(prev) {
			return false
		}
		next, size := utf8.DecodeRuneInString(text[j+1:])
		return size == 0 || !isWordRune(next)
	}

	for i := strings.IndexByte(text[start:], '_'); i >= 0; {
		open := start + i
		if !inSpans(open, urls) && canOpen(open) {
			for j := open + 2; j < len(text); j++ {
				if text[j] == '_' && !inSpans(j, urls) && canClose(j) {
					return markdownMatch{
						kind:       "italic",
						start:      open,
						end:        j + 1,
						submatches: []int{open, j + 1, open + 1, j},
					}
				}
			}
		}
		next := strings.IndexByte(text[open+1:], '_')
		if next < 0 {
			break
		}
		i = open + 1 + next - start
	}
	return markdownMatch{kind: "italic", start: -1, end: -1}
}
