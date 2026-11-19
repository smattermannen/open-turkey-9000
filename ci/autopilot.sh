#!/bin/bash
# Unattended loop: one headless Claude Code session per ROADMAP.md item.
#
# Each round: a fresh attempt branch in a private clone, a `claude -p`
# session that takes the next open item, then this script (not the session)
# decides: build, vet, tests, SPEC.md untouched, ROADMAP.md changed. Pass →
# fast-forwarded onto $AUTOPILOT_BRANCH (and pushed if AUTOPILOT_PUSH=1).
# Fail → the branch is kept for you, the reason goes to the next session.
#
# Run it as a user that is NOT in group sudo (no root, no kc9k-bin via the
# sudoers rule, so tests can never lock, freeze or cut the real computer):
#   sudo adduser --disabled-password kc9k-bot
#   sudo -iu kc9k-bot   # then: claude login, git credentials, run this
#
# A usage limit is not a failure: the round is retried after the reset.
# Stop: touch ~/.local/state/kc9k-autopilot/STOP (ends after the round).
set -uo pipefail

ORIGIN=${AUTOPILOT_ORIGIN:-$(git -C "$(dirname "$0")" remote get-url origin)}
BRANCH=${AUTOPILOT_BRANCH:-autopilot}         # where passing work lands
FROM=${AUTOPILOT_FROM:-main}                  # where $BRANCH starts if new
ROUNDS=${AUTOPILOT_ROUNDS:-20}                # 0 = run until stopped
LIMIT_WAIT=${AUTOPILOT_LIMIT_WAIT:-30m}       # usage limit, reset time unknown
MAX_FAILS=${AUTOPILOT_MAX_FAILS:-3}           # failed rounds in a row → stop
TRIES=${AUTOPILOT_TRIES:-2}                   # failed tries per item → skip it
TURNS=${AUTOPILOT_TURNS:-300}
BUDGET=${AUTOPILOT_BUDGET_USD:-}              # per session, empty = none
TIMEOUT=${AUTOPILOT_TIMEOUT:-4h}
MODEL=${AUTOPILOT_MODEL:-}
PERM=${AUTOPILOT_PERMISSION_MODE:-bypassPermissions}
PUSH=${AUTOPILOT_PUSH:-0}
GUI_TESTS=${AUTOPILOT_GUI_TESTS:-auto}        # auto: when xvfb-run exists

STATE=${XDG_STATE_HOME:-$HOME/.local/state}/kc9k-autopilot
REPO=$STATE/repo
LOGS=$STATE/logs
NOTES=$STATE/notes.md       # what failed, read by the next session
SKIP=$STATE/skip.txt        # items given up on, one per line
RESULT=$STATE/result.txt    # written by the session
export PATH="/usr/local/go/bin:$PATH"

die() { echo "autopilot: $*" >&2; exit 1; }
say() { echo "[$(date '+%F %T')] $*" | tee -a "$STATE/autopilot.log"; }

[ "$(id -u)" -ne 0 ] || die "never as root"
if [ -z "${AUTOPILOT_ALLOW_ADMIN:-}" ] && id -nG | tr ' ' '\n' | grep -qxE 'sudo|wheel|admin'; then
	die "$(id -un) is an administrator: tests and the agent could run kc9k-bin as root
on this computer. Use a separate user (see the top of this file) or set
AUTOPILOT_ALLOW_ADMIN=1 inside a throwaway VM."
fi
command -v claude >/dev/null || die "claude not found"
command -v go >/dev/null || die "go not found (expected in /usr/local/go/bin)"

mkdir -p "$LOGS"; touch "$NOTES" "$SKIP"
if [ ! -d "$REPO/.git" ]; then
	git clone -q "$ORIGIN" "$REPO" || die "clone failed"
fi
cd "$REPO" || exit 1
git fetch -q origin
if git rev-parse -q --verify "refs/heads/$BRANCH" >/dev/null; then :
elif git rev-parse -q --verify "refs/remotes/origin/$BRANCH" >/dev/null; then
	git branch -q "$BRANCH" "origin/$BRANCH"
