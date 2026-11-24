# KC9K Android

Owner, 2026-11-24: Android work resumed; the app gets the Windows app's
functions (`ROADMAP.md` §3.1). No setting for Nokia mode on its own: it
becomes an ordinary block.

Blocks, places and schedules as on the computer; the 0.2 app's Nokia mode
is migrated into the block "Nokia mode" (All apps and Settings paused but
the default phone and SMS apps, Wi-Fi off, on at the place "Home", with a
schedule lock). Device Owner. GrapheneOS/Pixel, Android 16, Kotlin, minSdk
29, no third-party libraries, no Play Services.

## Code (`app/src/main/java/org/kc9k/android/`)

- `Model.kt` (pure): `Window` (day 0 = Monday, minutes, 5-minute steps),
  `Automation` (default enabled/disabled, own windows, schedules, places,
  both/either), `Block` (sites + own exceptions, phrase sets, social media settings, apps `*`/roles/packages +
  own app exceptions, Wi-Fi off, `lock: SettingLock`, `scheduleLock` =
  locked while its automation has it on, pause, `mode`), `Mode` (name +
  one automation for its blocks), `Place` (area, or Wi-Fi `networks` with
  `needed` in range; decided state), `WifiPlace.step` (there while `need`
  of its networks are in a scan; a scan with no networks changes nothing),
  `Schedule`, `Model` (+ `noWeakening`, refused log, `settingsPassword`,
  `lockNeedsPassword`, `access` hashes by area; `Block.viewPassword`; `automation(b)` = the
  mode's while in one, its own kept unused, R-MODE-1).
  `Phrases.kt` (pure): `Phrase` (the desktop's grammar `[a, b] [c]? :N`,
  bridges, `check`/`key`/`widens`, held to Go by the shared
  `internal/db/testdata/phrase_cases.json`, `PhrasesTest` and Go's
  `TestSharedPhraseCases`), `PhraseSet` (scope url/title/text, threshold,
  test mode; action: block), `Social.PRESETS` (as `db/social.go`, test
  compares). `Enforce` is the single source of truth: `state`/`enabled`/`locked`,
  `plan` (union of enabled blocks; `WebRule` per block for the extension), `suspend` (each block's apps minus its
  own exceptions, minus essentials).
- `Commands.kt` (pure, R-ANDROID-3): every change to blocks, places,
  schedules; weakening a locked block refused and logged (`Model.refused`);
  `whySettingsLocked` = no weakening while any block is locked (gates
  `Options` and `WebSettings`). `Weakening.strengthens` judges automation
  changes. Modes: add/rename/delete, `putInMode`/`takeOut`,
  `setModeAutomation` (refused as a whole naming the locked blocks it
  weakens, R-MODE-1/2), `switchMode` (off leaves locked blocks on),
  `lockMode` (every block at once), `modeLocked` (all blocks locked,
  R-MODE-3); schedule/place checks see blocks through their mode. Tests:
  `ModelTest.kt` (`CommandsTest.weakeningRefusedAndLogged` and
  `ModesTest.weakeningThroughAModeRefused`,
  `PhraseSetCommandsTest.weakeningRefusedAndLogged` list every weakening
  command). Phrase sets: adding/widening phrases, wider scope, lower
  threshold, test off strengthen; social media setting on strengthens.
- Passwords (SPEC §3.11; `Session.kt`): `Commands(settingsUnlocked =
  Session.settingsUnlocked)`; with a settings password set every weakening
  path (`whyLocked`, so also modes, schedules, places, unlock,
  `whySettingsLocked` for `Options`/`WebSettings`) stops with
  `Outcome.needsPassword` / `Commands.NEEDS_PASSWORD`, unlogged; the GUI
  asks and reruns (`run`, `result(err, retry)`). New password free;
  change/remove weakens. `lockNeedsPassword`: `lock`/`lockMode` need it
  too. Access passwords per `Access.AREAS` (app, overview, blocks, stats,
  more): `render` shows `accessPage` until typed; changing one needs the
  settings password. Block password hides the card and the block page
  until typed. `Session` (typed passwords, areas, revealed blocks) ends on
  screen off (`GuardService` receiver), app closed, "Lock KC9K now".
  Tests: `PasswordsTest.kt`. Forgotten settings password: SPEC §7
  (Android recovery) still open; debug builds `releaseForDebug`.
- `Db.kt`: `Codec` (JSON via `Json.kt`, own reader/writer since org.json
  is stubbed in unit tests), `Codec.migrate` (0.2 prefs to blocks), `Db`:
  one file `kc9k.json` in device-protected storage, atomic write, cached.
- `Area.kt` (pure, `AreaTest`): enter at once on a fix inside; leave after
  10 min with the accuracy circle fully outside; no fix changes nothing; no
  fix yet = not inside. `PlaceWatcher.kt` feeds every place GPS, network
  and fused fixes (last known at start; registers again each minute while
  no fix comes; skips Wi-Fi places); `MapMath.distance` (haversine).
  `WifiWatcher` (same file, run by `GuardService`): scan results every
  30 s, a scan asked every 2 min; `inRange` for the GUI. Commands
  `setWifiPlace` (new, or networks in range added), `setPlaceNeeded`,
  `removeNetwork`: on a place a locked block uses, wider (more networks,
  fewer needed) is allowed only where the place turns the block on,
  narrower only where it lifts it; else refused and logged.
- `Options.kt`: protections as `Option` (on free; off needs it unlocked and
  no weakening; trusted time: date/time and time zone automatic and
  unchangeable while anything is locked, `Store.anythingLocked`, re-applied
  by `GuardService` when that changes): block safe
  mode, USB debugging (default off; ignored in debug builds), users,
  location, date/time, factory reset, uninstall.
- `DeviceOwner.kt`: `AdminReceiver`; `applyBaseline()`;
  `DeviceOwnerEnforcer` resolves entries (`*` = launchable apps, roles =
  system default holders), `never()` = essentials + default launcher,
  `sync` suspends exactly the set (unpauses what it paused before,
  `Store.suspended`), Wi-Fi off + DISALLOW_CONFIG_WIFI; unsuspendable ones
  in `notSuspended`. `liftEverything()`/`resume()` (temporary):
  `Store.lifted` enforces nothing, clears setting/protection locks, stays
  Device Owner. Debug: `releaseForDebug()`, adb kept.
- `GuardService.kt`: location foreground service; boot (locked) and 15-min
  AlarmManager watchdog; every 5 s `Enforce.plan` (resolved apps cached a
  minute) plus paused IronFox into one `Enforcer.sync`.
- `Store.kt`: device-protected prefs for protections, browser settings and
  their locks, `lifted`; the 0.2 keys (home, `web.*`) only for migration.
- `MainActivity.kt` + `Ui.kt` (design layer: cards, pills): bottom bar
  Overview · Blocks · Statistics (not built) · More (Schedules, Places,
  Settings, Refused changes). Blocks: modes above blocks (`modeCard`,
  switch = its blocks; mode page `sub = "mode:<name>"`, tabs Blocks ·
  Automation · Lock, rename, delete). Block page: card, tabs Websites · Apps ·
  Automation (in a mode: open it / take out; Move to mode…; pause, delete) · Lock (timer, challenge, password, restart,
  schedule lock; confirmed first, R-LOCK-12). Every change runs `Commands`.
  One `ScrollView` kept across renders: same page keeps its position.
- Place map (`MapPickerActivity.kt` with `EXTRA_NAME`, `MapView.kt`,
  `MapMath.kt`): OSM tiles (User-Agent `KC9K-Android/<version>`), tap sets
  the centre, radius 50–2000 m, saved by `Commands.setPlace`.
- `Updater.kt`: PackageInstaller (silent as DO, same key), always allowed.
- `Picker.kt`: DocumentsUI let through ≤ 5 min while KC9K opens it;
  `Picker.done` + `GuardService.resync` re-pause.
- Lists (`ListFile` in `Web.kt`, `Commands.importSites`): the computer's
  list format. Import adds, skips non-addresses; into exceptions only when
  the block isn't locked.
- IronFox (`ExtensionGuard.kt`, `RulesServer.kt`, `Web.kt`,
  `WebSettings.kt`, `Locks.kt`, `ChallengeActivity.kt`): desktop extension
  from `internal/webext/files`, packed by Gradle `packExtension`.
  `RulesServer` on 127.0.0.1:47613 serves `/rules` (`enforce.Rules` format:
  one list per enabled block with sites, one rule per usable phrase set
  (negative id, `phrases`, `phrase_scope`, `test`), each with that block's
  exceptions; `social` = union of the enabled blocks' settings,
  `safesearch` without `safesearch_engines` = every engine, `web_only`) and
  `/kc9k.xpi`. Check-in valid if fresh (15 s), not older than bundled,
  private tabs allowed. `ExtensionGuard.step` suspends IronFox after 60 s
  in front without one (5 s once checked in), 2 min doubling per pause to
  1 h, reset by 10 min unbroken; foreground from usage events; no usage
  access = paused. SafeSearch: Google, Bing, DDG, Brave, Yahoo, SearXNG
  strict (POST too), YouTube Restricted; Startpage, Mojeek, Marginalia,
  Ecosia, Yandex blocked. `SettingLock`s (time, challenge 10–5000 chars at
  98%, password, restart; strengthen only). Challenge screen: only
  characters typed one by one count (`Challenge.typedOneByOne`: paste,
  swipe, suggestions undone, R-LOCK-5).

Not built yet (`ROADMAP.md` §3.1): mode wallpapers and custom actions,
drag into a mode, layered locks and the other lock
kinds, breaks/allowance/statistics, VPN layer, monk mode, Frozen Chicken,
presets, actions, circadian, guided setup, accessibility layer, Compose.
"Lift everything" waits for SPEC §7 (Android recovery).

## Build and provision

```bash
cd android && ./gradlew assembleDebug testDebugUnitTest
```

Needs Android SDK (`local.properties` or `ANDROID_HOME`), JDK 17+. Cloud
container: tools in `/opt/android-sdk`, `sdkmanager "platforms;android-36"
"build-tools;35.0.0" "platform-tools"`, then `ANDROID_HOME=/opt/android-sdk
./gradlew --no-daemon testDebugUnitTest assembleDebug`. `node
ci/android_rules_test.js` (served rules with `match.js`: sites,
phrase sets on page text, social), `node
ci/android_background_test.js` (`background.js` as in IronFox). Never run
on a phone yet.

Provision (no accounts): `adb install -t <apk>`; `adb shell dpm
set-device-owner org.kc9k.android/.AdminReceiver`. IronFox: add-on installs
allowed, `xpinstall.signatures.required=false`. Usage access: `adb shell
appops set org.kc9k.android GET_USAGE_STATS allow`. Debug removal: `adb
shell dpm remove-active-admin org.kc9k.android/.AdminReceiver`. Release
updates need the same signing key.
