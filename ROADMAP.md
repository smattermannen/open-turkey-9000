# KC9K Roadmap

Where the code stands against `SPEC.md`, and what comes next. This is the
**only** file that holds status. `SPEC.md` says what KC9K should be;
`README.md` says how to use what is built.

**Rules for this file**

1. Every item points to the part of `SPEC.md` it serves (§ or rule ID). Work
   that serves nothing in `SPEC.md` is raised with the owner before it is
   built, and listed under [Proposed](#5-proposed-not-agreed) meanwhile.
2. Update this file in the same commit as the code it describes.
3. A status is a claim about the code. If you find one wrong, fix it; don't
   build on it. Write "Unchecked" rather than guess.
4. Order of work: section 0 before anything else (principle 1), top to
   bottom, then section 2.

Status: **Built**, **Partial**, **Differs** (built, but not as `SPEC.md`
says), **Not built**, **Unchecked** (nobody has compared it with the code).
Sizes: S under a day, M a few days, L a week or more.

Last checked against the code: 2026-11-13.

---

## 0. Fix what exists

### 0.1 Code that contradicts `SPEC.md`

- [ ] **The lock model** (§3.3, §3.4, R-LOCK-1 to 11). Today:
      - a block is protected without a lock: while its schedule window is
        open, while a place rule has it on, and always once it has allow
        windows (weakening is queued for 24 h: `db.DefaultLoosenDelay`,
        `gateEdit`, `pending`, `--now`); a built-in 2 h edit freeze
        (`schedule.EditFreeze`) comes before every window, Frozen Chicken
        schedules included. All of that goes: only locks protect, and a
        schedule lock's lead time is the user's;
      - a timer lock ending switches the block off, and `unlock` of a text
        or password lock switches the block off too. A lock must only be
        lifted; the block goes on until its schedule or the user ends it;
      - the delay lock counts from `stop`, not from asking to unlock;
      - trusted time counts only time the computer is on or suspended; it
        must be the real clock, with the tamper settings of §4.10;
      - `kc9k break relock` is a text break (rename it); **re-lock**, the
        lock option, is not built;
      - uninstalling is refused in schedule windows and for allow windows,
        not only while something is locked (R-LOCK-9). (L)
- [ ] **Monk mode** is an ordinary block named `monk-mode`, editable while
      on; R-MONK-1 makes it system-owned, without the `monk` action inside
      it, and §3.8 allows exceptions and apps only while it is off. (S)
- [ ] **R-LIST-3**: new blocks don't get the exception `file://*`. (S)
- [ ] **R-SCHED-2** and §3.6: block windows may overlap, and times are not
      held to 5-minute steps. (S)
- [ ] **Text lock** (§3.4): random characters only, 10–5000. Random words,
      custom text and lengths from 1 are missing. (S)
- [ ] **Window titles** (R-TITLE-1) also act in browsers, through the
      extension. They must leave browsers to term lists. (S)
- [ ] **Tamper** (§3.10): every daemon restart within a boot counts, an
      upgrade included. (S)
- [ ] **Lists by type** (§3.2): a list ticks url, title and text with mixed
      entries; it must be an address, term or search list. (M)
- [ ] **Project** (§3.13): export leaves out locks, passwords, hotkeys and
      statistics; it must offer *backup* (everything) and *share* (without
      places and user names), with a preview on import. (M)
- [ ] **Words** (§3.14): commands, GUI labels, messages and `README.md` still
      say active (for enforcing), char lock, time lock, held, quota,
      `break allowance` (a reward), `break relock` (a text break), option,
      browser option, template (for shipped lists, which `SPEC.md` calls
      categories), loosen, bypass detection; `kc9k nepsis` becomes
      `kc9k bypass`. Commands get the glossary's names (`--text-lock`,
      `list scope`), keeping the old ones for a while. (M)
- [ ] **Warnings** (R-WARN-1): a scheduled freeze is announced 5 minutes
      ahead; a list's logout or shutdown is not announced at all. (S)
