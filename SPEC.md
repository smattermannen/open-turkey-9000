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

**The circadian rhythm.** KC9K follows the time of day: dark mode in the
browser and in apps, a blue light filter, screen brightness, and wallpapers. A bright and positive wallpaper in the morning, a
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
2. **A locked block can't be weakened; an unlocked block is free.** Only
   locks protect a block (R-LOCK-1). Strengthening never needs a lock to be
   unlocked. When a settings password is set, it is also needed to weaken
   anything, and to set a lock if the user chose that (§3.11).
3. **Custom security.** The user makes KC9K as secure as they want, in the
   settings. Nothing protects a block unless the user chose it: no built-in
   locks, delays or lead times (R-LOCK-10). Describe KC9K by what the user's
   chosen settings give, never as "only friction" and never as more than
   those settings deliver. The levels:
   1. *Basic:* the user keeps `sudo` and can always stop the daemon. Locks
      only need to outlast an impulse.
   2. *Strict mode* (§3.8): while a block is locked, the user is an ordinary
      user, so anything an ordinary user can do to reach a blocked site is a
      real hole.
   3. *Boot and hardware:* no-escape closes the boot-time exits; a firmware
      password held by someone else, and factory reset protection, make
      other boot media and reinstalling slow and deliberate.
4. **The user can never be trapped by a bug.** There is always a recovery
   path (the `kc9k.nofreeze` boot option, recovery mode) unless the user has
   explicitly chosen no-escape.
5. **A refused attempt to weaken is ordinary use.** It is logged, never
   counted by bypass prevention (R-WEAK-3).
6. **Free, forever.** No paid tiers, no accounts, no telemetry. What KC9K
   records (events, statistics, recordings) stays on the computer, and what
   it has to ask other computers (such as DNS servers) is said plainly.
7. **Only what improves the experience gets built.** Changes in behaviour
   are settings, not new defaults.
8. **Platform.** Linux Mint Cinnamon on X11. The extension runs in LibreWolf
   and in Google Chrome/Chromium. English only. An Android companion app runs
   the extension in IronFox (`android/`).
9. **Browsing stays fast.** KC9K must never make browsing feel slower.
   However many lists, rendered terms and phrases a user adds, reading and
   judging a page happens in the background, without holding up scrolling,
   typing or page loads. KC9K is kind to the hardware:
   - no single piece of KC9K's work in a page takes longer than 16 ms (one
     screen frame);
   - judging a typical page with 1,000 rendered terms and 100 phrases takes
     under 50 ms in total, measured on the test laptop;
   - an address is judged before its page loads; page text within 0.5 s of
     appearing;
   - checking an address costs the same with 10 entries or 100,000;
   - a gigantic list never slows the network or the computer.
10. **One rule-maker.** Every decision about what may be weakened is made in
    the CLI; the GUI and the extension only run CLI commands or follow the
    daemon.

---

## 3. Glossary

Command and code names are in `backticks`.

### 3.1 Targets — what can be blocked

| Term | Meaning | Not |
|---|---|---|
| **Target** | Anything a block acts on: site, URL entry, search entry, app, folder, window title entry, rendered term, phrase. | — |
| **Site** | A domain with all its subdomains (`reddit.com`, written without `www.`), or a wildcard domain (`*.xxx`). | A URL entry. |
| **URL entry** | A domain plus path or query (`reddit.com/r/funny`, `youtube.com/@channel`). | A site. |
| **Wildcard** | In sites and URL entries, `*` is any text (`google.com/*q=*unicorn*`). In rendered terms and phrases, a plain word matches whole words and `*` matches inside a word (`*gorilla` matches "silverbackgorilla", `porn*` matches "pornstar"). | A bridge. |
| **Search entry** | A word in a search list: searches containing it are blocked. | A rendered term. |
| **Whole internet** | The target `*.*`: every website. Local files (`file://`), this computer (localhost) and the local network always stay reachable. | — |
| **App** | A program, matched by name, path, script, file fingerprint (`sha256:…`), Flatpak ID or desktop unit. Everything it started is closed with it. | A window. |
| **Folder entry** | A directory: every program run from inside it is a blocked app, and windows showing it are closed. | — |
| **Window title entry** | A word or several words matched against the titles of app windows. Never applies to browsers: a tab's title is judged by term lists and phrase sets. | A rendered term. |
| **Rendered term** | A word or several words (one **term**), matched on its own in a term list. | A phrase. |
| **Phrase** | Terms from two or more **word sets** that occur together (formerly *word combination*). R-PHRASE-1 to 10. Example: `[eat, nice] [hello, grandpa]` matches "Let's eat grandpa", not "Let's eat, grandpa!". | A rendered term. |
| **Term** | One entry in a word set or term list: a word or several words (`long cat`). | — |
| **Word set** | The terms one place in a phrase may be filled with. *Necessary* (a match needs a term from it) or *optional*. | — |
| **Phrase length** | How many terms a match of a phrase needs. At least the number of necessary word sets (the default), at most the number of word sets. | — |
| **Bridge** | A term written with a dash, `big-` or `-gorilla`: between a `big-` and a `-gorilla` of another word set, up to 20 words of the same sentence may be skipped, or none. A term with both, `-strong-`, is a middle link. | A wildcard. |

