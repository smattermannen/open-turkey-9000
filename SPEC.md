# KC9K Specification

What KC9K is meant to be. Status lives in `ROADMAP.md`.

Rules for this file:
1. Changed only with the owner's agreement; log it in §6. Exception: the
   autopilot's resolver (`ci/autopilot.sh`) decides open questions; every
   change it makes ends with *(By Opus)*. It never changes these rules or
   an owner decision; the owner can overrule any of its decisions.
2. No status or work items.
3. Use the glossary's words exactly, everywhere. New names go here first.
4. Where code or docs disagree, this file wins. If it seems wrong, ask the owner
   (unattended sessions: an entry in `ROADMAP.md` §5 for the resolver).
5. Rules (§5) have IDs; tests name the rule they prove.
6. *CT: …* gives Cold Turkey's word.

## Index

Navigation only; the sections below hold the content. Read the sections
for your topic, not the whole file: headings `## N.` and `### 3.N`,
functions `- **4.N`, rules `- **R-…**` (`grep -n 'R-LOCK-6' SPEC.md`),
decisions in §6 by date.

| Topic | Read |
|---|---|
| Purpose, principles (speed: principle 9) | §1, §2 |
| Glossary words to avoid | §3.14 |
| Targets: sites, apps, window titles, files | §3.1, R-TITLE-1, R-DESKTOP-1 |
| Blocks, lists, exceptions, matching | §3.1, §3.2, §4.2, §4.3, R-LIST-1–6 |
| Phrase sets | §3.2, §4.3, R-PHRASE-1–10 |
| Block states: active, enabled, lifted | §3.3 |
| Locks | §3.4, §4.6, R-LOCK-1–12 |
| Weakening, strengthening | §3.5, R-WEAK-1–5 |
| Automation: schedules, places, modes | §3.2 Mode/Schedule, §3.6, §4.2 Modes, §4.5, R-SCHED-1–2, R-MODE-1–5 |
| Breaks, allowance, Pomodoro, autostart, use | §3.6, §3.7, §4.7, R-SCHED-3, R-BREAK-1–2 |
| Device: monk mode, Frozen Chicken, strict mode, no-escape | §3.8, §4.8, §4.10 Device protection, R-MONK-1–3, R-FREEZE-1–3, R-STRICT-1, R-ESCAPE-1 |
| Actions, triggers, custom actions, warnings | §3.9, R-CUSTOM-1–2, R-WARN-1, R-MODE-4 |
| Bypass prevention, events | §3.10, R-WARN-2, R-EVENT-1 |
| Passwords | §3.11, §4.10 Passwords |
| Recording, presets | §3.12, §4.4, R-LIST-2, R-PHRASE-10 |
| The program: CLI, daemon, extension, browsers | §3.13, §4.10 Extension/Browsers/Tamper protection/Browser settings, §4.12, R-LOCK-9 |
| Page behaviour in the browser | R-PAGE-1, R-LIST-4–5, R-PHRASE-4–8 |
| Overview, statistics | §4.1, §4.9 |
| Settings, notifications, circadian rhythm, wallpapers | §4.10, R-MODE-5 |
| Install, uninstall | §4.11, R-LOCK-9 |
| Hotkeys | §4.13 |
| GUI rules | R-GUI-1–3 |
| Decisions, open decisions | §6, §7 |

## 1. Vision

Goal: control over one's digital life; break harmful habits (gaming, doom
scrolling, adult content). Fights software designed to exploit psychology.

- **Very hard to get around.** Strictest setup: only a factory reset gets
  out, and that is made deliberate by options the user chooses.
- **More than block.** Actions: remove images, greyscale, mute, stop video,
  darken, close tab/browser, logout, shutdown, switching a mode on and
  locking it (monk mode, Frozen Chicken), custom actions (open sites,
  files, folders, programs; run commands). Enforce SafeSearch; social media
  settings.
- **Recognises more than addresses.** Address, title, page text; planned:
  the ads a page loads. Apps by name, sandbox ID, script, file fingerprint.
- **Any trigger to any action.** Several lists per block, each with its own
  actions.
- **Stops the loophole hunt.** Bypass prevention: a user-chosen ladder of
  consequences.
- **One button.** Hotkeys run any action: start and lock blocks and modes,
  open sites, programs and folders.
- **Circadian rhythm.** Dark mode, blue light filter, brightness, wallpapers
  by time of day.
- **Shareable projects.** Export/import the whole setup.
- **Free.** Young people can't pay for a blocker.

In short: everything Cold Turkey does, plus user-shaped bypass prevention,
more triggers and actions, several lists per block, layered sequential
locks, project export, monk mode, lockable SafeSearch, circadian rhythm.

After the Android app: Windows support.

## 2. Principles

1. Fix current functionality before adding features (bugs, failing tests,
   undocumented behaviour, code contradicting this file).
2. A locked block can't be weakened; an unlocked block is free. Strengthening
   never needs unlocking. A settings password, when set, is needed to weaken
   anything, and to set locks if the user chose that.
3. **Custom security.** Nothing protects a block unless the user chose it: no
   built-in locks, delays or lead times (R-LOCK-10). Describe KC9K by what the
   chosen settings give. Levels: *basic* (user keeps `sudo`; locks outlast an
   impulse); *strict mode* (while locked the user is an ordinary user: any
   ordinary-user path to a blocked site is a real hole); *boot and hardware*
   (no-escape; firmware password held by someone else; factory reset
   protection).
4. The user is never trapped by a bug: recovery (`kc9k.nofreeze`, recovery
   mode) exists unless no-escape is on.
5. A refused weakening is logged, never counted by bypass prevention.
6. Free forever: no paid tiers, accounts or telemetry. Data stays local;
   outside queries (DNS) are stated plainly.
7. Build only what improves the experience. Behaviour changes are settings,
   not new defaults.
8. Platform: Linux Mint Cinnamon on X11. Extension in LibreWolf and Google
   Chrome/Chromium. English only. Android companion runs the extension in
   IronFox (`android/`); on hold until the Linux project is finished
   (owner, 2026-11-16).
