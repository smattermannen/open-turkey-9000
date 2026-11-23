# App picker, editable lists, list cards, phrase sets.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

# Widgets -----------------------------------------------------------------

def installed_apps():
    """The apps installed on this computer, as (name, what to block, icon):
    the program's file name, or the app ID for a Flatpak."""
    seen, out = set(), []
    for info in Gio.AppInfo.get_all():
        try:
            exe = os.path.basename(info.get_executable() or "")
            app_id = (info.get_id() or "").removesuffix(".desktop")
            if exe == "flatpak" and app_id.count(".") >= 2:
                entry = app_id
            elif exe in ("", "env", "sh", "bash", "python3", "python", "flatpak", "snap", "gtk-launch"):
                # Started through a launcher: the last word of its command names it.
                words = [w for w in (info.get_commandline() or "").split() if not w.startswith(("-", "%"))]
                entry = os.path.basename(words[-1]) if len(words) > 1 else ""
            else:
                entry = exe
            if not entry or entry in seen or not info.should_show():
                continue
            seen.add(entry)
            out.append((info.get_display_name() or entry, entry, info.get_icon()))
        except Exception:  # one broken launcher mustn't hide the rest
            continue
    return sorted(out, key=lambda row: row[0].lower())


def running_programs():
    """The names of the programs this user is running now."""
    names = set()
    uid = os.getuid()
    for pid in os.listdir("/proc"):
        if not pid.isdigit():
            continue
        try:
            if os.stat(f"/proc/{pid}").st_uid != uid:
                continue
            with open(f"/proc/{pid}/comm", encoding="utf-8") as file:
                name = file.read().strip()
        except OSError:
            continue
        if name and not name.startswith(("kworker", "systemd", "dbus", "gvfs", "xdg-", "csd-", "at-spi", "cinnamon",
                                         "pipewire", "wireplumber", "kc9k", "python", "bash", "sh", "(sd-pam)")):
            names.add(name)
    return sorted(names, key=str.lower)


class AppPicker(Gtk.Dialog):
    """Picks apps to block from the installed apps and the running programs,
    with a search box."""

    def __init__(self, parent):
        super().__init__(title="Choose apps to block", transient_for=parent, modal=True)
        self.add_buttons("Cancel", Gtk.ResponseType.CANCEL, "Add", Gtk.ResponseType.OK)
        self.set_default_size(520, 560)
        self.set_default_response(Gtk.ResponseType.OK)
        box = self.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        self.search = Gtk.SearchEntry(placeholder_text="Search apps and running programs")
        box.pack_start(self.search, False, False, 0)
        # ticked, icon, name, what is blocked, where it is from
        self.store = Gtk.ListStore(bool, Gio.Icon, str, str, str)
        for name, entry, icon in installed_apps():
            self.store.append([False, icon, name, entry, "Installed"])
        known = {row[3] for row in self.store}
        for name in running_programs():
            if name not in known:
                self.store.append([False, None, name, name, "Running now"])
        self.filtered = self.store.filter_new()
        self.filtered.set_visible_func(self._visible)
        view = Gtk.TreeView(model=self.filtered, headers_visible=True)
        toggle = Gtk.CellRendererToggle()
        toggle.connect("toggled", self._toggled)
        view.append_column(Gtk.TreeViewColumn("", toggle, active=0))
        column = Gtk.TreeViewColumn("App")
        icon = Gtk.CellRendererPixbuf()
        text = Gtk.CellRendererText(ellipsize=Pango.EllipsizeMode.END)
        column.pack_start(icon, False)
        column.pack_start(text, True)
        column.add_attribute(icon, "gicon", 1)
        column.add_attribute(text, "text", 2)
        column.set_expand(True)
        view.append_column(column)
        view.append_column(Gtk.TreeViewColumn("Blocked as", Gtk.CellRendererText(), text=3))
        view.append_column(Gtk.TreeViewColumn("", Gtk.CellRendererText(), text=4))
        view.connect("row-activated", lambda _v, path, _c: self._toggled(None, path))
        scroll = Gtk.ScrolledWindow()
        scroll.add(view)
        box.pack_start(scroll, True, True, 0)
        self.search.connect("search-changed", lambda _e: self.filtered.refilter())
        self.show_all()

    def _visible(self, model, iterator, _data):
        words = self.search.get_text().lower().split()
        text = f"{model[iterator][2]} {model[iterator][3]}".lower()
        return all(word in text for word in words)

    def _toggled(self, _renderer, path):
        child = self.filtered.convert_path_to_child_path(Gtk.TreePath(path) if isinstance(path, str) else path)
        self.store[child][0] = not self.store[child][0]

    def chosen(self):
        return [row[3] for row in self.store if row[0]]


COMBO_GROUP = re.compile(r"\(([^()]*)\)(?:\s*:\s*(\d+))?")
MAX_BOXES = 9


def parse_combination(text):
    """A word combination's boxes, [(terms, min)], or None when text isn't
    one. The same rules as kc9kCombination in the extension's match.js."""
    value = text.strip()
    if "(" not in value and ")" not in value:
        return None
    boxes = []

    def bare(part):
        term = part.strip()
        if not term:
            return True
        if any(c in term for c in ",:()") or not term.replace("*", "").strip():
            return False
        boxes.append(([term], 1))
        return True

    last = 0
    for m in COMBO_GROUP.finditer(value):
        if not bare(value[last:m.start()]):
            return None
        terms = [t.strip() for t in m.group(1).split(",")]
        need = int(m.group(2)) if m.group(2) else 1
        if not all(t.replace("*", "").strip() for t in terms) or not 1 <= need <= MAX_BOXES:
            return None
        boxes.append((terms, need))
        last = m.end()
    rest = value[last:]
    if "(" in rest or ")" in rest or not bare(rest):
        return None
    if sum(need for _t, need in boxes) < 2 or len(boxes) > MAX_BOXES:
        return None
    return boxes


def split_entries(text):
    """Splits typed entries at spaces, keeping word combinations whole: a
    space inside brackets doesn't split, nor one between two boxes, and a
    box standing alone joins the word before it ("cars (red, big)") or, at
    the start, the one after it ("(red, big) cars")."""
    parts, current, depth = [], "", 0
    for char in text:
        if char.isspace() and depth == 0:
            if current:
                parts.append(current)
            current = ""
            continue
        depth = max(depth + {"(": 1, ")": -1}.get(char, 0), 0)
        current += char
    if current:
        parts.append(current)

    def lone(part):
        """A single box that isn't a combination on its own."""
        return re.fullmatch(r"\([^()]*\)(:\d+)?", part) is not None and \
            parse_combination(part) is None

    out = []
    for part in parts:
        if out and ((part.startswith(("(", ":")) and ")" in out[-1]) or lone(out[-1]) or
                    (lone(part) and "(" not in out[-1])):
            out[-1] += " " + part
        else:
            out.append(part)
    return out


