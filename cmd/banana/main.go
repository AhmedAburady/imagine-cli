package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/AhmedAburady/banana-cli/api"
	"github.com/AhmedAburady/banana-cli/cli"
	"github.com/AhmedAburady/banana-cli/config"
	"github.com/AhmedAburady/banana-cli/describe"
	"github.com/AhmedAburady/banana-cli/ui"
	"github.com/AhmedAburady/banana-cli/views"
)

// ViewState represents the current view
type ViewState int

const (
	APIKeyView ViewState = iota
	MenuView
	GenerateView
	EditView
	ProcessingView
	ResultsView
)

// Model is the main application model
type Model struct {
	currentView   ViewState
	menuModel     ui.MenuModel
	apiKeyModel   views.APIKeyModel
	generateModel views.GenerateModel
	editModel     views.EditModel

	apiKey  string
	width   int
	height  int
	spinner spinner.Model

	// Processing state
	processingMsg string
	results       []api.GenerationResult
	outputFolder  string
	successCount  int
	errorCount    int
	elapsed       time.Duration
}

// ProcessingStartMsg signals the start of image generation
type ProcessingStartMsg struct {
	Config *api.Config
}

// ProcessingDoneMsg signals completion of image generation
type ProcessingDoneMsg struct {
	api.GenerationOutput
}

// NewModel creates a new application model
func NewModel(apiKey string) Model {
	menuStyles := ui.MenuStyles{
		Window:       ui.WindowStyle,
		Title:        ui.TitleStyle,
		Item:         ui.MenuItemStyle,
		SelectedItem: ui.MenuSelectedStyle,
		Help:         ui.HelpStyle,
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.DraculaPurple)

	// Start in APIKeyView if no API key, otherwise go to menu
	initialView := MenuView
	if apiKey == "" {
		initialView = APIKeyView
	}

	return Model{
		currentView: initialView,
		menuModel:   ui.NewMenuModel(menuStyles),
		apiKeyModel: views.NewAPIKeyModel(),
		apiKey:      apiKey,
		spinner:     s,
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.EnterAltScreen}

	if m.currentView == APIKeyView {
		cmds = append(cmds, m.apiKeyModel.Init())
	} else {
		cmds = append(cmds, m.menuModel.Init())
	}

	return tea.Batch(cmds...)
}

// Update handles all application messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Global: Ctrl+C quits
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// Handle window resize
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
		m.menuModel.SetSize(size.Width, size.Height)
	}

	// Route to current view
	switch m.currentView {
	case APIKeyView:
		return m.updateAPIKeyView(msg)

	case MenuView:
		return m.updateMenuView(msg)

	case GenerateView:
		return m.updateGenerateView(msg)

	case EditView:
		return m.updateEditView(msg)

	case ProcessingView:
		return m.updateProcessingView(msg)

	case ResultsView:
		return m.updateResultsView(msg)
	}

	return m, cmd
}

func (m Model) updateMenuView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle menu selection
	if sel, ok := msg.(ui.MenuSelectionMsg); ok {
		switch sel.Choice {
		case ui.GenerateImage:
			m.currentView = GenerateView
			m.generateModel = views.NewGenerateModel()
			return m, m.generateModel.Init()
		case ui.EditImage:
			m.currentView = EditView
			m.editModel = views.NewEditModel()
			return m, m.editModel.Init()
		}
	}

	var cmd tea.Cmd
	m.menuModel, cmd = m.menuModel.Update(msg)
	return m, cmd
}

