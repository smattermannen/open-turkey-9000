#!/bin/bash
# =============================================================================
# Killer Chicken 9000 - Install Script
# Productivity blocker for Linux
#
# This script automates compiling and installing Killer Chicken 9000.
# It must be run as root (or via sudo).
# =============================================================================

set -euo pipefail

# --- Check that the script is running as root ---
# Killer Chicken 9000 needs root privileges to:
#   - copy the binary to /usr/local/bin
#   - install the systemd service
#   - create the database directory
if [ "$(id -u)" -ne 0 ]; then
    echo "Error: this script must be run as root."
    echo "Usage: sudo ./install.sh"
    exit 1
fi

# --- Change to the project directory ---
# Ensures the make commands find the Makefile
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# --- Compile the binary ---
echo "==> Step 1/3: Compiling Killer Chicken 9000..."
make build

# --- Install the binary and service, and configure systemd ---
echo "==> Step 2/3: Installing Killer Chicken 9000..."
make install

# --- Install LibreWolf, the browser that runs the Killer Chicken extension ---
# Without it, wildcards, keywords, greyscale, images and web-only aren't
# enforced in the browser. It needs the internet, so a failure (offline, say)
# doesn't stop the installation: 'kc9k status' keeps reminding, and
# 'kc9k browser install-librewolf' tries again. Set KC9K_NO_LIBREWOLF=1 to skip.
echo "==> Step 3/3: Installing LibreWolf..."
if [ "${KC9K_NO_LIBREWOLF:-}" = "1" ]; then
    echo "Skipped (KC9K_NO_LIBREWOLF=1)."
elif ! /usr/local/bin/kc9k-bin browser install-librewolf; then
    echo ""
    echo "WARNING: LibreWolf couldn't be installed. Killer Chicken 9000 works, but"
    echo "until LibreWolf is installed, website rules beyond plain domains aren't"
    echo "enforced in the browser. Try again later with: kc9k browser install-librewolf"
fi

# --- Success message with usage instructions ---
echo ""
echo "============================================="
echo "  Killer Chicken 9000 installed successfully!"
echo "============================================="
echo ""
echo "Useful commands:"
echo "  kc9k block create <block>                    - Create a block"
echo "  kc9k block add <block> <sites...>            - Block sites (its Blocked list)"
echo "  kc9k list show <block>                       - The block's website lists"
echo "  kc9k start <block>                           - Activate a block"
echo "  kc9k stop <block>                            - Deactivate a block (if not locked)"
echo "  kc9k status                                  - Show the status of the blocks"
echo ""
echo "Service management:"
echo "  systemctl status kc9k   - View service status"
echo "  systemctl restart kc9k  - Restart the service"
echo "  journalctl -u kc9k -f   - Follow logs in real time"
echo ""
