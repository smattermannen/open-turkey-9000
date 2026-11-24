# Window: the Account page (§4.14 cross-device sync, 2.10).
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _AccountPage:
    """Methods of Window (window.py); no state of its own beyond widgets.
    Every change runs a kc9k command: account signup|login|logout|status,
    sync upload, sync download (dry run first)."""

    def build_account_page(self):
        scroll, box = page_box()
        page_header(box, "Account", "Your KC9K account: copy your whole setup between your devices.",
                    help="A KC9K account is needed only for cross-device sync: nothing else leaves this "
                         "device. Upload all puts this device's setup in your account; Download all "
                         "replaces another device's setup with it. What belongs to a device stays there: "
                         "places and place rules, affected users, apps' paths, wallpapers, statistics, "
                         "strict mode, no-escape and the firmware lock. A lock set on a device stops "
                         "Download all there until it ends.")
        self.account_status = Gtk.Label(xalign=0, wrap=True, selectable=True)
        self.account_detail = Gtk.Label(xalign=0, wrap=True)
        self.account_detail.get_style_context().add_class("kc9k-note")
        self.account_buttons = {}
        row = Gtk.Box(spacing=8)
        for key, label, handler, tip in (
                ("signup", "Sign up…", self.account_signup, "Make a KC9K account"),
                ("login", "Log in…", self.account_login, "Sign in to your KC9K account"),
                ("logout", "Log out", self.account_logout, "Sign out on this device"),
                ("refresh", "Refresh", lambda _b: self.refresh_account(), "Ask the account again")):
            button = Gtk.Button(label=label, tooltip_text=tip)
            button.connect("clicked", handler)
            row.pack_start(button, False, False, 0)
            self.account_buttons[key] = button
        box.pack_start(section("Account", self.account_status, self.account_detail, row), False, False, 0)

        sync_row = Gtk.Box(spacing=8)
        for key, label, handler, tip in (
                ("upload", "Upload all", self.sync_upload, "Put this device's setup in your account"),
                ("download", "Download all…", self.sync_download,
                 "Replace this device's setup with the last upload (you see what changes first)")):
            button = Gtk.Button(label=label, tooltip_text=tip)
            button.connect("clicked", handler)
            sync_row.pack_start(button, False, False, 0)
            self.account_buttons[key] = button
        self.sync_note = Gtk.Label(xalign=0, wrap=True)
        self.sync_note.get_style_context().add_class("kc9k-note")
        box.pack_start(section("Your setup", self.sync_note, sync_row), False, False, 0)
        self.account_state = None
        self.build_partner_sections(box)  # §4.15, win_partners.py
        self.show_account({"available": False}, loading=True)
        return scroll

    def refresh_account(self):
        """kc9k account status --json, in the background (it asks the cloud)."""
        passwords = list(self.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, ["account", "status", "--json"], timeout=60)
            state = None
            for line in output.splitlines():
                if line.startswith("{"):
                    try:
                        state = json.loads(line)
                    except ValueError:
                        pass
            if state is None:
                state = {"available": False, "error": output or "No answer."}
            GLib.idle_add(self.show_account, state)
            partners = inbox = None
            if state.get("email"):
                partners, inbox = self.read_partners(passwords)
            GLib.idle_add(self.show_partners, partners, inbox)

        threading.Thread(target=work, daemon=True).start()

    def show_account(self, state, loading=False):
        self.account_state = state
        email = state.get("email", "")
        upload = state.get("last_upload")
        if loading:
            status, detail = "Asking your account…", ""
        elif not state.get("available"):
            status = "KC9K's cloud isn't set up in this build yet."
            detail = state.get("error", "")
        elif not email:
            status, detail = "Not signed in.", "Sign up or log in to copy your setup between devices."
        else:
            status = f"Signed in as {email}."
            detail = state.get("error", "")
        self.account_status.set_text(status)
        self.account_detail.set_text(detail)
        self.account_detail.set_visible(bool(detail))
        if upload:
            self.sync_note.set_text(f"Last upload: {account_time(upload.get('uploaded_at', ''))} "
                                    f"from {upload.get('device', '?')} ({upload.get('size', 0) // 1024 + 1} KB).")
        elif email and not state.get("error"):
            self.sync_note.set_text("Nothing uploaded yet.")
        else:
            self.sync_note.set_text("Sign in first.")
        available, signed_in = bool(state.get("available")) and not loading, bool(email) and not loading
        for key, on in (("signup", available and not signed_in), ("login", available and not signed_in),
                        ("logout", signed_in), ("refresh", not loading), ("upload", signed_in),
                        ("download", signed_in and bool(upload))):
            self.account_buttons[key].set_sensitive(on)
        return False

    def ask_account(self, title, detail, new):
        email = self.ask_text(title, "Your email address.",
                              initial=(self.account_state or {}).get("email", ""),
                              problem=lambda text: None if re.fullmatch(r"[^@\s]+@[^@\s]+\.[^@\s]+", text.strip())
                              else "An email address, such as you@example.org.")
        if not email:
            return None, None
        password = self.ask_text(title, detail, password=True,
                                 problem=(lambda text: None if len(text) >= 8 else "At least 8 characters.")
                                 if new else None)
        if not password:
            return None, None
        if new and self.ask_text(title, "The same password again.", password=True) != password:
            self.message("The two passwords differ: nothing was made.")
            return None, None
        return email.strip(), password

    def account_signup(self, _button):
        email, password = self.ask_account("Sign up", "A new password for your KC9K account.", True)
        if email:
            self.run(["account", "signup", email], password + "\n", then=self.account_done)

    def account_login(self, _button):
        email, password = self.ask_account("Log in", "Your KC9K account password.", False)
        if email:
            self.run(["account", "login", email], password + "\n", then=self.account_done)

    def account_logout(self, _button):
        if self.confirm("Log out on this device?", "Your account and its upload stay; this device forgets "
                        "the session.", "Log out"):
            self.run(["account", "logout"], then=self.account_done)

    def account_done(self, output):
        if output:
            self.message(output, Gtk.MessageType.INFO)
        self.refresh_account()

    def sync_upload(self, _button):
        upload = (self.account_state or {}).get("last_upload")
        if upload and not self.confirm(
                "Upload all?", f"This replaces the upload of {account_time(upload.get('uploaded_at', ''))} "
                f"from {upload.get('device', '?')} with this device's setup.", "Upload"):
            return
        self.run(["sync", "upload"], then=self.account_done)

    def sync_download(self, _button, ack=False):
        """Download all: the dry run says what would change, then one
        confirmation (the replace and the locks it sets, R-LOCK-12)."""
        extra = ["--no-confirm-ok"] if ack else []
        ok, output = cli_with_access(self.access_passwords, ["sync", "download", "--dry-run", *extra], timeout=120)
        if not ok and "--no-confirm-ok" in output and not ack:
            if self.acknowledge_no_confirmation("The upload"):
                self.sync_download(_button, ack=True)
            return
        if not ok:
            self.message(output)
            return
        locks = [line.split("Would: set a lock: ", 1)[1] for line in output.splitlines()
                 if "Would: set a lock: " in line]
        if self.confirm("Download all?", "This device's setup is replaced by the upload:\n\n"
                        + output.replace("Would: ", "• "), "Replace", destructive=True) \
                and self.confirm_lock(locks):
            self.run(["sync", "download", "--yes", *extra], then=self.account_done)


def account_time(stamp):
    """An upload time from the cloud (RFC 3339) as local date and time."""
    try:
        when = datetime.datetime.fromisoformat(stamp.replace("Z", "+00:00")).astimezone()
    except (ValueError, AttributeError):
        return stamp or "?"
    return when.strftime("%Y-%m-%d " + ("%H:%M" if CLOCK_24 else "%-I:%M %p"))
