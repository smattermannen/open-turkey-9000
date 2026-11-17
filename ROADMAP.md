# KC9K Roadmap

Code status against `SPEC.md`. The only status file.

Rules: every item cites a § or rule ID (unagreed work goes in §5 first).
Update in the same commit as the code. A wrong status is fixed, not built
on; write "Unchecked" rather than guess. Order: §0 first, top to bottom,
then §2.

Status: Built, Partial, Differs (built unlike SPEC), Not built, Unchecked.
Sizes: S < 1 day, M days, L ≥ week. Last checked: 2026-11-17.

## 0. Fix what exists

**Android is on hold until the Linux project is finished** (owner,
2026-11-16): no Android work; its items below wait.

### 0.1 Code contradicting `SPEC.md`

- [ ] **Owner report 2026-11-17** (real use on the laptop; top priority).
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
  - [ ] `no-images` doesn't hide GIFs (§3.9 Look actions: animated GIFs
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
  - [ ] Looks from page text (greyscale, no-images, no-video) unreliable
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
  - [ ] `no-downloads` doesn't block downloads (§3.9). Checked 2026-11-16
    in LibreWolf: a link on the page, a file from another host (only the
    origin as referrer) and a file the page makes itself (`blob:`) are all
    cancelled (`ci/download_test.py`, four cases). Likely cause in Chrome:
    its service worker is stopped after a while idle and forgot which pages
    forbid downloads (each page reports once). Fixed in 1.21.0: the pages
    are kept in session storage; a download that finishes before it can be
    cancelled has its file removed. Unchecked in Chrome (not run here). S
  - [ ] Desktop brightness slider gone from the menu; blue-light filter
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
  - [ ] Phrase editor: saving a change said the set has no such phrase
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
  - [ ] Locking an enabled block refused password, restart, delay, range
    and word/own-text locks ("already active; … are set when it starts"),
    against R-LOCK-7. Fixed: `strengthenLocks` adds every kind last in the
    order given (`TestLockActiveBlock`). The GUI's lock dialog offers every
    kind on a running block too (merged 2026-11-16); confirm on the laptop.
  - [ ] Phrase editor UI awkward (§4.3). Needs the owner's specifics. M
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
  - [ ] No flashing (R-PAGE-1). Built (extension 1.20.0): while a list or
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
    (a 0.5 s blank on pages while text lists are on). M
  - [ ] GUI overall unclear, hard to read, not pretty or easy to use (owner,
    2026-11-15; R-GUI-3). Redesign page by page. Done: a design layer
    (cards, status pills, page titles with one-line subtitles, ? popovers
    for long explanations, an icon sidebar); Overview rebuilt (status card
    with the longest lock and "Start a block", stat cards, block cards with
    pills); block page header with pills, primary actions, the rest behind
    ⋯; tabs 8 → 6 (Places under Schedule, "Breaks & allowance"); Clusters
    and Frozen Chicken regrouped; long rows wrap, so the window fits 1000
    pixels (was 1213: pages were cut off at the right); lock times in days
    ("69 days 10 h", not "1666 h 40 min"). Left: Settings, Breaks tab,
    Circadian, list cards. L
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
- [ ] **Lock model** (§3.3, §3.4, R-LOCK-1–11). Done: schedule lock
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
  on); the GUI block start and setting lock dialogs ask the unlock order. Not built:
  hardware/internet clock check (see §5). (L)
- [x] **Android extension can be switched off** (§3.13): built (5 s grace
  once checked in, pauses doubling 2 min → 1 h, reset after 10 min unbroken;
  `ExtensionGuard`). Unit tests run in the cloud 2026-11-16:
  `ExtensionGuardTest` (7) and the other 30 pass. On a phone: §0.2. (S)
- [ ] **Page reading slow on big pages** (principle 9). Done: the first
  read of a page goes in ≤ 8 ms pieces in idle time (`readPageText`,
  `READ_SLICE_MS`), then only added visible text, full re-read ≤ every 30 s;
  one-word terms via `kc9kWordIndex`; phrases split the text once per page
  and look words up in an index (100 phrases + 1,000 terms on a 3,000-word
  page: 115 ms → 4 ms, timed in `ci/match_test.js`). Left: judging a huge
  page (40,000 words) is still one piece (~37 ms). (M)
