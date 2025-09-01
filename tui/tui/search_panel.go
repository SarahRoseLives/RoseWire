// tui/search_panel.go
package tui

import (
	"fmt"
	"rosetui/ssh"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Command Messages ---
type searchFilesCmd string
type fetchTopFilesCmd struct{}

type searchPanelModel struct {
	textInput   textinput.Model
	viewport    viewport.Model
	spinner     spinner.Model
	results     []ssh.SearchResult
	isLoading   bool
	hasSearched bool
	cursor      int
	styles      *SearchPanelStyles
}

func NewSearchPanel() *searchPanelModel {
	styles := DefaultSearchPanelStyles()
	ti := textinput.New()
	ti.Placeholder = "Search for files or users (@user@host)..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80 // Dynamic
	ti.Prompt = "┃ "
	ti.PromptStyle = styles.InputPrompt
	ti.TextStyle = styles.InputText

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	vp := viewport.New(80, 20) // Dynamic
	vp.Style = styles.ResultsViewport

	return &searchPanelModel{
		textInput: ti,
		spinner:   sp,
		viewport:  vp,
		styles:    styles,
		isLoading: true,
	}
}

func (m *searchPanelModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg { return fetchTopFilesCmd{} })
}

func (m *searchPanelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var vpCmd, tiCmd, spCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.textInput.Focused() {
			switch msg.String() {
			case "enter":
				query := strings.TrimSpace(m.textInput.Value())
				m.isLoading = true
				m.hasSearched = true
				m.results = nil
				m.cursor = 0
				cmds = append(cmds, func() tea.Msg { return searchFilesCmd(query) })
			case "down", "esc":
				m.textInput.Blur()
			}
		} else { // Handle viewport navigation
			switch msg.String() {
			case "up":
				if m.cursor > 0 {
					m.cursor--
					m.updateViewportContent() // re-render on change
				} else {
					m.textInput.Focus()
				}
			case "down":
				if m.cursor < len(m.results)-1 {
					m.cursor++
					m.updateViewportContent() // re-render on change
				}
			case "enter":
				// Placeholder for download functionality
			}
		}

	case ssh.SearchResultsMsg:
		m.isLoading = false
		m.results = msg.Results
		m.updateViewportContent()
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.spinner, spCmd = m.spinner.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, tiCmd, spCmd, vpCmd)
	return m, tea.Batch(cmds...)
}

func (m *searchPanelModel) View() string {
	var resultsContent string
	if m.isLoading {
		resultsContent = fmt.Sprintf("%s Searching...", m.spinner.View())
	} else if len(m.results) == 0 {
		if m.hasSearched {
			resultsContent = "No results found."
		} else {
			resultsContent = "No files currently shared on the network."
		}
	} else {
		resultsContent = m.viewport.View()
	}

	searchBarStyle := m.styles.SearchBar
	if m.textInput.Focused() {
		searchBarStyle = m.styles.SearchBarFocused
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		searchBarStyle.Render(m.textInput.View()),
		m.styles.ResultsBox.Render(resultsContent),
	)
}

func (m *searchPanelModel) updateViewportContent() {
	var b strings.Builder
	for i, res := range m.results {
		if i == m.cursor {
			b.WriteString(m.styles.ResultSelected.Render(formatResult(res)))
		} else {
			b.WriteString(m.styles.ResultNormal.Render(formatResult(res)))
		}
		b.WriteString("\n")
	}
	m.viewport.SetContent(b.String())
}

func formatResult(r ssh.SearchResult) string {
	return fmt.Sprintf("%s\n  └─ Size: %s  From: %s", r.FileName, formatBytes(r.Size), r.Peer)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m *searchPanelModel) SetSize(w, h int) {
	barStyle := m.styles.SearchBar
	if m.textInput.Focused() {
		barStyle = m.styles.SearchBarFocused
	}

	searchBarHorizontalFrameSize := barStyle.GetHorizontalFrameSize()
	m.textInput.Width = w - searchBarHorizontalFrameSize - len(m.textInput.Prompt)

	// **FIX:** The search bar component takes up 4 lines total
	// (1 for top border, 1 for content, 1 for bottom border, and 1 for margin).
	// The height calculation is now correct.
	resultsBoxHeight := h - 4
	m.styles.ResultsBox.Width(w)
	m.styles.ResultsBox.Height(resultsBoxHeight)

	m.viewport.Width = w - m.styles.ResultsBox.GetHorizontalPadding()
	m.viewport.Height = resultsBoxHeight - m.styles.ResultsBox.GetVerticalPadding()
	m.updateViewportContent() // Re-render content on resize
}

// --- Styling ---

type SearchPanelStyles struct {
	SearchBar        lipgloss.Style
	SearchBarFocused lipgloss.Style
	InputPrompt      lipgloss.Style
	InputText        lipgloss.Style
	Spinner          lipgloss.Style
	ResultsBox       lipgloss.Style
	ResultsViewport  lipgloss.Style
	ResultNormal     lipgloss.Style
	ResultSelected   lipgloss.Style
}

func DefaultSearchPanelStyles() *SearchPanelStyles {
	return &SearchPanelStyles{
		SearchBar: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginBottom(1),
		SearchBarFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 1).
			MarginBottom(1),
		InputPrompt:     lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		InputText:       lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		Spinner:         lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		ResultsBox:      lipgloss.NewStyle().Padding(1, 2),
		ResultsViewport: lipgloss.NewStyle(),
		ResultNormal:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		ResultSelected:  lipgloss.NewStyle().Foreground(lipgloss.Color("#EC4899")).Bold(true),
	}
}