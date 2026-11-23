# Window: KC9K account page (§4.14 cross-device sync).
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _AccountPage:
    """Methods of Window (window.py). Every change runs a kc9k command:
    account signup|login|logout|status, sync upload|download."""

    # Account -------------------------------------------------------------
    def build_account(self):
        scroll, box = page_box()
        page_header(box, "Account",
                    "Your KC9K account is only for cross-device sync. Nothing else leaves this computer.",
                    help="Upload all sends this computer's whole setup to your KC9K account. Download all "
                         "replaces this computer's setup with the last upload: blocks, lists, phrase sets, locks "
                         "and their state, settings and presets. What belongs to this computer stays: places, "
                         "affected users, apps' paths, wallpapers, statistics, strict mode, no-escape and "
                         "firmware lock.")
        self.account_state = None
        self.account_status = Gtk.Label(xalign=0, wrap=True, label="Reading your account…")
        self.account_email = Gtk.Entry(placeholder_text="Email", width_chars=28,
                                       input_purpose=Gtk.InputPurpose.EMAIL)
        self.account_signup = Gtk.Button(label="Sign up…")
        self.account_signup.connect("clicked", lambda _b: self.account_sign_in(new=True))
        self.account_login = Gtk.Button(label="Log in…")
        self.account_login.get_style_context().add_class("suggested-action")
        self.account_login.connect("clicked", lambda _b: self.account_sign_in(new=False))
        self.account_email.connect("activate", lambda _e: self.account_sign_in(new=False))
        self.account_signed_out = Gtk.Box(spacing=6)
        for widget, expand in ((self.account_email, True), (self.account_login, False),
                               (self.account_signup, False)):
            self.account_signed_out.pack_start(widget, expand, expand, 0)
        self.account_logout = Gtk.Button(label="Log out", halign=Gtk.Align.START)
        self.account_logout.connect("clicked", self.account_log_out)
        self.account_signed_out.set_no_show_all(True)
        self.account_logout.set_no_show_all(True)
        box.pack_start(section("Account", self.account_status, self.account_signed_out, self.account_logout),
                       False, False, 0)

        self.sync_status = Gtk.Label(xalign=0, wrap=True)
        self.sync_upload = Gtk.Button(label="Upload all")
        self.sync_upload.set_tooltip_text("Send this computer's whole setup to your account, "
                                          "in place of the last upload")
        self.sync_upload.connect("clicked", self.upload_all)
        self.sync_download = Gtk.Button(label="Download all…")
        self.sync_download.set_tooltip_text("Replace this computer's setup with the last upload")
        self.sync_download.connect("clicked", self.download_all)
        buttons = Gtk.Box(spacing=6)
        buttons.pack_start(self.sync_upload, False, False, 0)
        buttons.pack_start(self.sync_download, False, False, 0)
        box.pack_start(section("Cross-device sync", self.sync_status, buttons), False, False, 0)
        self.update_account()
        return scroll

    def refresh_account(self):
        """Read `kc9k account status --json` in the background (it asks the
        cloud for the last upload)."""
        if getattr(self, "account_busy", False):
            return
        self.account_busy = True
        passwords = list(self.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, ["account", "status", "--json"])
            GLib.idle_add(self.account_read, ok, output)

        threading.Thread(target=work, daemon=True).start()

    def account_read(self, ok, output):
        self.account_busy = False
        try:
            self.account_state = json.loads(output) if ok else {"available": True, "error": output}
        except ValueError:
            self.account_state = {"available": True, "error": output}
        self.update_account()
        return False

    def update_account(self):
        st = self.account_state
        signed_in = bool(st and st.get("email"))
        available = bool(st and st.get("available"))
        if st is None:
            text = "Reading your account…"
        elif not available:
            text = "KC9K's cloud isn't set up in this version yet."
        elif signed_in:
            text = f"Signed in as {st['email']}."
        else:
            text = "Not signed in. Log in, or sign up with your email."
        if st and st.get("error"):
            text += f"\n{st['error']}"
        self.account_status.set_text(text)
        # no_show_all keeps the window's show_all off it; shown here.
        self.account_signed_out.set_visible(available and not signed_in)
        for child in self.account_signed_out.get_children():
            child.show()
        self.account_logout.set_visible(signed_in)
        upload = (st or {}).get("last_upload")
        if not signed_in:
            self.sync_status.set_text("Log in to upload or download your setup.")
        elif upload:
            self.sync_status.set_text(f"Last upload: {upload_time(upload.get('uploaded_at'))} "
                                      f"from {upload.get('device') or 'a device'}.")
        else:
            self.sync_status.set_text("Nothing uploaded yet.")
        self.sync_upload.set_sensitive(signed_in)
        self.sync_download.set_sensitive(signed_in and bool(upload))

    def account_sign_in(self, new):
        email = self.account_email.get_text().strip()
        if not email:
            self.message("Type your email first.")
            return
        if new:
            password = self.ask_text("Sign up", f"Choose a password for the KC9K account {email}.", password=True)
            if not password:
                return
            again = self.ask_text("Sign up", "Type the password again.", password=True)
            if again is None:
                return
            if again != password:
                self.message("The passwords aren't the same.")
                return
        else:
            password = self.ask_text("Log in", f"The password of the KC9K account {email}.", password=True)
            if not password:
                return
        self.run(["account", "signup" if new else "login", email], secret=password + "\n",
                 then=self.account_done, timeout=60)

    def account_log_out(self, _button):
        if self.confirm("Log out of your KC9K account?",
                        "Your setup stays on this computer and in your account.", "Log out"):
            self.run(["account", "logout"], then=self.account_done, timeout=60)

    def account_done(self, output):
        if output:
            self.message(output, Gtk.MessageType.INFO)
        self.refresh_account()

    def upload_all(self, _button):
        upload = (self.account_state or {}).get("last_upload")
        if upload and not self.confirm(
                "Upload all?",
                f"This computer's setup replaces the upload of {upload_time(upload.get('uploaded_at'))} "
                f"from {upload.get('device') or 'a device'}.", "Upload all"):
            return
        self.run(["sync", "upload"], then=self.account_done, timeout=300)

    def download_all(self, _button):
        """Download all: what it would do first (its dry run), then one
        confirmation naming every lock it sets (R-LOCK-12), and the tick for
        monk mode starting by itself when the upload has that."""
        self.set_sensitive(False)
        passwords = list(self.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, ["sync", "download", "--dry-run"], timeout=300)
            GLib.idle_add(self.download_all_confirm, ok, output)

        threading.Thread(target=work, daemon=True).start()

    def download_all_confirm(self, ok, output):
        self.set_sensitive(True)
        if not ok:
            self.message(output or "Download all can't run.")
            self.refresh_account()
            return False
        lines = output.splitlines()
        head = [line for line in lines if not line.startswith("Would: ")]
        would = [line[len("Would: "):] for line in lines if line.startswith("Would: ")]
        detail = "\n".join(head + ["", "It will:"] + [f"• {w}" for w in would] + [
            "", "Locks it sets aren't asked about again: this is their confirmation."])
        if not self.confirm("Replace this computer's setup with the last upload?", detail,
                            "Download all", destructive=True):
            return False
        self.download_all_run(["sync", "download", "--yes"])
        return False

    def download_all_run(self, args):
        def then(output):
            self.account_done(output)

        def finished_check(ok, output, then_, request=None):
            # Monk mode that starts by itself needs the tick (R-LOCK-12).
            if not ok and "--no-confirm-ok" in output and "--no-confirm-ok" not in args:
                self.set_sensitive(True)
                if self.acknowledge_no_confirmation("The upload"):
                    self.download_all_run(args + ["--no-confirm-ok"])
                return False
            return self.finished(ok, output, then_, request)

        self.set_sensitive(False)
        passwords = list(self.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, args, timeout=300)
            GLib.idle_add(finished_check, ok, output, then, None)

        threading.Thread(target=work, daemon=True).start()


def upload_time(value):
    """An upload's time as the cloud gives it (RFC 3339), shown local."""
    if not value:
        return "an unknown time"
    try:
        when = datetime.datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone()
    except ValueError:
        return value
    return when.strftime("%Y-%m-%d %H:%M")
