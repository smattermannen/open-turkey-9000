#!/bin/bash
# Checks the testing-only master password, and that normal builds lack it.
set -uo pipefail

if kc9k master </dev/null 2>/dev/null; then
    echo "::error::FAIL: a normal build has the master command"
    exit 1
fi

sudo env "PATH=$PATH" make install TAGS=devmaster
echo testpassword | kc9k master-set
kc9k freeze --for 5m --yes
if echo fel | kc9k master; then
    echo "::error::FAIL: wrong master password accepted"
    exit 1
fi
echo testpassword | kc9k master
state=$(kc9k gui-state)
python3 - "$state" <<'EOF'
import json, sys
state = json.loads(sys.argv[1])
locked = (state["freeze_seconds"] or state["strict"] or state.get("freeze_schedules")
          or any(b["active"] for b in state["blocks"]))
if locked:
    print("::error::FAIL: the master password left locks in place")
    sys.exit(1)
EOF
