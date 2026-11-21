#!/bin/bash
# Tamper protection live, the VM parts of ROADMAP 2.9 (§4.10): "block the
# installer" and "close guest profiles" (the login screen's guest session).
# Only on the disposable test VM (notes/testing.md: kc9k-vm, snapshot
# "clean"): it locks, reinstalls KC9K and restarts. Never on a real
# computer.
#
# From the test machine:
#
#     ci/tamper_vm_test.sh host [domain] [ssh-alias]    # default kc9k-vm
#
# Reverts the VM to "clean" (running), copies this tree there, builds two
# .debs of it (9000.1~vmcheck, installed first, and a newer 9000.2~vmcheck),
# runs the checks inside as root and reverts the VM again at the end.
# "inside 1": with "installer" on and a block locked, `sudo dpkg -i` of the
# installed .deb, `sudo ./install.sh` and an older .deb are refused and
# change nothing, the newer .deb installs, and the same .deb installs again
# once the lock is over; then "guest" on and a restart. "inside 2", at the
# next boot: LightDM offers no guest session. One PASS/FAIL line per
# check; the verdict is PASS only if all pass.
#
# INSTALLER_LOCK_MINUTES (default 10): the lock, long enough for
# install.sh's build in the VM (it needs Go there, /usr/local/go).
set -uo pipefail

STATE=/var/lib/kc9k-vmcheck-tamper
UNIT=kc9k-vmcheck-tamper
# shellcheck source=ci/vm_check_lib.sh
. "$(dirname "$(readlink -f "$0")")/vm_check_lib.sh"

LOCK_MIN=${INSTALLER_LOCK_MINUTES:-10}
V1=9000.1~vmcheck
V2=9000.2~vmcheck
GUEST_BASELINE=/etc/lightdm/lightdm.conf.d/10-kc9k-vmcheck-guest.conf
NO_GUEST=/etc/lightdm/lightdm.conf.d/90-kc9k-no-guest.conf  # blocker.lightdmGuestConf
REFUSAL='block the installer'                                # prepare-install's message

deb() { ls "$STATE"/debs/kc9k_"$1"_*.deb; }
installed() { dpkg-query -W -f='${Status} ${Version}' kc9k 2>/dev/null; }
is_installed() { [ "$(installed)" = "install ok installed $1" ]; }
binsum() { sha256sum "$KC" | cut -d' ' -f1; }

# sudo_user CMD...: CMD through the user's sudo, as `sudo CMD` typed by
# them; output to $STATE/out (and the log).
sudo_user() {
	as_user sudo -n "$@" </dev/null >"$STATE/out" 2>&1
	local rc=$?
	sed 's/^/     | /' "$STATE/out" >>"$STATE/log"
	return $rc
}

# refused NAME VERSION CMD...: CMD fails with prepare-install's refusal and
# leaves VERSION installed with the same binary, inside the lock.
refused() {
	local name=$1 version=$2 sum rc
	shift 2
	sum=$(binsum)
	sudo_user "$@"
	rc=$?
	if ! locked; then
		fail "$name: the lock ended before the check ran (raise INSTALLER_LOCK_MINUTES)"
		return
	fi
	if [ $rc = 0 ]; then
		fail "$name: went through"
	elif ! grep -q "$REFUSAL" "$STATE/out"; then
		fail "$name: failed, but not with the installer guard's refusal (see the output above)"
	elif ! is_installed "$version" || [ "$(binsum)" != "$sum" ]; then
		fail "$name: refused, but the installed KC9K changed ($(installed))"
	else
		ok "$name: refused, nothing changed"
	fi
}

# LightDM's first seat: does it offer a guest session?
guest_offered() {
	local seat
	seat=$(busctl --system get-property org.freedesktop.DisplayManager /org/freedesktop/DisplayManager \
		org.freedesktop.DisplayManager Seats 2>/dev/null | grep -o '"[^"]*"' | head -1 | tr -d '"')
	[ -n "$seat" ] || return 2
	busctl --system get-property org.freedesktop.DisplayManager "$seat" \
		org.freedesktop.DisplayManager.Seat HasGuestAccount 2>/dev/null | grep -qx 'b true'
}
lightdm_answers() { guest_offered; [ $? != 2 ]; }

