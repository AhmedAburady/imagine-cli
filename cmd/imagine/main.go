// Command imagine is a CLI for generating and editing images via Gemini,
// Vertex, or OpenAI.
package main

import (
	"context"
	"log/slog"
	"os"
	"runtime/debug"
	"syscall"

	"charm.land/fang/v2"

	"github.com/AhmedAburady/imagine-cli/commands"
	// providers/all blank-imports every built-in provider, triggering each
	// one's init() to self-register. Adding a provider = edit providers/all,
	// never this file.
	_ "github.com/AhmedAburady/imagine-cli/providers/all"
)

// version is set at build time via: -ldflags "-X main.version=v0.1.0".
// When built without ldflags (e.g. `go install …@latest`), we fall back
// to the module version Go records in the binary's BuildInfo.
var version = "dev"

// resolveVersion returns the injected ldflags version when set, otherwise
// the module version Go stamps into BuildInfo (what `go install pkg@vX.Y`
// records). Falls back to "dev" for `go build` in a dirty tree.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func main() {
	// Silence default slog so nothing in the codebase leaks log lines.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	// Peek --provider + config before fang renders help, so flag visibility
	// and the provider cheatsheet reflect the right provider.
	hint := commands.ProviderHintFromArgs(os.Args[1:])
	v := resolveVersion()
	root := commands.NewRootCmd(v, hint)
	if err := fang.Execute(context.Background(), root,
		fang.WithVersion(v),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}
