package telegram

import (
	"testing"
)

func TestSendMessageRequest_NoWebpage(t *testing.T) {
	req := SendMessageRequest{
		ChatId:    "@testchannel",
		Message:   "Check https://example.com",
		ParseMode: "",
		Silent:    false,
		NoWebpage: true,
	}

	if !req.NoWebpage {
		t.Errorf("Expected NoWebpage to be true, got false")
	}
}

func TestSendMessageRequest_WithWebpage(t *testing.T) {
	req := SendMessageRequest{
		ChatId:    "@testchannel",
		Message:   "Check https://example.com",
		ParseMode: "",
		Silent:    false,
		NoWebpage: false,
	}

	if req.NoWebpage {
		t.Errorf("Expected NoWebpage to be false, got true")
	}
}

func TestEditMessageRequest_NoWebpage(t *testing.T) {
	req := EditMessageRequest{
		ChatId:    "@testchannel",
		PostId:    42,
		Message:   "Updated https://example.com",
		ParseMode: "",
		Silent:    false,
		NoWebpage: true,
	}

	if !req.NoWebpage {
		t.Errorf("Expected NoWebpage to be true, got false")
	}
}

func TestEditMessageRequest_WithWebpage(t *testing.T) {
	req := EditMessageRequest{
		ChatId:    "@testchannel",
		PostId:    42,
		Message:   "Updated https://example.com",
		ParseMode: "",
		Silent:    false,
		NoWebpage: false,
	}

	if req.NoWebpage {
		t.Errorf("Expected NoWebpage to be false, got true")
	}
}
