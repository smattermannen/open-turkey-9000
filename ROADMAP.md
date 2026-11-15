# KC9K Roadmap

Code status against `SPEC.md`. The only status file.

Rules: every item cites a § or rule ID (unagreed work goes in §5 first).
Update in the same commit as the code. A wrong status is fixed, not built
on; write "Unchecked" rather than guess. Order: §0 first, top to bottom,
then §2.

Status: Built, Partial, Differs (built unlike SPEC), Not built, Unchecked.
Sizes: S < 1 day, M days, L ≥ week. Last checked: 2026-11-15.

## 0. Fix what exists

### 0.1 Code contradicting `SPEC.md`

- [ ] **Owner report 2026-11-15** (real use on the laptop; top priority).
  Status per item; check against the latest build before calling it fixed.
  - [ ] `no-images` doesn't hide GIFs (§3.9 Look actions: animated GIFs
    included). Not built. Cause unknown (CSS `img` vs background/`<video>`
    GIFs, or images already decoded). S
  - [ ] `no-downloads` doesn't block downloads (§3.9). `ci/download_test.py`
    passes, so the real path differs (browser? download started from a
    link/blob, policy-blocked page?). Unchecked which browser. S
  - [ ] Desktop brightness slider gone from the menu; blue-light filter
    toggles on and off; desktop night light turns itself off while its
    switch shows on, **also with circadian off and strict mode off** (§4.10
    circadian: must leave the desktop's controls working, off touches
    nothing). Cause unknown. Suspects: `RunCircadian` still drives the
    gamma ramp or backlight when off (`gamma()` restore path, `levels`),
    `circadian nightlight off` run earlier, or something outside KC9K.
    Next: on the laptop, `kc9k circadian` status, `journalctl -u kc9k |
    grep circadian`, and whether it stops with the daemon stopped. M
    (Strict lockdown's polkit rule also blocks the backlight helper and
    colord, which breaks the same controls while strict mode is on; fix
    done.)
  - [ ] Phrase editor: "Save changes" sometimes becomes "Add phrase"
    (copies), and saving sometimes says the set has no such phrase (§4.3).
    Cause found: after a change the card keeps the removed phrase as the one
    being edited; next save removes a phrase that is gone, or a rebuilt card
    adds a copy. Fixed (GUI keeps editing the saved phrase); confirm on the
    laptop, `ci/gui_phrase_smoke.py` not run here (no GTK).
  - [ ] Phrase editor UI awkward (§4.3). Needs the owner's specifics. M
  - [x] R-PHRASE-1 (changed 2026-11-15): any word set necessary or optional,
    none allowed; length ≥ 2. Built in `db.Phrase`, `match.js`, GUI.
  - [x] Lists and phrase sets may have no action (§3.9, changed
    2026-11-15). Built: the last action can be turned off; no-action rules
    reach the extension with no actions and do nothing.
  - [ ] Phrase import mode 1 (term list → one word set) and mode 2 (phrase
    format) in the GUI (§4.3). Built: editor "Import word set…" (file or
    KC9K list), phrase set card Import/Export. Confirm on the laptop.
  - [ ] Statistics bar chart: one colour per site/app over time (§4.9). Not
    built. M
  - [ ] GUI overall unclear, hard to read, not pretty or easy to use (owner,
    2026-11-15; §4). Needs the owner's worst pages first. L
  - Strict lockdown polkit now allows the backlight helper and colord.

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
  on); the GUI block start and setting lock dialogs ask the unlock order. Not built:
  hardware/internet clock check (see §5). (L)
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

- [ ] Notification before an allowance session ends. (Not tried overnight:
  a test allowance can only be removed at midnight, leaving its block on.)
- [ ] Scheduled Frozen Chicken in logout/shutdown modes.
- [ ] Strict mode holes: `unshare -Urm` (`kernel.apparmor_restrict_unprivileged_userns`), `--user` Flatpaks killed, Electron not taken for browsers, other accounts and guest blocked.
- [ ] Shutdown mode and `kc9k.nofreeze`; measure fast relock.
- [ ] Rendered terms on real sites for days; hotkey on a real keyboard; hotkey under GNOME.
- [ ] Websites tab on a real screen. (Address change without load: a
  `history.pushState` page is caught by a URL list in LibreWolf,
  `extras_test.py`; on YouTube itself still Unchecked.)
- [ ] One-off logout/shutdown from a list; Dashboard monk button. (Checked
  2026-11-15: a timer-locked global ladder refuses `bypass off` and a weaker
  ladder, and turns off once the lock ends.)
