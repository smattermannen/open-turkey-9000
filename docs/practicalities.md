# How we build the roadmap

Design notes for the roadmap in TODO.md: what each item takes, the choices
made, and what to test. Written for whoever picks an item up next.

## Threat model first

KC9K is custom security: the user picks how far it defends, at three levels.
Every loophole should be judged at the level where it matters:

1. **Basic (strict mode off).** You keep `sudo`, so you can always stop
   the daemon. Loopholes here only need to be more annoying than waiting.
2. **Strict mode during a lock.** No `sudo`, no `pkexec`, nothing executes
   from `/home` or `/tmp`. You are an ordinary user. **This is where loophole
   hunting pays off**: anything an ordinary user can do to reach a blocked
   site is a real hole.
3. **Physical.** Other boot media and recovery mode. `no-escape` plus a
   firmware password is the answer; KC9K can't do more.

Seen this way, a renamed browser in `/opt` is not a level-2 hole (it needs
root), but `docker run` is (the docker group is root without sudo).

## Level-2 holes and what closes them

| Hole | Status |
|---|---|
| docker / lxd / incus / libvirt group = root | Closed: strict mode makes their sockets root-only during a lock (`blocker/sockets.go`). |
| `disk` group = raw drive access | Can't be closed while running; `strict on` warns. |
| Container or VM traffic skips the firewall (FORWARD chain) | Closed: the jump is first in OUTPUT and FORWARD, and is put back on top (`blocker/firewall.go`). |
| Portable, renamed or copied browser, Tor Browser | Closed for require-extension: browsers are also known by engine (`blocker/browserdetect.go`). |
| Site shown through Google Translate or a web archive | Closed in the extension (`webext/files/match.js`). |
| **Unprivileged user namespaces** | **To test.** `unshare -Urm` lets a user mount an exec-allowed tmpfs and run a downloaded browser despite noexec. Engine detection still catches the browser, but check: `sysctl kernel.apparmor_restrict_unprivileged_userns` and `unshare -Urm sh -c 'mount -t tmpfs t /mnt && echo works'`. If it works on Mint, consider `user.max_user_namespaces=0` during a lock (breaks Flatpak sandboxes, so it needs thought). |
| Flatpak `--user` installs | **To test.** They live in `~/.local/share/flatpak`, which is noexec during a lock, so they should fail to start. Confirm. |
| Other user accounts / guest session | **To test.** Hosts and firewall cover everyone; process killing and the extension guard should too (they scan all processes). Strict mode's sudoers line covers every user. Check the guest session and a second account. |
| VPN / SSH tunnel / web proxy sites | Mostly harmless: the extension judges URLs regardless of route, and the hosts file and firewall still apply on a VPN. Generic web-proxy sites can't be decoded; a "proxies" template list could block the common ones. |
| Your test sudo rule | Remove `/etc/sudoers.d/99-claude-test` before real use. |

Every hole that gets fixed should get a test in `ci/integration.sh` (or a Go
test with `-tags roottest`, like `blocker/firewall_root_test.go`), so it
can't come back.

## Window-title blocking

Cold Turkey closes any app whose window title contains a keyword.

- **Data:** a `titles` list per block, like sites and apps
  (`block add-title <block> <keyword...>`, `remove-title`). Adding is
  strengthening; removing is a loosening edit, gated like the others.
- **Reading titles:** Cinnamon on Mint is X11. The daemon is root, so it can
  connect to the user's display with their `~/.Xauthority` (as `freeze.go`
  already does through `asUser`). Two options:
  - shell out to `xprop` as the user (installed on Mint): simple, but one
    process per window per cycle;
  - a small X11 client in Go (`github.com/jezek/xgb`, pure Go, so CGo stays
    off): read `_NET_CLIENT_LIST`, then each window's `_NET_WM_NAME` and
    `_NET_WM_PID`. Faster, and lets us close windows politely. **Preferred.**
- **Acting:** for a normal app, ask the window to close (`_NET_CLOSE_WINDOW`),
  and kill its process only if the window is still there a few seconds later,
  so unsaved work gets the app's own "save?" prompt first. For **LibreWolf**,
  don't kill the browser over one tab: send the keywords to the extension,
  which checks `tab.title` and shows the block page for that tab only.
- **Wayland:** window titles aren't readable from outside on Wayland. Mint's
  Cinnamon is X11 by default; document the limit, and detect Wayland sessions
  (`XDG_SESSION_TYPE`) to warn instead of silently doing nothing.

## GUI

- **Weekly calendar:** a `Gtk.DrawingArea` with 7 columns × 24 hours.
  Dragging creates a window (`schedule add`); clicking an existing window
  offers removal (refused inside the 2-hour freeze, with the CLI's message).
  Colours per block; block windows and allow windows look different.
- **Tray icon:** `AyatanaAppIndicator3` (on Mint through `gir1.2-ayatanaappindicator3-0.1`).
  Shows active blocks and lock time left; menu items open the interface.
  Start it with the session through an autostart `.desktop` file.
- **First-run wizard:** shown when there are no blocks: pick templates,
  install LibreWolf, turn on require-extension, explain strict mode.
- **Typing challenge in its own window:** a GTK window that shows the text a
  screenful at a time, so the 5000-character challenge stays usable. It must
  keep the terminal version's protections: reject pastes (watch for
  multi-character inserts and clipboard events), no per-character hints,
  score by edit distance. The CLI does the checking; the window only collects
  the input and passes it on standard input, like passwords now.
- **Errors in plain words:** every refused change should say why and when it
  becomes possible ("locked until 15:00").

## Remaining features

- **Scheduled Frozen Chicken:** reuse `schedule.Window` with a special target
  instead of a block, so "every day 23:00–06:00" freezes the computer.
  Removing such a schedule is loosening (2-hour freeze before a window).
- **Notifications:** `notify-send` as the user (pattern: `asUser` in
  `freeze.go`). Events: a schedule window opens in 5 minutes, a quota has 5
  minutes left, a lock ends.
- **Settings password:** one password for the whole interface and CLI
  (`kc9k settings-password set`); every changing command asks for it. Store
  a PBKDF2 hash like block passwords. Removing it needs the password.

## Security against attackers

Here the threat is malware or a malicious website, not you.

- **The passwordless sudo rule is the attack surface.** Any program running
  as you can run `kc9k` as root. Audit every command for hostile arguments:
  paths (symlinks, `..`), imports (already read as the caller), anything
  written under `/etc`, the LibreWolf installer. sudo's `env_reset` stops
  `KC9K_DB` and similar from being passed in; keep it that way (never
  `sudo -E` in the wrapper).
- **The rules server on 127.0.0.1:47613:** any local program can read the
  block lists, post fake statistics, or make the extension guard believe a
  browser checked in. Give the extension a secret token (generated at
  install, placed in the extension through policy) and require it.
- **The extension's block page** is reachable by web pages; it only follows
  http(s) addresses back. Keep every input from pages treated as hostile.
- **Releases:** build without `devmaster`, pin dependencies (`go.sum`
  already does), and sign release files.

## Packaging

A `.deb` built by `make deb` (nfpm, or plain `dpkg-deb` with a staging
directory) that installs the binary, wrapper, sudoers rule, unit, GUI and
desktop file, with a `postinst` that starts the service and a `prerm` that
runs `prepare-uninstall`, so uninstalling is refused during a lock.
