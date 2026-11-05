#!/bin/bash
# Generates the boot lock on a disposable machine. This checks the files and the
# boot menu; it can't prove that a real computer boots afterwards.
set -euo pipefail

sudo apt-get install -y grub-common grub2-common grub-pc-bin >/dev/null
if ! command -v update-grub >/dev/null || [ ! -d /boot/grub ]; then
    echo "::notice::boot lock skipped: no GRUB on this machine"
    exit 0
fi

# Earlier tests leave long locks running, so 'off' is refused here on purpose.
open-turkey no-escape on --yes
sudo grep -q '^password_pbkdf2 openturkey grub.pbkdf2' /boot/grub/grub.cfg
sudo grep -q -- '--unrestricted' /boot/grub/grub.cfg
if open-turkey no-escape off; then
    echo "::error::FAIL: no-escape turned off during a lock"
    exit 1
fi
echo "Boot lock generated."
