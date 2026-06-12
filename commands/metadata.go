package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/internal/images"
)

func newMetadataCmd() *cobra.Command {
	var wantPrompt, wantModel, wantProvider, wantRef bool
	cmd := &cobra.Command{
		Use:   "metadata <image.png>...",
		Short: "Read embedded text metadata from PNG file(s)",
		Long: "Read embedded text (iTXt) metadata from PNG file(s) — including the\n" +
			"prompt/model/provider/reference_image fields written by --embed-metadata.\n\n" +
			"With no flags, prints all fields in a readable table. With one or more field\n" +
			"flags (--prompt/--model/--provider/--reference-image), prints only those raw\n" +
			"values — no labels or colour, one per line in that fixed order — for piping.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var keys []string // requested fields, in stable order
			if wantPrompt {
				keys = append(keys, "prompt")
			}
			if wantModel {
				keys = append(keys, "model")
			}
			if wantProvider {
				keys = append(keys, "provider")
			}
			if wantRef {
				keys = append(keys, "reference_image")
			}
			raw := len(keys) > 0

			var failed bool
			for i, path := range args {
				data, err := os.ReadFile(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", path, err) // stderr keeps stdout pipeable
					failed = true
					continue
				}
				tags, err := images.ReadPNGText(data)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
					failed = true
					continue
				}

				if raw {
					for _, k := range keys {
						fmt.Println(tagValue(tags, k)) // raw value, empty line if absent
					}
					continue
				}

				if len(args) > 1 {
					if i > 0 {
						fmt.Println()
					}
					fmt.Println(paint(uiHeader, path))
				}
				if len(tags) == 0 {
					fmt.Println(paint(uiPale, "  no embedded metadata"))
					continue
				}
				printTags(tags)
			}
			if failed {
				return fmt.Errorf("some files could not be read")
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVar(&wantPrompt, "prompt", false, "Print only the embedded prompt (raw, pipeable)")
	f.BoolVar(&wantModel, "model", false, "Print only the embedded model")
	f.BoolVar(&wantProvider, "provider", false, "Print only the embedded provider")
	f.BoolVar(&wantRef, "reference-image", false, "Print only the embedded reference image path(s)")
	return cmd
}

func tagValue(tags []images.TextTag, key string) string {
	for _, t := range tags {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}

func printTags(tags []images.TextTag) {
	width := 0
	for _, t := range tags {
		if len(t.Key) > width {
			width = len(t.Key)
		}
	}
	indent := strings.Repeat(" ", width+4) // 2 leading spaces + key column + 2-space gap
	for _, t := range tags {
		value := strings.ReplaceAll(t.Value, "\n", "\n"+indent)
		fmt.Printf("  %s  %s\n", paint(uiHeader, fmt.Sprintf("%-*s", width, t.Key)), value)
	}
}
