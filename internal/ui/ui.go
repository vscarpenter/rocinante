// Package ui holds the Bubble Tea bridge: the model, the update loop, the view,
// and the Lip Gloss theme. It renders the Fleet panel and the Ship's Log in
// v0.1; the Reactor and Comms panels arrive with their adapters.
package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vscarpenter/rocinante/internal/config"
	"github.com/vscarpenter/rocinante/internal/fleet"
)

// Run opens the fleet store, then launches the TUI bridge. It blocks until the
// user quits.
func Run(cfg config.Config) error {
	store, err := fleet.NewStore(cfg.Fleet.Dir)
	if err != nil {
		return fmt.Errorf("ui: open fleet store: %w", err)
	}
	defer func() { _ = store.Close() }()

	program := tea.NewProgram(newModel(cfg, store), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("ui: run program: %w", err)
	}
	return nil
}
