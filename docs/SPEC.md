# KC9K Specification and Glossary — DRAFT 2

Status: **draft for the owner's review.** Nothing is final until the owner has
corrected it. Code that disagrees with an approved rule is a bug; a rule nobody
has approved is a question.

Sources: the owner's notes, the current code, and Cold Turkey Blocker's
*Features* page and *User Guide* (including *Command Line Parameters*), as
pasted by the owner on 2026-11-11. The owner wants every Cold Turkey feature
in KC9K.

How to read this:

- **Terms** are defined in §1 and mean exactly that everywhere else.
- **Rules** have IDs (`R-LOCK-3`); tests should name the rule they prove.
- Status tags: **HAVE** exists today · **PART** partly exists · **NEW** to
  build · **?** open question (§5).
- **CT** marks a function that Cold Turkey has. *CT name* gives Cold Turkey's
  word where KC9K's differs.
- **n/a** marks a Cold Turkey feature that has no meaning on Linux (stated why).

---

## 1. Glossary

### 1.1 Targets — what can be blocked

| Term | Meaning | Not |
|---|---|---|
| **Target** | Anything a block acts on: site, URL entry, search, app, folder, window title, page word. | — |
| **Site** | A domain with its subdomains (`reddit.com`), or a wildcard domain (`*.xxx`). | A URL entry. |
| **URL entry** | A domain plus path or query (`reddit.com/r/funny`, `youtube.com/@channel`), wildcards allowed. | A site. |
| **Wildcard** | `*` = any text, in URL entries (`google.com/*q=*unicorn*`). In word lists: plain words match whole words, `*` matches inside words. | — |
| **Search entry** | A word that blocks searches containing it, on any search engine. *CT: "All searches containing…" (made from wildcards).* | A page word. |
| **Whole internet** | The target `*.*`. | — |
| **File URL** | A `file://` entry. Local files are allowed by default (a default exception `file://*`). | — |
| **App** | A program, matched by name, path, Python script, file hash, Flatpak ID or desktop unit. *CT: exe / application / binary / Microsoft Store app.* | A window. |
| **Folder entry** | A directory: every program run from inside it is a blocked app; file managers showing it are closed. | — |
| **Title entry** | Matched against window titles. In a browser only the matching **tab** is blocked, not the browser. | — |
| **Page word** | Matched against the rendered text of a page. | Title entry. |

### 1.2 Grouping

