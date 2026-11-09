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

`ci/browser_test.py` passed in LibreWolf on 2026-11-09 (all checks, with page keywords and no-private added). It could not run while block-devtools was on: the DisableDeveloperTools policy turns off Marionette, so testing builds now leave that policy out while the remote-control marker exists.

## Next
- The interface looks rough throughout and needs a design pass. (Its window now fits small and scaled screens: the block details scroll.)
- Hotkeys only work in Cinnamon; GNOME keeps custom shortcuts under another dconf path.
- Blocks tab: a "+" button for a new block.
- Several lists per block, so one block can hold several layers: one section of allowlists, one of blocklists. Each list has a name. The threshold (strictness) only applies to lists set to "Keyword", the kind that searches a page's rendered text.
- More actions per list: open a custom link instead of the block page; close the tab; close the browser; close the browser and log out; close the browser and shut down; close the browser and start monk mode for a custom time.
- Monk mode: a quick button that blocks and locks `*.*` for a set time.
- Nepsis mode (optional, lockable like the other options): when many block rules trigger within a short time (likely hunting for loopholes), take an action the user chooses, e.g. shut down for 1 hour, or start and lock monk mode for 1 hour, so the user really shifts mental state. Think about more ways to notice attempts to bypass KC9K or find loopholes.
- The terms in a list are hidden until the list's terms area is clicked, like sensitive content on Reddit.
- A search box in each list to find terms in it.
- Remove every note saying some parts only work with the extension: it is forced by default, so they no longer apply (interface, README, command help).

## Later
- Count quota time only while a block's sites or apps are in use, measured by the extension (its statistics sampling in background.js can be reused) and the daemon's foreground-app sampling. Watch for loopholes: a muted video beside another window, and browsers without the extension while a session is open.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