class ResizeGrip(Gtk.EventBox):
    """A handle under a list: drag it down to make the list taller, up to
    make it shorter; a double-click puts it back. The height is remembered
    in gui.json under key, so the list opens the same next time; the other
    lists of that kind on screen take it at once (B6: a block's exception
    list kept its old height while the card stayed)."""

    MIN = 80
    live = weakref.WeakSet()

    def __init__(self, target, key, default):
        super().__init__(tooltip_text="Drag to make the list taller or shorter")
        self.target, self.key, self.default = target, key, default
        self.drag = None
        line = Gtk.Box(halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER)
        line.get_style_context().add_class("kc9k-grip-line")
        self.add(line)
        self.get_style_context().add_class("kc9k-grip")
        self.add_events(Gdk.EventMask.BUTTON_PRESS_MASK | Gdk.EventMask.BUTTON_RELEASE_MASK |
                        Gdk.EventMask.BUTTON1_MOTION_MASK)
        self.connect("realize", lambda w: w.get_window().set_cursor(
            Gdk.Cursor.new_from_name(w.get_display(), "ns-resize")))
        self.connect("button-press-event", self._press)
        self.connect("motion-notify-event", self._motion)
        self.connect("button-release-event", self._release)
        ResizeGrip.live.add(self)
        self.connect("destroy", lambda w: ResizeGrip.live.discard(w))
        height = self._heights().get(key)
        if isinstance(height, int) and height >= self.MIN:
            target.set_size_request(-1, height)

    @staticmethod
    def _prefs():
        app = Gtk.Application.get_default()
        return getattr(app, "prefs", None)

    def _heights(self):
        prefs = self._prefs()
        heights = prefs.get("list_heights") if prefs is not None else None
        return heights if isinstance(heights, dict) else {}

    def set_height(self, height):
        """Sets the list's height (None: back to its own) and remembers it."""
        if height is not None:
            height = max(self.MIN, int(height))
        for grip in list(ResizeGrip.live):
            if grip is self or (self.key and grip.key == self.key):
                grip.target.set_size_request(-1, -1 if height is None else height)
        prefs = self._prefs()
        if prefs is not None and self.key:
            heights = dict(self._heights())
            if height is None:
                heights.pop(self.key, None)
            else:
                heights[self.key] = height
            prefs["list_heights"] = heights
            save_prefs(prefs)

    def _press(self, _widget, event):
        if event.button != 1:
            return False
        if event.type == Gdk.EventType._2BUTTON_PRESS:
            self.drag = None
            self.set_height(None)
            return True
        self.drag = (event.y_root, self.target.get_allocated_height())
        return True

    def _motion(self, _widget, event):
        if self.drag is None:
            return False
        y0, start = self.drag
        self.target.set_size_request(-1, max(self.MIN, int(start + event.y_root - y0)))
        return True

    def _release(self, _widget, event):
        if self.drag is None or event.button != 1:
            return False
        self.drag = None
        self.set_height(self.target.get_size_request()[1])
        return True


