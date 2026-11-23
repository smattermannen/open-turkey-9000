# Window: Overview page.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _DashboardPage:
    """Methods of Window (window.py); no state of its own."""

    # Dashboard -----------------------------------------------------------
    def build_dashboard(self):
        """§4.1 Overview (R-GUI-3: tasks first): what is on, until when, how
        locked; starting a block is one click away."""
        scroll, box = page_box()
        page_header(box, "Overview", "What is on now, and how it is locked.")

        # The status card: the one line that matters, and the main action.
        hero = card(kind="kc9k-hero", spacing=6)
        top = Gtk.Box(spacing=12)
        texts = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        self.dash_headline = Gtk.Label(xalign=0, wrap=True)
        self.dash_headline.get_style_context().add_class("kc9k-hero-title")
        self.dash_detail = dim("", wrap=True)
        texts.pack_start(self.dash_headline, False, False, 0)
        texts.pack_start(self.dash_detail, False, False, 0)
        top.pack_start(texts, True, True, 0)
        self.dash_start = Gtk.MenuButton(label="Start a block", valign=Gtk.Align.CENTER)
        self.dash_start.get_style_context().add_class("suggested-action")
        self.dash_start_menu = Gtk.Menu()
        self.dash_start.set_popup(self.dash_start_menu)
        top.pack_end(self.dash_start, False, False, 0)
        hero.pack_start(top, False, False, 0)
        self.dash_device = Gtk.Box(spacing=6)
        hero.pack_start(self.dash_device, False, False, 0)
        box.pack_start(hero, False, False, 0)
        # R-DESKTOP-1: on Wayland or another desktop, titles and app time may not work.
        self.dash_desktop = dim("", wrap=True)
        self.dash_desktop.set_no_show_all(True)
        box.pack_start(self.dash_desktop, False, False, 0)

        tiles = Gtk.Box(spacing=12, homogeneous=True)
        self.tile_active = StatTile("Blocks on")
        self.tile_sites = StatTile("Websites today")
        self.tile_apps = StatTile("Apps today")
        for tile in (self.tile_active, self.tile_sites, self.tile_apps):
            tiles.pack_start(card(tile), True, True, 0)
        box.pack_start(tiles, False, False, 0)

        self.dash_blocks = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.dash_blocks.connect("row-activated", self.open_block_from_dashboard)
        on_now = section("On now", self.dash_blocks)
        on_now.get_children()[-1].get_style_context().add_class("kc9k-tight")
        box.pack_start(on_now, False, False, 0)

        self.dash_locks = dim("")
        box.pack_start(section("Device", self.dash_locks), False, False, 0)

        self.dash_top = TopChart()
        self.dash_top_empty = dim("Nothing recorded today yet.")
        box.pack_start(section("Most used today", self.dash_top, self.dash_top_empty), False, False, 0)

        actions = Gtk.Box(spacing=8)
        new_block = Gtk.Button(label="New block…")
        new_block.connect("clicked", self.create_block)
        freeze = Gtk.Button(label="Frozen Chicken…")
        freeze.connect("clicked", lambda _b: self.open_mode(FROZEN_CHICKEN))
        monk = Gtk.Button(label="Monk mode…")
        monk.connect("clicked", self.monk_mode)
        actions.pack_start(new_block, False, False, 0)
        actions.pack_start(monk, False, False, 0)
        actions.pack_start(freeze, False, False, 0)
        box.pack_start(section("Quick actions", actions, plain=True), False, False, 0)
        return scroll

    def start_from_dashboard(self, name):
        self.selected = name
        self.start_block(None)

    def update_dashboard(self):
        warning = self.state.get("desktop_warning") or ""
        self.dash_desktop.set_text(warning)
        self.dash_desktop.set_visible(bool(warning))
        blocks = self.state["blocks"]
        active = [b for b in blocks if b["active"]]
        locked = [b for b in active if b.get("locked") or b.get("lock_seconds") or b.get("char_locked")
                  or b.get("password_locked") or b.get("restart_locked")]
        self.tile_active.set_value(str(len(active)))

        # The headline: how many are on, how many locked, and the longest lock.
        if not active:
            self.dash_headline.set_text("Nothing is on right now")
            self.dash_detail.set_text("Start a block to keep sites and apps away." if blocks else
                                      "Make a block first: the sites, words and apps to keep away.")
        else:
            self.dash_headline.set_text(f"{len(active)} block{'s' if len(active) != 1 else ''} on"
                                        + (f" · {len(locked)} locked" if locked else ""))
            longest = max(active, key=lambda b: b.get("lock_seconds") or 0)
            if longest.get("lock_seconds"):
                until = datetime.datetime.now() + datetime.timedelta(seconds=longest["lock_seconds"])
                self.dash_detail.set_text(f"{longest['name']} is timer-locked until {date_words(until)} "
                                          f"({fmt_seconds(longest['lock_seconds'])}).")
            elif locked:
                self.dash_detail.set_text(f"{', '.join(b['name'] for b in locked)}: locked until unlocked.")
            else:
                self.dash_detail.set_text("Nothing is locked: any block can be stopped.")
        for item in self.dash_start_menu.get_children():
            self.dash_start_menu.remove(item)
        off = [b for b in blocks if not b["active"]]
        for block in off:
            item = Gtk.MenuItem(label=block["name"])
            item.connect("activate", lambda _i, n=block["name"]: self.start_from_dashboard(n))
            self.dash_start_menu.append(item)
        # §4.2 modes (owner 2026-11-17): a mode's blocks switched on at once.
        modes_off = [m for m in self.state.get("modes") or []
                     if not m.get("monk") and m.get("blocks") and m.get("on", 0) < len(m["blocks"])]
        if off and modes_off:
            self.dash_start_menu.append(Gtk.SeparatorMenuItem())
        for mode in modes_off:
            item = Gtk.MenuItem(label=f"Mode: {mode['name']} ({len(mode['blocks'])} blocks)")
            item.connect("activate", lambda _i, n=mode["name"]: self.run(["mode", "on", n]))
            self.dash_start_menu.append(item)
        off = off or modes_off
        if off:
            self.dash_start_menu.append(Gtk.SeparatorMenuItem())
        new_item = Gtk.MenuItem(label="New block…")
        new_item.connect("activate", self.create_block)
        self.dash_start_menu.append(new_item)
        self.dash_start_menu.show_all()
        self.dash_start.set_label("Start a block" if off else "New block")

        for child in self.dash_device.get_children():
            self.dash_device.remove(child)
        device = []
        if self.state["freeze_seconds"]:
            device.append((f"Frozen Chicken {fmt_seconds(self.state['freeze_seconds'])}", "locked"))
        if self.state.get("monk_seconds"):
            device.append((f"Monk mode {fmt_seconds(self.state['monk_seconds'])}", "locked"))
        else:
            # §4.1 today's windows: monk mode run by its automation later today.
            monk = next((m for m in self.state.get("modes") or [] if m.get("monk")), None)
            if monk and monk.get("active") and monk.get("today_windows"):
                device.append(("Monk mode today " + ", ".join(monk["today_windows"]), "lifted"))
        for text, kind in device:
            self.dash_device.pack_start(pill(text, kind), False, False, 0)
        self.dash_device.show_all()

        for row in self.dash_blocks.get_children():
            self.dash_blocks.remove(row)
        if not active:
            row = Gtk.ListBoxRow(activatable=False)
            row.add(dim("No block is on.", wrap=False))
            row.get_child().set_margin_start(16)
            row.get_child().set_margin_top(12)
            row.get_child().set_margin_bottom(12)
            self.dash_blocks.add(row)
        for block in active:
            row = Gtk.ListBoxRow()
            row.block_name = block["name"]
            inner = Gtk.Box(spacing=12, margin_start=16, margin_end=16, margin_top=10, margin_bottom=10)
            texts = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
            name = Gtk.Label(xalign=0)
            name.set_markup(f"<b>{GLib.markup_escape_text(block['name'])}</b>")
            texts.pack_start(name, False, False, 0)
            extra = []
            if block.get("today_windows"):
                extra.append("today: " + ", ".join(block["today_windows"]))
            if block["quota_limit"]:
                extra.append(f"allowance {block['quota_used'] // 60} of {block['quota_limit'] // 60} min")
            if extra:
                texts.pack_start(dim(" · ".join(extra)), False, False, 0)
            inner.pack_start(texts, True, True, 0)
            pills = pill_row(block_pills(block))
            pills.set_valign(Gtk.Align.CENTER)
            inner.pack_end(Gtk.Image.new_from_icon_name("go-next-symbolic", Gtk.IconSize.MENU), False, False, 0)
            inner.pack_end(pills, False, False, 0)
            row.add(inner)
            self.dash_blocks.add(row)
        self.dash_blocks.show_all()

        locks = []
        locks.append("Strict mode on" if self.state["strict"] else "Strict mode off")
        locks.append("no-escape on" if self.state["no_escape"] else "no-escape off")
        if self.state["freeze_seconds"]:
            locks.insert(0, f"Frozen Chicken ({self.state['freeze_mode']}) for {fmt_seconds(self.state['freeze_seconds'])}")
        if self.state.get("monk_seconds"):
            locks.insert(0, f"Monk mode for {fmt_seconds(self.state['monk_seconds'])}")
        line = " · ".join(locks)
        self.dash_locks.set_text(line[:1].upper() + line[1:])

    def open_block_from_dashboard(self, _box, row):
        self.selected = row.block_name
        self.blocks_stack.set_visible_child_name("detail")
        self.stack.set_visible_child_name("blocks")
        self.state_json = None
        self.refresh()

    def update_dashboard_stats(self):
        today = datetime.date.today().isoformat()
        rows = [r for r in self.stats_rows if r["day"] == today]
        sites = sum(r["seconds"] for r in rows if r["kind"] == "site")
        apps = sum(r["seconds"] for r in rows if r["kind"] == "app")
        self.tile_sites.set_value(fmt_spent(sites) if self.stats_on else "off")
        self.tile_apps.set_value(fmt_spent(apps) if self.stats_on else "off")
        top = sorted(((r["name"], r["seconds"]) for r in rows if r["kind"] in ("site", "app")),
                     key=lambda item: -item[1])[:5]
        self.dash_top.set_items(top)
        self.dash_top.set_visible(bool(top))
        self.dash_top_empty.set_visible(not top)
        self.dash_top_empty.set_text(
            "Nothing recorded today yet." if self.stats_on
            else "Statistics are off. Turn them on in Statistics.")
