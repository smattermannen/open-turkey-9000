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
      - done for blocks: the **schedule lock** (`kc9k schedule lock B on
        --lead D`) replaces the built-in schedule/place protection, the 2 h
        edit freeze and the 24 h allow-window queue; upgrading gives blocks
        that had schedules or place rules a schedule lock with a 2 h lead.
        Frozen Chicken schedules have their own lock with a lead time too
        (`--lock-lead`, `freeze schedule lock`); older ones keep 2 h;
      - done: `unlock` lifts text and password locks and leaves the block
        on; the delay lock counts down from `unlock` and is then lifted
        (`delay_until`); a timer lock ending lifts only the lock (only a
        schedule or monk mode's own end switches a block off);
      - done: trusted time is the real clock unless the tamper setting
        `kc9k tamper clock` is on (then clock changes are ignored while a
        timer lock runs, and powered-off time isn't counted; installs from
        before have it on). Not built: checking against the hardware clock
        and internet time (§4.10), which would let powered-off time count
        with protection on;
      - done: `kc9k break text` (alias `relock`), and **re-lock** for the
        text and password locks of blocks (`start --relock D`, GUI start
        dialog). Not built: re-lock on setting locks;
      - done: uninstalling is refused only while something is locked
        (R-LOCK-9);
      - not built: blocks are still switched on and off by their schedule
        (the daemon activates them); §3.3 wants active (the user's switch)
        apart from enabled (what the schedule says), and layered locks in
        the user's order (R-LOCK-6, R-LOCK-7). (L)
- [ ] **Lists by type** (§3.2): address and term lists are built (a list
      can't look at the address and the page both; upgrading split mixed
      lists and clusters losslessly into `<name>` and `<name> terms`).
      Left: search lists (a new type), and a term list's url scope
      (terms in the decoded address, R-PHRASE-6). (M)
- [ ] **Project** (§3.13): *backup* (with places and place rules) and
      *share* (`--share`, without them) are built, with a preview on import
      and new names for taken block names. A backup still leaves out
      running locks, passwords, hotkeys and statistics: whether "everything"
      includes those is for the owner to say. (S)
- [ ] **Words** (§3.14): done in commands, messages and GUI labels: time
      lock, char lock, held, loosen, bypass detection, browser option;
      `kc9k bypass` (alias `nepsis`), `--text-lock`, `list scope` /
      `cluster scope` (alias `where`), `break text` (alias `relock`).
      Also done: `kc9k allowance` (alias `quota`), `break reward` (alias
      `allowance`) and their messages. Left: option (behaviour setting),
      template (category), active for enforcing, and `README.md` prose. (S)
- [ ] **Page reading is slow on big pages** (principle 9). In every frame,
      `content.js` reads the whole page (`body.innerText`, up to 2 MB, which
      makes the browser lay out the page) every 1.5 s while the page changes,
      so a feed that keeps loading is read again and again. Then
      `match.js` runs one regular expression per term over all of it. Must
      be reworked before phrase sets are built:
      - done: the page is read whole once per address, then only the
        visible text of what is added (`readPageText` in `content.js`), with
        a whole re-read at most every 30 s for text changed in place;
      - done: one pass over the words, each looked up in tables of all
        one-word terms (`kc9kWordIndex`: exact, prefix, suffix, and globs by
        their first or last part): 1,000 terms on 40,000 words went from
        1.6 s to 37 ms. Terms of several words, with symbols, and
        combinations still use one regular expression each;
      - done: scans run in idle time (`requestIdleCallback`), within 0.5 s
        of new text; reading stops once the page is taken away or every
        list has done all it can. Left: splitting one big first read into
        pieces under 16 ms;
      - a speed test in `ci/` with a large page and many rules, run on every
        change to the matcher. (M)

- [ ] **Gigantic URL lists are slow** (principle 9). Four places grow with
      the size of a list:
      - **Extension:** done. `kc9kIndex` looks sites up by host suffix and
        files path entries under their domain; only wildcard entries are
        tried one by one. `ci/match_test.js` checks it against the regular
        expressions and times 100,000 entries.
      - **Firewall:** done. With `nft` (Mint has it), the addresses live in
        sets of our own table `inet kc9k`, loaded in one atomic `nft -f`
        and looked up in one step per packet; a drop there is final, so no
        rule has to come first. The iptables chain stays as the fallback.
      - **DNS when starting a block:** lookups now run 16 at a time and are
        cached for 30 minutes (`resolveAll`); the firewall is still filled
        in before `start` returns instead of afterwards.
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
| File URL, `file://*` default | Partial | every new block has the exception `file://*` (R-LIST-3); file URL entries in blacklists don't block anything yet |
| App (name, path, script, fingerprint, Flatpak, desktop unit) | Built | `blocker/scripts.go`, `blocker/process.go` |
| Folder entry | Partial | programs inside it are closed; windows showing it are not |
| Window title entry | Built | X11 only; never in browsers (R-TITLE-1) |
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
| Address, term and search lists | Partial | address and term lists; search lists: Not built |
| Phrase sets in clusters and categories | Not built | |
| Template (a block definition) | Not built | |
| Affected users | Not built | |
| Social preset, allowed playlist | Built | see 0.2 |
| Test mode | Built | per list (`kc9k list test B L on`, a box on each list); phrase sets: with them |

### §3.3 Block states

| Item | Status | Notes |
|---|---|---|
| Active / inactive (`start` / `stop`) | Built | the code calls these active and inactive too, but also uses "active" for enforcing |
| Enabled / disabled, lifted | Partial | enforcing is tracked; the words aren't used |
| Continuous / scheduled types | Not built | schedules start and stop blocks |
| Locked | Built | only locks hold a block (the schedule lock included) |
| Rename, duplicate | Built | that a duplicate starts disabled: Unchecked |
| `kc9k toggle` | Not built | |

### §3.4 Locks

| Item | Status | Notes |
|---|---|---|
| Timer lock (`--for`, `--until`) | Built | follows trusted time; over 24 h asks to confirm (`--yes`); only a schedule ends a block it started |
| Text lock | Built | random characters, random words (`--lock-words`) or custom text (`--lock-text`), 1–5000 |
| Range lock | Not built | |
| Delay lock | Built | `start --delay`, 1 min to 40 days, counted from `unlock` |
| Schedule lock | Built | `kc9k schedule lock B on --lead D`, Schedule tab |
| Re-lock | Partial | text and password locks of blocks (`start --relock`); setting locks: Not built |
| Restart lock | Built | |
| Password lock | Built | |
| Layered sequential locks | Partial | locks stack, but `unlock` checks them all at once in a fixed order (`cli/control.go`) |
| Trusted time | Built | the real clock; with `kc9k tamper clock` on, clock changes are ignored while a timer lock runs |
| While any block or setting is locked (R-LOCK-9) | Partial | uninstalling is refused only then; tamper protection can't be turned off; the extension and other protection settings: Unchecked |

### §3.6–3.7 Time, breaks and use

| Item | Status | Notes |
|---|---|---|
| Block window, allow window | Built | 5-minute steps, no overlap (R-SCHED-2) in `schedule add`; project import takes windows as they are |
| Break per block window | Not built | |
| Pomodoro, long rest | Built | the R-SCHED-3 rhythm: Unchecked |
| Allowance | Partial | daily only (`kc9k quota`); no rolling, window, week or month periods, no refill time |
| Reward | Partial | `kc9k break allowance`, earned only while in use (R-BREAK-2); capped at one reward instead of adding up and expiring per window/day/week/month |
| Text break | Built | `kc9k break text` (alias `relock`) |
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
| Frozen Chicken: lock screen, logout, shutdown, scheduled | Built | a running freeze can't be ended; a scheduled freeze has a lock with a lead time (other lock kinds on it: Not built) |
| Warning 60 s before logout, shutdown, freeze (R-WARN-1) | Built | list actions wait 60 s after the notice; scheduled freezes are announced at 5 min and 1 min |
| Bypass prevention notices at every event and step (R-WARN-2) | Built | one notification per new event across ladders (`checkScopes`) |
| Allowed apps only | Not built | |
| Monk mode | Built | system-owned block `monk-mode` (R-MONK-1) |
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
| Tamper settings (§4.10) | Partial | `kc9k tamper clock` (block time and date changes); the hardware clock and internet time checks: Not built |
| Hotkeys | Built | Cinnamon tried; GNOME see 0.2 |
| Project export / import | Partial | backup and share; see 0.1 |
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
