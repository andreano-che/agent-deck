# Attach-Mode Hotkeys Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Alt+F (quick fork with worktree) and Alt+U (mark unread) hotkeys that work while attached to a tmux session, and revert the Home `f` key to its original instant-fork behavior.

**Architecture:** Two new CLI commands (`fork-request`, `mark-unread`) following the `tab-switch` pattern. tmux root key bindings dispatch to these CLIs during attach. Home handles the `fork_request` file on detach return, shows the existing QuickForkPrompt, then re-attaches to the new forked session. `mark-unread` runs silently in the background without detaching.

**Tech Stack:** Go, Bubble Tea, tmux root key bindings, SQLite (statedb)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `cmd/agent-deck/fork_request_cmd.go` | Create | CLI: write fork_request file + tmux detach |
| `cmd/agent-deck/mark_unread_cmd.go` | Create | CLI: set acknowledged=false in SQLite |
| `cmd/agent-deck/main.go` | Modify | Route `fork-request` and `mark-unread` subcommands |
| `internal/ui/home.go` | Modify | Revert `f` key, add forkRequestMsg handler, modify QuickForkMsg/Cancel handlers, add tmux bindings/cleanup, add fork_request file check in tea.Exec callback |

---

### Task 1: `fork-request` CLI Command

**Files:**
- Create: `cmd/agent-deck/fork_request_cmd.go`
- Modify: `cmd/agent-deck/main.go:298-300`

- [ ] **Step 1: Create fork_request_cmd.go**

```go
// cmd/agent-deck/fork_request_cmd.go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// handleForkRequest writes the source session ID to a fork_request file
// and detaches the tmux client so Home can pick it up.
// All errors are swallowed so this never disrupts an active terminal session.
func handleForkRequest(args []string) {
	var sessionID string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--session=") {
			sessionID = strings.TrimPrefix(arg, "--session=")
		}
	}
	if sessionID == "" {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	adDir := filepath.Join(homeDir, ".agent-deck")
	_ = os.WriteFile(filepath.Join(adDir, "fork_request"), []byte(sessionID), 0644)
	_ = exec.Command("tmux", "detach-client").Run()
}
```

- [ ] **Step 2: Register in main.go**

In `cmd/agent-deck/main.go`, find:

```go
		case "tab-switch":
			handleTabSwitch(args[1:])
			return
		}
```

Add BEFORE the closing `}`:

```go
		case "fork-request":
			handleForkRequest(args[1:])
			return
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add cmd/agent-deck/fork_request_cmd.go cmd/agent-deck/main.go
git commit -m "feat: add fork-request CLI command for attach-mode fork"
```

---

### Task 2: `mark-unread` CLI Command

**Files:**
- Create: `cmd/agent-deck/mark_unread_cmd.go`
- Modify: `cmd/agent-deck/main.go`

- [ ] **Step 1: Create mark_unread_cmd.go**

```go
// cmd/agent-deck/mark_unread_cmd.go
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// handleMarkUnread sets acknowledged=false for the given session in SQLite.
// Runs silently — no detach, no output. All errors swallowed.
func handleMarkUnread(args []string) {
	var sessionID string
	for _, arg := range args {
		if strings.HasPrefix(arg, "--session=") {
			sessionID = strings.TrimPrefix(arg, "--session=")
		}
	}
	if sessionID == "" {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Try profile path first (matches tab-switch pattern)
	profileDB := filepath.Join(homeDir, ".agent-deck", "profiles", "default", "state.db")
	db, err := statedb.Open(profileDB)
	if err != nil {
		// Fallback to root path
		db, err = statedb.Open(filepath.Join(homeDir, ".agent-deck", "state.db"))
		if err != nil {
			return
		}
	}
	defer func() { _ = db.Close() }()

	_ = db.SetAcknowledged(sessionID, false)
}
```

- [ ] **Step 2: Register in main.go**

In `cmd/agent-deck/main.go`, find the `fork-request` case just added, and add after it:

```go
		case "mark-unread":
			handleMarkUnread(args[1:])
			return
```

- [ ] **Step 3: Verify build**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add cmd/agent-deck/mark_unread_cmd.go cmd/agent-deck/main.go
git commit -m "feat: add mark-unread CLI command for attach-mode unread toggle"
```

---

### Task 3: Add tmux Bindings for Alt+F and Alt+U

**Files:**
- Modify: `internal/ui/home.go` — `attachSession()` (~line 7150) and `cleanupTabStrip()` (~line 7219)

- [ ] **Step 1: Add bindings in attachSession()**

Find this block in `internal/ui/home.go` (around line 7149-7155):

```go
	// Register Alt+1-9 key bindings for tab switching
	for i := 1; i <= 9; i++ {
		key := fmt.Sprintf("M-%d", i)
		cmd := fmt.Sprintf("%s tab-switch %d", exe, i)
		_ = exec.Command("tmux", "bind-key", "-T", "root", key,
			"run-shell", cmd).Run()
	}
