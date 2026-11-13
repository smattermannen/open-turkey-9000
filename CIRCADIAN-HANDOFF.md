# Circadian rhythm — handoff for the next session

Temporary file on branch `circadian`. Delete it once the feature is merged
(CLAUDE.md allows no other status files; the owner asked for this one).

Worktree: `/home/tjena/kc9k-circadian` (branch `circadian`, pushed). Another
session works on `main` in `/home/tjena/open-turkey-9000` at the same time:
don't touch that checkout. Fetch `origin/main` and merge before pushing.

## The owner's decisions (2026-11-14)

- **Automation clips, like FL Studio.** One 24-hour clip each for
  *backlight* (hardware, %), *software brightness* (gamma, %) and *colour
  temperature* (kelvin). Right-click on the clip adds a point (right-click
  on a point deletes it), left-drag moves a point, left-click on the curve
  and drag changes its tension (slope). Fully adjustable.
- **One set of clips for every day** (no weekday/weekend variants).
- **No locks at all** on circadian settings. `SPEC.md` (glossary and §4.10)
  says "Lockable": change it to match, since the owner said so.
- **Never override the user.** KC9K never puts a level back. If the user
  changes a level by hand, that clip pauses **until its next point** (or a
  Resume button in the GUI).
- **Wallpapers:** the user picks an image and sets its time interval. No
  fixed morning/afternoon/evening/night periods.
- **Dark mode = system dark mode**, on or off, like a phone: "Use system"
  (KC9K leaves it alone), always light, always dark, or "Light mode between
  HH:MM and HH:MM".
- **Dark pages** (spec §4.10): follow dark mode, with exempt sites.
- **Testing:** never install, never run live hardware tests on the laptop
  (another session installs there). Unit tests with `KC9K_DB`, the GUI smoke
  test, and Xvfb are fine.
- Use the new words: "weaken", not "loosen"; "timer lock", not "time lock".

## Done (commit 7b57435)

- `internal/circadian/`, pure logic with tests:
  - `circadian.go`: `Config`, `Lane`, `Point{Minute, Value, Tension}`,
    `ValueAt` (wraps past midnight), `Shape` (curve `x^p`; the middle of the
    curve sits at `0.5 + 0.49·tension` of the way; positive tension always
    bulges up), `NextPointAfter`, `WallpaperAt`, `DarkAt`,
    `ParsePoints`/`FormatPoints` (`HH:MM=VALUE~TENSION`), `ParseClock`,
    `Clock`.
  - `presets.go`: presets natural, early-bird, night-owl, gentle and
    candlelight. `WhitePoint(kelvin)` uses Tanner Helland's fit normalised
    so 6500 K is white; `Ramp(size, kelvin, brightness)`.
  - `themes.go`: `DarkTheme`/`LightTheme` (Mint-Y-Aqua ↔ Mint-Y-Dark-Aqua,
    Adwaita ↔ Adwaita-dark).
- `internal/blocker/`:
  - `backlight.go`: `Backlights()` (prefers firmware, then platform, then
    raw), `Raw`/`SetRaw`, `PercentToRaw` (never 0). External screens through
    `ddcutil` (`ExternalScreens`, cached for 10 min; `Brightness`,
    `SetBrightness`).
  - `gamma.go`: RandR over the cgo-free X11 client (`randrOpcode`, `crtcs`,
    `gammaSize`, `gamma`, `setGamma`). `ReadGamma(want)` returns
    Ours/Identity/Other/Unknown; an all-zero ramp counts as Identity.
    `SetGamma(nil)` restores plain ramps. Tested on Xvfb; the test keeps one
    connection open, because an X server resets when its last client leaves.
  - `desktopprefs.go`: gsettings run as each desktop user: `SetWallpaper`
    (Cinnamon/GNOME/MATE), `SetDarkMode` (`org.x.apps.portal` and GNOME
    `color-scheme`, plus the gtk and cinnamon theme variants),
    `NightLightOn`, `TurnOffNightLight`.

## To do

1. **Engine** `internal/enforce/circadian.go`, run from its **own goroutine
   in `internal/daemon/daemon.go`** every 5 s, not inside `Tick` (to avoid
   conflicts with the other session).
   - Settings keys, all JSON in the existing settings table, so no schema
     change: `circadian` (Config), `circadian.paused` ({lane or "gamma":
     unix}), `circadian.preview` ({lane, until}), `circadian.applied`
     ({wallpaper key, dark key}), `circadian.resume` (marker), and
     `circadian.status` (written only when it changes).
   - Backlight: read the raw level every tick. If it differs from what KC9K
     last wrote by more than 1 % → pause until the next point. While paused,
     if it returns to KC9K's last value → resume (Cinnamon's idle dimming).
     When a pause ends → apply. Check external screens every 30 s the same way.
   - Gamma (software brightness × kelvin, one ramp): Identity → apply;
     Ours → apply when the target changes; Other → pause until the earliest
     next point of the two clips. Right after the daemon starts, treat Other
     as ours. With both clips off, restore Identity once.
   - Wallpaper and dark mode: apply only when the active interval or state
     key changes (respects manual changes). Record the key only if at least
     one user was reached; retry at most once a minute.
   - Hardware behind replaceable vars for tests; nothing applied when
     `KC9K_DB` is set.
2. **CLI** `internal/cli/circadian.go`: `kc9k circadian [show --json] |
   on|off | lane <l> [on|off] [--points "..."] | point add|remove | preset
   <name> | preview <lane> <value> [--end] | resume | wallpaper add <img>
   HH:MM HH:MM | remove N | dark system|light|dark|light-between A B |
   dark-pages on|off | exempt add|remove <site> | nightlight off`. Store
   wallpapers in `<db dir>/wallpapers` (0644). Read the image as the calling
   user; `readAsCaller` caps reads at 4 MiB, so wallpapers need a larger
   limit. Add `"circadian": {areaSettings}` to `commandAreas` in `access.go`.
3. **Extension:** add `Rules.Dark {on, follow_system, exempt}` in
   `enforce/browser.go`; add it to `page` in `background.js` (around line
   650); in `content.js`, `render()` ORs dark pages into `on("dark")`
   (follow_system uses `matchMedia('(prefers-color-scheme: dark)')`).
   `content.js` has uncommitted changes in the other session's checkout:
   keep this edit small.
4. **GUI:** a new "Circadian Rhythm" page in `gui/kc9k-gui.py` (guarded by
   the settings area), reading `kc9k circadian show --json`:
   - a master switch, the current values, a preset picker;
   - three clip lanes (a `Gtk.DrawingArea` each) with a sky-gradient time
     axis, a "now" line, the kelvin curve filled in its own colour, hover
     tooltips, snapping to 5 min (Shift: 1 min), double-click a point to
     type exact values, and a live preview on screen while dragging (call
     `preview` in a thread at most every 300 ms, without `self.run`, which
     makes the window insensitive);
   - wallpaper rows with thumbnails and time intervals;
   - dark mode radio buttons and the dark pages exempt list;
   - warnings: no backlight, no ddcutil, Cinnamon night light on (with a
     button to turn it off), paused clips (with Resume).
   Use the same curve formula as `circadian.Shape`.
5. **Docs:** `SPEC.md` (clips; not lockable; never overrides), `ROADMAP.md`
   2.1 and the status table, `README.md` usage, `CLAUDE.md` architecture.
   Optionally include circadian settings in `kc9k project export`.
6. **Push:** fetch and merge `origin/main`, run `CGO_ENABLED=0 go build
   ./...` and `go test ./...` as an ordinary user, then push `circadian`.