9. Browsing stays fast; page judging runs in the background:
   - no single piece of work in a page over 16 ms;
   - a typical page with 1,000 rendered terms and 100 phrases judged in
     under 50 ms total on the test laptop;
   - address judged before load; page text within 0.5 s of appearing;
   - address check cost independent of list size (10 or 100,000 entries);
   - huge lists never slow the network or computer.
10. One rule-maker: the CLI decides all weakening; GUI and extension only run
    CLI commands or follow the daemon.

## 3. Glossary

### 3.1 Targets

| Term | Meaning | Not |
|---|---|---|
| **Target** | Anything a block acts on: site, URL entry, app, folder, window title entry, rendered term, phrase. | |
| **Site** | Domain with subdomains (`reddit.com`, no `www.`), or wildcard domain (`*.xxx`). | URL entry |
| **URL entry** | Domain plus path or query (`reddit.com/r/funny`). | Site |
| **Wildcard** | Sites/URLs: `*` is any text. Terms/phrases: plain word = whole word; `*` matches inside a word (`porn*`). | Bridge |
| **Whole internet** | `*.*`. `file://`, localhost and LAN stay reachable. | |
| **App** | Program matched by name, path, script, fingerprint (`sha256:…`), Flatpak ID or desktop unit. Its descendants close with it. | Window |
| **Folder entry** | Directory: programs run from it are blocked apps; windows showing it close. | |
| **Window title entry** | Words matched against app window titles. Never browsers (tabs are judged by lists). | Rendered term |
| **Rendered term** | One **term** matched alone in title or text, or as words in the address. | Phrase |
| **Phrase** | Terms from two or more **word sets** occurring together (formerly *word combination*). `[eat, nice] [hello, grandpa]` matches "Let's eat grandpa", not "Let's eat, grandpa!". | Rendered term |
| **Term** | A word or several words (`long cat`). | |
| **Word set** | Terms that may fill one place in a phrase; *necessary* or *optional*. | |
| **Phrase length** | Terms a match needs: from the number of necessary sets, but at least 2 (default), up to all sets. | |
| **Bridge** | `big-` … `-gorilla`: up to 20 words of one sentence may be skipped between them. `-strong-` is a middle link. | Wildcard |

### 3.2 Grouping

| Term | Meaning | Not |
|---|---|---|
| **Block** | Named group of lists, phrase sets, apps, folders, title entries, schedule, breaks, autostart, affected users, locks. | |
| **Mode** | Named group of blocks with **one automation for all of them** (§3.6), or monk mode or Frozen Chicken (§3.8). Shown among the blocks; a block is dragged into a mode and is in at most one. A block in a mode has the mode's automation; its own is kept, unused, until it is taken out. A mode may have **wallpapers** (§4.10) and custom actions run when it becomes enabled; it can be locked like a block (R-MODE-3). | Block |
| **List** | Named entries in one block, with a **kind** and a **scope**. No list types: sites, URL entries and rendered terms can share a list. | Preset |
| **Entry** | A site, URL entry or rendered term in a list. | |
| **Scope** | Boxes *URL*, *title*, *text*. With URL: sites/URL entries match the address (and feed the layers); rendered terms match words in the address. | |
| **Phrase set** | Named phrases in a block, with scope and actions, like a blacklist. Any number per block. | List |
| **Blacklist** | Kind: matches get the list's actions. *CT: Websites.* | |
| **Exception list** | Kind: spares matches from blacklists and phrase sets of its own block only. A YouTube playlist address in it (`youtube.com/playlist?list=…`) lets that playlist's videos through, and only them. No phrase exceptions. *CT: Website Exceptions.* | Allowlist block |
| **Allowlist block** | Blacklist `*.*`; only exceptions reachable. | |
| **Preset** | A saved copy kept outside blocks, of one type: **block preset**, **list preset**, **phrase set preset**, **schedule preset**, **automation preset**, **hotkey preset**. Never enforced, freely editable; using one copies it, and saving one copies too: a later change on either side changes nothing on the other. Kept on the Presets page. | Schedule |
| **Affected users** | Local accounts a block applies to (default all); admin only. | |
| **Social media setting** | Keeps a site but removes a part (YouTube without Shorts, …). | List |
| **Test mode** | Per list/phrase set: report matches instead of acting. | |

### 3.3 Block states

| Term | Meaning | Not |
|---|---|---|
| **Active** | Switched on (`kc9k start`); enforces when its automation and breaks allow. | Enabled |
| **Inactive** | Switched off (`kc9k stop`); never enforces. | Unlocked |
| **Enabled** | Enforcing now. | Active |
| **Disabled** | Not enforcing now. | Inactive |
| **Lifted** | Active but disabled (its automation says off, break, pause, Pomodoro rest, allowance time). | Inactive |
| **Continuous** | Type: no schedule and no place rules: enabled whenever active (breaks, pauses, Pomodoro rest and allowance time still lift it). | Automated |
| **Automated** | Type: has a schedule and/or places, which switch it from its default (enabled or disabled) while active (§3.6 Automation); in a mode, its mode's. | Scheduled |
| **Locked** | At least one lock on, its own or its mode's; can't be weakened. | Active |

Enforcing ends by schedule or switch-off. A lock never ends or starts a block.

### 3.4 Locks

A lock belongs to a block or a mode (R-MODE-3). While **on**, the block can't
be weakened (so not switched off or deleted). **Unlocked** as its kind says, then gone unless
it has re-lock. Never changes what is blocked.

