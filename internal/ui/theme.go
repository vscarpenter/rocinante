package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

// Palette.
//
// Only the accent and surface below are confirmed Signal Ledger tokens. The
// rest are neutral, adaptive placeholders chosen to read in both light and dark
// terminals. They are not the real Signal Ledger ramp, which must be confirmed
// before the theme is locked. See build spec sections 8 and 15.
var (
	accent  = lipgloss.Color("#2167f3") // running, confirmed token
	surface = lipgloss.Color("#fbfbfa") // confirmed token

	colorFG    = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#e6e6e6"}
	colorMuted = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9aa0aa"}
	colorDim   = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#4b5563"}
	colorAmber = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#f59e0b"}
	colorRed   = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#ef4444"}
	colorGreen = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#22c55e"}
	colorBlue  = lipgloss.AdaptiveColor{Light: "#2167f3", Dark: "#5b8cff"}
)

// Shared styles. Borders use the muted color so the accent stays reserved for
// meaning, not chrome.
var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(colorBlue)

	styleHeaderMeta = lipgloss.NewStyle().Foreground(colorMuted)

	stylePanelTitle = lipgloss.NewStyle().Bold(true).Foreground(colorFG)

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDim).
			Padding(0, 1)

	stylePanelFocused = stylePanel.
				BorderForeground(colorBlue)

	styleFooter = lipgloss.NewStyle().Foreground(colorMuted)

	// styleSelected is the focused selection: near-white surface text on the
	// accent, which reads in both light and dark terminals.
	styleSelected = lipgloss.NewStyle().Bold(true).Background(accent).Foreground(surface)

	styleMuted = lipgloss.NewStyle().Foreground(colorMuted)

	styleError = lipgloss.NewStyle().Foreground(colorRed)

	styleGood = lipgloss.NewStyle().Foreground(colorGreen)

	styleWarn = lipgloss.NewStyle().Foreground(colorAmber)

	stylePlaceholder = lipgloss.NewStyle().Foreground(colorDim).Italic(true)
)

// glyphStyle is a state's glyph paired with the color that carries its meaning.
type glyphStyle struct {
	glyph string
	style lipgloss.Style
}

// reportedGlyph maps a reported state to its glyph and color, per the spec's
// state table.
func reportedGlyph(state fleet.State) glyphStyle {
	switch state {
	case fleet.StateRunning:
		return glyphStyle{"●", lipgloss.NewStyle().Foreground(accent)} // filled circle
	case fleet.StateIdle:
		return glyphStyle{"○", lipgloss.NewStyle().Foreground(colorMuted)} // hollow circle
	case fleet.StateBlocked:
		return glyphStyle{"▲", lipgloss.NewStyle().Foreground(colorAmber)} // triangle
	case fleet.StateError:
		return glyphStyle{"✕", lipgloss.NewStyle().Foreground(colorRed)} // cross
	case fleet.StateOffline:
		return glyphStyle{"◌", lipgloss.NewStyle().Foreground(colorDim)} // dotted circle
	default:
		return glyphStyle{"○", lipgloss.NewStyle().Foreground(colorMuted)}
	}
}

// liveGlyph derives the glyph and style actually shown, folding in liveness.
// Derived offline overrides the reported state, since a crashed agent never
// writes offline itself. Stale keeps the reported glyph but dims the row.
func liveGlyph(state fleet.State, live fleet.Liveness) glyphStyle {
	if live == fleet.LiveOffline {
		return reportedGlyph(fleet.StateOffline)
	}
	gs := reportedGlyph(state)
	if live == fleet.LiveStale {
		gs.style = gs.style.Faint(true)
	}
	return gs
}

// effectiveStateLabel is the word shown next to the glyph, folding in derived
// staleness and offline.
func effectiveStateLabel(state fleet.State, live fleet.Liveness) string {
	switch live {
	case fleet.LiveOffline:
		return "offline"
	case fleet.LiveStale:
		return string(state) + " (stale)"
	default:
		return string(state)
	}
}
