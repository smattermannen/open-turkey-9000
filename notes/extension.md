# Browser extension: matching, page behaviour, social, toolbar

- Covers: `internal/webext/files/` (`match.js`, `content.js`, `background.js`, `social.js`, `popup.js`), `webext/webext.go`, `db/social.go`, `cli/social.go`
- SPEC: §2 principle 9, §3.1, §3.9, R-LIST-4–6, R-PHRASE-1–9, R-PAGE-1, R-EVENT-1
- Connects to: [lists](lists.md) (what it matches), [actions](actions.md) (what it does), [browsers](browsers.md), [allowance](allowance.md) (usage, toolbar), [circadian](circadian.md) (dark pages)

## Code

- Browser policy stops many pages before the extension sees the request; it
  then learns from the tab every 3 s. Request-dependent features need a
  fallback (`trail` in `background.js`).
- Judging is resumable (`kc9kListMatchesSteps` and the other `*Steps`
  generators in `match.js`; `kc9kDrain` runs them to the end): the content
  script runs it READ_SLICE_MS at a time (`judgeOn`/`judged`; principle 9).
  Frames judge their own text (`scanFrame`, R-LIST-4) and ask the
  background (`frame-hits` → `frameHit`) for the block page.
- Folding (R-LIST-5, `kc9kFold`): capitals that look Latin first
  (`KC9K_LOOKALIKES_UPPER`: Greek "Η" is "h", not "η" = "n"), NFKD
  (fullwidth, mathematical, circled letters, ligatures), lower case, marks
  and format characters (zero-width, soft hyphen) dropped, then
  `KC9K_LOOKALIKES` (Cyrillic, Greek, Armenian, small capitals). Terms are
  folded the same way; a long page text is folded in pieces between steps
  (`kc9kFoldSteps`, cut after a blank; principle 9). A term's regular
  expression is made at its first use (`kc9kKeywordMatcher`): one-word
  terms are counted by the word index, and making 1,000 at load was a
  70 ms long task. The first read waits for idle time after
  DOMContentLoaded (`queueScan`): read at once, its first visibility check
  forced the browser's layout of the whole page inside the event.
  `ci/match_test.js`; frames in Chromium
  (`ci/content_harness_test.js`); `frameHit` in `ci/background_test.js`.
- **File pages** (§3.1): judged only when a rule has `file_pages`
  (tamper "allow file:// blocking"; `enforce.withFilePages`), by `files`
  and `except.files` (`kc9kFilePaths`, `kc9kFileMatch`: decoded path,
  `*` only at the end) and the list's terms; `kc9kJudgedAddress` is the
  one check for "is this address judged" (match, content `judgedHere`,
  background `pageHit`, `frameHit`, `blockTitledTab`,
  `closeNewlyBlockedTabs`). `*.*` never matches a file. Chrome sends no
  request events for files: `blockTitledTab` judges their address and
  title, and leaves `reason=title` off when the address alone did it.
- No flashing (R-PAGE-1, `content.js`): while a list could still act on the
  top page by its title or text (`mayActByText`), what its root holds stays
  at opacity 0 (`hidePage`; not the root itself, which takes the canvas and
  flashes white; the page's own background shows) until judged and settled (`SETTLE_MS` after DOMContentLoaded
  or an address change seen in the MutationObserver), never over
  `HIDE_CAP_MS`. Don't hide with `visibility`: `innerText` skips it.
- `no-images` pictures drawn as CSS content: `content.js` marks elements
  (`data-kc9k-noimg` before/after/self, idle-time `scanPseudoImages`) and
  both sheets empty them; shadow roots get an adopted sheet. Page script
  harness (Node + Playwright, cloud only): `node ci/content_harness_test.js`.
- **Tamper protection** (§4.10): rules `inactive_tabs_off` (then
  `closeNewlyBlockedTabs` skips background tabs and `onActivated` runs
  `judgeTab` on the tab switched to) and `embedded_off` (then sub-frame
  requests aren't blocked by address and `frameHit` answers nothing);
  absent = on (Android, older daemons). `node ci/tamper_tabs_test.js`
  (stub browser, background script).
- **Workarounds**: `kc9kUnwrap` reads addresses in query/path; `checkMirror`
  (content.js) reports canonical addresses/front-ends; `/hit` with `via`
  logs `EventWorkaround`.
- **Social** (`db/social.go`, `cli/social.go`): `block_social`,
  `BlockDetail.Social`; `rules.social` → `webext/files/social.js`
  (`KC9K_SOCIAL`). Playlist = only its address in an exception list
  (`youtube.com/playlist?list=<id>`; `kc9k social B playlist ID on|off`
  adds/removes it); `db.withPlaylists` adds `youtube:playlist=<id>` and
  the `youtube.com/watch` exception to the active block, unstored
  (`migratePlaylistExceptions`, `OldPlaylists` for older projects);
  `checkPlaylistVideo` (`ci/gui_playlist_smoke.py`).
  A redirect may take `query` (a regex on `?…`): matching it stops the
  redirect (`instagram:following-only` → `/?variant=following`).

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Matching**: plain term = whole word (`porn` ∉ "pornhub", `18+` ∉
  "1118+"); `*` inside words; case ignored. Address/title match fires every
  action; text needs threshold (distinct) and hits (total), both default 1;
  once met the page stays acted on until left. Typed phrase `[a, b] [c]? :N`
  (`?` optional, third set on; `:N` length). Exceptions by title/text leave
  that block's sites to the extension only (page flashes before blocking);
  address exceptions remove their domain from hosts/firewall.
- **Social media settings**: YouTube (no Shorts, no home feed → Subscriptions, no
  recommendations, no comments); Instagram/Facebook/X (messages only, no
  Reels/Explore); Reddit (no Popular/All, no home feed); TikTok (Following
  only); LinkedIn (no feed, no news). Disabled parts redirect; leftovers
  hidden. Playlist: videos need `list=` and must appear in the shown
  playlist panel.
- **Toolbar button**: time left (allowance, break, schedule within 2 h); red
  in the last 5 min; `!` when the daemon is unreachable; popup lists covering
  blocks and adds the site to Blocked.
- **Workarounds**: proxied addresses in `?url=`, base64, or after a slash;
  searches for an address don't count. Front-ends: Invidious, Piped, Nitter,
  Redlib, ProxiTok….
