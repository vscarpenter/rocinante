# Rocinante: Build Spec

**A terminal cockpit for your agent fleet.**
Version 0.1 of the spec. Owner: Vinny Carpenter. Target: hand straight to Claude Code.

---

## 1. Mission

Rocinante is the ship's bridge you watch your whole agent fleet from. It is a keyboard-driven terminal app that shows, at a glance, what every agent is doing, how much it is burning, and what changed while you were in meetings.

The product is a viewer. It reads state that agents already report; it does not orchestrate them. That single boundary keeps the design honest and the blast radius small.

The metaphor organizes the layout. The fleet is the crew, token burn is the reactor, your monitors are comms, and the activity feed is the ship's log.

### Goals

- See fleet status, token burn, and GitHub activity in one keyboard-driven view.
- Add a new agent to the cockpit with zero code changes to Rocinante itself.
- Ship a runnable v0.1 in a weekend, then grow it in clean phases.
- Prove again that the stack was never the bottleneck, this time in a language you have not shipped.

### Non-goals

- No agent control. Rocinante never starts, stops, or commands agents in v1.
- No web UI, no daemon, no database. State lives in flat files and live API pulls.
- No telemetry. Nothing leaves your machines except the API calls you configure.

---

## 2. Requirements

### Functional

- Render four panels: Fleet, Reactor, Comms, and Ship's Log.
- Watch a local directory of agent status files and update the view the instant a file changes.
- Pull token usage from `ccusage` on an interval and show today's burn with a trend.
- Pull open pull requests and CI status from GitHub on an interval.
- Let any agent, hook, or cron job report status through a `rocinante report` subcommand, so nothing hand-writes JSON.
- Detect agents that stopped reporting and mark them stale, then offline.
- Support an inspect view: select an agent and see its full task and recent detail.

### Non-functional

- Single static binary, installable through Homebrew.
- Low idle cost. The app should sit open in a tmux pane all day without noticeable load.
- Keyboard-first. No mouse required for any action.
- Cross-platform: macOS arm64 and amd64 for the bridge, plus Linux amd64 so the same binary can report from Roci on EC2.
- Graceful degradation. A failing adapter shows an error line, never a crash.

### Constraints

- Solo developer, building in a stack that is new to you.
- Existing tools to lean on: `gh`, `ccusage`, and Tailscale are already installed and authenticated.
- v0.1 crew is local only: live Claude Code sessions and launchd or cron monitors. Roci joins one phase later.

---

## 3. The keystone: one binary, two modes

Rocinante ships as a single binary that behaves differently based on how you invoke it.

```bash
rocinante              # launches the TUI bridge
rocinante report ...   # writes or updates an agent status file, then exits
```

The `report` subcommand is what makes the whole design cheap to adopt. A cron job, a Claude Code hook, or Roci itself calls `rocinante report` to announce what it is doing. None of them need to know the file format, the directory, or the schema version. The binary owns all of that.

---

## 4. The Fleet Status Contract

This is the heart of the system, and it is deliberately a versioned, language-agnostic contract. Every crew member speaks it, whether it is three lines of shell or a long-running agent on EC2.

### Location

One file per agent, written locally:

```
~/.rocinante/fleet/<id>.json
```

### Schema, version 1

```json
{
  "schema": 1,
  "id": "gh-watch",
  "name": "GitHub Watcher",
  "kind": "cron",
  "state": "running",
  "task": "Polling org repos for new PRs",
  "detail": "Last pass found 1 new PR (#57)",
  "since": "2026-06-22T13:01:00Z",
  "heartbeat": "2026-06-22T14:55:10Z",
  "tokens_today": 41200000,
  "meta": { "pid": 4823, "cwd": "/Users/vinny/dev/foo" }
}
```

| Field          | Type    | Required | Notes                                                                 |
| -------------- | ------- | -------- | --------------------------------------------------------------------- |
| `schema`       | int     | yes      | Contract version. The bridge refuses files from a future major.       |
| `id`           | string  | yes      | Stable, unique. Becomes the filename.                                  |
| `name`         | string  | yes      | Human label shown in the Fleet panel.                                  |
| `kind`         | enum    | yes      | `always-on`, `cron`, `launchd`, `claude-code`, `other`.               |
| `state`        | enum    | yes      | `running`, `idle`, `blocked`, `error`, `offline`.                      |
| `task`         | string  | no       | One line describing current work.                                     |
| `detail`       | string  | no       | Longer text or last action, shown in the inspect view.                |
| `since`        | RFC3339 | yes      | When the current state began. Drives uptime display.                  |
| `heartbeat`    | RFC3339 | yes      | Last update. Drives staleness detection.                              |
| `tokens_today` | int     | no       | For agents that consume tokens. Feeds nothing in v1 beyond display.   |
| `meta`         | object  | no       | Freeform. Process id, working directory, anything useful to inspect.  |

