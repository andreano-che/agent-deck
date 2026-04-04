package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// tabStripAnimTickMsg advances animation frames.
type tabStripAnimTickMsg struct{}

// tabStripRefreshMsg triggers data reload from SQLite.
type tabStripRefreshMsg struct{}

// TabStripApp is a lightweight Bubble Tea program that renders
// a standalone tab strip, designed to run in a tmux split pane.
type TabStripApp struct {
	tabStrip  *TabStripModel
	db        *statedb.StateDB
	currentID string
	tabFile   string // ~/.agent-deck/tab_current
	width     int
	height    int
	err       error
}

// NewTabStripApp creates a new standalone tab strip application.
func NewTabStripApp(dbPath, currentID string) (*TabStripApp, error) {
	// If the given path has no tables, try profiles/default/state.db
	db, err := statedb.Open(dbPath)
	if err != nil {
		return nil, err
	}
	// Check if this DB has the instances table
	rows, checkErr := db.LoadInstances()
	if checkErr != nil || len(rows) == 0 {
		_ = db.Close()
		// Try profile path
		profileDB := filepath.Join(filepath.Dir(dbPath), "profiles", "default", "state.db")
		db, err = statedb.Open(profileDB)
		if err != nil {
			return nil, err
		}
	}

	homeDir, _ := os.UserHomeDir()
	tabFile := filepath.Join(homeDir, ".agent-deck", "tab_current")

	ts := NewTabStrip("horizontal", 0, false)

	app := &TabStripApp{
		tabStrip:  ts,
		db:        db,
		currentID: currentID,
		tabFile:   tabFile,
		height:    24,
	}

	return app, nil
}

// Init implements tea.Model.
func (a *TabStripApp) Init() tea.Cmd {
	// Load initial data and start ticks
	return tea.Batch(
		a.loadInstances,
		a.animTick(),
		a.refreshTick(),
	)
}

// Update implements tea.Model.
func (a *TabStripApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			idx := a.tabIndexAtPosition(msg.X, msg.Y)
			if idx >= 0 && idx < len(a.tabStrip.instances) {
				a.tabStrip.SelectTab(idx)
				a.currentID = a.tabStrip.instances[idx].ID
				a.writeTabSwitch(a.currentID)
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tabStripAnimTickMsg:
		a.tabStrip.Tick()
		return a, a.animTick()

	case tabStripRefreshMsg:
		return a, tea.Batch(
			a.loadInstances,
			a.refreshTick(),
		)

	case instancesLoadedMsg:
		a.tabStrip.UpdateInstances(msg.instances)
		// Update unread state directly from SQLite acknowledged flags
		a.tabStrip.UpdateUnreadState(msg.ackMap)
		// Update selection based on currentID
		a.syncSelection()
	}

	return a, nil
}

// View implements tea.Model.
func (a *TabStripApp) View() string {
	if a.err != nil {
		return "error: " + a.err.Error()
	}
	// Horizontal layout uses width, vertical uses height
	if a.tabStrip.layout == TabStripHorizontal {
		return a.tabStrip.View(a.width)
	}
	return a.tabStrip.View(a.height)
}

// Close releases resources.
func (a *TabStripApp) Close() {
	if a.db != nil {
		_ = a.db.Close()
	}
}

// --- internal messages and commands ---

type instancesLoadedMsg struct {
	instances []*session.Instance
	ackMap    map[string]bool // ID -> acknowledged (from SQLite)
}

func (a *TabStripApp) loadInstances() tea.Msg {
	rows, err := a.db.LoadInstances()
	if err != nil {
		return instancesLoadedMsg{}
	}

	// Also check tab_current file for selection changes
	if data, err := os.ReadFile(a.tabFile); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			a.currentID = id
		}
	}

	// Read acknowledged state for unread markers
	statuses, _ := a.db.ReadAllStatuses()
	ackMap := make(map[string]bool, len(rows))

	instances := make([]*session.Instance, 0, len(rows))
	for _, r := range rows {
		inst := &session.Instance{
			ID:          r.ID,
			Title:       r.Title,
			ProjectPath: r.ProjectPath,
			Status:      session.Status(r.Status),
			Tool:        r.Tool,
			Order:       r.Order,
			GroupPath:   r.GroupPath,
			CreatedAt:   r.CreatedAt,
		}
		instances = append(instances, inst)
		// Build ack map directly from SQLite
		if sr, ok := statuses[r.ID]; ok {
			ackMap[r.ID] = sr.Acknowledged
		}
	}

	return instancesLoadedMsg{instances: instances, ackMap: ackMap}
}

func (a *TabStripApp) syncSelection() {
	if a.currentID == "" {
		return
	}
	for i, inst := range a.tabStrip.instances {
		if inst.ID == a.currentID {
			a.tabStrip.SelectTab(i)
			return
		}
	}
}

// tabIndexAtPosition maps a mouse click coordinate to a tab index.
// Returns -1 if the click is outside any tab.
func (a *TabStripApp) tabIndexAtPosition(x, y int) int {
	n := len(a.tabStrip.instances)
	if n == 0 {
		return -1
	}

	if a.tabStrip.layout == TabStripHorizontal {
		// Only respond on the tab text rows (row 0 = tab labels, row 1 = underline)
		if y > 1 {
			return -1
		}
		// Compute actual visual width of each rendered tab to match viewHorizontal.
		// Each tab is: " " + icon(with ANSI) + " " + name + " ", joined by " ".
		cursor := 0
		for i, inst := range a.tabStrip.instances {
			icon := a.tabStrip.statusIcon(inst.Status, inst.ID)
			color := statusColor(inst.Status)
			iconRendered := lipgloss.NewStyle().Foreground(color).Render(icon)

			tabWidth := a.width / n
			if tabWidth < 8 {
				tabWidth = 8
			}
			nameWidth := tabWidth - 4
			if nameWidth < 3 {
				nameWidth = 3
			}
			name := inst.Title
			if len(name) > nameWidth {
				name = name[:nameWidth]
			}

			tab := " " + iconRendered + " " + name + " "
			visWidth := lipgloss.Width(tab)

			if x >= cursor && x < cursor+visWidth {
				return i
			}
			cursor += visWidth + 1 // +1 for the " " separator between tabs
		}
		return -1
	}

	// Vertical: each tab is one row, in order from top
	if y < 0 || y >= n {
		return -1
	}
	return y
}

// writeTabSwitch writes the tab switch request files and detaches the tmux
// client so the main app picks up the switch (same mechanism as keyboard tab-switch).
func (a *TabStripApp) writeTabSwitch(id string) {
	if a.tabFile == "" {
		return
	}
	dir := filepath.Dir(a.tabFile)
	_ = os.WriteFile(a.tabFile, []byte(id), 0644)
	_ = os.WriteFile(filepath.Join(dir, "tab_switch_request"), []byte(id), 0644)
	_ = exec.Command("tmux", "detach-client").Run()
}

func (a *TabStripApp) animTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return tabStripAnimTickMsg{}
	})
}

func (a *TabStripApp) refreshTick() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return tabStripRefreshMsg{}
	})
}
