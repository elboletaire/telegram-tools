package telegram

import (
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
)

func collectMessages(resp tg.MessagesMessagesClass) ([]*tg.Message, error) {
	switch m := resp.(type) {
	case *tg.MessagesMessages:
		return extractNonEmptyMessages(m.Messages), nil
	case *tg.MessagesMessagesSlice:
		return extractNonEmptyMessages(m.Messages), nil
	case *tg.MessagesChannelMessages:
		return extractNonEmptyMessages(m.Messages), nil
	default:
		return nil, fmt.Errorf("unsupported response type %T", resp)
	}
}

func extractNonEmptyMessages(items []tg.MessageClass) []*tg.Message {
	result := make([]*tg.Message, 0, len(items))
	for _, item := range items {
		msg, ok := item.AsNotEmpty()
		if !ok {
			continue
		}
		if _, isService := msg.(*tg.MessageService); isService {
			continue
		}
		if concrete, ok := msg.(*tg.Message); ok {
			result = append(result, concrete)
		}
	}
	return result
}

func describeMedia(media tg.MessageMediaClass) string {
	switch m := media.(type) {
	case nil, *tg.MessageMediaEmpty:
		return "text"
	case *tg.MessageMediaPhoto:
		return "photo"
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.AsNotEmpty(); ok {
			mt := strings.TrimSpace(doc.MimeType)
			if mt != "" {
				return mt
			}
		}
		return "document"
	case *tg.MessageMediaUnsupported:
		return "unsupported"
	case *tg.MessageMediaDice:
		return "dice"
	case *tg.MessageMediaContact:
		return "contact"
	case *tg.MessageMediaPoll:
		return "poll"
	case *tg.MessageMediaWebPage:
		return "webpage"
	default:
		return fmt.Sprintf("%T", m)
	}
}

func extractMediaFilename(media tg.MessageMediaClass) string {
	documentMedia, ok := media.(*tg.MessageMediaDocument)
	if !ok {
		return ""
	}
	document, ok := documentMedia.Document.AsNotEmpty()
	if !ok {
		return ""
	}
	for _, attr := range document.Attributes {
		if filename, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return strings.TrimSpace(filename.FileName)
		}
	}
	return ""
}
