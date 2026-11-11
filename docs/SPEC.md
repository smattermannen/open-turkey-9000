# KC9K Specification and Glossary — DRAFT 1

Status: **draft for the owner's review.** Nothing here is final until the owner
has corrected it. Code that disagrees with an approved rule is a bug; a rule
nobody has approved is a question.

How to read this:

- **Terms** are in bold where they are defined (§1). Everywhere else they mean
  exactly that and nothing else.
- **Rules** have IDs (`R-LOCK-3`). Tests should name the rule they prove.
- Tags: **[HAVE]** exists in the code today · **[NEW]** to build ·
  **[CT]** matches a Cold Turkey feature (from memory: the getcoldturkey.com
  pages could not be fetched in this session, so check every [CT] item
  against them) · **[?]** open question for the owner.

---

## 1. Glossary

### 1.1 What gets blocked

| Term | Meaning | Not |
|---|---|---|
| **Target** | Anything a block can act on: a site, an app, a window title, page words. | — |
| **Site** | A domain, with or without wildcards (`*.reddit.com`, `*.*`). | A URL path (that is a URL entry). |
| **URL entry** | A domain plus path/query fragment (`youtube.com/shorts`). | — |
| **App** | A program identified by process name, path, script, file hash, Flatpak ID or desktop unit. | A window. |
| **Title entry** | A word or phrase matched against window titles (apps) and tab titles (browser). | — |
| **Page word** | A word or phrase matched against the rendered text of a page. | Title entry. |
| **Folder entry** [NEW][CT] | A directory: any program started from inside it counts as a blocked app. | — |
| **Whole internet** | The target `*.*`: every site. | — |

### 1.2 How targets are grouped

| Term | Meaning | Not |
|---|---|---|
| **Block** | A named set of rules: lists, apps, titles, schedule, actions, social settings, place rules. A block by itself does nothing. | A lock. |
| **List** | A named set of entries that belongs to one block. It has a **scope** (url / title / text) and a **kind**. | A cluster. |
| **Blacklist** | A list of kind *blacklist*: matching pages/apps get the list's actions. | — |
| **Exception list** | A list of kind *exception*: matching pages are spared from the blacklists **of the same block only**. | An allowlist block. |
| **Allowlist block** | A block whose blacklist is `*.*` with exception lists: everything is blocked except the exceptions. | — |
| **Cluster** | A list that belongs to no block. Never enforced, always freely editable. Raw material for lists. | A list. |
| **Default list** [NEW] | A cluster shipped with KC9K (e.g. "Social media", "Adult", "News"), read-only, copyable. | A template block. |
| **Template block** [HAVE] | A ready-made block definition that `block create --template` copies. | A default list. |
| **Scope** | Where a list looks: *url*, *title*, *text*. A list may tick several. | — |

### 1.3 States of a block

| Term | Meaning | Not |
|---|---|---|
| **Inactive** | Not being enforced. | Unlocked. |
| **Active** | Being enforced right now. | Locked. |
| **Started** | Made active by hand (`start`), a hotkey, or a consequence. | Scheduled. |
| **Scheduled** | Made active by a schedule window, a place rule, or a quota running out. | Started. |
| **Paused** | Active but lifted until a set time (a pause or a break). | Inactive. |
| **Held** | Cannot currently be weakened, because it is locked, frozen, or allow-windowed. | Active. |

### 1.4 Locks — what makes a block hard to weaken

A **lock** is attached to an *active* block (or to a setting) and decides what
it costs to **stop** or **weaken** it. Locks never decide what is blocked.

| Term | Meaning |
|---|---|
| **Time lock** [HAVE][CT] | Cannot be stopped/weakened until a trusted time (`--for`, `--until`). |
| **Typing lock** [HAVE][CT "random text"] | Stopping requires typing N random characters (the challenge). |
| **Password lock** [HAVE][CT] | Stopping requires the settings password. |
| **Restart lock** [NEW][CT] | Stopping is only possible after the computer has rebooted. |
| **Window lock** [NEW][CT "range"] | Stopping is only possible during a set time-of-day window (e.g. Sundays 18–19). |
| **Unlocked** | No lock: an active block can be stopped and edited freely. |
| **Trusted time** [HAVE] | Time KC9K believes despite clock changes; time locks count on it. |

### 1.5 Weakening — the central concept

| Term | Meaning |
|---|---|
| **Strengthening** | A change that blocks more or makes locks stronger. Always allowed, instantly. |
| **Weakening** (= loosening) | A change that blocks less: removing entries, narrowing an entry, removing actions, raising a threshold, shrinking schedule windows, stopping, unlocking, shortening a lock. |
| **Gate** | What a weakening must pass before it happens. One of: *free*, *password*, *typing challenge*, *queue*, *refused*. |
| **Queue** (pending edit) [HAVE] | A weakening stored and applied after the **loosen delay** (default 24 h, per block). |
| **Edit freeze** [HAVE] | The 2 h before a schedule window: weakening edits to that block are refused. |

