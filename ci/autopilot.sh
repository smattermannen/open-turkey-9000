#!/bin/bash
# Unattended loop: headless Claude Code sessions working down ROADMAP.md.
#
# This script (not the sessions) decides which item each session gets and
# whether its work lands. Each round: the next open Index row this worker
# can do and no other worker holds, a fresh attempt branch in this worker's
# own clone, a `claude -p` session told its item and the other workers'
# items. Then, one worker at a time: rebase onto what the others landed (a
# short session resolves conflicts), gate (build, vet, tests, SPEC.md
# untouched, ROADMAP.md changed), fast-forward $AUTOPILOT_BRANCH in the
# shared hub (and push to GitHub if AUTOPILOT_PUSH=1). A failure keeps the
# branch and tells the next session why.
#
# Several workers: one per Claude account, each with its own config dir
# (log in once with `CLAUDE_CONFIG_DIR=<dir> claude`, then /login), started
# by ci/autopilot-team.sh or as
#   AUTOPILOT_WORKER=a CLAUDE_CONFIG_DIR=~/.claude-a ci/autopilot.sh
#
# Run it as a user that is NOT in group sudo (no root, no kc9k-bin via the
# sudoers rule, so tests can never lock, freeze or cut the real computer):
#   sudo adduser --disabled-password kc9k-bot
#   sudo -iu kc9k-bot   # then: claude login, git credentials, run this
#
# A usage limit is not a failure: the round is retried after the reset.
# Stop: touch ~/.local/state/kc9k-autopilot/STOP (every worker ends after
# its round; starting a worker clears it).
set -uo pipefail

ORIGIN=${AUTOPILOT_ORIGIN:-$(git -C "$(dirname "$0")" remote get-url origin)}
BRANCH=${AUTOPILOT_BRANCH:-autopilot}         # where passing work lands
FROM=${AUTOPILOT_FROM:-main}                  # where $BRANCH starts if new
WORKER=${AUTOPILOT_WORKER:-solo}              # one name per Claude account
NEEDS=${AUTOPILOT_NEEDS:-agent}               # Index "Needs" this worker can do (regex)
ROUNDS=${AUTOPILOT_ROUNDS:-20}                # 0 = run until stopped
LIMIT_WAIT=${AUTOPILOT_LIMIT_WAIT:-30m}       # usage limit, reset time unknown
IDLE_WAIT=${AUTOPILOT_IDLE_WAIT:-15m}         # only other workers' items left
MAX_FAILS=${AUTOPILOT_MAX_FAILS:-3}           # failed rounds in a row → stop
TRIES=${AUTOPILOT_TRIES:-2}                   # failed tries per item → skip it
TURNS=${AUTOPILOT_TURNS:-300}
BUDGET=${AUTOPILOT_BUDGET_USD:-}              # per session, empty = none
TIMEOUT=${AUTOPILOT_TIMEOUT:-4h}
MODEL=${AUTOPILOT_MODEL:-}
PERM=${AUTOPILOT_PERMISSION_MODE:-bypassPermissions}
PUSH=${AUTOPILOT_PUSH:-0}
GUI_TESTS=${AUTOPILOT_GUI_TESTS:-auto}        # auto: when xvfb-run exists

ROOT=${XDG_STATE_HOME:-$HOME/.local/state}/kc9k-autopilot
HUB=$ROOT/hub.git           # shared by the workers; $BRANCH lives here
STATE=$ROOT/$WORKER
REPO=$STATE/repo
LOGS=$STATE/logs
RESULT=$STATE/result.txt    # written by the session
NOTES=$ROOT/notes.md        # what failed, read by the next sessions
SKIP=$ROOT/skip.txt         # item IDs given up on, one per line
CLAIMS=$ROOT/claims.txt     # ID <tab> worker <tab> pid
export PATH="/usr/local/go/bin:$PATH"

die() { echo "autopilot[$WORKER]: $*" >&2; exit 1; }
say() { echo "[$(date '+%F %T')] $WORKER: $*" | tee -a "$ROOT/autopilot.log"; }
locked() { local l=$1; shift; ( flock 9; "$@" ) 9>"$ROOT/$l.lock"; } # one worker at a time

