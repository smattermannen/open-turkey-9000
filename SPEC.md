# KC9K Specification

What KC9K should be. Status: `ROADMAP.md`.

1. Changed only with the owner's agreement, logged in §6. Untagged = the
   owner's; an agent's change is tagged *(By Opus)*/*(By claude)*, and the
   owner can overrule it.
2. No status or work items.
3. Glossary words exactly, everywhere; new names go here first.
4. Code or docs disagree: this file wins (code is fixed). Seems wrong: ask
   the owner (unattended: `ROADMAP.md` §5 entry for the resolver).
5. Rules (§5) have IDs; tests name the rule they prove.
6. *CT: …* = Cold Turkey's word.

## Index

Navigation only, adds no rules. Headings `## N.`/`### 3.N`, functions
`- **4.N`, rules `- **R-…**` (`grep -n 'R-LOCK-6' SPEC.md`), §6 by date.

| Topic | Read |
|---|---|
| Purpose, principles (speed: principle 9) | §1, §2 |
| Words to avoid | §3.14 |
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
| Page behaviour in the browser | R-PAGE-1–2, R-LIST-4–5, R-PHRASE-4–8 |
| Overview, statistics | §4.1, §4.9 |
| Settings, notifications, circadian rhythm, wallpapers | §4.10, R-MODE-5 |
| Install, uninstall | §4.11, R-LOCK-9 |
| Hotkeys | §4.13 |
| Sync, accountability partner, website, one-click installer | §3.13, §4.11, §4.14–4.16 |
| GUI rules | R-GUI-1–3 |
| Windows | §6 2026-11-21 onward |
| Decisions, open decisions | §6, §7 |
| Android | §8, R-ANDROID-1–5 |

## 1. Vision

Control over one's digital life; break harmful habits (gaming, doom
scrolling, adult content); fight software built to exploit psychology.

- **Very hard to get around.** Strictest setup: only a factory reset gets
  out, made deliberate by options the user chooses.
- **More than block.** Actions: remove images, greyscale, mute, stop video,
  darken, close tab/browser, logout, shutdown, switch a mode on and lock it
  (monk mode, Frozen Chicken), custom actions (open sites, files, folders,
  programs; run commands). Enforce SafeSearch; social media settings.
- **Recognises more than addresses.** Address, title, page text; planned:
  ads a page loads. Apps by name, sandbox ID, script, file fingerprint.
- **Any trigger to any action.** Several lists per block, each with its own
  actions.
- **Stops the loophole hunt.** Bypass prevention: user-chosen ladder of
  consequences.
- **One button.** Hotkeys run any action.
- **Circadian rhythm.** Dark mode, blue light filter, brightness,
  wallpapers by time of day.
- **Shareable projects.** Export/import the whole setup.
- **Free.** Young people can't pay for a blocker.
- **Every device, one setup.** Cross-device sync, one-click installer,
  Chrome, Edge, Firefox and Bing on every platform, accountability partner,
  a really neat website.

In short: everything Cold Turkey does, plus user-shaped bypass prevention,
more triggers and actions, several lists per block, layered sequential
locks, project export, monk mode, lockable SafeSearch, circadian rhythm.

Order: Windows now (2026-11-21), then Android.

## 2. Principles

1. Fix current functionality before adding features (bugs, failing tests,
   undocumented behaviour, code contradicting this file).
2. A locked block can't be weakened; an unlocked block is free.
   Strengthening never needs unlocking. A settings password, when set, is
   needed to weaken anything, and to set locks if the user chose that.
3. **Custom security.** Nothing protects a block unless the user chose it:
   no built-in locks, delays or lead times (R-LOCK-10). Describe KC9K by
   what the chosen settings give. Levels: *basic* (user keeps `sudo`; locks
   outlast an impulse); *strict mode* (while locked the user is an ordinary
   user: any ordinary-user path to a blocked site is a real hole); *boot and
   hardware* (no-escape; firmware password held by someone else; factory
   reset protection).
4. The user is never trapped by a bug: recovery (`kc9k.nofreeze`, recovery
   mode) exists unless no-escape is on.
5. A refused weakening is logged, never counted by bypass prevention.
6. Free forever: no paid tiers or telemetry. Data stays local; outside
   queries (DNS) stated plainly. Only exception: the **KC9K account**,
   needed only for cross-device sync and the accountability partner; what
   those send is kept in KC9K's cloud. No account: nothing leaves the
   device.
7. Build only what improves the experience. Behaviour changes are settings,
   not new defaults.
8. Platform: Linux Mint Cinnamon on X11. Extension in LibreWolf and Google
   Chrome/Chromium; Chrome, Edge, Firefox and Bing to be supported browsers
   on every platform. English only. Android app (`android/`, §8) built
   after the Linux project is finished.
9. Browsing stays fast; page judging runs in the background:
   - no single piece of work in a page over 16 ms;
   - typical page (1,000 rendered terms, 100 phrases) judged in < 50 ms
     total on the test laptop;
   - address judged before load; page text within 0.5 s of appearing;
   - address check cost independent of list size (10 or 100,000 entries);
   - huge lists never slow the network or computer.
10. One rule-maker: the CLI decides all weakening; GUI and extension only
    run CLI commands or follow the daemon.

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
| **Mode** | Named group of blocks with **one automation for all** (§3.6), or monk mode or Frozen Chicken (§3.8). Shown among the blocks; a block is dragged in and is in at most one mode. A block in a mode has the mode's automation; its own is kept, unused, until taken out. May have **wallpapers** (§4.10) and custom actions run when it becomes enabled; lockable (R-MODE-3). | Block |
| **List** | Named entries in one block, with a **kind** and a **scope**. No list types: sites, URL entries and rendered terms can share a list. | Preset |
| **Entry** | A site, URL entry or rendered term in a list. | |
| **Scope** | Boxes *URL*, *title*, *text*. With URL: sites/URL entries match the address (and feed the layers); rendered terms match words in the address. | |
| **Phrase set** | Named phrases in a block, with scope and actions, like a blacklist. Any number per block. | List |
| **Blacklist** | Kind: matches get the list's actions. *CT: Websites.* | |
| **Exception list** | Kind: spares matches from blacklists and phrase sets of its own block only. A YouTube playlist address in it (`youtube.com/playlist?list=…`) lets that playlist's videos through, and only them (a plain exception can't: YouTube plays any video with any `list=` added). No phrase exceptions. *CT: Website Exceptions.* | Allowlist block |
| **Allowlist block** | Blacklist `*.*`; only exceptions reachable. | |
| **Preset** | Saved copy kept outside blocks, of one type: **block**, **list**, **phrase set**, **schedule**, **automation**, **hotkey preset**. Never enforced, freely editable; using or saving one copies (no link either way). Presets page. | Schedule |
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
| **Lifted** | Active but disabled (automation says off, break, pause, Pomodoro rest, allowance time). | Inactive |
| **Continuous** | Type: no schedule or place rules: enabled whenever active (breaks, pauses, Pomodoro rest, allowance time still lift it). | Automated |
| **Automated** | Type: has a schedule and/or places, which switch it from its default (enabled or disabled) while active (§3.6); in a mode, the mode's. | Scheduled |
| **Locked** | At least one lock on, its own or its mode's; can't be weakened. | Active |

Enforcing ends by schedule or switch-off. A lock never ends or starts a block.

### 3.4 Locks

A lock belongs to a block or a mode (R-MODE-3). While **on**, the block
can't be weakened (so not switched off or deleted). **Unlocked** as its
kind says, then gone unless it has re-lock. Never changes what is blocked.

| Term | Meaning | CT |
|---|---|---|
| **Timer lock** | Until a time (`--until`) or for a duration (`--for`). Extendable, no upper limit. | Timer |
| **Text lock** | Unlocked by typing a challenge: random characters, random words or custom text, 1–5000 chars, no paste. | Random/custom text |
| **Range lock** | On between set times on set days (or inverted). | Time range |
| **Delay lock** | Unlock request starts a countdown (1 min–40 days). | Delay |
| **Schedule lock** | On during windows, at places, and a user-chosen lead time before a window; with allow windows: on outside them. | Schedule |
| **Restart lock** | Unlocked by a restart. Any block, scheduled ones too. | Restart |
| **Password lock** | Unlocked by a password (e.g. a partner's). | Password |
| **Re-lock** | Optional on every lock: a set time after unlocking, the block's whole lock configuration (every lock, in order) comes on again. | |
| **Layered sequential locks** | Several locks on a block or setting, unlocked one by one in the user's order. | |
| **Trusted time** | Real time, protected by the tamper settings the user enabled. All locks use it. | |

While any block or setting is locked: uninstall refused, extension
required, tamper protection settings can't be turned off.

### 3.5 Weakening

- **Strengthening**: a change that clearly blocks or protects more.
- **Weakening**: every other change (removing/narrowing entries, adding
  exceptions, removing actions, raising thresholds, shrinking windows,
  widening allow windows, looser break, test mode on, social media setting
  off, changing a redirect target, pausing, switching off, deleting,
  uninstalling, removing/shortening a lock).
- **No weakening during active locks**: global setting; while anything is
  locked no setting (strict mode included) can be weakened. Unlocked
  blocks stay editable.

### 3.6 Time and breaks

| Term | Meaning |
|---|---|
| **Block window** | Weekly time in the schedule of a block **disabled by default**: enables it. 5-minute steps; a block's windows never overlap. |
| **Allow window** | Weekly time in the schedule of a block **enabled by default**: lifts it. |
| **Break** | Lifts an enabled block for a while. One per continuous block, or one per block window. |
| **Pomodoro** | Work (enabled) and rest (lifted) alternate, work first; optional long rest every N rounds. |
| **Allowance** | N minutes of use per period (rolling 24 h, window, day, week, month; refill time), then enabled. |
| **Reward** | Rest time earned by working (blocked, computer in use); saved up to one break, or accumulates until the window/day/week/month ends, or each part kept for a user-picked time (no upper limit on time or amount). |
| **Text break** | Preset break after typing a challenge; block stays locked. |
| **Delay break** | Preset break after a countdown; block stays locked. |
| **Session limit** | N openings per period; a session ends 3 min after switching away. |
| **Pause** | One-off lift until a set time. |
| **Autostart** | Switches a block on: once, time of day on set days, sign-in/wake, or next block window. |
| **Schedule** | Named weekly windows kept outside blocks (*Evening*, *Weekend*), made on the Schedules page. A block or mode uses any number besides its own windows (*Apply schedule*); changing a schedule changes everything using it. Overlapping windows count once. |
| **Place** | A set of Wi-Fi access points (no GPS), made on the Places page or from a block. The computer is there when the user-chosen number of them (default 2) are in range. Android: an area picked on a map, found by GPS (§8.2). |
| **Place rule** | A place in a block's automation; **any number** per block (any block). Arriving switches the block as its default says (enabled by default: off; disabled by default: on). **No actions of its own.** |
| **Automation** | What switches an active block (or a mode's blocks, monk mode, Frozen Chicken) by itself: its windows (own and its schedules') and places. **Enabled by default** (a window or place turns it off) or **Disabled by default** (turns it on). With schedule and places: switching needs **both** (at once) or **either**, user's choice. None: continuous. A block in a mode keeps its own automation, unused, until taken out. |

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
| **Monk mode** | A mode, not a block (a block could be edited or deleted while unlocked); preconfigured, nothing editable but its automation and locks. Blocks `*.*` (exception `file://*` only) and cuts the whole computer's network while on. Switched like any mode: by hand, by its automation (windows, schedules, places; disabled by default), or by a Mode action, which locks it as the action says. | Block |
| **Consequence** | Device action: close tab, close browser, lock screen, logout, shutdown, Mode action. Announced 60 s ahead except from bypass prevention. | Page action |
| **Strict mode** | Off by default. On and something locked: no `sudo`/`pkexec`; nothing runs from `/home` or `/tmp` except exempted folders (adding one weakens); no containers/VMs. Own setting lock. | |
| **No-escape** | Closes boot exits: hides recovery entries and kernel options, GRUB password nobody knows. | Strict mode |
| **Factory reset** | Wiping/reinstalling/booting another OS. Software can't forbid it. | |
| **Factory reset protection** | Makes it slow and deliberate: firmware lock, installer media guard. Without a firmware password the boot menu stays open. | |

### 3.9 Actions

| Term | Meaning |
|---|---|
| **Action** | What a blacklist or phrase set does on a match, and what a block's apps and window titles do when they fire (one set each): page action, consequence or custom action. Apps and window titles close by default; may untick that and take consequences instead or too (fired when found, at most once a minute). Any of them may have no action (does nothing). Place rules have none (§3.6). |
| **Mode action** | Makes a chosen mode run now for a set time, locked for that time, whatever its automation says (R-MODE-4); then as it was. Frozen Chicken: also the freeze mode. Replaces the old monk action and ladder step. |
| **Custom action** | Opens sites, files, folders or programs, or runs a command, as the signed-in user. Wherever actions are (lists, phrase sets, apps, window titles, ladder steps), for a mode becoming enabled, and in hotkeys (R-CUSTOM-1/2). |
| **Hotkey action** | What a hotkey does, in order: switch a block on (with locks) or off, switch a mode on or off, Mode actions (monk mode, Frozen Chicken, modes of blocks; locked), every consequence, custom actions. |
| **Block page** | Replaces the page, with a motivational message. |
| **Close tab / Redirect / Image** | Alternatives to the block page. |
| **Look actions** | Greyscale, no images (every image, animated GIFs too), no video, mute, dark. Change, don't block; combine. |
| **No downloads** | Cancels downloads from matching pages. |
| **Strongest action** | Across all blocks: consequence > close tab > image > redirect > block page. Look actions only if none applies. |
| **Threshold** | Per action. List: different rendered terms in page text. Phrase set: different phrases (default 1). |
| **Hit count** | Per action, lists only (text): total matches incl. repeats. |
| **Rate** | Per action, lists only (text): matches ÷ words, %; pages of ≥ 200 words. |
| **Ad trigger** | Planned fourth scope: ad networks a page loads, by industry. |

### 3.10 Bypass prevention

| Term | Meaning |
|---|---|
| **Bypass prevention** | Counting stopped events and climbing a ladder. `kc9k bypass`. |
| **Event** | Something stopped or noticed; kept a day. |
| **Detector** | Event type a ladder counts: `page`, `search`, `app`, `title`, `browser`, `tamper`, `access` (wrong access password), `workaround`. |
| **Ladder** | N events within T trigger consequences, step by step. Global or per block; one ladder for all detectors. |
| **Step** | One rung: "N events within T: these consequences". |
| **Quiet time** | Without tripping this long, the ladder drops a step. |
| **Workaround** | Mirror, proxy address, alternative front-end, fetch tool. |
| **Tamper** | Clock jump, or daemon restart other than upgrade, shutdown or self-reported crash; Windows: also a session agent ended from outside (one that crashed and said so — Go crash report, exit code 2 — or ended by itself is logged only). *(By claude)* |

### 3.11 Passwords

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Optional; needed to weaken anything; optionally to set locks. | Password lock |
| **Access password** | Optional; needed to view areas: app, dashboard, blocks, device, stats, settings. Commands ask for it too. | Password lock |
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
| **Daemon** | Service re-applying everything every few seconds: systemd service (Linux), Windows service (Windows). |
| **Layer** | Hosts file, firewall, browser policy, closing apps. The extension works on top. |
| **Extension** | Bundled browser extension. Required while anything is locked or uses websites; browsers without it are closed. |
| **Supported browser** | LibreWolf, Chrome or Chromium with the extension; Chrome, Edge, Firefox and Bing (the Microsoft Bing browser app) on Linux, Windows and Android. KC9K shows which are installed and whether each has the extension, and installs it in one click. |
| **KC9K account** | The user's account in KC9K's cloud; only for cross-device sync and the accountability partner (principle 6). |
| **Cross-device sync** | Copying the whole setup between the user's devices: **Upload all**, **Download all** (§4.14). Not a project: a project only adds; Download all replaces. |
| **Device-specific item** | Belongs to one device; never synced (§4.14). |
| **Accountability partner** | A person the user shares their KC9K with (§4.15). |
| **One-click installer** | One download installing everything KC9K needs on its platform with one click (§4.11). |
| **Website** | KC9K's public website (§4.16). |
| **Setting** | Configured outside blocks: **browser setting** (lockable: `safesearch`, `force-search`, `web-only`, `block-devtools`, `no-private`), **tamper protection setting** (§4.10), **behaviour setting** (never locked, exempt from no-weakening: `polite-close`, `block-page-details`), **preference** (interface only). |
| **Hotkey** | Desktop key running its hotkey actions (§3.9). |
| **Project** | Whole setup in one file; presets (every type) only if the user chooses. *Backup*: everything, presets included. *Share*: without places and user names. Import previews, only adds, renames clashing blocks; a file whose custom actions open or run something asks first *(By Opus)*. |
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

GUI and CLI offer the same functions. Users work in the GUI.

- **4.1 Overview**: active blocks (enabled/lifted, locks), today's remaining
  windows, break status, ladder positions.
- **4.2 Blocks**: create (empty, block preset, allowlist) · on/off ·
  continuous or automated · rename · duplicate (copy inactive) · delete
  (refused while locked) · affected users · export/import.
  - The list (R-GUI-3): one card per block: **name** (opens it), **on/off
    switch**, **"Continuous block"/"Automated block"** (opens the
    automation view, §4.5), **breaks** ("No breaks" or what it allows;
    opens them), **locked/unlocked** (opens the locks); nothing else.
    Deleting an opened block returns to the list.
  - Modes: on the same page, a card each like a block's: name (opens the
    mode), on/off switch (switches its blocks; monk mode: lets its
    automation run it), Continuous/Automated, its blocks. Drag a block onto
    a mode to put it in; take it out in the mode. Monk mode and Frozen
    Chicken always there; other modes created, renamed, deleted (blocks
    stay). Frozen Chicken's switch asks for an end time (≤ a week ahead).
    Mode view: its blocks as cards, *Add blocks…* with *Create new
    block…*, automation, wallpapers, custom actions on becoming enabled,
    locks.
  - Names: blocks, modes and schedules each have their own; a block and a
    mode may share one.
- **4.3 Block editor**:
  - Add: type, Enter; focus stays. Remove: button or Delete/Backspace;
    selection moves to next. Multi-select: Ctrl/Shift-click, Select all,
    Ctrl+A. Edit: double-click moves entry to text box (remove + add).
    Find: Ctrl+F.
  - Block menu: Start, Stop, Unlock, **Automation** (§4.5; greyed out in a
    mode: "Automated by *mode*"). Apply at once.
  - Save edits/Cancel: an opened block is always a draft; nothing applies
    until *Save edits*, on every tab, Breaks & allowance and Bypass
    prevention too (the last setting of each kind kept, shown as not saved,
    applied through the same checks as when set directly). Locked block:
    Save edits refuses weakening changes and names them. Things done
    rather than set (taking a break, opening/ending an allowance session),
    like the block menu, apply at once. *(By Opus)*
  - Import *From KC9K* (other blocks' lists/phrase sets, presets; several)
    or *From file* (`.txt`); also *Whole internet* and *All searches
    containing…*. Export list/phrase set to `.txt`.
  - Import is one button; KC9K reads the layout: phrases (`[a, b] [c]`, one
    per line) go into a phrase set; rendered terms (one per line, or a KC9K
    list) become **one** word set `[term 1, term 2, …]`; sites, also from
    hosts files (`0.0.0.0 example.com`) and adblock rules
    (`||example.com^`), go into a list. Phrase sets export in phrase
    format, one per line.
  - Lists: create (kind; scope boxes), rename, delete, scope, actions,
    threshold, hit count, rate, test mode. Block password.
  - Phrase sets: create (unnamed gets a name), rename, delete, scope,
    actions, threshold, test mode. Phrase editor inside the set's card (not
    a pop-up): any number of word sets, each necessary/optional, phrase
    length; double-click a phrase to load it; saving a loaded phrase
    changes it (never adds a copy).
  - Apps: program, folder, title entry; their actions. The block's own
    ladder.
  - Social media settings: a collapsible section after the blacklists,
    above the exceptions.
  - *Save as preset*: on a block, on its lists (asks *Blacklists*,
    *Exceptions* or *Both*, and *Include actions?*), on a phrase set, on its
    automation.
- **4.4 Presets and recording**: Presets page, grouped by type (blocks,
  lists, phrase sets, schedules, automations, hotkeys): create, edit,
  rename, delete, use (copied into a block, mode or hotkey), save from one.
  Link recording into a list or list preset; word recording, ranked, cut
  before saving.
- **4.5 Automation** (§3.6): weekly grid; drag to add/move, click to edit;
  15-min grid, 5-min times; copy to days; similar windows edited together;
  other blocks greyed; break per window; allow windows. Enabled/Disabled by
  default; places, any number, each showing plainly that it is added,
  whether arriving switches the block on or off, and whether the computer
  is there now; with schedule and places, both or either. Same for a mode
  and monk mode; Frozen Chicken has windows only.
  - Schedules and Places each have a sidebar page: made and changed
    without a block, each showing what uses it. A block's or mode's
    automation has **Apply schedule…** and its places to tick.
- **4.6 Locks**: all of §3.4, alone or layered, on blocks, browser settings
  and settings (global ladder too). Layer order fixed while locked; new
  lock goes last. Timer locks always extendable; set for a duration or
  until a calendar date (R-GUI-2).
  - Stacking, no exceptions (2026-11-20): a lock of a kind already there is
    another layer of that kind, last, never a strengthening of the first
    (two partners' password locks; timer, text, then timer). A new timer
    or delay lock counts from the previous lock's unlocking (R-LOCK-6);
    extending a timer lock is a separate action; range, text and password
    locks stack, with or without re-lock; each hotkey press adds its locks
    again (confirmed per R-LOCK-12).
  - Setting locks (browser settings, tamper protection, strict mode, global
    ladder, scheduled freezes) stack the same way; their re-lock restores
    the setting's whole lock configuration. A lock option given twice in
    one command is two locks, in order. A hotkey or mode lock keeps one
    password, so at most one password lock. *(By Opus)*
  - Text lock challenge: typed in the command's terminal or, without one,
    in a window the command itself opens on the user's desktop; only keys
    typed there count. Paste, drop or piped answer refused (R-LOCK-5): the
    CLI can tell typing from pasting only in input it reads itself. The
    GUI only runs the command. *(By Opus)*
- **4.7 Breaks and autostart**: all of §3.6. Break status also in the
  extension's toolbar button and popup.
- **4.8 Frozen Chicken**: a mode among the blocks (no own page): on for a
  time or until a date, ≤ a week ahead; its automation schedules freezes;
  both may coexist.
  - A running lock screen/logout/shutdown freeze, extended by hand or by a
    scheduled freeze, keeps its mode, never becoming allowed apps only
    (`kc9k freeze --mode apps` from a text console would reopen programs).
    The reverse strengthens and stays free. *(By Opus)*
- **4.9 Statistics**: time used and times blocked per site, app, title.
  Time frame (presets/custom), bar chart over time (a colour per site/app),
  per user, top 5/10, search, filter by block, JSON export, delete, on/off,
  optionally skip private windows. Separate from events; local only.
- **4.10 Settings**:
  - Preferences: 24-hour clock, first weekday, theme with darkness slider
    (dark grey to black), open at login, tray.
  - Extension: install dialog when missing.
  - Notifications: timer lock end, window start/end, Pomodoro change,
    allowance out, app blocked; advance warnings in minutes (`2,5`).
  - No weakening during active locks.
  - **Tamper protection** (one group; once on, can't be turned off while
    anything is locked, and its own lock must be unlocked; turning one on
    asks OK/Cancel):
    - close private windows and Tor; close guest profiles (also turns off
      the login screen's guest session); close browsers if the extension
      stops; grace for missing extension (default 60 s);
    - allow `file://` blocking: local files judged like web pages (their
      `file://` entries, rendered terms and phrases in address, title and
      text), spared by the block's file exceptions (`file://*` by default);
      *Whole internet* still leaves them reachable *(By Opus)*;
    - block unsupported browsers (all but supported browsers with the
      extension);
    - block the installer: while anything is locked, no reinstall,
      downgrade or removal via installer, package manager or `install.sh`;
      a newer version installs;
    - block inactive tabs (background tabs blocked at once);
    - block embedded content (blocked site inside another page);
    - block task managers while locked (System Monitor, htop);
    - clock: block time/date changes; check against hardware clock and
      internet time (an outside query the owner agreed to, 2026-11-18).
    - Defaults *(By Opus)*: close browsers if the extension stops, block
      inactive tabs and block embedded content (always done before they
      were settings) are on until turned off; the rest off until turned
      on. Off, browsers without the extension are closed only while
      anything is locked, a background tab is blocked when switched to,
      and frames are left alone (hosts and firewall still block whole
      domains; see §6 2026-11-21 policies).
  - Browsers: supported browsers found, extension status, one-click install.
  - **Browser settings**, each lockable: SafeSearch on Google, Bing,
    DuckDuckGo, YouTube (Restricted Mode), each its own setting;
    *force-search* (Google only, others blocked, browser search box uses
    Google); *web-only* (Google web results only: no images, shopping, AI
    answers); block developer tools; no private windows. While SafeSearch
    on Google, Bing or DuckDuckGo is on, Brave, Yahoo and SearXNG are kept
    strict and engines that can't be (Startpage, Ecosia, Mojeek, Yandex,
    Marginalia) are blocked; no settings of their own (each is a way
    around the strict engine). YouTube Restricted Mode alone does neither.
    *(By Opus)*
  - DNS: server asked for blocked sites' real addresses (firewall layer
    only). Default: the network's own; Quad9 if it can't answer.
  - Use counting: strict use counting.
  - **Circadian rhythm** (own sidebar page, not on Settings; listed here
    so references stay):
    - automation clips (one set for all days): backlight %, software
      brightness %, colour temperature K; add/move/remove points, bend the
      curve between them; presets;
    - wallpapers: picture plus times on a video-editor-like timeline (drag
      a bar to move, an end to resize); no fixed periods; *Override modes*
      = a mode's *Override circadian rhythm wallpapers* the other way round
      (one on is the other off);
    - *Play the day*: next 24 hours shown in about 15 s, then back to now;
    - dark mode = system dark mode: system default, always light, always
      dark, or light between two times;
    - dark pages follow dark mode, with exempt sites (toolbar button
      exempts a site or shows it dark again); switch *Show dark web pages
      while dark mode is on* turns on the browser's own dark mode;
    - not lockable;
    - never overrides the user: never puts a level back; a manual change
      pauses that clip until its next point or resume; a hand-picked
      wallpaper/dark mode stays until the next scheduled change;
    - leaves the desktop's own brightness control and night light working;
      off, touches nothing.
  - Device protection: strict mode, no-escape, factory reset protection,
    guided setup.
    - **Firmware lock** (factory reset protection) *(By Opus)*: sets
      firmware settings Linux can reach: USB and network boot and the
      one-time boot menu off (a password alone leaves it open on many
      computers), boot order locked, then last a firmware password the
      trusted person types and keeps (KC9K never stores it). Confirmed like
      a lock (R-LOCK-12). Removing it needs that password, restores what
      it changed, refused while anything is locked. Not a KC9K lock (holds
      nothing else on). Settings the firmware lacks are named as steps in
      its own setup.
    - **Guided setup** *(By Opus)*: a read-only checklist, not a wizard (no
      first-run wizard, 2026-11-14): each strictest-setup step in order
      (settings password, no weakening, tamper protection, strict mode and
      its lock, root-equivalent groups, root password, installer media
      guard, firmware lock, other systems in the boot menu, no-escape
      last), whether done, and a button running that setting's own switch.
  - **Wallpaper default** (not circadian rhythm): shown when no mode or
    circadian rhythm wallpaper applies, and while the place isn't known:
    *Latest wallpaper*, *Circadian rhythm* or *Custom* (picture chosen
    here).
  - Passwords: settings, access. Global ladder, lockable. Project: backup,
    share, import. Behaviour settings.
  - **Linux-only** parts of a project *(By claude)*: app entries that are a
    Linux path (`/` not `//`, or `~/`), Linux app ID or X11 window class
    (`org.gnome.Nautilus`), or Linux program (desktop, systemd, PAM tools:
    `gnome-terminal-server`, `systemctl`, `pkexec`); custom actions running
    such a program or opening a Linux path; setting locks of Linux device
    settings (PAM guard, no-escape). Imported on Windows: kept unused,
    listed by the dry run as "not used on Windows"; never an error, never
    mapped to Windows names (mapping would block programs never named;
    dropping would lose them on the way back). File may have CRLF and a
    UTF-8 BOM.
- **4.11 Install**: `.deb`. No automatic updates for now. Uninstall refused
  while anything is locked. Every platform (Linux, Windows, Android) gets a
  one-click installer: one download, one click, KC9K and its extension in
  the supported browsers set up.
- **4.12 CLI**: every function has a command; commands ask for access
  passwords too. Cold Turkey equivalents:

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

- **4.13 Hotkeys**: own sidebar page. Each: keys, hotkey actions in order
  (§3.9), *Run without confirmation* (§6 2026-11-14); add, change, remove,
  *Save as preset*.
- **4.14 Cross-device sync** (Linux, Windows, Android; KC9K account):
  *Upload all* sends this device's whole setup to KC9K's cloud; *Download
  all* replaces this device's whole setup with the last upload. Everything
  copied one to one except device-specific items: blocks, lists, phrase
  sets, locks and their state, settings, presets. Download all is not
  refused by a lock or counted as weakening (exception to R-WEAK-1): a
  block unlocked and deleted on one device is unlocked and deleted on the
  other after Upload all there and Download all here; unlocking still
  needed the lock lifted on the uploading device.
  **Device-specific items** (never synced) *(By Opus; owner called the
  list obvious)*: places, affected users and user names, strict mode,
  no-escape, factory reset protection and firmware lock, circadian
  rhythm's screens and hardware, wallpapers, apps' paths and file
  fingerprints, statistics, the device's own system passwords (root,
  firmware).
- **4.15 Accountability partner** (Linux, Windows, Android; KC9K account):
  everything similar software offers except screenshots. The user invites
  a partner, who gets: activity reports (daily or weekly: time per site
  and app, times blocked, top blocked); alerts as they happen (bypass
  prevention events; a block switched off, unlocked or deleted; a lock
  ended early; tamper events; KC9K stopped, uninstalled or not heard
  from); can hold password locks and the settings password and approve
  unlock requests. By email and on the partner's own KC9K. Several
  partners; removing one is weakening.
- **4.16 Website**: public, like Cold Turkey's in kind (own design): what
  KC9K does, one-click installer for Linux, Windows, Android, free forever,
  FAQ, KC9K account sign-in. "No user-facing docs" (`CLAUDE.md`) doesn't
  apply to it.

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
- **R-SCHED-1** An active automated block is in its default state (enabled or disabled) unless its automation switches it (§3.6: window, place; both or either); an inactive block is never enabled. Automation never switches a block on or off (active/inactive) and has no actions.
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
- **R-PHRASE-1** A phrase has ≥ 2 word sets; each freely necessary or optional (none necessary allowed). Phrase length ≥ 2 and ≥ the number of necessary sets.
- **R-PHRASE-2** A match takes one term from each necessary set, plus optional ones up to the phrase length.
- **R-PHRASE-3** One word set gives at most one term: `[long cat, neko] [running, sitting]` matches "long cat running", not "long cat neko".
- **R-PHRASE-4** Terms in any order, directly adjacent, separated only by blank space. Punctuation breaks the match. An in-word apostrophe is part of the word ("let's"); a hyphen between letters is blank space (`long cat` matches "long-cat").
- **R-PHRASE-5** A break between page parts (heading, paragraph, list item, button) ends a sentence; phrases and bridges don't cross it.
- **R-PHRASE-6** URL: decoded first (`%20` = space); separators `-` `_` `/` `.` `?` `=` `&` `+` `#` `:` `,` are blank space. (So no search lists: URL scope catches queries, text scope results.)
- **R-PHRASE-7** A bridge skips 0–20 words between terms of different sets: `[big-, strong] [-gorilla, cat]` matches "big and long gorilla" and "big gorilla". Commas allowed, no `.` `!` `?`; order fixed (`big-` before `-gorilla`).
- **R-PHRASE-8** A bridge needs both ends: `-gorilla` only after a `term-` of another set; `[-gorilla, cat] [jumping, river]` doesn't match "jumping gigantic gorilla". One-ended bridges are refused on entry. A middle link needs ≥ 3 word sets.
- **R-PHRASE-9** Each phrase set action has a threshold of different phrases (default 1). No hit count or rate.
- **R-PHRASE-10** Phrase sets can be exported, imported and kept as phrase set presets; no recording into them.
- **R-TITLE-1** Window title entries never apply to browsers.

**GUI**
- **R-GUI-1** Every duration field takes days, hours and minutes (or typed text such as `2d 4h`, `300m`); never minutes only. A value outside the allowed range is never changed silently: the field says why and keeps what was typed.
- **R-GUI-2** Every timer lock is set either for a duration (R-GUI-1) or until a date and time picked on a calendar.
- **R-GUI-3** Beautiful, intelligent, intuitive; GNOME Human Interface Guidelines and the usual usability heuristics: tasks first (overview shows what is on, until when, how locked; common tasks one or two clicks away); one primary action per view; advanced options behind *More options*; plain glossary words, short sentences; lock state shown wherever a locked thing appears; unusable controls disabled with the reason; input checked where typed; one widget per concept everywhere; everything reachable by keyboard (Enter presses a dialog's OK); the window never freezes while a command runs. The Android app has the same pages and behaviour; tapping never moves the view back to the top.

**Browsing**
- **R-PAGE-1** No flashing: a page that a list or phrase set will block, replace or give a look action never shows its content without it, also when moving within a site. Content stays hidden until judged, at most 3 s (then shown and judged as usual).
- **R-PAGE-2** Search results leading to a blacklisted address are removed from search results pages (`reddit.com` blacklisted: no Reddit results).

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

Only what §1–5 don't say, or a pointer. One line each; untagged = owner.
Windows mechanisms: `notes/windows.md`.

- 2026-11-10: One ladder for all detectors.
- 2026-11-14: Monk mode is a mode (§3.8). Hotkey *Run without confirmation* off: pressing it with a lock asks in a desktop dialog; on: confirmed only at setup ("I know this will not ask for confirmation"), as automatic Mode actions/ladder steps (R-LOCK-12). No page score. Root login/`su` PAM guard only during strict mode's lockdown; no boot order guard, no hiding other systems behind the GRUB password. Every sudo-allowed command checked for hostile arguments (symlinks, `..`, files under `/etc`). Text lock challenge in its own window (§4.6). No first-run wizard. No categories (no shipped lists or phrase sets). `force-librewolf` → *block unsupported browsers*. Declutter removed (broke pages). No search lists (R-PHRASE-6).
- 2026-11-17 *(By Opus, to confirm with owner)*: Place unknown (R-MODE-5): at once at boot and after waking, else after a minute without networks; the wallpaper default also replaces circadian rhythm's picture when a places-switched mode would override it; a mode by time alone still shows; with a picture or circadian rhythm as default it also goes on before sleep; the default's own picture is kept when another default is chosen.
- 2026-11-18: Clock check may ask internet time (§4.10). SafeSearch lockable per engine (§4.10). When Linux is finished, the owner's brothers test it.
- 2026-11-20: Lock stacking, no exceptions; re-lock optional, restores the whole configuration (§3.4, §4.6). Project presets optional, backup always (§3.13).
- 2026-11-20 (By Opus): Text lock challenge read by the command itself (§4.6).
- 2026-11-21 (By Opus): Draft tabs (§4.3); `file://` blocking (§4.10); firmware lock (§4.10); tamper protection defaults (§4.10); setting locks stack (§4.6); freeze extension (§4.8); guided setup (§4.10); SafeSearch on other engines (§4.10).
- 2026-11-21: Android app planned (§8); the built Nokia mode becomes an ordinary block (All apps; phone and SMS apps as app exceptions) automated by a Home place, monk mode for the network.
- 2026-11-21 (By Opus): **Rules server has no secret** (replaces 2026-11-14's; a policy-given secret is readable by every local program). It holds by: answering only requests addressed to its own address (no DNS rebinding); posts only with the extension's own header (needs a CORS preflight, never answered); no CORS header in any answer; a check-in counts only for the browser whose own connection made it (`/proc/net/tcp`); no post weakens a block.
- 2026-11-21 (By Opus): Block embedded content off: a blacklist's paths and patterns also stay out of browser policies (they block frames too); whole sites stay; the extension still blocks paths in pages; monk mode's and factory reset protection's own lists keep theirs.
- 2026-11-21 (By Opus): Ad trigger industries ship only real ad-request domains (EasyList, AdGuard, Peter Lowe's, StevenBlack's, or seen in real pages), never an affiliate platform's or game maker's own site.
- 2026-11-21: **Windows** now, before Android. Built for an **administrator account** (bypass prevention holds against an admin). Native GUI: C# on WinUI 3 (agent's choice). Chrome gets the extension via an unlisted Chrome Web Store listing. Tested in CI, a VM, the owner's laptop.
- 2026-11-21 (later): Windows better than Cold Turkey. Strict mode on Windows: while locked, the admin is denied what would stop KC9K (service, files, registry keys, boot and recovery settings). Edge supported. Code-signed. Works on Windows Home (no Pro-only features).
- 2026-11-21 (By Opus): Windows GUI preferences per Windows user (`%APPDATA%\kc9k\gui.json`); one GUI per user.
- 2026-11-21 (By Opus): Windows commands run in the service via the named pipe; the caller is taken from the pipe client's token, never from what it says; files read for the caller are opened impersonating it; only Administrators run commands (no elevation needed); the client talks only to the KC9K service's own pipe; no remote clients.
- 2026-11-21 (By Opus): Windows install: `kc9k install` sets up program (Program Files, SYSTEM-owned), data (ProgramData, SYSTEM only) and service (restarts after any failure); rerun with a newer exe upgrades; `kc9k uninstall` as on Linux, data stays. Session agent: no per-session lock a user program could take first.
- 2026-11-21 (By Opus): Windows equivalents: firewall = WFP under KC9K's own persistent provider (holds from boot, while the service is stopped); Chrome/Edge supported only once the Web Store listing exists; strict mode's noexec = closing programs run from a profile's Downloads, Desktop, Documents (not AppData), temp, removable drives; logout freeze signs out every new session; Night light read, never switched (person asked to turn it off); wallpaper/dark mode reach only signed-in people; service runs in Safe Mode; factory reset protection and no-escape turn the recovery environment off.
- 2026-11-21 (By Opus): Windows installs from one MSI that decides nothing (`kc9k install`/`uninstall`/`prepare-install` do). Removal refused while locked or with a settings password set (installer can't type it); reinstall/older refused while locked with *block the installer*; newer always installs. Signed with the owner's certificate when configured.
- 2026-11-22: Sync, one-click installer, Chrome/Edge/Firefox/Bing, website, accountability partner on Linux, Windows, Android (§1, principle 6, §3.13, §4.11, §4.14–4.16); dark pages switch (§4.10), R-PAGE-2, R-GUI-3 additions, Android places by map (§3.6).
- 2026-11-22 (By Opus): Windows parity audit: system proxy/VPN, Windows DoH and WebView2 apps are not holes (hosts, extension, policies, WFP cover them). Holes in a chosen protection are closed inside it, not as new settings (principle 3): Windows strict mode also stops ways to SYSTEM, admin privileges, new accounts, VMs, running downloaded programs.
- 2026-11-22 (By Opus): Windows recovery (principle 4) is Safe Mode: freezes and strict lockdown skipped, `kc9k recover` works, unless no-escape; blocks still enforced. F8 menu stays on until no-escape.
- 2026-11-22 (By Opus): Windows hardware clock = OS start time logged at boot (Kernel-General event 12) plus boot clock; only ever gives back less time.
- 2026-11-22 (By Opus): Windows *block time and date changes* while locked: a changed time zone, or a clock jump internet time doesn't confirm, is put back and counted (schedules read local time).
- 2026-11-22 (By Opus): Windows firmware lock via the firmwares' WMI interfaces (Dell, Lenovo, HP), not vendor tools.
- 2026-11-22 (By Opus): Windows one-click browser install: winget as SYSTEM, an outside query the command states.
- 2026-11-22 (By Opus): Windows tray menu offers only strengthening actions, each bringing the window forward first; weakening stays in the window.
- 2026-11-22 (By Opus): Windows GUI start arguments (jump list, shortcut, second start) only open a page or GUI dialog, never a kc9k command (R-LOCK-12).
- 2026-11-22 *(By claude)*: Windows GUI has no first view ("Get started" dropped); pages follow the GTK layout.
- 2026-11-22 (By claude): Linux projects import on Windows as they are (§4.10 Linux-only); session agent crashes (§3.10 Tamper).
- 2026-11-23 *(By claude; owner asked the agent to choose)*: KC9K account and cloud on **Supabase** (not Firebase): Auth, Postgres, private Storage with row-level security, reached over plain HTTPS from Go and Android (no SDK, no Play Services, which Firebase's Android libraries lean on, and no Go client SDK); open source, so it can be self-hosted. Upload all = one file per account; Download all takes the last upload (last upload wins).
- 2026-11-23 *(By claude; owner asked the agent to choose)*: One-click installer (§4.11): Linux the `.deb` (double-click → Mint's package installer → Install; it also installs LibreWolf); Windows the signed MSI; both on GitHub Releases, later the website. Android open (§7).

## 7. Open decisions

- **Android recovery** (principle 4, §8.7): the user's way out when a bug
  traps them on the phone; what becomes of the built "Lift everything".
- **Factory reset protection with a trusted Google account** (§8.4):
  offered on phones with Google's protection or not (principle 6: the
  account's address goes to Google's reset check).
- **Android one-click installer** (§4.11) *(By claude)*: KC9K needs
  device owner, which Android grants only by `adb` on a phone without
  accounts or by a QR code at setup after a factory reset; an APK alone
  can't. Options: (a) QR code at setup (factory reset); (b) "Set up phone"
  in the computer's KC9K over USB (bundled `adb`; accounts removed first);
  (c) without device owner (weaker protection). Proposed: (a) and (b).
- **Signing the extension for Firefox** (§8.2): release Firefox installs
  only Mozilla-signed add-ons; signing (unlisted) needs a Mozilla account
  held by the owner, not users.

## 8. Android

KC9K on a phone (2026-11-21): every function of §4 except computer-only
ones (§8.6). Built after Linux is finished (principle 8). §3 words mean the
same unless said here.

### 8.1 Platform

- Works excellently on GrapheneOS and the most common Android (stock on
  Pixel, Samsung One UI), with or without Google Play Services; tested on
  all three (R-ANDROID-1).
- Kotlin. Jetpack Compose with Material 3 (AndroidX) allowed; no other
  third-party libraries. Never needs Google Play Services or an account
  (R-ANDROID-2).
- **Device Owner** required, set once at setup.
- Rule-maker: the app's own Kotlin command layer, held to Linux behaviour
  by test cases shared with Go (as `match.js` is shared), not Go compiled
  for Android (R-ANDROID-3).
- No sync: nothing exchanged with the Linux app; a phone's projects
  (§3.13) are between phones.

### 8.2 Words on Android

| Term | Meaning | Not |
|---|---|---|
| **Device Owner** | Android's strongest app role, given once at setup (`adb`, or a QR code on a freshly reset phone): lets KC9K suspend apps, restrict settings, keep its VPN on, refuse its own uninstall. | Device admin |
| **App** (Android) | Matched by package name, app name, signing certificate (`cert:sha256:…`) or role (`role:dialer`, `role:sms`: default phone and SMS apps). **All apps** (`*`): every launchable app but the essentials (KC9K, launcher, system UI, emergency calls). | Window |
| **App exception** | An app a block's All apps entry lets through; spares it from its own block only (as R-LIST-1). | Exception list |
| **Area** | A circle on the map (centre, radius 50–2000 m). A **place** is Wi-Fi networks (§3.6) or an area: in it when the location says so; left when the accuracy circle is fully outside for 10 minutes. | Wi-Fi place |
| **Screen text entry** | Words matched against text an app shows (Shorts in the YouTube app); the phone's window title entry. Needs the accessibility layer. Never browsers (as R-TITLE-1). | Rendered term |
| **Layer** (Android) | **VPN layer** (KC9K's own VPN filtering names: hosts and firewall), **app suspension**, **Device Owner restrictions**, optional **accessibility layer** (screen text entries). Extension on top. | |
| **Supported browser** (Android) | IronFox or Firefox with the extension. Others (Chrome, Samsung Internet, Vanadium, …) suspended while the extension is required. | |
| **Block page** (Android) | Also shown full screen when a blocked app is opened. | |

### 8.3 Functions on Android

- **Blocks, lists, phrase sets, social media settings, exceptions, test
  mode, presets, recording**: as §4.2–4.4; websites via extension and VPN
  layer; apps as §8.2, with app exceptions.
- **Locks**: every kind of §3.4, layered, re-lock, confirmation
  (R-LOCK-12), setting locks, passwords. Restart lock: a later boot of the
  phone. Text lock challenge typed in KC9K's own screen; only characters
  typed one by one count: paste, swipe typing, a suggestion or any input
  of several characters at once refused (R-LOCK-5). Trusted time:
  automatic time required; date, time and time zone unchangeable while
  anything is locked.
- **Automation**: schedules, places (§8.2 Area), modes, breaks, Pomodoro,
  allowance, reward, session limit, pause as §3.6. Autostart
  *sign-in/wake* = phone starting or being unlocked.
- **Use** (§3.7): blocked item in front, screen on and unlocked; sound
  counts where Android tells KC9K.
- **Monk mode** (R-MONK-2): VPN layer drops all but KC9K's local traffic;
  Wi-Fi off (on but unused while places decide). Mobile calls and SMS
  keep working.
- **Frozen Chicken**: freeze modes *lock screen* and *allowed apps only*.
- **Consequences**: close tab, close browser, lock screen, Mode action.
- **Custom actions**: open sites, files, folders and apps; no commands.
- **Bypass prevention**: as §3.10; detector `title` counts screen text
  entries.
- **Statistics**: as §4.9; apps from Android's usage records, sites from
  the extension.
- **Settings**: preferences (24-hour clock, first weekday, theme with
  darkness slider), notifications, passwords, no weakening, browser
  settings, DNS (asked by the VPN layer), use counting, behaviour setting
  `block-page-details`, global ladder, projects.
- **Install**: APK with KC9K's own updater; uninstall refused while
  anything is locked (Device Owner).

### 8.4 Protection on Android

- **Tamper protection** (§4.10) on a phone: close private windows and
  guest profiles (also: no other users, guests, work profiles or app
  clones can be added; a clone escapes suspension); close browsers if the
  extension stops; grace for a missing extension; allow `file://`
  blocking; block unsupported browsers; block the installer (no unknown
  sources, no older KC9K); block inactive tabs; block embedded content;
  block task managers becomes *app controls* (no force-stop, data clearing
  or permission removal for KC9K); clock.
- While the VPN layer is needed, KC9K's VPN is always on, traffic can't
  bypass it, and VPN and Private DNS settings can't change: the layer
  working, not a chosen protection.
- **Strict mode** (§3.8): on and something locked, closes safe mode, USB
  debugging, adding users, turning location off, factory reset from
  Settings. Each can be left out (leaving one out weakens); own setting
  lock.
- **Factory reset protection**: reset from Settings closed by strict mode;
  reset from recovery can't be stopped by software (§3.8). Open: §7.
- **Guided setup** (§4.10): Device Owner, usage access, notifications, VPN
  layer, battery (never put to sleep), extension in a supported browser,
  accessibility layer (optional; Android 13+ sideloaded apps first need
  *Allow restricted settings*), settings password, no weakening, tamper
  protection, strict mode and its lock.
- **Circadian rhythm**: one brightness clip, wallpapers (home and lock
  screen), dark mode, dark pages; colour temperature where the phone
  allows (Night Light, Eye comfort shield). What a phone can't do is shown
  disabled, with the reason.

### 8.5 GUI on a phone

- R-GUI-1–3 hold; R-GUI-3 follows Material 3 in place of GNOME, same
  heuristics.
- Looks and speaks like the Linux app: same page names and order, same
  words, cards (12 dp corners, thin outline), status pills (on green,
  locked red, lifted blue, warning amber), theme with darkness slider to
  black; KC9K's own colours, not the wallpaper's.
- Phone: bottom bar **Overview · Blocks · Statistics · More**; *More*
  holds Schedules, Places, Presets, Circadian rhythm, Settings. Wider
  screens: a rail with every page.
- Overview: what is on, until when, how locked first (countdown ring for
  timer locks); Start, monk mode, Frozen Chicken one tap away.
- Blocks: modes above blocks; two-line card (name and switch; then
  *Continuous/Automated block*, breaks, lock state, each opening its
  part). Into a mode by long-press and drag, or *Move to mode…*.
- Block editor: tabs Websites · Apps · Breaks & allowance · Bypass
  prevention; *Save edits* bar shows once something changed. Lists full
  screen, add field on top; long-press selects several.
- Automation: calendar-like week view (long-press and drag adds a window,
  tap edits) and a list of windows for exact times.
- Dialogs are bottom sheets; layered locks shown as a numbered stack.

### 8.6 Not on Android

Hotkeys, hotkey presets and hotkey actions; folder entries; scripts;
affected users; logout and shutdown (freeze modes, consequences); commands
in custom actions; no-escape; firmware lock and installer media guard;
behaviour setting `polite-close`; open at login and tray; window title
entries (replaced by screen text entries); R-DESKTOP-1; projects exchanged
with the Linux app.

### 8.7 Rules

- **R-ANDROID-1** Every function works on GrapheneOS and on the most common Android alike; what a phone can't offer is shown disabled with the reason, never silently missing.
- **R-ANDROID-2** KC9K never needs Google Play Services or an account.
- **R-ANDROID-3** The app's command layer makes every weakening decision (principle 10); the GUI, the extension and the guard service only run it or follow it. A refused weakening is logged (R-WEAK-3).
- **R-ANDROID-4** The accessibility layer is never the only protection: everything but screen text entries works without it.
- **R-ANDROID-5** The user is never trapped by a bug (principle 4): the recovery path is open (§7).
