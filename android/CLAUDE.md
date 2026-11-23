# KC9K Android

**On hold until the Linux project is finished (owner).** No Android work until then.

At home (GPS geofence) the phone is in **Nokia mode**: only default dialer
and SMS app; everything else (Settings included) suspended; Wi-Fi off.
Device Owner. GrapheneOS/Pixel, Android 16, Kotlin, minSdk 29, no
third-party libraries, no Play Services.

## Code (`app/src/main/java/org/kc9k/android/`)

- `Core.kt`: `Trigger`, `Policy` (blacklist/whitelist: `*.*`, `role:dialer`,
  `role:sms`, packages), `Enforcer`. `Policies.NOKIA` only (adds
  `com.android.settings`). `Plan.suspend` pure, tested.
- `GeofenceTrigger.kt`: GPS + network. Enter immediate; leave after 10 min
  with accuracy circle fully outside; no fix 15 min = home.
- `Options.kt`: protections as `Option` (on free; off needs unlocked and
  away; date/time stays on while any lock runs): block safe mode, USB
  debugging (default off; ignored in debug builds), users, location,
  date/time, factory reset, uninstall.
- `DeviceOwner.kt`: `AdminReceiver`; `applyBaseline()`;
  `DeviceOwnerEnforcer` suspends launchable packages minus whitelist,
  essentials, default launcher; Wi-Fi off + DISALLOW_CONFIG_WIFI;
  unsuspendable ones in `notSuspended`. `liftEverything()`/`resume()`
  (temporary): `Store.lifted` turns everything off, clears locks, stays
  Device Owner. Debug: `releaseForDebug()`, adb kept.
- `GuardService.kt`: location foreground service; boot (locked) and 15-min
  AlarmManager watchdog; reapplies every minute; unions home targets and
  paused IronFox into one `Enforcer.sync`.
- `Store.kt`: device-protected prefs; `atHome` defaults true;
  `nokiaMode` = home set && at home && !lifted (the "loosening refused" test).
- `SettingsActivity.kt`: home, options/locks, lists, updater, lift; no
  loosening in Nokia mode.
- Home map (`MapPickerActivity.kt`, `MapView.kt`, `MapMath.kt`; refused in
  Nokia mode): OSM tiles (User-Agent `KC9K-Android/<version>`), tap sets
  home, radius 50–2000 m.
- `Updater.kt`: PackageInstaller (silent as DO, same key), allowed in Nokia mode.
- `Picker.kt`: DocumentsUI let through Nokia mode ≤ 5 min while KC9K opens
  it; `Picker.done` + `GuardService.resync` re-pause.
- Lists (`ListFile` in `Web.kt`, `WebSettings.importList`): desktop list
  format. Import adds, skips non-addresses; Exceptions only when adding
  exceptions is allowed.
- IronFox (`ExtensionGuard.kt`, `RulesServer.kt`, `Web.kt`,
  `WebSettings.kt`, `Locks.kt`, `ChallengeActivity.kt`): desktop extension
  from `internal/webext/files`, packed by Gradle `packExtension`.
  `RulesServer` on 127.0.0.1:47613 serves `/rules` (`enforce.Rules` format:
  one list `phone/Blocked` + exceptions, `safesearch` without
  `safesearch_engines` = every engine, `web_only`) and `/kc9k.xpi`.
  Check-in valid if fresh (15 s), not older than bundled, private tabs
  allowed. `ExtensionGuard.step` suspends IronFox after 60 s in front
  without one (5 s once checked in), 2 min doubling per pause to 1 h, reset
  by 10 min unbroken; foreground from usage events; no usage access =
  paused. SafeSearch: Google, Bing, DDG, Brave, Yahoo, SearXNG strict (POST
  too), YouTube Restricted; Startpage, Mojeek, Marginalia, Ecosia, Yandex
  blocked. `SettingLock`s (time, challenge 10–5000 chars at 98%, password,
  restart; strengthen only); `WebSettings` refuses loosening while locked or at home.

Planned (SPEC §8; `ROADMAP.md` §3.1 `N0`–`N8`): every non-computer-only
Linux function, on GrapheneOS and common Android; Compose + Material 3
allowed; Nokia mode becomes an ordinary block; no hotkeys, no Linux sync.
"Lift everything" waits for SPEC §7 (Android recovery).

## Build and provision

```bash
cd android && ./gradlew assembleDebug testDebugUnitTest
```

Needs Android SDK (`local.properties` or `ANDROID_HOME`), JDK 17+. Cloud
container: tools in `/opt/android-sdk`, `sdkmanager "platforms;android-36"
"build-tools;35.0.0" "platform-tools"`, then `ANDROID_HOME=/opt/android-sdk
./gradlew --no-daemon testDebugUnitTest assembleDebug`. `node
ci/android_rules_test.js` (served rules with `match.js`), `node
ci/android_background_test.js` (`background.js` as in IronFox). Never run
on a phone yet.

Provision (no accounts): `adb install -t <apk>`; `adb shell dpm
set-device-owner org.kc9k.android/.AdminReceiver`. IronFox: add-on installs
allowed, `xpinstall.signatures.required=false`. Usage access: `adb shell
appops set org.kc9k.android GET_USAGE_STATS allow`. Debug removal: `adb
shell dpm remove-active-admin org.kc9k.android/.AdminReceiver`. Release
updates need the same signing key.
