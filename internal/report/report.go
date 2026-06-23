// Package report implements the "rocinante report" subcommand, which writes or
// merges one agent status file under the fleet directory, then exits.
//
// report is what makes the whole design cheap to adopt. A cron job, a Claude
// Code hook, or a remote agent calls it to announce what it is doing. None of
// them need to know the file format, the directory, or the schema version.
package report

import (
	"fmt"
	"time"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

// Options carries the values parsed from report's flags. Provided records which
// optional flags the caller actually set, so Run merges only those fields into
// an existing file rather than clobbering the rest.
type Options struct {
	ID       string
	Name     string
	Kind     string
	State    string
	Task     string
	Detail   string
	Tokens   int64
	Provided map[string]bool
}

// provided reports whether the caller explicitly set the named flag.
func (o Options) provided(name string) bool {
	return o.Provided[name]
}

// Run writes or merges the status file for opts.ID into dir, stamping the
// heartbeat to now.
func Run(dir string, opts Options) error {
	return run(dir, opts, time.Now().UTC())
}

// run is the testable core. It takes an explicit clock so tests can pin time
// without sleeping.
func run(dir string, opts Options, now time.Time) error {
	if opts.ID == "" {
		return fmt.Errorf("report: --id is required")
	}
	// Validate provided enums eagerly so the error names the bad value.
	if opts.provided("kind") && !fleet.Kind(opts.Kind).Valid() {
		return fmt.Errorf("report: invalid --kind %q; want one of always-on, cron, launchd, claude-code, other", opts.Kind)
	}
	if opts.provided("state") && !fleet.State(opts.State).Valid() {
		return fmt.Errorf("report: invalid --state %q; want one of running, idle, blocked, error, offline", opts.State)
	}

	// Start from the existing file when it reads cleanly, so a partial report
	// merges. A missing or unreadable file means we are creating fresh.
	base, existed := loadBase(dir, opts.ID)
	priorState := base.State

	s := base
	s.Schema = fleet.SchemaVersion
	s.ID = opts.ID
	if opts.provided("name") {
		s.Name = opts.Name
	}
	if opts.provided("kind") {
		s.Kind = fleet.Kind(opts.Kind)
	}
	if opts.provided("state") {
		s.State = fleet.State(opts.State)
	}
	if opts.provided("task") {
		s.Task = opts.Task
	}
	if opts.provided("detail") {
		s.Detail = opts.Detail
	}
	if opts.provided("tokens") {
		s.TokensToday = opts.Tokens
	}

	s.Heartbeat = now
	// Reset since only when the state actually changes, or on a fresh create.
	if !existed || s.State != priorState {
		s.Since = now
	}

	// A complete contract file needs an id, a name, a valid kind, and a valid
	// state. A partial report into a missing file can fall short, so validate
	// the merged result before writing.
	if s.Name == "" {
		return fmt.Errorf("report: --name is required for a new agent")
	}
	if !s.Kind.Valid() {
		return fmt.Errorf("report: --kind is required for a new agent")
	}
	if !s.State.Valid() {
		return fmt.Errorf("report: --state is required for a new agent")
	}

	return fleet.Write(dir, s)
}

// loadBase reads the current status for id. It returns a zero Status and false
// when no readable file exists, so a corrupt file does not block a fresh
// report from recording current status.
func loadBase(dir, id string) (fleet.Status, bool) {
	s, err := fleet.Read(fleet.Path(dir, id))
	if err != nil {
		return fleet.Status{}, false
	}
	return s, true
}
