#!/bin/bash
# Exercises the installed program on a disposable machine. It changes the
# clock, the firewall and /etc/hosts, so never run it on a computer you use.
set -uo pipefail

failures=0
check() {
    local name=$1
    shift
    if "$@"; then
        echo "PASS: $name"
    else
        echo "FAIL: $name"
        failures=$((failures + 1))
    fi
}
fails() { ! "$@"; }
ot() { open-turkey "$@"; }

check "sudo rule limited to group sudo" sudo grep -q '^%sudo ALL' /etc/sudoers.d/open-turkey
check "daemon is running" systemctl is-active --quiet open-turkey

ot block create sites
ot block add-site sites example.com
ot start sites
check "hosts entry" grep -q 'example.com' /etc/hosts
check "firewall jump" sudo iptables -C OUTPUT -j OPEN-TURKEY
check "browser policy" sudo test -f /etc/opt/chrome/policies/managed/open-turkey.json

sudo iptables -D OUTPUT -j OPEN-TURKEY
sleep 8
check "daemon restores firewall jump" sudo iptables -C OUTPUT -j OPEN-TURKEY

sudo sed -i '/example.com/d' /etc/hosts
sleep 8
check "daemon restores hosts entry" grep -q 'example.com' /etc/hosts

check "gui-state is JSON" bash -c 'open-turkey gui-state | python3 -m json.tool >/dev/null'
check "import cannot read root files" fails open-turkey block import /etc/shadow

ot stop sites
sleep 8
check "stop clears hosts" fails grep -q 'example.com' /etc/hosts

cp /bin/sleep /tmp/otsleep
ot block create apps
ot block add-app apps otsleep
ot start apps
/tmp/otsleep 60 &
pid=$!
sleep 8
check "blocked app is killed" fails kill -0 "$pid"
ot stop apps

ot block create locked
ot block add-site locked example.org
ot start locked --for 1m
check "time lock refuses stop" fails open-turkey stop locked
sudo systemctl stop systemd-timesyncd 2>/dev/null
sudo date -s '+2 hours' >/dev/null
check "time lock ignores a clock moved forward" fails open-turkey stop locked
sudo date -s '-2 hours' >/dev/null
sleep 65
check "time lock ends after real time" open-turkey stop locked

check "short freeze rejected" fails open-turkey freeze --for 10s --yes
check "freeze needs confirmation" fails open-turkey freeze --for 1m
ot freeze --for 2m --yes
check "freeze recorded" bash -c 'open-turkey gui-state | python3 -c "import json,sys; sys.exit(0 if json.load(sys.stdin)[\"freeze_seconds\"] > 0 else 1)"'
check "uninstall refused during freeze" fails sudo open-turkey-bin prepare-uninstall

echo "Failures: $failures"
exit "$failures"
