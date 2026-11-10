# KC9K status

## Note to the next session (2026-11-10)
Read this first, then `git fetch origin`: other sessions push TODO ideas straight to GitHub.

- Everything here is committed and pushed. Installed on this machine (it is the test laptop) up to extension 1.9.1, with `TAGS=devmaster`. The bypass-detection ladder below is committed but NOT installed or tried in a real browser yet: reinstall (`sudo env TAGS=devmaster ./install.sh`), then run `ci/actions_test.py` and try a ladder with `close-tab` by hand (the extension 1.10.0 closes the tab when `/hit` answers `close_tab`).
- Done this session: website lists (blacklists/exceptions ticking url/title/text, per-list actions, thresholds by different words and by hits), more actions (redirect, picture, close-tab, close-browser, logout, shutdown, monk), monk mode, events and bypass detection with an escalation ladder (`kc9k nepsis set --step ... --detect ... --quiet ...`), quota spent only while in use, GNOME hotkeys, hidden/searchable lists in the interface, tamper events. Real-browser scripts: `ci/browser_test.py`, `ci/actions_test.py`, `ci/quota_test.py` (run with `GECKOWEBDRIVER=~/.local/share/kc9k-browsertest ~/.local/share/kc9k-browsertest/venv/bin/python ci/...`).
- Agreed with the user: Linux Mint Cinnamon on X11 only (no Wayland). Interface work waits until the functionality is pinned down; build daemon/CLI/extension first. The interface only needs to keep showing things correctly (e.g. it shows a ladder's first step; the full ladder is CLI-only for now).
- Agreed order for "Next": 1) bypass detection (the ladder is done; left: new detectors such as a burst of blocked pages from search results, "browser closed" tied to a block, and the workaround guard feeding it), 2) clusters, then link recording into lists/clusters, 3) keyword test mode (show which keywords matched, how often, how many distinct, against each action's thresholds), 4) workaround guard and app matching by path/hash, 5) auto-relock, 6) project export/import (after clusters), 7) new list actions (audio, video, declutter, dark mode), then social media settings (their own interface tab), 8) location (Wi-Fi now; GPS only with real hardware), 9) root-path hardening, interface design pass.
- Left on the test machine: block `quotatest` (has a quota, so it stays on until midnight; it only blocks addresses with "kc9kquota"; delete it after midnight), `monk-mode` (kept on purpose), `hej` (an empty block of unknown origin, created 12:41; ask the user).


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
Supported: Linux Mint Cinnamon on X11 only (Mint's Wayland session is experimental; window titles, app statistics and quota counting rely on X11).

Interface work waits until the functionality below is pinned down: build each feature in the daemon, CLI and extension first.

- The interface looks rough throughout and needs a design pass. (Its window now fits small and scaled screens: the block details scroll.)
- Bypass detection, a full system (the ladder, detector choice and quiet fall-back are done: `kc9k nepsis set --step`; left below):
  - The user picks which detectors run (blocked pages, apps, window titles, browsers without the extension, refused loosening, daemon restarts, clock jumps, and new ones: e.g. a burst of blocked pages from search results; tie "other browser closed" to a block) and what each does.
  - An escalation ladder instead of one consequence, customizable in detail: each step has its own trigger (count within a window) and actions, e.g. 1) a warning: the tab closes and you are signed out for 1 minute; 2) the browser closes and monk mode runs for 5 minutes; 3) the computer shuts down for 15 minutes. The ladder falls back a step after a quiet time the user sets. Add a "warning" consequence (a notification).
- Workaround guard (built in, no per-mirror domain lists): treat any URL that carries a blocked domain inside it as blocked (translate proxies and web archives are done; still: `?url=` web proxies, caches); spot front-ends and mirrors by page fingerprints (generator meta tags, canonical/og:url and embeds pointing at a blocked site, known front-end markup such as Invidious/Piped); kill any process whose command line names a blocked domain (yt-dlp, mpv, curl, streamlink), plus a built-in list of download/stream tools in strict mode. Detections feed bypass detection.
- App blocking by more than exact name: match the executable's path and hash, so renaming a binary doesn't escape.
- Python programs: stop sparing every Python process; spare only scripts that are root-owned in system paths (desktop helpers) and judge user-owned scripts like any other program.
- Close local paths to root: hide or lock recovery mode and `init=`/`single` kernel arguments (check what the no-escape boot lock already covers), lock root login on TTYs and `su`, and warn about firmware password and boot order (live USB).
- Clusters: lists made independently of blocks, with all the same features as lists (where they look, entries, import/export). A cluster can be imported into a block's list or added as a standalone list within a block, so a new block can quickly allowlist e.g. the academia or anime cluster.
- Link recording: start recording, then every link opened in the browser is added to a list (or cluster) until recording is stopped. Makes allowlisting school sites with many verification steps easy.
- Keyword test mode: turn it on for a block, then browse; the extension shows on each page which keywords matched, how often each appears, and how many distinct keywords, against each action's thresholds (different words and hits), so it is easy to see whether a rule would be too harsh. (Thresholds by frequency are done: `kc9k list hits`.)
- Auto-relock, chosen when setting a lock: e.g. the typing challenge lifts the block for 5 minutes (a break, like Pomodoro's), editing it stays refused, then it locks again by itself.
- Location-based activation: a block (or monk mode) starts when the computer is at a place, sensed by nearby Wi-Fi networks (nmcli scan: SSIDs/BSSIDs, e.g. the neighbours' networks) and/or GPS; the user picks which, or both. Locking such a block forces Wi-Fi on and guards against GPS tampering (no fix or a jump counts as being there, or as a bypass event). Note: most laptops have no GPS receiver, and geoclue's network location service (Mozilla's) is gone, so GPS needs real hardware.
- Project export and import: the whole setup (every block with its lists, apps, titles, schedules, quota, Pomodoro and bypass detection; clusters; browser options; freeze schedules; hotkeys) in one file, to share setups with the community. Locks and passwords are never exported; importing never weakens anything that is held.
- More list actions, ticked like the others: block audio (mute the page's sound), block video (stop and hide videos), declutter (experimental: render the page bare-bones, reader-style), force dark mode (Dark Reader's engine, MIT licensed, bundled and configured from KC9K).
- Social media settings (their own tab in the interface): per major site, presets such as "DMs only", "No reels/shorts", "No feed", "No discover/explore page", enforced by redirecting addresses (and hiding elements where needed), e.g. YouTube, Instagram, Facebook, X, Reddit, TikTok.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
