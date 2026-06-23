# Claude Code Kickoff: Rocinante

You are helping me build **Rocinante**, a terminal cockpit for my agent fleet, written in Go with the Charm stack (Bubble Tea, Lip Gloss, Bubbles). I am the sole developer, and this is the first session on a fresh repository.

## Source of truth

The complete build spec is in `rocinante-build-spec.md`. Read it first, in full, and treat it as the source of truth. If this prompt and the spec ever disagree, the spec wins. If the spec is silent or ambiguous on something, ask me rather than guessing.

## What we are building this session

The local spine of v0.1, end to end. By the end I want a compiling binary where four pieces work together with no external dependencies:

1. The Fleet Status Contract, the versioned JSON schema in spec section 4.
2. The `rocinante report` subcommand that writes status files.
3. A local fleet store that watches the directory and derives staleness.
4. A minimal Bubble Tea TUI that renders the Fleet panel and the Ship's Log.

I am deliberately deferring the Reactor (ccusage), Comms (GitHub), Roci, and Claude Code hooks to later sessions. Those depend on tools whose exact behavior we need to verify first, and the spine does not need them. Leave clean interfaces where they will plug in, and stop there.

## Build order

Work through these in order. Build and test after each step, and pause at natural checkpoints so I can redirect.

1. **Scaffold.** Create the Go module `github.com/vscarpenter/rocinante` and the directory structure from spec section 10. Set up the cobra command tree so the bare command launches the TUI and `report` is a subcommand. Add an MIT LICENSE, a short README stub, and a `CLAUDE.md` that captures the conventions in this prompt, so future sessions inherit them. Confirm it builds.
2. **The contract.** Implement the status types, atomic writes (temp file plus rename), reads, and the staleness and offline derivation from heartbeat. Add table-driven tests for the threshold logic and the atomic write.
3. **The report subcommand.** Wire the flags (`--id`, `--name`, `--kind`, `--state`, `--task`, `--detail`, `--tokens`), validate them, and write or merge the file atomically. Stamp `heartbeat` to now, and reset `since` only when the state actually changes. Test that it produces valid contract files.
4. **The fleet store.** Load existing files on startup, then watch the fleet directory with fsnotify. Bridge the watcher channel into Bubble Tea using the listen-and-reissue pattern described in spec section 7, so the render loop stays single-threaded.
5. **The minimal TUI.** Build the Bubble Tea model, update, and view. Render the Fleet panel and the Ship's Log roughly as laid out in spec section 8. Style it with Lip Gloss using the Signal Ledger palette, accent `#2167f3` and surface `#fbfbfa`, with adaptive colors for light and dark terminals. Wire keyboard navigation for tab, arrows, and quit, and dim agents that have gone stale.
6. **End to end.** Run the binary, call `rocinante report` from another shell, and confirm the agent appears, updates live, and goes stale on schedule.

## Engineering guardrails

- Write idiomatic Go in small, focused packages, matching the layout in spec section 10.
- Nothing panics on bad input. A malformed status file becomes a visible error state, not a crash. This holds for the render loop and every file read.
- Use only the dependencies approved in spec section 11. Flag before adding anything else, and tell me why.
- Write status files atomically, every time.
- Keep `golangci-lint` clean and tests table-driven.
- Make small, logical commits with conventional commit messages.

## House style for anything you write

Any prose you produce, including the README, code comments, commit messages, and the CLAUDE.md, follows my style: active voice, AP style, Oxford commas, no em dashes, and sentences under 25 words. Use commas, periods, or semicolons in place of em dashes.

## Do not invent

Do not fabricate Claude Code hook event names, the ccusage JSON shape, the full Signal Ledger color ramp, or Roci's Tailscale transport. All of these are out of scope this session. When we reach them, we verify against my installed tools and my real design tokens before writing a line that depends on them.

## How to start

Before writing code, give me a short confirmation of the build order as a todo list, so I can catch anything early. Then execute step by step. Do not build ahead of this slice or gold-plate it.

## Definition of done for this session

I can run `rocinante report --id test --name "Test" --kind cron --state running --task "smoke test"`, then watch it appear live in `rocinante`, update when I report again, and flip to stale and then offline on the configured thresholds. The build is green and the tests pass.
