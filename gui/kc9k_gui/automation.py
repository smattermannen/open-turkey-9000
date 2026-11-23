# Automation editors: triggers, custom actions, place rules, schedules.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class TriggerEditor(Gtk.Box):
    """What a block's apps or window titles do when found (owner,
    2026-11-16: any trigger to any action): their own action (close the
    app or the window) and the consequences, each with its time. (A place
    rule has no actions, §3.6.) Ticking runs
    'kc9k block trigger', as does Enter in a time field while its action is
    on. set_actions shows what the state says."""

    CONSEQUENCES = (("close-browser", "Close the browsers", None),
                    ("lock-screen", "Lock the screen for", 30 * 60),
                    ("logout", "Log out", 0), ("shutdown", "Shut down", 0),
                    ("monk", "Start monk mode for", 60 * 60),
                    ("mode", "Run a mode, locked, for", 60 * 60),
                    ("custom", "Custom action", None))

    def __init__(self, window, group, own, own_label, intro):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        self.window, self.group, self.own = window, group, own
        self.block = None
        self.filling = False
        self.pack_start(dim(intro), False, False, 0)
        self.checks, self.times = {}, {}
        rows = [(own, own_label, None)] + list(self.CONSEQUENCES)
        for name, label, seconds in rows:
            row = Gtk.Box(spacing=6)
            check = Gtk.CheckButton(label=label)
            check.connect("toggled", self.toggled, name)
            row.pack_start(check, False, False, 0)
            self.checks[name] = check
            if seconds is not None:
                once = name in ("logout", "shutdown")
                field = DurationField(seconds, minimum=60, zero="once" if once else None,
                                      maximum=30 * 86400 if name in ("monk", "mode") else 86400, width=8)
                field.entry.connect("activate", lambda _e, n=name: self.time_changed(n))
                row.pack_start(field, False, False, 0)
                self.times[name] = field
            if name == "mode":
                # §3.9 Mode action: which mode (owner 2026-11-17).
                self.mode_combo = Gtk.ComboBoxText(valign=Gtk.Align.START)
                self.mode_combo.connect("changed", lambda _c: self.filling or self.time_changed("mode"))
                row.pack_start(self.mode_combo, False, False, 0)
            if name == "custom":
                # §3.9 Custom action: what it opens or runs.
                self.custom_items = []
                self.custom_label = dim("")
                change = Gtk.Button(label="Change…", valign=Gtk.Align.START)
                change.connect("clicked", lambda _b: self.change_custom())
                row.pack_start(change, False, False, 0)
                row.pack_start(self.custom_label, False, False, 0)
            self.pack_start(row, False, False, 0)

    def set_actions(self, block, actions):
        self.block = block
        self.filling = True
        try:
            have = {a["name"]: a for a in actions or []}
            for name, check in self.checks.items():
                check.set_active(name in have)
                if name in self.times and name in have and not self.times[name].has_focus():
                    self.times[name].set_seconds(have[name].get("minutes", 0) * 60)
            choices = self.window.mode_choices()
            chosen = (have.get("mode") or {}).get("param") or self.mode_combo.get_active_text()
            self.mode_combo.remove_all()
            for m in choices:
                self.mode_combo.append_text(m)
            if choices:
                self.mode_combo.set_active(choices.index(chosen) if chosen in choices else 0)
            if "custom" in have:
                try:
                    self.custom_items = [f"{list(i)[0]} {list(i.values())[0]}"
                                         for i in json.loads(have["custom"].get("param") or "[]")]
                except (ValueError, IndexError, AttributeError):
                    self.custom_items = []
            self.custom_label.set_text("; ".join(self.custom_items))
        finally:
            self.filling = False

    def change_custom(self):
        items = self.window.ask_custom("Custom action", self.custom_items)
        if items is None:
            return
        self.custom_items = items
        self.custom_label.set_text("; ".join(items))
        if self.block and self.checks["custom"].get_active():
            args = self.command("custom", True)
            if args:
                self.window.run(args)

    def command(self, name, on):
        args = ["block", "trigger", self.block, self.group, name, "on" if on else "off"]
        field = self.times.get(name)
        if on and field is not None:
            if not field.is_valid():
                self.window.message(f"{self.checks[name].get_label()}: {field.problem}", Gtk.MessageType.INFO)
                return None
            if field.value:
                args += ["--for", field.arg()]
        if on and name == "mode":
            if not self.mode_combo.get_active_text():
                self.window.message("Make a mode first: a Mode action runs one.", Gtk.MessageType.INFO)
                return None
            args += ["--target", self.mode_combo.get_active_text()]
        if on and name == "custom":
            if not self.custom_items:
                items = self.window.ask_custom("Custom action", [])
                if not items:
                    return None
                self.custom_items = items
            args += [a for item in self.custom_items for a in ("--custom", item)]
        if on and name in ("monk", "mode"):
            if not self.window.acknowledge_no_confirmation("This trigger"):
                return None
            args.append("--no-confirm-ok")
        return args

    def toggled(self, check, name):
        if self.filling or not self.block:
            return
        args = self.command(name, check.get_active())
        if args is None:
            self.filling = True
            check.set_active(not check.get_active())
            self.filling = False
            return
        self.window.run(args)
        self.reload_later()

    def reload_later(self):
        """Shows the state again once the command and the refresh are done
        (a refused change goes back)."""
        def check():
            if not self.window.get_sensitive() or getattr(self.window, "busy", False):
                return True
            block = self.window.blocks.get(self.block) or {}
            actions = (block.get("triggers") or {}).get(self.group)
            if actions is not None:
                self.set_actions(self.block, actions)
            return False
        GLib.timeout_add(150, check)

    def time_changed(self, name):
        if self.block and self.checks[name].get_active():
            args = self.command(name, True)
            if args:
                self.window.run(args)


