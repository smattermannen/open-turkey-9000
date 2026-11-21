#!/bin/bash
# Does cinnamon-screensaver set logind's LockedHint? (ROADMAP B8, §3.7.)
# KC9K tells a locked screen from logind's LockedHint alone
# (blocker.lockedByLogind): if the screensaver never sets it, a locked
# screen counts as use until the idle time (3 min) has passed.
#
# On the box (a Cinnamon desktop), as yourself, from a terminal in that
# desktop (not over ssh, not with sudo):
#
#     ci/lockedhint_check.sh
#
# It locks the screen (cinnamon-screensaver-command --lock, as Ctrl+Alt+L
# does) and reads LockedHint for your desktop sessions; unlock with your
# password when it locks. One PASS/FAIL line per check at the end.
set -uo pipefail

LOCK_WAIT=${LOCK_WAIT:-15}         # seconds for the hint to follow the lock
UNLOCK_WAIT=${UNLOCK_WAIT:-180}    # seconds for you to unlock

FAILED=0
RESULTS=()
pass() { RESULTS+=("PASS $*"); }
fail() { RESULTS+=("FAIL $*"); FAILED=1; }

[ "$(id -u)" != 0 ] || { echo "Run it as yourself, not root."; exit 2; }
[ -n "${DISPLAY:-}" ] && [ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ] ||
	{ echo "Run it from a terminal in your desktop (DISPLAY and the session bus are needed)."; exit 2; }
command -v cinnamon-screensaver-command >/dev/null || { echo "cinnamon-screensaver-command not found: not Cinnamon?"; exit 2; }

# Your desktop sessions (x11 or wayland), as lockedByLogind finds them.
sessions=()
for id in $(loginctl show-user "$(id -u)" --property=Sessions --value 2>/dev/null); do
	type=$(loginctl show-session "$id" --property=Type --value 2>/dev/null)
	case $type in x11 | wayland) sessions+=("$id") ;; esac
done
[ ${#sessions[@]} -gt 0 ] || { echo "No x11/wayland session of yours in loginctl."; exit 2; }

hints() { local id; for id in "${sessions[@]}"; do printf '%s=%s ' "$id" "$(loginctl show-session "$id" --property=LockedHint --value)"; done; }
all_hints() { # $1 = yes|no
	local id
	for id in "${sessions[@]}"; do
		[ "$(loginctl show-session "$id" --property=LockedHint --value)" = "$1" ] || return 1
	done
}
saver_active() { cinnamon-screensaver-command --query 2>/dev/null | grep -qi 'is active'; }
within() { # SECS CMD...
	local end=$((SECONDS + $1))
	shift
	until "$@"; do
		[ $SECONDS -ge $end ] && return 1
		sleep 1
	done
}

echo "Desktop: ${XDG_CURRENT_DESKTOP:-?}; $(cinnamon-screensaver --version 2>/dev/null || dpkg-query -W -f='cinnamon-screensaver ${Version}' cinnamon-screensaver 2>/dev/null)"
echo "Sessions: ${sessions[*]}; LockedHint now: $(hints)"
if all_hints no; then pass "before: LockedHint is no while unlocked"; else fail "before: LockedHint isn't no while unlocked ($(hints))"; fi

echo "Locking the screen in 3 seconds; unlock it with your password (within ${UNLOCK_WAIT} s)."
sleep 3
start=$SECONDS
cinnamon-screensaver-command --lock || fail "cinnamon-screensaver-command --lock failed"
if within "$LOCK_WAIT" all_hints yes; then
	pass "locked: LockedHint is yes after $((SECONDS - start)) s"
	locked_hint=1
else
	locked_hint=0
	fail "locked: LockedHint still not yes after ${LOCK_WAIT} s ($(hints); screensaver active: $(saver_active && echo yes || echo no))"
fi
during=$(hints)

# Wait for the unlock: the screensaver goes inactive (or, if it never
# showed as active, the hint clears).
within "$UNLOCK_WAIT" bash -c "! cinnamon-screensaver-command --query 2>/dev/null | grep -qi 'is active'" ||
	fail "not unlocked within ${UNLOCK_WAIT} s"
if [ "$locked_hint" = 1 ]; then
	if within 10 all_hints no; then pass "unlocked: LockedHint back to no"; else fail "unlocked: LockedHint still yes ($(hints))"; fi
fi

echo
echo "LockedHint while locked: $during"
printf '%s\n' "${RESULTS[@]}"
if [ "$FAILED" = 0 ]; then
	echo PASS
else
	echo "FAIL: on this desktop KC9K doesn't see the screen locked (use counted until idle, 3 min); report it in ROADMAP B8."
	exit 1
fi