Rule of thumb the owner set: **blocks are rules; locks protect rules.** An
unlocked, un-scheduled block is ordinary editable data.

### 1.6 Time

| Term | Meaning | Not |
|---|---|---|
| **Block window** | Recurring time when a block is active (Mon–Fri 9–17). | Allow window. |
| **Allow window** | Recurring time when an otherwise-always-on block is lifted. | Break. |
| **Pause** | One-off lifting of an active block until a time. | Break. |
| **Break** | Pause time **earned** by an allowance or given by Pomodoro. | Pause. |
| **Allowance** [HAVE][CT] | A block lifted for X minutes per period of *use*, then active. | Quota. [?] merge the two names |
| **Quota** [HAVE] | Same mechanism, counted by time in use. [?] Proposal: call both "**allowance**". |
| **Pomodoro** [HAVE][CT] | Alternating work (active) / break (lifted) periods. |

### 1.7 Device-level

| Term | Meaning | Not |
|---|---|---|
| **Frozen Chicken** (screen freeze) [HAVE][CT "Frozen Turkey"] | Locks, logs out or shuts down the whole computer for a time. Modes: *lock*, *logout*, *shutdown*, *allowed apps only* [NEW][CT]. | Edit freeze. |
| **Monk mode** [HAVE] | A system-owned whole-internet block, time-locked, started by `kc9k monk` or a `monk` action. [?] Proposal: reserve the block, hide it from Blocks, forbid the `monk` action inside it. | An ordinary block. |
| **Consequence** | Something KC9K does to the device: close browser, close tab, lock screen, logout, shutdown, monk mode. |
| **Strict mode** [HAVE] | While any lock exists, closes escape routes (containers, VM sockets, uninstall). |
| **No-escape** [HAVE] | [?] owner to define relative to strict mode. |

### 1.8 Page actions (what happens to a matching page)

| Term | Meaning |
|---|---|
| **Block** | Replace the page with the block page. |
| **Close tab** | Close the tab. |
| **Redirect** | Send to another URL. |
| **Image** | Show an image instead. |
| **Greyscale / No images / No video / Mute / Declutter / Dark** | **Look actions**: alter, don't block. |
| **Threshold** | For page words: minimum distinct entries matched before the action fires. |
| **Hit count** | For page words: minimum total matches. |
| **Rate** [NEW] | For page words: matches ÷ rendered words, as a percentage. |

### 1.9 Bypass prevention

| Term | Meaning |
|---|---|
| **Event** | Something KC9K stopped or noticed (page hit, app kill, tamper). |
| **Detector** | An event type that counts toward bypass prevention. |
| **Ladder** | Escalating steps: N events within T → consequence C. |
| **Workaround** | An event where a block was dodged (mirror site, proxy URL, fetch tool). |
| **Tamper** | Daemon restart, clock jump, file change. |

### 1.10 Access

| Term | Meaning | Not |
|---|---|---|
| **Settings password** | Guards weakening everywhere. | Access password. |
| **Access password** | Guards *viewing* areas of the app (dashboard, blocks…). | Password lock. |
| **Setting lock** | A lock on one browser/system option instead of a block. | Block lock. |

---

## 2. Functions

Each function: name · what it does · status. Commands are the CLI form; the
GUI must offer the same thing.

### 2.1 Blocks
| Name | Does | Status |
|---|---|---|
| Create block | Empty, from template, or as allowlist. | HAVE |
| Rename / duplicate block | | Rename ? / duplicate NEW |
| Delete block | Weakening; gated. | HAVE |
| Block info | Everything about a block in one view. | HAVE |
| Export / import block | File. | HAVE |
| Enable whole-internet | One switch for `*.*`. | HAVE (as entry) |
| Block embedded content [CT] | Also block the site when embedded in other pages (iframes, video). | ? check |
| Block local files / file:// [CT] | | NEW ? |
| Block incognito when extension missing [CT] | | HAVE via extension guard |

### 2.2 Lists and clusters
| Name | Does | Status |
|---|---|---|
| Create / rename / delete list | | HAVE |
| Add / remove entries | Enter adds and keeps focus; remove selects next. | HAVE (UI fixes NEW) |
| Edit entry | Double-click → editor bar. Treated as remove+add; gated if narrowing. | NEW |
| Set scope / actions / threshold / hits / rate | | HAVE (rate NEW) |
| Test mode | Lists report, don't act. | HAVE |
| Import picker | "From KC9K" (default lists / block lists / clusters, multi-select) or "From file". | NEW |
| Clusters: create, copy into block, save from block | | HAVE |
| Default lists | Shipped read-only clusters. | NEW |

