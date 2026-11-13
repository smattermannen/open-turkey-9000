# KC9K Specification

What Killer Chicken 9000 (KC9K) **is meant to be**: its goal, its principles,
its words, what it does and the rules it keeps. Where the code stands is in
`ROADMAP.md`, never here.

**Rules for this file**

1. Only the owner decides what goes in here. An agent may propose a change,
   but makes it only when the owner has agreed, and logs it under
   [Decisions](#6-decisions).
2. No status, no work items, no dates other than in [Decisions](#6-decisions).
   "Built", "planned" and "to do" belong in `ROADMAP.md`.
3. Use the words of the [Glossary](#3-glossary) exactly, in code, interface,
   docs and conversation. Something new that needs a name gets it here first.
4. When the code, `README.md` or `ROADMAP.md` disagree with this file, this
   file is what was meant. If this file seems wrong, raise it with the owner
   ([Open decisions](#7-open-decisions)); don't change it alone.
5. Rules (§5) have IDs; tests name the rule they prove.
6. Where KC9K uses a different word from Cold Turkey, the Cold Turkey word is
   given as *CT: …*.

---

## 1. Vision

### 1.1 The goal

KC9K gives people control over their digital lives. It helps them stay on the
right task and break free from harmful digital habits: excessive gaming,
doom scrolling, adult content, and more. It is a one-stop shop for digital
health.

Much of today's software is designed to use our own psychology against us,
for the profit of a few. KC9K is built to fight back against that design.

### 1.2 How it gets there

**It is very hard to get around.** In its strictest setup, the only way out
should be a factory reset. Even that should be made hard enough that nobody
does one on impulse, with options the user chooses for how hard.

**It does more than block.** Cold Turkey Blocker has two actions: block a
page in the browser, or close an app. KC9K can also take the images out of a
page, turn it grey, mute it, stop its videos, darken it, declutter it, close
the tab or the browser, log out, shut down the computer, or start monk mode.
It enforces SafeSearch, and kills addictive social media feeds with one
switch.

**It recognises more than addresses.** KC9K looks at a page's address, its
title and the text it shows. The goal is to also look at the advertisements a
page carries. For example: take the images and videos off any page that shows
ads from the gambling or adult industries, or block such pages outright.
Apps are recognised by name, by sandbox ID, by the script they run, and by a
fingerprint of the program file, so renamed copies are caught too.

**Any trigger can lead to any action.** Each block holds several lists, and
each list has its own actions. A combination of words on a page can shut the
computer down if that is what the user sets up.

**It stops the loophole hunt.** Getting around a tricky system can feel even
more rewarding than the thing it blocks. Cold Turkey lets you spend hours
trying to slip past your own rules. KC9K's bypass prevention watches for that
and climbs a ladder the user chooses: close the tab on one match, close the
browser on the next, and so on up to shutting the computer down and locking
the user out for the rest of the day. Killing the browser when blocks trip in
quick succession is often enough to break the spell. Nobody goes on a goblin
mode loophole hunt against a system like this.

**One button.** Hotkeys start a block, such as a study block with a Pomodoro,
at the press of a key. Sometimes opening an app is too big a hurdle until it
is too late. A key that is always there breaks the spell when it is needed.

**The circadian rhythm.** KC9K follows the time of day, and the user can lock
it: dark mode in the browser and in apps, a blue light filter, screen
brightness, and wallpapers. A bright and positive wallpaper in the morning, a
soothing warm one in the late afternoon, a starlit one at night: these are
cues for the body clock. It also means KC9K isn't "just a blocking app" when
friends ask.

**The collective imagination is the limit.** Cold Turkey exports lists. KC9K
exports a whole project — blocks, lists, actions, schedules, bypass
prevention — so people can share setups with each other.

**It is completely free.** Young people, some as young as ten and with
computers of their own, are hooked on harmful media, and some of them want
out. They won't ask a parent to pay 45 dollars for a blocker. KC9K costs
nothing, so anyone who wants to quit can.

### 1.3 In one paragraph

Everything Cold Turkey does, plus: bypass prevention the user shapes, more
triggers, more actions, any trigger to any action, several lists per block
each with its own actions, layered sequential locks (several locks on one
block, lifted in the order the user chose), whole projects exported and
imported, ready-made modes such as monk mode, lockable SafeSearch, and
circadian rhythm support.

### 1.4 After publishing

- A **page score**: KC9K weighs what a page delivers (word frequencies, ad
  networks, page layout) and acts on the score.
- **Windows support.**

---

## 2. Principles

They apply to every change.

1. **Fix current functionality before adding new features.** Known bugs,
   failing tests, features that don't work as documented and code that
   disagrees with this file come before any new feature.
2. **Strengthening is always free; weakening always costs.** Anything that
   weakens a held block, a locked browser option or a locked setting passes
   its gate and the settings password. Anything that strengthens needs no
   permission.
3. **Custom security.** The user makes KC9K as secure as they want, in the
   settings. Nothing protects a block unless the user chose it (§4.8a).
   Describe KC9K by what the user's chosen settings give, never as "only
   friction" and never as more than those settings deliver. The levels:
   1. *Basic:* the user keeps `sudo` and can always stop the daemon. Locks
      only need to outlast an impulse.
   2. *Strict mode during a lock* (a setting the user turns on): no `sudo`,
      no `pkexec`, nothing runs from `/home` or `/tmp`. The user is an ordinary user, so anything an
      ordinary user can do to reach a blocked site is a real hole.
   3. *Boot and hardware:* no-escape closes the boot-time exits; a firmware
      password held by someone else, and factory reset protection, make
      other boot media and reinstalling slow and deliberate.
4. **The user can never be trapped by a bug.** There is always a recovery
   path (the `kc9k.nofreeze` boot option, recovery mode) unless the user has
   explicitly chosen no-escape.
5. **A refused attempt to weaken is ordinary use.** It is logged, never
   counted by bypass prevention (R-WEAK-5).
6. **Free, forever.** No paid tiers, no accounts, no telemetry. What KC9K
   records (events, statistics, recordings) stays on the computer.
7. **Only what improves the experience gets built.** Behaviour changes are
   options, not new defaults.
8. **Platform.** Linux Mint Cinnamon on X11. The extension runs in LibreWolf
   and in Google Chrome/Chromium. English only. An Android companion app runs
   the extension in IronFox (`android/`).
9. **One rule-maker.** Every decision about what may be weakened is made in
   the CLI; the GUI and the extension only run CLI commands or follow the
   daemon (R-WEAK-2).

---

## 3. Glossary

Command and code names are in `backticks`.

### 3.1 Targets — what can be blocked

| Term | Meaning | Not |
|---|---|---|
| **Target** | Anything a block acts on: site, URL entry, search entry, app, folder, title entry, keyword, phrase. | — |
| **Site** | A domain with all its subdomains (`reddit.com`, written without `www.`), or a wildcard domain (`*.xxx`). | A URL entry. |
| **URL entry** | A domain plus path or query (`reddit.com/r/funny`, `youtube.com/@channel`). | A site. |
| **Wildcard** | In URL entries, `*` is any text (`google.com/*q=*unicorn*`). In keywords and phrases, a plain word matches whole words and `*` matches inside a word (`*gorilla` matches "silverbackgorilla", `porn*` matches "pornstar"). | — |
| **Search entry** | A word that blocks searches containing it, on any search engine. | A keyword. |
| **Whole internet** | The target `*.*`. | — |
| **File URL** | A `file://` entry. Local files are allowed by the default exception `file://*`. | — |
| **App** | A program, matched by name, path, script, file fingerprint (`sha256:…`), Flatpak ID or desktop unit. Everything it started is closed with it. | A window. |
| **Folder entry** | A directory: every program run from inside it is a blocked app, and windows showing it are closed. | — |
| **Title entry** | A word or phrase matched against window titles. The window is asked to close, then its program is killed. In a browser only the matching tab is blocked, never the browser. | A keyword. |
| **Keyword** | A word or several words (one **term**) matched on its own, in a list that looks at the page title or the rendered, visible text of a page. | A phrase, a title entry. |
| **Phrase** | Terms from two or more **word sets** that occur together (formerly *word combination*). Rules R-PHRASE-1 to 8. Example: `[eat, nice] [hello, grandpa]` matches "Let's eat grandpa", not "Let's eat, grandpa!". | A keyword. |
| **Term** | One entry in a word set: a word or several words (`long cat`). | — |
| **Word set** | The terms one place in a phrase may be filled with. *Necessary* (a match needs a term from it) or *optional*. A phrase has at least two necessary word sets. | — |
| **Bridge** | A term written with a dash, `big-` or `-gorilla`: between `big-` and the next set's `-gorilla`, any words of the same sentence may be skipped. A term with both, `-strong-`, is a middle link. | A wildcard (`*`). |

### 3.2 Grouping

| Term | Meaning | Not |
|---|---|---|
| **Block** | A named set of rules: lists, apps, folders, titles, schedule, break, autostart, affected users. A block does nothing by itself; it acts only while **active**. | A lock. |
| **List** | A named set of entries inside one block, with a **scope** and a **kind**. | A cluster. |
| **Entry** | One item in a list: a domain, a pattern with `*`, words, or a word combination. | — |
| **Scope** | Where a list or phrase set looks: *url*, *title*, *text* (one or several). `kc9k list where`. | — |
| **Phrase set** | A named set of phrases inside one block, with a scope and its own actions, like a blacklist. A block can have any number. There are no phrase exceptions. | A list. |
| **Blacklist** | List kind: matches get the list's actions. *CT: Websites tab.* | — |
| **Exception list** | List kind: matches are spared from the blacklists **of the same block only**. *CT: Website Exceptions.* | An allowlist block. |
| **Allowlist block** | A block whose blacklist is `*.*`, so that only its exceptions are reachable. | — |
| **Cluster** | A list belonging to no block: never enforced, always freely editable, copied into blocks. | A list. |
| **Category** | A read-only list shipped with KC9K (Social, News, Adult, …), to import into blocks. *CT: Website categories.* | A cluster, a template. |
| **Default list** | The categories a new block starts with. | — |
| **Template** | A ready-made block definition to create a block from. | A category. |
| **Affected users** | The local user accounts a block applies to (default: all). | — |
| **Social preset** | A switch that keeps a site but takes a part of it away (YouTube without Shorts, Instagram messages only, …). Also: an allowed YouTube playlist. | A list. |
| **Test mode** | A block's title and text lists and its phrase sets report what they would match instead of acting. | — |

### 3.3 Block states

| Term | Meaning | Not |
|---|---|---|
| **Enabled** | Switched on: runs now if continuous, follows its schedule if scheduled. *CT: the toggle.* | Active. |
| **Disabled** | Switched off: never active, schedule ignored. | Unlocked. |
| **Continuous** | Block type: active the whole time it is enabled. | Scheduled. |
| **Scheduled** | Block type: active only inside its block windows or place rules, while enabled. | — |
| **Active** | Being enforced right now. | Enabled, locked. |
| **On break** | Active, but currently lifted by its break. | Disabled. |
| **Locked** | At least one lock is in force on the block. | Active. |
| **Held** | The block cannot be weakened right now, because a lock on it is in force (a schedule lock counts). Only locks hold a block: schedules, allow windows and place rules never do. | Active. |

### 3.4 Locks — what protects a block

A **lock** decides what it costs to **disable or weaken** a block. A lock never
changes what is blocked. *Blocks are rules; locks protect rules.*

| Term | Meaning | CT |
|---|---|---|
| **Timer lock** | No weakening until a trusted end time, given as a duration (`--for`) or a clock time (`--until`). It can always be extended. | Timer |
| **Text lock** | Weakening needs a typed challenge: random characters, random words or custom text, 1–5000 characters, paste refused. `--lock`. | Random and custom text |
| **Range lock** | Weakening is refused, or only allowed, between set times on set days. | Time range |
| **Delay lock** | Weakening starts a countdown (1 min to 40 days) and happens only when it ends. | Delay |
| **Schedule lock** | The schedule can't be changed while a window runs, nor for a lead time the user chooses before one starts. No lead time is built in. | Schedule |
| **Restart lock** | The block can be disabled only after the computer has restarted. Continuous blocks only. | Restart |
| **Password lock** | Weakening needs a password, e.g. one an accountability partner holds. | Password |
| **Layered sequential locks** | Several locks on one block, browser option or setting, lifted one at a time in the order the user chose. | — |
| **Trusted time** | The time KC9K believes despite clock changes: changing the system clock has no effect, and time powered off is not counted. All time-based locks count on it. | — |

While **any** lock is in force on any block:
uninstalling is refused, the browser extension is required, protective
settings (§4.10 *Protection*) can't be turned off, and escape routes are
closed (private or Tor windows, guest profiles, unsupported browsers).
Strict mode is not part of this: it is a setting of its own (§3.8).

### 3.5 Weakening

| Term | Meaning |
|---|---|
| **Strengthening** | A change that blocks more or makes protection stronger. |
| **Weakening** | A change that blocks less: removing or narrowing entries, removing actions, raising thresholds, shrinking windows, a looser break, disabling, deleting, removing or shortening a lock. |
| **Gate** | What a weakening must pass: *free*, *password*, *text*, *delay*, or *refused*. The locks in force decide it. |
| **Pending edit** | A weakening waiting for its delay to end. |
| **No weakening during active blocks** | A global setting: while any block is active, no browser option or setting can be weakened (R-WEAK-6). Unlocked blocks stay freely editable. |

### 3.6 Time and breaks

| Term | Meaning |
|---|---|
| **Block window** | A recurring weekly time when a scheduled block is active. Times in 5-minute steps; one block's windows never overlap. |
| **Allow window** | The reverse of a block window: a recurring time when a block that is otherwise always active is lifted, e.g. games allowed on Friday evenings only. |
| **Break** | A rule that lifts an active block for a while. One break per continuous block, or one per block window. |
| **Pomodoro** | Work (active) and rest (lifted) alternate, starting with work, with an optional long rest every N rounds. |
| **Allowance** | Blocked items may be used for N minutes per period, then the block takes effect. Period: rolling, per window, day, week or month, with a refill time. Counts only **use** (§3.7). |
| **Reward** | Rest time earned by working (being blocked) for a while. It adds up, and expires at the end of each window, day, week or month. |
| **Text break** | A break of preset length, taken after typing a challenge. |
| **Delay break** | A break of preset length, taken after a countdown. |
| **Session break** | Blocked items may be opened N times per period. A session ends a few minutes after you switch to something else. |
| **Relock** | Unlocking a locked block gives a break of set length instead of ending it; unlocking again during that break ends it. |
| **Pause** | A one-off lift of an active block until a set time. |
| **Autostart** | Re-enables a disabled block: *once* at a time, at a *time of day* on set days, at *sign-in or wake*, or at the *next block window*. |
| **Place** | A set of Wi-Fi access points. Wi-Fi only, no GPS. |
| **Place rule** | A block is active while the computer is at a place. |

### 3.7 Use

| Term | Meaning |
|---|---|
| **Use** | Time a blocked item is in the foreground (focused app or focused tab) with input in the last 3 minutes, or with media playing. |
| **Strict use counting** | Setting: background tabs and minimised apps count as use too. |

### 3.8 Device-level

| Term | Meaning | Not |
|---|---|---|
| **Frozen Chicken** | The device block: locks the screen, logs out or shuts down for a time (at most 24 h), on demand or by schedule. *CT: Frozen Turkey / Device Blocks.* | A schedule lock. |
| **Freeze mode** | What Frozen Chicken does: *lock*, *logout*, *shutdown*, or *allowed apps only*. | — |
| **Allowed apps only** | Freeze mode: the computer stays usable, but only the programs on the freeze's app list may run; every other user program is closed. | An allowlist block (websites). |
| **Monk mode** | A system-owned whole-internet block with a timer lock, started by `kc9k monk` or by the `monk` action. | An ordinary block. |
| **Consequence** | Something KC9K does to the device: close tab, close browser, lock screen, logout, shutdown, monk mode. | A page action. |
| **Strict mode** | A setting against tampering, off by default because it makes the computer less usable. While it is on and a lock is in force, it closes escape routes: `sudo` and `pkexec`, programs started from `/home` and `/tmp`, containers and VMs. The user may give it its own setting lock. | — |
| **No-escape** | Removing the boot-time exits: recovery entries, kernel options, and an unknown password on GRUB. | Strict mode. |
| **Factory reset** | Wiping or reinstalling the operating system, or booting another one, to get rid of KC9K. Software can't forbid it. | — |
| **Factory reset protection** | Options that make a factory reset slow and deliberate: firmware lock, boot order guard, installer media guard, other systems in GRUB behind the password. | — |

### 3.9 Page actions

| Term | Meaning |
|---|---|
| **Action** | What a blacklist does to a matching page: page actions and consequences. The strongest page action wins. |
| **Block page** | Replaces a blocked page, with a motivational message. |
| **Close tab / Redirect / Image** | Alternatives to the block page. |
| **Look actions** | Greyscale, no images, no video, mute, declutter, dark: they change a page, they don't block it. |
| **No downloads** | Action: downloads started from matching pages are cancelled. |
| **Threshold** | Set per action. For keywords: how many different keywords must match. For a phrase set: how many different phrases must match (default 1). |
| **Hit count** | Set per action, for keywords only: how many keyword matches in total, repeats included. Phrase sets have no hit count. |
| **Rate** | Set per action, for keywords: keyword matches ÷ rendered words, as a percentage. |
| **Ad trigger** | A fourth scope beside url, title and text: the ad networks a page loads, named by industry (gambling, adult, …). |
| **Page score** | *After publishing:* a score for what a page delivers, from its words, ads and layout; lists act on a score above a threshold. |

### 3.10 Bypass prevention

| Term | Meaning |
|---|---|
| **Bypass prevention** | Counting what KC9K stops and climbing a ladder of consequences. Its command is `kc9k nepsis` (*nepsis*, watchfulness). |
| **Event** | Something KC9K stopped or noticed, kept for a day. |
| **Detector** | An event type that counts toward escalation: `page`, `search`, `app`, `title`, `browser`, `tamper`, `access`, `workaround`. |
| **Ladder** | Escalation: N events within T trigger a consequence, step after step. Global, or per block. |
| **Step** | One rung of a ladder: "N events within T: these consequences". |
| **Quiet time** | After this long without tripping, the ladder falls back a step. |
| **Workaround** | A dodged block: mirror site, proxy address, alternative front-end, fetch tool. |
| **Tamper** | Daemon restart, clock jump. |

### 3.11 Passwords

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Needed for every weakening and every settings change. | A password lock. |
| **Access password** | Needed to *view* parts of the app (per area: app, dashboard, blocks, device, stats, settings). | A password lock. |
| **Setting lock** | A lock on one setting or browser option instead of a block. | A block lock. |

### 3.12 Recording

| Term | Meaning |
|---|---|
| **Link recording** | Collects the sites you visit into a list or cluster. |
| **Word recording** | Collects the rendered, visible words of pages you visit, with counts. |
| **Specificity** | How much more common a word is on recorded pages than in normal browsing; recorded words are ranked by it. |
| **Cut** | Dropping the lowest-ranked X % of recorded words before saving. |

### 3.13 The program

| Term | Meaning |
|---|---|
| **Daemon** | The systemd service that re-applies everything every few seconds. |
| **Layer** | One of the four ways a block is enforced: `/etc/hosts`, firewall, browser policy, closing apps. The extension works on top of them. |
| **Extension** | The bundled browser extension. Mandatory while any block needs it; browsers without it are closed. |
| **Browser option** | A lockable browser setting: `safesearch`, `force-search`, `web-only`, `force-librewolf`, `block-devtools`, `no-private`. |
| **Option** | A behaviour switch that is never held: `polite-close`, `block-page-details`. |
| **Hotkey** | A desktop key that starts a block, optionally with a lock. |
| **Project** | The whole setup in one file, minus locks, passwords, hotkeys and statistics. Import only adds. |
| **Circadian rhythm** | Settings that follow the time of day: blue light filter, brightness, wallpaper, dark mode in apps and browser. Lockable. |

### 3.14 Words to avoid

| Don't say | Say | Why |
|---|---|---|
| char lock, character lock, typing challenge (as a lock) | text lock | the challenge is what a text lock asks for |
| time lock | timer lock | |
| time range (for `--until`) | timer lock | a range lock is days and times |
| quota | allowance | |
| allowance (for earned break time) | reward | |
| edit freeze | schedule lock | its lead time is user-set |
| nepsis, bypass detection (in prose) | bypass prevention | `nepsis` is only the command name |
| rule (for a block) | block, list or entry | `rules` is only the extension's data feed |
| blocklist | blacklist | |
| word combination, page word | phrase, keyword | renamed 2026-11-13 |
| whitelist (for exceptions) | exception list | an **allowlist block** is something else |
| profile (for a block) | block, template | |
| unblock | disable, unlock, pause, weaken | each means something different |
| loosen | weaken | |
| kids, children | youth, young people | decided 2026-11-12 |
| friction, not a security boundary (for KC9K as a whole) | custom security | decided 2026-11-12; "friction" is fine for what one lock does |

---

## 4. Functions

What each part of KC9K does. The GUI and the CLI offer the same functions.

### 4.1 Overview
Shows active blocks (with locks and end times), windows still to come today,
and break status (allowance left, Pomodoro phase).

### 4.2 Blocks
Create (empty, from a template, as an allowlist block) · enable / disable ·
choose continuous or scheduled · rename · duplicate (the copy starts disabled) ·
delete (refused while locked) · set affected users · export / import all blocks.

### 4.3 Block editor
- **Add entry**: type, press Enter; the text box keeps focus for the next one.
- **Remove entry**: button or Delete/Backspace; the selection moves to the next entry.
- **Multi-select**: Ctrl/Shift-click, Select all, Ctrl+A.
- **Edit entry**: double-click moves the entry into the text box (remove + add).
- **Find entry**: Ctrl+F.
- **Save changes / Cancel**: one draft per block; nothing is applied until
  Save, which shows which changes are free, gated or delayed.
- **Import**: *From KC9K* (categories, lists of other blocks, clusters;
  several at once) or *From file* (`.txt`, one entry per line). Also *Whole
  internet* and *All searches containing…*.
- **Export list** to `.txt`.
- **Lists**: create, rename, delete, set scope, actions, threshold, hit count,
  rate, test mode.
- **Phrase sets**: create, rename, delete, set scope, actions, threshold,
  test mode, import and export. In each, "+ Create new phrase"; in each
  phrase, its word sets, each ticked necessary or optional, and the phrase
  length.
- **Apps**: add program, add folder, add title entry.
- **Social presets** and allowed playlists.

### 4.4 Clusters and recording
Clusters: create, edit, copy into a block, save from a block. Link recording
and word recording into a list or cluster; ranking and cut before saving.

### 4.5 Schedules
Weekly grid: drag to add, drag to move, click to edit; 15-minute grid, 5-minute
times; copy to other days; similar windows edited together; other blocks shown
greyed out; a break per window. Allow windows. Place rules.

### 4.6 Locks
All locks of §3.4, alone or combined, lifted in the order the user chose. A
timer lock can always be extended. The same locks protect browser options and
setting locks (the global bypass prevention ladder included).

### 4.7 Breaks and autostart
All breaks and autostart kinds of §3.6. Break status is also shown in the
browser extension's toolbar button and popup.

### 4.8 Frozen Chicken
Start for a time; schedule; both may exist together. Freeze mode lock / logout /
shutdown / allowed apps only (with its own app list).

### 4.8a No built-in protection
Nothing protects a block unless the user chose it. There is no fixed edit
freeze and no fixed delay: every lock, lead time and duration is set by the
user per block.

### 4.9 Statistics
Time used and times blocked, per site, app and title. Time frame (presets or
custom), per user, top 5 / top 10 / search, filter by block, export JSON,
delete, on/off, optionally skip private windows (except during a locked
allowance). Stored only on the computer.

### 4.10 Settings
- **Preferences**: 24-hour clock, first day of the week, theme with a darkness
  slider (dark grey to black), open at login, tray.
- **Extension**: show the install dialog when an extension is missing.
- **Notifications**: timer lock ends, window starts/ends, Pomodoro changes,
  allowance runs out, app blocked; advance warnings in minutes, comma-separated
  (`2,5`).
- **No weakening during active blocks** (R-WEAK-6).
- **Protection** (can't be turned off while locked): block unsupported browsers;
  close browsers if the extension is stopped; grace period for a missing
  extension (default 60 s); block time and date changes; block task managers;
  block inactive tabs; block embedded content; block the installer; allow
  `file://` blocking.
- **Browser options** (§3.13), each lockable on its own. SafeSearch on Google,
  Bing, DuckDuckGo and YouTube (Restricted Mode).
- **Use counting**: strict use counting.
- **Dark pages**: off / always / during a time window, with exempt sites.
- **Circadian rhythm**: per time of day (morning, afternoon, evening, night,
  at times the user sets): blue light filter, screen brightness, wallpaper,
  dark mode for apps and pages. Lockable.
- **Device protection**: strict mode, no-escape, factory reset protection
  (firmware lock, boot order guard, installer media guard, other systems in
  GRUB behind the password), with a guided setup.
- **Passwords**: settings password, access passwords.
- **Bypass prevention**: the global ladder, lockable.
- **Hotkeys**: a key starts a block, optionally with a lock.
- **Project**: export / import the whole setup.
- **Options**: polite close, block page details.

### 4.11 Command line
Every function has a command. Equivalents of Cold Turkey's parameters:

| Cold Turkey | KC9K |
|---|---|
| `-add "B" -web "URL"` | `kc9k block add B URL` |
| `-add "B" -exception "URL"` | `kc9k block add-exception B URL` |
| `-start "B"` | `kc9k start B` |
| `-start "B" -lock X` | `kc9k start B --for Xm` |
| `-start "B" -password X` | `kc9k start B --password-stdin` |
| `-start "B" -random-text X` | `kc9k start B --lock --lock-chars X` |
| `-stop "B"` | `kc9k stop B` |
| `-stop "B" -password X` | `kc9k stop B` (password on stdin) |
| `-toggle "B"` | `kc9k toggle B` |

---

## 5. Rules

**Locks**
- **R-LOCK-1** A lock protects only an enabled block; a disabled block can't be deleted while a lock is in force.
- **R-LOCK-2** Starting an active block again may only strengthen its locks; a timer lock may always be extended.
- **R-LOCK-3** A block that is not held can be enabled, disabled or edited in any direction at once (the settings password still applies), also while it is active.
- **R-LOCK-4** A timer lock refuses every weakening before its trusted end time.
- **R-LOCK-5** A text lock allows weakening only after the challenge is typed correctly; pasting is refused.
- **R-LOCK-6** A delay lock turns every weakening into a pending edit applied when its countdown ends.
- **R-LOCK-7** A restart lock can be removed only after a boot later than the lock's start.
- **R-LOCK-8** A range lock refuses weakening inside (or, inverted, outside) its ranges.
- **R-LOCK-9** While any lock is in force: uninstall refused, extension required, protective settings can't be turned off.
- **R-LOCK-10** No lock, delay or freeze applies unless the user set it on that block.
- **R-LOCK-11** Layered locks are lifted one at a time, in the user's order; adding a lock or putting one anywhere in the order is strengthening, removing one or moving it earlier is weakening.
- **R-LOCK-12** Only locks prevent weakening. Schedules, allow windows and place rules never hold a block.

**Weakening**
- **R-WEAK-1** Strengthening is never gated.
- **R-WEAK-2** Every weakening gets exactly one gate decision, made in the CLI; the GUI only runs CLI commands.
- **R-WEAK-3** Editing an entry is remove + add; it is weakening if the new entry matches less.
- **R-WEAK-4** With several locks, every gate must be passed.
- **R-WEAK-5** A refused weakening is logged and never counted as a bypass.
- **R-WEAK-6** With *no weakening during active blocks* on, no browser option or setting (strict mode included) can be weakened while any block is active; disable the active blocks first. Blocks themselves are not affected: only their own locks hold them (R-LOCK-3, R-LOCK-12).

**Schedules and breaks**
- **R-SCHED-1** An enabled scheduled block is active inside its windows; a disabled block never is.
- **R-SCHED-2** A block's windows never overlap.
- **R-SCHED-3** Pomodoro starts with work. Its rhythm is counted from midnight, except from the start time on the day a continuous block is started, and from each window's start on a scheduled block.
- **R-BREAK-1** An allowance counts only use (§3.7).

**Lists**
- **R-LIST-1** Exceptions spare pages only from their own block's blacklists.
- **R-LIST-2** Clusters and categories are never enforced.
- **R-LIST-3** Every new block has the exception `file://*`.
- **R-LIST-4** Keywords and phrases are matched against rendered, visible text only (or the title or URL, when ticked).
- **R-PHRASE-1** A phrase has at least two word sets, both necessary. Further word sets are each necessary or optional.
- **R-PHRASE-2** A match takes one term from each necessary word set, plus terms from optional sets until it reaches the phrase's **length**. The length is at least the number of necessary sets (its default) and at most the number of sets.
- **R-PHRASE-3** One word set gives at most one term to a match: with `[long cat, neko] [running, sitting]`, "long cat running" matches, and "long cat neko" doesn't: both terms come from the same set.
- **R-PHRASE-4** The terms may come in any order, and must follow each other directly, separated only by blank space. Punctuation between them breaks the match.
- **R-PHRASE-5** In the URL, every separator (`-` `_` `/` `.` `?` `=` `&` `+`) counts as blank space.
- **R-PHRASE-6** A bridge skips words between two terms: `[big-, strong] [-gorilla, cat]` matches "big and long gorilla". The skipped part must stay in one sentence (no `.` `!` `?`), and the order is fixed: `big-` before `-gorilla`.
- **R-PHRASE-7** A bridge needs both ends: `-gorilla` matches only after a `term-` of another set. `[-gorilla, cat] [jumping, river]` doesn't match "jumping gigantic gorilla". A middle link (`-strong-`) needs a phrase with at least three word sets.
- **R-PHRASE-8** Each action of a phrase set has a threshold: how many different phrases of the set must match (default 1).
- **R-TITLE-1** A title match in a browser blocks the tab, never the whole browser.

**Monk mode**
- **R-MONK-1** The monk-mode block is system-owned, and the `monk` action is not offered inside it.

**Device**
- **R-FREEZE-1** In *allowed apps only* mode, only programs on the freeze's app list (and system processes) may run.
- **R-STRICT-1** Strict mode is on only if the user turned it on. Turning it off is gated only by its own setting lock, if the user gave it one, and by R-WEAK-6.
- **R-ESCAPE-1** Until the user turns on no-escape, a recovery path always exists (principle 4).

---

## 6. Decisions

Made with the owner. Newest last.

- 2026-11-10: Linux Mint Cinnamon on X11 only. English only. Users work in
  the interface; commands exist for it and ask for access passwords too.
- 2026-11-10: Only what improves the experience gets built; behaviour changes
  are options.
- 2026-11-10: A refused attempt to weaken a held block is ordinary use, never
  counted by bypass prevention.
- 2026-11-10: Places use Wi-Fi only (no GPS). One ladder for all detectors
  (no ladder per detector).
- 2026-11-12: Windows support and the page score come after publishing.
- 2026-11-12: KC9K gets options to make a factory reset hard, as far as
  software allows.
- 2026-11-12: "Layered sequential locks" means several locks on one block,
  lifted in the order the user specified; they can be set on all locks, not
  only on blocks.
- 2026-11-12: KC9K is custom security, not "friction, not a security
  boundary".
- 2026-11-12: Fix current functionality before adding new features.
- 2026-11-12: KC9K is for youth too; the project speaks of "youth", not "kids".
- 2026-11-13: This file replaces `VISION.md` and `docs/SPEC.md`. Where they
  disagreed, the SPEC's terms and features were kept; what only `VISION.md`
  had (goal, principles, circadian rhythm, ads, no downloads, SafeSearch
  beyond Google, factory reset protection, layered locks, hotkeys, project,
  browser options, relock) was added.
- 2026-11-13: Nothing protects a block unless the user chose it: no fixed
  edit freeze, no fixed loosening delay (§4.8a, R-LOCK-10).
- 2026-11-13: Where the code contradicts this file, the code is fixed.
- 2026-11-13: Only locks prevent weakening a block, always (R-LOCK-12). A
  block in a schedule window, with allow windows or at a place can be
  weakened freely unless a lock is on it.
- 2026-11-13: Strict mode is a voluntary setting against tampering, with
  its own setting lock (R-STRICT-1).
- 2026-11-13: An unlocked block can always be enabled, disabled or edited
  freely, also while it is active.
- 2026-11-13: A global setting *no weakening during active blocks*
  (R-WEAK-6): with it on, a block must be disabled before, for example,
  strict mode can be turned off. With it off, the user decides whether
  strict mode gets a lock of its own. The setting doesn't touch blocks,
  and turning it off is itself refused while a block is active.
- 2026-11-13: Word combinations are renamed phrases and leave ordinary
  lists; blocks get phrase sets, each with its own actions (R-PHRASE-1 to
  8). Thresholds are per action; keyword lists also have hits and rate,
  phrase sets have neither hits nor phrase exceptions. Bridges are written
  with a dash; `*` keeps matching inside a word.

## 7. Open decisions

Contradictions not yet settled by the owner. Don't resolve them alone.

None right now.