### Derived state: staleness

Agents crash. When they do, they never write `offline`, so the bridge has to infer it from silence.

- If `now - heartbeat` exceeds `stale_after` (config, default 90s), the agent renders **stale** and dims.
- If it exceeds `offline_after` (config, default 300s), the agent renders **offline**.
- Thresholds are configurable per `kind`, since a cron job that runs every five minutes is not stale at 90 seconds.

### Atomic writes

`rocinante report` writes to a temp file and renames it into place. The rename is atomic on a single filesystem, so the bridge never reads a half-written file. This is small but it is the difference between a solid tool and a flaky one.

---

## 5. How each crew member reports

### launchd and cron monitors (day one)

You own these, so they simply call the binary. A monitor announces itself at the start and end of each pass.

```bash
# at the top of the run
rocinante report --id gh-watch --name "GitHub Watcher" \
  --kind cron --state running --task "Polling org repos for new PRs"

# at the end
rocinante report --id gh-watch --state idle \
  --detail "Found 1 new PR (#57)"
```

### Claude Code sessions (day one)

The primary mechanism is Claude Code hooks. A hook fires on session and tool lifecycle events and calls `rocinante report`, so a live session appears on the bridge with no polling and no log scraping.

Illustrative hook configuration. The exact event names, matcher syntax, and available environment variables must be confirmed against your installed Claude Code version before building this part.

```jsonc
{
  "hooks": {
    "SessionStart": [
      { "hooks": [{ "type": "command",
        "command": "rocinante report --id cc-$SESSION_ID --kind claude-code --state running --task \"$PROMPT\"" }] }
    ],
    "Stop": [
      { "hooks": [{ "type": "command",
        "command": "rocinante report --id cc-$SESSION_ID --state idle" }] }
    ]
  }
}
```

Fallback if hooks prove insufficient: a `claudecode` adapter watches `~/.claude/projects/**/*.jsonl`, treats a recent modification time as running, and tail-parses the last entry for the current task. This path is fragile across versions, so it is the backup, not the default.

### Roci (one phase later)

Roci writes the same-schema status file on EC2. The bridge pulls it across the tailnet on an interval. Since Roci already lives on Tailscale, the recommended transport is Tailscale SSH reading the file, which adds no new long-running service to Roci. The adapter caches the last good read so a brief network blip shows stale rather than vanishing.

---

## 6. Pulled data sources

Some state cannot write a file, so the bridge pulls it on a timer. Push for what you own, pull for what you do not.

### Reactor (token burn)

- Shell out to `ccusage` with JSON output on an interval (default 60s).
- Display today's total tokens, cache-read ratio, and a sparkline of recent samples.
- The exact subcommand and JSON shape must be confirmed against your installed `ccusage`, since you know the precise invocation.

### Comms (GitHub)

- Shell out to `gh` rather than managing raw API auth, which reuses your existing token cleanly.
- Example: `gh pr list --json number,title,reviewDecision,statusCheckRollup` plus a call for CI rollup state.
- Repos or org to watch come from config. Default interval 90s.

---

## 7. Architecture

Rocinante follows the Bubble Tea model, which is the Elm pattern: a single immutable `Model`, an `Update` function that folds messages into new state, and a `View` that renders the current state.

### Data flow

```
                 fsnotify (instant)
  fleet/*.json  ───────────────────►┐
                                     │
  ccusage    ──poll 60s──► tea.Cmd ──┤
  gh         ──poll 90s──► tea.Cmd ──┼──► Update(Model, Msg) ──► View ──► terminal
  roci file  ──pull 30s──► tea.Cmd ──┤
                                     │
  keyboard   ─────────────► KeyMsg ──┘
```

### Message types

- `fleetUpdatedMsg` from the file watcher.
- `reactorMsg`, `commsMsg`, `rociMsg` from the polling commands.
- `tickMsg` to drive the polling cadence.
- `errMsg` so a failed adapter surfaces a banner instead of crashing.
- Standard `tea.KeyMsg` for navigation.

### The tricky integration: fsnotify inside Bubble Tea

