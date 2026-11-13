# KC9K Android

Companion to the desktop KC9K. v1: at home (GPS geofence) the phone is in **Nokia mode**
(calls and texts only): everything is blocked except the default dialer and SMS app, Android
Settings included, and Wi-Fi is off. Target GrapheneOS / Pixel, Android 16.
Kotlin, minSdk 29, no third-party libraries, no Google Play Services.

## Architecture (`app/src/main/java/org/kc9k/android/`)
- `Core.kt`: interfaces `Trigger` (decides when a policy is on), `Policy` (a block:
  blacklist/whitelist; `*.*`, `role:dialer`, `role:sms`, package names), `Enforcer`.
  `Policies.NOKIA` is the only block in v1 (it names `com.android.settings` besides `*.*`).
  `Plan.suspend` is what GuardService suspends (pure, tested).
- `GeofenceTrigger.kt`: LocationManager (GPS + network). Entering is immediate, leaving
  takes 10 min outside the radius (accuracy circle fully outside), no fix for 15 min = home.
- `Options.kt`: every protection is an `Option` (on/off + time lock; on is free, off needs
  unlocked + away; date/time stays on while any lock runs). USB debugging block is off by
  default and always ignored in debug builds.
- `DeviceOwner.kt`: `AdminReceiver`; `DeviceOwner.applyBaseline()` (applies the options,
  permission grants); `DeviceOwnerEnforcer` (suspends all
  launchable packages minus whitelist, essentials (not Settings) and default launcher; Wi-Fi
  off + DISALLOW_CONFIG_WIFI; what Android refuses to suspend is kept in `notSuspended` for
  the status line). `liftEverything()`/`resume()` (all builds, for now): `Store.lifted` makes
  every option count as off, the rules empty and Nokia mode off, and clears the locks; Device
  Owner stays. Debug builds keep adb and offer `releaseForDebug()`.
- `GuardService.kt`: location foreground service, started on (locked) boot and by a
  15-min AlarmManager watchdog; re-applies the policy every minute.
- `Store.kt`: device-protected SharedPreferences; `atHome` defaults to true (fail closed);
  `nokiaMode` = home set && at home && not lifted, the one test for "loosening refused".
- `SettingsActivity.kt`: home, options + locks, website lists (+ export/import), updater,
  lift; redrawn on every resume; loosening never in Nokia mode.
- `Updater.kt`: installs a picked APK via PackageInstaller (silent as DO; same key only),
  allowed in Nokia mode too.
- `Picker.kt`: the system file picker (DocumentsUI) is let through Nokia mode for at most
  5 min while KC9K opens it (update, list files, saving the .xpi); `Picker.done` +
  `GuardService.resync` pause it again on the answer.
- List files (`ListFile` in `Web.kt`, `WebSettings.importList`): the desktop's format (one
  entry per line, `#` comments; export carries the `# kc9k cluster, looks at: url` header),
  so `kc9k list/cluster export|import` files go both ways. Import adds, skips non-addresses;
  into Exceptions only when adding exceptions is allowed.
- IronFox (`ExtensionGuard.kt`, `RulesServer.kt`, `Web.kt`, `WebSettings.kt`, `Locks.kt`,
  `ChallengeActivity.kt`): the desktop extension (`internal/webext/files`, packed into the
  assets by the Gradle task `packExtension`) runs in IronFox; `RulesServer` serves `/rules`
  on 127.0.0.1:47613 in the daemon's `enforce.Rules` format (one list `phone/Blocked` with
  its exceptions, `safesearch`, `web_only`) and `/kc9k.xpi`. A check-in counts when fresh
  (15 s), not older than the bundled version, with private tabs allowed. `ExtensionGuard.step`
  pauses (suspends) IronFox after 60 s in front without one, for 2 min; foreground from usage
  events (`Foreground`), no usage access = paused. `GuardService` unions the home targets and
  the paused IronFox into one `Enforcer.sync`. Browser settings and the website block take
  `SettingLock`s (time, challenge, password, restart; strengthen only, as the desktop's
  `strengthenSettingLock`); `WebSettings` refuses loosening while locked or at home.

Later: more Triggers (Wi-Fi BSSID places, schedules), the rest of the website lists (titles,
text, actions, several blocks), other browsers, sync with desktop `kc9k project export` JSON,
loosening delays; guard (or remove) "Lift everything" once the app is trusted.

## Build
`cd android && ./gradlew assembleDebug testDebugUnitTest` (needs the Android SDK in
`local.properties` or `ANDROID_HOME`, JDK 17+). Then `node ci/android_rules_test.js` checks the
served rules with `match.js`; `node ci/android_background_test.js` runs `background.js` as in
IronFox (no windows/idle API). No emulator in CI: nothing here has run on a phone yet.

## Provisioning
See README.md: install, then `adb shell dpm set-device-owner org.kc9k.android/.AdminReceiver`
(no accounts on the device). Any build: "Lift everything (keeps Device Owner)". Debug builds
also: "DEBUG: lift everything and remove Device Owner" or
`adb shell dpm remove-active-admin org.kc9k.android/.AdminReceiver`.
