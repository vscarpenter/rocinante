package fleet

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSnapshotEmptyOrMissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	snap := LoadSnapshot(missing)
	if len(snap.Agents) != 0 || len(snap.Errors) != 0 {
		t.Fatalf("missing dir should yield empty snapshot, got %d agents, %d errors", len(snap.Agents), len(snap.Errors))
	}
}

func TestLoadSnapshotSortsValidAgentsByID(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"zeta", "alpha", "mid"} {
		s := sampleStatus()
		s.ID = id
		if err := Write(dir, s); err != nil {
			t.Fatalf("write %s: %v", id, err)
		}
	}

	snap := LoadSnapshot(dir)
	if len(snap.Agents) != 3 {
		t.Fatalf("want 3 agents, got %d", len(snap.Agents))
	}
	got := []string{snap.Agents[0].ID, snap.Agents[1].ID, snap.Agents[2].ID}
	want := []string{"alpha", "mid", "zeta"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("agents not sorted by id: got %v, want %v", got, want)
		}
	}
}

func TestLoadSnapshotRecordsMalformedAsError(t *testing.T) {
	dir := t.TempDir()
	good := sampleStatus()
	good.ID = "good"
	if err := Write(dir, good); err != nil {
		t.Fatalf("write good: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{ broken"), 0o644); err != nil {
		t.Fatalf("write bad: %v", err)
	}

	snap := LoadSnapshot(dir)
	if len(snap.Agents) != 1 || snap.Agents[0].ID != "good" {
		t.Errorf("want one good agent, got %+v", snap.Agents)
	}
	if _, ok := snap.Errors["bad.json"]; !ok {
		t.Errorf("want a recorded error for bad.json, errors=%v", snap.Errors)
	}
}

func TestLoadSnapshotIgnoresNonJSONAndTempFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test.1234.tmp"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	snap := LoadSnapshot(dir)
	if len(snap.Agents) != 0 || len(snap.Errors) != 0 {
		t.Errorf("non-json and temp files should be ignored, got %+v", snap)
	}
}

// TestStoreEmitsSnapshotOnFileChange exercises the fsnotify wiring end to end:
// after a report writes a file, the store must publish a snapshot that includes
// the new agent. This is an integration test with a timeout.
func TestStoreEmitsSnapshotOnFileChange(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if got := len(store.Snapshot().Agents); got != 0 {
		t.Fatalf("expected empty initial snapshot, got %d agents", got)
	}

	s := sampleStatus()
	s.ID = "watched"
	if err := Write(dir, s); err != nil {
		t.Fatalf("write watched: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case snap := <-store.Updates():
			for _, a := range snap.Agents {
				if a.ID == "watched" {
					return // success
				}
			}
		case <-deadline:
			t.Fatal("timed out waiting for the store to publish the watched agent")
		}
	}
}