- [ ] **Page reading is slow on big pages** (principle 9). In every frame,
      `content.js` reads the whole page (`body.innerText`, up to 2 MB, which
      makes the browser lay out the page) every 1.5 s while the page changes,
      so a feed that keeps loading is read again and again. Then
      `match.js` runs one regular expression per term over all of it. Must
      be reworked before phrase sets are built:
      - read only the text that is new since the last scan;
      - one pass over the words, looking each up in one table of all terms
        (prefix and suffix tables for `*`), so the cost doesn't grow with the
        number of terms; phrases checked only where a term was found;
      - work in small pieces in idle time, never blocking scrolling or typing;
      - stop reading a page once its strongest action has been taken;
      - a speed test in `ci/` with a large page and many rules, run on every
        change to the matcher. (M)

- [ ] **Gigantic URL lists are slow** (principle 9). Four places grow with
      the size of a list:
      - **Extension:** every request a page makes (scripts, images,
        frames: often 100–300 per page) is tested against every URL entry,
        one regular expression each (`kc9kMatchesOne` in `match.js`), and the
        request waits for the answer. Needs a lookup table by domain, with
        regular expressions only for entries that really need them.
      - **Firewall:** one `iptables` rule per blocked IP, each added by its
        own process, and every outgoing packet walks the whole chain. Needs
        one set (nftables set or `ipset`) that answers in one step.
      - **DNS when starting a block:** `ApplyFirewall` looks up every
        domain's real address one after another (up to 3 s each when a
        server is slow), so starting a block with thousands of sites can
        take minutes. Needs lookups in parallel, a cache, and the firewall
        filled in afterwards instead of holding up `start`.
      - **`/etc/hosts`:** one line per domain; how fast Mint's resolver
        reads a huge hosts file: Unchecked.
      A speed test in `ci/` with 100,000 entries. (M)
- [ ] **Phrases** (R-PHRASE-1 to 10): today's word combinations sit inside
      title and text lists, need every box, take several words from one box
      (`:N`), and have no length, bridges, URL scope or phrase sets. Rebuild
      them as phrase sets, after the page reading rework. Existing
      combinations are turned into phrases. (L)

### 0.2 Not yet confirmed in real use

Built and tested by scripts, never tried by a person. Each becomes a
checklist entry with the exact commands; what can be scripted goes into `ci/`.

- [ ] A notification before the end of an allowance session.
- [ ] Scheduled Frozen Chicken in logout and shutdown modes.
- [ ] Strict mode holes: `unshare -Urm` (check
      `sysctl kernel.apparmor_restrict_unprivileged_userns`), `--user`
      Flatpak apps are killed, Electron apps aren't taken for browsers,
      another user account and the guest session are blocked too.
- [ ] Shutdown mode and the `kc9k.nofreeze` GRUB escape; measure the fast
      relock.
- [ ] Rendered terms on real sites over a few days (false hits, slow pages); a
      hotkey pressed on the real keyboard.
- [ ] A page that changes address without loading (YouTube) caught by a url
      list; the Websites tab on a real screen (hidden lists, search box).
- [ ] One-off logout and shutdown from a list; the Dashboard's Monk mode
      button; a locked global ladder refusing `nepsis off`.
- [ ] A hotkey pressed under GNOME.
- [ ] Use counting with someone at the computer: the focused tab and a
      blocked app in front count, idle time doesn't; another browser is
      closed during a website session.
- [ ] Mute, declutter and dark on real sites; polite close with a real app.
- [ ] Social presets beyond YouTube Shorts (the selectors in
      `webext/files/social.js` were written from memory).
- [ ] A playlist's videos playing; a block following the user between
      places; a text break (`break relock`) coming back.
- [ ] Google Chrome on the laptop: the extension pinned within seconds,
      can't be removed or disabled, incognito and guest greyed out, a block
      works, toolbar button and popup. Then `ci/browser_test.py` and
      `ci/extras_test.py` again in LibreWolf (`background.js` changed).
- [ ] The GUI's design pass: no page added since 2026-11-10 has been looked
      at on a real screen.