- [ ] **Gigantic URL lists** (principle 9). Extension: done (`kc9kIndex`,
  timed at 100,000 in `ci/match_test.js`). Firewall: done (nft sets,
  iptables fallback); the CLI no longer fills it (`deferFirewall`), the
  daemon does within a cycle with its 30 min lookup cache, so `start`
  doesn't wait for DNS. `/etc/hosts`: measured 15 ms per name lookup at
  100,000 sites, so capped at `MaxHostsDomains` (20,000, ~3 ms; see §5);
  drift check one pass (was quadratic). 100,000-site test:
  `blocker/biglist_test.go`. Left: browser policy with 100,000 entries:
  Unchecked. (M)
- [ ] **R-LOCK-12 gaps**: manual `kc9k monk` now confirmed (`--yes`), and
  `project import` refuses monk steps/actions without `--no-confirm-ok`
  (`projectMonk`); both in `confirm_test.go`. `block import` the same. The
  hotkey press dialog (`blocker.ConfirmDesktop`) ran live on the laptop
  (2026-11-15): zenity shows "Lock 'B'?" with target, kind and length;
  unanswered (screen locked) it times out after 60 s and nothing is locked;
  synthetic keys from other programs are ignored. Left: a person clicks
  Lock and Cancel on an unlocked screen. (S)

### 0.2 Not yet confirmed in real use

Script-tested, never by a person. Script what can be into `ci/`.

- [ ] Notification before an allowance session ends. (Not tried overnight:
  a test allowance could only be removed at midnight, leaving its block on;
  since 2026-11-16 it is removed at once, so this can be scripted.)
- [ ] Scheduled Frozen Chicken in logout/shutdown modes.
- [ ] Strict mode holes: `unshare -Urm` (`kernel.apparmor_restrict_unprivileged_userns`), `--user` Flatpaks killed, Electron not taken for browsers, other accounts and guest blocked.
- [ ] Shutdown mode and `kc9k.nofreeze`; measure fast relock.
- [ ] Rendered terms on real sites for days; hotkey on a real keyboard; hotkey under GNOME.
- [ ] Websites tab on a real screen. (Address change without load: a
  `history.pushState` page is caught by a URL list in LibreWolf,
  `extras_test.py`; on YouTube itself still Unchecked.)
- [ ] One-off logout/shutdown from a list; Dashboard monk button. (Checked
  2026-11-15: a timer-locked global ladder refuses `bypass off` and a weaker
  ladder, and turns off once the lock ends.)
- [ ] Use counting with a person: focused tab/app count, idle not; other browser closed during a website session.
- [ ] Mute and dark on real sites; polite close with a real app.
- [ ] No-video from page text on a real embedded player (YouTube/Vimeo iframe), in LibreWolf and Chrome (`extras_test.py` checks a local one in LibreWolf).
- [ ] Social presets beyond Shorts (`social.js` selectors written from memory); playlist videos playing; block following places; text break returning.
- [ ] Chrome on the laptop. Checked 2026-11-15: the extension checks in from
  Chrome and Chromium, both show the block page; the policy force-installs
  and pins it, turns off incognito, guest and developer mode. Left (needs
  a person): the popup, and that the extension really can't be removed in
  the Extensions page.
- [ ] Block unsupported browsers (`kc9k tamper unsupported-browsers`).
  Checked 2026-11-15: Firefox is closed within ~2 s with the reason logged;
  LibreWolf with the extension stays (browser_test); `kc9k browsers install
  chromium` installs Mint's chromium (`/usr/lib/chromium/chromium`, policy
  in `/etc/chromium/policies`) and works. Left: upgrade from
  `force-librewolf` keeping a lock; a portable LibreWolf.
