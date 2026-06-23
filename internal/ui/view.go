package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vscarpenter/rocinante/internal/adapters/github"
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
	if m.inspecting {
		return m.inspectView()
	}
	if m.expandedLog {
		return m.logExpandView()
	}
	if m.showHelp {
		return m.helpView()
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

// inspectView renders the full-screen inspect overlay for the selected agent.
func (m model) inspectView() string {
	name := m.inspected.Name
	if name == "" {
		name = m.inspected.ID
	}
	title := styleHeader.Render("ROCINANTE") + styleHeaderMeta.Render("  ·  inspect: "+name)
	body := stylePanelFocused.Width(m.inspectWidth()).Height(m.inspectHeight()).Render(m.viewport.View())
	footer := styleFooter.Render("[↑↓] scroll   [esc] back   [q] quit")
	return strings.Join([]string{title, body, footer}, "\n")
}

// logExpandView renders the Ship's Log full screen so a long history is
// scrollable, mirroring the inspect overlay.
func (m model) logExpandView() string {
	title := styleHeader.Render("ROCINANTE") + styleHeaderMeta.Render("  ·  SHIP'S LOG")
	body := stylePanelFocused.Width(m.inspectWidth()).Height(m.inspectHeight()).Render(m.viewport.View())
	footer := styleFooter.Render("[↑↓] scroll   [esc] back   [q] quit")
	return strings.Join([]string{title, body, footer}, "\n")
}

// buildLogExpand renders the full Ship's Log for the overlay, newest first to
// match the panel, with file-read errors on top.
func (m model) buildLogExpand(width int) string {
	lines := m.logLines(width, maxLogLines)
	if len(lines) == 0 {
		return styleMuted.Render("quiet so far")
	}
	return strings.Join(lines, "\n")
}

// helpView renders a centered overlay listing every keybinding, so the cockpit
// is discoverable without the README.
func (m model) helpView() string {
	bindings := []struct{ key, desc string }{
		{"tab / shift+tab", "cycle panels"},
		{"↑ ↓ / k j", "move the selection"},
		{"enter", "inspect the selected agent"},
		{"l", "expand the Ship's Log"},
		{"r", "refresh the Reactor and Comms"},
		{"?", "toggle this help"},
		{"q", "quit"},
	}

	var b strings.Builder
	b.WriteString(stylePanelTitle.Render("KEYBINDINGS") + "\n\n")
	keyCol := lipgloss.NewStyle().Width(18).Foreground(colorBlue)
	for _, kb := range bindings {
		b.WriteString(keyCol.Render(kb.key) + styleMuted.Render(kb.desc) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("any key to dismiss"))

	box := stylePanelFocused.Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderTooSmall() string {
	msg := fmt.Sprintf("Rocinante needs a bigger window.\nResize to at least %dx%d.\nCurrent: %dx%d.",
		minWidth, minHeight, m.width, m.height)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, styleMuted.Render(msg))
}

func (m model) renderHeader() string {
	title := styleHeader.Render("ROCINANTE")

	// The clock stays muted; the health note turns amber when anything needs
	// attention, so a blocked or offline agent reads as an alert, not chrome.
	summary := m.statusSummary()
	summaryStyle := styleHeaderMeta
	if summary != "all systems nominal" {
		summaryStyle = styleWarn
	}
	meta := styleHeaderMeta.Render(m.now.Format("15:04")+" · ") + summaryStyle.Render(summary)

	pad := m.width - lipgloss.Width(title) - lipgloss.Width(meta)
	if pad < 1 {
		pad = 1
	}
	return title + strings.Repeat(" ", pad) + meta
}

// statusSummary is the one-line health note in the header. It leads with the
// loudest problem, a blocked agent, so trouble surfaces without hunting, then
// folds in offline, stale, and file-read errors.
func (m model) statusSummary() string {
	var blocked, stale, offline int
	for _, a := range m.snapshot.Agents {
		live := m.liveness(a)
		switch {
		case live == fleet.LiveOffline:
			offline++
		case a.State == fleet.StateBlocked:
			blocked++
		case live == fleet.LiveStale:
			stale++
		}
	}

	var parts []string
	if blocked > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", blocked))
	}
	if offline > 0 {
		parts = append(parts, fmt.Sprintf("%d offline", offline))
	}
	if stale > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", stale))
	}
	if n := len(m.snapshot.Errors); n > 0 {
		parts = append(parts, fmt.Sprintf("%d file error(s)", n))
	}
	if len(parts) == 0 {
		return "all systems nominal"
	}
	return strings.Join(parts, ", ")
}

