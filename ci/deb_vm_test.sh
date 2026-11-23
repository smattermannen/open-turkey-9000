#!/bin/bash
# The one-click installer on a clean Linux Mint (ROADMAP 2.11, §4.11): the
# .deb of this tree installs and sets everything up, LibreWolf included.
# Only on a disposable VM with a snapshot of a fresh Mint install (no KC9K,
# no LibreWolf; the desktop user with passwordless sudo and an ssh key
# login, as kc9k-vm's "tester"); it installs packages and is reverted at
# the end. Never on a computer in use.
#
# From the test machine (Go in /usr/local/go for the build):
#
#     ci/deb_vm_test.sh host [domain] [ssh-alias]      # default kc9k-mint
#     MANUAL=1 ci/deb_vm_test.sh host [domain] [ssh-alias]
#
# Reverts the VM to its snapshot (VM_SNAPSHOT, default "clean"; running),
# builds kc9k_9000.1~vmcheck_amd64.deb (a release build: no devmaster),
# copies it and ci/deb_install_check.sh in, and runs the checks as root
# through the VM user's sudo (so kc9k is also run as them). Default: the
# check installs the .deb with apt, as Mint's package installer does.
# MANUAL=1: the .deb goes to the user's Downloads; a person double-clicks
# it in the VM, presses Install, then Enter here; only the checks run.
# LIBREWOLF_MINUTES (default 20). Exit status 0 only when every check
# passes; the VM is reverted (shut off) at the end.
set -uo pipefail

[ "${1:-}" = host ] || { sed -n '2,22p' "$0"; exit 2; }
# shellcheck source=ci/vm_check_lib.sh
. "$(dirname "$(readlink -f "$0")")/vm_check_lib.sh"

VM_DOMAIN=${2:-kc9k-mint}
VM_SSH=${3:-$VM_DOMAIN}
VERSION=9000.1~vmcheck
root=$(cd "$(dirname "$0")/.." && pwd)

trap 'echo "reverting $VM_DOMAIN to $VM_SNAPSHOT"; vm_revert' EXIT
echo "reverting $VM_DOMAIN to $VM_SNAPSHOT (running)"
vm_revert running || exit 2
TAGS='' vm_build_debs "$VERSION" || { echo "building the .deb failed"; exit 2; }
deb=$(cd "$VM_DEBS" && ls kc9k_"$VERSION"_*.deb)

before=$(ssh "$VM_SSH" 'test -x /usr/bin/librewolf && echo yes || echo no')
echo "LibreWolf in the VM before: $before"
if [ "$before" = yes ]; then
	echo "the snapshot already has LibreWolf: not a clean Mint" >&2
fi

scp -q "$VM_DEBS/$deb" "$root/ci/deb_install_check.sh" "$VM_SSH:/tmp/" || exit 2
rm -rf "$VM_DEBS"
minutes=${LIBREWOLF_MINUTES:-20}
if [ "${MANUAL:-}" = 1 ]; then
	ssh "$VM_SSH" "mkdir -p ~/Downloads && mv /tmp/$deb ~/Downloads/" || exit 2
	echo "In the VM: open Downloads, double-click $deb, press Install (and"
	echo "give the password). Press Enter here when the installer says it's done."
	read -r _
	ssh "$VM_SSH" "sudo DEB_INSTALLED=1 LIBREWOLF_BEFORE=$before LIBREWOLF_MINUTES=$minutes bash /tmp/deb_install_check.sh ~/Downloads/$deb $VERSION"
else
	ssh "$VM_SSH" "sudo LIBREWOLF_MINUTES=$minutes bash /tmp/deb_install_check.sh /tmp/$deb $VERSION"
fi
rc=$?
if [ $rc -eq 0 ]; then echo PASS; else echo "FAIL ($rc checks)"; fi
exit $rc
