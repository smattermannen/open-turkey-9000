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
| 2.10 | Cross-device sync | §4.14 | db-project | agent |
| 2.11 | One-click installer on Linux, Windows, Android | §4.11 | install, install-windows | agent, owner |
| 2.12 | Chrome, Edge, Firefox, Bing as supported browsers everywhere | §3.13, §4.10 | browsers | agent |
| 2.13 | Accountability partner | §4.15 | — | agent |
| 2.14 | Website | §4.16 | — | agent, owner |
| 2.15 | Remove search results leading to blacklisted addresses | R-PAGE-2 | extension | agent, box |
| 2.16 | "Show dark web pages while dark mode is on" turns on the browser's dark mode | §4.10 | circadian, browsers, gui, gui-windows | agent, box |
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
| W2 | Windows web layers: real browsers obeying; Chrome Web Store ID | §3.13, §4.10 | windows | win-vm, laptop, owner |
| W3 | Windows apps and desktop: notifications, challenge window, hotkeys, lock freeze | §3.1, §3.7, §3.8, §4.8, §4.13, R-LOCK-5 | windows | win-vm, laptop |
| W4 | Windows GUI on a real desktop: Narrator, scaling, drag and drop | §4.1–4.13, R-GUI-1–3 | gui-windows | win-vm, laptop |
| W5 | Windows tamper protection: boot/Safe Mode | §3.10, §4.10 | windows | win-vm, laptop |
| W6 | Windows circadian: gamma, wallpaper, brightness, Night light | §4.10 | windows | win-vm, laptop |
| W7 | Windows MSI: signing certificate, first tag, Apps & features by a person | §4.11 | install-windows | owner, win-vm |
| W10 | Places on Windows: real networks | §3.6, R-MODE-5 | windows, automation | laptop |
| W13 | Recovery path on Windows: a real Safe Mode boot | R-ESCAPE-1 | windows, device | win-vm |
| W18 | One-click browser install on Windows as SYSTEM | §4.10 | windows, browsers | win-vm |
| W19 | Firmware lock on Windows: real firmware | §3.8, §4.10 | windows, device | laptop, owner |
| W24 | Strict mode: real WSL, Sandbox, Hyper-V | §3.8 | windows, device | win-vm |
| W25 | Strict mode's noexec on Windows 11 Home | §3.8 | windows, device | laptop |
| W32 | Windows GUI: UI Automation counterparts of the GTK smoke tests | R-GUI-1–3 | gui-windows | win-ci |
| W49 | Core's `GUI:` lines given their place in the GUI | §4 | gui-windows | agent, win-ci |
| W50 | Freeze fixed; laptop re-runs the soak (idle: 0 PowerShell by the service) | principle 1 | windows | laptop |
| A15 | Enter presses OK in every dialog (GTK and Windows GUI) | R-GUI-3 | gui, gui-windows | agent |
| W54 | Windows layout as GTK: built, CI green; the owner's look on the laptop | §4.1–4.13 | gui-windows | owner |
| A14 | Clip curves mirrored, tested in Go, C#, GTK; the owner's look on the laptop | §4.10 | circadian | owner |

## 0. Fix what exists

