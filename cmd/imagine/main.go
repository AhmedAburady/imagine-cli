// Command imagine is a CLI for generating and editing images via Gemini, Vertex, or OpenAI.
package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"

	"charm.land/fang/v2"

	"github.com/AhmedAburady/imagine-cli/commands"
)

// version is set at build time via: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	// Silence default slog so nothing in the codebase leaks log lines.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	root := commands.NewRootCmd(version)
	if err := fang.Execute(context.Background(), root,
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}
