#!/bin/bash
# Checks the testing-only master password, and that normal builds lack it.
set -uo pipefail

if open-turkey master </dev/null 2>/dev/null; then
    echo "::error::FAIL: a normal build has the master command"
    exit 1
fi

sudo env "PATH=$PATH" make install TAGS=devmaster
echo testpassword | open-turkey master-set
open-turkey freeze --for 5m --yes
if echo fel | open-turkey master; then
    echo "::error::FAIL: wrong master password accepted"
    exit 1
fi
echo testpassword | open-turkey master
state=$(open-turkey gui-state)
python3 - "$state" <<'EOF'
import json, sys
state = json.loads(sys.argv[1])
locked = state["freeze_seconds"] or state["strict"] or any(b["active"] for b in state["blocks"])
if locked:
    print("::error::FAIL: the master password left locks in place")
    sys.exit(1)
EOF
