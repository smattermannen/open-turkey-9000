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
4. Order of work: section 0 before anything else (principle 1), then section 2.

Status: **Built**, **Partial**, **Differs** (built, but not as `SPEC.md`
says), **Not built**, **Unchecked** (nobody has compared it with the code).
Sizes: S under a day, M a few days, L a week or more.

Last checked against the code: 2026-11-13.

---

## 0. Fix what exists

### 0.1 Code that contradicts `SPEC.md`

- [ ] **Fixed 2 h edit freeze** (`schedule.EditFreeze`, also used for
      Frozen Chicken schedules). §4.8a and R-LOCK-10: a schedule lock has a
      lead time the user chooses, none by default. (M)
- [ ] **Fixed 24 h loosening delay** for blocks with allow windows
      (`db.DefaultLoosenDelay`, `kc9k pending delay`). Only a delay lock the
      user set delays a weakening (R-LOCK-6, R-LOCK-10). Depends on
      `SPEC.md` open decision 2. (M)
- [ ] **Use** (§3.7): input in the last 3 minutes. The code uses 1 minute
      for allowances (`quota`) and 2 minutes for statistics. (S)
- [ ] **Monk mode** is an ordinary block named `monk-mode`; R-MONK-1 makes it
      system-owned, without the `monk` action inside it. (S)
- [ ] **R-LIST-3**: new blocks don't get the exception `file://*`. (S)
- [ ] **R-SCHED-2** and §3.6: block windows may overlap, and times are not
      held to 5-minute steps. (S)
- [ ] **Text lock** (§3.4): random characters only, 10–5000. Random words,
      custom text and lengths from 1 are missing. (S)
- [ ] **Words** (§3.14): commands, GUI labels, messages and `README.md` still
      say char lock, time lock, quota, `break allowance` (a reward),
      template (for shipped lists, which `SPEC.md` calls categories),
      loosen, bypass detection. Prose and labels can change now; renaming
      commands (`kc9k quota`, `break allowance`, `--template`) needs a plan
      agreed with the owner. (M)

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
- [ ] Page words on real sites over a few days (false hits, slow pages); a
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
      places; relock after a break.
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
| Site, URL entry, wildcard, word combination | Built | website lists |
| Search entry | Not built | the `search` detector exists, not the entry |
| File URL, `file://*` default | Not built | see 0.1 |
| App (name, path, script, fingerprint, Flatpak, desktop unit) | Built | `blocker/scripts.go`, `blocker/process.go` |
| Folder entry | Partial | programs inside it are closed; windows showing it are not |
| Title entry | Built | X11 only |
| Page word | Built | |

### §3.2 Grouping

| Item | Status | Notes |
|---|---|---|
| Block, list, scope, blacklist, exception list | Built | |
| Allowlist block | Built | `block create --allowlist` |
| Cluster | Built | |
| Category | Partial | 14 shipped lists, offered only when creating a block (`--template`) |
| Default list | Not built | |
| Template (a block definition) | Not built | |
| Affected users | Not built | |
| Social preset, allowed playlist | Built | see 0.2 |
| Test mode | Built | `kc9k list test` |

### §3.3 Block states

| Item | Status | Notes |
|---|---|---|
| Enabled / disabled, continuous / scheduled | Not built | blocks are started and stopped; schedules start them |
| Active, locked, held | Built | |
| On break | Built | |
| Rename, duplicate | Built | that a duplicate starts disabled: Unchecked |
| `kc9k toggle` | Not built | |

### §3.4 Locks

| Item | Status | Notes |
|---|---|---|
| Timer lock (`--for`, `--until`) | Built | |
| Text lock | Partial | see 0.1 |
| Range lock | Not built | |
| Delay lock | Built | `start --delay`, 1 min to 40 days |
| Schedule lock | Differs | fixed 2 h, see 0.1 |
| Restart lock | Built | |
| Password lock | Built | |
| Layered sequential locks | Partial | locks stack, but `unlock` checks them all at once in a fixed order (`cli/control.go`) |
| Trusted time | Built | |
| "While any lock is in force" (uninstall, extension, protection, escape routes) | Unchecked | uninstall refusal is built; the rest depends on `SPEC.md` open decision 1 |