### 3.2 Grouping

| Term | Meaning | Not |
|---|---|---|
| **Block** | A named group of lists, phrase sets, apps, folders, window title entries, schedule, breaks, autostart, affected users and locks. | — |
| **List** | A named set of entries inside one block. It has a **type** (address, term or search) and a **kind** (blacklist or exception list). | A cluster. |
| **Address list** | List type: sites and URL entries, matched against a page's address. | — |
| **Term list** | List type: rendered terms, matched where its scope says. | — |
| **Search list** | List type: search entries, matched against searches on any search engine and in a site's own search. | — |
| **Entry** | One item in a list: a site, a URL entry, a rendered term or a search entry. | — |
| **Scope** | Where a term list or phrase set looks: *url*, *title* (the tab's), *text* (one or several). | — |
| **Phrase set** | A named set of phrases inside one block, with a scope and its own actions, like a blacklist. A block can have any number. | A list. |
| **Blacklist** | List kind: matches get the list's actions. *CT: Websites tab.* | — |
| **Exception list** | List kind: matches are spared from the blacklists and phrase sets **of the same block only**. There are no phrase exceptions. *CT: Website Exceptions.* | An allowlist block. |
| **Allowlist block** | A block whose blacklist is `*.*`, so that only its exceptions are reachable. | — |
| **Cluster** | A list or phrase set belonging to no block: never enforced, always freely editable, copied into blocks. | A list. |
| **Template** | A ready-made block definition to create a block from. | A cluster. |
| **Affected users** | The local user accounts a block applies to (default: all). For shared computers; only an administrator sets them. | — |
| **Social preset** | A switch that keeps a site but takes a part of it away (YouTube without Shorts, Instagram messages only, …). Also: an allowed YouTube playlist. | A list. |
| **Test mode** | A term list or phrase set reports what it would match instead of acting. Set per list and per phrase set. | — |

### 3.3 Block states

A block is **active** or **inactive**, and at any moment **enabled** or
**disabled**.

| Term | Meaning | Not |
|---|---|---|
| **Active** | Switched on by the user (`kc9k start`): it enforces whenever its type, schedule and breaks say so. | Enabled. |
| **Inactive** | Switched off (`kc9k stop`): never enforces, schedule ignored. | Unlocked. |
| **Enabled** | Enforcing right now. | Active. |
| **Disabled** | Not enforcing right now. | Inactive. |
| **Lifted** | Active but disabled: outside its block windows or place, inside an allow window, on a break, paused, in a Pomodoro rest or in allowance time. | Inactive. |
| **Continuous** | Block type: enabled the whole time it is active, except during its allow windows and breaks. | Scheduled. |
| **Scheduled** | Block type: enabled only inside its block windows or at its places, while active. | — |
| **Locked** | At least one of its locks is on. A locked block can't be weakened. | Active. |

A block's enforcing ends when its schedule says so or when it is switched
off. A lock never ends a block, and never starts one.

### 3.4 Locks — what protects a block

A **lock** belongs to a block. While it is **on**, the block can't be
weakened, and so can't be switched off or deleted. It is set when the user
creates it and is **unlocked** (lifted) as its kind says; then it is gone,
unless it has re-lock. A lock never changes what is blocked.

| Term | Meaning | CT |
|---|---|---|
| **Timer lock** | On until a date and time (`--until`), or for a duration (`--for`). It can always be extended. There is no upper limit. | Timer |
| **Text lock** | Unlocked by typing a challenge: random characters, random words or custom text, 1–5000 characters, paste refused. | Random and custom text |
| **Range lock** | On between set times on set days (or, inverted, outside them). | Time range |
| **Delay lock** | Asking to unlock starts a countdown (1 min to 40 days); the lock is unlocked when it ends. | Delay |
| **Schedule lock** | On during the block's windows, at its places, and for a lead time the user chooses before a window. For a block with allow windows: on outside them. No lead time is built in. | Schedule |
| **Restart lock** | Unlocked when the computer has restarted. Continuous blocks only. | Restart |
| **Password lock** | Unlocked by a password, e.g. one an accountability partner holds. | Password |
| **Re-lock** | An option of a lock: once unlocked, the lock comes on again after a time the user sets, counted from the unlocking. | — |
| **Layered sequential locks** | Several locks on one block, browser setting or setting, unlocked one at a time in the order the user chose. | — |
| **Trusted time** | The real date and time, as protected against changes by the tamper settings the user turned on (§4.10). All locks use it. | — |

While any block or setting is locked: uninstalling is refused, the browser
extension is required, and protection settings (§4.10) can't be turned off.

### 3.5 Weakening

| Term | Meaning |
|---|---|
| **Strengthening** | A change that clearly blocks more or protects more. |
| **Weakening** | Every other change: removing or narrowing entries, adding exceptions, removing actions, raising thresholds, shrinking windows, widening allow windows, a looser break, test mode on, a social preset off, changing a redirect's target, pausing, switching off, deleting, uninstalling, removing or shortening a lock. |
| **No weakening during active locks** | A global setting: while any block or setting is locked, no setting can be weakened (strict mode included). Unlocked blocks stay freely editable. |

### 3.6 Time and breaks

| Term | Meaning |
|---|---|
| **Block window** | A recurring weekly time when a scheduled block is enabled. Times in 5-minute steps; one block's windows never overlap. |
| **Allow window** | The reverse of a block window: a recurring time when a continuous block is lifted, e.g. games allowed on Friday evenings only. |
| **Break** | A rule that lifts an enabled block for a while. One break per continuous block, or one per block window. |
| **Pomodoro** | Work (enabled) and rest (lifted) alternate, starting with work, with an optional long rest every N rounds. |
| **Allowance** | Blocked items may be used for N minutes per period, then the block is enabled. Period: rolling (the last 24 hours), per window, day, week or month, with a refill time. Counts only **use** (§3.7). |
| **Reward** | Rest time earned by working (blocked, with the computer in use) for a while. It adds up, and expires at the end of each window, day, week or month. |
| **Text break** | A break of preset length, taken after typing a challenge. The block stays locked. |
| **Delay break** | A break of preset length, taken after a countdown. The block stays locked. |
| **Session limit** | Blocked items may be opened N times per period. A session ends 3 minutes after you switch to something else. |
| **Pause** | A one-off lift of an enabled block until a set time. |
| **Autostart** | Switches an inactive block on: *once* at a time, at a *time of day* on set days, at *sign-in or wake*, or at the *next block window*. |
| **Place** | A set of Wi-Fi access points. Wi-Fi only, no GPS. |
| **Place rule** | A scheduled block is enabled while the computer is at a place. |

### 3.7 Use

| Term | Meaning |
|---|---|
| **Use** | Time a blocked item is in the foreground (focused app or focused tab) with input in the last 3 minutes, or plays sound, also in a background tab. |
| **Strict use counting** | Setting: background tabs and minimised apps count as use too. |

### 3.8 Device-level

| Term | Meaning | Not |
|---|---|---|
| **Frozen Chicken** | The device block: in its freeze mode, for a time (at most 24 h), on demand or by schedule. A running freeze can't be ended. A scheduled freeze can have locks like a block. *CT: Frozen Turkey / Device Blocks.* | A schedule lock. |
| **Freeze mode** | What Frozen Chicken does: *lock screen*, *logout*, *shutdown*, or *allowed apps only*. | — |
| **Allowed apps only** | Freeze mode: the computer stays usable, but only the programs on the freeze's app list may run; every other user program is closed. | An allowlist block. |
| **Monk mode** | A **mode**, not a block, and fully preconfigured: nothing in it can be edited or added. For a set time it blocks every website (`*.*`, with only the default exception `file://*`) and cuts the internet connection of the whole computer, so no program can reach the network (for example a game on Steam). Started by `kc9k monk`, a list action or a ladder step; it ends only when its time ends, and can't be switched off early. | A block, ordinary or system-owned. |
| **Consequence** | Something KC9K does to the device: close tab, close browser, lock screen, logout, shutdown, monk mode. Announced 60 s ahead, except from bypass prevention (R-WARN-1, R-WARN-2). | A page action. |
| **Strict mode** | A setting against tampering, off by default because it makes the computer less usable. While it is on and a block is locked: no `sudo` or `pkexec`; nothing runs from `/home` or `/tmp` except from folders the user exempted (adding one is a weakening); no containers or VMs. It can have its own setting lock. | — |
| **No-escape** | Closes the boot-time exits: hides recovery entries and kernel options, and puts a password nobody knows on GRUB. | Strict mode. |
| **Factory reset** | Wiping or reinstalling the operating system, or booting another one, to get past KC9K. Software can't forbid it. | — |
| **Factory reset protection** | Settings that make a factory reset slow and deliberate: firmware lock, boot order guard, installer media guard, other systems in GRUB behind the password. | — |

### 3.9 Actions

| Term | Meaning |
|---|---|
| **Action** | What a blacklist, phrase set, app or window title entry does on a match: a page action or a consequence, wherever it makes sense. |
| **Block page** | Replaces a blocked page, with a motivational message. |
| **Close tab / Redirect / Image** | Alternatives to the block page. |
| **Look actions** | Greyscale, no images, no video, mute, declutter, dark: they change a page, they don't block it. They add up. |
| **No downloads** | Action: downloads started from matching pages are cancelled. |
| **Strongest action** | When several actions apply, from any blocks: a consequence first, then close tab, image, redirect, block page. Look actions apply only when none of these does. |
| **Threshold** | Set per action. For a term list: how many different rendered terms must match. For a phrase set: how many different phrases must match (default 1). |
| **Hit count** | Set per action, for term lists only: how many matches in total, repeats included. |
| **Rate** | Set per action, for term lists only: matches ÷ words on the page, as a percentage. Counted only on pages of at least 200 words. |
| **Ad trigger** | A planned fourth scope beside url, title and text: the ad networks a page loads, named by industry (gambling, adult, …). |
| **Page score** | *After publishing:* a score for what a page delivers, from its words, ads and layout; lists act on a score above a threshold. |

### 3.10 Bypass prevention

| Term | Meaning |
|---|---|
| **Bypass prevention** | Counting what KC9K stops and climbing a ladder of consequences. Its command is `kc9k bypass`. |
| **Event** | Something KC9K stopped or noticed, kept for a day (R-EVENT-1). |
| **Detector** | An event type a ladder counts, chosen by the user: `page`, `search`, `app`, `title`, `browser`, `tamper`, `access` (a wrong access password), `workaround`. |
| **Ladder** | Escalation: N events within T trigger consequences, step after step. Global, or per block. |
| **Step** | One rung of a ladder: "N events within T: these consequences". |
| **Quiet time** | After this long without tripping, the ladder falls back a step. |
| **Workaround** | A dodged block: mirror site, proxy address, alternative front-end, fetch tool. |
| **Tamper** | A clock jump, or the daemon restarting for any reason but an upgrade, a normal shutdown or a crash KC9K reports itself. |

### 3.11 Passwords

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Optional. When set, it is needed to weaken anything. The user can also choose to require it for setting a lock, so nobody else can lock them out. | A password lock. |
| **Access password** | Optional, needed to *view* parts of the app (per area: app, dashboard, blocks, device, stats, settings). | A password lock. |
| **Setting lock** | A lock on one setting or browser setting instead of a block. | A block's lock. |

A forgotten settings password or password lock can be removed from recovery
mode, unless no-escape is on.

### 3.12 Recording

| Term | Meaning |
|---|---|
| **Link recording** | Collects the sites you visit into an address list or cluster. |
| **Word recording** | Collects the rendered, visible words of pages you visit, with counts. |
| **Specificity** | How much more common a word is on recorded pages than in normal browsing; recorded words are ranked by it. |
| **Cut** | Dropping the lowest-ranked X % of recorded words before saving. |

### 3.13 The program

| Term | Meaning |
|---|---|
| **Daemon** | The systemd service that re-applies everything every few seconds. |
| **Layer** | One of the four ways a block is enforced: `/etc/hosts`, firewall, browser policy, closing apps. The extension works on top of them. |
| **Extension** | The bundled browser extension. Required while any block is locked or uses websites; browsers without it are closed. |
| **Setting** | Anything the user configures outside blocks. Kinds: **browser setting** (lockable: `safesearch`, `force-search`, `web-only`, `force-librewolf`, `block-devtools`, `no-private`), **protection setting** (§4.10), **behaviour setting** (never locked, not touched by *no weakening during active locks*: `polite-close`, `block-page-details`), **preference** (the interface only). |
| **Hotkey** | A desktop key that switches a block on, optionally setting a lock. |
| **Project** | The whole setup in one file. *Backup*: everything. *Share*: without places and user names. Import shows a preview, only adds, and gives a block that exists already a new name. |
| **Circadian rhythm** | Settings that follow the time of day: the screen's brightness and colour temperature (each an **automation clip**: a 24-hour curve of points), wallpaper, dark mode in apps and pages. Never locked, and it never overrides the user. |

### 3.14 Words to avoid

| Don't say | Say | Why |
|---|---|---|
| char lock, character lock, typing challenge (as a lock) | text lock | the challenge is what a text lock asks for |
| time lock, time range (for `--until`) | timer lock | a range lock is days and times |
| held, in force | locked, on | |
| edit freeze | schedule lock | |
| relock (as a break) | text break | **re-lock** is a lock option |
| quota | allowance | |
| allowance (for earned break time) | reward | |
| session break | session limit | |
| option (for `polite-close` …), browser option | behaviour setting, browser setting | |
| nepsis, bypass detection | bypass prevention | the command is `kc9k bypass`; *Nepsis* may become the app's name (§7) |
| rule (for a block or list) | block, list, entry | "rule" is only for rule IDs, place rules and the extension's `rules` feed |
| blocklist | blacklist | |
| word combination, page word, keyword | phrase, rendered term | |
| whitelist (for exceptions) | exception list | an **allowlist block** is something else |
| profile (for a block) | block, template | |
| unblock | switch off, unlock, pause, weaken | each means something different |
| loosen | weaken | |
| kids, children | youth, young people | |
| friction, not a security boundary (for KC9K as a whole) | custom security | "friction" is fine for what one lock does |

---

## 4. Functions

What each part of KC9K does. The GUI and the CLI offer the same functions.

### 4.1 Overview
Shows active blocks (enabled or lifted, with their locks), windows still to
come today, break status (allowance left, Pomodoro phase) and where each
ladder stands.

### 4.2 Blocks
Create (empty, from a template, as an allowlist block) · switch on / off ·
choose continuous or scheduled · rename · duplicate (the copy starts
inactive) · delete (refused while locked) · set affected users · export /
import.

### 4.3 Block editor
- **Add entry**: type, press Enter; the text box keeps focus for the next one.
- **Remove entry**: button or Delete/Backspace; the selection moves to the next entry.
- **Multi-select**: Ctrl/Shift-click, Select all, Ctrl+A.
- **Edit entry**: double-click moves the entry into the text box (remove + add).
- **Find entry**: Ctrl+F.
- **Save changes / Cancel**: one draft per block; nothing is applied until
  Save. On a locked block, Save refuses the weakening changes and says which.
- **Import**: *From KC9K* (lists and phrase sets of other blocks,
  clusters; several at once) or *From file* (`.txt`, one entry per line).
  Also *Whole internet* and *All searches containing…*.
- **Export** a list or phrase set to `.txt`.
- **Lists**: create (address, term or search; blacklist or exception list),
  rename, delete, scope, actions, threshold, hit count, rate, test mode.
- **Phrase sets**: create, rename, delete, scope, actions, threshold, test
  mode. In each, "+ Create new phrase"; in each phrase, its word sets, each
  ticked necessary or optional, and the phrase length.
- **Apps**: add program, add folder, add window title entry; their actions.
- **Social presets** and allowed playlists.
- **Bypass prevention**: the block's own ladder.

### 4.4 Clusters and recording
Clusters: create, edit, copy into a block, save from a block. Link recording
into an address list or cluster; word recording into a term list or
cluster, ranked, with a cut before saving.

### 4.5 Schedules
Weekly grid: drag to add, drag to move, click to edit; 15-minute grid, 5-minute
times; copy to other days; similar windows edited together; other blocks shown
greyed out; a break per window. Allow windows. Place rules.

### 4.6 Locks
All locks of §3.4, alone or layered, on blocks, browser settings and
settings (the global ladder included). The order of layered locks can't be
changed while the block is locked; a new lock goes last. A timer lock can
always be extended.

### 4.7 Breaks and autostart
All breaks and autostart kinds of §3.6. Break status is also shown in the
browser extension's toolbar button and popup.

### 4.8 Frozen Chicken
Start for a time; schedule; both may exist together. Freeze modes as in §3.8.

### 4.9 Statistics
Time used and times blocked, per site, app and title. Time frame (presets or
custom), per user, top 5 / top 10 / search, filter by block, export JSON,
delete, on/off, optionally skip private windows. Kept apart from events, and
only on the computer.

### 4.10 Settings
- **Preferences**: 24-hour clock, first day of the week, theme with a darkness
  slider (dark grey to black), open at login, tray.
- **Extension**: show the install dialog when an extension is missing.
- **Notifications**: timer lock ends, window starts/ends, Pomodoro changes,
  allowance runs out, app blocked; advance warnings in minutes, comma-separated
  (`2,5`).
- **No weakening during active locks** (R-WEAK-4).
- **Protection settings** (can't be turned off while any block or setting is
  locked):
  - close private windows and Tor; close guest profiles; block unsupported
    browsers; close browsers if the extension is stopped; grace period for a
    missing extension (default 60 s); allow `file://` blocking;
  - *block the installer:* while anything is locked, KC9K can't be
    reinstalled, downgraded or removed, through its own installer or the
    package manager;
  - *block inactive tabs:* when a block is enabled, blocked sites in
    background tabs are blocked at once, not only when shown;
  - *block embedded content:* a blocked site shown inside another page is
    blocked too;
  - *block task managers:* while anything is locked, programs that can end
    other programs (System Monitor, htop) are closed.
- **Tamper protection** (each optional): block time and date changes; check
  the clock against the hardware clock and internet time.
- **Browser settings** (§3.13), each lockable on its own. SafeSearch on Google,
  Bing, DuckDuckGo and YouTube (Restricted Mode). *force-search:* Google is
  the only search engine; others are blocked and the browser's search box
  uses Google. *web-only:* Google shows web results only, without images,
  shopping or AI answers.
- **DNS**: which server KC9K asks for blocked sites' real addresses (needed
  only for the firewall layer). By default the network's own DNS server, the
  one the computer already uses, so nobody new learns which sites are
  blocked; Quad9 when that one can't answer.
