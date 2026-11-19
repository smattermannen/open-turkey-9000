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
# Box mode (AUTOPILOT_BOX=1), on the test machine only (one that may be
# wiped): sessions may use sudo, install kc9k and run the daemon, real
# browsers and the X11 desktop, and also take Index items whose Needs is
# "box". The gate adds the roottest tests (run as root). Needs sudo without
# a password for this user (/etc/sudoers.d/kc9k-autopilot:
# "<user> ALL=(ALL) NOPASSWD: ALL"). Box workers share the machine: only
# one holds an item that needs it (Needs without "agent") at a time; "vm" items
# (network cuts, logout, shutdown, reboots) are left alone unless a VM is set.
#
# VM (AUTOPILOT_VM=<libvirt domain>, with box mode): also takes "vm" items.
# Before such a round the script reverts the VM to its snapshot
# (AUTOPILOT_VM_SNAPSHOT, default clean) and starts it; the session reaches
# it with ssh (AUTOPILOT_VM_SSH, default the domain name) and virsh
# (AUTOPILOT_VM_URI, default qemu:///system); after the round it is reverted
# again. One vm item at a time on the machine.
#
# The resolver: when ROADMAP.md §5, SPEC.md §7 or the skipped items hold
# questions it has not had, a worker runs a resolver session instead of an
# item. Only the resolver may change SPEC.md, only on $RESOLVER_MODEL
# (every turn checked in its log, no subagents), and every change carries
# the tag "(By Opus)" (checked per hunk); the rules for the file above
# SPEC.md's Index and the owner's §6 decisions are never changed.
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
MODEL=${AUTOPILOT_MODEL:-}                    # set: every item session on this model
EFFORT=${AUTOPILOT_EFFORT:-}                  # set: every item session at this effort
# Otherwise a triage session sizes each item and the tier picks both
# ("model effort"; effort ultracode = agent teams). A retry is one tier up.
TIER1=${AUTOPILOT_TIER1:-sonnet high}         # small, low risk
TIER2=${AUTOPILOT_TIER2:-opus xhigh}          # medium, or high risk
TIER3=${AUTOPILOT_TIER3:-opus xhigh}          # large
TRIAGE_MODEL=${AUTOPILOT_TRIAGE_MODEL:-haiku}
RESOLVER_EFFORT=${AUTOPILOT_RESOLVER_EFFORT:-max} # one careful chain, no agents
PERM=${AUTOPILOT_PERMISSION_MODE:-bypassPermissions}
PUSH=${AUTOPILOT_PUSH:-0}
RESOLVE=${AUTOPILOT_RESOLVE:-1}               # 0 = no resolver rounds
RESOLVER_MODEL=claude-opus-5-5                # the owner's rule: no other model decides
TAG='(By Opus)'
GUI_TESTS=${AUTOPILOT_GUI_TESTS:-auto}        # auto: when xvfb-run exists
BOX=${AUTOPILOT_BOX:-0}                       # 1 = sudo, root tests, box items
VM=${AUTOPILOT_VM:-}                          # libvirt domain for "vm" items
VM_SSH=${AUTOPILOT_VM_SSH:-$VM}
VM_SNAPSHOT=${AUTOPILOT_VM_SNAPSHOT:-clean}
VM_URI=${AUTOPILOT_VM_URI:-qemu:///system}
[ "$BOX" = 1 ] && NEEDS=${AUTOPILOT_NEEDS:-agent|box}
[ -n "$VM" ] && NEEDS=${AUTOPILOT_NEEDS:-agent|box|vm}

ROOT=${XDG_STATE_HOME:-$HOME/.local/state}/kc9k-autopilot
HUB=$ROOT/hub.git           # shared by the workers; $BRANCH lives here
STATE=$ROOT/$WORKER
REPO=$STATE/repo
LOGS=$STATE/logs
RESULT=$STATE/result.txt    # written by the session
NOTES=$ROOT/notes.md        # what failed, read by the next sessions
SKIP=$ROOT/skip.txt         # item IDs given up on, one per line
CLAIMS=$ROOT/claims.txt     # ID <tab> worker <tab> pid
TRIAGE=$ROOT/triage.txt     # ID <tab> hash of the item's text <tab> size <tab> risk
SEEN=$ROOT/resolver-seen.txt # the questions the resolver last had (hash)
export PATH="/usr/local/go/bin:$PATH"

die() { echo "autopilot[$WORKER]: $*" >&2; exit 1; }
say() { echo "[$(date '+%F %T')] $WORKER: $*" | tee -a "$ROOT/autopilot.log"; }
locked() { local l=$1; shift; ( flock 9; "$@" ) 9>"$ROOT/$l.lock"; } # one worker at a time

[ "$(id -u)" -ne 0 ] || die "never as root (tests refuse as root; box mode uses sudo)"
if [ "$BOX" = 1 ]; then
	sudo -n true 2>/dev/null || die "box mode needs sudo without a password for $(id -un):
  echo '$(id -un) ALL=(ALL) NOPASSWD: ALL' | sudo tee /etc/sudoers.d/kc9k-autopilot
  sudo chmod 440 /etc/sudoers.d/kc9k-autopilot && sudo visudo -c"
	if [ -n "$VM" ]; then
		command -v virsh >/dev/null || die "virsh not found (package libvirt-clients)"
		virsh -c "$VM_URI" snapshot-info "$VM" --snapshotname "$VM_SNAPSHOT" >/dev/null \
			|| die "VM $VM has no snapshot $VM_SNAPSHOT on $VM_URI (are you in group libvirt?)"
	fi
elif [ -n "$VM" ]; then
	die "AUTOPILOT_VM needs box mode (AUTOPILOT_BOX=1)"
elif [ -z "${AUTOPILOT_ALLOW_ADMIN:-}" ] && id -nG | tr ' ' '\n' | grep -qxE 'sudo|wheel|admin'; then
	die "$(id -un) is an administrator: tests and the agent could run kc9k-bin as root
on this computer. Use a separate user (see the top of this file) or set
AUTOPILOT_ALLOW_ADMIN=1 inside a throwaway VM."
fi
[[ $WORKER =~ ^[A-Za-z0-9_-]+$ ]] || die "AUTOPILOT_WORKER: letters, digits, - and _ only"
command -v claude >/dev/null || die "claude not found"
command -v go >/dev/null || die "go not found (expected in /usr/local/go/bin)"
command -v flock >/dev/null || die "flock not found (util-linux)"
# A private D-Bus session bus per worker: the GUI smoke tests register fixed
# GTK application IDs, and two workers' tests on one bus would meet.
if [ -z "${AUTOPILOT_OWN_BUS:-}" ]; then
	command -v dbus-run-session >/dev/null || die "dbus-run-session not found (package dbus)"
	AUTOPILOT_OWN_BUS=1 exec dbus-run-session -- "$0" "$@"
fi

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

box_only() { ! grep -qE '(^|[ ,])agent([ ,]|$)' <<<"$1"; } # $1 = Needs
vm_item() { [ -n "$VM" ] && grep -qE '(^|[ ,])vm([ ,]|$)' <<<"$1"; } # $1 = Needs

# Back to the snapshot; $1 = start: running, and reachable over ssh.
vm_reset() {
	if [ "${1:-}" = start ]; then
		virsh -c "$VM_URI" snapshot-revert "$VM" "$VM_SNAPSHOT" --running --force >/dev/null || return 1
		local i; for i in $(seq 36); do
			ssh -o BatchMode=yes -o ConnectTimeout=5 "$VM_SSH" true 2>/dev/null && return 0
			sleep 5
		done
		return 1
	fi
	virsh -c "$VM_URI" snapshot-revert "$VM" "$VM_SNAPSHOT" --force >/dev/null
}

claim_next() { # prints the claimed row, or "WAIT" / "NONE"
	local live id title notes needs held=0 box_busy=0 vm_busy=0
	live=$(others)
	# Box items use the machine itself (daemon, browsers, desktop): one at a time.
	while IFS=$'\t' read -r id _ _ needs; do
		cut -f1 <<<"$live" | grep -qxF -- "$id" || continue
		if vm_item "$needs"; then vm_busy=1; elif box_only "$needs"; then box_busy=1; fi
	done < <(index_rows)
	# Keep only live claims of others; ours is written below.
	awk -F'\t' -v me="$WORKER" '$2!=me' "$CLAIMS" | while IFS=$'\t' read -r id w pid; do
		kill -0 "$pid" 2>/dev/null && printf '%s\t%s\t%s\n' "$id" "$w" "$pid"
	done >"$CLAIMS.new"; mv "$CLAIMS.new" "$CLAIMS"
	while IFS=$'\t' read -r id title notes needs; do
		grep -qE "(^|[ ,])($NEEDS)([ ,]|$)" <<<"$needs" || continue
		grep -qxF -- "$id" "$SKIP" && continue
		if cut -f1 <<<"$live" | grep -qxF -- "$id"; then held=1; continue; fi
		if [ "$box_busy" = 1 ] && box_only "$needs" && ! vm_item "$needs"; then held=1; continue; fi
		if [ "$vm_busy" = 1 ] && vm_item "$needs"; then held=1; continue; fi
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
	grep -qx 'ROADMAP.md' <<<"$changed" || { echo "ROADMAP.md not updated"; return 1; }
	if [ "$MODE" = resolver ]; then
		local other l
		other=$(grep -vxE 'SPEC.md|ROADMAP.md' <<<"$changed")
		[ -z "$other" ] || { echo "the resolver changed more than SPEC.md and ROADMAP.md:"; echo "$other"; return 1; }
		spec_check "$base" || return 1
		for l in "${MODEL_LOGS[@]}"; do model_check "$l" || return 1; done
		return 0
	fi
	if grep -qx 'SPEC.md' <<<"$changed"; then echo "SPEC.md changed (only the resolver may)"; return 1; fi
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
	[ "$BOX" = 1 ] && { root_tests || return 1; }
	return 0
}

# Box mode: the roottest tests, as root, in each package that has them.
root_tests() {
	local f pkgs pkg names
	pkgs=$(grep -l '^//go:build roottest' $(git ls-files '*_test.go') | xargs -rn1 dirname | sort -u)
	for pkg in $pkgs; do
		names=$(grep -ohE '^func (Test[A-Za-z0-9_]+)' $(grep -l '^//go:build roottest' "$pkg"/*_test.go) \
			| sed 's/^func //' | paste -sd'|')
		[ -n "$names" ] || continue
		timeout 20m sudo -n env PATH="$PATH" HOME=/root go test -count=1 -tags roottest \
			-run "^($names)\$" "./$pkg" >"$STATE/root.out" 2>&1 \
			|| { echo "root tests in $pkg failed:"; tail -40 "$STATE/root.out"; return 1; }
	done
}

# Every hunk of SPEC.md's diff carries the tag; the rules for the file
# (above its Index) and the owner's §6 decisions are untouched.
spec_check() { # $1 = base
	local index_at
	index_at=$(git show "$1":SPEC.md | grep -n '^## Index' | cut -d: -f1)
	git diff -U0 "$1"..HEAD -- SPEC.md | awk -v tag="$TAG" -v index_at="$index_at" '
		function end_hunk() { if (inhunk && !tagged) { print "SPEC.md change without " tag " near line " start; bad = 1 } }
		/^@@/ { end_hunk(); inhunk = 1; tagged = 0
			split($2, o, ","); start = substr(o[1], 2) + 0
			if (start < index_at) { print "SPEC.md: the rules for the file (above its Index) belong to the owner"; bad = 1 }
			next }
		/^(\+\+\+|---) / { next }
		/^\+/ && index($0, tag) { tagged = 1 }
		/^-- 20[0-9][0-9]-/ && !index($0, tag) { print "SPEC.md: an owner decision was removed or rewritten: " substr($0, 2, 70); bad = 1 }
		END { end_hunk(); exit bad }'
}

# Every turn of the session was served by $RESOLVER_MODEL.
model_check() { # $1 = session log (stream-json)
	local models
	models=$( { grep -oE '"model": *"[^"]*"' "$1" | sed 's/.*"\([^"]*\)"$/\1/'
		python3 -I -c 'import json,sys
for l in open(sys.argv[1], errors="replace"):
    try: d = json.loads(l)
    except ValueError: continue
    if d.get("type") == "result": print("\n".join(d.get("modelUsage") or {}))' "$1"
	} | grep -vx '<synthetic>' | grep . | sort -u)
	[ "$models" = "$RESOLVER_MODEL" ] && return 0
	echo "resolver log $1: served by '$(tr '\n' ' ' <<<"$models")', not $RESOLVER_MODEL only"; return 1
}

claude_args() { # $1 = turns, $2 = model, $3 = effort (empty: the CLI's default)
	args=(-p --output-format stream-json --verbose --max-turns "$1"
		--permission-mode "$PERM" --disallowedTools "Bash(git push:*)") # the script lands, after the gate
	[ "$BOX" = 1 ] || args+=("Bash(sudo:*)" "Bash(kc9k-bin:*)" "Bash(./install.sh:*)" "Bash(make install:*)")
	[ "$MODE" = resolver ] && args+=(Agent Task Workflow) # the decision is the resolver model's alone
	[ -n "${2:-}" ] && args+=(--model "$2")
	[ -n "${3:-}" ] && args+=(--effort "$3")
	[ -n "$BUDGET" ] && args+=(--max-budget-usd "$BUDGET")
}

# The item's own text: its Index row and its section in ROADMAP.md.
item_text() { # $1 = ID
	index_rows | awk -F'\t' -v i="$1" '$1==i'
	awk -v id="\`$1\`" 'f && (/^- \[/ || /^#/) {exit} index($0, id) && /^- \[/ {f=1} f' ROADMAP.md
}

# Prints "size risk" (S|M|L, low|high), from a cheap read-only session,
# cached until the item's text changes. Unsure → "M high": the safe side.
triage() { # $1 = ID
	local text key line size risk out
	text=$(item_text "$1"); key=$(sha256sum <<<"$text" | cut -c1-16)
	line=$(awk -F'\t' -v i="$1" -v k="$key" '$1==i && $2==k {print $3" "$4}' "$TRIAGE" 2>/dev/null | tail -1)
	[ -n "$line" ] && { echo "$line"; return; }
	out=$(timeout 15m claude -p --model "$TRIAGE_MODEL" --effort low --max-turns 12 \
		--allowedTools Read Grep Glob --disallowedTools Bash Edit Write Agent Task Workflow <<EOF 2>/dev/null
Classify one work item of this repository for an unattended coding loop.
Read only what you need (CLAUDE.md's note table, the SPEC.md rules the
item cites, the files its notes name). Do not change anything.

$text

SIZE: S (one or two files, under a few hours), M (several files, about a
day), or L (many files or a new subsystem, days).
RISK: high if a subtle mistake would weaken a lock, a block, enforcement,
the weakening audit, passwords or security, or lose the user's data;
otherwise low.
Answer with exactly two lines and nothing else:
SIZE: S|M|L
RISK: low|high
EOF
)
	size=$(grep -oE '^SIZE: *[SML]\b' <<<"$out" | grep -oE '[SML]$' | head -1)
	risk=$(grep -oE '^RISK: *(low|high)' <<<"$out" | grep -oE '(low|high)$' | head -1)
	if [ -z "$size" ] || [ -z "$risk" ]; then echo "M high"; return; fi
	printf '%s\t%s\t%s\t%s\n' "$1" "$key" "$size" "$risk" >>"$TRIAGE"
	echo "$size $risk"
}

# Sets lead_model, lead_effort, lead_why for the item.
pick_lead() { # $1 = ID
	local size risk tier tries t
	read -r size risk <<<"$(triage "$1")"
	tier=1
	{ [ "$size" = M ] || [ "$risk" = high ]; } && tier=2
	[ "$size" = L ] && tier=3
	tries=$(grep -cF -- "— [$1] " "$NOTES")
	[ "$tries" -gt 0 ] && tier=$((tier + tries))
	[ "$tier" -gt 3 ] && tier=3
	case $tier in 1) t=$TIER1 ;; 2) t=$TIER2 ;; *) t=$TIER3 ;; esac
	lead_model=${MODEL:-${t%% *}}
	lead_effort=${EFFORT:-${t#* }}
	lead_why="size $size, risk $risk, $tries earlier failed tries: tier $tier"
}

# Which models served the session (its result's modelUsage: the lead and
# every subagent), their output tokens, and what it cost.
usage_line() { # $1 = session log
	python3 -I - "$1" <<'EOF' 2>/dev/null || echo "models: ?"
import json, sys
res = None
for line in open(sys.argv[1], errors="replace"):
    try:
        d = json.loads(line)
    except ValueError:
        continue
    if d.get("type") == "result":
        res = d
if not res:
    print("models: ? (no result)"); sys.exit()
mu = res.get("modelUsage") or {}
parts = ["%s %dk out" % (m, (u.get("outputTokens") or 0) // 1000) for m, u in sorted(mu.items())]
print("models: %s; cost: $%.2f" % (", ".join(parts) or "?", res.get("total_cost_usd") or 0))
EOF
}

prompt() { # $1 = ID, $2 = item, $3 = notes, $4 = other workers' rows, $5 = lead, $6 = VM is set up
	cat <<EOF
You are one round of an unattended loop working on this repository. Nobody
is watching and nobody will answer questions: decide, or stop.

1. Read CLAUDE.md and follow it exactly: ROADMAP.md's Index, then only
   your item; SPEC.md's Index, then only what your item cites; the notes
   for the files you touch.
2. Your item is \`$1\`: $2 (notes: $3). Find it with
   grep -n '\`$1\`' ROADMAP.md. Work on this item only.
$(if [ "$BOX" = 1 ]; then cat <<'BOXEOF'
   You run on the test machine (box): you may use sudo, root, install.sh,
   the installed daemon, kc9k, real browsers and the X11 desktop. Nothing
   on it needs keeping. Never cut its network, log out, shut down, reboot,
   freeze it or turn on strict mode: that ends this session. If the item
   needs that (a VM) and no VM is named below, real use, or a decision of
   the owner, change no code: say so under the item in ROADMAP.md, commit
   that, report STUCK.
   Before you finish, remove the blocks, locks and schedules you added.
BOXEOF
fi)$(if [ -n "$VM" ] && [ -n "${6:-}" ]; then echo; cat <<VMEOF
   A VM is yours for this item: libvirt domain $VM, fresh from snapshot
   $VM_SNAPSHOT and running; ssh $VM_SSH (user with passwordless sudo,
   kc9k installed, Cinnamon autologin on X11, this repo at
   ~/open-turkey-9000 from GitHub, older than yours). Copy your tree there
   (rsync -a --delete --exclude .git ./ $VM_SSH:open-turkey-9000/), build
   and sudo ./install.sh there. Do network cuts, logout, shutdown, reboots,
   freezing and strict mode only in the VM; drive and check it from here
   (ssh, virsh -c $VM_URI: start, reboot, screenshot, snapshot-revert
   $VM $VM_SNAPSHOT --running to start over). Never change the snapshot.
   The script reverts the VM after this round.
VMEOF
fi)$(if [ "$BOX" != 1 ]; then cat <<'BOXEOF'
   If it cannot be finished and verified in this container (it needs the
   owner's laptop, real use, a real browser, root, the installed daemon,
   a VM, or a decision of the owner), change no code: say so under the
   item in ROADMAP.md, commit that, and report STUCK.
BOXEOF
fi)
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
   describing the question, commit it, and report STUCK. A resolver
   session decides it later.
5. Build it with tests. You run as $5. Use subagents (Agent
   tool) where they save time, and set their model and effort parameters
   to fit each job, as these instructions ask:
   - finding and reading code, listing call sites: model haiku, effort low;
   - mechanical edits or tests, each agent on its own files: model sonnet,
     effort medium;
   - a plan for a change beyond a few files: model opus, effort high;
   - always, before committing: a fresh review agent, model opus, effort
     high, given git diff, the SPEC.md rules your item cites and
     CLAUDE.md's shared rules, asked whether the tests prove those rules
     and what the change breaks or misses. Fix what it finds.
   Two agents never edit the same file. Decisions stay with you, and
   SPEC.md questions go to §5 (step 4). For a small item, do the work
   yourself: agents cost more than they save there.
6. Verify as a normal user: CGO_ENABLED=0 go build ./..., go vet ./...,
   go test ./... and the ci/gui_*_smoke.py touching your change under
   xvfb-run -a if present.$(if [ "$BOX" = 1 ]; then echo "
   Run go test as yourself, never with sudo (as root KC9K_DB is ignored);
   root tests: sudo go test -tags roottest -run <name> ./<package>.
   Check on the real desktop what your item says needs it."; else echo " Never use sudo, install.sh, make install,
   kc9k/kc9k-bin outside tests, or the browser tests in ci/*_test.py."; fi)
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

# Questions for the resolver: open §5 bullets, SPEC.md §7 unless "None.",
# and the skipped items.
open_questions() {
	awk '/^## 5\./{f=1;next} f&&/^## /{exit}
		f&&/^- /{if(b!="")print b"\n"; b=$0; next}
		f&&/^  /&&b!=""{b=b"\n"$0; next}
		f{if(b!="")print b"\n"; b=""}
		END{if(b!="")print b"\n"}' ROADMAP.md | awk 'BEGIN{RS=""} !/^- (DECIDED|FOR THE OWNER)/{print; print ""}'
	awk '/^## 7\./{f=1;next} f&&/^## /{exit} f&&NF&&$0!="None."' SPEC.md
	if [ -s "$SKIP" ]; then echo "Skipped items:"; cat "$SKIP"; fi
}

resolve_due() { # questions the resolver has not had
	[ "$RESOLVE" = 1 ] || return 1
	local q; q=$(open_questions)
	[ -n "$q" ] || return 1
	[ "$(sha256sum <<<"$q" | cut -c1-16)" != "$(cat "$SEEN" 2>/dev/null)" ]
}

claim_resolver() { # one resolver at a time
	others | cut -f1 | grep -qx RESOLVER && return 1
	printf 'RESOLVER\t%s\t%s\n' "$WORKER" "$$" >>"$CLAIMS"
}

resolver_prompt() {
	cat <<EOF
You are the resolver of an unattended loop working on this repository.
Nobody is watching and nobody will answer questions. The owner allowed you,
and only you, to decide open questions and write the decisions into
SPEC.md, as long as every change you make there carries the tag $TAG,
so the owner can tell their decisions from yours.

1. Read CLAUDE.md, SPEC.md's rules for the file, its Index, §2
   (principles) and §6 (decisions), and ROADMAP.md's Index and §5. Read
   further sections only where a question needs them.
2. The open questions (ROADMAP.md §5 entries not marked DECIDED or FOR
   THE OWNER, SPEC.md §7, and items the loop skipped):
$(open_questions | sed 's/^/   /')

   Failure notes of the loop (why items were skipped):
$(tail -60 "$NOTES" | sed 's/^/   /')
3. For each question, decide or hand it to the owner:
   - Decide when SPEC.md's principles and the owner's decisions point to
     an answer. Prefer the smallest decision that unblocks the work; do
     not invent features. Write it where it belongs in SPEC.md (§3–§5,
     glossary words exactly) and end every paragraph, bullet or table row
     you add or change with $TAG. Add a §6 entry:
     - $(date +%F) $TAG: <the decision, and why, in one or two sentences>
     In ROADMAP.md §5, start the entry with "DECIDED $(date +%F) $TAG:"
     and say where SPEC.md now has it. If it leads to work, add an Index
     row with the next ID of its group (Needs as the Index explains).
   - Hand it to the owner (start the §5 entry with "FOR THE OWNER:" and
     say what needs deciding) when it is a matter of taste, money,
     privacy beyond what §2 settles, real hardware or real use, or when
     deciding would change or reverse an owner decision (§6 entries
     without $TAG, and the rules for the file above SPEC.md's Index:
     never edit those).
   - A skipped item that failed for code reasons, not a question: leave it.
4. Change only SPEC.md and ROADMAP.md. Do not use subagents. Commit on
   the current branch with a message starting "Resolver $TAG:". Leave no
   uncommitted files. Do not push, do not switch branches.
5. Last, write exactly these lines to $RESULT:
   STATUS: DONE | NOTHING
   UNSKIP: <IDs of skipped items your decisions unblock, space-separated, or empty>
   SUMMARY: <one line>
EOF
}

limit_hit() { # $1 = session log; true if the session ended on a usage limit
	# (not the stream's rate_limit_event lines, which every session has)
	tail -5 "$1" | grep -v '"type": *"rate_limit_event"' \
		| grep -qiE 'usage limit|limit reached|hit your limit|rate.?limit|out of (extra )?usage'
}

# Why a session ended without a result: its result text or last lines.
session_error() { # $1 = session log
	python3 -I - "$1" <<'PYEOF' 2>/dev/null || tail -3 "$1"
import json, sys
last, res = [], None
for line in open(sys.argv[1], errors="replace"):
    try:
        d = json.loads(line)
    except ValueError:
        if line.strip(): last.append(line.strip())
        continue
    if d.get("type") == "result": res = d
if res and (res.get("is_error") or res.get("subtype") != "success"):
    print(res.get("subtype"), str(res.get("result") or res.get("errors") or "")[:400])
else:
    print(" | ".join(last[-3:])[:400] or "(empty log)")
PYEOF
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
		if [ "$MODE" = resolver ]; then claude_args 80 "$RESOLVER_MODEL" "$RESOLVER_EFFORT"; MODEL_LOGS+=("$rlog")
		else claude_args 80 "${TIER1%% *}" "${TIER1#* }"; fi
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

resolver_round() {
	local base attempt log code status unskip id why
	MODE=resolver
	base=$(git rev-parse HEAD)
	attempt="autopilot-resolve/$WORKER-$(date +%Y%m%d-%H%M%S)"
	git checkout -q -b "$attempt"; rm -f "$RESULT"
	log="$LOGS/$(basename "$attempt").jsonl"; MODEL_LOGS=("$log")
	say "resolver: $attempt from $(git log -1 --format=%h "$base"), log $log"
	claude_args 150 "$RESOLVER_MODEL" "$RESOLVER_EFFORT"
	resolver_prompt | timeout 2h claude "${args[@]}" >"$log" 2>&1
	code=$?
	status=$(sed -n 's/^STATUS: *//p' "$RESULT" 2>/dev/null | head -1)
	unskip=$(sed -n 's/^UNSKIP: *//p' "$RESULT" 2>/dev/null | head -1)
	say "resolver exit $code, ${status:-no result}: $(sed -n 's/^SUMMARY: *//p' "$RESULT" 2>/dev/null | head -1)"
	if [ -z "$status" ] && limit_hit "$log"; then
		git checkout -q -f "$BRANCH"; release
		wait_for_limit "$log"; return
	fi
	if [ "$(git rev-list --count "$base"..HEAD)" -eq 0 ]; then
		say "resolver: decided nothing"
	elif why=$(locked land land "$attempt" 2>&1); then
		say "RESOLVER PASSED: $BRANCH now $(git log -1 --format='%h %s')"
		for id in $unskip; do
			grep -vxF -- "$id" "$SKIP" >"$SKIP.new"; mv "$SKIP.new" "$SKIP"; say "unskipped \`$id\`"
		done
	else
		say "RESOLVER FAILED, kept as $attempt: $(head -1 <<<"$why")"
		{ echo "## $(date '+%F %T') $attempt — resolver"; echo '```'; tail -25 <<<"$why"; echo '```'; } >>"$NOTES"
	fi
	git checkout -q -f "$BRANCH"
	git fetch -q origin "$BRANCH" && git reset -q --hard "origin/$BRANCH"
	git rev-list --count "$BRANCH..$attempt" 2>/dev/null | grep -qx 0 && git branch -qD "$attempt"
	# The questions as they now stand: it runs again only when they change.
	sha256sum <<<"$(open_questions)" | cut -c1-16 >"$SEEN"
}

trap release EXIT
MODE=item
MODEL_LOGS=()
fails=0
round=0
while [ "$ROUNDS" -eq 0 ] || [ "$round" -lt "$ROUNDS" ]; do
	[ -e "$ROOT/STOP" ] && { say "STOP file found"; break; }
	git fetch -q origin "$BRANCH" || { say "fetch from hub failed"; sleep 60; continue; }
	git checkout -q -f -B "$BRANCH" "origin/$BRANCH" && git clean -qfd
	if resolve_due && locked claims claim_resolver; then
		resolver_round; release; MODE=item; continue
	fi
	row=$(locked claims claim_next)
	if [ "$row" = WAIT ]; then
		say "only other workers' items left; waiting $IDLE_WAIT"; sleep "$IDLE_WAIT"; continue
	elif [ "$row" = NONE ]; then
		say "nothing left in scope (Needs: $NEEDS); done"; break
	fi
	IFS=$'\t' read -r id title notes needs <<<"$row"
	use_vm=
	if vm_item "$needs"; then
		say "reverting $VM to $VM_SNAPSHOT for \`$id\`"
		if vm_reset start; then use_vm=1
		else
			say "VM $VM did not come up on ssh $VM_SSH; waiting $IDLE_WAIT"
			vm_reset; release; sleep "$IDLE_WAIT"; continue
		fi
	fi
	pick_lead "$id"
	round=$((round + 1))
	base=$(git rev-parse HEAD)
	attempt="autopilot-try/$WORKER-$(date +%Y%m%d-%H%M%S)"
	git checkout -q -b "$attempt"
	rm -f "$RESULT"
	log="$LOGS/$(basename "$attempt").jsonl"
	say "round $round: \`$id\` $title, $attempt from $(git log -1 --format=%h "$base"), log $log"
	say "lead: $lead_model at $lead_effort ($lead_why)"

	claude_args "$TURNS" "$lead_model" "$lead_effort"
	prompt "$id" "$title" "$notes" "$(locked claims others | while IFS=$'\t' read -r oid _; do
		index_rows | awk -F'\t' -v i="$oid" '$1==i'; done)" "$lead_model at effort $lead_effort ($lead_why)" "$use_vm" \
		| timeout "$TIMEOUT" claude "${args[@]}" >"$log" 2>&1
	code=$?
	[ -n "$use_vm" ] && { vm_reset || say "could not revert $VM"; }
	say "$(usage_line "$log")"

	status=$(sed -n 's/^STATUS: *//p' "$RESULT" 2>/dev/null | head -1)
	summary=$(sed -n 's/^SUMMARY: *//p' "$RESULT" 2>/dev/null | head -1)
	say "session exit $code, ${status:-no result}: \`$id\` — ${summary:-}"
	[ -z "$status" ] && say "session output: $(session_error "$log")"

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
