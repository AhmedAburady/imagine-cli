// Command imagine is a CLI for generating and editing images via Gemini,
// Vertex, or OpenAI.
package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"

	"charm.land/fang/v2"

	"github.com/AhmedAburady/imagine-cli/commands"
	// Blank-import every provider. Each one's init() self-registers with the
	// providers registry; the CLI then discovers them by name. Adding a
	// provider means one new directory under providers/ and one line here.
	_ "github.com/AhmedAburady/imagine-cli/providers/gemini"
	_ "github.com/AhmedAburady/imagine-cli/providers/openai"
	_ "github.com/AhmedAburady/imagine-cli/providers/vertex"
)

// version is set at build time via: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	// Silence default slog so nothing in the codebase leaks log lines.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	// Peek --provider + config before fang renders help, so flag visibility
	// and the provider cheatsheet reflect the right provider.
	hint := commands.ProviderHintFromArgs(os.Args[1:])
	root := commands.NewRootCmd(version, hint)
	if err := fang.Execute(context.Background(), root,
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}