- [ ] Use counting with a person: focused tab/app count, idle not; other browser closed during a website session.
- [ ] Mute and dark on real sites; polite close with a real app.
- [ ] No-video from page text on a real embedded player (YouTube/Vimeo iframe), in LibreWolf and Chrome (`extras_test.py` checks a local one in LibreWolf).
- [ ] Social presets beyond Shorts (`social.js` selectors written from memory); playlist videos playing; block following places; text break returning.
- [ ] Chrome on the laptop. Checked 2026-11-15: the extension checks in from
  Chrome and Chromium, both show the block page; the policy force-installs
  and pins it, turns off incognito, guest and developer mode. Left (needs
  a person): the popup, and that the extension really can't be removed in
  the Extensions page.
- [ ] Block unsupported browsers (`kc9k tamper unsupported-browsers`).
  Checked 2026-11-15: Firefox is closed within ~2 s with the reason logged;
  LibreWolf with the extension stays (browser_test); `kc9k browsers install
  chromium` installs Mint's chromium (`/usr/lib/chromium/chromium`, policy
  in `/etc/chromium/policies`) and works. Left: upgrade from
  `force-librewolf` keeping a lock; a portable LibreWolf.
- [ ] Monk mode live (`ci/monk_live_test.sh` unwritten), incl. reboot mid-way.
- [ ] GUI design pass: pages since 2026-11-10 never seen on a real screen.
- [ ] Lists without types (§3.2): checked 2026-11-15: `H` and `test` have
  one `Blocked` each; `H` blocks reddit.com by address and a page by its
  text in LibreWolf. Left: the New list dialog on a real screen.
- [ ] Active vs enabled on the laptop: checked 2026-11-15 with a throwaway
  block: off it stays out of /etc/hosts in its window; on, it is in /etc/hosts
  in its window, and outside it shows "lifted by its schedule" and leaves
  /etc/hosts. Left: the GUI's "lifted by its schedule now" on a real screen.

## 1. `SPEC.md` coverage

Only items not simply Built.