- [ ] Monk mode live (`ci/monk_live_test.sh` unwritten), incl. reboot mid-way.
- [ ] GUI design pass: pages since 2026-11-10 never seen on a real screen.
- [ ] Lists without types (§3.2): checked 2026-11-15: `H` and `test` have
  one `Blocked` each; `H` blocks reddit.com by address and a page by its
  text in LibreWolf. Left: the New list dialog on a real screen.
- [ ] Active vs enabled on the laptop: checked 2026-11-15 with a throwaway
  block: off it stays out of /etc/hosts in its window; on, it is in /etc/hosts
  in its window, and outside it shows "lifted by its schedule" and leaves
  /etc/hosts. Left: the GUI's "lifted by its schedule now" on a real screen.

## 1. `SPEC.md` coverage

Only items not simply Built.

| Item | Status | Notes |
|---|---|---|
| §3.1 File URL entries | Partial | built: with tamper `file-urls` on, blacklist `file:///…` entries go into Firefox/LibreWolf and Chrome policies unless the block's file exceptions spare them (checked live in LibreWolf); Chrome live: Unchecked; the extension doesn't judge file pages |
| §3.2 Template | Built | your own saved definitions: `template save B T`, `template`, `template remove T`, `block create X --template T`; GUI "Save as template…" and a choice in New block; projects. None shipped |
| §3.2 Affected users | Partial | `block users B all|USER…` (GUI "Users:" button, gui-state `users`/`accounts`, projects): a block for some accounts stays out of hosts/firewall/policies (shared by every account) unless it covers them all; its lists reach only those users' extensions (`BuildRulesFor`, user from the socket uid) and its apps are closed only in their processes (`KillBlockedFor`). Fewer users weakens. Not per user yet: window titles, folders, fetch tools, the extension guard. Live: one account on the laptop, so only "every account" checked |
| §3.3 Continuous/scheduled types | Partial | type follows the block's windows/place rules (no explicit type field); schedules and allowance sessions lift, never switch |
| §3.4 Range lock | Partial | blocks (`start --range "mon-fri 09:00-17:00" [--range-invert]`) and setting locks (browser settings, tamper, strict, bypass, freeze schedules: `--range`); a layer that holds while in range and can't be unlocked. GUI: block start dialog and the setting lock dialogs |
| §3.4 Layered sequential locks | Partial | blocks and setting locks built (CLI flag order); GUI start and setting lock dialogs ask the order (Up/Down); one lock per kind |
| §3.6 Break per block window | Built | `schedule break B N 10m|off`: one break each time window N is open (`break start` takes it, then reward); new/longer weakens; GUI window editor; projects |
| §3.6 Allowance | Built | per day, week or month with a refill time (`allowance set B 5h --per week --refill 04:00`, GUI, projects). and rolling 24 hours (`--per rolling`, hourly buckets), and per window (`--per window`: each opening of a block window is a period, `enforce.QuotaPeriod`; outside windows nothing refills; an overnight span is two windows, so two periods) |
| §3.6 Reward | Built | `break reward B 10m --per 1h --keep one|window|day|week|month|<time ≥ 1h>` (default one; a time keeps each part that long after it is earned, no upper limit, 2026-11-16; the 1-day cap on what is saved is gone, so is the 4 h cap on one break taken from it), GUI choice with a duration field, projects; longer keep, or period ↔ time, weakens. Ending a break early used to give back at most one break's worth also for an accumulating reward: fixed |
| §3.6 Delay break | Built | `break delay B LENGTH WAIT`, `break request B`, GUI row, projects |
| §3.6 Session limit | Built | `allowance set B 1h --sessions N`: N openings per period; a limited session ends 3 min after its last use; more weakens. GUI, projects |
| §3.6 Autostart | Built | `kc9k autostart B add once DATE HH:MM / at HH:MM [DAYS] / sign-in / next-window`, GUI list, projects; removing weakens. `at` checked live; sign-in/wake: Unchecked |
| §3.7 Strict use counting | Built | `kc9k use-counting strict|normal`, GUI switch; every open tab (`rules.strict_use`) and running apps (`RunningApps`) count; normal weakens. Live: Unchecked |
| §3.8 Scheduled freeze lock kinds beyond lead | Built | `freeze schedule lock N --for/--lock/--password-stdin/--restart [--relock]`, `freeze schedule unlock N`; setting lock `freeze:<id>`; removal refused while on; counts as a lock. GUI: Not built |
| §3.8 Allowed apps only | Partial | freeze mode `apps` (`freeze --mode apps`, schedules too, GUI choice), list `kc9k freeze apps add|remove`; closes every regular-user process but allowed apps, their children and system/desktop helpers (`allowedAppKeep`, unit-tested). GUI list editor on the Frozen Chicken page. Live: never run (no freeze overnight) |
| §5 R-LOCK-12 | Partial | see 0.1 |
| §3.8 Strict mode exempt folders | Built | `kc9k strict exempt add|remove DIR`, GUI list; exec bind mounts over the noexec folders (`noexecPlan`); adding weakens. Not in projects (import never weakens). Live: Unchecked (strict mode not run overnight) |
| §3.8 Forgotten password recovery | Built | `kc9k-bin recover` (root, recovery boot, not with no-escape): settings password and password locks (`--access`: access passwords). Live: Unchecked (needs a recovery boot) |
| §3.8 Factory reset protection | Partial | installer media guard built (`reset-guard on|off`, GUI switch under device protection): while anything is locked an in-memory block (`enforce/resetguard.go`) blocks OS image and USB writer download pages and closes image writers and Disks; checked live (hosts follow the lock). Firmware lock, PAM root guard, guided setup: Not built (2.6) |
| §5 R-LIST-4/5 | Partial | case, accents and common Cyrillic/Greek look-alikes folded (`kc9kFold`); hidden text, frames Unchecked |
| §3.9 Ad trigger | Partial | scope `ads` on blacklists and clusters (`list create --ads`, `list where B L ads on`, GUI "Ads" box): entries are ad network domains or industries (`db.AdIndustries`: adult, crypto, dating, gambling); rules `ads`; the extension watches each tab's third-party requests (`noteAdRequest`) and a page loading a listed network matches with every action (`kc9kAdMatches`, unit-tested). Not in shared layers. Browser test: Not run (the owner was using the laptop; the browser test changes LibreWolf settings and would close their windows) |
| §3.11 Block password | Built | `block password B set|remove`; hides entries, apps, titles in gui-state (`--reveal`), `block info`, exports, `list show/export`, `phrase-set show`; GUI "Show contents…"; backups keep it. A project export still holds the contents |
| §3.12 Word recording, specificity, cut | Built | `record start B L --words` / `--cluster C --words` (text lists): extension reports each page's rendered words once (`kc9kPageWords`, `/words`); `kc9k record` ranks by specificity against the baseline (`db.Specificity`); `record stop --cut N`; `record baseline on|off`. Checked live in LibreWolf. GUI: Clusters page "Record words" with cut % and the most specific words |
| §4.10 SafeSearch | Partial | Google, Bing, DuckDuckGo, YouTube Restricted pinned in hosts (every browser) and `rules.safesearch` to the extension (Brave, Yahoo, SearXNG, engines it can't hold blocked); one setting for all, not one lock each |
| §4.10 Tamper settings | Partial | GUI group with OK/Cancel window; clock, block unsupported browsers (own lock), allow file:// blocking, block task managers while locked (`tamper task-managers`), grace for a missing extension (`tamper grace S`, 10–600 s, longer weakens); gui-state `anything_locked`, settings stuck on show so. No group lock. close private windows and Tor (`tamper private`: private browsing off in every browser policy, Tor Browser/tor closed; checked live for the policies; no Tor here). Not built as settings: guest profiles (Chrome guest mode is always off by policy), extension stop (the guard always does it), installer (dpkg prerm refuses removal while locked; install.sh reinstall not guarded), inactive tabs, embedded content (frames always get the block page) |
| §4.10 Extension install dialog | Built | once per session when gui-state `extension_missing`: Install LibreWolf, or Show browsers |
| §4.10 Device protection: factory reset protection, guided setup | Not built | 2.6 |
| §4.10 Browsers list | Built | GUI Browsers group and `kc9k browsers`: LibreWolf, Chrome, Chromium, installed, per-browser extension check-in, Install (`browsers install`); live check in 0.2 |
| §4.11 `.deb` | Built | `make deb` (`packaging/build-deb.sh`): same paths as install.sh; postinst sets up sudoers, database folder, service; prerm runs prepare-uninstall, so removal is refused while locked (checked live); postrm removes sudoers. Installed on the laptop as 0.9.0~overnight (devmaster) |
| §4.3 Draft Save/Cancel | Partial | `block draft B [save|cancel]`, GUI "Edit as draft" with a Save/Cancel bar: a copy of the lists, phrase sets, apps and window titles that never runs; save applies each difference through its gate, refusing and naming weakening ones on a locked block; needs and keeps the block password; drafts stay out of the block list and projects. Not drafted: schedules, places, allowance and the other settings, changed on the block itself (refused on a draft, `draftRefused`) |
| §4.3 Ctrl+F, multi-select, double-click edit | Built | `EditableList`: Ctrl+F to its search, Ctrl/Shift-click and Ctrl+A in website lists, phrase sets, apps, titles, clusters, exempt folders; Delete/Backspace removes, selection moves on; double-click moves an entry into the box |
| §4.3 Import "From KC9K" | Built | `list copy`; a list's Import… offers From KC9K: tick other blocks' lists and clusters. "All searches containing…" not built |
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
- **2.2 Ad trigger** (M): built (scope `ads`); left: a browser test
  (`ci/extras_test.py`), and growing the shipped industry lists.
- **2.4 SafeSearch** (S): Bing, DuckDuckGo, YouTube Restricted built under
  the one `safesearch` setting. Left: a lock per engine, if wanted.
- **2.5 Layered sequential locks** (M): blocks and setting locks done (stored
  order, `unlock` lifts only the first, status and GUI show the order). Left:
  several locks of one kind (the order is chosen in the GUI now).
- **2.6 Factory reset protection** (M): firmware lock via
  `/sys/class/firmware-attributes/` (password given to a trusted person,
  no USB/network boot, fixed order); installer media guard (block OS image
  sites, close mintstick, balenaEtcher, Ventoy, Popsicle, Startup Disk
  Creator, GNOME Disks restore); PAM root guard in `login`, `su`, `su-l`
  via `ApplyLockdown`/`RemoveLockdown` (test on generated config); guided
  setup checklist for strict/no-escape.
- **2.7 Hardening** (M each): sudo rule arguments audited 2026-11-15: caller
  paths are read as the caller (`readAsCaller`), direct reads only of
  KC9K's own files, writes only into KC9K's folders under hash names;
  `audit_test.go` keeps it so (`env_reset` untouched). Left: rules server
  secret by policy (not rolled out overnight: if the extension can't read
  it, every check-in fails and the guard closes browsers; the policy files
  are user-readable anyway); GTK window for the typing challenge (needs a
  way to answer that keeps R-LOCK-5: a stdin answer would let scripts
  type it).
- **2.8** Everything else Not built/Partial in §1.

## 3. Before release

- [ ] Build without `TAGS=devmaster`.
- [ ] Remove `ci/vm.sh`; on the laptop remove `/etc/sudoers.d/99-claude-test`.
- [ ] Tagged release.

## 4. After publishing

- Windows (L): platform layer; `internal/blocker/` is Linux-only.

## 5. Proposed, not agreed

- DECIDED OVERNIGHT: how to check the clock against hardware and internet
  time (§4.10) → not built yet; trusted time stays CLOCK_BOOTTIME with
  powered-off time not counted, because that is the strictest choice: a
  hardware-clock count would only end timer locks sooner and the hardware
  clock can be set in firmware without a firmware password, and an internet
  time query is a new outside query (principle 6) the owner should agree to.
- DECIDED OVERNIGHT: a huge list in /etc/hosts makes every name lookup on
  the computer slow (15 ms each at 100,000 sites) → the hosts layer holds
  at most 20,000 sites, the rest are blocked by the browser policy, the
  extension and the firewall, because principle 9 says huge lists never
  slow the computer and those layers already cover every site.
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
