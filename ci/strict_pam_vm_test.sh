#!/bin/bash
# Strict mode's PAM root guard live (§3.8, §4.10, ROADMAP 2.6). Only on the
# disposable test VM (notes/testing.md: kc9k-vm, snapshot "clean"): it sets
# a root password, adds an account, locks, edits GRUB and restarts. Never on
# a real computer.
#
# From the test machine:
#
#     ci/strict_pam_vm_test.sh host [domain] [ssh-alias]    # default kc9k-vm
#
# Reverts the VM to "clean" (running), copies this tree there, installs a
# .deb built from it, runs the checks inside as root, and reverts it again
# at the end. During the lockdown sudo is gone, so the VM checks itself:
# "inside 1" (before, during a strict lock, after it ends, then a second
# lock and a restart with kc9k.nofreeze on the kernel line) and, at the
# next boot, "inside 2" (with kc9k.nofreeze: everything back while still
# locked). One PASS/FAIL line per check; the verdict is PASS only if all
# pass.
#
# The root login goes through the "login" PAM service on a pseudo-terminal
# (the same PAM stack a text console uses); su through "su" and "su-l".
# STRICT_LOCK_MINUTES (default 3): the first lock; NOFREEZE_LOCK_MINUTES
# (default 10): the second, which must outlast the restart.
set -uo pipefail

STATE=/var/lib/kc9k-vmcheck-strict
UNIT=kc9k-vmcheck-strict
# shellcheck source=ci/vm_check_lib.sh
. "$(dirname "$(readlink -f "$0")")/vm_check_lib.sh"

LOCK_MIN=${STRICT_LOCK_MINUTES:-3}
NOFREEZE_MIN=${NOFREEZE_LOCK_MINUTES:-10}
OTHER=kc9kother                    # the second account the user su's to
PW=kc9k-vmcheck-Pw1                # root's and $OTHER's password, VM only
GRUB_DROPIN=/etc/default/grub.d/99-kc9k-vmcheck-nofreeze.cfg
PAM_MARK='# kc9k strict mode'      # blocker.pamGuardMarker's start
SUDOERS=/etc/sudoers.d/zz-kc9k-lockdown

# pty_run USER PASSWORD TYPED CMD...: runs CMD as USER on a pseudo-terminal,
# answers a password prompt with PASSWORD, then types TYPED. Exit 0: the
# output has KC9K-OK-42 (TYPED or CMD prints it with $((6*7)), so the
# typed text echoed back can't match); 2: refused before any password was
# asked; 1: otherwise refused or wrong. The transcript goes to the log.
pty_run() {
	local user=$1 pw=$2 typed=$3
	shift 3
	runuser -u "$user" -- python3 -I -c '
import os, pty, re, select, sys, time
pw, typed, argv = sys.argv[1], sys.argv[2], sys.argv[3:]
pid, fd = pty.fork()
if pid == 0:
    os.execvp(argv[0], argv)
out, asked, sent_at, typed_sent = b"", False, None, False
end = time.time() + 25
while time.time() < end:
    if sent_at and not typed_sent and time.time() - sent_at > 2:
        os.write(fd, typed.encode() + b"\n")
        typed_sent = True
    r, _, _ = select.select([fd], [], [], 0.5)
    if not r:
        continue
    try:
        data = os.read(fd, 4096)
    except OSError:
        break
    if not data:
        break
    out += data
    if not asked and re.search(rb"[Pp]assword: *$", out):
        asked = True
        os.write(fd, pw.encode() + b"\n")
        sent_at = time.time()
try:
    os.kill(pid, 9)
except OSError:
    pass
os.waitpid(pid, 0)
sys.stderr.write(out.decode(errors="replace"))
if b"KC9K-OK-42" in out:
    sys.exit(0)
sys.exit(1 if asked else 2)
' "$pw" "$typed" "$@" 2>>"$STATE/pty.log"
}

# What su and login run: the $((6*7)) is worked out by their shell, not here.
# shellcheck disable=SC2016
OK42='echo KC9K-OK-$((6*7))'
su_root() { pty_run "$(vm_user)" "$PW" "" su - -c "$OK42"; }
login_root() { pty_run root "$PW" "$OK42; exit" login root; }
su_other() { pty_run "$(vm_user)" "$PW" "" su - "$OTHER" -c "$OK42"; }

guarded() { grep -qF "$PAM_MARK" /etc/pam.d/login /etc/pam.d/su /etc/pam.d/su-l 2>/dev/null; }
guard_in_all() { local f; for f in login su su-l; do grep -qF "$PAM_MARK" "/etc/pam.d/$f" || return 1; done; }
lockdown_on() { [ -e "$SUDOERS" ] && guard_in_all; }
lockdown_off() { ! [ -e "$SUDOERS" ] && ! guarded; }

