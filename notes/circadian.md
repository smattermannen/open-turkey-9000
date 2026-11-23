# Circadian rhythm and wallpapers

- Covers: `internal/circadian/`, `enforce/circadian.go`, `wallpapers.go`, `blocker/backlight.go`, `gamma.go`, `desktop*.go`, `sleephook.go`, `cli/circadian.go`, `wallpapers.go`, `projectcircadian.go`, `projectwallpapers.go`
- SPEC: §4.10 Circadian rhythm / Wallpaper default, R-MODE-5
- Connects to: [automation](automation.md) (modes, places), [extension](extension.md) (dark pages), [gui](gui.md)

## Circadian (`internal/circadian/`, `enforce/circadian.go`, `cli/circadian.go`)

- `kc9k circadian` (`on|off`, `preset [N]`, `lane L on --points
  "07:00=6500 19:00=6500~-0.4"`, `point add L HH:MM V --tension T`,
  `resume [L]`, `wallpaper add F A B | time N A B | remove N`, `dark
  system|light|dark|light-between A B`, `dark-pages`, `exempt`,
  `nightlight off`). Software 10–100 %, kelvin 1000–10000 (6500 neutral),
  tension −1..1 (> 0 bulges up). Wallpaper PNG/JPEG/WebP ≤ 32 MB; may span
  midnight; overlap → latest start wins. Desktop night light conflicts.
- JSON setting `circadian` (`circadian.Config`: three clips of
  `Point{Minute, Value, Tension}`, wallpapers, dark mode, dark pages);
  `.paused`, `.preview`, `.applied`, `.resume`, `.status`.
- `RunCircadian` goroutine every 5 s, never in `Tick`; `ddcutil` and night
  light every 30 s. Foreign levels pause a clip (`PauseBacklight`,
  `PauseExternal`, `PauseGamma`; brightness and kelvin share one gamma
  ramp), except right after start. Panels (`circadianBacklights`) looked
  up each step only while the backlight clip is on.
- Wallpaper/dark mode applied once per key change (`WallpaperKey`,
  `DarkKey`), also into signed-out users' dconf (`blocker.Reach`); who got
  it in `circadian.applied`; later sign-in without it gets it at once. The
  wallpaper given is `pickWallpaper`'s (below).
- `PreviewCircadian` drives hardware; engine takes over the gamma ramp
  written last (`lastGammaPreview`), never mixed with an ended preview.
  `circadian play` (`PlayCircadian`: 24 h as previews in `--seconds`,
  `ReadDesktopLook`/`RestoreDesktopLook` restore the desktop; ≤ 1 hardware
  set/s plus last frame).
- `circadian.Shape` = GTK `clip_shape` = C# `Clips.Shape` (same samples in
  `TestShapeMirrors`, `ci/gui_circadian_smoke.py`, `CoreTests.cs`): middle
  m = 0.5 + 0.49·tension at the bend handle; below x^p, above 1 − (1 −
  x)^p, so ±t mirror, no corner (A14). Drag: 80 px = tension 1.
- Dark pages, switch "Show dark web pages while dark mode is on"
  (`dark-pages`): `rules.dark` (`CircadianDarkRule`) → `darkPage` (light
  pages turned over) and the browser's own dark mode (`applyColorScheme`,
  [extension](extension.md)): sites with a dark design show it. LibreWolf
  tells sites the real colour scheme only with `DarkPagesOn` →
  `blocker.SyncLibreWolfDarkPages` ([browsers](browsers.md)), from its next
  start. Chrome, Chromium, Edge: no policy or extension setting; they
  follow the desktop's dark mode (their default "Device"). Exempt sites
  aren't turned over but may show their own dark design (one setting per
  browser). Popup "Exempt from dark pages" → `darkExempt` → daemon
  `/dark-exempt` (refused behind an app/settings access password).
  `ci/circadian_test.py` (box, real daemon); `ci/dark_browser_test.py`
  (real LibreWolf, no KC9K: stands in for the daemon on 47613; passes with
  LibreWolf 160 headless).
- Hardware vars `circadian*` (`fakeCircadian`); `KC9K_DB` → `circadianDry`.
  Wallpapers in `<db dir>/wallpapers` (0644). Windows: PowerShell per
  look/read/set ([windows-layers](windows-layers.md)).
- GUI: `CircadianPage`, `ClipEditor`, "Play the day" with playhead,
  `WallpaperTimeline` (5-min steps, Shift minutes). No Circadian section in
  Settings.

## Wallpapers of modes (R-MODE-5; `enforce/wallpapers.go`, `cli/wallpapers.go`, `blocker/sleephook.go`)

- Settings `mode.wallpapers.<mode id>` (`[]circadian.Wallpaper`; Frozen
  Chicken id 0) and `wallpaper` (`WallpaperSettings`: `CircadianFirst`,
  `Default` latest|circadian|custom, `Custom` kept when another default is
  chosen).
- `lookAtModes` (enabled modes; one switched by places while `placesHere`
  is unknown → `unsure`; enabled last wins) → `pickWallpaper` (`keep`:
  nothing given, nothing forgotten). Engine holds a place known in the
  last minute (`placeGrace`), forgets it on wake (`ForgetSeenNetworks`),
  steps every second while `lookingForPlace`.
- `syncSleepHook` writes `/usr/lib/systemd/system-sleep/kc9k` while a
  place-switched mode has wallpapers and default isn't latest; it runs
  hidden `kc9k wallpaper before-sleep` (access-exempt) → after wake the
  engine gives afresh (`beforeSleepGiven`).
- Unused pictures removed (`WallpaperInUse`, `forgetWallpaper`).
- CLI `kc9k mode wallpaper M [add|time|remove]`, `kc9k wallpaper
  [priority|default]`; gui-state `modes[].wallpapers`, `wallpaper`. GUI
  `WallpaperEditor`, `mode_override`, `override_modes`,
  `wallpaper_default`.
- Project files: modes' `custom`/`wallpapers`, `wallpaper`; import lists
  custom actions (`projectCustoms`), needs `--yes` (`confirmCustoms`).