inside1() {
	vm_start_state
	[ "$(id -u)" = 0 ] || { echo "inside 1 runs as root (systemd-run)"; exit 2; }
	say "kc9k $(installed); user $(vm_user)"
	check "setup: $V1 installed from its .deb" is_installed "$V1"

	# --- block the installer ---
	kc tamper installer on >>"$STATE/log" 2>&1
	say "locking for ${LOCK_MIN}m"
	lock_for "$LOCK_MIN" || fail "kc9k start --for refused"
	check "a block is locked" locked
	refused "during: sudo dpkg -i of the installed .deb ($V1)" "$V1" dpkg -i "$(deb "$V1")"
	if [ -x /usr/local/go/bin/go ] || command -v go >/dev/null; then
		refused "during: sudo ./install.sh" "$V1" bash -c "cd '$(dirname "$(readlink -f "$0")")/..' && ./install.sh"
	else
		fail "setup: no Go in the VM, so sudo ./install.sh can't be checked"
	fi
	if sudo_user dpkg -i "$(deb "$V2")" && is_installed "$V2"; then
		ok "during: sudo dpkg -i of a newer .deb ($V2) installs"
	else
		fail "during: sudo dpkg -i of a newer .deb ($V2): $(installed)"
	fi
	check "during: still locked after the upgrade" within 30 locked
	refused "during: sudo dpkg -i of an older .deb ($V1)" "$V2" dpkg -i "$(deb "$V1")"

	say "waiting for the lock to end"
	within $((LOCK_MIN * 60 + 30)) bash -c "! $KC status 2>/dev/null | grep -q 'vmcheck .*timer lock'" ||
		fail "the lock didn't end in time"
	if sudo_user dpkg -i "$(deb "$V2")" && is_installed "$V2"; then
		ok "after the lock: sudo dpkg -i of the installed .deb ($V2) installs again"
	else
		fail "after the lock: sudo dpkg -i of the installed .deb ($V2): $(installed)"
	fi

	# --- close guest profiles: the login screen's guest session ---
	if ! within 30 lightdm_answers; then
		fail "setup: LightDM doesn't answer on the system bus (no LightDM here?)"
		vm_finish
		return
	fi
	if ! guest_offered; then
		say "no guest session yet: allowing it ($GUEST_BASELINE) and restarting LightDM"
		mkdir -p "$(dirname "$GUEST_BASELINE")"
		printf '# kc9k VM check (ci/tamper_vm_test.sh)\n[Seat:*]\nallow-guest=true\ngreeter-allow-guest=true\n' >"$GUEST_BASELINE"
		systemctl restart display-manager
		within 60 lightdm_answers
	fi
	check "before: the login screen offers a guest session" guest_offered
	kc tamper guest on >>"$STATE/log" 2>&1
	check "guest on: LightDM's drop-in is written within 60 s" within 60 test -s "$NO_GUEST"
	vm_after_restart "$UNIT"
	say "restarting"
	sync
	systemctl reboot
}

inside2() {
	say "after the restart"
	systemctl disable "$UNIT.service" >/dev/null 2>&1
	if ! within 90 lightdm_answers; then
		fail "LightDM doesn't answer after the restart"
	else
		guest_offered
		case $? in
		1) ok "after the restart: the login screen offers no guest session" ;;
		*) fail "after the restart: the login screen still offers a guest session" ;;
		esac
	fi
	if lightdm --show-config 2>&1 | grep -q 'allow-guest=false'; then
		ok "after the restart: LightDM's configuration has allow-guest=false"
	else
		fail "after the restart: allow-guest=false not in lightdm --show-config"
		lightdm --show-config >>"$STATE/log" 2>&1
	fi
	vm_finish
}

case ${1:-} in
host)
	shift
	vm_prepare "${1:-kc9k-vm}" "${2:-${1:-kc9k-vm}}" "$V1" "$V2"
	echo "Running in the VM: a ${LOCK_MIN}m lock with reinstalls, then a restart with guest on."
	vm_run_inside "$UNIT-1"
	vm_wait_verdict $((LOCK_MIN + 20))
	;;
inside)
	case ${2:-} in
	1) inside1 ;;
	2) inside2 ;;
	*) echo "inside 1|2"; exit 2 ;;
	esac ;;
*) sed -n '2,23p' "$0"; exit 2 ;;
esac
