// Package fleet is the heart of Rocinante: the Fleet Status Contract, atomic
// reads and writes of per-agent status files, and the staleness derivation that
// infers when an agent has gone quiet.
//
// The contract is versioned and language agnostic. Every crew member speaks it,
// whether it is three lines of shell or a long-running agent on EC2.
package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion is the current contract version. The bridge refuses files from
// a future major so it never misreads a newer shape.
const SchemaVersion = 1

// Kind is how an agent runs. It maps to the file's "kind" field.
type Kind string

const (
	KindAlwaysOn   Kind = "always-on"
	KindCron       Kind = "cron"
	KindLaunchd    Kind = "launchd"
	KindClaudeCode Kind = "claude-code"
	KindOther      Kind = "other"
)

// Valid reports whether k is one of the contract's allowed kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindAlwaysOn, KindCron, KindLaunchd, KindClaudeCode, KindOther:
		return true
	default:
		return false
	}
}

// State is what an agent reports it is doing. It maps to the "state" field.
// Note that offline can also be derived from silence; see Liveness.
type State string

const (
	StateRunning State = "running"
	StateIdle    State = "idle"
	StateBlocked State = "blocked"
	StateError   State = "error"
	StateOffline State = "offline"
)

// Valid reports whether s is one of the contract's allowed states.
func (s State) Valid() bool {
	switch s {
	case StateRunning, StateIdle, StateBlocked, StateError, StateOffline:
		return true
	default:
		return false
	}
}

// Status is one agent's entry in the fleet. Its JSON form is the contract on
// disk, one file per agent at <dir>/<id>.json.
type Status struct {
	Schema      int            `json:"schema"`
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Kind        Kind           `json:"kind"`
	State       State          `json:"state"`
	Task        string         `json:"task,omitempty"`
	Detail      string         `json:"detail,omitempty"`
	Since       time.Time      `json:"since"`
	Heartbeat   time.Time      `json:"heartbeat"`
	TokensToday int64          `json:"tokens_today,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// Liveness is the agent's freshness derived from its heartbeat, independent of
// the state it last reported. An agent that crashes never writes offline, so
// the bridge infers it from silence.
type Liveness int

const (
	LiveFresh Liveness = iota
	LiveStale
	LiveOffline
)

// String renders a Liveness for logs and test output.
func (l Liveness) String() string {
	switch l {
	case LiveFresh:
		return "fresh"
	case LiveStale:
		return "stale"
	case LiveOffline:
		return "offline"
	default:
		return "unknown"
	}
}

// Liveness derives freshness from the heartbeat. Silence must exceed a
// threshold to cross it, so a heartbeat exactly at staleAfter is still fresh.
func (s Status) Liveness(now time.Time, staleAfter, offlineAfter time.Duration) Liveness {
	silence := now.Sub(s.Heartbeat)
	switch {
	case silence > offlineAfter:
		return LiveOffline
	case silence > staleAfter:
		return LiveStale
	default:
		return LiveFresh
	}
}

// Path returns the status file path for an agent id within dir.
func Path(dir, id string) string {
	return filepath.Join(dir, id+".json")
}

// Write encodes s and writes it to <dir>/<id>.json atomically. It writes a temp
// file in the same directory, then renames it into place. The rename is atomic
// on a single filesystem, so the bridge never reads a half-written file.
func Write(dir string, s Status) error {
	if s.ID == "" {
		return fmt.Errorf("fleet: cannot write a status with an empty id")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("fleet: create dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("fleet: encode status: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, s.ID+".*.tmp")
	if err != nil {
		return fmt.Errorf("fleet: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best effort cleanup if anything below fails before the rename.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fleet: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fleet: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fleet: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, Path(dir, s.ID)); err != nil {
		return fmt.Errorf("fleet: rename into place: %w", err)
	}
	return nil
}

// Read loads and validates a single status file. A malformed file or one from a
// future major schema returns an error, never a panic, so callers can render a
// visible error state instead of crashing.
func Read(path string) (Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Status{}, fmt.Errorf("fleet: read %s: %w", filepath.Base(path), err)
	}

	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{}, fmt.Errorf("fleet: parse %s: %w", filepath.Base(path), err)
	}
	if s.Schema > SchemaVersion {
		return Status{}, fmt.Errorf("fleet: %s uses schema %d, newer than supported %d", filepath.Base(path), s.Schema, SchemaVersion)
	}
	return s, nil
}
