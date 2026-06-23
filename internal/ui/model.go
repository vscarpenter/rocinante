package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vscarpenter/rocinante/internal/adapters/ccusage"
	"github.com/vscarpenter/rocinante/internal/adapters/github"
	"github.com/vscarpenter/rocinante/internal/config"
	"github.com/vscarpenter/rocinante/internal/fleet"
)

// maxLogLines caps the Ship's Log history kept in memory.
const maxLogLines = 200

// Per-fetch timeouts bound a hung adapter so it never stalls the bridge.
const (
	reactorTimeout = 12 * time.Second
	commsTimeout   = 20 * time.Second
)

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

	reactor    *ccusage.Reactor
	reactorErr string
	comms      *github.Comms
	commsErr   string

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
	fleetMsg       fleet.Snapshot
	tickMsg        time.Time
	reactorMsg     ccusage.Reactor
	commsMsg       github.Comms
	reactorTickMsg struct{}
	commsTickMsg   struct{}
	errMsg         struct{ source, text string }
)

// Init kicks off the fleet listener, the staleness tick, and the first poll of
// each enabled adapter.
func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{listenFleet(m.store), tickEvery()}
	if c := fetchReactor(m.cfg.Reactor); c != nil {
		cmds = append(cmds, c)
	}
	if c := fetchComms(m.cfg.Comms); c != nil {
		cmds = append(cmds, c)
	}
	return tea.Batch(cmds...)
}

// fetchReactor polls ccusage once under a timeout. It returns nil when the
// Reactor is disabled, so Init and refresh can skip it.
func fetchReactor(cfg config.ReactorConfig) tea.Cmd {
	if !cfg.Enabled {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), reactorTimeout)
		defer cancel()
		r, err := ccusage.Fetch(ctx, cfg.Command, cfg.Args)
		if err != nil {
			return errMsg{source: "reactor", text: err.Error()}
		}
		return reactorMsg(r)
	}
}

// fetchComms polls gh once under a timeout. It returns nil when Comms is
// disabled or no repos are configured.
func fetchComms(cfg config.CommsConfig) tea.Cmd {
	if !cfg.Enabled || len(cfg.Repos) == 0 {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), commsTimeout)
		defer cancel()
		c, err := github.Fetch(ctx, cfg.Repos)
		if err != nil {
			return errMsg{source: "comms", text: err.Error()}
		}
		return commsMsg(c)
	}
}

func reactorTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return reactorTickMsg{} })
}

func commsTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return commsTickMsg{} })
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

	case reactorMsg:
		r := ccusage.Reactor(msg)
		m.reactor = &r
		m.reactorErr = ""
		return m, reactorTick(m.cfg.Reactor.Interval)

	case commsMsg:
		c := github.Comms(msg)
		m.comms = &c
		m.commsErr = ""
		return m, commsTick(m.cfg.Comms.Interval)

	case errMsg:
		return m.handleErr(msg)

	case reactorTickMsg:
		return m, fetchReactor(m.cfg.Reactor)

	case commsTickMsg:
		return m, fetchComms(m.cfg.Comms)
	}
	return m, nil
}

// handleErr records an adapter error and schedules its next attempt, so a
// failing source shows a line and keeps retrying rather than going dark.
func (m model) handleErr(msg errMsg) (tea.Model, tea.Cmd) {
	switch msg.source {
	case "reactor":
		m.reactorErr = msg.text
		return m, reactorTick(m.cfg.Reactor.Interval)
	case "comms":
		m.commsErr = msg.text
		return m, commsTick(m.cfg.Comms.Interval)
	}
	return m, nil
}

// handleKey wires the v0.1 keybindings: tab to cycle panels, arrows to move the
// fleet selection, and q to quit.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		return m, tea.Batch(fetchReactor(m.cfg.Reactor), fetchComms(m.cfg.Comms))
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