fsnotify runs in its own goroutine and pushes events onto a channel. Bridge that channel into Bubble Tea with a command that blocks on the channel, emits a `fleetUpdatedMsg`, and is re-issued from `Update` so it listens again. This is the standard Bubble Tea pattern for external event streams, and it keeps the render loop single-threaded and safe.

### Adapters are isolated

Each pulled source is an adapter with one job and a timeout. If `gh` hangs or `ccusage` is missing, that adapter returns an `errMsg`, the Comms or Reactor panel shows a one-line error, and the rest of the cockpit keeps running. No single source can take down the bridge.

---

## 8. Interface

### Layout

```
┌─ ROCINANTE ───────────────────────────── 14:55 CDT · all systems nominal ─┐
│ FLEET                             │ REACTOR  (tokens, today)              │
│  ● cc-session  running  33m       │  1.91B tok    cache-read 96%          │
│    └ refactor: bridge render loop │  ▁▂▃▅▇▆▄▃▂▁▂▄   pace: on budget       │
│  ● gh-watch    idle     ··        ├─ COMMS  (GitHub)                      │
│  ◌ inbox-agent offline  12m       │  3 open PRs    2 need review          │
│                                   │  CI green      1 branch ahead         │
├─ SHIP'S LOG ──────────────────────┴───────────────────────────────────────┤
│ 14:52  cc-session  committed "wire up fsnotify watcher"                    │
│ 14:40  gh-watch    found 1 new PR (#57)                                    │
├────────────────────────────────────────────────────────────────────────────┤
│ [tab] panels  [enter] inspect  [l] logs  [r] refresh  [q] abandon ship      │
└────────────────────────────────────────────────────────────────────────────┘
```

### Keybindings

| Key       | Action                                    |
| --------- | ----------------------------------------- |
| `tab`     | Cycle focus across panels                 |
| `↑` `↓`   | Move selection within the focused panel   |
| `enter`   | Inspect the selected agent                |
| `l`       | Expand the Ship's Log to full screen      |
| `r`       | Force-refresh all pulled sources          |
| `?`       | Toggle a help overlay                     |
| `q`       | Quit                                      |

### State colors, mapped to Signal Ledger

Lip Gloss carries truecolor, so the palette transfers even though the terminal supplies its own font. Geist does not render here; only the colors come along.

| State    | Glyph | Color intent                       |
| -------- | ----- | ---------------------------------- |
| running  | `●`   | Accent blue, `#2167f3`             |
| idle     | `○`   | Muted foreground                   |
| blocked  | `▲`   | Amber, needs attention             |
| error    | `✕`   | Red                                |
| offline  | `◌`   | Dimmed gray                        |
| stale    | `●`   | Running glyph, dimmed              |

Surface is `#fbfbfa`. Use `lipgloss.AdaptiveColor` so the cockpit reads well in both light and dark terminals. Confirm the full Signal Ledger ramp before locking the theme; the two values above are from memory.

### Responsive behavior

- Define a minimum size, roughly 80 by 24. Below it, show a friendly "make the window bigger" message rather than a broken layout.
- Above the minimum, the top row splits Fleet on the left and a stacked Reactor and Comms on the right. The Ship's Log spans the full width.

---

## 9. Configuration

A single TOML file at `~/.rocinante/config.toml`.

```toml
[fleet]
dir           = "~/.rocinante/fleet"
stale_after   = "90s"
offline_after = "300s"

[reactor]
enabled  = true
command  = "ccusage"
args     = ["--json"]
interval = "60s"

[comms]
enabled  = true
repos    = ["vscarpenter/rocinante", "vscarpenter/inkwell"]
interval = "90s"

[roci]
enabled  = false        # turns on in the tailnet phase
host     = "roci"       # Tailscale machine name
path     = "~/.rocinante/status.json"
interval = "30s"

[theme]
mode = "adaptive"       # adaptive | light | dark
```

Ship sane defaults so the app runs with no config at all. The file only overrides.

---

## 10. Project structure

```
rocinante/
  cmd/rocinante/main.go          # cobra root (TUI) plus the report subcommand
  internal/
    config/                      # TOML load, validate, defaults
    fleet/                       # contract types, atomic read/write, staleness, fsnotify store
    report/                      # the `report` subcommand logic
    adapters/
      ccusage/                   # Reactor
      github/                    # Comms
      roci/                      # remote file pull over Tailscale (later phase)
      claudecode/                # JSONL fallback watcher (hooks need no adapter)
    ui/                          # bubbletea model, update, view, components, theme
  hooks/                         # sample Claude Code hook snippets
  .goreleaser.yaml
  .github/workflows/ci.yml
  README.md
  LICENSE                        # MIT
```

