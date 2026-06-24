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
