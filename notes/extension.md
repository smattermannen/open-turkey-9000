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
  top page by its title or text (`mayActByText`), it stays at opacity 0
  (`hidePage`) until judged and settled (`SETTLE_MS` after DOMContentLoaded
  or an address change seen in the MutationObserver), never over
  `HIDE_CAP_MS`. Don't hide with `visibility`: `innerText` skips it.
- `no-images` pictures drawn as CSS content: `content.js` marks elements
  (`data-kc9k-noimg` before/after/self, idle-time `scanPseudoImages`) and
  both sheets empty them; shadow roots get an adopted sheet. Page script
  harness (Node + Playwright, cloud only): `node ci/content_harness_test.js`.
- **Workarounds**: `kc9kUnwrap` reads addresses in query/path; `checkMirror`
  (content.js) reports canonical addresses/front-ends; `/hit` with `via`
  logs `EventWorkaround`.
- **Social** (`db/social.go`, `cli/social.go`): `block_social`,
  `BlockDetail.Social`; `rules.social` → `webext/files/social.js`
  (`KC9K_SOCIAL`). Playlist = preset `youtube:playlist=<id>` plus
  exceptions `youtube.com/playlist?list=<id>`, `youtube.com/watch`; GUI:
  `PlaylistBox` in the Exceptions card (`ci/gui_playlist_smoke.py`);
  `checkPlaylistVideo`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Matching**: plain term = whole word (`porn` ∉ "pornhub", `18+` ∉
  "1118+"); `*` inside words; case ignored. Address/title match fires every
  action; text needs threshold (distinct) and hits (total), both default 1;
  once met the page stays acted on until left. Typed phrase `[a, b] [c]? :N`
  (`?` optional, third set on; `:N` length). Exceptions by title/text leave
  that block's sites to the extension only (page flashes before blocking);
  address exceptions remove their domain from hosts/firewall.
- **Social presets**: YouTube (no Shorts, no home feed → Subscriptions, no
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