class EditableList(Gtk.Box):
    """A titled list with an entry for adding and a button for removing.

    search adds a box that filters the list. hidden covers the terms until
    the cover is clicked, like sensitive content on Reddit: lists of words
    for adult sites shouldn't greet whoever opens the window. The cover
    comes back when the list is shown for something else (see set_items)."""

    def __init__(self, title, placeholder, on_add, on_remove,
                 on_import=None, on_export=None, help_text="", search=False, hidden=False,
                 on_activate=None, split=None, entry=True, multi=False, size_key=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.on_add, self.on_remove = on_add, on_remove
        self.split = split or split_entries
        self.hidden, self.owner = hidden, None
        if title:
            self.pack_start(heading(title, "medium"), False, False, 0)
        if help_text:
            self.pack_start(dim(help_text), False, False, 0)

        self.store = Gtk.ListStore(str)
        self.filter_text = ""
        self.filtered = self.store.filter_new()
        self.filtered.set_visible_func(
            lambda model, it, _data: self.filter_text in (model[it][0] or "").lower())
        self.view = Gtk.TreeView(model=self.filtered, headers_visible=False)
        self.view.append_column(
            Gtk.TreeViewColumn("", Gtk.CellRendererText(), text=0))
        # §4.3: several entries at once (Ctrl/Shift-click, Ctrl+A) where
        # entries are removed by value; Delete or Backspace removes.
        self.multi = multi
        if multi:
            self.view.get_selection().set_mode(Gtk.SelectionMode.MULTIPLE)
        self.view.connect("key-press-event", self._key)
        if on_activate:
            self.view.connect("row-activated", lambda view, path, _c: on_activate(
                view.get_model()[path][0]))
        elif entry:
            # §4.3: double-click moves an entry into the box to edit it
            # (removed, then added again: R-WEAK-5).
            self.view.connect("row-activated", self._edit)
        scroll = Gtk.ScrolledWindow(min_content_height=120)
        scroll.set_shadow_type(Gtk.ShadowType.IN)
        scroll.add(self.view)

        self.search = None
        if search:
            self.search = Gtk.SearchEntry(placeholder_text="Find in this list")
            self.search.connect("search-changed", self._search)
            self.pack_start(self.search, False, False, 0)

        # The cover: a button over the list's place until clicked.
        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE)
        self.cover = Gtk.Button(relief=Gtk.ReliefStyle.NONE)
        self.cover_label = Gtk.Label(justify=Gtk.Justification.CENTER)
        self.cover.add(self.cover_label)
        self.cover.connect("clicked", lambda _b: self.reveal())
        frame = Gtk.Frame()
        frame.add(self.cover)
        self.stack.add_named(frame, "cover")
        self.stack.add_named(scroll, "list")
        self.pack_start(self.stack, True, True, 0)
        # Long lists are easier to read through taller: drag the grip down.
        self.grip = ResizeGrip(self.stack, size_key or title or placeholder, 120)
        self.pack_start(self.grip, False, False, 0)

        row = self.button_row = Gtk.Box(spacing=6)
        self.entry = Gtk.Entry(placeholder_text=placeholder)
        self.entry.connect("activate", self._add)
        add = Gtk.Button(label="Add")
        add.connect("clicked", self._add)
        remove = Gtk.Button(label="Remove")
        remove.connect("clicked", self._remove)
        if entry:
            row.pack_start(self.entry, True, True, 0)
            row.pack_start(add, False, False, 0)
        row.pack_start(remove, False, False, 0)
        # Import and export behind one small menu, so the row fits a narrow
        # window (R-GUI-3).
        files = [(text, handler) for text, handler in (("Import…", on_import), ("Export…", on_export)) if handler]
        self.file_buttons = {}
        if files:
            more = Gtk.MenuButton(tooltip_text="Import or export this list")
            more.add(Gtk.Image.new_from_icon_name("document-open-symbolic", Gtk.IconSize.BUTTON))
            popover = Gtk.Popover()
            menu = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2, margin=6)
            for label_text, handler in files:
                button = Gtk.Button(label=label_text, relief=Gtk.ReliefStyle.NONE)
                button.get_child().set_xalign(0)
                button.connect("clicked", lambda _b, h=handler: (popover.popdown(), h()))
                menu.pack_start(button, False, False, 0)
                self.file_buttons[label_text] = button
            menu.show_all()
            popover.add(menu)
            more.set_popover(popover)
            row.pack_start(more, False, False, 0)
        self.pack_start(row, False, False, 0)
        self.conceal()

    def conceal(self):
        """Covers the entries again (when the list hides them and has any)."""
        self.revealed = not self.hidden
        self._show_cover()

    def reveal(self):
        self.revealed = True
        self._show_cover()

    def _show_cover(self):
        count = len(self.store)
        covered = not self.revealed and count > 0
        self.cover_label.set_markup(
            f"<b>{count} {'entry' if count == 1 else 'entries'} hidden</b>\n"
            "<small>Click to show</small>")
        self.stack.set_visible_child_name("cover" if covered else "list")
        if self.search is not None:
            self.search.set_sensitive(not covered)

    def _search(self, entry):
        self.filter_text = entry.get_text().strip().lower()
        self.filtered.refilter()

    def set_items(self, items, owner=None):
        """Shows items. A different owner (another block, say) covers the
        entries again."""
        if owner != self.owner:
            self.owner = owner
            if self.search is not None:
                self.search.set_text("")
            self.conceal()
        current = [row[0] for row in self.store]
        if current != list(items):
            self.store.clear()
            for item in items:
                self.store.append([item])
            # §4.3: after a removal, the selection moves to the next entry.
            index = getattr(self, "next_index", None)
            if index is not None and len(self.filtered) > 0:
                self.next_index = None
                self.view.get_selection().select_path(Gtk.TreePath(min(index, len(self.filtered) - 1)))
        self._show_cover()

    def _add(self, _widget):
        values = self.split(self.entry.get_text())
        if values:
            self.entry.set_text("")
            self.on_add(values)

    def _selected(self):
        """The selected rows as (value, 1-based position), last first."""
        model, paths = self.view.get_selection().get_selected_rows()
        out = []
        for path in paths:
            child = model.convert_iter_to_child_iter(model.get_iter(path))
            out.append((self.store[child][0], self.store.get_path(child)[0] + 1))
        return sorted(out, key=lambda pair: -pair[1])

    def _remove(self, _button):
        chosen = self._selected()
        if not chosen:
            return
        # The selection moves to the row after the (first) removed one.
        self.next_index = min(i for _v, i in chosen) - 1
        for value, index in chosen if self.multi else chosen[:1]:
            self.on_remove(value, index)

    def _key(self, _view, event):
        ctrl = event.state & Gdk.ModifierType.CONTROL_MASK
        if event.keyval in (Gdk.KEY_Delete, Gdk.KEY_BackSpace):
            self._remove(None)
            return True
        if ctrl and event.keyval in (Gdk.KEY_f, Gdk.KEY_F) and self.search is not None:
            self.search.grab_focus()  # §4.3 Find
            return True
        return False

    def _edit(self, view, path, _column):
        value = view.get_model()[path][0]
        child = self.filtered.convert_iter_to_child_iter(self.filtered.get_iter(path))
        self.entry.set_text(value)
        self.entry.grab_focus()
        self.on_remove(value, self.store.get_path(child)[0] + 1)


DEFAULT_EXCEPTIONS = "Exceptions"  # db.DefaultExceptionsList


