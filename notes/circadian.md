# Circadian rhythm and wallpapers

- Covers: `internal/circadian/`, `enforce/circadian.go`, `wallpapers.go`, `blocker/backlight.go`, `gamma.go`, `desktop*.go`, `sleephook.go`, `cli/circadian.go`, `wallpapers.go`, `projectcircadian.go`, `projectwallpapers.go`
- SPEC: §4.10 Circadian rhythm / Wallpaper default, R-MODE-5
- Connects to: [automation](automation.md) (modes, places), [extension](extension.md) (dark pages), [gui](gui.md)

## Code

- **Circadian** (`internal/circadian/`, `enforce/circadian.go`,
  `cli/circadian.go`; GUI `CircadianPage`, `ClipEditor`): JSON settings
  `circadian` (`circadian.Config`: three clips of `Point{Minute, Value,
  Tension}`, wallpapers, dark mode, dark pages), `.paused`, `.preview`,
  `.applied`, `.resume`, `.status`. `RunCircadian` goroutine every 5 s,
  never in `Tick`; `ddcutil` and night light every 30 s. Foreign levels
  pause a clip (`PauseBacklight`, `PauseExternal`, `PauseGamma`; brightness
  and kelvin share one gamma ramp), except right after start. Wallpaper and
  dark mode applied once per key change (`WallpaperKey`, `DarkKey`), also
  into signed-out users' dconf (`blocker.Reach`, `dbus-run-session`); who
  got it is in `circadian.applied` (`WallpaperUsers`), a later sign-in
  without it gets it at once.
  `PreviewCircadian` drives hardware; engine adopts it: a gamma preview
  notes the ramp it wrote (`Kelvin`, `Brightness`, `At`), the engine
  takes over the one written last (`lastGammaPreview`), never one mixed
  with an ended preview's value. `circadian play`
  (`PlayCircadian`: 24 h as previews in `--seconds`, wallpaper/dark mode at
  their times, `ReadDesktopLook`/`RestoreDesktopLook` put the desktop back;
  GUI "Play the day" with a playhead). GUI wallpapers: `WallpaperTimeline`
  (drag bar/ends, 5-min steps, Shift minutes; covered parts faint). No
  Circadian section in Settings. Hardware vars
  `circadian*` (`fakeCircadian`); `KC9K_DB` → `circadianDry`. Wallpapers in
  `<db dir>/wallpapers` (0644). `rules.dark` (`CircadianDarkRule`) →
  `darkPage`; the popup's "Exempt from dark pages" → background
  `darkExempt` → daemon `/dark-exempt` (`darkExemptHandler`, refused
  behind an app/settings access password). `circadian.Shape` mirrored by
  GUI `clip_shape`.
  Panels (`circadianBacklights`) are looked for each step only while the
  backlight clip is on, else once and after a wake (status keeps the
  names); Play the day finds them once and sets panels and external
  screens at most once a second plus the last frame. On Windows a look,
  read or set starts PowerShell, limited further in `notes/windows.md`
  (Circadian). *(By claude)*
  `ci/circadian_test.py` tests dark pages. The wallpaper the engine gives
  is `pickWallpaper`'s (wallpapers.go, below), not only circadian rhythm's.
- **Wallpapers of modes** (R-MODE-5, §4.10; owner 2026-11-17;
  `enforce/wallpapers.go`, `cli/wallpapers.go`, `cli/projectwallpapers.go`,
  `blocker/sleephook.go`): settings `mode.wallpapers.<mode id>`
  (`[]circadian.Wallpaper`; Frozen Chicken `FrozenChickenWallpapers` = 0)
  and `wallpaper` (`WallpaperSettings`: `CircadianFirst`, `Default`
  latest|circadian|custom, `Custom` picture kept when another default is
  chosen). `lookAtModes` (enabled modes; one switched by places while
  `placesHere` is unknown makes it `unsure`; `modeOnSince`: the one
  enabled last wins) → `pickWallpaper` (`keep`: nothing given, nothing
  forgotten). The engine (`Circadian.desktop`) holds a place known in the
  last minute (`placeGrace`), forgets it at once on a wake
  (`sleptSeconds` grew; `ForgetSeenNetworks`), steps every second while
  `lookingForPlace`. `syncSleepHook` writes `/usr/lib/systemd/system-sleep/kc9k`
  (`circadianSleepHook`, faked in `fakeCircadian`) while a mode switched by
  places has wallpapers and the default isn't latest; it runs hidden
  `kc9k wallpaper before-sleep` (`WallpaperBeforeSleep`, access-exempt),
  which marks `wallpaper.before_sleep`; after the wake the engine gives
  afresh (`beforeSleepGiven`). Pictures go when nothing uses them
  (`WallpaperInUse`, `forgetWallpaper`). CLI `kc9k mode wallpaper M
  [add|time|remove]`, `kc9k wallpaper [priority|default]`; gui-state
  `modes[].wallpapers`, `wallpaper`; circadian view `override_modes`. GUI
  `WallpaperEditor` (circadian page and mode view), `choose_picture`,
  mode view `mode_override`, circadian `override_modes`, Settings
  `wallpaper_default`. Project files: modes' `custom`/`wallpapers`,
  `wallpaper` (settings, Frozen Chicken's, pictures); import lists custom
  actions (`projectCustoms`) and needs `--yes` (`confirmCustoms`).
  `TestModeWallpaper*`, `TestWallpaperBeforeSleep`,
  `TestModeWallpaperCommands`, `ci/gui_wallpaper_smoke.py`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Circadian**: `kc9k circadian` (`on|off`, `preset [N]`, `lane L on
  --points "07:00=6500 19:00=6500~-0.4"`, `point add L HH:MM V --tension
  T`, `resume [L]`, `wallpaper add F A B | time N A B | remove N`, `dark
  system|light|dark|light-between A B`, `dark-pages`, `exempt`,
  `nightlight off`). Ranges: software 10–100 %, kelvin 1000–10000 (6500
  neutral), tension −1..1 (> 0 bulges up). Wallpaper PNG/JPEG/WebP ≤ 32 MB;
  spans midnight; overlap → latest start wins. Desktop night light conflicts.
