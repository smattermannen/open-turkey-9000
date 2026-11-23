# Window: Blocks page and block cards.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _BlocksPage:
    """Methods of Window (window.py); no state of its own."""

    # Blocks --------------------------------------------------------------
    def build_blocks(self):
        paned = Gtk.Paned()

        left = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        header = Gtk.Box(spacing=6, margin_start=10, margin_end=6, margin_top=6, margin_bottom=6)
        header.pack_start(heading("Blocks", "medium"), False, False, 0)
        plus = Gtk.Button.new_from_icon_name("list-add-symbolic", Gtk.IconSize.BUTTON)
        plus.set_tooltip_text("New block")
        plus.connect("clicked", self.create_block)
        header.pack_end(plus, False, False, 0)
        left.pack_start(header, False, False, 0)
        self.block_list = Gtk.ListBox()
        self.block_list.connect("row-selected", self.block_selected)
        list_scroll = Gtk.ScrolledWindow(min_content_width=210, vexpand=True)
        list_scroll.add(self.block_list)
        left.pack_start(list_scroll, True, True, 0)
        paned.pack1(left, False, False)
        # Owner 2026-11-17: an opened block shows just that block, in the
        # page's own style; the old block list beside it is gone (the cards
        # and "← All blocks" choose blocks). The list widget stays, hidden.
        left.set_no_show_all(True)
        left.hide()

        self.detail = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10,
                              margin_start=20, margin_end=20, margin_top=16, margin_bottom=16)
        # Owner 2026-11-20 (D1): the bar from "← All blocks" down to the
        # Start/Stop/… row stays put; only the tabs below scroll, so the
        # window fits small and scaled screens. Never wider than the window
        # (R-GUI-3: nothing cut off at the right): the tabs wrap and reflow.
        paned.pack2(self.detail, True, False)
        self.detail_scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
        back = Gtk.Button(label="← All blocks", halign=Gtk.Align.START, relief=Gtk.ReliefStyle.NONE)
        back.get_style_context().add_class("kc9k-link")
        back.connect("clicked", lambda _b: self.leave_block())
        self.detail.pack_start(back, False, False, 0)
        self.title_label = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.END)
        self.title_label.get_style_context().add_class("kc9k-page-title")
        self.status_pills = Gtk.Box(spacing=6)
        self.status_label = dim("")
        self.detail.pack_start(self.title_label, False, False, 0)
        self.detail.pack_start(self.status_pills, False, False, 0)
        self.detail.pack_start(self.status_label, False, False, 0)
        # R-MODE-1 (owner 2026-11-17): a block in a mode has the mode's
        # automation; its own is kept for when it is taken out.
        self.block_mode_bar = Gtk.Box(spacing=8, no_show_all=True)
        self.block_mode_label = Gtk.Label(xalign=0, wrap=True)
        self.block_mode_bar.pack_start(self.block_mode_label, True, True, 0)
        open_mode = Gtk.Button(label="Open the mode")
        open_mode.connect("clicked", lambda _b: self.open_mode((self.blocks.get(self.selected) or {}).get("mode")))
        take_out = Gtk.Button(label="Take it out")
        take_out.set_tooltip_text("It gets its own automation back")
        take_out.connect("clicked", lambda _b: self.selected and self.take_out_of_mode(self.selected))
        self.block_mode_bar.pack_start(open_mode, False, False, 0)
        self.block_mode_bar.pack_start(take_out, False, False, 0)
        for child in self.block_mode_bar.get_children():
            child.show()
        self.detail.pack_start(self.block_mode_bar, False, False, 0)
        # §3.11: a block password hides the contents until typed.
        self.hidden_bar = Gtk.Box(spacing=8, no_show_all=True)
        self.hidden_bar.pack_start(dim("Its lists, apps and window titles are hidden by its block password."),
                                   False, False, 0)
        reveal = Gtk.Button(label="Show contents…")
        reveal.connect("clicked", self.reveal_block)
        reveal.show()
        self.hidden_bar.pack_start(reveal, False, False, 0)
        self.hidden_bar.get_children()[0].show()
        self.detail.pack_start(self.hidden_bar, False, False, 0)
        # §4.3 (owner 2026-11-17): the block is always a draft; its edits
        # apply with Save edits.
        self.draft_bar = Gtk.Box(spacing=8, no_show_all=True)
        self.draft_label = dim("")
        draft_save = Gtk.Button(label="Save edits")
        draft_save.get_style_context().add_class("suggested-action")
        draft_save.connect("clicked", lambda _b: self.finish_draft("save"))
        draft_cancel = Gtk.Button(label="Discard")
        draft_cancel.set_tooltip_text("Drop the edits; the block stays as it is")
        draft_cancel.connect("clicked", lambda _b: self.finish_draft("cancel"))
        for widget in (self.draft_label, draft_save, draft_cancel):
            widget.show()
            self.draft_bar.pack_start(widget, widget is self.draft_label, widget is self.draft_label, 0)
        self.detail.pack_start(self.draft_bar, False, False, 0)

        actions = Gtk.Box(spacing=6)
        self.start_button = Gtk.Button(label="Start")
        self.start_button.get_style_context().add_class("suggested-action")
        self.start_button.connect("clicked", self.start_block)
        self.stop_button = Gtk.Button(label="Stop")
        self.stop_button.connect("clicked", self.stop_block)
        self.unlock_button = Gtk.Button(label="Unlock…")
        self.unlock_button.connect("clicked", self.unlock_block)
        # §4.6: extending a timer lock is its own action (kc9k extend).
        self.extend_button = Gtk.Button(label="Extend timer…")
        self.extend_button.connect("clicked", self.extend_timer)
        # §4.3 (owner 2026-11-17): Automation in the block's menu; greyed
        # out in a mode, which automates it.
        self.automation_button = Gtk.Button(label="Automation")
        self.automation_button.connect("clicked", lambda _b: self.open_automation())
        for button in (self.start_button, self.stop_button, self.unlock_button, self.extend_button,
                       self.automation_button):
            actions.pack_start(button, False, False, 0)
        self.delete_button = Gtk.Button(label="Delete…")
        self.delete_button.get_style_context().add_class("destructive-action")
        self.delete_button.connect("clicked", self.delete_block)
        self.rename_button = rename = Gtk.Button(label="Rename…")
        rename.connect("clicked", self.rename_block)
        duplicate = Gtk.Button(label="Duplicate…")
        duplicate.connect("clicked", self.duplicate_block)
        as_template = Gtk.Button(label="Save as block preset…")
        as_template.connect("clicked", self.save_template)
        # §3.2 affected users.
        self.users_button = Gtk.Button(label="Users: all", no_show_all=True)
        self.users_button.set_tooltip_text("The accounts this block applies to")
        self.users_button.connect("clicked", self.choose_block_users)
        # The rest behind ⋯ (R-GUI-3: one primary action; nothing runs off
        # the edge of the window).
        more = Gtk.MenuButton(tooltip_text="More: rename, duplicate, block preset, users, delete")
        more.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.BUTTON))
        popover = Gtk.Popover()
        menu = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, margin=8)
        for button in (rename, duplicate, as_template, self.users_button, self.delete_button):
            button.set_relief(Gtk.ReliefStyle.NONE)
            button.get_child().set_xalign(0)
            button.connect("clicked", lambda _b: popover.popdown())
            menu.pack_start(button, False, False, 0)
        self.delete_button.get_style_context().remove_class("destructive-action")
        self.delete_button.get_child().get_style_context().add_class("error")
        menu.show_all()
        self.users_button.set_visible(False)
        popover.add(menu)
        more.set_popover(popover)
        self.more_menu = (more, popover)  # for tests
        actions.pack_end(more, False, False, 0)
        self.detail.pack_start(actions, False, False, 0)

        tabs = Gtk.Notebook(scrollable=True)
        self.block_tabs = tabs
        self.detail_scroll.add(tabs)
        self.detail.pack_start(self.detail_scroll, True, True, 0)

        websites = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14, margin=12)
        intro = Gtk.Box(spacing=6)
        intro_text = dim("Lists of sites and words, each with what it does to the pages it matches.")
        intro.pack_start(intro_text, False, False, 0)
        intro.pack_start(help_button(
            "Each list ticks where it looks: the address, the page title (the tab title) or the "
            "page text. Addresses take domains, paths and wildcards (*.xxx, *casino*, *.* for "
            "everything); titles and text take words: a plain word counts only on its own "
            "(porn misses pornhub), and * matches inside words (*porn* hits pornhub). Words that "
            "count only together are phrases, in phrase sets below. A match in "
            "the address or title takes every action; in the text, each action needs its number "
            "of different words. Unticking or removing weakens the block, so it isn't possible "
            "while it is locked."), False, False, 0)
        websites.pack_start(intro, False, False, 0)
        # §4.3 (owner 2026-11-17): the lists saved as a preset, and one used.
        preset_row = Gtk.Box(spacing=6)
        save_lists = Gtk.Button(label="Save lists as preset…")
        save_lists.connect("clicked", lambda _b: self.selected and self.save_preset("lists", self.selected))
        use_lists = Gtk.Button(label="Add lists from a preset…")
        use_lists.connect("clicked", lambda _b: self.add_from_preset("lists"))
        preset_row.pack_start(save_lists, False, False, 0)
        preset_row.pack_start(use_lists, False, False, 0)
        websites.pack_start(flow_row(preset_row), False, False, 0)
        self.keyword_test_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.keyword_test_switch.connect("state-set", self.toggle_keyword_test)
        test_row = self.switch_row(
            self.keyword_test_switch, "Test mode",
            "Lists that read pages only show what they find. Turning it on weakens the block.")
        test_row.get_children()[0].get_children()[0].set_tooltip_text(
            "Lists that read a page's title or text do nothing; instead a panel on each page "
            "shows which words were found, how often, and whether each action's thresholds "
            "are met. For tuning a list.")
        websites.pack_start(test_row, False, False, 0)
        self.list_cards = {}
        self.list_boxes = {}
        social = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, margin_top=6)
        help_label = dim(
            "Keep a site, take a part of it away while this block is on. Don't put the site on "
            "the blacklist too. Ticking is always allowed; unticking weakens the block. Sites "
            "change their pages now and then, and a setting that hides something may then miss "
            "it until an update.")
        help_label.set_max_width_chars(70)
        social.pack_start(help_label, False, False, 0)
        self.social_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        self.social_checks = {}
        social.pack_start(self.social_box, False, False, 0)
        self.social_expander = Gtk.Expander(use_markup=True)
        self.social_expander.set_label("<b>Social media</b>")
        self.social_expander.add(card(social))
        for kind, title, button_label, detail in (
                ("blacklist", "Blacklists", "New blacklist…", ""),
                ("exceptions", "Exceptions", "New exception list…",
                 "A page an exception list matches is spared from every blacklist of this block. "
                 "A YouTube playlist's address (youtube.com/playlist?list=…) lets that playlist's "
                 "videos through, and only them."),
                ("phrases", "Phrase sets", "New phrase set…",
                 "Phrases are words that count only together, such as [eat, nice] [hello, grandpa]. "
                 "The block's exceptions spare pages from its phrase sets too.")):
            if kind == "exceptions":
                websites.pack_start(self.social_expander, False, False, 0)
            head = Gtk.Box(spacing=6)
            head.pack_start(heading(title, "medium"), False, False, 0)
            new = Gtk.Button(label=button_label)
            new.connect("clicked", self.create_list, kind)
            head.pack_end(new, False, False, 0)
            websites.pack_start(head, False, False, 0)
            if detail:
                websites.pack_start(dim(detail), False, False, 0)
            self.list_boxes[kind] = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
            websites.pack_start(self.list_boxes[kind], False, False, 0)
        tabs.append_page(websites, Gtk.Label(label="Websites"))

        apps = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, margin=12)
        self.apps = EditableList(
            "Apps and folders", "discord  com.discordapp.Discord  /opt/games/",
            lambda values: self.block_cmd("add-app", values),
            lambda value, _num: self.block_cmd("remove-app", [value]),
            lambda: self.import_list(["block", "import-list", self.selected, "apps"]),
            lambda: self.export_list(["block", "export-list", self.selected, "apps"]),
            help_text="Executable names, Flatpak app IDs (see 'flatpak list --app'), or folders "
                      "whose programs can't run. Everything a blocked app started is closed too.",
            search=True, multi=True)
        pick_row = Gtk.Box(spacing=6)
        choose_apps = Gtk.Button(label="Choose from installed apps…")
        choose_apps.connect("clicked", self.pick_apps)
        choose_folder = Gtk.Button(label="Add a folder…")
        choose_folder.connect("clicked", self.pick_app_folder)
        choose_file = Gtk.Button(label="Add a program file…")
        choose_file.connect("clicked", self.pick_app_file)
        for button in (choose_apps, choose_folder, choose_file):
            pick_row.pack_start(button, False, False, 0)
        apps.pack_start(pick_row, False, False, 0)
        flow_row(pick_row)
        apps.pack_start(self.apps, True, True, 0)
        self.titles = EditableList(
            "Window titles", "YouTube  Twitch",
            lambda values: self.block_cmd("add-title", values),
            lambda value, _num: self.block_cmd("remove-title", [value]),
            lambda: self.import_list(["block", "import-list", self.selected, "titles"]),
            lambda: self.export_list(["block", "export-list", self.selected, "titles"]),
            help_text="Any app whose window title contains one of these words is found while the "
                      "block is on. Browsers are never matched by title: their tabs are judged by "
                      "the lists (R-TITLE-1). Separate words with spaces.",
            search=True, hidden=True, multi=True)
        apps.set_spacing(14)
        self.app_triggers = TriggerEditor(self, "apps", "close", "Close the app",
                                          "When one of these apps runs (logout, shutdown and the lock screen "
                                          "come with a minute's warning):")
        apps.pack_start(self.app_triggers, False, False, 0)
        apps.pack_start(self.titles, True, True, 0)
        self.title_triggers = TriggerEditor(self, "titles", "close", "Close the window",
                                            "When a window with one of these titles is open:")
        apps.pack_start(self.title_triggers, False, False, 0)
        apps_scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
        apps_scroll.add(apps)
        tabs.append_page(apps_scroll, Gtk.Label(label="Apps"))

        schedule = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, margin=12)
        # §3.6 Automation (owner, 2026-11-16): the block's windows and places
        # switch it from its default; with both, both or either.
        self.auto_summary = Gtk.Label(xalign=0, wrap=True)
        self.auto_buttons = {}

        def choice(pairs, setting):
            row, group = Gtk.Box(), None
            row.get_style_context().add_class("linked")
            for value, label in pairs:
                button = Gtk.RadioButton.new_with_label_from_widget(group, label)
                button.set_mode(False)  # drawn as a toggle button
                group = group or button
                button.connect("toggled", self.set_automation, setting, value)
                row.pack_start(button, False, False, 0)
                self.auto_buttons[(setting, value)] = button
            return row

        default_row = choice((("disabled", "Disabled by default"), ("enabled", "Enabled by default")), "default")
        self.auto_need_row = Gtk.Box(spacing=8)
        self.auto_need_row.pack_start(Gtk.Label(label="With windows and places:"), False, False, 0)
        self.auto_need_row.pack_start(choice((("either", "Either is enough"), ("both", "Both at once")), "needs"),
                                      False, False, 0)
        # Its contents shown once; the row comes and goes (show_all does
        # nothing on a no-show-all widget, so it can't show them later).
        for child in self.auto_need_row.get_children():
            child.show_all()
        self.auto_need_row.set_no_show_all(True)
        # §3.6 Schedule: schedules kept outside blocks, applied here.
        applied_row = Gtk.Box(spacing=6)
        applied_row.pack_start(Gtk.Label(label="Schedules:"), False, False, 0)
        self.block_applied = Gtk.Box(spacing=6)
        applied_row.pack_start(self.block_applied, False, False, 0)
        apply_button = Gtk.Button(label="Apply schedule…")
        apply_button.set_tooltip_text("Evening, Weekend…: made on the Schedules page; changing it there "
                                      "changes this block too")
        apply_button.connect("clicked", self.apply_schedule_to_block)
        applied_row.pack_start(apply_button, False, False, 0)
        applied_flow = flow_row(applied_row)
        self.block_auto_widgets = [default_row, self.auto_need_row, applied_flow]
        schedule.pack_start(section(
            "Automation", self.auto_summary, default_row, self.auto_need_row, applied_flow,
            help="While the block is on, its windows (below) and its places (further down; as many as "
                 "you like) switch it by themselves. Disabled by default: they turn it on, and it is "
                 "lifted otherwise. Enabled by default: they turn it off, and it is enabled otherwise. "
                 "With both windows and places, switching needs either one, or both at once. A "
                 "block with neither is continuous: enabled whenever it is on. A change that lifts a "
                 "locked block when it is enabled now is refused; one that only adds enabled time is "
                 "always allowed."), False, False, 0)
        self.calendar = WeekCalendar(self.create_window_from_calendar, self.edit_window_from_calendar,
                                     self.move_window_from_calendar)
        sched_intro = Gtk.Box(spacing=6)
        sched_intro.pack_start(dim("Its windows. Drag down a day to add one."), False, False, 0)
        sched_intro.pack_start(help_button(
            "Drag down a day to add a window; click one to edit or remove it; drag it to move it. "
            "Filled: the block is enabled. Outlined: an allow window, when it is lifted. Other "
            "blocks' windows are greyed."), False, False, 0)
        schedule.pack_start(sched_intro, False, False, 0)
        schedule.pack_start(self.calendar, False, False, 0)
        self.schedules = EditableList(
            "Schedule", "weekdays 09:00 17:00", self.add_schedule, self.remove_schedule,
            help_text="Days, start and end: weekdays 09:00 17:00, or mon,wed,fri 18:00 22:00. "
                      "While the block is on, it is enabled only in its windows (switch it on "
                      "with Start). With a schedule lock (below) it "
                      "can't be weakened during its windows and for a lead time before them.")
        schedule.pack_start(self.schedules, True, True, 0)
        self.block_auto_widgets += [self.calendar, self.schedules]
        # §3.6 autostart: rules that switch the block on by itself.
        self.autostarts = EditableList(
            "Autostart", "at 08:00 mon-fri",
            lambda specs: [self.run(["autostart", self.selected, "add", *spec.split()]) for spec in specs],
            lambda _value, num: self.run(["autostart", self.selected, "remove", str(num)]),
            help_text="Switches the block on by itself, without locks: at 08:00 [days], once "
                      "2026-11-20 08:00, sign-in (also waking up), or next-window.",
            split=lambda text: [text.strip()] if text.strip() else [])
        schedule.pack_start(self.autostarts, False, False, 0)
        preset_row = Gtk.Box(spacing=6)
        preset_row.pack_start(Gtk.Label(label="Add:"), False, False, 0)
        for label, days, start, end in SCHEDULE_PRESETS:
            button = Gtk.Button(label=label, tooltip_text=f"{days} {start}-{end}")
            button.connect("clicked", lambda _b, d=days, s=start, e=end: self.selected and self.run(
                ["schedule", "add", self.selected, "--days", d, "--from", s, "--to", e, *self.window_flags()],
                then=self.queued_note))
            preset_row.pack_start(button, False, False, 0)
        self.schedule_total = dim("")
        preset_row.pack_end(self.schedule_total, False, False, 0)
        schedule.pack_start(preset_row, False, False, 0)
        lock_row = Gtk.Box(spacing=6)
        self.schedule_lock = Gtk.CheckButton(label="Schedule lock, from")
        lock_row.pack_start(self.schedule_lock, False, False, 0)
        self.schedule_lead = DurationField(0, minimum=60, maximum=7 * 86400, zero="Only while each window is open",
                                           presets=(900, 3600, 7200), width=9)
        lock_row.pack_start(self.schedule_lead, False, False, 0)
        lock_row.pack_start(Gtk.Label(label="before each window", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
        lock_save = Gtk.Button(label="Save")
        lock_save.connect("clicked", self.save_schedule_lock)
        lock_row.pack_start(lock_save, False, False, 0)
        top_align(lock_row)
        schedule.pack_start(lock_row, False, False, 0)
        lock_row.pack_start(help_button(
            "While the lock is on (during the windows, at the block's places, for the lead time "
            "before each window, and outside allow windows) the block can't be weakened or "
            "switched off. A longer lead time is always allowed; a shorter one or off is refused "
            "while the lock is on."), False, False, 0)
        flow_row(lock_row)
        self.block_auto_widgets.append(flow_row(preset_row))
        schedule_scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER, vexpand=True)
        schedule_scroll.add(schedule)
        # §4.3 (owner 2026-11-17): the automation is its own view, opened
        # from the block's menu (no tab).
        self.automation_page = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8,
                                       margin_start=20, margin_end=20, margin_top=16, margin_bottom=8)
        back_to_block = Gtk.Button(label="← Back to the block", halign=Gtk.Align.START, relief=Gtk.ReliefStyle.NONE)
        back_to_block.get_style_context().add_class("kc9k-link")
        back_to_block.connect("clicked", lambda _b: self.blocks_stack.set_visible_child_name("detail"))
        self.automation_title = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.END)
        self.automation_title.get_style_context().add_class("kc9k-page-title")
        self.automation_page.pack_start(back_to_block, False, False, 0)
        title_row = Gtk.Box(spacing=6)
        title_row.pack_start(self.automation_title, True, True, 0)
        # Owner 2026-11-17: an automation saved as a preset, to use on another
        # block after this one is gone; and one used here.
        for label, handler in (("Save as preset…", lambda _b: self.selected and self.save_preset("automation", self.selected)),
                               ("Use a preset…", lambda _b: self.add_from_preset("automation"))):
            button = Gtk.Button(label=label, valign=Gtk.Align.CENTER)
            button.connect("clicked", handler)
            title_row.pack_start(button, False, False, 0)
        self.automation_page.pack_start(title_row, False, False, 0)
        self.automation_page.pack_start(schedule_scroll, True, True, 0)

        # §3.6 places, under the schedule: both switch the block (Automation).
        places = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        places_head = Gtk.Box(spacing=6)
        places_head.pack_start(heading("Places", "medium"), False, False, 0)
        places_head.pack_start(help_button(
            "Being at one of these places switches the block as its default says (Automation, "
            "above): on when it is disabled by default, off when it is enabled by default. A "
            "block can have as many places as you like. A place is known by the Wi-Fi networks "
            "in range when you save it (yours and the neighbours'); the computer is there when as "
            "many of them as you choose are in range again. Only a schedule lock keeps the block "
            "from being switched off or weakened while its automation has it enabled. With no "
            "network seen (Wi-Fi off), places count as whatever keeps the block enabled, and "
            "Wi-Fi is switched back on."), False, False, 0)
        places.pack_start(places_head, False, False, 0)
        # §4.5 (owner 2026-11-17): each added place says plainly that it is
        # added and what arriving there does; how many networks a place
        # needs is set on the Places page.
        self.block_places = PlaceRules(
            self, lambda p: self.selected and self.run(["place", "rule", self.selected, p, "on"]),
            lambda p: self.selected and self.run(["place", "rule", self.selected, p, "off"]),
            lambda: self.save_place(None))
        self.place_combo = self.block_places.combo
        places.pack_start(self.block_places, False, False, 0)
        self.block_auto_widgets.append(self.block_places)
        schedule.pack_start(Gtk.Separator(margin_top=8, margin_bottom=4), False, False, 0)
        schedule.pack_start(places, False, False, 0)

        tabs.append_page(self.build_quota_tab(), Gtk.Label(label="Breaks & allowance"))

        nepsis = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10, margin=12)
        nepsis.pack_start(dim(
            "Bypass prevention: when this block stops many things in a short time (pages, apps, "
            "windows), likely a hunt for a loophole, it takes a "
            "consequence so you step away. It counts while the block is on; the one under "
            "Settings counts everything, and both apply."), False, False, 0)
        self.block_nepsis_label = Gtk.Label(xalign=0, wrap=True)
        nepsis.pack_start(self.block_nepsis_label, False, False, 0)
        self.nepsis_kept_label = Gtk.Label(xalign=0, wrap=True, no_show_all=True)
        nepsis.pack_start(self.nepsis_kept_label, False, False, 0)
        row = Gtk.Box(spacing=6)
        change = Gtk.Button(label="Change…")
        change.connect("clicked", self.change_block_nepsis)
        off = Gtk.Button(label="Turn off…")
        off.connect("clicked", self.nepsis_off, lambda: self.selected)
        self.block_nepsis_off_button = off
        row.pack_start(change, False, False, 0)
        row.pack_start(off, False, False, 0)
        nepsis.pack_start(row, False, False, 0)
        tabs.append_page(nepsis, Gtk.Label(label="Bypass prevention"))

        # §4.2 (owner, 2026-11-16): the page opens on the block list, one
        # card per block; a card opens its block here.
        self.blocks_stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE)
        self.blocks_stack.add_named(self.build_block_cards(), "cards")
        self.blocks_stack.add_named(paned, "detail")
        self.blocks_stack.add_named(self.automation_page, "automation")
        self.blocks_stack.add_named(self.build_mode_view(), "mode")
        return self.blocks_stack

    def build_block_cards(self):
        scroll, box = page_box()
        page_header(box, "Blocks", "What each block blocks, what switches it, its breaks and its locks. "
                    "Drag a block onto a mode to automate it with the mode's blocks.")
        # §4.2 modes (owner 2026-11-17): on the same page, above the blocks.
        self.mode_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.pack_start(section(
            "Modes", self.mode_cards, plain=True,
            help="A mode is a group of blocks with one automation for all of them: one schedule and "
                 "places switch them together. Drag a block onto a mode to put it in (it then has the "
                 "mode's automation), or open the mode and use Add blocks…. Monk mode is a mode too: give it windows or "
                 "places and switch it on, and it runs then by itself."), False, False, 0)
        self.block_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        self.block_cards_shown = None
        box.pack_start(section("Blocks", self.block_cards, plain=True), False, False, 0)
        buttons = Gtk.Box(spacing=8)
        new = Gtk.Button(label="New block…")
        new.get_style_context().add_class("suggested-action")
        new.connect("clicked", self.create_block)
        buttons.pack_start(new, False, False, 0)
        new_mode = Gtk.Button(label="New mode…")
        new_mode.connect("clicked", self.create_mode)
        buttons.pack_start(new_mode, False, False, 0)
        box.pack_start(buttons, False, False, 0)
        return scroll

    def update_block_cards(self):
        """One card per block (drafts stay with their block, blocks in a
        mode are inside it): its name, an on/off switch, continuous or
        automated, its breaks, its locks; above them one card per mode."""
        blocks = [b for b in self.state.get("blocks", []) if not b.get("draft_of")]
        modes = self.state.get("modes") or []
        shown = [(b["name"], b["active"], b.get("held"), b.get("continuous"), breaks_words(b), lock_words(b),
                  b.get("enabled"), b.get("mode"), b.get("draft")) for b in blocks] + [json.dumps(modes, sort_keys=True)]
        if self.blocks_stack.get_visible_child_name() == "mode":
            self.update_mode_view()
        if shown == self.block_cards_shown:
            return
        self.block_cards_shown = shown
        self.card_switches = {}
        for child in self.block_cards.get_children() + self.mode_cards.get_children():
            child.get_parent().remove(child)
        for m in modes:
            self.mode_cards.pack_start(self.mode_card(m), False, False, 0)
        loose = [b for b in blocks if not b.get("mode")]
        for b in loose:
            self.block_cards.pack_start(self.block_card(b), False, False, 0)
        if not blocks:
            self.block_cards.pack_start(dim("No blocks yet. A block holds what to block, when, and how it holds."),
                                        False, False, 0)
        elif not loose:
            self.block_cards.pack_start(dim("Every block is in a mode."), False, False, 0)
        self.block_cards.show_all()
        self.mode_cards.show_all()

    def block_card(self, block):
        name = block["name"]

        def link(text, icon, tooltip, action):
            button = Gtk.Button(relief=Gtk.ReliefStyle.NONE, tooltip_text=tooltip)
            button.get_style_context().add_class("kc9k-link")
            inner = Gtk.Box(spacing=6)
            inner.pack_start(Gtk.Label(label=text), False, False, 0)
            inner.pack_start(Gtk.Image.new_from_icon_name(icon, Gtk.IconSize.MENU), False, False, 0)
            button.add(inner)
            button.connect("clicked", lambda _b: action())
            return button

        title = Gtk.Button(label=name, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                           tooltip_text="Open the block")
        title.get_style_context().add_class("kc9k-card-name")
        title.connect("clicked", lambda _b: self.open_block(name))
        switch = Gtk.Switch(active=block["active"], valign=Gtk.Align.CENTER)
        switch.set_tooltip_text("Locked: it can't be switched off" if block.get("held") else
                                "Switch the block off" if block["active"] else "Switch the block on (without locks)")
        switch.set_sensitive(not (block["active"] and block.get("held")))
        switch.connect("state-set", self.toggle_block_card, name)
        self.card_switches[name] = switch
        state = Gtk.Label(label=("On" if block.get("enabled") else "On, lifted now") if block["active"] else "Off")
        state.get_style_context().add_class("dim-label")
        top = Gtk.Box(spacing=10)
        top.pack_start(title, False, False, 0)
        if block.get("draft"):
            # §4.3: edits made in it and not saved yet.
            edits = pill("Unsaved edits", "warn")
            edits.set_tooltip_text("Open the block to save or discard them")
            top.pack_start(edits, False, False, 0)
        top.pack_end(switch, False, False, 0)
        top.pack_end(state, False, False, 0)
        automated = not block.get("continuous", True)
        links = Gtk.Box(spacing=8)
        links.pack_start(link("Automated block" if automated else "Continuous block",
                              "alarm-symbolic" if automated else "media-playlist-repeat-symbolic",
                              f"Automated by '{block['mode']}'" if block.get("mode") else
                              "What switches it by itself: schedule and places",
                              lambda: self.open_automation(name)), False, False, 0)
        links.set_center_widget(link(breaks_words(block), "preferences-system-time-symbolic", "Its breaks",
                                     lambda: self.open_block(name, "Breaks & allowance")))
        links.pack_end(link(lock_words(block), "changes-prevent-symbolic" if block.get("held") else
                            "changes-allow-symbolic", "Unlock it" if block.get("held") else "Add locks",
                            lambda: self.card_locks(name)), False, False, 0)
        # §4.2 (owner 2026-11-17): dragged onto a mode it goes in; nothing
        # else on the card (no ⋯ menu). From the keyboard (R-GUI-3): the
        # mode's Add blocks… and Take out….
        for widget in (title,):
            widget.drag_source_set(Gdk.ModifierType.BUTTON1_MASK, [BLOCK_TARGET], Gdk.DragAction.MOVE)
            widget.drag_source_set_icon_name("view-list-symbolic")
            widget.connect("drag-data-get", lambda _w, _c, data, _i, _t: data.set(data.get_target(), 8, name.encode()))
        box = card(top, links, spacing=6)
        if not block["active"]:
            box.get_style_context().add_class("kc9k-block-off")
        return box
