# Demo Fleet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a one-command living demo so a fresh clone can watch the Rocinante cockpit fill with agents that change state, scroll the Ship's Log, trip the blocked alert, and decay to stale and offline.

**Architecture:** One small env override, `ROCINANTE_FLEET_DIR`, sandboxes a fleet directory for both the bridge and `report`. A bash script, `examples/demo-fleet.sh`, drives five fake agents purely through `rocinante report`, so it adds no demo code to the shipped binary and doubles as living documentation of the contract.

**Tech Stack:** Go 1.26 with the existing config package, plus a POSIX-friendly bash script. No new Go dependencies.

## Global Constraints

- House style for all prose: active voice, AP style, Oxford commas, no em dashes, sentences under 25 words.
- Idiomatic Go in small packages; nothing panics on bad input; tests table-driven.
- Use only the dependencies already approved in build spec section 11. Add none.
- Atomic writes for status files. `report` already does this; the script must not write JSON by hand.
- Keep golangci-lint clean.
- Conventional commit messages, small logical commits.
- Commit message footer line: `Claude-Session: https://claude.ai/code/session_01TJtQ77Uiv19mGhhS1yQkM3`

---

### Task 1: ROCINANTE_FLEET_DIR env override

**Files:**
- Modify: `internal/config/config.go` (the `loadFrom` function, plus a new `applyEnv` helper)
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: existing `Default()`, `applyFile`, `expandHome`, and the `loadFrom(path string) (Config, error)` shape.
- Produces: `loadFrom` now applies `ROCINANTE_FLEET_DIR` last, so precedence is defaults, then `config.toml`, then the env var. A new unexported `applyEnv(cfg *Config)` helper.

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadFromEnvOverridesFleetDir -v`
Expected: FAIL. The "set overrides the file value" and "leading tilde expands" subtests fail, because `loadFrom` ignores the env var.

- [ ] **Step 3: Restructure loadFrom and add applyEnv**

In `internal/config/config.go`, replace the body of `loadFrom` so the env override runs on every successful path, including the missing-file path. Replace this:

```go
// loadFrom layers the file at path over the defaults. A missing file is not an
// error; the defaults stand.
func loadFrom(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}

	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return cfg, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := applyFile(&cfg, fc); err != nil {
		return cfg, fmt.Errorf("config: %s: %w", path, err)
	}
	return cfg, nil
}
```

with this:

```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/config/ -run TestLoadFromEnvOverridesFleetDir -v`
Expected: PASS, all three subtests.

- [ ] **Step 5: Run the full config suite and lint**

Run: `go test ./internal/config/ && golangci-lint run ./internal/config/`
Expected: ok, and 0 issues.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add the ROCINANTE_FLEET_DIR env override

Apply the env var last in loadFrom, after the config file, so precedence
reads defaults, then config.toml, then the env. It points both the bridge
and report at an alternate fleet directory, which sandboxes the demo fleet
and eases testing.

Claude-Session: https://claude.ai/code/session_01TJtQ77Uiv19mGhhS1yQkM3"
```

---

### Task 2: The demo-fleet.sh living simulation

**Files:**
- Create: `examples/demo-fleet.sh` (executable bash)

**Interfaces:**
- Consumes: the `ROCINANTE_FLEET_DIR` override from Task 1, and the public `rocinante report` flags `--id`, `--name`, `--kind`, `--state`, `--task`, `--detail`, `--tokens`.
- Produces: a runnable demo with modes `--live` (default), `--seed`, `--clean`, and a `--tick SECONDS` option.

- [ ] **Step 1: Write the script**

Create `examples/demo-fleet.sh` with exactly this content:

```bash
#!/usr/bin/env bash
#
# demo-fleet.sh drives a fake agent fleet through `rocinante report`, so a fresh
# clone can watch the cockpit come alive. It only calls the public report
# interface, exactly as a real agent would, so it doubles as documentation of
# the Fleet Status Contract.
#
# The demo writes to ROCINANTE_FLEET_DIR, defaulting to a temp directory, so it
# never touches a real ~/.rocinante/fleet.
set -euo pipefail

TICK="${TICK:-10}"
MODE="live"

usage() {
	cat <<'USAGE'
Usage: examples/demo-fleet.sh [--live | --seed | --clean] [--tick SECONDS]

  --live          drive an evolving fleet until Ctrl-C (default)
  --seed          write one round of agents, then exit
  --clean         delete the demo's sandbox fleet directory
  --tick SECONDS  seconds between rounds in --live (default 10)

The demo writes to ROCINANTE_FLEET_DIR, defaulting to a temp directory. Point
the bridge at the same directory in another pane:

  ROCINANTE_FLEET_DIR=<dir> rocinante
USAGE
}

while [ $# -gt 0 ]; do
	case "$1" in
	--live) MODE="live" ;;
	--seed) MODE="seed" ;;
	--clean) MODE="clean" ;;
	--tick)
		shift
		TICK="${1:?--tick needs a value}"
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "demo-fleet: unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
	shift
done

export ROCINANTE_FLEET_DIR="${ROCINANTE_FLEET_DIR:-${TMPDIR:-/tmp}/rocinante-demo}"

if [ "$MODE" = "clean" ]; then
	rm -rf "$ROCINANTE_FLEET_DIR"
	echo "demo-fleet: removed $ROCINANTE_FLEET_DIR"
	exit 0
fi

# resolve_bin finds a rocinante binary, building one only as a last resort so the
# report calls stay fast. The build lands outside the watched fleet directory.
resolve_bin() {
	if [ -n "${ROCINANTE_BIN:-}" ]; then
		echo "$ROCINANTE_BIN"
		return
	fi
	if [ -x "./rocinante" ]; then
		echo "./rocinante"
		return
	fi
	if command -v rocinante >/dev/null 2>&1; then
		echo "rocinante"
		return
	fi
	local built="${TMPDIR:-/tmp}/rocinante-demo-bin/rocinante"
	mkdir -p "$(dirname "$built")"
	echo "demo-fleet: building rocinante..." >&2
	go build -o "$built" ./cmd/rocinante >&2
	echo "$built"
}

BIN="$(resolve_bin)"

report() {
	"$BIN" report "$@"
}

TASKS_CC=(
	"reading internal/ui/view.go"
	"extracting the renderHeader helper"
	"writing table-driven tests"
	"running golangci-lint"
	"committing: tighten the header"
)

tokens=420000000

# seed_inbox reports inbox-agent once. The loop never reports it again, so it
# falls silent and decays to stale, then offline.
seed_inbox() {
	report --id inbox-agent --name "Inbox Agent" --kind always-on --state running \
		--task "triaging 12 unread messages" --detail "started at session open"
}

# report_round re-reports every living agent for round r, refreshing heartbeats
# and advancing the screenplay. gh-watch cycles into a blocked state, and
# deploy-bot blips to an error, to exercise the header alert and the red glyph.
report_round() {
	local r="$1"

	local task="${TASKS_CC[$((r % ${#TASKS_CC[@]}))]}"
	report --id cc-refactor --name "Claude: refactor bridge" --kind claude-code \
		--state running --task "$task" --tokens "$tokens"

	case $((r % 4)) in
	0) report --id gh-watch --name "GitHub Watcher" --kind cron --state running \
		--task "polling org repos for new PRs" ;;
	1) report --id gh-watch --state idle --detail "found 1 new PR (#57)" ;;
	2) report --id gh-watch --state blocked --task "waiting on review for PR #57" ;;
	3) report --id gh-watch --state idle --detail "review posted, merging" ;;
	esac

	if [ $((r % 3)) -eq 0 ]; then
		report --id nightly-sync --name "Nightly Sync" --kind launchd --state running \
			--task "syncing the vault to S3"
	else
		report --id nightly-sync --name "Nightly Sync" --kind launchd --state idle \
			--detail "last sync clean"
	fi

	if [ $((r % 5)) -eq 2 ]; then
		report --id deploy-bot --name "Deploy Bot" --kind other --state error \
			--detail "build failed: exit 1 on the lint step"
	else
		report --id deploy-bot --name "Deploy Bot" --kind other --state running \
			--task "watching main for green builds"
	fi

	tokens=$((tokens + 85000000))
}

mkdir -p "$ROCINANTE_FLEET_DIR"
echo "demo-fleet: writing to $ROCINANTE_FLEET_DIR"
echo "demo-fleet: in another pane, run:"
echo
echo "    ROCINANTE_FLEET_DIR=$ROCINANTE_FLEET_DIR $BIN"
echo

seed_inbox
report_round 0
echo "[t+0s] seeded 5 agents; inbox-agent will fall silent and decay"

if [ "$MODE" = "seed" ]; then
	echo "demo-fleet: seeded one round. run --live to evolve, --clean to remove."
	exit 0
fi

trap 'echo; echo "demo-fleet: stopped. clean up with: examples/demo-fleet.sh --clean"; exit 0' INT

round=1
while true; do
	sleep "$TICK"
	report_round "$round"

	elapsed=$((round * TICK))
	note=""
	if [ $((round % 4)) -eq 2 ]; then
		note="gh-watch blocked -> header turns amber"
	fi
	if [ $((round % 5)) -eq 2 ]; then
		note="${note:+$note; }deploy-bot error -> red glyph"
	fi
	echo "[t+${elapsed}s] round $round${note:+  -- $note}"

	if [ "$elapsed" -ge 90 ] && [ "$elapsed" -lt $((90 + TICK)) ]; then
		echo "demo-fleet: inbox-agent should now read STALE (silent ~90s)"
	fi
	if [ "$elapsed" -ge 300 ] && [ "$elapsed" -lt $((300 + TICK)) ]; then
		echo "demo-fleet: inbox-agent should now read OFFLINE (silent ~300s)"
	fi

	round=$((round + 1))
done
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x examples/demo-fleet.sh`