- [ ] Remove the leftovers on the test laptop (`quotatest`, `hej`, the
      `Papers` list in `test`).

---

## 1. `SPEC.md` coverage

### §3.1 Targets

| Item | Status | Notes |
|---|---|---|
| Site, URL entry, wildcard | Built | website lists |
| Search list, search entry | Not built | the `search` detector exists, not the list |
| File URL, `file://*` default | Not built | see 0.1 |
| App (name, path, script, fingerprint, Flatpak, desktop unit) | Built | `blocker/scripts.go`, `blocker/process.go` |
| Folder entry | Partial | programs inside it are closed; windows showing it are not |
| Window title entry | Differs | X11 only; also acts in browsers, see 0.1 |
| Rendered term | Built | |
| Phrase (formerly word combination) | Differs | built inside text and title lists: every box necessary, no length, `:N` takes several words from one box, no bridges, no URL |
| Phrase set | Not built | |

### §3.2 Grouping

| Item | Status | Notes |
|---|---|---|
| Block, list, scope, blacklist, exception list | Built | |
| Allowlist block | Built | `block create --allowlist` |
| Cluster | Built | |
| Category | Partial | 14 shipped lists, offered only when creating a block (`--template`) |
| Address, term and search lists | Differs | see 0.1 |
| Phrase sets in clusters and categories | Not built | |
| Template (a block definition) | Not built | |
| Affected users | Not built | |
| Social preset, allowed playlist | Built | see 0.2 |
| Test mode | Differs | per block (`kc9k list test`); must be per list and phrase set |

### §3.3 Block states

| Item | Status | Notes |
|---|---|---|
| Active / inactive (`start` / `stop`) | Built | the code calls these active and inactive too, but also uses "active" for enforcing |
| Enabled / disabled, lifted | Partial | enforcing is tracked; the words aren't used |
| Continuous / scheduled types | Not built | schedules start and stop blocks |
| Locked | Differs | the code also holds unlocked blocks; see 0.1 |
| Rename, duplicate | Built | that a duplicate starts disabled: Unchecked |
| `kc9k toggle` | Not built | |

### §3.4 Locks

| Item | Status | Notes |
|---|---|---|
| Timer lock (`--for`, `--until`) | Differs | ends the block; counts only time switched on; see 0.1. Confirmation over 24 h: Unchecked |
| Text lock | Partial | see 0.1 |
| Range lock | Not built | |
| Delay lock | Differs | `start --delay`, 1 min to 40 days, counted from `stop` |
| Schedule lock | Differs | built in, fixed 2 h; see 0.1 |
| Re-lock | Not built | |
| Restart lock | Built | |
| Password lock | Built | |
| Layered sequential locks | Partial | locks stack, but `unlock` checks them all at once in a fixed order (`cli/control.go`) |
| Trusted time | Differs | counts only time the computer is on; see 0.1 |
| While any block or setting is locked (R-LOCK-9) | Partial | uninstalling is refused (also without a lock, see 0.1); extension and protection settings: Unchecked |

### §3.6–3.7 Time, breaks and use

| Item | Status | Notes |
|---|---|---|
| Block window, allow window | Built | see 0.1 for overlap and 5-minute steps |
| Break per block window | Not built | |
| Pomodoro, long rest | Built | the R-SCHED-3 rhythm: Unchecked |
| Allowance | Partial | daily only (`kc9k quota`); no rolling, window, week or month periods, no refill time |
| Reward | Partial | `kc9k break allowance`, earned only while in use (R-BREAK-2); capped at one reward instead of adding up and expiring per window/day/week/month |
| Text break | Built | as `kc9k break relock` |
| Delay break | Not built | |
| Session limit | Not built | |
| Pause | Built | |
| Autostart | Not built | |
| Place, place rule | Built | Wi-Fi only |
| Use | Partial | 3 minutes without input everywhere; statistics leave out sound in background tabs |
| Strict use counting | Not built | |

### §3.8 Device-level

