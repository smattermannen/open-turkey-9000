# KC9K status

Works on Linux Mint 22.3: strict mode, noexec, no-escape reboot, master password, screen lock in Frozen Chicken.
Tested on a real Mint/Windows dual boot: the LibreWolf extension (`ci/browser_test.py`) and no-escape (Mint and Windows boot without the password, entry editing is locked, the firmware boot menu still reaches Windows).

## Next
1. Test Frozen Chicken unlock, restart, logout and shutdown modes.
2. Rebuild the interface: Dashboard, Blocks, Device Blocks, Statistics, Settings.
3. Measure real quota time in the extension.
4. Weekly calendar, statistics, notifications and settings password.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`.
