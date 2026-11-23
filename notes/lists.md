# Website lists, phrase sets, import, presets, recording, drafts, blocks

- Covers: `db/weblists.go`, `listjoin.go`, `listcase.go`, `combinations.go`, `exceptions.go`, `phrases.go`, `clusters.go`, `presets.go`, `templates.go`, `recording.go`, `words.go`, `testmode.go`, `pagekeywords.go`, `enforce/recording.go`, `words.go`, `add.go`, `cli/weblists.go`, `lists.go`, `listtest.go`, `keywords.go`, `phrases.go`, `phraseexport.go`, `entryfile.go`, `clusters.go`, `presets.go`, `templates.go`, `record.go`, `draft.go`, `block.go`, `blockextra.go`
- SPEC: §3.1, §3.2, §3.12, §4.2, §4.3, §4.4, R-LIST-1–6, R-PHRASE-1–10
- Connects to: [extension](extension.md) (matching), [actions](actions.md), [locks](locks.md) (gating), [gui](gui.md)

## Blocks

- New block = blacklist `Blocked` + exception list `Exceptions` (`file://*`;
  spares no website); `Rendered terms on page` made on first `add-term`.
  `--allowlist` = `*.*` in Blocked. `block action B A on|off` sets all
  blacklists. `add-pattern`, `remove-site`, `remove-entry` = Blocked
  shortcuts. `block import` reads pre-list exports. Remove block: inactive
  and unlocked.
- List/apps/titles files: one entry per line, `#` comments (`block
  export-list|import-list B apps|titles|blocked|exceptions`).

## Website lists (`db/weblists.go`, `cli/weblists.go`, `kc9k list`)

- Per block: blacklists and exception lists, scope url/title/text (≥ 1 for
  a blacklist), no types (§3.2). Entry is a site/URL entry if
  `blocker.IsAddressEntry` (domain, path, or pattern with `*`/`/`), else a
  rendered term; validity independent of scope (`prepareEntries`). With
  url: `WebList.Addresses` → `url` (and layers), `URLTerms` → `url_terms`
  matched as words (`kc9kUrlWords`); title/text get every entry.
- Only blacklists with url and `block` feed addresses to
  hosts/firewall/policy (`enforce.Inputs`), and not for blocks with
  title/text/url-term exceptions.
- To the extension as `lists` rules (`enforce.BuildRules`;
  `kc9kListActions`: plain words whole, `*` inside words; a list waits if
  its exceptions need unread page parts). Per action: threshold (distinct
  entries), hit count, rate (`list rate`, % of page words, pages ≥ 200
  words). Lists/phrase sets may have no action (do nothing).
- Strengthening free; else gated (`EditList*` by id). Shortcuts (`block
  add`, `add-exception`, `add-term`/`add-keyword`, `threshold`, `action`)
  use default lists Blocked / Exceptions / Rendered terms on page.
  Case-insensitive names.
- Migrations on every open: `migrateJoinLists` (`db/listjoin.go`) joins
  identical split pairs `<name>` + `<name> terms` (reads only the pair's
  entries, `readJoinEntries`); `migrateCombinations` turns old
  `(a, b) (c):2` blacklist/cluster combinations into phrase sets
  (leftovers use `kc9kCombination`); `refuseCombinations` refuses new ones.
  Old `type` columns/fields ignored; `--terms` no-op alias.
- GUI create dialog: name + scope boxes (`list_dialog`).

## Phrase sets (`db/phrases.go`, `cli/phrases.go`, `kc9k phrase-set`)

`db.Phrase` JSON, typed `[a, b] [c]? :N` (`ParsePhrase`, `String`);
`Check` refuses one-ended bridges (mirrored `kc9kPhraseCheck`). Rules with
negative IDs (`phraseRules`), matched by `kc9kPhraseMatcher`. `HasPhrases`
makes a block non-empty. Names unique case-insensitively; ≤ `db.MaxWordSets`
(100) word sets. GUI: `PhraseSetCard` (`ListCard` keyed by −id) with
in-card `PhraseEditor`. Files: `cli/phraseexport.go`. Phrase presets:
`list-preset copy` → phrase set, `save` from one; no recording.

## Import (`cli/entryfile.go`, §4.3)

