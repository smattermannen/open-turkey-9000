# Window: Statistics page.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _StatsPage:
    """Methods of Window (window.py); no state of its own."""

    # Statistics ----------------------------------------------------------
    def build_stats(self):
        scroll, box = page_box()
        page_header(box, "Statistics", "Time on sites and apps, and how often pages were blocked. Only on this computer.")

        controls = Gtk.Box(spacing=12)
        self.stats_period = Gtk.ComboBoxText()
        for days, label in ((1, "Today"), (7, "Last 7 days"), (30, "Last 30 days"), (90, "Last 90 days")):
            self.stats_period.append(str(days), label)
        self.stats_period.append("custom", "Custom…")  # §4.9: presets and custom
        self.stats_period.set_active_id("7")
        self.stats_range, self.stats_last_period = None, "7"
        self.stats_period.connect("changed", self.stats_period_changed)
        self.stats_range_button = Gtk.Button()
        self.stats_range_button.set_tooltip_text("Change the time frame")
        self.stats_range_button.connect("clicked", lambda _b: self.choose_stats_range() and self.refresh_stats())
        self.stats_range_button.set_no_show_all(True)
        kinds = Gtk.Box()
        kinds.get_style_context().add_class("linked")
        self.kind_sites = Gtk.RadioButton.new_with_label(None, "Websites")
        self.kind_apps = Gtk.RadioButton.new_with_label_from_widget(self.kind_sites, "Apps")
        self.kind_blocked = Gtk.RadioButton.new_with_label_from_widget(self.kind_sites, "Times blocked")
        for button in (self.kind_sites, self.kind_apps, self.kind_blocked):
            button.set_mode(False)
            button.connect("toggled", lambda b: b.get_active() and self.update_stats())
            kinds.pack_start(button, False, False, 0)
        # §4.9: one user's time, or everyone's.
        self.stats_user = Gtk.ComboBoxText()
        self.stats_user.append("", "All users")
        self.stats_user.set_active_id("")
        self.stats_user.connect("changed", lambda c: not self.updating and self.refresh_stats())
        # §4.9: top 5 or 10, search, one block's sites and apps.
        self.stats_top = Gtk.ComboBoxText()
        for n in ("5", "10"):
            self.stats_top.append(n, f"Top {n}")
        self.stats_top.set_active_id("5")
        self.stats_top.connect("changed", lambda _c: self.update_stats())
        self.stats_search = Gtk.SearchEntry(placeholder_text="Find a site or app", width_chars=18)
        self.stats_search.connect("search-changed", lambda _e: self.refresh_stats())
        self.stats_block = Gtk.ComboBoxText()
        self.stats_block.append("", "All blocks")
        self.stats_block.set_active_id("")
        self.stats_block.connect("changed", lambda c: not self.updating and self.refresh_stats())
        # Two rows (R-GUI-3: each wraps on a narrow window): what time and
        # whose, what kind; then which. The period, its time frame and the
        # user chooser stay together (hidden ones leave no gap).
        when = Gtk.Box(spacing=6)
        when.pack_start(self.stats_period, False, False, 0)
        when.pack_start(self.stats_range_button, False, False, 0)
        when.pack_start(self.stats_user, False, False, 0)
        controls.pack_start(when, False, False, 0)
        controls.pack_start(kinds, False, False, 0)
        which = Gtk.Box(spacing=12)
        which.pack_start(self.stats_top, False, False, 0)
        which.pack_start(self.stats_block, False, False, 0)
        which.pack_start(self.stats_search, False, False, 0)
        box.pack_start(controls, False, False, 0)
        box.pack_start(which, False, False, 0)
        flow_row(controls)
        flow_row(which)
        self.series_slots = {}  # name -> palette slot: a site keeps its colour

        self.stats_off = Gtk.InfoBar(message_type=Gtk.MessageType.INFO)
        self.stats_off.get_content_area().add(
            Gtk.Label(label="Statistics are off, so nothing new is recorded. Turn them on under Recording, below.", xalign=0))
        self.stats_off.set_no_show_all(True)
        box.pack_start(self.stats_off, False, False, 0)

        tiles = Gtk.Box(spacing=40)
        self.tile_total = StatTile("Total")
        self.tile_average = StatTile("Average per day")
        self.tile_count = StatTile("Different websites")
        for tile in (self.tile_total, self.tile_average, self.tile_count):
            tiles.pack_start(tile, False, False, 0)
        box.pack_start(tiles, False, False, 0)

        self.daily_chart = DailyChart()
        self.daily_legend = Legend()
        self.daily_section = section("Per day", self.daily_chart, self.daily_legend)
        box.pack_start(self.daily_section, False, False, 0)

        self.top_chart = TopChart()
        self.top_empty = dim("Nothing recorded in this period.")
        self.top_title = heading("Most used", "medium")
        top_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        top_box.pack_start(self.top_title, False, False, 0)
        top_box.pack_start(self.top_chart, False, False, 0)
        top_box.pack_start(self.top_empty, False, False, 0)
        box.pack_start(top_box, False, False, 0)

        box.pack_start(dim(
            "App time counts the app in the focused window; website time counts the active "
            "tab in the browser. Time behind a locked screen or after two "
            "minutes without input isn't counted. Everything stays on this computer."), False, False, 0)

        self.stats_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.stats_switch.connect("state-set", self.toggle_stats)
        self.stats_private_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.stats_private_switch.connect("state-set", self.toggle_stats_private)
        clear = Gtk.Button(label="Delete all statistics…", halign=Gtk.Align.START)
        clear.get_style_context().add_class("destructive-action")
        clear.connect("clicked", self.clear_stats)
        box.pack_start(section(
            "Recording",
            self.switch_row(self.stats_switch, "Record time spent on websites and apps",
                            "Stored only on this computer and kept for a year."),
            self.switch_row(self.stats_private_switch, "Leave out private windows",
                            "Time on websites in private windows isn't counted."),
            clear), False, False, 0)
        return scroll

    def update_stats_setting(self):
        set_switch(self.stats_switch, self.state["stats"])
        self.update_stats_blocks()
        set_switch(self.stats_private_switch, self.state.get("stats_skip_private", False))

    def stats_frame(self):
        """The Statistics page's first and last day (§4.9 presets and custom)."""
        today = datetime.date.today()
        period = self.stats_period.get_active_id() if hasattr(self, "stats_period") else "7"
        if period == "custom" and self.stats_range:
            return self.stats_range
        days = int(period) if (period or "").isdigit() else 7
        return today - datetime.timedelta(days=days - 1), today

    def stats_period_changed(self, combo):
        if self.updating:
            return
        if combo.get_active_id() == "custom":
            if not self.choose_stats_range():
                was, self.updating = self.updating, True
                combo.set_active_id(self.stats_last_period)  # cancelled: as before
                self.updating = was
                return
        else:
            self.stats_last_period = combo.get_active_id()
        self.stats_range_button.set_visible(combo.get_active_id() == "custom")
        self.refresh_stats()

    def choose_stats_range(self):
        """§4.9 custom time frame: from one day to another, within the year
        statistics are kept. Returns whether one was chosen."""
        today = datetime.date.today()
        first, last = self.stats_range or (today - datetime.timedelta(days=29), today)
        dialog = Gtk.Dialog(title="Time frame", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Show", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(10)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(18)
        box.pack_start(dim("Statistics are kept for a year."), False, False, 0)
        grid = Gtk.Grid(column_spacing=10, row_spacing=8)
        start, end = DateButton(first), DateButton(last)
        for row, (label, button) in enumerate((("From", start), ("To", end))):
            grid.attach(Gtk.Label(label=label, xalign=0), 0, row, 1, 1)
            grid.attach(button, 1, row, 1, 1)
        box.pack_start(grid, False, False, 0)
        note = Gtk.Label(xalign=0, wrap=True, max_width_chars=40)
        note.get_style_context().add_class("kc9k-note")
        box.pack_start(note, False, False, 0)

        def check():
            problem = stats_range_problem(start.day, end.day, datetime.date.today())
            days = (end.day - start.day).days + 1
            note.set_text(problem or ("1 day" if days == 1 else f"{days} days"))
            context = note.get_style_context()
            (context.add_class if problem else context.remove_class)("error")
            ok.set_sensitive(problem is None)

        start.on_change(check)
        end.on_change(check)
        check()
        self.range_dialog = (dialog, start, end, note, ok)  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen = (start.day, end.day)
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or stats_range_problem(*chosen, datetime.date.today()):
            return False
        self.stats_range = chosen
        self.stats_range_button.set_label(range_words(*chosen))
        return True

    def refresh_stats(self):
        if self.locked("stats") and self.locked("dashboard"):
            return True
        passwords = list(self.access_passwords)
        who = self.stats_user.get_active_id() if hasattr(self, "stats_user") else ""
        user = ["--user", who] if who else []
        first, last = self.stats_frame()
        if hasattr(self, "stats_period") and self.stats_period.get_active_id() == "custom":
            frame = ["--from", first.isoformat(), "--to", last.isoformat()]
        else:
            frame = ["--days", str((last - first).days + 1)]
        # §4.9: the page's search and block filter. The dashboard needs
        # today, unfiltered: one read when the page's is that, else two.
        filters, secret = [], None
        if hasattr(self, "stats_search"):
            search = self.stats_search.get_text().strip()
            block = self.stats_block.get_active_id() or ""
            if search:
                filters += ["--search", search]
            if block:
                filters += ["--block", block]
                if block in self.block_passwords:
                    secret = self.block_passwords[block] + "\n"

        page_args = ["stats", "--json", *frame, *user, *filters]
        same = not filters and last == datetime.date.today()

        def work():
            page = cli_with_access(passwords, page_args, secret)
            dashboard = page if same else cli_with_access(passwords, ["stats", "--json", "--days", "1", *user])
            GLib.idle_add(self.apply_stats, *dashboard, *page)

        threading.Thread(target=work, daemon=True).start()
        return True

    def apply_stats(self, ok, output, page_ok=None, page_output=None):
        """The rows read: output for the dashboard (today at least),
        page_output for the Statistics page (its time frame and filters)."""
        if not ok:
            return False
        if page_ok is None:
            page_ok, page_output = ok, output
        data = json.loads(output)
        self.stats_rows = data["rows"]
        self.stats_on = data["enabled"]
        if page_ok:
            self.stats_page_rows = json.loads(page_output)["rows"]
        elif "block password" in (page_output or "") and hasattr(self, "stats_block"):
            self.message("That block's sites and apps are behind its block password: open the block "
                         "and use Show contents… first.", Gtk.MessageType.INFO)
            self.stats_block.set_active_id("")  # reads again, without it
        else:
            self.message(page_output or "The statistics could not be read.", Gtk.MessageType.WARNING)
        users = [""] + (data.get("users") or [])
        if hasattr(self, "stats_user") and users != getattr(self, "stats_users_shown", [""]):
            chosen = self.stats_user.get_active_id() or ""
            was, self.updating = self.updating, True
            self.stats_user.remove_all()
            for u in users:
                self.stats_user.append(u, u or "All users")
            self.stats_user.set_active_id(chosen if chosen in users else "")
            self.updating = was
            self.stats_users_shown = users
        self.stats_user.set_visible(len(users) > 2) if hasattr(self, "stats_user") else None
        if not self.locked("stats"):
            self.update_stats()
        if not self.locked("dashboard"):
            self.update_dashboard_stats()
        return False

    def assign_series(self, names):
        """Palette slots for names (§4.9: colour follows the site or app,
        never its rank): a name keeps the slot it had; a new one takes a
        free slot, one no remembered name holds if there is one; never two
        shown names in one colour; past seven: Other (None)."""
        out, used = {}, set()
        for name in names:
            slot = self.series_slots.get(name)
            if slot is not None and slot not in used:
                out[name] = slot
                used.add(slot)
        remembered = set(self.series_slots.values())
        for name in names:
            if name in out:
                continue
            free = [slot for slot in range(len(SERIES_LIGHT)) if slot not in used]
            if not free:
                out[name] = None
                continue
            slot = ([f for f in free if f not in remembered] or free)[0]
            out[name] = slot
            used.add(slot)
            # The slot is this name's now: whoever had it before forgets it.
            self.series_slots = {n: s for n, s in self.series_slots.items() if s != slot}
            self.series_slots[name] = slot
        return out

    def update_stats(self):
        kind = "site" if self.kind_sites.get_active() else "blocked" if self.kind_blocked.get_active() else "app"
        # §4.9: times blocked are counts, not seconds.
        fmt = fmt_count if kind == "blocked" else fmt_spent
        self.daily_chart.fmt = self.top_chart.fmt = fmt
        first, last = self.stats_frame()
        dates = [first + datetime.timedelta(days=n) for n in range((last - first).days + 1)]
        days = len(dates)
        per_day = {d.isoformat(): 0 for d in dates}
        by_name = {d.isoformat(): {} for d in dates}
        totals = {}
        for row in self.stats_page_rows:
            if row["kind"] != kind or row["day"] not in per_day:
                continue
            per_day[row["day"]] += row["seconds"]
            by_name[row["day"]][row["name"]] = by_name[row["day"]].get(row["name"], 0) + row["seconds"]
            totals[row["name"]] = totals.get(row["name"], 0) + row["seconds"]
        total = sum(per_day.values())

        self.stats_off.set_visible(not self.stats_on)
        if not self.stats_on:
            self.stats_off.show_all()
        self.tile_total.set_value(fmt(total))
        self.tile_average.set_value(fmt(total // days))
        self.tile_average.set_visible(days > 1)
        self.tile_count.get_children()[0].set_text(
            {"site": "Different websites", "app": "Different apps", "blocked": "Different sites blocked"}[kind])
        self.tile_count.set_value(str(len(totals)))
        self.daily_section.set_visible(days > 1)

        ranked = sorted(totals.items(), key=lambda item: (-item[1], item[0]))
        top_n = int(self.stats_top.get_active_id() or 5)
        top = ranked[:top_n]
        # The chart: the top names, in at most seven colours, the rest Other.
        named = [name for name, _v in top][:len(SERIES_LIGHT)]
        slots = self.assign_series(named)
        series = [(name, slots[name]) for name in named if slots[name] is not None]
        shown = {name for name, _slot in series}
        stacks = []
        for d in dates:
            day = by_name[d.isoformat()]
            stack = {name: day.get(name, 0) for name in shown}
            rest = sum(v for n, v in day.items() if n not in shown)
            if rest:
                stack["Other"] = rest
            stacks.append(stack)
        if any("Other" in stack for stack in stacks):
            series.append(("Other", None))
        self.daily_chart.set_stacks([(d, per_day[d.isoformat()]) for d in dates], stacks, series)
        self.daily_legend.set_series(series)

        self.top_title.set_markup(
            f"<span size='medium' weight='bold'>"
            f"{ {'site': 'Most used websites', 'app': 'Most used apps', 'blocked': 'Most blocked sites'}[kind] }</span>")
        self.top_chart.colors = {name: slots.get(name) for name, _v in top}
        self.top_chart.set_items(top)
        self.top_chart.set_visible(bool(top))
        self.top_empty.set_visible(not top)

    def update_stats_blocks(self):
        """The block filter's choices: the blocks there are."""
        names = sorted(n for n, b in self.blocks.items() if not b.get("draft_of"))
        if getattr(self, "stats_blocks_shown", None) == names:
            return
        chosen = self.stats_block.get_active_id() or ""
        was, self.updating = self.updating, True
        self.stats_block.remove_all()
        self.stats_block.append("", "All blocks")
        for name in names:
            self.stats_block.append(name, name)
        self.stats_block.set_active_id(chosen if chosen in names else "")
        self.updating = was
        self.stats_blocks_shown = names