| Term | Meaning | CT |
|---|---|---|
| **Timer lock** | Until a time (`--until`) or for a duration (`--for`). Extendable, no upper limit. | Timer |
| **Text lock** | Unlocked by typing a challenge: random characters, random words or custom text, 1–5000 chars, no paste. | Random/custom text |
| **Range lock** | On between set times on set days (or inverted). | Time range |
| **Delay lock** | Unlock request starts a countdown (1 min–40 days). | Delay |
| **Schedule lock** | On during windows, at places, and for a user-chosen lead time before a window; with allow windows: on outside them. | Schedule |
| **Restart lock** | Unlocked by a restart. Any block. | Restart |
| **Password lock** | Unlocked by a password (e.g. a partner's). | Password |
| **Re-lock** | Lock option: comes on again a set time after unlocking. | |
| **Layered sequential locks** | Several locks on a block or setting, unlocked one by one in the user's order. | |
| **Trusted time** | Real time, protected by the tamper settings the user enabled. All locks use it. | |

While any block or setting is locked: uninstall refused, extension required,
tamper protection settings can't be turned off.

### 3.5 Weakening

- **Strengthening**: a change that clearly blocks or protects more.
- **Weakening**: every other change (removing/narrowing entries, adding
  exceptions, removing actions, raising thresholds, shrinking windows,
  widening allow windows, looser break, test mode on, social media setting off,
  changing a redirect target, pausing, switching off, deleting, uninstalling,
  removing/shortening a lock).
- **No weakening during active locks**: global setting; while anything is
  locked no setting (strict mode included) can be weakened. Unlocked blocks
  stay editable.

### 3.6 Time and breaks

| Term | Meaning |
|---|---|
| **Block window** | Weekly time in the schedule of a block **disabled by default**: it enables the block. 5-minute steps; a block's windows never overlap. |
| **Allow window** | Weekly time in the schedule of a block **enabled by default**: it lifts the block. |
| **Break** | Lifts an enabled block for a while. One per continuous block, or one per block window. |
| **Pomodoro** | Work (enabled) and rest (lifted) alternate, work first; optional long rest every N rounds. |
| **Allowance** | N minutes of use per period (rolling 24 h, window, day, week, month; refill time), then enabled. |
| **Reward** | Rest time earned by working (blocked, computer in use); saved up to one break, or accumulates until the window/day/week/month ends, or each part is kept for a time the user picks (no upper limit). |
| **Text break** | Preset break after typing a challenge; block stays locked. |
| **Delay break** | Preset break after a countdown; block stays locked. |
| **Session limit** | N openings per period; a session ends 3 min after switching away. |
| **Pause** | One-off lift until a set time. |
| **Autostart** | Switches a block on: once, time of day on set days, sign-in/wake, or next block window. |
| **Schedule** | Named weekly windows kept outside blocks (*Evening*, *Weekend*), made on the Schedules page. A block or mode uses any number of schedules besides windows of its own (*Apply schedule*); changing a schedule changes everything using it. Overlapping windows count once. |
| **Place** | A set of Wi-Fi access points (no GPS), made on the Places page or from a block. The computer is there when as many of them as the user chooses (default 2) are in range. |
| **Place rule** | A place in a block's automation; a block may have **any number**. Arriving there switches the block as its default says (enabled by default: off; disabled by default: on). A place rule has **no actions of its own**. |
| **Automation** | What switches an active block (or a mode's blocks, or monk mode, or Frozen Chicken) by itself: its windows (its own and its schedules') and its places. The user sets the block **Enabled by default** (a window or a place turns it off) or **Disabled by default** (a window or a place turns it on). With both a schedule and places, the user sets whether switching needs **both** (in a window and at a place at once) or **either** (one is enough). Without a schedule or places the block is continuous. A block in a mode keeps its own automation, unused, until it is taken out. |

### 3.7 Use

- **Use**: blocked item in foreground (focused app/tab) with input in the
  last 3 min, or playing sound (background tabs too).
- **Strict use counting**: setting; background tabs and minimised apps count.

### 3.8 Device-level

| Term | Meaning | Not |
|---|---|---|
| **Frozen Chicken** | A mode (§3.2), not a block: freezes the device in a freeze mode. Switched on only with an end time at most a week ahead, or by its automation (windows only: scheduled freezes, each with its freeze mode); nothing else editable but its allowed apps and locks. A running freeze can't end early. *CT: Frozen Turkey.* | Schedule lock |
| **Freeze mode** | *lock screen*, *logout*, *shutdown*, *allowed apps only*. | |
| **Allowed apps only** | Only programs on the freeze's app list run. | Allowlist block |
| **Monk mode** | A mode, not a block; preconfigured, nothing editable but its automation and locks. Blocks `*.*` (exception `file://*` only) and cuts the whole computer's network while on. Switched on and off like any mode: by hand, by its automation (windows, schedules, places; disabled by default), or by a Mode action, which locks it as the action says. | Block |
| **Consequence** | Device action: close tab, close browser, lock screen, logout, shutdown, Mode action. Announced 60 s ahead except from bypass prevention. | Page action |
| **Strict mode** | Off by default. On and something locked: no `sudo`/`pkexec`; nothing runs from `/home` or `/tmp` except exempted folders (adding one weakens); no containers/VMs. Own setting lock. | |
| **No-escape** | Closes boot exits: hides recovery entries and kernel options, GRUB password nobody knows. | Strict mode |
| **Factory reset** | Wiping/reinstalling/booting another OS. Software can't forbid it. | |
| **Factory reset protection** | Makes it slow and deliberate: firmware lock, installer media guard. Without a firmware password the boot menu stays open. | |

### 3.9 Actions

| Term | Meaning |
|---|---|
| **Action** | What a blacklist or phrase set does on a match, and what a block's apps and its window titles do when they fire (one set each): page action, consequence or custom action. Apps and window titles close by default; each may untick that and take consequences instead or too (fired when an app or window is found, at most once a minute). Any of them may have no action: it then does nothing (an action can be unticked before another is ticked). A place rule has no actions (§3.6). |
| **Mode action** | Makes a chosen mode run now for a set time, locked for that time, whatever its automation says (R-MODE-4); then it is as it was. Frozen Chicken: also the freeze mode. |
| **Custom action** | Opens sites, files, folders or programs, or runs a command, as the signed-in user. Offered wherever actions are (lists, phrase sets, apps, window titles, ladder steps), for when a mode becomes enabled, and in hotkeys (R-CUSTOM-1/2). |
| **Hotkey action** | What a hotkey does, in the order set: switch a block on (with locks) or off, switch a mode on or off, Mode actions (monk mode, Frozen Chicken and modes of blocks, locked), every consequence (close the browsers, lock the screen, log out, shut down) and custom actions. |
| **Block page** | Replaces the page, with a motivational message. |
| **Close tab / Redirect / Image** | Alternatives to the block page. |
| **Look actions** | Greyscale, no images, no video, mute, dark. Change, don't block; combine. No images covers every image, animated GIFs included. |
| **No downloads** | Cancels downloads from matching pages. |
| **Strongest action** | Across all blocks: consequence > close tab > image > redirect > block page. Look actions only if none applies. |
| **Threshold** | Per action. List: different rendered terms in page text. Phrase set: different phrases (default 1). |
| **Hit count** | Per action, lists only (text): total matches incl. repeats. |
| **Rate** | Per action, lists only (text): matches ÷ words, %; pages of ≥ 200 words. |
| **Ad trigger** | Planned fourth scope: ad networks a page loads, by industry. |

### 3.10 Bypass prevention

| Term | Meaning |
|---|---|
| **Bypass prevention** | Counting stopped events and climbing a ladder. Command `kc9k bypass`. |
| **Event** | Something stopped or noticed; kept a day. |
| **Detector** | Event type a ladder counts: `page`, `search`, `app`, `title`, `browser`, `tamper`, `access` (wrong access password), `workaround`. |
| **Ladder** | N events within T trigger consequences, step by step. Global or per block. |
| **Step** | One rung: "N events within T: these consequences". |
| **Quiet time** | Without tripping this long, the ladder drops a step. |
| **Workaround** | Mirror, proxy address, alternative front-end, fetch tool. |
| **Tamper** | Clock jump, or daemon restart other than upgrade, shutdown or self-reported crash. |

### 3.11 Passwords

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Optional; needed to weaken anything; optionally to set locks. | Password lock |
| **Access password** | Optional; needed to view areas: app, dashboard, blocks, device, stats, settings. | Password lock |
| **Block password** | Optional per block; hides its contents until typed. Viewing only; locks unaffected. | Password lock |
| **Setting lock** | A lock on a setting or browser setting. | Block lock |

A forgotten settings password or password lock is removable from recovery
mode unless no-escape is on.

### 3.12 Recording

- **Link recording**: collects visited sites into a list or list preset.
- **Word recording**: collects rendered visible words of visited pages, with counts.
- **Specificity**: how much more common a word is on recorded pages than in normal browsing; ranks recorded words.
- **Cut**: drop the lowest X % before saving.

### 3.13 The program

| Term | Meaning |
|---|---|
| **Daemon** | systemd service re-applying everything every few seconds. |
| **Layer** | `/etc/hosts`, firewall, browser policy, closing apps. The extension works on top. |
| **Extension** | Bundled browser extension. Required while anything is locked or uses websites; browsers without it are closed. |
| **Supported browser** | LibreWolf, Chrome or Chromium with the extension. KC9K shows which are installed, whether each has it, and installs it in one click. |
| **Setting** | Configured outside blocks: **browser setting** (lockable: `safesearch`, `force-search`, `web-only`, `block-devtools`, `no-private`), **tamper protection setting** (§4.10), **behaviour setting** (never locked, exempt from no-weakening: `polite-close`, `block-page-details`), **preference** (interface only). |
| **Hotkey** | Desktop key that runs its hotkey actions (§3.9): starts a block with a lock, opens a set of sites, programs and folders, … |
| **Project** | Whole setup in one file, presets included. *Backup*: everything. *Share*: without places and user names. Import previews, only adds, renames clashing blocks. |
| **Circadian rhythm** | Time-of-day settings: brightness and colour temperature (each an **automation clip**: 24-hour curve of points), wallpaper, dark mode in apps and pages. Never locked, never overrides the user. |

### 3.14 Words to avoid

| Don't say | Say |
|---|---|
| char lock, character lock, typing challenge (as lock) | text lock |
| time lock, time range (for `--until`) | timer lock |
| held, in force | locked, on |
| edit freeze | schedule lock |
| relock (as a break) | text break (**re-lock** is a lock option) |
| quota | allowance |
| allowance (earned time) | reward |
| session break | session limit |
| option, browser option | behaviour setting, browser setting |
| nepsis, bypass detection | bypass prevention |
| rule (for block/list) | block, list, entry ("rule" only for rule IDs, place rules, the `rules` feed) |
| blocklist | blacklist |
| word combination, page word, keyword | phrase, rendered term |
| whitelist | exception list |
| profile | block, block preset |
| unblock | switch off, unlock, pause, weaken |
| loosen | weaken |
| kids, children | youth, young people |
| friction (for KC9K as a whole) | custom security |
| cluster | list preset, phrase set preset |
| template | block preset |
| social preset, social feed preset | social media setting |

## 4. Functions

GUI and CLI offer the same functions.

- **4.1 Overview**: active blocks (enabled/lifted, locks), today's remaining
  windows, break status, ladder positions.
- **4.2 Blocks**: create (empty, block preset, allowlist) · on/off · continuous
  or automated · rename · duplicate (copy inactive) · delete (refused while
  locked) · affected users · export/import.
  - Modes (owner, 2026-11-17): on the same page as the blocks, each on a
    card like a block's: name (opens the mode: its blocks as cards, and
    its automation), on/off switch (switches its blocks on or off; monk
    mode: lets its automation run it), Continuous/Automated, its blocks.
    Drag a block onto a mode to put it in; take it out in the mode. Monk
    mode and Frozen Chicken are always there; other modes are created,
    renamed and deleted (its blocks stay). Frozen Chicken's switch asks for
    an end time (at most a week ahead). A mode's view: its blocks as cards,
    *Add blocks…* with *Create new block…*, its automation, its wallpapers,
    custom actions for when it becomes enabled, its locks.
  - Names (owner, 2026-11-17): blocks, modes and schedules each have their
    own names; a block and a mode may share one.
  - The list (owner, 2026-11-16; modern and sleek, R-GUI-3): each block on
    one card with its **name** (opens the block), its **on/off switch**,
    **"Continuous block" or "Automated block"** (opens the automation view:
    schedule and places, §4.5), its **breaks** ("No breaks" or what it
    allows; opens them) and **locked/unlocked** (opens the locks); nothing
    else. Deleting an opened block comes back to this list.
