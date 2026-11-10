# KC9K status

## Note to the next session (2026-11-10)
Read this first, then `git fetch origin`: other sessions push TODO ideas straight to GitHub.
The order of work for everything below is in `docs/PLAN.md` (agreed with the user 2026-11-10: do all of it, any order).

- Everything here is committed and pushed, and installed on this machine (the test laptop) with `TAGS=devmaster`, extension 1.13.0. `ci/browser_test.py` (20) and `ci/extras_test.py` (14) pass in a real LibreWolf after the last install.
- Done in the session before this one: website lists, more actions, monk mode, events and bypass detection with an escalation ladder, quota spent only while in use, GNOME hotkeys, hidden/searchable lists in the interface, tamper events. Real-browser scripts: `ci/browser_test.py`, `ci/actions_test.py`, `ci/quota_test.py` (run with `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest ~/.local/share/kc9k-browsertest/venv/bin/python ci/...`).
- Done this session (2026-11-10, afternoon): the ladder tried in a real browser; a `search` detector (a blocked page opened from a search engine's results); closed browsers and tampering now count for blocks' own bypass detection; bypass detection starts fresh when switched on; a quota block no longer flips off and on every cycle; clusters (`kc9k cluster ...`); link recording (`kc9k record ...`).
- Agreed with the user: Linux Mint Cinnamon on X11 only (no Wayland). Interface work waits until the functionality is pinned down; build daemon/CLI/extension first. The interface has nothing for clusters or recording yet (CLI only), and shows only a ladder's first step.
- Agreed order for "Next": 1) bypass detection (left: the workaround guard feeding it; per-detector actions, if wanted, would mean one ladder per detector), 2) clusters and link recording: done, 3) keyword test mode (show which keywords matched, how often, how many distinct, against each action's thresholds), 4) workaround guard and app matching by path/hash, 5) auto-relock, 6) project export/import (now that clusters exist; include them), 7) new list actions (audio, video, declutter, dark mode), then social media settings (their own interface tab), 8) location (Wi-Fi now; GPS only with real hardware), 9) root-path hardening, interface design pass.
- Left on the test machine: block `quotatest` (has a quota, so it stays on until midnight; it only blocks addresses with "kc9kquota"; delete it after midnight), `monk-mode` (kept on purpose), `hej` (an empty block of unknown origin, created 12:41; ask the user). The locked block `test` got a blacklist `Papers` (arxiv.org, scholar.google.com, jstor.org) while trying `cluster copy`; it can't be removed while `test` is locked: after `kc9k unlock test`, run `kc9k list delete test Papers`.
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
Supported: Linux Mint Cinnamon on X11 only (Mint's Wayland session is experimental; window titles, app statistics and quota counting rely on X11).

Interface work waits until the functionality below is pinned down: build each feature in the daemon, CLI and extension first.

- The interface looks rough throughout and needs a design pass. (Its window now fits small and scaled screens: the block details scroll.)
- Bypass detection (done: the ladder, detector choice with `search`, quiet fall-back, the warning action, browsers and tampering counted per block). Left: what each detector does on its own (one ladder per detector, if wanted), and the workaround guard feeding it.
- Workaround guard (built in, no per-mirror domain lists): treat any URL that carries a blocked domain inside it as blocked (translate proxies and web archives are done; still: `?url=` web proxies, caches); spot front-ends and mirrors by page fingerprints (generator meta tags, canonical/og:url and embeds pointing at a blocked site, known front-end markup such as Invidious/Piped); kill any process whose command line names a blocked domain (yt-dlp, mpv, curl, streamlink), plus a built-in list of download/stream tools in strict mode. Detections feed bypass detection.
- App blocking by more than exact name: match the executable's path and hash, so renaming a binary doesn't escape.
- Python programs: stop sparing every Python process; spare only scripts that are root-owned in system paths (desktop helpers) and judge user-owned scripts like any other program.
- Close local paths to root: hide or lock recovery mode and `init=`/`single` kernel arguments (check what the no-escape boot lock already covers), lock root login on TTYs and `su`, and warn about firmware password and boot order (live USB).
- Clusters and link recording are done in the CLI (`kc9k cluster`, `kc9k record`); the interface has nothing for them yet. Possible later: recording that adds to an unheld block's list as you go, so a sign-in flow gets through while the block is on.
- Keyword test mode: turn it on for a block, then browse; the extension shows on each page which keywords matched, how often each appears, and how many distinct keywords, against each action's thresholds (different words and hits), so it is easy to see whether a rule would be too harsh. (Thresholds by frequency are done: `kc9k list hits`.)
- Auto-relock, chosen when setting a lock: e.g. the typing challenge lifts the block for 5 minutes (a break, like Pomodoro's), editing it stays refused, then it locks again by itself.
- Location-based activation: a block (or monk mode) starts when the computer is at a place, sensed by nearby Wi-Fi networks (nmcli scan: SSIDs/BSSIDs, e.g. the neighbours' networks) and/or GPS; the user picks which, or both. Locking such a block forces Wi-Fi on and guards against GPS tampering (no fix or a jump counts as being there, or as a bypass event). Note: most laptops have no GPS receiver, and geoclue's network location service (Mozilla's) is gone, so GPS needs real hardware.
- Project export and import: the whole setup (every block with its lists, apps, titles, schedules, quota, Pomodoro and bypass detection; clusters; browser options; freeze schedules; hotkeys) in one file, to share setups with the community. Locks and passwords are never exported; importing never weakens anything that is held.
- More list actions, ticked like the others: block audio (mute the page's sound), block video (stop and hide videos), declutter (experimental: render the page bare-bones, reader-style), force dark mode (Dark Reader's engine, MIT licensed, bundled and configured from KC9K).
- Social media settings (their own tab in the interface): per major site, presets such as "DMs only", "No reels/shorts", "No feed", "No discover/explore page", enforced by redirecting addresses (and hiding elements where needed), e.g. YouTube, Instagram, Facebook, X, Reddit, TikTok.

## Ideas from Cold Turkey users (added 2026-11-10)
Things that work badly in Cold Turkey Blocker on Windows, or that people asked for. Not ordered or agreed yet.

- Allow one YouTube playlist while YouTube is blocked. An exception list on the address (`list=<id>`) only covers the playlist page and videos opened with that parameter; a video opened without it, or after YouTube changes the address without loading, falls outside. Needs the extension to check that the playing video belongs to the playlist. Fits the social media settings.
- Daylight saving time: Cold Turkey has bugs here. Schedules compare the wall clock (`schedule.go`), which should hold, but nothing tests it. Add tests across both changes for schedule windows, `EditFreeze`, `--until HH:MM`, time locks, the quota day turning at midnight, Pomodoro phases and scheduled Frozen Chicken.
- Breaks: Pomodoro and auto-relock (above) cover part of it. Open: breaks you take when you choose, from an allowance (e.g. 10 minutes per hour of blocking).
- Time left in the extension: a badge on the toolbar button showing how long until the site in the tab is blocked (quota left in an open session, time until a schedule starts, break left). During a break the badge shows the break remaining for the block the site is spending it from. The extension has no badge yet; `rules.quota` already lists the session's blocks.
- Search when adding an app or folder: the interface takes a typed name (`add-app`). Offer a searchable picker of installed apps (`.desktop` files, Flatpak IDs) and running processes.
- Searchable lists: done for website lists in the interface ("Find in this list"). Still missing for apps, folders, window titles and clusters, and in the CLI (`list show --find`).
- Projects to read for ideas: open-turkey and Focuser, and the GitHub topics `distraction-blocker` and `cold-turkey-alternative` (the second has one Linux project so far).

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
