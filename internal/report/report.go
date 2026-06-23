// Package report implements the "rocinante report" subcommand, which writes or
// merges one agent status file under the fleet directory, then exits.
package report

import "errors"

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

// Run writes or merges the status file for opts.ID into dir. Implemented in the
// report step; see build order step 3.
func Run(dir string, opts Options) error {
	return errors.New("report: not yet implemented")
}
