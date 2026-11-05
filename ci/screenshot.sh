#!/bin/bash
# Starts the interface on a virtual screen and saves a screenshot.
set -eu
mkdir -p screenshots
open-turkey block create exempel || true
open-turkey block add-site exempel youtube.com reddit.com || true
xvfb-run -a -s "-screen 0 1000x760x24" bash -c '
    open-turkey-gui > screenshots/gui.log 2>&1 &
    gui=$!
    sleep 8
    import -window root screenshots/gui.png
    kill -0 "$gui"
    kill "$gui"
'
cat screenshots/gui.log
