# KC9K Roadmap

Code status against `SPEC.md`. The only status file.

Rules: every item cites a § or rule ID (unagreed work goes in §5 first).
Update in the same commit as the code. A wrong status is fixed, not built
on; write "Unchecked" rather than guess. Order: §0 first, top to bottom,
then §2.

Status: Built, Partial, Differs (built unlike SPEC), Not built, Unchecked.
Sizes: S < 1 day, M days, L ≥ week. Last checked: 2026-11-20 (docs against code: commands, rule IDs, paths, notes' Covers lines; §0.1 *Doc audit 2026-11-20*).

## Index

One row per open item; find it with ``grep -n '`A3`' ROADMAP.md`` and
read only that item. Needs: **agent** (code or scripts, cloud or test machine),
**box** (an agent on the test machine: root, X11 desktop, real browsers,
network, screenshots), **vm** (box, but in a VM with a snapshot: logout,
shutdown, network cuts, strict mode, reboots), **owner** (only the owner:
a decision, a verdict, or real hardware input). New items get the next ID
of their group and a row here; a finished item loses its row.

| ID | Item | SPEC | Notes | Needs |
|---|---|---|---|---|
| A1 | `no-images` GIF drawn by a pseudo-element: run in LibreWolf, the owner's site | §3.9 | extension | box |
| A2 | Looks from page text lifted on clicks: suites pass (2026-11-20); real use on the laptop | §3.9 | extension | owner |
| A3 | `no-downloads` in Chrome | §3.9 | extension, browsers | box |
| A4 | Brightness slider gone, night light off | §4.10 | circadian | box |
| A5 | Phrase editor save: confirm | §4.3, R-WEAK-5 | lists, gui | box |
| A6 | Locking an enabled block: confirm | R-LOCK-7 | locks, gui | box |
| A8 | No flashing: Chrome and real sites | R-PAGE-1 | extension | box |
| A10 | Clock check built: a real power-off mid-lock | §3.4, §4.10 | locks | vm |
| A11 | Gigantic URL lists: Firefox/LibreWolf with 100,000 entries | §2 principle 9 | browsers, layers-net | box |
| A12 | R-LOCK-12: a person clicks Lock and Cancel (synthetic input is ignored by design) | R-LOCK-12 | hotkeys, locks | owner |
| A13 | `install.sh` DB race: fixed; confirm a fresh install in the VM | — | install, db-project | vm |
| B2 | Scheduled Frozen Chicken in logout/shutdown modes | §3.8 | device | vm |
| B3 | Strict mode holes | §3.8 | device | vm |
| B4 | Shutdown mode and `kc9k.nofreeze`; fast relock | §3.8 | device | vm |
| B5 | Rendered terms on real sites (box); hotkeys on a real keyboard (owner); GNOME (vm) | §3.1, §4.13 | extension, hotkeys | box, vm, owner |
| B6 | Websites tab on a real screen; YouTube address change | §4.3 | gui, extension | box |
| B7 | One-off logout/shutdown from a list; Dashboard monk button | §3.9 | actions, device | vm |
| B8 | Use counting with input and idle (simulated input) | §3.7 | allowance | box |
| B9 | Mute and dark on real sites; polite close with a real app | §3.9 | extension, apps-titles | box |
| B10 | No-video on a real embedded player | §3.9 | extension | box |
| B11 | Social media settings beyond Shorts; playlists; places; text break | §3.2, §3.6 | extension, automation | box |
| B12 | Chrome: popup, extension can't be removed | §3.13 | browsers | box |
| B13 | Block unsupported browsers: upgrade keeping a lock; portable LibreWolf | §4.10 | browsers | box |
| B14 | Monk mode live: run `ci/monk_live_test.sh host` (written, never run), reboot mid-way | R-MONK-2 | device | vm |
| B16 | New list dialog on a real screen | §3.2 | gui, lists | box |
| B17 | GUI "lifted by its schedule now" on a real screen | §3.3 | gui, automation | box |
| 2.1 | Circadian on real hardware (laptop backlight, screens) | §4.10 | circadian | box |
| 2.2 | Ad trigger: browser test; the new industry domains in real ad requests (Unchecked) | §3.9 | actions, extension | box |
| 2.4 | SafeSearch per engine: built; in real browsers (Unchecked) | §4.10 | browsers | box |
| 2.6 | Factory reset protection: built; PAM root guard in the VM (vm), firmware lock on real Dell/Lenovo/HP firmware (owner) | §4.10 | device | vm, owner |
| 2.7 | Hardening: rules server secret (for the owner, §5); the text lock's window on a real desktop (Unchecked) | R-LOCK-5, §4.6 | browsers, locks | owner, box |
| 2.8 | Every Partial/Not built row of §1 | see §1 | see §1 | agent |
| 2.9 | Tamper settings of 2026-11-21 (guest, extension-stop, installer, inactive-tabs, embedded): live checks | §4.10 | browsers, extension, install | box, vm |
| D3 | Instagram "Following only": built; on the real site (the redirect sticks, no suggested posts) | §3.2 | extension | box |
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
    `query` that stops them; `ci/match_test.js`). Left (box): on the real
    site, incl. clicking the logo; nothing hides suggested units yet (CSS
    can't match their text).
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
  - [ ] `A3` `no-downloads` doesn't block downloads (§3.9). Checked 2026-11-16
    in LibreWolf: a link on the page, a file from another host (only the
    origin as referrer) and a file the page makes itself (`blob:`) are all
    cancelled (`ci/download_test.py`, four cases). Likely cause in Chrome:
    its service worker is stopped after a while idle and forgot which pages
    forbid downloads (each page reports once). Fixed in 1.21.0: the pages
    are kept in session storage; a download that finishes before it can be
    cancelled has its file removed. Unchecked in Chrome (not run here). S
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
  - [ ] `A5` Phrase editor: saving a change said the set has no such phrase
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
    locked block, export/import). Confirm on the laptop.
  - [ ] `A6` Locking an enabled block refused password, restart, delay, range
    and word/own-text locks ("already active; … are set when it starts"),
    against R-LOCK-7. Fixed: `strengthenLocks` adds every kind last in the
    order given (`TestLockActiveBlock`). The GUI's lock dialog offers every
    kind on a running block too (merged 2026-11-16); confirm on the laptop.
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
  - [ ] `A8` No flashing (R-PAGE-1). Built (extension 1.20.0): while a list or
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
    sites and branded Chrome: Unchecked.
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
- [ ] `A10` **Lock model** (§3.3, §3.4, R-LOCK-1–11). Done: schedule lock
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
  off, RTC exact). Unchecked: a real power-off during a timer lock (needs a
  VM: shut it down for an hour mid-lock, boot, `journalctl -u kc9k | grep
  'clock check'`, the lock's end in `kc9k status`). (L)
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
- [ ] `A11` **Gigantic URL lists** (principle 9). Extension: done (`kc9kIndex`,
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
  an allowlist past 1,000 exceptions blocks the rest in Chrome. Firefox
  and LibreWolf with 100,000 entries: Unchecked. (M)
- [ ] `A12` **R-LOCK-12 gaps**: manual `kc9k monk` now confirmed (`--yes`), and
  `project import` refuses monk steps/actions without `--no-confirm-ok`
  (`projectMonk`); both in `confirm_test.go`. `block import` the same. The
  hotkey press dialog (`blocker.ConfirmDesktop`) ran live on the laptop
  (2026-11-15): zenity shows "Lock 'B'?" with target, kind and length;
  unanswered (screen locked) it times out after 60 s and nothing is locked;
  synthetic keys from other programs are ignored. Left: a person clicks
  Lock and Cancel on an unlocked screen. (S)
- [ ] `A13` **`install.sh` races the daemon for the DB.** Seen 2026-11-19 on a
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
  Left: a fresh install on the VM (`ci/vm.sh`), Unchecked. Not covered:
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

- [ ] `B2` Scheduled Frozen Chicken in logout/shutdown modes.
- [ ] `B3` Strict mode holes: `unshare -Urm` (`kernel.apparmor_restrict_unprivileged_userns`), `--user` Flatpaks killed, Electron not taken for browsers, other accounts and guest blocked.
- [ ] `B4` Shutdown mode and `kc9k.nofreeze`; measure fast relock.
- [ ] `B5` Rendered terms on real sites for days; hotkey on a real keyboard; hotkey under GNOME.
- [ ] `B6` Websites tab on a real screen. (Address change without load: a
  `history.pushState` page is caught by a URL list in LibreWolf,
  `extras_test.py`; on YouTube itself still Unchecked.) Lists are
  taller by dragging the grip under them (owner 2026-11-19; height kept
  per kind in gui.json, `ci/gui_list_resize_smoke.py` under Xvfb): try it
  on a real screen.
- [ ] `B7` One-off logout/shutdown from a list; Dashboard monk button. (Checked
  2026-11-15: a timer-locked global ladder refuses `bypass off` and a weaker
  ladder, and turns off once the lock ends.)
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
- [ ] `B9` Mute and dark on real sites; polite close with a real app.
- [ ] `B10` No-video from page text on a real embedded player (YouTube/Vimeo iframe), in LibreWolf and Chrome (`extras_test.py` checks a local one in LibreWolf).
- [ ] `B11` Social media settings beyond Shorts (`social.js` selectors written from memory); playlist videos playing; block following places; text break returning.
- [ ] `B12` Chrome on the laptop. Checked 2026-11-15: the extension checks in from
  Chrome and Chromium, both show the block page; the policy force-installs
  and pins it, turns off incognito, guest and developer mode. Left (needs
  a person): the popup, and that the extension really can't be removed in
  the Extensions page.
- [ ] `B13` Block unsupported browsers (`kc9k tamper unsupported-browsers`).
  Checked 2026-11-15: Firefox is closed within ~2 s with the reason logged;
  LibreWolf with the extension stays (browser_test); `kc9k browsers install
  chromium` installs Mint's chromium (`/usr/lib/chromium/chromium`, policy
  in `/etc/chromium/policies`) and works. Left: upgrade from
  `force-librewolf` keeping a lock; a portable LibreWolf.
- [ ] `B14` Monk mode live, incl. reboot mid-way. `ci/monk_live_test.sh`
  written 2026-11-20 (the VM checks itself while cut: three cuts, undoing
  each put back, locked, restart, held after it, restored at the end);
  never run: `ci/monk_live_test.sh host kc9k-vm` from the test machine.
- [x] `B15` GUI design pass: seen by the owner on a real screen (2026-11-18: tremendous).
- [ ] `B16` Lists without types (§3.2): checked 2026-11-15: `H` and `test` have
  one `Blocked` each; `H` blocks reddit.com by address and a page by its
  text in LibreWolf. Left: the New list dialog on a real screen.
- [ ] `B17` Active vs enabled on the laptop: checked 2026-11-15 with a throwaway
  block: off it stays out of /etc/hosts in its window; on, it is in /etc/hosts
  in its window, and outside it shows "lifted by its schedule" and leaves
  /etc/hosts. Left: the GUI's "lifted by its schedule now" on a real screen.

## 1. `SPEC.md` coverage

Only items not simply Built.

| Item | Status | Notes |
|---|---|---|
| §3.1 File URL entries | Partial | built: with tamper `file-urls` on, blacklist `file:///…` entries go into Firefox/LibreWolf and Chrome policies unless the block's file exceptions spare them (checked live in LibreWolf); Chrome live: Unchecked. The extension judges file pages too (2026-11-21, SPEC §6): rules `file_pages`, `files`, `except.files` (`withFilePages`); `kc9kFileMatch`, `kc9kJudgedAddress`; by address, title and text (`ci/match_test.js`, `TestRulesFilePages`, a real `file://` page in Chromium in `ci/content_harness_test.js`). In LibreWolf: Unchecked (Needs: box). Chrome runs no content script on files unless the user allows it (no policy for that): there only the address and title (background) |
| §3.2 Block preset | Built | your own saved blocks: `block-preset save B P`, `block-preset`, `block-preset remove P`, `block create X --preset P` (old names `template`, `--template` kept as aliases); GUI ⋯ "Save as block preset…" and a choice in New block; Presets page; projects (a choice, always in a backup). None shipped |
| §3.2 Affected users | Partial | `block users B all|USER…` (GUI "Users:" button, gui-state `users`/`accounts`, projects): a block for some accounts stays out of hosts/firewall/policies (shared by every account) unless it covers them all; its lists reach only those users' extensions (`BuildRulesFor`, user from the socket uid) and its apps are closed only in their processes (`KillBlockedFor`), its window titles and folder entries judge only their windows (`TitlesByUser`, Xvfb-tested), its sites close only their fetch tools (`KillNamingFor`), and the extension guard closes only their browsers unless something needs it everywhere (`extensionUsers`, 2026-11-21). Fewer users weakens. The GUI's "isn't running" dialog still looks at every account's browsers. Live: one account on the laptop, so only "every account" checked (Unchecked; Needs: a second account on the test machine) |
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
| §3.8 Allowed apps only | Partial | freeze mode `apps` (`freeze --mode apps`, schedules too, GUI choice), list `kc9k freeze apps add|remove`; closes every regular-user process but allowed apps, their children and system/desktop helpers (`allowedAppKeep`, unit-tested). GUI list editor on the Frozen Chicken page. Live: never run (no freeze overnight) |
| §5 R-LOCK-12 | Partial | see 0.1 |
| §3.8 Strict mode exempt folders | Built | `kc9k strict exempt add|remove DIR`, GUI list; exec bind mounts over the noexec folders (`noexecPlan`); adding weakens. Not in projects (import never weakens). Live: Unchecked (strict mode not run overnight) |
| §3.8 Forgotten password recovery | Built | `kc9k-bin recover` (root, recovery boot, not with no-escape): settings password and password locks (`--access`: access passwords). Live: Unchecked (needs a recovery boot) |
| §3.8 Factory reset protection | Built | installer media guard (`reset-guard on|off`, GUI switch under device protection): while anything is locked an in-memory block (`enforce/resetguard.go`) blocks OS image and USB writer download pages and closes image writers and Disks; checked live (hosts follow the lock). Firmware lock (`firmware-lock on|off`, GUI Firmware lock…; 2026-11-21) tested on generated firmware-attributes trees only: on real firmware Unchecked (`2.6`) |
| §5 R-LIST-4/5 | Partial | case, accents and look-alikes folded (`kc9kFold`; 2026-11-21: also capitals by their own look, small capitals, Armenian, fullwidth/mathematical/circled letters, ligatures, and zero-width characters and soft hyphens inside words; extension 1.28.0, `ci/match_test.js`). R-LIST-4 checked in Chromium (2026-11-17, `ci/content_harness_test.js`): hidden (`display:none`, closed `<details>`, `visibility:hidden`) text isn't read, text far down is. Frames were never judged by their text (the content script read the top frame only, whose text holds none of its frames'): now each frame judges its own (`scanFrame`); what would take a page away gives that frame the block page (background `frameHit` decides from its own rules; no events, R-EVENT-1), its looks show in the frame; extension 1.25.0. Checked in Chromium (2026-11-21, `ci/content_harness_test.js`): a frame's hidden text isn't read, a frame inside a frame is judged alone (the frame around it and the page stay), look-alikes in a frame are read; `frameHit` edge cases (looks alone, actions the list hasn't, test mode, the top frame, non-web addresses) in `ci/background_test.js`. Frames in LibreWolf: Unchecked (Needs: box) |
| §3.9 Ad trigger | Partial | scope `ads` on blacklists and clusters (`list create --ads`, `list where B L ads on`, GUI "Ads" box): entries are ad network domains or industries (`db.AdIndustries`: adult, crypto, dating, gambling); rules `ads`; the extension watches each tab's third-party requests (`noteAdRequest`) and a page loading a listed network matches with every action (`kc9kAdMatches`, unit-tested). Not in shared layers. 2026-11-21: URL-term exceptions spare a page from it too (they didn't); a list whose block has exceptions by title or text is judged with the page (`kc9kAdsByPage`: the content script's resource timing, `page.adHosts`), so those exceptions hold (R-LIST-1). Edge cases unit-tested (`ci/match_test.js`, `ci/background_test.js`: hosts with capitals or a final dot, test mode, no actions, first-party, non-web pages, once per page) and in Chromium (`ci/content_harness_test.js`); extension 1.30.0. Browser test: Not run (the owner was using the laptop; the browser test changes LibreWolf settings and would close their windows) |
| §3.11 Block password | Built | `block password B set|remove`; hides entries, apps, titles in gui-state (`--reveal`), `block info`, exports, `list show/export`, `phrase-set show`; GUI "Show contents…"; backups keep it. A project export still holds the contents |
| §3.12 Word recording, specificity, cut | Built | `record start B L --words` / `--preset C --words` (text lists): extension reports each page's rendered words once (`kc9kPageWords`, `/words`); `kc9k record` ranks by specificity against the baseline (`db.Specificity`); `record stop --cut N`; `record baseline on|off`. Checked live in LibreWolf. GUI: Presets page (list presets) "Record words" with cut % and the most specific words |
| §4.10 SafeSearch | Built | a browser setting per engine (`safesearch-google`, `-bing`, `-duckduckgo`, `-youtube`), each its own lock; `browser safesearch` switches all four. Pinned in hosts (every browser) per engine, `safesearch_engines` to the extension (Brave, Yahoo, SearXNG, engines it can't hold blocked: with Google's, Bing's or DuckDuckGo's, §5). In real browsers: Unchecked (`2.4`) |
| §4.10 Tamper settings | Built | GUI group with OK/Cancel window; clock, block unsupported browsers (own lock), allow file:// blocking, block task managers while locked (`tamper task-managers`), grace for a missing extension (`tamper grace S`, 10–600 s, longer weakens); gui-state `anything_locked`, settings stuck on show so. No group lock. close private windows and Tor (`tamper private`: private browsing off in every browser policy, Tor Browser/tor closed; checked live for the policies; no Tor here). Built 2026-11-21 (SPEC §4.10 *(By Opus)*): `tamper guest` (guest browsing off in every Chromium-family policy, LightDM guest session off), `extension-stop`, `inactive-tabs`, `embedded` (on until turned off: what KC9K always did), `installer` (`prepare-install` from `make install`/`install.sh` and the `.deb`'s prerm for a reinstall or older version; removal always refused). `TestTamperGuestExtensionInstallerTabsEmbedded`, `TestSyncGuestOff`, `ci/tamper_tabs_test.js`, GUI tamper window. Live: Unchecked (box: Chrome-family guest windows, a background tab and an embedded frame with each setting off in LibreWolf and Chrome; vm: the LightDM guest session, `dpkg -i` of the same version and `install.sh` refused during a lock) |
| §4.10 Extension install dialog | Built | once per session when gui-state `extension_missing`: Install LibreWolf, or Show browsers |
| §4.10 Device protection: factory reset protection, guided setup | Built | 2026-11-21: firmware lock, guided setup (`kc9k guided-setup [--json]`, GUI Guided setup… with Do it…, `ci/gui_settings_smoke.py`), strict mode's PAM root guard (`login`, `su`, `su-l`, tested on generated files only). Live: Unchecked (`2.6`) |
| §4.10 Browsers list | Built | GUI Browsers group and `kc9k browsers`: LibreWolf, Chrome, Chromium, installed, per-browser extension check-in, Install (`browsers install`); live check in 0.2 |
| §4.11 `.deb` | Built | `make deb` (`packaging/build-deb.sh`): same paths as install.sh; postinst sets up sudoers, database folder, service; prerm runs prepare-uninstall, so removal is refused while locked (checked live); postrm removes sudoers. Installed on the laptop as 0.9.0~overnight (devmaster) |
| §4.3 Draft Save/Cancel | Built | `block draft B [save|cancel]`; GUI: an opened block is always a draft, made at the first change of the Websites or Apps tab, with Save edits/Cancel (§5 DECIDED 2026-11-17): a copy of the lists, phrase sets, apps and window titles that never runs; save applies each difference through its gate, refusing and naming weakening ones on a locked block; needs and keeps the block password; drafts stay out of the block list and projects. The Breaks & allowance and Bypass prevention tabs are drafted too (2026-11-21, SPEC §6): their commands on a draft are kept (`draftQueued`, `keepOnDraft`; the last of each kind), shown in the tab ("Not saved yet", gui-state `draft_kept`) and run at save through their own gates (`runQueued`); taking a break or an allowance session, the automation and affected users still apply at once (`draftRefused`). `TestDraftKeepsBreaksAndBypass`, the weakening audit, `ci/gui_lists_smoke.py`. On a real screen: Unchecked (Needs: box) |
| §4.3 Ctrl+F, multi-select, double-click edit | Built | `EditableList`: Ctrl+F to its search, Ctrl/Shift-click and Ctrl+A in website lists, phrase sets, apps, titles, clusters, exempt folders; Delete/Backspace removes, selection moves on; double-click moves an entry into the box |
| §4.3 Import "From KC9K" | Built | `list copy`; a list's Import… offers From KC9K: tick other blocks' lists and clusters; Whole internet (`*.*`, asked first) and All searches containing… (the words as rendered terms, the list's URL scope on: a search's words are in its address, R-PHRASE-6; any other address with the word matches too, and the dialog says so) (2026-11-17, `ci/gui_import_smoke.py`) |
| §4.5 Grid drag-move, copy to days, similar windows | Built | drag a window to move it (time; day for one-day windows); click to edit days (copy to days), times, its break, or remove; a window on several days is edited as one; other blocks' windows greyed |
| §4.9 Statistics: times blocked, top N, search, filter by block, colour per site/app | Built | blocked pages counted per site and block (`blocked`, `blocked-block` rows); `stats --top N --search T --block B` (JSON filtered too); GUI Times blocked view, Top 5/10, search, block filter, stacked daily chart with a colour per site/app and a legend (`ci/gui_stats_smoke.py`). Time frame: presets (`--days N`) and custom (`--from`/`--to`, GUI Custom…) |
| §4.9 Statistics per user, skip private windows | Built | skip private windows (`stats private skip|count`, rules `stats_skip_private`, GUI switch); per user (`usage_users`: apps by desktop session user, sites by the uid of the extension's socket, `blocker.ClientUser`; `stats --user U`, `--users`, GUI chooser; checked live for apps). Times blocked are counted for all users only; backups carry totals only |
| §4.10 Preferences | Built | theme with darkness slider (dark grey to black), clock 24-/12-hour (shown times: grid and clip axes, tooltips, pauses; entry fields stay HH:MM), first day of the week (schedule grid), open at login, tray (Ayatana AppIndicator or XApp status icon: what is on, open, quit; closing the window keeps it) built; tray untried on a real panel |
| §4.10 Notifications choice | Built | `kc9k notifications`, GUI group; window start/end, timer lock end, Pomodoro, allowance, app closed; advance minutes. Live: Unchecked |

## 2. Next features (after §0)

- **2.1 Circadian rhythm** (§4.10): built. Untried on real hardware: laptop
  backlight, GNOME/MATE, multiple screens. Wayland: no gamma ramps.
  System-following dark pages rely on `prefers-color-scheme`, which
  LibreWolf reports as light. In project files with its wallpapers
  (`cli/projectcircadian.go`; import only where none is set).
- **2.2 Ad trigger** (M): built (scope `ads`). Industry lists grown
  2026-11-20 (adult: TrafficHaus, TrafficShop, ClickAdilla; crypto: Mellow
  Ads, Blockchain-Ads, HypeLab; `TestAdIndustries`); networks named in
  public ad-network roundups, domains not checked against live ad
  requests: Unchecked. Left: a browser test (`ci/extras_test.py`, box).
- **2.4 SafeSearch** (S): a lock per engine built 2026-11-20 (SPEC §6
  2026-11-18): four browser settings, the old one and its lock moved to
  each (`TestMigrateSafeSearch`, `TestSafeSearchPerEngine`,
  `ci/match_test.js`, `ci/gui_settings_smoke.py`). Unchecked (box): in
  LibreWolf and Chrome, with only one engine on, that engine's searches
  are strict and the others' are not; the hosts section changes with the
  settings; YouTube Restricted Mode alone.
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
  comes back when the lock ends or with `kc9k.nofreeze`. Unchecked (owner,
  real hardware): the firmware lock on a Dell, Lenovo or HP computer (the
  VM has no firmware-attributes; vendors' names and how each driver clears
  a password are untried).
- **2.7 Hardening** (M each): sudo rule arguments audited 2026-11-15: caller
  paths are read as the caller (`readAsCaller`), direct reads only of
  KC9K's own files, writes only into KC9K's folders under hash names;
  `audit_test.go` keeps it so (`env_reset` untouched). Left: rules server
  secret by policy (not rolled out overnight: if the extension can't read
  it, every check-in fails and the guard closes browsers; the policy files
  are user-readable anyway; for the owner, §5); GTK window for the text
  lock's challenge: built 2026-11-20 (SPEC §4.6, §5 decision): without a
  terminal the command opens its own window as the signed-in user and
  reads the keys itself; stdin is never read for it; the GUI's Unlock runs
  the command, no terminal. Checked on Xvfb with xdotool
  (`TestChallengeWindowOnX`: human pace passes; a burst, Ctrl+V, Escape,
  keys sent to an unfocused window don't). On the owner's real desktop
  (Mint, Cinnamon: the window opens from the GUI under sudo, keeps focus,
  a real keyboard): Unchecked.
- **2.8** Everything else Not built/Partial in §1.
- **2.9 Tamper settings live** (S): built 2026-11-21 (§1 *§4.10 Tamper
  settings*). Unchecked (box): with `guest` on, Brave/Edge/Vivaldi offer no
  guest window; with `inactive-tabs` off, a background tab on a newly
  blocked site stays until switched to, then gets the block page (LibreWolf
  and Chrome); with `embedded` off, a page embedding a path-blocked video
  plays it. Unchecked (vm): the LightDM guest session gone after a restart
  with `guest` on; during a lock with `installer` on, `sudo dpkg -i` of the
  same `.deb` and `sudo ./install.sh` are refused and a newer `.deb`
  installs.

## 3. Testers and Android (owner 2026-11-18)

When §0–§2 are done: the owner's brothers test KC9K on their computers
while the Android app is built (its §0 items, `android/CLAUDE.md`).

- [ ] `C1` Build for the brothers' computers with `TAGS=devmaster`
  (owner 2026-11-18; the master password stays with the owner).
- [ ] `C2` Remove `ci/vm.sh`; on the laptop remove `/etc/sudoers.d/99-claude-test`.

## 4. After Android

- Windows (L): platform layer; `internal/blocker/` is Linux-only.

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

- PROPOSED 2026-11-20 (session, building `2.4`): SPEC §4.10 and §6
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
- FOR THE OWNER: the rules server secret (`2.7`; SPEC §6 2026-11-14, its
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
