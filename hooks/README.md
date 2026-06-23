# Claude Code hooks

These hooks make your live Claude Code sessions appear on the Rocinante bridge
with no polling and no log scraping. Claude Code runs a command on each
lifecycle event and passes the event as JSON on stdin. The `rocinante hook`
subcommand reads that payload and reports the session, so your settings never
touch the contract.

## What each event does

| Event              | Effect on the session card                         |
| ------------------ | -------------------------------------------------- |
| `SessionStart`     | Appears as running, labeled by its directory.      |
| `UserPromptSubmit` | Sets the current task to your prompt's first line. |
| `PreToolUse`       | Refreshes the heartbeat so a busy turn stays fresh.|
| `Stop`             | Marks the session idle when the turn finishes.     |
| `SessionEnd`       | Marks the session offline when it closes.          |

`PreToolUse` only refreshes the heartbeat. It does not change the task or detail,
so the Ship's Log stays quiet during a busy turn while the card stays fresh.

## Install

First put `rocinante` on your `PATH`, or replace `rocinante` in the snippet with
an absolute path.

The events live under `hooks` in `~/.claude/settings.json`. If you already have
hooks, merge these entries rather than replacing the file. Each event is an array,
so append a new object to any event you already use.

See [claude-code-hooks.json](claude-code-hooks.json) for the snippet to merge.

## Verify

Start a fresh Claude Code session in any project, then launch `rocinante` in
another pane. The session appears as a `claude-code` agent. Send a prompt, and
its task updates. The card goes idle when the turn ends, then offline when you
close the session.

The hook never fails loudly. Any error exits zero, so a status hook can never
disrupt the session it is observing.
