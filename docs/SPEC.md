# KC9K Specification and Glossary

Draft for the owner's review. This document says **what KC9K is and does**,
and what each thing is called. It does not track work: that is `TODO.md` and
`docs/PLAN.md`.

- Terms (§1) mean exactly what they say here, everywhere: code, GUI, docs.
- Rules (§3) have IDs; tests name the rule they prove.
- Where KC9K uses a different word from Cold Turkey, the Cold Turkey word is
  given as *CT: …* so the two can be compared.

---

## 1. Glossary

### 1.1 Targets — what can be blocked

| Term | Meaning | Not |
|---|---|---|
| **Target** | Anything a block acts on: site, URL entry, search entry, app, folder, title entry, page word. | — |
| **Site** | A domain with all its subdomains (`reddit.com`), or a wildcard domain (`*.xxx`). | A URL entry. |
| **URL entry** | A domain plus path or query (`reddit.com/r/funny`, `youtube.com/@channel`). | A site. |
| **Wildcard** | In URL entries, `*` is any text (`google.com/*q=*unicorn*`). In word lists, a plain word matches whole words and `*` matches inside words. | — |
| **Search entry** | A word that blocks searches containing it, on any search engine. | A page word. |
| **Whole internet** | The target `*.*`. | — |
| **File URL** | A `file://` entry. Local files are allowed by the default exception `file://*`. | — |
| **App** | A program, matched by name, path, script, file hash, Flatpak ID or desktop unit. | A window. |
| **Folder entry** | A directory: every program run from inside it is a blocked app, and windows showing it are closed. | — |
| **Title entry** | A word or phrase matched against window titles. In a browser only the matching tab is blocked, never the browser. | A page word. |
| **Page word** | A word or phrase matched against the rendered, visible text of a page. | A title entry. |

### 1.2 Grouping

| Term | Meaning | Not |
|---|---|---|
| **Block** | A named set of rules: lists, apps, folders, titles, schedule, break, autostart, affected users. A block does nothing by itself; it acts only while **active**. | A lock. |
| **List** | A named set of entries inside one block, with a **scope** and a **kind**. | A cluster. |
| **Scope** | Where a list looks: *url*, *title*, *text* (one or several). | — |
| **Blacklist** | List kind: matches get the list's actions. *CT: Websites tab.* | — |
| **Exception list** | List kind: matches are spared from the blacklists **of the same block only**. *CT: Website Exceptions.* | An allowlist block. |
| **Allowlist block** | A block whose blacklist is `*.*`, so that only its exceptions are reachable. | — |
| **Cluster** | A list belonging to no block: never enforced, always freely editable. | A list. |
| **Category** | A read-only list shipped with KC9K (Social, News, Adult, …), to import into blocks. *CT: Website categories.* | A cluster, a template. |
| **Default list** | The categories a new block starts with. | — |
| **Template** | A ready-made block definition to create a block from. | A category. |
| **Affected users** | The local user accounts a block applies to (default: all). | — |

### 1.3 Block states

| Term | Meaning | Not |
|---|---|---|
| **Enabled** | Switched on: runs now if continuous, follows its schedule if scheduled. *CT: the toggle.* | Active. |
| **Disabled** | Switched off: never active, schedule ignored. | Unlocked. |
| **Continuous** | Block type: active the whole time it is enabled. | Scheduled. |
| **Scheduled** | Block type: active only inside its block windows or place rules, while enabled. | — |
| **Active** | Being enforced right now. | Enabled, locked. |
| **On break** | Active, but currently lifted by its break. | Disabled. |
| **Locked** | At least one lock is in force on the block. | Active. |
| **Held** | The block cannot be weakened right now (by a lock, a schedule lock or allow windows). | Locked. |

### 1.4 Locks — what protects a block

A **lock** decides what it costs to **disable or weaken** a block. A lock never
changes what is blocked. *Blocks are rules; locks protect rules.*

| Term | Meaning | CT |
|---|---|---|
| **Timer lock** | No weakening until a trusted end time. It can always be extended. | Timer |
| **Text lock** | Weakening needs a typed challenge: random characters, random words or custom text, 1–5000 characters, paste refused. | Random and custom text |
| **Range lock** | Weakening is refused, or only allowed, between set times on set days. | Time range |
| **Delay lock** | Weakening starts a countdown (1 min to 40 days) and happens only when it ends. | Delay |
| **Schedule lock** | The schedule can't be changed while a window runs, nor for a lead time the user chooses before one starts. No lead time is built in. | Schedule |
| **Restart lock** | The block can be disabled only after the computer has restarted. Continuous blocks only. | Restart |
| **Password lock** | Weakening needs a password, e.g. one an accountability partner holds. | Password |
| **Trusted time** | The time KC9K believes despite clock changes. All time-based locks count on it. | — |

