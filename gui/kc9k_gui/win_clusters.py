# Window: Clusters page and presets.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _ClustersPage:
    """Methods of Window (window.py); no state of its own."""

    # Clusters ------------------------------------------------------------
    def build_clusters(self):
        """The Presets page (§3.2 Preset, §4.4; owner 2026-11-17): saved
        copies by type. Block presets are templates, list and phrase set
        presets clusters; the rest is kept as presets (kc9k preset)."""
        scroll, box = page_box()
        page_header(box, "Presets", "Saved copies of blocks, lists, phrase sets, automations and schedules, "
                    "to use again.",
                    help="A preset blocks nothing by itself and can always be changed. Using one copies it "
                         "into a block or mode, and saving one copies too: changing either afterwards changes "
                         "nothing on the other. Copying into a block follows that block's rules: a locked "
                         "block refuses what would weaken it. Save presets from a block: its menu (the whole "
                         "block), its Websites tab (its lists) and its Automation.")
        self.preset_sections = {}
        for kind, title, empty in (
                ("block", "Block presets", "Save one from a block's ⋯ menu: Save as block preset…"),
                ("lists", "Lists presets", "Save one from a block's Websites tab: Save lists as preset…"),
                ("automation", "Automation presets", "Save one from a block's or mode's Automation."),
                ("schedule", "Schedule presets", "Save one from a block's or mode's Automation, or a schedule."),
                ("hotkey", "Hotkey presets", "Save one on the Hotkeys page: Save as preset….")):
            rows = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
            note = dim(empty)
            box.pack_start(section(title, rows, note), False, False, 0)
            self.preset_sections[kind] = (rows, note)
        self.presets_shown = None
        top = Gtk.Box(spacing=6)
        self.cluster_combo = Gtk.ComboBoxText()
        self.cluster_combo.connect("changed", lambda _c: self.updating or self.show_cluster())
        top.pack_start(self.cluster_combo, False, False, 0)
        copy = Gtk.Button(label="Copy into a block…")
        copy.get_style_context().add_class("suggested-action")
        copy.connect("clicked", self.copy_cluster)
        new = Gtk.Button(label="New…")
        new.connect("clicked", self.new_cluster)
        top.pack_start(new, False, False, 0)
        top.pack_start(copy, False, False, 0)
        # The rest behind ⋯ (R-GUI-3).
        more = Gtk.MenuButton(tooltip_text="Rename, delete, import, export")
        more.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.BUTTON))
        popover = Gtk.Popover()
        menu = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, margin=6)
        for label, handler in (("Rename…", self.rename_cluster), ("Import…", self.import_cluster),
                               ("Export…", self.export_cluster), ("Delete…", self.delete_cluster)):
            button = Gtk.Button(label=label, relief=Gtk.ReliefStyle.NONE)
            button.get_child().set_xalign(0)
            button.connect("clicked", lambda _b, h=handler: (popover.popdown(), h(_b)))
            menu.pack_start(button, False, False, 0)
        menu.show_all()
        popover.add(menu)
        more.set_popover(popover)
        top.pack_start(more, False, False, 0)
        one = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        one.pack_start(dim("One list or phrase set, edited here and copied into blocks."), False, False, 0)
        one.pack_start(top, False, False, 0)
        box.pack_start(section("List and phrase set presets", one), False, False, 0)

        where = Gtk.Box(spacing=12)
        where.pack_start(Gtk.Label(label="Looks at:"), False, False, 0)
        self.cluster_where = {}
        for key, label in LIST_PLACES + (AD_PLACE,):
            check = Gtk.CheckButton(label=label)
            check.connect("toggled", self.toggle_cluster_where, key)
            self.cluster_where[key] = check
            where.pack_start(check, False, False, 0)
        box.pack_start(where, False, False, 0)

        self.cluster_entries = EditableList(
            "Entries", "reddit.com  *casino*",
            lambda values: self.cluster_name() and self.run(["list-preset", "add", self.cluster_name(), *values]),
            lambda value, _n: self.cluster_name() and self.run(["list-preset", "remove", self.cluster_name(), value]),
            search=True, multi=True)
        box.pack_start(self.cluster_entries, True, True, 0)

        self.record_label = Gtk.Label(xalign=0, wrap=True)
        record_row = Gtk.Box(spacing=6)
        self.record_start = Gtk.Button(label="Record into this preset")
        self.record_start.connect("clicked", lambda _b: self.cluster_name() and self.run(
            ["record", "start", "--preset", self.cluster_name()]))
        # §3.12 word recording: the rendered words of the pages, ranked by
        # how specific they are; the least specific cut before adding.
        self.record_words = Gtk.Button(label="Record words")
        self.record_words.set_tooltip_text("Record the words shown on the pages you open (the preset must look at page text)")
        self.record_words.connect("clicked", lambda _b: self.cluster_name() and self.run(
            ["record", "start", "--preset", self.cluster_name(), "--words"]))
        self.record_cut = NumberField(0, 0, 95, width=4)
        self.record_cut.set_tooltip_text("Words: drop this percentage of the least specific before adding")
        self.record_stop = Gtk.Button(label="Stop and add")
        self.record_stop.connect("clicked", lambda _b: self.fields_ok(("Cut", self.record_cut)) and self.run(
            ["record", "stop", "--cut", str(self.record_cut.get_value_as_int())],
            then=lambda output: self.message(output, Gtk.MessageType.INFO)))
        self.record_discard = Gtk.Button(label="Stop and discard")
        self.record_discard.connect("clicked", lambda _b: self.run(["record", "stop", "--discard"]))
        for button in (self.record_start, self.record_words, Gtk.Label(label="cut %"), self.record_cut,
                       self.record_stop, self.record_discard):
            record_row.pack_start(button, False, False, 0)
        box.pack_start(section(
            "Record the sites or words you open",
            dim("While recording, every site opened in the browser (pages and the frames inside "
                "them, such as a login from another site) is noted. Stopping adds them to the "
                "list preset. Use it to collect what a task needs, then copy the preset into a "
                "block's exceptions. Recording words notes the words shown on each page instead, "
                "ranked by how much more common they are there than in your normal browsing; "
                "cut drops the least specific before adding."),
            self.record_label, record_row), False, False, 0)
        flow_row(record_row)
        return scroll

    def cluster_name(self):
        return self.cluster_combo.get_active_text()

    def update_presets(self):
        """The Presets page's sections: a row per preset with Use…, Rename…,
        Delete."""
        shown = json.dumps([self.state.get("templates") or [], self.state.get("presets") or []])
        if shown == self.presets_shown:
            return
        self.presets_shown = shown
        items = {"block": [{"kind": "block", "name": n, "what": "a whole block"} for n in self.state.get("templates") or []]}
        for p in self.state.get("presets") or []:
            items.setdefault(p["kind"], []).append(p)
        for kind, (rows, note) in self.preset_sections.items():
            for child in rows.get_children():
                rows.remove(child)
            for p in items.get(kind, []):
                line = Gtk.Box(spacing=8)
                text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                name = Gtk.Label(label=p["name"], xalign=0)
                name.get_style_context().add_class("kc9k-card-name")
                text.pack_start(name, False, False, 0)
                if p.get("what"):
                    text.pack_start(dim(p["what"]), False, False, 0)
                line.pack_start(text, True, True, 0)
                use = Gtk.Button(label="New block from it…" if kind == "block" else "Use…", valign=Gtk.Align.CENTER)
                use.connect("clicked", lambda _b, k=kind, n=p["name"]: self.use_preset(k, n))
                line.pack_start(use, False, False, 0)
                if kind != "block":
                    rename = Gtk.Button(label="Rename…", valign=Gtk.Align.CENTER)
                    rename.connect("clicked", lambda _b, k=kind, n=p["name"]: self.rename_preset(k, n))
                    line.pack_start(rename, False, False, 0)
                delete = Gtk.Button(label="Delete", valign=Gtk.Align.CENTER)
                delete.connect("clicked", lambda _b, k=kind, n=p["name"]: self.delete_preset(k, n))
                line.pack_start(delete, False, False, 0)
                rows.pack_start(card(line), False, False, 0)
            rows.show_all()
            note.set_visible(not items.get(kind))

    def use_preset(self, kind, name, target=None):
        """Copies a preset: a block preset into a new block; lists into a
        block; an automation or windows into a block or mode."""
        if kind == "block":
            new = target or self.ask_text(f"New block from {name}", "A short name for the new block.")
            if new:
                self.selected = new
                self.run(["block", "create", new, "--preset", name])
            return
        if kind == "hotkey":
            # §4.13: added again, on keys typed now (blank: the preset's own).
            keys = target if target is not None else self.ask_text(
                f"Add the hotkey {name}", "Its keys, such as ctrl+alt+s.", problem=hotkey_problem)
            if not keys:
                return
            what = next((p.get("what", "") for p in self.state.get("presets") or []
                         if p["kind"] == "hotkey" and p["name"] == name), "")
            if self.confirm_lock([f"a hotkey: {what}"]) if any(
                    w in what for w in ("locked", "with ", "Frozen Chicken", "monk mode")) else True:
                self.run(["preset", "use", name, "--kind", "hotkey", "--yes"] + (["--keys", keys] if keys.strip() else []),
                         then=lambda output: self.message(output, Gtk.MessageType.INFO))
            return
        modes = [m["name"] for m in self.state.get("modes") or [] if not m.get("freeze")]
        blocks = [n for n, b in self.blocks.items() if not b.get("draft_of")]
        choices = blocks if kind == "lists" else blocks + [f"mode {m}" for m in modes]
        if target is None:
            target = self.choose(f"Use {name}", "It is copied into the one you choose; what would weaken a "
                                 "locked block is refused.", choices) if choices else None
        if not target:
            return
        args = ["preset", "use", name, target.removeprefix("mode "), "--kind", kind]
        if target.startswith("mode ") and target not in blocks:
            args.append("--mode")
        self.run(args, then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def rename_preset(self, kind, name):
        new = self.ask_text(f"Rename {name}", initial=name)
        if new and new != name:
            self.run(["preset", "rename", name, new, "--kind", kind])

    def delete_preset(self, kind, name):
        if self.confirm(f"Delete the preset {name}?", "What was copied from it stays as it is.", "Delete",
                        destructive=True):
            self.run(["block-preset", "remove", name] if kind == "block" else ["preset", "delete", name, "--kind", kind])

    def save_preset(self, kind, source, mode=False, answer=None):
        """Saves a copy as a preset: lists (asks which and whether with
        actions), automation or windows. answer: (name, which, actions)
        for tests."""
        if answer is None:
            dialog = Gtk.Dialog(title="Save as preset", transient_for=self, modal=True)
            dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
            ok = dialog.add_button("Save", Gtk.ResponseType.OK)
            ok.get_style_context().add_class("suggested-action")
            dialog.set_default_response(Gtk.ResponseType.OK)
            box = dialog.get_content_area()
            box.set_spacing(10)
            box.set_border_width(14)
            box.pack_start(dim({"lists": f"A copy of the lists of {source}, to copy into other blocks.",
                                "automation": f"A copy of the automation of {source}: its windows, schedules, "
                                              "places and settings.",
                                "schedule": f"A copy of the windows of {source}."}[kind]), False, False, 0)
            entry = Gtk.Entry(activates_default=True, placeholder_text="A name")
            box.pack_start(entry, False, False, 0)
            which, actions = {}, None
            if kind == "lists":
                row, group = Gtk.Box(), None
                row.get_style_context().add_class("linked")
                for key, label in (("both", "Both"), ("blacklists", "Blacklists"), ("exceptions", "Exceptions")):
                    button = Gtk.RadioButton.new_with_label_from_widget(group, label)
                    button.set_mode(False)
                    group = group or button
                    which[key] = button
                    row.pack_start(button, False, False, 0)
                box.pack_start(row, False, False, 0)
                actions = Gtk.CheckButton(label="Include actions?")
                box.pack_start(actions, False, False, 0)
            self.preset_dialog = (dialog, entry, which, actions)  # for tests
            dialog.show_all()
            reply = dialog.run()
            name = entry.get_text().strip()
            chosen = next((k for k, b in which.items() if b.get_active()), "both")
            with_actions = bool(actions and actions.get_active())
            dialog.destroy()
            if reply != Gtk.ResponseType.OK or not name:
                return
        else:
            name, chosen, with_actions = answer
        args = ["preset", "save", kind, source, name]
        if kind == "lists":
            args += ([f"--{chosen}"] if chosen in ("blacklists", "exceptions") else []) + (["--actions"] if with_actions else [])
        if mode:
            args.append("--mode")
        self.run(args, then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def update_clusters(self):
        self.update_presets()
        names = [c["name"] for c in self.state.get("clusters") or []]
        if getattr(self, "clusters_shown", None) != names:
            current = self.cluster_name()
            self.clusters_shown = names
            self.cluster_combo.remove_all()
            for name in names:
                self.cluster_combo.append_text(name)
            wanted = getattr(self, "cluster_wanted", None) or current
            if names:
                self.cluster_combo.set_active(names.index(wanted) if wanted in names else 0)
            self.cluster_wanted = None
        self.show_cluster()
        recording = self.state.get("recording")
        if recording:
            target = (f"the list preset {recording['cluster']}" if recording.get("cluster")
                      else f"the list {recording.get('list')} of {recording.get('block')}")
            if recording.get("words"):
                top = ", ".join(recording.get("top") or [])
                self.record_label.set_markup(
                    f"<b>Recording words</b> into {GLib.markup_escape_text(target)}: {recording.get('word_count', 0)} "
                    f"words on {recording.get('pages', 0)} pages so far."
                    + (f"\nMost specific: {GLib.markup_escape_text(top)}" if top else ""))
            else:
                self.record_label.set_markup(
                    f"<b>Recording</b> into {GLib.markup_escape_text(target)}: {recording.get('sites', 0)} sites so far.")
        else:
            self.record_label.set_text("Not recording.")
        shown = next((c for c in self.state.get("clusters") or [] if c["name"] == self.cluster_name()), {})
        # R-PHRASE-10: phrases take no part in recording.
        self.record_start.set_sensitive(not recording and bool(self.cluster_name()) and not shown.get("phrases"))
        self.record_words.set_sensitive(not recording and bool(self.cluster_name()) and not shown.get("phrases")
                                        and "text" in (shown.get("where") or []))
        self.record_stop.set_sensitive(bool(recording))
        self.record_discard.set_sensitive(bool(recording))

    def show_cluster(self):
        cluster = next((c for c in self.state.get("clusters") or [] if c["name"] == self.cluster_name()), None)
        was, self.updating = self.updating, True
        try:
            for key, check in self.cluster_where.items():
                check.set_active(bool(cluster) and key in cluster["where"])
                check.set_sensitive(bool(cluster))
            self.cluster_entries.set_items(cluster["entries"] if cluster else [], self.cluster_name())
            # A phrase cluster takes one phrase per line, as typed.
            phrases = bool(cluster and cluster.get("phrases"))
            self.cluster_entries.split = (lambda text: [text.strip()] if text.strip() else []) if phrases else split_entries
            self.cluster_entries.entry.set_placeholder_text("[eat, nice] [hello, grandpa]" if phrases else "reddit.com  *casino*")
        finally:
            self.updating = was

    def toggle_cluster_where(self, check, place):
        if self.updating or not self.cluster_name():
            return
        self.run(["list-preset", "scope", self.cluster_name(), place, "on" if check.get_active() else "off"])

    def new_cluster(self, _button):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL, text="New list or phrase set preset",
            secondary_text="A name, such as study or shopping.")
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        phrases = Gtk.CheckButton(label="Holds phrases (copied into blocks as a phrase set)")
        dialog.get_message_area().pack_start(entry, False, False, 0)
        dialog.get_message_area().pack_start(phrases, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        name, as_phrases = entry.get_text().strip(), phrases.get_active()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and name:
            self.cluster_wanted = name
            self.run(["list-preset", "create", name] + (["--phrases"] if as_phrases else []))

    def rename_cluster(self, _button):
        old = self.cluster_name()
        name = old and self.ask_text(f"Rename {old}", initial=old)
        if name and name != old:
            self.cluster_wanted = name
            self.run(["list-preset", "rename", old, name])

    def delete_cluster(self, _button):
        name = self.cluster_name()
        if name and self.confirm(f"Delete the preset {name}?",
                                 "Lists copied from it into blocks stay as they are.", "Delete", destructive=True):
            self.run(["list-preset", "delete", name])

    def copy_cluster(self, _button):
        name = self.cluster_name()
        if not name or not self.blocks:
            return
        cluster = next((c for c in self.state.get("clusters") or [] if c["name"] == name), {})
        phrases = cluster.get("phrases", False)
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=f"Copy {name} into a block",
            secondary_text=("It becomes a new phrase set of the block, named after the preset, which "
                            "makes the block stricter." if phrases else
                            "It becomes a new list of the block, named after the preset. As a blacklist it "
                            "makes the block stricter; as exceptions it weakens it, which a locked block refuses."),
            message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
        combo = Gtk.ComboBoxText()
        for block in self.blocks:
            combo.append_text(block)
        combo.set_active(0)
        exceptions = Gtk.CheckButton(label="As an exception list")
        area = dialog.get_message_area()
        area.pack_start(combo, False, False, 0)
        if not phrases:  # there are no phrase exceptions
            area.pack_start(exceptions, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        block, as_exceptions = combo.get_active_text(), exceptions.get_active()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and block:
            self.run(["list-preset", "copy", name, block] + (["--exceptions"] if as_exceptions else []),
                     then=self.queued_note)

    def import_cluster(self, _button):
        name = self.cluster_name()
        path = name and self.choose_file("Import entries into the preset", Gtk.FileChooserAction.OPEN, "Import")
        if path:
            self.run(["list-preset", "import", name, path])

    def export_cluster(self, _button):
        name = self.cluster_name()
        path = name and self.choose_file("Export the preset", Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, ["list-preset", "export", name])
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n" if output else "")
        except OSError as error:
            self.message(str(error))