Use `cobra` for the command tree, with the bare command launching the TUI and `report` as a subcommand.

---

## 11. Stack and dependencies

- Go, latest stable, 1.22 or newer.
- `github.com/charmbracelet/bubbletea` for the loop.
- `github.com/charmbracelet/lipgloss` for styling and the palette.
- `github.com/charmbracelet/bubbles` for ready-made widgets, such as viewport and table.
- `github.com/fsnotify/fsnotify` for the fleet directory watch.
- `github.com/spf13/cobra` for the command tree.
- `github.com/BurntSushi/toml` for config, or `pelletier/go-toml/v2` if you prefer.

---

## 12. Distribution and open source

Public from day one, so the plumbing is part of v1, not an afterthought.

- **License:** MIT, matching your other repositories. Confirm before release.
- **Releases:** goreleaser builds darwin arm64, darwin amd64, and linux amd64, and publishes archives on a tag.
- **Homebrew:** a tap at `vscarpenter/homebrew-tap` with a generated formula, so `brew install vscarpenter/tap/rocinante` works.
- **CI:** GitHub Actions runs `golangci-lint`, `go test`, and a build on every pull request, then runs goreleaser on a version tag.
- **README:** the wireframe up top, a sixty-second quickstart, the Fleet Status Contract documented for contributors, and copy-paste recipes for wiring a cron monitor and a Claude Code hook.
- **AI-readable:** add an `llms.txt` describing the contract, consistent with how you document the rest of your work.

---

## 13. Milestones

### v0.1, the spark (a weekend)

- TUI shell with the Fleet and Ship's Log panels, themed and keyboard-navigable.
- The Fleet Status Contract and the `rocinante report` subcommand.
- fsnotify-backed local fleet store with staleness and offline detection.
- Day-one crew live: cron and launchd monitors through `report`, and Claude Code sessions through hooks.
- Outcome: a real binary showing real agents.

### v0.2, fuel and comms

- Reactor through `ccusage` and Comms through `gh`, both config-driven.
- The inspect view: select an agent, see its full task and detail.

### v0.3, the tailnet

- Roci remote-pull adapter over Tailscale, with cached last-good reads.
- A blocked-agent alert line in the header so problems surface without hunting.

### v1.0, polish and ship

- goreleaser, Homebrew tap, and CI green.
- Light and dark themes finalized against the real Signal Ledger ramp.
- README and contract docs complete.
- Optional flourish: your next F1 session in the status bar.
- A blog post writes itself, since you shipped a polished TUI in a language you had never used.

---

## 14. Trade-offs, made explicit

- **Go over Python.** A clean single binary and the Charm ecosystem win, at the cost of learning a new language. That cost is the point, and it is small when pairing with Claude Code.
- **Shell out to `gh` and `ccusage` rather than reimplement them.** Less code and reused auth, at the cost of depending on those binaries being installed. Acceptable, since they already are.
- **Hooks over log scraping for Claude Code.** Robust and push-based, at the cost of a small one-time hook setup. The JSONL watcher stays as a documented fallback.
- **Files over a database.** Trivial to inspect and debug, at the cost of no query layer. The fleet is small, so this is the right call for a long time.
- **Viewer, not controller.** Smaller blast radius and a simpler mental model, at the cost of not being able to act on an agent from the bridge. Revisit only if you genuinely want control later.

---

## 15. Verify before building

Honest unknowns to confirm rather than assume:

1. Exact Claude Code hook event names, matcher syntax, and environment variables for your installed version.
2. The precise `ccusage` subcommand and the shape of its JSON.
3. The full Signal Ledger color ramp. The surface and accent here are from memory.
4. The Tailscale transport for Roci's file. Recommendation is Tailscale SSH reading the file, for zero new services on Roci.
5. License confirmation. MIT assumed.

---

## 16. Definition of done, v1

- `brew install` lands a working binary.
- `rocinante` shows a live fleet of Claude Code sessions, cron monitors, and Roci.
- The Reactor shows today's token burn from `ccusage`.
- Comms shows open pull requests and CI status.
- Agents report through `rocinante report`, and stale and offline states are handled.
- Light and dark both read well.
- CI is green, and the README is complete enough that a stranger can add their own agent.