- **4.3 Block editor**:
  - Add: type, Enter; focus stays. Remove: button or Delete/Backspace;
    selection moves to next. Multi-select: Ctrl/Shift-click, Select all,
    Ctrl+A. Edit: double-click moves entry to text box (remove + add).
    Find: Ctrl+F.
  - The block's menu: Start, Stop, Unlock, **Automation** (§4.5; greyed
    out in a mode, saying "Automated by *mode*"). These apply at once.
  - Save edits/Cancel: an opened block is always a draft; nothing changed in
    it applies until *Save edits*. Locked block: Save edits refuses
    weakening changes and names them.
  - Import *From KC9K* (other blocks' lists/phrase sets, presets; several)
    or *From file* (`.txt`, one entry per line); also *Whole internet* and
    *All searches containing…*. Export list/phrase set to `.txt`.
  - Import: one button; KC9K reads the file's layout itself. Phrases
    (`[a, b] [c]`, one per line) go into a phrase set; rendered terms (one
    per line, or a list in KC9K) become **one** word set: `[term 1, term
    2, term 3]`; sites, also from hosts files (`0.0.0.0 example.com`) and
    adblock rules (`||example.com^`), go into a list. Phrase sets export
    in phrase format, one per line.
  - Lists: create (kind; scope boxes), rename, delete, scope, actions,
    threshold, hit count, rate, test mode. Block password.
  - Phrase sets: create (unnamed gets a name), rename, delete, scope,
    actions, threshold, test mode. Phrase editor inside the set's card (not
    a pop-up): any number of word sets, each necessary/optional, phrase
    length; double-click a phrase to load it. Saving a loaded phrase changes
    it (never adds a copy).
  - Apps: program, folder, title entry; their actions. The block's own
    ladder.
  - Social media settings: a section after the blacklists, above the
    exceptions, opened and closed. YouTube playlists let through: a
    playlist address in an exception list.
  - *Save as preset*: on a block (block preset), on its lists (asks
    *Blacklists*, *Exceptions* or *Both*, and *Include actions?*), on a
    phrase set, on its automation.