While **any** lock is in force on any block:
uninstalling is refused, the browser extension is required, protective
settings (§2.10) can't be turned off, and escape routes are closed (private or
Tor windows, guest profiles, unsupported browsers, containers and VMs).

### 1.5 Weakening

| Term | Meaning |
|---|---|
| **Strengthening** | A change that blocks more or makes protection stronger. |
| **Weakening** | A change that blocks less: removing or narrowing entries, removing actions, raising thresholds, shrinking windows, a looser break, disabling, deleting, removing or shortening a lock. |
| **Gate** | What a weakening must pass: *free*, *password*, *text*, *delay*, or *refused*. The locks in force decide it. |
| **Pending edit** | A weakening waiting for its delay to end. |

### 1.6 Time and breaks

| Term | Meaning |
|---|---|
| **Block window** | A recurring weekly time when a scheduled block is active. Times in 5-minute steps; one block's windows never overlap. |
| **Allow window** | A recurring time when an otherwise always-on block is lifted. |
| **Break** | A rule that lifts an active block for a while. One break per continuous block, or one per block window. |
| **Pomodoro** | Work (active) and rest (lifted) alternate, starting with work, with an optional long rest every N rounds. |
| **Allowance** | Blocked items may be used for N minutes per period, then the block takes effect. Period: rolling, per window, day, week or month, with a refill time. Counts only **use** (§1.7). |
| **Reward** | Rest time earned by working (being blocked) for a while. It adds up, and expires at the end of each window, day, week or month. |
| **Text break** | A break of preset length, taken after typing a challenge. |
| **Delay break** | A break of preset length, taken after a countdown. |
| **Session break** | Blocked items may be opened N times per period. A session ends a few minutes after you switch to something else. |
| **Pause** | A one-off lift of an active block until a set time. |
| **Autostart** | Re-enables a disabled block: *once* at a time, at a *time of day* on set days, at *sign-in or wake*, or at the *next block window*. |
| **Place rule** | A block is active while the computer is at a place (a set of Wi-Fi access points). |

### 1.7 Use

| Term | Meaning |
|---|---|
| **Use** | Time a blocked item is in the foreground (focused app or focused tab) with input in the last 3 minutes, or with media playing. |
| **Strict use counting** | Setting: background tabs and minimised apps count as use too. |

### 1.8 Device-level

| Term | Meaning | Not |
|---|---|---|
| **Frozen Chicken** | The device block: locks the screen, logs out or shuts down for a time, on demand or by schedule. *CT: Frozen Turkey / Device Blocks.* | A schedule lock. |
| **Freeze mode** | What Frozen Chicken does: *lock*, *logout*, *shutdown*, or *allowed apps only*. | — |
| **Allowed apps only** | Freeze mode: the computer stays usable, but only the programs on the freeze's app list may run; every other user program is closed. | An allowlist block (websites). |
| **Monk mode** | A system-owned whole-internet block with a timer lock, started by `kc9k monk` or by the `monk` action. | An ordinary block. |
| **Consequence** | Something KC9K does to the device: close tab, close browser, lock screen, logout, shutdown, monk mode. | A page action. |
| **Strict mode** | Closing escape routes (containers, VMs, uninstall) while locks are in force. | — |

### 1.9 Page actions

| Term | Meaning |
|---|---|
| **Block page** | Replaces a blocked page, with a motivational message. |
| **Close tab / Redirect / Image** | Alternatives to the block page. |
| **Look actions** | Greyscale, no images, no video, mute, declutter, dark: they change a page, they don't block it. |
| **Threshold** | For page words: how many distinct entries must match. |
| **Hit count** | For page words: how many matches in total. |
| **Rate** | For page words: matches ÷ rendered words, as a percentage. |

### 1.10 Bypass prevention

| Term | Meaning |
|---|---|
| **Event** | Something KC9K stopped or noticed, kept for a day. |
| **Detector** | An event type that counts toward escalation. |
| **Ladder** | Escalation: N events within T trigger a consequence, step after step. |
| **Workaround** | A dodged block: mirror site, proxy address, fetch tool. |
| **Tamper** | Daemon restart, clock jump. |

### 1.11 Passwords

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Needed for every weakening and every settings change. | A password lock. |
| **Access password** | Needed to *view* parts of the app (per area). | A password lock. |
| **Setting lock** | A lock on one setting instead of a block. | A block lock. |

### 1.12 Recording

| Term | Meaning |
|---|---|
| **Link recording** | Collects the sites you visit into a list or cluster. |
| **Word recording** | Collects the rendered, visible words of pages you visit, with counts. |
| **Specificity** | How much more common a word is on recorded pages than in normal browsing; recorded words are ranked by it. |
| **Cut** | Dropping the lowest-ranked X % of recorded words before saving. |

---

## 2. Functions

What each part of KC9K does. The GUI and the CLI offer the same functions.

### 2.1 Overview
Shows active blocks (with locks and end times), windows still to come today,
and break status (allowance left, pomodoro phase).

