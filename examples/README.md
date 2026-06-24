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
