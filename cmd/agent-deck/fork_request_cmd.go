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
