# Website lists, phrase sets, import, presets, recording, drafts, blocks

- Covers: `db/weblists.go`, `listjoin.go`, `listcase.go`, `combinations.go`, `exceptions.go`, `phrases.go`, `clusters.go`, `presets.go`, `templates.go`, `recording.go`, `words.go`, `testmode.go`, `pagekeywords.go`, `enforce/recording.go`, `words.go`, `add.go`, `cli/weblists.go`, `lists.go`, `listtest.go`, `keywords.go`, `phrases.go`, `phraseexport.go`, `entryfile.go`, `clusters.go`, `presets.go`, `templates.go`, `record.go`, `draft.go`, `block.go`, `blockextra.go`
- SPEC: §3.1, §3.2, §3.12, §4.2, §4.3, §4.4, R-LIST-1–6, R-PHRASE-1–10
- Connects to: [extension](extension.md) (matching), [actions](actions.md), [locks](locks.md) (gating), [gui](gui.md)

## Code

- **Website lists** (`db/weblists.go`, `cli/weblists.go`, `kc9k list`): per
  block, blacklists and exception lists with scope (url/title/text), no
  types (§3.2): an entry is a site/URL entry if `blocker.IsAddressEntry`
  (domain, path, or pattern with `*`/`/`, e.g. `*hot*girl*`), else a
  rendered term; validity doesn't depend on scope (`prepareEntries`).
  With url: `WebList.Addresses` → `url` (and layers), `URLTerms` →
  `url_terms`, matched as words (`kc9kUrlWords`); title/text get every
  entry. Only blacklists with url and `block` feed their addresses to
  hosts/firewall/policy (`enforce.Inputs`), and not for blocks with
  title/text/url-term exceptions. Old `web_lists.type`/`clusters.type`
  columns may remain; nothing reads them. `migrateJoinLists`
  (`db/listjoin.go`, every open) joins split pairs `<name>` (url) +
  `<name> terms` when identical (entries, actions, no test mode, no
  queued edit/recording); clusters too. `--terms` is a no-op alias; old
  `type` fields in block/project files are ignored. GUI create dialog:
  name plus scope boxes (`list_dialog`, `ci/gui_lists_smoke.py`).
  Everything goes to the extension as `lists` rules
  (`enforce.BuildRules`; `kc9kListActions` in `match.js`: plain words
  whole, `*` inside words; a list waits if its exceptions need unread page
  parts). Per action: threshold (distinct entries), hit count and rate
  (`list rate`, percent of the page's words, pages ≥ 200 words).
  Strengthening free; else gated (`EditList*` by id). Shortcuts (`block add`,
  `add-exception`, `add-term` (alias `add-keyword`), `threshold`, `action`)
  use default lists Blocked / Exceptions / Rendered terms on page
  (`migrateWebLists`; named Page words until `migrateWordsListName`).
  User-facing words follow SPEC §3.14 (`rendered term`, `window title
  entry`, `text lock`, `allowance`, `weakening`). Old combinations
  `(a, b) (c):2`: `migrateCombinations` (`db/combinations.go`, every open
  and after imports) turns blacklist/cluster ones (rendered terms only)
  into phrase sets; `refuseCombinations` refuses new typed ones; leftovers
  (one box, or in exception lists) use `kc9kCombination`.
  Case-insensitive lookup.
- **Phrase sets** (`db/phrases.go`, `cli/phrases.go`, `kc9k phrase-set`):
  `db.Phrase` JSON, typed `[a, b] [c]? :N` (`ParsePhrase`, `String`);
  `Check` refuses one-ended bridges (mirrored `kc9kPhraseCheck`). Rules
  with negative IDs (`phraseRules`), matched by `kc9kPhraseMatcher`.
  `HasPhrases` makes a block non-empty. Names unique case-insensitively.
  GUI: `PhraseSetCard` (a `ListCard` keyed by −id) with the in-card
  `PhraseEditor` (double-click loads a phrase); word sets bounded only by
  `db.MaxWordSets` (100). Files:
  `cli/phraseexport.go`; `db.AddPhraseSet`. Phrase clusters (`phrases`
  set): `cluster copy` → phrase set, `cluster save` from one; no recording.
- **Import files** (`cli/entryfile.go`, §4.3): `readEntries` sorts lines
  into phrases, sites (hosts files, adblock `||x^` read as sites), rendered
  terms, unusable; `list import`, `block import-list`, `phrase-set import`,
  `cluster import` take what fits and name the rest (`noteLeftOut`).
  `AddListEntries`/`RemoveListEntries` write in one transaction (one
  commit per entry: 5,000 took 215 s on the desktop's disk; 100,000 now
  10 s). Hidden `kc9k list read FILE` (JSON) lets the GUI's one Import route a
  file (`import_into_phrase_set`, `choose_phrase_set`;
  `ci/gui_import_smoke.py`).
- **Huge lists** (principle 9): entries indexed by list
  (`web_list_entries_list`). `GetBlock`/`GetActiveBlocks` take a list's
  entries from `DB.entries` (`entryCache`) while its
  `web_lists.entries_version` is the one cached: triggers give a list a
  new random version on any entry insert, change or removal and when it
  is made (`entryVersionTriggers`, `migrateEntryVersions`), so a change by
  any program is read again. The join at open (`listjoin.go`) reads only
  the entries of a name pair (`readJoinEntries`): reading all, deduplicated
  one by one, held every command over a minute at 100,000.
- **List and phrase set presets** (clusters in the code: `db/clusters.go`,
  `cli/clusters.go`): lists outside blocks,
  never enforced. `cluster copy` goes through `listAdd` (gating applies).
- **Presets** (§3.2 Preset, §4.4; owner 2026-11-17; `db/presets.go`,
  `cli/presets.go`, GUI page "Presets" = page `clusters`): block presets
  are templates, list and phrase set presets clusters; table `presets`
  (kind `lists` = `db.ListsPreset`, `automation` = `AutomationSpec`,
  `schedule` = windows, `hotkey`) for the rest. `kc9k preset save|use|
  rename|delete` (`--kind`, `--mode`/`--schedule`), use goes through the
  list and automation gates (`useListsPreset`, `changeAutomation`).
  gui-state `presets` (`guiPreset`, `describePreset`); GUI `save_preset`,
  `use_preset`, `add_from_preset`, `update_presets`.
- **Link recording** (`db/recording.go`, `enforce/recording.go`,
  `cli/record.go`): `rules.recording` → extension POSTs `/visit`; daemon
  buffers; `record stop` adds via `listAdd`/`clusterAdd`. Word recording
  (`--words`, `db/words.go`, `enforce/words.go`): `rules.words`
  record|baseline → content.js `kc9kPageWords` → `/words`;
  `RecordedWords` ranks by `Specificity`; `--cut N`; `record baseline`.
- **Drafts** (`cli/draft.go`, §4.3; owner 2026-11-17: an opened block is
  always a draft): `block draft B` (`makeDraft`) = `DuplicateBlock` into
  "B (draft)" (setting `db.SettingDraftOf`) plus its social media
  settings, trigger actions and lists' test mode, and keeps the block as it
  was (`draftContent` JSON, setting `draft.base.<draft>`); never runs
  (`db.ActivateBlock` refuses, autostart skips). `saveDraft` applies only
  what changed in the draft since (base → draft) onto the block as it is
  now (an entry added meanwhile stays) via `listAdd`/`listWhere`/
  `listAction`…, `saveDraftPhraseSets`, `saveDraftTriggers` (each through
  its gate) and names refusals; commands for anything else refuse a draft
  (`draftRefused`, `refuseDraftEdit` in the root PreRun). GUI: no Edit as
  draft; `Window.run` sends the commands of `DRAFT_EDITS` for the block
  opened on the Blocks page to its draft (`to_draft`, made at the first
  edit, `start_draft`), `DRAFT_READS` read it; `self.selected` stays the
  block; Websites and Apps show the draft (`show_block`: `content`), the
  bar says Unsaved edits with Save edits / Discard, leaving asks
  (`leave_block`, `ask_unsaved`), the card shows "Unsaved edits"; list
  cards keep their widget by name when the draft takes over.
- **Test mode** (`db/testmode.go`): lists go out `test: true`; extension
  shows `kc9kTestReport`, skips them in `kc9kListMatches`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Blocks**: new block = blacklist `Blocked` + exception list `Exceptions`
  (`file://*`; spares no website); `Rendered terms on page` made on first
  `add-term`.
  `--allowlist` = `*.*` in Blocked. `block action B A on|off` sets all
  blacklists. A blacklist keeps ≥ 1 scope; lists and phrase sets may have no action (they then do nothing). Pre-list blocks
  (sites, patterns, exceptions, keywords, allowlist) were migrated into the
  three lists; `block import` reads pre-list exports. `add-pattern`,
  `remove-site`, `remove-entry` are Blocked shortcuts. List/apps/titles
  files: one entry per line, `#` comments (`block export-list|import-list
  B apps|titles|blocked|exceptions`). Remove block: inactive and unlocked.
- **List presets** (`cluster`): copies are independent. `--exceptions`, `--as N`,
  `--into L`. Export = plain entry file with a scope header (`list import`
  reads it).
- **Recording**: domains without `www.`, pages and frames, private windows
  and blocked pages included; ≤ 2000 sites; one recording at a time; a
  refused `stop` keeps the recording.
- **Test mode**: `list test B [L] on|off`; corner panel shows found entries,
  counts, distinct count and per-action threshold state; address-only lists
  keep working.
