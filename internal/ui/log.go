package ui

import (
	"fmt"
	"sort"
	"time"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

// logEntry is one line in the Ship's Log. The timestamp is the agent's own
// heartbeat, so the log reflects when the agent acted, not when the bridge
// noticed.
type logEntry struct {
	at      time.Time
	agentID string
	text    string
}

// diffLog finds the meaningful changes between two fleet states and returns one
// log line for each. A new agent, a state transition, or a fresh detail or task
// is meaningful. A bare heartbeat is not, so steady reporting does not flood the
// log. Entries come back ordered by heartbeat.
func diffLog(prev, next map[string]fleet.Status) []logEntry {
	var out []logEntry
	for id, ns := range next {
		ps, existed := prev[id]
		if text, ok := logText(ps, ns, !existed); ok {
			out = append(out, logEntry{at: ns.Heartbeat, agentID: id, text: text})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].at.Before(out[j].at)
	})
	return out
}

// logText decides what, if anything, a change is worth saying. It prefers the
// most specific signal: a new detail, then a state transition, then a new task.
func logText(prev, next fleet.Status, isNew bool) (string, bool) {
	switch {
	case isNew:
		if next.Detail != "" {
			return next.Detail, true
		}
		if next.Task != "" {
			return next.Task, true
		}
		return "reported in: " + string(next.State), true
	case next.Detail != "" && next.Detail != prev.Detail:
		return next.Detail, true
	case next.State != prev.State:
		return "is now " + string(next.State), true
	case next.Task != "" && next.Task != prev.Task:
		return next.Task, true
	default:
		return "", false
	}
}

// compactDuration renders a duration in a single unit for a tight column, such
// as 5s, 33m, 2h, or 1d.
func compactDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
