// Package config holds Rocinante's runtime settings and their defaults.
//
// The app runs with no config file at all. Load returns sane defaults today.
// A later session adds TOML loading from ~/.rocinante/config.toml and per-kind
// staleness overrides, as described in build spec section 9. The Config struct
// is the seam those features plug into.
package config

import (
	"os"
	"path/filepath"
	"time"
)

// Config is the full runtime configuration. Only Fleet is wired in v0.1. The
// Reactor, Comms, and Roci sections arrive with their adapters.
type Config struct {
	Fleet FleetConfig
}

// FleetConfig controls the local fleet directory and staleness thresholds.
type FleetConfig struct {
	// Dir is the directory of per-agent status files.
	Dir string
	// StaleAfter is the silence after which an agent renders stale and dims.
	StaleAfter time.Duration
	// OfflineAfter is the silence after which an agent renders offline.
	OfflineAfter time.Duration
}

// Default thresholds, from build spec section 4.
const (
	defaultStaleAfter   = 90 * time.Second
	defaultOfflineAfter = 300 * time.Second
)

// Default returns the configuration used when no config file is present.
func Default() Config {
	return Config{
		Fleet: FleetConfig{
			Dir:          defaultFleetDir(),
			StaleAfter:   defaultStaleAfter,
			OfflineAfter: defaultOfflineAfter,
		},
	}
}

// Load resolves the active configuration. For now it returns defaults. The
// signature returns an error so TOML loading can report a bad file later
// without changing every caller.
func Load() (Config, error) {
	return Default(), nil
}

// defaultFleetDir resolves ~/.rocinante/fleet, falling back to a relative path
// if the home directory cannot be determined.
func defaultFleetDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".rocinante", "fleet")
	}
	return filepath.Join(home, ".rocinante", "fleet")
}
