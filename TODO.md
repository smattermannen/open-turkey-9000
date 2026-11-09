# KC9K status

Works on Linux Mint 22.3: strict mode, noexec, no-escape reboot, master password, screen lock in Frozen Chicken.
Tested on a real Mint/Windows dual boot: the LibreWolf extension (`ci/browser_test.py`) and no-escape (Mint and Windows boot without the password, entry editing is locked, the firmware boot menu still reaches Windows).

## Next: try on the dual boot
Tried on the real dual boot on 2026-11-09 (Mint 22.3, LibreWolf 159.0.1, extension 1.5.0). No bugs found. Passed:
- The mandatory extension: open LibreWolf closed once after the upgrade, then checks in every few seconds (`kc9k status`); Firefox with `--marionette` closed in 3 s; a renamed copy of Firefox, spotted by its engine, closed after 62 s. Plain Firefox is closed at once by force-librewolf.
- LibreWolf hardening: developer-tool shortcuts and Inspect are off, in private windows too; the five prefs are padlocked in about:config; the extension can't be disabled or removed and shows 1.5.0 in about:addons.
- Window titles: an xterm titled YouTube was asked to close; a window that ignored it was killed 5 s later; youtube.com and a Google search for youtube go to the block page. Python programs are spared on purpose (desktop helpers), so test with something else.
- Notifications 5 minutes before a block schedule and a scheduled freeze; the scheduled lock freeze locked and kept the screen locked from start to end.
- The settings password in the terminal and the interface (setting it there counts as giving it; a reopened interface asks once).
- The interface: week calendar (Blocks > a block > Schedule), window-titles list, Scheduled list under Device Blocks, Unlock… opens the paged challenge in a terminal.

Still to try:
1. Notification before the end of a quota session.
2. Scheduled Frozen Chicken in logout and shutdown modes.
3. Strict mode (`unshare -Urm` gives no way out, `--user` Flatpak apps are killed, Electron apps aren't taken for browsers). Needs a moment when the `test` block can be unlocked: strict mode denies sudo while any block is locked.
4. From before: shutdown mode and the `kc9k.nofreeze` GRUB escape; measure the fast relock (89c71bf); run `ci/browser_test.py` in LibreWolf again (per-block greyscale and images, title blocking).

## Next: new features
- Page keyword threshold: match keywords in a page's text, and act once enough of them are found. A threshold per block and per action (block page, greyscale, no images). Distinct keywords count, not repeat hits. A plain keyword matches whole words only (`porn` misses `pornhub`; `18+` misses `1118+`); wildcards match inside words (`*porn*` hits `pornhub`). Raising a threshold is a loosening edit.
- Keybinds: a key combination that starts a chosen block, optionally with a preset time lock (e.g. one hour), for studying.
- Browser option `disable-private-browsing`, lockable like the other options.
- The interface looks rough throughout and needs a design pass.

## Later
- Count quota time only while a block's sites or apps are in use, measured by the extension (its statistics sampling in background.js can be reused) and the daemon's foreground-app sampling. Watch for loopholes: a muted video beside another window, and browsers without the extension while a session is open.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