def window_words(w):
    """A window as the CLI writes it: days, from, to."""
    names = [DAY_NAMES[day] for day in range(7) if w["days"] & (1 << ((day + 1) % 7))]  # Monday first
    return names, clock_text(w["start"]), "24:00" if w["end"] >= 1440 else clock_text(w["end"])


def schedule_summary(windows):
    """A schedule's windows in a line, and its hours a week."""
    covered = set()
    for w in windows:
        for day in range(7):
            if w["days"] & (1 << day):
                covered.update((day, minute) for minute in range(w["start"], w["end"]))
    parts = []
    for w in windows[:3]:
        names, frm, to = window_words(w)
        days = ",".join(names) if len(names) < 7 else "daily"
        parts.append(f"{days.replace('mon,tue,wed,thu,fri', 'weekdays').replace('sat,sun', 'weekends')} "
                     f"{show_clock(w['start'])}–{show_clock(w['end'], True)}")
    if len(windows) > 3:
        parts.append(f"{len(windows) - 3} more")
    hours = f"{len(covered) / 60:.0f} h a week" if covered else "no windows yet"
    return (", ".join(parts) + " · " if parts else "") + hours


class CustomEditor(Gtk.Box):
    """§3.9 Custom action (owner 2026-11-17): what to open or run, one row
    each with Remove, and a row to add one: open a site, file or folder, or
    run a program or command. Items are "open …" / "run …" as the CLI takes
    them. on_change(items), when given, runs after each change."""

    KINDS = (("open", "Open a site, file or folder"), ("run", "Run a program or command"))

    def __init__(self, window, items=(), on_change=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.window, self.on_change = window, on_change
        self.rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.rows.get_style_context().add_class("frame")
        self.rows.set_no_show_all(True)
        self.pack_start(self.rows, False, False, 0)
        add = Gtk.Box(spacing=6)
        self.kind = Gtk.ComboBoxText()
        for key, label in self.KINDS:
            self.kind.append(key, label)
        self.kind.set_active_id("open")
        self.kind.connect("changed", lambda _c: self.entry.set_placeholder_text(
            "https://… or ~/Documents/notes.odt" if self.kind.get_active_id() == "open" else "libreoffice --writer"))
        self.entry = Gtk.Entry(placeholder_text="https://… or ~/Documents/notes.odt", width_chars=28)
        self.entry.connect("activate", lambda _e: self.add_item())
        choose = Gtk.Button(label="Choose…", tooltip_text="Choose a file or folder to open")
        choose.connect("clicked", lambda _b: self.choose())
        button = Gtk.Button(label="Add")
        button.connect("clicked", lambda _b: self.add_item())
        for widget in (self.kind, self.entry, choose, button):
            add.pack_start(widget, widget is self.entry, widget is self.entry, 0)
        self.pack_start(flow_row(add), False, False, 0)
        self.items = []
        self.set_items(items)

    def set_items(self, items):
        self.items = list(items)
        for row in self.rows.get_children():
            self.rows.remove(row)
        for number, item in enumerate(self.items):
            verb, _sep, what = item.partition(" ")
            line = Gtk.Box(spacing=8, margin=6)
            line.pack_start(Gtk.Label(label="Open" if verb == "open" else "Run", xalign=0), False, False, 0)
            line.pack_start(Gtk.Label(label=what, xalign=0, ellipsize=Pango.EllipsizeMode.MIDDLE, selectable=True),
                            True, True, 0)
            remove = Gtk.Button(label="Remove", relief=Gtk.ReliefStyle.NONE)
            remove.connect("clicked", lambda _b, n=number: self.remove_item(n))
            line.pack_start(remove, False, False, 0)
            self.rows.add(line)
        self.rows.set_visible(bool(self.items))
        self.rows.show_all()

    def add_item(self, text=None):
        what = (text if text is not None else self.entry.get_text()).strip()
        if not what or "\n" in what:
            return
        self.entry.set_text("")
        self.set_items(self.items + [f"{self.kind.get_active_id()} {what}"])
        if self.on_change:
            self.on_change(self.items)

    def remove_item(self, number):
        self.set_items(self.items[:number] + self.items[number + 1:])
        if self.on_change:
            self.on_change(self.items)

    def choose(self):
        path = self.window.choose_file("Choose a file or folder to open", Gtk.FileChooserAction.OPEN, "Choose")
        if path:
            self.kind.set_active_id("open")
            self.add_item(path)


def place_effect(subject, default_on, need_both, has_windows):
    """What arriving at a place added to an automation does (§3.6), said
    plainly: subject is "this block", "its blocks" or "monk mode"."""
    state = "off" if default_on else "on"
    if need_both and has_windows:
        return f"Here during one of its windows, {subject} {'are' if subject == 'its blocks' else 'is'} {state}."
    return f"Arriving here turns {subject} {state}."


class PlaceRules(Gtk.Box):
    """§3.6, §4.5 (owner 2026-11-17): the places in an automation, a row
    each saying plainly that it is added, whether arriving there turns the
    block (a mode's blocks, monk mode) on or off, and whether the computer is
    there now; below, a saved place to add and, where given, Save where I am
    now as…. add(place) and remove(place) run the commands."""

    def __init__(self, window, add, remove, save_here=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.window, self.add_place, self.remove_place = window, add, remove
        self.rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.rows.get_style_context().add_class("frame")
        self.rows.set_no_show_all(True)
        self.pack_start(self.rows, False, False, 0)
        self.empty = dim("")
        self.pack_start(self.empty, False, False, 0)
        pick = Gtk.Box(spacing=6)
        self.combo = Gtk.ComboBoxText(tooltip_text="A saved place (Places page)")
        pick.pack_start(self.combo, False, False, 0)
        self.add_button = Gtk.Button(label="Add this place")
        self.add_button.connect("clicked", lambda _b: self.combo.get_active_text() and self.add_place(
            self.combo.get_active_text()))
        pick.pack_start(self.add_button, False, False, 0)
        if save_here:
            here = Gtk.Button(label="Save where I am now as…")
            here.connect("clicked", lambda _b: save_here())
            pick.pack_start(here, False, False, 0)
        self.pick = flow_row(pick)
        self.pack_start(self.pick, False, False, 0)
        self.names = []  # the places shown as added (for tests)
        self.shown = None

    def show_places(self, added, subject, default_on=False, need_both=False, has_windows=False):
        places = {p["name"]: p for p in (self.window.state or {}).get("places") or []}
        effect = place_effect(subject, default_on, need_both, has_windows)
        key = (list(added), effect, [(n, p.get("here")) for n, p in places.items()])
        if key != self.shown:
            self.shown = key
            self.names = list(added)
            for row in self.rows.get_children():
                self.rows.remove(row)
            for name in added:
                line = Gtk.Box(spacing=10, margin=8)
                line.pack_start(Gtk.Image.new_from_icon_name("emblem-ok-symbolic", Gtk.IconSize.MENU),
                                False, False, 0)
                text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                title = Gtk.Box(spacing=8)
                label = Gtk.Label(label=name, xalign=0)
                label.get_style_context().add_class("kc9k-card-name")
                title.pack_start(label, False, False, 0)
                here = (places.get(name) or {}).get("here")
                title.pack_start(pill("You are here" if here else "Not here now", "on" if here else "off"),
                                 False, False, 0)
                text.pack_start(title, False, False, 0)
                text.pack_start(dim("Added. " + effect), False, False, 0)
                line.pack_start(text, True, True, 0)
                remove = Gtk.Button(label="Remove", valign=Gtk.Align.CENTER)
                remove.set_tooltip_text("Take this place out of the automation")
                remove.connect("clicked", lambda _b, p=name: self.remove_place(p))
                line.pack_start(remove, False, False, 0)
                row = Gtk.ListBoxRow()
                row.add(line)
                row.place_name = name
                self.rows.add(row)
            self.rows.set_visible(bool(added))
            self.rows.show_all()
        self.empty.set_text("" if added else
                            "No places added. A place added here switches it by itself: " + effect.lower()
                            .replace("arriving here", "arriving at the place") if places else
                            "No places saved yet: save where the computer is now.")
        self.empty.set_visible(not added)
        saved = [n for n in places if n not in added]
        chosen = getattr(self, "to_choose", None) or self.combo.get_active_text()
        self.to_choose = None
        self.combo.remove_all()
        for n in saved:
            self.combo.append_text(n)
        if saved:
            self.combo.set_active(saved.index(chosen) if chosen in saved else 0)
        self.combo.set_sensitive(bool(saved))
        self.add_button.set_sensitive(bool(saved))


class AutomationEditor(Gtk.Box):
    """§3.6, §4.5 (owner 2026-11-17): the automation of a mode (one for all
    its blocks), of monk mode, or the windows of a schedule kept outside
    blocks. Every change runs the commands a block's does (schedule add and
    remove, schedule apply and take-off, place rule, automation) with the
    mode's or schedule's name; the CLI decides (R-MODE-2, R-MONK-3)."""

    def __init__(self, window, kind):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        self.window, self.kind, self.name, self.data = window, kind, None, {}
        self.summary = Gtk.Label(xalign=0, wrap=True)
        self.pack_start(self.summary, False, False, 0)
        self.buttons = {}
        self.updating = False

        def choice(pairs, setting):
            row, group = Gtk.Box(), None
            row.get_style_context().add_class("linked")
            for value, label in pairs:
                button = Gtk.RadioButton.new_with_label_from_widget(group, label)
                button.set_mode(False)
                group = group or button
                button.connect("toggled", self.set_setting, setting, value)
                row.pack_start(button, False, False, 0)
                self.buttons[(setting, value)] = button
            return row

        if kind == "mode":
            self.pack_start(choice((("disabled", "Disabled by default"), ("enabled", "Enabled by default")),
                                   "default"), False, False, 0)
        self.need_row = Gtk.Box(spacing=8)
        if kind != "schedule":
            self.need_row.pack_start(Gtk.Label(label="With windows and places:"), False, False, 0)
            self.need_row.pack_start(choice((("either", "Either is enough"), ("both", "Both at once")), "needs"),
                                     False, False, 0)
            for child in self.need_row.get_children():
                child.show_all()
            self.need_row.set_no_show_all(True)
            self.pack_start(self.need_row, False, False, 0)
        self.calendar = WeekCalendar(self.create_window, self.edit_window, self.move_window)
        intro = Gtk.Box(spacing=6)
        intro.pack_start(dim("Drag down a day to add a window; click one to change or remove it; drag it "
                             "to move it."), False, False, 0)
        self.pack_start(intro, False, False, 0)
        self.pack_start(self.calendar, False, False, 0)
        presets = Gtk.Box(spacing=6)
        presets.pack_start(Gtk.Label(label="Add:"), False, False, 0)
        for label, days, start, end in SCHEDULE_PRESETS:
            button = Gtk.Button(label=label, tooltip_text=f"{days} {start}-{end}")
            button.connect("clicked", lambda _b, d=days, f=start, t=end: self.add_window(d, f, t))
            presets.pack_start(button, False, False, 0)
        self.total = dim("")
        presets.pack_end(self.total, False, False, 0)
        self.pack_start(flow_row(presets), False, False, 0)
        self.applied = Gtk.Box(spacing=6)
        if kind != "schedule":
            row = Gtk.Box(spacing=6)
            row.pack_start(Gtk.Label(label="Schedules:"), False, False, 0)
            row.pack_start(self.applied, False, False, 0)
            apply = Gtk.Button(label="Apply schedule…")
            apply.set_tooltip_text("Evening, Weekend…: made on the Schedules page; changing it there changes this too")
            apply.connect("clicked", self.apply_schedule)
            row.pack_start(apply, False, False, 0)
            self.pack_start(flow_row(row), False, False, 0)
            self.pack_start(heading("Places", "medium"), False, False, 0)
            self.places = PlaceRules(window, self.add_named_place,
                                     lambda p: self.run(["place", "rule", self.name, p, "off"]),
                                     lambda: window.save_place(None, target=self.name, flags=self.flags()))
            self.place_combo = self.places.combo
            self.pack_start(self.places, False, False, 0)

    # The commands -----------------------------------------------------
    def monk_ok(self, what):
        """R-LOCK-12: adding to monk mode's time while it is on asks first;
        the command then takes --yes. Others need nothing."""
        if self.kind != "monk" or not self.data.get("active"):
            return []
        if self.window.confirm_lock([f"monk mode (no websites, no internet, can't be switched off while it "
                                     f"runs): also {what}"]):
            return ["--yes"]
        return None

    def flags(self):
        """§4.2 Names (owner 2026-11-17): a mode or schedule may share a
        block's name, so its commands say which is meant."""
        return {"mode": ["--mode"], "monk": ["--mode"], "schedule": ["--schedule"]}.get(self.kind, [])

    def run(self, args, then=None):
        self.window.run(args + self.flags(), then=then)

    def add_window(self, days, frm, to):
        if not self.name:
            return
        yes = self.monk_ok(f"{days} {frm}–{to}")
        if yes is None:
            return
        flags = ["--allow"] if self.kind == "mode" and (self.data.get("automation") or {}).get("default_on") else []
        self.run(["schedule", "add", self.name, "--days", days, "--from", frm, "--to", to, *flags, *yes])

    def create_window(self, column, start, end):
        day = DAY_NAMES[column_day(column)]
        if self.window.confirm(f"Add {day.capitalize()} {show_clock(start)}–{show_clock(end, True)}?",
                               "Tick more days afterwards by clicking it.", "Add"):
            self.add_window(day, clock_text(start), "24:00" if end >= 1440 else clock_text(end))

    def from_schedule(self, w):
        """A window from a schedule is changed on the Schedules page."""
        if self.window.confirm(f"This window comes from the schedule {w['source']}",
                               "Change it on the Schedules page (that changes everything using it), or take "
                               "the schedule off here.", "Open Schedules"):
            self.window.open_schedule(w["source"])

    def windows(self):
        return self.data.get("windows") or []

    def move_window(self, index, days, start, end):
        windows = self.windows()
        if index >= len(windows):
            return
        if windows[index].get("source"):
            self.from_schedule(windows[index])
            return
        self.replace(index, {"days": days, "start": start, "end": end})

    def replace(self, index, w):
        """Removes window index, then adds w (a change that weakens is
        refused by the CLI, and the old one stays)."""
        names, frm, to = window_words(w)
        if not names:
            return
        yes = self.monk_ok(f"{','.join(names)} {frm}–{to}")
        if yes is None:
            return
        flags = ["--allow"] if self.kind == "mode" and (self.data.get("automation") or {}).get("default_on") else []
        add = ["schedule", "add", self.name, "--days", ",".join(names), "--from", frm, "--to", to, *flags, *yes]
        self.run(["schedule", "remove", self.name, str(index + 1)], then=lambda _o: self.run(add))

    def edit_window(self, index):
        windows = self.windows()
        if index >= len(windows):
            return
        w = windows[index]
        if w.get("source"):
            self.from_schedule(w)
            return
        dialog = Gtk.Dialog(title="Window", transient_for=self.window, modal=True)
        dialog.add_button("Remove", 3)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Save", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        days_row, checks = Gtk.Box(spacing=6), {}
        for column in range(7):
            day = column_day(column)
            bit = 1 << ((day + 1) % 7)
            check = Gtk.CheckButton(label=DAY_NAMES[day].capitalize(), active=bool(w["days"] & bit))
            checks[bit] = check
            days_row.pack_start(check, False, False, 0)
        box.pack_start(dim("Tick more days to copy the window to them."), False, False, 0)
        box.pack_start(days_row, False, False, 0)
        times = Gtk.Box(spacing=6)
        start = Gtk.Entry(text=clock_text(w["start"]), width_chars=5)
        end = Gtk.Entry(text="24:00" if w["end"] >= 1440 else clock_text(w["end"]), width_chars=5)
        for widget in (Gtk.Label(label="From"), start, Gtk.Label(label="to"), end):
            times.pack_start(widget, False, False, 0)
        box.pack_start(times, False, False, 0)
        self.dialog = (checks, start, end)  # for tests
        dialog.show_all()
        answer = dialog.run()
        days = sum(bit for bit, check in checks.items() if check.get_active())
        frm, to = start.get_text().strip(), end.get_text().strip()
        dialog.destroy()
        if answer == 3:
            if self.window.confirm("Remove this window?", "A change that weakens a locked block is refused.",
                                   "Remove", destructive=True):
                self.run(["schedule", "remove", self.name, str(index + 1)])
            return
        if answer != Gtk.ResponseType.OK:
            return
        new_start, new_end = parse_clock(frm), (1440 if to == "24:00" else parse_clock(to))
        if new_start is None or new_end is None or new_start >= new_end:
            self.window.message("Times are HH:MM, the start before the end, e.g. 09:00 and 17:00.",
                                Gtk.MessageType.INFO)
            return
        if (days, new_start, new_end) != (w["days"], w["start"], w["end"]):
            self.replace(index, {"days": days, "start": new_start, "end": new_end})

    def set_setting(self, button, setting, value):
        if self.updating or not button.get_active() or not self.name:
            return
        self.run(["automation", self.name, setting, value])

    def apply_schedule(self, _button):
        have = set(self.data.get("schedules") or [])
        names = [s["name"] for s in self.window.state.get("schedules") or [] if s["name"] not in have]
        if not names:
            if self.window.confirm("No schedule to apply", "Make schedules such as Evening or Weekend on the "
                                   "Schedules page, then apply them here.", "Open Schedules"):
                self.window.stack.set_visible_child_name("schedules")
            return
        name = self.window.choose("Apply a schedule", "Its windows join these; changing it on the Schedules "
                                  "page changes them too.", names)
        if not name:
            return
        yes = self.monk_ok(f"the schedule {name}")
        if yes is not None:
            self.run(["schedule", "apply", self.name, name, *yes])

    def add_place(self, _button):
        self.add_named_place(self.place_combo.get_active_text())

    def add_named_place(self, place):
        if not place or not self.name:
            return
        yes = self.monk_ok(f"while the computer is at {place}")
        if yes is not None:
            self.run(["place", "rule", self.name, place, "on", *yes])

    def chip(self, text, tooltip, remove):
        chip = Gtk.Box(spacing=2)
        chip.get_style_context().add_class("kc9k-pill")
        chip.get_style_context().add_class("on")
        chip.pack_start(Gtk.Label(label=text), False, False, 0)
        x = Gtk.Button.new_from_icon_name("window-close-symbolic", Gtk.IconSize.MENU)
        x.set_relief(Gtk.ReliefStyle.NONE)
        x.set_tooltip_text(tooltip)
        x.connect("clicked", lambda _b: remove())
        chip.pack_start(x, False, False, 0)
        return chip

    # Showing it ---------------------------------------------------------
    def show_data(self, name, data):
        """data: a mode or schedule from gui-state."""
        self.name, self.data = name, data or {}
        windows = self.windows()
        self.calendar.set_windows(windows)
        self.total.set_text(schedule_summary(windows).split(" · ")[-1])
        settings = self.data.get("automation") or {}
        places = self.data.get("places") or []
        if self.kind == "schedule":
            used = (self.data.get("blocks") or []) + [f"mode {m}" for m in self.data.get("modes") or []]
            self.summary.set_text("Used by " + ", ".join(used) + ": a change here changes them too; one that "
                                  "weakens a locked block is refused." if used else
                                  "Used by nothing yet: apply it to blocks and modes in their Automation.")
        elif self.data.get("continuous"):
            self.summary.set_text("No windows or places yet: add some below." if self.kind == "monk" else
                                  "Continuous: its blocks are enabled whenever they are on. Add windows or "
                                  "places below to switch them all by themselves.")
        else:
            by = " and ".join(p for p, has in (("its windows", windows), ("its places", places)) if has)
            if self.kind == "monk":
                self.summary.set_text(f"Monk mode runs while {by} have it on, once switched on.")
            else:
                self.summary.set_text(f"Automated: {by} turn its blocks {'off' if settings.get('default_on') else 'on'}.")
        self.updating = True
        if ("default", "enabled") in self.buttons:
            self.buttons[("default", "enabled" if settings.get("default_on") else "disabled")].set_active(True)
        if ("needs", "both") in self.buttons:
            self.buttons[("needs", "both" if settings.get("need_both") else "either")].set_active(True)
        self.updating = False
        if self.kind == "schedule":
            return
        self.need_row.set_visible(bool(windows) and bool(places))
        for child in self.applied.get_children():
            self.applied.remove(child)
        for sched in self.data.get("schedules") or []:
            self.applied.pack_start(self.chip(sched, "Take this schedule off",
                                              lambda s=sched: self.run(["schedule", "take-off", self.name, s])),
                                    False, False, 0)
        if not self.data.get("schedules"):
            self.applied.pack_start(dim("none"), False, False, 0)
        self.applied.show_all()
        self.places.show_places(places, "monk mode" if self.kind == "monk" else "its blocks",
                                bool(settings.get("default_on")), bool(settings.get("need_both")), bool(windows))
