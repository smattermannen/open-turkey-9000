#!/bin/bash
# B1 (§3.6 allowance, §4.10 notifications): an allowance session warns before
# it ends, on the real desktop. Run as the desktop user on a disposable box
# with kc9k installed and the daemon running, the screen unlocked:
#
#   ci/allowance_notify_test.sh
#
# A test block holds one app (gnome-calculator); its allowance is 3 minutes,
# advance notices 2 minutes. The app is opened, kept focused and the mouse
# moved now and then (use = the block's app focused, input < 3 min), and a
# session started. dbus-monitor watches the session bus for the daemon's
# notify-send. Passes when "2 minutes … left" comes after about 1 minute of
# use, and the session is over (the block blocks the app again) after about 3.
# Takes ~4 minutes. Removes its block and restores the notification settings.
set -u
B=kc9k-b1test
APP=gnome-calculator
fail=0
check() { if [ "$2" = 0 ]; then echo "PASS: $1"; else echo "FAIL: $1"; fail=1; fi; }

for cmd in kc9k xdotool dbus-monitor $APP; do
	command -v "$cmd" >/dev/null || { echo "needs $cmd"; exit 2; }
done
[ -n "${DISPLAY:-}" ] || { echo "needs the desktop's DISPLAY"; exit 2; }
systemctl is-active -q kc9k || { echo "the kc9k daemon isn't running"; exit 2; }

saved=$(kc9k notifications)
old_advance=$(awk '$1=="advance"{print $2}' <<<"$saved")
old_allowance=$(awk '$1=="allowance"{print $2}' <<<"$saved")
work=$(mktemp -d)
cleanup() {
	kill "$mon" "$jiggle" 2>/dev/null
	pkill -f "^(/usr/bin/)?$APP" 2>/dev/null
	kc9k allowance end "$B" >/dev/null 2>&1
	kc9k allowance remove "$B" >/dev/null 2>&1
	kc9k stop "$B" >/dev/null 2>&1
	kc9k block remove "$B" >/dev/null 2>&1
	kc9k notifications advance "${old_advance:-5}" >/dev/null
	kc9k notifications allowance "${old_allowance:-on}" >/dev/null
	rm -rf "$work"
}
mon=; jiggle=
trap cleanup EXIT

kc9k block remove "$B" >/dev/null 2>&1
kc9k block create "$B" >/dev/null || exit 1
kc9k block add-app "$B" "$APP" >/dev/null || exit 1
kc9k notifications allowance on >/dev/null
kc9k notifications advance 2 >/dev/null
kc9k allowance set "$B" 3m >/dev/null || exit 1

dbus-monitor --session "interface='org.freedesktop.Notifications',member='Notify'" >"$work/notify" 2>&1 &
mon=$!

kc9k allowance start "$B" || exit 1
started=$(date +%s)
"$APP" >/dev/null 2>&1 &
sleep 3
# Keep it focused and the person "at the desktop".
( while :; do
	w=$(xdotool search --onlyvisible --class "$APP" 2>/dev/null | head -1)
	[ -n "$w" ] && xdotool windowactivate "$w" 2>/dev/null
	xdotool mousemove_relative -- 1 0 2>/dev/null; sleep 10
	xdotool mousemove_relative -- -1 0 2>/dev/null; sleep 10
done ) &
jiggle=$!

warned=0; ended=0
while [ $(( $(date +%s) - started )) -lt 300 ]; do
	if [ $warned = 0 ] && grep -q "of today's allowance left" "$work/notify"; then
		warned=$(( $(date +%s) - started ))
		grep -o "'$B': [^\"]*left" "$work/notify" | head -1
	fi
	if ! pidof "$APP" >/dev/null && [ $(( $(date +%s) - started )) -gt 20 ]; then
		ended=$(( $(date +%s) - started )); break
	fi
	sleep 2
done
echo "warning after ${warned}s, app closed after ${ended}s"

check "the 2-minute warning came" $([ $warned -gt 0 ] && echo 0 || echo 1)
check "it said 2 minutes left" $(grep -q "'$B': 2 minutes of today's allowance left" "$work/notify" && echo 0 || echo 1)
check "after about a minute of use (40-120 s)" $([ $warned -ge 40 ] && [ $warned -le 120 ] && echo 0 || echo 1)
check "the session ended after about 3 minutes of use (150-280 s)" $([ $ended -ge 150 ] && [ $ended -le 280 ] && echo 0 || echo 1)
check "it warned before it ended" $([ $warned -gt 0 ] && [ $ended -gt $warned ] && echo 0 || echo 1)
exit $fail