### §3.6–3.7 Time, breaks and use

| Item | Status | Notes |
|---|---|---|
| Block window, allow window | Built | see 0.1 for overlap and 5-minute steps |
| Break per block window | Not built | |
| Pomodoro, long rest | Built | the R-SCHED-3 rhythm: Unchecked |
| Allowance | Partial | daily only (`kc9k quota`); no rolling, window, week or month periods, no refill time |
| Reward | Partial | `kc9k break allowance`; the expiry rule: Unchecked |
| Text break, delay break, session break | Not built | |
| Relock | Built | `kc9k break relock` |
| Pause | Built | |
| Autostart | Not built | |
| Place, place rule | Built | Wi-Fi only |
| Use | Differs | see 0.1 |
| Strict use counting | Not built | |

### §3.8 Device-level

| Item | Status | Notes |
|---|---|---|
| Frozen Chicken: lock, logout, shutdown, scheduled | Built | |
| Allowed apps only | Not built | |
| Monk mode | Differs | see 0.1 |
| Consequences | Built | `enforce/consequence.go` |
| Strict mode | Built | a separate switch (`kc9k strict on`) |
| No-escape | Built | |
| Factory reset protection | Not built | section 2.6 |

### §3.9 Actions

| Item | Status | Notes |
|---|---|---|
| Block page, close tab, redirect, image | Built | |
| Look actions | Built | see 0.2 |
| Close browser, logout, shutdown, monk | Built | |
| No downloads | Not built | section 2.3 |
| Threshold, hit count | Built | |
| Rate | Not built | |
| Ad trigger | Not built | section 2.2 |

### §3.10–3.13 Bypass prevention, passwords, recording, the program

| Item | Status | Notes |
|---|---|---|
| Bypass prevention: events, detectors, ladder, steps, quiet time | Built | `kc9k nepsis` |
| Workaround detection | Built | proxies, mirrors, front-ends, fetch tools |
| Settings password, access passwords, setting locks | Built | |
| Link recording | Built | |
| Word recording, specificity, cut | Not built | |
| Four layers, daemon | Built | `internal/blocker/` |
| Extension in LibreWolf | Built | mandatory, `extguard.go` |
| Extension in Google Chrome / Chromium | Built | 2026-11-12; see 0.2 |
| Browser options | Built | |
| SafeSearch | Partial | Google only; section 2.4 |
| Options | Built | |
| Hotkeys | Built | Cinnamon tried; GNOME see 0.2 |
| Project export / import | Built | |
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
| 4.10 Dark pages setting | Not built | the `dark` action exists per list |

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
- [ ] Each lockable like a browser option.

### 2.2 Ads as a trigger (M) — §3.9

- [ ] A fourth scope: the ad networks a page loads (the extension already
      sees third-party requests through `webRequest`).
- [ ] A shipped list of ad networks per industry (gambling, adult, …).

### 2.3 No downloads (S) — §3.9

- [ ] The extension cancels downloads from matching pages (`downloads`
      permission).

### 2.4 SafeSearch beyond Google (S) — §4.10

- [ ] Bing, DuckDuckGo and YouTube Restricted Mode, each lockable.

### 2.5 Layered sequential locks (M) — §3.4, R-LOCK-11

- [ ] Store the user's order of locks; `unlock` lifts only the first
      remaining one.
- [ ] Decide with the owner: does a timer lock in the middle start counting
      when it is reached, or from the start?
- [ ] The same for browser options and setting locks (the global ladder
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
- [ ] Rename the GitHub repository to `kc9k`.
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
- **A `.deb` package** (`make deb`) whose `prerm` runs `prepare-uninstall`.
- **A first-run wizard** when there are no blocks.
- **The typing challenge in its own GTK window** instead of a terminal.