[ "$(id -u)" -ne 0 ] || die "never as root"
if [ -z "${AUTOPILOT_ALLOW_ADMIN:-}" ] && id -nG | tr ' ' '\n' | grep -qxE 'sudo|wheel|admin'; then
	die "$(id -un) is an administrator: tests and the agent could run kc9k-bin as root
on this computer. Use a separate user (see the top of this file) or set
AUTOPILOT_ALLOW_ADMIN=1 inside a throwaway VM."
fi
[[ $WORKER =~ ^[A-Za-z0-9_-]+$ ]] || die "AUTOPILOT_WORKER: letters, digits, - and _ only"
command -v claude >/dev/null || die "claude not found"
command -v go >/dev/null || die "go not found (expected in /usr/local/go/bin)"
command -v flock >/dev/null || die "flock not found (util-linux)"

mkdir -p "$LOGS"; touch "$NOTES" "$SKIP" "$CLAIMS"
rm -f "$ROOT/STOP"

init_hub() {
	[ -d "$HUB" ] && return
	git clone -q --bare "$ORIGIN" "$HUB.tmp" || return 1
	git -C "$HUB.tmp" rev-parse -q --verify "refs/heads/$BRANCH" >/dev/null \
		|| git -C "$HUB.tmp" branch -q "$BRANCH" "$FROM" || return 1
	mv "$HUB.tmp" "$HUB"
}
locked hub init_hub || die "could not set up $HUB from $ORIGIN"
if [ ! -d "$REPO/.git" ]; then
	git clone -q -b "$BRANCH" "$HUB" "$REPO" || die "clone failed"
	git -C "$REPO" remote add github "$ORIGIN"
fi
cd "$REPO" || exit 1

