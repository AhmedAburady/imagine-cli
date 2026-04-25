package batch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// EntryResult is one entry's outcome after running.
type EntryResult struct {
	Resolved Resolved
	Output   api.GenerationOutput
}

// Run fans every resolved entry out to api.RunGeneration in parallel,
// reports per-entry success/failure as a styled summary table, and
// returns a non-nil error if any image failed.
func Run(ctx context.Context, resolved []Resolved) error {
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Running %d batch entries...", len(resolved))
	_ = s.Color("magenta")
	s.Start()

	var wg sync.WaitGroup
	results := make([]EntryResult, len(resolved))
	startTime := time.Now()
	for i, r := range resolved {
		wg.Go(func() {
			out := api.RunGeneration(ctx, r.Provider, r.Request, r.Params)
			results[i] = EntryResult{Resolved: r, Output: out}
		})
	}
	wg.Wait()
	s.Stop()

	fmt.Println()
	totalFail := printSummary(results, time.Since(startTime))

	if path := commonOutputFolder(resolved); path != "" {
		if !filepath.IsAbs(path) {
			if abs, err := filepath.Abs(path); err == nil {
				path = abs
			}
		}
		fmt.Printf("\nOutput: %s\n", path)
	} else {
		fmt.Println("\nOutput: per-entry folders")
	}

	_ = os.Stdout.Sync()

	if totalFail > 0 {
		return fmt.Errorf("%d image(s) failed across batch", totalFail)
	}
	return nil
}

// --- Summary table (marina-style lipgloss) ----------------------------------
//
// Style palette mirrors marina/internal/ui/table.go: header bold cyan
// (12), borders dim grey (8), partial-success amber (214), all-failed
// muted mauve (183). Same colour semantics across both CLIs so users
// switching between tools see status the same way.

var (
	tblHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	tblBorderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	tblCellStyle    = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	tblFailedStyle  = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("183"))
	tblPartialStyle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(lipgloss.Color("214"))
)

// summaryRow holds the data for one table row plus its rendering state.
type summaryRow struct {
	entry    string
	provider string
	model    string
	images   string // "5/5" or "3/5"
	elapsed  string
	status   string
	state    rowState
}

type rowState int

const (
	stateAllOK rowState = iota
	statePartial
	stateAllFailed
)

// printSummary renders the marina-style table and returns the failed-
// image count so Run can build its return error.
func printSummary(results []EntryResult, elapsed time.Duration) int {
	var success, fail int
	rows := make([]summaryRow, 0, len(results))
	for _, er := range results {
		ok, bad := 0, 0
		for _, ir := range er.Output.Results {
			if ir.Error != nil {
				bad++
			} else {
				ok++
			}
		}
		success += ok
		fail += bad

		state := stateAllOK
		switch {
		case ok == 0:
			state = stateAllFailed
		case bad > 0:
			state = statePartial
		}

		statusText := "ok"
		switch state {
		case stateAllFailed:
			statusText = "failed"
		case statePartial:
			statusText = "partial"
		}

		rows = append(rows, summaryRow{
			entry:    er.Resolved.DisplayName,
			provider: er.Resolved.Provider.Info().Name,
			model:    requestLabel(er.Resolved.ProviderOpts),
			images:   fmt.Sprintf("%d/%d", ok, ok+bad),
			elapsed:  fmt.Sprintf("%.1fs", er.Output.Elapsed.Seconds()),
			status:   statusText,
			state:    state,
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(tblBorderStyle).
		BorderHeader(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tblHeaderStyle.PaddingLeft(1).PaddingRight(1)
			}
			if row >= 0 && row < len(rows) {
				switch rows[row].state {
				case stateAllFailed:
					return tblFailedStyle
				case statePartial:
					return tblPartialStyle
				}
			}
			return tblCellStyle
		}).
		Headers("ENTRY", "PROVIDER", "MODEL", "IMAGES", "TIME", "STATUS")

	for _, r := range rows {
		t.Row(r.entry, r.provider, r.model, r.images, r.elapsed, r.status)
	}

	fmt.Println(t.String())

	// Per-image failure detail goes below the table — the table tells
	// you which entry failed; this tells you why, without bloating the
	// table itself.
	for _, er := range results {
		var failures []api.GenerationResult
		for _, ir := range er.Output.Results {
			if ir.Error != nil {
				failures = append(failures, ir)
			}
		}
		if len(failures) == 0 {
			continue
		}
		fmt.Printf("\n%s — %d failure(s):\n", er.Resolved.DisplayName, len(failures))
		for _, ir := range failures {
			fmt.Printf("  Image %d: %v\n", ir.Index+1, ir.Error)
		}
	}

	fmt.Printf("\nDone: %d success, %d failed across %d entries (%.1fs)\n",
		success, fail, len(results), elapsed.Seconds())

	return fail
}

// commonOutputFolder returns the output folder if every entry shares
// the same one, else "". Used to avoid printing N copies of the same
// path at the end of a homogeneous batch run.
func commonOutputFolder(resolved []Resolved) string {
	if len(resolved) == 0 {
		return ""
	}
	folder := resolved[0].Params.OutputFolder
	for _, r := range resolved[1:] {
		if r.Params.OutputFolder != folder {
			return ""
		}
	}
	return folder
}

// requestLabel mirrors commands/run.go's requestLabel; duplicated to
// avoid an import cycle (commands → batch).
func requestLabel(opts any) string {
	if l, ok := opts.(providers.RequestLabeler); ok {
		return l.RequestLabel()
	}
	if m, ok := opts.(map[string]any); ok {
		if s, _ := m["model"].(string); s != "" {
			return s
		}
	}
	return ""
}
