package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vscarpenter/rocinante/internal/config"
	"github.com/vscarpenter/rocinante/internal/fleet"
)

// freshAgent builds a running agent whose heartbeat is now, so it reads fresh.
func freshAgent(id, name string, now time.Time) fleet.Status {
	return fleet.Status{
		Schema: fleet.SchemaVersion, ID: id, Name: name,
		Kind: fleet.KindCron, State: fleet.StateRunning, Task: "doing " + id,
		Since: now.Add(-5 * time.Minute), Heartbeat: now,
	}
}

func sizedModel(now time.Time, agents ...fleet.Status) model {
	m := model{cfg: config.Default(), now: now, width: 120, height: 40}
	return m.applySnapshot(fleet.Snapshot{Agents: agents})
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestViewTooSmallShowsFriendlyMessage(t *testing.T) {
	m := model{cfg: config.Default(), now: t0, width: 50, height: 12}
	view := m.View()
	if !strings.Contains(view, "bigger window") {
		t.Errorf("expected a resize hint for a small window, got:\n%s", view)
	}
}

func TestViewRendersFleetAndLog(t *testing.T) {
	m := sizedModel(t0, freshAgent("gh-watch", "GitHub Watcher", t0))
	view := m.View()

	for _, want := range []string{"ROCINANTE", "FLEET", "SHIP'S LOG", "GitHub Watcher", "[q] quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q\n---\n%s", want, view)
		}
	}
}

func TestViewMarksStaleAndOffline(t *testing.T) {
	stale := freshAgent("a", "Agent A", t0)
	stale.Heartbeat = t0.Add(-2 * time.Minute) // 120s silence: past 90s stale
	offline := freshAgent("b", "Agent B", t0)
	offline.Heartbeat = t0.Add(-10 * time.Minute) // 600s silence: past 300s offline

	m := sizedModel(t0, stale, offline)
	view := m.View()

	if !strings.Contains(view, "(stale)") {
		t.Errorf("expected a stale marker in the view:\n%s", view)
	}
	if !strings.Contains(view, "offline") {
		t.Errorf("expected an offline label in the view:\n%s", view)
	}
}

func TestViewRendersAtExactMinimumSize(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	m.width, m.height = minWidth, minHeight
	if view := m.View(); !strings.Contains(view, "FLEET") {
		t.Errorf("min-size view should still render the Fleet panel, got:\n%s", view)
	}
}

func TestUpdateQuitsOnQ(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should produce a tea.QuitMsg")
	}
}

func TestUpdateTabCyclesFocus(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	if m.focus != panelFleet {
		t.Fatalf("focus should start on the fleet panel, got %v", m.focus)
	}
	next, _ := m.Update(key("tab"))
	if next.(model).focus != panelLog {
		t.Errorf("tab should move focus to the log panel, got %v", next.(model).focus)
	}
}

func TestUpdateArrowsMoveAndClampCursor(t *testing.T) {
	m := sizedModel(t0,
		freshAgent("a", "A", t0),
		freshAgent("b", "B", t0),
		freshAgent("c", "C", t0),
	)

	next, _ := m.Update(key("down"))
	if next.(model).cursor != 1 {
		t.Fatalf("down should move cursor to 1, got %d", next.(model).cursor)
	}

	// Push past the end; it must clamp at the last index.
	for i := 0; i < 5; i++ {
		next, _ = next.(model).Update(key("down"))
	}
	if got := next.(model).cursor; got != 2 {
		t.Errorf("cursor should clamp at last index 2, got %d", got)
	}

	next, _ = next.(model).Update(key("up"))
	if got := next.(model).cursor; got != 1 {
		t.Errorf("up should move cursor to 1, got %d", got)
	}
}

func TestStatusSummaryNominalWhenHealthy(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	if got := m.statusSummary(); got != "all systems nominal" {
		t.Errorf("a healthy fleet should read nominal, got %q", got)
	}
}

func TestStatusSummaryFlagsBlockedAgent(t *testing.T) {
	blocked := freshAgent("a", "A", t0)
	blocked.State = fleet.StateBlocked
	m := sizedModel(t0, blocked, freshAgent("b", "B", t0))
	if got := m.statusSummary(); !strings.Contains(got, "1 blocked") {
		t.Errorf("the header should flag a blocked agent, got %q", got)
	}
}

func TestStatusSummaryCombinesProblems(t *testing.T) {
	blocked := freshAgent("a", "A", t0)
	blocked.State = fleet.StateBlocked
	offline := freshAgent("b", "B", t0)
	offline.Heartbeat = t0.Add(-10 * time.Minute) // 600s silence: past 300s offline
	m := sizedModel(t0, blocked, offline)
	got := m.statusSummary()
	if !strings.Contains(got, "1 blocked") || !strings.Contains(got, "1 offline") {
		t.Errorf("the header should surface both problems, got %q", got)
	}
}

func TestUpdateLExpandsLog(t *testing.T) {
	m := sizedModel(t0, freshAgent("gh-watch", "GitHub Watcher", t0))
	next, _ := m.Update(key("l"))
	if !next.(model).expandedLog {
		t.Fatal("l should expand the Ship's Log to full screen")
	}
}

func TestExpandedLogViewShowsHistoryAndBackHint(t *testing.T) {
	m := sizedModel(t0, freshAgent("gh-watch", "GitHub Watcher", t0))
	next, _ := m.Update(key("l"))
	view := next.(model).View()

	for _, want := range []string{"SHIP'S LOG", "gh-watch", "[esc] back"} {
		if !strings.Contains(view, want) {
			t.Errorf("expanded log view missing %q, got:\n%s", want, view)
		}
	}
}

func TestUpdateEscClosesExpandedLog(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	next, _ := m.Update(key("l"))
	next, _ = next.(model).Update(key("esc"))
	if next.(model).expandedLog {
		t.Error("esc should close the expanded log")
	}
}

func TestUpdateQuestionOpensHelp(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	next, _ := m.Update(key("?"))
	if !next.(model).showHelp {
		t.Fatal("? should open the help overlay")
	}
}

func TestHelpViewListsKeybindings(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	next, _ := m.Update(key("?"))
	view := next.(model).View()

	for _, want := range []string{"inspect", "Ship's Log", "refresh", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("help overlay missing %q, got:\n%s", want, view)
		}
	}
}

func TestUpdateEscClosesHelp(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	next, _ := m.Update(key("?"))
	next, _ = next.(model).Update(key("esc"))
	if next.(model).showHelp {
		t.Error("esc should close the help overlay")
	}
}

func TestApplySnapshotSeedsAndUpdatesLog(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	if len(m.logs) != 1 {
		t.Fatalf("initial snapshot should seed one log line, got %d", len(m.logs))
	}

	// A state change appends one more line.
	changed := freshAgent("a", "A", t0.Add(time.Minute))
	changed.State = fleet.StateIdle
	m = m.applySnapshot(fleet.Snapshot{Agents: []fleet.Status{changed}})
	if len(m.logs) != 2 {
		t.Errorf("a state change should append a log line, got %d", len(m.logs))
	}
}