else
	git branch -q "$BRANCH" "origin/$FROM" || die "no origin/$FROM"
fi

gate() { # $1 = base commit; prints why it fails, returns 1
	local base=$1 changed
	[ -z "$(git status --porcelain)" ] || { echo "uncommitted changes left:"; git status --short; return 1; }
	[ "$(git rev-list --count "$base"..HEAD)" -gt 0 ] || { echo "no commits"; return 1; }
	changed=$(git diff --name-only "$base"..HEAD)
	if grep -qx 'SPEC.md' <<<"$changed"; then echo "SPEC.md changed (owner only)"; return 1; fi
	grep -qx 'ROADMAP.md' <<<"$changed" || { echo "ROADMAP.md not updated"; return 1; }
	CGO_ENABLED=0 go build ./... 2>&1 | tail -30; [ "${PIPESTATUS[0]}" -eq 0 ] || { echo "build failed"; return 1; }
	go vet ./... 2>&1 | tail -30; [ "${PIPESTATUS[0]}" -eq 0 ] || { echo "vet failed"; return 1; }
	go test -count=1 ./... 2>&1 | grep -vE '^(ok|\?) ' | tail -60; [ "${PIPESTATUS[0]}" -eq 0 ] || { echo "go test failed"; return 1; }
	if [ "$GUI_TESTS" = 1 ] || { [ "$GUI_TESTS" = auto ] && command -v xvfb-run >/dev/null; }; then
		CGO_ENABLED=0 go build -o kc9k ./cmd/kc9k || return 1
		local t
		for t in ci/gui_*_smoke.py; do
			timeout 15m xvfb-run -a python3 "$t" >"$STATE/gui.out" 2>&1 \
				|| { echo "$t failed:"; tail -40 "$STATE/gui.out"; return 1; }
		done
	fi
}

