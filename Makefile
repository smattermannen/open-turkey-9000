# =============================================================================
# Killer Chicken 9000 - Makefile
# Productivity blocker for Linux
# =============================================================================

# Name of the final binary (compiled by Go)
BINARY_NAME=kc9k

# Name of the binary installed on the system
# We rename it to "kc9k-bin" to separate the real binary
# from the wrapper script the user runs directly
BINARY_INSTALLED=kc9k-bin

# Binary install path
INSTALL_PATH=/usr/local/bin

# Path of the sudoers file that allows running without a password
# We use /etc/sudoers.d/ (a "drop-in" directory) instead of editing
# /etc/sudoers directly — that way the rule is isolated and easy to remove
SUDOERS_FILE=/etc/sudoers.d/kc9k

# Path to install the systemd service file
SERVICE_PATH=/etc/systemd/system

# Directory to store the SQLite database
DB_DIR=/var/lib/kc9k

# Set TAGS=devmaster for a testing build with the master password
TAGS?=

# Include the Go installed at /usr/local/go/bin in PATH
export PATH := /usr/local/go/bin:$(PATH)

# -------------------------------------------------------
# build: compiles the binary
# Uses CGO_ENABLED=0 because we use modernc.org/sqlite
# (a pure Go implementation, with no CGo dependency)
# -------------------------------------------------------
.PHONY: build
build:
	@echo ">> Compiling $(BINARY_NAME)..."
	CGO_ENABLED=0 go build -tags "$(TAGS)" -o $(BINARY_NAME) ./cmd/kc9k

# -------------------------------------------------------
# install: compiles, copies the binary, installs the systemd
#          service, creates the database directory,
#          configures sudoers and creates the wrapper script
#
# After installation, the user can run:
#   kc9k start ...
# without having to type "sudo" or a password.
# -------------------------------------------------------
.PHONY: install
install: build
	@echo ">> Installing the real binary as $(BINARY_INSTALLED)..."
	# The real binary gets a different name so that the wrapper
	# script can take the "kc9k" name
	# We copy to a temporary file and do an atomic rename.
	# This avoids the "Text file busy" (ETXTBSY) error when the daemon
	# is running the current binary.
	tmp="$(INSTALL_PATH)/.$(BINARY_INSTALLED).tmp.$$"; \
	cp $(BINARY_NAME) "$$tmp"; \
	chmod 755 "$$tmp"; \
	mv -f "$$tmp" $(INSTALL_PATH)/$(BINARY_INSTALLED)

	@echo ">> Installing the graphical interface..."
	install -m 755 gui/kc9k-gui.py $(INSTALL_PATH)/kc9k-gui
	install -m 644 gui/kc9k.desktop /usr/share/applications/kc9k.desktop

	@echo ">> Creating wrapper script at $(INSTALL_PATH)/$(BINARY_NAME)..."
	# The wrapper script is what the user runs when typing "kc9k".
	# It calls "sudo kc9k-bin" under the hood.
	# "exec" replaces the shell process with sudo, avoiding an
	# unnecessary intermediate process in the process tree.
	# "$$@" passes through all of the user's arguments (e.g. start, block, etc.)
	printf '#!/bin/bash\n\
# Killer Chicken 9000 wrapper script\n\
# Calls the real binary (kc9k-bin) with sudo, transparently.\n\
# The rule in /etc/sudoers.d/kc9k allows this without a password.\n\
# "exec" replaces this process with sudo — no leftover shell.\n\
exec sudo $(INSTALL_PATH)/$(BINARY_INSTALLED) "$$@"\n' > $(INSTALL_PATH)/$(BINARY_NAME)
	chmod 755 $(INSTALL_PATH)/$(BINARY_NAME)

	@$(MAKE) --no-print-directory install-sudoers

	@echo ">> Installing the systemd service..."
	cp systemd/kc9k.service $(SERVICE_PATH)/kc9k.service
	chmod 644 $(SERVICE_PATH)/kc9k.service

	@echo ">> Creating database directory at $(DB_DIR)..."
	mkdir -p $(DB_DIR)

	@echo ">> Enabling and (re)starting the service..."
	# restart, not start: on a reinstall the daemon is already running the old
	# binary, and "start" would leave it that way. Stopping the daemon never
	# lifts blocks, so the restart leaves no gap in enforcement.
	systemctl daemon-reload
	systemctl enable kc9k.service
	systemctl restart kc9k.service

	@echo ">> Installation completed successfully!"

# -------------------------------------------------------
# install-sudoers: lets administrators (group sudo) run kc9k-bin as root
#                  without a password:
#
#   %sudo ALL=(root) NOPASSWD: /usr/local/bin/kc9k-bin
#
# The interface asks for its state every 5 seconds (gui-state) and for its
# statistics every 30 (stats --json ...). sudo logs every command it runs and
# opens a PAM session for it: about 2,500 log lines an hour while the
# interface is open. Those two read-only commands are kept out of the log;
# every other kc9k command is still logged.
#
# visudo checks the rules before they are installed, because a broken
# sudoers file would break sudo altogether. sudo requires mode 440.
# -------------------------------------------------------
.PHONY: install-sudoers
install-sudoers:
	@echo ">> Configuring sudoers for passwordless execution..."
	@tmp=$$(mktemp) || exit 1; \
	bin="$(INSTALL_PATH)/$(BINARY_INSTALLED)"; \
	printf '%%sudo ALL=(root) NOPASSWD: %s\nCmnd_Alias KC9K_POLL = %s gui-state, %s stats --json *\nDefaults!KC9K_POLL !log_allowed, !pam_session\n' \
		"$$bin" "$$bin" "$$bin" > "$$tmp"; \
	if visudo -cf "$$tmp" >/dev/null; then \
		install -m 440 -o root -g root "$$tmp" "$(SUDOERS_FILE)"; status=$$?; \
	else \
		echo "visudo rejected the sudoers rule; $(SUDOERS_FILE) is unchanged" >&2; status=1; \
	fi; \
	[ -n "$$tmp" ] && rm -f -- "$$tmp"; \
	exit $$status

# -------------------------------------------------------
# uninstall: stops the service, disables it, removes the binary
#            and the systemd service file
# -------------------------------------------------------
.PHONY: uninstall
uninstall:
	@echo ">> Checking for locked blocks and cleaning blocking layers..."
	# Refuse to uninstall while a locked block is active, and clean up all
	# blocking layers (/etc/hosts, firewall, browser policies) so the system
	# is not left blocked. This line has no leading "-", so a non-zero exit
	# (a locked block) aborts the uninstall. Skipped if the binary is gone.
	if [ -x $(INSTALL_PATH)/$(BINARY_INSTALLED) ]; then $(INSTALL_PATH)/$(BINARY_INSTALLED) prepare-uninstall; fi

	@echo ">> Stopping the service..."
	-systemctl stop kc9k.service

	@echo ">> Disabling the service..."
	-systemctl disable kc9k.service

	@echo ">> Removing the service file..."
	rm -f $(SERVICE_PATH)/kc9k.service
	systemctl daemon-reload

	@echo ">> Removing the wrapper script and the real binary..."
	rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	rm -f $(INSTALL_PATH)/$(BINARY_INSTALLED)
	rm -f $(INSTALL_PATH)/kc9k-gui /usr/share/applications/kc9k.desktop

	@echo ">> Removing sudoers rule..."
	rm -f $(SUDOERS_FILE)

	@echo ">> Uninstallation completed successfully!"

# -------------------------------------------------------
# clean: removes build artifacts
# -------------------------------------------------------
.PHONY: clean
clean:
	@echo ">> Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	@echo ">> Cleanup complete!"
