package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

func TestBuildInspectShowsAllFields(t *testing.T) {
	now := time.Date(2026, 6, 22, 15, 0, 0, 0, time.UTC)
	s := fleet.Status{
		Schema: 1, ID: "gh-watch", Name: "GitHub Watcher", Kind: fleet.KindCron,
		State: fleet.StateRunning, Task: "Polling org repos for new PRs",
		Detail:      "Last pass found 1 new PR (#57) and merged nothing",
		Since:       now.Add(-33 * time.Minute),
		Heartbeat:   now.Add(-10 * time.Second),
		TokensToday: 41200000,
		Meta:        map[string]any{"pid": float64(4823), "cwd": "/Users/vinny/dev/foo"},
	}

	out := buildInspect(s, fleet.LiveFresh, now, 60)

	for _, want := range []string{
		"gh-watch", "GitHub Watcher", "cron", "running",
		"Task", "Polling org repos for new PRs",
		"Detail", "found 1 new PR (#57)",
		"Meta", "pid", "cwd", "/Users/vinny/dev/foo",
		"41.2M",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inspect content missing %q:\n%s", want, out)
		}
	}
}

func TestEnterOpensAndEscClosesInspect(t *testing.T) {
	a := freshAgent("gh-watch", "GitHub Watcher", t0)
	a.Detail = "found 1 new PR (#57)"
	m := sizedModel(t0, a)

	opened, _ := m.Update(key("enter"))
	mo := opened.(model)
	if !mo.inspecting {
		t.Fatal("enter should open the inspect view")
	}
	view := mo.View()
	if !strings.Contains(view, "inspect: GitHub Watcher") {
		t.Errorf("inspect header missing:\n%s", view)
	}
	if !strings.Contains(view, "found 1 new PR (#57)") {
		t.Errorf("inspect detail missing:\n%s", view)
	}

	closed, _ := mo.Update(key("esc"))
	if closed.(model).inspecting {
		t.Error("esc should close the inspect view")
	}
}

func TestBuildInspectReflectsDerivedState(t *testing.T) {
	now := time.Date(2026, 6, 22, 15, 0, 0, 0, time.UTC)
	s := fleet.Status{
		ID: "a", Name: "A", Kind: fleet.KindCron, State: fleet.StateRunning,
		Since: now.Add(-time.Hour), Heartbeat: now.Add(-time.Hour),
	}
	out := buildInspect(s, fleet.LiveOffline, now, 60)
	if !strings.Contains(out, "offline") {
		t.Errorf("a derived-offline agent should read offline:\n%s", out)
	}
}
