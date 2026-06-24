# Demo fleet: a living simulation for new clones

Date: 2026-06-23. Status: approved, ready for planning.

## Goal

Give anyone who clones Rocinante a one-command, living demonstration. A small
script drives a fake fleet through `rocinante report`, so the cockpit fills with
agents that change state, scroll the Ship's Log, trip the blocked alert, and
decay to stale and offline while the operator watches.

The demo is a viewer-faithful showcase. It only calls the public `report`
interface, exactly as a real agent would, so it doubles as living documentation
of the Fleet Status Contract.

## Decisions

- **Living simulation, not a static snapshot.** A loop keeps healthy agents
  fresh and lets one go silent, so staleness and offline derive naturally.
- **Sandbox through one env var.** A new `ROCINANTE_FLEET_DIR` override points
  both the reader and the writer at a temp directory, so the demo never touches
  a real `~/.rocinante/fleet`.
- **Bash, not a Go subcommand.** The script dogfoods `rocinante report` and adds
  no demo code to the shipped binary.
- **Default thresholds.** Stale lands near 90 seconds and offline at 5 minutes.
  We accept the real timescale rather than add a config override to rush it.

## Components

### 1. Fleet-dir env override (`internal/config/config.go`)

Apply `ROCINANTE_FLEET_DIR` after the TOML file loads, so precedence reads
defaults, then `config.toml`, then the env var. A leading `~` expands through the
existing `expandHome`. This is the only change to the shipped binary. Both
`rocinante` and `rocinante report` call `config.Load`, so the single change
sandboxes the whole demo. The override is useful beyond the demo, for testing
and for running more than one fleet.

A table-driven test in `config_test.go` covers three cases: env unset keeps the
file or default value, env set overrides it, and a leading `~` expands.

### 2. The demo script (`examples/demo-fleet.sh`)

Plain bash. It:

- Resolves a binary: `$ROCINANTE_BIN`, then `./rocinante`, then `rocinante` on
  `PATH`, then a one-time `go build` to a temp path.
- Defaults `ROCINANTE_FLEET_DIR` to a temp directory when unset, exports it, and
  prints the exact `ROCINANTE_FLEET_DIR=... rocinante` command to run in another
  pane.
- Loops a scripted timeline on a tick, default 10 seconds, narrating each round.
- Supports `--live` (loop, the default), `--seed` (one pass, then exit),
  `--clean` (delete the sandbox), and `--tick N`.
- Traps Ctrl-C, leaves the final frame in place for inspection, and prints the
  clean command.

Because `report` always stamps a fresh heartbeat, a living agent stays healthy
only while the loop re-reports it. Dropping an agent from the loop is what makes
it decay.

### 3. The cast and screenplay

Five agents, each showing a different mechanic. The demo exercises the Fleet
panel, the Ship's Log, the header alert, and the inspect view. The Reactor and
Comms pull from real `ccusage` and `gh`, so they light up when those tools exist
or show a graceful one-line error when they do not.

| id | kind | arc | demonstrates |
|----|------|-----|--------------|
| `cc-refactor` | claude-code | runs throughout; task and detail evolve; `--tokens` grows | running glyph, log scroll, tokens in inspect |
| `gh-watch` | cron | running, idle, blocked, idle | the amber header alert |
| `nightly-sync` | launchd | idle, with one running pass | idle and kind variety |
| `deploy-bot` | other | running, error, running | the red error glyph |
| `inbox-agent` | always-on | one pass, then goes silent | derived stale at 90s, offline at 5m |

### 4. Supporting files

- `examples/config.toml`: a fully commented reference config. It documents every
  section and doubles as the config reference the build spec asks for.
- `examples/README.md`: how to run the demo, in two panes.
- Root `README.md`: a short "Live demo" block that points at `examples/`.

## Out of scope

- A `rocinante demo` Go subcommand. Demo logic stays out of the binary.
- A config-path override. The fleet-dir override is enough to sandbox.
- Faster demo thresholds. The default timescale is the accepted behavior.

## Testing

- `config_test.go` gains the table-driven env-override test.
- The script is verified by running `--seed` against a temp `ROCINANTE_FLEET_DIR`,
  then asserting the expected `<id>.json` files exist and parse. A `--clean` run
  then leaves the sandbox empty.
