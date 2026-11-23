# Window: block actions: create, lock, start, stop, breaks, schedules, import, export.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _BlockActions:
    """Methods of Window (window.py); no state of its own."""

    # Block actions -------------------------------------------------------
    def create_block(self, _button):
        templates = (self.state or {}).get("templates") or []
        template = None
        if templates:
            # §3.2: start from one of your block presets, or empty.
            dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                       buttons=Gtk.ButtonsType.OK_CANCEL, text="New block",
                                       secondary_text="Start empty or from a block preset.")
            combo = Gtk.ComboBoxText()
            combo.append("", "Empty")
            for name in templates:
                combo.append(name, "Block preset: " + name)
            combo.set_active(0)
            dialog.get_message_area().pack_start(combo, False, False, 0)
            dialog.show_all()
            answer = dialog.run()
            template = combo.get_active_id()
            dialog.destroy()
            if answer != Gtk.ResponseType.OK:
                return
        name = self.ask_text("New block", "A short name, such as social-media.")
        if name:
            self.selected = name
            self.blocks_stack.set_visible_child_name("detail")
            self.stack.set_visible_child_name("blocks")
            self.run(["block", "create", name] + (["--preset", template] if template else []))

    def save_template(self, _button):
        if not self.selected:
            return
        name = self.ask_text("Save as block preset", "A name for a copy of this block, to make new blocks from.",
                             initial=self.selected)
        if name:
            self.run(["block-preset", "save", self.selected, name])

    def choose_block_users(self, _button, answer=None):
        """§3.2: tick the accounts the block applies to; all ticked (or
        none) means every account. answer: the ticked names, for tests."""
        block = self.blocks.get(self.selected)
        if not block:
            return
        accounts = self.state.get("accounts") or []
        current = block.get("users") or []
        if answer is None:
            dialog = Gtk.MessageDialog(
                transient_for=self, modal=True, text=f"Who does {block['name']} apply to?",
                secondary_text="For some accounts only, its websites are blocked in their browsers by the "
                               "extension and its apps closed in their sessions. Fewer accounts weakens the "
                               "block, so it isn't possible while it is locked.",
                message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
            checks = {}
            for name in accounts:
                checks[name] = Gtk.CheckButton(label=name, active=not current or name in current)
                dialog.get_message_area().pack_start(checks[name], False, False, 0)
            dialog.show_all()
            ok = dialog.run() == Gtk.ResponseType.OK
            answer = [name for name, check in checks.items() if check.get_active()]
            dialog.destroy()
            if not ok:
                return
        if not answer or set(answer) >= set(accounts):
            self.run(["block", "users", block["name"], "all"])
        else:
            self.run(["block", "users", block["name"], *answer])

    def rename_block(self, _button):
        name = self.ask_text("Rename block", initial=self.selected or "")
        if self.selected and name and name != self.selected:
            old, self.selected = self.selected, name
            self.run(["block", "rename", old, name])

    def duplicate_block(self, _button):
        name = self.ask_text("Duplicate block", "Name of the copy.")
        if self.selected and name:
            self.run(["block", "duplicate", self.selected, name])

    def lock_dialog(self, title, intro, button, picker, extra=None, need_lock=False):
        """A dialog around a LockPicker; None when cancelled, else what it
        asks for, read before the dialog goes: (locks as (flags, words),
        re-lock flags, password). extra, when given, is more fields above
        the locks (on_change, problem()). The button waits until every
        field reads right, and with need_lock until a lock is chosen."""
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button(button, Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(12)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(18)
        box.pack_start(dim(intro), False, False, 0)
        if extra is not None:
            box.pack_start(extra, False, False, 0)
        box.pack_start(picker, False, False, 0)

        def update(*_args):
            ok.set_sensitive(picker.is_valid() and (extra is None or extra.problem() is None)
                             and (not need_lock or bool(picker.chosen())))

        picker.on_change(update)
        if extra is not None:
            extra.on_change(update)
        update()
        self.lock_picker, self.lock_extra = picker, extra  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen = (picker.chosen(), picker.relock_flags(), picker.secret()) if picker.is_valid() else None
        dialog.destroy()
        return chosen if answer == Gtk.ResponseType.OK else None

    def start_block(self, _button):
        """§4.3 (owner 2026-11-20, D2): Start applies at once, without
        locks; once on, the same button is Add locks…"""
        block = self.blocks.get(self.selected)
        if not block:
            return
        if block["active"]:
            self.add_locks(None)
        else:
            self.run(["start", block["name"]])

    def add_locks(self, _button):
        """The lock dialog: locks for the selected block, which starts it
        if it is off (a card's lock state asks for this)."""
        block = self.blocks.get(self.selected)
        if not block:
            return
        active = block["active"]
        # R-LOCK-7: a running block takes any kind of lock, added after the
        # ones already on; those only get stronger.
        picker = LockPicker("stop", "stopped", LockPicker.ALL_KINDS, relock_any=True, partner=True)
        asked = self.lock_dialog(
            f"Add locks to {block['name']}" if active else f"Start {block['name']} with locks",
            "New locks come after the ones already on, unlocked last. A kind it already has is "
            "a second lock of that kind, after the others. "
            "A lock that is on is never changed." if active else
            "It starts with the locks below. Once on, a lock is never changed; more can be added.",
            "Add locks", picker, need_lock=True)
        if asked is None:
            return
        chosen, relock_flags, secret = asked
        args = ["start", block["name"]]
        target = f"block '{block['name']}'"
        if not chosen:
            return  # nothing to add
        # R-LOCK-6: locks unlock one at a time in the order of their flags.
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        chosen = waiting_words(chosen, block.get("lock_order") or [])  # behind the locks it has
        locks = []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"{target}: {order}{text}")
        args += relock_flags
        locks += relock_lines(target, relock_flags)
        if not self.confirm_lock(locks):
            return
        if locks:
            args.append("--yes")
        self.run(args, secret + "\n" if secret else None)

    def choose_lock_order(self, chosen):
        """R-LOCK-6: the user puts the locks in the order they are unlocked
        (first at the top). None when cancelled."""
        dialog = Gtk.Dialog(title="Unlock order", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("OK", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim("The locks are unlocked one at a time, top first. A timer lock below "
                           "another counts from when the one above is unlocked."), False, False, 0)
        order = list(chosen)
        listbox = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        box.pack_start(listbox, False, False, 0)

        def fill(select=0):
            for row in listbox.get_children():
                listbox.remove(row)
            for _flags, text in order:
                label = Gtk.Label(label=text.split(" (")[0], xalign=0, margin=4)
                listbox.add(label)
            listbox.show_all()
            listbox.select_row(listbox.get_row_at_index(select))

        def move(step):
            row = listbox.get_selected_row()
            if row is None:
                return
            i, j = row.get_index(), row.get_index() + step
            if 0 <= j < len(order):
                order[i], order[j] = order[j], order[i]
                fill(j)

        buttons = Gtk.Box(spacing=6)
        for label, step in (("Up", -1), ("Down", 1)):
            button = Gtk.Button(label=label)
            button.connect("clicked", lambda _b, s=step: move(s))
            buttons.pack_start(button, False, False, 0)
        box.pack_start(buttons, False, False, 0)
        fill()
        self.lock_order_dialog = (dialog, move)  # for tests
        dialog.show_all()
        answer = dialog.run()
        dialog.destroy()
        return waiting_words(order) if answer == Gtk.ResponseType.OK else None

    def reveal_block(self, _button):
        """Asks a block's password and shows its contents until the window closes."""
        name = self.selected
        password = self.ask_text(f"Show '{name}'", "Its block password.", password=True)
        if not password:
            return
        args = ["--reveal", name]
        ok, output = cli_with_access(list(self.access_passwords), ["gui-state", *args], password + "\n")
        try:
            hidden = ok and next(b for b in json.loads(output)["blocks"] if b["name"] == name).get("hidden")
        except (ValueError, StopIteration, KeyError):
            hidden = True
        if not ok or hidden:
            self.message("Wrong block password.", Gtk.MessageType.INFO)
            return
        self.block_passwords[name] = password
        self.state = None
        self.refresh()

    def stop_block(self, _button):
        if self.selected:
            self.run(["stop", self.selected])

    def save_schedule_lock(self, _button):
        if not self.selected:
            return
        if self.schedule_lock.get_active():
            if not self.fields_ok(("Schedule lock", self.schedule_lead)):
                return
            lead = self.schedule_lead.get_value_as_int()
            if not self.confirm_lock([f"block '{self.selected}': schedule lock, while each window is open"
                                      + (f" and {duration_words(lead * 60)} before it" if lead else "")]):
                return
            self.run(["schedule", "lock", self.selected, "on", "--lead", f"{lead}m", "--yes"])
        elif self.blocks.get(self.selected, {}).get("schedule_lock"):
            self.run(["schedule", "lock", self.selected, "off"])

    def extend_timer(self, _button):
        """§3.4, §4.6: extends the block's timer lock (kc9k extend): time
        added, or a later end picked on a calendar (R-GUI-2)."""
        block = self.blocks.get(self.selected)
        if not block:
            return
        dialog = Gtk.Dialog(title=f"Extend the timer lock of {block['name']}", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Extend", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim("For a time adds that much to the timer lock; Until a date moves its end "
                           "later. A timer lock is never shortened. To add a second timer lock "
                           "instead, use Add locks…."), False, False, 0)
        field = TimerLockField(allow_none=False, mode="for", seconds=1800)
        box.pack_start(field, False, False, 0)
        field.on_change(lambda f: ok.set_sensitive(f.is_valid()))
        ok.set_sensitive(field.is_valid())
        self.extend_field = field  # for tests
        dialog.show_all()
        answer = dialog.run()
        flags = field.flags()
        what = (f"{duration_words(field.duration.value)} more" if flags[:1] == ["--for"]
                else f"until {date_words(field.until())}" if flags else "")
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or not flags:
            return
        if not self.confirm_lock([f"block '{block['name']}': timer lock extended, {what}"]):
            return
        self.run(["extend", block["name"], *flags, "--yes"])

    def unlock_block(self, _button):
        block = self.blocks.get(self.selected)
        if not block:
            return
        if not block["char_locked"]:
            if block["password_locked"]:
                password = self.ask_text(f"Unlock {block['name']}", "Enter the block's password.", password=True)
                if password:
                    self.run(["unlock", block["name"]], password + "\n",
                             then=lambda output: self.message(output, Gtk.MessageType.INFO))
            else:  # only a delay lock: unlocking starts its countdown
                self.run(["unlock", block["name"]], then=lambda output: self.message(output, Gtk.MessageType.INFO))
            return
        self.run_challenge(["unlock", block["name"]])

    def run_challenge(self, command):
        """A text lock's challenge (R-LOCK-5, SPEC §4.6): the command opens
        its own window on this desktop and reads the keys typed there; the
        GUI only runs it, and opens no terminal."""
        self.run(command, then=lambda output: self.message(output, Gtk.MessageType.INFO),
                 timeout=CHALLENGE_TIMEOUT)

    def delete_block(self, _button):
        """§4.2 (owner 2026-11-17): deleted, the page comes back to the block
        list; refused, the block stays open."""
        name = self.selected
        if name and self.confirm(f"Delete {name}?", "The block and its lists are removed.",
                                 "Delete", destructive=True):
            def gone(_output):
                if self.selected == name:
                    self.selected = None
                self.blocks_stack.set_visible_child_name("cards")
            self.run(["block", "remove", name], then=gone)

    def fields_ok(self, *named):
        """R-GUI-1: before a command, every (name, field) must read right;
        else the first problem is shown and nothing runs."""
        for name, field in named:
            if field is not None and not field.is_valid():
                self.message(f"{name}: {field.problem}", Gtk.MessageType.INFO)
                return False
        return True

    def set_quota(self, _button):
        if self.selected and self.fields_ok(("Allowance", self.quota_minutes), ("Sessions", self.quota_sessions)):
            self.run(["allowance", "set", self.selected, f"{self.quota_minutes.get_value_as_int()}m",
                      "--per", self.quota_per.get_active_id() or "day",
                      "--refill", self.quota_refill.get_text().strip() or "00:00",
                      "--sessions", str(self.quota_sessions.get_value_as_int())])

    def start_quota(self, _button):
        if self.selected:
            self.run(["allowance", "start", self.selected])

    def end_quota(self, _button):
        if self.selected:
            self.run(["allowance", "end", self.selected])

    def remove_quota(self, _button):
        if self.selected:
            self.run(["allowance", "remove", self.selected])

    def set_pomodoro(self, _button):
        long = self.long_check.get_active()
        if self.selected and self.fields_ok(("Work", self.work_minutes), ("Break", self.break_minutes),
                                            ("Long break", self.long_minutes if long else None),
                                            ("Work periods", self.long_every if long else None)):
            args = ["pomodoro", "set", self.selected,
                    f"{self.work_minutes.get_value_as_int()}m",
                    f"{self.break_minutes.get_value_as_int()}m"]
            if self.long_check.get_active():
                args += ["--long", f"{self.long_minutes.get_value_as_int()}m",
                         "--every", str(self.long_every.get_value_as_int())]
            self.run(args)

    def set_allowance(self, _button):
        keep = self.reward_keep.get_active_id() or "one"
        kept = self.reward_keep_time if keep == "time" else None
        if self.selected and self.fields_ok(("Break", self.allowance_minutes), ("Blocking", self.allowance_per),
                                            ("Kept for", kept)):
            self.run(["break", "reward", self.selected,
                      f"{self.allowance_minutes.get_value_as_int()}m",
                      "--per", f"{self.allowance_per.get_value_as_int()}m",
                      "--keep", kept.arg() if kept else keep])

    def clear_allowance(self, _button):
        if self.selected:
            self.run(["break", "reward", self.selected, "off"])

    def start_break(self, _button):
        if self.selected:
            self.run(["break", "start", self.selected])

    def end_break(self, _button):
        if self.selected:
            self.run(["break", "end", self.selected])

    def set_relock(self, _button):
        if self.selected and self.fields_ok(
                ("Text break", self.relock_minutes if self.relock_check.get_active() else None)):
            value = f"{self.relock_minutes.get_value_as_int()}m" if self.relock_check.get_active() else "off"
            self.run(["break", "text", self.selected, value])

    def set_delay_break(self, _button):
        if not self.selected:
            return
        if self.delay_break_check.get_active():
            if not self.fields_ok(("Delay break", self.delay_break_minutes), ("Waiting", self.delay_wait_minutes)):
                return
            self.run(["break", "delay", self.selected, f"{self.delay_break_minutes.get_value_as_int()}m",
                      f"{self.delay_wait_minutes.get_value_as_int()}m"])
        else:
            self.run(["break", "delay", self.selected, "off"])

    def clear_pomodoro(self, _button):
        if self.selected:
            self.run(["pomodoro", "off", self.selected])

    def add_schedule(self, values):
        if len(values) != 3:
            self.message("Enter days, start and end, for example: weekdays 09:00 17:00")
            return
        self.run(["schedule", "add", self.selected, "--days", values[0],
                  "--from", values[1], "--to", values[2], *self.window_flags()])

    def create_window_from_calendar(self, column, start, end):
        if not self.selected:
            return
        day, frm, to = DAY_NAMES[column_day(column)], f"{start // 60:02d}:{start % 60:02d}", f"{end // 60:02d}:{end % 60:02d}"
        if self.confirm(f"Add {day.capitalize()} {show_clock(start)}–{show_clock(end, True)} to {self.selected}?",
                        "While the block is on, it is enabled then. To repeat it on other days, type "
                        "them in the list below, e.g. weekdays " + frm + " " + to + ".", "Add"):
            self.run(["schedule", "add", self.selected, "--days", day, "--from", frm, "--to", to, *self.window_flags()])

    def remove_window_from_calendar(self, index):
        block = self.blocks.get(self.selected)
        if not block or index >= len(block["schedules"]):
            return
        if self.confirm(f"Remove {block['schedules'][index]}?",
                        "With a schedule lock, this can't be done during the window or its lead time.",
                        "Remove", destructive=True):
            self.run(["schedule", "remove", self.selected, str(index + 1)])

    def set_automation(self, button, setting, value):
        """§3.6: changes the block's default or its both/either; a refused
        change goes back (update_automation shows the state)."""
        if self.updating or not button.get_active() or not self.selected:
            return
        self.run(["automation", self.selected, setting, value])

    def update_automation(self, block):
        """Shows what switches the block (§3.6)."""
        settings = block.get("automation") or {}
        windows, places = bool(block.get("schedules")), bool(block.get("places"))
        if block.get("continuous", not (windows or places)):
            text = ("Continuous: enabled whenever it is on. Add windows or places below to switch it "
                    "by itself.")
        else:
            by = " and ".join(part for part, has in (("its windows", windows), ("its places", places)) if has)
            text = (f"Automated: {by} turn it {'off' if settings.get('default_on') else 'on'}; "
                    f"it is {'enabled' if settings.get('default_on') else 'lifted'} otherwise.")
        self.auto_summary.set_text(text)
        was, self.updating = self.updating, True
        self.auto_buttons[("default", "enabled" if settings.get("default_on") else "disabled")].set_active(True)
        self.auto_buttons[("needs", "both" if settings.get("need_both") else "either")].set_active(True)
        self.updating = was
        self.auto_need_row.set_visible(windows and places)

    def window_flags(self):
        """§3.6: a window follows its block's default: on a block enabled by
        default it turns the block off (an allow window)."""
        return ["--allow"] if ((self.blocks.get(self.selected) or {}).get("automation") or {}).get("default_on") else []

    def replace_window(self, index, days, start, end, allow=False):
        """Changes a window: removes it, then adds the new one (§4.5; a
        change that weakens is refused while the block is locked)."""
        names = [DAY_NAMES[(bit - 1) % 7] for bit in range(7) if days & (1 << bit)]
        if not names:
            return
        args = ["schedule", "add", self.selected, "--days", ",".join(names),
                "--from", clock_text(start), "--to", "24:00" if end >= 1440 else clock_text(end)]
        if allow:
            args.append("--allow")
        self.run(["schedule", "remove", self.selected, str(index + 1)], then=lambda _o: self.run(args))

    def move_window_from_calendar(self, index, days, start, end):
        windows = (self.blocks.get(self.selected) or {}).get("schedule_windows") or []
        if index < len(windows) and windows[index].get("source"):
            self.window_from_schedule(windows[index])
        elif index < len(windows):
            self.replace_window(index, days, start, end, windows[index].get("allow", False))

    def window_from_schedule(self, w):
        """§3.6: a window from a schedule is changed on the Schedules page."""
        if self.confirm(f"This window comes from the schedule {w['source']}",
                        "Change it on the Schedules page (that changes everything using it), or take the "
                        "schedule off this block under Automation.", "Open Schedules"):
            self.open_schedule(w["source"])

    def apply_schedule_to_block(self, _button):
        block = self.blocks.get(self.selected) or {}
        have = set(block.get("applied_schedules") or [])
        names = [sc["name"] for sc in self.state.get("schedules") or [] if sc["name"] not in have]
        if not names:
            if self.confirm("No schedule to apply", "Make schedules such as Evening or Weekend on the "
                            "Schedules page, then apply them here.", "Open Schedules"):
                self.stack.set_visible_child_name("schedules")
            return
        name = self.choose("Apply a schedule", "Its windows join the block's; changing the schedule on the "
                           "Schedules page changes them too.", names)
        if name and self.selected:
            self.run(["schedule", "apply", self.selected, name])

    def edit_window_from_calendar(self, index):
        """§4.5: click a window to change its days (copy to days) and times,
        or remove it."""
        block = self.blocks.get(self.selected)
        windows = (block or {}).get("schedule_windows") or []
        if not block or index >= len(windows):
            return
        w = windows[index]
        if w.get("source"):
            self.window_from_schedule(w)
            return
        dialog = Gtk.Dialog(title="Window", transient_for=self, modal=True)
        dialog.add_button("Remove", 3)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Save", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        days_row = Gtk.Box(spacing=6)
        checks = {}
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
        # §3.6: one break each time this window is open.
        break_row = Gtk.Box(spacing=6)
        break_minutes = DurationField(w.get("break", 0), minimum=60, maximum=4 * 3600, zero="No break", width=8)
        for widget in (Gtk.Label(label="Break each time"), break_minutes):
            break_row.pack_start(widget, False, False, 0)
        if not w.get("allow"):
            box.pack_start(break_row, False, False, 0)
        self.window_dialog = (checks, start, end, break_minutes)  # for tests
        dialog.show_all()
        answer = dialog.run()
        days = sum(bit for bit, check in checks.items() if check.get_active())
        frm, to = start.get_text().strip(), end.get_text().strip()
        new_break = break_minutes.get_value_as_int()
        break_problem = break_minutes.problem
        dialog.destroy()
        if answer == 3:
            self.remove_window_from_calendar(index)
            return
        if answer != Gtk.ResponseType.OK:
            return
        if break_problem:
            self.message(f"Break: {break_problem}", Gtk.MessageType.INFO)
            return
        try:
            new_start = int(frm[:2]) * 60 + int(frm[3:5])
            new_end = 1440 if to == "24:00" else int(to[:2]) * 60 + int(to[3:5])
        except (ValueError, IndexError):
            self.message("Times are HH:MM, e.g. 09:00 and 17:00.", Gtk.MessageType.INFO)
            return
        if (days, new_start, new_end) != (w["days"], w["start"], w["end"]):
            self.replace_window(index, days, new_start, new_end, w.get("allow", False))
        elif new_break != w.get("break", 0) // 60:
            self.run(["schedule", "break", self.selected, str(index + 1), f"{new_break}m" if new_break else "off"])

    def remove_schedule(self, _value, number):
        self.run(["schedule", "remove", self.selected, str(number)])

    def choose_file(self, title, action, button):
        dialog = Gtk.FileChooserDialog(title=title, transient_for=self, action=action)
        dialog.add_buttons("Cancel", Gtk.ResponseType.CANCEL, button, Gtk.ResponseType.OK)
        if action == Gtk.FileChooserAction.SAVE:
            dialog.set_do_overwrite_confirmation(True)
            dialog.set_current_name(f"{self.selected}.txt")
        answer = dialog.run()
        path = dialog.get_filename()
        dialog.destroy()
        return path if answer == Gtk.ResponseType.OK else None

    def import_list(self, command):
        """Runs command (an import with its block and list) with a file, or
        for a block's list, copies other lists and clusters into it (§4.3
        import from KC9K)."""
        if not self.selected:
            return
        if command[:2] == ["list", "import"]:
            answer = self.ask_import_source(
                "Import into this list", "From a text file with one entry per line (a hosts file and "
                "adblock rules are read too), or from other lists and list presets in Killer Chicken 9000; "
                "or the whole internet, or all searches containing some words.", more=True)
            if answer == "kc9k":
                self.import_from_kc9k(command[2], command[3])
                return
            if answer == "internet":
                self.import_internet(command[2], command[3])
                return
            if answer == "searches":
                self.import_searches(command[2], command[3])
                return
            if answer != "file":
                return
        path = self.choose_file("Import list", Gtk.FileChooserAction.OPEN, "Import")
        if not path:
            return
        if command[:2] != ["list", "import"]:
            self.run(command + [path], then=self.queued_note)
            return
        # §4.3: KC9K reads the file's layout; phrases in it go to a phrase set.
        found = self.read_import_file(path)
        if found is None:
            return
        block, steps = command[2], []
        if found["phrases"]:
            target = self.choose_phrase_set(block, len(found["phrases"]))
            if target:
                name, new = target
                if new:
                    steps.append(["phrase-set", "create", block, name])
                steps.append(["phrase-set", "import", block, name, path])
        if found["entries"]:
            steps.append(command + [path])
        if not steps:
            if not found["phrases"]:
                self.message("Nothing in that file can go in a list: " + describe_found(found) + ".",
                             Gtk.MessageType.INFO)
            return
        self.run_steps(steps, routed_phrases=bool(found["phrases"]))

    def import_from_kc9k(self, block, list_name):
        """Ticks other blocks' lists and clusters to copy into a list."""
        sources = []
        for other in (self.state or {}).get("blocks") or []:
            if other.get("hidden"):
                continue
            for lst in other.get("lists") or []:
                if (other["name"], lst["name"]) != (block, list_name) and lst.get("entries"):
                    sources.append((f"{other['name']}: {lst['name']}", ["list", "copy", other["name"], lst["name"], block, list_name]))
        for cluster in (self.state or {}).get("clusters") or []:
            if not cluster.get("phrases"):
                sources.append((f"List preset: {cluster['name']}", ["list-preset", "copy", cluster["name"], block, "--into", list_name]))
        if not sources:
            self.message("No other lists or list presets to import from.", Gtk.MessageType.INFO)
            return
        dialog = Gtk.Dialog(title="Import from Killer Chicken 9000", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Import", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(4)
        box.set_border_width(12)
        checks = []
        for label, command in sources:
            check = Gtk.CheckButton(label=label)
            box.pack_start(check, False, False, 0)
            checks.append((check, command))
        dialog.show_all()
        answer = dialog.run()
        chosen = [command for check, command in checks if check.get_active()]
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return

        def next_copy(_output=None):
            if chosen:
                self.run(chosen.pop(0), then=next_copy)

        next_copy()

    def ask_import_source(self, title, detail, more=False):
        """From a file or from KC9K: "file", "kc9k" or None; with more (a
        list, §4.3), also "internet" (the whole internet) and "searches"
        (all searches containing some words)."""
        dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                   buttons=Gtk.ButtonsType.NONE, text=title, secondary_text=detail)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        if more:
            dialog.add_button("Whole internet", 3)
            dialog.add_button("All searches containing…", 4)
        dialog.add_button("From KC9K…", 2)
        dialog.add_button("From file…", Gtk.ResponseType.OK)
        answer = dialog.run()
        dialog.destroy()
        return {Gtk.ResponseType.OK: "file", 2: "kc9k", 3: "internet", 4: "searches"}.get(answer)

    def import_internet(self, block, name):
        """§4.3 Whole internet: *.* into the list."""
        if self.confirm(f"Add the whole internet to {name}?",
                        "It gets *.*: every site is matched (local files, localhost and the local network "
                        "stay reachable). In a blacklist, only the block's exceptions are let through.", "Add"):
            self.run(["list", "add", block, name, "*.*"])

    def import_searches(self, block, name):
        """§4.3 All searches containing…: the words as rendered terms that
        look at the address, where a search's words are (R-PHRASE-6)."""
        text = self.ask_text(
            "All searches containing…",
            "Words, separated by commas. A search for one of them is matched by its address (it holds "
            "the words searched for), and so is any other address with that word in it.",
            problem=lambda t: None if [w for w in t.split(",") if w.strip()] else "Type at least one word.")
        words = [w.strip() for w in (text or "").split(",") if w.strip()]
        if not words:
            return
        list_data = next((l for l in (self.blocks.get(block) or {}).get("lists", []) if l["name"] == name), {})
        steps = [["list", "add", block, name, *words]]
        if "url" not in (list_data.get("where") or []):
            steps.append(["list", "scope", block, name, "url", "on"])
        self.run_steps(steps)

    def read_import_file(self, path):
        """What a file to import holds, as KC9K reads it (§4.3): phrases,
        entries (sites and rendered terms), what it can't use. None after
        saying why it can't be read."""
        ok, output = cli_with_access(self.access_passwords, ["list", "read", path])
        if not ok:
            self.message(output)
            return None
        try:
            return json.loads(output)
        except ValueError:
            self.message(output)
            return None

    def run_steps(self, steps, routed_phrases=False):
        """Runs commands one after another; then says what they did, once."""
        said = []

        def next_step(output=None):
            if output:
                said.extend(line for line in output.splitlines()
                            if not (routed_phrases and line.startswith("Left out") and " phrase" in line))
            if steps:
                self.run(steps.pop(0), then=next_step)
            elif said:
                self.message("\n".join(said), Gtk.MessageType.INFO)

        next_step()

    def choose_phrase_set(self, block, count):
        """Where the phrases of an imported file go: one of the block's
        phrase sets or a new one, as (name, new); None leaves them out."""
        names = [p["name"] for p in self.blocks.get(block, {}).get("phrase_sets") or []]
        dialog = Gtk.Dialog(title="Phrases in the file", transient_for=self, modal=True)
        dialog.add_button("Leave them out", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Add them", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim(f"{count} {'line is a phrase' if count == 1 else 'lines are phrases'} "
                           "([a, b] [c]): a list can't hold phrases. Add them to a phrase set of "
                           f"'{block}':"), False, False, 0)
        combo = Gtk.ComboBoxText()
        for name in names:
            combo.append(name, name)
        combo.append("", "A new phrase set:")
        combo.set_active(0)
        new_name = Gtk.Entry(text="Imported phrases", sensitive=not names)
        combo.connect("changed", lambda c: new_name.set_sensitive(c.get_active_id() == ""))
        box.pack_start(combo, False, False, 0)
        box.pack_start(new_name, False, False, 0)
        self.phrase_target = (dialog, combo, new_name)  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen, typed = combo.get_active_id(), new_name.get_text().strip()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return None
        if chosen:
            return chosen, False
        return (typed, typed.lower() not in (n.lower() for n in names)) if typed else None

    def import_into_phrase_set(self, card):
        """§4.3, one Import for a phrase set (owner, 2026-11-16): KC9K reads
        the file's layout. Phrases ([eat, nice] [hello, grandpa]) go into
        the set; rendered terms, one per line, become one word set in the
        phrase editor below ([term 1, term 2, …]); a list of sites can go
        to the block's Blocked list. From KC9K: other lists' and clusters'
        entries as one word set."""
        if not self.selected:
            return
        block, name = self.selected, card.data["name"]
        answer = self.ask_import_source(
            "Import into this phrase set",
            "From a text file, one per line: phrases such as [eat, nice] [hello, grandpa] are added to "
            "the set; rendered terms become one word set in the phrase editor. Or from lists and "
            "list presets in Killer Chicken 9000: their entries become one word set.")
        if answer == "kc9k":
            terms = self.pick_kc9k_terms()
            if terms:
                card.editor.add_imported(terms)
                self.message(f"{len(terms)} rendered terms are now a word set in the phrase editor of "
                             f"'{name}'. Fill the other word set and click Add phrase.", Gtk.MessageType.INFO)
            return
        if answer != "file":
            return
        path = self.choose_file("Import into a phrase set", Gtk.FileChooserAction.OPEN, "Import")
        if not path:
            return
        found = self.read_import_file(path)
        if found is None:
            return
        if found["phrases"]:
            self.run_steps([["phrase-set", "import", block, name, path]])
        elif found["terms"]:
            card.editor.add_imported(word_set_terms(found["terms"]))
            note = f" ({len(found['sites'])} sites in the file were left out)" if found["sites"] else ""
            self.message(f"{len(found['terms'])} rendered terms are now a word set in the phrase editor of "
                         f"'{name}'{note}. Fill the other word set and click Add phrase.", Gtk.MessageType.INFO)
        elif found["sites"]:
            if self.confirm("The file lists sites",
                            f"It lists {len(found['sites'])} sites, which belong in a list, not in a phrase "
                            f"set. Add them to the list 'Blocked' of '{block}'?", "Add to Blocked"):
                self.run_steps([["list", "import", block, "Blocked", path]])
        else:
            self.message("Nothing in that file can be imported: " + describe_found(found) + ".",
                         Gtk.MessageType.INFO)

    def pick_kc9k_terms(self):
        """Entries of lists and clusters in KC9K, ticked, for one word set."""
        sources = []
        for other in (self.state or {}).get("blocks") or []:
            if other.get("hidden"):
                continue
            for lst in other.get("lists") or []:
                if lst.get("entries"):
                    sources.append((f"{other['name']}: {lst['name']}", lst["entries"]))
        for cluster in (self.state or {}).get("clusters") or []:
            if not cluster.get("phrases") and cluster.get("entries"):
                sources.append((f"List preset: {cluster['name']}", cluster["entries"]))
        if not sources:
            self.message("No lists or list presets to import from.", Gtk.MessageType.INFO)
            return []
        dialog = Gtk.Dialog(title="Import a word set from Killer Chicken 9000", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Import", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(4)
        box.set_border_width(12)
        checks = []
        for label, entries in sources:
            check = Gtk.CheckButton(label=label)
            box.pack_start(check, False, False, 0)
            checks.append((check, entries))
        self.term_sources = checks  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen = [e for check, entries in checks if check.get_active() for e in entries]
        dialog.destroy()
        return word_set_terms(chosen) if answer == Gtk.ResponseType.OK else []

    def export_list(self, command):
        """Saves what command (an export) prints to a file."""
        if not self.selected:
            return
        path = self.choose_file("Export list", Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, command)
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n" if output else "")
        except OSError as error:
            self.message(str(error))

    def share_project(self, button):
        self.export_project(button, share=True)

    def ask_share_presets(self):
        """Sharing the setup: include the presets? True, False, or None
        (cancelled). A backup always includes them (SPEC §3.13)."""
        dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                   text="Include your presets?",
                                   secondary_text="Block presets, list and phrase set presets, schedule, "
                                                  "automation and hotkey presets.")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Without presets", 2)
        dialog.add_button("With presets", Gtk.ResponseType.OK)
        answer = dialog.run()
        dialog.destroy()
        return {Gtk.ResponseType.OK: True, 2: False}.get(answer)

    def export_project(self, _button, share=False):
        command = ["project", "export"]
        if share:
            presets = self.ask_share_presets()
            if presets is None:
                return
            command += ["--share", "--presets" if presets else "--presets=false"]
        path = self.choose_file("Share the setup (without places)" if share else "Back up the whole setup",
                                Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, command)
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n")
        except OSError as error:
            self.message(str(error))

    def import_project(self, _button):
        path = self.choose_file("Import a setup", Gtk.FileChooserAction.OPEN, "Import")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, ["project", "import", "--dry-run", path])
        if not ok:
            self.message(output)
            return
        if "Would:" not in output:
            self.message(output or "The file adds nothing new.", Gtk.MessageType.INFO)
            return
        locks = [line.split("Would: set a lock: ", 1)[1] for line in output.splitlines()
                 if "Would: set a lock: " in line]
        # R-CUSTOM-1: what its custom actions open or run is in the list above.
        customs = "Would: add a custom action that can " in output
        if self.confirm("Import this setup?", output.replace("Would: ", "• "), "Import") \
                and self.confirm_lock(locks):
            self.run(["project", "import", path] + (["--yes"] if locks or customs else []),
                     then=lambda result: self.message(result, Gtk.MessageType.INFO))
