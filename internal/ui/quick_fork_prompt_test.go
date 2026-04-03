package ui

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Add user settings", "add-user-settings"},
		{"Fix auth bug!", "fix-auth-bug"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPERCASE", "uppercase"},
		{"special@#$chars", "specialchars"},
		{"multiple---hyphens", "multiple-hyphens"},
		{"-leading-trailing-", "leading-trailing"},
		{"a", "a"},
		{"", ""},
		{"hello world 123", "hello-world-123"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewQuickForkPrompt(t *testing.T) {
	p := NewQuickForkPrompt()
	if p == nil {
		t.Fatal("NewQuickForkPrompt() returned nil")
	}
	if p.IsVisible() {
		t.Error("Prompt should not be visible initially")
	}
}

func TestQuickForkPrompt_ShowHide(t *testing.T) {
	p := NewQuickForkPrompt()

	p.Show()
	if !p.IsVisible() {
		t.Error("Prompt should be visible after Show()")
	}

	p.Hide()
	if p.IsVisible() {
		t.Error("Prompt should not be visible after Hide()")
	}
}

func TestQuickForkPrompt_ShowClearsPreviousValue(t *testing.T) {
	p := NewQuickForkPrompt()
	p.Show()
	p.input.SetValue("old value")
	p.Hide()

	p.Show()
	if p.input.Value() != "" {
		t.Errorf("Show() should clear previous value, got %q", p.input.Value())
	}
}

func TestQuickForkPrompt_Name(t *testing.T) {
	p := NewQuickForkPrompt()
	p.Show()
	p.input.SetValue("  Add user settings  ")
	if got := p.Name(); got != "Add user settings" {
		t.Errorf("Name() = %q, want %q", got, "Add user settings")
	}
}

func TestQuickForkPrompt_ViewNotVisibleReturnsEmpty(t *testing.T) {
	p := NewQuickForkPrompt()
	if v := p.View(80); v != "" {
		t.Errorf("View() when not visible should be empty, got %q", v)
	}
}

func TestQuickForkPrompt_ViewVisibleContainsFork(t *testing.T) {
	p := NewQuickForkPrompt()
	p.Show()
	v := p.View(80)
	if !strings.Contains(v, "Fork") {
		t.Errorf("View() should contain 'Fork', got %q", v)
	}
}

func TestQuickForkPrompt_ErrorDisplay(t *testing.T) {
	p := NewQuickForkPrompt()
	p.Show()
	p.SetError("branch already exists")
	v := p.View(80)
	if !strings.Contains(v, "branch already exists") {
		t.Errorf("View() should show error, got %q", v)
	}
}
