package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

const (
	minWidth  = 80
	minHeight = 24
)

// View renders the current model. It never panics on odd sizes or empty state;
// a too-small window gets a friendly message instead of a broken layout.
func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Starting Rocinante..."
	}
	if m.width < minWidth || m.height < minHeight {
		return m.renderTooSmall()
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// Reserve one row each for the header and footer.
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 6 {
		bodyHeight = 6
	}

	topHeight := bodyHeight * 55 / 100
	logHeight := bodyHeight - topHeight

	gap := 2
	leftOuter := m.width * 58 / 100
	rightOuter := m.width - leftOuter - gap

	fleet := m.renderFleet(leftOuter-2, topHeight-2)
	right := m.renderRightColumn(rightOuter-2, topHeight-2)
	top := lipgloss.JoinHorizontal(lipgloss.Top, fleet, strings.Repeat(" ", gap), right)

	log := m.renderLog(m.width-2-2, logHeight-2)

	return strings.Join([]string{header, top, log, footer}, "\n")
}

func (m model) renderTooSmall() string {
	msg := fmt.Sprintf("Rocinante needs a bigger window.\nResize to at least %dx%d.\nCurrent: %dx%d.",
		minWidth, minHeight, m.width, m.height)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styleMuted.Render(msg))
}

func (m model) renderHeader() string {
	title := styleHeader.Render("ROCINANTE")
	meta := styleHeaderMeta.Render(m.now.Format("15:04") + " · " + m.statusSummary())

	pad := m.width - lipgloss.Width(title) - lipgloss.Width(meta)
	if pad < 1 {
		pad = 1
	}
	return title + strings.Repeat(" ", pad) + meta
}

// statusSummary is the one-line health note in the header.
func (m model) statusSummary() string {
	var stale, offline int
	for _, a := range m.snapshot.Agents {
		switch m.liveness(a) {
		case fleet.LiveStale:
			stale++
		case fleet.LiveOffline:
			offline++
		}
	}
	if n := len(m.snapshot.Errors); n > 0 {
		return fmt.Sprintf("%d file error(s)", n)
	}
	if stale == 0 && offline == 0 {
		return "all systems nominal"
	}
	return fmt.Sprintf("%d stale, %d offline", stale, offline)
}

func (m model) renderFooter() string {
	return styleFooter.Render("[tab] panels   [↑↓] select   [q] quit")
}

// panelStyle picks the focused or unfocused border for a panel.
func (m model) panelStyle(p panel, innerW, innerH int) lipgloss.Style {
	style := stylePanel
	if m.focus == p {
		style = stylePanelFocused
	}
	return style.Width(innerW).Height(innerH)
}

func (m model) renderFleet(innerW, innerH int) string {
	var b strings.Builder
	b.WriteString(stylePanelTitle.Render("FLEET"))
	b.WriteString("\n")

	if len(m.snapshot.Agents) == 0 {
		b.WriteString(styleMuted.Render("no agents reporting yet"))
	}

	for i, a := range m.snapshot.Agents {
		b.WriteString(m.renderAgentLine(a, i == m.cursor && m.focus == panelFleet, innerW-2))
		if i < len(m.snapshot.Agents)-1 {
			b.WriteString("\n")
		}
	}

	return m.panelStyle(panelFleet, innerW, innerH).Render(b.String())
}

// renderAgentLine renders one agent as a status line plus an indented task line.
// Each field is a width-bounded Lip Gloss cell, so colored text still aligns;
// fmt padding would miscount the ANSI escape codes.
func (m model) renderAgentLine(a fleet.Status, selected bool, width int) string {
	live := m.liveness(a)
	g := liveGlyph(a.State, live)

	name := a.Name
	if name == "" {
		name = a.ID
	}

	marker := "  "
	nameStyle := lipgloss.NewStyle().Width(18)
	if selected {
		marker = "› "
		nameStyle = styleSelected.Width(18)
	}

	cells := []string{
		marker,
		g.style.Render(g.glyph) + " ",
		nameStyle.Render(truncate(name, 18)),
		lipgloss.NewStyle().Width(16).Foreground(colorMuted).Render(truncate(effectiveStateLabel(a.State, live), 16)),
		styleMuted.Render(compactDuration(m.now.Sub(a.Since))),
	}
	out := lipgloss.JoinHorizontal(lipgloss.Top, cells...)

	if a.Task != "" {
		out += "\n" + styleMuted.Render(truncate("   └ "+a.Task, width))
	}

	// Stale agents dim the whole row.
	if live == fleet.LiveStale {
		out = lipgloss.NewStyle().Faint(true).Render(out)
	}
	return out
}

// renderRightColumn stacks the Reactor and Comms placeholders. Their adapters
// are deferred, so they show a clear pending note rather than fabricated data.
func (m model) renderRightColumn(innerW, innerH int) string {
	half := innerH / 2
	topH := half - 2          // first panel's border
	botH := innerH - half - 2 // second panel's border
	if topH < 1 {
		topH = 1
	}
	if botH < 1 {
		botH = 1
	}

	reactor := stylePanelTitle.Render("REACTOR") + "\n" +
		stylePlaceholder.Render("token burn") + "\n" +
		styleMuted.Render("awaiting ccusage adapter")
	comms := stylePanelTitle.Render("COMMS") + "\n" +
		stylePlaceholder.Render("GitHub") + "\n" +
		styleMuted.Render("awaiting gh adapter")

	top := stylePanel.Width(innerW).Height(topH).Render(reactor)
	bottom := stylePanel.Width(innerW).Height(botH).Render(comms)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (m model) renderLog(innerW, innerH int) string {
	var b strings.Builder
	b.WriteString(stylePanelTitle.Render("SHIP'S LOG"))
	b.WriteString("\n")

	lines := m.logLines(innerW, innerH-1)
	if len(lines) == 0 {
		b.WriteString(styleMuted.Render("quiet so far"))
	} else {
		b.WriteString(strings.Join(lines, "\n"))
	}

	return m.panelStyle(panelLog, innerW, innerH).Render(b.String())
}

// logLines builds the visible log, file errors first in red, then the newest
// activity, ordered newest at the top to match the wireframe.
func (m model) logLines(width, max int) []string {
	if max < 1 {
		max = 1
	}
	var lines []string

	for name, err := range m.snapshot.Errors {
		lines = append(lines, styleError.Render(truncate("!! "+name+": "+err.Error(), width)))
	}

	for i := len(m.logs) - 1; i >= 0; i-- {
		e := m.logs[i]
		line := fmt.Sprintf("%s  %-14s %s", e.at.Format("15:04"), truncate(e.agentID, 14), e.text)
		lines = append(lines, truncate(line, width))
		if len(lines) >= max {
			break
		}
	}
	if len(lines) > max {
		lines = lines[:max]
	}
	return lines
}

// truncate shortens s to w columns, adding an ellipsis when it cuts. It is rune
// based, which suits the short labels and single-line text shown here.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	if w == 1 {
		return "…"
	}
	if len(r) <= w {
		return s
	}
	return string(r[:w-1]) + "…"
}
