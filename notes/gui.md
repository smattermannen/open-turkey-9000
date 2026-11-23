# GUI

- Covers: `gui/kc9k-gui.py`, `gui/kc9k.desktop`, `ci/gui_*.py`, `cli/duration.go`
- SPEC: §4.1–4.13, R-GUI-1–3
- Connects to: every note (each GUI change runs a `kc9k` command); [testing](testing.md) (GUI smoke tests)

## Structure (`gui/kc9k-gui.py`, GTK 3)

- Reads `kc9k gui-state`, `kc9k stats --json`; every change runs a `kc9k`
  command. Menu entry / `kc9k-gui` (`python3-gi`); theme in
  `~/.config/kc9k/gui.json`.
- Blocks page (§4.2): `blocks_stack` "cards" (`update_block_cards`,
  `block_card`: name, switch, Continuous/Automated, `breaks_words`,
  `lock_words`), "detail" (`open_block(name, tab)`; top fixed, only tabs
  scroll in `detail_scroll`), "mode" (`open_mode`, `update_mode_view`).
  Modes above blocks (`mode_card`, drop target `BLOCK_TARGET`; drag a
  card's title, or ⋯ "Move to the mode"); a block in a mode greys
  `block_auto_widgets`. `AutomationEditor` (kind mode/monk/schedule);
  sidebar Schedules (`build_schedules_page`) and Places
  (`build_places_page`).
- Design layer (R-GUI-3, `APP_CSS`): `page_header`, `section` (card;
  `help=` → ? popover), `card`, `pill`/`block_pills`, `flow_row` (into a
  `WrapBox`: laid out like words). `WrapBox` is a `Gtk.Fixed` overriding
  only size vfuncs: never write a container in Python (`do_forall`): GTK
  calls it on a finalized wrapper and spins.
- Inputs (R-GUI-1/2): `DurationField` (typed `2d 4h`, says why when wrong,
  never clamps), `NumberField`, `TimerLockField` (none / for / until
  date), `LockPicker` (every lock kind; `relock_any=True` for blocks,
  modes, hotkeys; setting locks: re-lock only with text/password).
  CLI side `cli/duration.go` (`parseUserDuration` with d/w,
  `humanDuration`, `untilToday`).
- Locks: Start runs `kc9k start` at once (`start_block`, D2); once on, Add
  locks… (`add_locks`). Lock state follows unlock order (`lock_order`,
  R-LOCK-6): cards name the next lock to unlock, pills in that order;
  `waiting_words` words a timer/restart lock behind a non-timer one as
  waiting, as the CLI confirmation does. Extend timer… (`extend_timer`)
  runs `kc9k extend` while `lock_order` has a timer lock.
- `EditableList` + `ResizeGrip` (drag taller, double-click resets); height
  in gui.json `list_heights` per `size_key`, applied live to same-key lists.
- Schedule grid: drag adds, click removes, filled = block window, outlined
  = allow window, one-click presets.
- ClipEditor: right-click add/remove point, drag (Shift = minute), drag
  mid-segment handle to bend (D4), double-click to type.
- Phrase editor (`PhraseSetCard`): Save runs `phrase-set change`, stays off
  until the new state is read; Cancel asks before dropping unsaved work;
  Delete phrase asks, then empties the editor (`remove_loaded`).
