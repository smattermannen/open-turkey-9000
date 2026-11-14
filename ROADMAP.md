# KC9K Roadmap

Code status against `SPEC.md`. The only status file.

Rules: every item cites a § or rule ID (unagreed work goes in §5 first).
Update in the same commit as the code. A wrong status is fixed, not built
on; write "Unchecked" rather than guess. Order: §0 first, top to bottom,
then §2.

Status: Built, Partial, Differs (built unlike SPEC), Not built, Unchecked.
Sizes: S < 1 day, M days, L ≥ week. Last checked: 2026-11-14.

## 0. Fix what exists

### 0.1 Code contradicting `SPEC.md`

- [ ] **Lock model** (§3.3, §3.4, R-LOCK-1–11). Done: schedule lock
  (`kc9k schedule lock B on --lead D`) replaces built-in schedule/place
  protection, 2 h edit freeze and 24 h allow-window queue (upgrade gives
  scheduled/placed blocks a 2 h-lead schedule lock); freeze schedules have
  `--lock-lead` (old ones 2 h); `unlock` lifts text/password locks, leaves
  the block on; delay lock counts from `unlock` (`delay_until`); timer lock
  end lifts only the lock; trusted time = real clock unless `kc9k tamper
  clock` (then clock changes ignored during timer locks, powered-off time
  not counted; old installs have it on); `break text` (alias `relock`);
  re-lock for block text/password locks (`start --relock D`); uninstall
  refused only while locked. Not built: hardware/internet clock check; re-lock
  on setting locks; active vs enabled (the daemon still activates blocks by
  schedule); layered order (R-LOCK-6/7). (L)
- [ ] **Lists without types** (§3.2): Differs. Code has address/term lists
  (`web_lists.type`, `list create --terms`, GUI asks the type; upgrade split
  mixed lists into `<name>` and `<name> terms`). Needs: mixed lists, an
  upgrade rejoining split lists where lossless, a create dialog with three
  scope boxes, `--terms` as alias. URL-word matching (`kc9kUrlWords`) exists. (M)
- [ ] **Phrase sets in the GUI** (§4.3): Differs. Unnamed "New phrase set"
  silently creates nothing (`create_phrase_set`). Two editors (typed box and
  pop-up `PhraseDialog`, too tall); want only boxes in the card, unlimited
  word sets (today 9: `MAX_WORD_SETS`, `db.MaxWordSets`; bit mask in
  `kc9kPhraseMatcher` caps at 31). New sets default to text only, so titles
  seem missed. (S)
- [ ] **Declutter**: still present (`db.ActionDeclutter`, `content.js`, GUI).
  Remove; upgrade drops stored ones; imports skip it. (S)
- [ ] **Android extension can be switched off** (§3.13) in IronFox's add-ons
  manager: ~1 free minute every few minutes (15 s stale + 60 s grace, 2 min
  pause). Want: short grace once checked in before; each pause doubles (to
  1 h) until it runs unbroken a while. (S)
- [ ] **Settings groups** (§4.10): GUI doesn't group as SPEC; want a Tamper
  protection group (incl. `tamper clock`) with OK/Cancel window, apart from
  Browser settings. (S)
- [ ] **Block unsupported browsers** (§4.10): built is `force-librewolf`.
  Want a tamper protection setting allowing LibreWolf/Chrome/Chromium with
  the extension, carrying over state and lock. Also the Browsers list with
  one-click install. (M)
- [ ] **Project** (§3.13): backup/share (`--share`), preview, renaming
  built. Left: backup must include running locks, passwords, hotkeys,
  statistics. (S)
- [ ] **Words** (§3.14): done in commands/messages/GUI incl. `bypass`,
  `--text-lock`, `list scope`, `break text`, `allowance`, `break reward`.
  Left: "option" → behaviour setting; "active" used for enforcing. (S)
- [ ] **Page reading slow on big pages** (principle 9). Done: whole read once
  per address, then only added visible text (`readPageText`), full re-read
  ≤ every 30 s; one-word terms via `kc9kWordIndex` (1,000 terms × 40,000
  words: 1.6 s → 37 ms; multi-word/symbol terms and combinations still one
  regex each); scans in `requestIdleCallback` within 0.5 s, stop when done.
  Left: split the first read into < 16 ms pieces; speed test in `ci/`. (M)
