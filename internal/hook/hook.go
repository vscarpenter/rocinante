// Package hook translates a Claude Code hook event into a fleet status report.
//
// Claude Code delivers each lifecycle event as a JSON object on stdin. The
// "rocinante hook" subcommand reads that payload and maps it to a report, so the
// user's settings never need to parse JSON or know the contract. The event names
// and fields here were verified against the installed Claude Code; see CLAUDE.md.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/vscarpenter/rocinante/internal/fleet"
	"github.com/vscarpenter/rocinante/internal/report"
)

// taskMaxLen caps the prompt-derived task line.
const taskMaxLen = 80

// Payload is the subset of a Claude Code hook event we read from stdin.
type Payload struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Event     string `json:"hook_event_name"`
	Prompt    string `json:"prompt"`
	ToolName  string `json:"tool_name"`
	Source    string `json:"source"`
}

// Run reads a hook payload from r and writes the mapped status into dir. An
// event we do not handle is a silent no-op, so registering extra hooks never
// errors.
func Run(dir string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("hook: read stdin: %w", err)
	}
	p, err := parsePayload(data)
	if err != nil {
		return err
	}
	opts, ok := optionsFor(p)
	if !ok {
		return nil
	}
	return report.Run(dir, opts)
}

func parsePayload(data []byte) (Payload, error) {
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, fmt.Errorf("hook: parse payload: %w", err)
	}
	return p, nil
}

// optionsFor maps one event to a report. It always sets the identity fields, so
// any event can create the file, then sets state and, where meaningful, the
// task. Tool events refresh only the heartbeat, which keeps a busy session fresh
// without flooding the Ship's Log.
func optionsFor(p Payload) (report.Options, bool) {
	if p.SessionID == "" {
		return report.Options{}, false
	}

	opts := report.Options{
		ID:       "cc-" + p.SessionID,
		Name:     sessionName(p.Cwd),
		Kind:     string(fleet.KindClaudeCode),
		Provided: map[string]bool{"name": true, "kind": true},
	}
	setState := func(state string) {
		opts.State = state
		opts.Provided["state"] = true
	}
	setTask := func(task string) {
		opts.Task = task
		opts.Provided["task"] = true
	}

	switch p.Event {
	case "SessionStart":
		setState("running")
		if p.Source != "" {
			setTask("started (" + p.Source + ")")
		} else {
			setTask("started")
		}
	case "UserPromptSubmit":
		setState("running")
		setTask(firstLine(p.Prompt, taskMaxLen))
	case "PreToolUse", "PostToolUse":
		setState("running") // heartbeat refresh only
	case "Stop":
		setState("idle")
	case "SessionEnd":
		setState("offline")
	default:
		return report.Options{}, false
	}
	return opts, true
}

// sessionName labels the session by its working directory, falling back to a
// generic name when the directory is empty or a root.
func sessionName(cwd string) string {
	base := filepath.Base(cwd)
	if cwd == "" || base == "." || base == string(filepath.Separator) {
		return "claude-code"
	}
	return base
}

// firstLine returns the first non-empty line of s, trimmed and capped at max
// runes with an ellipsis.
func firstLine(s string, max int) string {
	line := s
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		line = s[:i]
	}
	line = strings.TrimSpace(line)

	r := []rune(line)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return line
}
