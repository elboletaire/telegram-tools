package telegram

import (
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/gotd/td/tg"
)

const quotePrefix = "▎ "

// RenderPreview renders a chunk for terminal display. With color, entities are
// shown with ANSI styles; without it, only link targets and quote markers are
// added so the output stays readable when piped.
func RenderPreview(chunk MessageChunk, color bool) string {
	type span struct {
		index      int
		start, end int
		open       string
		close      string
		quote      bool
	}

	spans := make([]span, 0, len(chunk.Entities))
	for i, e := range chunk.Entities {
		open, close := entityMarkers(e, color)
		_, quote := e.(*tg.MessageEntityBlockquote)
		spans = append(spans, span{
			index: i,
			start: e.GetOffset(),
			end:   e.GetOffset() + e.GetLength(),
			open:  open,
			close: close,
			quote: quote,
		})
	}

	prefix := quotePrefix
	if color {
		prefix = "\x1b[90m▎\x1b[39m "
	}

	// A quote that starts exactly at p gets its prefix from its open marker.
	insideQuote := func(p int) bool {
		for _, s := range spans {
			if s.quote && s.start < p && p < s.end {
				return true
			}
		}
		return false
	}

	// Entities ending at the same position close in reverse opening order.
	closing := make([]span, len(spans))
	copy(closing, spans)
	sort.SliceStable(closing, func(a, b int) bool {
		if closing[a].start != closing[b].start {
			return closing[a].start > closing[b].start
		}
		return closing[a].index > closing[b].index
	})

	var b strings.Builder
	emitBoundary := func(p int) {
		for _, s := range closing {
			if s.end == p && s.start < p {
				b.WriteString(s.close)
			}
		}
		for _, s := range spans {
			if s.start == p {
				b.WriteString(s.open)
			}
		}
	}

	p := 0
	for _, r := range chunk.Text {
		emitBoundary(p)
		b.WriteRune(r)
		p += len(utf16.Encode([]rune{r}))
		if r == '\n' && insideQuote(p) {
			b.WriteString(prefix)
		}
	}
	emitBoundary(p)

	return b.String()
}

func entityMarkers(e tg.MessageEntityClass, color bool) (open, close string) {
	switch v := e.(type) {
	case *tg.MessageEntityTextURL:
		if color {
			return "\x1b[4m", "\x1b[24m\x1b[90m <" + v.URL + ">\x1b[39m"
		}
		return "", " <" + v.URL + ">"
	case *tg.MessageEntityBlockquote:
		if color {
			return "\x1b[90m▎\x1b[39m ", ""
		}
		return quotePrefix, ""
	}

	if !color {
		return "", ""
	}
	switch e.(type) {
	case *tg.MessageEntityBold:
		return "\x1b[1m", "\x1b[22m"
	case *tg.MessageEntityItalic:
		return "\x1b[3m", "\x1b[23m"
	case *tg.MessageEntityCode, *tg.MessageEntityPre:
		return "\x1b[36m", "\x1b[39m"
	}
	return "", ""
}
