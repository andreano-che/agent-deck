package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// slugify converts a session name to a branch-safe slug.
// "Add user settings!" -> "add-user-settings"
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	// Replace whitespace with hyphens
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	// Remove any characters that are not alphanumeric or hyphens
	s = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(s, "")
	// Collapse multiple hyphens
	s = regexp.MustCompile(`-{2,}`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// QuickForkMsg is sent when the user submits a name in the quick fork prompt.
type QuickForkMsg struct {
	Name string
}

// QuickForkCancelMsg is sent when the user cancels the quick fork prompt.
type QuickForkCancelMsg struct{}

// QuickForkPrompt is a minimal one-line prompt for quick session forking.
type QuickForkPrompt struct {
	visible bool
	input   textinput.Model
	errMsg  string
	width   int
}

// NewQuickForkPrompt creates a new quick fork prompt.
func NewQuickForkPrompt() *QuickForkPrompt {
	ti := textinput.New()
	ti.Placeholder = "Session name..."
	ti.CharLimit = MaxNameLength
	ti.Width = 40
	return &QuickForkPrompt{input: ti}
}

// Show makes the prompt visible and focuses the input.
func (p *QuickForkPrompt) Show() {
	p.visible = true
	p.input.SetValue("")
	p.input.Focus()
	p.errMsg = ""
}

// Hide hides the prompt and blurs the input.
func (p *QuickForkPrompt) Hide() {
	p.visible = false
	p.input.Blur()
	p.errMsg = ""
}

// IsVisible returns whether the prompt is visible.
func (p *QuickForkPrompt) IsVisible() bool {
	return p.visible
}

// Name returns the trimmed input value.
func (p *QuickForkPrompt) Name() string {
	return strings.TrimSpace(p.input.Value())
}

// SetError displays an error message in the prompt.
func (p *QuickForkPrompt) SetError(msg string) {
	p.errMsg = msg
}

// SetWidth sets the available width for rendering.
func (p *QuickForkPrompt) SetWidth(w int) {
	p.width = w
	inputWidth := w - 10 // "Fork: " + padding
	if inputWidth < 20 {
		inputWidth = 20
	}
	p.input.Width = inputWidth
}

// Update handles key events. Returns a tea.Cmd with QuickForkMsg or QuickForkCancelMsg.
func (p *QuickForkPrompt) Update(msg tea.Msg) tea.Cmd {
	if !p.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			name := p.Name()
			if name == "" {
				return nil // Don't submit empty names
			}
			return func() tea.Msg {
				return QuickForkMsg{Name: name}
			}
		case "esc":
			return func() tea.Msg {
				return QuickForkCancelMsg{}
			}
		}
	}

	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.errMsg = "" // Clear error on new input
	return cmd
}

// View renders the prompt as a single styled line.
func (p *QuickForkPrompt) View(width int) string {
	if !p.visible {
		return ""
	}

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorCyan).
		Bold(true)

	line := labelStyle.Render("Fork: ") + p.input.View()

	if p.errMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
		line += "  " + errStyle.Render(p.errMsg)
	}

	return line
}
