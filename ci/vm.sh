#!/bin/bash
# Prepares a disposable test VM for remote control. Never run this on a real
# computer: it gives the matching key passwordless root.
set -e
sudo apt-get update
sudo apt-get install -y openssh-server
mkdir -p ~/.ssh
echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOIpLkwjhk7yPPbNTp0EGg1CY00XJyBwnpFeFmXZb1tI kc9k-test-vm' >> ~/.ssh/authorized_keys
chmod 700 ~/.ssh
chmod 600 ~/.ssh/authorized_keys
echo "$USER ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/90-claude-test >/dev/null
echo KLART