func (m model) renderFooter() string {
	return styleFooter.Render("[tab] panels   [enter] inspect   [l] logs   [?] help   [r] refresh   [q] quit")
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

// renderRightColumn stacks the live Reactor and Comms panels.
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

	contentW := innerW - 2
	reactor := strings.Join(capLines(m.reactorLines(contentW), topH), "\n")
	comms := strings.Join(capLines(m.commsLines(contentW), botH), "\n")

	top := stylePanel.Width(innerW).Height(topH).Render(reactor)
	bottom := stylePanel.Width(innerW).Height(botH).Render(comms)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

// reactorLines renders the Reactor panel body, today's token burn from ccusage.
func (m model) reactorLines(w int) []string {
	title := stylePanelTitle.Render("REACTOR") + "  " + styleMuted.Render("tokens, today")
	switch {
	case !m.cfg.Reactor.Enabled:
		return []string{title, stylePlaceholder.Render("disabled")}
	case m.reactorErr != "":
		return []string{title, styleError.Render(truncate("! "+m.reactorErr, w))}
	case m.reactor == nil:
		return []string{title, styleMuted.Render("loading...")}
	}

	r := m.reactor
	headline := fmt.Sprintf("%s tok   cache-read %.0f%%", formatTokens(r.TotalTokens), r.CacheReadRatio()*100)
	trend := sparkline(r.Spark)
	if trend != "" {
		trend += "   "
	}
	cost := fmt.Sprintf("%s$%.2f today", trend, r.CostUSD)
	return []string{title, truncate(headline, w), styleMuted.Render(truncate(cost, w))}
}

// commsLines renders the Comms panel body, open PRs and CI from gh.
func (m model) commsLines(w int) []string {
	title := stylePanelTitle.Render("COMMS") + "  " + styleMuted.Render("GitHub")
	switch {
	case !m.cfg.Comms.Enabled:
		return []string{title, stylePlaceholder.Render("disabled")}
	case m.commsErr != "":
		return []string{title, styleError.Render(truncate("! "+m.commsErr, w))}
	case m.comms == nil:
		return []string{title, styleMuted.Render("loading...")}
	}

	c := m.comms
	lines := []string{
		title,
		truncate(fmt.Sprintf("%d open PRs   %d need review", c.OpenPRs, c.NeedReview), w),
		m.ciSummary(c),
	}
	if n := len(c.Errors); n > 0 {
		lines = append(lines, styleError.Render(truncate(fmt.Sprintf("! %d repo error(s)", n), w)))
	}
	return lines
}

// ciSummary renders the rolled-up CI state, colored by severity.
func (m model) ciSummary(c *github.Comms) string {
	switch {
	case c.CIFailing > 0:
		return styleError.Render(fmt.Sprintf("CI %d failing", c.CIFailing))
	case c.CIPending > 0:
		return styleWarn.Render(fmt.Sprintf("CI %d pending", c.CIPending))
	case c.CIGreen > 0:
		return styleGood.Render("CI green")
	default:
		return styleMuted.Render("CI idle")
	}
}

// capLines trims a slice to at most n lines, so panel content never overflows
// its box height.
func capLines(lines []string, n int) []string {
	if n < 1 {
		n = 1
	}
	if len(lines) > n {
		return lines[:n]
	}
	return lines
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
