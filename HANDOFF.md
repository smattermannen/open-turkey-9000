# Handoff: phrase-sets session (main, laptop), 2026-11-14

Read this, then `git fetch`, SPEC.md and ROADMAP.md.

**Built and merged into main** (tests named after rule IDs in `internal/cli/phrases_test.go`, `internal/db/*_test.go`, `ci/match_test.js`):
- R-PHRASE-1 to 9 in the CLI (`kc9k phrase-set …`), tested; one-ended bridges refused per term (a `-x` needs a `y-` in another word set, a middle link both), in Go (`Phrase.Check`) and JS (`kc9kPhraseCheck`). Phrase set names unique regardless of case. Monk action refused inside monk mode (R-MONK-1).
- GUI (§4.3): `PhraseSetCard` per phrase set on a block's Websites tab, `PhraseDialog` (word sets ticked necessary/optional, length). `ci/gui_phrase_smoke.py` is the GUI smoke test (builds its own binary and scratch DB; needs an X display).
- R-PHRASE-10: phrase sets in block and project files, `phrase-set export/import` (.txt), phrase clusters (`cluster create --phrases`, `save`, `copy`), no recording into them.
- R-PHRASE-6 for term lists: lists and clusters have a `type` (address/term); `list create --terms`; `url_terms` in the rules, matched by `kc9kUrlWords`. Fixed on the way: `migrateListTypes` ran on every open and would have split today's url+page term lists and phrase clusters; it now only splits untyped rows.

**Word combinations → phrases** (`db/combinations.go`), merged after the owner's decision: each box a word set, `:N` dropped, thresholds 1; one-box combinations and those in exception lists stay. Tested on a copy of a DB made by the previous build with `(a, b) (c):2` notation.

**Categories dropped** (the owner's decision): the 14 shipped lists and `block create --template` are gone; SPEC updated.

**Browser tests on the laptop:** devmaster build of main (03104ce) installed 2026-11-14. browser_test 20/20, actions_test 15/15, quota_test 2/2, extras_test 21/22: "another site inside a proxy address is not" fails only because the owner's locked `test` block lists example.com. quota_test used to fail on a second run the same day: `quotatest` stays on until midnight, and LibreWolf's policy file is rewritten only by the daemon (Firefox's at once by the CLI), so a browser started right after `quota start` read the old policy; the test now waits 12 s.

**For other sessions:** R-LOCK-12's acknowledgement for a phrase set's `monk` action (`phrase-set action … monk on`) is in the locks session's list below; `internal/cli/phrases.go` is where it goes. List names differing only in case are still allowed (ROADMAP §0.1). `TestBreakAllowance` failed once under load and passed 5/5 alone (timing).

---

# Handoff: locks session (branch lock-confirm), 2026-11-14

R-LOCK-12 is merged into main and pushed: `confirmLocks` (`internal/cli/confirm.go`) on every lock path, `--yes` for scripts, GUI `confirm_lock`, tests `TestRLOCK12_*` in `internal/cli/confirm_test.go`, GUI smoke test (scratchpad, not in repo: copy its pattern from `ci/gui_phrase_smoke.py`). Hotkey locks are confirmed at `hotkey add` today; `hotkey run` passes `yes`.

**The owner's decisions (logged in SPEC.md §6), not built yet:**
1. Hotkeys: a per-hotkey option "Run without confirmation" (`hotkey add … --no-confirm`, a GUI tick; new column in the hotkeys table). Off (default): `hotkey run` shows a Lock/Cancel desktop dialog before starting the block with its lock (it runs as root through sudo: show the dialog as the session user, like `blocker.NotifyDesktop` reaches the session; python3 GTK is there since the GUI needs it); on: confirm only at `hotkey add`, as now. Existing hotkeys: treat as off.
2. Monk mode started automatically asks nothing when it starts, so setting up a bypass prevention step with `monk` (`bypass set`, GUI `ask_nepsis`) and a list's or phrase set's `monk` action (`list action … monk on`, `phrase-set action … monk`, GUI action ticks) must need an acknowledgement: a flag (e.g. `--no-confirm-ok`) and a GUI tick "I know this will not ask for confirmation". Refused without it. Tests `TestRLOCK12_*` for each.
3. `kc9k monk` itself (manual start) should ask through `confirmLocks`: coordinate with the monk session (branch monk), which is rewriting monk mode.

**Open question:** hotkeys added before R-LOCK-12 never had their lock confirmed; with option 1 they default to asking at each press, which settles it.
