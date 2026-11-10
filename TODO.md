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
4. From before: shutdown mode and the `kc9k.nofreeze` GRUB escape; measure the fast relock (89c71bf).
5. Page keywords on real sites over a few days (false hits, slow pages), and a hotkey pressed on the real keyboard (tested with xdotool).
6. Website lists (extension 1.7.0): the upgrade turns the test blocks into lists (`kc9k list show <block>`); a title list, and an exception list by title (e.g. block youtube.com except pages titled "lecture"); a page that changes address without loading (YouTube) is caught by an address list; the interface's Websites tab on the laptop's screen.
7. Extension 1.8.0 actions: redirect, a picture (does LibreWolf load it from 127.0.0.1?), close-tab, close-browser (the tab mustn't come back in the restored session), monk mode from a list, and a one-off logout. Bypass prevention firing (global and per block), and a locked global one refusing `nepsis off`. Monk mode from the Dashboard. `ci/browser_test.py` has new checks for a title exception and a redirect.
8. Hotkeys under GNOME (written beside Cinnamon's; check they don't disturb Cinnamon), the hidden terms and search box in the interface, and a daemon restart (`sudo systemctl restart kc9k`) showing up in `kc9k nepsis` counts.
9. Quota by use (extension 1.9.0): a session spends nothing while idle or on another site, counts a muted video playing in a background tab, counts the block's app in front; another browser is closed during a website session.

`ci/browser_test.py` passed in LibreWolf on 2026-11-09 (all checks, with page keywords and no-private added). It could not run while block-devtools was on: the DisableDeveloperTools policy turns off Marionette, so testing builds now leave that policy out while the remote-control marker exists.

## Next
- The interface looks rough throughout and needs a design pass. (Its window now fits small and scaled screens: the block details scroll.)
- Bypass prevention: more signs of loophole hunting to log as events. Logged so far: blocked pages, apps, window titles (per block), browsers closed (globally), refused loosening, the daemon restarting within a boot, clock jumps.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