class ListCard(Gtk.Frame):
    """One website list: where it looks, its actions (blacklists), its
    thresholds for page text, and its entries. Every change runs through
    the window, which runs a kc9k command."""

    def __init__(self, window, data):
        super().__init__()
        self.get_style_context().add_class("kc9k-listcard")
        self.window, self.data = window, data
        self.filling = False
        self.is_exceptions = data["kind"] == "exceptions"
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, margin=14)
        self.add(box)

        top = Gtk.Box(spacing=6)
        self.title = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.END)
        top.pack_start(self.title, True, True, 0)
        for label, handler in (("Delete…", self.delete), ("Rename…", self.rename)):
            button = Gtk.Button(label=label, relief=Gtk.ReliefStyle.NONE)
            button.connect("clicked", handler)
            top.pack_end(button, False, False, 0)
        box.pack_start(top, False, False, 0)

        # §3.2: no list types; the scope boxes say where each entry is
        # matched (sites and URL entries against the URL, words anywhere).
        self.place_checks = {}
        places = Gtk.Box(spacing=12)
        places.pack_start(Gtk.Label(label="Looks at:"), False, False, 0)
        for name, title in LIST_PLACES + (() if self.is_exceptions else (AD_PLACE,)):
            check = Gtk.CheckButton(label=title)
            check.connect("toggled", self.toggle_place, name)
            if name == "ads":
                check.set_tooltip_text("The ad networks a page loads: entries are ad network domains "
                                       "or adult, crypto, dating, gambling")
            self.place_checks[name] = check
            places.pack_start(check, False, False, 0)
        box.pack_start(places, False, False, 0)

        self.action_checks, self.spins = {}, {}
        self.threshold_row = None
        self.test_check = None
        if not self.is_exceptions:
            self.test_check = Gtk.CheckButton(
                label="Test mode: show what it finds in pages instead of acting",
                tooltip_text="For lists that read a page's title or text. Turning it on weakens the block.")
            self.test_check.connect("toggled", self.toggle_test)
            box.pack_start(self.test_check, False, False, 0)
            # What it does in a line; every action behind Change actions…
            # (R-GUI-3: the entries come first, details on demand).
            does = Gtk.Box(spacing=8)
            does.pack_start(Gtk.Label(label="Does:", valign=Gtk.Align.START), False, False, 0)
            self.actions_summary = Gtk.Label(xalign=0, wrap=True, valign=Gtk.Align.START)
            does.pack_start(self.actions_summary, True, True, 0)
            self.actions_toggle = Gtk.ToggleButton(label="Change actions…", valign=Gtk.Align.START)
            does.pack_start(self.actions_toggle, False, False, 0)
            box.pack_start(does, False, False, 0)
            actions = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=5,
                                  column_spacing=12, row_spacing=2)
            for name, title in SITE_ACTIONS:
                check = Gtk.CheckButton(label=title)
                check.connect("toggled", self.toggle_action, name)
                self.action_checks[name] = check
                actions.add(check)
            self.actions_revealer = Gtk.Revealer(transition_type=Gtk.RevealerTransitionType.SLIDE_DOWN)
            self.actions_revealer.add(actions)
            self.actions_toggle.connect("toggled", lambda t: self.actions_revealer.set_reveal_child(t.get_active()))
            box.pack_start(self.actions_revealer, False, False, 0)
            # The settings of ticked actions that have some, each with Change….
            self.param_rows = {}
            for name in PARAM_ACTIONS:
                row = Gtk.Box(spacing=8)
                label = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.END)
                change = Gtk.Button(label="Change…")
                change.connect("clicked", lambda _b, n=name: self.ask_settings(n))
                row.pack_start(label, True, True, 0)
                row.pack_start(change, False, False, 0)
                self.param_rows[name] = (row, label)
                box.pack_start(row, False, False, 0)

            # For page text: how many different words each action needs, and
            # how many times they must occur in all (repeats counted).
            self.threshold_row = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
            self.threshold_row.pack_start(dim(
                "What a page's text needs for each action: different words, and how many "
                "times they occur in all (both must be met)."), False, False, 0)
            grid = Gtk.Grid(column_spacing=8, row_spacing=4)
            grid.attach(dim("Different", wrap=False), 1, 0, 1, 1)
            grid.attach(dim("In all", wrap=False), 2, 0, 1, 1)
            # §3.9 rate: share of the page's words, pages of 200+ words; 0 = none.
            grid.attach(dim("Rate %", wrap=False), 3, 0, 1, 1)
            self.rates = {}
            for row, (name, title) in enumerate(SITE_ACTIONS, start=1):
                label = Gtk.Label(label=title, xalign=0)
                distinct = Gtk.SpinButton.new_with_range(1, 1000, 1)
                hits = Gtk.SpinButton.new_with_range(1, 1000, 1)
                rate = Gtk.SpinButton.new_with_range(0, 100, 0.1)
                rate.set_digits(1)
                self.spins[name] = (label, distinct, hits)
                self.rates[name] = rate
                grid.attach(label, 0, row, 1, 1)
                grid.attach(distinct, 1, row, 1, 1)
                grid.attach(hits, 2, row, 1, 1)
                grid.attach(rate, 3, row, 1, 1)
            save = Gtk.Button(label="Save numbers", halign=Gtk.Align.START)
            save.connect("clicked", self.save_numbers)
            grid.attach(save, 1, len(SITE_ACTIONS) + 1, 2, 1)
            self.threshold_row.pack_start(grid, False, False, 0)
            box.pack_start(self.threshold_row, False, False, 0)

        self.entries = EditableList(
            "", "reddit.com  *casino*  nsfw", self.add_entries, self.remove_entry,
            lambda: window.import_list(["list", "import", window.selected, self.data["name"]]),
            lambda: window.export_list(["list", "export", window.selected, self.data["name"]]),
            search=True, hidden=True, multi=True, size_key="website list")
        box.pack_start(self.entries, True, True, 0)
        self.update(data)

    def update(self, data):
        self.data = data
        self.filling = True
        try:
            kind = "exceptions" if self.is_exceptions else "blacklist"
            count = len(data["entries"])
            self.title.set_markup(f"<b>{GLib.markup_escape_text(data['name'])}</b>  "
                                  f"<small>{kind} · {count} {'entry' if count == 1 else 'entries'}</small>")
            for name, check in self.place_checks.items():
                check.set_active(name in data["where"])
            actions = {a["action"]: a for a in data["actions"]}
            for name, check in self.action_checks.items():
                check.set_active(name in actions)
            if getattr(self, "actions_summary", None) is not None:
                titles = [title for name, title in SITE_ACTIONS if name in actions]
                self.actions_summary.set_text(", ".join(titles) if titles else
                                              "Nothing yet: it only finds (tick an action under Change actions…)")
            for name, (row, label) in getattr(self, "param_rows", {}).items():
                row.set_no_show_all(name not in actions)
                row.set_visible(name in actions)
                if name in actions:
                    label.set_text(self.settings_text(name, actions[name]))
            for name, (label, distinct, hits) in self.spins.items():
                distinct.set_value(actions.get(name, {}).get("distinct", 1))
                hits.set_value(actions.get(name, {}).get("hits", 1))
                widgets = [label, distinct, hits]
                if name in getattr(self, "rates", {}):
                    self.rates[name].set_value(actions.get(name, {}).get("rate", 0) / 10)
                    widgets.append(self.rates[name])
                for widget in widgets:
                    widget.set_no_show_all(name not in actions)
                    widget.set_visible(name in actions)
            if self.threshold_row is not None:
                self.threshold_row.set_no_show_all("text" not in data["where"])
                self.threshold_row.set_visible("text" in data["where"])
            self.entries.set_items(data["entries"])
            words = "title" in data["where"] or "text" in data["where"]
            if self.test_check is not None:
                self.test_check.set_active(data.get("test", False))
                self.test_check.set_no_show_all(not words)
                self.test_check.set_visible(words)
            self.entries.entry.set_placeholder_text(
                ("reddit.com  *.xxx  nsfw" if words else "reddit.com  *.xxx  *casino*")
                if "url" in data["where"] else "nsfw  18+  *porn*")
        finally:
            self.filling = False

    def command(self, *args):
        return ["list", args[0], self.window.selected, self.data["name"], *args[1:]]

    def add_entries(self, values):
        self.window.run(self.command("add", *values), then=self.window.queued_note)

    def remove_entry(self, value, _num):
        self.window.run(self.command("remove", value), then=self.window.queued_note)

    def toggle_test(self, check):
        if self.filling or self.window.updating:
            return
        self.window.run(["list", "test", self.window.selected, self.data["name"],
                         "on" if check.get_active() else "off"])

    def toggle_place(self, check, name):
        if self.filling or self.window.updating:
            return
        self.window.run(self.command("scope", name, "on" if check.get_active() else "off"),
                        then=self.window.queued_note)

    def toggle_action(self, check, name):
        if self.filling or self.window.updating:
            return
        if check.get_active() and name in PARAM_ACTIONS:
            self.ask_settings(name)
            return
        self.window.run(self.command("action", name, "on" if check.get_active() else "off"),
                        then=self.window.queued_note)

    @staticmethod
    def settings_text(name, action):
        minutes = action.get("minutes", 0)
        if name == "redirect":
            return f"Opens {action.get('param', '')}"
        if name == "image":
            return "Shows your picture"
        if name == "monk":
            return f"Monk mode for {fmt_minutes(minutes)}"
        if name == "mode":
            return f"Runs {action.get('param', '')} for {fmt_minutes(minutes)}, locked"
        if name == "custom":
            try:
                return "; ".join(f"{list(i)[0]} {list(i.values())[0]}" for i in json.loads(action.get("param") or "[]"))
            except (ValueError, IndexError, AttributeError):
                return "Custom action"
        verb = "Logs out" if name == "logout" else "Shuts down"
        return f"{verb} for {fmt_minutes(minutes)}" if minutes else f"{verb} once"

    def ask_settings(self, name):
        """Asks for an action's address, picture or time, and turns it on with
        them (or changes them). Cancelling leaves the list as it was."""
        window, args = self.window, None
        current = {a["action"]: a for a in self.data["actions"]}.get(name, {})
        if name == "redirect":
            url = window.ask_text("Open another page", "The address to open instead, starting "
                                  "with https://.", initial=current.get("param", "https://"),
                                  problem=lambda text: None if re.fullmatch(r"https?://[^\s/?#]+\S*", text)
                                  else "An address such as https://example.com.")
            if url:
                args = ["--url", url]
        elif name == "image":
            path = window.choose_file("Choose a picture", Gtk.FileChooserAction.OPEN, "Choose")
            if path:
                args = ["--image", path]
        elif name == "mode":
            chosen = window.ask_mode_run("Run a mode", current.get("param"), current.get("minutes") or 60)
            if chosen and window.acknowledge_no_confirmation("This action"):
                args = ["--target", chosen[0], "--for", f"{chosen[1]}m", "--no-confirm-ok"]
        elif name == "custom":
            try:
                have = [f"{list(i)[0]} {list(i.values())[0]}" for i in json.loads(current.get("param") or "[]")]
            except (ValueError, IndexError, AttributeError):
                have = []
            items = window.ask_custom("Custom action", have)
            if items:
                args = [a for item in items for a in ("--custom", item)]
        else:
            details = {
                "monk": ("Monk mode", "The browser closes and every website is blocked, locked, "
                         "and the internet connection is cut, for this long.", 1, 60),
                "logout": ("Log out", "The browser closes and you are logged out. With a time, "
                           "you stay logged out that long; with 0, just once.", 0, 0),
                "shutdown": ("Shut down", "The browser closes and the computer shuts down. With a "
                             "time, it stays off that long; with 0, just once.", 0, 0),
            }[name]
            title, detail, minimum, initial = details
            seconds = window.ask_duration(
                title, detail, current.get("minutes", initial) * 60, minimum * 60 or 60,
                30 * 86400 if name == "monk" else 86400, zero=None if name == "monk" else "Just once",
                presets=(3600, 86400) if name == "monk" else (3600, 8 * 3600),
                why_max="" if name == "monk" else "It goes through Frozen Chicken, which lasts at most a week.")
            if seconds is not None:
                args = ["--for", f"{seconds // 60}m"] if seconds else []
                if name == "monk":
                    if window.acknowledge_no_confirmation("This action"):
                        args.append("--no-confirm-ok")
                    else:
                        args = None
        if args is None:
            self.update(self.data)  # untick the box again
            return
        window.run(self.command("action", name, "on", *args), then=window.queued_note)

    def save_numbers(self, _button):
        current = {a["action"]: a for a in self.data["actions"]}
        changes = []
        for name, (_label, distinct, hits) in self.spins.items():
            if name not in current:
                continue
            for command, spin, key in (("threshold", distinct, "distinct"), ("hits", hits, "hits")):
                if spin.get_value_as_int() != current[name].get(key, 1):
                    changes.append((command, name, spin.get_value_as_int()))
            if name in getattr(self, "rates", {}):
                tenths = round(self.rates[name].get_value() * 10)
                if tenths != current[name].get("rate", 0):
                    changes.append(("rate", name, f"{tenths / 10:g}" if tenths else "off"))
        self.window.run_thresholds(self.window.selected, self.data["name"], changes)

    def rename(self, _button):
        name = self.window.ask_text("Rename list", initial=self.data["name"])
        if name and name != self.data["name"]:
            self.window.run(self.command("rename", name))

    def delete(self, _button):
        if self.window.confirm(f"Delete the list {self.data['name']}?",
                               "Its entries go with it. Deleting a blacklist weakens the block.",
                               "Delete", destructive=True):
            self.window.run(self.command("delete"), then=self.window.queued_note)


