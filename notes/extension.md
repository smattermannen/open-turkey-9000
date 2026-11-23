# Browser extension: matching, page behaviour, social, toolbar

- Covers: `internal/webext/files/` (`match.js`, `content.js`, `background.js`, `social.js`, `search.js`, `popup.js`), `webext/webext.go`, `db/social.go`, `cli/social.go`
- SPEC: §2 principle 9, §3.1, §3.9, R-LIST-4–6, R-PHRASE-1–9, R-PAGE-1–2, R-EVENT-1
- Connects to: [lists](lists.md) (what it matches), [actions](actions.md) (what it does), [browsers](browsers.md), [allowance](allowance.md) (usage, toolbar), [circadian](circadian.md) (dark pages)

## Matching (`match.js`)

- Plain term = whole word (`porn` ∉ "pornhub"); `*` inside words; case
  ignored. Address/title match fires every action; text needs threshold
  (distinct) and hits (total), default 1; once met the page stays acted on
  until left. Phrase `[a, b] [c]? :N` (`?` optional, `:N` length).
- Title/text exceptions leave that block's sites to the extension only
  (page flashes before blocking); address exceptions remove their domain
  from hosts/firewall.
- Folding (R-LIST-5, `kc9kFold`): Latin-looking capitals first
  (`KC9K_LOOKALIKES_UPPER`), NFKD, lower case, marks and format characters
  dropped, then `KC9K_LOOKALIKES`. Terms folded the same; long text folded
  in pieces (`kc9kFoldSteps`).
- Principle 9: judging is resumable (`*Steps` generators, `kc9kDrain`);
  content script runs READ_SLICE_MS at a time (`judgeOn`/`judged`); term
  regexes made at first use (`kc9kKeywordMatcher`), one-word terms via the
  word index; first read waits for idle after DOMContentLoaded
  (`queueScan`).
- Frames judge their own text (`scanFrame`, R-LIST-4) and ask the
  background (`frame-hits` → `frameHit`) for the block page.
- Browser policy stops many pages before the extension sees the request; it
  then learns from the tab every 3 s. Request-dependent features need a
  fallback (`trail` in `background.js`).
