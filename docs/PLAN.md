# KC9K implementation plan (2026-11-10)

Covers everything in `TODO.md`, the ideas from Cold Turkey users, what the
suggested projects do that KC9K doesn't, and the new access passwords.
`docs/practicalities.md` keeps the design notes per feature; this file is the
order of work and what each step consists of.

## Rules for every step

- Order inside a step: database, then `enforce`/daemon, then CLI, then
  extension, then interface. The interface for a feature is built in phase 9
  unless the feature is nothing without it (access passwords, the badge).
- Anything that weakens a held block goes through `gateEdit`/`ensureEditable`
  and the settings password. Anything that strengthens is free.
- Each step ends with: Go tests, a check in `ci/browser_test.py` or
  `ci/actions_test.py` when the extension is involved, a README section,
  the CLAUDE.md summary line, and the item struck from `TODO.md`.
- Extension changes bump the version in `manifest.json`.
- Supported: Linux Mint Cinnamon on X11, LibreWolf. No Wayland, no Chromium
  extension.

Sizes: S = under a day of work, M = a few days, L = a week or more.

## Phase 1: finish what is in flight

**1.1 Keyword test mode (M). Done 2026-11-10** (`kc9k list test`, extension 1.13.0, tried in LibreWolf). Next in the agreed order.
- `kc9k list test <block> on|off` sets a flag per block; `rules` carries
  `test: true` for its lists.
- `match.js` already counts distinct entries and total hits per list; return
  the per-entry counts as well instead of only the totals.
