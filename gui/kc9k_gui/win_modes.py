# Window: modes, an opened block, its lists and quota tab.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _ModesAndBlockView:
    """Methods of Window (window.py); no state of its own."""

    # Modes (§3.2, §3.8, §4.2; owner 2026-11-17) -------------------------
    def mode_card(self, mode):
        """A mode on the Blocks page: its name (opens it), its switch, what
        switches it, its blocks; blocks dropped on it go in."""
        name = mode["name"]
        title = Gtk.Button(label=name, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                           tooltip_text="Open the mode")
        title.get_style_context().add_class("kc9k-card-name")
        title.connect("clicked", lambda _b: self.open_mode(name))
        icon = Gtk.Image.new_from_icon_name("weather-clear-night-symbolic" if mode.get("monk") else
                                            "weather-snow-symbolic" if mode.get("freeze") else
                                            "view-grid-symbolic", Gtk.IconSize.LARGE_TOOLBAR)
        top = Gtk.Box(spacing=10)
        top.pack_start(icon, False, False, 0)
        top.pack_start(title, False, False, 0)
        switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        blocks = mode.get("blocks") or []
        if mode.get("freeze"):
            left = mode.get("freeze_left") or 0
            switch.set_active(bool(mode.get("running")))
            switch.set_sensitive(not mode.get("running"))
            switch.set_tooltip_text(f"Frozen for {fmt_seconds(left)} more: it can't end early" if left else
                                    "Freeze the computer until a time you choose (at most a week)")
            state_text = f"On, {fmt_seconds(left)} left" if left else "Off"
        elif mode.get("monk"):
            # R-MONK-3 (owner 2026-11-17, later): switched like any mode;
            # only a run for a set time is locked.
            left = mode.get("monk_left") or 0
            switch.set_active(bool(mode.get("active")) or bool(left))
            switch.set_sensitive(not left)
            switch.set_tooltip_text(f"Locked for {fmt_seconds(left)} more: it was started for a time" if left else
                                    "Switch monk mode off" if mode.get("active") else
                                    "Switch monk mode on: no websites and no internet while it runs")
            state_text = (f"Locked, {fmt_seconds(left)} left" if left else
                          ("Running" if mode.get("running") else "On, waiting for its windows or places")
                          if mode.get("active") else "Off")
        else:
            on, total = mode.get("on", 0), len(blocks)
            run_left = mode.get("run_left") or 0
            switch.set_active(total > 0 and on == total)
            switch.set_sensitive(total > 0 and not run_left)
            switch.set_tooltip_text(f"Run by a Mode action, locked for {fmt_seconds(run_left)} more" if run_left else
                                    "Switch its blocks off (locked ones stay on)" if on == total and total else
                                    "Switch its blocks on (without locks)")
            state_text = (f"Running, locked, {fmt_seconds(run_left)} left" if run_left else
                          "No blocks yet" if not total else "On" if on == total else "Off" if not on else
                          f"{on} of {total} on")
        switch.connect("state-set", self.toggle_mode_card, mode)
        state = Gtk.Label(label=state_text)
        state.get_style_context().add_class("dim-label")
        top.pack_end(switch, False, False, 0)
        top.pack_end(state, False, False, 0)
        if mode.get("freeze"):
            what = ("No scheduled freezes: switched on with an end time" if mode.get("continuous") else
                    "Scheduled: " + schedule_summary(mode.get("windows") or []).split(" · ")[0])
        elif mode.get("monk"):
            what = ("No windows or places: on until switched off" if mode.get("continuous") else
                    "Automated: " + (", ".join(mode.get("schedules") or []) or schedule_summary(mode.get("windows") or [])
                                     .split(" · ")[0] or "") + (" · at " + ", ".join(mode["places"]) if mode.get("places") else ""))
        else:
            what = ("Continuous" if mode.get("continuous") else "Automated") + " · " + (
                f"{len(blocks)} {'block' if len(blocks) == 1 else 'blocks'}: {', '.join(blocks)}" if blocks else
                "drag blocks here")
            if mode.get("locked"):
                what += f" · {mode['locked']} locked"
        line = Gtk.Button(label=what, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                          tooltip_text="Open the mode: its blocks and what switches them")
        line.get_style_context().add_class("kc9k-link")
        line.connect("clicked", lambda _b: self.open_mode(name))
        box = card(top, line, spacing=6)
        if not mode.get("monk") and not mode.get("freeze"):
            box.drag_dest_set(Gtk.DestDefaults.ALL, [BLOCK_TARGET], Gdk.DragAction.MOVE)
            box.connect("drag-data-received",
                        lambda _w, _c, _x, _y, data, _i, _t: self.move_to_mode(data.get_data().decode(), name))
        return box

    def create_mode(self, _button, block=None):
        name = self.ask_text("New mode", "A name, such as Evening or Work. Then drag blocks onto it; one "
                             "schedule and places switch them together.")
        if not name:
            return
        if block:
            self.run(["mode", "create", name], then=lambda _o: self.move_to_mode(block, name))
        else:
            self.run(["mode", "create", name], then=lambda _o: self.open_mode(name))

    def move_to_mode(self, block, mode):
        """R-MODE-1: the block gets the mode's automation (refused if that
        weakens it while it is locked)."""
        if not block or block not in self.blocks:
            return
        if self.blocks[block].get("mode") == mode:
            return
        if self.confirm(f"Put {block} into {mode}?",
                        f"It then has the mode's automation: its own windows and places give way to "
                        f"{mode}'s. Taken out later, it keeps the mode's.", "Put it in"):
            self.block_cards_shown = None
            self.run(["mode", "add", mode, block])

    def take_out_of_mode(self, block):
        self.block_cards_shown = None
        self.run(["mode", "take-out", block])

    def toggle_mode_card(self, switch, wanted, mode):
        if self.updating:
            return False
        self.block_cards_shown = None
        if mode.get("freeze"):
            if wanted:
                self.freeze_dialog()
            return True
        if mode.get("monk") and wanted:
            when = ("while its windows and places have it on" if not mode.get("continuous") else
                    "until you switch it off")
            if not self.confirm_lock([f"monk mode, switched on: no websites and no internet {when}"]):
                return True
            self.run(["mode", "on", mode["name"], "--yes"])
        else:
            self.run(["mode", "on" if wanted else "off", mode["name"]])
        return True

    def find_mode(self, name):
        for m in (self.state or {}).get("modes") or []:
            if m["name"] == name:
                return m
        return None

    def open_mode(self, name):
        self.shown_mode = name
        # Made a moment ago: the state that has it is on its way.
        self.mode_wait = None if self.find_mode(name) else name
        self.stack.set_visible_child_name("blocks")
        self.blocks_stack.set_visible_child_name("mode")
        self.update_mode_view()

    def build_mode_view(self):
        scroll, box = page_box()
        back = Gtk.Button(label="← All blocks", halign=Gtk.Align.START, relief=Gtk.ReliefStyle.NONE)
        back.get_style_context().add_class("kc9k-link")
        back.connect("clicked", lambda _b: self.blocks_stack.set_visible_child_name("cards"))
        box.pack_start(back, False, False, 0)
        head = Gtk.Box(spacing=10)
        self.mode_title = Gtk.Label(xalign=0)
        self.mode_title.get_style_context().add_class("kc9k-page-title")
        head.pack_start(self.mode_title, False, False, 0)
        self.mode_menu = Gtk.MenuButton(relief=Gtk.ReliefStyle.NONE, valign=Gtk.Align.CENTER, tooltip_text="More")
        self.mode_menu.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.MENU))
        menu = Gtk.Menu()
        for label, action in (("Rename…", self.rename_mode), ("Delete…", self.delete_mode),
                              ("Save automation as preset…", lambda: self.save_preset(
                                  "automation", getattr(self, "shown_mode", None), mode=True)),
                              ("Use an automation preset…", lambda: self.add_from_preset(
                                  "automation", mode=getattr(self, "shown_mode", None)))):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", lambda _i, a=action: a())
            menu.append(item)
        menu.show_all()
        self.mode_menu.set_popup(menu)
        head.pack_end(self.mode_menu, False, False, 0)
        # R-MODE-3 (owner 2026-11-17, later): a mode of blocks is locked by
        # locking all its blocks at once.
        self.mode_lock_button = Gtk.Button(label="Lock…", valign=Gtk.Align.CENTER)
        self.mode_lock_button.set_tooltip_text("Lock every block of the mode: it then can't be switched off or "
                                               "weakened until they are unlocked")
        self.mode_lock_button.connect("clicked", lambda _b: self.lock_mode())
        head.pack_end(self.mode_lock_button, False, False, 0)
        self.mode_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.mode_switch.connect("state-set", lambda sw, wanted: self.toggle_mode_card(
            sw, wanted, self.find_mode(getattr(self, "shown_mode", None)) or {"name": ""}))
        head.pack_end(self.mode_switch, False, False, 0)
        box.pack_start(head, False, False, 0)
        self.mode_status = dim("")
        box.pack_start(self.mode_status, False, False, 0)
        # A mode of blocks: its blocks as cards.
        self.mode_block_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        add = Gtk.Button(label="Add blocks…")
        add.connect("clicked", self.add_blocks_to_mode)
        self.mode_take_out_button = take_out = Gtk.Button(label="Take out…")
        take_out.set_tooltip_text("Take a block out of the mode")
        take_out.connect("clicked", self.take_blocks_out_of_mode)
        mode_buttons = Gtk.Box(spacing=6)
        mode_buttons.pack_start(add, False, False, 0)
        mode_buttons.pack_start(take_out, False, False, 0)
        self.mode_blocks_section = section(
            "Its blocks", self.mode_block_cards, mode_buttons, plain=True,
            help="They share the mode's automation below: changing it changes all of them, and a change "
                 "that weakens one of them while it is locked is refused for all. Each keeps its own lists, "
                 "apps, breaks and locks. Take out… takes one out again.")
        box.pack_start(self.mode_blocks_section, False, False, 0)
        # Monk mode: what it is, and starting it for a time.
        self.mode_monk_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.mode_monk_box.pack_start(dim(
            "Every website is blocked and the internet connection is cut (Wi-Fi off, Ethernet down, firewall "
            "closed). Switch it on like any mode: with windows or places below it runs in them by itself, "
            "without them until you switch it off. Started for a time it is locked: it can't be switched off "
            "until then. While its places decide, Wi-Fi stays on to see where you are; the firewall still "
            "cuts it."), False, False, 0)
        monk_now = Gtk.Button(label="Run now, locked for a time…", halign=Gtk.Align.START)
        monk_now.connect("clicked", self.monk_mode)
        self.mode_monk_box.pack_start(monk_now, False, False, 0)
        box.pack_start(self.mode_monk_box, False, False, 0)
        self.mode_freeze_box = self.guarded("device", FROZEN_CHICKEN, self.build_device())
        box.pack_start(self.mode_freeze_box, False, False, 0)
        self.mode_editors = Gtk.Stack()
        self.mode_editor = AutomationEditor(self, "mode")
        self.monk_editor = AutomationEditor(self, "monk")
        self.mode_editors.add_named(self.mode_editor, "mode")
        self.mode_editors.add_named(self.monk_editor, "monk")
        # §3.9 Custom action (owner 2026-11-17): what the mode opens or runs
        # each time it becomes enabled.
        self.mode_custom = CustomEditor(self, on_change=self.set_mode_custom)
        self.mode_custom_section = section(
            "When it becomes enabled", dim("Open sites, files and folders, or run programs, each time the mode "
                                           "becomes enabled (also once at sign-in, when it is enabled then)."),
            self.mode_custom)
        box.pack_start(self.mode_custom_section, False, False, 0)
        # R-MODE-5 (owner 2026-11-17): the mode's wallpapers, laid out as
        # circadian rhythm's, and which win (one setting with circadian
        # rhythm's Override modes).
        self.mode_wallpapers = WallpaperEditor(self, lambda: ["mode", "wallpaper", getattr(self, "shown_mode", None) or ""])
        override_row = Gtk.Box(spacing=12)
        override_row.pack_start(Gtk.Label(label="Override circadian rhythm wallpapers", xalign=0), False, False, 0)
        self.mode_override = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.mode_override.connect("state-set", self.toggle_mode_override)
        override_row.pack_end(self.mode_override, False, False, 0)
        self.mode_wallpaper_section = section(
            "Wallpapers", dim("While the mode is enabled, each picture is the wallpaper between its two times."),
            self.mode_wallpapers, override_row,
            help="Drag a bar to move it, or an end to change when it starts or stops (Shift: to the minute). "
                 "Of several enabled modes, the one enabled last shows its wallpaper. Override circadian "
                 "rhythm wallpapers is one setting for all modes, the same as Override modes on the "
                 "circadian rhythm page the other way round. A mode switched by places shows its wallpaper "
                 "once KC9K knows where you are; until then (no Wi-Fi, the first seconds after starting or "
                 "waking) the wallpaper default in Settings shows.")
        box.pack_start(self.mode_wallpaper_section, False, False, 0)
        self.mode_automation_section = section(
            "Automation", self.mode_editors,
            help="Its windows (its own, and those of schedules applied to it) and its places. Disabled by "
                 "default: they turn its blocks on, and they are lifted otherwise; enabled by default: they "
                 "turn them off. With windows and places, either or both at once. Monk mode is always "
                 "disabled by default.")
        box.pack_start(self.mode_automation_section, False, False, 0)
        self.shown_mode = None
        return scroll

    def update_mode_view(self):
        mode = self.find_mode(getattr(self, "shown_mode", None))
        if mode is None and getattr(self, "mode_wait", None) == getattr(self, "shown_mode", None):
            return
        if mode is None:
            self.blocks_stack.set_visible_child_name("cards")
            return
        self.mode_wait = None
        monk = bool(mode.get("monk"))
        frozen = bool(mode.get("freeze"))
        self.mode_title.set_text(mode["name"])
        self.mode_wallpapers.fill(mode.get("wallpapers") or [])
        was, self.updating = self.updating, True
        self.mode_override.set_active(not (self.state.get("wallpaper") or {}).get("override_modes"))
        self.updating = was
        self.mode_freeze_box.set_visible(frozen)
        self.mode_automation_section.set_visible(not frozen)
        self.mode_custom_section.set_visible(not frozen)
        if not frozen and (mode.get("custom") or []) != self.mode_custom.items:
            self.mode_custom.set_items(mode.get("custom") or [])
        if frozen:
            # §3.8 (owner 2026-11-17): on only with an end time; a running
            # freeze can't end.
            was, self.updating = self.updating, True
            self.mode_switch.set_active(bool(mode.get("running")))
            self.mode_switch.set_sensitive(not mode.get("running"))
            self.updating = was
            for widget in (self.mode_menu, self.mode_lock_button, self.mode_blocks_section, self.mode_monk_box):
                widget.set_visible(False)
            left = mode.get("freeze_left") or 0
            self.mode_status.set_text(f"Frozen for {fmt_seconds(left)} more; it can't end early, only be extended."
                                      if left else "Off. Switching it on asks until when (at most a week).")
            return
        # The switch, as on the mode's card (set without running anything).
        was, self.updating = self.updating, True
        blocks = mode.get("blocks") or []
        if monk:
            self.mode_switch.set_active(bool(mode.get("active")) or bool(mode.get("monk_left")))
            self.mode_switch.set_sensitive(not mode.get("monk_left"))
        else:
            self.mode_switch.set_active(bool(blocks) and mode.get("on", 0) == len(blocks))
            self.mode_switch.set_sensitive(bool(blocks) and not mode.get("run_left"))
        self.updating = was
        self.mode_menu.set_visible(not monk)
        self.mode_lock_button.set_visible(not monk)
        self.mode_lock_button.set_sensitive(bool(mode.get("blocks")))
        self.mode_blocks_section.set_visible(not monk)
        self.mode_monk_box.set_visible(monk)
        if monk:
            left = mode.get("monk_left") or 0
            self.mode_status.set_text(
                (f"Locked for {fmt_seconds(left)} more (started for a time). " if left else "") +
                ("Switched on: it runs until you switch it off." if mode.get("active") and mode.get("continuous") else
                 "Switched on: its windows and places run it." if mode.get("active") else
                 "Switched off."))
        else:
            blocks = mode.get("blocks") or []
            self.mode_take_out_button.set_sensitive(bool(blocks))
            self.mode_status.set_text((f"Run by a Mode action: locked for {fmt_seconds(mode['run_left'])} more. "
                                       if mode.get("run_left") else "") +
                                      (f"{mode.get('on', 0)} of {len(blocks)} blocks on" +
                                       (f", {mode['locked']} locked" if mode.get("locked") else "") + "."
                                       if blocks else "No blocks yet: drag blocks onto its card, or add them here."))
            shown = json.dumps([self.blocks.get(b) for b in blocks], sort_keys=True)
            if getattr(self, "mode_blocks_shown", None) != (mode["name"], shown):
                self.mode_blocks_shown = (mode["name"], shown)
                for child in self.mode_block_cards.get_children():
                    self.mode_block_cards.remove(child)
                for b in blocks:
                    if b in self.blocks:
                        self.mode_block_cards.pack_start(self.block_card(self.blocks[b]), False, False, 0)
                self.mode_block_cards.show_all()
        editor = self.monk_editor if monk else self.mode_editor
        self.mode_editors.set_visible_child(editor)
        editor.show_data(mode["name"], mode)

    CREATE_NEW = 1  # the Add blocks… dialog's Create new block… answer

    def add_blocks_to_mode(self, _button, answer=None):
        """Ticks blocks to put into the shown mode, or Create new block…
        (§4.2, owner 2026-11-17), which makes one there. answer: for tests,
        CREATE_NEW and the new block's name."""
        mode = getattr(self, "shown_mode", None)
        if not mode:
            return
        names = [n for n, b in self.blocks.items() if not b.get("draft_of") and b.get("mode") != mode]
        dialog = Gtk.Dialog(title=f"Add blocks to {mode}", transient_for=self, modal=True)
        create = dialog.add_button("Create new block…", self.CREATE_NEW)
        create.set_tooltip_text(f"A new, empty block in {mode}")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Add", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim("They get the mode's automation; their own windows and places give way to it."
                           if names else "Every block is in this mode already."), False, False, 0)
        checks = {}
        for n in names:
            other = self.blocks[n].get("mode")
            checks[n] = Gtk.CheckButton(label=n + (f" (now in {other})" if other else ""))
            box.pack_start(checks[n], False, False, 0)
        ok.set_sensitive(False)
        ok.set_visible(bool(names))
        for check in checks.values():
            check.connect("toggled", lambda _c: ok.set_sensitive(any(c.get_active() for c in checks.values())))
        self.mode_add_dialog = (dialog, checks)  # for tests
        if answer is None:
            dialog.show_all()
            ok.set_visible(bool(names))
            reply = dialog.run()
        else:
            reply = answer[0]
        chosen = [n for n, c in checks.items() if c.get_active()]
        dialog.destroy()
        if reply == self.CREATE_NEW:
            name = answer[1] if answer is not None else self.ask_text(
                f"New block in {mode}", "A short name, such as social-media. It gets the mode's automation.")
            if name:
                self.mode_blocks_shown = None
                self.run(["block", "create", name],
                         then=lambda _o: self.run(["mode", "add", mode, name]))
        elif reply == Gtk.ResponseType.OK and chosen:
            self.mode_blocks_shown = None
            self.run(["mode", "add", mode, *chosen])

    def take_blocks_out_of_mode(self, _button):
        """The keyboard's way to take a block out of the shown mode (R-GUI-3;
        the cards have no ⋯ menu)."""
        mode = self.find_mode(getattr(self, "shown_mode", None)) or {}
        name = self.choose("Take a block out", f"It leaves {mode.get('name')} and keeps the automation it has now.",
                           mode.get("blocks") or [])
        if name:
            self.mode_blocks_shown = None
            self.take_out_of_mode(name)

    def toggle_mode_override(self, _switch, wanted):
        """R-MODE-5: modes' wallpapers over circadian rhythm's, or the other
        way round, for all modes."""
        if not self.updating:
            self.run(["wallpaper", "priority", "modes" if wanted else "circadian"])
        return False

    def set_mode_custom(self, items):
        """What the shown mode opens or runs when it becomes enabled: the
        list as it is now (clear, then each added)."""
        mode = getattr(self, "shown_mode", None)
        if not mode:
            return
        commands = [["mode", "custom", mode, "clear"]] + [["mode", "custom", mode, "add", item] for item in items]

        def step(rest):
            if rest:
                self.run(rest[0], then=lambda _o: step(rest[1:]))
        step(commands)

    def lock_mode(self):
        """R-MODE-3: locks every block of the shown mode with the chosen
        locks (any kind), confirmed once (R-LOCK-12)."""
        mode = self.find_mode(getattr(self, "shown_mode", None)) or {}
        if not mode.get("blocks") or mode.get("monk"):
            return
        picker = LockPicker("switch the mode off", "switched off", LockPicker.ALL_KINDS, relock_any=True)
        asked = self.lock_dialog(
            f"Lock {mode['name']}",
            f"Every block in it ({', '.join(mode['blocks'])}) gets these locks, after the ones it has. While "
            "they are on, the mode can't be switched off or weakened.", "Lock", picker, need_lock=True)
        if asked is None:
            return
        chosen, relock_flags, secret = asked
        if not chosen:
            return
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        args, locks = ["mode", "lock", mode["name"]], []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"mode '{mode['name']}' (each of its blocks): {order}{text}")
        args += relock_flags
        locks += relock_lines(f"mode '{mode['name']}' (each of its blocks)", relock_flags)
        if self.confirm_lock(locks):
            self.run(args + ["--yes"], secret + "\n" if secret else None)

    def add_from_preset(self, kind, mode=None):
        """Uses a preset on the opened block (or a mode): lists, or an
        automation or schedule preset."""
        kinds = ("lists",) if kind == "lists" else ("automation", "schedule")
        names = [f"{p['name']} ({p['kind']})" for p in self.state.get("presets") or [] if p["kind"] in kinds]
        if not names:
            self.message("No such presets yet: save one first.", Gtk.MessageType.INFO)
            return
        chosen = self.choose("Use a preset", "It is copied in; what would weaken a locked block is refused.", names)
        if not chosen:
            return
        name, kind = chosen.rsplit(" (", 1)
        target = mode or self.selected
        args = ["preset", "use", name, target, "--kind", kind.rstrip(")")] + (["--mode"] if mode else [])
        self.run(args, then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def rename_mode(self):
        mode = getattr(self, "shown_mode", None)
        name = mode and self.ask_text("Rename the mode", initial=mode)
        if name and name != mode:
            self.run(["mode", "rename", mode, name], then=lambda _o: setattr(self, "shown_mode", name))

    def delete_mode(self):
        mode = getattr(self, "shown_mode", None)
        if mode and self.confirm(f"Delete the mode {mode}?", "Its blocks stay, with the automation they "
                                 "have now.", "Delete", destructive=True):
            self.blocks_stack.set_visible_child_name("cards")
            self.run(["mode", "delete", mode])

    def choose(self, title, detail, names):
        """One of names, picked in a dialog; None when cancelled."""
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        if detail:
            box.pack_start(dim(detail), False, False, 0)
        combo = Gtk.ComboBoxText()
        for n in names:
            combo.append_text(n)
        combo.set_active(0)
        box.pack_start(combo, False, False, 0)
        self.choose_dialog = (dialog, combo)  # for tests
        dialog.show_all()
        answer = dialog.run()
        value = combo.get_active_text()
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK else None

    def open_automation(self, name=None):
        """The automation view of a block (the opened one): from its menu
        (§4.3); a block in a mode opens the mode instead (R-MODE-1)."""
        name = name or self.selected
        block = self.blocks.get(name) or {}
        if not block:
            return
        if block.get("mode"):
            self.open_mode(block["mode"])
            return
        if name != self.selected or self.blocks_stack.get_visible_child_name() not in ("detail", "automation"):
            self.open_block(name)
        if self.selected != name:
            return  # stayed with unsaved edits elsewhere
        self.automation_title.set_text(f"Automation of {name}")
        self.blocks_stack.set_visible_child_name("automation")

    def open_block(self, name, tab=None):
        """Opens a block on the Blocks page, at a tab when given; another
        block's unsaved edits are asked about first (§4.3)."""
        if tab in ("Automation", "Schedule & places"):
            self.open_automation(name)
            return
        if (self.selected != name and self.blocks_stack.get_visible_child_name() == "detail"
                and not self.leave_block(to_cards=False)):
            return
        self.selected = self.blocks.get(name, {}).get("draft_of") or name
        self.blocks_stack.set_visible_child_name("detail")
        self.stack.set_visible_child_name("blocks")
        self.update_block_list()
        if tab is not None:
            for number in range(self.block_tabs.get_n_pages()):
                if self.block_tabs.get_tab_label_text(self.block_tabs.get_nth_page(number)) == tab:
                    self.block_tabs.set_current_page(number)

    def toggle_block_card(self, switch, wanted, name):
        """On: switched on without locks; off: switched off (refused while
        locked, and the switch goes back)."""
        if self.updating:
            return False
        self.selected = name
        self.block_cards_shown = None  # redrawn from the state after it, refused or not
        self.run(["start", name] if wanted else ["stop", name])
        return True  # the switch follows the state after the command

    def card_locks(self, name):
        """Locked: the unlock flow; else the start/add-locks dialog. On the
        block itself, never its draft (§4.3: a draft never runs)."""
        self.selected = name
        block = self.blocks.get(name) or {}
        (self.unlock_block if block.get("held") else self.add_locks)(None)

    def build_quota_tab(self):
        """Breaks & allowance (R-GUI-3, redesigned 2026-11-17): a card each,
        a line saying what it is (the rest behind ?), rows of a label and its
        fields, one primary button."""
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18, margin=12)
        labels = Gtk.SizeGroup(mode=Gtk.SizeGroupMode.HORIZONTAL)
        # §4.3: what was set here is kept in the draft until Save edits.
        self.quota_kept_label = Gtk.Label(xalign=0, wrap=True, no_show_all=True)
        box.pack_start(self.quota_kept_label, False, False, 0)

        def form_row(text, *widgets):
            row = Gtk.Box(spacing=8)
            label = Gtk.Label(label=text, xalign=0, valign=Gtk.Align.START, margin_top=6)
            labels.add_widget(label)
            row.pack_start(label, False, False, 0)
            for widget in widgets:
                row.pack_start(widget, False, False, 0)
            return row

        def buttons(primary, *others):
            row = Gtk.Box(spacing=6)
            primary.get_style_context().add_class("suggested-action")
            for button in (primary, *others):
                row.pack_start(button, False, False, 0)
            return row

        # §3.6 allowance: per day, week, month, rolling 24 hours or window,
        # refilled at a set time; sessions.
        self.quota_minutes = DurationField(1800, minimum=60, maximum=86400, width=9)
        self.quota_per = Gtk.ComboBoxText()
        for key, label in (("day", "per day"), ("week", "per week"), ("month", "per month"),
                           ("rolling", "per 24 hours, rolling"), ("window", "per window")):
            self.quota_per.append(key, label)
        self.quota_per.set_active_id("day")
        self.quota_per.connect("changed", lambda combo: self.quota_minutes.set_limits(
            maximum={"week": 7 * 86400, "month": 31 * 86400}.get(combo.get_active_id(), 86400)))
        self.quota_refill = Gtk.Entry(text="00:00", width_chars=5, max_length=5)
        quota_row = form_row("Time", self.quota_minutes, self.quota_per,
                             Gtk.Label(label="refill at", valign=Gtk.Align.START, margin_top=6), self.quota_refill)
        self.quota_sessions = NumberField(0, 0, 100, width=4, hint="0: any number")
        sessions_row = form_row("Sessions", self.quota_sessions)
        self.quota_set = Gtk.Button(label="Save allowance")
        self.quota_set.connect("clicked", self.set_quota)
        self.quota_start = Gtk.Button(label="Use allowance now")
        self.quota_start.connect("clicked", self.start_quota)
        self.quota_end = Gtk.Button(label="End session")
        self.quota_end.connect("clicked", self.end_quota)
        self.quota_remove = Gtk.Button(label="Remove")
        self.quota_remove.connect("clicked", self.remove_quota)
        self.quota_label = dim("")
        box.pack_start(section(
            "Allowance", dim("Time to use what the block blocks, in sessions you open."),
            quota_row, sessions_row, buttons(self.quota_set, self.quota_start, self.quota_end, self.quota_remove),
            self.quota_label,
            help="Sessions you open lift the block until the period's time is spent. A session only "
                 "spends time while the block is in use: its app in front, its site in the focused "
                 "tab, or playing sound or video. An allowance protects nothing by itself: changes "
                 "apply at once until the block is locked; then it only takes less time or fewer "
                 "sessions. Per window: each opening of a block window is a period."), False, False, 0)

        self.work_minutes = DurationField(25 * 60, minimum=60, maximum=86400, width=8)
        self.break_minutes = DurationField(5 * 60, minimum=60, maximum=86400, width=8)
        pomodoro_row = form_row("Work", self.work_minutes,
                                Gtk.Label(label="then a break of", valign=Gtk.Align.START, margin_top=6),
                                self.break_minutes)
        set_pomodoro = Gtk.Button(label="Save")
        set_pomodoro.connect("clicked", self.set_pomodoro)
        clear_pomodoro = Gtk.Button(label="Turn off")
        clear_pomodoro.connect("clicked", self.clear_pomodoro)
        self.long_check = Gtk.CheckButton(valign=Gtk.Align.START, margin_top=6,
                                          tooltip_text="A long break now and then, instead of the short one")
        self.long_minutes = DurationField(15 * 60, minimum=60, maximum=86400, width=8)
        self.long_every = NumberField(4, 2, 20, width=4)
        long_row = form_row("Long break", self.long_check, self.long_minutes,
                            Gtk.Label(label="every", valign=Gtk.Align.START, margin_top=6), self.long_every,
                            Gtk.Label(label="work periods", valign=Gtk.Align.START, margin_top=6))
        box.pack_start(section(
            "Pomodoro", dim("Work and break take turns while the block is on, work first."),
            pomodoro_row, long_row, buttons(set_pomodoro, clear_pomodoro),
            help="While the block is on, it pauses for the break after each work period; its rhythm "
                 "starts again each time the block is enabled (R-SCHED-3). During a work period the "
                 "block's allowance can't be used. Locks stay on through the breaks."), False, False, 0)

        # §3.6 reward: saved up to one break, accumulating until a period
        # ends, or each part kept for a time the user picks (no upper limit).
        self.allowance_minutes = DurationField(600, minimum=60, maximum=4 * 3600, width=8)
        self.allowance_per = DurationField(3600, minimum=300, maximum=86400, width=8)
        allowance_row = form_row("Earn", self.allowance_minutes,
                                 Gtk.Label(label="of break for every", valign=Gtk.Align.START, margin_top=6),
                                 self.allowance_per,
                                 Gtk.Label(label="of blocking", valign=Gtk.Align.START, margin_top=6))
        self.reward_keep = Gtk.ComboBoxText()
        for key, label in (("one", "up to one break"), ("window", "until the window ends"),
                           ("day", "until the day ends"), ("week", "until the week ends"),
                           ("month", "until the month ends"), ("time", "for a time after it is earned:")):
            self.reward_keep.append(key, label)
        self.reward_keep.set_active_id("one")
        self.reward_keep_time = DurationField(7 * 86400, minimum=3600, width=8)
        self.reward_keep_time.set_no_show_all(True)
        self.reward_keep.connect("changed", lambda combo: self.reward_keep_time.set_visible(
            combo.get_active_id() == "time"))
        keep_row = form_row("Keep it", self.reward_keep, self.reward_keep_time)
        save_reward = Gtk.Button(label="Save reward")
        save_reward.connect("clicked", self.set_allowance)
        remove_reward = Gtk.Button(label="Remove")
        remove_reward.connect("clicked", self.clear_allowance)
        self.break_start = Gtk.Button(label="Take a break now")
        self.break_start.connect("clicked", self.start_break)
        self.break_end = Gtk.Button(label="End break")
        self.break_end.connect("clicked", self.end_break)
        self.break_label = dim("")
        box.pack_start(section(
            "Reward", dim("Break time earned while the block works, to take when you choose."),
            allowance_row, keep_row, buttons(save_reward, remove_reward, self.break_start, self.break_end),
            self.break_label,
            help="Break time is earned while the block is on and the computer is in use, and kept as "
                 "chosen: up to one break, until the window, day, week or month ends, or each part "
                 "for as long as you choose. Take it when you choose, also while the block is locked; "
                 "the block comes back by itself. Allowing more breaks weakens the block, so decide "
                 "before locking it."), False, False, 0)

        self.relock_check = Gtk.CheckButton(label="Unlocking gives a break of")
        self.relock_minutes = DurationField(5 * 60, minimum=60, maximum=4 * 3600, width=8)
        relock_save = Gtk.Button(label="Save")
        relock_save.connect("clicked", self.set_relock)
        relock_row = form_row("Text break", self.relock_check, self.relock_minutes,
                              Gtk.Label(label="then the block is back with its locks", valign=Gtk.Align.START,
                                        margin_top=6), relock_save)
        # §3.6 delay break: asked for, it begins after a countdown.
        self.delay_break_check = Gtk.CheckButton(label="Delay break of")
        self.delay_break_minutes = DurationField(600, minimum=60, maximum=4 * 3600, width=8)
        self.delay_wait_minutes = DurationField(1800, minimum=60, maximum=86400, width=8)
        delay_save = Gtk.Button(label="Save")
        delay_save.connect("clicked", self.set_delay_break)
        self.delay_request = Gtk.Button(label="Request break")
        self.delay_request.connect("clicked", lambda _b: self.selected and self.run(["break", "request", self.selected]))
        delay_row = form_row("Delay break", self.delay_break_check, self.delay_break_minutes,
                             Gtk.Label(label="after waiting", valign=Gtk.Align.START, margin_top=6),
                             self.delay_wait_minutes, delay_save, self.delay_request)
        box.pack_start(section(
            "Breaks that keep the locks", dim("A break after a challenge or a countdown; the block stays locked."),
            relock_row, delay_row,
            help="Text break: passing the text lock's challenge or giving the password doesn't end the "
                 "block's lock, it lifts the block for a while; unlocking again during that break ends "
                 "it. Delay break: asked for, it begins after the wait you chose."), False, False, 0)
        for row in (quota_row, sessions_row, pomodoro_row, long_row, allowance_row, keep_row, relock_row, delay_row):
            top_align(row)
            flow_row(row)
        return box

    def update_block_list(self):
        # §4.3: a draft shows in its block's place, never in the list; the
        # page keeps the block itself selected.
        names = [n for n in self.blocks if not self.blocks[n].get("draft_of")]
        self.selected = self.blocks.get(self.selected, {}).get("draft_of") or self.selected
        shown = self.selected
        rows = self.block_list.get_children()
        if [getattr(r, "block_name", None) for r in rows] != names or any(
                r.active != self.blocks[r.block_name]["active"] for r in rows):
            for row in rows:
                self.block_list.remove(row)
            for name in names:
                block = self.blocks[name]
                row = Gtk.ListBoxRow()
                row.block_name, row.active = name, block["active"]
                inner = Gtk.Box(spacing=8, margin=8)
                dot = Gtk.Label(label="●" if block["active"] else "○")
                inner.pack_start(dot, False, False, 0)
                inner.pack_start(Gtk.Label(label=name, xalign=0, ellipsize=Pango.EllipsizeMode.END), True, True, 0)
                row.add(inner)
                row.set_tooltip_text("Active" if block["active"] else "Inactive")
                self.block_list.add(row)
            self.block_list.show_all()
        if self.selected not in self.blocks:
            if self.selected is not None and self.blocks_stack.get_visible_child_name() == "detail":
                # The opened block is gone (deleted elsewhere): back to the
                # list rather than opening another block (§4.2).
                self.blocks_stack.set_visible_child_name("cards")
            self.selected = shown = names[0] if names else None
        for row in self.block_list.get_children():
            if row.block_name == shown:
                self.block_list.select_row(row)
        self.update_block_cards()
        self.show_block()

    def block_selected(self, _box, row):
        if row is None or self.updating:
            return
        self.selected = row.block_name
        self.show_block()

    def start_draft(self, _button, then=None):
        """§4.3: the block's draft, made at its first edit; with a block
        password, that password (the draft keeps it)."""
        name = self.selected
        block = self.blocks.get(name) or {}
        if not name:
            return
        password = self.block_passwords.get(name)
        if block.get("block_password") and not password:
            password = self.ask_text(f"Edit '{name}'", "Its block password.", password=True)
            if not password:
                return
        if password:
            self.block_passwords[name] = self.block_passwords[draft_of_name(name)] = password
        self.run(["block", "draft", name], password + "\n" if password else None, then=then)

    def has_edits(self, name=None):
        """Whether a block (the opened one) has unsaved edits: a draft."""
        name = name or self.selected
        return bool(name and (self.blocks.get(name) or {}).get("draft"))

    def finish_draft(self, how, name=None):
        """§4.3: Save edits applies the draft to its block (weakening parts
        refused on a locked block, and named); Discard drops it."""
        name = name or self.selected
        if self.has_edits(name):
            self.run(["block", "draft", name, how])

    def ask_unsaved(self, name):
        """Leaving a block with unsaved edits: "save", "cancel" (discard)
        or None (stay)."""
        dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                   text=f"Save the edits to {name}?",
                                   secondary_text="Its websites and apps change only when you save them.")
        dialog.add_button("Discard", 2)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        save = dialog.add_button("Save edits", Gtk.ResponseType.OK)
        save.get_style_context().add_class("suggested-action")
        answer = dialog.run()
        dialog.destroy()
        return {Gtk.ResponseType.OK: "save", 2: "cancel"}.get(answer)

    def leave_block(self, to_cards=True):
        """Leaves the opened block (back to the list unless to_cards is
        False); unsaved edits are asked about first. False: stay."""
        name = self.selected
        if self.has_edits(name) and self.blocks_stack.get_visible_child_name() == "detail":
            answer = self.ask_unsaved(name)
            if answer is None:
                return False
            self.finish_draft(answer, name)
        if to_cards:
            self.blocks_stack.set_visible_child_name("cards")
        return True

    def show_block(self):
        block = self.blocks.get(self.selected)
        self.detail.set_sensitive(block is not None)
        if block is None:
            self.title_label.set_text("No blocks yet")
            for child in self.status_pills.get_children():
                self.status_pills.remove(child)
            self.status_label.set_text("Create a block, add sites or apps, then start it.")
            for widget in (self.apps, self.titles, self.schedules, self.autostarts):
                widget.set_items([])
            self.update_lists([])
            self.calendar.set_windows([])
            self.quota_label.set_text("")
            self.block_nepsis_label.set_text("")
            return
        self.title_label.set_text(block["name"])
        for child in self.status_pills.get_children():
            self.status_pills.remove(child)
        for text, kind in block_pills(block):
            self.status_pills.pack_start(pill(text, kind), False, False, 0)
        self.status_pills.show_all()
        self.status_label.set_text(block_summary(block))
        users = block.get("users") or []
        self.users_button.set_label("Users: " + (", ".join(users) if users else "all"))
        self.users_button.set_visible(len(self.state.get("accounts") or []) > 1 or bool(users))
        self.hidden_bar.set_visible(bool(block.get("hidden")))
        # §4.3 (owner 2026-11-17): the block is always a draft: its Websites
        # and Apps tabs show the draft once there is one (made at the first
        # edit); Save edits applies it, Discard drops it.
        content = self.blocks.get(block.get("draft") or "") or block
        editing = content is not block
        self.draft_bar.set_visible(editing)
        self.draft_label.set_text("Unsaved edits to its websites and apps: they apply when you save them."
                                  + (" What would weaken it while it is locked is refused then, and named."
                                     if block.get("held") else "") if editing else "")
        self.update_lists(content.get("lists", []), content.get("phrase_sets", []))
        self.apps.set_items(content["apps"], owner=block["name"])
        self.titles.set_items(content.get("titles", []), owner=block["name"])
        triggers = content.get("triggers") or {}
        self.app_triggers.set_actions(block["name"], triggers.get("apps", [{"name": "close"}]))
        self.title_triggers.set_actions(block["name"], triggers.get("titles", [{"name": "close"}]))
        self.schedules.set_items(block["schedules"])
        self.update_automation(block)
        in_mode = block.get("mode")
        self.block_mode_bar.set_visible(bool(in_mode))
        self.block_mode_label.set_text(f"In the mode {in_mode}: it has the mode's automation, changed on the "
                                       f"mode for all its blocks. Its own is kept for when it is taken out."
                                       if in_mode else "")
        self.automation_button.set_sensitive(not in_mode)
        self.automation_button.set_tooltip_text(f"Automated by '{in_mode}'" if in_mode else
                                                "What switches it by itself: its windows and places")
        self.automation_title.set_text(f"Automation of {block['name']}")
        if in_mode and self.blocks_stack.get_visible_child_name() == "automation":
            self.blocks_stack.set_visible_child_name("detail")
        for widget in self.block_auto_widgets:
            widget.set_sensitive(not in_mode)
        for child in self.block_applied.get_children():
            self.block_applied.remove(child)
        for sched in block.get("applied_schedules") or []:
            chip = Gtk.Box(spacing=2)
            chip.get_style_context().add_class("kc9k-pill")
            chip.get_style_context().add_class("on")
            chip.pack_start(Gtk.Label(label=sched), False, False, 0)
            x = Gtk.Button.new_from_icon_name("window-close-symbolic", Gtk.IconSize.MENU)
            x.set_relief(Gtk.ReliefStyle.NONE)
            x.set_tooltip_text("Take this schedule off the block")
            x.connect("clicked", lambda _b, sc=sched, b=block["name"]: self.run(["schedule", "take-off", b, sc]))
            chip.pack_start(x, False, False, 0)
            self.block_applied.pack_start(chip, False, False, 0)
        if not block.get("applied_schedules"):
            self.block_applied.pack_start(dim("none"), False, False, 0)
        self.block_applied.show_all()
        self.autostarts.set_items(block.get("autostarts") or [])
        others = [w for other in self.state.get("blocks", []) if other["name"] != block["name"]
                  for w in other.get("schedule_windows") or [] if not w.get("allow")]
        self.calendar.set_windows(block.get("schedule_windows", []), others)
        set_switch(self.keyword_test_switch, content.get("keyword_test", False))
        self.update_social(content)
        covered = set()
        for window in block.get("schedule_windows", []):
            if not window.get("allow"):
                for day in range(7):
                    if window["days"] & (1 << day):
                        covered.update((day, minute) for minute in range(window["start"], window["end"]))
        self.schedule_total.set_text(f"{len(covered) / 60:.0f} of 168 hours a week" if covered else "")
        settings = block.get("automation") or {}
        self.block_places.to_choose = getattr(self, "place_to_show", None)
        self.place_to_show = None
        self.block_places.show_places(block.get("places") or [], "this block", bool(settings.get("default_on")),
                                      bool(settings.get("need_both")), bool(block.get("schedules")))
        if not self.schedule_lead.has_focus():
            self.schedule_lock.set_active(block.get("schedule_lock", False))
            self.schedule_lead.set_value(block.get("schedule_lead", 0))
        allowance, balance = block.get("break_allowance", 0), block.get("break_balance", 0)
        if allowance and not self.reward_keep.has_focus() and not self.reward_keep_time.entry.has_focus():
            keep = block.get("break_keep") or "one"
            if keep.isdigit():
                self.reward_keep_time.set_seconds(int(keep))
                keep = "time"
            self.reward_keep.set_active_id(keep)
        paused = block.get("paused_seconds", 0)
        self.break_start.set_sensitive(block["active"] and allowance > 0 and balance >= 60 and not paused)
        self.break_end.set_sensitive(bool(paused))
        if paused:
            text = f"On a break for {fmt_seconds(paused)} more."
        elif allowance:
            text = (f"Reward: {allowance // 60} min per {block.get('break_per', 0) // 60} min of blocking. "
                    f"Earned: {balance // 60} min.")
        else:
            text = "No reward."
        if block.get("relock"):
            text += f" Unlocking gives a break of {block['relock'] // 60} min."
        if block.get("delay_break_in"):
            text += f" Its delay break begins in {fmt_seconds(block['delay_break_in'])}."
        self.break_label.set_text(text)
        if not self.delay_break_minutes.has_focus() and not self.delay_wait_minutes.has_focus():
            self.delay_break_check.set_active(bool(block.get("delay_break")))
            if block.get("delay_break"):
                self.delay_break_minutes.set_value(block["delay_break"] // 60)
                self.delay_wait_minutes.set_value(block.get("delay_wait", 0) // 60)
        self.delay_request.set_sensitive(bool(block.get("delay_break")) and not block.get("delay_break_in") and not paused)

        self.start_button.set_label("Add locks…" if block["active"] else "Start")
        self.start_button.set_sensitive(True)
        # §3.4: unlocking lifts locks (a delay lock starts counting down) and
        # leaves the block on; stopping needs every lock lifted.
        self.stop_button.set_sensitive(block["active"] and not block["held"])
        self.unlock_button.set_sensitive(
            (block["char_locked"] or block["password_locked"]
             or (block["delay_seconds"] > 0 and not block.get("delay_left")))
            and not block["lock_seconds"] and not block["restart_locked"])
        self.extend_button.set_sensitive(block["active"] and "timer" in (block.get("lock_order") or []))
        self.delete_button.set_sensitive(not block["active"] and not block["held"])

        limit = block["quota_limit"]
        self.quota_start.set_sensitive(limit > 0 and not block["quota_open"] and block["quota_used"] < limit)
        self.quota_end.set_sensitive(block["quota_open"])
        self.quota_remove.set_sensitive(limit > 0)
        if limit and not self.quota_refill.has_focus() and not self.quota_minutes.has_focus():
            self.quota_per.set_active_id(block.get("quota_period") or "day")
            self.quota_refill.set_text(block.get("quota_refill") or "00:00")
            self.quota_sessions.set_value(block.get("quota_sessions", 0))
            self.quota_minutes.set_value(limit // 60)
        self.quota_label.set_text(
            f"Used this {block.get('quota_period') or 'day'}: {block['quota_used'] // 60} of {limit // 60} min"
            + (f" · sessions {block.get('quota_sessions_used', 0)} of {block['quota_sessions']}"
               if block.get("quota_sessions") else "")
            + (" · session open" if block["quota_open"] else "") if limit else "No allowance set.")
        self.block_nepsis_label.set_text(describe_nepsis(block.get("nepsis"), block.get("nepsis_count")
                                                         if block["active"] else None))
        self.block_nepsis_off_button.set_visible(bool((block.get("nepsis") or {}).get("steps")))
        # §4.3: settings of these two tabs kept in the draft until Save edits.
        kept = block.get("draft_kept") or []
        for label, items in ((self.quota_kept_label, [k for k in kept if not k.startswith("Bypass prevention")]),
                             (self.nepsis_kept_label, [k for k in kept if k.startswith("Bypass prevention")])):
            label.set_markup("<b>Not saved yet</b> (applies with Save edits): " +
                             GLib.markup_escape_text("; ".join(items)) if items else "")
            label.set_visible(bool(items))

    def update_lists(self, lists, phrase_sets=()):
        """Shows the selected block's lists and phrase sets, keeping the cards
        of those still there, so a refresh doesn't disturb what is being
        typed. Phrase sets are keyed by minus their ID, as in the rules."""
        lists = list(lists) + [dict(p, id=-p["id"], kind="phrases") for p in phrase_sets]
        # A list keeps its card when the block's draft takes over (other
        # IDs, same names): what is being typed stays (§4.3).
        by_name = {(c.data.get("kind") == "phrases", c.data["name"].lower()): i for i, c in self.list_cards.items()}
        for entry in lists:
            old = by_name.get((entry["kind"] == "phrases", entry["name"].lower()))
            if old is not None and old != entry["id"] and entry["id"] not in self.list_cards:
                self.list_cards[entry["id"]] = self.list_cards.pop(old)
        ids = [entry["id"] for entry in lists]
        for list_id in list(self.list_cards):
            if list_id not in ids:
                card = self.list_cards.pop(list_id)
                card.get_parent().remove(card)
        for entry in lists:
            card = self.list_cards.get(entry["id"])
            if card is None:
                card = (PhraseSetCard if entry["kind"] == "phrases" else ListCard)(self, entry)
                self.list_cards[entry["id"]] = card
                self.list_boxes[entry["kind"]].pack_start(card, False, False, 0)
                card.show_all()
            card.update(entry)

    def create_list(self, _button, kind):
        if not self.selected:
            return
        if kind == "phrases":
            self.create_phrase_set()
            return
        dialog, entry, checks = self.list_dialog(kind)
        dialog.show_all()
        answer = dialog.run()
        args = self.list_create_args(kind, entry, checks)
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and args:
            self.run(args)

    def list_dialog(self, kind):
        """The New list dialog: a name and the three scope boxes (§3.2).
        Returns the dialog, the name entry and the boxes by place."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL,
            text="New exception list" if kind == "exceptions" else "New blacklist",
            secondary_text="A name, and where it looks. Sites (reddit.com), URL entries and patterns "
                           "(*casino*) match the URL; words (long cat) match words in the URL, title "
                           "or text. One list can hold both.")
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        area = dialog.get_message_area()
        area.pack_start(entry, False, False, 0)
        checks = {}
        boxes = Gtk.Box(spacing=12)
        for name, title in LIST_PLACES + (() if kind == "exceptions" else (AD_PLACE,)):
            checks[name] = Gtk.CheckButton(label=title, active=name == "url")
            boxes.pack_start(checks[name], False, False, 0)
        area.pack_start(boxes, False, False, 0)
        ok = dialog.get_widget_for_response(Gtk.ResponseType.OK)
        def sync(*_):
            ok.set_sensitive(bool(entry.get_text().strip()) and any(c.get_active() for c in checks.values()))
        entry.connect("changed", sync)
        for check in checks.values():
            check.connect("toggled", sync)
        sync()
        return dialog, entry, checks

    def list_create_args(self, kind, entry, checks):
        """The kc9k command the New list dialog runs, or None without a
        name or a scope box."""
        name = entry.get_text().strip()
        places = [place for place, check in checks.items() if check.get_active()]
        if not self.selected or not name or not places:
            return None
        args = ["list", "create", self.selected, name] + ["--" + place for place in places]
        if kind == "exceptions":
            args.append("--exceptions")
        return args

    def create_phrase_set(self):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL, text="New phrase set",
            secondary_text="A name, and where its phrases are looked for.")
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        area = dialog.get_message_area()
        area.pack_start(entry, False, False, 0)
        checks = {}
        for name, title in LIST_PLACES:
            # Titles too by default: a phrase in a page's title should count.
            checks[name] = Gtk.CheckButton(label=title, active=name in ("title", "text"))
            area.pack_start(checks[name], False, False, 0)
        entry.set_placeholder_text("optional")
        dialog.show_all()
        answer = dialog.run()
        name = entry.get_text().strip()
        places = [place for place, check in checks.items() if check.get_active()]
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        if not name:
            # §4.3: an unnamed phrase set gets a name.
            name = self.free_phrase_set_name()
        self.run(["phrase-set", "create", self.selected, name] + ["--" + p for p in places])

    def free_phrase_set_name(self):
        """'Phrase set N', the first N no phrase set of the block has
        (names compare case-insensitively)."""
        block = self.blocks.get(self.selected) or {}
        taken = {p["name"].lower() for p in block.get("phrase_sets") or []}
        n = 1
        while f"phrase set {n}" in taken:
            n += 1
        return f"Phrase set {n}"
