// Package all blank-imports every built-in provider so the CLI entry point
// can pull them in with a single import. Adding a new provider means editing
// this file (one line) — never cmd/imagine/main.go.
//
// This file is the canonical list of "providers shipped with the binary."
// Custom downstream builds can import specific providers instead of the
// bundle if they want a narrower binary.
package all

import (
	_ "github.com/AhmedAburady/imagine-cli/providers/gemini"
	_ "github.com/AhmedAburady/imagine-cli/providers/openai"
	_ "github.com/AhmedAburady/imagine-cli/providers/vertex"
)