# A sanity bound only: a phrase may have any number of word sets (§4.3).
MAX_WORD_SETS = 100


def phrase_text(sets, length=0):
    """Writes a phrase as it is typed: "[eat, nice] [hello, grandpa] [today]? :3"
    (like db.Phrase.String). sets is [(terms, necessary)]."""
    parts, necessary = [], 0
    for terms, needed in sets:
        necessary += 1 if needed else 0
        parts.append("[" + ", ".join(terms) + "]" + ("" if needed else "?"))
    text = " ".join(parts)
    return f"{text} :{length}" if length and length != max(necessary, 2) else text


def phrase_key(text):
    """A phrase's text for comparing, as the command keeps it: letter case
    and spacing don't count (db.Phrase.Canonical)."""
    return " ".join((text or "").lower().replace(",", ", ").split())


def describe_found(found):
    """What a file to import holds, from `kc9k list read`: '3 phrases, 2 sites'."""
    parts = []
    for key, one, many in (("phrases", "phrase", "phrases"), ("sites", "site", "sites"),
                           ("terms", "rendered term", "rendered terms"),
                           ("skipped", "line KC9K can't use", "lines KC9K can't use")):
        n = len(found.get(key) or [])
        if n:
            parts.append(f"{n} {one if n == 1 else many}")
    return ", ".join(parts) or "nothing"