- File pages (§3.1): judged only when a rule has `file_pages` (tamper "allow
  file:// blocking"; `enforce.withFilePages`), by `files`/`except.files`
  (`kc9kFileMatch`: decoded path, `*` only at end) and terms.
  `kc9kJudgedAddress` is the one "is this address judged" check. `*.*`
  never matches a file. Chrome sends no request events for files:
  `blockTitledTab` judges address and title.
- Tests: `ci/match_test.js`, `ci/content_harness_test.js`, `ci/background_test.js`.
- Real LibreWolf in the cloud, no KC9K installed (`ci/fake_daemon.py`
  answers on 47613 and refuses to start beside a daemon): LibreWolf from
  `repo.librewolf.net` (apt, key `pubkey.gpg`), `cargo install geckodriver`
  (GitHub downloads are refused), `pip install selenium`; as a normal user
  with `GECKOWEBDRIVER=<dir>`: `ci/dark_browser_test.py`,
  `ci/search_results_librewolf_test.py`.

## Page behaviour (`content.js`, `background.js`)

- No flashing (R-PAGE-1): while a list could act on the top page by title
  or text (`mayActByText`), the root's children stay at opacity 0
  (`hidePage`; not the root: it flashes white; never `visibility`:
  `innerText` skips it) until judged and settled (`SETTLE_MS`), max
  `HIDE_CAP_MS`. Read only once `styled()` (every stylesheet loaded/failed
  or `SHEETS_WAIT_MS`): Firefox paints a page laid out before its
  stylesheets apply. `ci/flash_test.py` "darklate".
- `no-images` CSS-content pictures: elements marked `data-kc9k-noimg`
  (`scanPseudoImages`); shadow roots get an adopted sheet.
- No downloads (§3.9, `downloadSources`, `downloadBlocked`): cancelled
  (finished: file removed) when referrer, address or a source page matched
  no-downloads (`downloadPages`). Chrome gives cross-host files only an
  origin referrer, or none: then that origin's tabs, or the front tabs,
  stand in. Tests `ci/download_test.py`, `ci/download_background_test.js`.
- Looks earned by title/text carry to the tab's next address until read
  (`carryLooks`, ≤ `CARRY_MS`; only hide). Picture/media requests cancelled
  only for the page's own no-images/no-video.
- Frame looks (no-video, no-images, greyscale) also as a user sheet
  (`LOOK_CSS`, `syncLookCss`). Chrome stacks duplicate sheets and
  `removeCSS` removes one copy: each tab counts `cssGiven`; insert new
  before removing old. `ci/looks_background_test.js`.
- `dark`: light pages only (`pageIsLight`); `html` inverted, `DARK_MEDIA`
  inverted back once (nested in a background image: `filter: none`); SVG
  `img` turn with the page.
- Browser's own dark mode (`applyColorScheme`, permission
  `browserSettings`): `rules.dark.on` → Firefox
  `browserSettings.overrideContentColorScheme` "dark", else cleared (the
  browser's own setting); told only on change, again after a refusal.
  Chrome has no such setting (skipped). Test `ci/background_test.js`.
- Tamper protection (§4.10): `inactive_tabs_off` (`closeNewlyBlockedTabs`
  skips background tabs; `onActivated` → `judgeTab`), `embedded_off`
  (sub-frames not blocked by address, `frameHit` silent); absent = on.
  `ci/tamper_tabs_test.js`.
- Workarounds: `kc9kUnwrap` reads addresses in `?url=`, base64, after a
  slash, Wayback with or without a time (searches for an address don't count); `checkMirror` reports
  front-ends (Invidious, Piped, Nitter, Redlib, ProxiTok…); `/hit` with
  `via` logs `EventWorkaround`.

## Search results (R-PAGE-2, `search.js`)

- Results pages known by address (`KC9K_SEARCH_ENGINES`, `kc9kSearchEngine`):
  Google, Bing, DuckDuckGo (and `/html/`; not Lite), Brave Search, Yahoo,
  Startpage, Ecosia, Yandex, Mojeek, SearXNG (hosts as SafeSearch knows
  them). Top frame only.
- A link's target (`kc9kResultTarget`): the engine's redirect (Google
  `/url?q=`, `/amp/s/`; Bing `/ck/a?u=a1…`; DuckDuckGo `/l/?uddg=`, ads'
  `y.js?ad_domain=`; Yahoo `/RU=…/RK=`), else `kc9kUnwrap` (Bing ads'
  `?u=<base64>`, Google ads' `adurl=`, archives), else the link. Links to
  the engine's own sites (`own`) are menus, never results.
- A result leads away when a visit to its target is taken away by address
  alone (`kc9kTakesAway` of `kc9kListMatches({url})`: page or device
  actions, after the block's exceptions). Looks-only lists, and lists whose
  block has title/text exceptions, leave results alone. Ads count.
- Hidden, not deleted (`data-kc9k-removed`, `display: none`; the engine's
  scripts keep working): the largest part around the link, inside the
  results area (`results`, else the body), holding no result kept and no
  search box or navigation (`KC9K_RESULT_GUARD`). Rechecked as the page
  grows: a part holding a kept result later is shown and judged again.
- `content.js`: judged in `load` before the page shows, then by a
  MutationObserver on added links (before they paint); lists changed →
  `forget` and judged afresh.
- Tests: `ci/search_results_test.js` (Chromium, engines' fixtures
  `ci/search_results/` at their real addresses, every frame checked:
  R-PAGE-1), `ci/search_results_librewolf_test.py` (real LibreWolf and
  extension, fixtures through a local proxy, `ci/fake_daemon.py`).

## Social media (`db/social.go`, `cli/social.go`, `social.js`)

- `block_social`; `rules.social` → `KC9K_SOCIAL`. YouTube (no Shorts, home
  feed → Subscriptions, no recommendations, no comments);
  Instagram/Facebook/X (messages only, no Reels/Explore); Reddit (no
  Popular/All, no home feed); TikTok (Following only); LinkedIn (no feed,
  no news). Disabled parts redirect; leftovers hidden. A redirect's `query`
  regex on `?…` stops it when matched.
- Playlist = its address in an exception list (`kc9k social B playlist ID
  on|off`); `db.withPlaylists` adds `youtube:playlist=<id>` and the
  `youtube.com/watch` exception, unstored. Videos need `list=` and must be
  in the shown playlist panel (`checkPlaylistVideo`).

## Toolbar button (`popup.js`)

Time left (allowance, break, schedule within 2 h); red in last 5 min; `!`
when daemon unreachable; popup lists covering blocks, adds the site to
Blocked. Uses its own window's front tab (`popupTab`; Chrome's panel
window isn't focused). `ci/popup_background_test.js`.