- [ ] **Gigantic URL lists** (principle 9). Extension: done (`kc9kIndex`,
  timed at 100,000 in `ci/match_test.js`). Firewall: done (nft sets,
  iptables fallback). DNS on start: 16 parallel, 30 min cache
  (`resolveAll`), but still filled before `start` returns. `/etc/hosts`
  size cost: Unchecked. Left: 100,000-entry speed test in `ci/`. (M)
- [ ] **R-LOCK-12 gaps**: manual `kc9k monk` now confirmed (`--yes`), and
  `project import` refuses monk steps/actions without `--no-confirm-ok`
  (`projectMonk`); both in `confirm_test.go`. `block import` the same. Left: the hotkey
  press dialog is tested only with a stub (`blocker.ConfirmDesktop` is never
  run for real; needs a live check). (S)

### 0.2 Not yet confirmed in real use

Script-tested, never by a person. Script what can be into `ci/`.

- [ ] Notification before an allowance session ends.
- [ ] Scheduled Frozen Chicken in logout/shutdown modes.
- [ ] Strict mode holes: `unshare -Urm` (`kernel.apparmor_restrict_unprivileged_userns`), `--user` Flatpaks killed, Electron not taken for browsers, other accounts and guest blocked.
- [ ] Shutdown mode and `kc9k.nofreeze`; measure fast relock.
- [ ] Rendered terms on real sites for days; hotkey on a real keyboard; hotkey under GNOME.
- [ ] Address change without load (YouTube) caught by a URL list; Websites tab on a real screen.
- [ ] One-off logout/shutdown from a list; Dashboard monk button; locked global ladder refusing `bypass off`.
- [ ] Use counting with a person: focused tab/app count, idle not; other browser closed during a website session.
- [ ] Mute and dark on real sites; polite close with a real app.
- [ ] No-video from page text on a real embedded player (YouTube/Vimeo iframe), in LibreWolf and Chrome (`extras_test.py` checks a local one in LibreWolf).
- [ ] Social presets beyond Shorts (`social.js` selectors written from memory); playlist videos playing; block following places; text break returning.
- [ ] Chrome on the laptop: extension pinned, unremovable, incognito/guest off, block works, popup. Then rerun `browser_test.py`, `extras_test.py` in LibreWolf.
- [ ] Monk mode live (`ci/monk_live_test.sh` unwritten), incl. reboot mid-way.
- [ ] GUI design pass: pages since 2026-11-10 never seen on a real screen.
- [ ] Remove laptop leftovers (`quotatest`, `hej`, `Papers` list in `test`). `extras_test` fails one case only because the locked `test` block lists example.com.

## 1. `SPEC.md` coverage

Only items not simply Built.