| Term | Meaning | Not |
|---|---|---|
| **Block** | A named set of rules: lists, apps, folders, titles, schedule, break, locks-to-apply, autostart, affected users. On its own it does nothing; it acts only when **active**. | A lock. |
| **List** | A named set of entries inside one block, with a **scope** and a **kind**. | A cluster. |
| **Scope** | Where a list looks: *url*, *title*, *text* (one or several). | — |
| **Blacklist** | List kind: matches get the list's **actions**. *CT: "Websites" tab.* | — |
| **Exception list** | List kind: matches are spared from the blacklists **of the same block only**. *CT: "Website Exceptions" tab.* | An allowlist. |
| **Allowlist block** | A block whose blacklist is `*.*`, so only its exceptions work. | — |
| **Cluster** | A list that belongs to no block: never enforced, always freely editable. | A list. |
| **Category** | A read-only list shipped with KC9K (Social, News, Adult, Games, Shopping, …), importable. *CT: "Website categories".* | A cluster (yours) or template (a whole block). |
| **Default list** | The category set preloaded into a new block (CT's "default distraction list"). | — |
| **Template** | A ready-made block definition. | A category. |
| **Affected users** | Which local user accounts a block applies to (default: all). | — |

### 1.3 Block states

| Term | Meaning | Not |
|---|---|---|
| **Enabled** | The block is switched on: it runs now if continuous, or follows its schedule. *CT: the toggle.* | Active. |
| **Disabled** | Switched off: never runs, schedule ignored. | Unlocked. |
| **Continuous** | Block type: active the whole time it is enabled. | Scheduled. |
| **Scheduled** | Block type: active only inside its **block windows** (or place rules) while enabled. | — |
| **Active** | Being enforced right now. | Enabled, Locked. |
| **On break** | Active but currently lifted by a break (pomodoro rest, allowance left, reward, …). | Disabled. |
| **Locked** | Has at least one lock in force. | Active. |
| **Held** | Cannot currently be weakened (locked, schedule-locked, edit-frozen, or allow-windowed). | — |

KC9K today has no separate *enabled*: `start` = enable + active, `stop` =
disable. **?** Q1: adopt CT's split (toggle + type)?

### 1.4 Locks — what protects a block

A **lock** decides what it costs to **disable or weaken** a block. A lock never
changes what is blocked. Blocks are rules; locks protect rules.

| Term | Meaning | CT | Status |
|---|---|---|---|
| **Timer lock** | No weakening until a trusted end time; can always be extended. | Timer | HAVE (`--for`, `--until`) |
| **Text lock** | Weakening needs a typed challenge: random characters, random words, or custom text, 1–5000 chars. | Random and custom text | PART (random characters only, ≤?) |
| **Range lock** | Weakening is refused (or only allowed) between set times on set days. | Time range | NEW |
| **Delay lock** | Weakening starts a countdown (1 min–40 days); it happens only when it ends. | Delay | HAVE (`--delay`) |
| **Schedule lock** | The schedule can't be changed while a window is running, or up to 24 h before one starts. | Schedule | PART (fixed 2 h **edit freeze**) |
| **Restart lock** | Can be disabled only after the computer has restarted. Continuous blocks only. | Restart | HAVE (`--restart`) |
| **Password lock** | Weakening needs a password (for an accountability partner). | Password | HAVE (`--password-stdin`) |
| **Trusted time** | Time KC9K believes despite clock changes. | — | HAVE |

What **any lock** also does while in force (CT "a locked block will also…"):

- uninstalling is refused;
- the browser extension is required;
- the protective settings (§2.10) can't be turned off;
- escape routes are closed: Tor windows, guest profiles, unsupported
  browsers, containers/VMs (**strict mode**).

### 1.5 Weakening

| Term | Meaning |
|---|---|
| **Strengthening** | Blocks more or makes locks stronger. Always allowed, instantly. |
| **Weakening** | Blocks less: removing or narrowing entries, removing actions, raising thresholds, shrinking windows, disabling, removing a lock, shortening a timer, a looser break. |
| **Gate** | What a weakening must pass: *free*, *password*, *text*, *delay*, *refused*. Decided by the locks in force. |
| **Pending edit** | A weakening waiting for its delay. |
| **Allow-window delay** | Today's rule: blocks with allow windows queue every weakening 24 h. **?** Q2: fold into **delay lock**? |
| **Edit freeze** | 2 h before a block window, weakening that block is refused. **?** Q3: replace with **schedule lock** (configurable 0–24 h)? |

### 1.6 Time and breaks

| Term | Meaning | CT | Status |
|---|---|---|---|
| **Block window** | Recurring weekly time when a scheduled block is active. 5-minute precision; one block's windows can't overlap. | Scheduled block | HAVE |
| **Allow window** | Recurring time when an otherwise-on block is lifted. | — | HAVE |
| **Break** | A rule that lifts an active block for a while. One break per continuous block, or per block window. | Break | — |
| **Pomodoro** | Work (active) and rest (lifted) alternate, starting with work. Anchored at midnight, at the start time on the day it was started, and at each window's start for scheduled blocks. Optional long rest. | Pomodoro | HAVE |
| **Allowance** | Blocked items usable for N minutes per period, then active. Period: rolling window, per window, day, week or month, with a refill time. Counts only **use** (§1.7). | Allowance | PART (periods?) |
| **Reward** | Rest time **earned** by working (being blocked) for a while; it adds up but expires each window/day/week/month. | Reward | PART (`break allowance … --per`) |
| **Text break** | A break of a preset length, taken after typing a challenge. | Random and custom text | NEW |
| **Delay break** | A break of a preset length, taken after a countdown. | Delay | NEW |
| **Session break** | Blocked items may be opened N times per period; a session ends a few minutes after you do something else. | Sessions | NEW |
| **Pause** | A one-off lift until a time (also what `relock` makes `unlock` do). | — | HAVE |
| **Autostart** | Re-enables a disabled block: *once* at a time, at a *time of day* on set days, on *sign-in/wake*, or at the *next block window*. | Autostart | NEW |

Proposal: today's **quota** = **allowance**; one word only. (Q4)

### 1.7 Use

| Term | Meaning |
|---|---|
| **Use** | Time a blocked item is foreground (focused app, focused tab) **and** there has been input in the last 3 min (or media is playing). |
| **Strict use counting** | Setting: also count background tabs and minimised apps. *CT: Allowance Strictness.* |

### 1.8 Device blocks

| Term | Meaning | Not |
|---|---|---|
| **Frozen Chicken** | KC9K's device block: lock, log out or shut down for a time, on demand or scheduled. *CT: Frozen Turkey / Device Blocks.* | Edit freeze. |
| **Freeze mode** | *lock* / *logout* / *shutdown* / *allowed apps only* (**?** Q5). | — |
| **Monk mode** | A system-owned whole-internet block started by `kc9k monk` or a `monk` action, time-locked. Proposal: reserved, hidden from Blocks, `monk` action not offered inside it. | An ordinary block. |
| **Consequence** | What KC9K does to the device: close tab, close browser, lock screen, logout, shutdown, monk mode. | — |

### 1.9 Page actions

| Term | Meaning |
|---|---|
| **Block page** | Replaces a blocked page; shows a motivational message. |
| **Close tab / Redirect / Image** | Alternatives to the block page. |
| **Look actions** | Greyscale, no images, no video, mute, declutter, dark: alter, don't block. |
| **Threshold** | Page words: minimum distinct entries matched. |
| **Hit count** | Page words: minimum total matches. |
| **Rate** (NEW) | Page words: matches ÷ rendered words, %. |

### 1.10 Bypass prevention

| Term | Meaning |
|---|---|
| **Event** | Something KC9K stopped or noticed. |
| **Detector** | An event type that counts toward escalation. |
| **Ladder** | N events within T → consequence; climbing steps. *(code: nepsis)* |
| **Workaround** | A dodged block (mirror, proxy URL, fetch tool). |
| **Tamper** | Daemon restart, clock jump. |

### 1.11 Passwords

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Guards every weakening and settings change. | Password lock. |
| **Access password** | Guards *viewing* parts of the app (per area). *CT: "Password" settings tab (privacy).* | Password lock. |
| **Setting lock** | A lock on a single setting, not a block. | — |

---

## 2. Functions

### 2.1 Overview (Dashboard)
| Function | Does | CT | Status |
|---|---|---|---|
| Active now | Lists active blocks with lock, end time. | ✓ | HAVE |
| Later today | Scheduled windows still to come. | ✓ | ? |
| Break status | Allowance left, pomodoro phase/time. | ✓ | HAVE |

### 2.2 Blocks tab
| Function | Does | CT | Status |
|---|---|---|---|
| Create block | Empty, from template, allowlist. | ✓ | HAVE |
| Enable / disable toggle | See §1.3. | ✓ | PART |
| Rename | | ✓ | HAVE |
| Duplicate | Copy comes disabled. | ✓ | HAVE |
| Delete | Refused while locked. | ✓ | HAVE |
| Affected users | Per-block user selection. | ✓ | NEW |
| Export / import all blocks | | ✓ | HAVE (`project`) |
| Edit schedule / break / lock | From the block row. | ✓ | PART (GUI) |

### 2.3 Block editor
| Function | Does | CT | Status |
|---|---|---|---|
| Add entry | Type, Enter; focus **stays** in the box. | ✓ | PART (focus bug) |
| Remove entry | Delete/Backspace key or button; selection moves to the next entry. | ✓ | PART |
| Multi-select | Ctrl/Shift-click, Select all, Ctrl+A. | ✓ | ? |
| Edit entry | Double-click: moved to the text box for editing (remove+add, gated if narrowing). | ✓ | NEW |
| Find entry | Ctrl+F. | ✓ | ? |
| Save changes / Cancel | One draft per block; nothing reverts; Save says what is free, gated, delayed. | — | NEW |
| Import picker | *From KC9K*: categories, lists of other blocks, clusters — multi-select. *From file*: `.txt`, one entry per line. Plus *Whole internet* and *All searches containing…*. | ✓ | NEW |
| Export list | `.txt`. | ✓ | HAVE |
| Lists: create, rename, delete, scope, actions, thresholds, rate, test mode | | — | HAVE (rate NEW) |
| Apps: add program, add folder, add title | Linux: file picker for programs; Flatpak list instead of Store apps. | ✓ | PART (folder NEW) |
| Social presets, playlists | | — | HAVE |

### 2.4 Clusters and recording
| Function | Does | Status |
|---|---|---|
| Clusters: create, edit, copy into block, save from block | | HAVE |
| Record links | Visited sites into a list/cluster. | HAVE |
| Record words | Rendered, visible words with counts. | NEW |
| Word ranking | By specificity vs normal browsing. | NEW |
| Cut bottom X % | Before saving. | NEW |

### 2.5 Schedules
| Function | Does | CT | Status |
|---|---|---|---|
| Weekly grid | Drag to add, drag to move, click to edit, 15-min grid, 5-min times. | ✓ | ? (GUI) |
| Copy to days | Checkboxes; similar windows edited together. | ✓ | ? |
| Show other blocks | Greyed out on the grid. | ✓ | NEW |
| Break per window | | ✓ | ? |
| Allow windows | | — | HAVE |
| Place rules | Active at Wi-Fi places. | — | HAVE |

### 2.6 Locks
All of §1.4. Missing today: **range lock**, **text lock** with words/custom
text and length up to 5000, **schedule lock** as an option (0–24 h). Extending a
timer lock: HAVE (`start` only strengthens).

### 2.7 Breaks and autostart
All of §1.6. Missing: allowance periods and refill time, rolling allowance,
reward expiry, text break, delay break, session break, autostart (4 kinds).

### 2.8 Frozen Chicken
| Function | Does | CT | Status |
|---|---|---|---|
| Start for a time | | ✓ | HAVE |
| Schedule | | ✓ | HAVE |
| Schedule **and** on-demand at once | CT can't; KC9K should. | ✗ | HAVE? |
| Mode lock/logout/shutdown | | ✓ | HAVE |

### 2.9 Statistics
| Function | Does | CT | Status |
|---|---|---|---|
| Time per site/app, blocked counts | | ✓ | HAVE |
| Time frame | Presets + custom range. | ✓ | PART (`--days`) |
| Per user | | ✓ | NEW |
| Top 5 / top 10 / search | Search matches part of URL/app/title. | ✓ | ? |
| Filter by block | | ✓ | ? |
| Export JSON | | ✓ | HAVE |
| Delete, on/off, skip private windows | Private windows still count during locked allowances. | ✓ | PART |

### 2.10 Settings
| Group | Function | CT | Status |
|---|---|---|---|
| Preferences | 24 h clock, week starts Mon/Sun, theme + darkness slider, open at login, tray. | ✓ | PART (slider NEW) |
| Extension | Show install dialog for missing extensions. | ✓ | ? |
| Notifications | Lock timer ends; window starts/ends; pomodoro changes; allowance runs out; app blocked. Warnings in minutes, comma-separated ("2,5"). Status in the extension popup. | ✓ | PART |
| Protection | Block unsupported browsers. | ✓ | HAVE (extension guard) |
| | Close browsers if the extension is stopped. | ✓ | HAVE |
| | Grace period for a missing extension (default 60 s). | ✓ | PART (fixed 60 s) |
| | Block time and date changes. | ✓ | PART (trusted time; settings app not closed) |
| | Block task managers (System Monitor, htop…). | ✓ | NEW |
| | Block inactive tabs. | ✓ | ? |
| | Block embedded content (iframes, embedded videos). | ✓ | ? |
| | Block reinstalling / the installer. | ✓ | ? |
| | Allow `file://` blocking. | ✓ | ? |
| | Block screen-time settings | macOS | n/a |
| | Pause for a Cause (pay charity for 10 min) | ✓ | ? Q6 |
| Use counting | Strict use counting. | ✓ | ? |
| Dark pages | off / always / time window, exempt sites. | — | NEW |
| Passwords | Settings password, access passwords. | ✓ | HAVE |
| Options | polite-close, block-page-details, … | — | HAVE |

### 2.11 Command line
Every GUI function has a command. CT parity:

| CT | KC9K | Status |
|---|---|---|
| `-add "B" -web "URL"` | `kc9k block add B URL` | HAVE |
| `-add "B" -exception "URL"` (unlocked only) | `kc9k block add-exception B URL` (gated) | HAVE |
| `-start "B"` | `kc9k start B` | HAVE |
| `-start "B" -lock X` (X ≥ remaining) | `kc9k start B --for Xm` | HAVE |
| `-start "B" -password X` | `kc9k start B --password-stdin` | HAVE |
| `-start "B" -random-text X` (1–999) | `kc9k start B --lock --lock-chars X` | HAVE |
| `-stop "B"` | `kc9k stop B` | HAVE |
| `-stop "B" -password X` | `kc9k stop B` + password on stdin | HAVE |
| `-toggle "B"` (not Frozen) | `kc9k toggle B` | NEW |

### 2.12 Not applicable or replaced
- Windows Store apps → Flatpak apps (HAVE).
- Exe file locking against rename → hash matching (HAVE).
- Reinstall Windows service → `systemctl` (HAVE).
- Safari / macOS-only items → n/a.

---

## 3. Rules

**Locks**
- **R-LOCK-1** A disabled block is never held by its locks — except it cannot be *deleted* while a lock is in force.
- **R-LOCK-2** Starting an already active block may only strengthen its locks; a timer may always be extended.
- **R-LOCK-3** An unlocked block with no schedule lock, no edit freeze and no allow windows can be edited in any direction at once (settings password still applies).
- **R-LOCK-4** A timer lock refuses every weakening before its trusted end time.
- **R-LOCK-5** A text lock allows weakening only after the challenge is typed correctly, paste refused.
- **R-LOCK-6** A delay lock turns every weakening into a pending edit applied when the countdown ends.
- **R-LOCK-7** A restart lock can be removed only after a boot later than the lock's start.
- **R-LOCK-8** A range lock refuses weakening inside (or, inverted, outside) its ranges.
- **R-LOCK-9** While any lock is in force: uninstall refused, extension required, protective settings can't be turned off, strict mode on.

**Weakening**
- **R-WEAK-1** Strengthening is never gated.
- **R-WEAK-2** Every weakening gets exactly one gate decision, in the CLI; the GUI only calls the CLI.
- **R-WEAK-3** Editing an entry is remove+add; it is weakening if the new entry matches less.
- **R-WEAK-4** A refused weakening is logged, never counted as a bypass.
- **R-WEAK-5** Several gates combine as the strictest of them (all must pass).

**Schedules and breaks**
- **R-SCHED-1** An enabled scheduled block is active inside its windows; a disabled one never is.
- **R-SCHED-2** A block's windows may not overlap.
- **R-SCHED-3** Pomodoro starts with work and is anchored as in §1.6.
- **R-BREAK-1** Allowance counts only use (§1.7).

**Lists**
- **R-LIST-1** Exceptions spare pages only from their own block's blacklists.
- **R-LIST-2** Clusters and categories are never enforced.
- **R-LIST-3** `file://*` is a default exception in every new block.
- **R-LIST-4** Page words are matched against rendered, visible text only.
- **R-TITLE-1** A title match in a browser blocks the tab, not the browser.

**Monk mode**
- **R-MONK-1** The monk-mode block is system-owned and the `monk` action is not offered inside it.

---

## 4. Already in KC9K beyond Cold Turkey
Clusters, link recording, page-word lists with actions, look actions,
social-media presets and playlists, places, ladder (bypass escalation),
hotkeys, access passwords per area, project export, polite close, Flatpak/script/hash
app matching, monk mode, quota by use, LibreWolf hardening.

## 5. Open questions
1. **Enabled vs active**: adopt CT's toggle + continuous/scheduled type?
2. **Allow-window 24 h delay**: keep, or make it a delay lock the user chooses?
3. **Edit freeze**: replace with an optional schedule lock (0–24 h before)?
4. **Quota = allowance**: one term?
5. **Frozen Chicken "allowed apps only"**: wanted? (CT keeps app whitelists in a separate product, Micromanager.)
6. **Pause for a Cause**: wanted? KC9K has no payments; it could be replaced by a "costly break" (long text challenge).
7. **Rate**: beside threshold/hit count, or instead?
8. **Affected users**: per block, or per whole install?
9. **Categories**: which ones, and where do the lists come from?
10. Items marked **?** in §2 need checking in code.
