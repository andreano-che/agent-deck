# Attach-Mode Hotkeys: Quick Fork (Alt+F) and Mark Unread (Alt+U)

**Date:** 2026-04-03
**Status:** Draft

## Problem

The Quick Fork and Mark Unread features only work from the Home list view. When attached to a Claude Code session (the primary working state), pressing `f` or `u` just types into the terminal. Users need these features most while working inside a session — "I'm fixing a bug but need to fork for a parallel task" or "I want to flag this tab as unread for later."

## Solution

Add tmux root key bindings for Alt+F and Alt+U during attach mode, following the exact pattern used by Alt+1-9 tab switching:

- **Alt+F**: Detach → show name prompt in Home → create worktree + fork → re-attach to new session
- **Alt+U**: Background CLI command (no detach) — marks current session as unread in SQLite

Also revert the `f` hotkey in Home back to its original instant-fork behavior.

## Design

### Alt+F Flow

```
User inside Claude Code session "Fix auth bug"
    │
    ▼  Alt+F
tmux root binding fires → runs: agent-deck fork-request --session=abc123
    │
    ▼
CLI writes ~/.agent-deck/fork_request with source session ID
CLI runs: tmux detach-client
    │
    ▼
Agent Deck Home picks up fork_request file in tea.Exec callback
Returns forkRequestMsg{sourceID: "abc123"}
    │
    ▼
Home.Update handles forkRequestMsg:
  - Finds source instance by ID
  - Opens QuickForkPrompt
  - Stores source instance reference for use after prompt submission
    │
    ▼
┌─────────────────────────────────────────┐
│  Fork: [Add user settings            ] │
└─────────────────────────────────────────┘
    │
    ▼  Enter
QuickForkMsg handler:
  1. slugify name → branch fork/add-user-settings
  2. Deduplicate branch if exists
  3. Create worktree
  4. Fork session with worktree
  5. Attach to new session (not just select tab)
    │
    ▼
User is inside new Claude Code session "Add user settings"

Esc → cancel, re-attach to original session
```

### Alt+U Flow

```
User inside Claude Code session
    │
    ▼  Alt+U
tmux root binding fires → runs: agent-deck mark-unread --session=abc123
    │
    ▼
CLI opens SQLite, sets acknowledged=false for session abc123
    │
    ▼
Done. No detach. User stays in session.
Tab strip updates on next tick (✓ marker appears).
```

### Revert `f` Hotkey in Home

Restore the original `quickForkSession()` method and revert the `case "f":` handler to call it (instant fork with "(fork)" suffix, no prompt, no worktree).

The `QuickForkPrompt` component stays — it's now used exclusively by the `forkRequestMsg` handler.

## Architecture

### New CLI Commands

**`fork-request`** (`cmd/agent-deck/fork_request_cmd.go`, ~30 lines)

Follows `tab_switch_cmd.go` pattern exactly:
- Takes `--session=<id>` flag
- Writes session ID to `~/.agent-deck/fork_request`
- Runs `tmux detach-client`
- All errors swallowed (never disrupts terminal)

**`mark-unread`** (`cmd/agent-deck/mark_unread_cmd.go`, ~30 lines)

- Takes `--session=<id>` flag
- Opens SQLite (`profiles/default/state.db`)
- Sets `acknowledged=false` for the session
- No detach, no output, errors swallowed

### New tmux Bindings (in `attachSession()`)

Added alongside existing Alt+1-9 bindings (home.go ~line 7150):

```go
// Alt+F for quick fork
forkCmd := fmt.Sprintf("%s fork-request --session=%s", exe, inst.ID)
_ = exec.Command("tmux", "bind-key", "-T", "root", "M-f",
    "run-shell", forkCmd).Run()

// Alt+U for mark unread
unreadCmd := fmt.Sprintf("%s mark-unread --session=%s", exe, inst.ID)
_ = exec.Command("tmux", "bind-key", "-T", "root", "M-u",
    "run-shell", unreadCmd).Run()
```