- **4.4 Presets and recording**: the Presets page shows presets by type
  (blocks, lists, phrase sets, schedules, automations, hotkeys), tidily
  grouped: create, edit, rename, delete, use (copied into a block, mode or
  hotkey), save from one. Link recording into a list or list preset; word
  recording, ranked, cut before saving.
- **4.5 Automation** (schedules and places, §3.6): weekly grid; drag to
  add/move, click to edit; 15-min grid, 5-min times; copy to days; similar
  windows edited together; other blocks greyed; break per window; allow
  windows. Enabled by default / Disabled by default; places, any number
  per block, each added place showing plainly that it is added, whether
  arriving there switches the block on or off, and whether the computer
  is there now; with a schedule and places, both or either. The same for
  a mode (one automation for its blocks) and monk mode; Frozen Chicken
  has windows only.
  - Schedules and Places each have a page in the sidebar (owner,
    2026-11-17): made and changed there without a block; each shows what
    uses it. A block's or mode's automation has **Apply schedule…**
    (*Evening*, *Weekend*) and its places to tick.
- **4.6 Locks**: all of §3.4, alone or layered, on blocks, browser settings
  and settings (global ladder too). Layer order fixed while locked; new lock
  goes last. Timer locks always extendable; set for a duration or until a
  date picked on a calendar (R-GUI-2).
  - Several locks of one kind may be layered, each its own layer (two
    partners' password locks; a timer lock, a text lock, then another timer
    lock). A further lock of a kind already on is added, last, only when the
    user asks for a new lock; otherwise a lock given for that kind
    strengthens one already on (a later timer, a longer text or delay lock).
    *(By Opus)*
  - The text lock's challenge is typed in the terminal the command runs in
    or, without one, in a window the command itself opens on the user's
    desktop (§6 2026-11-14); only keys typed there count. A paste, a drop or
    an answer piped to the command is refused (R-LOCK-5); the GUI only runs
    the command. *(By Opus)*
- **4.7 Breaks and autostart**: all of §3.6. Break status also in the
  extension's toolbar button and popup.
- **4.8 Frozen Chicken**: a mode among the blocks (no page of its own):
  switched on for a time or until a date, at most a week ahead; its
  automation schedules freezes; both may coexist.
- **4.9 Statistics**: time used and times blocked per site, app, title. Time
  frame (presets/custom), bar chart over time with one colour per site/app, per user, top 5/10, search, filter by block, JSON
  export, delete, on/off, optionally skip private windows. Separate from
  events; local only.
- **4.10 Settings**:
  - Preferences: 24-hour clock, first weekday, theme with darkness slider
    (dark grey to black), open at login, tray.
  - Extension: install dialog when missing.
  - Notifications: timer lock end, window start/end, Pomodoro change,
    allowance out, app blocked; advance warnings in minutes (`2,5`).
  - No weakening during active locks.
  - **Tamper protection** (one group; once on, can't be turned off while
    anything is locked, and its own lock must be unlocked; turning one on
    shows an OK/Cancel window):
    - close private windows and Tor; close guest profiles; close browsers
      if the extension stops; grace for missing extension (default 60 s);
      allow `file://` blocking;
    - block unsupported browsers (all but supported browsers with the
      extension);
    - block the installer (no reinstall/downgrade/removal while locked, via
      installer or package manager);
    - block inactive tabs (background tabs blocked at once);
    - block embedded content (blocked site inside another page);
    - block task managers while locked (System Monitor, htop);
    - clock: block time/date changes; check against hardware clock and
      internet time.
  - Browsers: supported browsers found, extension status, one-click install.
  - **Browser settings**, each lockable: SafeSearch on Google, Bing,
    DuckDuckGo, YouTube (Restricted Mode); *force-search* (Google only,
    others blocked, browser search box uses Google); *web-only* (Google web
    results only: no images, shopping, AI answers); block developer tools;
    no private windows.
  - DNS: server asked for blocked sites' real addresses (firewall layer
    only). Default: the network's own server; Quad9 if it can't answer.
  - Use counting: strict use counting.
  - **Circadian rhythm**:
    - automation clips (one set for all days) for backlight %, software
      brightness %, colour temperature K; add/move/remove points, bend the
      curve between points; presets;
    - wallpapers: picture plus times, on a timeline like a video editor's
      (drag a bar to move it, an end to change it); no fixed periods;
      *Override modes*: the same setting as a mode's *Override circadian
      rhythm wallpapers*, the other way round (one on is the other off);
    - *Play the day*: the next 24 hours shown on the screen in about 15
      seconds, then everything is back to now;
    - dark mode = system dark mode: system default, always light, always
      dark, or light between two times;
    - dark pages follow dark mode, with exempt sites; the extension's
      toolbar button exempts a site (or shows it dark again);
    - not lockable;
    - never overrides the user: never puts a level back; a manual change
      pauses that clip until its next point or resume; a hand-picked
      wallpaper/dark mode stays until the next scheduled change;
    - leaves the desktop's own brightness control and night light working;
      off, it touches nothing.
  - Device protection: strict mode, no-escape, factory reset protection,
    guided setup.
  - **Wallpaper default** (not part of circadian rhythm): what the desktop
    shows when no mode's wallpaper and no circadian rhythm wallpaper
    applies, and while the place isn't known yet: *Latest wallpaper*,
    *Circadian rhythm* or *Custom* (a picture chosen here).
  - Passwords: settings, access. Global ladder, lockable. Project:
    backup, share, import. Behaviour settings.
