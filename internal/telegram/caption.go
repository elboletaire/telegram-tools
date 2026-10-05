package telegram

import (
	"fmt"

	"github.com/gotd/td/tg"
)

// Telegram limits media captions, in UTF-16 code units. Premium accounts get a
// higher limit; bots can never be Premium.
const (
	MaxCaptionLength        = 1024
	MaxPremiumCaptionLength = 2048
)

// CheckCaptionLength validates a media caption before anything is uploaded.
// It returns an error when Telegram will certainly reject the caption, and a
// warning when it only fits within the Telegram Premium limit.
func CheckCaptionLength(caption string, bot bool) (warning string, err error) {
	n := utf16Len(caption)
	switch {
	case n <= MaxCaptionLength:
		return "", nil
	case bot:
		return "", fmt.Errorf("caption too long: %d/%d characters (bots can't use the Premium limit of %d)", n, MaxCaptionLength, MaxPremiumCaptionLength)
	case n > MaxPremiumCaptionLength:
		return "", fmt.Errorf("caption too long: %d characters, Telegram allows %d (%d with Premium)", n, MaxCaptionLength, MaxPremiumCaptionLength)
	default:
		return fmt.Sprintf("caption is %d characters: over the %d limit, it will only be accepted if the account has Telegram Premium (up to %d)", n, MaxCaptionLength, MaxPremiumCaptionLength), nil
	}
}

// prepareCaption parses a caption according to parseMode and checks the
// length of the result (markup doesn't count towards Telegram's limit),
// printing a warning when it needs Telegram Premium.
func (s *Service) prepareCaption(caption, parseMode string) (string, []tg.MessageEntityClass, error) {
	text, entities, err := ParseMessage(caption, parseMode)
	if err != nil {
		return "", nil, fmt.Errorf("caption: %w", err)
	}
	warning, err := CheckCaptionLength(text, s.cfg.Session.BotToken != "")
	if err != nil {
		return "", nil, err
	}
	if warning != "" && s.io.err != nil {
		fmt.Fprintf(s.io.err, "warning: %s\n", warning)
	}
	return text, entities, nil
}
