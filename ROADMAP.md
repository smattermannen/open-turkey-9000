# KC9K Roadmap

Code status against `SPEC.md`. The only status file.

Rules: every item cites a § or rule ID (unagreed work goes in §5 first).
Update in the same commit as the code. A wrong status is fixed, not built
on; write "Unchecked" rather than guess. Order: §0 first, top to bottom,
then §2; Windows (§4) is built now, alongside them (owner 2026-11-21).

Status: Built, Partial, Differs (built unlike SPEC), Not built, Unchecked.
Sizes: S < 1 day, M days, L ≥ week. Last checked: 2026-11-21 (device, locks, tamper, install: notes' Covers lines, help texts against the stacking rule; earlier, 2026-11-20, all docs against code: §0.1 *Doc audit 2026-11-20*).

## Index

One row per open item; find it with ``grep -n '`A3`' ROADMAP.md`` and
read only that item. Needs: **agent** (code or scripts, cloud or test machine),
**box** (an agent on the test machine: root, X11 desktop, real browsers,
network, screenshots), **vm** (box, but in a VM with a snapshot: logout,
shutdown, network cuts, strict mode, reboots), **owner** (only the owner:
a decision, a verdict, or real hardware input). Windows (§4): **win-ci**
(GitHub's `windows-latest` runner), **win-vm** (a Windows VM with a
snapshot), **laptop** (the owner's Windows laptop: anything may break, a
factory reset is acceptable, but never trip its bypass prevention). New items get the next ID
of their group and a row here; a finished item loses its row.

| ID | Item | SPEC | Notes | Needs |
|---|---|---|---|---|
| A1 | `no-images` GIFs: the owner's own site; pseudo-element GIFs left empty after address → text → unmatched page | §3.9 | extension | box, owner |
| A2 | Looks from page text lifted on clicks: suites pass (2026-11-20); real use on the laptop | §3.9 | extension | owner |
| A4 | Brightness slider gone, night light off | §4.10 | circadian | box |
| A12 | R-LOCK-12: a person clicks Lock and Cancel (synthetic input is ignored by design) | R-LOCK-12 | hotkeys, locks | owner |
| B5 | Rendered terms on real sites (box); hotkeys on a real keyboard (owner); GNOME (vm) | §3.1, §4.13 | extension, hotkeys | box, vm, owner |
| B8 | Use counting with input and idle (simulated input); LockedHint under Cinnamon: run `ci/lockedhint_check.sh` on the desktop (written 2026-11-21, never run) | §3.7 | allowance | box |
| B11 | Social media settings beyond Shorts; playlists; places; text break | §3.2, §3.6 | extension, automation | box |
| B12 | Chrome: a person opens the popup and tries to remove the extension (synthetic checked 2026-11-20) | §3.13 | browsers | owner |
| 2.1 | Circadian on a laptop backlight, external screens (ddcutil), several screens, GNOME/MATE | §4.10 | circadian | owner, vm |
| 2.2 | Ad trigger: gambling and dating industry domains (none seen live, 2026-11-21); browser test built (2026-11-20) | §3.9 | actions | agent |
| 2.4 | SafeSearch per engine: built; checked in Chrome (2026-11-21); in LibreWolf (Unchecked) | §4.10 | browsers | box |
| 2.6 | Factory reset protection: built; PAM root guard: run `ci/strict_pam_vm_test.sh host kc9k-vm` (written 2026-11-21, never run) (vm); firmware lock on real Dell/Lenovo/HP firmware (owner) | §4.10 | device | vm, owner |
| 2.7 | Hardening: the text lock's window on a real desktop (Unchecked); rules server secret dropped (§5, 2026-11-21) | R-LOCK-5, §4.6 | browsers, locks | box |
| 2.8 | Every Partial/Not built row of §1 | see §1 | see §1 | agent |
| 2.9 | Tamper settings of 2026-11-21: guest, inactive-tabs, embedded checked in Chrome-family browsers (2026-11-21); inactive-tabs, embedded in LibreWolf (box); VM parts: run `ci/tamper_vm_test.sh host kc9k-vm` (written 2026-11-21, never run; needs Go in the VM) | §4.10 | browsers, extension, install | box, vm |
| 2.10 | Cross-device sync: KC9K account, cloud, Upload all, Download all | §4.14 | db-project | agent |
| 2.11 | One-click installer on Linux, Windows, Android | §4.11 | install, install-windows | agent, owner |
| 2.12 | Chrome, Edge, Firefox, Bing (browser app) as supported browsers on Linux, Windows, Android | §3.13, §4.10 | browsers | agent |
| 2.13 | Accountability partner: reports, alerts, partner-held passwords, unlock approval | §4.15 | — | agent |
| 2.14 | Website: like Cold Turkey's in kind; hosting to choose | §4.16 | — | agent, owner |
| D3 | Instagram "Following only": redirect checked (2026-11-20); logged in, the feed shows no suggested posts | §3.2 | extension | owner |
| W1 | Windows service, session agent, named pipe, install/uninstall: built, checked on `windows-latest`; agent and console Unchecked | §3.13 | windows | win-vm |
| W2 | Windows web layers: built; WFP checked in CI; browsers obeying Unchecked; Chrome Web Store ID | §3.13, §4.10 | windows | win-vm, laptop, owner |
| W3 | Windows apps and desktop: built, Unchecked on a desktop | §3.1, §3.7, §3.8, §4.8, §4.13, R-LOCK-5 | windows | win-vm, laptop |
| W4 | Windows GUI: C# / WinUI 3. Built, checked in CI against the fake kc9k and the real kc9k.exe with its service (2026-11-21); Narrator, scaling, drag and drop, a real desktop: Unchecked | §4.1–4.13, R-GUI-1–3 | gui-windows | win-vm, laptop |
| W5 | Windows tamper protection: built, checked on `windows-latest`; boot/Safe Mode Unchecked; firmware lock in W19 | §3.10, §4.10 | windows | win-vm, laptop |
| W6 | Windows circadian rhythm and wallpapers: built, Unchecked | §4.10 | windows | win-vm, laptop |
| W7 | Windows MSI: built, checked in CI (install, removal, refused while locked, reinstall guard, 2026-11-21); signing needs the owner's certificate; Apps & features by a person Unchecked | §4.11 | install-windows | owner, win-vm |
| W32 | Windows GUI: UI Automation counterparts of the GTK smoke tests | R-GUI-1–3 | gui-windows | win-ci |
| W36 | Windows GUI: jump list and start arguments | §4.10 | gui-windows | agent, win-ci, win-vm |
| W40 | MSI release workflow on a tag | §4.11 | install-windows | win-ci, owner |
| W41 | MSI CI: upgrade over a lock, older refused, repair, full-UI removal | §4.11, R-LOCK-9 | install-windows | win-ci, win-vm |
| W42 | Signing proven with a throwaway certificate; Azure Artifact Signing | §6 2026-11-21 | install-windows | win-ci, owner |
| W49 | Core's `GUI:` lines given their place in the GUI | §4 | gui-windows | agent, win-ci |
| W10 | Places on Windows (Native Wifi; Wi-Fi kept on): built; real networks Unchecked | §3.6, R-MODE-5 | windows, automation | laptop |
| W12 | Scripts and script hosts on Windows | §3.1, §3.9 | windows, apps-titles | win-ci |
| W13 | Recovery path on Windows (Safe Mode, `kc9k recover`): built; a real Safe Mode boot Unchecked | R-ESCAPE-1 | windows, device | win-vm |
| W18 | One-click browser install on Windows (winget): built; as SYSTEM Unchecked | §4.10 | windows, browsers | win-vm |
| W19 | Firmware lock on Windows (vendors' firmware WMI): built; real firmware Unchecked | §3.8, §4.10 | windows, device | laptop, owner |
| W20 | Strict mode: Windows' ways to SYSTEM | §3.8, principle 3 | windows, device | win-ci |
| W21 | Strict mode: administrators' privileges | §3.8, principle 3 | windows, device | win-ci |
| W22 | Strict mode: new accounts | §3.8 | windows, device | win-ci |
| W23 | Clock and time zone put back (tamper clock) | §4.10 | windows, locks | win-ci |
| W24 | Strict mode: VMs, WSL, Sandbox, containers: built; real WSL, Sandbox, Hyper-V Unchecked | §3.8 | windows, device | win-vm |
| W25 | Strict mode's noexec: Software Restriction Policies: built; Windows 11 Home Unchecked | §3.8 | windows, device | laptop |
| W26 | Other ways around: Firefox registry policies, DoT, IPv6 hosts | §3.13, §4.10 | windows, browsers | win-ci |
| W27 | Desktop checks of `windows_vm_test.ps1` in CI | — | windows, testing | win-ci |
| C1 | Build for the brothers' computers with `TAGS=devmaster` | §4.11 | install | agent |
| C2 | Remove test helpers | §4.11 | install | agent |

Done, never run on a real desktop (box): §0.1 *Owner report 2026-11-17
(later)*, *Owner report 2026-11-17*, *Owner 2026-11-16*.

## 0. Fix what exists

**Android is on hold until the Linux project is finished** (owner,
2026-11-16): no Android work; its items below wait.

### 0.1 Code contradicting `SPEC.md`

- [x] **Doc audit 2026-11-20** (docs and code compared; notes fixed in the
  same session; what the code still gets wrong is below).
  - [x] **E1** Presets in projects are a choice (owner 2026-11-20, SPEC
    §3.13, §6): built 2026-11-20. A backup (`project export`) holds every
    preset (block presets, list and phrase set presets, the `presets`
    table); `--share` only with `--presets` (a terminal asks; the GUI's
    Share… asks). Import adds them like blocks (same content skipped, a
    taken name gets `name-2`). `TestProjectPresetsAreAChoice`; the GUI
    question on a real screen: Unchecked.
  - [x] **E2** Words to avoid (§3.14) still shown to the user: built
    2026-11-20. Commands `block-preset` and `list-preset` (aliases
    `template`, `cluster`, `phrase-set-preset`), `block create --preset`
    and `record start --preset` (old flags kept); every help text, message
    and GUI label says block preset, list preset, phrase set preset.
    `TestPresetCommandNames`, `ci/gui_lists_smoke.py` (import sources).
    2026-11-21 (quality pass): the last ones shown: `break` help ("relock",
    "typing challenge"), the text lock's challenge error, the old "word
    combination" error for a term with brackets, the GUI's text break help,
    the toolbar's "today's quota left" (now "of the allowance left", also
    right for weekly and monthly allowances) and the page panel's "Keyword
    test mode" (extension 1.31.0).

- [ ] **Owner report 2026-11-20** (from real use; D1, D2, D4 built; D3 left).
  - [x] **D1** In an opened block, the bar from "← All blocks" down to Save
    edits and the Start/Stop/Unlock/Automation/⋯ row stays at the top of
    the window while the tabs below scroll (built 2026-11-20: only the tabs
    are in `detail_scroll`; `ci/gui_cards_smoke.py`; on a real screen:
    Unchecked).
  - [x] **D2** Start runs `kc9k start <block>` at once, no lock dialog;
    once on, the button is "Add locks…" (R-LOCK-7, still confirmed,
    R-LOCK-12). SPEC §4.3 already says the menu applies at once. Built
    2026-11-20 (`start_block`, `add_locks`; `ci/gui_cards_smoke.py`; on a
    real screen: Unchecked).
  - **D3** Instagram: social setting `instagram:following-only`: `/`
    goes to `/?variant=following`, Instagram's feed of accounts followed
    (no suggested posts). Built 2026-11-20 (`social.js` redirects take a
    `query` that stops them; `ci/match_test.js`). Real site, 2026-11-20,
    desktop, LibreWolf (headless, own profile, logged out; scratch block):
    `/` → `/?variant=following` at once, no reload in the next 15 s;
    `/?hl=sv` → `/?variant=following`; `/?variant=following` and
    `/instagram/` left alone; clicking the logo on `/instagram/` → `/`,
    then `/?variant=following` 3 s later, then still; `history.pushState`
    to `/` → `/?variant=following` within 1 s; no loop. Left (owner,
    Unchecked): logged in, the feed shows only accounts followed; nothing
    hides suggested units (CSS can't match their text).
  - [x] **D4** ClipEditor: a small circle in the middle of each segment
    between two points; left-drag it to change the bend (`tension`).
    Was: the whole curve was the drag target (`on_curve`, gone).
    Built 2026-11-20 (`handles`, `handle_at`; `ci/gui_circadian_smoke.py`;
    on a real screen: Unchecked).

- [x] **Owner report 2026-11-17 (later)** (after testing the latest build;
  top priority; SPEC changed the same day, §6). In three steps, each
  merged when done; on the laptop: Unchecked.
  - [x] Step 1, fixes (built 2026-11-17; on the laptop: Unchecked):
    - [x] Deleting an opened block opened another block (the page fell back
      to the first block): now back to the block list; a refused delete
      leaves the block open (§4.2). `ci/gui_cards_smoke.py`.
    - [x] Block cards without the ⋯ menu (§4.2: nothing else); from the
      keyboard a mode's *Add blocks…* and *Take out…* (R-GUI-3).
      `ci/gui_cards_smoke.py`, `ci/gui_modes_smoke.py`.
    - [x] A mode's *Add blocks…* offers *Create new block…*: made and put in
      the mode at once (§4.2). `ci/gui_modes_smoke.py`.
    - [x] Social media settings: a section of the Websites tab after the
      blacklists, above the exceptions, opened and closed (closed it says
      how many are on); no tab of their own (§4.3).
      `ci/gui_playlist_smoke.py`.
    - [x] Places in an automation (a block's, a mode's, monk mode's): one
      widget (`PlaceRules`), a row per added place saying it is added,
      what arriving there does ("Arriving here turns this block on/off";
      with both, "Here during one of its windows …") and "You are here" /
      "Not here now", with Remove; below, a saved place to add and Save
      where I am now as… (for a mode too). How many networks a place needs
      is set on the Places page only (§4.5). `ci/gui_places_smoke.py`.
    - [x] A block and a mode may share a name (§4.2 Names): blocks, modes
      and schedules each have their own names (`db.NameTaken(kind, name)`);
      commands that take any of them (`schedule add|list|remove|apply|
      take-off`, `automation`, `place rule`) read a block first, then a
      mode, then a schedule, and `--mode`/`--schedule` say otherwise; `mode
      …` commands read only modes. The GUI's mode and schedule editors pass
      the flag. `TestModes`, `TestSavedSchedules`,
      `TestModeBlocksShareAutomation`, `ci/gui_modes_smoke.py`.
    - [x] An opened block is always a draft until *Save edits*; no *Edit as
      draft* (§4.3: code contradicted SPEC). Its first edit in the
      Websites or Apps tab makes the draft; the bar says "Unsaved edits"
      with Save edits / Discard; leaving asks; its card shows "Unsaved
      edits". The draft now also holds social media settings, the apps' and
      window titles' actions and lists' test mode, and keeps what the block
      was when it was made, so Save edits applies only what was changed in
      it, on the block as it is now (an entry added meanwhile from the
      browser's popup stays). Start, Stop, Unlock, automation, breaks,
      allowance and bypass prevention act on the block at once (see §5).
      `TestDraftRebased`, `TestDraftNeverRuns`, `ci/gui_lists_smoke.py`.
    - [x] YouTube playlists: a playlist's address in an exception list
      (kept as `youtube.com/playlist?list=<id>` however pasted) lets that
      playlist's videos through, only them; the separate box goes (§3.2
      Exception list). Enforcement adds what it needs, never stored
      (`db.withPlaylists`: the setting `youtube:playlist=<id>` for the
      extension and the video pages exception); `kc9k social B playlist
      ID on|off` adds or removes the entry; opening an older database
      (`migratePlaylistExceptions`) and importing an older project file
      (`AdoptOldPlaylists`) turn the old settings into entries.
      `TestPlaylistException`, `TestPlaylistExceptionsMigrated`,
      `TestPlaylistInEntry`, `ci/gui_playlist_smoke.py`. Playing a video of
      a real playlist: still Unchecked (§0.2).
  - [x] Step 2 (built 2026-11-17; on the laptop: Unchecked):
    - [x] Frozen Chicken is a mode among the blocks, up to a week; its
      page leaves the sidebar (§3.8, §4.8, R-FREEZE-3). Its card is next to
      monk mode's (switched on it asks how and until when, `freeze_dialog`
      with the calendar; running, the switch is greyed out); its view holds
      Freeze now, Scheduled freezes and Allowed apps. `freeze --until`, at
      most a week (also list actions and ladder steps through it); `kc9k
      mode` lists it, `mode on|off|show freeze` say what to do; no mode takes
      its name. `TestFrozenChickenMode`, `ci/gui_modes_smoke.py`,
      `ci/gui_freeze_lock_smoke.py`.
    - [x] Monk mode switched on and off like any mode (§3.8, R-MONK-3):
      switched on (confirmed) it runs while its automation has it on, with
      none until switched off; switched off it stops; its automation is
      free to change. Only a run for a set time (`kc9k monk --for`) is
      locked: `monk.until`, the switch greyed out, off refused. The daemon
      no longer pushes `monk.until` ahead (`runMonkAutomation` gone):
      `MonkAutomationOn` decides each cycle. `TestMonkModeAutomation`,
      `TestMonkAutomationWindow`, `TestMonkAutomationPlace`,
      `ci/gui_modes_smoke.py`.
    - [x] Locks on modes (R-MODE-3, refined the same day: a mode of blocks
      through its blocks' locks): `kc9k mode lock M <start's lock flags>`
      starts every block with them after one confirmation; GUI Lock… in the
      mode's view. `TestModeLock`.
    - [x] Automation in the block's menu, a view of its own (no Schedule &
      places tab; "← Back to the block"); greyed out in a mode with the
      tooltip "Automated by 'M'", whose bar (Open the mode, Take it out) is
      on the block's page; the card's Continuous/Automated block opens it
      (§4.3). A block keeps its own automation in a mode (`mode_blocks.own`,
      kept when it first goes in, also through a move to another mode) and
      gets it back when taken out or the mode is deleted, through the gate
      as a whole (R-MODE-1: refused if that weakens a locked block).
      `TestModeKeepsOwnAutomation`, `TestModeBlocksShareAutomation`,
      `ci/gui_modes_smoke.py`, `ci/gui_cards_smoke.py`.
    - [x] Presets instead of clusters and templates (§3.2 Preset, §4.4):
      the Presets page (it was Clusters) shows block presets (the
      templates: New block from it…), lists, automation and schedule
      presets (Use…, Rename…, Delete), and list and phrase set presets (the
      clusters' editor and recording). Saved from a block's ⋯ menu (Save as
      block preset…), its Websites tab (Save lists as preset…: Both /
      Blacklists / Exceptions, Include actions?; Add lists from a
      preset…), its or a mode's Automation (Save as preset…, Use a
      preset…). Always copies. CLI `kc9k preset save lists|automation|
      schedule`, `use`, `rename`, `delete` (table `presets`); lists presets
      used into an opened block go to its draft. Hotkey presets come with
      hotkey actions (step 3). `TestPresets`, `ci/gui_presets_smoke.py`.
    - [x] *Social preset* renamed *social media setting* (§3.2): the words
      people see already said "social media settings"; SPEC's glossary now
      does too.
  - [x] Step 3 (built 2026-11-17; on the laptop: Unchecked):
    - [x] Mode action from every trigger (§3.9, R-MODE-4, refined the same
      day: the mode runs now for a time, locked, whatever its automation
      says, then is as it was): list and phrase set action `mode`
      (`--target M --for D`), the apps' and window titles' `mode`, ladder
      step `mode NAME 2h`; by hand `kc9k mode run M --for D --yes` (for
      hotkeys too). A mode of blocks: its blocks switched on, kept enabled
      (`enabledBlocks`) and held (`blockHold.modeRun`, stop refused) until
      the time is over (`db.ModeRun`, setting `mode.runs`), then those that
      were off go off (`endModeRuns`); monk mode runs for the time, Frozen
      Chicken locks the screen. Set up only with `--no-confirm-ok`
      (R-LOCK-12, as the monk action). The monk action stays, as the Mode
      action for monk mode. Extension 1.26.0: `mode` is a device action.
      GUI: "Run a mode, locked" in lists' actions, the apps' and titles'
      (`TriggerEditor`) and ladder steps (sent as `--step-json`); the mode's
      card and view say "Running, locked". `TestModeRun`,
      `TestModeRunCommand`, `TestModeAndCustomActions`,
      `ci/gui_actions_smoke.py`. On the laptop: Unchecked.
    - [x] Custom actions wherever actions are, and for a mode becoming
      enabled (§3.9, R-CUSTOM-1/2): `custom` with `--custom "open …"` /
      `"run …"` (list and phrase set actions, the apps' and titles', ladder
      steps `custom "open …" "run …"`, `kc9k mode custom M add|remove|
      clear`); run as the first signed-in desktop user (`blocker.RunCustom`:
      xdg-open, sh -c; never root), at most once a minute per block and
      trigger (`customDue`); a page's custom action is reported even when
      nothing is taken away (extension 1.26.0: `outcome.custom`). A mode's
      run each time it becomes enabled, once per boot while it stays so,
      at sign-in when nobody was (`runModeCustoms`). GUI `CustomEditor`
      (Open a site, file or folder / Run a program or command), in the
      action dialogs and the mode's view ("When it becomes enabled").
      `TestCustomAction`, `TestModeCustom`, `ci/gui_actions_smoke.py`,
      `ci/match_test.js`. Opening on a real desktop: Unchecked.
    - [x] Hotkey actions and the Hotkeys page (§3.9, §4.13): a hotkey runs
      its actions in order (`hotkeys.actions`, `db.HotkeyAction`; older
      hotkeys start their block): start (with start's locks) or stop a
      block, switch a mode on or off, run a mode locked, monk mode, Frozen
      Chicken, close the browsers, lock the screen, log out, shut down,
      custom actions (`kc9k hotkey add KEYS [BLOCK] [locks] --do …
      --do-json …`). Locks confirmed when added and, unless Run without
      confirmation, once at each press (one desktop question for all);
      switching off needs the settings password when added; the press
      goes through each command's own refusals. Hotkey presets (`preset
      save hotkey N NAME`, `preset use NAME --keys K`). GUI: Hotkeys page
      in the sidebar (out of Settings): the list, Add hotkey… (keys, Add
      action… for each, Run without confirmation), Start a block… (the
      old quick dialog), Save as preset…; hotkey presets on the Presets
      page. SPEC's list of hotkey actions lost "pause a block" (there is no
      pause function). `TestHotkeyActions`, `TestHotkeyPreset`,
      `ci/gui_hotkey_smoke.py`. On a real keyboard: Unchecked.
    - [x] Mode wallpapers (§4.10, R-MODE-5): every mode, monk mode and
      Frozen Chicken too, has wallpapers laid out as circadian rhythm's
      (`kc9k mode wallpaper M add|time|remove`, settings
      `mode.wallpapers.<id>`, Frozen Chicken's `.0`); the GUI's
      `WallpaperEditor` is the circadian rhythm page's editor, in the
      mode's view too. While a mode is enabled its picture due shows; of
      several, the one enabled last. One setting decides which win
      (`kc9k wallpaper priority modes|circadian`): "Override circadian
      rhythm wallpapers" on every mode's view, "Override modes" on the
      circadian rhythm page. The wallpaper default (`kc9k wallpaper
      default latest|circadian|custom FILE`; Settings, Wallpaper default)
      shows when neither has one and while the place isn't known for a
      mode switched by places, which then shows nothing and doesn't let
      circadian rhythm's picture through either (it might override it:
      the owner's home/school example). A mode by time alone still shows.
      The place counts as unknown at once at boot and after waking (time
      asleep grew), otherwise after a minute without networks (one empty
      look isn't leaving). Places looked for at once: the engine steps
      every second while the place decides and isn't known (3 minutes
      after starting or waking), networks read every 2 s and a scan asked
      every 15 s while none are seen; the choice goes into signed-out
      users' settings before anyone signs in, as circadian rhythm's did.
      Before sleep a systemd hook the daemon writes while it matters
      (`/usr/lib/systemd/system-sleep/kc9k`, `blocker.SyncSleepHook`,
      removed by uninstall) runs `kc9k-bin wallpaper before-sleep`: with a
      picture or circadian rhythm as default it goes on first, so the
      laptop wakes with it, not with the last place's; after waking what
      should show is given afresh. `kc9k wallpaper` says what shows now
      and why. Project files carry the modes' wallpapers, the default and
      its picture. `TestModeWallpaper*`, `TestWallpaperBeforeSleep`,
      `TestWallpaperInUse`, `TestModeWallpaperCommands`,
      `TestProjectModeWallpapersAndCustoms`, `ci/gui_wallpaper_smoke.py`.
      On the laptop (a real boot and wake at two places): Unchecked.
    - [x] Found while building it, fixed: project files didn't carry a
      mode's custom actions; importing a file now lists what its custom
      actions would open or run and needs `--yes` (the GUI shows them in
      its Import question), since a shared file can come from anyone
      (R-CUSTOM-1). A backup's hotkeys that run actions (or start no block)
      were left out at import, and their blocks weren't renamed with the
      file's: now checked action by action (`hotkeyImportProblem`).
      `TestBackupHotkeyActions`.

- [x] **Owner report 2026-11-17** (real use on the laptop; top priority).
  Built the same day; on the laptop: Unchecked.
  - [x] Wrong wallpaper after a boot (§4.10 circadian wallpapers): at
    boot nobody is signed in, so the engine's first try reached nobody and
    the next waited a minute: the desktop started with the last picture
    given and changed up to a minute after sign-in. Now a wallpaper or dark
    mode change also goes into the settings of people who aren't signed in
    (`blocker.Reach`, `gsettings` on a private session bus,
    `dbus-run-session`), so their desktop starts with it; someone who signs
    in without having been given it gets it at once (a sign-in ends the
    minute's wait), and only them: a hand-picked wallpaper stays until the
    next change. Older saved state counts as given to everyone signed in.
    `TestCircadianWallpaperBeforeSignIn`, `TestCircadianWallpaperOldState`,
    `TestReachTargets`; the private bus write checked in a cloud container.
    On the laptop: Unchecked.
  - [x] "The Killer Chicken extension isn't running" at every start and now
    and then while it ran (§4.10 install dialog): the GUI said so whenever
    no browser had checked in for 5 minutes, so always before the browser
    was opened. Now only when no supported browser is installed, or, while
    the extension is needed, a supported browser has run past the grace and
    a minute more without its extension checking in; the dialog names it
    (gui-state `extension_missing_in`). `TestExtensionMissing`. On the
    laptop: Unchecked.
  - [x] Modes, schedules and places pages (§3.2 Mode, §3.6 Schedule, §3.8
    Monk mode, §4.2, §4.5, R-MODE-1/2, R-MONK-1–3). Drafts take no
    schedule and join no mode.
    - [x] Schedules outside blocks; Apply schedule on blocks and modes
      (CLI: `kc9k schedule create|apply|take-off|rename|delete`; `schedule
      add|remove|list` take a block, mode or schedule). Shared like
      places: a change reaches everything using it, refused as a whole
      when it weakens a locked block; overlapping schedules stored clipped
      (R-SCHED-2 holds); deleting one leaves its windows as their own.
      `TestSavedSchedules`, `TestResolveWindowsClips`,
      `TestSetBlockAutomation`, the weakening audit.
    - [x] Modes (CLI `kc9k mode …`): blocks share the mode's automation
      (copies in each block, so the daemon and every other reader are
      unchanged); a block in a mode isn't changed on its own; put in:
      gated, taken out: keeps it; the switch switches the blocks (locked
      ones stay on). Monk mode is a mode: windows, schedules and places;
      switched on (confirmed) its automation runs it, to the window's end
      or half a minute at a time while places decide; while it runs it
      can't be switched off or weakened; Wi-Fi stays on for its places.
      `TestModeBlocksShareAutomation`, `TestMonkModeAutomation`,
      `TestMonkAutomationWindow`, `TestMonkAutomationPlace`; announced
      5 and 1 minutes before its window, as a scheduled freeze is
      (`TestMonkNotices`), and on the Overview ("Monk mode today …"),
      `TestModes`. In projects: schedules and modes (with their own
      windows, schedules, places in backups, settings); a block's windows
      that are copies of a schedule say `copy_of` (an older KC9K reads
      them as its own) and import applies the schedule instead; import
      only adds (a schedule or mode of that name here is used as it is;
      monk mode's automation only when it has none here, never switched
      on). `TestProjectModes`.
    - [x] GUI: modes among the block cards above the blocks (monk mode
      always first; a block card dragged onto a mode goes in, or its ⋯
      menu "Move to the mode…"; blocks in a mode show only in its view),
      the mode's view (its blocks as cards, Add blocks…, Rename/Delete,
      its automation: `AutomationEditor` with grid, presets, Apply
      schedule…, places; monk mode: Start now for a time…), Schedules and
      Places pages in the sidebar (cards with what uses each; a schedule's
      grid; Save where I am now as…, networks needed, "You are here"). A
      block's Schedule & places: Apply schedule… with its schedules as
      chips, windows from a schedule drawn dashed with its name (a click
      offers the Schedules page), and in a mode its automation greyed out
      with Open the mode / Take it out. `ci/gui_modes_smoke.py`; the
      click-everything, cards, automation and places smoke tests pass
      (the automation one expected the wording from before 2026-11-16's
      "its place turns it": fixed). Drag and drop with a real pointer
      (xdotool in Xvfb): `ci/gui_modes_drag_smoke.py`. On a real screen:
      Unchecked.

- [x] **Owner 2026-11-16: automation and the block list (TOP PRIORITY).**
  Built the same day; on the laptop: Unchecked.
  - [x] Place rules have no actions (§3.6 Place rule, §3.9 Action,
    R-SCHED-1). A place only enables or lifts an active block. Built by
    mistake the same day (it should have been questioned: a place already
    means "enabled here" in the active/enabled model). Removed: the
    trigger group `place:<id>`, `placeArrivals` and `placeEnables`, `block
    trigger B place:NAME` (says a place rule has no actions), the GUI's
    "Actions…" for a place rule, projects' place keys (old files' are
    ignored); opening drops stored place sets (`migratePlaceTriggers`,
    `TestPlaceTriggersDropped`, `TestPlaceRuleEnables`,
    `ci/gui_trigger_smoke.py`).
  - [x] Automation (§3.6 Place rule, Automation; R-SCHED-1; built
    2026-11-16): Enabled by default / Disabled by default for windows
    and places alike, both/either, any number of places; with no network
    seen a place counts as what keeps the block enabled (switching Wi-Fi
    off lifts nothing). `kc9k automation`, GUI Automation section atop
    Schedule & places, projects (`automation`), gui-state. Every change
    of windows, places or settings is gated by `AutomationWeakens`
    (enabled→lifted at any time and place weakens). That closed a lock
    bypass: a first place rule on a locked continuous block (it was taken
    as strengthening) made it enabled only there; and it fixed two
    mistakes: a first window on a place-ruled block under "both" counted
    as strengthening, the last allow window couldn't be removed from a
    locked block though that makes it continuous (still refused when it
    has a schedule lock, which would never hold again). Blocks with allow
    windows and place rules (the places turned them on; now off, as the
    windows do): Unchecked whether any exist. `TestAutomationEnabled`,
    `TestAutomationWeakens`, `TestAutomationCommand`,
    `ci/gui_automation_smoke.py`.
  - [x] Block list (§4.2, R-GUI-3; built 2026-11-16): the Blocks page
    opens on one card per block: its name (opens the block), an on/off
    switch (on: started without locks; off: stopped; greyed out while
    locked), "Continuous block" or "Automated block" (opens Schedule &
    places with the Automation section), its breaks ("No breaks" or
    "Breaks: 10 min every hour, Pomodoro…"; opens Breaks & allowance) and
    "Unlocked" / "Locked, 2 h left" (Add locks or Unlock). "← All blocks"
    and Blocks in the sidebar come back to the list; the Overview and New
    block open the block itself. `ci/gui_cards_smoke.py`. On a real screen:
    Unchecked (screenshots in Xvfb only).

- [ ] **Owner report 2026-11-15** (real use on the laptop; top priority).
  Status per item; check against the latest build before calling it fixed.
  - [ ] `A1` `no-images` doesn't hide GIFs (§3.9 Look actions: animated GIFs
    included). Checked 2026-11-16 in LibreWolf (cloud container), from page
    text and from an address list: GIFs as `<img>`, `<picture>`, inline
    `background-image … !important`, `<input type=image>`, inside a shadow
    root and inside a frame are all hidden. One gap found: a GIF drawn by a
    pseudo-element (`::before { content: url(x.gif) }`) still showed. Fixed
    in 1.22.0: on no-images pages each element's `::before`, `::after` and
    own `content` is looked at in idle time (8 ms pieces; shadow roots, added
    content, rescans after 1, 2, 4 s then every 10 s), marked
    (`data-kc9k-noimg`) and emptied by both sheets; shadow roots get the
    no-images rules as an adopted sheet. `ci/content_harness_test.js`
    (Chromium, page script with stubbed APIs: url(), image-set(), own
    content, !important, shadow root, added later, text kept, a
    20,000-element page without long tasks) passes. Not run in LibreWolf
    yet; confirm the owner's case on the laptop (which site?). S
    2026-11-20 (overnight, desktop, LibreWolf 159, installed devmaster
    build): all LibreWolf suites pass, each run alone (browser_test 20,
    extras_test 27, flash_test 10, download_test 4, actions_test 14,
    quota_test 2); `ci/content_harness_test.js` passes in Playwright's
    Chromium. The owner's own site: still Unchecked.
    2026-11-20 (desktop, LibreWolf 159.0.1, installed devmaster build,
    headless, screenshots): no-images from an address entry and from a
    page-text list on a local page with an animated GIF drawn by `::before`,
    `::after`, own `content`, `image-set()`, `!important`, an open and a
    closed shadow root, an element added after 3 s and a class added after
    5 s, plus an `<img>`: all hidden at 4 s and 18 s (computed content `""`
    or `normal`, `data-kc9k-noimg` set, no GIF pixels; text kept); without
    the block all show. Real sites from an address list: en.wikipedia.org
    /wiki/GIF (9 GIFs), commons File:Rotating_earth_(large).gif (12 GIFs),
    giphy.com (24 pictures): 0 visible, 0 background images. Found: a page
    that earned no-images from its text carried it to the next page
    (≤ 15 s, `carryLooks`) by cancelling its picture requests, so a next
    page without the word kept broken pictures. Fixed (extension 1.27.1):
    a carried look only hides; `ci/extras_test.py` "the next page without
    the word shows its picture once read" (failed before, 29/29 after).
    Seen twice, cause not found: after address-matched → text-matched →
    unmatched page of one local site (one GIF address, `no-store`), the
    unmatched page's pseudo-element GIFs stay empty (its `<img>` shows);
    text → unmatched and address → unmatched show them. Left: the owner's
    own site (unknown here).
  - [ ] `A2` Looks from page text (greyscale, no-images, no-video) unreliable
    and lifted for a moment on every click inside a site (§3.9; owner
    2026-11-15). Causes found: `render` forgot the page's looks on a new
    address while `scanKeywords` thought the page already read, so a page
    reached without loading never got them back; the background dropped
    them on every address change; the hiding CSS was an author sheet
    (beaten by page `!important`/inline styles, not in shadow roots, not on
    `::before`/`::after`); videos stopped only once a second, not in shadow
    roots. Fixed: one reset point (`enterAddress`); looks carried to the
    next address until its first read (`page-judged`, ≤ 15 s); greyscale,
    no-images, no-video drawn by a user-origin sheet in every frame
    (`syncLookCss`, Chrome via `scripting`); `play` stopped at once, shadow
    roots included; bfcache pages read again. Extension 1.19.0. Not run in
    a browser here; confirm on the laptop (`ci/extras_test.py`,
    `ci/browser_test.py`). S
    2026-11-20 (overnight, desktop): `extras_test` 27/27 and `browser_test`
    20/20 pass in LibreWolf 159 (local pages, not real sites). Left: real
    use on the laptop.
  - [x] `A3` `no-downloads` doesn't block downloads (§3.9). Checked 2026-11-16
    in LibreWolf: a link on the page, a file from another host (only the
    origin as referrer) and a file the page makes itself (`blob:`) are all
    cancelled (`ci/download_test.py`, four cases). Likely cause in Chrome:
    its service worker is stopped after a while idle and forgot which pages
    forbid downloads (each page reports once). Fixed in 1.21.0: the pages
    are kept in session storage; a download that finishes before it can be
    cancelled has its file removed. Unchecked in Chrome (not run here). S
    2026-11-20 (desktop, Google Chrome 158, policy-installed extension,
    fresh profile driven over CDP): link and `blob:` cancelled, but a file
    from another host went through (Chrome names only the origin as
    referrer). Fixed in 1.27.3: with an origin-only referrer the open tabs
    of that origin stand in, with none the tabs in front
    (`downloadSources`; `node ci/download_background_test.js`, 9 cases).
    Then 8/8: link, other host, `blob:` cancelled and a plain page's file
    kept, with the worker running and again after it was stopped in
    `chrome://serviceworker-internals`; in a 10-min idle wait the worker
    was never stopped (keepalive alarm), 8/8 after it too.
    `ci/download_test.py` 4/4 in LibreWolf after the fix.
  - [ ] `A4` Desktop brightness slider gone from the menu; blue-light filter
    toggles on and off; desktop night light turns itself off while its
    switch shows on, **also with circadian off and strict mode off** (§4.10
    circadian: must leave the desktop's controls working, off touches
    nothing). Found 2026-11-16 (code reading): once KC9K had set colours,
    every daemon restart (tests and upgrades restart it) with circadian or
    its colour clips off gave the screens plain gamma ramps, also when the
    ramps on them were the desktop night light's (after a restart it didn't
    know what it had set): the night light went off while its switch
    showed on. Fixed: KC9K saves the ramp it set (`circadian.applied`),
    gives plain ramps back only if that one is still on the screens, and
    after a restart someone else's ramp pauses the clips instead of being
    overridden; off with nothing to give back, it doesn't even read the
    ramps (`TestCircadianGammaAfterRestart`). The backlight is never
    written while its clip is off (`levels`). Left: the brightness slider
    gone from the menu with strict mode off is not explained by this;
    confirm both on the laptop (`kc9k circadian`, `journalctl -u kc9k |
    grep circadian`, whether it stops with the daemon stopped). M
    (Strict lockdown's polkit rule also blocks the backlight helper and
    colord, which breaks the same controls while strict mode is on; fix
    done.)
  - [x] `A5` Phrase editor: saving a change said the set has no such phrase
    (owner, again after the first fix: set 2's terms pasted into set 1 of a
    7-set phrase). Causes: phrases stored by older versions (migrated
    combinations, imports) differ from their shown form in case, spacing or
    a spelled-out default length, and remove compared exact forms; the GUI
    skipped the refresh after a command while one was running, so Save stayed
    on with a stale phrase. Fixed: phrases compared by `db.Phrase.Key`
    (gui-state shows it), `phrase-set change` replaces in place in one step
    (free when it only adds terms, R-WEAK-5), GUI refreshes again after a
    stale read and keeps Save off until it lands. Editor buttons: Clear →
    Cancel (asks before dropping unsaved work, also on double-click),
    Delete phrase while a saved one is loaded. `ci/gui_phrase_smoke.py`
    passes here. Merged 2026-11-16 with the overnight session's fix of the
    same bug: a phrase stored in a form today's rules refuse can still be
    changed or removed (`ParsePhraseLoose`), one phrase stored twice is
    listed once, the GUI compares ignoring spacing as well as case and
    keeps Add off for a phrase the set has; `TestPhraseChange`,
    `TestPhraseChangeOldRefusedForm`, `TestStoredPhrasesFoundByCanonicalForm`,
    `ci/gui_phrase_edit_smoke.py` (edit twice in a row, case-only edits,
    old stored rows, refresh and rename mid-edit, unicode, 100 word sets,
    locked block, export/import). Built: desktop 2026-11-20 (Mint 22.3,
    X11, real DB, own GUI instance on :0 driven by xdotool, throwaway
    block with a 7-set phrase and an old raw row `["Porn ", "XXX"]
    ["Free  Video"]… "length":7`): loaded, set 2's terms pasted into set 1,
    Save changes (opened the draft, one `phrase-set change`, rc 0, no
    error); edited twice in a row and a double-clicked Save (one change
    each, Save greyed between); same paste on the old raw row: changed in
    place; Cancel with unsaved work asked, its Cancel kept the work, Drop
    changes ran nothing; Delete phrase asked and removed it; Save edits
    saved the draft. "The set has no such phrase" never shown. Fixed:
    after Delete phrase the deleted phrase stayed in the editor as a new
    unsaved one (Add phrase lit, Cancel asked to drop it; a periodic
    refresh can read the set before the command's result): it now empties
    the editor (`remove_loaded`; `ci/gui_phrase_smoke.py` "editor empty
    after Delete phrase", also refresh first); seen fixed on :0.
  - [x] `A6` Locking an enabled block refused password, restart, delay, range
    and word/own-text locks ("already active; … are set when it starts"),
    against R-LOCK-7. Fixed: `strengthenLocks` adds every kind last in the
    order given (`TestLockActiveBlock`). The GUI's lock dialog offers every
    kind on a running block too (merged 2026-11-16). Checked 2026-11-20 on
    the desktop (devmaster build, X11 :0): CLI, a started block without
    locks, then `start B` with password, restart, delay 2m, range, word
    lock, `--for 3m`, `--lock-text`, one at a time: each refused without
    `--yes` (nothing written), then added last; `kc9k status` unlock order
    password → restart (after it) → delay → range → text → timer (3m after
    it) → another text. GUI, on a second started block, its card's
    Unlocked → Add locks to B: all six kinds in one dialog, Unlock order
    dialog (moved), Set these locks? listed 1 of 6 … 6 of 6; Cancel set
    nothing, Lock set all six in that order; `kc9k unlock` lifted the
    password lock first, then refused the range lock (on now). Fixed: the
    card said "Locked: text lock" with a password lock first, Overview
    pills were in a fixed order, the confirmation said a timer lock behind
    others "can't be stopped until then" (it counts from their unlocking),
    the unlock order said "range": now the lock unlocked next, pills in
    unlock order, `waiting_words`, "range lock" (`ci/gui_cards_smoke.py`
    "A6"); seen fixed on :0. `block info` shows no locks (status does).
  - [x] `A7` Phrase editor UI awkward (§4.3). Owner 2026-11-18: good now.
  - [x] R-PHRASE-1 (changed 2026-11-15): any word set necessary or optional,
    none allowed; length ≥ 2. Built in `db.Phrase`, `match.js`, GUI.
  - [x] Lists and phrase sets may have no action (§3.9, changed
    2026-11-15). Built: the last action can be turned off; no-action rules
    reach the extension with no actions and do nothing.
  - [x] Any trigger to any action (§1, §3.9; owner, 2026-11-16): a
    block's apps and its window titles have one action set each, like a
    list's: their own action (close the app / close the window, the
    default, can be unticked) and close-browser, lock-screen, logout,
    shutdown, monk mode with their times. Apps and windows fire when found
    (at most once a minute); logout, shutdown and lock-screen are
    announced a minute ahead (R-WARN-1). CLI `block trigger`, GUI
    checkboxes under Apps and Window titles, projects. (Place rules had
    actions too for a day; withdrawn, see the top item.)
    `TestTriggerActionsStored`, `TestAppsTrigger`, `TestBlockTriggerCommand`,
    the weakening audit,
    `ci/gui_trigger_smoke.py`. On the laptop: Unchecked. The window title
    help said "in a browser, just that tab"; R-TITLE-1: never browsers.
  - [x] Dark pages from the browser (§4.10; owner, 2026-11-16): the
    toolbar button's popup has "Exempt from dark pages" (or "Show it dark
    again") while dark pages are on; the daemon's `/dark-exempt` keeps
    the list (refused behind an app or settings access password).
    Extension 1.23.0. `TestDarkExemptHandler`, the popup in
    `ci/content_harness_test.js`. In LibreWolf/Chrome: Unchecked.
  - [x] Circadian (§4.10; owner, 2026-11-16): the Settings page had a
    Circadian section beside its own page: gone. Wallpapers sit on a
    timeline, one lane each, dragged like clips in a video editor (bar:
    move; end: start or stop; 5-minute steps, Shift: minutes; past
    midnight it wraps; what a later picture covers is faint). "Play the
    day" plays the next 24 hours on the screen in 15 s (`kc9k circadian
    play`: clips as previews, wallpaper and dark mode at their times),
    then the clips carry on from now and the desktop's wallpaper, dark mode
    and themes are put back. `TestPlayCircadian`,
    `ci/gui_circadian_smoke.py` (drags, draws, plays). On real hardware:
    Unchecked.
  - [x] YouTube playlists (§3.2; owner, 2026-11-16): a huge box on the
    Social media tab. Now a box in the Exceptions card, as tall as its
    playlists (a row each, with Remove). The refusal of a short ID read
    as if an address were wrong: it now says what an ID and an address
    look like. `ci/gui_playlist_smoke.py`.
  - [x] One Import (§4.3; owner, 2026-11-16: the GUI forced mode 1 or 2
    by which Import was clicked). KC9K reads a file's layout (`kc9k list
    read`, `cli/entryfile.go`): a phrase set's Import adds phrases, makes
    rendered terms one word set in its editor, offers sites to Blocked; a
    list's Import sends phrases to a phrase set of the user's choice. List
    import used to store phrase lines, hosts-file lines (`0.0.0.0
    example.com`) and adblock rules (`||example.com^`) as rendered terms:
    now they are phrases and sites. `TestClassifyEntryLines`,
    `TestImportLayouts`, `ci/gui_import_smoke.py` (clicks the Import
    buttons).
  - [x] Statistics bar chart: one colour per site/app over time (§4.9;
    built 2026-11-16): each day's column stacks the top sites or apps
    (Top 5 or 10; at most seven named, the rest as Other in grey), bottom
    up from the most used, in the palette checked with the chart guide's
    validator (light and dark), with a legend naming every series and a
    tooltip per day; a site keeps its colour while the period, Top N,
    search or block filter change, never two shown in one colour
    (`assign_series`); the Most used list's bars take the same colours.
    The GUI gained search and the block filter (`stats --json` now filters
    its rows too; a block's contents behind its block password, §3.11:
    refused with a reason, then read with the password typed); the
    dashboard always reads today unfiltered. Custom time frame (§4.9
    "presets/custom"): `stats --from D [--to D]`; GUI period Custom… opens
    From/To calendars that say why a range is wrong (after the last day,
    still to come, older than the year kept), the chosen range shows on a
    button that changes it. `ci/gui_stats_smoke.py` (draws the chart and
    reads its pixels), `TestStatsFilters`, `TestStatsRange`.
  - [x] Every button clicked (owner, 2026-11-16: "make sure that every
    function actually works in the GUI"): `ci/gui_click_everything_smoke.py`
    clicks each visible, usable button and switch on every page, block tab
    and ⋯ menu, and the Overview's Start menu (dialogs answered Cancel);
    no handler may raise, no command may be one the CLI can't read; the
    messages are printed for review. First run: ~700 clicks, no exception.
    A second round answers every dialog OK as a person could (greyed-out
    buttons aren't pressed), empty fields and no file chosen included: no
    handler raised; it found OK pressable with nothing chosen in a setting
    lock's and Add locks' dialogs and with no address for "Open another
    page" (now greyed out, the reason shown), and Start Frozen Chicken
    failing with a bare "exit status 1" when the screen couldn't be locked
    at once (now a warning naming loginctl: the service relocks within a
    second; a test's database never locks the real screen).
    It found: a draft could be switched on (an allowance set on it, or an
    autostart), after which neither Save nor Cancel could remove it ("it is
    active"), and its schedule, places, allowance, breaks, bypass
    prevention, social media, phrase sets and app/window actions were
    editable though Save drops them. Now no draft ever runs
    (`db.ActivateBlock` refuses, autostart skips it), commands for what a
    draft doesn't hold refuse it and name the block (`refuseDraftEdit`,
    `TestDraftNeverRuns`), and the GUI shows a draft only its Websites and
    Apps (without actions) tabs, saying the rest is changed on the block
    itself. Settings: Lock…/Unlock… beside each
    browser setting's and strict mode's switch (not on a line of their
    own); the access passwords text named old pages (Dashboard, Device
    Blocks); "Turn off…" bypass prevention showed while it was off.
  - [x] Phrase sets in drafts (§4.3 "nothing applies until Save"; found
    2026-11-16): a draft (and a duplicate) didn't copy the block's phrase
    sets, and saving carried no phrase-set changes. Now `DuplicateBlock`
    copies them as stored (`CopyPhraseSets`: also phrases in older forms,
    sets without actions) and `saveDraftPhraseSets` replays each change
    through its command's gate, stronger steps first: new sets, phrases
    added, places ticked, actions on or stricter, lower thresholds, test
    mode off; then phrases removed, places unticked, actions off, test
    mode on, sets deleted (free when empty). `TestDraftPhraseSets`.
  - [x] Rows of controls one per line (R-GUI-3; found 2026-11-16 on
    screenshots): `flow_row` used GTK 3's FlowBox, which asks only for its
    widest child's width, so every wrapped row (Statistics controls, the
    apps pickers, schedule lock, places, allowance and Pomodoro rows,
    recording, Frozen Chicken) put one control per line, stretched, with
    gaps for hidden ones. `WrapBox` lays them out like words: natural
    widths, a new line only when the next doesn't fit (a `Gtk.Fixed`
    with its own sizing; a first version written as a Python container
    hung GTK when a destroyed one was finalized, caught by the smoke
    tests). Checked on screenshots of every page with such rows. Also: a duration's "none"
    meaning too long for its box goes under it ("Only while each window is
    open" was cut off); hidden lists say "N entries hidden" (they hold
    sites too, §3.14 Entry); the Breaks text and `kc9k break` help said the
    reward is "saved up to one allowance" (§3.14: reward; kept as chosen).
  - [x] Android: pick home on a map (owner, 2026-11-16: OpenStreetMap):
    `MapPickerActivity` (Settings → "Pick home on the map…", refused in
    Nokia mode) shows OpenStreetMap tiles (`MapView`: drag, pinch, double
    tap; one tap sets home; radius 50–2000 m drawn as a circle; "Where I
    am"; the OSM credit), Web Mercator in `MapMath` (`MapMathTest`). The
    Android unit tests now run in the cloud (Android SDK, Gradle): 37
    pass; `RulesServerTest` failed once on a port still in TIME_WAIT, so the
    rules server reuses the address (also after a restart on the phone).
    Unchecked on a phone.
  - [x] Durations (R-GUI-1): `DurationField` (days, hours, minutes or
    typed `2d 4h`; out of range or unreadable it says why under the box and
    keeps the text; dialogs' main buttons wait) in every duration: timer
    lock, delay, re-lock, monk mode, list action times, ladder steps and
    quiet time, schedule lock lead, allowance, Pomodoro, breaks, window
    break, Frozen Chicken and its lead, idle timeout, extension grace;
    counts as `NumberField` (text lock length, sessions, ladder count,
    record cut). The block timer lock was capped at 7 days and setting locks
    at 40 (SPEC: no upper limit): gone. `ci/gui_duration_smoke.py`.
  - [x] Timer lock until a calendar date (R-GUI-2): `TimerLockField` (No
    timer / For a time / Until a date: calendar with a year box, time of
    day) in one `LockPicker` used by the block start and setting lock
    dialogs (password asked twice, range lock as day buttons and two
    times), and in the global ladder's lock. CLI: `--until` takes
    `YYYY-MM-DD [HH:MM]` (a passed date refused), durations take `d`/`w`
    (`parseUserDuration`), confirmations say "69 days 10 hours 40
    minutes". The monk mode button never passed `--yes`, so the CLI always
    refused it: now confirmed (R-LOCK-12) and run.
  - [x] `A8` No flashing (R-PAGE-1). Built (extension 1.20.0): while a list or
    phrase set could still act on the top page by its title or text (and no
    address exception spares it), the page is invisible (opacity 0, so it is
    still read) until judged: the first full read 500 ms after its document
    is ready, or after an address change without loading (noticed by the
    page's own DOM changes, before the next paint); a page taken away stays
    hidden until the block page; never longer than 3 s; pages no list reads
    show at once. `ci/flash_test.py` records, at every animation frame,
    whether the page was painted with the word: the old extension painted a
    blocked word for 2–16 frames on load and ~2 s after an address change;
    now none. All browser suites pass in LibreWolf (run in a cloud
    container, 98 checks). Confirm on the laptop: Chrome, and real sites
    (a 0.5 s blank on pages while text lists are on). Owner 2026-11-19:
    every page flashed white on opening or a click, with only small lists.
    Cause: opacity 0 on the root also hid the canvas, so the blank was
    white, dark pages too. Extension 1.27.0 hides what the root holds
    instead; the page's own background stays (`ci/content_harness_test.js`
    checks a dark page's pixel while hidden). Confirm on the laptop. M
    2026-11-20 (overnight, desktop): `flash_test` 10/10 in LibreWolf 159,
    the content harness's no-white-flash checks in Chromium, and the root
    Chrome for Testing test (`TestChromeRunsTheExtension`) pass. Real
    sites and branded Chrome: Unchecked. 2026-11-20 (desktop, Google Chrome
    158 over CDP and LibreWolf 159 over BiDi, headed on :0, a text list
    on): a script recorded at every animation frame whether the page showed
    a listed word, the screen was captured meanwhile. Wikipedia load and
    click, GitHub soft and full load to a listed word: blocked, 0 frames
    with the word, both browsers. Dark sites (GitHub dark, Steam, Spotify)
    in Chrome: 0 white frames, open, soft and full clicks. LibreWolf: a click
    inside Steam showed white ~0.7 s (none with the list off). Cause:
    reading laid the page out before its stylesheets applied, and Firefox
    then paints it unstyled on white. Fixed in 1.27.4: reading waits for the
    body and the page's stylesheets (`styled()`, at most 2 s). After: Steam
    open and clicks, Spotify open and SPA click 0 white frames, blocked
    pages as before; `ci/flash_test.py` 12/12 with a new dark page whose
    stylesheet is late (failed on 1.27.3: 73 unstyled frames). GitHub and
    Twitch stay light in LibreWolf (it reports a light colour scheme).
  - [x] `A9` (Owner 2026-11-18: the design has become tremendous.) GUI overall unclear, hard to read, not pretty or easy to use (owner,
    2026-11-15; R-GUI-3). Redesign page by page. Done: a design layer
    (cards, status pills, page titles with one-line subtitles, ? popovers
    for long explanations, an icon sidebar); Overview rebuilt (status card
    with the longest lock and "Start a block", stat cards, block cards with
    pills); block page header with pills, primary actions, the rest behind
    ⋯; tabs 8 → 6 (Places under Schedule, "Breaks & allowance"); Clusters
    and Frozen Chicken regrouped; long rows wrap, so the window fits 1000
    pixels (was 1213: pages were cut off at the right); lock times in days
    ("69 days 10 h", not "1666 h 40 min"). Breaks & allowance tab
    (2026-11-17): a card each (Allowance, Pomodoro, Reward, Breaks that
    keep the locks) with one line on the page and the rest behind ?,
    labelled rows in a column, one primary button each. Settings: checked
    on a screenshot, already cards of rows. List cards: what a blacklist
    does is one line ("Does: Block page, Greyscale") with Change actions…
    opening the fourteen tick boxes, so its entries come first.
    Circadian: the page title and a line like every other page, the long
    explanations behind ?. Phrase set cards the same as list cards.
    Left: the owner's verdict on a real screen. L
  - Strict lockdown polkit now allows the backlight helper and colord.
  - [x] Hotkey locks (§3.4 timer lock "no upper limit", §3.13, R-GUI-2;
    found 2026-11-16): a hotkey's timer lock was capped at 24 hours and the
    GUI took it as typed text ("ctrl+alt+s study 1h", no calendar, block
    names with spaces broke). Now a hotkey takes every lock `start` takes,
    in the same unlock order, no cap (`hotkey add K B --lock --for 2h …`;
    a password lock's password is given when it is added); the GUI's Add
    hotkey… dialog has keys checked as typed, a block chooser and the
    usual lock picker. `cli/hotkeys_test.go`, `ci/gui_hotkey_smoke.py`.
  - [x] GUI gave a password lock the settings password (R-LOCK-12, §3.11;
    found 2026-11-16): with a settings password remembered and `password
    locks` off, the GUI's start with a password lock sent the settings
    password first and the CLI took it as the lock's. Now sent with
    `--settings-line` and always read as the settings password
    (`TestSettingsLine`).
  - [x] A scratch database (`KC9K_DB`, GUI smoke tests) set hotkeys on the
    real desktop, clobbering KC9K's own; now it never does.
  - [x] Places couldn't be used (owner, 2026-11-16; §3.6): "Turn on at
    this place" refused a block with nothing to enforce yet (its lists
    without actions count as nothing), so a saved place never reached "On
    at"; the help said a place alone keeps the block from being stopped
    (only a schedule lock does). Now a rule goes on any block; saving a
    place turns the selected block on there in the same step (ticked by
    default); the user picks how many of a place's networks must be in
    range (default 2; `place need`, GUI field, projects). `TestPlaceNeed`,
    `ci/gui_places_smoke.py` (fake nmcli: an empty block, both buttons,
    networks needed, no networks in range).
  - [x] An allowance protected its block like a lock (R-LOCK-1, R-LOCK-10;
    found 2026-11-16): a block with an allowance and no lock couldn't be
    switched off, weakened or uninstalled, the daemon switched it back on,
    and raising or removing the allowance waited for the next refill (for a
    rolling allowance a raise never came). Now an allowance protects
    nothing by itself: unlocked, every change applies at once; locked, it
    only takes less time or fewer sessions; removing it is always free and
    leaves the block on; the daemon never switches it. `allowance set`
    flags not given keep the allowance's (a weekly allowance stayed weekly
    only if --per was repeated). `TestAllowanceProtectsNothing`.
  - [x] Words SPEC §3.14 avoids, in messages and help (found 2026-11-16):
    the default list "Page words" is now "Rendered terms on page" (renamed
    once in every block, `migrateWordsListName`); `block add-term` /
    `remove-term` (old names `add-keyword` / `remove-keyword` stay as
    aliases, §4.12); "keyword" → rendered term / window title entry;
    "typing challenge lock" → text lock; "quota", "loosening", "held",
    "bypass detection", "option" in messages → allowance, weakening,
    locked, bypass prevention, behaviour setting; GUI "Keyword test mode" →
    "Test mode". `start`'s help listed text locks as 10–5000 characters and
    `--until` as a clock time only; `place`'s said a place rule alone
    makes a block unstoppable (only a schedule lock does, R-LOCK-10).

- [x] **Lock-bypass audit 2026-11-15** (R-LOCK-1, §3.5): every command
  that could weaken a block tried on a locked block that has something of
  everything (`TestLockedBlockRefusesEveryWeakening`). Two holes found and
  closed: `allowance set` on a locked block was taken (a new allowance, or
  a larger one at the next refill: up to a whole day unblocked); the first
  `schedule add` on a locked continuous block made it enabled only in that
  window. Both now refused while locked; another window on a scheduled
  block and a smaller allowance stay free
  (`TestLockedScheduleAndAllowanceStrengthening`). The old `block.go`
  commands called `os.Exit`, so they couldn't be tested in-process: they
  return errors now (same message).
- [x] `A10` **Lock model** (§3.3, §3.4, R-LOCK-1–11). Done: schedule lock
  (`kc9k schedule lock B on --lead D`) replaces built-in schedule/place
  protection, 2 h edit freeze and 24 h allow-window queue (upgrade gives
  scheduled/placed blocks a 2 h-lead schedule lock); freeze schedules have
  `--lock-lead` (old ones 2 h); `unlock` lifts text/password locks, leaves
  the block on; delay lock counts from `unlock` (`delay_until`); timer lock
  end lifts only the lock; trusted time = real clock unless `kc9k tamper
  clock` (then clock changes ignored during timer locks, powered-off time
  not counted; old installs have it on); `break text` (alias `relock`);
  re-lock for block text/password locks (`start --relock D`); uninstall
  refused only while locked; re-lock on setting locks (browser settings,
  tamper, strict mode, global ladder: `--relock D`; unlock lifts, setting
  stays on; off meanwhile ends it); layered order on blocks (R-LOCK-6/7:
  `db/layers.go`, order of the lock flags, `unlock` lifts only the first,
  duration timer and restart behind others count from their unlocking, new
  locks last; the daemon moves layers on; old blocks: timer, restart,
  password, text, delay), and on setting locks (browser settings, tamper,
  strict mode, global ladder); active vs enabled for schedules and places
  (`enabledBlocks`; the daemon no longer switches blocks; upgrade switched
  scheduled blocks on once); allowance sessions lift their block (it stays
  on); the GUI block start and setting lock dialogs ask the unlock order.
  Clock check built 2026-11-20 (overnight, desktop): powered-off time
  counts again once internet time (two of three HTTPS sources agreeing)
  proves it, never more than the hardware clock says; anchors per boot
  (`db/clock.go` `CheckClock`, `enforce/clockcheck.go`, daemon every 5
  min; `TestCheckClock`, `TestAdvanceClockKeepsAnchorFloor`,
  `TestClockCheckEndsLockAfterPoweredOffTime`,
  `TestClockAnchorDoesNotShortenNextLock`, `TestAgreedTime`,
  `TestCheckClockAsksOnlyWhenDue`). The live query answered here (1.6 s
  off, RTC exact). Real power-off checked 2026-11-21, kc9k-vm (Ubuntu
  24.04, Cinnamon X11), `tamper clock on`, a block timer-locked `--for 20m
  --yes`, anchor taken within 5 min, `virsh shutdown`: off 25 min, booted
  past the lock's end, still locked until the check 30 s after the daemon
  started ("clock check: 25m53s of powered-off time counted"), then no
  timer lock in `kc9k status`. No internet after boot (link cut from the
  host, off 15 min): two checks failed ("no two sources agree", 0
  answered), lock held 9 min past its real end (trusted 05:11 at real
  05:28); link back, next check counted 17m13s and the lock ended. Built. (L)
- [x] **Android extension can be switched off** (§3.13): built (5 s grace
  once checked in, pauses doubling 2 min → 1 h, reset after 10 min unbroken;
  `ExtensionGuard`). Unit tests run in the cloud 2026-11-16:
  `ExtensionGuardTest` (7) and the other 30 pass. On a phone: §0.2. (S)
- [x] **Page reading slow on big pages** (principle 9). Done: the first
  read of a page goes in ≤ 8 ms pieces in idle time (`readPageText`,
  `READ_SLICE_MS`), then only added visible text, full re-read ≤ every 30 s;
  one-word terms via `kc9kWordIndex`; phrases split the text once per page
  and look words up in an index (100 phrases + 1,000 terms on a 3,000-word
  page: 115 ms → 4 ms, timed in `ci/match_test.js`). Judging is in pieces
  too (2026-11-17): `kc9kListMatchesSteps` (and `kc9kTextHitsSteps`,
  `kc9kPhrasesFoundSteps`, `kc9kPhraseTokenizedSteps`) yields every 1,000
  words, term of several words and phrase; the content script runs it
  READ_SLICE_MS at a time (`judgeOn`, `judged`), others run it to the end.
  A phrase with every word set optional allocated a slot per word of the
  sentence for each phrase (a page without full stops: 40,000 per phrase,
  garbage collection pauses of 30 ms): now skipped unless enough sets are
  found, and sparse. 40,000 words, 1,000 terms, 100 phrases: slowest step
  3–5 ms (`ci/match_test.js`); in Chromium the page was held up at most
  12 ms at a time, 150 ms before (`ci/content_harness_test.js`, which
  fails on the old code). Extension 1.24.0. In LibreWolf: Unchecked.
- [x] `A11` **Gigantic URL lists** (principle 9). Extension: done (`kc9kIndex`,
  timed at 100,000 in `ci/match_test.js`). Firewall: done (nft sets,
  iptables fallback); the CLI no longer fills it (`deferFirewall`), the
  daemon does within a cycle with its 30 min lookup cache, so `start`
  doesn't wait for DNS. `/etc/hosts`: measured 15 ms per name lookup at
  100,000 sites, so capped at `MaxHostsDomains` (20,000, ~3 ms; see §5);
  drift check one pass (was quadratic). 100,000-site test:
  `blocker/biglist_test.go`. Browser policy (2026-11-17): Chrome and
  Chromium take only the first 1,000 entries of URLBlocklist/URLAllowlist
  (Chrome for Testing in a cloud container, 100,000 entries: the 500th
  blocked, the 1,500th and 50,000th not), so past 1,000 the policy holds
  what /etc/hosts can't block first (patterns, file URLs, sites past its
  cap), at most 1,000 (`ChromiumPolicyLimit`, `TestChromiumPolicyLimit`);
  an allowlist past 1,000 exceptions blocks the rest in Chrome.
  Built, checked 2026-11-21 (desktop, Firefox 146, LibreWolf 159): block
  of 100,000 `.invalid` sites (`block import-list`). Found and fixed:
  every `kc9k` command took 1–4 min (the list join at each open read and
  deduplicated every entry one by one; now only a name pair's,
  `TestPrinciple9_OpenWithHugeList`), so the GUI's polls piled up to 430
  processes; import one transaction per call (100,000 in 10 s); the
  daemon used half a core for good (each cycle read the entries several
  times without an index and rebuilt 200,004 policy patterns): now an
  entries index, `entryCache` (versions set by triggers), `Inputs` and
  policy sets kept per key, hosts check kept (`TestPrinciple9_EntryCache…`,
  `TestInputsKeptForTheSameBlocks`, `TestPolicySetsFollowThePolicy`).
  Firefox and LibreWolf take only the first 1,000 WebsiteFilter entries
  (200,004 written: the 1,000th blocked, the 1,001st not) and its 7.4 MB
  slowed LibreWolf's start (6–8.5 s against 3.8–4.2): capped like Chrome
  (`FirefoxPolicyLimit`, `TestFirefoxPolicyLimit`; 38 KB, the sites just
  past the hosts cap). Firewall lookups never hold the cycle (background,
  `ApplyFirewallKnown`); an NXDOMAIN is kept 30 min (at 2 min the daemon
  looked up for good). Now: `start` 1.5 s, other commands 0.2 s; /etc/hosts
  20,000 sites; LibreWolf: 1st blocked (policy/hosts, then extension),
  50,000th and 100,000th by the extension (ready ~10 s after the start, as
  with 1,500 sites); Firefox (no extension): 1st by hosts, 20,001st–21,000th
  by policy, a site past both (example.org) dropped by the firewall.
  Browser memory as without the list (LibreWolf 1.1–1.2 GB, Firefox
  0.9 GB). Daemon: 4 % of a core (1.5 % without the list), 70–90 MB;
  13–17 % during the background lookup run after each daemon start
  (100,000 names: 11 min; the firewall holds those sites only after it).
  Block removed: policies (sha1 5e3f0e900db3) and /etc/hosts as before. (M)
- [ ] `A12` **R-LOCK-12 gaps**: manual `kc9k monk` now confirmed (`--yes`), and
  `project import` refuses monk steps/actions without `--no-confirm-ok`
  (`projectMonk`); both in `confirm_test.go`. `block import` the same. The
  hotkey press dialog (`blocker.ConfirmDesktop`) ran live on the laptop
  (2026-11-15): zenity shows "Lock 'B'?" with target, kind and length;
  unanswered (screen locked) it times out after 60 s and nothing is locked;
  synthetic keys from other programs are ignored. Left: a person clicks
  Lock and Cancel on an unlocked screen. (S)
- [x] `A13` **`install.sh` races the daemon for the DB.** Seen 2026-11-19 on a
  fresh Ubuntu 24.04 VM: step 3 (`kc9k-bin browser install-librewolf`)
  failed with `database is locked (SQLITE_BUSY)`, apparently while the
  just-started daemon created the database; running it again afterwards
  worked. Find why the 5 s `busy_timeout` did not cover it and fix it;
  prove it on a fresh install in the VM (snapshot `clean` has kc9k
  installed: remove it first, or start from a VM without it). (S)
  Fixed 2026-11-20 (overnight, desktop): busy_timeout doesn't cover a
  write after a read whose snapshot another process has changed (SQLITE_BUSY
  at once), and two processes could both find a column missing and both
  add it ("duplicate column"). The schema setup in `OpenDB` now runs under
  an flock on `<db>.setup-lock`. `TestFreshDatabaseOpenedByManyAtOnce` (four
  processes create one new DB; failed without the lock). On the desktop:
  the installed build, DB deleted, daemon start + `install-librewolf` + two
  `status` at once, 10 times: no error; `install.sh` with no DB: no error.
  Checked 2026-11-20 on kc9k-vm (Ubuntu 24.04, Cinnamon X11): from
  snapshot `clean`, `make uninstall`, `/var/lib/kc9k` and
  `/usr/local/bin/kc9k*` removed (3 of 4 runs also LibreWolf and its
  extrepo source), then `sudo TAGS=devmaster ./install.sh`; 4 runs,
  reverted in between: exit 0, no SQLITE_BUSY, "database is locked" or
  "duplicate column" in the output or the daemon's journal, step 3
  "LibreWolf installed" (once "already installed"), `kc9k status` fine,
  daemon active. A new DB's setup alone takes 8–13 s on that disk; the
  daemon reached its loop 13–20 s after starting (beside
  `install-librewolf`; the setup lock waits ≤ 30 s). Then 10 times: daemon stopped, DB deleted, daemon start +
  `install-librewolf` + two `status` at once: all exit 0, no error. Not covered:
  a deferred transaction that reads then writes (some migrations, runtime
  code) can still get SQLITE_BUSY at once beside the daemon's writes;
  `_txlock=immediate` would close that (not done: changes every `Begin`).

### 0.2 Not yet confirmed in real use

Script-tested, never by a person. Script what can be into `ci/`.

- [x] `B1` Notification before an allowance session ends (§3.6, §4.10
  notifications): `ci/allowance_notify_test.sh` (a test block with
  gnome-calculator, 3 min allowance, advance 2 min, the app kept focused
  with xdotool input, dbus-monitor on the session bus). Run 2026-11-20 on
  the desktop (Mint 22.3, installed build, real daemon): "'kc9k-b1test': 2
  minutes of today's allowance left" after 58 s of use, the session over
  and the app closed after 177 s. Looking at the bubble on a screen: not
  needed (notify-send's text is what was checked).

- [x] `B2` Scheduled Frozen Chicken in logout/shutdown modes (§3.8): Built.
  2026-11-20, kc9k-vm (Ubuntu 24.04, Cinnamon X11), `TAGS=devmaster`,
  `freeze schedule add --days daily --from … --to … --mode …` 2 min ahead.
  Logout: at the window the session ended within 1 s, greeter shown. Found:
  a login during the freeze (greeter, then ssh) was ended again every
  cycle, and once the ssh session closed logind spun at 100% CPU (systemd
  255): after the freeze an ssh login took 2 min ("Failed to create
  session"), `loginctl` timed out, until a reboot.
  Fixed: a logout freeze writes `/run/nologin` (pam_nologin) and ends only
  users not already closing. Again: greeter login refused ("Failed to
  authenticate"), ssh refused with "Frozen Chicken until 06:26", the file
  gone at the end, a greeter login then opened the desktop, logind at 0 s
  CPU. Shutdown: the VM powered off ~5 s into the window; booted during
  it twice, powered off 6 s and 1 s after the daemon started (17-18 s
  after `virsh start`); booted after it, stayed up with autologin.
- [x] `B3` Strict mode holes: Built, 2026-11-20, kc9k-vm (Ubuntu 24.04,
  Cinnamon X11), throwaway block (app `b3app`) timer-locked 4-30 min,
  strict on, a loop per account copying `true` to its writable folders
  every 5 s. Before the fix: `unshare -Urm` mounted an exec tmpfs and ran
  a copy with the AppArmor userns restriction at Mint's 0 (Ubuntu's 1
  holds it); copies ran from `/run/user/<uid>` and `/var/crash` (all
  accounts); a `--user` Flatpak (own busybox runtime) started before kept
  running its `/app` helper (a fresh start was refused: "execvp b3test:
  Permission denied"); a guest (`guest-account add`, uid 999) kept
  `b3app` (43 s, others killed each cycle) and the bind hid a logged-in
  guest's home (root-owned empty folder); an unpacked Electron app
  (resources/app, no asar) was closed as a browser after 60 s, also with a
  locked block off; lifting the lock unmounted every guest's home. Fixed:
  strict sets the restriction to 1 (`unshare: write failed
  /proc/self/uid_map`, back to 0 after); runtime folders and guest homes
  noexec (both a guest logged in before and one 80 s into the lockdown);
  the Flatpak and a `sleep` copy started from `~` before were killed in
  the first cycle ("closed 3 processes"), a `sleep` copy in `/var/crash`
  within 4 s; guest and other account's `b3app` killed; the Electron app
  ran 100 s with no close, a renamed portable LibreWolf still judged a
  browser; after the lock: both guests' homes mounted and running
  programs, the exempt folder with no noexec copy left, `/run/user` exec,
  sudo back. Cinnamon logged in again during the lockdown (lightdm
  restart) with `/run/user/1000` noexec: panel, Nemo, gnome-terminal
  fine. Known limit: a short program copied to a world-writable folder outside
  the noexec ones still runs once; Electron apps in an exempt folder and
  the restriction (§5).
- [x] `B4` Shutdown mode and `kc9k.nofreeze`; measure fast relock. Built.
  2026-11-20, kc9k-vm (Ubuntu 24.04, Cinnamon X11), `TAGS=devmaster`.
  Lock mode: `freeze --for 25m --mode lock` locked at once; five unlocks
  (`cinnamon-screensaver-command --deactivate`) locked again after 0.42,
  0.62, 0.34, 0.64, 0.34 s (state polled every 50 ms); reboot during it:
  autologin session at 14.2 s after the kernel, screensaver up 14.98 s,
  locked 15.42 s. Shutdown mode: switched on mid-session, "System is
  powering down" 3 s later; at each boot the daemon's first cycle powered
  off before any login (unit started → powering down: 10.8 → 13.7 s,
  17.9 → 22.9 s, 22.4 → 24.0 s after the kernel; `virsh start` → off
  25.3 s, BIOS and GRUB included). Daemon start → first cycle 0.5 s on a
  running system, up to 4.7 s while booting. `kc9k.nofreeze` added in
  GRUB (e, end of the `linux` line, F10): stayed up, screen not locked,
  `mode show freeze` "on (shutdown) until 10:16; it can't end early"; the
  next plain boot powered off again; the first boot after 10:16 stayed up,
  "Frozen Chicken is off" (time counted while off). A power-off with a
  session open took 90 s more: the test's own monitor script held the
  session scope (systemd's stop timeout), not kc9k. Found: extending a
  running lock freeze with `--mode apps` switches it to allowed apps only
  and the screen stays unlocked (6 checks over 12 s), so from a text
  console a lock freeze can be weakened (§5).
- [ ] `B5` Rendered terms on real sites for days; hotkey on a real keyboard; hotkey under GNOME.
- [x] `B6` Websites tab on a real screen. (Address change without load: a
  `history.pushState` page is caught by a URL list in LibreWolf,
  `extras_test.py`.) Lists are taller by dragging the grip under them
  (owner 2026-11-19; height kept per kind in gui.json,
  `ci/gui_list_resize_smoke.py` under Xvfb). Built, checked 2026-11-20 on
  the desktop (Mint 22.3, X11 :0, devmaster install of this tree).
  Websites tab of a throwaway block (60 entries) in kc9k-gui, screenshots:
  tabs, Test mode, the blacklist card, cover "60 entries hidden", grip
  under the list. xdotool drag down: list 120 → 330 px, gui.json
  `"website list": 330`; up: 254; back to All blocks and in again: 254.
  Found: the block's exception list (same kind) stayed 120 until the GUI
  restarted (its card is kept while the block is open again). Fixed:
  `ResizeGrip.set_height` gives the height to every list of that kind on
  screen (`ResizeGrip.live`); smoke test checks it (failed before). After
  reinstall and a GUI restart: blacklist opens 254; drag to 160 → the
  exception list 160 at once; double-click → both back to 120,
  `list_heights` emptied. YouTube on the real site (LibreWolf headless,
  Marionette, URL list entry `youtube.com/watch`): block off, clicking a
  search result goes to `/watch?v=…` with a marker on `window` kept (no
  load); block on, the same click → the block page ("Stopped by the list
  Blocked of the block b6yt") at the first poll (< 0.25 s); the results
  page itself opens. Block, gui.json as found.
- [x] `B7` One-off logout/shutdown from a list; Dashboard monk button. (Checked
  2026-11-15: a timer-locked global ladder refuses `bypass off` and a weaker
  ladder, and turns off once the lock ends.) Built: 2026-11-20, kc9k-vm
  (Ubuntu 24.04, Cinnamon X11), this tree installed `TAGS=devmaster`; a block
  with `example.net` in its Blocked list, action logout (once), then shutdown
  (once), tripped by typing the address in LibreWolf on :0 (xdotool). Logout:
  the extension closed the browser at once, the daemon logged the user out
  60 s after the hit (10:37:48 → 10:38:48). Shutdown: off 60 s after the hit
  (10:41:37 → 10:42:37, powered off by 10:42:39). The 60 s notice (R-WARN-1)
  first never showed: `NotifyDesktop` needed `notify-send`, which Ubuntu's
  Cinnamon lacks (libnotify-bin only a Recommends of the .deb). Fixed: else
  `busctl --user` calls the notification service
  (`TestR_WARN_1_NotifyWithoutNotifySend`); then "you will be logged out in
  60 seconds" / "the computer will shut down in 60 seconds" showed on screen
  and on the session bus. Overview's "Monk mode…" (xdotool): 2m, OK, then
  "Set this lock?" (R-LOCK-12) with target, kind and end; Cancel left monk
  mode off; Lock started it within 1 s (link down, 1.1.1.1 silent, card
  "Monk mode 1 min"), ended 10:36:30 and the network came back. The GUI's
  confirmation is a plain dialog: synthetic clicks count (only hotkey
  confirmations ignore them).
- [ ] `B8` Use counting with a person: focused tab/app count, idle not; other browser closed during a website session.
  2026-11-20 (overnight, desktop, Mint 22.3): idle was never seen there:
  cinnamon-screensaver 6.6 answers `GetSessionIdleTime` with "not
  implemented", so the check said "there" and idle time counted as use (and
  earned break rewards). Fixed: idle from the X server (Xwayland not
  trusted), else Cinnamon (`blocker/idle.go`; `TestX11IdleMillis` on Xvfb,
  `TestAwayFromDesktopUsesX11Idle`). On the desktop's :0 as root:
  637 s idle → away, after a mouse move → there. Focused app counted with
  simulated input: `ci/allowance_notify_test.sh` (B1). Left: with a person
  (real keyboard), and a website session. The X server's idle time now
  comes before the session bus's (the user's own: an answer there saying
  "idle" stopped use being counted). "Screen locked" was asked only on the
  session bus (`GetActive`), so a program holding `org.cinnamon.ScreenSaver`
  that said "locked" stopped allowance use being counted: fixed 2026-11-21,
  the lock now comes from logind's `LockedHint` (`lockedByLogind`,
  `TestScreenLockedByLogind`); the session bus isn't asked. Unchecked:
  that cinnamon-screensaver sets `LockedHint` on the desktop (Needs: box,
  `loginctl show-session <id> -p LockedHint` with the screen locked; if it
  doesn't, a locked screen counts as there until idle, 3 min).
  2026-11-21: `ci/lockedhint_check.sh` (box: run as yourself from a
  terminal on the Cinnamon desktop) locks the screen, reads `LockedHint`
  for your x11/wayland sessions, waits for you to unlock and prints
  PASS/FAIL per check (no while unlocked, yes while locked, no again).
  Script written, never run (tried here only against fake `loginctl` and
  `cinnamon-screensaver-command`: both outcomes reported right).
- [x] `B9` Mute and dark on real sites; polite close with a real app.
  Checked 2026-11-20, desktop (Mint 22.3, LibreWolf 159, devmaster),
  a throwaway block (lists by url, block action off). Mute (youtube.com):
  "Me at the zoo" plays (13 s in), not blocked, not dark; the tab muted
  by `blocker@kc9k`, not audible, no LibreWolf sink input in `pactl list
  sink-inputs` over 12 s; block off: unmuted, audible, one sink input;
  nothing in /etc/hosts. Dark (wikipedia.org): page turned over, photos
  as they are, block off: light again. Found and fixed: a picture in a
  `<picture>` was turned over three times (a negative; theguardian.com
  photos) and SVG logos turned back (Wikipedia's black wordmark vanished
  on the dark page); now each picture turns back once, SVG `img` turn with
  the page (extension 1.27.2; `ci/extras_test.py` counts the turns; checked
  again on wikipedia.org, theguardian.com, bbc.com). Polite close
  (`polite-close` on for the test): xed with unsaved text asked, its save
  dialog opened, killed (SIGKILL) ~5 s later; gnome-calculator asked and
  closed itself (exit 0) in 3.5 s.
- [x] `B10` No-video from page text on a real embedded player (YouTube/Vimeo iframe), in LibreWolf and Chrome (`extras_test.py` checks a local one in LibreWolf).
  2026-11-20 (desktop, LibreWolf 159 over WebDriver and Google Chrome 158
  over CDP, headed on :0, fresh profiles): a local page embedding the real
  YouTube and Vimeo players (autoplay, muted), a text list with no-video
  and a word on the page. Without the word both play (YouTube 0.9 → 36.7
  s, Vimeo 0.4 → 25.8 s in 40 s). With it, both browsers: both frames
  `display: none`, the videos inside paused at 0 s, 20 s on. The word
  added 30 s after load: within 5 s both frames hidden and paused (YouTube
  stopped at 28.6 s LibreWolf, 2.4 s Chrome), and stayed so. Found in
  Chrome: the page after one that earned no-video kept its players hidden
  (a user sheet left in the document). Cause: Chrome stacks a sheet
  inserted twice (the frame's ask, then the load events) and `removeCSS`
  takes one copy. Fixed in 1.27.5 (`cssGiven`; `node
  ci/looks_background_test.js`, fails before): the next page shows and
  plays both players at once. `ci/extras_test.py` 35/35 in LibreWolf after
  the fix. Vimeo shows a bot check to WebDriver-driven LibreWolf after a few
  loads (not KC9K).
- [ ] `B11` Social media settings beyond Shorts (`social.js` selectors written from memory); playlist videos playing; block following places; text break returning.
- [ ] `B12` Chrome on the laptop. Checked 2026-11-15: the extension checks in from
  Chrome and Chromium, both show the block page; the policy force-installs
  and pins it, turns off incognito, guest and developer mode. 2026-11-20,
  desktop (Google Chrome 158, fresh profile on :0 driven over CDP, xdotool
  clicks): in 3 s the extension runs; `chrome://policy` all OK; Extensions
  page: "Installed by enterprise policy", on/off switch greyed, no Remove
  (card and details), a click on the switch leaves it on;
  `management.setEnabled(false)` and `management.uninstall` from that page:
  "cannot be modified by user"; unpinning ignored; the button's right-click
  menu: "Installed/Pinned by your administrator", no Remove. Popup: a click
  on the button over example.com showed "Nothing to show for this page."
  (Chrome's window isn't the focused one while its panel is open, and the
  background's front tab asks for a focused window). Fixed in 1.27.6: the
  popup sends its own window's front tab (`popupTab`; `node
  ci/popup_background_test.js`, fails before); then Chrome and LibreWolf
  show "example.com", "No block covers this site right now." and the add
  menu. Left (owner): a person clicks the button and tries to remove the
  extension.
- [x] `B13` Block unsupported browsers (`kc9k tamper unsupported-browsers`).
  Checked 2026-11-15: Firefox is closed within ~2 s with the reason logged;
  LibreWolf with the extension stays (browser_test); `kc9k browsers install
  chromium` installs Mint's chromium (`/usr/lib/chromium/chromium`, policy
  in `/etc/chromium/policies`) and works. Built, checked 2026-11-20:
  upgrade in `kc9k-vm` (desktop: A6's restart locks would hold the setting
  on): fresh DB, `install.sh` of c19f40e (before b4304c7), `kc9k browser
  force-librewolf on --for 7m`, then `install.sh` of this tree: `tamper`
  "unsupported-browsers on (locked: until 13:58)", `off` (and the
  `force-librewolf` alias) refused until then; after it, `off` worked and
  no lock row was left. Same with the old binary on a scratch DB on the
  desktop. Portable LibreWolf (official tarball 146.0.1, AppImage):
  reads its own `distribution/policies.json`, never gets the extension. In
  the VM, setting on: both closed within ~2 s ("not a supported browser"),
  the installed LibreWolf with the extension stayed. Desktop, setting off,
  blocks locked: closed after ~62 s ("no Killer Chicken extension").
  Found: an AppImage's files sit on FUSE, shown as root's and closed to
  root, so its exe counted as installed by root: never judged by engine (a
  browser repacked under another name ran without the extension); now an
  exe on FUSE or closed to root is the user's (`TestUserPlacedExe`,
  roottest `TestGuardJudgesAppImageBrowser` with the real AppImage: fails
  before, passes).
- [x] `B14` Monk mode live, incl. reboot mid-way (R-MONK-2). Built:
  2026-11-20, kc9k-vm (Ubuntu 24.04, Cinnamon X11), this tree installed
  `TAGS=devmaster`: `ci/monk_live_test.sh host kc9k-vm` PASS, 6 min. Before:
  radio enabled, `enp1s0` up, online. Within 2 s of `kc9k monk --for 6m
  --yes`: radio off, `enp1s0` down, firewall drops, 1.1.1.1 silent, loopback
  and the rules server answer. Undone by hand (link up, `nmcli radio wifi
  on`, `nft delete table inet kc9k`): each cut again within 3 s; `mode off
  monk` refused (locked). Restart mid-way: 20 s after boot all three held
  ("enp1s0 was brought back up; down again"). At the end (23:02:24, +4 s):
  radio back on, `enp1s0` up, online, `kc9k monk` off. The VM has no Wi-Fi
  card: the radio checked is NetworkManager's switch only.
- [x] `B15` GUI design pass: seen by the owner on a real screen (2026-11-18: tremendous).
- [x] `B16` Lists without types (§3.2): checked 2026-11-15: `H` and `test` have
  one `Blocked` each; `H` blocks reddit.com by address and a page by its
  text in LibreWolf. 2026-11-20, desktop (X11 :0, devmaster install of
  06f44ac): throwaway block, Websites tab, "New blacklist…": a name and
  URL/Title/Text/Ads boxes, no list type; OK greyed out without a name or
  without a box. "Mixed" (URL + Text) made, `b16test.example *b16pat*
  kcbsixteen` added in one field, Save edits: `block info` shows `Mixed
  (blacklist) looks at: url, text` holding all three beside `Blocked`.
  Started: /etc/hosts got b16test.example; headless LibreWolf blocked
  b16test.example and 127.0.0.1/b16pat/ by address (policy, then the
  block page) and a local page by the word in its text
  (`reason=keyword`); a plain page opened. Block removed, hosts cleared.
- [x] `B17` Active vs enabled on the laptop: checked 2026-11-15 with a throwaway
  block: off it stays out of /etc/hosts in its window; on, it is in /etc/hosts
  in its window, and outside it shows "lifted by its schedule" and leaves
  /etc/hosts. GUI on a real screen: Built, checked 2026-11-20 on the desktop
  (X11 :0, installed build = HEAD's code). Throwaway block, one entry, window
  daily 03:00-04:00, started without locks at 23:47: `status` "[lifted] [lifted
  by its schedule]", not in /etc/hosts; Blocks page card "On, lifted now",
  "Automated block"; block page pill "Lifted now" and "Active, lifted ·
  automated · lifted by its automation now". Window 23:00-24:00 added: within
  ~10 s `status` "[enabled] [today: 23:00-24:00]", in /etc/hosts, block page
  pill "On" and "Active, enabled · automated · today: 23:00-24:00". Window
  removed: lifted again on both, out of /etc/hosts. Block stopped, windows and
  block removed.

## 1. `SPEC.md` coverage

Only items not simply Built.

| Item | Status | Notes |
|---|---|---|
| §3.1 File URL entries | Built | built: with tamper `file-urls` on, blacklist `file:///…` entries go into Firefox/LibreWolf and Chrome policies unless the block's file exceptions spare them (checked live in LibreWolf). Chrome live 2026-11-21 (cloud container, Chrome 158.0.8127.36, daemon as root): Chrome reads a policy path as a plain prefix and `*` in it literally, so `file:///…/games/*` in URLBlocklist had never blocked anything (Chrome for Testing 153 with a test policy: `…/d1/*` loaded, `…/d2/`, `…/d3` and an exact file were refused; `file:///*` loaded, `file:///` and `file://*` refused); only the extension caught the page. Fixed: Chromium-family policies get the entry without its trailing `*` (`chromiumFilePatterns`, `TestFilePatterns`); then `games/game.html` was refused by Chrome itself (`ERR_BLOCKED_BY_ADMINISTRATOR`), `notes/ok.html` opened, a file exception `file:///…/games/*` took the entry out of the policy and the page opened, and with `file-urls` off every file opened. The extension's block page named the title for a file taken by its address (Chrome sends no request event for files, so `blockTitledTab` judges both); fixed, extension 1.34.0 (`ci/background_test.js`), seen live: no `reason=title` for the address, `reason=title` for a file whose title had the blocked word. The extension judges file pages too (2026-11-21, SPEC §6): rules `file_pages`, `files`, `except.files` (`withFilePages`); `kc9kFileMatch`, `kc9kJudgedAddress`; by address, title and text (`ci/match_test.js`, `TestRulesFilePages`, a real `file://` page in Chromium in `ci/content_harness_test.js`). In LibreWolf: Unchecked (Needs: box). Chrome runs no content script on files unless the user allows it (no policy for that): there only the address and title (background) |
| §3.2 Block preset | Built | your own saved blocks: `block-preset save B P`, `block-preset`, `block-preset remove P`, `block create X --preset P` (old names `template`, `--template` kept as aliases); GUI ⋯ "Save as block preset…" and a choice in New block; Presets page; projects (a choice, always in a backup). None shipped |
| §3.2 Affected users | Built | `block users B all|USER…` (GUI "Users:" button, gui-state `users`/`accounts`, projects): a block for some accounts stays out of hosts/firewall/policies (shared by every account) unless it covers them all; its lists reach only those users' extensions (`BuildRulesFor`, user from the socket uid) and its apps are closed only in their processes (`KillBlockedFor`), its window titles and folder entries judge only their windows (`TitlesByUser`, Xvfb-tested), its sites close only their fetch tools (`KillNamingFor`), and the extension guard closes only their browsers unless something needs it everywhere (`extensionUsers`, 2026-11-21). Fewer users weakens. The GUI's "isn't running" dialog asks for its own account (`ExtensionRequiredNowFor`, `callerUID`: sudo's user; its browsers only), though browsers' check-ins are kept per browser, not per account. Live: one account on the laptop, so only "every account" checked (Unchecked; Needs: a second account on the test machine) |
| §3.3 Continuous/automated types | Built | the type follows the block's windows and place rules (SPEC defines it so; no field to set), in a mode its mode's (the mode's automation is written into its blocks); breaks, allowance sessions and schedules lift, never switch; gui-state `continuous`, the card's "Continuous block"/"Automated block". `TestBlockTypeContinuousOrAutomated` (2026-11-21) |
| §3.4 Range lock | Built | blocks (`start --range "mon-fri 09:00-17:00" [--range-invert]`) and setting locks (browser settings, tamper, strict, bypass, freeze schedules: `--range`); a layer that holds while in range and can't be unlocked; several stack (2026-11-21, also given twice in one command). GUI: block start dialog and the setting lock dialogs |
| §3.4 Layered sequential locks | Built | blocks and setting locks (CLI flag order); GUI start and setting lock dialogs ask the order (Up/Down); several of one kind on blocks and settings (any kind, also with re-lock and from hotkeys; a flag given twice in one command is two locks, 2026-11-21); `extend` for timer locks; re-lock brings back the whole configuration on blocks and settings (SPEC §4.6 *(By Opus)*). The GUI flow on a real screen: Unchecked (`2.5`) |
| §3.6 Break per block window | Built | `schedule break B N 10m|off`: one break each time window N is open (`break start` takes it, then reward); new/longer weakens; GUI window editor; projects |
| §3.6 Allowance | Built | per day, week or month with a refill time (`allowance set B 5h --per week --refill 04:00`, GUI, projects). and rolling 24 hours (`--per rolling`, hourly buckets), and per window (`--per window`: each opening of a block window is a period, `enforce.QuotaPeriod`; outside windows nothing refills; an overnight span is two windows, so two periods) |
| §3.6 Reward | Built | `break reward B 10m --per 1h --keep one|window|day|week|month|<time ≥ 1h>` (default one; a time keeps each part that long after it is earned, no upper limit, 2026-11-16; the 1-day cap on what is saved is gone, so is the 4 h cap on one break taken from it), GUI choice with a duration field, projects; longer keep, or period ↔ time, weakens. Ending a break early used to give back at most one break's worth also for an accumulating reward: fixed |
| §3.6 Delay break | Built | `break delay B LENGTH WAIT`, `break request B`, GUI row, projects |
| §3.6 Session limit | Built | `allowance set B 1h --sessions N`: N openings per period; a limited session ends 3 min after its last use; more weakens. GUI, projects |
| §3.6 Autostart | Built | `kc9k autostart B add once DATE HH:MM / at HH:MM [DAYS] / sign-in / next-window`, GUI list, projects; removing weakens. `at` checked live; sign-in/wake: Unchecked |
| §3.7 Strict use counting | Built | `kc9k use-counting strict|normal`, GUI switch; every open tab (`rules.strict_use`) and running apps (`RunningApps`) count; normal weakens. Live: Unchecked |
| §3.8 Scheduled freeze lock kinds beyond lead | Built | `freeze schedule lock N --for/--lock/--password-stdin/--restart [--relock]`, `freeze schedule unlock N`; setting lock `freeze:<id>`; removal refused while on; counts as a lock. GUI (2026-11-17): Lock…/Unlock… for the schedule picked in the list (every kind, unlock order, confirmed; a password asked; a text lock in a terminal), the list says "locked (timer, password)" (gui-state `freeze_schedule_locks`); `ci/gui_freeze_lock_smoke.py` |
| §3.8 Allowed apps only | Built | freeze mode `apps` (`freeze --mode apps`, schedules too, GUI choice), list `kc9k freeze apps add|remove`; closes every regular-user process but allowed apps, their children and system/desktop helpers (`allowedAppKeep`; 2026-11-21: which processes on a made-up tree, `TestAppsToClose`, `TestCloseAllExcept`; the daemon by mode, `TestEnforceFreezeByMode`); a running lock/logout/shutdown freeze can't be extended into it (SPEC §4.8 *(By Opus)*, `TestFreezeExtendKeepsStrongerMode`, `TestScheduledAppsFreezeKeepsRunningLock`). GUI list editor on the Frozen Chicken page. Live: Unchecked (vm: an apps freeze with an allowed app and its children, everything else closed; B2) |
| §5 R-LOCK-12 | Partial | see 0.1 |
| §3.8 Strict mode exempt folders | Built | `kc9k strict exempt add|remove DIR`, GUI list; exec bind mounts over the noexec folders (`noexecPlan`); adding weakens. Not in projects (import never weakens). Live: Unchecked (strict mode not run overnight) |
| §3.8 Forgotten password recovery | Built | `kc9k-bin recover` (root, recovery boot, not with no-escape): settings password and password locks (`--access`: access passwords). Live: Unchecked (needs a recovery boot) |
| §3.8 Factory reset protection | Built | installer media guard (`reset-guard on|off`, GUI switch under device protection): while anything is locked an in-memory block (`enforce/resetguard.go`) blocks OS image and USB writer download pages and closes image writers and Disks; checked live (hosts follow the lock). Firmware lock (`firmware-lock on|off`, GUI Firmware lock…; 2026-11-21) tested on generated firmware-attributes trees only: on real firmware Unchecked (`2.6`) |
| §5 R-LIST-4/5 | Built | case, accents and look-alikes folded (`kc9kFold`; 2026-11-21: also capitals by their own look, small capitals, Armenian, fullwidth/mathematical/circled letters, ligatures, and zero-width characters and soft hyphens inside words; extension 1.28.0, `ci/match_test.js`). R-LIST-4 checked in Chromium (2026-11-17, `ci/content_harness_test.js`): hidden (`display:none`, closed `<details>`, `visibility:hidden`) text isn't read, text far down is. Frames were never judged by their text (the content script read the top frame only, whose text holds none of its frames'): now each frame judges its own (`scanFrame`); what would take a page away gives that frame the block page (background `frameHit` decides from its own rules; no events, R-EVENT-1), its looks show in the frame; extension 1.25.0. Checked in Chromium (2026-11-21, `ci/content_harness_test.js`): a frame's hidden text isn't read, a frame inside a frame is judged alone (the frame around it and the page stay), look-alikes in a frame are read; `frameHit` edge cases (looks alone, actions the list hasn't, test mode, the top frame, non-web addresses) in `ci/background_test.js`. Frames in LibreWolf: Unchecked (Needs: box) |
| §3.9 Ad trigger | Built | scope `ads` on blacklists and clusters (`list create --ads`, `list where B L ads on`, GUI "Ads" box): entries are ad network domains or industries (`db.AdIndustries`: adult, crypto, dating, gambling); rules `ads`; the extension watches each tab's third-party requests (`noteAdRequest`) and a page loading a listed network matches with every action (`kc9kAdMatches`, unit-tested). Not in shared layers. 2026-11-21: URL-term exceptions spare a page from it too (they didn't); a list whose block has exceptions by title or text is judged with the page (`kc9kAdsByPage`: the content script's resource timing, `page.adHosts`), so those exceptions hold (R-LIST-1). Edge cases unit-tested (`ci/match_test.js`, `ci/background_test.js`: hosts with capitals or a final dot, test mode, no actions, first-party, non-web pages, once per page) and in Chromium (`ci/content_harness_test.js`); extension 1.30.0. Browser test: Not run (the owner was using the laptop; the browser test changes LibreWolf settings and would close their windows) |
| §3.11 Block password | Built | `block password B set|remove`; hides entries, apps, titles in gui-state (`--reveal`), `block info`, exports, `list show/export`, `phrase-set show`; GUI "Show contents…"; backups keep it. A project export still holds the contents |
| §3.12 Word recording, specificity, cut | Built | `record start B L --words` / `--preset C --words` (text lists): extension reports each page's rendered words once (`kc9kPageWords`, `/words`); `kc9k record` ranks by specificity against the baseline (`db.Specificity`); `record stop --cut N`; `record baseline on|off`. Checked live in LibreWolf. GUI: Presets page (list presets) "Record words" with cut % and the most specific words |
| §4.10 SafeSearch | Built | a browser setting per engine (`safesearch-google`, `-bing`, `-duckduckgo`, `-youtube`), each its own lock; `browser safesearch` switches all four. Pinned in hosts (every browser) per engine, `safesearch_engines` to the extension (Brave, Yahoo, SearXNG, engines it can't hold blocked: with Google's, Bing's or DuckDuckGo's, SPEC §4.10 *(By Opus)*, §5 decided 2026-11-21). Checked live in Chrome 2026-11-21 (`2.4`); in LibreWolf: Unchecked |
| §4.10 Tamper settings | Built | GUI group with OK/Cancel window; clock, block unsupported browsers (own lock), allow file:// blocking, block task managers while locked (`tamper task-managers`), grace for a missing extension (`tamper grace S`, 10–600 s, longer weakens); gui-state `anything_locked`, settings stuck on show so. No group lock. close private windows and Tor (`tamper private`: private browsing off in every browser policy, Tor Browser/tor closed; checked live for the policies; no Tor here). Built 2026-11-21 (SPEC §4.10 *(By Opus)*): `tamper guest` (guest browsing off in every Chromium-family policy, LightDM guest session off), `extension-stop`, `inactive-tabs`, `embedded` (on until turned off: what KC9K always did), `installer` (`prepare-install` from `make install`/`install.sh` and the `.deb`'s prerm for a reinstall or older version; removal always refused). `TestTamperGuestExtensionInstallerTabsEmbedded`, `TestSyncGuestOff`, `ci/tamper_tabs_test.js`, GUI tamper window. Live (2026-11-21, `2.9`): guest windows in Brave/Edge/Vivaldi, a background tab and an embedded frame in Chrome: Built; with `embedded` off the browser policies hold only whole sites (SPEC §6 *(By Opus)*). Unchecked (box: the background tab and embedded frame in LibreWolf; vm: the LightDM guest session, `dpkg -i` of the same version and `install.sh` refused during a lock) |
| §4.10 Extension install dialog | Built | once per session when gui-state `extension_missing`: Install LibreWolf, or Show browsers |
| §4.10 Device protection: factory reset protection, guided setup | Built | 2026-11-21: firmware lock, guided setup (`kc9k guided-setup [--json]`, GUI Guided setup… with Do it…, `ci/gui_settings_smoke.py`), strict mode's PAM root guard (`login`, `su`, `su-l`, tested on generated files only). Live: Unchecked (`2.6`) |
| §4.10 Browsers list | Built | GUI Browsers group and `kc9k browsers`: LibreWolf, Chrome, Chromium, installed, per-browser extension check-in, Install (`browsers install`); live check in 0.2 |
| §4.11 `.deb` | Built | `make deb` (`packaging/build-deb.sh`): same paths as install.sh; postinst sets up sudoers, database folder, service; prerm runs prepare-uninstall, so removal is refused while locked (checked live); postrm removes sudoers. Installed on the laptop as 0.9.0~overnight (devmaster) |
| §4.11 One-click installer, every platform | Not built | `2.11` |
| §3.13 Chrome, Edge, Firefox, Bing on every platform | Partial | `2.12` |
| §4.14 Cross-device sync | Not built | `2.10` |
| §4.15 Accountability partner | Not built | `2.13` |
| §4.16 Website | Not built | `2.14` |
| §4.3 Draft Save/Cancel | Built | `block draft B [save|cancel]`; GUI: an opened block is always a draft, made at the first change of the Websites or Apps tab, with Save edits/Cancel (§5 DECIDED 2026-11-17): a copy of the lists, phrase sets, apps and window titles that never runs; save applies each difference through its gate, refusing and naming weakening ones on a locked block; needs and keeps the block password; drafts stay out of the block list and projects. The Breaks & allowance and Bypass prevention tabs are drafted too (2026-11-21, SPEC §6): their commands on a draft are kept (`draftQueued`, `keepOnDraft`; the last of each kind), shown in the tab ("Not saved yet", gui-state `draft_kept`) and run at save through their own gates (`runQueued`); taking a break or an allowance session, the automation and affected users still apply at once (`draftRefused`). `TestDraftKeepsBreaksAndBypass`, the weakening audit, `ci/gui_lists_smoke.py`. On a real screen: Unchecked (Needs: box) |
| §4.3 Ctrl+F, multi-select, double-click edit | Built | `EditableList`: Ctrl+F to its search, Ctrl/Shift-click and Ctrl+A in website lists, phrase sets, apps, titles, clusters, exempt folders; Delete/Backspace removes, selection moves on; double-click moves an entry into the box |
| §4.3 Import "From KC9K" | Built | `list copy`; a list's Import… offers From KC9K: tick other blocks' lists and clusters; Whole internet (`*.*`, asked first) and All searches containing… (the words as rendered terms, the list's URL scope on: a search's words are in its address, R-PHRASE-6; any other address with the word matches too, and the dialog says so) (2026-11-17, `ci/gui_import_smoke.py`) |
| §4.5 Grid drag-move, copy to days, similar windows | Built | drag a window to move it (time; day for one-day windows); click to edit days (copy to days), times, its break, or remove; a window on several days is edited as one; other blocks' windows greyed |
| §4.9 Statistics: times blocked, top N, search, filter by block, colour per site/app | Built | blocked pages counted per site and block (`blocked`, `blocked-block` rows); `stats --top N --search T --block B` (JSON filtered too); GUI Times blocked view, Top 5/10, search, block filter, stacked daily chart with a colour per site/app and a legend (`ci/gui_stats_smoke.py`). Time frame: presets (`--days N`) and custom (`--from`/`--to`, GUI Custom…) |
| §4.9 Statistics per user, skip private windows | Built | skip private windows (`stats private skip|count`, rules `stats_skip_private`, GUI switch); per user (`usage_users`: apps by desktop session user, sites by the uid of the extension's socket, `blocker.ClientUser`; `stats --user U`, `--users`, GUI chooser; checked live for apps). Times blocked are counted for all users only; backups carry totals only |
| §4.10 Preferences | Built | theme with darkness slider (dark grey to black), clock 24-/12-hour (shown times: grid and clip axes, tooltips, pauses; entry fields stay HH:MM), first day of the week (schedule grid), open at login, tray (Ayatana AppIndicator or XApp status icon: what is on, open, quit; closing the window keeps it) built; tray untried on a real panel |
| §4.10 Notifications choice | Built | `kc9k notifications`, GUI group; window start/end, timer lock end, Pomodoro, allowance, app closed; advance minutes. Live: Unchecked |

## 2. Next features (after §0)

- **2.1 Circadian rhythm** (§4.10): built. Checked 2026-11-20, desktop
  (Mint 22.3 Cinnamon, X11, one screen HDMI-1 1920x1080, no backlight;
  devmaster install of this fix), software and colour clips on (owner's
  clips, 6500 K / 100 % then), `xrandr --verbose` per step: clips on →
  `gamma ours`; `preview kelvin 3000` → Gamma 0.48:1.0:1.7, ran out → 1.0;
  then `preview software 50` (0.50) and `--end` → 1.0 again, unpaused
  (before: the screen stayed at 50 %, paused; the engine took over the
  ramp mixed with the ended colour preview's 3000 K, so it took the real
  one for someone else's and, switched off, never gave plain ramps back;
  now each gamma preview notes the ramp it wrote, the engine takes over
  the last one, `TestCircadianPreviewAfterAnEndedPreview`). Run out
  without `--end`: same. `circadian off` during a preview at 60 % → plain
  ramps once it ends. Daemon stopped during a preview at 70 %: the ramp
  stays (nothing restores on stop), started within 30 s → 1.0. Cinnamon
  night light on (1700 K) → `gamma other`, both clips paused until 18:00,
  `night_light true`; off → plain ramps, pause lifted, ours again. `play
  --seconds 6`: colours over the day, back to 1.0, no pause. Owner's
  settings put back (off, both lanes off). Unchecked: laptop backlight,
  external screens' brightness by ddcutil (backlight clip off here), several
  screens, GNOME/MATE, dark mode and wallpapers on this desktop (owner
  leaves them to the system). Wayland: no gamma ramps.
  System-following dark pages rely on `prefers-color-scheme`, which
  LibreWolf reports as light. In project files with its wallpapers
  (`cli/projectcircadian.go`; import only where none is set).
- **2.2 Ad trigger** (M): built (scope `ads`). Industry lists grown
  2026-11-20 (adult: TrafficHaus, TrafficShop, ClickAdilla; crypto: Mellow
  Ads, Blockchain-Ads, HypeLab; `TestAdIndustries`). Checked against real
  ad requests 2026-11-21 (cloud container, Chrome 158.0.8127.36; SPEC §6
  *(By Opus)*): every domain against EasyList (with its ad-server and
  adult lists), AdGuard DNS, Peter Lowe's and StevenBlack's lists, and a
  crawl of 60 real pages (crypto, gambling and adult publishers, the last
  also where dating ads run; third-party request hosts only, matched as
  `kc9kAdHost` does). Seen in
  real pages' requests: ExoClick (`ads.exoclick.com`, `a.magsrv.com`,
  `a.pemsrv.com`, `a.orbsrv.com`), TrafficStars (`cdn.tsyndicate.com`),
  TwinRed (`twinrdengine.com`), `sexad.net`, `adspector.io`, Coinzilla
  (`cdn.coinzilla.com`, `coinzillatag.com`, `czilladx.com`). Added the
  serving domains seen and listed (`pemsrv.com`, `orbsrv.com`,
  `twinrdengine.com`, `sexad.net`, `adspector.io`, `coinzillatag.com`,
  `czilladx.com`; `TestAdIndustriesSeenLive`). Removed what no list names
  and no page loaded: affiliate platforms whose tracking runs on the
  operator's own domains (`income-access.com`, `cellxpert.com`,
  `referon.com`, `raventrack.com`, `crakrevenue.com`), a game maker
  (`playtech.com`), `affiliatehub.com` (doesn't resolve),
  `quintessence.media`, `gamblingads.com`, `betpartners.com`,
  `adex.network`, `datingpartners.com`, `datingfactory.net`. Gambling and
  dating keep only list-named domains (`netrefer.com`, `myaffiliates.com`,
  `gambling-affiliation.com`; `adultfriendfinder.com`, `sexsearchcom.com`):
  no page in the crawl loaded them. Gambling sites' own ads came through
  general exchanges (Google, Xandr, Flashtalking, Bannerflow), which no
  industry can name. Many adult and some crypto sites showed an age or bot
  check to the container and loaded nothing. Browser test built
  2026-11-20, desktop (Mint 22.3, LibreWolf headless, devmaster install of
  d3a7b1b): `ci/extras_test.py`, block `extraads`, a list `--ads` with
  `kcnine-ads.test` and one with industry `crypto`; pages load a picture
  from `cdn.kcnine-ads.test` / `cdn.coinzilla.com` (resolved to the test
  server): both blocked, the block page naming the list; a page loading
  from another site opens; with the block off it opens. Suite run alone:
  33/33 pass (4 new).
  Left: gambling and dating domains seen live.
- **2.4 SafeSearch** (S): a lock per engine built 2026-11-20 (SPEC §6
  2026-11-18): four browser settings, the old one and its lock moved to
  each (`TestMigrateSafeSearch`, `TestSafeSearchPerEngine`,
  `ci/match_test.js`, `ci/gui_settings_smoke.py`). Built in Chrome
  (2026-11-21, cloud container, Chrome 158.0.8127.36, the daemon running
  as root, real `/etc/hosts` and policies): each engine on alone, then all
  four (`browser safesearch on`), then off. Each alone made only its own
  searches strict: Google got `safe=active` and connected to
  forcesafesearch's 216.239.38.120 (the results page itself is Google's
  CAPTCHA for the container's address); Bing got `adlt=strict` and
  strict.bing.com's address; DuckDuckGo went to `safe.duckduckgo.com` with
  `kp=1` and the page's own `settings_url_params={"kp":1}`; YouTube said
  "Restricted Mode is enabled by your network administrator". The other
  three stayed as with all off. The hosts section held only the engines on
  (Google 4 names, Bing 2, DuckDuckGo 2, YouTube 6) and was gone with all
  off; it is written by the daemon's next tick (about 30 s), not by the
  command. Unchecked (box): the same in LibreWolf.
- **2.5 Layered sequential locks** (M): blocks and setting locks done (stored
  order, `unlock` lifts only the first, status and GUI show the order).
  Several locks of one kind on a block built 2026-11-20 (owner: a second
  one is added, not the first strengthened): `LockExtras.Queue`,
  `TestSeveralLocksOfOneKind`, `TestQueuePromotion`. The build's four
  exceptions removed 2026-11-20 (owner, §5): a new timer/delay lock is a
  new layer, range locks stack, text/password locks stack with re-lock,
  hotkey presses stack. Extending a timer lock is its own action:
  `kc9k extend B --for|--until`, GUI Extend timer… (`TestExtendTimerLock`,
  `ci/gui_extend_smoke.py`). A block's re-lock brings back its whole lock
  configuration in its order (`LockExtras.Config`,
  `TestRelockWholeConfiguration`), with any lock. Built 2026-11-21 (SPEC
  §4.6 *(By Opus)*): a lock flag given twice is two locks (`lockSpecs`,
  `splitSpecs`; `TestSeveralOfOneKindInOneStart`, hotkeys keep them,
  `TestHotkeyKeepsRepeatedLockFlags`); setting locks stack
  (`SettingLock.Queue`, `TestSettingLocksStack`) and re-lock their whole
  configuration (`SettingLock.Config`, `TestSettingRelockWholeConfiguration`).
  Left (owner/box): a person checking the GUI's Add locks…, Extend timer…
  and unlock flow on the laptop.
- **2.6 Factory reset protection** (M): code built 2026-11-21. Firmware
  lock via `/sys/class/firmware-attributes/` (`firmware-lock on|off`:
  USB/network boot and the one-time boot menu off, boot order lock on,
  then the password a trusted person types; SPEC §4.10 *(By Opus)*):
  `TestFirmwareLockOnFakeFirmware`, `TestFirmwarePurposeNames`,
  `TestFirmwareLockCommand`. Installer media guard: built earlier. PAM
  root guard in `login`, `su`, `su-l` via `ApplyLockdown`/`RemoveLockdown`:
  `TestApplyRemovePAMGuardOnGeneratedFiles`, `TestGuardPAMConfigPlacement`
  (generated Mint/Ubuntu files, never the real `/etc/pam.d`). Guided setup:
  `TestGuidedSetup`, GUI in `ci/gui_settings_smoke.py`. Unchecked (vm):
  during a strict lock in the VM, `su -` and a root login on a text console
  are refused, the user's own `su` to another account works, and all of it
  comes back when the lock ends or with `kc9k.nofreeze`: 2026-11-21
  `ci/strict_pam_vm_test.sh host kc9k-vm` checks all of it (snapshot
  `clean` before and after; root login through the `login` PAM service on
  a pseudo-terminal; refused means before a password is asked), script
  written, never run (its terminal driver was tried in a container with
  the guard lines added by hand: refused and allowed as expected). Unchecked (owner,
  real hardware): the firmware lock on a Dell, Lenovo or HP computer (the
  VM has no firmware-attributes; vendors' names and how each driver clears
  a password are untried).
- **2.7 Hardening** (M each): sudo rule arguments audited 2026-11-15: caller
  paths are read as the caller (`readAsCaller`), direct reads only of
  KC9K's own files, writes only into KC9K's folders under hash names;
  `audit_test.go` keeps it so (`env_reset` untouched). Rules server
  secret: dropped 2026-11-21 (By Opus, §5, SPEC §6); instead every
  address the server answers is held to its guards (own host name, the
  extension's header on posts, no CORS answer, nothing read before a
  refusal) by `TestRulesServerGuardsEveryRoute` (`rulesRoutes`). GTK window for the text
  lock's challenge: built 2026-11-20 (SPEC §4.6, §5 decision): without a
  terminal the command opens its own window as the signed-in user and
  reads the keys itself; stdin is never read for it; the GUI's Unlock runs
  the command, no terminal. Checked on Xvfb with xdotool
  (`TestChallengeWindowOnX`: human pace passes; a burst, Ctrl+V, Escape,
  keys sent to an unfocused window don't). On the owner's real desktop
  (Mint, Cinnamon: the window opens from the GUI under sudo, keeps focus,
  a real keyboard): Unchecked.
- **2.8** Everything else Not built/Partial in §1.
- **2.10 Cross-device sync** (L, owner 2026-11-22; SPEC §4.14): Not built.
  KC9K account and cloud (principle 6's one exception); Upload all and
  Download all on Linux, Windows and Android; one to one except
  device-specific items (SPEC §4.14 lists them); Download all replaces
  even while locked (an exception to R-WEAK-1, so the weakening audit
  must name it as allowed). The project export (`cli/project*.go`) is the nearest thing built, but it
  only adds; Download all replaces.
- **2.11 One-click installer** (M per platform, owner 2026-11-22; SPEC
  §4.11): Not built. Linux has a `.deb` and `install.sh`; Windows an MSI
  (W7); Android none. Each becomes one download and one click.
- **2.12 Chrome, Edge, Firefox, Bing everywhere** (L, owner 2026-11-22;
  SPEC §3.13): Partial. Built: Chrome and Chromium on Linux; Chrome and
  Edge on Windows waiting for the store listing (W2). Not built: Firefox
  (LibreWolf only), Edge on Linux, every browser on Android (IronFox only),
  Bing (the Microsoft Bing browser app).
- **2.13 Accountability partner** (L, owner 2026-11-22; SPEC §4.15): Not
  built. Needs the KC9K account (`2.10`): invites, reports, alerts, the
  partner holding password locks and the settings password, unlock
  approval; no screenshots.
- **2.14 Website** (M, owner 2026-11-22; SPEC §4.16): Not built. Content
  per SPEC §4.16; where it is hosted is the owner's choice when built.
- **2.9 Tamper settings live** (S): built 2026-11-21 (§1 *§4.10 Tamper
  settings*). Built in Chrome-family browsers (2026-11-21, cloud container,
  Chrome 158.0.8127.36, daemon as root, real policies): `guest` on, each
  started with `--guest` opened its Default profile, off its Guest profile
  (Brave 1.100.58, Edge 158.0.4370.33, Vivaldi 8.3.4191.47).
  `inactive-tabs`: on (default), a background tab on example.com got the
  block page within 15 s of `kc9k start`; off, it kept example.com for 40 s
  and got the block page the moment it was switched to (from the extension
  and from outside, DevTools protocol). `embedded`: a local page embedding
  a Vimeo video whose path (`player.vimeo.com/video/…`) was blacklisted;
  on, the frame was blocked; off, it still showed Chrome's "Your
  organization doesn't allow you to view this site": the policy's
  URLBlocklist blocks frames too. Fixed (SPEC §6 2026-11-21 *(By Opus)*):
  with `embedded` off the browser policies hold only whole sites
  (`TestEmbeddedOffKeepsPatternsOutOfPolicy`); then the video played
  (time 9.99 → 12.99 s), the same address opened as a page got the block
  page, and turning `embedded` on blocked the frame again. Unchecked (box):
  `inactive-tabs` and `embedded` in LibreWolf (its WebsiteFilter gets the
  same change). Unchecked (vm): the LightDM guest session gone after a restart
  with `guest` on; during a lock with `installer` on, `sudo dpkg -i` of the
  same `.deb` and `sudo ./install.sh` are refused and a newer `.deb`
  installs. 2026-11-21: `ci/tamper_vm_test.sh host kc9k-vm` checks these
  VM parts (also an older `.deb` refused, the same one installing again
  after the lock), script written, never run. Found writing it, fixed
  2026-11-21: when the installed package's prerm refused, dpkg asked the
  incoming package's prerm (`failed-upgrade`), which let the same or an
  older version install anyway; it now refuses too
  (`packaging/build-deb.sh`; checked in a container with the real `.deb`s
  and a refusing stub binary: same and older refused, nothing unpacked;
  newer installs; same again once unrefused). A `.deb` built before the fix
  still gets in that way.

## 3. Testers and Android (owner 2026-11-18)

When §0–§2 are done: the owner's brothers test KC9K on their computers
while the Android app is built (its §0 items, `android/CLAUDE.md`).

- [ ] `C1` Build for the brothers' computers with `TAGS=devmaster`
  (owner 2026-11-18; the master password stays with the owner).
- [ ] `C2` Remove `ci/vm.sh`; on the laptop remove `/etc/sudoers.d/99-claude-test`.

## 4. Windows (owner 2026-11-21)

Built now, for an administrator account (SPEC §6 2026-11-21). Core's
state: the W items below. What carries over: `cli`, `db`, `enforce`, `schedule`, `quota`,
`freeze`, `circadian`'s logic (pure Go, `modernc.org/sqlite`, no CGo), the
extension and the rules server. What doesn't: `internal/blocker/` (14.5k
lines: `/proc`, nftables, X11, PAM, systemd), `internal/daemon/`,
`internal/lock/`'s Python window, `gui/kc9k-gui.py` (GTK), `install.sh`,
`packaging/`. Rules unchanged: `enforce` decides, the CLI decides
weakening, the GUI decides nothing. Each item gets its note (or a Windows
part of the existing one) in the commit that builds it.

Shape: `kc9k.exe` (same Go program) runs as a **Windows service**
(LocalSystem) and as the CLI; the CLI hands its arguments to the service
over a **named pipe** and prints what comes back, as the sudo wrapper does
on Linux, so no UAC prompt per command. Services can't reach the desktop
(session 0), so the service starts a **session agent** (`kc9k-agent.exe`,
`CreateProcessAsUser`) in each logged-in session and restarts it if it
ends: window titles, polite close, hotkeys, notifications, idle, gamma,
wallpapers and the text lock window run there and report to the service.
Everything works on Windows Home: nothing uses Pro-only features (Group
Policy Editor, AppLocker); policies are written to the registry directly.

Two sessions build this at once (owner 2026-11-21), each in its own
files. **Core** (`W0`, `W1`, `W2`, `W3`, `W5`, `W6`, in that order): all Go
(`cmd/`, `internal/`, `go.mod`), `.github/workflows/windows.yml`, the
`notes/` for those areas. **GUI** (`W4`, then `W7`): `windows/` (C#
GUI, installer), `.github/workflows/windows-gui.yml`,
`notes/gui-windows.md`, `notes/install-windows.md`. The GUI only runs
`kc9k.exe` and reads its JSON (`gui-state`, `stats --json`); it never
opens the pipe, the DB or the registry. GUI-only additions to the CLI's
JSON or commands (`internal/cli/`, additive, with tests) are the GUI
session's; everything else in Go is Core's. Shared files (`ROADMAP.md`,
`SPEC.md` §6, `CLAUDE.md`): each edits only its own items and lines.
Each works on its own branch (`windows-core`, `windows-gui`) and lands
each finished step on `main`: rebase onto `origin/main`, run the gate
(Linux tests unchanged, Windows build), push; in a conflict keep both
sides. Neither touches the Linux GUI, the extension or Android, except
where its item needs it and the Linux tests prove nothing changed.

- [x] `W0` Windows build (M). Built, checked (2026-11-21): Linux-only
  files carry `//go:build linux`, their Windows counterparts are
  `_windows.go`, shared types in `blocker/shared.go` (notes/windows.md);
  every layer has its Windows version. `GOOS=windows go build ./...`,
  `go vet` and the whole unit suite pass on `windows-latest`
  (`.github/workflows/windows.yml`, `unit` job; the Linux workflow is
  unchanged).
- [ ] `W1` Service and agent (M). Built, Partial checked (2026-11-21):
  `kc9k install`/`uninstall`, the service (automatic, restarted after any
  failure, restarts itself on an upgrade), the command pipe (caller from
  the client's token, Administrators only, files read as the caller,
  consoles relayed with raw input for the challenge), the session agents
  (SPEC §6 2026-11-21 By Opus). Checked on `windows-latest`
  (`ci/windows_service_test.ps1`, 2026-11-21): install, the service and
  its restart settings, PATH, forwarded commands and exit codes, a file
  read as the caller (the database refused to an administrator), the data
  folder closed to administrators, uninstall refused while locked, then
  clean, data kept. The agent and the
  console's raw mode need a signed-in desktop: Needs win-vm
  (`ci/windows_vm_test.ps1`, written 2026-11-21, never run).
- [ ] `W2` Web layers (L). Built (2026-11-21): hosts (+ DNS cache
  flush), WFP firewall (persistent provider, sublayer, filters; monk mode's
  cut), Chromium policies in the registry (Chrome, Chromium, Edge, Brave,
  Vivaldi), Firefox-family `policies.json` in every installation, LibreWolf
  setting locks, extension guard and browser recognition (version
  resource, engine DLLs), supported browsers. Checked on `windows-latest`:
  WFP blocks for real, registry policies, process snapshot, TCP owner.
  Unchecked: real browsers obeying the policies, the guard closing a
  browser, monk mode's cut (Needs win-vm). Needs owner: the Chrome Web
  Store listing's ID (`blocker.ChromeStoreExtensionID`); until then Chrome
  and Edge are not supported browsers and get no extension.
- [ ] `W3` Apps and desktop (L). Built, Unchecked (2026-11-21): apps by
  name, path, folder or contents (kill tree, polite close through the
  agent), allowed apps, Tor, tools naming blocked sites (command lines),
  titles (agent `EnumWindows`/`WM_CLOSE`), notifications (balloon/toast),
  questions, locking, idle and the app in front, logout, power off, the
  logout freeze's hold, monk mode's adapters, hotkeys (agent
  `RegisterHotKey`), custom actions, the challenge window (Win32, Raw
  Input for injected keys). Checked on `windows-latest`: process kill
  tree. The rest needs a desktop: Needs win-vm, laptop: run
  `ci/windows_vm_test.ps1` (written 2026-11-21, never run).
- [ ] `W4` GUI (L). Native, C# on WinUI 3, .NET LTS, unpackaged and
  self-contained, in `windows/gui/` (SPEC §6: no browser, no web view).
  Reads `kc9k gui-state` and `kc9k stats --json`, runs `kc9k` commands
  (`kc9k.exe`, which forwards to the service); the same pages as the GTK
  GUI (§4.1–4.13). Smoke tests through UI Automation on
  `windows-latest`, as `ci/gui_*.py` do on Linux.
  Built in CI (2026-11-21, notes/gui-windows.md), Unchecked on a real
  machine:
  - Built, checked on `windows-latest` (`windows-gui.yml`: 33 Core tests,
    publish, UI Automation smoke tests against the fake kc9k replaying
    the real CLI's JSON, screenshots looked at in light and dark): every
    page of the GTK GUI: Overview, Blocks (cards, modes with drag and drop
    and a keyboard way, the block editor's four tabs with drafts and Save
    edits, list and phrase set cards, imports and exports, locks with the
    one confirmation R-LOCK-12 and the unlock order R-LOCK-6, Extend
    timer, users, presets), automation (week calendar, windows, schedules
    applied, places, schedule lock, autostarts), monk mode, Frozen Chicken
    (freeze now and until a date, scheduled freezes and their locks,
    allowed apps), Schedules, Places, Hotkeys, Presets and recording,
    Statistics (frames, users, top, search, block filter, JSON export),
    Circadian rhythm (clips, presets, Play the day, wallpapers, dark mode,
    dark pages), mode wallpapers, Settings (all of §4.10's groups), the
    access-password covers, the settings password, the tray icon, Open at
    sign-in.
  - Checked against the real `kc9k.exe` and its service on `windows-latest`
    (`RealTests`: Core's `kc9k install`, then a block made, an entry added
    and saved, the block switched on and off, every page opened; it found
    a new block opening before its state was read, fixed: a command now
    returns once the state after it is read).
  - Unchecked (needs win-vm or laptop): the text lock's challenge window
    opened by the CLI (W3); drag
    and drop (block onto mode, calendar, clips, wallpaper bars), file
    dialogs, the tray menu and Open at sign-in by a person; Narrator, high
    contrast, 150–200 % scaling and a second screen.
  - Since 2026-11-22: W30–W49 (§4.1): parity, conveniences, accessibility
    and tests built on top, checked on `windows-latest`; what needs a
    person is listed at §4.1's end.
  - Windows wording the CLI's output will have (W1–W6) may differ from the
    help texts here; those are the GUI's own and change with Core.

- [ ] `W5` Tamper protection against an administrator (L). Built
  (2026-11-21), Partial checked: the service's permissions (no stop,
  change or delete by Administrators), restart on failure, the SYSTEM
  watchdog task, Safe Mode registration, all put back and counted each
  cycle; strict mode's deny entries (service key, SafeBoot, BCD, recovery
  configuration, policy keys, hosts, program folder; for Interactive,
  Network and Batch logons, since LocalSystem holds Administrators), kept
  and taken off again; downloaded and portable programs closed during
  strict mode; no-escape's boot settings (recovery off, automatic repair
  off, boot menu hidden); factory reset protection turns the recovery
  environment off while locked; Windows task managers and service tools
  in `task-managers` (notes/windows.md). Checked on `windows-latest`
  (`ci/windows_service_test.ps1`, 2026-11-21, all 30 checks): an
  administrator can't stop, reconfigure or delete the service; a deleted
  watchdog task is back within a cycle (Task Scheduler lets an elevated
  administrator delete it); Safe Mode registration; strict mode denies the
  hosts file and the service's key while the service still writes hosts,
  and both open again after the lock; uninstall refused while locked, then
  clean. Unchecked: Safe Mode boot, no-escape and the recovery environment
  on a real boot, an administrator taking ownership (Needs win-vm:
  `ci/windows_vm_test.ps1 -Reboots`). The firmware lock: `W19`.
- [ ] `W6` Circadian rhythm and wallpapers (M). Built, Unchecked
  (2026-11-21): gamma ramps per screen through the agent (warm ramps
  allowed by `GdiIcmGammaRange`), wallpapers (copied where desktops may
  read them), dark mode, Night light read (Windows offers no way to switch
  it: the person is asked), laptop panel brightness (WMI), external
  screens (DDC/CI, dxva2), the sleep hook by power events. Needs win-vm
  (gamma, wallpaper, dark mode) and laptop (panel, external screens,
  Night light).
- [ ] `W7` Installer and release (M). MSI (WiX), with the service, the
  agent and the GUI; `installer` tamper setting (`prepare-install`)
  refuses removal and downgrade while locked, as the `.deb` does.
  Code-signed (SPEC §6 2026-11-21 later): the owner picks the certificate
  (Azure Artifact Signing is the cheapest serious one). Since 2026-11-22
  also W40 (release), W41 (upgrade, downgrade, repair, full UI: checked),
  W42 (signing proven with a throwaway certificate); left: the owner's
  certificate and first tag, Apps & features by a person.
  Built (2026-11-21, notes/install-windows.md, SPEC §6 2026-11-21 By
  Opus): `windows/installer/` (WiX 6): one x64 MSI with the GUI and
  `kc9k.exe`; `kc9k install`, `uninstall` and `prepare-install` run as
  SYSTEM custom actions that show kc9k's own words when it refuses.
  Checked on `windows-latest` (`windows-gui.yml` job `installer`,
  `test-msi.ps1`): install (service running, files, Start menu entry,
  `gui-state` through the service), removal (service and program gone),
  removal refused while a block is locked (everything stays), a reinstall
  refused with "block the installer" on while locked. Not done: signing
  (Needs owner: a certificate; `sign.ps1` and the workflow's steps sign
  once `SIGNING_PFX_BASE64`/`SIGNING_PFX_PASSWORD` are set, or swap in
  Azure Artifact Signing); a release workflow publishing the MSI.
  Unchecked (win-vm): installing and removing from Apps & features with
  the full UI, an upgrade to a newer version over a locked install, an
  older version refused.

### 4.1 Windows GUI: parity and release (W30–W49, GUI session 2026-11-22)

Items and choices in this section are the GUI session's (By Opus), from
an audit on 2026-11-22: the GTK GUI's pages, `gui-state` fields and
`kc9k` commands against `windows/gui/`; the `ci/gui_*.py` tests against
`Kc9k.UiTests`; Windows conveniences; the MSI and its release. Found:
every `kc9k` command and every `gui-state` field the GTK GUI uses is used
by the Windows GUI too (W4 ported them one to one), so the gaps are
Windows-only behaviour, tests, conveniences and release. Order: fixes to
what is built, then parity, then the rest. Core's `GUI:` lines (W10–W29):
none on `main` yet; rechecked at each rebase (`W49`).

Fixes to what is built:

- [x] `W30` Supported browsers on Windows (S; §3.13, §4.10 Browsers, §6
  2026-11-21 Windows equivalents). Chrome and Edge, installed but without
  the store listing's ID, show "not installed" with an Install button whose
  command can only fail. Build: `gui-state` browsers get `found` (an
  executable is there; additive, `internal/cli`, tested); the GUI says
  "installed, but Killer Chicken 9000 can't give it the extension yet:
  treated as an unsupported browser", and Install on Windows shows where
  to get the browser (the CLI's words) as information, not as an error.
  Built 2026-11-22 (`TestBrowsersFound`, `BrowserFoundButNotSupported`,
  UI test `BrowsersWaitingForTheStoreAreExplained`).
- [x] `W31` Overview as §4.1 asks (S; §4.1). Break status is only a pill;
  ladder positions are missing (the GTK GUI lacks them too). Build: per
  block on now, "on a break" or a break's time left and its bypass
  prevention ladder position; the global ladder's position under Device.
  Built 2026-11-22: `gui-state` gives `nepsis_level` (global and per
  block; `TestGuiStateNepsisLevel`); `Words.LadderPosition`, UI test
  `OverviewShowsBreaksAndLadderPositions`.

Parity with the GTK GUI and its tests:

- [ ] `W32` UI Automation counterparts of the GTK smoke tests (M;
  R-GUI-1–3, §4.2–4.13). Missing: `gui_actions` (run a mode locked,
  custom action), `gui_automation` (enabled/disabled by default,
  either/both), `gui_cards` (stop, a refusal put back), `gui_duration`
  (R-GUI-1 fields say why and keep the text), `gui_extend`,
  `gui_freeze_lock`, `gui_import`, `gui_list_resize`, `gui_lists` (new
  list's scope boxes), `gui_modes` (into and out of a mode), `gui_phrase`
  and `gui_phrase_edit`, `gui_places`, `gui_playlist` (social media
  settings), `gui_presets` (save lists as a preset), `gui_trigger`,
  `gui_wallpaper`, `gui_click_everything` (every button and switch once,
  Cancel then OK, no crash). Partial (2026-11-22, `ParityTests.cs`): built
  for `gui_cards`, `gui_extend`, `gui_duration`, `gui_lists`,
  `gui_presets`, `gui_modes` (by the card's menu), `gui_freeze_lock`,
  `gui_automation`, `gui_click_everything` (every button once, questions
  cancelled). Not yet: `gui_actions`, `gui_import` (file dialogs),
  `gui_list_resize`, `gui_phrase`, `gui_phrase_edit`, `gui_places`,
  `gui_playlist`, `gui_trigger`, `gui_wallpaper`. Needs: win-ci.
- [x] `W33` Accessibility checks (M; R-GUI-3). Build: a test walking every
  page's UI Automation tree (each button, switch, box and list has a name
  and takes keyboard focus), screenshots in high contrast and with text
  at 150 % and 200 %. Narrator read by a person, display scaling and a
  second screen: Needs win-vm, laptop. Built, checked 2026-11-22
  (`AccessibilityTests.cs`: every usable control on every page and block
  tab named and focusable; high-contrast and 200 % text screenshots).
- [x] `W34` Keyboard shortcuts (S; R-GUI-3 "everything reachable by
  keyboard"). Ctrl+1…9 open the sidebar's pages, Ctrl+N a new block, F5
  reads the state again, Alt+Left (and Esc) goes back from a block or a
  mode; listed under Settings › Keyboard. The GTK GUI has none of these.
  Built 2026-11-22 (also the mouse's back button; UI test
  `KeyboardShortcuts`). Esc is left to dialogs and text boxes.

Windows conveniences:

- [x] `W35` Tray quick actions (S; §4.10 tray). Start a block ▸, Start a
  mode ▸, Monk mode…, Frozen Chicken…, New block…, Open, Quit. Only
  actions that strengthen: Stop, Unlock and pauses are never in the tray,
  so it is never an easier way past a block than the window (By Opus).
  Built 2026-11-22: `Core/TrayMenu.cs` (`TrayMenuOnlyStrengthens`),
  rendered with submenus by `Tray.cs`; each action brings the window
  forward first, so a question or refusal is seen. The menu opened by a
  person: Unchecked (Needs laptop).
- [x] `W36` Jump list and start arguments (M). The taskbar's and Start
  menu's jump list: Overview, Blocks, New block, Monk mode, Frozen
  Chicken, Statistics, Settings, as `kc9k-gui.exe --page KEY` or
  `--do new-block|monk|freeze`; a second start hands its arguments to the
  running GUI. Built 2026-11-22 (`StartArgsOnlyPagesAndDialogs`, UI test
  `StartArgumentsAndASecondStart`); the jump list itself shown by the
  taskbar: Unchecked (Needs win-vm).
- [x] `W37` Window state (S; §6 2026-11-21 GUI preferences). Position,
  size, maximized and the last page kept per user and restored on a
  screen that is still there; preference "Open on: Overview / the last
  page". Built 2026-11-22 (UI tests `WindowStateIsKept`,
  `AWindowOffEveryScreenComesBack`). Several screens and 150–200 %
  scaling: Unchecked (Needs win-vm, laptop).
- [x] `W38` First view (S; R-GUI-3 tasks first; §6 2026-11-14 no wizard,
  §6 2026-11-21 guided setup). While nothing is set up (no block, no
  supported browser with the extension), the Overview shows a short
  "Get started" list, each step opening its place, and the stricter
  options (strict mode, no-escape, guided setup) named last. It switches
  nothing on (principles 3, 7). Built 2026-11-22 (`Words.FirstSteps`,
  UI tests `AFirstViewForNothingSetUp`, `NoFirstViewOnceSetUp`).
- [x] `W39` Device protection explained for Windows (S; §4.10 Device
  protection, principle 3, §6 2026-11-21 Windows equivalents). Settings
  says per level what holds on Windows: basic; strict mode (the
  administrator account denied the service, files, registry keys, boot
  and recovery settings; downloaded and portable programs closed); boot
  and hardware (Safe Mode runs the service; no-escape and factory reset
  protection turn the recovery environment off; the firmware lock and its
  status shown on the page, not only in its dialog). Circadian rhythm's
  Night light note exists (W6). Built 2026-11-22 (`Words.ProtectionLevels`,
  UI test `DeviceProtectionExplainedForWindows`); each level's claims are
  Core's (W5) and Unchecked on a real boot until W5 is (Needs win-vm).

Installer and release (W7):

- [x] `W40` Release workflow (S; §4.11). A tag `v1.2.3` builds `kc9k.exe`
  (with its version), the GUI and the MSI with that version, signs them
  when a certificate is configured, and publishes a GitHub release with
  the MSI and its SHA-256. Built 2026-11-22 (`windows-release.yml`;
  by hand it builds and checks without publishing). Needs owner: the
  first tag. Checked 2026-11-22: a by-hand run (`0.0.1-dryrun.1`) on
  `windows-latest` built, installed, read `kc9k version` back and removed
  the MSI.
- [x] `W41` MSI checks in CI (M; §4.11, R-LOCK-9, §6 2026-11-21 MSI).
  Add to `test-msi.ps1`: a newer version over a locked install (installs,
  the lock stays), an older one refused while locked with block the
  installer on, a repair refused likewise, and removal through the MSI's
  own full UI driven by UI Automation. Built 2026-11-22 (`test-msi.ps1
  -Older -Newer`, `MsiUi`; install and removal through the full UI
  too). Checked on `windows-latest` 2026-11-22 (every check of
  `test-msi.ps1` passes). Apps & features by a person: Needs win-vm.
- [x] `W42` Signing that switches on (S; §6 2026-11-21 later). A CI job
  signs with a throwaway self-signed certificate and checks each file
  carries a signature, so the owner's certificate only has to be added;
  Azure Artifact Signing chosen by its own secrets. Built 2026-11-22: the
  throwaway-certificate check in the `installer` job. Not built: Azure
  Artifact Signing steps (their action's inputs depend on the owner's
  account; swap them in for `sign.ps1`). Needs owner: the certificate.
- [x] `W43` Versions shown (S; §4.11). `kc9k version` (additive,
  `internal/cli`; set at build), the GUI's version and kc9k's under
  Settings › About, so a person can tell what is installed. Built
  2026-11-22 (`TestVersion`, UI test `AboutShowsTheVersions`).

Core's features:

- [ ] `W49` Core's `GUI:` lines (W10–W29) given their place in the GUI,
  each with a UI Automation test, once the command is on `main`. Read
  2026-11-22: W15 (guided setup's step IDs and texts differ on Windows:
  the checklist's buttons are keyed by step ID), W16 (help texts), W18
  (one-click browser install: W30's Install already shows its result);
  none built on `main` yet. Rechecked 2026-11-22 after Core's W11–W27
  landed: W15's Windows steps (`other-admins`, `builtin-admin`: Do it…
  opens Windows' account settings; `safe-mode`: information) shown, UI
  test `WindowsGuidedSetupSteps`; W16's help texts come from kc9k.exe as
  they are; W18's install result is shown by W30's Install. Nothing else
  in Core's items asks the GUI for a place.

What only the owner, a VM or the laptop can show (GUI session, 2026-11-22;
everything else in W4, W7 and W30–W49 is checked on `windows-latest`):

- **Owner**: a code-signing certificate (secrets `SIGNING_PFX_BASE64`,
  `SIGNING_PFX_PASSWORD`, or Azure Artifact Signing's steps in place of
  `sign.ps1`); the first release tag (`v0.1.0`); the Chrome Web Store
  listing's ID (W2), after which Chrome and Edge count as supported (W30's
  wording goes by itself).
- **A person on a real desktop** (win-vm or laptop): Narrator reading the
  pages, the lock dialogs and the charts; display scaling at 150–200 % and a
  second screen (W33, W37: text scale and high contrast are only
  screenshots); drag and drop (block onto mode, calendar, clips, wallpaper
  bars); file dialogs; the tray icon's menu (W35) and the jump list (W36)
  opened by hand; Open at sign-in; installing and removing from Apps &
  features (W41 drives the MSI's own dialogs, not Settings); the text lock's
  challenge window opened from the GUI (W3).
- **Laptop only**: the firmware lock's status on real firmware (W39 shows
  what `firmware-lock --json` says), the Night light note with Night light
  on (W6).

### 4.2 Windows Core: parity audit 2026-11-22 (By Opus)

Each Linux-only file (`//go:build linux`), command and setting compared
with its Windows behaviour (Core session, 2026-11-22). Found weaker or
missing on Windows, in the order they are built: fixes to what is built
(`W10`–`W16`), missing features (`W17`–`W19`), added strictness
(`W20`–`W26`), then CI (`W27`). The choices marked (By Opus) are the
resolver's, not the owner's (SPEC §6 2026-11-22 By Opus). Not holes, so no
item (SPEC §6 2026-11-22 By Opus): a system proxy or VPN (hosts, the
extension and the browser policies judge names and addresses whichever way
the traffic goes; WFP's ALE filters see the real address before a tunnel);
Windows' own DNS over HTTPS (the hosts file is read first, and the DoH
filters cover Windows' known servers); WebView2 apps (hosts and WFP block
their sites; their pages are an app's, not a browser's).

- [ ] `W10` Places on Windows (M). Built (2026-11-22), Partial checked:
  `wifi_windows.go`, `winapi/wlan*.go`; layouts tested on Linux; on
  `windows-latest` (`TestWlanScanForReal`): no WLAN service, nothing seen,
  nothing breaks. Unchecked: real networks, the radio switch put back
  (Needs laptop). Was planned: SPEC §3.6, §4.5 (places), R-MODE-5,
  §6 2026-11-16. Was: `enforce/places.go` runs `nmcli`, so on Windows no
  network is ever seen: every place counts as here (blocks with a place
  rule always on) and a place can't be saved. Build: `ScanWifi` and the
  rescan through the Native Wifi API (`WlanOpenHandle`,
  `WlanEnumInterfaces`, `WlanScan`, `WlanGetNetworkBssList`; SSID and
  BSSID of every interface), and keeping Wi-Fi on (radio switched on with
  `WlanSetInterface` software radio state; a disabled Wi-Fi adapter
  enabled), as `wifiRadio` already is for monk mode. Pure parts (BSS list
  parsing) tested on Linux. Needs: win-ci (no Wi-Fi on the runner: "nothing
  seen" path), laptop (real networks; Windows 11 24H2 asks location
  permission of Wi-Fi scans by apps: Unchecked whether a service is asked).
- [x] `W11` Browsers started for remote control on Windows (S). Built
  (2026-11-22): `remotecontrol_shared.go`, `remoteControlledWin`; checked
  on `windows-latest` (`TestWinProcRemoteControlEnvironment`: another
  process's environment read, MOZ_MARIONETTE found); switches tested on
  both systems. A real browser closed by the guard: Unchecked (W2's
  win-vm run). Was planned: SPEC
  §3.13, §4.10 (tamper protection: close browsers if the extension
  stops). Was: `RemoteControlAllowed` false and `guardedProcess.remote`
  never set: a browser with `--remote-debugging-port`, `--marionette`,
  `--remote-debugging-pipe`, `--disable-extensions` or `MOZ_MARIONETTE`
  kept running. Build: the guard reads each browser's command line
  (`commandLine`) and environment (its PEB, `ReadProcessMemory`) and
  closes it as on Linux (`parseRemoteControl`, shared). Needs: win-ci
  (a renamed test program with the switches).
- [ ] `W12` Scripts and script hosts on Windows (S). Built (2026-11-22):
  `scripthosts.go` (tested on Linux), `procScript`, `matchScript`;
  checked on `windows-latest` by `TestWinProcScriptEntries` (a batch file,
  a PowerShell script, a Python script closed; another cmd left) and
  `TestWinProcFetchToolModule` (`python -m yt_dlp` with a blocked site).
  Was planned: SPEC §3.1 (apps),
  §3.9 (workaround: fetch tool). Was: `scripts.go` is Linux-only; on
  Windows an app entry naming a script (`bot.py`, `game.ps1`) matches
  nothing and a fetch tool run as a script (`python -m yt_dlp`,
  `yt-dlp.py`, `node`) isn't seen. Build: an app entry that names a script
  file matches the interpreter process running it (py/python/pythonw,
  PowerShell/pwsh `-File`, cmd `/c` batch files, wscript/cscript,
  mshta, node, java `-jar`, AutoHotkey), by name, path or folder; fetch
  tools by their script or module too. Needs: win-ci (real interpreters on
  the runner).
- [ ] `W13` A recovery path on Windows (S). Built (2026-11-22):
  `SafeModeBoot`, `KernelCmdline`, `recoveryMenuOn`,
  `cli/recover_windows.go`; checked on `windows-latest`
  (`TestRecoveryBootWin`: a normal boot is none; Safe Mode carries
  kc9k.nofreeze). Unchecked: a real Safe Mode boot, F8 (Needs win-vm,
  `-Reboots`). Was planned: Principle 4, R-ESCAPE-1,
  §3.11 (`kc9k recover`). Was: `kc9k recover` reads `/proc/cmdline` and
  never works; Windows has no `kc9k.nofreeze`, so a buggy freeze or strict
  mode could trap the user, the more so with the recovery environment off
  (factory reset protection). Build (By Opus): Safe Mode is Windows'
  recovery boot: in it, freeze and strict mode's lockdown are skipped for
  that boot and `kc9k recover` works, unless no-escape is on (blocks stay
  enforced: the service runs in Safe Mode, SPEC §6 2026-11-21); while the
  recovery environment is off and no-escape isn't on, the F8 boot menu
  (`bootmenupolicy Legacy`) keeps Safe Mode reachable; no-escape turns it
  off. Needs: win-ci (fakes), win-vm (`-Reboots`).
- [ ] `W14` Guest account (S). Built (2026-11-22): `syncGuestAccount`,
  `winapi.LocalAccounts`, `SetAccountDisabled`; checked on
  `windows-latest` (`TestWinGuestAccount`: off while on, back on only if
  it was). A person signing in as Guest: not possible while off. Was
  planned: SPEC §4.10 (close guest profiles also
  turns off the login screen's guest session). Was: `SyncGuestOff` does
  nothing. Build: the built-in Guest account (RID 501) is disabled while
  the setting is on, put back after; turned on by hand meanwhile: put back
  and counted. Needs: win-ci.
- [ ] `W15` Guided setup on Windows (S). Built (2026-11-22):
  `guidedsetup_windows.go`; `TestGuidedSetupWindows` (fake accounts) and
  `TestLocalAdminsForReal` (the runner's accounts) on `windows-latest`.
  GUI: on Windows `kc9k guided-setup --json` has `other-admins`,
  `builtin-admin` and `safe-mode` (state `info`) instead of `groups` and
  `root-password`; show each like any step. Was planned: SPEC §4.10, §6 2026-11-21 (By
  Opus). Was: Linux's steps (sudo groups, `/etc/shadow`, GRUB). Build:
  Windows' steps: settings password, no weakening, tamper protection,
  strict mode and its lock, other administrator accounts and the
  built-in Administrator (instead of root-equivalent groups and the root
  password), installer media guard, firmware lock, Safe Mode and other
  systems (info), no-escape last. GUI: the step IDs and texts of `kc9k
  guided-setup --json` differ on Windows.
- [ ] `W16` Windows wording (S). Built (2026-11-22): `cli/wording.go`,
  `TestWindowsWording` (every command's texts on `windows-latest`);
  `browser` install and `firmware-lock` texts change with W18 and W19.
  GUI: help texts shown on Windows come from `kc9k.exe` and now name
  Windows' things. Was planned: SPEC §3.14, R-GUI-3. Was: help texts and
  messages name `sudo`, GRUB, `kc9k.nofreeze`, `/etc/hosts`, `systemctl`,
  `journalctl` on Windows too. Build: each says Windows' own on Windows
  (the service, Safe Mode, the hosts file, `kc9k.log`). GUI: help texts.
- [ ] `W17` Hardware clock on Windows (S). Built (2026-11-22):
  `enforce/rtc_*.go`; `TestParseBootEvent` on both systems,
  `TestWinHardwareClockForReal` on `windows-latest`. Was planned: SPEC §4.10 (clock: check
  against hardware clock and internet time). Was: no reader; the clock
  check gives back powered-off time by internet time alone. Build (By
  Opus): Windows reads the hardware clock at boot and logs it (event 12 of
  Kernel-General, "The operating system started at system time"); that
  time plus the boot clock is the hardware clock's count, which caps what
  the check gives back as on Linux. A cleared log: internet time alone, as
  now. Needs: win-ci (the event on the runner).
- [ ] `W18` One-click browser install on Windows (S). Built (2026-11-22):
  `InstallBrowser` (winget); `TestWinInstallBrowserWinget` (the call,
  faked) on `windows-latest`; a real install on manual runs of
  `windows.yml`; as SYSTEM through the service: Unchecked (Needs win-vm).
  GUI: on Windows `kc9k browsers install` takes librewolf and chrome
  (no chromium). Was planned: SPEC §4.10
  (Browsers: one-click install). Was: refused with a hint. Build:
  `browser install librewolf|chrome` runs winget as SYSTEM (`--scope
  machine --silent`, its published package ids), an outside query stated
  in the help (principle 6). Needs: win-ci (winget on the runner;
  `workflow_dispatch` only), laptop.
- [ ] `W19` Firmware lock on Windows (M). Built (2026-11-22):
  `firmware_wmi.go` (tested on Linux), `firmware_windows.go`;
  `TestWinFirmwareStatusForReal` on `windows-latest` (no vendor's
  interface there: none, no error). Unchecked: real Dell, Lenovo and HP
  firmware (Needs laptop, owner: which vendor, and a person to type and
  keep the password). Was planned: SPEC §3.8, §4.10, §6 2026-11-21
  (By Opus). Was: not built. Build: the same firmware interfaces Linux's
  `firmware-attributes` drivers use, through WMI, built into the
  firmware and Windows (no vendor tool to install): Dell
  (`root\dcim\sysman\biosattributes`), Lenovo (`root\wmi`
  `Lenovo_BiosSetting`, `Lenovo_SetBiosSetting`, `Lenovo_SaveBiosSettings`,
  `Lenovo_SetBiosPassword`), HP (`root\HP\InstrumentedBIOS`,
  `HP_BIOSSettingInterface`); the same purposes and order (settings first,
  password last; the password over stdin, never stored); a computer
  without them: "set the firmware password by hand", as on Linux. The
  vendors' tools (Dell Command | Configure, HP BCU) are not needed.
  Pure parts (reading each vendor's answer) tested on Linux. Needs: laptop
  (which vendor), owner.
- [ ] `W20` Strict mode: Windows' ways to SYSTEM (M). Built (2026-11-22):
  `strict_windows.go`; checked by `ci/windows_service_test.ps1` on
  `windows-latest` (each refused for real during a lock, and allowed
  again after). Was planned: Principle 3 (strict
  mode: any ordinary-user path to a blocked site is a real hole), SPEC §6
  2026-11-21 (later). Linux's are sudo, pkexec, root login, su and the
  container sockets (`lockdown.go`, `pamguard.go`, `sockets.go`);
  Windows' are what makes an administrator SYSTEM, which strict mode
  never denies. Build (By Opus), while strict mode holds: the service
  manager denies people's logons creating services (`sc create`, PsExec,
  drivers); every service denies them changing its configuration, owner
  or permissions; the `Services` key tree is denied to them; scheduled
  tasks people make to run as SYSTEM (or another service account) are
  deleted and counted each cycle; permanent WMI event subscriptions
  likewise; Windows Installer is turned off for everyone but SYSTEM
  (`DisableMSI`; strict mode is Linux's "no sudo apt"). All kept and put
  back as the deny entries are. Needs: win-ci (each refused for real).
- [ ] `W21` Strict mode: administrators' privileges (M). Built (2026-11-22):
  `strict_windows.go`; checked by `ci/windows_service_test.ps1` on
  `windows-latest` (each refused for real during a lock, and allowed
  again after). Was planned: Same SPEC. An
  elevated administrator's backup, restore, take-ownership, debug, driver
  and firmware-variable privileges pass by deny entries (`robocopy /B`
  rewrites the hosts file). Build (By Opus), while strict mode holds:
  those privileges are removed from every person's running process's
  token (`SE_PRIVILEGE_REMOVED`, each cycle) and from Administrators'
  user rights (LSA, from the next sign-in), put back after. Needs:
  win-ci.
- [ ] `W22` Strict mode: new accounts (S). Built (2026-11-22):
  `strict_windows.go`; checked by `ci/windows_service_test.ps1` on
  `windows-latest` (each refused for real during a lock, and allowed
  again after). Was planned: Same SPEC. Linux's user can't
  make accounts without root; Windows' administrator can, and the built-in
  Administrator can be turned on. Build: accounts made or turned on while
  strict mode holds are turned off and counted (each cycle); put back as
  they were after. Needs: win-ci.
- [ ] `W23` The clock and the time zone (S). Built (2026-11-22):
  `clockguard_windows.go`; checked by `ci/windows_service_test.ps1` on
  `windows-latest` (a time zone and a clock moved 2 h, each put back
  during a lock). Was planned: SPEC §4.10 (clock: block
  time/date changes). Trusted time already ignores the clock; schedules
  and windows read the local time, which an administrator (and, for the
  time zone, any user) may change. Build (By Opus): with tamper clock on
  and anything locked, a time zone change is put back and counted, and a
  jump of the system clock against the boot clock that internet time
  doesn't confirm is put back and counted. Needs: win-ci.
- [ ] `W24` Strict mode: virtual machines, WSL, Windows Sandbox,
  containers (S). Built (2026-11-22): `closeVirtualMachines`;
  `TestStrictVMProcesses`, and on `windows-latest` a VM program closed and
  Docker's engine stopped during a lock. Unchecked: a real WSL
  distribution, Sandbox and a Hyper-V VM (Needs win-vm). Was planned: SPEC §3.8 (strict mode: no containers/VMs). Was: nothing
  on Windows; a WSL distribution or VM has its own DNS and network path
  past hosts and possibly WFP. Build: while strict mode holds, people's
  WSL, Windows Sandbox, Hyper-V, VirtualBox, VMware, QEMU and Docker
  programs are closed and WSL's VMs shut down. Needs: win-ci (WSL on the
  runner), win-vm.
- [ ] `W25` Strict mode's noexec on Windows (M). Built (2026-11-22):
  `noexec_windows.go`; checked by `ci/windows_service_test.ps1` on
  `windows-latest` (a program in Downloads refused during a lock, the
  rules gone after). Unchecked: Windows 11 Home (SRP's enforcement there:
  Needs laptop). Was planned: SPEC §3.8 (nothing runs
  from home or temporary folders except exempt folders), §6 2026-11-21
  (By Opus). Was: `ApplyNoexec` does nothing; downloaded programs are
  closed within a cycle (a short one still runs). Build (By Opus):
  Software Restriction Policies, registry only so Home has them:
  disallowed paths for each profile's own folders (not AppData), the
  temporary folders and removable drives, exempt folders unrestricted, for
  every user including administrators; put back after. Closing stays.
  Needs: win-ci (a program refused for real).
- [ ] `W26` Other ways around on Windows (S). Built (2026-11-22):
  `firefoxreg_windows.go` (`TestRegPolicyFirefoxMirror`), port 853 in the
  DoH filters (`TestWFPBlocks`); IPv6 through hosts checked by
  `TestWFPHostsIPv6` on `windows-latest` (`::` lines only if it fails).
  The Store's Firefox obeying: Unchecked (Needs win-vm). Was planned: SPEC §3.13 (policies),
  §4.10. Firefox-family policies also in the registry
  (`HKLM\SOFTWARE\Policies\Mozilla\Firefox`, which Store/MSIX and portable
  Firefox read where no `distribution` folder is found); DNS over TLS/QUIC
  (port 853) to the DoH resolvers blocked like DoH; IPv6: a hosts entry
  (`0.0.0.0` only) checked to answer AAAA queries too, `::` lines added if
  not. Needs: win-ci.
- [ ] `W27` Desktop checks in CI (M). Built (2026-11-22):
  `ci/windows_desktop_test.ps1` in `windows.yml`. Stays in the VM script:
  what needs a person (the challenge window and console typed in,
  notifications seen, gamma looked at) or a reboot. Was planned: `notes/testing.md`. GitHub's
  `windows-latest` has a signed-in desktop: the checks of
  `ci/windows_vm_test.ps1` that need one but no person or reboot move to
  `windows.yml` (agent running in the session, titles, apps killed, the
  challenge window refusing injected keys, notifications, dark mode,
  wallpaper, gamma). Needs: win-ci.

#### For the owner: what CI can't show (Core, 2026-11-22)

Ready to do, in this order. Never on a computer in use; the laptop runs
only what is marked laptop.

- **win-vm** (a Windows 11 VM with a snapshot, signed in as an
  administrator): `pwsh ci/windows_vm_test.ps1 kc9k.exe` (the person's
  checks: Edge and LibreWolf showing a blocked site, the notification, the
  text lock's challenge in a console and in its window with Ctrl+V and the
  on-screen keyboard refused, the lock freeze, takeown against strict
  mode, gamma); then with `-Reboots` from a fresh snapshot: Safe Mode
  holds blocks but not a freeze or strict mode, `kc9k recover` there, F8
  while factory reset protection acts (W13), no-escape closing Shift+
  Restart, Reset this PC failing during a lock (W5). Also: a real WSL
  distribution, Windows Sandbox and a Hyper-V VM closed by strict mode
  (W24); the Store's Firefox obeying the registry policies (W26); `kc9k
  browsers install librewolf` through the service (W18).
- **laptop** (the owner's Windows laptop): places with real networks, and
  Wi-Fi switched back on after "Wi-Fi off" and airplane mode (W10);
  Software Restriction Policies on Windows 11 Home: a program in Downloads
  refused during strict mode (W25); the panel's brightness, external
  screens and Night light (W6); the GUI's Narrator, scaling and drag and
  drop (W4, the GUI session's).
- **owner**: the Chrome Web Store listing's ID for `ChromeStoreExtensionID`
  (W2; until then Chrome and Edge get no extension); which firmware the
  laptop has (Dell, Lenovo, HP) and a trusted person to type and keep the
  firmware password for `kc9k firmware-lock on` (W19); the code-signing
  certificate (W7, the GUI session's).
- Not Core's but seen: the Linux workflow `Test` fails in
  `ci/integration.sh` on every push since before 2026-11-22 (commands
  without `--yes` where a lock is now confirmed, R-LOCK-12); its unit
  tests pass.

## 5. Proposed, not agreed

- DECIDED 2026-11-21 (By Opus; SPEC §4.6, §6): setting locks stack like a
  block's, as proposed below; built (`2.5`).
  PROPOSED 2026-11-20 (session, `2.8`, §1 "Layered sequential locks"):
  SPEC §4.6 and §6 2026-11-20 say a lock of a kind a *block* already has is
  added as a second lock, last, and that stacking has no exceptions; they
  don't say whether setting locks (browser, tamper, strict, bypass, freeze
  schedules) stack too. Built: one lock per kind on a setting, and a new
  one strengthens it (timer extended, challenge lengthened). Proposed: the
  same as blocks (a queue on `setting_locks`, as `LockExtras.Queue`),
  because a setting lock is "a lock" in §3.4's sense and the owner's
  examples (two partners' passwords) fit settings as well. Not built until
  the owner agrees.

- DECIDED 2026-11-21 (By Opus; SPEC §4.10, §6): keep what was built: the
  other engines go with any of Google's, Bing's or DuckDuckGo's SafeSearch,
  not YouTube's, and get no setting of their own. Why: each of them is a
  way around whichever engine is kept strict, so a setting of their own
  could only open that hole; tied to Google's alone they would be open
  while only Bing's or DuckDuckGo's is on; YouTube Restricted Mode is about
  video, not search. Nothing to build: the GUI (under Google's setting)
  and `kc9k browser --help` already say it.
  PROPOSED 2026-11-20 (session, building `2.4`): SPEC §4.10 and §6
  2026-11-18 name four engines with their own SafeSearch setting and are
  silent on the rest: Brave, Yahoo and SearXNG (kept strict from the
  address) and the engines that can't be kept strict (Startpage, Ecosia,
  Mojeek, Yandex, Marginalia: blocked). Built meanwhile: they go with any of
  Google's, Bing's or DuckDuckGo's SafeSearch (not YouTube's), because
  with one search engine kept strict the others are the way around it, and
  YouTube Restricted Mode alone shouldn't block search engines. Owner: keep
  this, give them a setting of their own, or tie them to Google's only?

- DECIDED 2026-11-17 (session; the owner asked that a block always be a
  draft until Save edits; extended 2026-11-21 (By Opus), SPEC §6: the
  Breaks & allowance and Bypass prevention tabs are drafted now too): the draft holds what the Websites and Apps tabs
  show (lists, phrase sets, social media settings, test mode, apps, window
  titles and their actions); the block's menu (Start, Stop, Unlock,
  Automation) and the Breaks & allowance and Bypass prevention tabs still
  apply at once, each with its own button, because those are forms that
  already wait for their button, and carrying their gates (an allowance
  switches its block on; Pomodoro, rewards and ladders have their own
  weakening rules) through a draft is a larger change the owner can ask
  for. The draft is made at the first edit rather than when the block is
  opened, so looking at a block changes nothing.

- DECIDED 2026-11-17 (session; the owner asked for modes and schedule
  pages, these details were left open): a block is in at most one mode,
  because its automation can follow only one; a mode's switch switches
  its blocks on or off (locked ones stay on and are named), rather than a
  second switch over theirs; schedules are shared like places (changing
  one changes every block using it, R-MODE-2) rather than copied, so
  *Evening* is changed in one place; where windows of a block's schedules
  overlap they are stored clipped, so a block's windows still never
  overlap (R-SCHED-2); deleting a schedule or a mode leaves the blocks as
  they are (their windows and automation stay), and taking a block out of
  a mode leaves it the mode's automation, so neither changes what is
  enforced; blocks, modes and schedules share one set of names, so a
  command names one without asking which. Monk mode's automation is
  always disabled by default (its windows and places turn it on): enabled
  by default it would cut the network everywhere else, with nothing to
  switch it off; with no network seen its places count as there, as a
  block's do (§3.6), and Wi-Fi is kept on to see them.

- DECIDED OVERNIGHT: how to check the clock against hardware and internet
  time (§4.10) → not built yet; trusted time stays CLOCK_BOOTTIME with
  powered-off time not counted, because that is the strictest choice: a
  hardware-clock count would only end timer locks sooner and the hardware
  clock can be set in firmware without a firmware password, and an internet
  time query is a new outside query (principle 6) the owner should agree to.
  Owner agreed 2026-11-18 (SPEC §6): build the check (`A10`). Built
  2026-11-20: see `A10`.
- DECIDED OVERNIGHT: a huge list in /etc/hosts makes every name lookup on
  the computer slow (15 ms each at 100,000 sites) → the hosts layer holds
  at most 20,000 sites, the rest are blocked by the browser policy, the
  extension and the firewall, because principle 9 says huge lists never
  slow the computer and those layers already cover every site. (Corrected
  2026-11-17: Chrome's policy takes 1,000 entries at most; they now go to
  what /etc/hosts leaves out first.)
- DECIDED OVERNIGHT: should SafeSearch make the extension required (closing
  browsers without it)? → no: Bing, DuckDuckGo and YouTube are pinned in
  /etc/hosts for every browser, and the extension adds the rest where it
  runs, because requiring it would close Firefox whenever SafeSearch is on,
  a new default behaviour (principle 7).
- DECIDED OVERNIGHT: where does "normal browsing" for specificity (§3.12)
  come from? → a word baseline gathered by the extension (per word, the
  pages it showed on; no addresses, no private windows, local only), off
  until the first word recording turns it on; `record baseline off`
  forgets it, because specificity needs it, but gathering browsing words
  unasked goes beyond what SPEC asks.
- DECIDED 2026-11-20 by the owner (SPEC §6 2026-11-20, §4.6): several
  locks of one kind (`2.5`) → each its own layer; a lock of a kind a block
  already has is added as a second lock of that kind, last in the unlock
  order, not a strengthening of the first. It replaced the resolver's
  decision of the same day, which had such a lock strengthen the one
  already on unless the user asked for a new lock; SPEC no longer has
  that. Built 2026-11-20 (§2 `2.5`), with exceptions: for the owner, the
  next entry.
- DECIDED 2026-11-20 by the owner (SPEC §6 2026-11-20 (later), §3.4,
  §4.6): the build's four exceptions to stacking (`2.5`) all go. (a) A new
  timer or delay lock is its own layer, not a change to the last one;
  extending a timer lock stays a separate action. (b) Range locks stack,
  unlocked in their order. (c) Text and password locks stack on a block
  with re-lock; re-lock brings back the whole lock configuration the block
  had, every lock in its order. (d) Each hotkey press adds its locks again
  (R-LOCK-12 confirms). Built 2026-11-20 (§2 `2.5`).
- DECIDED 2026-11-20 (By Opus): how the text lock's challenge window
  (`2.7`; SPEC §6 2026-11-14) gets its answer without breaking R-LOCK-5 →
  the command opens the window itself (as the signed-in user, on their
  desktop) when it runs without a terminal, and only keys typed there
  count; a paste, a drop or an answer piped to the command is refused; the
  GUI runs the command (`kc9k unlock …`) and opens no terminal. Why: the
  CLI decides all weakening (principle 10) and can tell typing from
  pasting only in input it reads itself; a GUI handing the typed text over
  on stdin would let any script do the same. Today a non-terminal stdin
  is read as one line (`runChallengeLine`, `internal/lock/lock.go`): that
  path goes too (tests keep a faked seam). SPEC §4.6 now has it.
- DECIDED 2026-11-21 (By Opus; SPEC §6; the owner asked for a decision):
  drop the secret. Why: it can't keep local programs out (the policy files
  carrying it are readable by every program the user runs, and the lists
  are in `/etc/hosts` and the policy files anyway), and an extension that
  can't read it fails every check-in, so the guard would close browsers:
  a real risk for no protection. What it was meant to stop is stopped
  without it, as below; built 2026-11-21: `rulesRoutes` lists every address
  the server answers and `TestRulesServerGuardsEveryRoute` holds each to
  the guards (rebound host name refused, posts without the extension's
  header refused, CORS preflight refused, no CORS header, nothing read
  before a refusal), so a new address can't forget one. No change to what
  the extension sends: check-ins can't fail because of it.
  Was: FOR THE OWNER: the rules server secret (`2.7`; SPEC §6 2026-11-14, its
  purpose as first written: "so other local programs can't read the
  lists, post statistics or fake a check-in"). As designed it can't do
  that: browsers run as the user, so the policy files that would carry the
  secret are readable by every local program (written 0644). What already
  holds without it: the server answers only its own address as host name
  (no DNS rebinding) and counts check-ins and takes posts only with a
  header of the extension's, which a web page can't send without a CORS
  preflight the daemon never answers; a check-in counts only for the
  browser whose own connection made it (`/proc/net/tcp`), so another
  program can't fake one. The user's own programs can read `/rules` with
  or without a secret. The risk: an extension that can't read the secret
  fails every check-in, and the extension guard closes its browser. To
  decide: build it as decided (it stops only programs that don't look for
  the secret), give the secret to the extension some other way, or drop
  it.

- OPEN 2026-11-20 (A1 check, desktop): a site entry misses an address with
  a port: `127.0.0.1` doesn't match `http://127.0.0.1:47999/` in the
  extension (`kc9kIndexTest` compares the host with its port);
  `127.0.0.1:47999` does. SPEC is silent on ports. Proposed: a site entry
  without a port matches any port, as browser match patterns do.

- FOR THE OWNER (B3, 2026-11-20): strict mode now turns on AppArmor's
  `kernel.apparmor_restrict_unprivileged_userns` while it acts (Ubuntu's
  default; Mint turns it off), or `unshare -Urm` mounts a tmpfs that runs
  programs and nothing from `/home` is stopped. Programs with an AppArmor
  profile (Chrome, LibreWolf, Firefox, Flatpak, Steam, Discord, Slack,
  VS Code on Mint 22) are unaffected; an Electron or Chromium app without
  one, run from an exempt folder (an AppImage or unpacked app), aborts
  ("SUID sandbox helper binary ... not configured correctly") unless
  started with `--no-sandbox`. To decide: keep it (as on stock Ubuntu),
  or have KC9K load an AppArmor profile allowing user namespaces for the
  exempt folders (they may already run anything).
- (B4, 2026-11-20) Changing a running freeze's mode: `freeze --for D
  --mode M` extends a running freeze and takes M as its mode, and a
  scheduled freeze whose window ends later does the same; SPEC says a
  running freeze can't end (R-FREEZE-2) but not whether its mode may
  change. Seen in the VM: a lock freeze extended with `--mode apps` (an
  allowed app listed before) stopped locking the screen; a text console
  login is open during a lock freeze, so anyone there can do it.
  Proposed: while a freeze runs, a mode only changes to a stronger one
  (allowed apps only < lock screen < logout < shutdown); a weaker one is
  refused (`refused()`, not a bypass), and an extension that names none
  keeps the running mode. Partly decided since (SPEC §4.8 *(By Opus)*,
  built as `freeze.ExtendMode` by the cloud session 2026-11-21): a running
  lock, logout or shutdown freeze never becomes allowed apps only, which
  closes the case seen. Still open: other weaker changes (shutdown →
  logout → lock screen); not rechecked in the VM.
