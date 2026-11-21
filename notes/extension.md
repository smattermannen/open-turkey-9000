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
- No downloads (§3.9, background `downloadSources`, `downloadBlocked`,
  `downloads` permission): a download is cancelled (finished: its file
  removed) when its referrer, its address or a page it may come from
  matched no-downloads (address, or text hits kept in `downloadPages`).
  Chrome gives a file from another host only the origin as referrer and a
  page may send none: then the tabs of that origin, or with no referrer the
  tabs in front, stand in. Tests: `ci/download_test.py` (LibreWolf),
  `node ci/download_background_test.js` (fake APIs: origin referrer, none,
  `blob:`, a worker restart).
- Looks a page earned from its title or text are carried to the tab's next
  address until that page is read (`carryLooks`, `page-judged`, ≤
  `CARRY_MS`); carried, they only hide (the sheets). Picture and media
  requests are cancelled only for the page's own no-images/no-video (its
  address, or `keywordNoImages` for its own text): a cancelled picture
  stays broken on a next page that doesn't earn the look
  (`ci/extras_test.py`).
- `dark` (and dark pages) in `render`: only light pages (`pageIsLight`);
  `html` inverted, `DARK_MEDIA` (pictures, video, canvas, frames,
  background images) inverted back once; nested inside a background image:
  `filter: none` (else turned over twice; `picture` not listed, its img
  is). SVG `img` turn with the page (logos: a black wordmark stays
  readable). `ci/extras_test.py` counts the turns.
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
