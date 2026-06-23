# Rocinante

[![CI](https://github.com/vscarpenter/rocinante/actions/workflows/ci.yml/badge.svg)](https://github.com/vscarpenter/rocinante/actions/workflows/ci.yml)

A terminal cockpit for your agent fleet.

Rocinante is the bridge you watch your whole agent fleet from. It shows, at a
glance, what every agent is doing, how much it is burning, and what changed
while you were in meetings. The product is a viewer. It reads state that agents
already report; it does not orchestrate them.

> Status: v0.2 in progress. This repository is being built in phases. Working
> today: the Fleet Status Contract, the `report` subcommand, a file-watching
> fleet store, and a four-panel TUI. The Fleet and Ship's Log panels show local
> agents, the Reactor panel shows today's token burn from `ccusage`, the Comms
> panel shows open pull requests and CI from `gh`, and `enter` inspects an agent.

## One binary, two modes

```bash
rocinante              # launch the TUI bridge
rocinante report ...   # write or update an agent status file, then exit
```

The `report` subcommand is what makes the design cheap to adopt. A cron job, a
Claude Code hook, or a remote agent calls `rocinante report` to announce what it
is doing. None of them need to know the file format, the directory, or the
schema version. The binary owns all of that.

## Install

Once a release is published, install with Go:

```bash
go install github.com/vscarpenter/rocinante/cmd/rocinante@latest
```

A Homebrew tap is planned, so `brew install vscarpenter/tap/rocinante` will work
after the first tagged release. Until then, build from source as below.

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

## Keys

| Key       | Action                          |
| --------- | ------------------------------- |
| `tab`     | Cycle focus across panels       |
| `↑` `↓`   | Move the selection or scroll    |
| `enter`   | Inspect the selected agent      |
| `l`       | Expand the Ship's Log full screen |
| `esc`     | Leave the inspect or log view   |
| `r`       | Refresh the Reactor and Comms   |
| `q`       | Quit                            |

## The Fleet Status Contract

Every crew member speaks one versioned, language-agnostic contract. One file per
agent lives at `~/.rocinante/fleet/<id>.json`. See the build spec for the full
schema and field reference.

## Configuration

The app runs with no config at all. To override defaults, write
`~/.rocinante/config.toml`. The file only overrides; omitted keys keep their
defaults.

```toml
[fleet]
dir           = "~/.rocinante/fleet"
stale_after   = "90s"
offline_after = "300s"

[reactor]
enabled  = true
command  = "ccusage"
args     = ["daily", "--json"]
interval = "60s"

[comms]
enabled  = true
repos    = ["vscarpenter/rocinante", "vscarpenter/inkwell"]
interval = "90s"

[theme]
mode = "adaptive"
```

The Reactor shells out to `ccusage`, and Comms shells out to `gh`, so both must
be installed and authenticated. A repo that gh cannot resolve shows an error
line in Comms, and the rest of the cockpit keeps running. Set your own repos in
the `[comms]` section.

## Claude Code sessions

Your live Claude Code sessions can appear on the bridge automatically through
hooks. Claude Code runs `rocinante hook` on each lifecycle event and passes the
event on stdin, so a session shows up as a `claude-code` agent with its current
task. See [hooks/README.md](hooks/README.md) for the setup.

## Releasing

CI runs `golangci-lint`, `go test`, and a build on every push and pull request.
To cut a release, tag a commit and push the tag:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The release workflow runs goreleaser, which builds darwin arm64, darwin amd64,
and linux amd64, then publishes archives and checksums on the GitHub release. To
add the Homebrew tap, create `vscarpenter/homebrew-tap`, store a
`HOMEBREW_TAP_GITHUB_TOKEN` secret, and uncomment the `brews` block in
`.goreleaser.yaml`.

## Roadmap

- v0.3 adds a remote Roci adapter over Tailscale.
- v1.0 finalizes the light and dark themes against the real Signal Ledger ramp.
  The CI, goreleaser, and `llms.txt` plumbing is in place.

## License

MIT. See [LICENSE](LICENSE).
