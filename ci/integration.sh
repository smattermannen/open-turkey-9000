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
        echo "::error::FAIL: $name"
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

ot block create daily
ot block add-site daily example.net
ot quota set daily 2m
check "quota turns the block on" grep -q 'example.net' /etc/hosts
check "stop refused on a quota block" fails open-turkey stop daily
check "removing a site refused" fails open-turkey block remove-site daily example.net
ot quota start daily --for 15s
check "session opens the block" fails grep -q 'example.net' /etc/hosts
sleep 25
check "block returns after the session" grep -q 'example.net' /etc/hosts
check "time was spent" bash -c 'open-turkey gui-state | python3 -c "import json,sys; b=[x for x in json.load(sys.stdin)[\"blocks\"] if x[\"name\"]==\"daily\"][0]; sys.exit(0 if b[\"quota_used\"] >= 10 and not b[\"quota_open\"] else 1)"'
ot quota set daily 10m
check "a raise waits for midnight" bash -c 'open-turkey gui-state | python3 -c "import json,sys; b=[x for x in json.load(sys.stdin)[\"blocks\"] if x[\"name\"]==\"daily\"][0]; sys.exit(0 if b[\"quota_limit\"]==120 and b[\"quota_next\"]==600 else 1)"'

ot block create race
ot block add-site race example.edu
ot quota set race 10m
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
    ot quota start race
    sleep 1.7
    check "session $i survives a daemon cycle" fails grep -q 'example.edu' /etc/hosts
    ot quota end race
    sleep 1.7
    check "block $i stays on" grep -q 'example.edu' /etc/hosts
done

ot block create lists
ot block add lists reddit.com 'reddit.com/r/funny' '*.xxx' example.info
ot block add-exception lists reddit.com/r/programming
ot start lists
CR=/etc/opt/chrome/policies/managed/open-turkey.json
check "exception reaches Chromium" sudo grep -q 'reddit.com/r/programming' "$CR"
check "wildcard reaches Chromium" sudo grep -q '"xxx"' "$CR"
check "excepted domain leaves hosts" fails grep -q ' reddit.com' /etc/hosts
check "other domain stays in hosts" grep -q 'example.info' /etc/hosts
check "blocked export" bash -c 'open-turkey block export-list lists blocked | grep -qx "\*.xxx"'
check "exceptions export" bash -c '[ "$(open-turkey block export-list lists exceptions)" = reddit.com/r/programming ]'
printf 'a.example.com\n# comment\nb.example.com/path\n' > /tmp/blocked.txt
printf 'c.example.com\n' > /tmp/exceptions.txt
ot block create lists2
check "import blocked" open-turkey block import-list lists2 blocked /tmp/blocked.txt
check "import exceptions" open-turkey block import-list lists2 exceptions /tmp/exceptions.txt
check "imports stay separate" bash -c '[ "$(open-turkey block export-list lists2 blocked | wc -l)" = 2 ] && [ "$(open-turkey block export-list lists2 exceptions)" = c.example.com ]'
ot start lists --for 1h
check "exception refused while locked" fails open-turkey block add-exception lists example.info
check "removing an exception stays allowed" open-turkey block remove-exception lists reddit.com/r/programming

check "short freeze rejected" fails open-turkey freeze --for 10s --yes
check "freeze needs confirmation" fails open-turkey freeze --for 1m
ot freeze --for 2m --yes
check "freeze recorded" bash -c 'open-turkey gui-state | python3 -c "import json,sys; sys.exit(0 if json.load(sys.stdin)[\"freeze_seconds\"] > 0 else 1)"'
check "uninstall refused during freeze" fails sudo open-turkey-bin prepare-uninstall

echo "Failures: $failures"
exit "$failures"
