# GUI

- Covers: `gui/kc9k-gui.py`, `ci/gui_*.py`, `cli/duration.go`
- SPEC: §4.1–4.13, R-GUI-1–3
- Connects to: every note (each GUI change runs a `kc9k` command); [testing](testing.md) (GUI smoke tests)

## Code

- **GUI** (`gui/kc9k-gui.py`, GTK 3): reads `kc9k gui-state`,
  `kc9k stats --json`; every change runs a `kc9k` command. Blocks page
  (§4.2): `blocks_stack` "cards" (`update_block_cards`, `block_card`:
  name, switch, Continuous/Automated, `breaks_words`, `lock_words`) and
  "detail" (the block, `open_block(name, tab)`) and "mode" (`open_mode`,
  `update_mode_view`); modes above the blocks (`mode_card`, drop target
  `BLOCK_TARGET`; block cards' title is the drag source, ⋯ "Move to the
  mode"); `AutomationEditor` (kind mode/monk/schedule) runs the same
  commands with the mode's or schedule's name; sidebar pages Schedules
  (`build_schedules_page`, `open_schedule`) and Places
  (`build_places_page`); a block in a mode greys `block_auto_widgets`.
  An opened block's top (← All blocks to the Start/⋯ row) stays put;
  only its tabs scroll (`detail_scroll`).
  `ci/gui_cards_smoke.py`, `ci/gui_modes_smoke.py`. Design layer
  (R-GUI-3, `APP_CSS`): `page_header`, `section` (a card; `help=` puts
  long text behind a ? popover), `card`, `pill`/`block_pills`,
  `flow_row` (a row into a `WrapBox`: laid out like words, natural widths,
  a new line only when the next doesn't fit; GTK 3's FlowBox put one per
  line). `WrapBox` is a `Gtk.Fixed` overriding only size vfuncs: never
  write a container in Python (`do_forall`): GTK calls it on a fresh
  wrapper when a destroyed one is finalized and spins. Inputs (R-GUI-1/2):
  `DurationField` (typed `2d 4h`, says why when wrong, never clamps),
  `NumberField`, `TimerLockField` (none / for / until a calendar date),
  `LockPicker` (every lock kind; `lock_dialog` reads it before closing).
  `EditableList` has a `ResizeGrip` under it (drag down: taller; double-click:
  back), height kept in gui.json `list_heights` under `size_key` (website
  list, phrase set) or the list's title; `ci/gui_list_resize_smoke.py`.
  CLI side: `cli/duration.go` (`parseUserDuration` with d/w,
  `humanDuration`, `untilToday` with dates).

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **GUI**: menu entry / `kc9k-gui` (`python3-gi`); theme in
  `~/.config/kc9k/gui.json`. Schedule grid: drag adds, click removes,
  filled = block window, outlined = allow window, one-click presets.
  ClipEditor: right-click add/remove point, drag (Shift = minute), drag
  curve to bend, double-click to type.
