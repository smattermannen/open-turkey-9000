# Window: Schedules and Places pages.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _SchedulesPlacesPages:
    """Methods of Window (window.py); no state of its own."""

    # Schedules and places pages (§4.5; owner 2026-11-17) ---------------
    def build_schedules_page(self):
        scroll, box = page_box()
        page_header(box, "Schedules", "Weekly windows kept outside blocks, such as Evening or Weekend. Apply "
                    "them to blocks and modes; changing one here changes everything using it.",
                    help="A block or mode can use any number of schedules beside its own windows (Apply "
                         "schedule… in its Automation). Where they overlap they count once. A change "
                         "that weakens a locked block using the schedule is refused as a whole. Deleting a "
                         "schedule leaves its windows to what used it, as its own.")
        self.schedule_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.pack_start(self.schedule_cards, False, False, 0)
        new = Gtk.Button(label="New schedule…", halign=Gtk.Align.START)
        new.get_style_context().add_class("suggested-action")
        new.connect("clicked", self.create_schedule)
        box.pack_start(new, False, False, 0)
        self.schedule_editor = AutomationEditor(self, "schedule")
        self.schedule_editor_title = heading("", "medium")
        self.schedule_editor_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, no_show_all=True)
        self.schedule_editor_box.pack_start(self.schedule_editor_title, False, False, 0)
        self.schedule_editor_box.pack_start(card(self.schedule_editor), False, False, 0)
        box.pack_start(self.schedule_editor_box, False, False, 0)
        self.shown_schedule, self.schedule_cards_shown = None, None
        return scroll

    def update_schedules_page(self):
        schedules = self.state.get("schedules") or []
        names = [sc["name"] for sc in schedules]
        if self.shown_schedule not in names:
            self.shown_schedule = names[0] if names else None
        key = json.dumps(schedules, sort_keys=True) + str(self.shown_schedule)
        if key != self.schedule_cards_shown:
            self.schedule_cards_shown = key
            for child in self.schedule_cards.get_children():
                self.schedule_cards.remove(child)
            for sc in schedules:
                self.schedule_cards.pack_start(self.schedule_card(sc), False, False, 0)
            if not schedules:
                self.schedule_cards.pack_start(dim("No schedules yet. Make Evening, Weekend or Work hours "
                                                   "once, then apply them to any block or mode."), False, False, 0)
            self.schedule_cards.show_all()
        chosen = next((sc for sc in schedules if sc["name"] == self.shown_schedule), None)
        self.schedule_editor_box.set_visible(chosen is not None)
        if chosen is not None:
            self.schedule_editor_title.set_text(chosen["name"])
            self.schedule_editor.show_all()
            self.schedule_editor_title.show()
            self.schedule_editor_box.get_children()[1].show()
            self.schedule_editor.show_data(chosen["name"], chosen)

    def schedule_card(self, sc):
        name = sc["name"]
        title = Gtk.Button(label=name, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                           tooltip_text="Show and change its windows")
        title.get_style_context().add_class("kc9k-card-name")
        title.connect("clicked", lambda _b: self.open_schedule(name))
        top = Gtk.Box(spacing=10)
        top.pack_start(title, False, False, 0)
        more = Gtk.MenuButton(relief=Gtk.ReliefStyle.NONE, valign=Gtk.Align.CENTER, tooltip_text="More")
        more.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.MENU))
        menu = Gtk.Menu()
        for label, action in (("Rename…", lambda: self.rename_schedule(name)),
                              ("Delete…", lambda: self.delete_schedule(name))):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", lambda _i, a=action: a())
            menu.append(item)
        menu.show_all()
        more.set_popup(menu)
        top.pack_end(more, False, False, 0)
        if name == self.shown_schedule:
            top.pack_end(pill("Shown below", "on"), False, False, 0)
        used = (sc.get("blocks") or []) + [f"mode {m}" for m in sc.get("modes") or []]
        lines = [dim(schedule_summary(sc.get("windows") or [])),
                 dim("Used by " + ", ".join(used) if used else "Not used yet")]
        return card(top, *lines, spacing=4)

    def open_schedule(self, name):
        self.shown_schedule = name
        self.stack.set_visible_child_name("schedules")
        self.schedule_cards_shown = None
        self.update_schedules_page()

    def create_schedule(self, _button):
        name = self.ask_text("New schedule", "A name, such as Evening or Weekend. Then give it windows below.")
        if name:
            self.run(["schedule", "create", name], then=lambda _o: self.open_schedule(name))

    def rename_schedule(self, name):
        new = self.ask_text("Rename the schedule", initial=name)
        if new and new != name:
            self.run(["schedule", "rename", name, new], then=lambda _o: self.open_schedule(new))

    def delete_schedule(self, name):
        if self.confirm(f"Delete the schedule {name}?", "Blocks and modes using it keep its windows as their "
                        "own: nothing they block changes.", "Delete", destructive=True):
            self.run(["schedule", "delete", name])

    def build_places_page(self):
        scroll, box = page_box()
        page_header(box, "Places", "Where the computer is, known by the Wi-Fi networks in range. Blocks and "
                    "modes are switched by being at one.",
                    help="A place is the set of Wi-Fi networks in range when you save it: yours and the "
                         "neighbours'. The computer is there when as many of them as you choose are in range "
                         "again. Add places to a block or mode in its Automation. Needing more networks "
                         "makes a place harder to recognise: refused while a block using it is locked. With "
                         "no network seen, places count as whatever keeps their blocks enabled.")
        save = Gtk.Button(label="Save where I am now as…", halign=Gtk.Align.START)
        save.get_style_context().add_class("suggested-action")
        save.connect("clicked", lambda _b: self.save_place_alone())
        box.pack_start(save, False, False, 0)
        self.place_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.pack_start(self.place_cards, False, False, 0)
        self.place_cards_shown = None
        return scroll

    def update_places_page(self):
        places = self.state.get("places") or []
        key = json.dumps(places, sort_keys=True)
        if key == self.place_cards_shown:
            return
        self.place_cards_shown = key
        for child in self.place_cards.get_children():
            self.place_cards.remove(child)
        for p in places:
            self.place_cards.pack_start(self.place_card(p), False, False, 0)
        if not places:
            self.place_cards.pack_start(dim("No places yet. Save where the computer is now: home, the "
                                            "library, work."), False, False, 0)
        self.place_cards.show_all()

    def place_card(self, p):
        name = p["name"]
        top = Gtk.Box(spacing=10)
        title = Gtk.Label(label=name, xalign=0)
        title.get_style_context().add_class("kc9k-card-name")
        top.pack_start(title, False, False, 0)
        if p.get("here"):
            top.pack_start(pill("You are here", "on"), False, False, 0)
        more = Gtk.MenuButton(relief=Gtk.ReliefStyle.NONE, valign=Gtk.Align.CENTER, tooltip_text="More")
        more.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.MENU))
        menu = Gtk.Menu()
        for label, action in (("Add the networks in range now", lambda: self.run(["place", "add", name], timeout=60)),
                              ("Remove…", lambda: self.remove_place(name))):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", lambda _i, a=action: a())
            menu.append(item)
        menu.show_all()
        more.set_popup(menu)
        top.pack_end(more, False, False, 0)
        need_row = Gtk.Box(spacing=6)
        need_row.pack_start(Gtk.Label(label="There when", valign=Gtk.Align.START, margin_top=6), False, False, 0)
        need = NumberField(p.get("need", 2), 1, max(p.get("networks", 1), 1), width=4)
        need_row.pack_start(need, False, False, 0)
        need_row.pack_start(Gtk.Label(label=f"of its {p.get('networks', 0)} networks are in range",
                                      valign=Gtk.Align.START, margin_top=6), False, False, 0)
        need_save = Gtk.Button(label="Save", valign=Gtk.Align.START)
        need_save.connect("clicked", lambda _b: self.run(["place", "need", name, str(need.get_value_as_int())]))
        guard_button(need_save, need)
        need_row.pack_start(need_save, False, False, 0)
        modes = p.get("modes") or []
        mode_blocks = {b for m in self.state.get("modes") or [] if m["name"] in modes for b in m.get("blocks") or []}
        used = [b for b in p.get("blocks") or [] if b not in mode_blocks] + [f"mode {m}" for m in modes]
        return card(top, flow_row(need_row), dim("Used by " + ", ".join(used) if used else
                                                 "Not used yet: add it to a block or mode in its Automation."),
                    spacing=6)

    def remove_place(self, name):
        if self.confirm(f"Forget the place {name}?", "Blocks and modes using it lose it: refused while that "
                        "weakens a locked block.", "Remove", destructive=True):
            self.run(["place", "remove", name])

    def save_place_alone(self):
        """Saves the networks in range as a place, for no block."""
        name = self.ask_text("Save this place", "A name for where the computer is now, such as home or library. "
                             "Saving under a name that exists adds the networks seen now.")
        if name:
            self.run(["place", "add", name], timeout=60, then=lambda out: self.message(out, Gtk.MessageType.INFO))

    def page_changed(self, *_args):
        row = getattr(self, "nav_rows", {}).get(self.stack.get_visible_child_name())
        if row is not None and self.nav.get_selected_row() is not row:
            self.nav.select_row(row)
        if hasattr(self, "circadian"):
            self.circadian.refresh()
        if self.stack.get_visible_child_name() in ("stats", "dashboard"):
            self.refresh_stats()
        if self.stack.get_visible_child_name() == "account":
            self.refresh_account()
