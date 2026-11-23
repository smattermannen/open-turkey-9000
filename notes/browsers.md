# Browser policies, extension guard, supported browsers, Chrome

- Covers: `cmd/chrome-store-zip`, `webext/store/`, `blocker/browser*.go`, `extguard.go`, `browserdetect.go`, `supported.go`, `remotecontrol*.go`, `librewolf*.go`, `chromesetup.go`, `webext/chrome.go`, `db/settinglocks.go`, `cli/browser*.go`, `cli/tamper.go`, `enforce/browser.go`, `enforce/tamper.go`, `enforce/tampersettings.go`, `blocker/guest.go`
- SPEC: §3.13, §4.10 Tamper protection / Browsers / Browser settings, R-LOCK-9
- Connects to: [extension](extension.md), [locks](locks.md) (setting locks), [layers-net](layers-net.md), [device](device.md) (strict mode)

## Browser policies

- Firefox, LibreWolf, Chrome, Chromium, Brave, Edge, Opera, Vivaldi:
  blocklist, allowlist, wildcards, DoH off. LibreWolf's policy is rewritten
  only by the daemon (Firefox's at once by the CLI): wait ~12 s before
  starting a browser after a change.
- `file://` entries (`BlockFiles`, `FilePatterns`) only with tamper
  `file-urls` on (`enforce.SettingFileURLs`), minus file exceptions;
  Chromium family without trailing `*` (`chromiumFilePatterns`: prefix
  match, `*` literal).
- ≤ 1,000 URL entries (browsers ignore the rest: `FirefoxPolicyLimit`,
  `ChromiumPolicyLimit`). Past it, patterns, file URLs and sites past
  `MaxHostsDomains` go first (`capSet`; allowlist's `*://*/*` always kept);
  exceptions cut (blocks more). `setsOf` built once per policy; daemon
  checks drift every cycle.
- Translate proxy (`*.translate.goog`) and archives (`web.archive.org`,
  `archive.ph`) blocked as the site. `web-only` uses Google `udm=14`.

## Rules server (`enforce.ServeRules`, `RulesAddress`)

Routes in `rulesRoutes`; each answers only its own host name, posts only
with `ExtensionHeader` (`/usage`: `X-KC9K-Usage`), no CORS, no secret
(SPEC §6, agent). `TestRulesServerGuardsEveryRoute` covers every route.

## Extension guard (`extguard.go`)

- Required while an active block has a non-empty blacklist,
  web-only/force-search is on, or anything is locked (R-LOCK-9); tamper
  `extension-stop` off: only while locked (`AnyLock`). Only in accounts
  that need it (`EnforceFor`, `enforce.extensionUsers`, §3.2).
- Closes a browser whose extension hasn't fetched `/rules` with
  `X-KC9K-Extension` within 60 s (traced via `/proc/net/tcp`); at once if
  remote-controlled (`remotecontrol.go`, `remotecontrol_shared.go`: Chrome
  `--disable-extensions[-except]`, `--remote-debugging-pipe`, Firefox
  `--marionette`/remote debugging) or started before the last protecting
  change (`noteSettingsChange`; older windows close a minute later).
  Decision in `closeReason`.
- Browser detection (`browserdetect.go`): by name or engine (libxul.so or
  icudtl.dat/V8 snapshot mapped by a user-owned exe), minus Electron
  (`.asar`, or renderer child with `--app-path=`); judged once 15 s old.
  FUSE exe (AppImage) counts as user-owned (`userPlacedExe`). Engine-based
  apps (Thunderbird, Spotify, Steam) aren't browsers.
- Block unsupported browsers on: guard always runs; closes at once every
  browser whose exe isn't a root-owned package file of `SupportedBrowsers`
  (`supported.go`: LibreWolf, Chrome, Chromium); supported ones after 60 s
  without check-in. Nothing closed without a supported browser installed
  (`status` says so).
- `kc9k browsers` lists them with check-in (`extension.last_checkin.<id>`;
  gui-state `found`: executable present even where it can't get the extension, W30); `browsers install librewolf|chromium|chrome`
  (`InstallBrowser`: extrepo; Mint's chromium .deb; Google's repo, key
  `/etc/apt/keyrings/google-chrome.asc`, amd64 only). Firefox needs signed
  extensions; Brave/Edge/Opera/Vivaldi not set up.
- GUI "isn't running" dialog (`extensionMissing`) only with no supported
  browser or one past the grace without check-in.
- Windows: never counts/closes Windows' own programs, never kills
  session-critical processes (W50, [windows-layers](windows-layers.md)).

## LibreWolf (`librewolf*.go`)

- `block-devtools` policy; locked prefs in librewolf.cfg
  (`extensions.webextensions.restrictedDomains`, two
  `extensions.quarantinedDomains*`, `devtools.chrome.enabled`,
  `devtools.debugger.remote-enabled`); user overrides replaced by a cleaned
  copy `/var/lib/kc9k/librewolf/<user>.cfg` (only `pref`/`defaultPref`/
  `lockPref` lines).
- Dark pages (§4.10, `SyncLibreWolfDarkPages`, `darkPrefs`): own section in
  librewolf.cfg while `enforce.DarkPagesOn` (circadian on and dark pages
  on): `defaultPref` resistFingerprinting off, `fingerprintingProtection`
  on with `+AllTargets,-CSSPrefersColorScheme` (LibreWolf FAQ; RFP tells
  every site light, nothing overrides it). Defaults: a value set by hand
  wins. Not in `LibreWolfLocksVersion`: read at LibreWolf's next start,
  never closes it. Uninstall (`SyncLibreWolfLocks(false)`) removes it.

## Chrome (`webext/chrome.go`, `blocker/chromesetup.go`)

- `PackChrome`: MV3 manifest + service worker (`chrome-worker.js`), CRX3
  signed with per-computer key `/var/lib/kc9k/chrome-extension.pem` (root
  only); daemon serves `/chrome/update.xml` and `.crx`;
  `rules.extension_version` triggers updates. `compat.js` aliases
  `browser`; keepalive.
- `/etc/opt/chrome/policies/managed/kc9k-setup.json` (Chromium
  `/etc/chromium/…`; Chrome for Testing `/etc/opt/chrome_for_testing/policies`):
  force-install, pinned, incognito/guest/dev mode off, no devtools on the
  extension; `force-search` sets Google web results as default.
- Worker state across restarts: `storage.session` (e.g.
  `kc9kDownloadPages`). A restarted worker may let the first page through
  (URL policy still blocks plain sites).
- Chrome Web Store (2.17): `make chrome-store-zip` (`cmd/chrome-store-zip`,
  `PackChromeStoreZip`: `PackChromeZip`'s files dated 2026-01-01, no `key`
  or `update_url`; `TestChromeStoreZip`) → `kc9k-chrome-store-<version>.zip`.
  Listing texts, permission reasons, data use and privacy policy:
  `webext/store/listing.md`, `privacy-policy.md`; small tile and four
  1280×800 screenshots drawn from the real pages by `webext/store/render.js`
  (Playwright). The package loads unpacked in Chromium (worker runs).

## Tamper settings (`enforce/tampersettings.go`, `TamperOn`)

- `extension-stop`, `inactive-tabs`, `embedded` on when unset
  (`tamperDefaultOn`); rest off. `cli/tamper.go`
  `simpleTamper`/`tamperOrder`/`tamperMessages`; gui-state `tamper_*`.
- `guest` → `blocker.SyncGuestOff` (`kc9k-guest.json` beside each
  Chromium-family `kc9k.json`; LightDM drop-in
  `lightdm.conf.d/90-kc9k-no-guest.conf`).
- `inactive-tabs`/`embedded` off → rules `inactive_tabs_off`/`embedded_off`.
  `embedded` off also keeps blacklists' paths/patterns out of browser
  policies (`embeddedOff`; Chrome's URLBlocklist blocks frames too; not
  monk mode's or factory reset protection's lists); whole sites stay.
- `installer` → hidden `prepare-install` ([install](install.md)).

## Browser setting locks (`db/settinglocks.go`, `cli/browser.go`)

- Per option (time, challenge, password, restart); `enforce.AnyLock` counts
  them. Same code serves tamper block unsupported browsers
  (`db.UnsupportedBrowsers`, `tamper.unsupported-browsers`,
  `IsLockableSetting`/`SettingKey`; was browser `force-librewolf`,
  `migrateForceLibreWolf`). `GuardsBlocks`: it and `block-devtools`.
- SafeSearch per engine (§4.10): `safesearch-google|bing|duckduckgo|youtube`
  (`enforce.SafeSearchEngines`, `SafeSearchOn`), each own lock. `kc9k
  browser safesearch on|off` = group (`safeSearchGroup`: lock flags set on
  each engine, one confirmation; off per engine; `unlock safesearch`
  refused). `migrateSafeSearch` copies old `browser.safesearch` + lock to
  all four. Hosts: `blocker.SyncSafeSearch(engines)` pins only those
  engines. Rules `safesearch` + `safesearch_engines`; `kc9kSafeSearch(url,
  form, engines)` (no list = all). Brave, Yahoo, SearXNG and blocked
  engines go with Google/Bing/DuckDuckGo (agent); YouTube-Restrict header
  with YouTube.
