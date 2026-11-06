This repository was originally forked from [bdcdo/open-turkey](https://github.com/bdcdo/open-turkey). The README below comes directly from that repo. This repo adds some changes to that repo that I added while testing out AI coding tools that I found particularly useful.

# Killer Chicken 9000 (KC9K)

A command-line productivity blocker for Linux, backed by a `systemd` service.

Killer Chicken 9000 is an open-source, Linux-native alternative to [Cold Turkey](https://getcoldturkey.com/): it blocks distracting websites and applications and keeps them blocked, even against your own attempts to undo it on impulse. Where Cold Turkey is paid and centered on Windows/macOS, Killer Chicken 9000 is free, built in Go, and designed around the way Linux actually enforces policy.

It organizes blocking into **blocks** — named groups of:

- **Sites** (domains, e.g. `youtube.com`)
- **Apps** (process names, e.g. `discord`, `telegram-desktop`, or Flatpak app IDs, e.g. `com.discordapp.Discord`)

When you **activate** a block, a daemon continuously ensures the blocking layers stay applied.

## How it works — 4 enforcement layers

Killer Chicken 9000 doesn't rely on a single, easily-bypassed mechanism. Each active block is enforced on four independent layers, and a `systemd` daemon re-applies them every 5 seconds if anything is tampered with:

1. **`/etc/hosts`** — resolves blocked domains to `0.0.0.0` (affects every program, not just browsers). Written atomically, and every entry is validated so a crafted domain can't inject extra lines.
2. **Firewall (`iptables` + `ip6tables`)** — drops packets to the blocked sites' real IPv4 and IPv6 addresses, and drops DNS-over-HTTPS (DoH) to well-known resolvers so browsers can't resolve names over HTTPS and slip past `/etc/hosts`. The real addresses are looked up directly from public resolvers, bypassing the poisoned `/etc/hosts`.
3. **Browser enterprise policies** — managed `URLBlocklist`/`WebsiteFilter` policies for Firefox, Chromium, Google Chrome, Brave, Edge, Opera and Vivaldi, plus DoH turned off and locked in the browser itself. The user cannot disable these from inside the browser, not even in private/incognito mode.
4. **Process kill** — terminates blocked apps that are running (SIGKILL), matched by exact executable name so unrelated processes aren't caught. A Flatpak app ID matches every process in that app's sandbox, whatever its executables are called. Everything a blocked app has launched is killed with it — e.g. blocking `lutris` (or `steam`, `heroic`) also kills the games started from it. And when an app was opened from your desktop's menu or dock, every process in its desktop app unit is killed too, even ones that were detached from it, as long as the unit's app ID is the blocked name or ends in `.<name>` (so `lutris` covers `net.lutris.Lutris`, and `discord` covers the `com.discordapp.Discord` Flatpak). If you block a shell or session process itself (`bash`, `tmux`, `plasmashell`, …), only that process is killed, not everything started from it.

Because the hosts and firewall layers act below the browser, blocking works even for browsers installed via Snap or Flatpak (where the system-wide browser policy file may not be read).

> **Threat model.** Killer Chicken 9000 creates *friction*, not a security boundary. Anyone with root can, with enough determination, tear the blocks down — that's intentional: it's your machine, and there is always a recovery path so you can't lock yourself out. The layers, the locked-block rules and the typing challenge exist to outlast an *impulse*, not a deliberate root-level effort. Blocking is IPv4/IPv6 by IP, but IP rules are best-effort: addresses change and CDNs are shared, so `/etc/hosts` and browser policy are the authoritative layers.

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

The installed program is independent of the source directory — once installed, you can remove the project folder and it keeps working.

## Concepts

- **Block**: a set of sites/apps (and optional browser patterns) you block together. A block is either a **blocklist** (block its sites) or an **allowlist** (`--allowlist`: block everything *except* its sites, enforced at the browser layer).
- **Overlapping blocks**: several blocks can be active at once, and everything they enforce is combined: all their blocked sites, apps and patterns apply. Allowlists combine too: with two allowlist blocks active, a site allowed by **either** one is reachable. To make a held block stricter, see [Strengthening a held block](#strengthening-a-held-block).
- **Active**: a block currently in effect (the system is enforcing it).
- **Character lock (`--lock`)**: prevents `stop`. The only way out is `unlock`, which shows a string of random characters and requires typing all of it in one go, with at least 98% accuracy (at most 6 errors in the default 300 characters). Mistakes aren't highlighted (Backspace works, but you have to spot them yourself), paste is rejected, and `--lock-chars` sets the length (default 300 characters, several minutes of typing) — deliberate friction to outlast an impulse. Each character is a letter (upper or lower case) 60% of the time, a digit 35% of the time and a symbol 5% of the time; symbols can be any printable ASCII symbol on a US keyboard (`` !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~ ``).
- **Time lock (`--for` / `--until`)**: the block cannot be removed *at all* — not even with the challenge — until the time passes. The strongest lock. While it runs, only time the computer is on or suspended counts: changing the system clock has no effect, and time powered off is not counted, so a shutdown extends the lock by that long.
- **Schedule**: recurring windows (e.g. weekdays 09:00–17:00). The daemon turns the block on when a window opens and off when it closes; while a window is open the block can't be stopped. With **allow windows** it's the other way round: the block is on at all times *except* during its allow windows (e.g. allow games only on Friday and Saturday evenings), and it can never be loosened on the spot — loosening changes are queued for 24 hours, or applied with `--now` after the typing challenge. Starting **2 hours before** a window opens, the block is frozen: it can't be weakened or removed (only made stricter), and that schedule can't be removed, until the window closes.

While a block is *held* (char-locked, time-locked, in a schedule window, or within 2 hours of one opening) it can't be removed or weakened, and `make uninstall` refuses. It can still be made **stricter**: see [Strengthening a held block](#strengthening-a-held-block). Everything is friction, not a security boundary — a determined root user can always recover (see the threat-model note above).

## Commands

### 1) Create and configure a block

```bash
kc9k block create social-media
kc9k block add-site social-media instagram.com x.com facebook.com
kc9k block add-app  social-media discord telegram
```

Blocking an app also kills everything it launched — block `lutris` and the games started from Lutris close too.

For a Flatpak app, block it by its app ID (see `flatpak list --app`). This is more reliable than guessing the process name, since many Flatpaks run under generic names like `python3` or `electron`:

```bash
kc9k block add-app social-media com.discordapp.Discord
```

Seed from a built-in template (`social-media`, `news`, `video`, `shopping`), or make an allowlist block:

```bash
kc9k block create focus --template social-media
kc9k block create work  --allowlist
kc9k block set-mode work allowlist   # or blocklist
```

Choose what a block does to its websites. Each block has three boxes to tick, in any combination: **block** them (the default), show them in **greyscale**, and block their **images**. In allowlist mode the actions apply to every site except the listed ones. A block without `block` leaves its sites reachable and only changes how they look; its apps are still blocked. Greyscale and no-images work in browsers with the extension (see Browser options). The interface shows the same three boxes on a block's Websites tab.

```bash
kc9k block action social-media greyscale on   # greyscale as well as blocked
kc9k block action reddit block off            # reachable, but grey and without images
kc9k block action reddit                      # list the block's actions
```

Ticking a box is always allowed. Unticking one loosens the block, so it is refused while the block is held, and queued for a block with allow windows. At least one action stays on.

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

Activate with a character lock, or a time lock:

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

Allow windows invert this: the block is on at all times **except** while one of its allow windows is open. For example, to allow a `games` block only on Friday and Saturday evenings:

```bash
kc9k schedule add games --allow --days fri,sat --from 18:00 --to 24:00
```

A block's schedules must be all block windows or all allow windows. `--to 24:00` means midnight, so a window can run to the end of the day without a one-minute gap.

Schedules are numbered `1, 2, 3, …` per block. Removing one moves the later ones down a number (remove `1` from `1, 2, 3` and the old `2`/`3` become `1`/`2`), and a new schedule takes the next number.

Windows are same-day; for an overnight span (e.g. 17:00–05:00) add two windows (`17:00–24:00` and `00:00–05:00`). From **2 hours before** a schedule's window opens until it closes, that schedule **can't be removed** — otherwise deleting it would end the block early or stop it from starting, defeating the point. Schedules whose windows aren't open or about to open can still be removed at any time, and you can always add new block windows.

Whenever its schedule has the block on, or will within 2 hours, the block itself is frozen: it can't be weakened (removing apps or patterns, changing its mode, …) or removed, though it can still be made stricter (see [Strengthening a held block](#strengthening-a-held-block)).

#### Loosening a block with allow windows

A block with allow windows is meant to stay blocked outside them, so it can't be loosened directly **at any time** — not even during an allow window, which would otherwise be the moment to dismantle it. Loosening changes are:

- removing apps or patterns, `remove-site` on a blocklist block, `add-site` on an allowlist block (or `remove-site` that would empty it), `set-mode`, and turning a website action off (`block action <block> <action> off`);
- `block remove`;
- adding another allow window, or removing the **last** one (removing one of several only makes the block stricter, so that's immediate).

Such a change is **queued** and applied by the daemon **24 hours** later. Add `--now` to apply it immediately instead, after passing the same typing challenge as `unlock`:

```bash
kc9k block remove-app games steam          # queued; takes effect in 24h
kc9k block remove-app games steam --now    # typing challenge, then immediate
kc9k pending                               # list queued changes
kc9k pending cancel 1                      # drop one
```

Queued changes also show in `block info` and `status`. If the block is also character- or time-locked (from `start --lock/--for/--until`), loosening is refused, and an already-queued change waits until those locks lift. Changes that make the block stricter (adding apps, sites to a blocklist, patterns, locks) always apply immediately.

Show status:

```bash
kc9k status
```

Deactivate (only if the block isn't held — see Concepts):

```bash
kc9k stop social-media
```

Unlock a character-locked block (this also **deactivates** it). Time locks and open schedule windows can't be unlocked — they lift on their own:

```bash
kc9k unlock social-media
```

To practice or test the challenge without touching any block (no root needed), run this from the source directory, optionally with a length:

```bash
./try-challenge.sh        # 300 characters
./try-challenge.sh 100
```

### 3) Editing a block's lists

Remove a domain from a block:

```bash
kc9k block remove-site social-media instagram.com
```

Notes:

- If the block is **active**, Killer Chicken 9000 re-applies the layers so the change takes effect immediately.
- If the block is **held** (character-locked, time-locked, in an open schedule window, or within 2 hours of one opening), you **cannot** loosen it: removing apps or patterns, removing sites from a blocklist, adding sites to an allowlist, changing its mode, and turning a website action off are refused. Clear the hold first (unlock, or wait for the time lock / schedule window to end), make your changes, then activate again. Edits that only make it stricter are always allowed (next section).

#### Strengthening a held block

A held block can always be made stricter, since that can't be used to escape it:

| Allowed while held | Why it's safe |
|---|---|
| `block add-app`, `block add-pattern` | apps and patterns are blocked in both modes |
| `block add-site` on a **blocklist** block | blocks more sites |
| `block action <block> <action> on` | blocks, greys out or strips images from more |
| `block remove-site` on an **allowlist** block | allows fewer sites — refused if it would leave the block with nothing at all, since an empty block is switched off |
| `start <block> --lock [--lock-chars N]` on an **active** block | adds a char lock, or lengthens the challenge (a shorter `--lock-chars` is refused) |
| `start <block> --for D` / `--until HH:MM` on an **active** block | adds a time lock, or extends it (an earlier end is refused) |

```bash
kc9k start social-media --for 3h       # already active: extend its time lock
kc9k block add-site social-media reddit.com
```

If you lock a block that its schedule switched on, the lock outlives the schedule: when the window closes, a time-locked block stays on until its time lock expires (the daemon then switches it off), and a char-locked one stays on until you `unlock` it.

Remove an app (process) from a block:

```bash
kc9k block remove-app social-media discord
```

Remove an entire block (must be inactive, and not within 2 hours of a schedule window):

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

Uninstalling first runs a guard that **refuses while any block is held** (character-locked, time-locked, in an open schedule window, or within 2 hours of one opening) **or has allow windows**, so uninstalling can't be used to sidestep a lock — clear the hold first, and remove allow windows (queued or `--now`, see above). When it proceeds, it deactivates all blocks and removes every blocking layer (`/etc/hosts` entries, firewall rules, browser policies) so the system is left clean rather than blocked-with-no-tool.

## Blocked lists and exceptions

Each block has a list of what is blocked and a list of exceptions, and an exception beats the block:

```bash
kc9k block add work reddit.com 'reddit.com/r/funny' '*.xxx' '*.*'
kc9k block add-exception work reddit.com/r/programming
kc9k block export-list work blocked > blocked.txt
kc9k block export-list work exceptions > exceptions.txt
kc9k block import-list work blocked blocked.txt
```

Files hold one entry per line, and `#` starts a comment. `*.*` blocks every website, so combine it with exceptions to allow only what you need. A domain with an exception leaves `/etc/hosts` and the firewall, because those block whole hosts, so only browser policy enforces it. Chromium-based browsers cannot match wildcards inside a path or keywords in policy, so those entries work only in Firefox for now, and the interface says which ones.

## Daily quotas

```bash
kc9k quota set social-media 30m     # the block now stays on
kc9k quota start social-media       # open it and spend the quota
kc9k quota end social-media         # close it early
kc9k quota remove social-media      # takes effect at midnight
```

All time in an open session is spent, whether or not the sites are used, because Killer Chicken 9000 cannot see inside a browser tab. Lowering a quota applies at once. Raising or removing it applies at midnight. A lock or a scheduled window closes a session. The day follows the system clock, so moving the clock to another date refills the quota.

## Locks

| Lock | Command | Ends when |
|---|---|---|
| Timer | `start b --for 2h` | the time has passed |
| Time range | `start b --until 17:00` | the clock time is reached |
| Random text | `start b --lock` | the challenge is typed with `unlock` |
| Password | `start b --password-stdin` | someone enters it with `unlock`, from standard input |
| Restart | `start b --restart` | the computer has restarted, then `stop` works |
| Delay | `start b --delay 30m` | the delay has passed after `stop` |

## Browser options

The daemon force-installs the bundled extension in Firefox-family browsers through policy, so it can't be removed or disabled. It enforces wildcards and keywords, shows a block page, and reads its rules from the daemon on `127.0.0.1:47613`.

```bash
kc9k browser install-librewolf    # install LibreWolf (official repository, via extrepo)
kc9k browser force-librewolf on   # close other browsers (needs LibreWolf installed)
kc9k browser force-search on      # Google as the only search engine
kc9k browser safesearch on        # lock Google SafeSearch
kc9k browser web-only on          # web results only: no images, shopping or AI
kc9k browser                      # list every option and whether it's on
```

Options can always be turned on, and can't be turned off during a lock. Greyscale and image blocking are chosen per block instead (`kc9k block action`), so a block's own locks protect them. Web-only relies on Google's unofficial `udm=14` parameter, which may change. Firefox only accepts signed extensions, so the bundled unsigned one needs signing to work there.

## Pomodoro

`kc9k pomodoro set work 25m 5m` alternates a block between work and breaks while it runs, with locks kept in place. Setting a cycle is refused while the block is held, and `pomodoro off` is always allowed.

## Blocking folders

`kc9k block add-app games /opt/spel/` blocks every program under a folder, and a full path blocks one file. System folders such as `/usr/bin/` are refused.

## Strict mode

`kc9k strict on` denies sudo and pkexec for every user while a block is locked or Frozen Chicken runs, in terminals and text consoles alike. Only `kc9k` keeps working, and rights return when the lock ends. Quotas and schedules do not trigger it. Recovery mode and another operating system still get around it, and a separately set root password still works with `su`.

During a lock, programs also can't start from `/home`, `/tmp`, `/var/tmp` or `/dev/shm`, which stops AppImages and downloaded browsers. Programs already running continue.

`kc9k no-escape on --yes` removes the remaining exits: the kernel parameter stops working, recovery entries disappear, and GRUB gets a random password nobody knows. After that, only other boot media or a reinstall gets past a lock, even if a bug caused it. Test it in a virtual machine first. It can't stop booting a USB stick unless the firmware has a password.

**Emergency exit without no-escape:** add `kc9k.nofreeze` to the `linux` line in GRUB for that boot, or remove `/etc/sudoers.d/zz-kc9k-lockdown` and `/etc/polkit-1/rules.d/00-kc9k-lockdown.rules` from recovery mode.

## Master password for testing

A build made with `sudo make install TAGS=devmaster` adds a master password. Set it once with `kc9k master-set`, then `kc9k master` lifts every lock, Frozen Chicken, strict mode and no-escape. Both commands read from standard input, and both work from a text console during a freeze. A normal build has no such commands, so a release only needs to leave out the tag.

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

- **Dashboard**: active blocks and their locks, the state of Frozen Chicken, strict mode and no-escape, and today's most used websites and apps.
- **Blocks**: websites and exceptions, apps and folders, schedules, quota and Pomodoro per block; starting a block with optional locks (time, typing challenge, until restart, delay, password). The typing challenge opens in a terminal.
- **Device Blocks**: whole-computer locks: Frozen Chicken, strict mode and no-escape.
- **Statistics**: time per day and the most used websites or apps, for today or the last 7, 30 or 90 days.
- **Settings**: browser options and statistics.

## Frozen Chicken

`kc9k freeze --for 2h --yes` (or the button in the interface) keeps every session locked until the time has passed. Add `--mode logout` to end sessions or `--mode shutdown` to power off whenever the computer starts during the freeze. The daemon locks the screen again every 5 seconds, a freeze can only be extended, and the longest freeze is 24 hours. The time keeps counting while the computer is off.

**Emergency exit:** hold Shift or press Esc at boot to open GRUB, press `e` on the boot entry, add `kc9k.nofreeze` to the line starting with `linux`, then press F10. The freeze is not enforced during that boot, and it returns on the next normal boot if time remains.

## Limits

IPv6 is covered by the firewall's IP rules, but those rules are best-effort (shared CDN IPs, changing addresses); `/etc/hosts` and browser policy remain the authoritative layers.

## License

MIT — see [LICENSE](LICENSE).