- [ ] **Step 3: Lint the script with shellcheck if available**

Run: `command -v shellcheck >/dev/null && shellcheck examples/demo-fleet.sh || echo "shellcheck not installed, skipping"`
Expected: no warnings, or the skip line. Fix any warning shellcheck reports.

- [ ] **Step 4: Build a binary for the verification**

Run: `go build -o /tmp/rocinante-demo-verify ./cmd/rocinante`
Expected: builds cleanly, exit 0.

- [ ] **Step 5: Verify --seed writes the five agents to a sandbox**

Run:

```bash
ROCINANTE_BIN=/tmp/rocinante-demo-verify ROCINANTE_FLEET_DIR=/tmp/roci-demo-test \
  ./examples/demo-fleet.sh --seed
ls /tmp/roci-demo-test
```

Expected: the script prints the run hint and the seed line, and `ls` shows exactly five files: `cc-refactor.json`, `deploy-bot.json`, `gh-watch.json`, `inbox-agent.json`, and `nightly-sync.json`.

- [ ] **Step 6: Verify the seeded files are valid JSON with the expected schema**

Run:

```bash
for f in /tmp/roci-demo-test/*.json; do python3 -c "import json,sys; d=json.load(open(sys.argv[1])); assert d['schema']==1 and d['id'] and d['state']; print(sys.argv[1], 'ok')" "$f"; done
```

Expected: five "ok" lines, no assertion errors.

- [ ] **Step 7: Verify --clean removes the sandbox**

Run:

```bash
ROCINANTE_FLEET_DIR=/tmp/roci-demo-test ./examples/demo-fleet.sh --clean
test ! -d /tmp/roci-demo-test && echo "sandbox removed"
```

Expected: "demo-fleet: removed /tmp/roci-demo-test" then "sandbox removed".

- [ ] **Step 8: Commit**

```bash
git add examples/demo-fleet.sh
git commit -m "feat(examples): add the demo-fleet living simulation script

demo-fleet.sh drives five fake agents through rocinante report so a fresh
clone can watch the cockpit come alive: states change, the Ship's Log
scrolls, gh-watch trips the amber blocked alert, deploy-bot blips red, and
inbox-agent falls silent and decays to stale then offline. It only calls
the public report interface and writes to a ROCINANTE_FLEET_DIR sandbox.

Claude-Session: https://claude.ai/code/session_01TJtQ77Uiv19mGhhS1yQkM3"
```

---

### Task 3: Documentation and the reference config

