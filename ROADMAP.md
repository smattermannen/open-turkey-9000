# KC9K Roadmap

Code status against `SPEC.md`. The only status file.

Rules: every item cites a § or rule ID (unagreed work goes in §5 first).
Update in the same commit as the code. A wrong status is fixed, not built
on; write "Unchecked" rather than guess. Order: §0 first, top to bottom,
then §2; Windows (§4) alongside them. Finished items are deleted (built
behaviour lives in `notes/`, history in git).

Status: Built, Partial, Differs (built unlike SPEC), Not built, Unchecked.
Sizes: S < 1 day, M days, L ≥ week.

## Index

One row per open item; find it with ``grep -n '`A3`' ROADMAP.md``. New
items get the next ID of their group and a row; a finished item loses its
row. Needs: **agent** (code, cloud or test machine); **box** (agent on the
test machine: root, X11, real browsers, screenshots); **vm** (box in a VM
with a snapshot: logout, shutdown, network cuts, strict mode, reboots);
**owner** (decision, verdict, real hardware). Windows: **win-ci**
(`windows-latest`), **win-vm** (Windows VM with a snapshot), **laptop**
(owner's Windows laptop: may break, factory reset acceptable, never trip
its bypass prevention).

| ID | Item | SPEC | Notes | Needs |
|---|---|---|---|---|
| A17 | Flaky CI checks fixed: Windows desktop check's lost key left | principle 9 | testing | win-ci |
| A1 | `no-images` GIFs: the owner's own site; pseudo-element GIFs left empty after address → text → unmatched page | §3.9 | extension | box, owner |
| A2 | Looks from page text: real use on the laptop | §3.9 | extension | owner |
| A4 | Brightness slider gone, night light off | §4.10 | circadian | box |
| A12 | R-LOCK-12: a person clicks Lock and Cancel | R-LOCK-12 | hotkeys, locks | owner |
| B5 | Rendered terms on real sites (box); hotkeys on a real keyboard (owner); GNOME (vm) | §3.1, §4.13 | extension, hotkeys | box, vm, owner |
| B8 | Use counting with a person; run `ci/lockedhint_check.sh` on the desktop | §3.7 | allowance | box |
| B11 | Social media settings beyond Shorts; playlists; places; text break | §3.2, §3.6 | extension, automation | box |
| B12 | Chrome: a person opens the popup and tries to remove the extension | §3.13 | browsers | owner |
| D3 | Instagram "Following only": logged in, no suggested posts | §3.2 | extension | owner |
| 2.1 | Circadian on a laptop backlight, ddcutil, several screens, GNOME/MATE | §4.10 | circadian | owner, vm |
| 2.2 | Ad trigger: gambling and dating domains seen live; by-page browser test | §3.9 | actions | agent, box |
| 2.4 | SafeSearch per engine in LibreWolf | §4.10 | browsers | box |
| 2.6 | PAM root guard (`ci/strict_pam_vm_test.sh`, never run); firmware lock on real firmware | §4.10 | device | vm, owner |
| 2.7 | Text lock's challenge window on a real desktop | R-LOCK-5, §4.6 | locks | box |
| 2.8 | Every Partial/Not built row of §1 | see §1 | see §1 | agent |
| 2.9 | Tamper settings: LibreWolf parts (box); `ci/tamper_vm_test.sh` (never run) | §4.10 | browsers, extension, install | box, vm |
| 2.10 | Cross-device sync (Supabase) | §4.14, §6 2026-11-23 | db-project | agent, owner |
| 2.11 | One-click installer: `.deb` built (clean Mint VM left), MSI; Android open | §4.11, §6 2026-11-23, §7 | install, install-windows | agent, vm, owner |
| 2.12 | Chrome, Edge, Firefox, Bing as supported browsers everywhere (Windows: waiting for 2.17 and Mozilla's signature) | §3.13, §4.10 | browsers, windows-layers | agent, owner, win-vm |
| 2.13 | Accountability partner | §4.15 | — | agent |
| 2.14 | Website | §4.16 | — | agent, owner |
| 2.15 | Search results from blacklisted addresses: the engines' real pages | R-PAGE-2 | extension | box |
| 2.16 | "Show dark web pages while dark mode is on": real sites, LibreWolf's window and Chrome on the desktop | §4.10 | circadian, browsers | box |
| 2.17 | Chrome extension in the Chrome Web Store (unlisted) | §3.13, §6 2026-11-21 | browsers, windows-layers | owner, agent |
| C1 | Build for the brothers' computers with `TAGS=devmaster` | §4.11 | install | agent |
| C2 | Remove test helpers | §4.11 | install | agent |
| C3 | Android: tapping jumps to the top; app works like the computer's GUI | R-GUI-3 | android | agent |
| C4 | Android places: map and GPS instead of Wi-Fi | §3 Place | android, automation | agent |
| N0 | Android plan (SPEC §8) agreed; §7's Android questions | §8, §7 | — | owner |
| N1 | Android foundation | §8.1, R-ANDROID-3 | android/CLAUDE.md | agent |
| N2 | Android blocks, lists, extension, VPN layer, apps | §8.2, §8.3 | android/CLAUDE.md | agent, owner |
| N3 | Android locks and passwords | §8.3, R-LOCK-1–12 | android/CLAUDE.md | agent, owner |
| N4 | Android automation, places, modes, monk mode, Frozen Chicken | §8.3 | android/CLAUDE.md | agent, owner |
| N5 | Android breaks, allowance, statistics, notifications | §8.3 | android/CLAUDE.md | agent, owner |
| N6 | Android actions, bypass prevention, presets, recording, projects | §8.3 | android/CLAUDE.md | agent, owner |
| N7 | Android protection, guided setup, circadian rhythm, accessibility layer | §8.4, R-ANDROID-4 | android/CLAUDE.md | agent, owner |
| N8 | Android wide screens, accessibility, three phones end to end | §8.5, R-ANDROID-1 | android/CLAUDE.md | owner |
| W1 | Windows service and agent: the console's raw mode | §3.13 | windows | win-vm |
| W2 | Windows web layers: real browsers obeying; Chrome Web Store ID | §3.13, §4.10 | windows-layers | win-vm, laptop, owner |
| W3 | Windows apps and desktop: notifications, challenge window on a real keyboard, lock freeze | §3.1, §3.7, §3.8, §4.8, §4.13, R-LOCK-5 | windows-layers | win-vm, laptop |
| W4 | Windows GUI on a real desktop: Narrator, scaling, drag and drop | §4.1–4.13, R-GUI-1–3 | gui-windows | win-vm, laptop |
| W5 | Windows tamper protection: boot/Safe Mode | §3.10, §4.10 | windows-tamper | win-vm, laptop |
| W6 | Windows circadian: gamma, brightness, Night light | §4.10 | windows-layers | win-vm, laptop |
| W7 | Windows MSI: signing certificate, first tag, Apps & features by a person | §4.11 | install-windows | owner, win-vm |
| W10 | Places on Windows: real networks | §3.6, R-MODE-5 | windows-layers, automation | laptop |
| W13 | Recovery path on Windows: a real Safe Mode boot | R-ESCAPE-1 | windows-tamper, device | win-vm |
| W19 | Firmware lock on Windows: real firmware | §3.8, §4.10 | windows-tamper, device | laptop, owner |
| W24 | Strict mode: Sandbox, Hyper-V, WSL 2 | §3.8 | windows-tamper, device | win-vm |
| W25 | Strict mode's noexec on Windows 11 Home | §3.8 | windows-tamper, device | laptop |
| W50 | Freeze fixed, idle soak hard in CI; the laptop confirms it | principle 1 | windows, windows-layers | laptop |
| W55 | Windows service without its pipe after an upgrade ("service is not running") | principle 1, §4.11 | windows | agent |
| W54 | Windows layout as GTK: built, CI green; the owner's look on the laptop | §4.1–4.13 | gui-windows | owner |
| W55 | CI time: parallel Windows GUI jobs, concurrency, paths, caches | §5 | testing, gui-windows | agent, win-ci |
| A14 | Clip curves mirrored, tested in Go, C#, GTK; the owner's look on the laptop | §4.10 | circadian | owner |

## 0. Fix what exists

**Android is on hold until the Linux project is finished** (owner): its
plan is SPEC §8, its items §3.1.

- [ ] `A17` **Flaky CI checks** (found 2026-11-23 while fixing A16).
  Linux and Android part done: `ci/match_test.js` (fastest of 3 rounds)
  and Android `RulesServerTest` (port 0) passed 48 `Test` runs in a row
  (698–745); the runs' other failures before 698 were work in progress,
  each fixed by a later commit (none flaky). Left, Windows: seen (Windows run 215,
  no Windows change): `ci/windows_desktop_test.ps1`'s "the challenge typed
  in the window unlocks the block" got 50 % accuracy (a posted key lost),
  so `ci-lock` stayed locked and "uninstall" failed after it; passed in
  runs 211 and 213. Not fixed. Done when: each passes 10 runs in a row
  (`Test` dispatched by hand on the fix).

### 0.1 Code contradicting `SPEC.md`

- [ ] **Owner report 2026-11-20** (D3 left).
  - **D3** Instagram `instagram:following-only`: `/` → `/?variant=following`
    (`social.js` redirect with a stopping `query`; `ci/match_test.js`);
    checked on the real site logged out (no loop, SPA navigation too).
    Left (owner): logged in, the feed shows only accounts followed; nothing
    hides suggested units (CSS can't match their text).

- [ ] **Owner report 2026-11-15** (real use on the laptop; top priority).
  Check against the latest build before calling one fixed.
  - [ ] `A1` `no-images` doesn't hide GIFs (§3.9: animated GIFs included).
    Fixed and checked locally and on real GIF sites in LibreWolf
    (`<img>`, pseudo-elements, `image-set()`, shadow roots, late content;
    `ci/content_harness_test.js`, `ci/extras_test.py`). Open: seen twice,
    cause not found: after address-matched → text-matched → unmatched page
    of one local site (one GIF address, `no-store`), the unmatched page's
    pseudo-element GIFs stay empty (its `<img>` shows); text → unmatched
    and address → unmatched are fine. Left: the owner's own site (which?). S
  - [ ] `A2` Looks from page text (greyscale, no-images, no-video)
    unreliable and lifted for a moment on every click (§3.9). Causes fixed
    (one reset point `enterAddress`, looks carried until first read,
    user-origin sheet `syncLookCss`); `extras_test` and `browser_test` pass
    in LibreWolf on local pages. Left: real use on the laptop. S
  - [ ] `A4` Desktop brightness slider gone from the menu; blue-light filter
    toggles; desktop night light turns itself off while its switch shows
    on, also with circadian and strict mode off (§4.10: off touches
    nothing). Fixed: KC9K saves the ramp it set (`circadian.applied`), gives
    plain ramps back only if that one is still on, pauses on someone
    else's ramp (`TestCircadianGammaAfterRestart`); backlight never written
    with its clip off; strict lockdown's polkit rule now allows the
    backlight helper and colord. Left: the brightness slider gone with
    strict mode off is unexplained; confirm both on the laptop (`kc9k
    circadian`, `journalctl -u kc9k | grep circadian`, whether it stops
    with the daemon stopped). M

- [ ] `A12` **R-LOCK-12 gaps**: `kc9k monk`, `project import`, `block
  import` confirmed (`confirm_test.go`). The hotkey press dialog
  (`blocker.ConfirmDesktop`, zenity) ran live: times out after 60 s with
  nothing locked; synthetic keys ignored. Left: a person clicks Lock and
  Cancel on an unlocked screen. (S)

### 0.2 Not yet confirmed in real use

Script-tested, never by a person. Script what can be into `ci/`.

- [ ] `B5` Rendered terms on real sites for days; hotkey on a real keyboard; hotkey under GNOME.
- [ ] `B8` Use counting with a person: focused tab/app count, idle not;
  other browser closed during a website session. Idle comes from the X
  server first (`blocker/idle.go`; cinnamon-screensaver doesn't implement
  `GetSessionIdleTime`); screen locked from logind's `LockedHint`
  (`lockedByLogind`). Unchecked: that cinnamon-screensaver sets
  `LockedHint` (if not, a locked screen counts as there until idle,
  3 min): run `ci/lockedhint_check.sh` as yourself on the Cinnamon desktop
  (written, never run).
- [ ] `B11` Social media settings beyond Shorts (`social.js` selectors written from memory); playlist videos playing; block following places; text break returning.
- [ ] `B12` Chrome on the laptop. Policy install, pinning, no Remove,
  popup (1.27.6 `popupTab`) checked over CDP. Left (owner): a person
  clicks the button and tries to remove the extension.

**Built, never checked by a person or on a real desktop** (no item; check
when the laptop or box allows):

- GTK on a real screen: D1 (block bar stays on top), D2 (Start at once,
  then Add locks…), D4 (ClipEditor bend handles); owner reports
  2026-11-16/17: block delete, cards, place rules widget, drafts, modes,
  schedules and places pages, automation view, presets, mode run, Frozen
  Chicken mode, monk mode switch; E1's presets question at export; the
  draft's Breaks & allowance and Bypass prevention tabs; the tray on a
  real panel.
- Laptop: mode and circadian wallpapers across a real boot, sign-in and
  wake at two places; the "extension isn't running" dialog; any trigger
  to any action; custom actions opening on a real desktop; circadian
  timeline and Play the day on real hardware; notifications choice.
- Whether blocks with both allow windows and place rules exist (they now
  go off like the windows).
- LibreWolf: big-page reading in pieces; dark pages' "Exempt" popup (also
  Chrome); frames judged by their text (R-LIST-4/5); file pages (§3.1).
- `2.5` Layered locks: a person checking Add locks…, Extend timer… and
  the unlock flow.
- Affected users with a second account (box); autostart at sign-in and
  wake; strict use counting; strict exempt folders; `kc9k-bin recover`
  from a recovery boot; allowed apps only (vm: an apps freeze with an
  allowed app and its children).
- Android: map picker and `ExtensionGuard` on a phone.
- Known gap: a deferred SQLite transaction that reads then writes can
  still get SQLITE_BUSY at once beside the daemon's writes;
  `_txlock=immediate` would close it (changes every `Begin`).

## 1. `SPEC.md` coverage

Only rows not fully built.

| Item | Status | Notes |
|---|---|---|
| §5 R-LOCK-12 | Partial | `A12` |
| §4.11 One-click installer, every platform | Partial | `2.11` |
| §3.13 Chrome, Edge, Firefox, Bing on every platform | Partial | `2.12` |
| §4.14 Cross-device sync | Partial | `2.10` |
| §4.15 Accountability partner | Not built | `2.13` |
| §4.16 Website | Not built | `2.14` |

## 2. Next features (after §0)

- **2.1 Circadian rhythm** (§4.10): built; gamma, previews, night light
  hand-over and Play the day checked on the desktop (one screen, X11).
  Unchecked: laptop backlight, external screens by ddcutil, several
  screens, GNOME/MATE, dark mode and wallpapers there. Wayland: no gamma
  ramps. LibreWolf reports `prefers-color-scheme` light (see `2.16`).
- **2.2 Ad trigger** (M): built (scope `ads`, `db.AdIndustries`), checked
  against EasyList and other lists and a crawl of 60 real pages
  (`TestAdIndustriesSeenLive`); `ci/extras_test.py` passes in LibreWolf.
  Left: gambling and dating domains seen live (only list-named ones kept;
  their sites' ads come through general exchanges no industry can name);
  the by-page judging of extension 1.30.0 (`kc9kAdsByPage`) not run in a
  browser test.
- **2.4 SafeSearch** (S): a lock per engine, built; checked live in
  Chrome (each engine alone strict; hosts section written by the daemon's
  next tick, ~30 s). Unchecked (box): the same in LibreWolf.
- **2.6 Factory reset protection** (M): firmware lock
  (`/sys/class/firmware-attributes/`), installer media guard, PAM root
  guard (`login`, `su`, `su-l`, tested on generated files only), guided
  setup: built. Unchecked (vm): `ci/strict_pam_vm_test.sh host kc9k-vm`
  (written, never run: during a strict lock `su -` and a root console
  login refused, the user's own `su` works, all back after the lock or
  with `kc9k.nofreeze`). Unchecked (owner): the firmware lock on a real
  Dell, Lenovo or HP (vendors' names and password clearing untried).
- **2.7 Hardening** (M): text lock's challenge window (no terminal: the
  command opens its own window, stdin never read) checked on Xvfb
  (`TestChallengeWindowOnX`). Unchecked: on the owner's real desktop (opens
  from the GUI under sudo, keeps focus, real keyboard).
- **2.8** Everything Not built/Partial in §1.
- **2.9 Tamper settings live** (S): Chrome-family parts checked. Unchecked
  (box): `inactive-tabs` and `embedded` in LibreWolf. Unchecked (vm):
  `ci/tamper_vm_test.sh host kc9k-vm` (written, never run; needs Go in the
  VM): LightDM guest session gone with `guest` on; during a lock with
  `installer` on, `dpkg -i` of the same or an older `.deb` and
  `install.sh` refused, a newer `.deb` installs. A `.deb` built before the
  prerm `failed-upgrade` fix still gets in.
- **2.10 Cross-device sync** (L; §4.14; backend Supabase, SPEC §6
  2026-11-23): Partial: account, Upload all and Download all built
  against a fake (`notes/sync.md`), the GTK page too; Windows GUI,
  Android, the real project not yet. KC9K
  account and cloud (principle 6's one exception); Upload all / Download all on Linux, Windows, Android; one to
  one except §4.14's device-specific items. Download all replaces even
  while locked (an exception to R-WEAK-1: the weakening audit must name
  it). Project backup (`cli/projectbackup.go`) is nearest, but only adds.
  To do, owner:
  - [ ] Supabase account; one project (EU region), free plan to start.
  - [ ] Auth → Email on; own SMTP for real users (built-in sender allows
    only a few emails an hour).
  - [ ] Give the agent the project URL and the anon key (public, safe
    behind row-level security); never the service role key.
  - [ ] Decide the plan: free projects pause after a week without use
    (Pro ~25 USD/month doesn't); principle 6 means users never pay.
  To do, agent:
  - [x] Schema as SQL in the repo (`cloud/supabase/schema.sql`): private Storage
    bucket `setups`, one object `<user id>/setup.json.gz` per account; a
    `setups` row (upload time, device name, format version, size);
    row-level security: an account reads and writes only its own.
  - [x] Setup file: versioned JSON from the backup's code (`buildProject`,
    `syncSetup`), everything §4.14 copies (lock state included),
    device-specific items left out.
  - [x] `internal/cloud/`: Supabase Auth and Storage over plain HTTPS
    (`net/http`, no SDK); tokens root-only in the DB; fake server in tests.
  - [x] CLI: `kc9k account signup|login|logout|status`, `kc9k sync upload`
    (built, `sync_test.go`).
  - [x] CLI: `kc9k sync download [--dry-run] [--yes]` (shows upload time
    and device, asks first, locks confirmed per R-LOCK-12, device items
    kept). Differs: the clear is one transaction, the import after it is
    not (checked whole first).
  - [x] Weakening audit entry for `sync download`; `refused()` untouched.
  - [x] Tamper protection settings and the grace in the project file
    (backup and sync; a project import only turns them on).
  - [x] GTK: Account page (sign up, log in, log out, status, Upload all,
    Download all with its dry run and one confirmation;
    `ci/gui_account_smoke.py` against the fake).
  - [ ] Windows GUI: the same page (session B). Android with N-items.
  - [ ] Check against the real project: Linux → Windows → Linux round trip.
  - [x] Note `notes/sync.md`.
- **2.11 One-click installer** (M per platform; §4.11; methods SPEC §6
  2026-11-23): Partial (Linux built, Windows MSI waiting for W7 and
  `2.17`, Android open). Linux: the `.deb` (double-click opens Mint's
  package installer, one click Install). Windows: the signed MSI (W7).
  Both on GitHub Releases, later the website (`2.14`). Android: open (SPEC
  §7).
  Linux built (`notes/install.md`): the `.deb` does all `install.sh` does,
  LibreWolf by the daemon after install (retried every 30 min);
  `linux-release.yml` (tag → `.deb` + `.sha256` on the release, checked
  first); CI job `deb` runs `ci/deb_install_check.sh` on every push
  (green since run 684: daemon installs LibreWolf ~1 min after install);
  `linux-release.yml` run by hand green (`0.0.1-beta.1`). Unchecked: its
  publish step (first tag, owner).
  To do, agent (Linux):
  - [ ] Installed by double-click on a clean Mint VM: daemon running, GUI
    in the menu, LibreWolf and Chrome get the extension (vm):
    `MANUAL=1 ci/deb_vm_test.sh host` (written, never run; needs a clean
    Mint VM `kc9k-mint` with a snapshot `clean`, see `notes/testing.md`).
  To do, agent (Windows):
  - [ ] MSI sets up the extension in Chrome and Edge (needs `2.17`'s ID).
  To do, owner:
  - [ ] Windows signing certificate (Azure Artifact Signing, or an OV
    certificate); unsigned, SmartScreen warns "unknown publisher".
  - [ ] First release tag; install both by a person (W7, C1).
  - [ ] Android: answer SPEC §7 "Android one-click installer".
- **2.12 Chrome, Edge, Firefox, Bing everywhere** (L; §3.13): Partial.
  Built: Chrome, Chromium on Linux. Windows (`notes/windows-layers.md`):
  Chrome and Edge waiting only for the store ID (2.17); Firefox (Program
  Files and the Microsoft Store's) recognised, given policies and guarded,
  waiting only for a Mozilla-signed `.xpi` (`webext/signed/`, SPEC §7
  "Signing the extension for Firefox", owner); Bing is Edge there (§5);
  gui-state `waiting` says which. Not built: Firefox and Edge on Linux,
  every browser on Android (IronFox only), the Bing app on Android.
  Unchecked (win-vm, `ci/windows_vm_test.ps1`): Firefox and the Store's
  Firefox obeying, the guard closing a waiting Firefox.
- **2.17 Chrome Web Store listing** (S agent, owner's review wait;
  §3.13, SPEC §6 2026-11-21): Partial. Built (`notes/browsers.md`
  Chrome): `make chrome-store-zip`, listing texts and privacy policy
  (`internal/webext/store/`), small tile and four screenshots. Chrome and
  Edge on Windows force-install only store extensions
  (`blocker.ChromeStoreExtensionID` empty: they get none, W2). Linux keeps
  its own signed `.crx` from the daemon.
  To do, agent:
  - [ ] With the ID: set `ChromeStoreExtensionID`; release step uploading
    each new extension version (Chrome Web Store API; owner's OAuth client
    as GitHub secrets).
  To do, owner:
  - [ ] Chrome Web Store developer account (Google account with 2-step
    verification; one-time 5 USD).
  - [ ] Upload `make chrome-store-zip`'s `.zip`, visibility **Unlisted**,
    fill the listing and privacy practices from
    `internal/webext/store/listing.md`, pictures from that folder, a
    privacy policy URL (`privacy-policy.md` published), submit (review
    days to weeks; access to all sites makes it slower).
  - [ ] Give the agent the extension ID.
  - [ ] Optional later: Edge Add-ons listing (free; Edge takes Chrome
    Web Store extensions by policy anyway).
  Done when: on the laptop Chrome and Edge get the extension by policy,
  pinned, the guard sees their check-ins (W2).
- **2.13 Accountability partner** (L; §4.15): Not built. Needs the KC9K
  account (`2.10`): invites, reports, alerts, partner holding password
  locks and the settings password, unlock approval; no screenshots.
- **2.14 Website** (M; §4.16): Not built. Content per §4.16; hosting the
  owner's choice when built.
- **2.15 Search results from blacklisted addresses** (M; R-PAGE-2): Built
  (`notes/extension.md` Search results, extension 1.37.0);
  `ci/search_results_test.js` (Chromium) and
  `ci/search_results_librewolf_test.py` (LibreWolf 160) pass on fixtures
  of 11 results pages. Left (box): each engine's real results page with a
  blacklisted site (markup drifts; DuckDuckGo Lite not covered).
  In `test.yml` (Chromium and LibreWolf, `notes/testing.md`).
- **2.16 Dark pages turn on the browser's dark mode** (S; §4.10): Built
  (`notes/circadian.md`); `ci/dark_browser_test.py` passes in LibreWolf
  160 headless (cloud: own dark design shown, light page turned over,
  light again with dark mode off; LibreWolf's window follows a dark
  desktop). Left (box): on Mint with KC9K's dark mode, LibreWolf's window
  and Chrome turn dark, and a real site with a dark design shows it in
  both (Chrome follows the desktop, not checked anywhere yet).

## 3. Testers and Android

When §0–§2 are done: the owner's brothers test KC9K while the Android app
is built.

- [ ] `C1` Build for the brothers' computers with `TAGS=devmaster` (the
  master password stays with the owner).
- [ ] `C2` Remove `ci/vm.sh`; on the laptop remove `/etc/sudoers.d/99-claude-test`.
- [ ] `C3` **Android app like the computer's GUI** (R-GUI-3): every tap
  scrolls the page back to the top; it must stay. Beyond that the app
  works as the computer's (same pages and behaviour). Done when: tapping
  on a long page keeps its scroll position (test), and pages match the
  GTK GUI's except where Android differs (`C4`).
- [ ] `C4` **Android places by map and GPS** (SPEC §3 Place): making a
  place opens the map (an area, found by GPS) instead of reading Wi-Fi.

### 3.1 The Android app (SPEC §8)

Not started: waits until Linux is finished. Each phase tested on
GrapheneOS, stock Android (Pixel), One UI (Samsung) (R-ANDROID-1); phone
tests need the owner.

- [ ] `N0` SPEC §8 agreed by the owner; §7's three Android questions
  answered (recovery, trusted Google account, Firefox signing). (S)
- [ ] `N1` Foundation: Kotlin core (model, command layer deciding all
  weakening, `Enforce`), SQLite in device-protected storage, cases shared
  with the Go tests (weakening audit, unlock order, schedules); Nokia
  mode and IronFox settings moved into blocks; Compose design layer and
  navigation (bottom bar / rail). Code words to the glossary
  (`whitelist`, `loosen`, `Option`). (L)
- [ ] `N2` Blocks, lists, phrase sets, social media settings, exceptions
  through the extension (IronFox, Firefox); VPN layer; apps (All apps,
  app exceptions, roles, certificates), app suspension and block page;
  unsupported browsers suspended. (L)
- [ ] `N3` Locks: every kind, layered, re-lock, extend, confirmation
  (R-LOCK-12), setting locks, settings/access/block passwords, text lock
  refusing multi-character input, trusted time. (M)
- [ ] `N4` Automation: schedules, places (Wi-Fi and areas), modes, monk
  mode (VPN drops all, Wi-Fi off), Frozen Chicken (lock screen, allowed
  apps only); week view and list editor. (L)
- [ ] `N5` Breaks, Pomodoro, allowance, reward, session limit, pause,
  autostart, use, statistics, notifications. (M)
- [ ] `N6` Actions and consequences, bypass prevention, custom actions,
  presets, link and word recording, projects (phone to phone). (M)
- [ ] `N7` Tamper protection, strict mode, guided setup, circadian rhythm
  and wallpapers, optional accessibility layer (screen text entries,
  R-ANDROID-4). (M)
- [ ] `N8` Wide screens (rail), TalkBack and keyboard, the three phones
  end to end. (M)

## 4. Windows

For an administrator account. Carries over: `cli`, `db`, `enforce`,
`schedule`, `quota`, `freeze`, `circadian`'s logic, the extension, the
rules server. Not: `internal/blocker/` (Linux parts), `internal/daemon/`,
`internal/lock/`'s Python window, the GTK GUI, `install.sh`, `packaging/`.
Rules unchanged: `enforce` decides, the CLI decides weakening, the GUI
decides nothing. Everything works on Windows Home (no Pro-only features;
policies written to the registry). Shape: `kc9k.exe` is the service
(LocalSystem) and the CLI, which forwards over a named pipe; the service
starts a session agent (`kc9k-agent.exe`) per session for titles, polite
close, hotkeys, notifications, idle, gamma, wallpapers and the text lock
window (notes/windows.md).

Two sessions: **Core** (all Go, `.github/workflows/windows.yml`, their
notes) and **GUI** (`windows/`, `windows-gui.yml`, `notes/gui-windows.md`,
`notes/install-windows.md`; GUI-only additive CLI JSON/commands in
`internal/cli/` with tests). The GUI only runs `kc9k.exe` and reads its
JSON; never the pipe, DB or registry. Shared files: each edits only its
own items. Land each step on `main`: rebase onto `origin/main`, run the
gate (Linux tests unchanged, Windows build), push; in a conflict keep both
sides. Neither touches the Linux GUI, extension or Android unless its item
needs it and the Linux tests prove nothing changed.

- [ ] `W1` Service and agent (M). Built, checked on `windows-latest`
  (`ci/windows_service_test.ps1`, `ci/windows_desktop_test.ps1`,
  including a command killed after its input ended). Left (win-vm,
  `ci/windows_vm_test.ps1`, a person): the console's raw mode while
  typing the challenge on a real keyboard, and a paste refused there.
- [ ] `W2` Web layers (L). Built: hosts, WFP, registry Chromium policies,
  Firefox-family `policies.json`, extension guard, browser recognition;
  WFP and policies checked in CI. Unchecked (win-vm, all in
  `ci/windows_vm_test.ps1`): real browsers obeying (Edge, LibreWolf,
  Firefox, the Store's Firefox), the guard closing a browser (W11's
  remote-control switches; a Firefox waiting for its extension), monk
  mode's cut and the network coming back. Needs owner: the Chrome Web Store listing's
  ID (`blocker.ChromeStoreExtensionID`); until then Chrome and Edge get no
  extension and are not supported browsers.
- [ ] `W3` Apps and desktop (L). Built; on `windows-latest`
  (`ci/windows_desktop_test.ps1`): apps and titles closed through the
  agent; the challenge window from `kc9k unlock` without a console (a
  `SendInput` key refused by Raw Input, posted keys unlocking, a killed
  unlock closing it); a hotkey press starting its block. Unchecked
  (win-vm, a person; `ci/windows_vm_test.ps1`): notifications seen, the
  challenge window typed on a real keyboard and the on-screen keyboard
  refused, the lock freeze (locking the runner's desktop would stop the
  job). A17's lost key (run 215): one race fixed (2026-11-23: a closed
  window's deferred clear could wipe the next window's state and handle);
  the check now prints each posted key's result. Cause not proven.
- [ ] `W4` GUI (L). C# WinUI 3, .NET LTS, unpackaged, self-contained, in
  `windows/gui/`; every GTK page ported and every GTK smoke test has a UI
  Automation counterpart (fake kc9k and real `kc9k.exe`). Checked on
  `windows-latest` (2026-11-23): drag and drop (block onto mode, a window
  drawn on the week, wallpaper bar, list grip), Windows' own open dialog,
  the tray menu (its message posted, not the icon clicked), Open at
  sign-in's shortcut, the challenge window opened by Unlock…, high
  contrast, the display at 150 % and 175 % (the runner's most; 200 % only
  as text size). Unchecked (win-vm, laptop): the display at 200 %, the
  tray icon clicked, the jump list from the taskbar, Narrator, a second
  screen, clip curves dragged.
- [ ] `W5` Tamper protection against an administrator (L). Built, checked
  on `windows-latest` (service can't be stopped/changed/deleted,
  watchdog task back, strict deny entries, uninstall refused while
  locked). Unchecked (win-vm, `ci/windows_vm_test.ps1 -Reboots`): Safe
  Mode boot, no-escape and the recovery environment on a real boot,
  takeown against strict mode, Reset this PC failing during a lock; W39's
  per-level claims rest on this. Firmware lock: `W19`.
- [ ] `W6` Circadian rhythm and wallpapers (M). Built; on
  `windows-latest`: dark mode, a wallpaper of the person's own reaching
  HKCU (copied to Program Files), the plain ramp back after circadian
  off. The runner's display driver refuses every gamma ramp (now said as
  such, not "operation completed successfully"), so gamma stays win-vm
  (`ci/windows_vm_test.ps1`: warm preview seen); panel brightness,
  external screens, Night light and its note (laptop).
- [ ] `W7` Installer and release (M). MSI (WiX 6) with service, agent,
  GUI; refuses removal, repair and downgrade while locked; upgrade over a
  lock, full-UI removal, release workflow (`windows-release.yml`) and
  throwaway-certificate signing checked in CI. Left (owner): the
  code signing, either way `sign.ps1` takes (a PFX in `SIGNING_PFX_BASE64`/
  `SIGNING_PFX_PASSWORD`, or an Azure Artifact Signing account in the
  `AZURE_SIGNING_*` and `AZURE_TENANT_ID`/`_CLIENT_ID`/`_CLIENT_SECRET`
  secrets; the Azure way built but never run with an account), and the
  first tag (`v0.1.0`). Unchecked (win-vm): install and removal from
  Apps & features by a person.

### 4.1 Windows GUI: parity and release (W30–W49)

Every `kc9k` command and `gui-state` field the GTK GUI uses, the Windows
GUI uses too; gaps are Windows-only behaviour and tests.

Laptop findings on finished items (2026-11-22, at 200 % scaling):

- Fixed in the tests (2026-11-22), to recheck at 200 % on the laptop:
  `HighContrast` reads and restores the owner's flags; scroll bar parts are
  skipped by `EveryControlHasANameAndTakesFocus`; the test process is
  per-monitor DPI aware (`GuiApp.Shot`, `WindowStateIsKept`).

- [ ] `W55` CI time, nothing dropped (M; §5 tests). After
  `ngr/modest-lovelace-l0zu0w` (adds `filter`, display passes) merges.
  Not built: (1) Windows GUI job split into parallel jobs (unit and
  build; smoke and parity; real CLI: `ParityRealTests`, `ImportTests`;
  geometry and accessibility incl. 150/175/200 %; desktop), GUI and fake
  built once and passed as artifacts; (2) `concurrency:` per branch with
  `cancel-in-progress: true` in every workflow; (3) tighter `paths:` so
  ROADMAP/notes-only commits start no Windows GUI run; (4) `setup-go` /
  `setup-dotnet` caching. Needs: agent, win-ci.

### 4.2 Windows Core: parity audit (W10–W27)

Not holes, so no item: a system proxy or VPN, Windows' DNS over HTTPS,
WebView2 apps (SPEC §6 2026-11-22). Unchecked parts of finished items:
W11 a real browser closed for remote-control switches (W2's win-vm run);
W26 the Store's Firefox obeying the registry policies (win-vm).

- [ ] `W10` Places on Windows (M; §3.6, §4.5, R-MODE-5). Built (Native
  Wifi: `wifi_windows.go`, `winapi/wlan*.go`; Wi-Fi kept on as for monk
  mode); CI has no Wi-Fi ("nothing seen" path). Unchecked (laptop): real
  networks, the radio switched back on after "Wi-Fi off" and airplane
  mode; whether Windows 11 24H2 asks location permission of a service's
  scans.
- [ ] `W13` A recovery path on Windows (S; Principle 4, R-ESCAPE-1).
  Built: Safe Mode is the recovery boot (freeze and strict lockdown
  skipped, `kc9k recover` works, unless no-escape); with the recovery
  environment off and no no-escape, F8 (`bootmenupolicy Legacy`) keeps Safe
  Mode reachable. Unchecked (win-vm, `-Reboots`): a real Safe Mode boot,
  F8 while factory reset protection acts.
- [ ] `W19` Firmware lock on Windows (M; §3.8, §4.10). Built through
  vendors' WMI (Dell `root\dcim\sysman\biosattributes`, Lenovo `root\wmi`,
  HP `root\HP\InstrumentedBIOS`; settings first, password last over
  stdin, never stored; none found: "set it by hand"). Unchecked (laptop,
  owner): which vendor, and a trusted person to type and keep the
  password.
- [ ] `W24` Strict mode: VMs, WSL, Windows Sandbox, containers (S; §3.8).
  Built (`closeVirtualMachines`; in CI: a VM program, Docker's engine, a
  real WSL 1 Ubuntu closed and closed again when restarted). Unchecked
  (win-vm, `ci/windows_vm_test.ps1`): Windows Sandbox, a Hyper-V VM, WSL 2.
- [ ] `W25` Strict mode's noexec on Windows (M; §3.8). Built: Software
  Restriction Policies in the registry (profile folders but not AppData,
  temp folders, removable drives; exempt folders unrestricted); processes
  keep the policy they read, so `CloseNoexecPrograms` closes what older
  shells start. Unchecked (laptop): Windows 11 Home, Explorer started
  before strict mode.

- [ ] `W55` **Service runs without its pipe** (Core; principle 1, §4.11;
  found by the GUI session 2026-11-23, windows-gui run 75's `installer`
  job). After `test-msi.ps1`'s upgrade over a lock the service was Running
  but every forwarded command (`version`, `gui-state`) said "the Killer
  Chicken 9000 service is not running" (`ERROR_FILE_NOT_FOUND`), while
  SYSTEM's `kc9k uninstall` (local) still read the lock. Cause in the code:
  `winsvc.Serve` returns on its first error (`Listen`, or
  `FILE_FLAG_FIRST_PIPE_INSTANCE` refused while the old process's pipe is
  still there) and `kc9kService.Execute` only logs it, so the pipe never
  comes back for the service's life; `Accept` errors other than access
  denied also loop without a pause. The GUI then shows only the banner.
  Done when: Serve retries (with a pause) until stop, and the installer
  job's "kc9k version says 0.2.0" passes 10 runs in a row. (S)

Never on a computer in use; the laptop runs only what is marked laptop.
win-vm (Windows 11, snapshot, admin): `pwsh ci/windows_vm_test.ps1
kc9k.exe`, then `-Reboots` from a fresh snapshot.

### 4.3 Owner report 2026-11-22: Windows GUI on the laptop

The owner ran the Windows GUI on the laptop and removed it ("it was
horrible"): looks better than GTK, but the GTK layout is better.
Stability first, then layout, then import. Order: top to bottom.

Still open from the report (built and in CI; what was built is in
`notes/windows*.md` and `notes/gui-windows.md`):

- `W50` **The whole computer froze** (principle 1): fixed (the extension
  guard closed System32 programs). Idle (6 min, no PowerShell started by
  the service or its commands, no policy refresh) is a failing check in
  `windows.yml`, the trace proven to see PowerShell starts. Left: the
  laptop confirms with `ci/windows_soak_test.ps1` (idle: 0 PowerShell).
- `W54` **Page layout as in the GTK GUI** (§4.1–4.13, R-GUI-3): built.
  Left: the owner's look on the laptop.
- Stability (2026-11-23, `StabilityTests`): a slow, hung or missing
  kc9k.exe never stops the window; dialogs one at a time; refusals shown;
  Enter in every dialog. Left: the owner's use on the laptop.
- `A14` **Automation clip curves bend one way only** (§4.10): one mirrored
  curve in Go, C#, GTK. Left: the owner's look on the laptop.

## 5. Proposed, not agreed

- DECIDED (By claude) 2026-11-23 (`2.10`): every tamper protection
  setting and the extension's grace are in the project file. A project
  import (a backup too) only turns protection on and the grace only
  shorter, saying what it leaves (turning off is weakening, `kc9k tamper
  X off`); Download all writes them as the file has them (§4.14 one to
  one). Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`A18`): list entries get `kc9k list
  change B L OLD NEW`, and a draft's save pairs an edited entry like a
  phrase. "Clearly matches more" (R-WEAK-5) is read narrowly: plain sites
  and addresses only (no `*` patterns, rendered terms or playlists); the
  same site or a parent domain with no path (a parent domain only in lists
  that read only the address or ad networks), or a path cut at `/`, `?`,
  `#` or `&`. For an exception list the direction flips (matching less is
  free). Anything else is gated as a removal. Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`2.12`): the Windows GUI shows every
  browser `gui-state.browsers` lists, with its extension's state and
  Install when not on the computer; "found, no extension yet" says what
  it waits for (`waiting`). Bing on Windows is Edge (decision
  below); Firefox's `waiting` reason is shown as Core sends it. Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`W4`): what `windows-latest` can't show
  (200 % display, a person clicking the tray icon) stays on the laptop
  list; CI proves 150/175 % and 200 % text size instead. Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`2.10`, owner away, asked the agent to
  decide): Download all refuses while a lock runs on this device that
  didn't come by an earlier Download all from the same account (unchanged
  since), naming it. SPEC §4.14 says Download all is "not refused by a
  lock"; taken literally, an empty setup uploaded from a second account
  would lift every lock here. §4.14's own example (unlocked on the
  uploading device, then downloaded) still works. Owner: keep, or loosen.
