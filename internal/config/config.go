package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultConfigName  = ".ttools.yaml"
	defaultSessionName = "session.json"
)

// Config represents the user configuration model loaded through Viper.
type Config struct {
	API      APIConfig      `mapstructure:"api"`
	Session  SessionConfig  `mapstructure:"session"`
	Defaults DefaultsConfig `mapstructure:"defaults"`
}

// APIConfig stores Telegram API credentials from apps.telegram.org
type APIConfig struct {
	ID   int    `mapstructure:"id"`
	Hash string `mapstructure:"hash"`
}

// SessionConfig stores MTProto session persistence and authentication details.
type SessionConfig struct {
	File     string `mapstructure:"file"`
	Phone    string `mapstructure:"phone"`
	Password string `mapstructure:"password"` // Optional 2FA password, NOT the login code
	BotToken string `mapstructure:"bot_token"` // Bot token from @BotFather (alternative to phone auth)
}

// DefaultsConfig hosts frequently reused flag values.
type DefaultsConfig struct {
	Chat  string `mapstructure:"chat"`
	Thumb string `mapstructure:"thumb"`
}

// ResolvePaths ensures configuration paths are expanded into absolute form.
func (c *Config) ResolvePaths() error {
	var err error
	if c.Session.File == "" {
		c.Session.File = DefaultSessionFile()
	}
	if c.Session.File, err = ExpandPath(c.Session.File); err != nil {
		return fmt.Errorf("expand session.file: %w", err)
	}

	if c.Defaults.Thumb != "" {
		if c.Defaults.Thumb, err = ExpandPath(c.Defaults.Thumb); err != nil {
			return fmt.Errorf("expand defaults.thumb: %w", err)
		}
	}

	c.Defaults.Chat = strings.TrimSpace(c.Defaults.Chat)
	return nil
}

// SessionFile returns the absolute path that stores MTProto session data.
func (c *Config) SessionFile() string {
	return c.Session.File
}

// DefaultConfigFile returns the user level configuration file.
func DefaultConfigFile() string {
	home := mustUserHome()
	return filepath.Join(home, defaultConfigName)
}

// DefaultSessionFile returns the preferred session storage path.
func DefaultSessionFile() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home := mustUserHome()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "ttools", defaultSessionName)
}

// ExpandPath resolves ~ and environment variables within a file path.
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	expanded := os.ExpandEnv(path)
	if expanded == "~" {
		return mustUserHome(), nil
	}

	if strings.HasPrefix(expanded, "~/") {
		return filepath.Join(mustUserHome(), expanded[2:]), nil
	}

	if strings.HasPrefix(expanded, "~") {
		return "", fmt.Errorf("cannot expand path %q", path)
	}

	if !filepath.IsAbs(expanded) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(cwd, expanded)
	}

	return filepath.Clean(expanded), nil
}

func mustUserHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("resolve home: %v", err))
	}
	return home
}
