# Handoff: overnight session 2026-11-13 → 14

  modified: 2026-11-13T23:29:59.776Z
---

Overnight session 2026-11-13 → 14 (owner asleep, asked for commits + pushes along the way). Last pushed commit: a45f205. All Go and matcher tests passed; devmaster build installed on the laptop.

**Done** (details in ROADMAP.md §0.1 and git log):
- Strict mode's own lock; `kc9k no-weakening`; `kc9k password locks on|off`; `kc9k dns`; `kc9k tamper clock` (real clock by default, old installs migrated to on); tamper ignores upgrades/self-reported crashes.
- Lock model: unlock only lifts locks (block stays on); delay lock counts from unlock (`delay_until`); re-lock (`start --relock`); schedule lock with user lead (`kc9k schedule lock B on --lead D`) replaces built-in 2 h freeze and 24 h queue (old scheduled blocks migrated to 2 h lead); freeze schedules have `--lock-lead`; timer lock >24 h needs `--yes`; text lock words/custom (`--lock-words`, `--lock-text`). Schedule lock counts in AnyLock/BlocksLocked. A block stopped inside its window stays off until the window ends.
- Speed: kc9kIndex (URL lookup by domain), kc9kWordIndex (one pass for terms), content.js reads only added text in idle time, firewall = nftables table `inet kc9k` with sets (iptables fallback), parallel cached DNS.
- Lists are address or term (CheckWhere); mixed lists split once into `<name>` + `<name> terms` (the owner's `test` block was split). Test mode per list. `file://*` default exception (also fixed file://* sparing all sites). Project backup/share. Monk block system-owned. R-WARN-1/2 notices. Words sweep (`bypass`, `allowance`, `break reward`, `break text`, `list scope`).

**Half-done: phrase sets.** Matcher (`kc9kPhraseMatcher`, `kc9kPhraseCheck` in match.js, tested in ci/match_test.js), DB (`internal/db/phrases.go`, `ParsePhrase` notation `[a, b] [c]? :N`), rules feed (negative list IDs, `phraseRules`, `findActiveList`), extension integration — done. `internal/cli/phrases.go` (`kc9k phrase-set …`) was committed but is UNTESTED, and `"phrase-set": {areaBlocks}` is NOT yet in `commandAreas` in internal/cli/access.go. Next: add that, write CLI tests, then GUI, export/import, clusters/categories, migrate word combinations → phrases, update README/ROADMAP.

**Not started:** active vs enabled separation (§3.3), layered locks in user order (R-LOCK-6/7), search lists, re-lock on setting locks, hardware/internet clock check.

**Browser tests:** browser_test 20/20, actions_test 15/15; extras_test's "another site inside a proxy address is not" fails only because the owner's locked `test` block blocks example.com.

**Open questions for the owner:** should a backup include running locks, passwords, hotkeys, statistics? How to check the clock against hardware/internet time?

**How to apply:** read this first next session, then `git fetch` and ROADMAP.md. 