# Index rows: ID <tab> item <tab> notes <tab> needs
index_rows() {
	awk -F'|' '/^## Index/{f=1;next} f&&/^## /{exit}
		f&&/^\| *`?[A-Z]?[0-9][0-9.]*`? *\|/{
			for(i=2;i<=6;i++){gsub(/^ +| +$|`/,"",$i)}
			print $2"\t"$3"\t"$5"\t"$6}' ROADMAP.md
}

# Live claims of other workers; dead workers' claims are dropped.
others() {
	local id w pid
	while IFS=$'\t' read -r id w pid; do
		[ "$w" != "$WORKER" ] && kill -0 "$pid" 2>/dev/null && printf '%s\t%s\n' "$id" "$w"
	done <"$CLAIMS"
}

release() { locked claims sed -i "/\t$WORKER\t/d" "$CLAIMS"; }

claim_next() { # prints the claimed row, or "WAIT" / "NONE"
	local live id title notes needs held=0
	live=$(others)
	# Keep only live claims of others; ours is written below.
	awk -F'\t' -v me="$WORKER" '$2!=me' "$CLAIMS" | while IFS=$'\t' read -r id w pid; do
		kill -0 "$pid" 2>/dev/null && printf '%s\t%s\t%s\n' "$id" "$w" "$pid"
	done >"$CLAIMS.new"; mv "$CLAIMS.new" "$CLAIMS"
	while IFS=$'\t' read -r id title notes needs; do
		grep -qE "(^|[ ,])($NEEDS)([ ,]|$)" <<<"$needs" || continue
		grep -qxF -- "$id" "$SKIP" && continue
		if cut -f1 <<<"$live" | grep -qxF -- "$id"; then held=1; continue; fi
		printf '%s\t%s\t%s\n' "$id" "$WORKER" "$$" >>"$CLAIMS"
		printf '%s\t%s\t%s\t%s\n' "$id" "$title" "$notes" "$needs"
		return
	done < <(index_rows)
	[ "$held" = 1 ] && echo WAIT || echo NONE
}

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
		rm -f kc9k
	fi
}

claude_args() {
	args=(-p --output-format stream-json --verbose --max-turns "$1"
		--permission-mode "$PERM"
		--disallowedTools "Bash(sudo:*)" "Bash(git push:*)" "Bash(kc9k-bin:*)" "Bash(./install.sh:*)" "Bash(make install:*)")
	[ -n "$MODEL" ] && args+=(--model "$MODEL")
	[ -n "$BUDGET" ] && args+=(--max-budget-usd "$BUDGET")
}

prompt() { # $1 = ID, $2 = item, $3 = notes, $4 = other workers' rows
	cat <<EOF
You are one round of an unattended loop working on this repository. Nobody
is watching and nobody will answer questions: decide, or stop.

1. Read CLAUDE.md and follow it exactly: ROADMAP.md's Index, then only
   your item; SPEC.md's Index, then only what your item cites; the notes
   for the files you touch.
2. Your item is \`$1\`: $2 (notes: $3). Find it with
   grep -n '\`$1\`' ROADMAP.md. Work on this item only.
   If it cannot be finished and verified in this container (it needs the
   owner's laptop, real use, a real browser, root, the installed daemon,
   a VM, or a decision of the owner), change no code: say so under the
   item in ROADMAP.md, commit that, and report STUCK.
   If the item is large, do one complete, merged-quality step of it and
   say in ROADMAP.md what is left.
3. Other sessions work on this repository at the same time, and your
   commits are rebased onto theirs. They own:
$(if [ -n "$4" ]; then awk -F'\t' '{printf "   - `%s` %s (notes: %s)\n", $1, $2, $3}' <<<"$4"; else echo "   (none right now)"; fi)
   Do not work on their items. Keep out of the code areas their notes
   name unless your item needs a change there, and then keep it small. In
   ROADMAP.md change only your item (its Index row, its section) and §5.
4. Where SPEC.md is silent or seems wrong, do not guess and do not edit
   SPEC.md: add a short entry to ROADMAP.md §5 (Proposed, not agreed)
   describing the question, commit it, and report STUCK.
5. Build it with tests. You may use subagents (Agent tool) for parallel
   exploration, a plan, and a review of your diff before committing.
6. Verify as a normal user: CGO_ENABLED=0 go build ./..., go vet ./...,
   go test ./... and the ci/gui_*_smoke.py touching your change under
   xvfb-run -a if present. Never use sudo, install.sh, make install,
   kc9k/kc9k-bin outside tests, or the browser tests in ci/*_test.py.
7. Update ROADMAP.md in the same commit as the code (status, "on the
   laptop: Unchecked" where real use is still needed, "Last checked").
   Commit on the current branch with a message like the existing ones.
   Leave no uncommitted files. Do not push, do not switch branches.
8. Last, write exactly these lines to $RESULT:
   STATUS: DONE | PARTIAL | STUCK
   SUMMARY: <one line>

Notes from earlier rounds (failures the script found after them):
$(tail -40 "$NOTES")
EOF
}

resolve_prompt() {
	cat <<EOF
Unattended, nobody will answer. This branch holds one finished ROADMAP.md
item; other sessions' work landed on origin/$BRANCH meanwhile. Run
git rebase origin/$BRANCH and resolve every conflict keeping both sides'
intent (in ROADMAP.md: both sides' rows and sections), then
git rebase --continue until it is done. Build and test as CLAUDE.md says.
Change nothing else. Do not push.
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

# Under the land lock: onto the hub's $BRANCH, gate, fast-forward the hub.
land() { # $1 = attempt branch; prints why it fails, returns 1
	local attempt=$1 tip rlog
	local gh
	if [ "$PUSH" = 1 ] && git fetch -q github "$BRANCH" 2>/dev/null; then
		# The owner may have pushed to $BRANCH on GitHub: follow it.
		gh=$(git rev-parse FETCH_HEAD)
		git fetch -q origin "$BRANCH"
		git merge-base --is-ancestor "origin/$BRANCH" "$gh" \
			&& git push -q origin "$gh:refs/heads/$BRANCH"
	fi
	git fetch -q origin "$BRANCH" || { echo "fetch from hub failed"; return 1; }
	tip=$(git rev-parse "origin/$BRANCH")
	if ! git rebase -q "origin/$BRANCH" >/dev/null 2>&1; then
		git rebase --abort
		say "rebase conflict; a session resolves it"
		rlog="$LOGS/$(basename "$attempt")-rebase.jsonl"
		claude_args 80
		resolve_prompt | timeout 1h claude "${args[@]}" >"$rlog" 2>&1
		if [ -d .git/rebase-merge ] || [ -d .git/rebase-apply ]; then
			git rebase --abort; echo "rebase conflict left unresolved (log $rlog)"; return 1
		fi
		git merge-base --is-ancestor "$tip" HEAD || { echo "rebase not done (log $rlog)"; return 1; }
	fi
	gate "$tip" || return 1
	git push -q origin "HEAD:refs/heads/$BRANCH" || { echo "push to hub failed"; return 1; }
	if [ "$PUSH" = 1 ]; then git push -q github "HEAD:refs/heads/$BRANCH" || say "push to GitHub failed"; fi
}

trap release EXIT
fails=0
round=0
while [ "$ROUNDS" -eq 0 ] || [ "$round" -lt "$ROUNDS" ]; do
	[ -e "$ROOT/STOP" ] && { say "STOP file found"; break; }
	git fetch -q origin "$BRANCH" || { say "fetch from hub failed"; sleep 60; continue; }
	git checkout -q -f -B "$BRANCH" "origin/$BRANCH" && git clean -qfd
	row=$(locked claims claim_next)
	if [ "$row" = WAIT ]; then
		say "only other workers' items left; waiting $IDLE_WAIT"; sleep "$IDLE_WAIT"; continue
	elif [ "$row" = NONE ]; then
		say "nothing left in scope (Needs: $NEEDS); done"; break
	fi
	IFS=$'\t' read -r id title notes _ <<<"$row"
	round=$((round + 1))
	base=$(git rev-parse HEAD)
	attempt="autopilot-try/$WORKER-$(date +%Y%m%d-%H%M%S)"
	git checkout -q -b "$attempt"
	rm -f "$RESULT"
	log="$LOGS/$(basename "$attempt").jsonl"
	say "round $round: \`$id\` $title, $attempt from $(git log -1 --format=%h "$base"), log $log"

	claude_args "$TURNS"
	prompt "$id" "$title" "$notes" "$(locked claims others | while IFS=$'\t' read -r oid _; do
		index_rows | awk -F'\t' -v i="$oid" '$1==i'; done)" \
		| timeout "$TIMEOUT" claude "${args[@]}" >"$log" 2>&1
	code=$?

	status=$(sed -n 's/^STATUS: *//p' "$RESULT" 2>/dev/null | head -1)
	summary=$(sed -n 's/^SUMMARY: *//p' "$RESULT" 2>/dev/null | head -1)
	say "session exit $code, ${status:-no result}: \`$id\` — ${summary:-}"

	if [ -z "$status" ] && limit_hit "$log"; then
		git checkout -q -f "$BRANCH"
		if [ "$(git rev-list --count "$base..$attempt")" -eq 0 ]; then git branch -qD "$attempt"
		else say "kept unfinished work as $attempt"; fi
		release; round=$((round - 1))
		wait_for_limit "$log"
		continue
	fi

	if why=$(locked land land "$attempt" 2>&1); then
		say "PASSED: $BRANCH now $(git log -1 --format='%h %s')"
		git checkout -q -f "$BRANCH"; git branch -qD "$attempt"
		fails=0
	else
		fails=$((fails + 1))
		say "FAILED ($fails in a row), kept as $attempt: $(head -1 <<<"$why")"
		{ echo "## $(date '+%F %T') $attempt — [$id] $title"; echo '```'; tail -25 <<<"$why"; echo '```'; } >>"$NOTES"
		if [ "$(grep -cF -- "— [$id] " "$NOTES")" -ge "$TRIES" ]; then
			grep -qxF -- "$id" "$SKIP" || { echo "$id" >>"$SKIP"; say "giving up on: \`$id\`"; }
		fi
		git checkout -q -f "$BRANCH"
	fi
	if [ "$status" = STUCK ]; then
		grep -qxF -- "$id" "$SKIP" || { echo "$id" >>"$SKIP"; say "STUCK, skipped from now on: \`$id\`"; }
	fi
	release
	[ "$fails" -ge "$MAX_FAILS" ] && { say "$MAX_FAILS failed rounds in a row; stopping"; break; }
done
say "finished"