# The three things that must work whenever there is no lockdown.
all_work() { # $1 = when
	su_root
	check "$1: the user's su - to root works" [ $? = 0 ]
	login_root
	check "$1: a root login (login PAM service) works" [ $? = 0 ]
	su_other
	check "$1: the user's su to $OTHER works" [ $? = 0 ]
}

inside1() {
	vm_start_state
	[ "$(id -u)" = 0 ] || { echo "inside 1 runs as root (systemd-run)"; exit 2; }
	say "kc9k $($KC --version 2>&1 | head -1); user $(vm_user)"
	# Setup, VM only: root gets a password (cloud images have none), and a
	# second account for the user's own su.
	echo "root:$PW" | chpasswd
	id "$OTHER" >/dev/null 2>&1 || useradd -m -s /bin/bash "$OTHER"
	echo "$OTHER:$PW" | chpasswd
	: >"$STATE/pty.log"

	check "before: no guard in the PAM files, no lockdown" lockdown_off
	all_work "before"

	kc strict on >>"$STATE/log" 2>&1
	check "kc9k strict on" bash -c "$KC status 2>/dev/null | grep -q 'Strict mode is on'"
	say "locking for ${LOCK_MIN}m"
	lock_for "$LOCK_MIN" || fail "kc9k start --for refused"
	check "during: the lockdown is in place within 60 s (sudoers, guard in login, su, su-l)" within 60 lockdown_on

	su_root
	case $? in
	2) ok "during: su - to root refused before a password is asked" ;;
	0) fail "during: su - to root works" ;;
	*) fail "during: su - to root refused only after the password was asked" ;;
	esac
	login_root
	case $? in
	2) ok "during: a root login refused before a password is asked" ;;
	0) fail "during: a root login works" ;;
	*) fail "during: a root login refused only after the password was asked" ;;
	esac
	su_other
	check "during: the user's su to $OTHER still works" [ $? = 0 ]
	check "during: still locked (the checks ran inside the lock)" locked

	say "waiting for the lock to end"
	within $((LOCK_MIN * 60 + 30)) bash -c "! $KC status 2>/dev/null | grep -q 'vmcheck .*timer lock'" ||
		fail "the lock didn't end in time"
	check "after the lock: the guard and the lockdown are gone within 90 s" within 90 lockdown_off
	all_work "after the lock"

	say "locking again for ${NOFREEZE_MIN}m, then restarting with kc9k.nofreeze"
	lock_for "$NOFREEZE_MIN" || fail "the second kc9k start --for refused"
	check "second lock: the lockdown is in place again" within 60 lockdown_on
	cat >"$GRUB_DROPIN" <<'EOF'
# kc9k VM check (ci/strict_pam_vm_test.sh): one boot with kc9k.nofreeze
GRUB_CMDLINE_LINUX_DEFAULT="$GRUB_CMDLINE_LINUX_DEFAULT kc9k.nofreeze"
EOF
	update-grub >>"$STATE/log" 2>&1 || fail "update-grub failed"
	vm_after_restart "$UNIT"
	say "restarting"
	sync
	systemctl reboot
}

inside2() {
	say "after the restart: $(cat /proc/cmdline)"
	systemctl disable "$UNIT.service" >/dev/null 2>&1
	rm -f "$GRUB_DROPIN"
	update-grub >/dev/null 2>&1
	if ! grep -qw kc9k.nofreeze /proc/cmdline; then
		fail "setup: the VM didn't boot with kc9k.nofreeze (GRUB drop-in not used?)"
		vm_finish
		return
	fi
	sleep 20 # the daemon's first cycles
	check "nofreeze: still locked after the restart" locked
	check "nofreeze: no guard in the PAM files, no lockdown (within 60 s)" within 60 lockdown_off
	all_work "nofreeze"
	vm_finish
}

case ${1:-} in
host)
	shift
	vm_prepare "${1:-kc9k-vm}" "${2:-${1:-kc9k-vm}}" 9000.1~vmcheck
	echo "Running in the VM: a strict lock of ${LOCK_MIN}m, then ${NOFREEZE_MIN}m with a restart."
	vm_run_inside "$UNIT-1"
	vm_wait_verdict $((LOCK_MIN + NOFREEZE_MIN + 15))
	;;
inside)
	case ${2:-} in
	1) inside1 ;;
	2) inside2 ;;
	*) echo "inside 1|2"; exit 2 ;;
	esac ;;
*) sed -n '2,22p' "$0"; exit 2 ;;
esac
