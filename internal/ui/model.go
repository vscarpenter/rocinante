package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vscarpenter/rocinante/internal/config"
	"github.com/vscarpenter/rocinante/internal/fleet"
)

// maxLogLines caps the Ship's Log history kept in memory.
const maxLogLines = 200

// panel identifies a focusable region. Tab cycles through them.
type panel int

const (
	panelFleet panel = iota
	panelLog
	panelCount
)

// model is the immutable Bubble Tea state. Update folds messages into a new
// model; View renders it. Nothing here blocks or touches the filesystem
// directly, so the render loop stays single threaded.
type model struct {
	cfg   config.Config
	store *fleet.Store

	snapshot fleet.Snapshot
	byID     map[string]fleet.Status
	logs     []logEntry

	focus  panel
	cursor int

	width  int
	height int
	now    time.Time
}

// newModel builds the starting state from the store's current snapshot. The
// initial snapshot seeds the Ship's Log, so the bridge opens with the fleet
// already checked in rather than a blank log.
func newModel(cfg config.Config, store *fleet.Store) model {
	m := model{
		cfg:   cfg,
		store: store,
		now:   time.Now(),
	}
	if store != nil {
		m = m.applySnapshot(store.Snapshot())
	}
	return m
}

// Messages flowing through Update.
type (
	fleetMsg fleet.Snapshot
	tickMsg  time.Time
)

// Init kicks off the fleet listener and the staleness tick.
func (m model) Init() tea.Cmd {
	return tea.Batch(listenFleet(m.store), tickEvery())
}

// listenFleet blocks on the store's Updates channel and emits one fleetMsg.
// Update reissues it, so the bridge keeps listening without a second goroutine
// touching the model. This is the standard Bubble Tea pattern for an external
// event stream.
func listenFleet(store *fleet.Store) tea.Cmd {
	if store == nil {
		return nil
	}
	return func() tea.Msg {
		snap, ok := <-store.Updates()
		if !ok {
			return nil
		}
		return fleetMsg(snap)
	}
}

// tickEvery drives a one second clock so derived staleness and offline advance
// even when no file changes.
func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update folds one message into a new model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case fleetMsg:
		m = m.applySnapshot(fleet.Snapshot(msg))
		return m, listenFleet(m.store)

	case tickMsg:
		m.now = time.Time(msg)
		return m, tickEvery()
	}
	return m, nil
}

// handleKey wires the v0.1 keybindings: tab to cycle panels, arrows to move the
// fleet selection, and q to quit.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.focus = (m.focus + 1) % panelCount
		return m, nil
	case "shift+tab":
		m.focus = (m.focus + panelCount - 1) % panelCount
		return m, nil
	case "up", "k":
		if m.focus == panelFleet {
			m.cursor = clamp(m.cursor-1, 0, m.lastAgentIndex())
		}
		return m, nil
	case "down", "j":
		if m.focus == panelFleet {
			m.cursor = clamp(m.cursor+1, 0, m.lastAgentIndex())
		}
		return m, nil
	}
	return m, nil
}

// applySnapshot records a new fleet state, appends any meaningful log lines, and
// keeps the cursor in range.
func (m model) applySnapshot(snap fleet.Snapshot) model {
	next := agentMap(snap)
	entries := diffLog(m.byID, next)

	m.logs = append(m.logs, entries...)
	if len(m.logs) > maxLogLines {
		m.logs = m.logs[len(m.logs)-maxLogLines:]
	}

	m.snapshot = snap
	m.byID = next
	m.cursor = clamp(m.cursor, 0, m.lastAgentIndex())
	return m
}

// lastAgentIndex is the highest valid cursor position, or zero when empty.
func (m model) lastAgentIndex() int {
	if len(m.snapshot.Agents) == 0 {
		return 0
	}
	return len(m.snapshot.Agents) - 1
}

// liveness derives an agent's freshness using the configured thresholds and the
// model's current clock.
func (m model) liveness(s fleet.Status) fleet.Liveness {
	return s.Liveness(m.now, m.cfg.Fleet.StaleAfter, m.cfg.Fleet.OfflineAfter)
}

func agentMap(snap fleet.Snapshot) map[string]fleet.Status {
	out := make(map[string]fleet.Status, len(snap.Agents))
	for _, a := range snap.Agents {
		out[a.ID] = a
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
