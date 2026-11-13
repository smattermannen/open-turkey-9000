# KC9K Android

At home (GPS geofence) the phone is in **Nokia mode**: calls and texts only. Every app except
the default Phone and SMS apps is paused, Android Settings included, and Wi-Fi is off. Away,
everything returns. Runs as Device Owner.

The status line says which mode the phone is in and where the last location fix put it
(distance from home, accuracy). No fix for 15 minutes counts as home.

## Build

```bash
cd android
gradle wrapper
./gradlew assembleDebug      # or assembleRelease
```

## Provisioning (adb)

Device Owner can only be set on a device with no accounts (on GrapheneOS that is normally
already the case; otherwise remove accounts or provision after a reset).

1. Enable Developer options → USB debugging, connect the phone.
2. Install: `adb install -t app/build/outputs/apk/debug/app-debug.apk`
3. Make it Device Owner:
   `adb shell dpm set-device-owner org.kc9k.android/.AdminReceiver`
4. Open KC9K, enter home latitude, longitude and radius (≥ 50 m) while away, Save.

Protections (in the app, each optional and lockable for N hours): block safe mode, USB
debugging (off by default, ignored in debug builds), adding/switching users, location
changes, date/time changes, factory reset, uninstalling KC9K. Turning one on is always
possible; turning one off needs it unlocked and the phone away from home.

## IronFox: websites, SafeSearch, web results

KC9K runs the desktop's own browser extension in IronFox (`org.ironfoxoss.ironfox`, also the
nightly). The app packs it from `internal/webext/files` and serves it its rules on
`127.0.0.1:47613`, the address the desktop daemon uses, so pages are judged by the same
`match.js` as on Linux: `reddit.com` (with subdomains), `reddit.com/r/funny`, `*.xxx`,
`*casino*`, `*.*`, exceptions that spare what they match, and sites shown through Google
Translate, web archives or `?url=` proxies.

In KC9K:
- **Browser**: *Enforce SafeSearch* (Google, Bing, DuckDuckGo, Brave Search, Yahoo and
  SearXNG get their strict setting, IronFox's POST searches included; YouTube gets Restricted
  Mode; Startpage, Mojeek, Marginalia, Ecosia and Yandex are blocked while it is on, since
  their setting can't be forced from the address) and *Force web results* (Google web results
  only, as `kc9k browser web-only` on the desktop).
- **Websites**: a Blocked list and its Exceptions. Adding to Blocked and removing an
  exception are always allowed; the rest only while the block is unlocked (and not in Nokia
  mode). **Export…** saves a list as a text file and **Import into…** adds a file's entries to
  it, in the desktop's format (one entry per line, `#` for comments): a phone export goes into
  `kc9k list import <block> <list> <file>` or `kc9k cluster import <name> <file>`, and a
  `kc9k list export` or `kc9k cluster export` file imports on the phone. Lines that aren't
  addresses are skipped and counted; importing into Exceptions follows the same rule as
  adding exceptions.
- **Lock…** on each of the three: any mix of a time lock (hours or until HH:MM), the typing
  challenge (10–5000 characters, 98% accuracy, no pasting, no screenshots), a password, and
  until the phone restarts, as `kc9k browser … on --for/--until/--lock/--password-stdin/--restart`.
  Locks only get stronger. **Unlock…** asks for the password and/or runs the challenge; time and
  restart locks are only waited out. Time locks need "Block date/time changes" on.

One-time setup:
1. Grant KC9K usage access (KC9K → *Grant usage access…*, or
   `adb shell appops set org.kc9k.android GET_USAGE_STATS allow`). Without it KC9K can't
   tell when IronFox is open, so IronFox stays paused whenever the extension is needed.
2. In IronFox: Settings → IronFox → IronFox settings → Security → *Allow installation of
   add-ons*; then `about:config` → `xpinstall.signatures.required` = `false` (IronFox is
   built without the signing requirement, so this switch works).
3. In KC9K: *Install the extension in IronFox* (opens `http://127.0.0.1:47613/kc9k.xpi`), or
   *Save the extension file…* and in IronFox use *Install extension from file*. Tick
   *Allow in private browsing*.

The extension is mandatory, as on the desktop: while websites are blocked or SafeSearch or web
results are on, IronFox may be open for 60 seconds without a current, private-tabs-enabled
extension checking in; then it is paused (suspended) for 2 minutes. KC9K's status line says
what is missing. After updating KC9K, install the extension again when it says so.

Not covered yet: other browsers (Vanadium, other apps' web views), and the rest of the
desktop's list features (page titles and text, other actions, several blocks).

## Updating

Debug: `adb install -r -t app-debug.apk` (same PC, same debug keystore).
Release: copy the signed APK to the phone, then KC9K → **Install update (APK)…** (also in
Nokia mode: the file picker is let through while it is open). It must be signed with the same
key, so keep the keystore and its password safe.

## Lift everything (for now, any build)

**Lift everything (keeps Device Owner)…** unpauses every app, allows Wi-Fi, lifts every
protection, clears every lock and gives the extension nothing to enforce, while KC9K stays
Device Owner (so updates still install silently). Settings, lists and home are kept;
**Resume protection** turns them back on. It works at home and under locks: it is the way out
while the app is being tested, and will be guarded or removed later.

## Removing Device Owner (debug builds only)

Debug builds never disable adb. Either:
- in the app tap **DEBUG: lift everything and remove Device Owner**, or
- `adb shell dpm remove-active-admin org.kc9k.android/.AdminReceiver`
  (works for test-only/debuggable apps), then `adb uninstall org.kc9k.android`.

Release builds have no built-in way out (only the in-app debug path above, which they lack).
