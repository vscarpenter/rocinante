# CLAUDE.md

Conventions for working on Rocinante. Future sessions inherit them from here.

## What this is

Rocinante is a terminal cockpit for an agent fleet, written in Go with the Charm
stack: Bubble Tea, Lip Gloss, and Bubbles. It is a viewer. It reads state that
agents already report; it never starts, stops, or commands them. That boundary
keeps the blast radius small, so hold it.

## Source of truth

`rocinante-build-spec.md` is the source of truth. Read it before building. If a
prompt and the spec disagree, the spec wins. If the spec is silent or ambiguous,
ask rather than guessing.

## House style for prose

This applies to every word you write: the README, code comments, commit
messages, and this file.

- Use active voice.
- Follow AP style.
- Use Oxford commas.
- Do not use em dashes. Use commas, periods, or semicolons instead.
- Keep sentences under 25 words.

## Engineering guardrails

- Write idiomatic Go in small, focused packages, matching the layout below.
- Nothing panics on bad input. A malformed status file becomes a visible error
  state, not a crash. This holds for the render loop and every file read.
- Write status files atomically, every time, with a temp file plus a rename.
- Use only the dependencies approved in build spec section 11. Flag before
  adding anything else, and say why.
- Keep golangci-lint clean and tests table-driven.
- Make small, logical commits with conventional commit messages.

## Project layout

```
cmd/rocinante/main.go     cobra root (TUI) plus the report subcommand
internal/config/          settings, defaults, and a seam for TOML loading
internal/fleet/           contract types, atomic read and write, staleness, fsnotify store
internal/report/          the report subcommand logic
internal/adapters/        ccusage, github, roci, and claudecode (all deferred)
internal/ui/              Bubble Tea model, update, view, components, and theme
hooks/                    sample Claude Code hook snippets
```

## Scope discipline

Do not build ahead of the current slice, and do not gold-plate it.

Done: v0.1, the local spine, the Fleet Status Contract, the `report` subcommand,
an fsnotify-backed fleet store with staleness, and a Fleet and Ship's Log TUI.
Also done: v0.2, config loading, the Reactor through `ccusage`, Comms through
`gh`, and the inspect view. Also done: Claude Code hooks through `rocinante hook`.

Deferred to later sessions, with clean seams left in place: the remote Roci
adapter over Tailscale (v0.3). The distribution plumbing, goreleaser, a Homebrew
tap, and CI, lands in v1.0.

## Verified invocations

Confirmed against the installed tools on 2026-06-22. Update here if they change.

- Reactor: `ccusage daily --json` on ccusage 18.0.10. The shape is a `daily[]`
  array of per-day objects with `totalTokens`, `cacheReadTokens`, and
  `totalCost`, plus a `totals` object.
- Comms: `gh pr list --json number,title,reviewDecision,statusCheckRollup,isDraft`
  on gh 2.95.0. Rollup entries are `CheckRun` with `status` and `conclusion`, or
  `StatusContext` with `state`.
- Hooks: Claude Code 2.1.186 delivers each event as JSON on stdin, not env vars.
  Fields used are `session_id`, `cwd`, `hook_event_name`, `prompt`, `tool_name`,
  and `source`. Handled events are SessionStart, UserPromptSubmit, PreToolUse,
  Stop, and SessionEnd.

## Do not invent

Do not fabricate any of these. Confirm each against the installed tools and the
real design tokens before writing code that depends on it.

- The full Signal Ledger color ramp. Only accent `#2167f3` and surface `#fbfbfa`
  are confirmed. The amber, red, green, and gray values are neutral placeholders.
- Roci's Tailscale transport.