- **4.11 Install**: `.deb`. No automatic updates for now. Uninstall refused
  while anything is locked.
- **4.12 CLI**: every function has a command. Cold Turkey equivalents:

| Cold Turkey | KC9K |
|---|---|
| `-add "B" -web "URL"` | `kc9k block add B URL` |
| `-add "B" -exception "URL"` | `kc9k block add-exception B URL` |
| `-start "B"` | `kc9k start B` |
| `-start "B" -lock X` | `kc9k start B --for Xm` |
| `-start "B" -password X` | `kc9k start B --password-stdin` |
| `-start "B" -random-text X` | `kc9k start B --text-lock --lock-chars X` |
| `-stop "B"` | `kc9k stop B` |
| `-stop "B" -password X` | `kc9k stop B` (password on stdin) |
| `-toggle "B"` | `kc9k toggle B` |

Command/flag names follow the glossary; old names stay as aliases for a while.

- **4.13 Hotkeys** (owner, 2026-11-17): their own page in the sidebar. Each
  hotkey: its keys, its hotkey actions in order (§3.9), *Run without
  confirmation* (§6 2026-11-14); add, change, remove, *Save as preset*.

## 5. Rules

**Locks**
- **R-LOCK-1** Only locks protect a block. Locked: can't be weakened. Unlocked: on, off or edited freely at once, even while enabled.
- **R-LOCK-2** A lock stays on until unlocked as its kind says; then gone unless re-lock.
- **R-LOCK-3** A locked block can't be deleted or switched off.
- **R-LOCK-4** Timer locks: extend only, never shorten.
- **R-LOCK-5** Text lock challenge refuses pasting.
- **R-LOCK-6** Layered locks unlock one at a time in the user's order, each only after the previous. A dated timer lock needs its date; a duration counts from the previous lock's unlocking.
- **R-LOCK-7** Layer order can't change while locked; new locks go last.
- **R-LOCK-8** A restart lock is unlocked by a boot later than its start.
- **R-LOCK-9** While anything is locked: uninstall refused, extension required, tamper protection can't be turned off.
- **R-LOCK-10** No lock, delay or lead time unless the user set it.
- **R-LOCK-11** A lock never starts itself, except via re-lock.
- **R-LOCK-12** Every lock of every kind on every target (block, mode, browser setting, setting, scheduled freeze, monk mode) asks for confirmation when set, showing target, kind and length. Nothing is locked before confirmation. GUI, CLI, hotkeys and presets alike. A lock an action sets later (Mode action, ladder step) is confirmed when the action is set up, with "I know this will not ask for confirmation".

**Weakening**
- **R-WEAK-1** Strengthening never needs unlocking. Setting a lock needs the settings password only if the user chose that.
- **R-WEAK-2** All weakening decisions are made in the CLI.
- **R-WEAK-3** A refused weakening is logged, never counted as a bypass.
- **R-WEAK-4** With no-weakening on, no setting (strict mode included) can be weakened while anything is locked; turning it off is refused then.
- **R-WEAK-5** Editing an entry is remove + add: weakening unless the new entry clearly matches more.

**Schedules and breaks**
- **R-SCHED-1** An active automated block is in its default state (enabled or disabled) unless its automation switches it (§3.6: a window, a place; both or either); an inactive block is never enabled. Automation never switches a block on or off (active/inactive), and has no actions of its own.
- **R-SCHED-2** A block's windows never overlap.
- **R-SCHED-3** Pomodoro starts with work; its rhythm restarts each time the block is enabled.
- **R-BREAK-1** Allowance counts only use.
- **R-BREAK-2** Reward is earned only while the computer is in use.

**Lists and phrases**
- **R-LIST-1** Exceptions spare pages only from their own block's blacklists and phrase sets.
- **R-LIST-2** Presets are never enforced.
- **R-LIST-3** Every new block has the exception `file://*`.
- **R-LIST-4** Text = rendered visible text; hidden/collapsed text doesn't count, text further down does. Each frame is judged on its own text.
- **R-LIST-5** Case and accents ignored (é = e); look-alike letters from other alphabets count as Latin. Languages without word spaces are out of scope.
- **R-LIST-6** Every count set on an action (threshold, hit count, rate) must be met.
- **R-PHRASE-1** A phrase has ≥ 2 word sets; each set is freely necessary or optional (none necessary is allowed). Phrase length ≥ 2 and ≥ the number of necessary sets (owner, 2026-11-15).
- **R-PHRASE-2** A match takes one term from each necessary set, plus optional ones up to the phrase length.
- **R-PHRASE-3** One word set gives at most one term: `[long cat, neko] [running, sitting]` matches "long cat running", not "long cat neko".
- **R-PHRASE-4** Terms in any order, directly adjacent, separated only by blank space. Punctuation breaks the match. An in-word apostrophe is part of the word ("let's"); a hyphen between letters is blank space (`long cat` matches "long-cat").
- **R-PHRASE-5** A break between page parts (heading, paragraph, list item, button) ends a sentence; phrases and bridges don't cross it.
- **R-PHRASE-6** URL: decoded first (`%20` = space); separators `-` `_` `/` `.` `?` `=` `&` `+` `#` `:` `,` are blank space.
- **R-PHRASE-7** A bridge skips 0–20 words between terms of different sets: `[big-, strong] [-gorilla, cat]` matches "big and long gorilla" and "big gorilla". Commas allowed, no `.` `!` `?`; order fixed (`big-` before `-gorilla`).
- **R-PHRASE-8** A bridge needs both ends: `-gorilla` only after a `term-` of another set; `[-gorilla, cat] [jumping, river]` doesn't match "jumping gigantic gorilla". One-ended bridges are refused on entry. A middle link needs ≥ 3 word sets.
- **R-PHRASE-9** Each phrase set action has a threshold of different phrases (default 1). No hit count or rate.
- **R-PHRASE-10** Phrase sets can be exported, imported and kept as phrase set presets; no recording into them.
- **R-TITLE-1** Window title entries never apply to browsers.

