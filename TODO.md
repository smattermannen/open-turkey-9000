# KC9K status

Works on Linux Mint 22.3: strict mode, noexec, no-escape reboot, master password, screen lock in Frozen Chicken.
Tested on a real Mint/Windows dual boot: the LibreWolf extension (`ci/browser_test.py`) and no-escape (Mint and Windows boot without the password, entry editing is locked, the firmware boot menu still reaches Windows).

## Next: try on the dual boot
Built and tested in a cloud sandbox (unit and root tests, Xvfb with xterm and openbox, headless Chromium, a pty for the challenge), not yet on real hardware:
1. The mandatory extension: with a website block active, a browser without it (Firefox, Chromium) closes within about 60 s, and at once when started with remote-control flags (`--marionette`, `--remote-debugging-port`); LibreWolf with the extension keeps running, and `kc9k status` shows its last check-in. The first website block after upgrading closes open LibreWolf windows once, because the settings protecting the extension changed.
2. LibreWolf hardening: F12 and the developer tools are off on every page, about:config shows the five locked prefs, the extension can't be disabled or removed in about:addons, and a new extension version shows up there after `make install`. Private windows are covered too.
3. Window titles: `kc9k block add-title <block> youtube`, then an app window with it in the title (e.g. `xterm -T YouTube`) is asked to close and killed 4 s later if it stays; a LibreWolf tab with that title goes to the block page.
4. Notifications 5 minutes before a schedule, a scheduled freeze or the end of a quota session.
5. The settings password (`kc9k password set`) in the terminal and in the interface (it asks once and remembers).
6. Scheduled Frozen Chicken (`kc9k freeze schedule add`), in lock, logout and shutdown modes.
7. The interface: the week calendar on the Schedule tab, the window-titles list, the Scheduled list under Device Blocks, the paged typing challenge.
8. Strict mode: `unshare -Urm` gives no way out, Flatpak apps installed with `--user` are killed, and Electron apps (VS Code) aren't taken for browsers.
9. From before: shutdown mode and the `kc9k.nofreeze` GRUB escape; the fast relock (89c71bf) isn't measured yet. Run `ci/browser_test.py` in LibreWolf again (per-block greyscale and images, title blocking).

## Later
- Count quota time only while a block's sites or apps are in use, measured by the extension (its statistics sampling in background.js can be reused) and the daemon's foreground-app sampling. Watch for loopholes: a muted video beside another window, and browsers without the extension while a session is open.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`. On the test laptop, remove `/etc/sudoers.d/99-claude-test`.