func (m Model) updateGenerateView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle back to menu
	if _, ok := msg.(ui.BackToMenuMsg); ok {
		m.currentView = MenuView
		return m, nil
	}

	// Handle form submission
	if submit, ok := msg.(views.GenerateSubmitMsg); ok {
		modelName := api.ModelPro
		thinkingLevel := ""
		imageSearch := false
		if submit.Model == "flash" {
			modelName = api.ModelFlash
			thinkingLevel = strings.ToUpper(submit.ThinkingLevel)
			imageSearch = submit.ImageSearch
		}

		config := &api.Config{
			OutputFolder:  submit.OutputFolder,
			NumImages:     submit.NumImages,
			Prompt:        submit.Prompt,
			APIKey:        m.apiKey,
			AspectRatio:   submit.AspectRatio,
			ImageSize:     submit.ImageSize,
			Model:         modelName,
			ThinkingLevel: thinkingLevel,
			Grounding:     submit.Grounding,
			ImageSearch:   imageSearch,
			RefImages:     nil, // No reference images for generate
		}

		m.currentView = ProcessingView
		m.processingMsg = fmt.Sprintf("Generating %d image(s)...", submit.NumImages)
		m.outputFolder = submit.OutputFolder

		return m, func() tea.Msg {
			return ProcessingStartMsg{Config: config}
		}
	}

	var cmd tea.Cmd
	m.generateModel, cmd = m.generateModel.Update(msg)
	return m, cmd
}

func (m Model) updateEditView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle back to menu
	if _, ok := msg.(ui.BackToMenuMsg); ok {
		m.currentView = MenuView
		return m, nil
	}

	// Handle form submission
	if submit, ok := msg.(views.EditSubmitMsg); ok {
		// Load reference images
		refImages, err := api.LoadReferences(submit.ReferencePath)
		if err != nil {
			// Show error and stay in edit view
			m.currentView = ResultsView
			m.errorCount = 1
			m.successCount = 0
			m.results = []api.GenerationResult{{
				Index: 0,
				Error: fmt.Errorf("failed to load references: %v", err),
			}}
			return m, nil
		}

		modelName := api.ModelPro
		thinkingLevel := ""
		imageSearch := false
		if submit.Model == "flash" {
			modelName = api.ModelFlash
			thinkingLevel = strings.ToUpper(submit.ThinkingLevel)
			imageSearch = submit.ImageSearch
		}

		config := &api.Config{
			OutputFolder:  submit.OutputFolder,
			NumImages:     submit.NumImages,
			Prompt:        submit.Prompt,
			APIKey:        m.apiKey,
			AspectRatio:   submit.AspectRatio,
			ImageSize:     submit.ImageSize,
			Model:         modelName,
			ThinkingLevel: thinkingLevel,
			Grounding:     submit.Grounding,
			ImageSearch:   imageSearch,
			RefImages:     refImages,
		}

		m.currentView = ProcessingView
		m.processingMsg = fmt.Sprintf("Generating %d image(s) with %d reference(s)...", submit.NumImages, len(refImages))
		m.outputFolder = submit.OutputFolder

		return m, func() tea.Msg {
			return ProcessingStartMsg{Config: config}
		}
	}

	var cmd tea.Cmd
	m.editModel, cmd = m.editModel.Update(msg)
	return m, cmd
}

func (m Model) updateProcessingView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle processing start
	if start, ok := msg.(ProcessingStartMsg); ok {
		return m, tea.Batch(
			m.spinner.Tick,
			func() tea.Msg {
				return ProcessingDoneMsg{api.RunGeneration(start.Config)}
			},
		)
	}

	// Handle processing done
	if done, ok := msg.(ProcessingDoneMsg); ok {
		m.results = done.Results
		m.outputFolder = done.OutputFolder
		m.elapsed = done.Elapsed
		m.currentView = ResultsView

		// Count successes and errors
		m.successCount = 0
		m.errorCount = 0
		for _, r := range done.Results {
			if r.Error != nil {
				m.errorCount++
			} else {
				m.successCount++
			}
		}

		return m, nil
	}

	// Update spinner
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m Model) updateResultsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", " ", "esc", "q":
			m.currentView = MenuView
			m.results = nil
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateAPIKeyView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle API key saved message
	if saved, ok := msg.(views.APIKeySavedMsg); ok {
		m.apiKey = saved.APIKey
		m.currentView = MenuView
		return m, m.menuModel.Init()
	}

	var cmd tea.Cmd
	m.apiKeyModel, cmd = m.apiKeyModel.Update(msg)
	return m, cmd
}

// View renders the current view
func (m Model) View() string {
	var content string

	switch m.currentView {
	case APIKeyView:
		content = m.renderAPIKeyView()
	case MenuView:
		content = m.renderMenuView()
	case GenerateView:
		content = m.renderFormView(m.generateModel.View())
	case EditView:
		content = m.renderFormView(m.editModel.View())
	case ProcessingView:
		content = m.renderProcessingView()
	case ResultsView:
		content = m.renderResultsView()
	}

	return content
}