**Android is on hold until the Linux project is finished** (owner): its
plan is SPEC §8, its items §3.1.

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
| §4.11 One-click installer, every platform | Not built | `2.11` |
| §3.13 Chrome, Edge, Firefox, Bing on every platform | Partial | `2.12` |
| §4.14 Cross-device sync | Not built | `2.10` |
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
- **2.10 Cross-device sync** (L; §4.14): Not built. KC9K account and
  cloud (principle 6's one exception); Upload all / Download all on
  Linux, Windows, Android; one to one except §4.14's device-specific
  items. Download all replaces even while locked (an exception to
  R-WEAK-1: the weakening audit must name it). Project export
  (`cli/project*.go`) is nearest, but only adds.
- **2.11 One-click installer** (M per platform; §4.11): Not built. Linux
  has `.deb` and `install.sh`, Windows an MSI (W7), Android none; each
  becomes one download, one click.
- **2.12 Chrome, Edge, Firefox, Bing everywhere** (L; §3.13): Partial.
  Built: Chrome, Chromium on Linux; Chrome, Edge on Windows waiting for the
  store ID (W2). Not built: Firefox (LibreWolf only), Edge on Linux, every
  browser on Android (IronFox only), Bing (Microsoft Bing browser app).
- **2.13 Accountability partner** (L; §4.15): Not built. Needs the KC9K
  account (`2.10`): invites, reports, alerts, partner holding password
  locks and the settings password, unlock approval; no screenshots.
- **2.14 Website** (M; §4.16): Not built. Content per §4.16; hosting the
  owner's choice when built.
- **2.15 Search results from blacklisted addresses** (M; R-PAGE-2): Not
  built. The extension removes each search result linking to a
  blacklisted address. Done when: a browser test on the supported engines
  shows no such result and the others untouched.
- **2.16 Dark pages turn on the browser's dark mode** (S; §4.10): rename
  "Show light web pages dark while dark mode is on" to "Show dark web
  pages while dark mode is on" (GTK GUI, Windows GUI, `kc9k circadian
  dark-pages` help); it turns on the browser's own dark mode (theme and
  `prefers-color-scheme: dark`). Done when: switch and dark mode on, a
  supported browser is dark and a site with its own dark design shows it.

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
  (`ci/windows_service_test.ps1`, `ci/windows_desktop_test.ps1`). Left:
  the console's raw mode for the challenge needs a person (win-vm).
- [ ] `W2` Web layers (L). Built: hosts, WFP, registry Chromium policies,
  Firefox-family `policies.json`, extension guard, browser recognition;
  WFP and policies checked in CI. Unchecked (win-vm): real browsers
  obeying, the guard closing a browser (also W11's remote-control
  switches), monk mode's cut. Needs owner: the Chrome Web Store listing's
  ID (`blocker.ChromeStoreExtensionID`); until then Chrome and Edge get no
  extension and are not supported browsers.
- [ ] `W3` Apps and desktop (L). Built; apps and titles closed through
  the agent checked on `windows-latest`. Unchecked (win-vm, laptop;
  `ci/windows_vm_test.ps1`): notifications seen, the challenge window
  (Raw Input refusing injected keys), hotkeys, the lock freeze.
- [ ] `W4` GUI (L). C# WinUI 3, .NET LTS, unpackaged, self-contained, in
  `windows/gui/` (no browser, no web view); every GTK page ported, checked
  in CI against the fake kc9k and the real `kc9k.exe` with its service.
  Unchecked (win-vm, laptop): the challenge window opened from the GUI;
  drag and drop (block onto mode, calendar, clips, wallpaper bars); file
  dialogs; tray menu (W35) and jump list (W36) by hand; Open at sign-in;
  Narrator; high contrast; 150–200 % scaling; a second screen.
- [ ] `W5` Tamper protection against an administrator (L). Built, checked
  on `windows-latest` (service can't be stopped/changed/deleted,
  watchdog task back, strict deny entries, uninstall refused while
  locked). Unchecked (win-vm, `ci/windows_vm_test.ps1 -Reboots`): Safe
  Mode boot, no-escape and the recovery environment on a real boot,
  takeown against strict mode, Reset this PC failing during a lock; W39's
  per-level claims rest on this. Firmware lock: `W19`.
- [ ] `W6` Circadian rhythm and wallpapers (M). Built; dark mode checked
  on `windows-latest`. Unchecked: gamma, wallpaper (win-vm); panel
  brightness, external screens, Night light and its note (laptop).
- [ ] `W7` Installer and release (M). MSI (WiX 6) with service, agent,
  GUI; refuses removal, repair and downgrade while locked; upgrade over a
  lock, full-UI removal, release workflow (`windows-release.yml`) and
  throwaway-certificate signing checked in CI. Left (owner): the
  code-signing certificate (`SIGNING_PFX_BASE64`/`SIGNING_PFX_PASSWORD`,
  or Azure Artifact Signing steps in place of `sign.ps1`: not built) and
  the first tag (`v0.1.0`). Unchecked (win-vm): install and removal from
  Apps & features by a person.

### 4.1 Windows GUI: parity and release (W30–W49)

Every `kc9k` command and `gui-state` field the GTK GUI uses, the Windows
GUI uses too; gaps are Windows-only behaviour and tests.

- [ ] `W32` UI Automation counterparts of the GTK smoke tests (M;
  R-GUI-1–3, §4.2–4.13). Built in `ParityTests.cs`: `gui_cards`,
  `gui_extend`, `gui_duration`, `gui_lists`, `gui_presets`, `gui_modes`
  (card's menu), `gui_freeze_lock`, `gui_automation`,
  `gui_click_everything`. Not yet: `gui_actions` (run a mode locked,
  custom action), `gui_list_resize`,
  `gui_phrase`, `gui_phrase_edit`, `gui_places`, `gui_playlist`,
  `gui_trigger`, `gui_wallpaper` (`gui_import`: `ProjectImportTests`). Needs: win-ci.
- [ ] `W49` Core's `GUI:` lines (W10–W29) given their place in the GUI,
  each with a UI Automation test, once on `main`. Done so far: W15's
  Windows steps (`WindowsGuidedSetupSteps`), W16's help texts as kc9k.exe
  gives them, W18's result via W30's Install. Recheck at each rebase.

Laptop findings on finished items (2026-11-22, at 200 % scaling):

- Fixed in the tests (2026-11-22), to recheck at 200 % on the laptop:
  `HighContrast` reads and restores the owner's flags; scroll bar parts are
  skipped by `EveryControlHasANameAndTakesFocus`; the test process is
  per-monitor DPI aware (`GuiApp.Shot`, `WindowStateIsKept`).

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
- [ ] `W18` One-click browser install on Windows (S; §4.10). Built
  (`InstallBrowser`, winget `--scope machine --silent`; librewolf and
  chrome, no chromium). Unchecked (win-vm): as SYSTEM through the service.
- [ ] `W19` Firmware lock on Windows (M; §3.8, §4.10). Built through
  vendors' WMI (Dell `root\dcim\sysman\biosattributes`, Lenovo `root\wmi`,
  HP `root\HP\InstrumentedBIOS`; settings first, password last over
  stdin, never stored; none found: "set it by hand"). Unchecked (laptop,
  owner): which vendor, and a trusted person to type and keep the
  password.
- [ ] `W24` Strict mode: VMs, WSL, Windows Sandbox, containers (S; §3.8).
  Built (`closeVirtualMachines`; VM program and Docker engine checked in
  CI). Unchecked (win-vm): a real WSL distribution, Sandbox, Hyper-V VM.
- [ ] `W25` Strict mode's noexec on Windows (M; §3.8). Built: Software
  Restriction Policies in the registry (profile folders but not AppData,
  temp folders, removable drives; exempt folders unrestricted); processes
  keep the policy they read, so `CloseNoexecPrograms` closes what older
  shells start. Unchecked (laptop): Windows 11 Home, Explorer started
  before strict mode.

Never on a computer in use; the laptop runs only what is marked laptop.
win-vm (Windows 11, snapshot, admin): `pwsh ci/windows_vm_test.ps1
kc9k.exe`, then `-Reboots` from a fresh snapshot.

Also seen: the Linux workflow `Test` fails in `ci/integration.sh` (commands
without `--yes` where a lock is now confirmed, R-LOCK-12); unit tests pass.

### 4.3 Owner report 2026-11-22: Windows GUI on the laptop

The owner ran the Windows GUI on the laptop and removed it ("it was
horrible"): looks better than GTK, but the GTK layout is better.
Stability first, then layout, then import. Order: top to bottom.

Static findings still open (code read, not seen on the laptop):

- The challenge window (`winagent/challenge_windows.go`) is a plain Win32
  window: now a fixed frame (close only) with a background brush; still no
  manifest in `kc9k.exe` (classic controls, not DPI-aware).
- `gui-state` changes every read while a countdown runs (each read
  redraws the page shown).

Stability (blocks every other Windows GUI test):

- [x] `W50` **The whole computer froze** (L; principle 1). With the GUI
  open the laptop stopped answering; only a forced power-off helped.
  Cause: the extension guard killed Windows: `isBrowserProgram` globbed
  `<exe dir>\*\msedge.dll`, and `System32\Microsoft-Edge-WebView\msedge.dll`
  made every System32 program a "browser" (dwm, svchost, fontdrvhost,
  sihost, ctfmon… closed every 5 s; Kernel-Power 41, a hang). Also load:
  Group Policy forced refresh every 2.5 s, SYSTEM PowerShell
  `WmiMonitorBrightness` every 5 s with circadian off.
  Fixed, checked on windows-latest: nothing under %SystemRoot% or a
  WindowsApps package is a browser; the engine dll counts only beside the
  program or in a version folder; every kill path refuses `sessionCritical`
  (`TestWinGuardSystem32NotBrowsers`, `TestWinGuardNeverClosesWindows`,
  `TestWinKillDenyList`); SRP removal refreshes nothing when idle
  (`TestStrictSRPRefreshOnlyOnChange`); panel PowerShell only with the
  backlight clip on; `ci/windows_soak_test.ps1` in windows.yml's `system`
  job. Laptop (MSI 0.1.1, 2-min soak): fixed — GP 0/min (was 24), no
  System32 closing, no agent restart. Its two idle failures were then
  fixed: a wake no longer re-reads the panel while the backlight clip is
  off; the handle slope skips warm-up. Left: the laptop re-runs the soak
  (idle must show 0 PowerShell by the service).
- [x] `W51` **Screen black several times, "overrun detection" warning**
  (M; §4.10). Cause: dwm.exe killed by the guard (W50; Dwminit exits every
  ~65 s). Also built: one package-level `EnumDisplayMonitors` callback (Go
  throws at 2000 callbacks; `TestWinDisplaysManyCalls`); agent screen calls
  one at a time, newer gamma set replacing a queued one, handlers under
  `recover()`; ramps checked (`CheckRamp`); identity ramp put back when
  circadian goes off and when the agent ends; an agent reporting its own
  crash is not tamper (SPEC §3.10, `TestAgentEndCrashVsKill`). Laptop 0.1.1
  (circadian, Play the day, every setting on/off ×5): no black screen, no
  DWM exit, gamma sane. "Overrun detection" found in no log.
- [x] `W52` **Blank white window appears at random** (M; R-GUI-3). Most
  likely ordinary windows drawn while dwm.exe was dead (classic frame;
  fontdrvhost and others killed too). With 0.1.1 and the GUI open ~30 min
  a window watcher saw no classic or blank KC9K window. Built: gui.log also
  gets `AppDomain` and unobserved-task errors; a crash leaves a note shown
  once at the next start; identical error dialogs collapse; only the page
  shown is updated (`StabilityTests`, CI). The LibreWolf restarts seen
  meanwhile: §5.

Layout (owner: keep the Windows look, use the GTK layout):

- [x] `W53` **The side menu pushes the page sideways** (S; R-GUI-3).
  Opening ☰ moves the page right and cuts off its right edge. Build: the
  menu never moves the page: pinned open like the GTK sidebar by default,
  the page fills the rest, no horizontal scroll at any width. Laptop
  (200 %, menu open): controls past the right edge at 400 and 512 DIP
  (Clusters, Stats, Circadian, Settings combo boxes), none from 683 DIP or
  with the menu folded. `gui.json` `window_width/height` are physical
  pixels (800 opens 400 DIP at 200 %). Built: sidebar pinned at 220 DIP
  (icons only below 640 DIP), page fills the rest; `GeometryTests` (400–1920
  DIP × text 100/150/200 %) green in windows-gui.yml.
- [x] `W54` **Page layout as in the GTK GUI** (M; §4.1–4.13, R-GUI-3).
  Pages are centred with wide margins and cut off on the right; GTK pages
  fill the width, put switches and Lock… buttons in one right column, show
  blocks as rows (switch, type, breaks, lock). Build: Windows pages follow
  GTK's page by page (Overview, Blocks with modes, Circadian rhythm,
  Settings), keeping the Windows look (colours, fonts, Mica). No "Get
  started" card (owner dropped it; W38 removed). Laptop: Circadian at 1440
  DIP is a centred column with ~260 DIP empty on the left; Blocks at 1024
  DIP fills the width.
  Built: pages fill the width, a right column for switches and Lock…,
  blocks as rows (`SidePanel`), "Get started" removed. Left: the owner's
  look on the laptop.

Import:

- [x] `W55` **Import project is missing on Windows** (S; §3 Project).
  Settings → Import project… opens a file dialog and runs `kc9k project
  import <file>` (first `--dry-run`, shown for confirmation). Built:
  `Files.ImportProject`; `ProjectImportTests.cs`.
- [x] `W56` **A Linux project imports safely on Windows** (M; §3 Project,
  R-WEAK-1/2). Linux app and format unchanged. Linux-only parts (paths,
  process names, X11 classes, LibreWolf on Linux, PAM and systemd
  settings) kept in the file, reported "not used on Windows" in the dry
  run, never guessed or an error. Import only strengthens while anything
  is locked; every lock confirmed (R-LOCK-12); CRLF and BOM accepted.
  Built: `testdata/linux-project*.json`, `cli/projectlinux.go`,
  `blocker/linuxonly.go`; `TestLinuxProjectOnWindows`,
  `TestLinuxProjectOnRealWindows` (`unit` job).

Both platforms:

- [ ] `A15` **Enter presses OK** (R-GUI-3): in every dialog of the GTK and
  Windows GUIs, Enter presses the default button. Done when: a GUI test
  per platform opens dialogs, presses Enter and sees each accepted.
- [x] `A14` **Automation clip curves bend one way only** (S; §4.10
  automation clips). Dragging a point far up made a sharp corner, down a
  soft curve, on Linux and Windows. Built: one mirrored curve (below
  straight x^p, above 1 − (1 − x)^p) in Go, C# and GTK, same samples tested
  in each; Go, C# and the GTK smoke test (`ci/gui_circadian_smoke.py`, in
  test.yml: drags a handle 40 px up and down, gets mirror images) pass.
  Left: the owner's look on the laptop.

## 5. Proposed, not agreed

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
