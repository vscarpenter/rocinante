package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultsAreSane(t *testing.T) {
	d := Default()
	if d.Fleet.StaleAfter != 90*time.Second || d.Fleet.OfflineAfter != 300*time.Second {
		t.Errorf("fleet thresholds: got %v / %v", d.Fleet.StaleAfter, d.Fleet.OfflineAfter)
	}
	if !d.Reactor.Enabled || d.Reactor.Command != "ccusage" || d.Reactor.Interval != 60*time.Second {
		t.Errorf("reactor defaults wrong: %+v", d.Reactor)
	}
	if len(d.Reactor.Args) != 2 || d.Reactor.Args[0] != "daily" || d.Reactor.Args[1] != "--json" {
		t.Errorf("reactor args should default to the verified invocation, got %v", d.Reactor.Args)
	}
	if !d.Comms.Enabled || d.Comms.Interval != 90*time.Second || len(d.Comms.Repos) == 0 {
		t.Errorf("comms defaults wrong: %+v", d.Comms)
	}
}

func TestLoadFromMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := loadFrom(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Reactor.Command != "ccusage" || cfg.Fleet.StaleAfter != 90*time.Second {
		t.Errorf("missing file should yield defaults, got %+v", cfg)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadFromOverridesAndKeepsDefaults(t *testing.T) {
	path := writeConfig(t, `
[fleet]
stale_after = "45s"

[reactor]
enabled = false
interval = "30s"

[comms]
repos = ["acme/widgets"]
`)
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}

	if cfg.Fleet.StaleAfter != 45*time.Second {
		t.Errorf("stale_after override: got %v", cfg.Fleet.StaleAfter)
	}
	if cfg.Fleet.OfflineAfter != 300*time.Second {
		t.Errorf("offline_after should keep its default, got %v", cfg.Fleet.OfflineAfter)
	}
	if cfg.Reactor.Enabled {
		t.Error("reactor enabled=false should override the default true")
	}
	if cfg.Reactor.Interval != 30*time.Second {
		t.Errorf("reactor interval override: got %v", cfg.Reactor.Interval)
	}
	if cfg.Reactor.Command != "ccusage" {
		t.Errorf("reactor command should keep its default, got %q", cfg.Reactor.Command)
	}
	if len(cfg.Comms.Repos) != 1 || cfg.Comms.Repos[0] != "acme/widgets" {
		t.Errorf("comms repos override: got %v", cfg.Comms.Repos)
	}
}

func TestLoadFromEnvOverridesFleetDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}

	tests := []struct {
		name string
		env  string
		file string
		want string
	}{
		{
			name: "unset keeps the file value",
			env:  "",
			file: "[fleet]\ndir = \"/from-file\"\n",
			want: "/from-file",
		},
		{
			name: "set overrides the file value",
			env:  "/from-env",
			file: "[fleet]\ndir = \"/from-file\"\n",
			want: "/from-env",
		},
		{
			name: "leading tilde expands",
			env:  "~/env-fleet",
			file: "",
			want: filepath.Join(home, "env-fleet"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ROCINANTE_FLEET_DIR", tc.env)
			path := filepath.Join(t.TempDir(), "none.toml")
			if tc.file != "" {
				path = writeConfig(t, tc.file)
			}
			cfg, err := loadFrom(path)
			if err != nil {
				t.Fatalf("loadFrom: %v", err)
			}
			if cfg.Fleet.Dir != tc.want {
				t.Errorf("fleet dir: got %q, want %q", cfg.Fleet.Dir, tc.want)
			}
		})
	}
}

func TestLoadFromBadDurationErrors(t *testing.T) {
	path := writeConfig(t, "[fleet]\nstale_after = \"banana\"\n")
	if _, err := loadFrom(path); err == nil {
		t.Error("expected an error for an unparseable duration")
	}
}

func TestLoadFromExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	path := writeConfig(t, "[fleet]\ndir = \"~/custom-fleet\"\n")
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("loadFrom: %v", err)
	}
	want := filepath.Join(home, "custom-fleet")
	if cfg.Fleet.Dir != want {
		t.Errorf("tilde expansion: got %q, want %q", cfg.Fleet.Dir, want)
	}
}
