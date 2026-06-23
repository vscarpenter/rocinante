package ui

import (
	"testing"
	"time"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

var t0 = time.Date(2026, 6, 22, 14, 0, 0, 0, time.UTC)

func TestCompactDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{90 * time.Second, "1m"},
		{59 * time.Minute, "59m"},
		{2 * time.Hour, "2h"},
		{25 * time.Hour, "1d"},
	}
	for _, tc := range cases {
		if got := compactDuration(tc.d); got != tc.want {
			t.Errorf("compactDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestDiffLogNewAgentUsesTask(t *testing.T) {
	next := map[string]fleet.Status{
		"a": {ID: "a", State: fleet.StateRunning, Task: "booting", Heartbeat: t0},
	}
	entries := diffLog(nil, next)
	if len(entries) != 1 {
		t.Fatalf("want 1 entry for a new agent, got %d", len(entries))
	}
	if entries[0].agentID != "a" || entries[0].text != "booting" {
		t.Errorf("new agent entry = %+v, want agent a text booting", entries[0])
	}
}

func TestDiffLogDetailChangeWins(t *testing.T) {
	prev := map[string]fleet.Status{"a": {ID: "a", State: fleet.StateRunning, Heartbeat: t0}}
	next := map[string]fleet.Status{"a": {ID: "a", State: fleet.StateIdle, Detail: "found 1 new PR", Heartbeat: t0.Add(time.Minute)}}

	entries := diffLog(prev, next)
	if len(entries) != 1 || entries[0].text != "found 1 new PR" {
		t.Errorf("expected the new detail to be logged, got %+v", entries)
	}
}

func TestDiffLogStateChangeWithoutDetail(t *testing.T) {
	prev := map[string]fleet.Status{"a": {ID: "a", State: fleet.StateRunning, Heartbeat: t0}}
	next := map[string]fleet.Status{"a": {ID: "a", State: fleet.StateIdle, Heartbeat: t0.Add(time.Minute)}}

	entries := diffLog(prev, next)
	if len(entries) != 1 || entries[0].text != "is now idle" {
		t.Errorf("expected a state-change line, got %+v", entries)
	}
}

func TestDiffLogIgnoresHeartbeatOnlyChange(t *testing.T) {
	base := fleet.Status{ID: "a", State: fleet.StateRunning, Task: "x", Detail: "y", Heartbeat: t0}
	prev := map[string]fleet.Status{"a": base}
	moved := base
	moved.Heartbeat = t0.Add(time.Minute)
	next := map[string]fleet.Status{"a": moved}

	if entries := diffLog(prev, next); len(entries) != 0 {
		t.Errorf("a heartbeat-only change should not log, got %+v", entries)
	}
}

func TestDiffLogSortsByHeartbeat(t *testing.T) {
	next := map[string]fleet.Status{
		"late":  {ID: "late", State: fleet.StateRunning, Task: "l", Heartbeat: t0.Add(2 * time.Minute)},
		"early": {ID: "early", State: fleet.StateRunning, Task: "e", Heartbeat: t0},
	}
	entries := diffLog(nil, next)
	if len(entries) != 2 || entries[0].agentID != "early" || entries[1].agentID != "late" {
		t.Errorf("entries should be ordered by heartbeat, got %+v", entries)
	}
}