| Item | Status | Notes |
|---|---|---|
| Frozen Chicken: lock screen, logout, shutdown, scheduled | Built | a running freeze can't be ended; locks on scheduled freezes: Not built |
| Warning 60 s before logout, shutdown, freeze (R-WARN-1) | Differs | see 0.1 |
| Bypass prevention notices at every event and step (R-WARN-2) | Built | one notification per new event across ladders (`checkScopes`) |
| Allowed apps only | Not built | |
| Monk mode | Differs | see 0.1 |
| Consequences | Built | `enforce/consequence.go` |
| Strict mode | Partial | a voluntary switch with its own setting lock (`kc9k strict on --for …`, `strict unlock`). Exempt folders: Not built |
| Recovery of a forgotten password (§3.11) | Partial | root can remove it while nothing is locked |
| Wayland warning (R-DESKTOP-1) | Unchecked | |
| No-escape | Built | |
| Factory reset protection | Not built | section 2.6 |

### §3.9 Actions

| Item | Status | Notes |
|---|---|---|
| Block page, close tab, redirect, image | Built | |
| Look actions | Built | see 0.2 |
| Close browser, logout, shutdown, monk | Built | |
| No downloads | Not built | section 2.3 |
| Threshold, hit count (rendered terms) | Built | per action; both must be met (R-LIST-6) |
| Strongest action across blocks | Unchecked | close tab, image, redirect, block page is built within a page |
| Matching details (R-LIST-4, R-LIST-5) | Unchecked | case is ignored; accents, look-alike letters, hidden text, frames: Unchecked |
| Threshold (phrase sets) | Not built | |
| Rate | Not built | |
| Ad trigger | Not built | section 2.2 |

### §3.10–3.13 Bypass prevention, passwords, recording, the program

| Item | Status | Notes |
|---|---|---|
| Bypass prevention: events, detectors, ladder, steps, quiet time | Built | as `kc9k nepsis`; see 0.1 for the name |
| Workaround detection | Built | proxies, mirrors, front-ends, fetch tools |
| Settings password | Built | needed to set a lock only with `kc9k password locks on` |
| Access passwords, setting locks | Built | |
| No weakening during active locks (R-WEAK-4) | Built | `kc9k no-weakening`: strict mode, browser settings, global bypass prevention, settings password. Other settings (stats, options, no-escape uses its own rule): Unchecked |
| Event counting (R-EVENT-1) | Unchecked | |
| Link recording | Built | |
| Word recording, specificity, cut | Not built | |
| Four layers, daemon | Built | `internal/blocker/` |
| Extension in LibreWolf | Built | mandatory, `extguard.go` |
| Extension in Google Chrome / Chromium | Built | 2026-11-12; see 0.2 |
| Browser settings | Built | |
| SafeSearch | Partial | Google only; section 2.4 |
| Behaviour settings | Built | called options |
| DNS server setting | Built | `kc9k dns`, Settings > DNS |
| Tamper settings (§4.10) | Not built | |
| Hotkeys | Built | Cinnamon tried; GNOME see 0.2 |
| Project export / import | Differs | see 0.1 |
| `.deb` package | Not built | |
| Circadian rhythm | Not built | section 2.1 |

### §4 Functions (interface)

| Item | Status | Notes |
|---|---|---|
| 4.1 Overview | Partial | Dashboard; "windows still to come today": Unchecked |
| 4.3 Block editor: draft with Save / Cancel | Not built | every change applies at once |
| 4.3 Block editor: Ctrl+F, multi-select, double-click to edit | Unchecked | search boxes exist |
| 4.3 Import from KC9K / file, export | Partial | file import and export built; "From KC9K" Unchecked |
| 4.5 Schedule grid | Partial | drag to add, click to remove, presets; drag to move, copy to days, similar windows together: Not built |
| 4.9 Statistics | Partial | time per site and app, presets of days, on/off/clear, JSON; per user, times blocked, top 5/10, filter by block, private windows: Not built |
| 4.10 Preferences | Partial | theme light / dark / system; 24-hour clock, first day, darkness slider, open at login: Unchecked; tray: Not built |
| 4.10 Notifications | Partial | fixed 5 minutes ahead; choosing which and when: Not built |
| 4.10 Protection settings | Partial | close browsers without the extension (grace fixed at 60 s) is built; the others: Not built |
| 4.10 Dark pages (part of circadian rhythm) | Not built | the `dark` action exists per list |

