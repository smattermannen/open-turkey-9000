# Window: Frozen Chicken page: freezes, device protection, guided setup.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _DevicePage:
    """Methods of Window (window.py); no state of its own."""

    # Frozen Chicken ------------------------------------------------------
    def build_device(self):
        """Frozen Chicken's part of its mode's view (§3.8, §4.8, owner
        2026-11-17): freeze now, its scheduled freezes, its allowed apps."""
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18)
        box.pack_start(dim(
            "Lock the whole computer for a while: the screen, your session, or the power. It keeps the "
            "computer unusable until the time is up, even across restarts, and can't be cancelled, only "
            "extended. The time keeps counting while the computer is off. At most a week at a time."),
            False, False, 0)

        self.freeze_status = Gtk.Label(xalign=0)
        row = Gtk.Box(spacing=8)
        self.freeze_mode = Gtk.ComboBoxText()
        self.freeze_mode.append("lock", "Lock the screen")
        self.freeze_mode.append("logout", "Log out")
        self.freeze_mode.append("shutdown", "Shut down")
        self.freeze_mode.append("apps", "Allowed apps only")  # §3.8: kc9k freeze apps
        self.freeze_mode.set_active_id("lock")
        self.freeze_minutes = DurationField(1800, minimum=60, maximum=FREEZE_MAX, presets=(1800, 3600, 4 * 3600, 86400),
                                            why_max="Frozen Chicken lasts at most a week.", width=9)
        self.freeze_button = Gtk.Button(label="Start Frozen Chicken…")
        self.freeze_button.get_style_context().add_class("destructive-action")
        self.freeze_button.connect("clicked", self.freeze)
        row.pack_start(self.freeze_mode, False, False, 0)
        row.pack_start(Gtk.Label(label="for"), False, False, 0)
        row.pack_start(self.freeze_minutes, False, False, 0)
        row.pack_end(self.freeze_button, False, False, 0)
        self.freeze_schedules = EditableList(
            "", "daily 23:00 24:00", self.add_freeze_schedule, self.remove_freeze_schedule,
            help_text="Freeze at set times in the mode chosen above, for example every night. "
                      "Days, start and end; for overnight add two (23:00 24:00 and 00:00 06:00). "
                      "A locked schedule can't be removed from its lead time before it starts until it ends.")
        # §3.8: the apps an "allowed apps only" freeze lets run.
        self.freeze_apps = EditableList(
            "", "libreoffice  /opt/notes/",
            lambda values: self.run(["freeze", "apps", "add", *values]),
            lambda value, _num: self.run(["freeze", "apps", "remove", value]),
            help_text="While an 'allowed apps only' freeze runs, every other program of yours is closed; "
                      "the desktop keeps working. Adding an app isn't possible while a freeze runs or "
                      "anything is locked.")
        self.freeze_lock = Gtk.CheckButton(label="Lock new schedules from")
        self.freeze_lead = DurationField(7200, minimum=60, maximum=7 * 86400, zero="No lead time", width=9)
        freeze_lock_row = Gtk.Box(spacing=6)
        freeze_lock_row.pack_start(self.freeze_lock, False, False, 0)
        freeze_lock_row.pack_start(self.freeze_lead, False, False, 0)
        freeze_lock_row.pack_start(Gtk.Label(label="before each window", valign=Gtk.Align.START, margin_top=6),
                                   False, False, 0)
        top_align(freeze_lock_row)
        top_align(row)
        box.pack_start(section("Freeze now", row, self.freeze_status), False, False, 0)
        flow_row(row)
        # §3.8: every lock kind on a scheduled freeze (not removable then).
        freeze_locks_row = Gtk.Box(spacing=6)
        freeze_locks_row.pack_start(Gtk.Label(label="The selected schedule:"), False, False, 0)
        for label, handler in (("Lock…", self.lock_freeze_schedule), ("Unlock…", self.unlock_freeze_schedule)):
            button = Gtk.Button(label=label)
            button.connect("clicked", handler)
            freeze_locks_row.pack_start(button, False, False, 0)
        box.pack_start(section("Scheduled freezes", self.freeze_schedules, freeze_lock_row, freeze_locks_row),
                       False, False, 0)
        flow_row(freeze_lock_row)
        box.pack_start(section("Allowed apps", self.freeze_apps), False, False, 0)
        return box

    def switch_row(self, switch, title, detail):
        row = Gtk.Box(spacing=16)
        text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        text.pack_start(Gtk.Label(label=title, xalign=0), False, False, 0)
        text.pack_start(dim(detail), False, False, 0)
        row.pack_start(text, True, True, 0)
        row.pack_end(switch, False, False, 0)
        return row

    def add_freeze_schedule(self, values):
        if len(values) != 3:
            self.message("Enter days, start and end, for example: daily 23:00 24:00")
            return
        mode = self.freeze_mode.get_active_id()
        lock = self.freeze_lock.get_active()
        lead = self.freeze_lead.get_value_as_int()
        if lock and not self.fields_ok(("Lead time", self.freeze_lead)):
            return
        if lock:
            confirmed = self.confirm_lock([
                f"freeze schedule {values[0]} {values[1]}-{values[2]} "
                f"({self.freeze_mode.get_active_text().lower()}): schedule lock, while each window is open"
                + (f" and {duration_words(lead * 60)} before it" if lead else "")])
        else:
            confirmed = self.confirm(f"Freeze {values[0]} {values[1]}-{values[2]}?",
                                     f"The computer will be frozen then "
                                     f"({self.freeze_mode.get_active_text().lower()}).", "Schedule")
        if confirmed:
            self.run(["freeze", "schedule", "add", "--days", values[0], "--from", values[1],
                      "--to", values[2], "--mode", mode, "--yes"] + (["--lock-lead", f"{lead}m"] if lock else []))

    def remove_freeze_schedule(self, _value, number):
        self.run(["freeze", "schedule", "remove", str(number)])

    def chosen_freeze_schedule(self):
        """The schedule picked in the list, as (its text, its number)."""
        chosen = self.freeze_schedules._selected()
        if not chosen:
            self.message("Pick a schedule in the list first.", Gtk.MessageType.INFO)
            return None
        return chosen[-1]

    def lock_freeze_schedule(self, _button):
        """§3.8: timer, text, password, restart, range locks and re-lock on
        a scheduled freeze; while on it can't be removed (R-FREEZE-2)."""
        chosen = self.chosen_freeze_schedule()
        if not chosen:
            return
        text, number = chosen
        picker = LockPicker("remove it", "removed", ("timer", "password", "text", "restart", "range", "relock"))
        asked = self.lock_dialog(f"Lock the freeze schedule {text}",
                                 "It can't be removed until the locks end. More locks can be added later, "
                                 "after these; a lock that is on is never changed.", "Lock", picker, need_lock=True)
        if asked is None:
            return
        locks, relock_flags, secret = asked
        if not locks:
            self.message("Choose at least one lock.", Gtk.MessageType.INFO)
            return
        if len(locks) > 1:
            locks = self.choose_lock_order(locks)
            if locks is None:
                return
        args = ["freeze", "schedule", "lock", str(number)]
        lines = []
        for i, (flags, words) in enumerate(locks):
            args += flags
            lines.append(f"freeze schedule {text}: " + (f"{i + 1} of {len(locks)}: " if len(locks) > 1 else "") + words)
        if self.confirm_lock(lines):
            self.run(args + relock_flags + ["--yes"], secret + "\n" if secret else None)

    def unlock_freeze_schedule(self, _button):
        chosen = self.chosen_freeze_schedule()
        if not chosen:
            return
        text, number = chosen
        locks = self.state.get("freeze_schedule_locks") or []
        info = locks[number - 1] if number - 1 < len(locks) else {}
        unlock = ["freeze", "schedule", "unlock", str(number)]
        if (info.get("lock_order") or [])[:1] == ["password"]:
            password = self.ask_text(f"Unlock the freeze schedule {text}", "Enter its password.", password=True)
            if password:
                self.run(unlock, password + "\n")
            return
        if not info.get("char_locked"):
            self.message("Its first lock isn't a text or password lock: it ends by itself.", Gtk.MessageType.INFO)
            return
        self.run_challenge(unlock)

    def update_device(self):
        self.freeze_schedules.set_items(self.state.get("freeze_schedules", []))
        self.freeze_apps.set_items(self.state.get("freeze_apps", []))
        frozen = self.state["freeze_seconds"]
        modes = {"lock": "locking the screen", "logout": "logging out", "shutdown": "shutting down"}
        self.freeze_status.set_markup(
            f"<b>On</b> for {fmt_seconds(frozen)} more, {modes.get(self.state['freeze_mode'], '')}. "
            "Starting again can only extend it." if frozen else "Off.")
        self.freeze_button.set_label("Extend…" if frozen else "Start Frozen Chicken…")

    def freeze_dialog(self, answer=None):
        """§3.8, R-FREEZE-3: Frozen Chicken switched on asks how (lock,
        log out, shut down, allowed apps) and until when, at most a week
        ahead. answer: (mode, flags) for tests."""
        if answer is None:
            dialog = Gtk.Dialog(title="Freeze the computer", transient_for=self, modal=True)
            dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
            ok = dialog.add_button("Freeze", Gtk.ResponseType.OK)
            ok.get_style_context().add_class("destructive-action")
            box = dialog.get_content_area()
            box.set_spacing(10)
            box.set_border_width(14)
            box.pack_start(dim("Until the time is up the computer can't be used as chosen here, also after a "
                               "restart. It can't be cancelled, only extended. Save your work first."), False, False, 0)
            how = Gtk.ComboBoxText()
            for key, label in (("lock", "Lock the screen"), ("logout", "Log out"), ("shutdown", "Shut down"),
                               ("apps", "Allowed apps only")):
                how.append(key, label)
            how.set_active_id(self.freeze_mode.get_active_id() or "lock")
            box.pack_start(how, False, False, 0)
            until = TimerLockField(allow_none=False, seconds=3600)
            until.duration.set_limits(60, FREEZE_MAX, why_max="Frozen Chicken lasts at most a week.")
            box.pack_start(until, False, False, 0)
            self.freeze_dialog_parts = (dialog, how, until)  # for tests
            dialog.show_all()
            reply = dialog.run()
            mode, flags, problem = how.get_active_id(), until.flags(), until.problem()
            moment = until.until() if until.mode == "until" else None
            dialog.destroy()
            if reply != Gtk.ResponseType.OK:
                return
            if problem or not flags:
                self.message(problem or "Choose how long.", Gtk.MessageType.INFO)
                return
            if moment and (moment - datetime.datetime.now()).total_seconds() > FREEZE_MAX:
                self.message("Frozen Chicken lasts at most a week: pick an earlier date.", Gtk.MessageType.INFO)
                return
        else:
            mode, flags = answer
        self.run(["freeze", *flags, "--mode", mode, "--yes"])

    def freeze(self, _button):
        if not self.fields_ok(("Frozen Chicken", self.freeze_minutes)):
            return
        minutes = self.freeze_minutes.get_value_as_int()
        mode = self.freeze_mode.get_active_id()
        effect = {
            "lock": "The screen locks now and keeps locking again until the time is up.",
            "logout": "Every session is logged out now, and logging in again ends at once.",
            "shutdown": "The computer shuts down now, and again whenever it starts.",
        }[mode]
        if self.confirm(f"Freeze the computer for {duration_words(minutes * 60)}?",
                        f"{effect} It can't be cancelled. Save your work first.",
                        "Freeze", destructive=True):
            self.run(["freeze", "--for", f"{minutes}m", "--mode", mode, "--yes"])

    def toggle_password_locks(self, check):
        if self.updating:
            return
        self.run(["password", "locks", "on" if check.get_active() else "off"])

    def dns_choice_changed(self, combo):
        self.dns_servers.set_visible(combo.get_active_id() == "own")

    def apply_dns(self, _button):
        choice = self.dns_choice.get_active_id()
        if choice == "own":
            choice = ",".join(s.strip() for s in self.dns_servers.get_text().split(",") if s.strip())
            if not choice:
                self.message("Enter one or more server addresses, separated by commas.")
                return
        self.run(["dns", choice])

    def toggle_reset_guard(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["reset-guard", "on" if wanted else "off"])
        return True

    def device_setup_row(self):
        """§4.10 Device protection: the firmware lock and the guided setup."""
        row = Gtk.Box(spacing=8)
        text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        text.pack_start(Gtk.Label(label="Factory reset protection: firmware lock", xalign=0), False, False, 0)
        text.pack_start(dim("USB and network boot off, the boot order locked, and a firmware password "
                            "a trusted person keeps. The guided setup lists every step of the "
                            "strictest setup, in order."), False, False, 0)
        row.pack_start(text, True, True, 0)
        guided = Gtk.Button(label="Guided setup…", valign=Gtk.Align.CENTER)
        guided.connect("clicked", self.show_guided_setup)
        firmware = Gtk.Button(label="Firmware lock…", valign=Gtk.Align.CENTER)
        firmware.connect("clicked", self.show_firmware_lock)
        row.pack_end(guided, False, False, 0)
        row.pack_end(firmware, False, False, 0)
        return row

    def cli_json(self, args):
        """Runs a reading command now and returns its JSON, or None after
        saying why."""
        ok, output = cli_with_access(list(self.access_passwords), args)
        if not ok:
            self.message(output or "The command failed.")
            return None
        try:
            return json.loads(output)
        except ValueError:
            self.message("Unexpected answer from kc9k: " + output[:200])
            return None

    def show_firmware_lock(self, _button):
        status = self.cli_json(["firmware-lock", "--json"])
        if status is None:
            return
        lines = []
        if not status["available"]:
            lines.append("This computer's firmware settings can't be changed from Linux. Do it in the "
                         "firmware's own setup (press F2, F1, F10 or Del while the computer starts): turn "
                         "off booting from USB and the network, lock the boot order, and set a firmware "
                         "password that a trusted person types and keeps.")
        else:
            lines.append("Firmware lock: " + ("on" if status["locked"] else "off"))
            lines.append("Firmware password: " + ("set" if status["password"] else "not set"))
            for setting in status["settings"]:
                state = setting["current"]
                if not setting["want"]:
                    state += " (can't be changed here)"
                elif not setting["done"]:
                    state += f" (the lock sets {setting['want']})"
                lines.append(f"{setting['purpose']} ({setting['name']}): {state}")
            for missing in status["missing"]:
                lines.append(f"{missing}: not offered here; set it in the firmware's own setup")
        dialog = Gtk.MessageDialog(transient_for=self, modal=True, text="Firmware lock",
                                   secondary_text="\n".join(lines), message_type=Gtk.MessageType.INFO)
        dialog.add_button("Close", Gtk.ResponseType.CLOSE)
        if status["available"] and status["set_by_kc9k"]:
            dialog.add_button("Remove…", 2)
        if status["available"] and not status["locked"]:
            dialog.add_button("Lock the firmware…", Gtk.ResponseType.OK)
        answer = dialog.run()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK:
            self.lock_firmware(status)
        elif answer == 2:
            password = self.ask_text("Firmware password", "The trusted person types it. The settings "
                                     "the lock changed are put back and the password is cleared.",
                                     password=True)
            if password:
                self.run(["firmware-lock", "off"], password + "\n")

    def lock_firmware(self, status):
        if status["password"]:
            password = self.ask_text("The firmware password already set",
                                     "Needed to change the firmware's settings; it stays as it is.",
                                     password=True)
        else:
            password = self.ask_text("Firmware password",
                                     "The trusted person types it and keeps it. Killer Chicken 9000 "
                                     "doesn't keep it.", password=True)
            if password and self.ask_text("Type it again", password=True) != password:
                self.message("The two passwords differ; nothing changed.", Gtk.MessageType.INFO)
                return
        if not password:
            return
        kind = ("USB and network boot off, boot order locked, under the firmware password already set"
                if status["password"] else
                "USB and network boot off, boot order locked, a firmware password a trusted person keeps")
        if not self.confirm_lock([f"the computer's firmware: firmware lock ({kind})"]):
            return
        self.run(["firmware-lock", "on", "--yes"], password + "\n")

    GUIDED_MARKS = {"done": "✓", "todo": "○", "by-hand": "○", "info": "ℹ", "unknown": "?"}

    def show_guided_setup(self, _button):
        steps = self.cli_json(["guided-setup", "--json"])
        if steps is None:
            return
        dialog = Gtk.Dialog(title="Guided setup", transient_for=self, modal=True)
        dialog.add_button("Close", Gtk.ResponseType.CLOSE)
        dialog.set_default_size(620, 560)
        scroll = Gtk.ScrolledWindow(vexpand=True)
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12, margin=16)
        box.pack_start(dim("The strictest setup, step by step: what protects the setup first, "
                           "no-escape last. Each step says whether it is done."), False, False, 0)
        chosen = []
        for i, step in enumerate(steps):
            row = Gtk.Box(spacing=12)
            mark = Gtk.Label(label=self.GUIDED_MARKS.get(step["state"], "?"), valign=Gtk.Align.START)
            row.pack_start(mark, False, False, 0)
            text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
            title = f"{i + 1}. {step['title']}" + (" (optional)" if step.get("optional") else "")
            text.pack_start(Gtk.Label(label=title, xalign=0), False, False, 0)
            text.pack_start(dim(step["detail"]), False, False, 0)
            command = step.get("command", "")
            if step["state"] != "done" and command and not command.startswith("kc9k "):
                shown = Gtk.Label(label=command, xalign=0, selectable=True)
                shown.get_style_context().add_class("monospace")
                text.pack_start(shown, False, False, 0)
            row.pack_start(text, True, True, 0)
            if step["state"] in ("todo", "by-hand") and self.guided_action(step["id"]):
                button = Gtk.Button(label="Do it…", valign=Gtk.Align.START)
                button.connect("clicked", lambda _b, s=step: (chosen.append(s), dialog.response(Gtk.ResponseType.OK)))
                row.pack_end(button, False, False, 0)
            box.pack_start(row, False, False, 0)
        scroll.add(box)
        dialog.get_content_area().pack_start(scroll, True, True, 0)
        dialog.show_all()
        answer = dialog.run()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and chosen:
            self.guided_action(chosen[0]["id"], chosen[0])()

    def guided_action(self, step_id, step=None):
        """What a guided setup step's Do it… runs: the same as its own
        switch or button elsewhere, so it asks the same questions."""
        def tamper():
            commands = [c.strip().split()[1:] for c in (step or {}).get("command", "").split(";") if c.strip()]

            def next_one(_output=None):
                if commands:
                    self.run(commands.pop(0), then=next_one)
            next_one()
        return {
            "settings-password": lambda: self.set_settings_password(None),
            "no-weakening": lambda: self.toggle_no_weakening(None, True),
            "tamper": tamper,
            "strict": lambda: self.toggle_strict(None, True),
            "strict-lock": lambda: self.lock_setting(None, "strict", "Strict mode"),
            "reset-guard": lambda: self.toggle_reset_guard(None, True),
            "firmware": lambda: self.show_firmware_lock(None),
            "no-escape": lambda: self.toggle_no_escape(None, True),
        }.get(step_id)

    def toggle_no_weakening(self, _switch, wanted):
        if self.updating:
            return False
        if wanted and not self.confirm(
                "Turn on no weakening during active locks?",
                "While any block or setting is locked, no setting can be weakened, and this "
                "can't be turned off either until every lock has ended.", "Turn on"):
            return True
        self.run(["no-weakening", "on" if wanted else "off"])
        return True

    def toggle_strict(self, switch, wanted):
        if self.updating:
            return False
        if wanted and not self.confirm(
                "Turn on strict mode?",
                "During a lock, sudo and pkexec are refused for every user, so you can't "
                "install software or change the system until it ends. Emergency exit: add "
                "kc9k.nofreeze to the linux line in the boot menu.", "Turn on"):
            return True
        self.run(["strict", "on" if wanted else "off"])
        return True  # the switch follows the state after the command

    def toggle_no_escape(self, switch, wanted):
        if self.updating:
            return False
        if wanted:
            if not self.confirm(
                    "Turn on no-escape?",
                    "During a lock, the only way out will be booting a USB stick or "
                    "reinstalling. Make sure you have a live USB and know your firmware's boot "
                    "menu key before relying on this.", "Turn on", destructive=True):
                return True
            self.run(["no-escape", "on", "--yes"])
        else:
            self.run(["no-escape", "off"])
        return True