| Item | Status | Notes |
|---|---|---|
| §3.1 File URL entries | Partial | `file://*` default exception built; file URLs in blacklists block nothing |
| §3.1 Folder entry | Partial | programs closed; windows showing it not |
| §3.2 Lists with scope, no types | Differs | 0.1 |
| §3.2 Template | Not built | |
| §3.2 Affected users | Not built | |
| §3.2 Test mode for phrase sets | Unchecked | lists built |
| §3.3 Enabled/disabled/lifted words | Partial | |
| §3.3 Continuous/scheduled types | Not built | schedules start/stop blocks |
| §3.3 Duplicate starts inactive | Unchecked | |
| §3.3 `kc9k toggle` | Not built | |
| §3.4 Range lock | Not built | |
| §3.4 Re-lock on setting locks | Not built | |
| §3.4 Layered sequential locks | Partial | `unlock` checks all at once in fixed order (`cli/control.go`) |
| §3.4 R-LOCK-9 | Partial | extension/other protection: Unchecked |
| §3.6 Break per block window | Not built | |
| §3.6 R-SCHED-3 rhythm | Unchecked | |
| §3.6 Allowance | Partial | daily only; no rolling/window/week/month, no refill |
| §3.6 Reward | Partial | capped at one; no accumulate/expire |
| §3.6 Delay break, session limit, autostart | Not built | |
| §3.7 Use | Partial | statistics ignore background-tab sound |
| §3.7 Strict use counting | Not built | |
| §3.8 Scheduled freeze lock kinds beyond lead | Not built | |
| §3.8 Allowed apps only | Not built | |
| §3.8 R-LOCK-12 | Partial | see 0.1 |
| §3.8 Strict mode exempt folders | Not built | |
| §3.8 Forgotten password recovery | Partial | root removes it while nothing locked |
| §3.8 R-DESKTOP-1 | Unchecked | |
| §3.8 Factory reset protection | Not built | 2.6 |
| §3.9 Declutter | Differs | 0.1 |
| §3.9 No downloads | Not built | 2.3 |
| §3.9 Strongest action across blocks | Unchecked | built within a page |
| §3.9 R-LIST-4/5 | Partial | accents not folded (`match.js` only lowercases); look-alikes not built; hidden text, frames Unchecked |
| §3.9 Rate | Not built | |
| §3.9 Ad trigger | Not built | 2.2 |
| §3.11 Block password | Not built | |
| §3.10 R-WEAK-4 beyond strict/browser/ladder/password | Unchecked | |
| §3.10 R-EVENT-1 | Unchecked | |
| §3.12 Word recording, specificity, cut | Not built | |
| §4.10 SafeSearch | Partial | Google only; 2.4 |
| §4.10 Tamper settings | Partial | clock only; grace fixed 60 s; rest Not built |
| §4.10 Browsers list | Not built | |
| §4.11 `.deb` | Not built | |
| §4.1 Windows still to come today | Unchecked | |
| §4.3 Draft Save/Cancel | Not built | changes apply at once |
| §4.3 Ctrl+F, multi-select, double-click edit | Unchecked | |
| §4.3 Import "From KC9K" | Unchecked | file import/export built |
| §4.5 Grid drag-move, copy to days, similar windows | Not built | |
| §4.9 Statistics per user, times blocked, top N, filter, private windows | Not built | |
| §4.10 Preferences | Partial | theme built; clock, first day, slider, login: Unchecked; tray Not built |
| §4.10 Notifications choice | Not built | fixed 5 min |

## 2. Next features (after §0)

- **2.1 Circadian rhythm** (§4.10): built. Untried on real hardware: laptop
  backlight, GNOME/MATE, multiple screens. Wayland: no gamma ramps.
  System-following dark pages rely on `prefers-color-scheme`, which
  LibreWolf reports as light. Not in `project export` yet (S).
- **2.2 Ad trigger** (M): fourth scope from third-party requests
  (`webRequest`); shipped ad-network list per industry.
- **2.3 No downloads** (S): extension cancels downloads (`downloads` permission).
- **2.4 SafeSearch** (S): Bing, DuckDuckGo, YouTube Restricted, each lockable.
- **2.5 Layered sequential locks** (M): stored order; `unlock` lifts only
  the first; dated timer waits for date, duration counts from previous
  unlock; same for settings; status shows order and next.
- **2.6 Factory reset protection** (M): firmware lock via
  `/sys/class/firmware-attributes/` (password given to a trusted person,
  no USB/network boot, fixed order); installer media guard (block OS image
  sites, close mintstick, balenaEtcher, Ventoy, Popsicle, Startup Disk
  Creator, GNOME Disks restore); PAM root guard in `login`, `su`, `su-l`
  via `ApplyLockdown`/`RemoveLockdown` (test on generated config); guided
  setup checklist for strict/no-escape.
- **2.7 Hardening** (M each): rules server secret by policy; audit sudo
  rule arguments (keep `env_reset`); GTK window for the typing challenge.
- **2.8** Everything else Not built/Partial in §1.

## 3. Before release

- [ ] Build without `TAGS=devmaster`.
- [ ] Remove `ci/vm.sh`; on the laptop remove `/etc/sudoers.d/99-claude-test`.
- [ ] Tagged release.

## 4. After publishing

- Windows (L): platform layer; `internal/blocker/` is Linux-only.

## 5. Proposed, not agreed

None.
