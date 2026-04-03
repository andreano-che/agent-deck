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
