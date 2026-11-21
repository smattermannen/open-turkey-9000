#!/bin/bash
# Shared by the VM check scripts (ci/strict_pam_vm_test.sh,
# ci/tamper_vm_test.sh). Sourced, never run. Only for the disposable test
# VM (notes/testing.md: kc9k-vm, snapshot "clean"): the checks set a root
# password, add accounts, lock, restart and reinstall.
#
# Host side: vm_prepare reverts the VM to its snapshot (running), copies
# this tree into it and installs a .deb built from it; vm_run_inside starts
# "inside 1" there as root (systemd-run: the checks outlive ssh and survive
# the lockdown, which takes sudo away); vm_wait_verdict waits through any
# restarts for the verdict, prints the log and returns 0 only on PASS; the
# EXIT trap reverts the VM again (shut off).
#
# Inside: say/ok/fail write to $STATE/log; fail also writes FAIL to
# $STATE/verdict; vm_finish writes PASS when nothing failed. $STATE is
# readable by everyone, so the host reads it without sudo.

# STATE is a fixed VM path, meant to expand on this side:
# shellcheck disable=SC2029

VM_URI=${VM_URI:-qemu:///system}
VM_SNAPSHOT=${VM_SNAPSHOT:-clean}
VM_REPO=open-turkey-9000           # the tree in the VM user's home
KC=/usr/local/bin/kc9k-bin          # as root: the wrapper's sudo is gone during a lockdown

say() { echo "$(date '+%H:%M:%S') $*" | tee -a "$STATE/log"; }
ok() { say "PASS $*"; }
fail() { say "FAIL $*"; echo FAIL >"$STATE/verdict"; chmod 644 "$STATE/verdict"; }

# within SECS CMD...: waits up to SECS seconds for CMD to succeed.
within() {
	local secs=$1 end
	shift
	end=$((SECONDS + secs))
	until "$@"; do
		[ $SECONDS -ge $end ] && return 1
		sleep 2
	done
}

# check NAME CMD...: one PASS or FAIL line for CMD.
check() {
	local name=$1
	shift
	if "$@"; then ok "$name"; else fail "$name"; fi
}

# The VM's user (the owner of the tree), and kc9k run as them through the
# wrapper, as a person would (strict mode records SUDO_USER).
vm_user() { stat -c %U "$(dirname "$(readlink -f "$0")")/.."; }
as_user() { runuser -u "$(vm_user)" -- "$@"; }
kc() { as_user kc9k "$@" </dev/null; }

# A block locked by a timer lock: any lock is "anything locked".
lock_for() { # $1 = minutes
	$KC block create vmcheck >/dev/null 2>&1
	$KC block add vmcheck example.com >/dev/null 2>&1
	kc start vmcheck --for "$1m" --yes >>"$STATE/log" 2>&1
}
locked() { $KC status 2>/dev/null | grep -q 'vmcheck .*timer lock'; }

vm_start_state() {
	mkdir -p "$STATE"
	chmod 755 "$STATE"
	: >"$STATE/log"
	chmod 644 "$STATE/log"
	rm -f "$STATE/verdict"
}

# A oneshot unit that runs "inside 2" at the next boot.
vm_after_restart() { # $1 = unit name
	cat >"/etc/systemd/system/$1.service" <<EOF
[Unit]
Description=KC9K VM check, after the restart
After=kc9k.service network.target display-manager.service
[Service]
Type=oneshot
ExecStart=/bin/bash $(readlink -f "$0") inside 2
[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload
	systemctl enable "$1.service" >/dev/null 2>&1
}

vm_finish() {
	[ -e "$STATE/verdict" ] || { echo PASS >"$STATE/verdict"; chmod 644 "$STATE/verdict"; }
	say "verdict: $(cat "$STATE/verdict")"
}

# --- host side ---

vm_revert() { # [running]
	if [ "${1:-}" = running ]; then
		virsh -c "$VM_URI" snapshot-revert "$VM_DOMAIN" "$VM_SNAPSHOT" --running --force >/dev/null || return 1
		local _
		for _ in $(seq 36); do
			ssh -o BatchMode=yes -o ConnectTimeout=5 "$VM_SSH" true 2>/dev/null && return 0
			sleep 5
		done
		echo "ssh $VM_SSH doesn't answer after the revert" >&2
		return 1
	fi
	virsh -c "$VM_URI" snapshot-revert "$VM_DOMAIN" "$VM_SNAPSHOT" --force >/dev/null
}

# Builds one .deb of this tree per version given, into $VM_DEBS.
vm_build_debs() {
	local root version
	root=$(cd "$(dirname "$0")/.." && pwd)
	VM_DEBS=$(mktemp -d)
	for version in "$@"; do
		(cd "$VM_DEBS" && TAGS=${TAGS:-} sh "$root/packaging/build-deb.sh" "$version" >/dev/null) || return 1
	done
}

# vm_prepare DOMAIN SSH VERSION...: revert (running), copy the tree, build
# the .debs, copy them to $STATE/debs in the VM and install the first.
vm_prepare() {
	VM_DOMAIN=$1 VM_SSH=$2
	shift 2
	local root first
	root=$(cd "$(dirname "$0")/.." && pwd)
	trap 'echo "reverting $VM_DOMAIN to $VM_SNAPSHOT"; vm_revert' EXIT
	echo "reverting $VM_DOMAIN to $VM_SNAPSHOT (running)"
	vm_revert running || exit 2
	vm_build_debs "$@" || { echo "building the .deb failed"; exit 2; }
	rsync -a --delete --exclude=.git/ "$root/" "$VM_SSH:$VM_REPO/" || { echo "rsync to $VM_SSH failed"; exit 2; }
	ssh "$VM_SSH" "sudo mkdir -p $STATE/debs && sudo chmod 755 $STATE" || exit 2
	scp -q "$VM_DEBS"/*.deb "$VM_SSH:/tmp/" || exit 2
	ssh "$VM_SSH" "sudo mv /tmp/kc9k_*.deb $STATE/debs/" || exit 2
	first=$(cd "$VM_DEBS" && ls kc9k_"$1"_*.deb)
	echo "installing $first in the VM"
	ssh "$VM_SSH" "sudo dpkg -i $STATE/debs/$first >/dev/null" || { echo "installing $first failed"; exit 2; }
	rm -rf "$VM_DEBS"
}

vm_run_inside() { # $1 = unit name
	ssh "$VM_SSH" "sudo systemd-run --unit $1 --collect /bin/bash \$HOME/$VM_REPO/ci/$(basename "$0") inside 1" || exit 2
}

vm_wait_verdict() { # $1 = minutes at most
	local end verdict
	end=$(($(date +%s) + $1 * 60))
	sleep 30
	until ssh -o BatchMode=yes -o ConnectTimeout=5 "$VM_SSH" "test -s $STATE/verdict" 2>/dev/null; do
		if [ "$(date +%s)" -ge "$end" ]; then
			echo "FAIL: no verdict within $1 minutes; the log so far:"
			ssh -o ConnectTimeout=5 "$VM_SSH" "cat $STATE/log" 2>/dev/null
			return 1
		fi
		sleep 15
	done
	ssh "$VM_SSH" "cat $STATE/log"
	verdict=$(ssh "$VM_SSH" "cat $STATE/verdict")
	echo "$verdict"
	[ "$verdict" = PASS ]
}
