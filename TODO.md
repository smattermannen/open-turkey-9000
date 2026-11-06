# KC9K status

Works on Linux Mint 22.3: strict mode, noexec, no-escape reboot, master password, screen lock in Frozen Chicken.
Tested on a real Mint/Windows dual boot: the LibreWolf extension (`ci/browser_test.py`) and no-escape (Mint and Windows boot without the password, entry editing is locked, the firmware boot menu still reaches Windows).

## Next
1. Test `require-extension` on the dual boot: with a block active, Firefox should close about 60 s after turning it on, while LibreWolf with the extension keeps running. Until it's on, blocked keywords can be searched in Firefox. Also try the per-option locks (0002) and the per-block greyscale/no-images (0001) by hand.
2. Frozen Chicken: lock mode, restart and logout mode work on the dual boot. Still untested: shutdown mode and the `kc9k.nofreeze` GRUB escape. The fast relock (89c71bf) is not yet measured. `kc9k master` can't be used in logout mode (text console sessions end within seconds), but the README says it works during a freeze: fix the README or the code.
3. Try the rebuilt interface by hand (dialogs, unlock, quota, schedules); only rendering and switches were tested automatically.
4. Measure real quota time in the extension (the statistics sampling in background.js can be reused).
5. Weekly calendar, notifications and settings password.

Retest `ci/browser_test.py` in LibreWolf: greyscale and images are now chosen per block (`kc9k block action`), and the block page returns to the site when its block ends.

## Before release
Build without `TAGS=devmaster`. Remove `ci/vm.sh`. Rename the GitHub repository to `kc9k`.
