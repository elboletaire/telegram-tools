package telegram

import (
	"fmt"
	"reflect"
	"unicode/utf16"

	"github.com/gotd/td/tg"
)

// MaxMessageLength is Telegram's limit for a text message, in UTF-16 code units
// of plaintext (entities do not count towards it).
const MaxMessageLength = 4096

// MessageChunk is one Telegram message worth of plaintext plus its entities.
type MessageChunk struct {
	Text     string
	Entities []tg.MessageEntityClass
}

// splitSeparators are tried in order of preference when looking for a cut point.
var splitSeparators = [][]uint16{
	utf16.Encode([]rune("\n\n")),
	utf16.Encode([]rune("\n")),
	utf16.Encode([]rune(" ")),
}

// SplitMessage splits parsed plaintext into chunks of at most limit UTF-16 code
// units. It prefers cutting at paragraph breaks, then line breaks, then spaces,
// avoiding positions inside entities when possible. Entities are clipped to each
// chunk and their offsets rebased, so formatting survives any cut.
func SplitMessage(text string, entities []tg.MessageEntityClass, limit int) []MessageChunk {
	units := utf16.Encode([]rune(text))
	if limit <= 0 || len(units) <= limit {
		return []MessageChunk{{Text: text, Entities: entities}}
	}

	var chunks []MessageChunk
	start := 0
	for start < len(units) {
		end, next := len(units), len(units)
		if end-start > limit {
			end, next = findCut(units, entities, start, start+limit)
		}
		chunks = append(chunks, MessageChunk{
			Text:     string(utf16.Decode(units[start:end])),
			Entities: clipEntities(entities, start, end),
		})
		start = next
	}
	return chunks
}

// findCut returns where the chunk starting at start should end (at most max) and
// where the next chunk should begin, skipping the separator whitespace.
func findCut(units []uint16, entities []tg.MessageEntityClass, start, max int) (end, next int) {
	for _, allowInside := range []bool{false, true} {
		for _, sep := range splitSeparators {
			for p := max; p > start; p-- {
				if !hasPrefixAt(units, p, sep) {
					continue
				}
				if !allowInside && insideEntity(entities, p) {
					continue
				}
				return p, skipWhitespace(units, p)
			}
		}
	}

	// No separator found: hard cut, without splitting a surrogate pair.
	end = max
	if utf16.IsSurrogate(rune(units[end-1])) && units[end-1] < 0xdc00 {
		end--
	}
	return end, end
}

func hasPrefixAt(units []uint16, p int, sep []uint16) bool {
	if p+len(sep) > len(units) {
		return false
	}
	for i, u := range sep {
		if units[p+i] != u {
			return false
		}
	}
	return true
}

func skipWhitespace(units []uint16, p int) int {
	for p < len(units) && (units[p] == '\n' || units[p] == ' ') {
		p++
	}
	return p
}

// insideEntity reports whether cutting at p would split an entity.
func insideEntity(entities []tg.MessageEntityClass, p int) bool {
	for _, e := range entities {
		if e.GetOffset() < p && p < e.GetOffset()+e.GetLength() {
			return true
		}
	}
	return false
}

// clipEntities returns copies of the entities overlapping [start, end), clipped
// to that range and rebased so offsets are relative to start.
func clipEntities(entities []tg.MessageEntityClass, start, end int) []tg.MessageEntityClass {
	var out []tg.MessageEntityClass
	for _, e := range entities {
		s := max(e.GetOffset(), start)
		en := min(e.GetOffset()+e.GetLength(), end)
		if s >= en {
			continue
		}
		if clipped := withRange(e, s-start, en-s); clipped != nil {
			out = append(out, clipped)
		}
	}
	return out
}

// withRange returns a copy of e with a new range. Every tg.MessageEntity* type
// stores its range in Offset and Length fields, so any entity produced by the
// markdown or HTML parsers can be clipped.
func withRange(e tg.MessageEntityClass, offset, length int) tg.MessageEntityClass {
	src := reflect.ValueOf(e)
	if src.Kind() != reflect.Pointer || src.IsNil() || src.Elem().Kind() != reflect.Struct {
		return nil
	}
	dst := reflect.New(src.Elem().Type())
	dst.Elem().Set(src.Elem())
	off, ln := dst.Elem().FieldByName("Offset"), dst.Elem().FieldByName("Length")
	if off.Kind() != reflect.Int || ln.Kind() != reflect.Int {
		return nil
	}
	off.SetInt(int64(offset))
	ln.SetInt(int64(length))
	clipped, _ := dst.Interface().(tg.MessageEntityClass)
	return clipped
}

// PrepareMessage turns a message into the chunks to send: markdown or HTML is
// parsed into entities when parseMode is "MarkdownV2" or "HTML", and the result
// is split at Telegram's message length limit.
func PrepareMessage(message, parseMode string) ([]MessageChunk, error) {
	var entities []tg.MessageEntityClass
	switch parseMode {
	case "MarkdownV2":
		parsedText, parsedEntities, err := ParseMarkdownV2(message)
		if err != nil {
			return nil, fmt.Errorf("parse markdown: %w", err)
		}
		message = parsedText
		entities = parsedEntities
	case "HTML":
		parsedText, parsedEntities, err := ParseHTML(message)
		if err != nil {
			return nil, fmt.Errorf("parse html: %w", err)
		}
		message = parsedText
		entities = parsedEntities
	}

	return SplitMessage(message, entities, MaxMessageLength), nil
}
