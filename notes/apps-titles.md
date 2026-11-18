# Apps, window titles, polite close, behaviour settings

- Covers: `blocker/process.go`, `scripts.go`, `titles.go`, `x11.go`, `polite.go`, `activity.go`, `cli/titles.go`, `cli/options.go`, `enforce/options.go`
- SPEC: §3.1, R-TITLE-1, R-DESKTOP-1
- Connects to: [enforce-daemon](enforce-daemon.md), [actions](actions.md) (trigger actions), [allowance](allowance.md) (app use)

## Code

- Process kill: exact name, Flatpak ID (`app-flatpak-<ID>-<n>.scope` or
  `/.flatpak-info`), desktop units `app-[<launcher>-]<ID>-<n>.scope`/`@….service`
  with ID equal to or ending in `.<name>`; descendants too, except under
  shells/session processes. `scripts.go`: Python by script
  (`pythonScript`, `essentialProcess`), copies by hash (`HashEntry`
  `sha256:<hex>:<size>:<path>`, hidden by `visibleApps`; system programs
  never match), `KillNaming` (fetch tools given a blocked domain).
- `titles.go`/`x11.go`: asks matching windows to close (also file manager
  windows showing a folder entry: `SetFolders`, `folderWindow`), kills after
  `TitleCloseGrace`; cgo-free X11 client (user's own Xauthority only,
  cookie only to root/user-owned socket, capped replies, 3 s budget, PID
  trusted only for the session user). Runs beside the cycle
  (`enforceTitles`). Browsers left to the extension.
- `polite.go`: `polite-close` asks windows to close, kills a cycle later.
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
