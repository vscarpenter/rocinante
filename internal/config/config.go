// Package config holds Rocinante's runtime settings and their defaults.
//
// The app runs with no config file at all. Load returns sane defaults, then
// overlays whatever ~/.rocinante/config.toml provides. Durations are written as
// strings like "60s" and parsed here. See build spec section 9.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the full runtime configuration. The Roci section arrives with its
// adapter in a later phase.
type Config struct {
	Fleet   FleetConfig
	Reactor ReactorConfig
	Comms   CommsConfig
	Theme   ThemeConfig
}

// FleetConfig controls the local fleet directory and staleness thresholds.
type FleetConfig struct {
	Dir          string
	StaleAfter   time.Duration
	OfflineAfter time.Duration
}

// ReactorConfig controls the token-burn adapter that shells out to ccusage.
type ReactorConfig struct {
	Enabled  bool
	Command  string
	Args     []string
	Interval time.Duration
}

// CommsConfig controls the GitHub adapter that shells out to gh.
type CommsConfig struct {
	Enabled  bool
	Repos    []string
	Interval time.Duration
}

// ThemeConfig selects the color mode.
type ThemeConfig struct {
	Mode string // adaptive | light | dark
}

// Default thresholds and intervals, from build spec sections 4, 6, and 9. The
// reactor args use the invocation verified against the installed ccusage.
const (
	defaultStaleAfter      = 90 * time.Second
	defaultOfflineAfter    = 300 * time.Second
	defaultReactorInterval = 60 * time.Second
	defaultCommsInterval   = 90 * time.Second
)

// Default returns the configuration used when no config file is present.
func Default() Config {
	return Config{
		Fleet: FleetConfig{
			Dir:          defaultFleetDir(),
			StaleAfter:   defaultStaleAfter,
			OfflineAfter: defaultOfflineAfter,
		},
		Reactor: ReactorConfig{
			Enabled:  true,
			Command:  "ccusage",
			Args:     []string{"daily", "--json"},
			Interval: defaultReactorInterval,
		},
		Comms: CommsConfig{
			Enabled:  true,
			Repos:    []string{"vscarpenter/rocinante", "vscarpenter/inkwell"},
			Interval: defaultCommsInterval,
		},
		Theme: ThemeConfig{Mode: "adaptive"},
	}
}

// Load resolves the active configuration from the default config path.
func Load() (Config, error) {
	return loadFrom(defaultConfigPath())
}

// fileConfig mirrors the TOML on disk. Durations are strings here so a bad value
// is a clear error, not a silent zero. Enabled is a pointer so an omitted flag
// keeps its default rather than reading as false.
type fileConfig struct {
	Fleet struct {
		Dir          string `toml:"dir"`
		StaleAfter   string `toml:"stale_after"`
		OfflineAfter string `toml:"offline_after"`
	} `toml:"fleet"`
	Reactor struct {
		Enabled  *bool    `toml:"enabled"`
		Command  string   `toml:"command"`
		Args     []string `toml:"args"`
		Interval string   `toml:"interval"`
	} `toml:"reactor"`
	Comms struct {
		Enabled  *bool    `toml:"enabled"`
		Repos    []string `toml:"repos"`
		Interval string   `toml:"interval"`
	} `toml:"comms"`
	Theme struct {
		Mode string `toml:"mode"`
	} `toml:"theme"`
}

// loadFrom layers the file at path over the defaults, then applies environment
// overrides last. A missing file is not an error; the defaults stand and the
// env overrides still apply.
func loadFrom(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// No file is fine; the defaults stand.
	case err != nil:
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	default:
		var fc fileConfig
		if err := toml.Unmarshal(data, &fc); err != nil {
			return cfg, fmt.Errorf("config: parse %s: %w", path, err)
		}
		if err := applyFile(&cfg, fc); err != nil {
			return cfg, fmt.Errorf("config: %s: %w", path, err)
		}
	}

	applyEnv(&cfg)
	return cfg, nil
}

// applyEnv overlays environment overrides last, so they win over the file and
// the defaults. ROCINANTE_FLEET_DIR points both the bridge and report at an
// alternate fleet directory, which sandboxes the demo and eases testing.
func applyEnv(cfg *Config) {
	if d := os.Getenv("ROCINANTE_FLEET_DIR"); d != "" {
		cfg.Fleet.Dir = expandHome(d)
	}
}

// applyFile overlays the file values that are present onto cfg.
func applyFile(cfg *Config, fc fileConfig) error {
	if fc.Fleet.Dir != "" {
		cfg.Fleet.Dir = expandHome(fc.Fleet.Dir)
	}
	if err := setDuration(&cfg.Fleet.StaleAfter, fc.Fleet.StaleAfter, "fleet.stale_after"); err != nil {
		return err
	}
	if err := setDuration(&cfg.Fleet.OfflineAfter, fc.Fleet.OfflineAfter, "fleet.offline_after"); err != nil {
		return err
	}

	if fc.Reactor.Enabled != nil {
		cfg.Reactor.Enabled = *fc.Reactor.Enabled
	}
	if fc.Reactor.Command != "" {
		cfg.Reactor.Command = fc.Reactor.Command
	}
	if fc.Reactor.Args != nil {
		cfg.Reactor.Args = fc.Reactor.Args
	}
	if err := setDuration(&cfg.Reactor.Interval, fc.Reactor.Interval, "reactor.interval"); err != nil {
		return err
	}

	if fc.Comms.Enabled != nil {
		cfg.Comms.Enabled = *fc.Comms.Enabled
	}
	if fc.Comms.Repos != nil {
		cfg.Comms.Repos = fc.Comms.Repos
	}
	if err := setDuration(&cfg.Comms.Interval, fc.Comms.Interval, "comms.interval"); err != nil {
		return err
	}

	if fc.Theme.Mode != "" {
		cfg.Theme.Mode = fc.Theme.Mode
	}
	return nil
}

// setDuration parses raw into dst when raw is non-empty, naming the field in any
// error.
func setDuration(dst *time.Duration, raw, field string) error {
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("%s: %q is not a valid duration", field, raw)
	}
	*dst = d
	return nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
		}
	}
	return path
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

// defaultConfigPath resolves ~/.rocinante/config.toml.
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".rocinante", "config.toml")
	}
	return filepath.Join(home, ".rocinante", "config.toml")
}