- **Use counting**: strict use counting.
- **Circadian rhythm** (owner's decisions, 2026-11-14):
  - *Automation clips*, one set for every day: a 24-hour curve each for the
    backlight (the screen's real brightness, %), software brightness (%)
    and colour temperature (the blue light filter, in kelvin). The user
    adds, moves and removes points and bends the curve between two points;
    ready-made presets to start from.
  - *Wallpapers*: the user picks a picture and the times between which it
    shows. There are no fixed periods (morning, afternoon, …).
  - *Dark mode* is the system's dark mode, on or off, like a phone's: left
    to the system, always light, always dark, or light between two times.
  - *Dark pages* follow dark mode, with exempt sites.
  - *Not lockable.* Nothing of the circadian rhythm can be locked, and no
    lock holds it.
  - *It never overrides the user.* KC9K never puts a level back. When the
    user changes a level by hand, that clip pauses until its next point, or
    until the user resumes it. A wallpaper or dark mode the user picks by
    hand stays until the next change the settings call for.
- **Device protection**: strict mode, no-escape, factory reset protection,
  with a guided setup.
- **Passwords**: settings password, access passwords.
- **Bypass prevention**: the global ladder, lockable.
- **Hotkeys**: a key switches a block on, optionally setting a lock.
- **Project**: backup, share, import.
- **Behaviour settings**: polite close, block page details.

### 4.11 Install and update
A `.deb` package. Automatic updates are out of scope for now. Uninstalling is
refused while any block or setting is locked.

### 4.12 Command line
Every function has a command. Equivalents of Cold Turkey's parameters:

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

Command and flag names follow this glossary (`--text-lock`, `list scope`);
old names keep working for a while.

---

## 5. Rules

**Locks**
- **R-LOCK-1** Only locks protect a block. A locked block can't be weakened; an unlocked block can be switched on, off or edited in any direction at once, also while enabled.
- **R-LOCK-2** A lock is on until it is unlocked as its kind says (§3.4); then it is gone, unless it has re-lock.
- **R-LOCK-3** A locked block can't be deleted or switched off.
- **R-LOCK-4** A timer lock can always be extended, never shortened.
- **R-LOCK-5** A text lock's challenge refuses pasting.
- **R-LOCK-6** Layered locks are unlocked one at a time, in the user's order: a lock can be unlocked only once the one before it is. A timer lock with a date needs that date to have come; one with a duration counts from when the lock before it was unlocked.
- **R-LOCK-7** The order of layered locks can't change while the block is locked; a new lock goes last.
- **R-LOCK-8** A restart lock is unlocked by a boot later than the lock's start.
- **R-LOCK-9** While any block or setting is locked: uninstalling is refused, the extension is required, protection settings can't be turned off.
- **R-LOCK-10** No lock, delay or lead time applies unless the user set it.
- **R-LOCK-11** A lock never starts by itself, except through re-lock.
- **R-LOCK-12** Every lock, of every kind and on everything it can protect (block, browser setting, setting, scheduled freeze, monk mode), asks the user to confirm when it is set, always: the confirmation shows what is locked, the kind and, where it has one, how long. Nothing is locked until the user confirms. This holds for the GUI and the CLI alike, and for a lock set by a hotkey or a preset.

**Weakening**
- **R-WEAK-1** Strengthening never needs a lock unlocked. Setting a lock needs the settings password only if the user chose that.
- **R-WEAK-2** Every decision about weakening is made in the CLI; the GUI only runs CLI commands.
- **R-WEAK-3** A refused weakening is logged and never counted as a bypass.
- **R-WEAK-4** With *no weakening during active locks* on, no setting (strict mode included) can be weakened while any block or setting is locked. Turning it off is itself refused then.
- **R-WEAK-5** Editing an entry is remove + add: weakening unless the new entry clearly matches more.

**Schedules and breaks**
- **R-SCHED-1** An active scheduled block is enabled inside its windows and at its places; an inactive block never is.
- **R-SCHED-2** A block's windows never overlap.
- **R-SCHED-3** Pomodoro starts with work, and its rhythm starts again each time the block is enabled.
- **R-BREAK-1** An allowance counts only use (§3.7).
- **R-BREAK-2** A reward is earned only while the computer is in use.

**Lists and phrases**
- **R-LIST-1** Exceptions spare pages only from their own block's blacklists and phrase sets.
- **R-LIST-2** Clusters are never enforced.
- **R-LIST-3** Every new block has the exception `file://*`.
- **R-LIST-4** Text means rendered, visible text: hidden or collapsed text doesn't count; text further down the page does. Each frame is judged on its own text.
- **R-LIST-5** Upper and lower case and accents are ignored (é = e); look-alike letters from other alphabets count as their Latin letters. Languages without spaces between words are out of scope.
- **R-LIST-6** Every count set on an action (threshold, hit count, rate) must be met.
- **R-PHRASE-1** A phrase has at least two word sets; the first two are necessary, further ones necessary or optional.
- **R-PHRASE-2** A match takes one term from each necessary word set, plus terms from optional sets until it reaches the phrase length.
- **R-PHRASE-3** One word set gives at most one term to a match: with `[long cat, neko] [running, sitting]`, "long cat running" matches, and "long cat neko" doesn't.
- **R-PHRASE-4** The terms may come in any order, and must follow each other directly, separated only by blank space. Punctuation between them breaks the match. An apostrophe inside a word is part of it ("let's"); a hyphen between letters counts as blank space, so `long cat` matches "long-cat".
- **R-PHRASE-5** A break between parts of a page (heading, paragraph, list item, button) ends a sentence; phrases and bridges don't cross it.
- **R-PHRASE-6** In the URL, the address is decoded first (`%20` is a space), and every separator (`-` `_` `/` `.` `?` `=` `&` `+` `#` `:` `,`) counts as blank space.
- **R-PHRASE-7** A bridge skips up to 20 words, or none, between two terms of different word sets: `[big-, strong] [-gorilla, cat]` matches "big and long gorilla" and "big gorilla". The skipped part may hold commas but stays in one sentence (no `.` `!` `?`), and the order is fixed: `big-` before `-gorilla`.
- **R-PHRASE-8** A bridge needs both ends: `-gorilla` matches only after a `term-` of another word set, so `[-gorilla, cat] [jumping, river]` doesn't match "jumping gigantic gorilla". A phrase with a one-ended bridge is refused when entered. A middle link (`-strong-`) needs at least three word sets.
- **R-PHRASE-9** Each action of a phrase set has a threshold: how many different phrases of the set must match (default 1). Phrase sets have no hit count and no rate.
- **R-PHRASE-10** Phrase sets can be exported, imported and kept as clusters; they take no part in recording.
- **R-TITLE-1** Window title entries never apply to browsers.

**Bypass prevention**
- **R-EVENT-1** At most one event per tab and blocked site per minute. Tabs restored when the browser starts, preloaded pages and embedded frames don't count.

**Monk mode**
- **R-MONK-1** Monk mode is a mode, not a block: it is not in the list of blocks and has no settings. It blocks `*.*` with the default exception `file://*` only, takes no exceptions, apps or other entries, and the `monk` action is not offered in it.
- **R-MONK-2** While monk mode runs, the computer's internet connection is cut, not only the browser's: no program reaches the network. It is cut three ways at once, so one failing doesn't open it: (1) the Wi-Fi radio is switched off, (2) every wired (Ethernet) interface is brought down, (3) the firewall drops all traffic except loopback. KC9K's own local traffic (the daemon and the extension on this computer) keeps working. Monk mode keeps all three held for its whole time, putting back anything the user switches on. When it ends, or after a restart or crash once its time is over, the connection is restored to what it was before.

**Device**
- **R-FREEZE-1** In *allowed apps only* mode, only programs on the freeze's app list (and system processes) may run.
- **R-FREEZE-2** A running freeze can't be ended; removing a scheduled freeze is refused while it is locked.
- **R-WARN-1** Logout, shutdown and a freeze started by a list action or a schedule are announced 60 s before they happen.
- **R-WARN-2** Bypass prevention gives no countdown: it notifies at every event and every step ("Event triggered, be careful"), and acts at once, up to an immediate shutdown if the user's ladder says so.
- **R-STRICT-1** Strict mode is on only if the user turned it on. Turning it off is refused only by its own setting lock, if it has one, and by R-WEAK-4.
- **R-ESCAPE-1** Until the user turns on no-escape, a recovery path always exists (principle 4).
- **R-DESKTOP-1** On Wayland or a desktop other than Cinnamon, KC9K warns that window titles and app time don't work; everything else works.

---

## 6. Decisions

Made with the owner. Newest last.

- 2026-11-10: Linux Mint Cinnamon on X11 only. English only. Users work in
  the interface; commands exist for it and ask for access passwords too.
- 2026-11-10: Only what improves the experience gets built; behaviour changes
  are settings.
- 2026-11-10: A refused attempt to weaken a locked block is ordinary use,
  never counted by bypass prevention.
- 2026-11-10: Places use Wi-Fi only (no GPS). One ladder for all detectors
  (no ladder per detector).
- 2026-11-12: Windows support and the page score come after publishing.
- 2026-11-12: KC9K gets settings to make a factory reset hard, as far as
  software allows.
- 2026-11-12: "Layered sequential locks" means several locks on one block,
  unlocked in the order the user specified; they can be set on all locks,
  not only on blocks.
- 2026-11-12: KC9K is custom security, not "friction, not a security
  boundary".
- 2026-11-12: Fix current functionality before adding new features.
- 2026-11-12: KC9K is for youth too; the project speaks of "youth", not "kids".
- 2026-11-13: This file replaces `VISION.md` and `docs/SPEC.md`.
- 2026-11-13: Where the code contradicts this file, the code is fixed.
- 2026-11-13: Only locks protect a block. A locked block can't be weakened;
  an unlocked block can always be switched on, off or edited freely. No
  built-in locks, delays or lead times.
- 2026-11-13: Strict mode is a voluntary setting against tampering, with
  its own setting lock. With *no weakening during active locks* on, it
  can't be turned off while anything is locked.
- 2026-11-13: Phrases (formerly word combinations) and phrase sets, with
  bridges, as in R-PHRASE-1 to 10; "keyword" is renamed rendered term.
- 2026-11-13: Browsing must stay fast, with the limits in principle 9.
- 2026-11-13: The lock model, like Cold Turkey's: a lock belongs to the
  block and is unlocked as its kind says; layered locks one after another;
  re-lock is an option of a lock; a lock never ends a block. Locks follow the
  real clock, with optional tamper protection. Blocks are active or inactive
  (switched on or off), and enabled or disabled (enforcing or not); lifted
  is active but disabled.
- 2026-11-13: The answers to the 65 questions on the review of this file
  (lists by type, matching details, events, warnings, project export,
  settings words and more), written into §3–§5.
- 2026-11-13: Requiring the settings password for setting a lock is the
  user's choice. Bypass prevention warns at every event and step, without a
  countdown, and acts at once. The DNS default is the network's own server,
  then Quad9. `kc9k nepsis` is renamed `kc9k bypass`.
- 2026-11-13: Definitions of the protection settings, `force-search` and
  `web-only` (§4.10).
- 2026-11-14: Monk mode is a mode, not a block. As a block it could be edited
  or deleted while unlocked, which would defeat it. All locks ask for
  confirmation when set, always (R-LOCK-12), not only timer locks over 24 h.
- 2026-11-14: Monk mode is fully preconfigured (`*.*`, exception `file://*`
  only, nothing editable) and also cuts the internet connection (Wi-Fi off,
  Ethernet down, firewall drop-all), so another program can't be used to get
  around it.
- 2026-11-14: A hotkey's lock is confirmed once, when the hotkey is added
  (R-LOCK-12); pressing the hotkey then sets it without asking, since a
  pressed hotkey has no terminal.
- 2026-11-14 (replaces the entry above): each hotkey has its own option
  "Run without confirmation". Off, pressing a hotkey with a lock asks in a
  desktop dialog before anything is locked; on, its lock is confirmed only
  when the hotkey is set up. Monk mode started automatically (a bypass
  prevention step, a list's or phrase set's `monk` action) asks nothing when
  it starts: setting up such a step or action needs ticking "I know this will
  not ask for confirmation".

- 2026-11-14: Word combinations become phrases once: each box a word set,
  its `:N` (several words from one box) dropped; a phrase set's threshold of
  different phrases does that job. The new phrase sets start at threshold 1.
- 2026-11-14: No categories: KC9K ships no ready-made lists or phrase sets
  (the 14 lists behind `block create --template` go too). Users build
  their own and keep them as clusters.

## 7. Open decisions

Not yet settled by the owner. Don't resolve them alone.

1. **The app's name.** Killer Chicken 9000 (KC9K), or Nepsis.
