package telegram

import (
	"time"

	"github.com/gotd/td/tg"
)

// extractChatInfo extracts ChatInfo from a dialog and its associated data
func extractChatInfo(dialog *tg.Dialog, chats []tg.ChatClass, messages []tg.MessageClass) *ChatInfo {
	if dialog == nil {
		return nil
	}

	peer := dialog.Peer
	if peer == nil {
		return nil
	}

	var chatInfo ChatInfo
	chatInfo.Unread = dialog.UnreadCount

	// Find the last message date
	if dialog.TopMessage > 0 {
		chatInfo.LastDate = time.Unix(int64(findMessageDate(messages, dialog.TopMessage)), 0).UTC()
	}

	// Extract chat details based on peer type
	switch p := peer.(type) {
	case *tg.PeerChannel:
		channel := findChannel(chats, p.ChannelID)
		if channel == nil {
			return nil
		}
		chatInfo.ID = channel.ID
		chatInfo.AccessHash = channel.AccessHash
		chatInfo.Title = channel.Title
		chatInfo.Username = channel.Username

		if channel.Broadcast {
			chatInfo.Type = "broadcast"
		} else if channel.Megagroup {
			chatInfo.Type = "megagroup"
		} else {
			chatInfo.Type = "channel"
		}

		if channel.ParticipantsCount > 0 {
			chatInfo.Participants = channel.ParticipantsCount
		}

	case *tg.PeerChat:
		chat := findChat(chats, p.ChatID)
		if chat == nil {
			return nil
		}
		chatInfo.ID = chat.ID
		chatInfo.Title = chat.Title
		chatInfo.Type = "group"
		chatInfo.Participants = chat.ParticipantsCount

	case *tg.PeerUser:
		// Skip user chats for now, we're focused on channels/groups
		return nil

	default:
		return nil
	}

	return &chatInfo
}

// findChat finds a regular chat (group) by ID
func findChat(chats []tg.ChatClass, chatID int64) *tg.Chat {
	for _, ch := range chats {
		chat, ok := ch.(*tg.Chat)
		if !ok {
			continue
		}
		if chat.ID == chatID {
			return chat
		}
	}
	return nil
}
