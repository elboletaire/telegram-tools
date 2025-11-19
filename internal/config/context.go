package config

import "context"

type contextKey struct{}

// ContextWith stores the configuration in the provided context.
func ContextWith(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext retrieves the Config from context when present.
func FromContext(ctx context.Context) *Config {
	if ctx == nil {
		return nil
	}
	cfg, _ := ctx.Value(contextKey{}).(*Config)
	return cfg
}