### 2.3 Recording
| Name | Does | Status |
|---|---|---|
| Record links | Collect visited sites into a list or cluster. | HAVE |
| Record words | Collect rendered visible words with counts per page. | NEW |
| Word ranking | Rank by specificity (recorded vs baseline browsing), not raw frequency. | NEW |
| Cut bottom X % | Trim ranked words before saving. | NEW |

### 2.4 Apps
| Name | Does | Status |
|---|---|---|
| Block app by name/path/script/hash/Flatpak/unit | | HAVE |
| Block folder [CT] | | NEW |
| Window-title blocking | | HAVE |
| Polite close | Ask windows to close first. | HAVE |
| App allowlist mode [CT "Micromanager"] | Only listed apps may run while active. | NEW [?] |

### 2.5 Activation
| Name | Does | Status |
|---|---|---|
| Start / stop | | HAVE |
| Start with lock | `--lock`, `--lock-chars`, `--for`, `--until`. | HAVE |
| Restart lock, window lock | | NEW |
| Toggle [CT `-toggle`] | Start if inactive, stop if active. | NEW |
| Schedule (block / allow windows) | Grid editor in GUI [CT]. | HAVE (grid ?) |
| Place rules | Active at Wi-Fi places. | HAVE |
| Hotkeys | | HAVE |
| Pause / break / allowance / Pomodoro | | HAVE |

### 2.6 Device
| Name | Does | Status |
|---|---|---|
| Frozen Chicken: lock / logout / shutdown | | HAVE |
| Frozen Chicken: allowed apps only [CT] | Only listed apps usable. | NEW |
| Frozen Chicken schedule | | HAVE |
| Monk mode | | HAVE (cleanup NEW) |
| Strict mode / no-escape / prevent uninstall [CT] | | HAVE |
| Prevent clock change [CT] | | HAVE (trusted time) |

### 2.7 Browser
| Name | Does | Status |
|---|---|---|
| Extension required / guard | | HAVE |
| Force LibreWolf, block devtools, disable DoH | | HAVE |
| Social media presets, playlists | | HAVE |
| Dark pages setting | Settings: off / always / time window; exempt sites. | NEW |

### 2.8 App (GUI) itself
| Name | Does | Status |
|---|---|---|
| Block editor draft | One **Save changes / Cancel** per block; no auto-revert. Save shows which changes are free, gated or queued. | NEW |
| Theme darkness slider | Dark grey → black. | NEW |
| Statistics [CT] | Time per site/app, per day. | HAVE |

### 2.9 Command line [CT parity]
Every GUI action has a command. Cold Turkey's CLI (from memory) has
`-start "Block" [-lock N]`, `-stop "Block"`, `-add "Block" -web site`,
`-toggle "Block"`. KC9K equivalents: `start`, `stop`, `block add`, and
`toggle` [NEW]. [?] verify the full CT command list against the guide.

---

## 3. Rules

**Locks**
- **R-LOCK-1** An inactive block is never held by its locks.
- **R-LOCK-2** Starting an already active block may only strengthen its locks.
- **R-LOCK-3** An unlocked, unscheduled, allow-window-free block can be edited in any direction without delay (settings password still applies if set).
- **R-LOCK-4** A time-locked block cannot be stopped or weakened before its trusted end time.
- **R-LOCK-5** A typing-locked block can be stopped only after the challenge is passed.

**Weakening**
- **R-WEAK-1** Strengthening is never gated.
- **R-WEAK-2** Every weakening goes through exactly one gate decision, made in the CLI.
- **R-WEAK-3** On a block with allow windows, weakening is queued for the loosen delay, or with `--now` requires the typing challenge.
- **R-WEAK-4** Editing an entry is remove+add; it is weakening if the new entry matches less.
- **R-WEAK-5** A refused weakening is logged, never counted as a bypass.

**Schedules**
- **R-SCHED-1** A block is active during each of its block windows.
- **R-SCHED-2** Within 2 h before a block window, weakening edits to that block/schedule are refused.
- **R-SCHED-3** An allow-window block is active except during its windows.

**Lists**
- **R-LIST-1** An exception list spares pages only from its own block's blacklists.
- **R-LIST-2** Clusters are never enforced and always editable.
- **R-LIST-3** Plain words match whole words; `*` matches inside words.
- **R-LIST-4** Page words are matched against rendered, visible text only. [NEW — today: ?]

**Monk mode**
- **R-MONK-1** The monk-mode block is system-owned; the `monk` action is not offered inside it. [NEW]

---

## 4. Open questions for the owner

1. Merge *quota* and *allowance* into one term?
2. Define *no-escape* vs *strict mode*.
3. Is Micromanager-style "only these apps" wanted, and is it a block kind or a Frozen Chicken mode?
4. Should KC9K have a Writer-style distraction-free mode (Cold Turkey Writer)?
5. Default lists: which ones, and who maintains them?
6. Rate threshold: replace hits/threshold or sit beside them?
7. Confirm the [CT] items against getcoldturkey.com (allow that domain in the environment and I'll check).
