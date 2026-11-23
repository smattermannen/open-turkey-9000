# Window: KC9K account page (§4.14 cross-device sync).
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _AccountPage:
    """Methods of Window (window.py). Every change runs a kc9k command:
    account signup|login|logout|status, sync upload|download, partner …"""

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

        # §4.15 Accountability partner.
        self.partner_state, self.partner_requests = None, []
        self.partner_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.partner_invite_email = Gtk.Entry(placeholder_text="Your partner's email", width_chars=28,
                                              input_purpose=Gtk.InputPurpose.EMAIL)
        invite = Gtk.Button(label="Invite")
        invite.connect("clicked", self.partner_invite)
        self.partner_invite_email.connect("activate", self.partner_invite)
        invite_row = Gtk.Box(spacing=6)
        invite_row.pack_start(self.partner_invite_email, True, True, 0)
        invite_row.pack_start(invite, False, False, 0)
        self.partner_reports = Gtk.ComboBoxText()
        for kind, label in (("daily", "Reports every day"), ("weekly", "Reports every week"), ("off", "No reports")):
            self.partner_reports.append(kind, label)
        self.partner_reports.connect("changed", self.partner_reports_changed)
        ask = Gtk.Button(label="Ask to unlock…")
        ask.set_tooltip_text("Ask your partners to unlock a block's password lock or the settings password they hold")
        ask.connect("clicked", self.partner_ask)
        inbox = Gtk.Button(label="Inbox…")
        inbox.set_tooltip_text("As a partner: the reports, alerts and passwords of the people you partner")
        inbox.connect("clicked", self.partner_inbox)
        send = Gtk.Button(label="Send now")
        send.set_tooltip_text("Send waiting alerts, a due report and this computer's sign of life now")
        send.connect("clicked", lambda _b: self.run(["partner", "send"], then=self.account_done, timeout=120))
        actions = Gtk.Box(spacing=6)
        for widget in (self.partner_reports, ask, inbox, send):
            actions.pack_start(widget, False, False, 0)
        self.partner_section = section(
            "Accountability partner", self.partner_box, invite_row, actions,
            help="A partner gets your reports (daily or weekly: time per site and app, times blocked, top "
                 "blocked) and alerts as they happen (bypass prevention, a block switched off, unlocked or "
                 "deleted, tamper events, Killer Chicken uninstalled). A partner can hold password locks and "
                 "the settings password, so you never see them, and approve your unlock requests. Removing "
                 "a partner or getting fewer reports weakens: refused while anything is locked.")
        self.partner_section.set_no_show_all(True)
        box.pack_start(self.partner_section, False, False, 0)
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
            partner = requests = None
            if ok and '"email"' in output:
                partner = cli_with_access(passwords, ["partner", "--json"])
                requests = cli_with_access(passwords, ["partner", "requests", "--json"])
            GLib.idle_add(self.account_read, ok, output, partner, requests)

        threading.Thread(target=work, daemon=True).start()

    def account_read(self, ok, output, partner=None, requests=None):
        self.account_busy = False
        self.partner_state, self.partner_requests = None, []
        try:
            if partner and partner[0]:
                self.partner_state = json.loads(partner[1])
            if requests and requests[0]:
                self.partner_requests = json.loads(requests[1]) or []
        except ValueError:
            pass
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
        # no_show_all keeps the window's show_all off it; shown here.
        self.partner_section.set_visible(signed_in)
        if signed_in:
            for child in self.partner_section.get_children():
                child.show_all()
            self.update_partners()

    def update_partners(self):
        """The partner section from `partner --json` and `partner requests
        --json`."""
        for child in self.partner_box.get_children():
            self.partner_box.remove(child)
        st = self.partner_state or {}
        me = (self.account_state or {}).get("email", "")

        def row(text, *buttons):
            line = Gtk.Box(spacing=6)
            line.pack_start(Gtk.Label(label=text, xalign=0, wrap=True), True, True, 0)
            for label, handler in buttons:
                button = Gtk.Button(label=label)
                button.connect("clicked", handler)
                line.pack_start(button, False, False, 0)
            self.partner_box.pack_start(line, False, False, 0)

        partners = st.get("partners") or []
        if not partners:
            row("No partner yet. Invite someone you trust; they need a KC9K account.")
        for p in partners:
            state = "active" if p.get("status") == "active" else "invited, not accepted yet"
            row(f"Your partner: {p['partner_email']} ({state})",
                ("Remove…", lambda _b, e=p["partner_email"]: self.partner_remove(e)))
        for p in st.get("partnered") or []:
            if p.get("status") == "active":
                row(f"You are the partner of {p['owner_email']}.",
                    ("Leave…", lambda _b, e=p["owner_email"]: self.partner_leave(e)))
            else:
                row(f"{p['owner_email']} asks you to be their partner.",
                    ("Accept", lambda _b, e=p["owner_email"]: self.run(["partner", "accept", e], then=self.account_done)),
                    ("Decline", lambda _b, e=p["owner_email"]: self.run(["partner", "leave", e], then=self.account_done)))
        for q in self.partner_requests:
            what = target_words(q.get("target", ""))
            if q.get("owner_email") == me:
                if q.get("status") == "approved":
                    arg = "settings" if q["target"] == "settings" else q["target"].split(":", 1)[-1]
                    row(f"Your request to unlock {what}: approved by {q.get('decided_by')}.",
                        ("Unlock", lambda _b, a=arg: self.run(["partner", "unlock", a], then=self.account_done)))
                else:
                    row(f"Your request to unlock {what}: {q.get('status')}.")
            elif q.get("status") == "pending":
                reason = f" ({q['reason']})" if q.get("reason") else ""
                row(f"{q['owner_email']} asks to unlock {what}{reason}.",
                    ("Approve", lambda _b, i=q["id"]: self.partner_decide(i, True)),
                    ("Deny", lambda _b, i=q["id"]: self.partner_decide(i, False)))
        if st.get("queued"):
            row(f"{st['queued']} alert(s) waiting to be sent.")
        self.partner_box.show_all()
        self.updating_reports = True
        self.partner_reports.set_active_id(st.get("reports") or "weekly")
        self.updating_reports = False

    def partner_invite(self, _widget):
        email = self.partner_invite_email.get_text().strip()
        if not email:
            self.message("Type your partner's email first.")
            return
        self.partner_invite_email.set_text("")
        self.run(["partner", "invite", email], then=self.account_done)

    def partner_remove(self, email):
        if self.confirm(f"Remove {email} as your partner?",
                        "They are told, and get no more reports or alerts. This weakens your setup, so it is "
                        "refused while anything is locked.", "Remove", destructive=True):
            self.run(["partner", "remove", email], then=self.account_done)

    def partner_leave(self, email):
        if self.confirm(f"Stop being the partner of {email}?", "You get no more of their reports and alerts.",
                        "Leave", destructive=True):
            self.run(["partner", "leave", email], then=self.account_done)

    def partner_reports_changed(self, combo):
        if getattr(self, "updating_reports", False) or not combo.get_active_id():
            return
        if combo.get_active_id() != (self.partner_state or {}).get("reports"):
            self.run(["partner", "reports", combo.get_active_id()], then=self.account_done)

    def partner_decide(self, request_id, approve):
        if approve and not self.confirm("Approve this unlock?",
                                        "The password you hold goes to them, for one use.", "Approve"):
            return
        self.run(["partner", "approve" if approve else "deny", request_id], then=self.account_done)

    def partner_ask(self, _button):
        dialog = Gtk.Dialog(title="Ask to unlock", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Ask", Gtk.ResponseType.OK)
        area = dialog.get_content_area()
        area.set_spacing(8)
        area.set_border_width(12)
        choice = Gtk.ComboBoxText()
        for name in sorted(getattr(self, "blocks", {}) or {}):
            choice.append(name, f"Block '{name}'")
        choice.append("settings", "The settings password")
        choice.set_active(0)
        reason = Gtk.Entry(placeholder_text="Why (your partners see it)")
        area.pack_start(Gtk.Label(label="What should your partners unlock?", xalign=0), False, False, 0)
        area.pack_start(choice, False, False, 0)
        area.pack_start(reason, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        target, why = choice.get_active_id(), reason.get_text().strip()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and target:
            args = ["partner", "request", target] + (["--reason", why] if why else [])
            self.run(args, then=self.account_done)

    def partner_inbox(self, _button):
        self.run(["partner", "inbox"], then=lambda output: self.message(output or "Nothing yet.",
                                                                         Gtk.MessageType.INFO))

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


def target_words(target):
    """A request's target for people (as the CLI's targetWords)."""
    if target.startswith("block:"):
        return f"block '{target[6:]}' (password lock)"
    if target == "settings":
        return "the settings password"
    return target