**GUI**
- **R-GUI-1** Every duration field takes days, hours and minutes (or typed text such as `2d 4h`, `300m`); never minutes only. A value outside the allowed range is never changed silently: the field says why and keeps what was typed.
- **R-GUI-2** Every timer lock is set either for a duration (R-GUI-1) or until a date and time picked on a calendar (e.g. 24 December 2099, 18:00).
- **R-GUI-3** The GUI follows the GNOME Human Interface Guidelines and the usual usability heuristics: tasks first (the overview shows what is on, until when and how locked; common tasks one or two clicks away); one primary action per view; advanced options behind *More options*; plain glossary words, short sentences; lock state shown wherever a locked thing appears; controls that can't be used are disabled with the reason; input checked where it is typed; one widget per concept everywhere; everything reachable by keyboard; the window never freezes while a command runs.

**Browsing**
- **R-PAGE-1** No flashing: a page that a list or phrase set will block, replace or give a look action never shows its content without it, also when moving within a site. Content stays hidden until judged, at most 3 s (then shown and judged as usual).

**Events**
- **R-EVENT-1** At most one event per tab and blocked site per minute. Restored tabs at browser start, preloaded pages and embedded frames don't count.

**Modes and schedules**
- **R-MODE-1** A block in a mode has the mode's automation; it isn't changed on the block, and its own is kept, unused. Putting a block into a mode, or taking it out (its own automation comes back), is refused if that weakens it while it or the mode is locked.
- **R-MODE-2** A change of a mode's automation or of a schedule is refused as a whole if it weakens any locked block using it, naming the blocks; otherwise it applies to all of them at once.
- **R-MODE-3** A mode of blocks is locked by locking all its blocks at once, with any lock kind (R-LOCK-1–12); monk mode and Frozen Chicken by running for a set time. Locked, a mode can't be switched off or weakened (its automation, a block taken out).
- **R-MODE-4** A Mode action makes its mode run now for the action's time, whatever its automation says, locked for that time (only strengthening: a running lock is never shortened); then the mode is as it was before.
- **R-MODE-5** A mode's wallpapers show while it is enabled, over circadian rhythm's unless circadian rhythm overrides modes (one setting, shown in both places); of several modes, the one enabled last wins. While the place isn't known (no known network seen, also at boot before the first look) a mode switched by places shows none and the wallpaper default applies. KC9K looks for places as early as possible after boot, before anyone signs in, and puts the wallpaper in place for the desktop that starts.

**Monk mode**
- **R-MONK-1** Not a block: no lists, no entries, no settings but its automation. Listed among the blocks as a mode. Blocks `*.*` with only `file://*` excepted; no Mode action of its own is offered in it.
- **R-MONK-2** Cuts the whole computer's network three ways at once: Wi-Fi radio off, every Ethernet interface down, firewall drops all but loopback. KC9K's local traffic (daemon ↔ extension) works. All three held for the whole time, undoing user changes. At the end, or after a restart/crash past the end, the previous connection state is restored. While its places decide, the Wi-Fi radio stays on (unused: the firewall drops everything) so the computer can tell where it is.
- **R-MONK-3** Monk mode is switched on and off like any mode: by hand, by its automation (on while its windows and places have it on), or by a Mode action. Only locks keep it on (R-MODE-3). Switching it on by hand asks for confirmation; its locks as R-LOCK-12.

**Device**
- **R-FREEZE-1** *Allowed apps only*: only listed programs (and system processes) run.
- **R-FREEZE-2** A running freeze can't end; removing a locked scheduled freeze is refused.
- **R-FREEZE-3** Frozen Chicken is switched on only with an end time at most a week ahead; extending keeps that limit, counted from now.

**Custom actions**
- **R-CUSTOM-1** A custom action runs as the signed-in user in their desktop session, never as root, and can do nothing the user couldn't: a KC9K command in it meets every lock, password and refusal as if typed. It never weakens a locked block, mode or setting.
- **R-CUSTOM-2** From a match or a fired app or window title, a custom action runs at most once a minute per block and trigger; for a mode, each time it becomes enabled (a computer started while it is enabled: once, when the user signs in).
- **R-WARN-1** Logout, shutdown, and freezes from a list action, a trigger's action (apps, window titles) or schedule are announced 60 s ahead.
- **R-WARN-2** Bypass prevention has no countdown: it notifies at every event and step ("Event triggered, be careful") and acts at once, up to immediate shutdown.
- **R-STRICT-1** Strict mode is on only if the user turned it on. Turning off is refused only by its own lock and R-WEAK-4.
- **R-ESCAPE-1** Until no-escape is on, a recovery path exists.
- **R-DESKTOP-1** On Wayland or non-Cinnamon desktops, warn that window titles and app time don't work.

## 6. Decisions

Decisions not already written into §1–5:

