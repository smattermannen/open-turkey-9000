# Apps, window titles, polite close, behaviour settings

- Covers: `blocker/process.go`, `owners.go`, `scripts.go`, `titles.go`, `x11.go`, `polite.go`, `activity.go`, `idle.go`, `cli/titles.go`, `cli/options.go`, `enforce/options.go`
- SPEC: §3.1, R-TITLE-1, R-DESKTOP-1
- Connects to: [enforce-daemon](enforce-daemon.md), [actions](actions.md) (trigger actions), [allowance](allowance.md) (app use)

## Apps (`process.go`, `scripts.go`, `owners.go`)

- Kill by exact name, Flatpak ID (`app-flatpak-<ID>-<n>.scope` or
  `/.flatpak-info`), desktop units `app-[<launcher>-]<ID>-<n>.scope`/`@….service`
  (ID equal to or ending in `.<name>`); descendants too, except under
  shells/session processes.
- Python by script (`pythonScript`; Windows: every script host,
  `scripthosts.go`); copies by hash (`HashEntry`
  `sha256:<hex>:<size>:<path>`, hidden by `visibleApps`; breaks on
  rebuild). `python3` itself, desktop essentials (`essentialProcess`:
  `Xorg`, `cinnamon`, `pipewire`…) and system folders (`/usr/bin/`) refused;
  system programs never match.
- Only people's processes killed (`personUser`: uid ≥ 1000, or a LightDM
  guest `guest-*` with home `/tmp/guest-*`; also for logout, lock screen,
  desktop users).
- `KillNaming`/`KillNamingFor`: fetch tools given a blocked domain
  (`yt-dlp`, `mpv`, `vlc`, `curl`, `wget`, `streamlink`…; never
  editors/shells), per user for a block's affected users.

## Window titles (`titles.go`, `x11.go`)

Case-insensitive substring; X11 only (Wayland: XWayland apps). Asks
matching windows to close (also file managers showing a folder entry:
`folderWindow`), kills with descendants after `TitleCloseGrace`. Targets
per user (`EnforceUsers(TitlesByUser)`, §3.2; `TitleHit` names the user).
Runs beside the cycle (`enforceTitles`). Browsers left to the extension.
Cgo-free X11 client: user's own Xauthority only, cookie only to a
root/user-owned socket, capped replies, 3 s budget, PID trusted only for
the session user.

## Activity (`activity.go`, `idle.go`)

- Focused app per desktop user (`xprop`); away (`awayFromDesktop`) = screen
  locked or idle past the limit.
- Locked = logind `LockedHint` on any X11/Wayland session of the user
  (`lockedByLogind` via `loginctl`), never the session bus (any user
  program can answer for `org.cinnamon.ScreenSaver`; B8). Known gap: the
  owner may call `SetLockedHint` themselves. No logind: idle decides.
- Idle: X server first (MIT-SCREEN-SAVER QueryInfo; not Xwayland), else
  Cinnamon `GetSessionIdleTime` (cinnamon-screensaver 6 answers "not
  implemented"). Neither: counts as there.

## Behaviour settings (`enforce/options.go`, `kc9k behaviour`, alias `option`)

Never locked. `polite-close` (on, `polite.go`): windowed apps asked, killed
~5 s later (a cycle); windowless or restarted within a minute killed at
once. `block-page-details` (on): block page names block, list, end time.