prompt() {
	cat <<EOF
You are one round of an unattended loop working on this repository. Nobody
is watching and nobody will answer questions: decide, or stop.

1. Read CLAUDE.md, SPEC.md and ROADMAP.md. Follow CLAUDE.md exactly.
2. Pick the FIRST open item in ROADMAP.md order (its own "Order" rule: §0
   first, top to bottom, then §2) that you can finish and verify in this
   container, skipping:
   - items under §0.2 or anything that needs the owner's laptop, real use,
     a real browser, root, the installed daemon or a VM;
   - Android (on hold) and §3/§4/§5 items;
   - items listed here as given up on:
$(sed 's/^/     - /' "$SKIP")
   If the item is large, do one complete, merged-quality step of it and
   say in ROADMAP.md what is left.
3. Where SPEC.md is silent or seems wrong, do not guess and do not edit
   SPEC.md: add a short entry to ROADMAP.md §5 (Proposed, not agreed)
   describing the question, and pick another item.
4. Build it with tests. You may use subagents (Agent tool) for parallel
   exploration, a plan, and a review of your diff before committing.
5. Verify as a normal user: CGO_ENABLED=0 go build ./..., go vet ./...,
   go test ./... and the ci/gui_*_smoke.py touching your change under
   xvfb-run -a if present. Never use sudo, install.sh, make install,
   kc9k/kc9k-bin outside tests, or the browser tests in ci/*_test.py.
6. Update ROADMAP.md in the same commit as the code (status, "on the
   laptop: Unchecked" where real use is still needed, "Last checked").
   Commit on the current branch with a message like the existing ones.
   Leave no uncommitted files. Do not push, do not switch branches.
7. Last, write exactly these lines to $RESULT:
   STATUS: DONE | PARTIAL | NOTHING_LEFT | STUCK
   ITEM: <the item's title as it appears in ROADMAP.md>
   SUMMARY: <one line>
   NOTHING_LEFT only when no item in scope remains.

Notes from earlier rounds (failures this script found after them):
$(tail -40 "$NOTES")
EOF
}

limit_hit() { # $1 = session log; true if the session ended on a usage limit
	tail -5 "$1" | grep -qiE 'usage limit|limit reached|hit your limit|rate.?limit|out of (extra )?usage'
}

wait_for_limit() { # sleeps until the reset the log names, else $LIMIT_WAIT
	local reset now
	reset=$(tail -5 "$1" | grep -oE 'limit reached\|[0-9]{10}' | grep -oE '[0-9]{10}$' | tail -1)
	now=$(date +%s)
	if [ -n "$reset" ] && [ "$reset" -gt "$now" ]; then
		say "usage limit; waiting until $(date -d "@$reset" '+%F %T')"
		sleep $((reset - now + 60))
	else
		say "usage limit; waiting $LIMIT_WAIT"
		sleep "$LIMIT_WAIT"
	fi
}

fails=0
round=0
while [ "$ROUNDS" -eq 0 ] || [ "$round" -lt "$ROUNDS" ]; do
	round=$((round + 1))
	[ -e "$STATE/STOP" ] && { say "STOP file found"; rm -f "$STATE/STOP"; break; }
	git checkout -q "$BRANCH" && git reset -q --hard && git clean -qfd
	base=$(git rev-parse HEAD)
	attempt="autopilot-try/$(date +%Y%m%d-%H%M%S)"
	git checkout -q -b "$attempt"
	rm -f "$RESULT"
	log="$LOGS/$(basename "$attempt").jsonl"
	say "round $round: $attempt from $(git log -1 --format=%h "$base"), log $log"

	args=(-p --output-format stream-json --verbose --max-turns "$TURNS"
		--permission-mode "$PERM"
		--disallowedTools "Bash(sudo:*)" "Bash(git push:*)" "Bash(kc9k-bin:*)" "Bash(./install.sh:*)" "Bash(make install:*)")
	[ -n "$MODEL" ] && args+=(--model "$MODEL")
	[ -n "$BUDGET" ] && args+=(--max-budget-usd "$BUDGET")
	prompt | timeout "$TIMEOUT" claude "${args[@]}" >"$log" 2>&1
	code=$?

	status=$(sed -n 's/^STATUS: *//p' "$RESULT" 2>/dev/null | head -1)
	item=$(sed -n 's/^ITEM: *//p' "$RESULT" 2>/dev/null | head -1)
	summary=$(sed -n 's/^SUMMARY: *//p' "$RESULT" 2>/dev/null | head -1)
	say "session exit $code, ${status:-no result}: ${item:-?} — ${summary:-}"

	if [ -z "$status" ] && limit_hit "$log"; then
		git checkout -q -f "$BRANCH"
		if [ "$(git rev-list --count "$base..$attempt")" -eq 0 ]; then git branch -qD "$attempt"
		else say "kept unfinished work as $attempt"; fi
		round=$((round - 1))
		wait_for_limit "$log"
		continue
	fi

	if [ "$status" = NOTHING_LEFT ] && [ "$(git rev-list --count "$base"..HEAD)" -eq 0 ]; then
		say "nothing left in scope; done"; git checkout -q "$BRANCH"; git branch -qD "$attempt"; break
	fi

	if why=$(gate "$base" 2>&1); then
		git checkout -q "$BRANCH" && git merge -q --ff-only "$attempt" && git branch -qd "$attempt"
		say "PASSED: $BRANCH now $(git log -1 --format='%h %s')"
		fails=0
		[ "$PUSH" = 1 ] && { git push -q origin "$BRANCH" || say "push failed"; }
	else
		fails=$((fails + 1))
		say "FAILED ($fails in a row), kept as $attempt: $(head -1 <<<"$why")"
		{ echo "## $(date '+%F %T') $attempt — ${item:-unknown item}"; echo '```'; tail -25 <<<"$why"; echo '```'; } >>"$NOTES"
		if [ -n "$item" ] && [ "$(grep -cF -- "— $item" "$NOTES")" -ge "$TRIES" ]; then
			grep -qxF -- "$item" "$SKIP" || { echo "$item" >>"$SKIP"; say "giving up on: $item"; }
		fi
		git checkout -q -f "$BRANCH"
		[ "$fails" -ge "$MAX_FAILS" ] && { say "$MAX_FAILS failed rounds in a row; stopping"; break; }
	fi
done
say "finished"
