# Quick Fork Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current `f` hotkey quick-fork (which silently appends "(fork)" to the name) with a one-line prompt that asks for a session name, then auto-creates a git worktree + new session and switches to it.

**Architecture:** New `QuickForkPrompt` component (~100 lines) with a single `textinput.Model`. Home wires `f` to show it, handles `QuickForkMsg` by reusing existing worktree + fork logic, then switches the tab strip. No changes to `ForkDialog`, `Instance`, or the `session` package.

**Tech Stack:** Go, Bubble Tea, Lipgloss, existing `internal/git` and `internal/session` packages.

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/ui/quick_fork_prompt.go` | Create | `QuickForkPrompt` component + `slugify` helper |
| `internal/ui/quick_fork_prompt_test.go` | Create | Unit tests for prompt + slugify |
| `internal/ui/home.go` | Modify | Wire prompt into Home: show on `f`, handle msgs, render |

---

### Task 1: QuickForkPrompt Component + Slugify

**Files:**
- Create: `internal/ui/quick_fork_prompt.go`
- Create: `internal/ui/quick_fork_prompt_test.go`

- [ ] **Step 1: Write failing tests for slugify**

```go
// internal/ui/quick_fork_prompt_test.go
package ui

import (
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go test ./internal/ui/ -run TestSlugify -v`
Expected: FAIL — `slugify` not defined

- [ ] **Step 3: Implement slugify**

```go
// internal/ui/quick_fork_prompt.go
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
	// Keep only alphanumeric and hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "-")
	// Collapse multiple hyphens
	s = regexp.MustCompile(`-{2,}`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go test ./internal/ui/ -run TestSlugify -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for QuickForkPrompt**

Add to `internal/ui/quick_fork_prompt_test.go`:

```go
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
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go test ./internal/ui/ -run TestQuickForkPrompt -v`
Expected: FAIL — types not defined

- [ ] **Step 7: Implement QuickForkPrompt**

Add to `internal/ui/quick_fork_prompt.go` (after the slugify function):

```go
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
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go test ./internal/ui/ -run "TestSlugify|TestQuickForkPrompt" -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add internal/ui/quick_fork_prompt.go internal/ui/quick_fork_prompt_test.go
git commit -m "feat: add QuickForkPrompt component with slugify helper"
```

---

### Task 2: Wire QuickForkPrompt into Home

**Files:**
- Modify: `internal/ui/home.go` — lines around 168 (field), 668 (init), 5048-5063 (hotkey dispatch), 6562-6571 (quickForkSession), and view rendering

This task modifies only `home.go`. The changes are:
1. Add `quickForkPrompt` field
2. Initialize it in constructor
3. Change `f` hotkey to show prompt instead of instant fork
4. Handle `QuickForkMsg` — create worktree + fork + switch tab
5. Handle `QuickForkCancelMsg` — hide prompt
6. Render prompt in View when visible
7. Block other hotkeys when prompt is visible

- [ ] **Step 1: Add field and initialize**

In `home.go`, find the `forkDialog` field (around line 171) and add the new field nearby:

```go
// Find this line:
forkDialog           *ForkDialog           // For forking sessions

// Add after it:
quickForkPrompt      *QuickForkPrompt      // For quick fork with name prompt
```

In the constructor (around line 668), find `forkDialog: NewForkDialog(),` and add:

```go
// Find this line:
forkDialog:           NewForkDialog(),

// Add after it:
quickForkPrompt:      NewQuickForkPrompt(),
```

- [ ] **Step 2: Change `f` hotkey to show prompt**

Replace the `case "f":` block (lines 5048-5064) with:

```go
	case "f":
		// Quick fork with name prompt
		if h.cursor < len(h.flatItems) {
			item := h.flatItems[h.cursor]
			if item.Type == session.ItemTypeSession && item.Session != nil {
				if h.hasActiveAnimation(item.Session.ID) {
					h.setError(fmt.Errorf("session is starting, please wait..."))
					return h, nil
				}
				if item.Session.CanFork() {
					h.quickForkPrompt.SetWidth(h.width)
					h.quickForkPrompt.Show()
				}
			}
		}
		return h, nil
```

- [ ] **Step 3: Handle QuickForkMsg and QuickForkCancelMsg in Update**

In the `Update` method's message switch (where `sessionForkedMsg` is handled, around line 3081), add two new cases:

```go
	case QuickForkMsg:
		h.quickForkPrompt.Hide()
		if h.cursor < len(h.flatItems) {
			item := h.flatItems[h.cursor]
			if item.Type == session.ItemTypeSession && item.Session != nil && item.Session.CanFork() {
				source := item.Session
				title := msg.Name
				groupPath := source.GroupPath
				slug := slugify(msg.Name)
				branchName := "fork/" + slug

				// Load default options from config
				var opts *session.ClaudeOptions
				if config, err := session.LoadUserConfig(); err == nil {
					panel := NewClaudeOptionsPanelForFork()
					panel.SetDefaults(config)
					opts = panel.GetOptions()
				}
				if opts == nil {
					opts = &session.ClaudeOptions{}
				}

				// Set up worktree if git repo
				if git.IsGitRepo(source.ProjectPath) {
					repoRoot, err := git.GetWorktreeBaseRoot(source.ProjectPath)
					if err != nil {
						h.setError(fmt.Errorf("failed to get repo root: %v", err))
						return h, nil
					}

					// Deduplicate branch name
					for i := 2; git.BranchExists(repoRoot, branchName); i++ {
						branchName = fmt.Sprintf("fork/%s-%d", slug, i)
					}

					wtSettings := session.GetWorktreeSettings()
					worktreePath := git.WorktreePath(git.WorktreePathOptions{
						Branch:    branchName,
						Location:  wtSettings.DefaultLocation,
						RepoDir:   repoRoot,
						SessionID: git.GeneratePathID(),
						Template:  wtSettings.Template(),
					})

					opts.WorkDir = worktreePath
					opts.WorktreePath = worktreePath
					opts.WorktreeRepoRoot = repoRoot
					opts.WorktreeBranch = branchName
				}

				return h, h.forkSessionCmdWithOptions(source, title, groupPath, opts, false)
			}
		}
		return h, nil

	case QuickForkCancelMsg:
		h.quickForkPrompt.Hide()
		return h, nil
```

Note: `git` package import will be needed — check if `"github.com/asheshgoplani/agent-deck/internal/git"` is already imported in `home.go`. It is (used in `handleForkDialogKey`).

- [ ] **Step 4: Delegate key events to prompt when visible**

In the `Update` method's `tea.KeyMsg` handling, at the top of the key switch (around line 4084 where `forkDialog.IsVisible()` is checked), add a check for the quick fork prompt:

```go
// Find this block:
	if h.forkDialog.IsVisible() {
		return h.handleForkDialogKey(msg)
	}

// Add BEFORE it:
	if h.quickForkPrompt.IsVisible() {
		cmd := h.quickForkPrompt.Update(msg)
		return h, cmd
	}
```

- [ ] **Step 5: Block other dialogs when prompt is visible**

Find the dialog-visibility guard (around line 4558):

```go
h.newDialog.IsVisible() || h.groupDialog.IsVisible() || h.forkDialog.IsVisible() ||
```

Add `h.quickForkPrompt.IsVisible() ||` to this condition.

- [ ] **Step 6: Render prompt in View**

Find where `forkDialog.View()` is rendered (around line 7581):

```go
	if h.forkDialog.IsVisible() {
		return h.forkDialog.View()
	}
```

Add BEFORE it:

```go
	if h.quickForkPrompt.IsVisible() {
		// Render main view with prompt at bottom
		mainView := h.renderMainView()
		promptLine := h.quickForkPrompt.View(h.width)
		// Replace last line of main view with prompt
		return mainView + "\n" + promptLine
	}
```

Note: The exact rendering approach depends on how `View()` is structured. If there's no `renderMainView()` helper, render the prompt at the bottom of the existing view. Look at how the status bar is rendered at the bottom and replace it with the prompt line when visible. The key requirement: the prompt appears as a single line at the very bottom of the screen.

- [ ] **Step 7: Update quickForkSession to remove old behavior**

The existing `quickForkSession` method (line 6562-6571) is no longer called by the `f` hotkey. Check if anything else calls it. If not, it can be removed. If other code paths call it, leave it.

Run: `grep -n 'quickForkSession' internal/ui/home.go`

If only the old `case "f":` called it, delete the method. Otherwise leave it.

- [ ] **Step 8: Verify compilation**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go build ./...`
Expected: No errors

- [ ] **Step 9: Run all existing tests**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go test ./internal/ui/ -v -count=1 2>&1 | tail -30`
Expected: All PASS (existing tests still pass)

- [ ] **Step 10: Commit**

```bash
git add internal/ui/home.go
git commit -m "feat: wire QuickForkPrompt into Home — f opens name prompt with auto-worktree"
```

---

### Task 3: Auto-Select New Tab After Fork

**Files:**
- Modify: `internal/ui/home.go` — the `sessionForkedMsg` handler (around line 3081)

The existing `sessionForkedMsg` handler already auto-selects the new session in the list view (moves cursor). But we need to also switch the tab strip to the new tab.

- [ ] **Step 1: Check current sessionForkedMsg handler**

Read the full handler at line 3081 to understand what it already does after a successful fork. It adds the instance to the list and moves the cursor.

- [ ] **Step 2: Add tab strip switch after fork**

In the `sessionForkedMsg` handler, after the cursor is moved to the new session (the `for i, item := range h.flatItems` loop), add tab strip selection:

```go
// After the cursor auto-select loop, add:
if h.tabStrip != nil {
	// Find the new instance index in tab strip
	for idx, inst := range h.tabStrip.instances {
		if inst.ID == msg.instance.ID {
			h.tabStrip.SelectTab(idx)
			break
		}
	}
}
```

Note: Check if `tabStrip` field exists and how instances are synced. The tab strip may be updated via `UpdateInstances` which runs on tick. If the new instance isn't in `tabStrip.instances` yet at this point, the tab switch may need to happen after the next `UpdateInstances` call. In that case, store a `pendingTabSwitch` ID and apply it in the next tick.

- [ ] **Step 3: Verify compilation and tests**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go build ./... && go test ./internal/ui/ -v -count=1 2>&1 | tail -20`
Expected: Build OK, all tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ui/home.go
git commit -m "feat: auto-switch tab strip to newly forked session"
```

---

### Task 4: Manual Testing Checklist

This task is manual verification in a running Agent Deck instance.

- [ ] **Step 1: Build and run**

```bash
cd /Users/andreyvashchenko/projects/agent-deck-fork && go build -o agent-deck ./cmd/agent-deck/ && ./agent-deck
```

- [ ] **Step 2: Test happy path**

1. Select a running Claude session
2. Press `f`
3. Verify: one-line prompt appears at bottom with "Fork: " label
4. Type "Add user settings"
5. Press Enter
6. Verify: new tab appears in tab strip with name "Add user settings"
7. Verify: tab strip switches to new tab (spinner icon for Starting)
8. Verify: git worktree created at expected path with branch `fork/add-user-settings`

- [ ] **Step 3: Test cancel**

1. Press `f` on a session
2. Press Esc
3. Verify: prompt disappears, nothing created

- [ ] **Step 4: Test empty name**

1. Press `f`
2. Press Enter without typing
3. Verify: prompt stays open, nothing happens

- [ ] **Step 5: Test non-git project**

1. Select a session in a non-git directory
2. Press `f`, type a name, Enter
3. Verify: new session created in same directory (no worktree), no error

- [ ] **Step 6: Test branch deduplication**

1. Fork a session as "test dedup" (creates `fork/test-dedup`)
2. Fork again as "test dedup"
3. Verify: second fork creates branch `fork/test-dedup-2`

- [ ] **Step 7: Test F still opens full dialog**

1. Press `F` (shift+f)
2. Verify: full ForkDialog still opens as before

- [ ] **Step 8: Final commit if any fixes needed**

```bash
git add -u
git commit -m "fix: address issues found during manual testing"
```