- DECIDED (By claude) 2026-11-23 (`2.12`): on Windows the Microsoft Bing
  app is Edge: Microsoft ships no separate Bing browser for Windows (Bing's
  app there is a web app in `msedge.exe`), so Edge's policies, extension
  and guard cover it; no own entry. Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`2.12`): release Firefox (Windows; later
  Linux) is a supported browser once a Mozilla-signed `.xpi` of the
  current version is committed in `internal/webext/signed/` (checked to be
  Mozilla's signature over exactly the extension's files); every
  Firefox-family browser then gets that file. Until then Firefox is found,
  gets the policies and is guarded as unsupported (gui-state `waiting`:
  `mozilla-signature`). Signing itself stays SPEC §7 (owner's Mozilla
  account). Firefox installed per user in AppData gets the policies but
  is no supported browser (its folder is the user's to change); Program
  Files and the Microsoft Store's are. Owner may overturn.

- DECIDED (By claude) 2026-11-23 (`2.15`): supported engines = Google,
  Bing, DuckDuckGo (and its HTML version), Brave Search, Yahoo, Startpage,
  Ecosia, Yandex, Mojeek, SearXNG: those KC9K already knows (SafeSearch,
  engines it can't keep strict); not DuckDuckGo Lite (a result spans
  several table rows). Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`2.15`): a result "leads to a
  blacklisted address" when its link, followed through the engine's
  redirect, an ad's click address or a web archive, goes where a visit is
  taken away by address alone (a page or device action, after the block's
  exceptions); ads count. Looks-only lists (greyscale, no images…) and
  lists whose block has title/text exceptions leave results alone (a visit
  isn't taken away, or can't be known from the address). The result is
  hidden, not deleted (the engine's scripts keep working): the largest
  part around the link holding no kept result and no search box. Owner may
  overturn.
- DECIDED (By claude) 2026-11-23 (`2.11`): the daemon makes sure of
  LibreWolf once per install, not once ever: `install.sh` and the `.deb`
  clear the marker at every install, as `install.sh` already tried
  LibreWolf at every run; a LibreWolf the user removes stays removed until
  the next install (principle 7: no reinstalling behind the user's back).
  `KC9K_NO_LIBREWOLF=1` skips it for both. Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`2.11`): Linux release file
  `KillerChicken9000-<version>-amd64.deb` (as the MSI's name); a
  pre-release `v1.2.3-beta.1` gets Debian version `1.2.3~beta.1` (sorts
  before `1.2.3`; `kc9k version` says that); the Linux release job waits
  for the Windows one on the same tag and adds its files to that release
  (`windows-release.yml` fails on an existing release and is session B's).
  Owner may overturn.
- DECIDED (By claude) 2026-11-23 (`2.16`): with dark pages on, KC9K
  writes defaults into librewolf.cfg turning resistFingerprinting off and
  fingerprinting protection on for every target but
  `CSSPrefersColorScheme` (LibreWolf FAQ's alternative). Why: RFP tells
  every site light and nothing overrides it (Mozilla bug 1535189,
  WONTFIX); a value set by hand still wins; read at LibreWolf's next
  start, KC9K never closes it for this. Owner may overturn (e.g. keep RFP
  and only turn light pages over).
- DECIDED (By claude) 2026-11-23 (`2.16`): "the browser's own dark mode"
  = Firefox/LibreWolf's website appearance set to dark by the extension
  while KC9K's dark mode is on (released otherwise); the browser's window
  follows the desktop's dark mode, which KC9K switches, and a theme picked
  by hand stays (never overrides the user). Chrome, Chromium and Edge
  have no policy or extension setting for it: they follow the desktop
  (their default "Device"). One setting per browser: an exempt site isn't
  turned over but may show its own dark design. Owner may overturn.
- PROPOSED 2026-11-23 (`2.10`): the setup file holds password hashes and
  lock state. Encrypt it on the device with a passphrase only the user
  knows (Supabase sees only ciphertext)? Lost passphrase = upload lost.
  Proposed: no (the account password already guards it; hashes are
  salted); owner to decide before `2.10` is built.
- PROPOSED 2026-11-22 (`notes/browsers.md`): KC9K closed the owner's
  LibreWolf twice while in use (once with Save As open) when a browser
  setting changed with a website block on: restarted so the new policy
  holds, but unsaved work is lost. Options: (a) warn first ("LibreWolf
  closes in 60 s to apply a setting"); (b) wait until idle a minute;
  (c) close at once. Proposed: (a) ((b) can be waited out forever;
  strengthening must take effect). Owner to decide; nothing built.
- PROPOSED 2026-11-20 (A1 check): a site entry misses an address with a
  port (`127.0.0.1` doesn't match `http://127.0.0.1:47999/`;
  `kc9kIndexTest` compares host with port). SPEC is silent. Proposed: an
  entry without a port matches any port, as browser match patterns do.
- FOR THE OWNER (B3): strict mode turns on AppArmor's
  `kernel.apparmor_restrict_unprivileged_userns` while it acts (Ubuntu's
  default; Mint turns it off), else `unshare -Urm` mounts an exec tmpfs.
  Profiled apps (Chrome, LibreWolf, Firefox, Flatpak, Steam, Discord,
  Slack, VS Code) are unaffected; an Electron/Chromium app without one run
  from an exempt folder aborts (SUID sandbox) unless `--no-sandbox`. Keep
  it, or load an AppArmor profile allowing user namespaces for exempt
  folders? Known limit too: a short program copied to a world-writable
  folder outside the noexec ones still runs once.
- PROPOSED (B4): while a freeze runs, its mode only changes to a stronger
  one (allowed apps < lock screen < logout < shutdown); a weaker one is
  refused (`refused()`, not a bypass); an extension naming none keeps the
  mode. Partly decided (SPEC §4.8, `freeze.ExtendMode`): a running lock,
  logout or shutdown freeze never becomes allowed apps only. Still open:
  other weaker changes (shutdown → logout → lock screen); not rechecked in
  the VM.
