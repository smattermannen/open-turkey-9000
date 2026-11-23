# Hotkeys and notifications

- Covers: `blocker/hotkeys.go`, `db/hotkeys.go`, `cli/hotkeys.go`, `notifications.go`, `notify.go`, `enforce/notices.go`
- SPEC: §4.13, §4.10 Notifications, R-LOCK-12
- Connects to: [locks](locks.md) (lock flags, confirmation), [actions](actions.md), [device](device.md)

## Hotkeys

- Cinnamon/GNOME dconf keybindings `kc9k<id>` (shown "KC9K: …") → `kc9k
  hotkey run <id>`. Keys `ctrl|alt|shift|super+key`, F1–F24 alone.
  Notification on press.
- Starts in-process with the hotkey's lock flags (`hotkeys.lock_args`, in
  unlock order; `addStartLockFlags`, `parseLockFlags`, `parseLockArgs`;
  old rows only `lock_for`). Any lock `start` takes; on an active block a
  press adds locks as `start` does. Password lock hash taken at `hotkey
  add` (`presetPasswordHash`; never in gui-state; backups
  `hotkey_password_hashes`); settings password asked at `hotkey add`.
- Actions (`db.HotkeyAction`, `hotkeys.actions`, `Hotkey.Steps`; `--do`
  via `parseHotkeyAction`, `runHotkeyAction`): start/stop, mode-on/off,
  mode-run, monk, freeze, close-browser, lock-screen, logout, shutdown,
  custom, in order; one desktop confirmation per press for all its locks
  (older hotkeys: asked at each press).
- GUI Hotkeys page (`build_hotkeys_page`; settings area): Add hotkey…
  (`add_hotkey_actions`), Start a block… (`add_hotkey`, `HotkeyFields`,
  `LockPicker`), Save as preset….

## Notifications (`blocker.NotifyDesktop`, `enforce/notices.go`, `kc9k notifications`)

As each person on their session bus: `notify-send`, else `busctl --user`
(Ubuntu Cinnamon lacks libnotify-bin). User picks kinds (window start/end,
timer lock end, Pomodoro, allowance, app closed) and advance minutes
(default 5, each once); windows only of blocks that are on; scheduled
freeze always (5 and 1 min).