`readEntries` sorts lines into phrases, sites (hosts files, adblock `||x^`),
rendered terms, unusable; `list import`, `block import-list`, `phrase-set
import`, `list-preset import` take what fits and name the rest
(`noteLeftOut`). `AddListEntries`/`RemoveListEntries` write in one
transaction (per-entry commits are far too slow). Hidden `kc9k list read
FILE` (JSON) lets the GUI's single Import route a file
(`import_into_phrase_set`, `choose_phrase_set`).

## Huge lists (principle 9)

Entries indexed by list. `GetBlock`/`GetActiveBlocks` use `DB.entries`
(`entryCache`) while `web_lists.entries_version` matches; triggers set a
new random version on any entry change (`entryVersionTriggers`), so
changes by any program are re-read.

## Presets (§3.2, §4.4; `db/presets.go`, `cli/presets.go`)

- Code/DB names: block presets = "template", list/phrase set presets =
  "cluster" (`db/clusters.go`, `cli/clusters.go`; outside blocks, never
  enforced). User-facing (§3.14, §4.12): `block-preset`, `list-preset`
  (also `phrase-set-preset`), `block create --preset`, `record start
  --preset`; old names are cobra aliases / `flagAliases`
  (`cli/templates.go`). `TestPresetCommandNames`. Export header `# kc9k
  list preset, looks at:` (old headers still read).
- `list-preset copy` via `listAdd` (gated); copies independent;
  `--exceptions`, `--as N`, `--into L`.
- Table `presets`: kind `lists` (`db.ListsPreset`), `automation`
  (`AutomationSpec`), `schedule`, `hotkey`. `kc9k preset
  save|use|rename|delete` (`--kind`, `--mode`/`--schedule`); use passes the
  list and automation gates (`useListsPreset`, `changeAutomation`). GUI
  page "Presets" (= `clusters`).

## Recording (`db/recording.go`, `enforce/recording.go`, `cli/record.go`)

- Links: `rules.recording` → extension POSTs `/visit`; daemon buffers;
  `record stop` adds via `listAdd`/`clusterAdd`. Domains without `www.`,
  pages and frames, private windows and blocked pages included; ≤ 2000
  sites; one at a time; refused `stop` keeps it.
- Words (`--words`, `db/words.go`, `enforce/words.go`): `rules.words`
  record|baseline → `kc9kPageWords` → `/words`; `RecordedWords` ranks by
  `Specificity`; `--cut N`; `record baseline`.

## Drafts (`cli/draft.go`, §4.3: an opened block is always a draft)

- `block draft B` (`makeDraft`) = `DuplicateBlock` → "B (draft)"
  (`db.SettingDraftOf`) + social media settings, trigger actions, lists'
  test mode; base kept (`draft.base.<draft>`). Never runs
  (`db.ActivateBlock` refuses, autostart skips).
- `saveDraft` applies only base → draft changes onto the block as it is now
  (entries added meanwhile stay), each through its gate
  (`listAdd`/`listWhere`/…, `saveDraftPhraseSets`, `saveDraftTriggers`),
  naming refusals. A phrase gone from the draft paired with a new one that
  `Widens` it is one free `ReplacePhrase` (R-WEAK-5, as `phrase-set
  change`; A18, `TestDraftPhraseChangeWidens`); list entries have no change
  command, so an edited entry stays remove + add.
- Breaks & allowance and Bypass prevention commands (`draftQueued`) on a
  draft are kept, not run (`queueOnDraft` → `keepOnDraft`,
  `draft.queue.<draft>`, last of each kind; gui-state `draft_kept`);
  `saveDraft` runs each through its gate (`runQueued`). Refused on a draft
  (`refuseDraftEdit`): break, allowance session, automation, affected users.
- GUI: `Window.run` sends `DRAFT_EDITS` for the opened block to its draft
  (`to_draft`, made at first edit), `DRAFT_READS` read it; `self.selected`
  stays the block; bar "Unsaved edits" with Save edits / Discard; leaving
  asks (`ask_unsaved`); kept commands labelled "Not saved yet".

## Test mode (`db/testmode.go`)

`list test B [L] on|off`: lists go out `test: true`; extension skips them
in `kc9kListMatches`, shows `kc9kTestReport` corner panel (found entries,
counts, distinct count, per-action threshold state). Address-only lists
keep working.
