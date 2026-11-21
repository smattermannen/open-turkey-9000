# Browser policies, extension guard, supported browsers, Chrome

- Covers: `blocker/browser*.go`, `extguard.go`, `browserdetect.go`, `supported.go`, `remotecontrol*.go`, `librewolf*.go`, `chromesetup.go`, `webext/chrome.go`, `db/settinglocks.go`, `cli/browser*.go`, `cli/tamper.go`, `enforce/browser.go`, `enforce/tamper.go`
- SPEC: §3.13, §4.10 Tamper protection / Browsers / Browser settings, R-LOCK-9
- Connects to: [extension](extension.md), [locks](locks.md) (setting locks), [layers-net](layers-net.md), [device](device.md) (strict mode)

## Code

- State the Chrome service worker must keep across restarts goes in
  `storage.session` (e.g. `kc9kDownloadPages`).
- LibreWolf's policy file is rewritten only by the daemon (Firefox's at once
  by the CLI): wait ~12 s before starting a browser after a change.
- Browser policies (Firefox, Chrome, Chromium, Brave, Edge, Opera,
  Vivaldi): blocklist, allowlist, wildcards, DoH off.
  `file://` entries (`BlockFiles`, `FilePatterns`) only with tamper
  `file-urls` on (`enforce.SettingFileURLs`), minus the block's file exceptions.
  URL lists hold at most 1,000 entries (the browsers ignore the rest):
  `FirefoxPolicyLimit` (WebsiteFilter Block/Exceptions, Firefox and
  LibreWolf), `ChromiumPolicyLimit` (URLBlocklist/URLAllowlist). Past it,
  patterns, file URLs and sites past `MaxHostsDomains` first (`capSet`;
  the allowlist's `*://*/*` always); exceptions cut (blocks more). Sets
  built once per policy (`setsOf`, also `Empty`): the daemon checks drift
  every cycle.
- `extguard.go`: extension mandatory while `enforce.ExtensionRequired` and
  a supported browser is installed; closes browsers whose extension hasn't
  fetched `/rules` with `X-KC9K-Extension` within 60 s (traced via
  `/proc/net/tcp`), at once if remote-controlled (`remotecontrol.go`) or
  started before the last protecting settings change
  (`noteSettingsChange`). Browsers known by name or engine
  (`browserdetect.go`: libxul.so or icudtl.dat/V8 snapshot mapped by a
  user-owned exe, minus Electron). An exe on FUSE (an AppImage's: shown as
  root's, closed to root) counts as user-owned (`userPlacedExe`). Decision
  in `closeReason`. With block unsupported browsers on, runs always and
  closes at once every browser whose exe isn't a root-owned package file
  of `SupportedBrowsers` (`supported.go`: LibreWolf, Chrome, Chromium; `InstallBrowser`, fakes
  `statFile`/`runInstall`). LibreWolf: `block-devtools` policy;
  five prefs locked in librewolf.cfg, user overrides replaced by a cleaned
  copy (`librewolfprefs.go`).
- **Browser setting locks** (`db/settinglocks.go`, `cli/browser.go`): per
  option (time, challenge, password, restart); `enforce.AnyLock` counts
  them. The same code serves the tamper setting block unsupported browsers
  (`db.UnsupportedBrowsers`, stored `tamper.unsupported-browsers`,
  `enforce.IsLockableSetting`/`SettingKey`, `cli/tamper.go`), formerly the
  browser setting `force-librewolf` (`migrateForceLibreWolf` moves state
  and lock row; hidden alias in `kc9k browser`; old project files read).
  `GuardsBlocks`: it and `block-devtools`.
- **Chrome** (`webext/chrome.go`, `blocker/chromesetup.go`): `PackChrome`
  builds MV3 manifest + service worker (`chrome-worker.js`), signs CRX3 with
  a per-computer key (`webext.ChromeKeyPath`); daemon serves
  `/chrome/update.xml` and `.crx`; `kc9k-setup.json` force-installs, disables
  incognito/guest/dev mode. `compat.js` aliases `browser`; keepalive;
  `rules.extension_version` triggers updates. Chrome for Testing reads
  `/etc/opt/chrome_for_testing/policies`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Extension guard**: the GUI's "isn't running" dialog (`extensionMissing`)
  only with no supported browser, or one running past the grace without a
  check-in (`browsersWithoutExtension`, `runningSupportedBrowsers`).
  Required while an active block has a non-empty
  blacklist, web-only/force-search is on, or anything is locked (R-LOCK-9);
  unlocked app-only blocks leave browsers alone. With block unsupported browsers on, always: other browsers
  (Firefox too) close at once, supported ones after 60 s without a check-in.
  Nothing closed without LibreWolf, Chrome or Chromium installed (`status`
  says so). `kc9k browsers` lists them with per-browser check-in
  (`extension.last_checkin.<id>`); `browsers install librewolf|chromium|chrome`
  (extrepo; Mint's chromium .deb; Google's repo with key in
  `/etc/apt/keyrings/google-chrome.asc`, amd64 only). Firefox needs signed extensions; Brave/Edge/Opera/Vivaldi not set
  up. Installed engine-based apps (Thunderbird, Spotify, Steam, Electron)
  aren't browsers. Chrome flags `--disable-extensions[-except]`,
  `--remote-debugging-pipe` and Firefox `--marionette`/remote-debugging
  close at once. After a protecting change, older browser windows close a
  minute later. LibreWolf locked prefs: `extensions.webextensions.restrictedDomains`,
  two `extensions.quarantinedDomains*`, `devtools.chrome.enabled`,
  `devtools.debugger.remote-enabled`; overrides copy at
  `/var/lib/kc9k/librewolf/<user>.cfg` keeps only `pref`/`defaultPref`/
  `lockPref` lines. Translate proxy (`*.translate.goog`) and archives
  (`web.archive.org`, `archive.ph`) blocked as the site. `web-only` uses
  Google `udm=14`.
- **Chrome**: policy `/etc/opt/chrome/policies/managed/kc9k-setup.json`
  (Chromium `/etc/chromium/…`); key `/var/lib/kc9k/chrome-extension.pem`
  (root only, ID per computer); pinned; devtools never on the extension;
  `force-search` sets Google web results as default engine. A restarted
  service worker may let the first page through the extension (URL policy
  still blocks plain sites) until the page reports in.
