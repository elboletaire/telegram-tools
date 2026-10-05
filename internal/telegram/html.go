package telegram

import (
	"strings"

	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/html"
	"github.com/gotd/td/tg"
)

// ParseHTML converts Telegram-style HTML (as in the Bot API "HTML" parse mode)
// into plaintext plus Telegram entities. Unsupported tags are ignored.
func ParseHTML(text string) (string, []tg.MessageEntityClass, error) {
	var b entity.Builder
	if err := html.HTML(strings.NewReader(text), &b, html.Options{}); err != nil {
		return "", nil, err
	}
	plaintext, entities := b.Complete()
	return plaintext, entities, nil
}
