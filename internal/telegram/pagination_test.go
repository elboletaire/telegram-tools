package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestDialogsDone(t *testing.T) {
	tests := []struct {
		name         string
		resp         tg.MessagesDialogsClass
		totalFetched int
		newInBatch   int
		want         bool
	}{
		{name: "full list in one response", resp: &tg.MessagesDialogs{}, totalFetched: 30, newInBatch: 30, want: true},
		{name: "slice with more to fetch", resp: &tg.MessagesDialogsSlice{Count: 250}, totalFetched: 100, newInBatch: 100, want: false},
		{name: "slice reached its count", resp: &tg.MessagesDialogsSlice{Count: 250}, totalFetched: 250, newInBatch: 50, want: true},
		{name: "page repeats already seen dialogs", resp: &tg.MessagesDialogsSlice{Count: 9999}, totalFetched: 300, newInBatch: 0, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dialogsDone(tt.resp, tt.totalFetched, tt.newInBatch); got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBuildInputPeer_Users(t *testing.T) {
	users := []tg.UserClass{&tg.User{ID: 7, AccessHash: 99}}

	got := buildInputPeer(&tg.PeerUser{UserID: 7}, nil, users)

	u, ok := got.(*tg.InputPeerUser)
	if !ok || u.UserID != 7 || u.AccessHash != 99 {
		t.Fatalf("expected InputPeerUser{7, 99}, got %#v", got)
	}
}

func TestFindMessageDate_ServiceMessages(t *testing.T) {
	messages := []tg.MessageClass{&tg.MessageService{ID: 5, Date: 1234}}

	if got := findMessageDate(messages, 5); got != 1234 {
		t.Errorf("expected 1234, got %d", got)
	}
}

func TestTakeWithinLimit(t *testing.T) {
	tests := []struct {
		name                string
		batch, fetched, max int
		want                int
	}{
		{name: "no limit", batch: 100, fetched: 0, max: 0, want: 100},
		{name: "limit smaller than first page", batch: 100, fetched: 0, max: 3, want: 3},
		{name: "limit reached mid page", batch: 100, fetched: 100, max: 150, want: 50},
		{name: "limit already reached", batch: 100, fetched: 150, max: 150, want: 0},
		{name: "page within limit", batch: 50, fetched: 0, max: 100, want: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := takeWithinLimit(tt.batch, tt.fetched, tt.max); got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}
