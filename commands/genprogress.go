package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/term"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/providers"
)

var (
	uiDim    = lipgloss.Color("#6C7086")
	uiCyan   = lipgloss.Color("#00B2C7")
	uiYellow = lipgloss.Color("#F2C94C")
	uiInk    = lipgloss.Color("#1A1A1A")

	uiPale      = lipgloss.NewStyle().Foreground(uiDim)
	uiPill      = lipgloss.NewStyle().Foreground(uiInk).Background(uiYellow).Bold(true).Padding(0, 1)
	uiCyanStyle = lipgloss.NewStyle().Foreground(uiCyan)
	uiHeader    = lipgloss.NewStyle().Foreground(uiCyan).Bold(true)
	uiAmber     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)

// stdoutIsTTY reports whether the live progress UI can take over stdout.
func stdoutIsTTY() bool { return term.IsTerminal(os.Stdout.Fd()) }

// paint applies a style only on a TTY, leaving piped output free of escape codes.
func paint(st lipgloss.Style, s string) string {
	if stdoutIsTTY() {
		return st.Render(s)
	}
	return s
}

// resultMsg carries one finished image; ok=false signals the channel closed.
type resultMsg struct {
	res api.GenerationResult
	ok  bool
}

// tickMsg drives the live elapsed clock so the view never looks frozen.
type tickMsg time.Time

func waitForResult(ch <-chan api.GenerationResult) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		return resultMsg{res: r, ok: ok}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type genProgressModel struct {
	header   string
	total    int
	done     int
	failed   int
	results  <-chan api.GenerationResult
	spinner  spinner.Model
	progress progress.Model
	cancel   context.CancelFunc
	start    time.Time
	elapsed  time.Duration
	finished bool
	aborted  bool
}

func newGenProgressModel(header string, total int, results <-chan api.GenerationResult, cancel context.CancelFunc) genProgressModel {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(uiYellow)
	p := progress.New(progress.WithDefaultBlend(), progress.WithWidth(28), progress.WithoutPercentage())
	return genProgressModel{
		header:   header,
		total:    total,
		results:  results,
		spinner:  s,
		progress: p,
		cancel:   cancel,
		start:    time.Now(),
	}
}

func (m genProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForResult(m.results), tickCmd())
}

func (m genProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			if m.cancel != nil {
				m.cancel()
			}
			m.finished = true
			m.aborted = true
			return m, tea.Quit
		}
	case tickMsg:
		m.elapsed = time.Since(m.start)
		return m, tickCmd()
	case resultMsg:
		if !msg.ok {
			m.finished = true
			return m, tea.Quit
		}
		if msg.res.Error != nil {
			m.failed++
		} else {
			m.done++
		}
		pct := float64(m.done+m.failed) / float64(m.total)
		return m, tea.Batch(m.progress.SetPercent(pct), waitForResult(m.results))
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m genProgressModel) View() tea.View {
	if m.finished {
		return tea.NewView("")
	}
	completed := m.done + m.failed
	line := fmt.Sprintf(" %s %s  %s  %s  %s",
		m.spinner.View(),
		paint(uiHeader, m.header),
		paint(uiPill, fmt.Sprintf("%d/%d", completed, m.total)),
		m.progress.View(),
		paint(uiPale, fmtDuration(m.elapsed)))
	return tea.NewView(line)
}

// fmtDuration renders an elapsed clock like "9s" or "3m14s".
func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int(d%time.Minute/time.Second))
}

// warnLine renders a non-blocking [WARNING] notice in the same amber tag style as [Aborted].
func warnLine(msg string) string {
	return " " + paint(uiAmber, "[WARNING]") + "  " + paint(uiPale, msg)
}

// abortBlock is the graceful summary shown when the user cancels mid-run.
func abortBlock(done, total int, elapsed, outputPath string) string {
	out := " " + paint(uiAmber, "[Aborted]") + "  " + paint(uiPale, fmt.Sprintf("%d/%d generated before cancel · %s", done, total, elapsed))
	if done > 0 {
		out += "\n  " + paint(uiCyanStyle, "→") + " " + paint(uiPale, outputPath)
	}
	return out
}

// resultsTable renders one row per image (IMAGE / MODEL / TIME / STATUS),
// matching the batch summary palette. Failures are detailed below the table.
func resultsTable(model string, results []api.GenerationResult, outputPath string) string {
	rows := append([]api.GenerationResult(nil), results...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Index < rows[j].Index })

	tty := stdoutIsTTY()
	border := lipgloss.NewStyle()
	header := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	cell := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	failed := cell
	if tty {
		border = border.Foreground(lipgloss.Color("8"))
		header = header.Bold(true).Foreground(lipgloss.Color("12"))
		failed = cell.Foreground(lipgloss.Color("183"))
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(border).
		BorderHeader(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return header
			}
			if row >= 0 && row < len(rows) && rows[row].Error != nil {
				return failed
			}
			return cell
		}).
		Headers("IMAGE", "MODEL", "TIME", "STATUS")

	for _, r := range rows {
		name, status := r.Filename, "ok"
		if r.Error != nil {
			name, status = fmt.Sprintf("image %d", r.Index+1), "failed"
		}
		t.Row(name, model, fmtDuration(r.Duration), status)
	}

	out := t.String()
	for _, r := range rows {
		if r.Error != nil {
			out += "\n  " + paint(uiPale, fmt.Sprintf("image %d: %v", r.Index+1, r.Error))
		}
	}
	return out + "\n  " + paint(uiCyanStyle, "→") + " " + paint(uiPale, outputPath)
}

// runWithProgress runs the generation with a live UI on a TTY (or plain lines when piped),
// returning the output and whether the user aborted (Ctrl+C / esc / q, or a cancelled context).
func runWithProgress(ctx context.Context, header string, provider providers.Provider, req providers.Request, params *api.Params) (api.GenerationOutput, bool) {
	if !stdoutIsTTY() {
		output := api.RunGeneration(ctx, provider, req, *params)
		return output, ctx.Err() != nil
	}

	progressCh := make(chan api.GenerationResult, params.NumImages)
	params.Progress = progressCh

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan api.GenerationOutput, 1)
	go func() { done <- api.RunGeneration(runCtx, provider, req, *params) }()

	m := newGenProgressModel(header, params.NumImages, progressCh, cancel)
	finalModel, err := tea.NewProgram(m).Run()

	aborted := ctx.Err() != nil
	if fm, ok := finalModel.(genProgressModel); ok && fm.aborted {
		aborted = true
	}
	if err != nil && !aborted {
		fmt.Fprintln(os.Stderr, "progress UI error:", err)
	}
	return <-done, aborted
}
