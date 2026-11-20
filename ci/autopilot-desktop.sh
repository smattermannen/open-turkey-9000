#!/bin/bash
# One command for the owner's desktop: the autopilot in box mode, one worker
# per Claude account, each in its own visible terminal window.
#   ci/autopilot-desktop.sh            start (or reopen the windows if running)
#   ci/autopilot-desktop.sh status     what each worker holds, the log's end
#   ci/autopilot-desktop.sh watch [W]  follow worker W (all: every worker)
#   ci/autopilot-desktop.sh stop       stop after the current items (STOP file)
#   ci/autopilot-desktop.sh stop --now kill every session now
#
# Start: checks passwordless sudo (offers to set it up), finds the logged-in
# Claude accounts (~/.claude and ~/.claude-*; with fewer than two it makes
# ~/.claude-a and ~/.claude-b and opens `claude` in a window to /login),
# uses the VM kc9k-vm when its snapshot `clean` exists (notes/testing.md),
# then starts ci/autopilot-team.sh detached (setsid: closing a window or the
# terminal stops nothing) and opens one watch window per worker. The
# autopilot's defaults stay: every landing is pushed and merged to main.
# AUTOPILOT_* settings pass through; AUTOPILOT_ACCOUNTS="dir dir" skips the
# search; AUTOPILOT_VM= (empty) leaves the VM out.
set -u
here=$(cd "$(dirname "$0")" && pwd)
ROOT=${XDG_STATE_HOME:-$HOME/.local/state}/kc9k-autopilot
SID=$ROOT/desktop.sid
mkdir -p "$ROOT"
die() { echo "autopilot-desktop: $*" >&2; exit 1; }

running() { [ -s "$SID" ] && pgrep -s "$(cat "$SID")" >/dev/null 2>&1; }

# A terminal window running "$@" (the title first).
window() {
	local title=$1; shift
	[ -n "${DISPLAY:-}${WAYLAND_DISPLAY:-}" ] || { echo "no desktop: run $*"; return 1; }
	if command -v gnome-terminal >/dev/null; then gnome-terminal --title="$title" -- "$@"
	elif command -v xfce4-terminal >/dev/null; then xfce4-terminal --title="$title" -x "$@"
	elif command -v konsole >/dev/null; then konsole -p tabtitle="$title" -e "$@"
	elif command -v x-terminal-emulator >/dev/null; then x-terminal-emulator -T "$title" -e "$@"
	elif command -v xterm >/dev/null; then xterm -T "$title" -e "$@"
	else echo "no terminal found: run $*"; return 1; fi >/dev/null 2>&1 &
}

logged_in() { [ -s "$1/.credentials.json" ]; }