- `content.js` draws a small panel on each page (shadow DOM, so the page's
  styles can't touch it): each matched keyword with its count, the number of
  distinct keywords, and per action the two thresholds with met / not met.
- In test mode the actions are shown, not run. So turning it on is a
  loosening edit (gated) for a held block.
- Test: a page with known words in `ci/actions_test.py`.

**1.2 Bypass detection leftovers (S).** The workaround guard feeds it
(2.1, a new detector `workaround`). One ladder per detector is not built:
a block's own ladder already gives a stricter track where one is wanted.

## Phase 2: holes in blocking

**2.1 Workaround guard (L). Extension part done 2026-11-10** (proxy addresses, canonical/front-end fingerprints, `workaround` events); command lines done too (`KillNaming`, a fixed list of fetch tools; the strict-mode "kill every download tool" idea was dropped: it would close players of local files). No per-mirror domain lists.
- URL carrying a blocked domain: extend the translate/archive decoder in
  `match.js` to any query parameter or path segment that parses as a URL or
  host (`?url=`, `?q=cache:`, base64 of a URL). Judge the inner address with
  the same lists.
- Page fingerprints, in `content.js` at document end: `link[rel=canonical]`,
  `og:url`, `meta[name=generator]`, iframes and video sources pointing at a
  blocked site, and a short built-in table of front-end markers (Invidious,
  Piped, Nitter, Redlib, Libreddit). A match is treated as the blocked site.
- Command lines: `blocker/process.go` reads `/proc/<pid>/cmdline`; a process
  whose arguments name a blocked domain is killed (yt-dlp, mpv, curl,
  streamlink). In strict mode a built-in list of download/stream tools is
  killed whenever a block with websites is active. Shells and editors are
  spared (a domain in a text file being edited is not a bypass).
- Each detection is an event of kind `workaround`, a detector for the ladder.

**2.2 Apps by path and hash (M). Done 2026-11-10** (tried live: a renamed copy is closed, the system program with the same contents is not).
- `apps` rows get optional `path` and `sha256`. `block add-app` given a path
  stores name, resolved path and hash. Matching: name, or `/proc/<pid>/exe`
  path, or hash of the executable (cached by inode + mtime + size).
- A renamed or copied binary is caught by hash. A rebuilt one is not; that is
  stated in the README.
- Window-title matching per app already exists (`titles.go`).

**2.3 Python programs (S). Done 2026-11-10** (tried live). Spare a Python process only when its script is
root-owned and under `/usr`, `/opt` or `/snap`; judge everything else by the
script's name and path like any program. Test with the Cinnamon applets that
made the exception necessary.

**2.4 Polite app closing (S, from Focuser). Done 2026-11-10** as the option `polite-close` (default on; not tried with a real app yet). For an app with a window, send
`_NET_CLOSE_WINDOW` first and kill after `TitleCloseGrace`, as titles do, so
the app can ask to save. Background processes are still killed at once. A
held block never waits longer than the grace period.

## Phase 3: time

**3.1 Daylight saving tests (S). Done 2026-11-10.** Tests in
`schedule/dst_test.go`, `freeze/dst_test.go`, `cli/until_test.go`
(Europe/Stockholm and America/New_York, both changes). Two bugs fixed:
`start --until HH:MM` for tomorrow was an hour off on the night of a change,
and a scheduled Frozen Chicken ended an hour late (spring) or early (autumn)
on the day of a change. The rule, now in the README: windows follow the
clock, so a window inside the skipped hour doesn't open that day, and one
inside the repeated hour is open both times. Time locks, Pomodoro, queued
edits and the edit freeze count real time and were already right; the quota
day follows the calendar date.

**3.2 Auto-relock (M). Done 2026-11-10** as `kc9k break relock <block> <D>` (a setting of the block rather than a `start` flag; a second unlock during the break ends the block). `start --relock 5m` with a lock: passing the
challenge (or giving the password) lifts the block for that long, editing
stays refused, then the daemon turns it back on with the same locks. Stored
on the block as `relock_minutes` and `relock_until`. A time lock is not
shortened by it.

**3.3 Breaks from an allowance (M). Done 2026-11-10** (`kc9k break allowance/start/end`). `kc9k break set <block> 10m --per 1h`
and `kc9k break start <block>`: time earned while the block is on, spent
when you choose, capped at one allowance. Shares the "lifted until" field
with 3.2 and Pomodoro's break so the three never stack. Setting or raising
an allowance on a held block is gated.

**3.4 Pomodoro additions (S, from Focuser). Done 2026-11-10.** A long break after N work
periods, and quota sessions closed during a work period.

**3.5 Time left in the extension (M). Done 2026-11-10** (badge, popup, add-site; tried in LibreWolf through the popup's message, the badge itself not looked at).
- `rules` gains per block `ends_at` / `starts_at`: quota seconds left in an
  open session, the next schedule start within 2 h, break or relock end.
- `background.js` sets the toolbar badge for the focused tab: minutes left
  until the site in it is blocked; during a break, the break left for the
  block that site belongs to (the shortest when several). Colour turns at 5
  minutes. Needs `browser_action` in the manifest and the button pinned by
  policy.
- A popup on the button (from Focuser): the site, which blocks and lists
  contain it, time left, and "Add this site to…" (adding strengthens, so it
  is free). A red badge when the daemon doesn't answer.

## Phase 4: access passwords

**Done 2026-11-10** (CLI, interface, tests, README). Relock after idle
minutes was added later the same day (an interface preference). Not yet installed on the test laptop.

Optional passwords to open the interface and each main page. They can all be
the same or all differ.

- **Areas:** `app` (opening the interface at all), `dashboard`, `blocks`,
  `device`, `stats`, `settings`.
- **Storage:** setting `access.password.<area>`, salted hash, same scheme as
  the settings password (`db.CheckPassword`). The database is root-only, so
  the hashes can't be read by the user.
- **CLI:** `kc9k access set <area…|--all>` (one password for all named
  areas), `kc9k access remove <area…>`, `kc9k access list`. Changing or
  removing asks for that area's current password, and for the settings
  password when one is set.
- **Enforcement in the CLI, not in the interface.** The interface runs as the
  user, so a check in Python alone could be edited out. `kc9k gui-state` and
  `kc9k stats --json` take `--area` and read passwords from standard input
  (`stdinLines`); without the right one they return only which areas are
  locked. `gui-state` is split so each page gets only its own data.
- **Interface:** a password page before the window's content when `app` is
  set; a cover with a password box over each locked page. A password that is
  entered is tried on every locked area, so equal passwords open together.
  Remembered until the window closes, a "Lock" button, and optional relock
  after N idle minutes.
- **Wrong password:** one second delay, and an event of kind `access` (shown
  in the event log; can be a ladder detector).
- **Terminal:** users work in the interface; the commands exist because the
  interface runs them. Every command that shows data asks for the password
  of the page it belongs to (`status` → dashboard; `block list/info`,
  `list show`, `cluster show`, `schedule list`, `pending`, `nepsis` →
  blocks; `freeze` listings → device; `stats` → stats; `browser`,
  `hotkey list` → settings). There is no switch to turn this off. Commands
  that change things are unchanged (settings password).
- **Lost password:** removed by root the same way as the settings password,
  only while nothing is locked.
- **Not the same thing as the settings password:** access passwords decide
  who can look; the settings password decides who can loosen. Both can be
  set to the same text.

## Phase 5: project export and import (M)

**Done 2026-11-10** (hotkeys are left out: each needs the settings password
and a desktop to bind on). No `--queue`: what exists is simply left alone.

`kc9k project export <file>` / `import <file> [--dry-run]`: one versioned
JSON file with every block (lists, apps, titles, schedules, quota, Pomodoro,
breaks, bypass detection), clusters, browser options, freeze schedules,
hotkeys, social media settings. Locks, passwords and statistics are never
exported. Import adds and strengthens freely; anything that would weaken a
held block or setting is listed and skipped (or queued with `--queue`).
Images are embedded base64.

## Phase 6: what the extension does to pages

**6.1 More list actions (L). Done 2026-11-10**, extension 1.14.0, without third-party code: `declutter` and `dark` are CSS only (no Readability, no Dark Reader), which is cruder but adds no bundled library. Ticked like the others in `db/actions.go`:
- `mute`: `tabs.update({muted:true})`, re-muted when the user unmutes.
- `no-video`: pause and hide `video` elements, block media requests.
- `declutter` (experimental): Mozilla Readability (Apache 2.0) on the page.
- `dark`: Dark Reader's engine (MIT) bundled, switched per page from rules.
Each is one module loaded by `content.js` only when a rule needs it.

**6.2 Social media settings (L). Done 2026-11-10** (`kc9k social`; 18 presets; selectors are in `social.js`, shipped with the extension rather than updatable on their own). Per site, presets enforced by redirects
plus element hiding: YouTube (no Shorts, no home feed, no recommendations,
no autoplay, no comments), Instagram (DMs only, no Reels, no Explore),
Facebook (no feed, no Reels), X (Following only, no trends), Reddit (no
home feed, no promoted), TikTok (Following only), LinkedIn (Recent sort, no
news), Threads (Following only). Stored per block as
`social.<site>.<preset>`, so locks, schedules and gating apply as for lists.
Selectors live in one table with a version, updatable without a new
extension. `not-for-you` is the reference for redirect targets.

**6.3 YouTube playlist exception (M). Done 2026-11-10** as `kc9k social <block> playlist <id> on`. `kc9k list add <block> <exceptions>
youtube-playlist:<id>`. The extension allows `/playlist?list=<id>` and a
`/watch` page only while `list=<id>` is in the address and the video is one
of the playlist's (read from the playlist panel on the page, cached). A
watch page that loses the parameter is blocked. Hosts/firewall/policy are
already skipped for a block with such exceptions.

**6.4 Block page (S, from Focuser). Done 2026-11-10** as the option `block-page-details`. Show which block and list stopped the
page, the matched entry (hidden lists show nothing), and when the block ends
if it has an end.

## Phase 7: location (L)

**Done 2026-11-10**, Wi-Fi only (GPS dropped, as agreed): `kc9k place`.
Tried against the real Wi-Fi scan on a scratch database; the daemon turning a
block on and off when moving between places has not been tried.

Wi-Fi first: `kc9k place add <name>` records the networks in range (`nmcli`
BSSIDs); a place matches when enough of them are seen. `block place <block>
<name>` or `monk --at <name>` starts it there. Locking such a block keeps
Wi-Fi on (rfkill and NetworkManager guarded); no scan result counts as being
there. GPS only behind an interface that real hardware can fill later
(gpsd); not built until there is a receiver to test with.

## Phase 8: hardening

**8.1 Local paths to root (M). Not done.** It changes boot and login configuration (GRUB entries, PAM, TTY logins), which can lock the owner out and cannot be tried safely on a machine in daily use. It needs a disposable machine or VM. Original note: First audit what no-escape's boot lock
covers. Then: recovery entries hidden and `init=`/`single` editing locked
(GRUB superuser already there), root login on TTYs and `su` locked during a
lock, and `strict on` warning about firmware password and boot order.

**8.2 Loosening delay per block (S, from CtblPlusPlus). Done 2026-11-10** (`kc9k pending delay <block> <D>`). A block's own
delay instead of the fixed 24 h (`LoosenDelay`); lowering it is itself a
queued edit.

## Phase 9: interface

**Mostly done 2026-11-10**: Clusters page with link recording, the whole
bypass ladder in an editor, a searchable app picker (installed apps and
running programs) plus folder and program-file choosers, schedule presets
and hours per week, a template choice for new blocks (14 ready-made lists),
pages for breaks, social media settings, places, options, access passwords
and project export/import. Apps, window titles, website lists and clusters
all have a search box. **Not done:** the design pass over every page (it
needs eyes on a real screen), and the tray icon (left out for now: the
toolbar button already shows time left where it matters, and a tray icon
that is only there while the window is open adds little). Everything here
was driven headlessly against a scratch database, not looked at.

Done last, on top of settled functionality.

- Design pass over every page (spacing, type, colours from `Theme`).
- Clusters page and link recording (start/stop, target list or cluster).
- Bypass detection: all ladder steps and per-detector ladders, not just the
  first step.
- Search: the "Find in this list" box on apps, folders, window titles and
  clusters; `--find` on `list show`, `cluster show`, `block info`.
- Adding an app or folder: a searchable picker of installed apps (`.desktop`
  files with icons, Flatpak IDs) and running processes; folders through the
  file chooser with its own search.
- Schedule calendar (from Focuser): presets (work hours, evenings,
  weekends), click a day heading to fill it, hours covered per week.
- Pages for everything new: keyword test mode, breaks, auto-relock, social
  media settings, new actions, places, project export/import, access
  passwords.
- Tray icon (XApp status icon) with the active blocks and time left (nothing
  shown while the dashboard is password-locked).
- More ready-made lists when creating a block (the four templates grow to
  about a dozen categories).
- English only.

## Phase 10: trying by hand

**Open.** These need a person at the dual-boot laptop; nothing here was done.

The nine items under "Still to try" in `TODO.md` need a person at the dual
boot laptop: quota-end notification, scheduled freeze in logout and shutdown
modes, strict mode holes (`unshare -Urm`, `--user` Flatpak, Electron),
shutdown mode and the `kc9k.nofreeze` escape, page keywords over days and a
real hotkey press, YouTube address changes and the Websites tab on the real
screen, one-off logout/shutdown from a list and the Monk mode button, a
GNOME hotkey, quota by use with someone present. Each becomes a checklist
entry with the exact commands; what can be scripted goes into `ci/`.
Leftovers on the test machine (`quotatest`, `hej`, the `Papers` list in
`test`) are cleaned up here.

## Phase 11: release

**Open**, by design: the test laptop still runs the `devmaster` build.

Build without `TAGS=devmaster`, remove `ci/vm.sh`, rename the repository to
`kc9k`, remove `/etc/sudoers.d/99-claude-test` from the test laptop, README
pass, a tagged release.

## What the suggested projects add

Only what a user would notice as better was kept.

- **open-turkey:** nothing new. KC9K grew out of it and has all of it.
- **Focuser:** polite closing (2.4), Pomodoro long break (3.4), toolbar
  popup (3.5), block page details (6.4), schedule presets, tray, ready-made
  lists (9).
- **CtblPlusPlus:** loosening delay per block (8.2).
- **not-for-you:** the presets in 6.2.
- **freedxm, ulysse:** nothing KC9K lacks.
- **Left out as clutter or no gain:** blocked-attempt charts, statistics
  retention setting, delete-all-data, block folders, Help page, Pomodoro
  presets, translations, app allowlist, forced DNS and integrity checks
  (the hosts file already wins over DNS and the installed files are
  root-owned), a Chromium extension, Windows/macOS.

## Decided with the user (2026-11-10)

All phases are to be done; the order is free. English only. Access
passwords apply to terminal commands too.
