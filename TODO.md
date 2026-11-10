# KC9K status

## Note to the next session (2026-11-10, evening)
Read this first, then `git fetch origin`: other sessions push TODO ideas straight to GitHub.
The order of work and what is done is in `docs/PLAN.md` (agreed with the user 2026-11-10: do all of it, any order).

- Everything is committed and pushed, and installed on this machine (the test laptop) with `TAGS=devmaster`, extension 1.14.0. `ci/browser_test.py` (20 checks) and `ci/extras_test.py` (22) pass in a real LibreWolf. Run them with `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest ~/.local/share/kc9k-browsertest/venv/bin/python ci/<name>.py` (the venv and geckodriver were set up there on 2026-11-10; there is no Node, so `cjs ci/match_test_cjs.js` runs the matcher tests).
- **The user uses this laptop for real now** (their own block, the interface open). Tests add and remove blocks, restart the daemon on install (a tamper event) and get pages blocked, all of which count for a bypass ladder set for the whole computer: on 2026-11-10 `ci/extras_test.py` tripped the user's ladder and started monk mode for 10 minutes. The test now paces itself when a global ladder is on. Don't leave test blocks with schedules behind: a schedule holds its block for 2 hours before each window.
- Interface smoke tests: build a binary, point a wrapper at it with `KC9K_DB` set to a scratch file, load `gui/kc9k-gui.py` as a module with `CLI` replaced, give the app another application ID (the user's interface may be running), and never `show()` the window. A database opened through `KC9K_DB` never applies layers.
- Done this session (all in `docs/PLAN.md`): access passwords; daylight saving fixes and tests; options (`polite-close`, `block-page-details`); the toolbar button with time left and add-site; keyword test mode; workaround guard (proxy addresses, mirrors and front-ends, download tools given an address); apps by script, by contents; breaks (allowance, relock) and the Pomodoro long break; project export/import; list actions mute, no-video, declutter, dark; social media settings and a YouTube playlist through a block; places by Wi-Fi; loosening delay per block; interface: Clusters page with recording, ladder editor, app picker, schedule presets, templates.
- Agreed with the user: Linux Mint Cinnamon on X11 only. English only. Users work in the interface; commands exist for it and ask for access passwords too. Only what improves the experience gets built; behaviour changes are options. A refused attempt to stop or loosen a held block is ordinary use and is never counted by bypass detection. Location is Wi-Fi only (no GPS). One ladder for all detectors (no ladder per detector).
- Left to do:
  1. Root-path hardening (`docs/PLAN.md` 8.1): needs a disposable machine or VM, it changes boot and login configuration.
  2. The interface's design pass: nothing new this session has been looked at on a real screen (layout of the access password covers and dialog, Breaks, Social media, Places, Clusters, the ladder editor, the app picker, the keyword test panel, the toolbar popup and badge, the block page's reason line).
  3. Not tried with real use: polite closing with a real app, mute and declutter and dark on real sites, the social media settings beyond YouTube Shorts (selectors in `webext/files/social.js` are from memory of the sites' markup and will need checking), a playlist's videos actually playing, a block turning on and off when moving between places, relocking after a break, idle relock of access passwords (not built).
  4. The tray icon (left out; see `docs/PLAN.md` phase 9).
  5. "Still to try" below, and "Before release".
- Good to know: the browser policy stops many blocked pages before the extension sees any request, so the extension then learns of them from the tab (every 3 s). Anything that needs the request itself (such as where a page was opened from) needs a fallback for that case (`trail` in `background.js`).

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

Tried on 2026-11-10 (afternoon, extension 1.12.0), unattended, in `ci/actions_test.py`. Passed: a ladder closing the tab on its first step and the browser on its second; the search detector (a blocked page opened by hand doesn't count, one clicked on a `/search?q=` page does, also when the browser policy stops the page first); link recording into a cluster (blocked pages too, nothing before the start or after the stop). `kc9k cluster` by hand: create, add, export, import, copy as a blacklist into the locked `test` block (allowed), copy as exceptions (refused, nothing left behind).

Tried on 2026-11-10 after installing (extension 1.9.1), unattended. Passed:
- The upgrade turned the `test` and `H` blocks into lists exactly: sites and patterns in Blocked, H's page keywords in Page words with their action, locks kept, `/etc/hosts` and the firewall unchanged.
- `ci/browser_test.py`: all 20 checks, including the new title exception and redirect.
- `ci/actions_test.py` (new): a list's picture shows instead of the page, close-tab, close-browser, bypass prevention closing the browser after three blocked pages, monk mode from a list starting and ending by itself after a minute.
- `ci/quota_test.py` (new): a quota session spends nothing on another page, and spends while the block's page plays muted audio.
- A daemon restart is logged as a tamper event; hotkeys are written to Cinnamon's and GNOME's dconf lists and removed from both (fixed: an empty GNOME list).
- Found and fixed: close-browser and monk mode from a list never reached the daemon, because closing the last tab quit the browser before the report went out.

`ci/browser_test.py` passed in LibreWolf on 2026-11-09 (all checks, with page keywords and no-private added). It could not run while block-devtools was on: the DisableDeveloperTools policy turns off Marionette, so testing builds now leave that policy out while the remote-control marker exists.

## Next
See "Left to do" at the top and `docs/PLAN.md`. Everything that was listed here on 2026-11-10 (bypass detection leftovers, workaround guard, apps by path and hash, Python programs, clusters in the interface, keyword test mode, auto-relock, location, project export, more list actions, social media settings) is done, except root-path hardening and the interface's design pass.

## Ideas from Cold Turkey users (2026-11-10): all built
- A YouTube playlist through a block: `kc9k social <block> playlist <id> on`.
- Daylight saving: tested across both changes; two bugs fixed.
- Breaks: `kc9k break` (allowance, relock), Pomodoro long break.
- Time left in the extension: the toolbar button's badge and popup.
- Search when adding an app or folder: the app picker.
- Searchable lists: website lists, apps, window titles and clusters.
- The suggested projects (open-turkey, Focuser, CtblPlusPlus, not-for-you): what was taken and what was left out is at the end of `docs/PLAN.md`.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
