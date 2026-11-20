#!/bin/bash
# Monk mode live (R-MONK-2, ROADMAP B14), with a restart mid-way. Only on the
# disposable test VM (notes/testing.md: kc9k-vm, snapshot "clean"): it cuts
# the VM's network for several minutes and restarts it. Never on a real
# computer.
#
# From the test machine (the VM reverted to its snapshot, kc9k installed in
# it from this tree, the repo at ~/open-turkey-9000 there):
#
#     ci/monk_live_test.sh host [ssh-alias]      # default kc9k-vm
#
# The host side can't watch while the network is cut (ssh goes with it), so
# the VM checks itself: "inside 1" runs there (systemd-run, as root), starts
# monk mode, checks the three cuts and that undoing each is put back, then
# restarts the VM; a oneshot unit runs "inside 2" at the next boot: the cuts
# are held after the restart, and at the end the network comes back as it
# was. The host waits for ssh to answer again and reads the verdict.
set -uo pipefail

MINUTES=${MONK_MINUTES:-6}          # monk mode's length; the restart fits well inside
STATE=/var/lib/kc9k-monk-live       # in the VM: log, before-state, verdict
LOG=$STATE/log
UNIT=kc9k-monk-live-2

say() { echo "$(date '+%H:%M:%S') $*" | tee -a "$LOG"; }
fail() { say "FAIL $*"; echo FAIL >"$STATE/verdict"; FAILED=1; }
ok() { say "ok   $*"; }
FAILED=0

# The Ethernet interfaces monk mode brings down: as blocker.EthernetLinks
# (type 1, a device behind it, not wireless).
ethernet() {
	local dir name
	for dir in /sys/class/net/*; do
		name=${dir##*/}
		[ "$(cat "$dir/type" 2>/dev/null)" = 1 ] || continue
		[ -e "$dir/device" ] || continue
		[ -e "$dir/wireless" ] || [ -e "$dir/phy80211" ] && continue
		echo "$name"
	done
}
link_up() { ip -o link show dev "$1" | grep -q '[<,]UP[,>]'; }
radio() { nmcli radio wifi 2>/dev/null || echo unknown; }
online() { curl -sS -m 5 -o /dev/null https://1.1.1.1 2>/dev/null || ping -c1 -W3 1.1.1.1 >/dev/null 2>&1; }

# The three cuts (R-MONK-2), and KC9K's own local traffic still working.
check_cut() { # $1 = when
	local when=$1 name up=0
	for name in $(ethernet); do link_up "$name" && up=1 && fail "$when: $name is up"; done
	[ "$up" = 0 ] && ok "$when: every Ethernet interface is down ($(ethernet | tr '\n' ' '))"
	case $(radio) in
	enabled) fail "$when: the Wi-Fi radio is on" ;;
	disabled) ok "$when: the Wi-Fi radio is off" ;;
	*) say "     $when: no Wi-Fi radio here (nmcli says $(radio)); not checked" ;;
	esac
	if online; then fail "$when: 1.1.1.1 answers"; else ok "$when: nothing outside answers"; fi
	if nft list chain inet kc9k output 2>/dev/null | grep -q drop || iptables -S KC9K 2>/dev/null | grep -q DROP; then
		ok "$when: the firewall drops"
	else
		fail "$when: no drop in the firewall"
	fi
	if ping -c1 -W2 127.0.0.1 >/dev/null 2>&1; then ok "$when: loopback works"; else fail "$when: loopback is cut"; fi
	if curl -s -m 5 -o /dev/null http://127.0.0.1:47613/ 2>/dev/null; then
		ok "$when: the rules server answers on loopback"
	else
		fail "$when: the rules server (127.0.0.1:47613) doesn't answer"
	fi
	kc9k monk | tee -a "$LOG" | grep -q 'Monk mode is on' || fail "$when: kc9k monk doesn't say it is on"
}

# Waits up to $1 seconds for "$2…" (a command) to succeed.
within() {
	local secs=$1 end
	shift
	end=$((SECONDS + secs))
	until "$@"; do
		[ $SECONDS -ge $end ] && return 1
		sleep 1
	done
}
all_down() { local n; for n in $(ethernet); do link_up "$n" && return 1; done; return 0; }
radio_off() { [ "$(radio)" != enabled ]; }

