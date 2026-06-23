package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

// buildInspect renders an agent's full record for the inspect view, wrapping the
// long task and detail to width. The viewport scrolls it when it overflows.
func buildInspect(s fleet.Status, live fleet.Liveness, now time.Time, width int) string {
	wrap := func(text string) string {
		return lipgloss.NewStyle().Width(width).Render(text)
	}
	var b strings.Builder

	field := func(name, value string) {
		b.WriteString(styleMuted.Render(fmt.Sprintf("%-12s", name)) + value + "\n")
	}
	field("id", s.ID)
	field("name", s.Name)
	field("kind", string(s.Kind))
	field("state", effectiveStateLabel(s.State, live))
	field("since", fmt.Sprintf("%s  (%s ago)", s.Since.Format("2006-01-02 15:04:05 MST"), compactDuration(now.Sub(s.Since))))
	field("heartbeat", fmt.Sprintf("%s  (%s ago)", s.Heartbeat.Format("2006-01-02 15:04:05 MST"), compactDuration(now.Sub(s.Heartbeat))))
	if s.TokensToday > 0 {
		field("tokens today", formatTokens(s.TokensToday))
	}

	if s.Task != "" {
		b.WriteString("\n" + stylePanelTitle.Render("Task") + "\n")
		b.WriteString(wrap(s.Task) + "\n")
	}
	if s.Detail != "" {
		b.WriteString("\n" + stylePanelTitle.Render("Detail") + "\n")
		b.WriteString(wrap(s.Detail) + "\n")
	}
	if len(s.Meta) > 0 {
		b.WriteString("\n" + stylePanelTitle.Render("Meta") + "\n")
		keys := make([]string, 0, len(s.Meta))
		for k := range s.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s: %v\n", k, s.Meta[k])
		}
	}
	return b.String()
}