---

## 2. Next features

After section 0. Each item's details are in `SPEC.md`; here only the steps.

### 2.1 Circadian rhythm (L) — §4.10

- [ ] Time of day model (morning, afternoon, evening, night, user-set
      times), reusing `internal/schedule/`.
- [ ] Blue light filter (Cinnamon's night light), screen brightness
      (`/sys/class/backlight`; something else for external screens),
      wallpaper (`org.cinnamon.desktop.background`), GTK colour scheme,
      the `dark` action on every page during chosen hours.
- [ ] Each lockable like a browser setting.

### 2.2 Ads as a trigger (M) — §3.9

- [ ] A fourth scope: the ad networks a page loads (the extension already
      sees third-party requests through `webRequest`).
- [ ] A shipped list of ad networks per industry (gambling, adult, …).

### 2.3 No downloads (S) — §3.9

- [ ] The extension cancels downloads from matching pages (`downloads`
      permission).

### 2.4 SafeSearch beyond Google (S) — §4.10

- [ ] Bing, DuckDuckGo and YouTube Restricted Mode, each lockable.

### 2.5 Layered sequential locks (M) — §3.4, R-LOCK-6, R-LOCK-7

- [ ] Store the user's order of locks; `unlock` lifts only the first
      remaining one (R-LOCK-6, R-LOCK-7).
- [ ] A timer lock with a date waits for the date; one with a duration
      counts from when the lock before it was unlocked.
- [ ] The same for browser settings and setting locks (the global ladder
      included).
- [ ] `kc9k status` and the interface show the order and which lock is next.

### 2.6 Factory reset protection (M) — §3.8

- [ ] Firmware lock where `/sys/class/firmware-attributes/` allows it
      (admin password given to someone the user trusts, never random; no
      USB or network boot; fixed boot order); a guide elsewhere.
- [ ] Boot order guard: put back the EFI boot order, clear `BootNext`.
- [ ] Installer media guard: block OS image sites, close USB writing tools
      (mintstick, balenaEtcher, Ventoy, Popsicle, Startup Disk Creator,
      GNOME Disks restoring an image).
- [ ] Other systems in GRUB behind the password.
- [ ] Root paths: hide recovery entries, lock `init=`/`single` editing, lock
      root login on TTYs and `su` during a lock. Needs a disposable machine.
- [ ] A guided setup when turning on strict mode or no-escape.

### 2.7 The rest of section 1

Everything marked Not built or Partial above, in an order agreed with the
owner.

---

## 3. Before release

- [ ] Build without `TAGS=devmaster` (no master password).
- [ ] Remove `ci/vm.sh`.
- [ ] Rename the GitHub repository after the app's name (`SPEC.md` §7: KC9K or Nepsis).
- [ ] On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
- [ ] A pass over `README.md`; a tagged release.

## 4. After publishing

- **Page score (L)** — §1.4. Needs 2.2 first.
- **Windows support (L)** — §1.4. A port behind a platform layer: everything
  in `internal/blocker/` is Linux-specific.

## 5. Proposed, not agreed

Ideas from earlier sessions that nothing in `SPEC.md` asks for yet. Raise
them with the owner before building; move an item into `SPEC.md` (and out of
here) once agreed, or delete it.

- **Rules server secret.** Any local program can send the
  `X-KC9K-Extension` header to `127.0.0.1:47613`: read the lists, post fake
  statistics, or make the extension guard believe a browser checked in. A
  secret given to the extension by policy would stop that.
- **Audit the sudo rule.** Anything running as the user can run `kc9k` as
  root: check every command for hostile arguments (symlinks, `..`, files
  written under `/etc`). Keep `env_reset` (never `sudo -E`).
- **A first-run wizard** when there are no blocks.
- **The typing challenge in its own GTK window** instead of a terminal.
