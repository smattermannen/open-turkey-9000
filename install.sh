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
echo "==> Step 1/2: Compiling Killer Chicken 9000..."
make build

# --- Install the binary and service, and configure systemd ---
echo "==> Step 2/2: Installing Killer Chicken 9000..."
make install

# --- Success message with usage instructions ---
echo ""
echo "============================================="
echo "  Killer Chicken 9000 installed successfully!"
echo "============================================="
echo ""
echo "Useful commands:"
echo "  kc9k block create <block>                    - Create a block"
echo "  kc9k block add-site <block> <domains...>     - Add sites to the block"
echo "  kc9k block remove-site <block> <domains...>  - Remove sites from the block"
echo "  kc9k start <block>                           - Activate a block"
echo "  kc9k stop <block>                            - Deactivate a block (if not locked)"
echo "  kc9k status                                  - Show the status of the blocks"
echo ""
echo "Service management:"
echo "  systemctl status kc9k   - View service status"
echo "  systemctl restart kc9k  - Restart the service"
echo "  journalctl -u kc9k -f   - Follow logs in real time"
echo ""