**Files:**
- Create: `examples/config.toml`
- Create: `examples/README.md`
- Modify: `README.md` (insert a "Live demo" section before "## Keys", and note the env override in "## Configuration")
- Modify: `CLAUDE.md` (add a scope note under "Scope discipline")

**Interfaces:**
- Consumes: the `examples/demo-fleet.sh` script and the `ROCINANTE_FLEET_DIR` override.
- Produces: run instructions and a commented reference config. No code.

- [ ] **Step 1: Create the reference config**

Create `examples/config.toml`:

```toml
# Rocinante reference config.
#
# Rocinante runs with no config at all. This file documents every setting and
# its default. Copy what you want to ~/.rocinante/config.toml; omitted keys keep
# their defaults. Durations are strings like "90s" or "5m".

[fleet]
# Where agents write their status files. The ROCINANTE_FLEET_DIR env var, if
# set, overrides this for both the bridge and `rocinante report`.
dir           = "~/.rocinante/fleet"
# Silence past stale_after dims an agent. Past offline_after marks it offline.
stale_after   = "90s"
offline_after = "300s"

[reactor]
# The Reactor shells out to ccusage for today's token burn. Set enabled = false
# to hide the panel, for example when running the demo without ccusage.
enabled  = true
command  = "ccusage"
args     = ["daily", "--json"]
interval = "60s"

[comms]
# Comms shells out to gh for open pull requests and CI. List the repos to watch.
enabled  = true
repos    = ["vscarpenter/rocinante", "vscarpenter/inkwell"]
interval = "90s"

[theme]
# adaptive reads the terminal background. Force it with light or dark.
mode = "adaptive"
```

- [ ] **Step 2: Create the examples README**

Create `examples/README.md`:

```markdown
# Examples

## Live demo

`demo-fleet.sh` drives a fake fleet through `rocinante report`, so you can watch
the cockpit come alive without wiring up real agents. It only calls the public
`report` interface, so it also documents the Fleet Status Contract.

The demo writes to a sandbox directory through the `ROCINANTE_FLEET_DIR`
override, so it never touches a real `~/.rocinante/fleet`.

```bash
# Build the binary once.
go build -o rocinante ./cmd/rocinante

# In one shell, drive the fleet. It prints the exact command for the other pane.
./examples/demo-fleet.sh --live

# In another shell, launch the bridge against the same sandbox.
ROCINANTE_FLEET_DIR=${TMPDIR:-/tmp}/rocinante-demo ./rocinante
```

What you will see:

- `cc-refactor` runs throughout, its task and token count climbing.
- `gh-watch` cycles to `blocked`, which turns the header alert amber.
- `deploy-bot` blips to `error`, showing the red glyph.
- `inbox-agent` falls silent and decays to `stale` near 90 seconds, then
  `offline` near 5 minutes.

Press Ctrl-C to stop. Remove the sandbox with:

```bash
./examples/demo-fleet.sh --clean
```

Other modes: `--seed` writes one round and exits, and `--tick SECONDS` sets the
pace of `--live`.

## config.toml

A fully commented reference config. Copy what you want to
`~/.rocinante/config.toml`; the app runs fine with no config at all.
```

- [ ] **Step 2a: Fix the nested fence**

The README block above contains a nested triple-backtick fence. After creating the file, open `examples/README.md` and confirm the outer document renders. Since Markdown cannot nest identical fences, write the inner shell blocks with triple backticks and ensure the file itself is not wrapped in a fence. The content between "## Live demo" and "## config.toml" must use real fenced code blocks, not an outer wrapper. Verify by viewing the file.

- [ ] **Step 3: Insert the Live demo section in the root README**

In `README.md`, find this exact text:

```markdown
The agent appears live. Report again to update it. Stop reporting, and it flips
to stale, then offline, on the configured thresholds.

## Keys
```

Replace it with:

```markdown
The agent appears live. Report again to update it. Stop reporting, and it flips
to stale, then offline, on the configured thresholds.

## Live demo

To see a whole fleet at once, run the demo. It drives five fake agents through
`rocinante report`, so the cockpit fills with changing state, a scrolling Ship's
Log, the amber blocked alert, and an agent that decays to stale, then offline.

```bash
go build -o rocinante ./cmd/rocinante

