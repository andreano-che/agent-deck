# Quick Fork: One-Hotkey Session Forking with Git Worktree

**Date:** 2026-04-02
**Status:** Draft

## Problem

When working in a Claude Code session and realizing you need to start a parallel task, the current fork workflow requires navigating a full dialog (name, group, worktree checkbox, branch, sandbox, options). This friction discourages forking — you lose momentum switching contexts.

## Solution

A single hotkey (`Alt+F`) opens a minimal one-line prompt. Type a name, press Enter — Agent Deck creates a git worktree, spawns a new Claude Code session, and switches to its tab. The whole flow takes under 5 seconds.

## UX Flow

```
User viewing session "Fix auth bug" in Agent Deck TUI
    │
    ▼  Alt+F
┌─────────────────────────────────────────┐
│  Fork: [                              ] │
└─────────────────────────────────────────┘
Single-line prompt at bottom of screen (Vim command-line style)
Placeholder: "Session name..."
    │
    ▼  Types "Add user settings", presses Enter
    │
    1. git worktree add → fork/add-user-settings branch
    2. New Instance created (parent = current session)
    3. Claude Code starts in worktree directory
    4. Tab strip switches to new tab
    │
    ▼
User is in the new session, working.

Esc at any point → cancel, nothing created.
```

## Architecture

### New Component: `QuickForkPrompt`

**File:** `internal/ui/quick_fork_prompt.go` (~100 lines)

A minimal overlay with a single `textinput.Model`.

```go
type QuickForkPrompt struct {
    visible bool
    input   textinput.Model
}
```

**Methods:**
- `Show()` — makes visible, focuses input, clears previous value
- `Hide()` — hides, blurs input
- `Update(msg) → (cmd)` — handles key input; Enter emits `QuickForkMsg`, Esc emits `QuickForkCancelMsg`
- `View() → string` — renders `Fork: [input]` as a single styled line
- `IsVisible() → bool`
- `Name() → string` — returns trimmed input value

**Messages:**

```go
type QuickForkMsg struct {
    Name string
}

type QuickForkCancelMsg struct{}
```

### Changes to Home (`internal/ui/home.go`)

- New field: `quickForkPrompt *QuickForkPrompt`
- `Update()`: `Alt+F` → `quickForkPrompt.Show()` (only when no dialog is visible and a session is selected)
- `Update()`: `QuickForkMsg` handler:
  1. Get currently selected session
  2. Slugify name → branch name `fork/<slug>`
  3. Deduplicate branch name if exists (append `-2`, `-3`, etc.)
  4. If git repo: create worktree
  5. Load default `ClaudeOptions` from `UserConfig` (same as ForkDialog does)
  6. Call `Instance.CreateForkedInstanceWithOptions()` with worktree path and default options
  7. Save to storage
  7. Switch tab strip to new tab
- `Update()`: `QuickForkCancelMsg` → `quickForkPrompt.Hide()`
- `View()`: when prompt visible, render it at bottom in place of status bar

### New Hotkey (`internal/ui/hotkeys.go`)

- `HotkeyQuickFork` bound to `Alt+F`

### Slugify Function

**Location:** `internal/ui/quick_fork_prompt.go` (unexported helper)

- Lowercase input
- Replace spaces with hyphens
- Strip non-alphanumeric characters (except hyphens)
- Collapse multiple hyphens
- Trim leading/trailing hyphens
- Example: "Add user settings!" → `add-user-settings`

### Branch Deduplication

If `fork/add-user-settings` already exists, try `fork/add-user-settings-2`, `-3`, etc. Check via `git branch --list`.

## Reused Existing Code

| What | Where | Already exists |
|------|-------|----------------|
| Git repo detection | `git.IsGitRepo()` | Yes |
| Worktree creation | `internal/git/` | Yes |
| Fork instance creation | `Instance.CreateForkedInstanceWithOptions()` | Yes |
| Session start in tmux | `Instance.Start()` | Yes |
| Tab switching | `TabStripModel.SelectTab()` | Yes |
| Branch name validation | `git.ValidateBranchName()` | Yes |

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Not a git repo | Fork creates a new session in the same directory (no worktree) |
| Branch name already exists | Auto-suffix: `-2`, `-3`, etc. |
| Worktree creation fails | Show error inline in the prompt line, keep prompt open |
| Empty name submitted | Prompt stays open, does not submit |
| No session selected | Hotkey does nothing |
| Dialog already visible | Hotkey does nothing |

## What Does NOT Change

- `ForkDialog` — remains as-is for full-featured fork workflow
- `Instance` model — no new fields or methods needed
- `session/` package — all fork logic already exists
- Existing hotkeys — no conflicts

## Tests

### `quick_fork_prompt_test.go`
- `Show()` makes visible, `Hide()` hides
- Enter with text → `QuickForkMsg` with correct name
- Enter with empty text → no message (stays open)
- Esc → `QuickForkCancelMsg`

### Slugify tests
- Spaces → hyphens
- Special characters stripped
- Unicode handled (transliterated or stripped)
- Multiple spaces/hyphens collapsed
- Leading/trailing hyphens trimmed

### Integration tests
- `Alt+F` opens prompt when session selected
- `Alt+F` does nothing when dialog is open
- `QuickForkMsg` creates instance with correct parent, worktree path, branch name
- Branch deduplication works with existing branches
- Non-git repo creates session without worktree
