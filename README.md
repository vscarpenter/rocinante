# Rocinante

A terminal cockpit for your agent fleet.

Rocinante is the bridge you watch your whole agent fleet from. It shows, at a
glance, what every agent is doing, how much it is burning, and what changed
while you were in meetings. The product is a viewer. It reads state that agents
already report; it does not orchestrate them.

> Status: v0.1 in progress. This repository is being built in phases. The local
> spine works today: the Fleet Status Contract, the `report` subcommand, a
> file-watching fleet store, and a Fleet and Ship's Log TUI.

## One binary, two modes

```bash
rocinante              # launch the TUI bridge
rocinante report ...   # write or update an agent status file, then exit
```

The `report` subcommand is what makes the design cheap to adopt. A cron job, a
Claude Code hook, or a remote agent calls `rocinante report` to announce what it
is doing. None of them need to know the file format, the directory, or the
schema version. The binary owns all of that.

## Quickstart

```bash
go build -o rocinante ./cmd/rocinante

# In one shell, launch the bridge.
./rocinante

# In another shell, report an agent.
./rocinante report --id test --name "Test" --kind cron \
  --state running --task "smoke test"
```

The agent appears live. Report again to update it. Stop reporting, and it flips
to stale, then offline, on the configured thresholds.

## The Fleet Status Contract

Every crew member speaks one versioned, language-agnostic contract. One file per
agent lives at `~/.rocinante/fleet/<id>.json`. See the build spec for the full
schema and field reference.

## Roadmap

- v0.2 adds the Reactor through `ccusage` and Comms through `gh`.
- v0.3 adds a remote Roci adapter over Tailscale.
- v1.0 adds goreleaser, a Homebrew tap, and finished themes.

## License

MIT. See [LICENSE](LICENSE).
