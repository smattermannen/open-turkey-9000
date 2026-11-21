#!/usr/bin/env bash
# Quick check on Linux before pushing (W4): the Core tests, and the GUI's C#
# compiled against WinUI (no packaging: that needs Windows; CI does it).
# Needs the .NET 10 SDK (DOTNET=/opt/dotnet/dotnet by default).
set -euo pipefail
cd "$(dirname "$0")"
DOTNET=${DOTNET:-$(command -v dotnet || echo /opt/dotnet/dotnet)}
export DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1
"$DOTNET" test Kc9k.Tests --nologo -v q 2>&1 | grep -E 'error|Passed!|Failed!|Failed ' || true
"$DOTNET" build Kc9k.Gui -p:WindowsAppSDKSelfContained=false -t:Compile -v q 2>&1 | grep -E 'error|warning CS' | sort -u || true
"$DOTNET" build Kc9k.UiTests -v q 2>&1 | grep -E ' error |warning CS' | sort -u || true