def word_set_terms(lines):
    """Terms for one word set from lines (one term each): # comments,
    blanks and duplicates dropped; brackets and commas can't be in a term."""
    out = []
    for line in lines:
        term = " ".join(str(line).split("#", 1)[0].replace("[", " ").replace("]", " ").split())
        for part in term.split(","):
            part = part.strip()
            if part and part.strip("*- ") and part.lower() not in (t.lower() for t in out):
                out.append(part)
    return out


class PhraseEditor(Gtk.Box):
    """Builds a phrase inside its set's card (§4.3, not a pop-up): its word
    sets, each ticked necessary or optional, and the phrase length;
    get_text() gives the typed form. on_change(text) follows every edit."""

    def __init__(self, initial=None, on_change=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.on_change = on_change
        self.loading = False
        self.pack_start(dim(
            "A page counts when it has one term of each necessary word set, directly after "
            "each other in any order, in one sentence (no punctuation between). Separate a "
            "set's terms with commas; * matches inside a word (porn*). A term ending in - "
            "(big-) and one starting with - (-gorilla) in another set make a bridge: up to "
            "20 words may stand between them. Any word set may be necessary or optional; the "
            "phrase length (at least 2) says how many terms a match needs."),
            False, False, 0)
        self.rows = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.pack_start(self.rows, False, False, 0)
        self.add_set = Gtk.Button(label="+ Add word set", halign=Gtk.Align.START)
        self.add_set.connect("clicked", lambda _b: self.add_row())
        set_buttons = Gtk.Box(spacing=6)
        set_buttons.pack_start(self.add_set, False, False, 0)
        self.pack_start(set_buttons, False, False, 0)
        length_row = Gtk.Box(spacing=8)
        length_row.pack_start(Gtk.Label(label="Phrase length (terms a match needs):"), False, False, 0)
        self.length = Gtk.SpinButton.new_with_range(2, MAX_WORD_SETS, 1)
        self.length.connect("value-changed", lambda _s: self.changed())
        length_row.pack_start(self.length, False, False, 0)
        self.pack_start(length_row, False, False, 0)
        self.preview = dim("", wrap=True)
        self.preview.set_selectable(True)
        self.pack_start(self.preview, False, False, 0)
        self.load(initial)

    def load(self, initial=None):
        """Shows a phrase (a phrase set's {"sets", "length"}), or two empty
        word sets."""
        for row in self.rows.get_children():
            self.rows.remove(row)
        sets = (initial or {}).get("sets") or [{"terms": []}, {"terms": []}]
        self.loading = True
        for word_set in sets:
            self.add_row(word_set.get("terms", []), word_set.get("necessary", True))
        self.loading = False
        self.last_necessary = self.necessary()
        self.length.set_range(2, max(len(sets), 2))
        self.length.set_value((initial or {}).get("length") or max(self.last_necessary, 2))
        self.changed()

    def add_row(self, terms=(), necessary=True):
        row = Gtk.Box(spacing=8)
        entry = Gtk.Entry(hexpand=True, placeholder_text="eat, nice", text=", ".join(terms))
        check = Gtk.CheckButton(label="Necessary", active=necessary)
        remove = Gtk.Button.new_from_icon_name("list-remove-symbolic", Gtk.IconSize.BUTTON)
        remove.set_tooltip_text("Remove this word set")
        remove.connect("clicked", lambda _b: (self.rows.remove(row), self.changed()))
        entry.connect("changed", lambda _e: self.changed())
        check.connect("toggled", lambda _c: self.changed())
        row.pack_start(entry, True, True, 0)
        row.pack_start(check, False, False, 0)
        row.pack_start(remove, False, False, 0)
        row.entry, row.check, row.remove = entry, check, remove
        self.rows.pack_start(row, False, False, 0)
        row.show_all()
        self.changed()

    def add_imported(self, terms):
        """Fills the first empty word set with terms, or adds a new one."""
        terms = [t for t in (terms or []) if t]
        if not terms:
            return
        for row in self.rows.get_children():
            if not row.entry.get_text().strip():
                row.entry.set_text(", ".join(terms))
                return
        if len(self.rows.get_children()) < MAX_WORD_SETS:
            self.add_row(terms)

    def word_sets(self):
        out = []
        for i, row in enumerate(self.rows.get_children()):
            terms = [" ".join(t.split()) for t in row.entry.get_text().replace("[", " ").replace("]", " ").split(",")]
            out.append(([t for t in terms if t.strip("*- ")], row.check.get_active()))
        return out

    def necessary(self):
        return sum(1 for _t, needed in self.word_sets() if needed)

    def get_text(self):
        sets = self.word_sets()
        if len(sets) < 2 or any(not terms for terms, _n in sets):
            return ""
        return phrase_text(sets, self.length.get_value_as_int())

    def changed(self):
        if not hasattr(self, "length") or self.loading:
            return
        rows = self.rows.get_children()
        for row in rows:
            row.remove.set_sensitive(len(rows) > 2)
        # The length follows the necessary word sets until it is set apart.
        low, high = max(self.necessary(), 2), max(len(rows), 2)
        follow = self.length.get_value_as_int() == max(getattr(self, "last_necessary", low), 2)
        self.last_necessary = low
        self.length.set_range(low, high)
        if follow:
            self.length.set_value(low)
        self.add_set.set_sensitive(len(rows) < MAX_WORD_SETS)
        text = self.get_text()
        sets = self.word_sets()
        self.preview.set_text(f"Phrase: {text}" if text else
                              "A phrase needs at least two word sets." if len(sets) < 2 else
                              "Every word set needs at least one term.")
        if self.on_change:
            self.on_change(text)


class PhraseSetCard(ListCard):
    """One phrase set (§4.3): scope, actions, a threshold of different
    phrases per action, test mode, and its phrases, edited in the
    PhraseEditor inside the card. Reuses ListCard's handling of actions and their settings."""

    def __init__(self, window, data):
        Gtk.Frame.__init__(self)
        self.get_style_context().add_class("kc9k-listcard")
        self.window, self.data = window, data
        self.filling = False
        self.is_exceptions = False
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, margin=12)
        self.add(box)

        top = Gtk.Box(spacing=6)
        self.title = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.END)
        top.pack_start(self.title, True, True, 0)
        for label, handler in (("Delete…", self.delete), ("Rename…", self.rename)):
            button = Gtk.Button(label=label)
            button.connect("clicked", handler)
            top.pack_end(button, False, False, 0)
        box.pack_start(top, False, False, 0)

        self.place_checks = {}
        places = Gtk.Box(spacing=12)
        places.pack_start(Gtk.Label(label="Looks at:"), False, False, 0)
        for name, title in LIST_PLACES:
            check = Gtk.CheckButton(label=title)
            check.connect("toggled", self.toggle_place, name)
            self.place_checks[name] = check
            places.pack_start(check, False, False, 0)
        box.pack_start(places, False, False, 0)

        self.test_check = Gtk.CheckButton(
            label="Test mode: show what it finds in pages instead of acting",
            tooltip_text="Turning it on weakens the block.")
        self.test_check.connect("toggled", self.toggle_test)
        box.pack_start(self.test_check, False, False, 0)

        # What it does in a line; every action behind Change actions… (as
        # a list's card).
        does = Gtk.Box(spacing=8)
        does.pack_start(Gtk.Label(label="Does:", valign=Gtk.Align.START), False, False, 0)
        self.actions_summary = Gtk.Label(xalign=0, wrap=True, valign=Gtk.Align.START)
        does.pack_start(self.actions_summary, True, True, 0)
        self.actions_toggle = Gtk.ToggleButton(label="Change actions…", valign=Gtk.Align.START)
        does.pack_start(self.actions_toggle, False, False, 0)
        box.pack_start(does, False, False, 0)
        actions = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=5,
                              column_spacing=12, row_spacing=2)
        self.action_checks, self.spins = {}, {}
        for name, title in SITE_ACTIONS:
            check = Gtk.CheckButton(label=title)
            check.connect("toggled", self.toggle_action, name)
            self.action_checks[name] = check
            actions.add(check)
        self.actions_revealer = Gtk.Revealer(transition_type=Gtk.RevealerTransitionType.SLIDE_DOWN)
        self.actions_revealer.add(actions)
        self.actions_toggle.connect("toggled", lambda t: self.actions_revealer.set_reveal_child(t.get_active()))
        box.pack_start(self.actions_revealer, False, False, 0)
        self.param_rows = {}
        for name in PARAM_ACTIONS:
            row = Gtk.Box(spacing=8)
            label = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.END)
            change = Gtk.Button(label="Change…")
            change.connect("clicked", lambda _b, n=name: self.ask_settings(n))
            row.pack_start(label, True, True, 0)
            row.pack_start(change, False, False, 0)
            self.param_rows[name] = (row, label)
            box.pack_start(row, False, False, 0)

        # R-PHRASE-9: per action, how many different phrases must match.
        self.threshold_row = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.threshold_row.pack_start(dim("Threshold: how many different phrases each action needs."),
                                      False, False, 0)
        grid = Gtk.Grid(column_spacing=8, row_spacing=4)
        for row, (name, title) in enumerate(SITE_ACTIONS):
            label = Gtk.Label(label=title, xalign=0)
            distinct = Gtk.SpinButton.new_with_range(1, 1000, 1)
            self.spins[name] = (label, distinct)
            grid.attach(label, 0, row, 1, 1)
            grid.attach(distinct, 1, row, 1, 1)
        save = Gtk.Button(label="Save thresholds", halign=Gtk.Align.START)
        save.connect("clicked", self.save_numbers)
        grid.attach(save, 1, len(SITE_ACTIONS), 1, 1)
        self.threshold_row.pack_start(grid, False, False, 0)
        box.pack_start(self.threshold_row, False, False, 0)

        self.entries = EditableList(
            "", "", self.add_entries, self.remove_entry,
            lambda: window.import_into_phrase_set(self),
            lambda: window.export_list(["phrase-set", "export", window.selected, self.data["name"]]),
            search=True, hidden=True, on_activate=self.edit_phrase, size_key="phrase set",
            split=lambda text: [text.strip()] if text.strip() else [], entry=False, multi=True)
        box.pack_start(self.entries, True, True, 0)
        box.pack_start(dim("Double-click a phrase to load it into the editor below."), False, False, 0)

        # The phrase editor lives in the card (§4.3), not in a pop-up.
        self.editing = None  # the phrase loaded for a change, or None for a new one
        self.editor_title = Gtk.Label(xalign=0)
        box.pack_start(self.editor_title, False, False, 0)
        self.editor = PhraseEditor(on_change=self.editor_changed)
        box.pack_start(self.editor, False, False, 0)
        buttons = Gtk.Box(spacing=6)
        self.save_phrase = Gtk.Button(label="Add phrase")
        self.save_phrase.get_style_context().add_class("suggested-action")
        self.save_phrase.connect("clicked", lambda _b: self.commit_phrase())
        # Cancel drops unsaved changes (asking first); it never deletes a
        # saved phrase. Delete phrase does, and says so.
        self.cancel_phrase = Gtk.Button(label="Cancel")
        self.cancel_phrase.set_tooltip_text("Stop editing; the saved phrases stay as they are")
        self.cancel_phrase.connect("clicked", lambda _b: self.edit_phrase(None))
        self.delete_phrase = Gtk.Button(label="Delete phrase")
        self.delete_phrase.get_style_context().add_class("destructive-action")
        self.delete_phrase.connect("clicked", lambda _b: self.remove_loaded())
        buttons.pack_start(self.save_phrase, False, False, 0)
        buttons.pack_start(self.cancel_phrase, False, False, 0)
        buttons.pack_end(self.delete_phrase, False, False, 0)
        box.pack_start(buttons, False, False, 0)
        self.saving = False
        self.update(data)
        self.edit_phrase(None, ask=False)

    def update(self, data):
        self.data = data
        self.filling = True
        try:
            count = len(data["phrases"])
            self.title.set_markup(f"<b>{GLib.markup_escape_text(data['name'])}</b>  "
                                  f"<small>phrase set · {count} {'phrase' if count == 1 else 'phrases'}</small>")
            for name, check in self.place_checks.items():
                check.set_active(name in data["where"])
            self.test_check.set_active(data.get("test", False))
            actions = {a["action"]: a for a in data["actions"]}
            if getattr(self, "actions_summary", None) is not None:
                titles = [title for name, title in SITE_ACTIONS if name in actions]
                self.actions_summary.set_text(", ".join(titles) if titles else
                                              "Nothing yet: it only finds (tick an action under Change actions…)")
            for name, check in self.action_checks.items():
                check.set_active(name in actions)
            for name, (row, label) in self.param_rows.items():
                row.set_no_show_all(name not in actions)
                row.set_visible(name in actions)
                if name in actions:
                    label.set_text(self.settings_text(name, actions[name]))
            for name, (label, distinct) in self.spins.items():
                distinct.set_value(actions.get(name, {}).get("distinct", 1))
                for widget in (label, distinct):
                    widget.set_no_show_all(name not in actions)
                    widget.set_visible(name in actions)
            self.entries.set_items([p["text"] for p in data["phrases"]])
            # A saved change replaced the phrase being edited: keep editing
            # the new one, never fall back to adding a copy. The editor's
            # work is never thrown away here: a phrase that is gone (deleted
            # elsewhere) leaves it as a new phrase.
            self.saving = False
            if getattr(self, "editing", None) is None and getattr(self, "saved_as", None):
                # A new phrase was added: it is now the one being edited.
                added = next((p for p in data["phrases"] if phrase_key(p["text"]) == self.saved_as), None)
                if added is not None:
                    self.editing = added
                    self.show_mode()
            elif getattr(self, "editing", None) is not None:
                texts = {phrase_key(p["text"]): p for p in data["phrases"]}
                current = texts.get(getattr(self, "saved_as", None)) or texts.get(phrase_key(self.editing["text"]))
                if current is None:
                    self.editing, self.saved_as = None, None
                    self.show_mode()
                elif current is not self.editing:
                    self.editing = current
                    self.show_mode()
            if hasattr(self, "save_phrase"):
                self.editor_changed(self.editor.get_text())
        finally:
            self.filling = False

    def command(self, *args):
        return ["phrase-set", args[0], self.window.selected, self.data["name"], *args[1:]]

    def toggle_test(self, check):
        if self.filling or self.window.updating:
            return
        self.window.run(self.command("test", "on" if check.get_active() else "off"))

    def unsaved(self):
        """Whether the editor holds work that isn't saved: a changed loaded
        phrase, or a new one with any term typed."""
        if self.editing is not None:
            return self.editor.get_text().lower() != self.editing["text"]
        return any(terms for terms, _n in self.editor.word_sets())

    def edit_phrase(self, text, ask=True):
        """Loads a phrase (text) into the editor to change it, or empties the
        editor for a new one (None). Unsaved work is dropped only after
        asking."""
        old = next((p for p in self.data["phrases"] if p["text"] == text), None) if text else None
        if text and old is None:
            return
        if ask and self.unsaved() and not self.window.confirm(
                "Drop the unsaved phrase?",
                "The phrase in the editor has changes that aren't saved. They will be lost.",
                "Drop changes", destructive=True):
            return
        self.editing, self.saved_as = old, None
        self.show_mode()
        self.editor.load(old)

    def show_mode(self):
        old = self.editing
        self.editor_title.set_markup("<b>Change phrase</b>" if old else "<b>New phrase</b>")
        self.save_phrase.set_label("Save changes" if old else "Add phrase")
        self.delete_phrase.set_no_show_all(old is None)
        self.delete_phrase.set_visible(old is not None)

    def editor_changed(self, text):
        if hasattr(self, "save_phrase"):
            same = self.editing is not None and phrase_key(text) == phrase_key(self.editing["text"])
            known = self.editing is None and phrase_key(text) in {phrase_key(p["text"]) for p in self.data["phrases"]}
            self.save_phrase.set_sensitive(bool(text) and not same and not known and not self.saving)
            self.save_phrase.set_tooltip_text("The set has this phrase already." if known else None)

    def commit_phrase(self):
        """Adds the editor's phrase, or changes the loaded one in one step
        (`phrase-set change`: in place, refused while locked unless it only
        adds terms, R-WEAK-5). Saving stays off until the new state is read,
        so a second click can't act on a phrase that is already gone. A
        change of letter case or spacing only is no change."""
        new, old = self.editor.get_text(), self.editing
        if not new or self.saving or (old and phrase_key(new) == phrase_key(old["text"])):
            return
        self.saving = True
        self.save_phrase.set_sensitive(False)
        self.saved_as = phrase_key(new)
        if old is None:
            self.add_entries([new])
            return
        self.window.run(self.command("change", old["text"], new))

    def remove_loaded(self):
        """Deletes the phrase loaded in the editor, after asking; once it is
        gone the editor is empty (the deleted phrase doesn't stay there as
        a new, unsaved one)."""
        old = self.editing
        if old is None or not self.window.confirm(
                "Delete this phrase?", old["text"] + "\n\nDeleting a phrase weakens the block.",
                "Delete", destructive=True):
            return

        def gone(_output):
            # A periodic refresh may have read the set without it first:
            # the editor then already holds it as a new phrase.
            if self.editing is old or (self.editing is None and
                                       phrase_key(self.editor.get_text()) == phrase_key(old["text"])):
                self.edit_phrase(None, ask=False)
        self.window.run(self.command("remove", old["text"]), then=gone)

    def save_numbers(self, _button):
        current = {a["action"]: a for a in self.data["actions"]}
        changes = [(name, spin.get_value_as_int()) for name, (_l, spin) in self.spins.items()
                   if name in current and spin.get_value_as_int() != current[name].get("distinct", 1)]

        def run(rest):
            if rest:
                (name, value), rest = rest[0], rest[1:]
                self.window.run(self.command("threshold", name, str(value)), then=lambda _o: run(rest))
        run(changes)

    def rename(self, _button):
        name = self.window.ask_text("Rename phrase set", initial=self.data["name"])
        if name and name != self.data["name"]:
            self.window.run(self.command("rename", name))

    def delete(self, _button):
        if self.window.confirm(f"Delete the phrase set {self.data['name']}?",
                               "Its phrases go with it. Deleting one with phrases weakens the block.",
                               "Delete", destructive=True):
            self.window.run(self.command("delete"))
