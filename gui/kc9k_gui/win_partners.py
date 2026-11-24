# Window: the accountability partner sections of the Account page (§4.15, 2.13).
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

REPORT_CHOICES = ("daily", "weekly", "off")


class _PartnersPage:
    """Methods of Window (window.py). Every change runs a `kc9k partner`
    command; the lists come from `partner list --json` and `partner inbox
    --json`, read with the account's status."""

    def build_partner_sections(self, box):
        self.partner_rows = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.partner_note = dim("")
        buttons = Gtk.Box(spacing=8)
        self.partner_buttons = {}
        for key, label, handler, tip in (
                ("invite", "Invite…", self.partner_invite, "Invite someone by email"),
                ("lock", "Lock a block…", self.partner_lock, "A password lock only your partner knows"),
                ("hold", "Give settings password…", self.partner_hold,
                 "Your settings password, made by KC9K, known only to your partner"),
                ("request", "Ask to unlock…", self.partner_request, "Ask the partner holding a password"),
                ("requests", "Check requests", self.partner_check_requests,
                 "Carries out an approved request")):
            button = Gtk.Button(label=label, tooltip_text=tip)
            button.connect("clicked", handler)
            buttons.pack_start(button, False, False, 0)
            self.partner_buttons[key] = button
        box.pack_start(section(
            "Accountability partners", self.partner_note, self.partner_rows, buttons,
            help="A partner gets reports of your time on sites and apps and how often they were blocked "
                 "(daily or weekly), and alerts as things happen: a block switched off, unlocked or deleted, "
                 "bypass prevention, tamper events, KC9K stopped or not heard from. A partner can hold a "
                 "password lock or your settings password: KC9K makes it and sends it only to them. "
                 "Removing a partner, fewer reports and alerts off are refused while anything is locked. "
                 "No screenshots, ever."), False, False, 0)

        self.watch_rows = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.watch_note = dim("")
        box.pack_start(section("You as a partner", self.watch_note, self.watch_rows,
                               help="People who asked you to be their accountability partner: accept "
                                    "their invitation, see when their KC9K was last heard from, the "
                                    "passwords you hold for them, and approve or deny their unlock "
                                    "requests."), False, False, 0)
        self.partners, self.partner_inbox = [], {}
        self.show_partners(None, None)

    def read_partners(self, passwords):
        """partner list --json and partner inbox --json (in a worker thread)."""
        out = []
        for args in (["partner", "list", "--json"], ["partner", "inbox", "--json"]):
            ok, output = cli_with_access(passwords, args, timeout=60)
            value = None
            for line in output.splitlines():
                if line.startswith(("[", "{")):
                    try:
                        value = json.loads(line)
                    except ValueError:
                        pass
            out.append(value if ok else None)
        return out

    def show_partners(self, partners, inbox):
        signed_in = bool((self.account_state or {}).get("email"))
        self.partners, self.partner_inbox = partners or [], inbox or {}
        for child in self.partner_rows.get_children() + self.watch_rows.get_children():
            child.destroy()
        if not signed_in:
            self.partner_note.set_text("Sign in first.")
        elif partners is None:
            self.partner_note.set_text("Your partners can't be read now.")
        elif not partners:
            self.partner_note.set_text("No partners yet.")
        else:
            self.partner_note.set_text("")
        for p in self.partners:
            row = Gtk.Box(spacing=8)
            name = Gtk.Label(xalign=0)
            name.set_markup(f"<b>{GLib.markup_escape_text(p['email'])}</b>")
            row.pack_start(name, False, False, 0)
            row.pack_start(pill("accepted" if p.get("accepted") else "invited", "on" if p.get("accepted") else "off"),
                           False, False, 0)
            words = [f"reports {p.get('reports')}", "alerts " + ("on" if p.get("alerts") else "off")]
            if p.get("holds"):
                words.append("holds " + ", ".join(hold_words(h) for h in p["holds"]))
            for r in p.get("requests") or []:
                words.append(f"request {r['id']} ({hold_words(r['what'])}): {r['status']}")
            row.pack_start(dim("; ".join(words)), True, True, 0)
            remove = Gtk.Button(label="Remove", valign=Gtk.Align.CENTER)
            remove.connect("clicked", self.partner_remove, p["email"])
            change = Gtk.Button(label="Change…", valign=Gtk.Align.CENTER)
            change.connect("clicked", self.partner_change, p)
            row.pack_end(remove, False, False, 0)
            row.pack_end(change, False, False, 0)
            self.partner_rows.pack_start(row, False, False, 0)
        accepted = [p for p in self.partners if p.get("accepted")]
        held = [h for p in self.partners for h in p.get("holds") or []]
        for key, on in (("invite", signed_in), ("lock", bool(accepted)), ("hold", bool(accepted)),
                        ("request", bool(held)), ("requests", signed_in and bool(self.partners))):
            self.partner_buttons[key].set_sensitive(on)

        inbox = self.partner_inbox
        lines = 0
        for i in inbox.get("invitations") or []:
            row = Gtk.Box(spacing=8)
            row.pack_start(Gtk.Label(label=f"{i['from']} asks you to be their partner.", xalign=0), True, True, 0)
            accept = Gtk.Button(label="Accept", valign=Gtk.Align.CENTER)
            accept.connect("clicked", lambda _b, who=i["from"]: self.run(["partner", "accept", who],
                                                                          then=self.account_done))
            row.pack_end(accept, False, False, 0)
            self.watch_rows.pack_start(row, False, False, 0)
            lines += 1
        for w in inbox.get("watching") or []:
            seen = account_time(w["last_seen"]) + f" ({w.get('device') or '?'})" if w.get("last_seen") else "never"
            self.watch_rows.pack_start(Gtk.Label(label=f"{w['email']}: KC9K last heard from {seen}.", xalign=0),
                                       False, False, 0)
            lines += 1
        for h in inbox.get("held") or []:
            row = Gtk.Box(spacing=8)
            row.pack_start(Gtk.Label(label=f"You hold {h['from']}'s {hold_words(h['what'])}.", xalign=0), True, True, 0)
            show = Gtk.Button(label="Show", valign=Gtk.Align.CENTER, tooltip_text="Show the password")
            show.connect("clicked", lambda _b, h=h: self.message(
                f"{h['from']}'s {hold_words(h['what'])}: {h['secret']}", Gtk.MessageType.INFO))
            row.pack_end(show, False, False, 0)
            self.watch_rows.pack_start(row, False, False, 0)
            lines += 1
        for r in inbox.get("requests") or []:
            row = Gtk.Box(spacing=8)
            reason = f": “{r['reason']}”" if r.get("reason") else ""
            row.pack_start(Gtk.Label(label=f"{r['from']} asks to unlock {hold_words(r['what'])}{reason}",
                                     xalign=0, wrap=True), True, True, 0)
            for verb in ("deny", "approve"):
                b = Gtk.Button(label=verb.capitalize(), valign=Gtk.Align.CENTER)
                b.connect("clicked", self.partner_answer, verb, r)
                row.pack_end(b, False, False, 0)
            self.watch_rows.pack_start(row, False, False, 0)
            lines += 1
        for m in (inbox.get("messages") or [])[:10]:
            self.watch_rows.pack_start(dim(f"{account_time(m.get('created_at', ''))}  {m['from']}: {m['subject']}"),
                                       False, False, 0)
            lines += 1
        self.watch_note.set_text("" if lines else "Nobody has asked you to be their partner.")
        self.partner_rows.show_all()
        self.watch_rows.show_all()
        return False

    def partner_invite(self, _button):
        email = self.ask_text("Invite a partner", "Their email address: they accept on their own KC9K.",
                              problem=lambda text: None if re.fullmatch(r"[^@\s]+@[^@\s]+\.[^@\s]+", text.strip())
                              else "An email address, such as friend@example.org.")
        if not email:
            return
        reports = self.choose("Reports", f"How often {email} gets a report of your time.", list(REPORT_CHOICES))
        if reports is None:
            return
        alerts = self.choose("Alerts", f"Does {email} get alerts as things happen?", ["on", "off"])
        if alerts is None:
            return
        self.run(["partner", "invite", email, "--reports", reports, f"--alerts={'true' if alerts == 'on' else 'false'}"],
                 then=self.account_done)

    def partner_change(self, _button, partner):
        order = [partner["reports"]] + [r for r in REPORT_CHOICES if r != partner["reports"]]
        reports = self.choose("Reports", f"How often {partner['email']} gets a report (fewer is refused "
                              "while anything is locked).", order)
        if reports is None:
            return
        now = "on" if partner.get("alerts") else "off"
        alerts = self.choose("Alerts", "Alerts as things happen (off is refused while anything is locked).",
                             [now, "off" if now == "on" else "on"])
        if alerts is None:
            return
        self.run(["partner", "set", partner["email"], "--reports", reports,
                  f"--alerts={'true' if alerts == 'on' else 'false'}"], then=self.account_done)

    def partner_remove(self, _button, email):
        if self.confirm(f"Remove {email}?", "They are told. Refused while anything is locked, or while they "
                        "hold your settings password.", "Remove", destructive=True):
            self.run(["partner", "remove", email], then=self.account_done)

    def accepted_partner(self, title, detail):
        names = [p["email"] for p in self.partners if p.get("accepted")]
        return self.choose(title, detail, names) if names else None

    def partner_lock(self, _button):
        blocks = [n for n, b in sorted(self.blocks.items()) if not b.get("draft_of")]
        if not blocks:
            self.message("Make a block first.")
            return
        block = self.choose("Lock a block", "The block to lock (it starts if it is off).", blocks)
        if block is None:
            return
        who = self.accepted_partner("Lock a block", f"Who holds the password of '{block}'? You never see it.")
        if who and self.confirm_lock([f"block '{block}': password lock only {who} knows"]):
            self.run(["partner", "lock", block, who, "--yes"], then=self.account_done)

    def partner_hold(self, _button):
        who = self.accepted_partner("Give the settings password",
                                    "KC9K makes a new settings password and sends it only to this partner; "
                                    "you never see it. Ask them for it with Ask to unlock.")
        if who and self.confirm_lock([f"the settings: settings password only {who} knows"]):
            self.run(["partner", "hold", "settings-password", who, "--yes"], then=self.account_done)

    def partner_request(self, _button):
        held = sorted({h for p in self.partners for h in p.get("holds") or []})
        what = self.choose("Ask to unlock", "What your partner holds the password of.",
                           [hold_words(h) for h in held])
        if what is None:
            return
        target = held[[hold_words(h) for h in held].index(what)]
        reason = self.ask_text("Ask to unlock", "Why (your partner reads it). Optional.")
        args = ["partner", "request", target.removeprefix("lock:")]
        self.run(args + (["--reason", reason] if reason else []), then=self.account_done)

    def partner_check_requests(self, _button):
        self.run(["partner", "requests"], then=self.account_done)

    def partner_answer(self, _button, verb, request):
        if verb == "approve" and not self.confirm(
                f"Approve {request['from']}'s request?", f"Their {hold_words(request['what'])} is lifted "
                "the next time their KC9K checks.", "Approve"):
            return
        self.run(["partner", verb, str(request["id"])], then=self.account_done)


def hold_words(what):
    """What a partner holds, for people: 'settings password', "block 'work'"."""
    if what == "settings-password":
        return "settings password"
    if what.startswith("lock:"):
        return f"lock of block '{what[5:]}'"
    return what