```

Add AFTER it:

```go
	// Register Alt+F for quick fork and Alt+U for mark unread
	forkCmd := fmt.Sprintf("%s fork-request --session=%s", exe, inst.ID)
	_ = exec.Command("tmux", "bind-key", "-T", "root", "M-f",
		"run-shell", forkCmd).Run()
	unreadCmd := fmt.Sprintf("%s mark-unread --session=%s", exe, inst.ID)
	_ = exec.Command("tmux", "bind-key", "-T", "root", "M-u",
		"run-shell", unreadCmd).Run()
```

- [ ] **Step 2: Add cleanup in cleanupTabStrip()**

Find this block (around line 7219-7223):

```go
	// Unbind Alt+1-9
	for i := 1; i <= 9; i++ {
		key := fmt.Sprintf("M-%d", i)
		_ = exec.Command("tmux", "unbind-key", "-T", "root", key).Run()
	}
```

Add AFTER it:

```go
	// Unbind Alt+F and Alt+U
	_ = exec.Command("tmux", "unbind-key", "-T", "root", "M-f").Run()
	_ = exec.Command("tmux", "unbind-key", "-T", "root", "M-u").Run()
```

Also find the cleanup of state files (around line 7226-7229):

```go
	// Clean up state files
	homeDir, err := os.UserHomeDir()
	if err == nil {
		_ = os.Remove(filepath.Join(homeDir, ".agent-deck", "tab_current"))
		_ = os.Remove(filepath.Join(homeDir, ".agent-deck", "tab_switch_request"))
	}
```

Add to the cleanup block:

```go
		_ = os.Remove(filepath.Join(homeDir, ".agent-deck", "fork_request"))
```

- [ ] **Step 3: Verify build and tests**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go build ./... && go test ./internal/ui/ -count=1 2>&1 | tail -5`
Expected: Build OK, all tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/ui/home.go
git commit -m "feat: register Alt+F and Alt+U tmux bindings during attach"
```

---

### Task 4: Revert `f` Hotkey + Add forkRequestMsg Handler

**Files:**
- Modify: `internal/ui/home.go` — struct fields, constructor, `f` hotkey, message handlers, tea.Exec callback

This is the largest task. All changes are in `home.go`.

- [ ] **Step 1: Add forkRequestMsg type and forkRequestSource field**

Find the existing message types (search for `type sessionForkedMsg`). Add nearby:

```go
// forkRequestMsg is sent when the user pressed Alt+F in attach mode.
type forkRequestMsg struct {
	sourceID string
}
```

Find the `quickForkPrompt` field (around line 172):

```go
	quickForkPrompt      *QuickForkPrompt      // For quick fork with name prompt
```

Add after it:

```go
	forkRequestSource    *session.Instance      // Source session for pending attach-mode fork
```

- [ ] **Step 2: Revert `f` hotkey to original behavior**

Replace the `case "f":` block (around line 5129-5144):

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

With:

```go
	case "f":
		// Quick fork session (same title with " (fork)" suffix)
		if h.cursor < len(h.flatItems) {
			item := h.flatItems[h.cursor]
			if item.Type == session.ItemTypeSession && item.Session != nil {
				if h.hasActiveAnimation(item.Session.ID) {
					h.setError(fmt.Errorf("session is starting, please wait..."))
					return h, nil
				}
				if item.Session.CanFork() {
					return h, h.quickForkSession(item.Session)
				}
			}
		}
		return h, nil
```

- [ ] **Step 3: Restore quickForkSession method**

Add this method (it was deleted in our earlier Task 2). Place it near `forkSessionCmd` (around line 6630):

```go
// quickForkSession performs a quick fork with default title suffix " (fork)"
func (h *Home) quickForkSession(source *session.Instance) tea.Cmd {
	if source == nil {
		return nil
	}
	title := source.Title + " (fork)"
	groupPath := source.GroupPath
	return h.forkSessionCmd(source, title, groupPath)
}
```

- [ ] **Step 4: Add fork_request file check in tea.Exec callback**

In `attachSession()`, find the tea.Exec callback (around line 7168-7187). After the `tab_switch_request` check block (which ends with `h.instancesMu.RUnlock()`), and BEFORE the `// No switch request — normal detach` comment, add:

```go
			// Check for fork request (Alt+F was pressed)
			forkFile := filepath.Join(homeDir, ".agent-deck", "fork_request")
			forkData, forkErr := os.ReadFile(forkFile)
			if forkErr == nil && len(forkData) > 0 {
				sourceID := strings.TrimSpace(string(forkData))
				_ = os.Remove(forkFile)
				h.cleanupTabStrip(tmuxSess.Name)
				h.isAttaching.Store(false)
				return forkRequestMsg{sourceID: sourceID}
			}
```

- [ ] **Step 5: Handle forkRequestMsg in Update()**

Find the `case QuickForkMsg:` handler (line 3084). Add BEFORE it:

```go
	case forkRequestMsg:
		// Alt+F was pressed in attach mode — find source and show prompt
		h.instancesMu.RLock()
		for _, inst := range h.instances {
			if inst.ID == msg.sourceID {
				h.forkRequestSource = inst
				break
			}
		}
		h.instancesMu.RUnlock()
		if h.forkRequestSource != nil && h.forkRequestSource.CanFork() {
			h.quickForkPrompt.SetWidth(h.width)
			h.quickForkPrompt.Show()
		}
		return h, nil
```

- [ ] **Step 6: Modify QuickForkMsg handler to use forkRequestSource**

Replace the `case QuickForkMsg:` handler (lines 3084-3138) with:

```go
	case QuickForkMsg:
		h.quickForkPrompt.Hide()

		// Determine source: forkRequestSource (attach mode) or cursor selection (Home)
		var source *session.Instance
		if h.forkRequestSource != nil {
			source = h.forkRequestSource
			h.forkRequestSource = nil
		} else if h.cursor < len(h.flatItems) {
			item := h.flatItems[h.cursor]
			if item.Type == session.ItemTypeSession && item.Session != nil && item.Session.CanFork() {
				source = item.Session
			}
		}

		if source == nil {
			h.quickForkPrompt.SetError("no forkable session selected")
			h.quickForkPrompt.Show()
			return h, nil
		}

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
```

- [ ] **Step 7: Modify QuickForkCancelMsg handler for re-attach**

Replace the `case QuickForkCancelMsg:` handler (lines 3140-3142) with:

```go
	case QuickForkCancelMsg:
		h.quickForkPrompt.Hide()
		if h.forkRequestSource != nil {
			source := h.forkRequestSource
			h.forkRequestSource = nil
			return h, h.attachSession(source)
		}
		return h, nil
```

- [ ] **Step 8: Auto-attach after fork completes**

In the `sessionForkedMsg` handler, find the block after auto-selecting the cursor (around line 3200, after `h.syncViewport()`). After `h.pendingTabSwitchID = msg.instance.ID`, add:

```go
			// Auto-attach to the newly forked session
			return h, h.attachSession(msg.instance)
```

Note: This replaces the `return h, h.fetchPreview(...)` that would otherwise follow. The session will be attached immediately, and preview fetching happens on return from attach. Make sure the `forceSaveInstances()` call happens BEFORE the return. Check the exact flow — if `forceSaveInstances()` is after this point, move the return to after it.

Read the full handler carefully to place the return correctly. The key: save must happen before attach. The auto-attach return should be the LAST thing, replacing the existing return at the end of the success block.

- [ ] **Step 9: Verify build and tests**

Run: `cd /Users/andreyvashchenko/projects/agent-deck-fork && go build ./... && go test ./internal/ui/ -count=1 2>&1 | tail -5`
Expected: Build OK, all tests pass

- [ ] **Step 10: Commit**

```bash
git add internal/ui/home.go
git commit -m "feat: handle fork-request from attach mode, revert f to instant fork, auto-attach after fork"
```

---

### Task 5: Manual Testing

- [ ] **Step 1: Build**

```bash
cd /Users/andreyvashchenko/projects/agent-deck-fork && go build -o agent-deck ./cmd/agent-deck/
```

- [ ] **Step 2: Test Alt+F (happy path)**

1. Start agent-deck, attach to a session (Enter)
2. Press Alt+F
3. Verify: detaches, shows "Fork: " prompt
4. Type "New feature", press Enter
5. Verify: creates worktree, forks session, auto-attaches to new session

- [ ] **Step 3: Test Alt+F cancel**

1. Attach to a session
2. Press Alt+F
3. Press Esc in the prompt
4. Verify: re-attaches to the original session

- [ ] **Step 4: Test Alt+U**

1. Attach to a session
2. Press Alt+U
3. Verify: stays in session (no detach)
4. Detach manually (Ctrl+Q)
5. Verify: tab shows unread marker (✓)

- [ ] **Step 5: Test `f` in Home (reverted)**

1. From Home list, select a session
2. Press `f`
3. Verify: instant fork with "(fork)" suffix, no prompt

- [ ] **Step 6: Test `F` in Home (unchanged)**

1. Press Shift+F
2. Verify: full ForkDialog opens as before