### 2.2 Blocks
Create (empty, from a template, as an allowlist block) · enable / disable ·
choose continuous or scheduled · rename · duplicate (the copy starts disabled) ·
delete (refused while locked) · set affected users · export / import all blocks.

### 2.3 Block editor
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
- **Apps**: add program, add folder, add title entry.
- **Social presets** and allowed playlists.

### 2.4 Clusters and recording
Clusters: create, edit, copy into a block, save from a block. Link recording
and word recording into a list or cluster; ranking and cut before saving.

### 2.5 Schedules
Weekly grid: drag to add, drag to move, click to edit; 15-minute grid, 5-minute
times; copy to other days; similar windows edited together; other blocks shown
greyed out; a break per window. Allow windows. Place rules.

### 2.6 Locks
All locks of §1.4, alone or combined. A timer lock can always be extended.

### 2.7 Breaks and autostart
All breaks and autostart kinds of §1.6. Break status is also shown in the
browser extension's popup.

### 2.8 Frozen Chicken
Start for a time; schedule; both may exist together. Freeze mode lock / logout /
shutdown / allowed apps only (with its own app list).

### 2.8a No built-in protection
Nothing protects a block unless the user chose it. There is no fixed edit
freeze and no fixed delay: every lock, lead time and duration is set by the
user per block.

### 2.9 Statistics
Time used and times blocked, per site, app and title. Time frame (presets or
custom), per user, top 5 / top 10 / search, filter by block, export JSON,
delete, on/off, optionally skip private windows (except during a locked
allowance). Stored only on the computer.

### 2.10 Settings
- **Preferences**: 24-hour clock, first day of the week, theme with a darkness
  slider (dark grey to black), open at login, tray.
- **Extension**: show the install dialog when an extension is missing.
- **Notifications**: timer lock ends, window starts/ends, pomodoro changes,
  allowance runs out, app blocked; advance warnings in minutes, comma-separated
  (`2,5`).
- **Protection** (can't be turned off while locked): block unsupported browsers;
  close browsers if the extension is stopped; grace period for a missing
  extension (default 60 s); block time and date changes; block task managers;
  block inactive tabs; block embedded content; block the installer; allow
  `file://` blocking.
- **Use counting**: strict use counting.
- **Dark pages**: off / always / during a time window, with exempt sites.
- **Passwords**: settings password, access passwords.
- **Options**: polite close, block page details.

### 2.11 Command line
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

## 3. Rules

**Locks**
- **R-LOCK-1** A lock protects only an enabled block; a disabled block can't be deleted while a lock is in force.
- **R-LOCK-2** Starting an active block again may only strengthen its locks; a timer lock may always be extended.
- **R-LOCK-3** A block that is not held can be edited in any direction at once (the settings password still applies).
- **R-LOCK-4** A timer lock refuses every weakening before its trusted end time.
- **R-LOCK-5** A text lock allows weakening only after the challenge is typed correctly; pasting is refused.
- **R-LOCK-6** A delay lock turns every weakening into a pending edit applied when its countdown ends.
- **R-LOCK-7** A restart lock can be removed only after a boot later than the lock's start.
- **R-LOCK-8** A range lock refuses weakening inside (or, inverted, outside) its ranges.
- **R-LOCK-9** While any lock is in force: uninstall refused, extension required, protective settings can't be turned off, strict mode on.

**Weakening**
- **R-WEAK-1** Strengthening is never gated.
- **R-WEAK-2** Every weakening gets exactly one gate decision, made in the CLI; the GUI only runs CLI commands.
- **R-WEAK-3** Editing an entry is remove + add; it is weakening if the new entry matches less.
- **R-WEAK-4** With several locks, every gate must be passed.
- **R-WEAK-5** A refused weakening is logged and never counted as a bypass.

**Schedules and breaks**
- **R-SCHED-1** An enabled scheduled block is active inside its windows; a disabled block never is.
- **R-SCHED-2** A block's windows never overlap.
- **R-SCHED-3** Pomodoro starts with work. Its rhythm is counted from midnight, except from the start time on the day a continuous block is started, and from each window's start on a scheduled block.
- **R-BREAK-1** An allowance counts only use (§1.7).

**Lists**
- **R-LIST-1** Exceptions spare pages only from their own block's blacklists.
- **R-LIST-2** Clusters and categories are never enforced.
- **R-LIST-3** Every new block has the exception `file://*`.
- **R-LIST-4** Page words are matched against rendered, visible text only.
- **R-TITLE-1** A title match in a browser blocks the tab, never the whole browser.

**Monk mode**
- **R-MONK-1** The monk-mode block is system-owned, and the `monk` action is not offered inside it.
- **R-LOCK-10** No lock, delay or freeze applies unless the user set it on that block.

**Device**
- **R-FREEZE-1** In *allowed apps only* mode, only programs on the freeze's app list (and system processes) may run.
