package cmd

import (
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/elboletaire/ttools/internal/config"
)

var appState = &state{
	v: viper.New(),
}

// Execute runs the CLI entrypoint. version is shown by --version.
func Execute(version string) error {
	return newRootCommand(version).Execute()
}

type state struct {
	v        *viper.Viper
	cfgFile  string
	cfg      *config.Config
	initOnce sync.Once
	initErr  error
}

func newRootCommand(version string) *cobra.Command {
	root := &cobra.Command{
		Use:     "ttools",
		Short:   "Telegram tooling for channel posts and maintenance",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := appState.init(cmd); err != nil {
				return err
			}
			if cfg := appState.cfg; cfg != nil {
				cmd.SetContext(config.ContextWith(cmd.Context(), cfg))
			}
			return nil
		},
	}

	root.PersistentFlags().StringVar(&appState.cfgFile, "config", "", fmt.Sprintf("config file path (default %s)", config.DefaultConfigFile()))

	addPersistentConfigFlag(root.PersistentFlags(), "api-id", "Telegram API ID from my.telegram.org", "api.id")
	addPersistentConfigFlag(root.PersistentFlags(), "api-hash", "Telegram API hash from my.telegram.org", "api.hash")
	addPersistentConfigFlag(root.PersistentFlags(), "phone", "Phone number for Telegram login", "session.phone")
	addPersistentConfigFlag(root.PersistentFlags(), "password", "Two-factor authentication password (not login code)", "session.password")
	addPersistentConfigFlag(root.PersistentFlags(), "bot-token", "Bot token from @BotFather (alternative to phone auth)", "session.bot_token")
	addPersistentConfigFlag(root.PersistentFlags(), "session", fmt.Sprintf("Session file path (default %s)", config.DefaultSessionFile()), "session.file")
	addPersistentConfigFlag(root.PersistentFlags(), "chat", "Default target chat username or ID", "defaults.chat")
	addPersistentConfigFlag(root.PersistentFlags(), "thumb", "Default thumbnail for video uploads", "defaults.thumb")

	root.AddCommand(newPostsCommand())
	root.AddCommand(newChatsCommand())

	root.SilenceUsage = true
	root.SilenceErrors = true

	return root
}

func (s *state) init(cmd *cobra.Command) error {
	s.initOnce.Do(func() {
		s.initErr = s.loadConfig()
	})
	return s.initErr
}

func (s *state) loadConfig() error {
	v := s.v
	v.SetEnvPrefix("TTOOLS")
	v.SetEnvKeyReplacer(config.EnvKeyReplacer())
	v.AutomaticEnv()

	v.SetDefault("session.file", config.DefaultSessionFile())

	if s.cfgFile != "" {
		expanded, err := config.ExpandPath(s.cfgFile)
		if err != nil {
			return err
		}
		v.SetConfigFile(expanded)
	} else {
		v.SetConfigFile(config.DefaultConfigFile())
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}

	cfg := &config.Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return err
	}

	if err := cfg.ResolvePaths(); err != nil {
		return err
	}

	s.cfg = cfg
	return nil
}

func addPersistentConfigFlag(flags *pflag.FlagSet, name, usage, key string) {
	switch name {
	case "api-id":
		flags.Int(name, 0, usage)
	default:
		flags.String(name, "", usage)
	}

	if err := appState.v.BindPFlag(key, flags.Lookup(name)); err != nil {
		panic(fmt.Sprintf("binding flag %s: %v", name, err))
	}
}

func configFromContext(cmd *cobra.Command) (*config.Config, error) {
	cfg := config.FromContext(cmd.Context())
	if cfg == nil {
		return nil, fmt.Errorf("configuration is not initialized")
	}
	return cfg, nil
}
