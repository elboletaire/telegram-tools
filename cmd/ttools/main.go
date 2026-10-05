package main

import (
	"log"
	"runtime/debug"

	"github.com/elboletaire/telegram-tools/internal/cmd"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = ""

func main() {
	if err := cmd.Execute(buildVersion()); err != nil {
		log.Fatal(err)
	}
}

// buildVersion falls back to the module version embedded by `go install
// module@version`, so installs from source also report a real version.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
