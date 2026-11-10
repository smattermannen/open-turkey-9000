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
- Link recording: start recording, then every link opened in the browser is added to a list until recording is stopped. Makes allowlisting school sites with many verification steps easy.
- Clusters: lists made independently of blocks, with all the same features as lists. A cluster can be imported into a block's list or added as a standalone list within a block, so a new block can quickly allowlist e.g. the academia or anime cluster.
- Keyword test mode: turn it on for a block, then browse; the extension shows the keyword matches on each page (which keywords matched, how often each appears, and how many distinct keywords), so it is easy to see whether a rule would be too harsh.
- Keyword thresholds by frequency (total matches), not only by distinct keywords.
- Workaround guard (built in, no per-mirror domain lists): treat any URL that carries a blocked domain inside it as blocked (translate proxies like `www-youtube-com.translate.goog`, `web.archive.org/web/*/youtube.com`, `?url=` web proxies, caches); spot front-ends and mirrors by page fingerprints (generator meta tags, canonical/og:url and embeds pointing at a blocked site, known front-end markup such as Invidious/Piped); kill any process whose command line names a blocked domain (yt-dlp, mpv, curl, streamlink), plus a built-in list of download/stream tools in strict mode.
- App blocking by more than exact name: match the executable's path and hash, so renaming a binary doesn't escape.
- Python programs: stop sparing every Python process; spare only scripts that are root-owned in system paths (desktop helpers) and judge user-owned scripts like any other program.
- Wayland support for the window-title check (Cinnamon's Wayland session is still experimental, so low priority).
- Close local paths to root: hide or lock recovery mode and `init=`/`single` kernel arguments (check what the no-escape boot lock already covers), lock root login on TTYs and `su`, and warn about firmware password and boot order (live USB).
- Auto-relock when setting a lock: e.g. after the typing challenge the block pauses for 5 minutes without becoming editable, then relocks by itself.
- Location-based blocking: by GPS, by nearby Wi-Fi networks, or both (the user picks).
- Project export/import: the whole setup (blocks, clusters, schedules, keywords, bypass rules, browser options) in one file, to share systems with others.
- Bypass detection: the user picks which detectors run and what each does, with an escalation ladder customizable in detail, e.g. 1st trigger: close the tab and sign out for 1 minute; 2nd: close the browser and start monk mode for 5 minutes; 3rd: shut down for 15 minutes. Ties in with Nepsis mode.
- Remove every note saying some parts only work with the extension: it is forced by default, so they no longer apply (interface, README, command help).

## Later
- Count quota time only while a block's sites or apps are in use, measured by the extension (its statistics sampling in background.js can be reused) and the daemon's foreground-app sampling. Watch for loopholes: a muted video beside another window, and browsers without the extension while a session is open.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
