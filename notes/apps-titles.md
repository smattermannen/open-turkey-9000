# Apps, window titles, polite close, behaviour settings

- Covers: `blocker/process.go`, `scripts.go`, `titles.go`, `x11.go`, `polite.go`, `activity.go`, `idle.go`, `cli/titles.go`, `cli/options.go`, `enforce/options.go`
- SPEC: §3.1, R-TITLE-1, R-DESKTOP-1
- Connects to: [enforce-daemon](enforce-daemon.md), [actions](actions.md) (trigger actions), [allowance](allowance.md) (app use)

## Code

- Process kill: exact name, Flatpak ID (`app-flatpak-<ID>-<n>.scope` or
  `/.flatpak-info`), desktop units `app-[<launcher>-]<ID>-<n>.scope`/`@….service`
  with ID equal to or ending in `.<name>`; descendants too, except under
  shells/session processes. `scripts.go`: Python by script
  (`pythonScript`, `essentialProcess`), copies by hash (`HashEntry`
  `sha256:<hex>:<size>:<path>`, hidden by `visibleApps`; system programs
  never match), `KillNaming`/`KillNamingFor` (fetch tools given a blocked
  domain; per user for a block's affected users).
- `titles.go`/`x11.go`: asks matching windows to close (also file manager
  windows showing a folder entry: `folderWindow`); targets per user
  (`EnforceUsers(TitlesByUser)`, §3.2; `TitleHit` names the user), kills after
  `TitleCloseGrace`; cgo-free X11 client (user's own Xauthority only,
  cookie only to root/user-owned socket, capped replies, 3 s budget, PID
  trusted only for the session user). Runs beside the cycle
  (`enforceTitles`). Browsers left to the extension.
- `polite.go`: `polite-close` asks windows to close, kills a cycle later.
- `activity.go`: focused app per desktop user (`xprop`), and who is at the
  desktop (`awayFromDesktop`: screen locked, or idle past the limit).
  Locked: logind's `LockedHint` on every X11/Wayland session of the user
  (`screenLocked`, `lockedByLogind` via `loginctl`; set by the desktop's
  locker), never the session bus (any program the user runs can answer
  for `org.cinnamon.ScreenSaver`; B8, 2026-11-21). Left: the session's
  owner may call `SetLockedHint` themselves (logind allows it); no logind:
  never locked, so idle time decides. Idle time from the X server first (`idle.go`, MIT-SCREEN-SAVER
  QueryInfo over the x11.go client; not Xwayland, which sees only X11
  input), else Cinnamon's `GetSessionIdleTime` (the session bus is the
  user's own; cinnamon-screensaver 6, Mint 22, answers "not implemented",
  so before 2026-11-20 idle time counted as use). Neither: counts as there.
- **Behaviour settings** (`enforce/options.go`, `kc9k behaviour`, alias
  `option`): `polite-close`, `block-page-details`; never locked.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Window titles**: case-insensitive substring; X11 only (Wayland: XWayland
  apps only); asked to close, killed with descendants a few seconds later.
- **Apps**: hash match breaks on rebuild; `python3` itself and desktop
  essentials (`Xorg`, `cinnamon`, `pipewire`…) can't be added; system
  folders (`/usr/bin/`) refused; only regular-user processes are killed.
  Fetch tools killed for blocked addresses: `yt-dlp`, `mpv`, `vlc`, `curl`,
  `wget`, `streamlink`…; never editors/shells.
- **Options**: `polite-close` (on): windowed apps asked, killed ~5 s later;
  windowless or restarted within a minute killed at once.
  `block-page-details` (on): block page names block, list, end time.
