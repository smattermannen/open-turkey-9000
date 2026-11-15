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
  refused only while locked; re-lock on setting locks (browser settings,
  tamper, strict mode, global ladder: `--relock D`; unlock lifts, setting
  stays on; off meanwhile ends it); layered order on blocks (R-LOCK-6/7:
  `db/layers.go`, order of the lock flags, `unlock` lifts only the first,
  duration timer and restart behind others count from their unlocking, new
  locks last; the daemon moves layers on; old blocks: timer, restart,
  password, text, delay), and on setting locks (browser settings, tamper,
  strict mode, global ladder); active vs enabled for schedules and places
  (`enabledBlocks`; the daemon no longer switches blocks; upgrade switched
  scheduled blocks on once); allowance sessions lift their block (it stays
  on). Not built: hardware/internet clock check (see §5); choosing the order in the GUI (fixed order). (L)
- [ ] **Android extension can be switched off** (§3.13): built (5 s grace
  once checked in, pauses doubling 2 min → 1 h, reset after 10 min unbroken;
  `ExtensionGuard`). Unchecked: unit tests not run (no JDK/Android SDK on the
  laptop overnight): run `./gradlew testDebugUnitTest`. (S)
- [ ] **Page reading slow on big pages** (principle 9). Done: the first
  read of a page goes in ≤ 8 ms pieces in idle time (`readPageText`,
  `READ_SLICE_MS`), then only added visible text, full re-read ≤ every 30 s;
  one-word terms via `kc9kWordIndex`; phrases split the text once per page
  and look words up in an index (100 phrases + 1,000 terms on a 3,000-word
  page: 115 ms → 4 ms, timed in `ci/match_test.js`). Left: judging a huge
  page (40,000 words) is still one piece (~37 ms). (M)
- [ ] **Gigantic URL lists** (principle 9). Extension: done (`kc9kIndex`,
  timed at 100,000 in `ci/match_test.js`). Firewall: done (nft sets,
  iptables fallback); the CLI no longer fills it (`deferFirewall`), the
  daemon does within a cycle with its 30 min lookup cache, so `start`
  doesn't wait for DNS. `/etc/hosts`: measured 15 ms per name lookup at
  100,000 sites, so capped at `MaxHostsDomains` (20,000, ~3 ms; see §5);
  drift check one pass (was quadratic). 100,000-site test:
  `blocker/biglist_test.go`. Left: browser policy with 100,000 entries:
  Unchecked. (M)
- [ ] **R-LOCK-12 gaps**: manual `kc9k monk` now confirmed (`--yes`), and
  `project import` refuses monk steps/actions without `--no-confirm-ok`
  (`projectMonk`); both in `confirm_test.go`. `block import` the same. The
  hotkey press dialog (`blocker.ConfirmDesktop`) ran live on the laptop
  (2026-11-15): zenity shows "Lock 'B'?" with target, kind and length;
  unanswered (screen locked) it times out after 60 s and nothing is locked;
  synthetic keys from other programs are ignored. Left: a person clicks
  Lock and Cancel on an unlocked screen. (S)

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
- [ ] Block unsupported browsers (`kc9k tamper unsupported-browsers`): upgrade from `force-librewolf` keeps state and lock; Firefox and a portable LibreWolf close at once; LibreWolf/Chrome with the extension stay; Browsers list check-ins. `kc9k browsers install chromium|chrome` never run (Mint's chromium .deb assumed at `/usr/lib/chromium/chromium`, reading `/etc/chromium/policies`).
- [ ] Monk mode live (`ci/monk_live_test.sh` unwritten), incl. reboot mid-way.
- [ ] GUI design pass: pages since 2026-11-10 never seen on a real screen.
- [ ] Lists without types (§3.2): after the upgrade the split `Blocked` /
  `Blocked terms` pairs of `H` and `test` are one `Blocked` each; check
  pages are blocked as before. New list dialog with scope boxes on a real
  screen.
- [ ] Active vs enabled on the laptop: after the upgrade, blocks with
  windows are on (`status`: "lifted by its schedule" outside windows) and
  enforce only in their windows; GUI shows "lifted by its schedule now".
- [ ] Remove laptop leftovers (`quotatest`, `hej`, `Papers` list in `test`). `extras_test` fails one case only because the locked `test` block lists example.com.

## 1. `SPEC.md` coverage

Only items not simply Built.

| Item | Status | Notes |
|---|---|---|
| §3.1 File URL entries | Partial | `file://*` default exception built; file URLs in blacklists block nothing |
| §3.1 Folder entry | Partial | programs closed; windows showing it not |
| §3.2 Template | Not built | |
| §3.2 Affected users | Not built | |
| §3.2 Test mode for phrase sets | Unchecked | lists built |
| §3.3 Enabled/disabled/lifted words | Partial | |
| §3.3 Continuous/scheduled types | Partial | type follows the block's windows/place rules (no explicit type field); schedules and allowance sessions lift, never switch |
| §3.3 Duplicate starts inactive | Unchecked | |
| §3.3 `kc9k toggle` | Not built | |
| §3.4 Range lock | Not built | |
| §3.4 Layered sequential locks | Partial | blocks and setting locks built (CLI flag order); GUI fixed order; one lock per kind |
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
| §4.10 Tamper settings | Partial | GUI group with OK/Cancel window: clock (`tamper clock`) and block unsupported browsers (`tamper unsupported-browsers`, own lock carried over from `force-librewolf`, `tamper unlock unsupported-browsers`; old command hidden alias). No group lock: clock has no lock flags. `gui-state` has no "anything locked" flag, so the window can't show which are stuck on (the command refuses). Grace fixed 60 s; private windows/Tor, guest profiles, extension stop, `file://`, installer, inactive tabs, embedded content, task managers: no command |
| §4.10 Extension install dialog | Not built | |
| §4.10 Device protection: factory reset protection, guided setup | Not built | 2.6 |
| §4.10 Browsers list | Built | GUI Browsers group and `kc9k browsers`: LibreWolf, Chrome, Chromium, installed, per-browser extension check-in, Install (`browsers install`); live check in 0.2 |
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
- **2.5 Layered sequential locks** (M): blocks and setting locks done (stored
  order, `unlock` lifts only the first, status and GUI show the order). Left:
  order chosen in the GUI; several locks of one kind.
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

- DECIDED OVERNIGHT: how to check the clock against hardware and internet
  time (§4.10) → not built yet; trusted time stays CLOCK_BOOTTIME with
  powered-off time not counted, because that is the strictest choice: a
  hardware-clock count would only end timer locks sooner and the hardware
  clock can be set in firmware without a firmware password, and an internet
  time query is a new outside query (principle 6) the owner should agree to.
- DECIDED OVERNIGHT: a huge list in /etc/hosts makes every name lookup on
  the computer slow (15 ms each at 100,000 sites) → the hosts layer holds
  at most 20,000 sites, the rest are blocked by the browser policy, the
  extension and the firewall, because principle 9 says huge lists never
  slow the computer and those layers already cover every site.
