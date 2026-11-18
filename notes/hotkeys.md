# Hotkeys and notifications

- Covers: `blocker/hotkeys.go`, `db/hotkeys.go`, `cli/hotkeys.go`, `notifications.go`, `notify.go`, `enforce/notices.go`
- SPEC: §4.13, §4.10 Notifications, R-LOCK-12
- Connects to: [locks](locks.md) (lock flags, confirmation), [actions](actions.md), [device](device.md)

## Code

- `hotkeys.go`: Cinnamon/GNOME dconf keybindings `kc9k<id>` →
  `kc9k hotkey run <id>`: starts in-process with the hotkey's lock flags
  (`hotkeys.lock_args`, start's flags in unlock order, `cli/lockflags.go`:
  `addStartLockFlags`, `parseLockFlags`, `parseLockArgs`; old rows only
  `lock_for`, `Hotkey.Args`); a password lock's hash is taken at `hotkey
  add` (`presetPasswordHash` at the press; never in gui-state, backups
  `hotkey_password_hashes`); settings password asked at `hotkey add`.

## Built behaviour

Not fixed by `SPEC.md`; the code is the truth.

- **Hotkeys**: `ctrl|alt|shift|super+key`, F1–F24 alone; shown as
  "KC9K: …" in keyboard settings; notification on press; any lock `start`
  takes, no upper limit; on an active block a press only adds or extends
  locks, as `start`. Older hotkeys ask at each press. Hotkey actions (owner
  2026-11-17; `db.HotkeyAction`, column `hotkeys.actions`, `Hotkey.Steps`;
  `cli/hotkeys.go`: `addHotkey`, `parseHotkeyAction` for `--do`,
  `runHotkeyAction`, `describeHotkey`): start/stop, mode-on/off,
  mode-run, monk, freeze, close-browser, lock-screen, logout, shutdown,
  custom, in order; one desktop confirmation per press for all its locks.
  GUI: Hotkeys page (`build_hotkeys_page`, `update_hotkeys_page`; area
  settings): Add hotkey… (`add_hotkey_actions`, `ask_hotkey_action`),
  Start a block… (`add_hotkey`: `HotkeyFields`, the `LockPicker`), Save as
  preset….
- **Notifications** (`notify-send`, `enforce/notices.go`, `kc9k notifications`):
  user picks kinds (window start/end, timer lock end, Pomodoro, allowance,
  app closed) and advance minutes (default 5; each advance once); windows
  only of blocks that are on; scheduled freeze always (5 and 1 min).
