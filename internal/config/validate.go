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
