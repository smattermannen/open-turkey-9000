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
6. Website lists: a page that changes address without loading (YouTube) caught by an address list; the Websites tab on the laptop's real screen (hidden terms, search box).
7. A one-off logout and shutdown from a list (they end the session, so not run unattended); the Dashboard's Monk mode button and a locked global bypass prevention refusing `nepsis off` by hand.
8. A hotkey pressed under GNOME (the dconf entries are right; only Cinnamon is installed here).
9. Quota by use with someone at the computer: the focused tab and a blocked app in front count, idle time doesn't; another browser is closed during a website session.

Tried on 2026-11-10 after installing (extension 1.9.1), unattended. Passed:
- The upgrade turned the `test` and `H` blocks into lists exactly: sites and patterns in Blocked, H's page keywords in Page words with their action, locks kept, `/etc/hosts` and the firewall unchanged.
- `ci/browser_test.py`: all 20 checks, including the new title exception and redirect.
- `ci/actions_test.py` (new): a list's picture shows instead of the page, close-tab, close-browser, bypass prevention closing the browser after three blocked pages, monk mode from a list starting and ending by itself after a minute.
- `ci/quota_test.py` (new): a quota session spends nothing on another page, and spends while the block's page plays muted audio.
- A daemon restart is logged as a tamper event; hotkeys are written to Cinnamon's and GNOME's dconf lists and removed from both (fixed: an empty GNOME list).
- Found and fixed: close-browser and monk mode from a list never reached the daemon, because closing the last tab quit the browser before the report went out.

`ci/browser_test.py` passed in LibreWolf on 2026-11-09 (all checks, with page keywords and no-private added). It could not run while block-devtools was on: the DisableDeveloperTools policy turns off Marionette, so testing builds now leave that policy out while the remote-control marker exists.

## Next
- The interface looks rough throughout and needs a design pass. (Its window now fits small and scaled screens: the block details scroll.)
- Bypass prevention: more signs of loophole hunting to log as events. Logged so far: blocked pages, apps, window titles (per block), browsers closed (globally), refused loosening, the daemon restarting within a boot, clock jumps.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
