# The main window: build, running commands, state, access passwords. Each page's methods are in win_*.py.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class Window(_SchedulesPlacesPages, _DashboardPage, _BlocksPage, _ModesAndBlockView, _BlockActions,
             _ClustersPage, _DevicePage, _StatsPage, _SettingsPages, Gtk.ApplicationWindow):
    def __init__(self, app):
        super().__init__(application=app, title="Killer Chicken 9000")
        self.set_default_size(980, 680)
        # §4.10 tray: closing hides the window while the tray icon is on.
        self.connect("delete-event", lambda w, _e: bool(getattr(w.get_application(), "tray", None)) and w.hide_on_delete())
        self.state_json = None
        self.state = None
        self.blocks = {}
        self.selected = None
        self.busy = False
        self.updating = False  # set while widgets are filled from the state
        self.settings_password = None  # remembered for this session once given
        self.access_passwords = []  # access passwords given, until closed or locked
        self.block_passwords = {}  # §3.11: block passwords typed, until closed
        self.card_switches = {}  # §4.2: the block list's on/off switches, by block
        self.access = {}  # area -> "none", "open" or "locked"
        self.covers = {}  # page name -> (stack, cover text, entry, title)
        self.last_input = GLib.get_monotonic_time()  # for locking again when idle
        self.connect("event-after", self.note_input)
        self.stats_rows = self.stats_page_rows = []  # the dashboard's and the Statistics page's (filtered)
        self.stats_on = True

        header = Gtk.HeaderBar(show_close_button=True, title="Killer Chicken 9000")
        self.set_titlebar(header)
        self.lock_button = Gtk.Button(label="Lock", no_show_all=True,
                                      tooltip_text="Forget the passwords given and lock the pages again")
        self.lock_button.connect("clicked", self.lock_access)
        header.pack_end(self.lock_button)

        root = Gtk.Box()
        self.add(root)
        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE)
        # The sidebar: each page with its icon (R-GUI-3: recognised at a
        # glance), the app's name on top.
        side = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        side.get_style_context().add_class("kc9k-sidebar")
        side.set_size_request(200, -1)
        brand = Gtk.Box(spacing=8, margin_start=18, margin_top=16, margin_bottom=10)
        brand.pack_start(Gtk.Image.new_from_icon_name("security-high-symbolic", Gtk.IconSize.LARGE_TOOLBAR),
                         False, False, 0)
        brand_name = Gtk.Label(label="Killer Chicken", xalign=0)
        brand_name.get_style_context().add_class("kc9k-brand")
        brand.pack_start(brand_name, False, False, 0)
        side.pack_start(brand, False, False, 0)
        self.nav = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        self.nav.get_style_context().add_class("kc9k-sidebar")
        self.nav_rows = {}
        side.pack_start(self.nav, True, True, 0)
        root.pack_start(side, False, False, 0)
        root.pack_start(Gtk.Separator(orientation=Gtk.Orientation.VERTICAL), False, False, 0)

        content = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        root.pack_start(content, True, True, 0)
        self.banner = Gtk.InfoBar(message_type=Gtk.MessageType.WARNING, revealed=False)
        self.banner_label = Gtk.Label(xalign=0, wrap=True)
        self.banner.get_content_area().add(self.banner_label)
        content.pack_start(self.banner, False, False, 0)
        content.pack_start(self.stack, True, True, 0)

        for name, title, icon, build in (
                ("dashboard", "Overview", "go-home-symbolic", self.build_dashboard),
                ("blocks", "Blocks", "action-unavailable-symbolic", self.build_blocks),
                ("schedules", "Schedules", "x-office-calendar-symbolic", self.build_schedules_page),
                ("places", "Places", "mark-location-symbolic", self.build_places_page),
                ("hotkeys", "Hotkeys", "input-keyboard-symbolic", self.build_hotkeys_page),
                ("clusters", "Presets", "view-list-symbolic", self.build_clusters),
                ("stats", "Statistics", "x-office-presentation-symbolic", self.build_stats),
                ("circadian", "Circadian rhythm", "weather-clear-night-symbolic", self.build_circadian),
                ("settings", "Settings", "emblem-system-symbolic", self.build_settings)):
            self.stack.add_titled(self.guarded(name, title, build()), name, title)
            row = Gtk.ListBoxRow()
            row.page = name
            line = Gtk.Box(spacing=12)
            line.pack_start(Gtk.Image.new_from_icon_name(icon, Gtk.IconSize.MENU), False, False, 0)
            line.pack_start(Gtk.Label(label=title, xalign=0), True, True, 0)
            row.add(line)
            self.nav.add(row)
            self.nav_rows[name] = row
        self.nav.connect("row-selected", lambda _list, row: row and self.stack.set_visible_child_name(row.page))
        # §4.2: Blocks in the sidebar opens the block list, also from a block.
        self.nav.connect("row-activated", lambda _list, row: row.page == "blocks" and hasattr(self, "blocks_stack")
                         and self.blocks_stack.set_visible_child_name("cards"))
        self.stack.connect("notify::visible-child-name", self.page_changed)
        self.nav.select_row(self.nav_rows["dashboard"])

        self.refresh()
        self.refresh_stats()
        GLib.timeout_add_seconds(REFRESH_SECONDS, self.refresh)
        GLib.timeout_add_seconds(STATS_REFRESH_SECONDS, self.refresh_stats)

    # Running commands ----------------------------------------------------
    def opened_block(self):
        """The block opened on the Blocks page (never a draft), or None."""
        name = getattr(self, "selected", None)
        if (name and name in getattr(self, "blocks", {}) and not self.blocks[name].get("draft_of")
                and self.stack.get_visible_child_name() == "blocks"
                and self.blocks_stack.get_visible_child_name() == "detail"):
            return name
        return None

    def to_draft(self, args, secret, then, timeout):
        """§4.3 (owner 2026-11-17): a change to what the opened block's
        Websites and Apps tabs show goes to its draft; the first one makes
        the draft. Returns True when it took the command over."""
        block = self.opened_block()
        if not block:
            return False
        draft = draft_of_name(block)
        for table, makes in ((DRAFT_EDITS, True), (DRAFT_READS, False)):
            index = draft_position(args, table)
            if index is None or index >= len(args) or args[index] != block:
                continue
            moved = list(args)
            moved[index] = draft
            if draft in self.blocks or self.blocks[block].get("draft") == draft:
                self.run(moved, secret, then, timeout)
            elif makes:
                self.start_draft(None, then=lambda _o: self.run(moved, secret, then, timeout))
            else:
                return False  # a read with no draft: the block itself
            return True
        return False

    def run(self, args, secret=None, then=None, timeout=120):
        """Run a command in the background, then refresh.

        With a settings password set, the password remembered for this
        session goes first on standard input; commands that don't need it
        never read it. A command that needs it and doesn't have it fails,
        and finished() asks for it and runs the command again."""
        if self.to_draft(args, secret, then, timeout):
            return
        self.set_sensitive(False)
        stdin, cli_args = secret, args
        if self.settings_password and self.state and self.state.get("settings_password"):
            # --settings-line: the CLI reads it whether it needs it or not,
            # so it is never taken for the next password (a lock's).
            stdin = self.settings_password + "\n" + (secret or "")
            cli_args = ["--settings-line", *args]
        passwords = list(self.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, cli_args, stdin, timeout)
            GLib.idle_add(self.finished, ok, output, then, (args, secret, timeout))

        threading.Thread(target=work, daemon=True).start()

    def finished(self, ok, output, then, request=None):
        self.set_sensitive(True)
        if not ok and request and ("needs the access password" in output
                                   or "wrong access password" in output):
            # A page's password that wasn't given yet (changing another
            # page's password asks for that one).
            area = output[output.rfind("(") + 1:output.rfind(")")] if "(" in output else ""
            password = self.ask_text(
                "Access password", f"This needs the password for {AREA_TITLES.get(area, area)}.",
                password=True)
            if password and self.try_access(password):
                args, secret, timeout = request
                self.run(args, secret, then, timeout)
            return False
        if not ok and request and ("needs the settings password" in output
                                   or "wrong settings password" in output):
            wrong = "wrong settings password" in output and self.settings_password
            self.settings_password = None
            password = self.ask_text(
                "Settings password",
                "That password was wrong. Try again." if wrong else
                "This change needs the settings password.", password=True)
            if password:
                self.settings_password = password
                args, secret, timeout = request
                self.state = dict(self.state or {}, settings_password=True)
                self.run(args, secret, then, timeout)
            return False
        if not ok:
            self.message(output or "The command failed.")
        elif then:
            then(output)
        self.state_json = None  # repaint even if the state looks unchanged
        self.refresh()
        return False

    def message(self, text, kind=Gtk.MessageType.ERROR):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=text, message_type=kind,
            buttons=Gtk.ButtonsType.OK)
        dialog.run()
        dialog.destroy()

    def confirm(self, text, detail, ok_label="OK", destructive=False):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=text, secondary_text=detail,
            message_type=Gtk.MessageType.WARNING)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button(ok_label, Gtk.ResponseType.OK)
        if destructive:
            ok.get_style_context().add_class("destructive-action")
        answer = dialog.run()
        dialog.destroy()
        return answer == Gtk.ResponseType.OK

    def confirm_lock(self, locks, option=None):
        """R-LOCK-12: the one confirmation for every lock. locks are lines
        "what is locked: kind, how long", as the CLI shows them. Returns True
        only when the user confirms; the caller then passes --yes. With
        option (a label), a tick box is shown too; whether it was ticked is
        left in self.lock_option."""
        self.lock_option = False
        if not locks:
            return True
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.WARNING,
            text="Set this lock?" if len(locks) == 1 else "Set these locks?",
            secondary_text="\n".join("• " + line for line in locks)
            + "\n\nWhile a lock is on, this can't be weakened.")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Lock", Gtk.ResponseType.OK)
        tick = None
        if option:
            tick = Gtk.CheckButton(label=option)
            dialog.get_message_area().pack_start(tick, False, False, 0)
            dialog.show_all()
        answer = dialog.run()
        self.lock_option = bool(tick and tick.get_active())
        dialog.destroy()
        return answer == Gtk.ResponseType.OK

    def acknowledge_no_confirmation(self, what):
        """R-LOCK-12: monk mode started by itself asks nothing, so setting it
        up needs the tick "I know this will not ask for confirmation"."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.WARNING,
            text="Monk mode will start without asking",
            secondary_text=f"{what} starts monk mode by itself, and it will not ask for confirmation "
                           "when it does.")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Set it up", Gtk.ResponseType.OK)
        tick = Gtk.CheckButton(label="I know this will not ask for confirmation")
        ok.set_sensitive(False)
        tick.connect("toggled", lambda t: ok.set_sensitive(t.get_active()))
        dialog.get_message_area().pack_start(tick, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        ticked = tick.get_active()
        dialog.destroy()
        return answer == Gtk.ResponseType.OK and ticked

    def ask_text(self, title, detail="", password=False, initial="", problem=None):
        """Asks for a line of text; "" when cancelled. problem, when given,
        says what is wrong with the text (None: nothing): OK waits and the
        reason shows under the box (R-GUI-1)."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=title, secondary_text=detail or None,
            message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
        entry = Gtk.Entry(activates_default=True, visibility=not password, text=initial)
        dialog.set_default_response(Gtk.ResponseType.OK)
        dialog.get_message_area().pack_start(entry, False, False, 0)
        if problem is not None:
            note = Gtk.Label(xalign=0, wrap=True)
            note.get_style_context().add_class("kc9k-note")
            dialog.get_message_area().pack_start(note, False, False, 0)
            ok = dialog.get_widget_for_response(Gtk.ResponseType.OK)

            def check(*_args):
                why = problem(entry.get_text().strip())
                note.set_text(why or "")
                ok.set_sensitive(why is None)

            entry.connect("changed", check)
            check()
        dialog.show_all()
        answer = dialog.run()
        value = entry.get_text() if password else entry.get_text().strip()
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK else ""

    def ask_duration(self, title, detail, seconds, minimum=60, maximum=None, zero=None,
                     presets=(), why_max=""):
        """R-GUI-1: asks for a length in days, hours and minutes; its
        seconds, or None when cancelled. OK waits until it reads right."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=title, secondary_text=detail,
            message_type=Gtk.MessageType.QUESTION)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        field = DurationField(seconds, minimum=minimum, maximum=maximum, zero=zero, presets=presets,
                              why_max=why_max)
        guard_button(ok, field)
        dialog.get_message_area().pack_start(field, False, False, 0)
        self.duration_field = field  # for tests
        dialog.show_all()
        answer = dialog.run()
        value = field.value
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK and value is not None else None

    def ask_nepsis(self, title, current, allow_lock):
        """Asks for a bypass detection setting, its whole ladder; returns the
        'nepsis set' arguments, or None when cancelled."""
        current = current or {}
        steps = [dict(step) for step in current.get("steps") or []] or [
            {"count": 10, "minutes": 5, "actions": [{"action": "warning"}]}]
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_buttons("Cancel", Gtk.ResponseType.CANCEL, "Save", Gtk.ResponseType.OK)
        dialog.set_default_size(620, 560)
        scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
        area = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10, border_width=16)
        scroll.add(area)
        dialog.get_content_area().pack_start(scroll, True, True, 0)
        intro = dim("A ladder of steps. The first time its number of things is stopped within its "
                    "time, step 1 acts; the next time, step 2; and so on, staying on the last. "
                    "Making it stricter is always allowed; anything else weakens it.")
        intro.set_max_width_chars(70)
        area.pack_start(intro, False, False, 0)
        steps_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        area.pack_start(steps_box, False, False, 0)
        cards, fields = [], []
        save = dialog.get_widget_for_response(Gtk.ResponseType.OK)
        save.get_style_context().add_class("suggested-action")

        def live_fields():
            """The fields that count: every step's numbers and the times of
            the ticked consequences, the quiet time and the lock."""
            out = []
            for _frame, count, within, actions in cards:
                out += [count, within]
                out += [minutes for check, minutes in actions.values() if minutes is not None and check.get_active()]
            return out + [f for f in fields if f is not None]

        def refresh_save(*_args):
            save.set_sensitive(all(f.is_valid() for f in live_fields()))

        def add_card(step):
            frame = Gtk.Frame(label=f"Step {len(cards) + 1}")
            grid = Gtk.Grid(column_spacing=10, row_spacing=4, border_width=8)
            count = NumberField(step.get("count", 3), 1, NEPSIS_MAX_COUNT, width=5)
            within = DurationField(step.get("minutes", 10) * 60, minimum=60, maximum=86400, width=9)
            head = Gtk.Box(spacing=6)
            head.pack_start(count, False, False, 0)
            head.pack_start(Gtk.Label(label="things stopped within", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
            head.pack_start(within, False, False, 0)
            grid.attach(head, 0, 0, 3, 1)
            count.on_change(refresh_save)
            within.on_change(refresh_save)
            chosen = {a["action"]: a.get("minutes", 0) for a in step.get("actions") or []}
            params = {a["action"]: a.get("param", "") for a in step.get("actions") or []}
            actions = {}
            for row, (key, label) in enumerate(CONSEQUENCES, start=1):
                check = Gtk.CheckButton(label=label, active=key in chosen)
                check.connect("toggled", refresh_save)
                grid.attach(check, 0, row, 1, 1)
                minutes = None
                if key in TIMED:
                    minutes = DurationField(
                        (chosen.get(key) or (30 if TIMED[key] else 0)) * 60, minimum=60,
                        maximum=30 * 86400 if key in ("monk", "mode") else 86400,
                        zero=None if TIMED[key] else "Just once", width=9)
                    grid.attach(minutes, 1, row, 1, 1)
                    minutes.on_change(refresh_save)
                if key == "mode":
                    # §3.9 Mode action (owner 2026-11-17): which mode.
                    combo = Gtk.ComboBoxText()
                    for name in self.mode_choices():
                        combo.append_text(name)
                    names = self.mode_choices()
                    if names:
                        combo.set_active(names.index(params.get("mode")) if params.get("mode") in names else 0)
                    grid.attach(combo, 2, row, 1, 1)
                    check.kc9k_param = lambda c=combo: c.get_active_text() or ""
                if key == "custom":
                    # §3.9 Custom action: what it opens or runs.
                    try:
                        items = [f"{list(i)[0]} {list(i.values())[0]}" for i in json.loads(params.get("custom") or "[]")]
                    except (ValueError, IndexError, AttributeError):
                        items = []
                    holder = {"items": items}
                    summary = dim("; ".join(items))
                    change = Gtk.Button(label="Change…")

                    def change_custom(_b, h=holder, s=summary, c=check):
                        got = self.ask_custom("Custom action", h["items"])
                        if got is not None:
                            h["items"] = got
                            s.set_text("; ".join(got))
                            c.set_active(True)
                    change.connect("clicked", change_custom)
                    grid.attach(change, 1, row, 1, 1)
                    grid.attach(summary, 2, row, 1, 1)
                    check.kc9k_param = lambda h=holder: json.dumps(
                        [{item.split(" ", 1)[0]: item.split(" ", 1)[1]} for item in h["items"]])
                actions[key] = (check, minutes)
            frame.add(grid)
            steps_box.pack_start(frame, False, False, 0)
            frame.show_all()
            cards.append((frame, count, within, actions))
            refresh_save()

        for step in steps:
            add_card(step)

        def remove_card(_button):
            if len(cards) > 1:
                frame = cards.pop()[0]
                steps_box.remove(frame)
                refresh_save()

        buttons = Gtk.Box(spacing=6)
        more = Gtk.Button(label="Add a step")
        more.connect("clicked", lambda _b: len(cards) < 10 and add_card(
            {"count": 3, "minutes": 10, "actions": [{"action": "close-browser"}]}))
        fewer = Gtk.Button(label="Remove the last step")
        fewer.connect("clicked", remove_card)
        buttons.pack_start(more, False, False, 0)
        buttons.pack_start(fewer, False, False, 0)
        area.pack_start(buttons, False, False, 0)

        quiet_row = Gtk.Box(spacing=6)
        quiet = DurationField(current.get("quiet_minutes", 0) * 60, minimum=60, maximum=30 * 86400,
                              zero="Never falls back", width=9)
        fields.append(quiet)
        quiet.on_change(refresh_save)
        quiet_row.pack_start(Gtk.Label(label="Fall back a step after quiet time of",
                                       valign=Gtk.Align.START, margin_top=6), False, False, 0)
        quiet_row.pack_start(quiet, False, False, 0)
        area.pack_start(quiet_row, False, False, 0)

        area.pack_start(heading("What counts", "medium"), False, False, 0)
        counted = current.get("detectors") or []
        detector_checks = {}
        for key, label in DETECTORS:
            check = Gtk.CheckButton(label=label, active=not counted or key in counted)
            detector_checks[key] = check
            area.pack_start(check, False, False, 0)
        lock = None
        if allow_lock:
            area.pack_start(heading("Timer lock", "medium"), False, False, 0)
            area.pack_start(dim("Can't be weakened until then."), False, False, 0)
            lock = TimerLockField()
            area.pack_start(lock, False, False, 0)
            fields.append(lock)
            lock.on_change(refresh_save)
        self.nepsis_fields = (cards, quiet, lock)  # for tests
        self.nepsis_lock_words = ""
        refresh_save()
        dialog.show_all()
        answer = dialog.run()
        args, problem = [], ""
        for number, (_frame, count, within, actions) in enumerate(cards, start=1):
            parts = []
            for key, (check, minutes) in actions.items():
                if check.get_active():
                    part = {"action": key}
                    if minutes is not None and minutes.minutes:
                        part["minutes"] = minutes.minutes
                    if hasattr(check, "kc9k_param"):
                        part["param"] = check.kc9k_param()
                        if not part["param"] or part["param"] == "[]":
                            problem = (f"Step {number}: choose the mode it runs." if key == "mode" else
                                       f"Step {number}: say what its custom action opens or runs.")
                    parts.append(part)
            if not parts:
                problem = f"Step {number} does nothing: tick at least one thing for it to do."
            # As JSON (--step-json): what a custom action runs may hold any character.
            args += ["--step-json", json.dumps({"count": count.value, "minutes": within.minutes, "actions": parts})]
        detectors = [key for key, check in detector_checks.items() if check.get_active()]
        if not detectors:
            problem = "Nothing is counted: tick at least one thing under What counts."
        elif len(detectors) < len(DETECTORS):
            args += ["--detect", ",".join(detectors)]
        if quiet.minutes:
            args += ["--quiet", f"{quiet.minutes}m"]
        if lock is not None:
            args += lock.flags()
            self.nepsis_lock_words = lock.describe()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return None
        if problem:
            self.message(problem, Gtk.MessageType.INFO)
            return None
        return args

    def mode_choices(self):
        """The modes a Mode action can run: monk mode, then the modes of
        blocks (Frozen Chicken is Lock the screen / Log out / Shut down)."""
        modes = [m["name"] for m in (self.state or {}).get("modes") or [] if not m.get("freeze")]
        return sorted(modes, key=lambda n: (n != "Monk mode", n.lower()))

    def ask_mode_run(self, title, mode=None, minutes=60, answer=None):
        """§3.9 Mode action: which mode, and for how long it runs, locked.
        Returns (mode, minutes) or None. answer: for tests."""
        if answer is not None:
            return answer
        choices = self.mode_choices()
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        box.set_border_width(14)
        box.pack_start(dim("The mode runs at once for this long, whatever its schedule and places say, and is "
                           "locked until then: it can't be switched off or weakened. Then it is as it was."),
                       False, False, 0)
        combo = Gtk.ComboBoxText()
        for name in choices:
            combo.append_text(name)
        if choices:
            combo.set_active(choices.index(mode) if mode in choices else 0)
        box.pack_start(combo, False, False, 0)
        length = DurationField(max(minutes, 1) * 60, minimum=60, maximum=30 * 86400, presets=(1800, 3600, 4 * 3600),
                               width=9)
        box.pack_start(length, False, False, 0)
        dialog.show_all()
        reply = dialog.run()
        chosen, valid, value = combo.get_active_text(), length.is_valid(), length.minutes
        dialog.destroy()
        if reply != Gtk.ResponseType.OK or not chosen:
            return None
        if not valid:
            self.message(f"How long: {length.problem}", Gtk.MessageType.INFO)
            return None
        return chosen, value

    def ask_custom(self, title, items=(), answer=None):
        """§3.9 Custom action: what it opens or runs. Returns the items
        ("open …"/"run …") or None. answer: for tests."""
        if answer is not None:
            return answer
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        box.set_border_width(14)
        box.pack_start(dim("Opened or run as you, in your desktop session. It can do nothing you couldn't do "
                           "yourself: a KC9K command in it meets every lock and password."), False, False, 0)
        editor = CustomEditor(self, items)
        box.pack_start(editor, False, False, 0)
        dialog.set_default_size(560, -1)
        dialog.show_all()
        reply = dialog.run()
        chosen = editor.items
        dialog.destroy()
        if reply != Gtk.ResponseType.OK:
            return None
        if not chosen:
            self.message("Add something to open or run first.", Gtk.MessageType.INFO)
            return None
        return chosen

    def monk_mode(self, _button):
        seconds = self.ask_duration(
            "Monk mode", "Every website is blocked and the internet connection is cut (Wi-Fi off, "
            "Ethernet down, firewall closed), for this long. It can't be ended early; a running "
            "monk mode can only be extended.",
            3600, minimum=60, maximum=30 * 86400, presets=(1800, 3600, 4 * 3600, 86400))
        if not seconds:
            return
        until = datetime.datetime.now() + datetime.timedelta(seconds=seconds)
        # R-LOCK-12: monk mode is a lock like any other; the command needs --yes.
        if self.confirm_lock([f"monk mode (no websites, no internet): {duration_words(seconds)}, "
                              f"until {date_words(until)}"]):
            self.run(["monk", "--for", f"{seconds // 60}m", "--yes"])

    def change_global_nepsis(self, _button):
        args = self.ask_nepsis("Bypass prevention", self.state.get("nepsis"), True)
        args = self.acknowledge_monk_steps(args)
        if args and ("--for" in args or "--until" in args):
            if not self.confirm_lock([f"bypass prevention: {self.nepsis_lock_words}"]):
                return
            args.append("--yes")
        if args:
            self.run(["bypass", "set", *args], then=self.queued_note)

    def acknowledge_monk_steps(self, args):
        """Bypass prevention steps with monk mode or a Mode action need the
        acknowledgement."""
        steps = [a for i, a in enumerate(args or []) if i and args[i - 1] in ("--step", "--step-json")]
        if any(re.search(r"\bmonk\b", step) or '"action": "mode"' in step for step in steps):
            if not self.acknowledge_no_confirmation("A bypass prevention step"):
                return None
            return args + ["--no-confirm-ok"]
        return args

    def change_block_nepsis(self, _button):
        block = self.blocks.get(self.selected)
        if block:
            args = self.acknowledge_monk_steps(
                self.ask_nepsis(f"Bypass prevention for {block['name']}", block.get("nepsis"), False))
            if args:
                self.run(["bypass", "set", "--block", block["name"], *args], then=self.queued_note)

    def nepsis_off(self, _button, block=None):
        if self.confirm("Turn bypass prevention off?", "This weakens it, so it isn't possible while "
                        "it is locked (or, for a block, while the block is locked).", "Turn off"):
            self.run(["bypass", "off"] + (["--block", block()] if block else []), then=self.queued_note)

    def run_thresholds(self, block_name, list_name, changes):
        """Set each (threshold or hits, action, number) of a list in turn: one
        command at a time."""
        if not changes:
            return
        (command, name, value), rest = changes[0], changes[1:]
        self.run(["list", command, block_name, list_name, name, str(value)],
                 then=lambda output: (self.queued_note(output),
                                      self.run_thresholds(block_name, list_name, rest)))

    def queued_note(self, output):
        """Says so when a change was queued instead of applied."""
        if "queued" in output:
            self.message(output, Gtk.MessageType.INFO)

    def block_cmd(self, command, values):
        if self.selected:
            self.run(["block", command, self.selected, *values])

    # State ---------------------------------------------------------------
    def refresh(self):
        self.lock_when_idle()
        if hasattr(self, "circadian"):
            self.circadian.refresh()
        if self.busy:
            # A read already running may predate the last change: read
            # again when it lands, so the last command is always shown.
            self.refresh_again = True
            return True
        self.busy = True
        self.refresh_again = False

        passwords = list(self.access_passwords)

        reveal = dict(self.block_passwords)

        def work():
            args = ["gui-state"]
            for name in reveal:
                args += ["--reveal", name]
            ok, output = cli_with_access(passwords, args, "".join(p + "\n" for p in reveal.values()) or None)
            GLib.idle_add(self.apply_state, ok, output, passwords)

        threading.Thread(target=work, daemon=True).start()
        return True

    def extension_dialog(self):
        """§4.10: the Killer Chicken extension isn't running: offer to
        install a supported browser, or show the Browsers list."""
        none = not any(b.get("installed") for b in (self.state or {}).get("browsers") or [])
        names = (self.state or {}).get("extension_missing_in") or []
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.WARNING, buttons=Gtk.ButtonsType.NONE,
            text="The Killer Chicken extension isn't running",
            secondary_text=("No supported browser is installed. LibreWolf runs the extension; installing it "
                            "takes a few minutes." if none else
                            f"{' and '.join(names) or 'A supported browser'} has run for a while without it "
                            "checking in, and your blocks need it. Close and open the browser again, or see "
                            "under Settings > Browsers which have it."))
        dialog.add_button("Later", Gtk.ResponseType.CANCEL)
        dialog.add_button("Install LibreWolf" if none else "Show browsers", Gtk.ResponseType.OK)
        answer = dialog.run()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK:
            if none:
                self.run(["browsers", "install", "librewolf"], timeout=900)
            else:
                self.stack.set_visible_child_name("settings")
        return False

    def apply_state(self, ok, output, passwords=()):
        self.busy = False
        if getattr(self, "refresh_again", False):
            # Read before the last change: show the newer state instead.
            self.refresh()
            return False
        if list(passwords) != self.access_passwords:
            # Locked or unlocked while this was being read: read again
            # rather than show what the old passwords opened.
            self.refresh()
            return False
        if not ok:
            self.show_banner(f"Can't read the state: {output}", Gtk.MessageType.ERROR)
            return False
        if output == self.state_json:
            return False
        self.state_json = output
        self.state = json.loads(output)
        tray = getattr(self.get_application(), "tray", None)
        if tray is not None:
            on = [b["name"] for b in self.state.get("blocks", []) if b.get("active")]
            tray.set_status(f"On: {', '.join(on)}" if on else "No block is on")
        # A password that opens nothing any more (it was changed) is dropped.
        for index in self.state.get("access_unused") or []:
            if index < len(passwords) and passwords[index] in self.access_passwords:
                self.access_passwords.remove(passwords[index])
        self.access = self.state.get("access") or {}
        self.update_covers()
        if self.state.get("extension_missing") and not getattr(self, "extension_asked", False):
            self.extension_asked = True  # once per session
            GLib.idle_add(self.extension_dialog)
        self.blocks = {block["name"]: block for block in self.state["blocks"]}
        self.updating = True
        try:
            frozen = self.state["freeze_seconds"]
            if frozen:
                self.show_banner(f"Frozen Chicken is on for {fmt_seconds(frozen)} more.",
                                 Gtk.MessageType.WARNING)
            else:
                self.banner.set_revealed(False)
            for area, update in (("dashboard", self.update_dashboard),
                                 ("blocks", self.update_block_list),
                                 ("clusters", self.update_clusters),
                                 ("schedules", self.update_schedules_page),
                                 ("places", self.update_places_page),
                                 ("device", self.update_device),
                                 ("stats", self.update_stats_setting),
                                 ("settings", self.update_settings),
                                 ("hotkeys", self.update_hotkeys_page)):
                if not self.locked(area):
                    update()
        finally:
            self.updating = False
        return False

    def show_banner(self, text, kind):
        self.banner_label.set_text(text)
        self.banner.set_message_type(kind)
        self.banner.set_revealed(True)

    def build_circadian(self):
        self.circadian = CircadianPage(self)
        return self.circadian


    # Access passwords ----------------------------------------------------
    def guarded(self, name, title, page):
        """The page under a cover that asks for its password while locked."""
        stack = Gtk.Stack()
        stack.add_named(page, "page")
        cover = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12,
                        halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER)
        cover.pack_start(Gtk.Image.new_from_icon_name("changes-prevent-symbolic", Gtk.IconSize.DIALOG),
                         False, False, 0)
        text = Gtk.Label(justify=Gtk.Justification.CENTER)
        cover.pack_start(text, False, False, 0)
        row = Gtk.Box(spacing=6)
        entry = Gtk.Entry(visibility=False, placeholder_text="Password", width_chars=24,
                          input_purpose=Gtk.InputPurpose.PASSWORD)
        button = Gtk.Button(label="Unlock")
        button.get_style_context().add_class("suggested-action")
        unlock = lambda *_args: self.unlock_from(entry)
        entry.connect("activate", unlock)
        button.connect("clicked", unlock)
        row.pack_start(entry, True, True, 0)
        row.pack_start(button, False, False, 0)
        cover.pack_start(row, False, False, 0)
        stack.add_named(cover, "cover")
        self.covers[name] = (stack, text, entry, title)
        return stack

    def locked(self, area):
        # The Clusters page belongs to the Blocks area.
        return self.access.get({"clusters": "blocks", "schedules": "blocks", "places": "blocks",
                                "circadian": "settings", "hotkeys": "settings"}.get(area, area)) == "locked"

    def update_covers(self):
        app_locked = self.access.get("app") == "locked"
        for name, (stack, text, _entry, title) in self.covers.items():
            if app_locked:
                text.set_markup("<b>Killer Chicken 9000 is locked.</b>\nEnter its password to open it.")
            else:
                text.set_markup(f"<b>{title} is locked.</b>\nEnter its password to open it.")
            stack.set_visible_child_name("cover" if self.locked(name) else "page")
        self.lock_button.set_visible(bool(self.access_passwords))
        if hasattr(self, "access_status"):
            self.update_access_section()

    def unlock_from(self, entry):
        password = entry.get_text()
        entry.set_text("")
        if password and not self.try_access(password):
            self.message("That password opens nothing.")
            entry.grab_focus()

    def try_access(self, password):
        """Keeps a password if it opens something; False if it doesn't."""
        if password in self.access_passwords:
            return True
        passwords = self.access_passwords + [password]
        ok, output = cli_with_access(passwords, ["gui-state"])
        if not ok:
            return False
        if len(passwords) - 1 in (json.loads(output).get("access_unused") or []):
            return False
        self.access_passwords = passwords
        self.state_json = None
        self.apply_state(True, output, passwords)
        self.refresh_stats()
        return True

    def note_input(self, _widget, event):
        if event.type in (Gdk.EventType.KEY_PRESS, Gdk.EventType.BUTTON_PRESS, Gdk.EventType.SCROLL,
                          Gdk.EventType.MOTION_NOTIFY):
            self.last_input = GLib.get_monotonic_time()

    def lock_when_idle(self):
        """Forgets the access passwords after the idle minutes chosen under
        Settings (0: never)."""
        minutes = self.get_application().prefs.get("access_idle_minutes", 0)
        idle = (GLib.get_monotonic_time() - self.last_input) / 60e6
        if minutes and self.access_passwords and idle >= minutes:
            self.lock_access()

    def lock_access(self, _button=None):
        self.access_passwords = []
        self.block_passwords = {}
        self.state_json = None
        self.refresh()

    def access_dialog(self, remove):
        """Asks which areas (and, to set, which password). Returns (areas, password)."""
        dialog = Gtk.Dialog(title="Remove access passwords" if remove else "Set an access password",
                            transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Remove" if remove else "Set", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("destructive-action" if remove else "suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        for side in ("margin_start", "margin_end", "margin_top", "margin_bottom"):
            box.set_property(side, 16)
        box.pack_start(dim("Remove the password of:" if remove else
                           "Everything ticked gets this one password. Do it again with "
                           "other boxes ticked for a different one."), False, False, 0)
        checks = {}
        for area in AREAS:
            has = self.access.get(area, "none") != "none"
            label = ("Opening the program" if area == "app" else AREA_TITLES[area])
            check = Gtk.CheckButton(label=label + (" (has a password)" if has and not remove else ""))
            if remove:
                check.set_sensitive(has)
            checks[area] = check
            box.pack_start(check, False, False, 0)
        first = Gtk.Entry(visibility=False, placeholder_text="Password (at least 4 characters)",
                          activates_default=True)
        again = Gtk.Entry(visibility=False, placeholder_text="Repeat it", activates_default=True)
        if not remove:
            box.pack_start(first, False, False, 0)
            box.pack_start(again, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        areas = [area for area, check in checks.items() if check.get_active()]
        password, repeat = first.get_text(), again.get_text()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or not areas:
            return None, None
        if not remove and password != repeat:
            self.message("The two passwords differ; nothing changed.", Gtk.MessageType.INFO)
            return None, None
        if not remove and len(password) < 4:
            self.message("Use at least 4 characters.", Gtk.MessageType.INFO)
            return None, None
        return areas, password

    def set_access_password(self, _button):
        areas, password = self.access_dialog(remove=False)
        if areas:
            # Remembered at once, so the page being worked on doesn't lock.
            self.run(["access", "set", *areas], password + "\n",
                     then=lambda _o: self.access_passwords.append(password)
                     if password not in self.access_passwords else None)

    def remove_access_password(self, _button):
        areas, _password = self.access_dialog(remove=True)
        if areas:
            self.run(["access", "remove", *areas])

    def update_access_section(self):
        with_password = [("Opening the program" if a == "app" else AREA_TITLES[a])
                         for a in AREAS if self.access.get(a, "none") != "none"]
        if with_password:
            self.access_status.set_markup("<b>On for:</b> " + GLib.markup_escape_text(", ".join(with_password)) + ".")
        else:
            self.access_status.set_text("Not set.")
        self.access_remove_button.set_visible(bool(with_password))