func (m Model) renderMenuView() string {
	banner := ui.RenderGradientBanner()
	subtitle := ui.RenderSubtitle()

	w := min(110, m.width-4)
	bannerStyle := lipgloss.NewStyle().Width(w - 2).Align(lipgloss.Center)
	centeredBanner := bannerStyle.Render(banner)

	header := lipgloss.JoinVertical(lipgloss.Center,
		"",
		centeredBanner,
		subtitle,
		"",
	)

	menuContent := m.menuModel.View()

	content := lipgloss.JoinVertical(lipgloss.Center,
		header,
		menuContent,
	)

	window := ui.WindowStyle.Width(w).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, window)
}

func (m Model) renderAPIKeyView() string {
	banner := ui.RenderGradientBanner()
	subtitle := ui.RenderSubtitle()

	w := min(110, m.width-4)
	bannerStyle := lipgloss.NewStyle().Width(w - 2).Align(lipgloss.Center)
	centeredBanner := bannerStyle.Render(banner)

	header := lipgloss.JoinVertical(lipgloss.Center,
		"",
		centeredBanner,
		subtitle,
		"",
	)

	content := lipgloss.JoinVertical(lipgloss.Center,
		header,
		m.apiKeyModel.View(),
	)

	window := ui.WindowStyle.Width(w).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, window)
}

func (m Model) renderFormView(formContent string) string {
	w := min(80, m.width-4)
	window := ui.WindowStyle.Width(w).Render(formContent)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, window)
}

func (m Model) renderProcessingView() string {
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.DraculaPurple)
	msgStyle := lipgloss.NewStyle().Foreground(ui.DraculaCyan).Bold(true)

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		spinnerStyle.Render(m.spinner.View())+" "+msgStyle.Render(m.processingMsg),
		"",
		ui.SubtleStyle.Render("Please wait..."),
		"",
	)

	window := ui.WindowStyle.Width(60).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, window)
}

func (m Model) renderResultsView() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, ui.TitleStyle.Render("Results"))
	lines = append(lines, "")

	// Show individual results
	for _, r := range m.results {
		if r.Error != nil {
			lines = append(lines, ui.ErrorStyle.Render(fmt.Sprintf("[X] Image %d: %v", r.Index+1, r.Error)))
		} else {
			lines = append(lines, ui.SuccessStyle.Render(fmt.Sprintf("[+] %s", r.Filename)))
		}
	}

	lines = append(lines, "")
	lines = append(lines, ui.SubtleStyle.Render("------------------------------"))
	lines = append(lines, ui.InfoStyle.Render(fmt.Sprintf("Summary: %d success, %d failed", m.successCount, m.errorCount)))
	lines = append(lines, ui.InfoStyle.Render(fmt.Sprintf("Time: %s", m.elapsed.Round(time.Millisecond))))
	lines = append(lines, ui.InfoStyle.Render(fmt.Sprintf("Output: %s", m.outputFolder)))
	lines = append(lines, "")
	lines = append(lines, ui.HelpStyle.Render("Press any key to continue..."))
	lines = append(lines, "")

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	window := ui.WindowStyle.Width(70).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, window)
}

func main() {
	// Handle subcommands first (before flag parsing)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			if cli.HandleConfigCommand(os.Args[1:]) {
				return
			}
		case "describe":
			describe.HandleDescribeCommand(os.Args[2:])
			return
		}
	}

	// Parse CLI flags
	opts, cliMode := cli.ParseFlags()

	// Show version if requested
	if opts.Version {
		cli.PrintVersion()
		return
	}

	// Show help if requested
	if opts.Help {
		cli.PrintHelp()
		return
	}

	// Get API key from config (checks env vars first, then config file)
	apiKey := config.GetAPIKey()

	// CLI mode: prompt for API key if not found
	if cliMode {
		if apiKey == "" {
			apiKey = cli.PromptForAPIKey()
		}
		cli.Run(opts, apiKey)
		return
	}

	// TUI mode: launch with API key (TUI will handle missing key)
	p := tea.NewProgram(NewModel(apiKey), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
