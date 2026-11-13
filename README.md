# Killer Chicken 9000 (KC9K)

A command-line productivity blocker for Linux, backed by a `systemd` service.

Killer Chicken 9000 is an open-source, Linux-native alternative to [Cold Turkey](https://getcoldturkey.com/): it blocks distracting websites and applications and keeps them blocked, even against your own attempts to undo it on impulse. Where Cold Turkey is paid and centered on Windows/macOS, Killer Chicken 9000 is free, built in Go, and designed around the way Linux actually enforces policy.

It organizes blocking into **blocks** — named groups of:

- **Sites** (domains, e.g. `youtube.com`)
- **Apps** (process names, e.g. `discord`, `telegram-desktop`, or Flatpak app IDs, e.g. `com.discordapp.Discord`)

When you **activate** a block, a daemon continuously ensures the blocking layers stay applied.

This file explains how to use what is built. What KC9K is meant to be, and the words it uses, are in [SPEC.md](SPEC.md); what is built and what comes next is in [ROADMAP.md](ROADMAP.md).

## How it works — 4 enforcement layers

Killer Chicken 9000 doesn't rely on a single, easily-bypassed mechanism. Each active block is enforced on four independent layers, and a `systemd` daemon re-applies them every 5 seconds if anything is tampered with:

1. **`/etc/hosts`** — resolves blocked domains to `0.0.0.0` (affects every program, not just browsers). Written atomically, and every entry is validated so a crafted domain can't inject extra lines.
2. **Firewall (`iptables` + `ip6tables`)** — drops packets to the blocked sites' real IPv4 and IPv6 addresses, from this computer and from Docker containers and virtual machines it hosts (its rules come first in `OUTPUT` and `FORWARD`, and are put back on top if another program inserts rules above them), and drops DNS-over-HTTPS (DoH) to well-known resolvers so browsers can't resolve names over HTTPS and slip past `/etc/hosts`. The real addresses are looked up directly from a DNS server, bypassing the poisoned `/etc/hosts`: by default the network's own server, the one the computer already uses (so nobody new learns which sites are blocked), and Quad9 when it can't answer. `kc9k dns quad9` uses Quad9 only, `kc9k dns 192.168.1.1,9.9.9.9` your own servers, `kc9k dns network` the default again; `kc9k dns` shows which servers are asked. Changing it needs the settings password. In the interface: Settings > DNS.
3. **Browser enterprise policies** — managed `URLBlocklist`/`WebsiteFilter` policies for Firefox, Chromium, Google Chrome, Brave, Edge, Opera and Vivaldi, plus DoH turned off and locked in the browser itself. The user cannot disable these from inside the browser, not even in private/incognito mode.
4. **Process kill** — terminates blocked apps that are running (SIGKILL), matched by exact executable name so unrelated processes aren't caught. A Flatpak app ID matches every process in that app's sandbox, whatever its executables are called. Everything a blocked app has launched is killed with it — e.g. blocking `lutris` (or `steam`, `heroic`) also kills the games started from it. And when an app was opened from your desktop's menu or dock, every process in its desktop app unit is killed too, even ones that were detached from it, as long as the unit's app ID is the blocked name or ends in `.<name>` (so `lutris` covers `net.lutris.Lutris`, and `discord` covers the `com.discordapp.Discord` Flatpak). If you block a shell or session process itself (`bash`, `tmux`, `plasmashell`, …), only that process is killed, not everything started from it.

Because the hosts and firewall layers act below the browser, blocking works even for browsers installed via Snap or Flatpak (where the system-wide browser policy file may not be read).

> **Custom security.** You decide how hard Killer Chicken 9000 is to get around, in its settings. With the defaults you keep `sudo`, so a determined effort can stop the daemon; the locks and the typing challenge are there to outlast an *impulse*. [Strict mode](#strict-mode) takes `sudo` and `pkexec` away during a lock, so you're an ordinary user who can't touch KC9K. `no-escape` closes the boot-time exits, and with a firmware password held by someone else, only other boot media or a factory reset gets past a lock. Until you turn on `no-escape` there is always a recovery path, so a bug can't lock you out. Blocking is IPv4/IPv6 by IP, but IP rules are best-effort: addresses change and CDNs are shared, so `/etc/hosts` and browser policy are the authoritative layers.

## Installation

Prerequisites:

- Linux with `systemd`
- `iptables` available
- Go (to build), at `/usr/local/go/bin` or otherwise on your `PATH`

Recommended:

```bash
sudo ./install.sh
```

This:

- compiles the binary (`CGO_ENABLED=0`, pure-Go SQLite via `modernc.org/sqlite`)
- installs `kc9k` (a wrapper) and `kc9k-bin` (the real binary) into `/usr/local/bin`
- adds a rule in `/etc/sudoers.d/kc9k` so it runs without a password prompt
- installs and starts the `systemd` service `kc9k.service`
- creates the database at `/var/lib/kc9k/kc9k.db`
- installs LibreWolf from its official repository (through `extrepo`), the browser that runs the Killer Chicken extension. This needs the internet; if it fails, the rest of the installation still completes, `kc9k status` reminds you, and `kc9k browser install-librewolf` tries again. Set `KC9K_NO_LIBREWOLF=1` to skip it.

The installed program is independent of the source directory — once installed, you can remove the project folder and it keeps working.

## Concepts

- **Block**: website lists, apps and window titles you block together. A block has **blacklists** and **exception lists**; each list ticks where it looks (the address, the page title, the page text), and each blacklist chooses its actions. See [Website lists](#website-lists).
- **Overlapping blocks**: several blocks can be active at once, and everything they enforce is combined. A block's exceptions spare pages only from that block's own blacklists. To make a held block stricter, see [Strengthening a held block](#strengthening-a-held-block).
- **Active**: a block currently in effect (the system is enforcing it).
- **Text lock (`--lock`)**: prevents `stop`. The only way out is `unlock`, which shows a string of random characters and requires typing all of it in one go, with at least 98% accuracy (at most 6 errors in the default 300 characters). Mistakes aren't highlighted (Backspace works, but you have to spot them yourself), paste is rejected, and `--lock-chars` sets the length, from 1 to 5000 characters (default 300, several minutes of typing; 5000 is about an hour). Instead of random characters, `--lock-words` asks for random common words (about `--lock-chars` characters), and `--lock-text "…"` for your own text, typed exactly (plain characters, up to 5000; a text you wrote to yourself works well). Either sets the text lock; `--text-lock` is another name for `--lock`. The kind is chosen when the lock is set and can't change while it is on. Browser settings, strict mode and the global bypass prevention take the same flags. In the interface, the lock dialogs choose the kind and the length. Deliberate friction to outlast an impulse. Each character is a letter (upper or lower case) 60% of the time, a digit 35% of the time and a symbol 5% of the time; symbols can be any printable ASCII symbol on a US keyboard (`` !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~ ``).
- **Timer lock (`--for` / `--until`)**: the block cannot be removed *at all* — not even with the challenge — until the time passes. The strongest lock. While it runs, only time the computer is on or suspended counts: changing the system clock has no effect, and time powered off is not counted, so a shutdown extends the lock by that long.
- **Schedule**: recurring windows (e.g. weekdays 09:00–17:00). The daemon turns the block on when a window opens and off when it closes. With **allow windows** it's the other way round: the block is on at all times *except* during its allow windows (e.g. allow games only on Friday and Saturday evenings). A schedule on its own protects nothing: add a **schedule lock** for that (below).
- **Schedule lock** (`kc9k schedule lock <block> on --lead 2h`): the block is locked during its windows, while a place rule has it on, for the lead time you choose before each window, and, with allow windows, outside them.

While a block is *locked* (any of its locks is on) it can't be switched off, removed or weakened, and `make uninstall` refuses. It can still be made **stricter**: see [Strengthening a held block](#strengthening-a-held-block). A block without locks is free to change in any direction. How hard a locked block is to get around depends on your settings (see the custom security note above).

## Commands

### 1) Create and configure a block

```bash
kc9k block create social-media
kc9k block add social-media instagram.com x.com facebook.com   # to its "Blocked" list
kc9k block add-app social-media discord telegram
```

A new block has a blacklist named **Blocked** and an exception list named **Exceptions**, both looking at addresses. Exceptions starts with `file://*`, so local files you open in the browser stay reachable; it never spares a website. `block add` and `block add-exception` are shortcuts to them; [Website lists](#website-lists) covers more lists, page titles, page text and actions.

Blocking an app also kills everything it launched — block `lutris` and the games started from Lutris close too.

For a Flatpak app, block it by its app ID (see `flatpak list --app`). This is more reliable than guessing the process name, since many Flatpaks run under generic names like `python3` or `electron`:

```bash
kc9k block add-app social-media com.discordapp.Discord
```

Seed from a built-in template (`social-media`, `video`, `streaming`, `news`, `forums`, `games`, `gambling`, `shopping`, `music`, `messaging`, `dating`, `ai-chat`, `sports`, `finance`), or block every website except the ones you add as exceptions:

```bash
kc9k block create focus --template social-media
kc9k block create work  --allowlist          # *.* in Blocked
kc9k block add-exception work docs.python.org
```

`kc9k block action <block> <action> on|off` turns an action on or off for every blacklist of the block at once (see [Website lists](#website-lists)).

Add keyword/wildcard URL patterns (browser layer only):

```bash
kc9k block add-pattern social-media '*casino*' '*://*/*ads*'
kc9k block remove-pattern social-media '*casino*'
```

Share a block between machines:

```bash
kc9k block export social-media > social.json
kc9k block import social.json
```

Show details:

```bash
kc9k block info social-media
```

List all blocks:

```bash
kc9k block list
```

### 2) Activate / deactivate

Activate:

```bash
kc9k start social-media
```

Activate with a text lock, or a timer lock:

```bash
kc9k start social-media --lock              # typing challenge to remove
kc9k start social-media --lock --lock-chars 200
kc9k start social-media --for 2h            # can't be removed for 2 hours
kc9k start social-media --until 17:00       # can't be removed until 17:00
```

Recurring schedule (the daemon activates/deactivates it automatically):

```bash
kc9k schedule add social-media --days weekdays --from 09:00 --to 17:00
kc9k schedule add social-media --days sat,sun --from 10:00 --to 12:00
kc9k schedule list social-media
kc9k schedule remove social-media 1
```

Times go in 5-minute steps, and one block's windows never overlap (a window that would is refused; widen the one that is there instead).

Allow windows invert this: the block is on at all times **except** while one of its allow windows is open. For example, to allow a `games` block only on Friday and Saturday evenings:

```bash
kc9k schedule add games --allow --days fri,sat --from 18:00 --to 24:00
```

A block's schedules must be all block windows or all allow windows. `--to 24:00` means midnight, so a window can run to the end of the day without a one-minute gap.

Schedules are numbered `1, 2, 3, …` per block. Removing one moves the later ones down a number (remove `1` from `1, 2, 3` and the old `2`/`3` become `1`/`2`), and a new schedule takes the next number.

Windows are same-day; for an overnight span (e.g. 17:00–05:00) add two windows (`17:00–24:00` and `00:00–05:00`).

#### Schedule lock

Without a lock, a schedule only switches its block on and off: you can still stop the block, weaken it or remove a window at any time. A **schedule lock** keeps it: during the block's windows, while a [place](#places) rule has it on, and for a lead time you choose before each window (for allow windows: at all times outside them, and for the lead time before an allow window ends), the block can't be stopped, removed or weakened (removing apps or list entries, adding exceptions, removing a window, adding an allow window, …), though it can still be made stricter.

```bash
kc9k schedule lock games on --lead 2h    # the lock, from 2 hours before each window
kc9k schedule lock games                 # show it
kc9k schedule lock games off             # refused while the lock is on
```

A longer lead time is always allowed; a shorter one, or off, weakens the block. In the interface: the Schedule tab. Upgrading from a version before schedule locks gives every block that had schedules or place rules a schedule lock with a 2-hour lead, which is how those blocks were protected before. Changes queued for blocks with allow windows by those versions are still applied by the daemon (`kc9k pending` lists them, `pending cancel N` drops one); new ones aren't queued.

Show status:

```bash
kc9k status
```

Deactivate (only if the block isn't held — see Concepts):

```bash
kc9k stop social-media
```

Unlock a block: its text or password lock is lifted, and its delay lock starts counting down. The block **stays on**; `stop` switches it off once nothing is locked. Timer locks and open schedule windows can't be unlocked — they lift on their own. A long challenge is shown a page at a time, sized to your terminal: type a page and press Enter for the next. Accuracy is counted over the whole text:

```bash
kc9k unlock social-media
```

To practice or test the challenge without touching any block (no root needed), run this from the source directory, optionally with a length:

```bash
./try-challenge.sh        # 300 characters
./try-challenge.sh 100
```

### 3) Editing a block's lists

Remove a domain from a block's Blocked list (or from any list with `kc9k list remove`):

```bash
kc9k block remove-site social-media instagram.com
kc9k list remove social-media Blocked instagram.com
```

Notes:

- If the block is **active**, Killer Chicken 9000 re-applies the layers so the change takes effect immediately.
- If the block is **locked** (any of its locks is on, its schedule lock included), you **cannot** weaken it: removing apps, window titles or blacklist entries, adding exceptions, and unticking a blacklist's places or actions are refused (see [Website lists](#website-lists) for the full list). Lift the locks first (unlock, or wait for the timer lock / schedule lock), then make your changes. Edits that only make it stricter are always allowed (next section).

#### Strengthening a held block

A held block can always be made stricter, since that can't be used to escape it:

| Allowed while held | Why it's safe |
|---|---|
| `block add-app`, `block add-title` | closes more apps |
| `list add` to a blacklist (`block add`, `add-keyword`), `list create` | matches more pages |
| `list scope … on` and `list action … on` on a blacklist, a lower `list threshold` | looks in more places, does more, or needs fewer words |
| `list remove` from an exception list (`block remove-exception`), `list scope … off` on one, `list delete` of one | spares fewer pages |
| `start <block> --lock [--lock-chars N]` on an **active** block | adds a text lock, or lengthens the challenge (a shorter `--lock-chars` is refused) |
| `start <block> --for D` / `--until HH:MM` on an **active** block | adds a time lock, or extends it (an earlier end is refused) |

```bash
kc9k start social-media --for 3h       # already active: extend its time lock
kc9k block add-site social-media reddit.com
```

If you lock a block that its schedule switched on, the lock outlives the schedule: when the window closes, a timer-locked block stays on until its timer lock expires (the daemon then switches it off), and a text-locked one stays on until you `unlock` it.

Remove an app (process) from a block:

```bash
kc9k block remove-app social-media discord
```

Remove an entire block (must be inactive and not locked):

```bash
kc9k block remove social-media
```

## Service (daemon) and logs

The daemon is managed by `systemd`:

```bash
systemctl status kc9k
systemctl restart kc9k
journalctl -u kc9k -f
```

## Uninstall

```bash
sudo make uninstall
```

Uninstalling first runs a guard that **refuses while any block or setting is locked** (R-LOCK-9), so uninstalling can't be used to sidestep a lock — lift the locks first. When it proceeds, it deactivates all blocks and removes every blocking layer (`/etc/hosts` entries, firewall rules, browser policies) so the system is left clean rather than blocked-with-no-tool. It also undoes KC9K's other changes: forced SafeSearch, the forced search engine, LibreWolf's permission for the unsigned extension, and the no-escape boot lock. Browsers are told to remove the extension at their next start, since a leftover copy would keep enforcing its last rules with no daemon to update them; that one small policy stays in `/etc/librewolf/policies/` and `/etc/firefox/policies/`, and can be deleted once LibreWolf has started once. LibreWolf itself stays installed.

## Website lists

A block has **blacklists** and **exception lists**, each with a name. Every list ticks where it looks, in any combination:

- **url**: the page's address. Entries are domains (`reddit.com`), paths (`reddit.com/r/funny`) and wildcards (`*.xxx`, `*casino*`, and `*.*` for every website).
- **title**: the page title, as shown on its tab.
- **text**: the page's text. Title and text entries are words or phrases: a plain word counts only on its own (`porn` doesn't count in "pornhub", `18+` not in "1118+"), and `*` matches inside words (`*porn*` counts in "pornhub", `porn*` in "pornstar"). Case is ignored.

**Word combinations** catch words only together. A combination is boxes of words side by side, as many as you like: each box is a list in brackets, or one bare word, and `:N` after a box asks for N *different* words of it (1 if left out):

| Entry | Counts in | Not in |
|---|---|---|
| `car(cheap)` | "cheap car", "car cheap" | "cheap reliable car" |
| `cars(red, big, blue, beautiful)` | "red beautiful cars", "big cars" | "red toyota", "cars, big and red" |
| `(car, lambo*, ferrari)(red, big, beautiful)` | "a beautiful Lamborghini", "ferrari red" | "a red toyota" |
| `(car, lambo, sheep) (big, small, nice):2 (sport, hello, *niece*)` | "a nice big sport car", "small sheep nice nieces" | "a big sport car", "big big sport car" |

A combination counts where a run of words, with only spaces between them (no commas or other words) and each a word of some box, holds what every box needs, in any order. Each word follows the rules above (whole words, `*` inside words; phrases such as `car(very cheap)`), and a word that fits two boxes counts for one of them. A combination is one entry: it counts once toward a threshold, and once per run toward hits. In the shell, quote it, since brackets mean something there; it is stored with one space between boxes and `, ` between words, so it is found again however it was typed. In the interface, **Word combination…** under a title or text list has one box per bracket, each with its minimal count; double-click a combination to change it.

Each blacklist chooses its **actions**, in any combination:

| Action | What happens |
|---|---|
| `block` | the block page (the default) |
| `greyscale`, `no-images` | the page shows in greyscale, or without its images |
| `mute`, `no-video` | the tab's sound is switched off; videos and players are stopped and hidden |
| `declutter` (experimental), `dark` | side columns, banners, pop-ups and suggestions are hidden and the text set in one column; a light page is shown dark |
| `redirect --url https://…` | another page opens instead |
| `image --image FILE` | a picture of yours (PNG, JPEG, GIF or WebP) shows instead |
| `close-tab` | the tab closes |
| `close-browser` | the browser closes |
| `logout [--for D]` | the browser closes and you are logged out; with `--for`, you stay logged out that long |
| `shutdown [--for D]` | the browser closes and the computer shuts down; with `--for`, it stays off that long |
| `monk --for D` | the browser closes and [monk mode](#monk-mode) starts for that long |

When several apply, the strongest page action wins: close-tab, then image, redirect, block. A redirect to a page that is blocked too shows the block page. Only the page itself triggers close-tab and the device actions, never something embedded in another page (that only gets the block page). A list's logout, shutdown or lock-screen is announced on the desktop 60 seconds before it happens, so you can save your work. A one-off logout or shutdown happens at most every 10 minutes, so a page restored after restarting can't loop the computer; staying logged out or off goes through [Frozen Chicken](#frozen-chicken), at most a day. Changing an action's settings: a longer time and another picture are always allowed; another address loosens the list.

A match in the address or the title takes every action. In the text, each action has two numbers, and both must be met: a **threshold**, how many *different* entries the page must contain (repeats count once), and **hits**, how many times they occur in all (repeats counted). Both start at 1. Threshold 1 with hits 5 acts on one word five times, or five words once. Once a page meets them, it stays greyed or blocked until you leave it, even if its text changes.

An **exception list** spares the pages it matches from every blacklist of its block. Exceptions by title or text have to wait until the page is read, so a block that has them leaves its websites to the extension alone: they don't go to `/etc/hosts` or the firewall, and a page shows for a moment before it is blocked. An address exception also takes its domain out of `/etc/hosts` and the firewall (those block whole hosts), leaving it to the browser.

```bash
kc9k list show work                                   # the block's lists
kc9k list create work Videos --url --title            # a blacklist (looks at the address unless told otherwise)
kc9k list add work Videos youtube.com twitch.tv "*stream*"
kc9k list create work Words --text
kc9k list add work Words nsfw 18+ "*porn*"
kc9k list add work Words "(car, lambo*, ferrari)(red, big, beautiful)" "(car, sheep) (big, small, nice):2 (sport, hello)"
kc9k list action work Words greyscale on
kc9k list threshold work Words greyscale 1            # one word greys the page
kc9k list threshold work Words block 3                # three different ones block it
kc9k list hits work Words block 10                    # ...and only when they occur 10 times in all
kc9k list create work Lectures --exceptions --title   # spare videos titled "lecture"
kc9k list add work Lectures lecture tutorial
kc9k list action work Videos redirect on --url https://example.com/focus
kc9k list action work Words monk on --for 1h          # close the browser, monk mode for an hour
kc9k list scope work Videos text on                   # tick or untick a place (also: list where)
kc9k list rename work Videos Video
kc9k list delete work Video
```

Each list exports and imports as a text file with one entry per line (`#` starts a comment), and so do a block's apps and window titles:

```bash
kc9k list export work Words > words.txt
kc9k list import work Words words.txt
kc9k block export-list work apps > apps.txt       # also: titles, blocked, exceptions
kc9k block import-list work apps apps.txt
```

What makes a block stricter is always allowed, even while it is held: adding to a blacklist, ticking a blacklist's places or actions, lowering a threshold or hits, adding a list, and removing exceptions or unticking an exception list's places. Everything else loosens it, so it is refused while the block is locked. A blacklist keeps at least one place and one action.

The commands from before lists still work on a block's default lists: `block add`, `remove-entry`, `add-site`, `remove-site`, `add-pattern` and `remove-pattern` on **Blocked**, `add-exception` and `remove-exception` on **Exceptions**, and `add-keyword`, `remove-keyword` and `threshold` on **Page words** (a text list, made when first needed). Upgrading turns each block's sites and patterns, exceptions and page keywords into those three lists, with the block's actions and thresholds; an allowlist block becomes `*.*` in Blocked with its sites as exceptions. `block export` writes the lists, and `block import` also reads exports from before lists.

## Clusters

A cluster is a website list of its own, outside any block: a name, where it looks (address, title, text) and entries. Nothing enforces a cluster; it is a stock to build blocks from, so a new block can spare the academia cluster or block the anime cluster in one command.

```bash
kc9k cluster create academia                        # looks at addresses; or --title / --text
kc9k cluster add academia arxiv.org scholar.google.com jstor.org
kc9k cluster list
kc9k cluster show academia
kc9k cluster copy academia study --exceptions       # a new exception list 'academia' in block 'study'
kc9k cluster copy anime focus                       # a new blacklist 'anime' in block 'focus' (it blocks)
kc9k cluster copy anime focus --as Shows            # ... under another name
kc9k cluster copy anime focus --into Blocked        # add the entries to a list the block has
kc9k cluster save focus Blocked distractions        # a block's list saved as a cluster
kc9k cluster export academia > academia.txt         # one entry per line, with a line saying where it looks
kc9k cluster import academia academia.txt           # makes the cluster if needed
kc9k cluster scope academia title on
kc9k cluster remove academia jstor.org
kc9k cluster rename academia school
kc9k cluster delete school
```

A copy is a copy: later changes to the cluster don't reach the block, and deleting the cluster leaves the copied lists alone. Editing a cluster is always allowed. Copying into a block follows the block's rules: into a blacklist strengthens it (always allowed), into an exception list weakens it (refused while the block is locked). A new blacklist made from a cluster blocks what it matches; choose other actions with `kc9k list action`. An exported cluster is a plain entry file, so `kc9k list import` reads it too.

## Link recording

Start a recording, browse, stop: every site opened in the browser in between is added to a block's list or to a cluster. It collects what a task really needs, such as a school's pages with all their sign-in steps, to spare them in a block.

```bash
kc9k record start study Exceptions      # into a block's list (one that looks at addresses)
kc9k record start --cluster school      # ... or into a cluster, made if needed
kc9k record                             # what was recorded so far
kc9k record stop                        # add the sites and stop
kc9k record stop --discard              # stop and drop them
```

The sites (domains, without `www.`) come from the extension in LibreWolf: pages and the frames inside them (a login or captcha from another site), private windows included, and pages a block stops too, so you can record with the block on and see what it would need. They stay on this computer. Nothing changes until `stop`, which follows the target's rules: into a blacklist or a cluster is always allowed; into an exception list it loosens the block, so it is refused while the block is locked (the recording then stays, to stop later or discard). One recording at a time, at most 2000 sites.

## Places

```bash
kc9k place add library               # remember the Wi-Fi networks in range now as "library"
kc9k place rule work library on      # block 'work' is on while the computer is there
kc9k place                           # places, and whether you are at one now
kc9k place rule work library off
kc9k place remove library
```

A place is known by the Wi-Fi networks in range when you save it: your own and the neighbours'. The computer is at the place when two of them are in range again (one, if the place only knows one). Saving a place again under the same name adds the networks seen now, which makes it easier to recognise. A block with a place rule turns on within seconds of arriving and is held while you are there, like a block in a schedule window: it can't be stopped or loosened, and the rule and the place can't be removed. It turns off when you leave, unless it was locked meanwhile.

When no network at all can be seen (Wi-Fi switched off, or no adapter) the computer counts as being at every place with a rule, and a switched-off Wi-Fi is switched back on, so turning Wi-Fi off is no way out. Location uses Wi-Fi only; there is no GPS. In the interface: Blocks > a block > Places.

## When the clocks change

Schedules follow the clock on the wall. On the night the clocks go forward, the skipped hour doesn't happen: a window that lies wholly inside it doesn't open that day, and one that spans it is an hour shorter. On the night they go back, the repeated hour is covered both times. `start --until 07:00` means 07:00 on the clock, however long the night is. Everything counted in time (`--for`, delays, Pomodoro, the 24-hour wait for loosening, the 2-hour edit freeze before a window) counts real hours.

## Daily allowance (`kc9k quota`)

```bash
kc9k quota set social-media 30m     # the block now stays on
kc9k quota start social-media       # open it and spend the quota
kc9k quota end social-media         # close it early
kc9k quota remove social-media      # takes effect at midnight
```

An open session spends only the time the block is in use: one of its apps is in the focused window, or one of its websites is in the focused tab (not after 3 minutes without input) or plays sound or video, even muted, in any tab. Websites are matched by the block's address and title lists; page text lists don't count here. While a session with websites is open, browsers without the extension are closed, so another browser can't be used for free. Without LibreWolf the extension can't see tabs, so a session with websites spends all of its time. Lowering a quota applies at once. Raising or removing it applies at midnight. A lock or a scheduled window closes a session. The day follows the system clock, so moving the clock to another date refills the quota.

## Locks

| Lock | Command | Unlocked when |
|---|---|---|
| Timer | `start b --for 2h` | the time has passed |
| Timer (clock time) | `start b --until 17:00` | the clock time is reached |
| Text | `start b --lock` | the challenge is typed with `unlock` |
| Password | `start b --password-stdin` | someone enters it with `unlock`, from standard input |
| Restart | `start b --restart` | the computer has restarted |
| Delay | `start b --delay 30m` | the delay has passed after `unlock` |

An unlocked lock is gone; the block stays on until you `stop` it or its schedule ends it (§3.4 of `SPEC.md`).

## Browser options

The daemon force-installs the bundled extension through policy, so it can't be removed or disabled. It enforces wildcards, page titles and text, greyscale, images and web-only, shows a block page, and reads its rules from the daemon on `127.0.0.1:47613`. It runs in LibreWolf, which installing KC9K installs, and in Google Chrome and Chromium (see [Google Chrome](#google-chrome)). Firefox only accepts signed extensions; Brave, Edge, Opera and Vivaldi aren't set up for it.

**The extension is not optional.** While an active block has a blacklist with entries, or web-only or force-search is on, any browser that has run for 60 seconds without the extension checking in is closed, like Cold Turkey does; a block of apps alone leaves browsers alone. `kc9k status` shows whether the extension is needed right now and when it last checked in. With neither LibreWolf nor Google Chrome installed nothing is closed, since there would be no browser left, and `kc9k status` says so.

```bash
kc9k browser install-librewolf    # install LibreWolf (official repository, via extrepo)
kc9k browser force-librewolf on   # close other browsers (needs LibreWolf installed)
kc9k browser force-search on      # Google as the only search engine
kc9k browser safesearch on        # lock Google SafeSearch
kc9k browser web-only on          # web results only: no images, shopping or AI
kc9k browser block-devtools off   # allow LibreWolf's developer tools (on by default: blocked)
kc9k browser no-private on        # no private windows in LibreWolf (from its next start)
kc9k browser                      # list every option, whether it's on, and its locks
```

Options can always be turned on. Each one can be **locked on its own**, independently of your blocks, with the same kinds of lock as a block (except the delay lock). A locked option can't be turned off until the lock ends or is unlocked:

```bash
kc9k browser safesearch on --for 168h         # can't be turned off for a week
kc9k browser web-only on --until 17:00         # until a clock time
kc9k browser force-search on --lock            # typing challenge to turn it off
kc9k browser web-only on --password-stdin      # password to turn it off
kc9k browser safesearch on --restart           # on until the computer restarts
kc9k browser unlock web-only                   # challenge/password, then turns it off
```

Like a block's locks, an option's locks can be added later but only made stronger: a time lock can only be extended, a challenge only lengthened, and a password can't be replaced. Time locks use the same tamper-proof clock as block time locks. While an option is locked, strict mode and no-escape hold and `make uninstall` is refused. In the interface, each option has **Lock…** and **Unlock…** buttons under Settings.

The extension checks in each time it fetches its rules, with a header web pages can't send; the daemon traces the connection to the browser process. Browsers are recognised by name, and portable, renamed or copied browsers (a Firefox or Chrome download, Tor Browser) by their engine: a program you put in place yourself that loads Firefox's or Chromium's engine counts as a browser. Installed apps built on those engines, such as Thunderbird, Spotify, Steam or Electron apps, are left alone. A browser started for remote control (`--marionette`, `--remote-debugging-port` and the like) is closed straight away during website blocks, since those switches let a script turn the extension off while it keeps checking in.

`force-librewolf` and `block-devtools` also can't be turned off while a block is locked or Frozen Chicken runs, because blocks rely on them.

**Protecting the extension inside LibreWolf.** A running extension isn't enough; it must also be unable to be switched off while it keeps checking in. So:

- LibreWolf's developer tools are blocked by default (`block-devtools`), since from the extension's own block page or `about:debugging` they can switch it off. Web developers can turn this off; domain blocks still hold then, but wildcards, keywords, greyscale, images and web-only can be got around.
- Five settings are locked in LibreWolf: `extensions.webextensions.restrictedDomains` and the two `extensions.quarantinedDomains` settings (sites extensions may not touch), and `devtools.chrome.enabled` and `devtools.debugger.remote-enabled` (privileged consoles). The rest of about:config stays open.
- LibreWolf loads `~/.librewolf/librewolf.overrides.cfg` after its own settings, so it could unlock those. KC9K points LibreWolf at a cleaned copy instead (`/var/lib/kc9k/librewolf/<user>.cfg`) that keeps your plain `pref()`/`defaultPref()`/`lockPref()` lines and drops anything else. Edit your original file as usual; the copy follows within seconds.
- Browsers read their settings when they start. When KC9K changes something that protects the extension (a new version of it, developer tools, the locks), browser windows opened before that are closed during website blocks, a minute after the change, so the browser restarts with the new settings. Greyscale and image blocking are chosen per block instead (`kc9k block action`), so a block's own locks protect them. The extension also blocks a site when it's shown through Google Translate's proxy (`www-reddit-com.translate.goog`) or a web archive (`web.archive.org`, `archive.ph`). Web-only relies on Google's unofficial `udm=14` parameter, which may change.

### Google Chrome

Google Chrome (the `.deb` from Google) and Chromium run the same extension, built for Chrome's Manifest V3. Chrome installs extensions from outside its Web Store only through policy, from an update address, as a signed `.crx`: the daemon serves both on `127.0.0.1:47613/chrome/`, and `/etc/opt/chrome/policies/managed/kc9k-setup.json` (for Chromium, `/etc/chromium/…`) makes Chrome install it. Chrome reads policies while it runs, so an open Chrome gets the extension within seconds, pinned to the toolbar, and can't remove or disable it. Being installed by policy is also what lets it block pages in Chrome.

- The extension is signed with a key made on this computer the first time (`/var/lib/kc9k/chrome-extension.pem`, root's alone), so its ID differs per computer: nobody else can sign an extension that does nothing under that ID.
- Chrome's incognito windows and guest mode are always off, since the extension can't run there, and so is developer mode (no unpacked extensions). The developer tools never open on the extension itself; `block-devtools` turns them off everywhere, which in Chrome takes effect at once.
- Chrome started with `--disable-extensions`, `--disable-extensions-except` or `--remote-debugging-pipe` is closed at once during website blocks, like remote control.
- Chrome looks for new extension versions only every few hours, so the extension asks for one as soon as the daemon has a newer version, and restarts on it.
- `force-search` sets Google with web results as Chrome's default search engine too. `force-librewolf` closes Chrome like any other browser.
- Chrome runs the extension's background as a service worker, which it stops when idle. The extension keeps it running; should Chrome stop it anyway, the first page loaded as it starts again may get through the extension (the browser's URL policy still blocks plain blocked sites), and is taken away as soon as the page itself reports in.

## Pomodoro

`kc9k pomodoro set work 25m 5m` alternates a block between work and breaks while it runs, with locks kept in place. Add `--long 15m --every 4` for a long break after every fourth work period. During a work period the block's quota session can't be opened, and an open one is closed. Setting a cycle is refused while the block is held, and `pomodoro off` is always allowed.

## Breaks

```bash
kc9k break allowance work 10m --per 1h   # earn 10 minutes of break per hour of blocking
kc9k break start work                    # take what is earned (or: break start work 5m)
kc9k break end work                      # back to work early; the rest is kept
kc9k break text work 5m                # unlocking gives a 5-minute break instead of ending the block
kc9k break                               # what each block allows and has earned
```

A **reward** (`break allowance`) earns break time while the block is on and the computer is in use (input in the last 3 minutes, screen not locked), saved up to one reward. A break lifts the block, locked or not, and the block comes back by itself with its locks; editing it stays refused meanwhile. With a **text break** (`break text`, formerly `relock`), passing the typing challenge or giving the password lifts the block for that long instead of unlocking it, and unlocking once more during that break lifts its locks. Allowing more breaks loosens a block, so it is refused while the block is held; allowing fewer (`allowance … off`, `text … off`) is always possible. The toolbar button shows the break time left on the block's sites.

## Window titles

Like Cold Turkey, a block can close apps by what their windows show:

```bash
kc9k block add-title social YouTube Twitch
kc9k block remove-title social Twitch
```

While the block is on, any app whose window title contains one of the words (case-insensitive) is asked to close, as its close button would, so it can save or ask first; if it is still open a few seconds later, it is closed with everything it started. Window title entries never apply to browsers: a tab's title is judged by the block's title lists. The usual rules apply: only your own programs, never ones the desktop needs. Window titles are read from X11 (Cinnamon on Linux Mint); under Wayland only apps running through XWayland are seen. Removing a word loosens the block, like removing an app.

## Monk mode

```bash
kc9k monk --for 2h
```

Monk mode blocks every website for a while, time-locked: it starts the block **monk-mode**, made the first time with `*.*` in its Blocked list, and ends it when the time is up. The block belongs to monk mode: it can't be removed or renamed, can't start monk mode itself, and is changed only while monk mode is off, so add exceptions (documentation you need), apps or anything else to it beforehand. `kc9k start monk-mode` is refused; use `kc9k monk --for`. On a monk mode that is already on, a longer time extends it. It also starts from the Dashboard (Monk mode…), from a blacklist's `monk` action, and from bypass prevention. Starting it needs the settings password only if setting a lock does (`kc9k password locks on`).

## Bypass prevention

Many blocked pages or closed apps in a short time usually mean someone is hunting for a loophole. Bypass prevention (command `bypass`; `nepsis` still works) counts what Killer Chicken stops and climbs a ladder of steps you choose, so you really step away:

```bash
kc9k bypass set --step "3 in 10m: warning, close-tab, logout 1m" \
                --step "3 in 10m: close-browser, monk 5m" \
                --step "2 in 30m: shutdown 15m" --quiet 2h      # the whole computer
kc9k bypass set --block social --step "5 in 10m: close-browser, monk 30m" --detect page,app
kc9k bypass set ... --for 7d                                      # global only: locked for a week
kc9k bypass                                                       # the ladders and where they stand
kc9k bypass off [--block social]
kc9k bypass unlock                                                # challenge or password lock
```

Each step trips when its number of events happens within its window, then takes its actions; the next time, the next step; past the last, the last again. After `--quiet` without tripping, the ladder falls back a step. Actions: `warning` (a notification), `close-tab` (the tab of the page that tripped it), `close-browser`, `lock-screen D`, `logout [D]`, `shutdown [D]` and `monk D`. With a time, locked, logged out or off lasts that long through Frozen Chicken (at most a day); a one-off logout or shutdown happens at most every 10 minutes. `--count N --within D --then ACTION [--then-for D]` is a short way to give one step.

Detectors (`--detect`, default all): `page` (a blacklist took a page away; greyscale or no-images alone don't count), `search` (such a page opened from a search engine's results: Google, Bing, DuckDuckGo, Brave, Startpage and others, or any `/search?q=` page; `page` counts these too, so pick `search` alone to count only pages that were looked for), `app` (a blocked app was closed), `title` (a window was closed for its title), `browser` (a browser without the extension was closed), `tamper` (the daemon started again within a boot, except after an upgrade of KC9K or a crash it reported itself, or the clock moved more than 2 minutes against the boot clock), `access` (a wrong [access password](#access-passwords)), `workaround` (a blocked site [reached another way](#sites-reached-another-way)). Asking to stop, unlock or loosen a held block is refused and logged, but never counted: that is ordinary use.

The global ladder counts every block's events; a block's own counts that block's events while it is on (a closed browser counts for every active block with websites, tampering for every active block, and once for the whole computer); when both are set, both apply. Every event a ladder counts is announced with a notification ("Event triggered, be careful", with how many more trip the next step), and every step that trips says what it does; there is no countdown, the step acts at once. A blocked page is judged at once, so close-tab hits the right tab; everything else every daemon cycle. Making one stricter (more detectors, a later fall-back, steps that trip on fewer events over longer windows and keep every action for at least as long, more steps at the end) is always allowed. Anything else loosens it: a block's is gated like other edits to the block, and the global one can be locked like a browser option (`--for`, `--until`, `--lock`, `--password-stdin`, `--restart` on `set`). Events are kept for a day. A setting from before the ladder became one step that closes the browsers and then takes its old consequence.

## Page keywords

Page keywords are a blacklist that looks at page text; see [Website lists](#website-lists). The older commands still work on a block's **Page words** list:

```bash
kc9k block add-keyword adult nsfw 18+ "*porn*"
kc9k block threshold adult greyscale 1   # one keyword greys the page
kc9k block threshold adult block 3       # three different ones block it
kc9k block remove-keyword adult nsfw
```

## Hotkeys

A key combination can start a block, optionally with a time lock, for example to study:

```bash
kc9k hotkey add ctrl+alt+s study --for 1h   # Ctrl+Alt+S: 'study' on, locked for an hour
kc9k hotkey list
kc9k hotkey remove 1
```

Keys are ctrl, alt, shift or super joined to a key with `+`; F1 to F24 also work alone. Hotkeys are added as Cinnamon and GNOME keyboard shortcuts (they show up in the desktop's keyboard settings as "KC9K: …"); your own shortcuts are left alone. Pressing one shows a notification saying whether the block started. On a block that is already on, it only extends the lock. A hotkey with a lock needs the settings password when you add it, not each time you press it. The interface has them under Settings.

## How apps are recognised

An app is matched by its program's name, by a full path or a folder (below), by its Flatpak or desktop app ID, and:

- **By contents.** `kc9k block add-app games /home/me/bin/game` also remembers what the file contains, so a renamed or moved copy of it is closed too. A program rebuilt or updated is a different file; add it again. The system's own programs (root-owned, under `/usr`, `/opt` and the like) are never matched this way.
- **Python programs by their script.** `kc9k block add-app games play.py` (or `play`, or the script's full path or folder) closes `python3 play.py`. Blocking `python3` itself stays refused, since the desktop runs on it. For window titles, a Python program is spared only when it runs one of the system's own scripts.
- **Download and streaming tools by the address they are given.** While a block with websites is on, `yt-dlp`, `mpv`, `vlc`, `curl`, `wget`, `streamlink` and similar tools started with the address of one of its sites are closed, and it is logged as a `workaround` event. Editors, shells and everything else are never touched for what their command line says.

## Blocking folders

`kc9k block add-app games /opt/spel/` blocks every program under a folder, and a full path blocks one file. System folders such as `/usr/bin/` are refused.

Blocked apps are only ever closed when they run as a regular user, so a block can't kill system programs that happen to share a name (an update running in Python, say). Programs the desktop needs, such as `Xorg`, `cinnamon`, `pipewire` or `python3`, can't be added at all, since blocking them would close your session over and over for as long as the block lasts.

## Notifications

The daemon warns on the desktop 5 minutes ahead (or less, when something is set up closer than that) before a block's schedule turns it on, before a scheduled Frozen Chicken starts (saying whether the screen will lock, you'll be logged out or the computer shuts down, and again 1 minute before), and when an open quota session has 5 minutes left. Each event is announced once. It needs `notify-send`, which Linux Mint has.

## Settings password

`kc9k` runs as root without a password through sudo, so anything running as you could use it: loosen a block, or lock you out for a year with no-escape. A settings password guards against that, and lets someone else decide when your blocks may loosen:

```bash
kc9k password set        # asks for the current one first, if there is one
kc9k password remove
```

With it set, these ask for it: any weakening edit, `stop`, `unlock`, schedules, quotas (setting or removing), turning browser settings off, strict mode off, no-escape, and uninstalling. These stay free: reading the state, starting a block, adding sites, apps and patterns, ticking a website action on, and opening a quota session. The interface asks for it the first time a change needs it and remembers it until it closes.

Setting a lock (starting a block with a lock, a locked browser setting or strict mode, Frozen Chicken, monk mode, a hotkey with a lock) asks for it only if you choose so, so that nobody else can lock you out: `kc9k password locks on` (off again with the password). In the interface: the check box under Settings > Settings password.

Passwords are read from standard input, hidden on a terminal; when a command also takes a lock's own password, the settings password comes first. If it's lost, root can remove it while nothing is locked (`kc9k password --help` shows how), and the testing build's master password clears it too.

## Social media settings

```bash
kc9k social                                  # the settings there are
kc9k social work youtube:no-shorts on
kc9k social work instagram:messages-only on
kc9k social work                             # what the block has on
```

A setting keeps a site but takes a part of it away while its block is on: YouTube without Shorts, without the home feed (it opens Subscriptions), without recommendations or comments; Instagram, Facebook and X for messages only, or without Reels or Explore; Reddit without Popular and All or without the home feed; TikTok with Following only; LinkedIn without the feed or the news column. Don't put the site on the block's blacklist as well. Addresses of a part that is switched off are sent to another page of the site, and what is left of it on other pages is hidden. Turning a setting on is always allowed; turning one off loosens the block. In the interface: Blocks > a block > Social media.

These sites change their pages now and then. A setting that sends addresses elsewhere keeps working; one that hides a part of a page may miss it until Killer Chicken 9000 is updated.

### One YouTube playlist through a block

```bash
kc9k block add study youtube.com
kc9k social study playlist "https://www.youtube.com/playlist?list=PL…" on
kc9k social study playlist PL… off
```

The playlist's page and its videos stay open; YouTube's front page, search and every other video are blocked. A video must be opened from the playlist (its address carries `list=…`), and it must really be one of the playlist's videos: YouTube plays any video with any `list=` added, so the extension checks the playlist shown beside the video. Letting a playlist through adds exceptions to the block, so it is refused while the block is held; taking it away is always allowed.

## The toolbar button

The extension puts a button in LibreWolf's toolbar. On the tab in front it shows the time left until the site is blocked: what remains of an open quota session, of a break, or until a schedule starts (announced two hours ahead). It turns red for the last five minutes, and shows `!` when Killer Chicken 9000 isn't answering. Clicking it shows which blocks cover the site and lets you add the site to a block's Blocked list (adding only makes a block stricter, so it is always allowed; it is switched off when the Blocks page has an [access password](#access-passwords)).

## Keyword test mode

```bash
kc9k list test adult on     # refused while the block is held: it loosens
kc9k list test adult off
```

While it is on, the block's lists that read a page's title or text do nothing. Instead a panel in the corner of every page shows which entries were found, how often each, how many different ones, and for each action whether its two thresholds are met. Use it to see whether a rule would be too harsh before you rely on it. Lists that only look at addresses keep working.

## Sites reached another way

A blocked site is also stopped when it is reached through another address: Google Translate, web archives, and any proxy that carries the address in its own (`?url=https://…`, also base64 or after a slash in the path; a search for an address doesn't count). A page that says it is another site (its canonical address) or that is a known alternative front-end (Invidious and Piped for YouTube, Nitter for X, Redlib for Reddit, ProxiTok for TikTok and others) is treated as that site. Each such page is logged as a `workaround` event, which bypass prevention can count.

## Options

```bash
kc9k option                          # shows them
kc9k option polite-close off
kc9k option block-page-details off
```

- `polite-close` (on): a blocked app with a window is asked to close, as if you pressed its close button, so it can offer to save; it is killed about five seconds later if it is still open. An app without a window is killed at once, and so is one started again within a minute of being asked. Off: every blocked app is killed at once.
- `block-page-details` (on): the block page names the block and the list that stopped the page, and when the block ends if it has an end. It is only what the block page says: redirects, pictures, closed tabs and every other action work exactly as before.

## The whole setup in one file

```bash
kc9k project export > my-setup.json
kc9k project import my-setup.json --dry-run   # says what would be added
kc9k project import my-setup.json
```

A project file holds every block (lists, apps, window titles, schedules, schedule lock, social media settings, quota, Pomodoro, breaks, bypass prevention), the clusters, browser settings, behaviour settings, the global bypass prevention, scheduled Frozen Chicken windows, and the pictures lists show. `kc9k project export` makes a **backup**, which also holds your places (with the Wi-Fi networks they are known by) and the blocks' place rules; `kc9k project export --share` leaves those out, so the file says nothing about where you are. Running locks, passwords, hotkeys and statistics are never in it. Importing only adds: a block whose name is taken here comes in as `name-2`, a cluster or place that exists is left as it is, browser settings are only turned on, and bypass prevention is replaced only by a stricter one, so an import never weakens anything. `--dry-run` shows what it would add; the interface always shows that first. Imported blocks aren't started. In the interface: Settings > Whole setup (Back up…, Share…, Import…).

## Access passwords

Optional passwords to open Killer Chicken 9000 and each of its pages. There is one for opening the program (`app`) and one per page: `dashboard`, `blocks`, `device`, `stats` and `settings`. Set none, one for everything, or a different one for each:

```bash
kc9k access                     # which are set
kc9k access set --all           # one password for everything
kc9k access set blocks stats    # one password for these two pages
kc9k access set settings        # and another for Settings
kc9k access remove stats
```

In the interface they are under Settings > Access passwords. A locked page shows a password box and nothing else; a password you type opens every page it belongs to, and stays remembered until you close the window, press Lock, or leave the window unused for the minutes you set there (a convenience kept in your own settings file, not a lock). With the `app` password set, nothing shows until it is given.

The check is made by `kc9k` itself, not by the interface, so the commands ask too: every command belongs to a page (`block`, `list`, `schedule`, `quota` … to Blocks; `freeze`, `strict`, `no-escape` to Device Blocks; `browser`, `hotkey`, `password`, `access` to Settings; `status`, `start`, `stop`, `unlock` to the Dashboard or Blocks, whichever is open) and runs only with the `app` password and that page's. On a terminal it asks; the interface passes them along. The daemon and hotkeys are not affected, so blocking goes on as before.

Changing or removing a password asks for the one it has (and for the settings password, when set). A wrong password takes a second and is logged as an `access` event, which bypass prevention can count. If one is lost, root can remove them all (`kc9k access --help` shows the command).

Access passwords decide who may look; the [settings password](#settings-password) decides who may loosen. They are separate and can be the same text.

## Strict mode

`kc9k strict on` denies sudo and pkexec for every user while a block or setting is locked or Frozen Chicken runs, in terminals and text consoles alike. Only `kc9k` keeps working, and rights return when the lock ends. Quotas and schedules do not trigger it. Recovery mode and another operating system still get around it, and a separately set root password still works with `su`.

Strict mode is off until you turn it on, and `kc9k strict off` works at any time, unless strict mode has its own setting lock or [no weakening during active locks](#no-weakening-during-active-locks) is on while something is locked. Its lock is set when turning it on, with the same flags as a browser setting's (`--for`, `--until`, `--lock`, `--password-stdin`, `--restart`); `kc9k strict unlock` lifts a text or password lock and leaves strict mode on. Strict mode's own lock only keeps it on: sudo is taken away only while something else is locked.

During a lock, programs also can't start from `/home`, `/tmp`, `/var/tmp` or `/dev/shm`, which stops AppImages and downloaded browsers. Programs already running continue.

Some groups are root in all but name: members of `docker`, `lxd`, `incus` or `libvirt` can start a container or virtual machine with the computer's disk inside it. During a lock, their control sockets are closed to everyone but root, and reopened when the lock ends. Members of `disk` can edit the drives directly, which strict mode can't prevent; `kc9k strict on` warns if you're in it.

### No weakening during active locks

`kc9k no-weakening on` makes every setting hold while any block or setting is locked: strict mode, browser settings, bypass prevention and the settings password can't be weakened, and `kc9k no-weakening off` is refused too. Blocks that aren't locked stay free to edit. In the interface: Settings.

`kc9k no-escape on --yes` removes the remaining exits: the kernel parameter stops working, recovery entries disappear, and GRUB gets a random password nobody knows. After that, only other boot media or a reinstall gets past a lock, even if a bug caused it. Test it in a virtual machine first. It can't stop booting a USB stick unless the firmware has a password.

**Emergency exit without no-escape:** add `kc9k.nofreeze` to the `linux` line in GRUB for that boot, or remove `/etc/sudoers.d/zz-kc9k-lockdown` and `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules` from recovery mode.

## Master password for testing

A build made with `sudo make install TAGS=devmaster` adds a master password. Set it once with `kc9k master-set`, then `kc9k master` lifts every lock, Frozen Chicken, strict mode and no-escape. Both commands read from standard input. During a freeze in lock mode they also work from a text console (Ctrl+Alt+F3); in logout and shutdown modes the console session ends within seconds, so use the `kc9k.nofreeze` boot escape below instead. A normal build has no such commands, so a release only needs to leave out the tag. A testing build also lets `ci/browser_test.py` drive LibreWolf while the root-owned file `/var/lib/kc9k/allow-remote-control` exists: browsers started for remote control aren't closed, and the developer-tools policy is left out, since it turns off the Marionette protocol WebDriver uses. Switching that policy closes open LibreWolf windows once during website blocks.

## Statistics

The daemon counts how long each app is in the foreground, and the bundled extension counts the time on each website in LibreWolf and other Firefox-family browsers. Time behind a locked screen, or after two minutes without input, isn't counted. Everything stays on this computer and is kept for a year.

```bash
kc9k stats              # totals for the last 7 days
kc9k stats --days 30    # the last 30 days
kc9k stats off          # stop recording ('on' to start again)
kc9k stats clear        # delete all statistics
```

App time needs an X11 session (Cinnamon's default). Website time is sent every 30 seconds, so up to half a minute is lost when the browser closes.

## Graphical interface

`make install` adds **Killer Chicken 9000** to the application menu (or run `kc9k-gui`). It needs GTK 3 for Python, which Linux Mint includes (`python3-gi`). Everything goes through the same command-line rules, so the interface can't do anything a lock forbids. It has five sections:

- **Dashboard**: active blocks and their locks, the state of Frozen Chicken, strict mode and no-escape, today's most used websites and apps, and a Monk mode… button.
- **Blocks**: website lists (blacklists and exceptions, each with its own boxes for where it looks and what it does, and import/export), apps, folders and window titles (with import/export), schedules, quota, Pomodoro and bypass prevention per block; starting a block with optional locks (time, text lock of random characters, words or your own text, until restart, delay, password). The typing challenge opens in a terminal. The Schedule tab shows the week as a calendar: drag down a day to add a window, click a window to remove it (filled windows turn the block on, outlined ones are allow windows).
- **Blocks**, more: a new block can start from a ready-made list; apps are picked from a searchable list of installed apps and running programs, or as a folder or a program file; the Schedule tab has one-click windows (work hours, evenings, weekends, nights) and shows the hours covered per week; further tabs for breaks, social media settings and places; the bypass prevention editor shows and edits every step of the ladder and what it counts.
- **Clusters**: website lists outside blocks, with a search box: create, edit, import and export them, copy one into a block, and record the sites you open into one.
- **Device Blocks**: whole-computer locks: Frozen Chicken, strict mode and no-escape.
- **Statistics**: time per day and the most used websites or apps, for today or the last 7, 30 or 90 days.
- **Settings**: the theme (light, dark or the system's, saved in `~/.config/kc9k/gui.json`), browser options, statistics, the settings password, bypass prevention for the whole computer, and hotkeys.

## Frozen Chicken

`kc9k freeze --for 2h --yes` (or the button in the interface) keeps every session locked until the time has passed.

Frozen Chicken can also run on a schedule, for example every night:

```bash
kc9k freeze schedule add --days daily --from 23:00 --to 24:00 --yes
kc9k freeze schedule add --days daily --from 00:00 --to 06:00 --mode shutdown --yes
kc9k freeze schedule list
kc9k freeze schedule remove 2
```

While a window is open, the computer stays frozen until the window ends, in that schedule's mode; back-to-back windows run on from one into the other, and a longer freeze already running is never shortened. A schedule can't be removed from 2 hours before its window opens until it closes, and uninstalling is refused then too. The interface has the same list under Device Blocks. Add `--mode logout` to end sessions or `--mode shutdown` to power off whenever the computer starts during the freeze. The daemon locks the screen again every 5 seconds, a freeze can only be extended, and the longest freeze is 24 hours. The time keeps counting while the computer is off.

**Emergency exit:** hold Shift or press Esc at boot to open GRUB, press `e` on the boot entry, add `kc9k.nofreeze` to the line starting with `linux`, then press F10. The freeze is not enforced during that boot, and it returns on the next normal boot if time remains.

## Limits

IPv6 is covered by the firewall's IP rules, but those rules are best-effort (shared CDN IPs, changing addresses); `/etc/hosts` and browser policy remain the authoritative layers. Containers and virtual machines have their own `/etc/hosts` and browsers, so for them only the firewall applies.

## License

MIT — see [LICENSE](LICENSE).
