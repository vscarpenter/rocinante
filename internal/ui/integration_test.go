package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/vscarpenter/rocinante/internal/config"
	"github.com/vscarpenter/rocinante/internal/fleet"
)

// TestEndToEndStoreToModel wires a real store to the model and proves the live
// path: a report writing a file reaches the store's channel, and folding that
// message into the model makes the agent appear in the rendered view. This is
// the same path the running program follows, minus the terminal.
func TestEndToEndStoreToModel(t *testing.T) {
	dir := t.TempDir()
	store, err := fleet.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	cfg := config.Default()
	cfg.Fleet.Dir = dir
	m := newModel(cfg, store)
	m.width, m.height = 120, 40
	if len(m.snapshot.Agents) != 0 {
		t.Fatalf("expected an empty fleet at start, got %d agents", len(m.snapshot.Agents))
	}

	// Simulate a report writing an agent file.
	now := time.Now()
	agent := fleet.Status{
		Schema: fleet.SchemaVersion, ID: "live", Name: "Live Agent",
		Kind: fleet.KindCron, State: fleet.StateRunning, Task: "working",
		Since: now, Heartbeat: now,
	}
	if err := fleet.Write(dir, agent); err != nil {
		t.Fatalf("write agent: %v", err)
	}

	// Drain the store until the agent shows up, then fold it into the model the
	// way the listen-and-reissue command does.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case snap := <-store.Updates():
			updated, _ := m.Update(fleetMsg(snap))
			m = updated.(model)
			if len(m.snapshot.Agents) == 1 && m.snapshot.Agents[0].ID == "live" {
				if view := m.View(); !strings.Contains(view, "Live Agent") {
					t.Fatalf("agent should appear in the view, got:\n%s", view)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the agent to reach the model")
		}
	}
}
