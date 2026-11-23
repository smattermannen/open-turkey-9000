#!/bin/bash
# The one-click installer's checks (ROADMAP 2.11, §4.11): installs a kc9k
# .deb as Mint's package installer does (apt resolves its dependencies),
# checks it did all install.sh does, LibreWolf included (installed by the
# daemon, as the postinst can't run apt), then removes it. As root, only on
# a disposable machine (a GitHub runner, the clean Mint VM): it installs
# packages and LibreWolf and changes /etc.
#
#     sudo ci/deb_install_check.sh kc9k_1.2.3_amd64.deb [1.2.3]
#
# The version, when given, must be what `kc9k version` says. When run
# through sudo, kc9k is also run as that user, through the wrapper.
# DEB_INSTALLED=1: a person installed the .deb already (double-click,
# Install: ci/deb_vm_test.sh with MANUAL=1); only the checks run, and
# LIBREWOLF_BEFORE=yes|no says whether LibreWolf was there before.
# LIBREWOLF_MINUTES (default 15): how long the daemon gets for LibreWolf
# (its first try is a minute after it starts). Exit status: the number of
# failed checks.
set -uo pipefail

[ "$(id -u)" -eq 0 ] || { echo "run as root (sudo)"; exit 1; }
[ -f "${1:-}" ] || { echo "usage: sudo $0 kc9k_<version>_<arch>.deb [version]"; exit 1; }
deb=$(readlink -f "$1")
want=${2:-}
user=${SUDO_USER:-}
marker=/var/lib/kc9k/librewolf-auto
lw_unit=kc9k-librewolf-install
failures=0

check() {
	local name=$1
	shift
	echo "RUN: $name"
	if "$@"; then
		echo "PASS: $name"
	else
		echo "::error::FAIL: $name"
		failures=$((failures + 1))
	fi
}
# within SECS CMD...: waits up to SECS seconds for CMD to succeed.
within() {
	local end=$((SECONDS + $1))
	shift
	until "$@"; do
		[ $SECONDS -ge $end ] && return 1
		sleep 3
	done
}
librewolf_installed() { [ -x /usr/bin/librewolf ] && dpkg-query -W -f='${Status}' librewolf 2>/dev/null | grep -q 'install ok installed'; }
policy_has_extension() { grep -q force_installed "$1" 2>/dev/null; }

# LIBREWOLF_BEFORE=yes|no: whether LibreWolf was there before the install
# (given with DEB_INSTALLED=1, as the install is over by now).
had_librewolf=${LIBREWOLF_BEFORE:-}
if [ -z "$had_librewolf" ]; then
	had_librewolf=no
	librewolf_installed && had_librewolf=yes
fi
echo "LibreWolf before the install: $had_librewolf"

export DEBIAN_FRONTEND=noninteractive
if [ "${DEB_INSTALLED:-}" = 1 ]; then
	echo "Installed by a person: checking only"
else
	echo "Installing $deb"
	check "the .deb installs" apt-get install -y "$deb"
fi
check "package installed" bash -c "dpkg-query -W -f='\${Status}' kc9k | grep -q 'install ok installed'"
if [ -n "$want" ]; then
	check "kc9k version says $want" bash -c "/usr/local/bin/kc9k-bin version | grep -qF -- '$want'"
fi
check "daemon running" within 30 systemctl is-active --quiet kc9k
check "daemon enabled at boot" systemctl is-enabled --quiet kc9k
check "sudo rule valid, limited to group sudo" bash -c "visudo -cf /etc/sudoers.d/kc9k >/dev/null && grep -q '^%sudo ALL=(root) NOPASSWD: /usr/local/bin/kc9k-bin' /etc/sudoers.d/kc9k"
check "wrapper runs kc9k-bin through sudo" grep -q 'exec sudo /usr/local/bin/kc9k-bin' /usr/local/bin/kc9k
check "database folder" test -d /var/lib/kc9k
check "GUI in the menu" bash -c "grep -qx 'Exec=kc9k-gui' /usr/share/applications/kc9k.desktop && test -x /usr/local/bin/kc9k-gui && test -f /usr/local/lib/kc9k-gui/window.py"
check "GUI's Python compiles" python3 -c "import sys, py_compile; [py_compile.compile(f, doraise=True) for f in sys.argv[1:]]" /usr/local/bin/kc9k-gui /usr/local/lib/kc9k-gui/*.py
check "kc9k status as root" /usr/local/bin/kc9k-bin status
if [ -n "$user" ]; then
	check "kc9k status as $user, through the wrapper" runuser -u "$user" -- /usr/local/bin/kc9k status
fi
check "extension policy for LibreWolf" within 60 policy_has_extension /etc/librewolf/policies/policies.json
check "extension policy for Chrome" within 60 policy_has_extension /etc/opt/chrome/policies/managed/kc9k-setup.json
check "extension policy for Chromium" within 60 policy_has_extension /etc/chromium/policies/managed/kc9k-setup.json

minutes=${LIBREWOLF_MINUTES:-15}
echo "Waiting up to $minutes minutes for the daemon's LibreWolf"
check "LibreWolf installed, the daemon done" within $((minutes * 60)) bash -c "test -s $marker && [ -x /usr/bin/librewolf ]"
echo "marker: $(cat "$marker" 2>/dev/null)"
if [ "$had_librewolf" = no ]; then
	check "the daemon installed it" grep -q 'installed by the daemon' "$marker"
fi
check "LibreWolf's own copy of the extension policy" within 60 bash -c "for d in /usr/lib/librewolf /usr/share/librewolf; do [ -d \$d ] && grep -q force_installed \$d/distribution/policies.json && exit 0; done; exit 1"
journalctl -u "$lw_unit" --no-pager 2>/dev/null | tail -20

# A restarted daemon finds the marker and leaves LibreWolf alone.
systemctl restart kc9k
check "a restart leaves the marker" test -s "$marker"

check "removal works while nothing is locked" apt-get remove -y kc9k
check "binary and wrapper gone" bash -c "! test -e /usr/local/bin/kc9k-bin && ! test -e /usr/local/bin/kc9k"
check "sudo rule gone" test ! -e /etc/sudoers.d/kc9k
check "service gone" bash -c "! systemctl is-active --quiet kc9k && test ! -e /etc/systemd/system/kc9k.service"
check "LibreWolf stays" librewolf_installed

echo "Failures: $failures"
exit "$failures"