# Drive the fleet. It prints the exact command for the other pane.
./examples/demo-fleet.sh --live

# In another shell, point the bridge at the same sandbox.
ROCINANTE_FLEET_DIR=${TMPDIR:-/tmp}/rocinante-demo ./rocinante
```

The demo writes to a sandbox through the `ROCINANTE_FLEET_DIR` override, so it
never touches your real fleet. Stop with Ctrl-C, then clean up with
`./examples/demo-fleet.sh --clean`. See [examples/README.md](examples/README.md)
for the details.

## Keys
```

- [ ] **Step 4: Note the env override in the README Configuration section**

In `README.md`, find this exact text:

```markdown
The Reactor shells out to `ccusage`, and Comms shells out to `gh`, so both must
be installed and authenticated. A repo that gh cannot resolve shows an error
line in Comms, and the rest of the cockpit keeps running. Set your own repos in
the `[comms]` section.
```

Replace it with:

```markdown
The Reactor shells out to `ccusage`, and Comms shells out to `gh`, so both must
be installed and authenticated. A repo that gh cannot resolve shows an error
line in Comms, and the rest of the cockpit keeps running. Set your own repos in
the `[comms]` section.

The `ROCINANTE_FLEET_DIR` environment variable overrides `[fleet] dir` for both
the bridge and `report`. It wins over the config file, which is how the demo
runs in a sandbox. A fully commented reference config lives at
[examples/config.toml](examples/config.toml).
```

- [ ] **Step 5: Add a scope note to CLAUDE.md**

In `CLAUDE.md`, find this exact text:

```markdown
Also done: the v0.3 blocked-agent alert in the header, which turns the health
note amber and leads with any blocked agent.

Deferred to a later session, with a clean seam left in place: the remote Roci
adapter over Tailscale (v0.3).
```

Replace it with:

```markdown
Also done: the v0.3 blocked-agent alert in the header, which turns the health
note amber and leads with any blocked agent.

Also done: a `ROCINANTE_FLEET_DIR` env override and an `examples/demo-fleet.sh`
living demo that drives a sandboxed fake fleet through `rocinante report`, plus
a commented `examples/config.toml` reference.

Deferred to a later session, with a clean seam left in place: the remote Roci
adapter over Tailscale (v0.3).
```

- [ ] **Step 6: Verify the repo still builds and tests pass**

Run: `go build ./... && go test ./... && golangci-lint run ./...`
Expected: builds, all tests ok, 0 lint issues. The docs change nothing in Go, so this is a regression guard.

- [ ] **Step 7: Commit**

```bash
git add examples/config.toml examples/README.md README.md CLAUDE.md
git commit -m "docs: document the live demo and the fleet-dir override

Add examples/README.md and a Live demo section to the README, a commented
examples/config.toml reference, a note on ROCINANTE_FLEET_DIR, and a scope
note in CLAUDE.md.

Claude-Session: https://claude.ai/code/session_01TJtQ77Uiv19mGhhS1yQkM3"
```

---

## Self-Review

**Spec coverage:**
- Fleet-dir env override, defaults < file < env, tilde expansion, table-driven test: Task 1.
- Bash script, dogfoods report, binary resolution, sandbox default, prints run command, `--live`/`--seed`/`--clean`/`--tick`, Ctrl-C trap: Task 2.
- The five-agent cast and screenplay (running hero, blocked alert, idle, error, decay): Task 2, `report_round` and `seed_inbox`.
- `examples/config.toml` reference, `examples/README.md`, root README "Live demo" block, CLAUDE.md scope note: Task 3.
- Testing: env-override test in Task 1; `--seed` plus JSON validation plus `--clean` checks in Task 2.

**Placeholder scan:** No TBDs. Every step shows the actual code or command. Step 2a is a verification note, not a placeholder, because the inner content is fully specified in Step 2.

**Type consistency:** `loadFrom`, `applyEnv`, `applyFile`, `expandHome`, and `Config.Fleet.Dir` match the existing package. The script's flags match `report`'s real flags from `cmd/rocinante/main.go`.

**Out of scope, confirmed absent:** no `rocinante demo` subcommand, no config-path override, no faster demo thresholds.
