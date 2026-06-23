package report

import (
	"testing"
	"time"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

// newOpts builds report Options, marking each named field as explicitly
// provided so the merge logic treats it as set rather than inherited.
func newOpts(id string, fields map[string]string) Options {
	o := Options{ID: id, Provided: map[string]bool{}}
	for k, v := range fields {
		switch k {
		case "name":
			o.Name = v
		case "kind":
			o.Kind = v
		case "state":
			o.State = v
		case "task":
			o.Task = v
		case "detail":
			o.Detail = v
		}
		o.Provided[k] = true
	}
	return o
}

var t0 = time.Date(2026, 6, 22, 14, 0, 0, 0, time.UTC)

func TestRunCreatesValidContractFile(t *testing.T) {
	dir := t.TempDir()
	o := newOpts("gh-watch", map[string]string{
		"name": "GitHub Watcher", "kind": "cron", "state": "running",
		"task": "Polling org repos for new PRs",
	})

	if err := run(dir, o, t0); err != nil {
		t.Fatalf("run: %v", err)
	}

	s, err := fleet.Read(fleet.Path(dir, "gh-watch"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if s.Schema != fleet.SchemaVersion {
		t.Errorf("schema: got %d, want %d", s.Schema, fleet.SchemaVersion)
	}
	if s.ID != "gh-watch" || s.Name != "GitHub Watcher" || s.Kind != fleet.KindCron ||
		s.State != fleet.StateRunning || s.Task != "Polling org repos for new PRs" {
		t.Errorf("fields not written as expected: %+v", s)
	}
	if !s.Heartbeat.Equal(t0) {
		t.Errorf("heartbeat: got %v, want %v", s.Heartbeat, t0)
	}
	if !s.Since.Equal(t0) {
		t.Errorf("since on create: got %v, want %v", s.Since, t0)
	}
}

func TestRunResetsSinceOnlyOnStateChange(t *testing.T) {
	dir := t.TempDir()
	t1 := t0.Add(1 * time.Minute)
	t2 := t1.Add(1 * time.Minute)

	if err := run(dir, newOpts("a", map[string]string{
		"name": "Agent A", "kind": "cron", "state": "running",
	}), t0); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Same state at t1: heartbeat advances, since holds.
	if err := run(dir, newOpts("a", map[string]string{"state": "running"}), t1); err != nil {
		t.Fatalf("update same state: %v", err)
	}
	s, _ := fleet.Read(fleet.Path(dir, "a"))
	if !s.Since.Equal(t0) {
		t.Errorf("since should stay at create time on unchanged state: got %v, want %v", s.Since, t0)
	}
	if !s.Heartbeat.Equal(t1) {
		t.Errorf("heartbeat should advance: got %v, want %v", s.Heartbeat, t1)
	}

	// Changed state at t2: since resets.
	if err := run(dir, newOpts("a", map[string]string{"state": "idle"}), t2); err != nil {
		t.Fatalf("update changed state: %v", err)
	}
	s, _ = fleet.Read(fleet.Path(dir, "a"))
	if !s.Since.Equal(t2) {
		t.Errorf("since should reset on state change: got %v, want %v", s.Since, t2)
	}
	if !s.Heartbeat.Equal(t2) {
		t.Errorf("heartbeat: got %v, want %v", s.Heartbeat, t2)
	}
}

func TestRunMergePreservesUnspecifiedFields(t *testing.T) {
	dir := t.TempDir()
	t1 := t0.Add(1 * time.Minute)

	if err := run(dir, newOpts("a", map[string]string{
		"name": "Agent A", "kind": "cron", "state": "running", "task": "first task",
	}), t0); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Report only state and detail. Name, kind, and task must survive.
	if err := run(dir, newOpts("a", map[string]string{
		"state": "idle", "detail": "found 1 new PR",
	}), t1); err != nil {
		t.Fatalf("update: %v", err)
	}

	s, _ := fleet.Read(fleet.Path(dir, "a"))
	if s.Name != "Agent A" || s.Kind != fleet.KindCron {
		t.Errorf("identity fields not preserved: %+v", s)
	}
	if s.Task != "first task" {
		t.Errorf("unspecified task should be preserved: got %q", s.Task)
	}
	if s.State != fleet.StateIdle || s.Detail != "found 1 new PR" {
		t.Errorf("provided fields not applied: state %q detail %q", s.State, s.Detail)
	}
}

func TestRunPreservesTokensWhenNotProvided(t *testing.T) {
	dir := t.TempDir()
	t1 := t0.Add(1 * time.Minute)

	create := newOpts("a", map[string]string{"name": "A", "kind": "cron", "state": "running"})
	create.Tokens = 41200000
	create.Provided["tokens"] = true
	if err := run(dir, create, t0); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update without tokens: the prior value must survive.
	if err := run(dir, newOpts("a", map[string]string{"state": "idle"}), t1); err != nil {
		t.Fatalf("update: %v", err)
	}
	s, _ := fleet.Read(fleet.Path(dir, "a"))
	if s.TokensToday != 41200000 {
		t.Errorf("tokens should be preserved: got %d", s.TokensToday)
	}
}

func TestRunRejectsInvalidEnums(t *testing.T) {
	dir := t.TempDir()
	badKind := newOpts("a", map[string]string{"name": "A", "kind": "daemon", "state": "running"})
	if err := run(dir, badKind, t0); err == nil {
		t.Error("expected error for invalid kind, got nil")
	}
	badState := newOpts("a", map[string]string{"name": "A", "kind": "cron", "state": "sleeping"})
	if err := run(dir, badState, t0); err == nil {
		t.Error("expected error for invalid state, got nil")
	}
}

func TestRunRejectsIncompleteCreate(t *testing.T) {
	dir := t.TempDir()
	// No name on a brand new agent cannot form a valid contract file.
	noName := newOpts("a", map[string]string{"kind": "cron", "state": "running"})
	if err := run(dir, noName, t0); err == nil {
		t.Error("expected error creating an agent without a name, got nil")
	}
}

func TestRunRejectsEmptyID(t *testing.T) {
	if err := run(t.TempDir(), Options{ID: "", Provided: map[string]bool{}}, t0); err == nil {
		t.Error("expected error for empty id, got nil")
	}
}
