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

// ResolveChannel returns the effective channel argument.
func (c *Config) ResolveChannel(provided string) (string, error) {
	channel := strings.TrimSpace(provided)
	if channel == "" {
		channel = c.Defaults.Channel
	}
	if channel == "" {
		return "", fmt.Errorf("channel is required (flag --channel or defaults.channel)")
	}
	return channel, nil
}