Cleanup in `cleanupTabStrip()`:
```go
_ = exec.Command("tmux", "unbind-key", "-T", "root", "M-f").Run()
_ = exec.Command("tmux", "unbind-key", "-T", "root", "M-u").Run()
// Also clean up fork_request file
_ = os.Remove(filepath.Join(homeDir, ".agent-deck", "fork_request"))
```

### Changes to Home (`internal/ui/home.go`)

**New message type:**
```go
type forkRequestMsg struct {
    sourceID string
}
```

**New field:**
```go
forkRequestSource *session.Instance // source session for pending fork request
```

**tea.Exec callback** (in `attachSession()`, ~line 7168):

After checking for `tab_switch_request`, also check for `fork_request`:
```go
forkFile := filepath.Join(homeDir, ".agent-deck", "fork_request")
data, readErr := os.ReadFile(forkFile)
if readErr == nil && len(data) > 0 {
    sourceID := strings.TrimSpace(string(data))
    _ = os.Remove(forkFile)
    h.cleanupTabStrip(tmuxSess.Name)
    h.isAttaching.Store(false)
    return forkRequestMsg{sourceID: sourceID}
}
```

**forkRequestMsg handler** (in `Update()`):

```go
case forkRequestMsg:
    // Find source instance
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

**QuickForkMsg handler** (modify existing):

When `h.forkRequestSource` is set, use it as the source instead of cursor selection. Clear `forkRequestSource` after initiating the fork.

**sessionForkedMsg handler** (modify existing):

After a successful fork, if the fork originated from attach mode (detected by checking if the new session was just created and we're not currently attached), auto-attach to the new session. The simplest approach: always auto-attach to forked sessions — this is the desired UX regardless of whether the fork came from Home or attach mode. On Esc (QuickForkCancelMsg), re-attach to `forkRequestSource`.

**QuickForkCancelMsg handler** (modify existing):

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

### Revert `f` Hotkey

Restore `quickForkSession()` method:
```go
func (h *Home) quickForkSession(source *session.Instance) tea.Cmd {
    if source == nil {
        return nil
    }
    title := source.Title + " (fork)"
    groupPath := source.GroupPath
    return h.forkSessionCmd(source, title, groupPath)
}
```

Revert `case "f":` to call `h.quickForkSession(item.Session)`.

## What Changes vs. Previous Implementation

| Aspect | Before (Task 1-3) | Now |
|--------|-------------------|-----|
| `f` in Home | Opens QuickForkPrompt | Instant fork with "(fork)" suffix (original) |
| Alt+F in attach | Not available | Detach → prompt → worktree fork → re-attach |
| Alt+U in attach | Not available | Background mark-unread (no detach) |
| QuickForkPrompt | Used by Home `f` key | Used by forkRequestMsg only |
| Tab auto-switch | pendingTabSwitchID | attachSession() for fork (direct re-attach) |

## What Does NOT Change

- `QuickForkPrompt` component (`quick_fork_prompt.go`) — stays as-is
- `slugify()` function — stays as-is
- `ForkDialog` — stays as-is
- `F` (shift+f) in Home — still opens full ForkDialog
- Alt+1-9 tab switching — unchanged
- `pendingTabSwitchID` — stays for non-attach fork scenarios

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Alt+F on non-forkable session | Detaches but forkRequestSource is nil → prompt doesn't open, stays in Home |
| Not a git repo | Fork without worktree (same directory) |
| Branch exists | Auto-suffix: `-2`, `-3`, etc. |
| Worktree creation fails | Error shown in prompt |
| Empty name | Prompt stays open |
| Esc from prompt | Re-attach to original session |
| Alt+U fails to write SQLite | Silently ignored (CLI errors swallowed) |
| fork_request file already exists | Overwritten (last writer wins) |

## Tests

### `fork_request_cmd_test.go`
- Writes fork_request file with correct session ID
- Handles missing --session flag gracefully

### `mark_unread_cmd_test.go`
- Sets acknowledged=false in SQLite
- Handles missing --session flag gracefully

### Integration (in `home_test.go`)
- `forkRequestMsg` with valid source opens prompt
- `forkRequestMsg` with invalid source doesn't open prompt
- `QuickForkCancelMsg` with forkRequestSource re-attaches to source
- `QuickForkMsg` with forkRequestSource uses source (not cursor)
