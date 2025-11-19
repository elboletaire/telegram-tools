package config

import "strings"

var envReplacer = strings.NewReplacer(".", "_")

// EnvKeyReplacer exposes the replacer used for translating nested keys into
// environment variable friendly names.
func EnvKeyReplacer() *strings.Replacer {
	return envReplacer
}
