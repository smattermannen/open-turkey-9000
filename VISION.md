# KC9K Vision

This is the reference for **why KC9K exists, where it is going, and what its
words mean**. It is written for people and AI agents working on the project.

**Rules for using this document**

1. Read it before planning work. If a plan doesn't serve the vision below,
   say so before building it.
2. Use the words in [Vocabulary](#vocabulary) exactly as defined. Don't invent
   synonyms. If something new needs a name, add it to the vocabulary in the
   same change that introduces it.
3. The status marks in [Pillars](#pillars) and [TODO](#todo) are claims about
   the code. If you find one is wrong, fix the mark; don't build on it.
4. Where the code, `README.md`, `docs/PLAN.md` or `TODO.md` contradict this
   document, don't pick one silently: raise it with the owner. Known
   contradictions are listed under [Open decisions](#open-decisions).
5. Only the owner changes the [Vision](#vision) and [Principles](#principles)
   sections. Agents may update status marks, TODO and vocabulary.

Related files: `README.md` (how to use every feature), `CLAUDE.md` (code
architecture), `docs/PLAN.md` (order of work), `docs/practicalities.md`
(design notes), `TODO.md` (status between sessions: what was tested, what is
left to try by hand). This file holds the product roadmap; `TODO.md` holds
the working notes.

Status marks: **Built**, **Partial**, **Not started**.

---

## Vision

### The goal

KC9K gives people control over their digital lives. It helps them stay on the
right task and break free from harmful digital habits: excessive gaming,
doom scrolling, adult content, and more. It is a one-stop shop for digital
health.

Much of today's software is designed to use our own psychology against us,
for the profit of a few. KC9K is built to fight back against that design.

### How it gets there

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

**The circadian rhythm.** KC9K follows the time of day, and the user can lock
it: dark mode in the browser and in apps, a blue light filter, screen
brightness, and wallpapers. A bright and positive wallpaper in the morning, a
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

### In one paragraph

Everything Cold Turkey does, plus: bypass detection the user shapes, more
triggers, more actions, any trigger to any action, several lists per block
each with its own actions, layered sequential locks (several locks on one
block, lifted in the order the user chose), whole projects
exported and imported, ready-made modes such as monk mode, lockable
SafeSearch, and circadian rhythm support.

### After publishing

- A **page score**: KC9K weighs what a page delivers (word frequencies, ad
  networks, page layout) and acts on the score.
- **Windows support.**

---

## Principles

These follow from the vision and from decisions already made. They apply to
every change.

1. **Strengthening is always free; loosening always costs.** Anything that
   weakens a held block, a locked option or a locked setting goes through
   `gateEdit`/`ensureEditable` and the settings password. Anything that
   strengthens needs no permission.
2. **Custom security.** The user makes KC9K as secure as they want, in the
   settings. The defaults outlast an impulse; strict mode takes away `sudo`
   during a lock; no-escape closes the boot-time exits; factory reset
   protection makes even a reset slow. Describe KC9K by what the user's
   chosen settings give, never as "only friction" and never as more than
   those settings deliver.
3. **The user can never be trapped by a bug.** There is always a recovery
   path (the `kc9k.nofreeze` boot option, recovery mode) unless the user has
   explicitly chosen no-escape.
4. **A refused attempt to loosen is ordinary use.** It is logged, never
   counted by bypass detection.
5. **Free, forever.** No paid tiers, no accounts, no telemetry. What KC9K
   records (events, statistics, link recordings) stays on the computer.
6. **Only what improves the experience gets built.** Behaviour changes are
   options, not new defaults.
7. **Current platform:** Linux Mint Cinnamon on X11, with LibreWolf as the
   browser the extension runs in. English only.

---

## Pillars

What the vision asks for, and where the code stands (checked 2026-11-12).

| Pillar | Status | Where / notes |
|---|---|---|
| Four enforcement layers: hosts, firewall, browser policy, closing apps | Built | `internal/blocker/` |
| Mandatory browser extension | Built | `extguard.go`; closes browsers without it |
| Triggers: address, title, page text | Built | website lists (`db/weblists.go`) |
| Trigger: the ads a page carries | Not started | `declutter` only hides ads by CSS; nothing reads which ad networks load |
| Actions: block, greyscale, no images, mute, no video, dark, declutter, redirect, image, close tab | Built | `db/actions.go` |
| Device actions: close browser, logout, shutdown, monk | Built | `enforce/consequence.go` |
| Action: prevent downloads | Not started | only download tools given a blocked address are closed (`KillNaming`) |
| Several lists per block, each with its own actions and thresholds | Built | |
| Bypass prevention with an escalating ladder | Built | `nepsis` |
| Hotkeys | Built | Cinnamon and GNOME |
| Pomodoro, breaks, quotas, schedules, allow windows, places | Built | |
| Monk mode | Built | |
| Locks: timer, time range, typing challenge, password, restart, delay | Built | stack on one block; can only grow |
| Layered sequential locks (lifted in the user's order) | Partial | locks stack, but `unlock` checks them all at once in a fixed order (`cli/control.go`) |
| Enforce SafeSearch, lockable | Partial | Google only |
| Social media feed killers | Built | YouTube Shorts tested; other sites' selectors untested |
| Export and import a whole project | Built | `cli/project.go`; locks, passwords, hotkeys, statistics stay out |
| Hard to get around (strict mode, no-escape) | Built | root-path hardening (`docs/PLAN.md` 8.1) not done |
| Factory reset protection | Not started | see TODO |
| Circadian rhythm | Not started | |
| Page score | Not started | after publishing |
| Windows support | Not started | after publishing |
| App recognition beyond names | Built | Flatpak IDs, desktop units, Python scripts, file fingerprints, browser engines, window titles |

---

## TODO

The product roadmap from the vision. Sizes as in `docs/PLAN.md`: S under a
day, M a few days, L a week or more. Day-to-day testing and handover notes
stay in `TODO.md`.

### 1. Circadian rhythm (L) — Not started

Settings that follow the time of day, each lockable like a browser option
(setting locks, trusted time).

- [ ] Time of day model: morning, afternoon, evening, night, with times the
      user sets. Reuse `internal/schedule/` rather than a new time engine.
- [ ] Blue light filter: Cinnamon's night light colour temperature.
- [ ] Screen brightness: `/sys/class/backlight` (laptops); something else for
      external screens.
- [ ] Wallpaper per time of day: `org.cinnamon.desktop.background`.
- [ ] Dark mode for apps: the GTK colour scheme.
- [ ] Dark mode for the browser: apply the existing `dark` action to every
      page during chosen hours.
- [ ] Interface page; README section; CLAUDE.md line.

### 2. Ads as a trigger (M) — Not started

- [ ] A fourth place a list can look, beside url, title and text: the ad
      networks a page loads. The extension already sees every third-party
      request through `webRequest`.
- [ ] A bundled, named list of ad networks per industry (gambling, adult …),
      so users can say "pages with gambling ads" without knowing hostnames.
- [ ] This is the first input to the page score (item 6).

### 3. Prevent downloads (S) — Not started

- [ ] A list action `no-downloads`: the extension cancels downloads started
      from matching pages (needs the `downloads` permission in
      `manifest.json`).

### 4. SafeSearch beyond Google (S) — Partial

- [ ] Bing, DuckDuckGo and YouTube Restricted Mode, each lockable.

### 5. Layered sequential locks (M) — Partial

Several locks on one block, browser option or setting lock, lifted in the
order the user specified. Every kind of lock can take part. Today the
locks stack, but `unlock` checks every lock at once in a fixed order (time,
restart, password, typing challenge; the delay runs after `stop`).

- [ ] Store the user's order of a block's locks (DB, then `blockHold`).
- [ ] `unlock` lifts only the first remaining lock; the next one takes over.
      A lock lifted stays lifted until the block ends.
- [ ] A time lock in the middle starts counting only when it is reached, or
      from the start. To be decided with the owner.
- [ ] Strengthening stays free: adding a lock, or putting a new one anywhere
      in the order. Removing one or moving it earlier loosens and is gated.
- [ ] The same for every lock: browser options and setting locks (the
      global bypass prevention lock included), not just blocks.
- [ ] `kc9k status` and the interface show the order and which lock is next.

### 6. Factory reset protection (M) — Not started

From the owner's note: *it must be possible to make a factory reset hard, so
nobody does one on impulse; the user chooses how hard.* The owner wants
options in the software for this, as far as it can go.

Software can't forbid a factory reset outright. It can make one slow and
deliberate. Each option below is the user's choice, and lockable like a
browser option:

- [ ] **Firmware lock.** Where the firmware exposes it to Linux (the
      `firmware-attributes` class in `/sys/class/firmware-attributes/`: many
      Dell, Lenovo and HP models), set an admin password, turn off booting
      from USB and the network, and fix the boot order. The password is given
      to someone the user trusts, never random: an unknown firmware password
      can't be undone. Elsewhere, a guide for doing it by hand.
- [ ] **Boot order guard.** The daemon puts back the EFI boot order and
      clears a one-time boot entry (`BootNext`) if they change during a lock.
- [ ] **Installer media guard.** During a lock, block the sites operating
      system images are downloaded from (a template list), close the tools
      that write them to USB sticks (mintstick, balenaEtcher, Ventoy,
      Popsicle, Startup Disk Creator, GNOME Disks restoring an image), and
      writes of an image to a whole disk.
- [ ] **Other systems in GRUB behind the password.** no-escape leaves Mint
      and Windows bootable without it; an option to lock the other entries.
- [ ] Root-path hardening (`docs/PLAN.md` 8.1; needs a disposable machine).
- [ ] A guided setup when turning on strict mode or no-escape that offers all
      of the above.

### After publishing

- **Page score (L).** Weigh word frequencies, ad networks and page layout
  into a score per kind of content; lists act on a score above a threshold.
  Needs item 2 first.
- **Windows support (L).** Everything in `internal/blocker/` is
  Linux-specific (systemd, iptables, `/proc`, X11, cgroups). This is a port
  behind a platform layer, not a feature.

### Decided with the owner

- 2026-11-12: Windows support and the page score come after publishing.
- 2026-11-12: KC9K gets options to make a factory reset hard, as far as
  software allows.
- 2026-11-12: "Layered sequential locking" means several locks on one block,
  lifted in the order the user specified.
- 2026-11-12: Sequential locks can be set on all locks, not only on blocks.
- 2026-11-12: KC9K is custom security, not "friction, not a security
  boundary": the user makes it as secure as they want in the settings.
- 2026-11-12: KC9K is for youth too. Tech-savvy young people, some as young as
  ten, have their own computers and want to quit habits; the project speaks
  of "youth", not "kids".

### Open decisions

Contradictions between this vision and earlier decisions. Raise these with
the owner; don't resolve them alone.

None right now.

---

## Vocabulary

Use these words, with these meanings, in code, docs and conversation.
Command and code names are in `backticks`.

### Blocking

- **Block** — a named group of website lists, apps and window titles that
  are turned on and off together. The unit everything else attaches to.
- **Active** — a block that is currently being enforced.
- **Site** — a domain, such as `youtube.com`, without `www.`.
- **App** — a program a block closes: a process name, a Flatpak app ID, a
  path, a Python script, or a file fingerprint (`sha256:…`).
- **Window title** — text that, when found in a window's title, makes KC9K
  ask that window to close, then kill its program.
- **Website list** (or just **list**) — a named list of entries inside a
  block. Two kinds:
  - **Blacklist** — pages it matches get its actions.
  - **Exception list** — spares pages from its own block's blacklists only.
- **Where** — what a list looks at: `url`, `title`, `text`.
- **Entry** — one item in a list: a domain, a pattern with `*`, or words.
  Plain words match whole words; `*` matches inside words.
- **Threshold** — for page text: how many distinct entries must appear
  before the list matches. **Hits** — the total number of matches.
- **Action** — what a blacklist does to a matching page. *Page actions*:
  `block`, `greyscale`, `no-images`, `mute`, `no-video`, `declutter`, `dark`,
  `redirect`, `image`, `close-tab`. *Device actions*: `close-browser`,
  `logout`, `shutdown`, `monk`. The strongest page action wins.
- **Look action** — an action that changes how a page looks without
  blocking it: greyscale, no-images, mute, no-video, declutter, dark.
- **Cluster** — a website list outside any block. Never enforced, always
  editable; it can be copied into a block.
- **Template** — a ready-made list offered when creating a block.
- **Link recording** — collecting the sites the user opens, to add them to a
  list or cluster afterwards.
- **Keyword test mode** — a block's title/text lists report what they would
  match instead of acting.
- **Layer** — one of the four ways a block is enforced: `/etc/hosts`,
  firewall, browser policy, closing apps. The extension works on top of
  them.
- **Extension** — the bundled LibreWolf extension. Mandatory while any
  block needs it; browsers without it are closed.
- **Workaround** — reaching a blocked site another way (proxies, mirrors,
  front-ends, download tools). Detected and logged.

### Time

- **Schedule** — recurring windows that turn a block on (**block windows**).
- **Allow window** — the reverse: the block is on at all times except during
  its allow windows.
- **Quota** — time a block allows per period; only time in use is spent.
- **Pomodoro** — alternating work and rest periods for a block, with an
  optional long break.
- **Break** — earned pause time for a block (**allowance**), or a pause set
  by `unlock` instead of turning the block off (**relock**).
- **Pause** — an active block lifted until a set time.
- **Place** — a set of Wi-Fi networks; a place rule turns a block on there,
  like an open schedule window.
- **Trusted time** — the tamper-proof clock locks use: changing the system
  clock has no effect, and time powered off is not counted.

### Locks and holds

- **Lock** — something that keeps a block, browser option or setting from
  being loosened. Kinds: **timer** (`--for`), **time range** (`--until`),
  **typing challenge** or **char lock** (`--lock`), **password**,
  **restart**, **delay**. Locks stack and can only grow.
- **Custom security** — how KC9K describes its strength: the user sets how
  hard it is to get around, from the defaults (outlast an impulse) through
  strict mode and no-escape to factory reset protection.
- **Layered sequential locks** — several locks on one block, browser option
  or setting lock, lifted one at a
  time in the order the user specified. *Partial*: today all are checked at
  once.
- **Held** — a block that can't be weakened right now: locked, in a schedule
  window, or within the **edit freeze** (2 h) before one.
- **Strengthen / loosen** — any change that makes blocking stricter / less
  strict. Strengthening is always allowed; loosening is gated.
- **Gated** — a loosening edit that needs something first: the settings
  password, the typing challenge (`--now`), or waiting.
- **Pending edit** (queued) — a loosening edit on a block with allow windows,
  applied by the daemon after the **loosening delay** (24 h by default, per
  block).
- **Setting lock** — a lock on one browser option or global setting,
  separate from blocks.
- **Settings password** — a password that guards every loosening; can be
  held by someone else.
- **Access password** — a password per area of the app (dashboard, blocks,
  settings …) that hides it.
- **Strict mode** — while anything is locked, nobody gets `sudo` or
  `pkexec`.
- **No-escape** — removes the boot-time exits (recovery entries, kernel
  options) and puts an unknown password on GRUB.
- **Frozen Chicken** — keeps every session locked, logged out or shut down
  for a time (at most 24 h). Also on a schedule. `kc9k freeze`.
- **Factory reset** — wiping or reinstalling the operating system, or
  booting another one, to get rid of KC9K. Software can't forbid it.
- **Factory reset protection** — *planned*: the options that make a factory
  reset slow and deliberate (firmware lock, boot order guard, installer
  media guard).
- **Monk mode** — the ordinary block `monk-mode`, blocking all websites
  (`*.*`), time-locked. `kc9k monk`.

### Bypass prevention

- **Bypass prevention** / **bypass detection** / **nepsis** — the same
  feature: counting what KC9K stops and climbing a ladder of consequences.
  *Nepsis* (watchfulness) is its command name, `kc9k nepsis`.
- **Event** — one thing KC9K stopped or noticed (a page, an app, a title, a
  browser, tampering, a wrong access password, a workaround). Kept a day.
- **Detector** — a kind of event a ladder counts: `page`, `search`, `app`,
  `title`, `browser`, `tamper`, `access`, `workaround`.
- **Ladder** — the ordered **steps** of bypass prevention. Global, or per
  block.
- **Step** — "N events within D: these actions". Each trip moves up a step.
- **Quiet time** — after this long without tripping, the ladder falls back
  a step.
- **Consequence** — what a step or device action does to the computer.
- **Tamper** — the daemon restarting within a boot, or the clock jumping.

### The program

- **Daemon** — the systemd service that re-applies everything every 5 s.
- **Option** — a behaviour switch that is never held (`polite-close`,
  `block-page-details`). `kc9k option`.
- **Browser option** — a lockable browser setting (`safesearch`,
  `force-search`, `web-only`, `force-librewolf`, `block-devtools`,
  `no-private`). `kc9k browser`.
- **Hotkey** — a desktop key that starts a block, optionally with a timer.
- **Project** — the whole setup in one JSON file, minus locks, passwords,
  hotkeys and statistics. Import only adds.
- **Circadian rhythm** — *planned*: settings that follow the time of day
  (blue light, brightness, wallpaper, dark mode).
- **Page score** — *planned*: a score for what a page delivers, from its
  words, ads and layout.

### Words to avoid

| Don't say | Say | Why |
|---|---|---|
| rule (for a block) | block, list or entry | "rule" has no meaning in KC9K; `rules` is only the extension's data feed |
| blocklist | blacklist | the code and interface say blacklist |
| whitelist / allowlist (for exceptions) | exception list | `block create --allowlist` is only a shortcut: `*.*` in Blocked, sites go to Exceptions |
| profile (for a block) | block, template | "preset" is taken: social media presets, schedule presets |
| system (for a block or feature) | the feature's name | |
| unblock | stop, unlock, pause, loosen | each means something different |
| kids, children | youth, young people | decided 2026-11-12 |
| friction, not a security boundary (for KC9K as a whole) | custom security | decided 2026-11-12; "friction" is fine for what one lock, such as the typing challenge, does |
