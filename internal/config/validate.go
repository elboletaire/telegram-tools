package config

import (
	"fmt"
	"strings"
)

// RequireAPI ensures the Telegram API credentials are available.
func (c *Config) RequireAPI() error {
	if c.API.ID == 0 {
		return fmt.Errorf("api.id is required (set via config, env or --api-id)")
	}
	if strings.TrimSpace(c.API.Hash) == "" {
		return fmt.Errorf("api.hash is required (set via config, env or --api-hash)")
	}
	return nil
}

// RequireAuth ensures at least one authentication method is configured.
func (c *Config) RequireAuth() error {
	hasBotToken := strings.TrimSpace(c.Session.BotToken) != ""
	hasPhone := strings.TrimSpace(c.Session.Phone) != ""

	if hasBotToken && hasPhone {
		return fmt.Errorf("cannot use both bot_token and phone authentication; choose one")
	}

	if !hasBotToken && !hasPhone {
		return fmt.Errorf("authentication required: provide either session.bot_token or session.phone")
	}

	return nil
}

// Requirements ensures all required session configuration is present (API credentials + authentication).
func (c *Config) Requirements() error {
	if err := c.RequireAPI(); err != nil {
		return err
	}
	if err := c.RequireAuth(); err != nil {
		return err
	}
	return nil
}

// ResolveChat returns the effective chat identifier argument.
func (c *Config) ResolveChat(provided string) (string, error) {
	chat := strings.TrimSpace(provided)
	if chat == "" {
		chat = c.Defaults.Chat
	}
	if chat == "" {
		return "", fmt.Errorf("chat is required (flag --chat or defaults.chat)")
	}
	return chat, nil
}
