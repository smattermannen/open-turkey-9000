#!/bin/bash
# Starts one ci/autopilot.sh worker per Claude account and waits for all.
#   ci/autopilot-team.sh ~/.claude-a ~/.claude-b
# Each argument is a CLAUDE_CONFIG_DIR logged in to its own account
# (`CLAUDE_CONFIG_DIR=~/.claude-b claude`, then /login); the worker is
# named after it. AUTOPILOT_* settings apply to every worker.
# Output: ~/.local/state/kc9k-autopilot/autopilot.log (all workers) and
# <worker>/out.txt. Stop: touch ~/.local/state/kc9k-autopilot/STOP.
set -u
[ $# -ge 1 ] || { echo "usage: $0 CLAUDE_CONFIG_DIR..." >&2; exit 1; }
ROOT=${XDG_STATE_HOME:-$HOME/.local/state}/kc9k-autopilot
here=$(cd "$(dirname "$0")" && pwd)
pids=()
for dir in "$@"; do
	[ -d "$dir" ] || { echo "$dir: no such directory (log in first)" >&2; exit 1; }
	name=$(basename "$dir" | sed 's/^\.//; s/[^A-Za-z0-9_-]/-/g')
	mkdir -p "$ROOT/$name"
	AUTOPILOT_WORKER=$name CLAUDE_CONFIG_DIR=$(cd "$dir" && pwd) \
		"$here/autopilot.sh" >>"$ROOT/$name/out.txt" 2>&1 &
	pids+=($!)
	echo "worker $name: pid $!, account in $dir"
	sleep 20 # the first worker sets up the hub and claims first
done
trap 'kill "${pids[@]}" 2>/dev/null' INT TERM
wait
