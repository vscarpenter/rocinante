package fleet

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatusLiveness pins the staleness and offline boundaries. The spec says
// an agent goes stale or offline when silence "exceeds" the threshold, so the
// boundary itself is strictly fresh or stale, never the next tier.
func TestStatusLiveness(t *testing.T) {
	const (
		staleAfter   = 90 * time.Second
		offlineAfter = 300 * time.Second
	)
	now := time.Date(2026, 6, 22, 14, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		silence time.Duration
		want    Liveness
	}{
		{"zero silence is fresh", 0, LiveFresh},
		{"just under stale is fresh", 89 * time.Second, LiveFresh},
		{"exactly at stale threshold is fresh", 90 * time.Second, LiveFresh},
		{"just past stale is stale", 91 * time.Second, LiveStale},
		{"just under offline is stale", 299 * time.Second, LiveStale},
		{"exactly at offline threshold is stale", 300 * time.Second, LiveStale},
		{"just past offline is offline", 301 * time.Second, LiveOffline},
		{"far past offline is offline", time.Hour, LiveOffline},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Status{Heartbeat: now.Add(-tc.silence)}
			got := s.Liveness(now, staleAfter, offlineAfter)
			if got != tc.want {
				t.Fatalf("silence %v: got %v, want %v", tc.silence, got, tc.want)
			}
		})
	}
}

func TestKindValid(t *testing.T) {
	cases := []struct {
		kind Kind
		want bool
	}{
		{KindAlwaysOn, true},
		{KindCron, true},
		{KindLaunchd, true},
		{KindClaudeCode, true},
		{KindOther, true},
		{"", false},
		{"daemon", false},
		{"Cron", false},
	}
	for _, tc := range cases {
		if got := tc.kind.Valid(); got != tc.want {
			t.Errorf("Kind(%q).Valid() = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestStateValid(t *testing.T) {
	cases := []struct {
		state State
		want  bool
	}{
		{StateRunning, true},
		{StateIdle, true},
		{StateBlocked, true},
		{StateError, true},
		{StateOffline, true},
		{"", false},
		{"paused", false},
		{"Running", false},
	}
	for _, tc := range cases {
		if got := tc.state.Valid(); got != tc.want {
			t.Errorf("State(%q).Valid() = %v, want %v", tc.state, got, tc.want)
		}
	}
}

func sampleStatus() Status {
	return Status{
		Schema:      SchemaVersion,
		ID:          "gh-watch",
		Name:        "GitHub Watcher",
		Kind:        KindCron,
		State:       StateRunning,
		Task:        "Polling org repos for new PRs",
		Detail:      "Last pass found 1 new PR (#57)",
		Since:       time.Date(2026, 6, 22, 13, 1, 0, 0, time.UTC),
		Heartbeat:   time.Date(2026, 6, 22, 14, 55, 10, 0, time.UTC),
		TokensToday: 41200000,
		Meta:        map[string]any{"pid": float64(4823)},
	}
}

// TestWriteReadRoundTrip confirms a status survives a write then read intact.
func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := sampleStatus()

	if err := Write(dir, want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(filepath.Join(dir, "gh-watch.json"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.ID != want.ID || got.Name != want.Name || got.Kind != want.Kind ||
		got.State != want.State || got.Task != want.Task || got.Detail != want.Detail ||
		got.TokensToday != want.TokensToday || got.Schema != want.Schema {
		t.Errorf("scalar fields differ:\n got %+v\nwant %+v", got, want)
	}
	if !got.Since.Equal(want.Since) {
		t.Errorf("Since: got %v, want %v", got.Since, want.Since)
	}
	if !got.Heartbeat.Equal(want.Heartbeat) {
		t.Errorf("Heartbeat: got %v, want %v", got.Heartbeat, want.Heartbeat)
	}
}

// TestWriteCreatesDirAndIsAtomic confirms Write creates the fleet directory and
// leaves exactly one file, the final one, with no temp leftovers.
func TestWriteCreatesDirAndIsAtomic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fleet") // does not exist yet
	if err := Write(dir, sampleStatus()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected exactly one file, got %v", names)
	}
	if entries[0].Name() != "gh-watch.json" {
		t.Errorf("filename: got %q, want gh-watch.json", entries[0].Name())
	}
}

// TestWriteOverwritesCleanly confirms a second write fully replaces the first,
// which is the property the temp-plus-rename approach guarantees.
func TestWriteOverwritesCleanly(t *testing.T) {
	dir := t.TempDir()
	first := sampleStatus()
	if err := Write(dir, first); err != nil {
		t.Fatalf("Write first: %v", err)
	}

	second := sampleStatus()
	second.State = StateIdle
	second.Task = "Idle between passes"
	if err := Write(dir, second); err != nil {
		t.Fatalf("Write second: %v", err)
	}

	got, err := Read(filepath.Join(dir, "gh-watch.json"))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.State != StateIdle || got.Task != "Idle between passes" {
		t.Errorf("second write did not replace first: got state %q task %q", got.State, got.Task)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected one file after overwrite, got %d", len(entries))
	}
}

// TestReadMalformedReturnsError confirms a corrupt file becomes an error, not a
// panic, so the store can surface it as a visible error state.
func TestReadMalformedReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{ this is not json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("expected an error reading malformed json, got nil")
	}
}

// TestReadFutureSchemaReturnsError confirms the bridge refuses a file from a
// future major schema rather than misreading it.
func TestReadFutureSchemaReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "future.json")
	body := `{"schema":999,"id":"x","name":"X","kind":"cron","state":"running","since":"2026-06-22T13:01:00Z","heartbeat":"2026-06-22T14:55:10Z"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("expected an error reading a future schema, got nil")
	}
}