- 2026-11-10: Users work in the GUI; commands exist and ask for access passwords too.
- 2026-11-10: One ladder for all detectors (no ladder per detector).
- 2026-11-13: Where code contradicts this file, the code is fixed.
- 2026-11-14: Monk mode is a mode because a block could be edited or deleted while unlocked.
- 2026-11-14: Each hotkey has "Run without confirmation". Off: pressing it with a lock asks in a desktop dialog. On: its lock is confirmed only at setup. Automatic monk mode (ladder step, `monk` action) asks nothing when it starts; setting it up needs ticking "I know this will not ask for confirmation".
- 2026-11-14: Word combinations became phrases once: each box a word set, `:N` dropped; new phrase sets start at threshold 1.
- 2026-11-14: No page score.
- 2026-11-14: The root login and `su` PAM guard applies only during strict mode's lockdown. No boot order guard, no hiding other systems behind the GRUB password (friction only without a firmware password).
- 2026-11-14: The rules server requires a secret given to the extension by policy. Every sudo-allowed command is checked for hostile arguments (symlinks, `..`, files under `/etc`). The typing challenge gets its own GTK window. No first-run wizard.
- 2026-11-14: No categories: no shipped lists or phrase sets; `block create --template` lists removed.
- 2026-11-14: `force-librewolf` becomes *block unsupported browsers* (tamper protection).
- 2026-11-14: Declutter removed (broke pages).
- 2026-11-14: No search lists: URL-scope terms catch queries (R-PHRASE-6), text scope catches results.
- 2026-11-15: Phrase word sets are freely necessary or optional, none necessary allowed; phrase length (≥ 2) sets how many terms a match needs (R-PHRASE-1).
- 2026-11-15: Lists and phrase sets may have no action.
- 2026-11-15: Durations in days, hours and minutes, never minutes only, never silently changed (R-GUI-1); timer locks also by calendar date (R-GUI-2); the GUI is redesigned to be beautiful, intelligent and intuitive (R-GUI-3); no flashing (R-PAGE-1).
- 2026-11-16: Restart locks on any block, scheduled ones too: the user decides how their locks work.
- 2026-11-16: Reward can also be kept for a time the user picks, with no upper limit (on the time or the amount saved).
- 2026-11-16: The user picks how many of a place's networks must be in range; a place rule can be given to any block.
- 2026-11-16: One Import button: KC9K tells phrases, rendered terms and sites (hosts files, adblock rules) apart by their layout.
- 2026-11-16: YouTube playlists let through belong to the exception list, not the social presets.
- 2026-11-16: Circadian rhythm: its own page only (not in Settings); wallpapers on a draggable timeline; Play the day preview.
- 2026-11-16: Any trigger to any action: a block's apps and window titles each get one action set (close by default, and consequences).
- 2026-11-16 (later): A place rule only enables or lifts its block; it has no actions (withdrawn from the decision above). An automated block is Enabled by default (its windows and places turn it off) or Disabled by default (they turn it on); any number of places per block; with a schedule and places, the user picks whether switching needs both or either. Blocks are continuous or automated; the block list shows name, on/off switch, Continuous/Automated block (opens the automation view), breaks and lock state. Android is on hold until the Linux project is finished.
- 2026-11-17: Modes: a group of blocks automated together (one schedule and places for all), or monk mode, which can be automated by schedule and place. Modes sit with the blocks; blocks are dragged into a mode and shown inside it. Schedules and places have their own pages, made without a block; a block or mode applies a schedule (*Evening*, *Weekend*).

- 2026-11-17 (session, to confirm with the owner): wallpapers while the place isn't known. A mode switched by places shows none, and the wallpaper default shows in place of circadian rhythm's picture too when such a mode would override it (the home/school example); a mode by time alone still shows. The place is unknown at once at boot and after waking, otherwise after a minute without networks. With a picture or circadian rhythm as the default, it also goes on before the computer sleeps. The default's own picture is kept when another default is chosen. Importing a file whose custom actions open or run something asks first.
- 2026-11-17 (session, to confirm with the owner): a mode is locked as its kind allows (R-MODE-3: a mode of blocks through its blocks' locks, monk mode and Frozen Chicken by a time); a Mode action runs its mode now for a time, locked, whatever its automation says (R-MODE-4), as the monk action and ladder step did.
- 2026-11-17 (later): Frozen Chicken is a mode (no page of its own), up to a week. Monk mode is switched on and off like any mode; only locks keep it on, and modes take locks as blocks do. A **Mode action** switches a mode on and locks it as set, from any trigger (replaces the monk action and ladder step). A block keeps its own automation while in a mode and gets it back when taken out. Automation is opened from the block's menu. An opened block is always a draft until *Save edits*. Blocks and modes may share names. Clusters and templates become **presets** of every type, always copies; schedules stay shared. *Social preset* is renamed *social media setting*. YouTube playlists are let through by their address in an exception list (a separate box withdrawn): a plain exception can't do it safely, because YouTube plays any video with any `list=` added. **Custom actions** (open sites, files, folders, programs; commands) wherever actions are and for a mode becoming enabled; free commands, but never past a lock. Hotkeys get a page and run **hotkey actions**. Modes have wallpapers; circadian rhythm and modes override each other by one setting; a **wallpaper default** covers an unknown place.
- 2026-11-18: An Index at the top of this file for navigation; it adds no rules (owner).
- 2026-11-18: The clock check (§4.10 tamper protection) may ask internet time: an outside query the owner agreed to (principle 6).
- 2026-11-18: SafeSearch is lockable per engine: Google, Bing, DuckDuckGo and YouTube are each their own browser setting (§4.10).
- 2026-11-18: When Linux is finished, the owner's brothers test it on their computers while the Android app is built; Windows comes after Android.
- 2026-11-19: The autopilot's resolver may write decisions into this file, each change tagged as rule 1 says; only the owner's decisions are untagged (owner).
- 2026-11-20 (By Opus): Layered sequential locks may hold several locks of one kind, each its own layer (§4.6): §3.4 says several locks, not several kinds, and the user decides how their locks work (2026-11-16). A new one of a kind already on is the user's explicit choice, so a lock given for that kind still strengthens the one on by default (principle 7).
- 2026-11-20 (By Opus): The text lock's challenge window (2026-11-14) is opened by the command itself, which reads only keys typed in it and never an answer piped to it (§4.6): the CLI decides all weakening (principle 10) and can tell typing from pasting only in input it reads itself (R-LOCK-5).

## 7. Open decisions

None.
