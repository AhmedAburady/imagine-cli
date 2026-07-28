// Package cli holds common-flag glue: the Options struct cobra binds the
// truly provider-agnostic flags onto, and provider-agnostic validation.
// Provider-specific flags (model, size, aspect ratio, quality, …) live
// inside each provider's BindFlags/ReadFlags.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/paths"
)

// Options holds the truly common CLI flags — same meaning for every
// provider. Everything provider-specific lives inside each provider's
// bundle and ends up in Request.Options.
type Options struct {
	// Prompts holds the raw -p values in order: text, file paths, or (alone) a batch file.
	Prompts []string
	// Separator is the raw --separator value, normalised by NormalizeSeparator.
	Separator string
	// Prompt is what Validate resolves Prompts to: parts read and joined, or the batch path.
	Prompt           string
	Output           string
	OutputFilename   string
	NumImages        int
	RefInputs        []string
	PreserveFilename bool

	// MaxParallel caps concurrent provider HTTP requests — covers both
	// single-shot per-count fan-out and batch per-entry fan-out via a
	// shared semaphore threaded into api.Params. 0 (default) means
	// unlimited, the pre-existing behaviour.
	MaxParallel int

	// EmbedMetadata embeds prompt/model/provider into PNG output (--embed-metadata).
	EmbedMetadata bool

	// IsBatch is set by Validate when the single -p resolves to a batch file
	// (.yaml/.yml/.json). Callers branch on this to call internal/batch
	// instead of building a single-shot Request.
	IsBatch bool
}

// IsBatchPath returns true if path's extension marks it as a batch
// file. Delegates to paths.IsBatchFile so cli and batch always agree.
func IsBatchPath(path string) bool {
	return paths.IsBatchFile(path)
}

// CommonFlagNames lists the truly provider-agnostic flag names — the
// long forms of the flags bound on the root command in commands/root.go.
// Single source of truth: both the single-shot validation gate and the
// batch path read from this map. Any flag not listed here must be
// claimed by at least one provider's Bundle.SupportedFlags.
var CommonFlagNames = map[string]bool{
	"prompt":         true,
	"separator":      true,
	"output":         true,
	"filename":       true,
	"count":          true,
	"input":          true,
	"replace":        true,
	"provider":       true,
	"max-parallel":   true,
	"embed-metadata": true,
	"help":           true,
	"version":        true,
}

// IsCommonFlag reports whether name is a common (provider-agnostic) flag.
func IsCommonFlag(name string) bool { return CommonFlagNames[name] }

// ResolvePromptText resolves a prompt value to literal text: a file path
// (relative to baseDir, "" = CWD) becomes its trimmed contents, else verbatim.
func ResolvePromptText(value, baseDir string) (string, error) {
	path := paths.ExpandTilde(value)
	if baseDir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return value, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %q: %v", path, err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("prompt file is empty: %s", path)
	}
	return text, nil
}

// DefaultSeparator is a blank line, so concatenated markdown sections don't weld together.
const DefaultSeparator = "\n\n"

// SeparatorFlagDefault is the escaped form, so --help prints (default "\n\n") on one line.
const SeparatorFlagDefault = `\n\n`

var separatorEscapes = strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\r`, "\r", `\\`, `\`)

// NormalizeSeparator defaults "" to DefaultSeparator, expands escapes, and puts an unpadded newline-free token on its own line ("---" → "\n---\n").
func NormalizeSeparator(raw string) string {
	if raw == "" {
		return DefaultSeparator
	}
	sep := separatorEscapes.Replace(raw)
	if !strings.Contains(sep, "\n") && strings.TrimSpace(sep) == sep {
		return "\n" + sep + "\n"
	}
	return sep
}

// ResolvePromptParts resolves each part through ResolvePromptText and joins them, dropping empty values.
func ResolvePromptParts(parts []string, separator, baseDir string) (string, error) {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		text, err := ResolvePromptText(part, baseDir)
		if err != nil {
			return "", err
		}
		texts = append(texts, text)
	}
	if len(texts) == 0 {
		return "", nil
	}
	return strings.Join(texts, NormalizeSeparator(separator)), nil
}

// Validate runs provider-agnostic checks:
//   - at least one non-empty -p (reading from a file if the value points at a path)
//   - tilde expansion in -o, -i
//   - -n is in range
//   - -i paths exist and contain supported images
//   - -f and -r are mutually exclusive (cobra also enforces)
//   - -r requires exactly one -i pointing at a single file.
func (opts *Options) Validate() error {
	// A batch file describes whole runs, so it can't be one part of a concatenation.
	batchPath := ""
	for _, part := range opts.Prompts {
		p := paths.ExpandTilde(part)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && IsBatchPath(p) {
			batchPath = p
			break
		}
	}
	switch {
	case batchPath != "" && len(opts.Prompts) > 1:
		return fmt.Errorf("batch file %s cannot be combined with other -p values", batchPath)
	case batchPath != "":
		opts.Prompt = batchPath // canonical path for the batch loader
		opts.IsBatch = true
	default:
		text, err := ResolvePromptParts(opts.Prompts, opts.Separator, "")
		if err != nil {
			return err
		}
		if text == "" {
			return fmt.Errorf("prompt is required (-p flag)")
		}
		opts.Prompt = text
	}

	opts.Output = paths.ExpandTilde(opts.Output)
	for i, ref := range opts.RefInputs {
		opts.RefInputs[i] = paths.ExpandTilde(ref)
	}

	if opts.NumImages < 1 || opts.NumImages > 20 {
		return fmt.Errorf("number of images must be between 1 and 20")
	}

	if opts.IsBatch {
		// Batch mode: -r is per-entry only; the rest of the common-flag
		// validation (-i existence, -f vs -r, single-input rule) runs
		// inside batch.Resolve so per-entry overrides win.
		if opts.PreserveFilename {
			return fmt.Errorf("--replace is not allowed in batch mode (set replace: true per entry instead)")
		}
		return nil
	}

	for _, ref := range opts.RefInputs {
		info, err := os.Stat(ref)
		if os.IsNotExist(err) {
			return fmt.Errorf("reference path does not exist: %s", ref)
		}
		if err != nil {
			return fmt.Errorf("cannot access reference path: %v", err)
		}
		if info.IsDir() {
			count, _ := images.CountInDir(ref)
			if count == 0 {
				return fmt.Errorf("no images found in reference directory: %s", ref)
			}
		} else if !images.IsSupported(ref) {
			return fmt.Errorf("unsupported image format: %s", ref)
		}
	}

	if opts.OutputFilename != "" && opts.PreserveFilename {
		return fmt.Errorf("-f and -r are mutually exclusive")
	}
	if opts.PreserveFilename {
		if len(opts.RefInputs) == 0 {
			return fmt.Errorf("-r flag requires -i with an input image file")
		}
		if len(opts.RefInputs) > 1 {
			return fmt.Errorf("-r flag only works with a single input file, not multiple")
		}
		info, _ := os.Stat(opts.RefInputs[0])
		if info != nil && info.IsDir() {
			return fmt.Errorf("-r flag only works with a single input file, not a folder")
		}
	}

	return nil
}