accounts() {
	if [ -n "${AUTOPILOT_ACCOUNTS:-}" ]; then echo $AUTOPILOT_ACCOUNTS; return; fi
	local d found=()
	for d in "$HOME/.claude" "$HOME"/.claude-*; do
		[ -d "$d" ] && logged_in "$d" && found+=("$d")
	done
	if [ ${#found[@]} -lt 2 ]; then
		for d in "$HOME/.claude-a" "$HOME/.claude-b"; do
			[ ${#found[@]} -ge 2 ] && break
			[[ " ${found[*]} " == *" $d "* ]] && continue
			mkdir -p "$d"
			echo "account $d is not logged in: type /login in the window that opens" >&2
			window "kc9k login $(basename "$d")" env CLAUDE_CONFIG_DIR="$d" claude >&2 \
				|| { echo "  then: CLAUDE_CONFIG_DIR=$d claude, /login" >&2; }
			until logged_in "$d"; do sleep 3; done
			echo "account $d: logged in" >&2
			found+=("$d")
		done
	fi
	echo "${found[@]}"
}

# The viewer: the worker's own output and its newest session, readable.
watch() {
	local w=${1:-all}
	if [ "$w" = all ]; then
		for d in "$ROOT"/*/; do [ -f "$d/out.txt" ] && window "kc9k $(basename "$d")" "$0" watch "$(basename "$d")"; done
		return
	fi
	[ -d "$ROOT/$w" ] || die "no worker $w in $ROOT"
	printf '\033]0;kc9k %s\007' "$w"
	exec python3 -I - "$ROOT/$w" <<'PY'
import glob, json, os, sys, time
d = sys.argv[1]
out = open(os.path.join(d, "out.txt"), errors="replace"); out.seek(0, 2)
cur = None; f = None
C = lambda c, s: f"\033[{c}m{s}\033[0m"
def show(line):
    try: e = json.loads(line)
    except ValueError: return
    t = e.get("type")
    if t == "system" and e.get("subtype") == "init":
        print(C("2", f"-- session, model {e.get('model')}"))
    elif t in ("assistant", "user"):
        for b in (e.get("message") or {}).get("content") or []:
            if not isinstance(b, dict): continue
            k = b.get("type")
            if k == "text" and t == "assistant": print(b["text"].strip())
            elif k == "tool_use":
                i = b.get("input") or {}
                arg = i.get("command") or i.get("file_path") or i.get("pattern") or i.get("description") or ""
                print(C("36", f"> {b.get('name')}: {str(arg).splitlines()[0][:160] if arg else ''}"))
            elif k == "tool_result" and b.get("is_error"):
                c = b.get("content"); c = c if isinstance(c, str) else json.dumps(c)
                print(C("31", "  error: " + c.strip().splitlines()[0][:160] if c.strip() else "  error"))
    elif t == "result":
        print(C("33", f"-- session ended: {e.get('subtype')}, {e.get('num_turns')} turns, ${e.get('total_cost_usd') or 0:.2f}"))
    sys.stdout.flush()
print(C("1", f"watching {os.path.basename(d)} (closing this window stops nothing)"))
while True:
    for l in out: print(C("1;32", l.rstrip()))
    logs = sorted(glob.glob(os.path.join(d, "logs", "*.jsonl")), key=os.path.getmtime)
    if logs and logs[-1] != cur and (cur is None or time.time() - os.path.getmtime(logs[-1]) < 600):
        cur = logs[-1]; f = open(cur, errors="replace")
        print(C("1;35", f"== {os.path.basename(cur)}"))
    if f:
        for l in f: show(l)
    sys.stdout.flush(); time.sleep(1)
PY
}

status() {
	if running; then echo "running (session $(cat "$SID"))"; else echo "not running"; fi
	[ -s "$ROOT/claims.txt" ] && { echo "claims (item, worker, pid):"; cat "$ROOT/claims.txt"; }
	[ -f "$ROOT/STOP" ] && echo "STOP file set: workers end after their round"
	echo "--- $ROOT/autopilot.log"; tail -15 "$ROOT/autopilot.log" 2>/dev/null
}

stop() {
	if [ "${1:-}" = --now ]; then
		running || { echo "not running"; return; }
		pkill -TERM -s "$(cat "$SID")"; sleep 5
		pkill -KILL -s "$(cat "$SID")" 2>/dev/null
		rm -f "$SID"; echo "stopped; claims of dead workers are dropped on the next start"
	else
		touch "$ROOT/STOP"
		echo "STOP set: each worker ends after its current round (stop --now kills them)"
	fi
}

start() {
	if running; then echo "already running: reopening the windows"; watch all; return; fi
	[ "$(id -u)" -ne 0 ] || die "run as your desktop user, not root (sessions get root through sudo)"
	if ! sudo -n true 2>/dev/null; then
		echo "the sessions need sudo without a password for $(id -un); setting it up (asks your password once):"
		echo "$(id -un) ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/kc9k-autopilot >/dev/null \
			&& sudo chmod 440 /etc/sudoers.d/kc9k-autopilot && sudo visudo -c >/dev/null \
			|| { sudo rm -f /etc/sudoers.d/kc9k-autopilot; die "could not set up sudo"; }
	fi
	export PATH="/usr/local/go/bin:$PATH" AUTOPILOT_BOX=1
	if [ -z "${AUTOPILOT_VM+x}" ]; then
		if command -v virsh >/dev/null && virsh -c "${AUTOPILOT_VM_URI:-qemu:///system}" \
			snapshot-info kc9k-vm --snapshotname "${AUTOPILOT_VM_SNAPSHOT:-clean}" >/dev/null 2>&1; then
			export AUTOPILOT_VM=kc9k-vm; echo "VM kc9k-vm: used for vm items"
		else
			echo "VM kc9k-vm or its snapshot not found: vm items are left for later"
		fi
	fi
	local accts; accts=$(accounts) || exit 1
	echo "accounts: $accts"
	rm -f "$ROOT/STOP"
	setsid nohup "$here/autopilot-team.sh" $accts >"$ROOT/team.out" 2>&1 </dev/null &
	sleep 2
	ps -o sid= -p $! | tr -d ' ' >"$SID"   # every worker and session shares it
	running || { cat "$ROOT/team.out"; die "the team did not start"; }
	echo "started; log $ROOT/autopilot.log"
	# The workers start 20 s apart; open each window when its output exists.
	for d in $accts; do
		local name; name=$(basename "$d" | sed 's/^\.//; s/[^A-Za-z0-9_-]/-/g')
		for _ in $(seq 60); do [ -f "$ROOT/$name/out.txt" ] && break; sleep 1; done
		window "kc9k $name" "$0" watch "$name"
	done
	echo "stop: $0 stop   (now: $0 stop --now)"
}

case "${1:-start}" in
	start) start ;;
	status) status ;;
	watch) watch "${2:-all}" ;;
	stop) stop "${2:-}" ;;
	*) die "usage: $0 [start|status|watch [worker]|stop [--now]]" ;;
esac