| Item | Status | Notes |
|---|---|---|
| §3.1 File URL entries | Partial | built: with tamper `file-urls` on, blacklist `file:///…` entries go into Firefox/LibreWolf and Chrome policies unless the block's file exceptions spare them (checked live in LibreWolf); Chrome live: Unchecked; the extension doesn't judge file pages |
| §3.2 Template | Built | your own saved definitions: `template save B T`, `template`, `template remove T`, `block create X --template T`; GUI "Save as template…" and a choice in New block; projects. None shipped |
| §3.2 Affected users | Partial | `block users B all|USER…` (GUI "Users:" button, gui-state `users`/`accounts`, projects): a block for some accounts stays out of hosts/firewall/policies (shared by every account) unless it covers them all; its lists reach only those users' extensions (`BuildRulesFor`, user from the socket uid) and its apps are closed only in their processes (`KillBlockedFor`). Fewer users weakens. Not per user yet: window titles, folders, fetch tools, the extension guard. Live: one account on the laptop, so only "every account" checked |
| §3.3 Continuous/scheduled types | Partial | type follows the block's windows/place rules (no explicit type field); schedules and allowance sessions lift, never switch |
| §3.4 Range lock | Partial | blocks (`start --range "mon-fri 09:00-17:00" [--range-invert]`) and setting locks (browser settings, tamper, strict, bypass, freeze schedules: `--range`); a layer that holds while in range and can't be unlocked. GUI: block start dialog and the setting lock dialogs |
| §3.4 Layered sequential locks | Partial | blocks and setting locks built (CLI flag order); GUI start and setting lock dialogs ask the order (Up/Down); one lock per kind |
| §3.6 Break per block window | Built | `schedule break B N 10m|off`: one break each time window N is open (`break start` takes it, then reward); new/longer weakens; GUI window editor; projects |
| §3.6 Allowance | Built | per day, week or month with a refill time (`allowance set B 5h --per week --refill 04:00`, GUI, projects). and rolling 24 hours (`--per rolling`, hourly buckets), and per window (`--per window`: each opening of a block window is a period, `enforce.QuotaPeriod`; outside windows nothing refills; an overnight span is two windows, so two periods) |
| §3.6 Reward | Built | `break reward B 10m --per 1h --keep one|window|day|week|month` (default one, as before), GUI choice, projects; longer keep weakens |
| §3.6 Delay break | Built | `break delay B LENGTH WAIT`, `break request B`, GUI row, projects |
| §3.6 Session limit | Built | `allowance set B 1h --sessions N`: N openings per period; a limited session ends 3 min after its last use; more weakens. GUI, projects |
| §3.6 Autostart | Built | `kc9k autostart B add once DATE HH:MM / at HH:MM [DAYS] / sign-in / next-window`, GUI list, projects; removing weakens. `at` checked live; sign-in/wake: Unchecked |
| §3.7 Strict use counting | Built | `kc9k use-counting strict|normal`, GUI switch; every open tab (`rules.strict_use`) and running apps (`RunningApps`) count; normal weakens. Live: Unchecked |
| §3.8 Scheduled freeze lock kinds beyond lead | Built | `freeze schedule lock N --for/--lock/--password-stdin/--restart [--relock]`, `freeze schedule unlock N`; setting lock `freeze:<id>`; removal refused while on; counts as a lock. GUI: Not built |
| §3.8 Allowed apps only | Partial | freeze mode `apps` (`freeze --mode apps`, schedules too, GUI choice), list `kc9k freeze apps add|remove`; closes every regular-user process but allowed apps, their children and system/desktop helpers (`allowedAppKeep`, unit-tested). GUI list editor on the Frozen Chicken page. Live: never run (no freeze overnight) |
| §3.8 R-LOCK-12 | Partial | see 0.1 |
| §3.8 Strict mode exempt folders | Built | `kc9k strict exempt add|remove DIR`, GUI list; exec bind mounts over the noexec folders (`noexecPlan`); adding weakens. Not in projects (import never weakens). Live: Unchecked (strict mode not run overnight) |
| §3.8 Forgotten password recovery | Built | `kc9k-bin recover` (root, recovery boot, not with no-escape): settings password and password locks (`--access`: access passwords). Live: Unchecked (needs a recovery boot) |
| §3.8 Factory reset protection | Partial | installer media guard built (`reset-guard on|off`, GUI switch under device protection): while anything is locked an in-memory block (`enforce/resetguard.go`) blocks OS image and USB writer download pages and closes image writers and Disks; checked live (hosts follow the lock). Firmware lock, PAM root guard, guided setup: Not built (2.6) |
| §3.9 R-LIST-4/5 | Partial | case, accents and common Cyrillic/Greek look-alikes folded (`kc9kFold`); hidden text, frames Unchecked |
| §3.9 Ad trigger | Partial | scope `ads` on blacklists and clusters (`list create --ads`, `list where B L ads on`, GUI "Ads" box): entries are ad network domains or industries (`db.AdIndustries`: adult, crypto, dating, gambling); rules `ads`; the extension watches each tab's third-party requests (`noteAdRequest`) and a page loading a listed network matches with every action (`kc9kAdMatches`, unit-tested). Not in shared layers. Browser test: Not run (the owner was using the laptop; the browser test changes LibreWolf settings and would close their windows) |
| §3.11 Block password | Built | `block password B set|remove`; hides entries, apps, titles in gui-state (`--reveal`), `block info`, exports, `list show/export`, `phrase-set show`; GUI "Show contents…"; backups keep it. A project export still holds the contents |
| §3.12 Word recording, specificity, cut | Built | `record start B L --words` / `--cluster C --words` (text lists): extension reports each page's rendered words once (`kc9kPageWords`, `/words`); `kc9k record` ranks by specificity against the baseline (`db.Specificity`); `record stop --cut N`; `record baseline on|off`. Checked live in LibreWolf. GUI: Clusters page "Record words" with cut % and the most specific words |
| §4.10 SafeSearch | Partial | Google, Bing, DuckDuckGo, YouTube Restricted pinned in hosts (every browser) and `rules.safesearch` to the extension (Brave, Yahoo, SearXNG, engines it can't hold blocked); one setting for all, not one lock each |
| §4.10 Tamper settings | Partial | GUI group with OK/Cancel window; clock, block unsupported browsers (own lock), allow file:// blocking, block task managers while locked (`tamper task-managers`), grace for a missing extension (`tamper grace S`, 10–600 s, longer weakens); gui-state `anything_locked`, settings stuck on show so. No group lock. close private windows and Tor (`tamper private`: private browsing off in every browser policy, Tor Browser/tor closed; checked live for the policies; no Tor here). Not built as settings: guest profiles (Chrome guest mode is always off by policy), extension stop (the guard always does it), installer (dpkg prerm refuses removal while locked; install.sh reinstall not guarded), inactive tabs, embedded content (frames always get the block page) |
| §4.10 Extension install dialog | Built | once per session when gui-state `extension_missing`: Install LibreWolf, or Show browsers |
| §4.10 Device protection: factory reset protection, guided setup | Not built | 2.6 |
| §4.10 Browsers list | Built | GUI Browsers group and `kc9k browsers`: LibreWolf, Chrome, Chromium, installed, per-browser extension check-in, Install (`browsers install`); live check in 0.2 |
| §4.11 `.deb` | Built | `make deb` (`packaging/build-deb.sh`): same paths as install.sh; postinst sets up sudoers, database folder, service; prerm runs prepare-uninstall, so removal is refused while locked (checked live); postrm removes sudoers. Installed on the laptop as 0.9.0~overnight (devmaster) |
| §4.3 Draft Save/Cancel | Partial | `block draft B [save|cancel]`, GUI "Edit as draft" with a Save/Cancel bar: a never-enforced copy of the lists, apps and window titles; save applies each difference through its gate, refusing and naming weakening ones on a locked block; needs and keeps the block password; drafts stay out of the block list and projects. Not drafted: phrase sets, schedules, allowance and other settings (apply at once) |
| §4.3 Ctrl+F, multi-select, double-click edit | Built | `EditableList`: Ctrl+F to its search, Ctrl/Shift-click and Ctrl+A in website lists, phrase sets, apps, titles, clusters, exempt folders; Delete/Backspace removes, selection moves on; double-click moves an entry into the box |
| §4.3 Import "From KC9K" | Built | `list copy`; a list's Import… offers From KC9K: tick other blocks' lists and clusters. "All searches containing…" not built |
| §4.5 Grid drag-move, copy to days, similar windows | Built | drag a window to move it (time; day for one-day windows); click to edit days (copy to days), times, its break, or remove; a window on several days is edited as one; other blocks' windows greyed |
| §4.9 Statistics: times blocked, top N, search, filter by block | Built | blocked pages counted per site and block (`blocked`, `blocked-block` rows); `stats --top N --search T --block B`; GUI Times blocked view |
| §4.9 Statistics per user, skip private windows | Built | skip private windows (`stats private skip|count`, rules `stats_skip_private`, GUI switch); per user (`usage_users`: apps by desktop session user, sites by the uid of the extension's socket, `blocker.ClientUser`; `stats --user U`, `--users`, GUI chooser; checked live for apps). Times blocked are counted for all users only; backups carry totals only |
| §4.10 Preferences | Built | theme with darkness slider (dark grey to black), clock 24-/12-hour (shown times: grid and clip axes, tooltips, pauses; entry fields stay HH:MM), first day of the week (schedule grid), open at login, tray (Ayatana AppIndicator or XApp status icon: what is on, open, quit; closing the window keeps it) built; tray untried on a real panel |
| §4.10 Notifications choice | Built | `kc9k notifications`, GUI group; window start/end, timer lock end, Pomodoro, allowance, app closed; advance minutes. Live: Unchecked |

## 2. Next features (after §0)

- **2.1 Circadian rhythm** (§4.10): built. Untried on real hardware: laptop
  backlight, GNOME/MATE, multiple screens. Wayland: no gamma ramps.
  System-following dark pages rely on `prefers-color-scheme`, which
  LibreWolf reports as light. In project files with its wallpapers
  (`cli/projectcircadian.go`; import only where none is set).
- **2.2 Ad trigger** (M): built (scope `ads`); left: a browser test
  (`ci/extras_test.py`), and growing the shipped industry lists.
- **2.4 SafeSearch** (S): Bing, DuckDuckGo, YouTube Restricted built under
  the one `safesearch` setting. Left: a lock per engine, if wanted.
- **2.5 Layered sequential locks** (M): blocks and setting locks done (stored
  order, `unlock` lifts only the first, status and GUI show the order). Left:
  several locks of one kind (the order is chosen in the GUI now).
- **2.6 Factory reset protection** (M): firmware lock via
  `/sys/class/firmware-attributes/` (password given to a trusted person,
  no USB/network boot, fixed order); installer media guard (block OS image
  sites, close mintstick, balenaEtcher, Ventoy, Popsicle, Startup Disk
  Creator, GNOME Disks restore); PAM root guard in `login`, `su`, `su-l`
  via `ApplyLockdown`/`RemoveLockdown` (test on generated config); guided
  setup checklist for strict/no-escape.
- **2.7 Hardening** (M each): sudo rule arguments audited 2026-11-15: caller
  paths are read as the caller (`readAsCaller`), direct reads only of
  KC9K's own files, writes only into KC9K's folders under hash names;
  `audit_test.go` keeps it so (`env_reset` untouched). Left: rules server
  secret by policy (not rolled out overnight: if the extension can't read
  it, every check-in fails and the guard closes browsers; the policy files
  are user-readable anyway); GTK window for the typing challenge (needs a
  way to answer that keeps R-LOCK-5: a stdin answer would let scripts
  type it).
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
- DECIDED OVERNIGHT: should SafeSearch make the extension required (closing
  browsers without it)? → no: Bing, DuckDuckGo and YouTube are pinned in
  /etc/hosts for every browser, and the extension adds the rest where it
  runs, because requiring it would close Firefox whenever SafeSearch is on,
  a new default behaviour (principle 7).
- DECIDED OVERNIGHT: where does "normal browsing" for specificity (§3.12)
  come from? → a word baseline gathered by the extension (per word, the
  pages it showed on; no addresses, no private windows, local only), off
  until the first word recording turns it on; `record baseline off`
  forgets it, because specificity needs it, but gathering browsing words
  unasked goes beyond what SPEC asks.