inside1() {
	mkdir -p "$STATE"
	: >"$LOG"
	rm -f "$STATE/verdict"
	[ "$(id -u)" = 0 ] || { echo "inside 1 runs as root (systemd-run)"; exit 2; }
	[ "$(kc9k monk)" = "Monk mode is off." ] || { fail "monk mode already on"; exit 1; }
	# The state monk mode must put back.
	{ echo "radio=$(radio)"; for n in $(ethernet); do link_up "$n" && echo "up=$n"; done; } >"$STATE/before"
	say "before: $(tr '\n' ' ' <"$STATE/before")"
	online && ok "online before" || say "     not online before: the restore can only be checked for the links"

	# At the next boot: inside 2.
	cat >/etc/systemd/system/$UNIT.service <<EOF
[Unit]
Description=KC9K monk mode live test, after the restart
After=kc9k.service network.target
[Service]
Type=oneshot
ExecStart=/bin/bash $(readlink -f "$0") inside 2
[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload
	systemctl enable $UNIT.service >/dev/null 2>&1

	say "starting monk mode for ${MINUTES}m"
	kc9k monk --for "${MINUTES}m" --yes >>"$LOG" 2>&1 || {
		fail "kc9k monk --for refused"
		systemctl disable $UNIT.service >/dev/null 2>&1
		exit 1
	}
	date -d "+${MINUTES} min" +%s >"$STATE/end"
	within 30 all_down || fail "the links didn't go down within 30 s"
	check_cut "started"

	# The user puts things back; the daemon undoes it within seconds.
	for n in $(ethernet); do ip link set "$n" up; done
	within 20 all_down && ok "a link brought up is down again" || fail "a link brought up stays up"
	if [ "$(radio)" = disabled ]; then
		nmcli radio wifi on
		within 20 radio_off && ok "the radio switched on is off again" || fail "the radio switched on stays on"
	fi
	nft delete table inet kc9k 2>/dev/null
	iptables -F KC9K 2>/dev/null
	within 20 bash -c '! curl -sS -m 3 -o /dev/null https://1.1.1.1' && ok "the firewall flushed is cut again" ||
		fail "the network works after flushing the firewall"
	# Switching off by hand is refused: it is locked for its time.
	if kc9k mode off monk </dev/null >>"$LOG" 2>&1; then fail "switched off while locked"; else ok "can't be switched off while locked"; fi

	say "restarting mid-way"
	sync
	systemctl reboot
}

inside2() {
	say "after the restart (boot $(cat /proc/sys/kernel/random/boot_id))"
	systemctl disable $UNIT.service >/dev/null 2>&1
	local end
	end=$(cat "$STATE/end")
	sleep 20 # the daemon's first cycles
	if [ "$(date +%s)" -lt "$((end - 20))" ]; then
		check_cut "after the restart"
	else
		fail "the restart took past monk mode's end: raise MONK_MINUTES"
	fi
	say "waiting for the end ($(date -d "@$end" +%H:%M:%S))"
	while [ "$(date +%s)" -lt "$((end + 5))" ]; do sleep 5; done
	restored() {
		local line
		while read -r line; do
			case $line in
			up=*) link_up "${line#up=}" || return 1 ;;
			radio=enabled) [ "$(radio)" = enabled ] || return 1 ;;
			esac
		done <"$STATE/before"
	}
	if within 90 restored; then ok "the links and the radio are back as before"; else fail "not restored: $(ip -o link | cut -d' ' -f2,9 | tr '\n' ' ') radio $(radio)"; fi
	if grep -q '^radio=disabled' "$STATE/before" && [ "$(radio)" = enabled ]; then fail "the radio that was off is on"; fi
	within 60 online && ok "online again" || fail "not online after the end"
	kc9k monk | grep -qx 'Monk mode is off.' && ok "kc9k monk says off" || fail "kc9k monk: $(kc9k monk)"
	[ "$FAILED" = 0 ] && ! [ -e "$STATE/verdict" ] && echo PASS >"$STATE/verdict"
	say "verdict: $(cat "$STATE/verdict")"
}

host() {
	local vm=${1:-kc9k-vm} end verdict
	ssh "$vm" true || { echo "ssh $vm doesn't answer"; exit 2; }
	echo "Starting in $vm; its network goes for ${MINUTES} minutes and it restarts."
	ssh "$vm" "sudo systemd-run --unit kc9k-monk-live-1 --collect env MONK_MINUTES=$MINUTES /bin/bash ~/open-turkey-9000/ci/monk_live_test.sh inside 1"
	end=$(($(date +%s) + MINUTES * 60 + 300))
	sleep 30
	until ssh -o ConnectTimeout=5 "$vm" "test -s $STATE/verdict" 2>/dev/null; do
		[ "$(date +%s)" -ge "$end" ] && { echo "FAIL: no verdict in time; ssh $vm sudo cat $LOG"; exit 1; }
		sleep 15
	done
	ssh "$vm" "sudo cat $LOG"
	verdict=$(ssh "$vm" "cat $STATE/verdict")
	echo "$verdict"
	[ "$verdict" = PASS ]
}

case ${1:-} in
host) shift; host "$@" ;;
inside)
	case ${2:-} in
	1) inside1 ;;
	2) inside2 ;;
	*) echo "inside 1|2"; exit 2 ;;
	esac ;;
*) sed -n '2,17p' "$0"; exit 2 ;;
esac
