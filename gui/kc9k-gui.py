#!/usr/bin/env python3
"""Graphical interface for Killer Chicken 9000 (GTK 3).

All changes go through the kc9k command, so the interface follows the
same lock rules as the terminal and needs no privileges of its own.

Pages: Dashboard, Blocks, Device Blocks (whole-computer locks),
Statistics and Settings.
"""
import datetime
import json
import math
import os
import shlex
import subprocess
import threading

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
gi.require_version("PangoCairo", "1.0")
from gi.repository import Gdk, GLib, Gtk, Pango, PangoCairo  # noqa: E402

CLI = "kc9k"
REFRESH_SECONDS = 5
STATS_REFRESH_SECONDS = 30
# Pointer events for the charts' hover.
POINTER_EVENTS = Gdk.EventMask.POINTER_MOTION_MASK | Gdk.EventMask.LEAVE_NOTIFY_MASK

LIBREWOLF_OPTIONS = (
    ("force-librewolf", "Allow only LibreWolf",
     "Closes every other browser at once, even when no block is on."),
)

BROWSER_OPTIONS = (
    ("block-devtools", "Block LibreWolf's developer tools",
     "They can switch the extension off. Turn this off only to develop websites; "
     "it applies when LibreWolf restarts."),
    ("no-private", "Disable private windows",
     "No private browsing in LibreWolf; it applies when LibreWolf restarts."),
    ("force-search", "Force Google as the search engine", ""),
    ("safesearch", "Force Google SafeSearch", ""),
    ("web-only", "Web results only",
     "No image, shopping or AI results in Google."),
)

# What a blacklist does to the pages it matches; any combination.
SITE_ACTIONS = (
    ("block", "Block page"),
    ("greyscale", "Greyscale"),
    ("no-images", "Block images"),
    ("redirect", "Open another page"),
    ("image", "Show a picture"),
    ("close-tab", "Close the tab"),
    ("close-browser", "Close the browser"),
    ("logout", "Log out"),
    ("shutdown", "Shut down"),
    ("monk", "Monk mode"),
)

# Actions with settings: what to ask for when one is ticked.
PARAM_ACTIONS = ("redirect", "image", "logout", "shutdown", "monk")

# What bypass prevention can do, mildest first.
CONSEQUENCES = (
    ("close-browser", "Close the browser"),
    ("lock-screen", "Lock the screen"),
    ("logout", "Log out"),
    ("shutdown", "Shut down"),
    ("monk", "Monk mode"),
)


def fmt_minutes(minutes):
    """30 min, 2 h, 1 h 30 min."""
    hours, rest = divmod(int(minutes), 60)
    if not hours:
        return f"{rest} min"
    return f"{hours} h" + (f" {rest} min" if rest else "")


def describe_nepsis(nepsis, count=None):
    """One line for a bypass prevention setting."""
    if not nepsis or not nepsis.get("count"):
        return "Off."
    then = dict(CONSEQUENCES).get(nepsis["consequence"], nepsis["consequence"])
    if nepsis.get("action_minutes"):
        then += f" for {fmt_minutes(nepsis['action_minutes'])}"
    line = f"{nepsis['count']} stopped things within {fmt_minutes(nepsis['minutes'])}: {then.lower()}."
    if count is not None:
        line += f" {count} so far."
    return line

# Where a list looks.
LIST_PLACES = (
    ("url", "Address"),
    ("title", "Page title"),
    ("text", "Page text"),
)


# Helpers -----------------------------------------------------------------

def fmt_seconds(seconds):
    """A remaining time, rounded to minutes: '2 h 5 min', '3 min'."""
    hours, rest = divmod(int(seconds), 3600)
    return f"{hours} h {rest // 60} min" if hours else f"{max(rest // 60, 1)} min"


def fmt_spent(seconds):
    """Time spent: '45s', '12m', '3h 04m'."""
    seconds = int(seconds)
    if seconds < 60:
        return f"{seconds}s"
    if seconds < 3600:
        return f"{seconds // 60}m"
    return f"{seconds // 3600}h {seconds % 3600 // 60:02d}m"


def run_cli(args, secret=None, timeout=120):
    """Return (ok, output) for one command."""
    try:
        result = subprocess.run(
            [CLI, *args], input=secret if secret else "",
            capture_output=True, text=True, timeout=timeout,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        return False, str(error)
    return result.returncode == 0, (result.stdout + result.stderr).strip()


def heading(text, size="large"):
    label = Gtk.Label(xalign=0)
    label.set_markup(f"<span size='{size}' weight='bold'>{GLib.markup_escape_text(text)}</span>")
    return label


def dim(text, wrap=True):
    label = Gtk.Label(label=text, xalign=0, wrap=wrap)
    label.get_style_context().add_class("dim-label")
    return label


CHALLENGE_MIN, CHALLENGE_MAX, CHALLENGE_DEFAULT = 10, 5000, 300

# Appearance is a preference of whoever runs the interface, so it lives in
# their own config, not in the KC9K database.
PREFS_PATH = os.path.join(os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config"),
                          "kc9k", "gui.json")
THEMES = (("system", "Same as the system"), ("light", "Light"), ("dark", "Dark"))
THEME_DIRS = ("/usr/share/themes", os.path.expanduser("~/.themes"),
              os.path.expanduser("~/.local/share/themes"))


def load_prefs():
    try:
        with open(PREFS_PATH) as f:
            prefs = json.load(f)
        return prefs if isinstance(prefs, dict) else {}
    except (OSError, ValueError):
        return {}


def save_prefs(prefs):
    try:
        os.makedirs(os.path.dirname(PREFS_PATH), exist_ok=True)
        with open(PREFS_PATH, "w") as f:
            json.dump(prefs, f)
    except OSError:
        pass  # the choice still applies until the interface closes


def theme_file(name, css):
    """Whether the GTK 3 theme `name` is installed with the stylesheet `css`."""
    if name == "Adwaita":
        return True  # built into GTK, with a dark variant
    return any(os.path.isfile(os.path.join(d, name, "gtk-3.0", css)) for d in THEME_DIRS)


def themed(base, dark):
    """The theme name and prefer-dark flag that make `base` light or dark.
    Themes with a dark stylesheet (gtk-dark.css) switch with the flag; others,
    like Mint-Y-Dark-Aqua, come as separate light and dark themes."""
    if dark:
        if theme_file(base, "gtk-dark.css") or "dark" in base.lower():
            return base, True
        parts = base.split("-")
        for i in range(len(parts), 0, -1):
            candidate = "-".join(parts[:i] + ["Dark"] + parts[i:])
            if theme_file(candidate, "gtk.css"):
                return candidate, True
        return "Adwaita", True
    for token in ("-Darker", "-Dark", "-dark"):
        candidate = base.replace(token, "", 1)
        if token in base and theme_file(candidate, "gtk.css"):
            return candidate, False
    return base, False


class Theme:
    """Applies the light, dark or system appearance to the whole interface."""

    def __init__(self):
        self.settings = Gtk.Settings.get_default()
        self.system = (self.settings.props.gtk_theme_name,
                       self.settings.props.gtk_application_prefer_dark_theme)

    def apply(self, choice):
        name, dark = self.system if choice not in ("light", "dark") else themed(self.system[0], choice == "dark")
        self.settings.props.gtk_theme_name = name
        self.settings.props.gtk_application_prefer_dark_theme = dark


def challenge_length(typing):
    """The challenge length chooser, usable only while typing is ticked."""
    chars = Gtk.SpinButton.new_with_range(CHALLENGE_MIN, CHALLENGE_MAX, 10)
    chars.set_value(CHALLENGE_DEFAULT)
    chars.set_sensitive(False)
    typing.connect("toggled", lambda button: chars.set_sensitive(button.get_active()))
    return chars


def attach_challenge_length(grid, chars, row):
    grid.attach(Gtk.Label(label="Challenge length (characters)", xalign=0), 0, row, 1, 1)
    grid.attach(chars, 1, row, 1, 1)
    grid.attach(dim(f"{CHALLENGE_MIN} to {CHALLENGE_MAX}. About 300 characters take a few "
                    "minutes to type; 5000 take about an hour."), 1, row + 1, 1, 1)


def set_switch(switch, value):
    """Show a switch's real state, also after a refused change (set_state alone
    is a no-op when the state didn't change, leaving the knob where it was)."""
    switch.set_active(value)
    switch.set_state(value)


def section(title, *children):
    """A titled group of widgets."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
    box.pack_start(heading(title, "medium"), False, False, 0)
    for child in children:
        box.pack_start(child, False, False, 0)
    return box


def page_box():
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18,
                  margin_start=24, margin_end=24, margin_top=20, margin_bottom=20)
    scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
    scroll.add(box)
    return scroll, box


def block_summary(block):
    """One line describing a block's state and locks."""
    parts = ["Active" if block["active"] else "Inactive"]
    if block["lock_seconds"]:
        parts.append(f"time-locked for {fmt_seconds(block['lock_seconds'])}")
    if block["char_locked"]:
        parts.append("typing challenge to stop")
    if block["password_locked"]:
        parts.append("password-locked")
    if block["restart_locked"]:
        parts.append("on until restart")
    if block["stop_seconds"]:
        parts.append(f"stops in {fmt_seconds(block['stop_seconds'])}")
    elif block["delay_seconds"]:
        parts.append(f"{fmt_seconds(block['delay_seconds'])} delay to stop")
    if block["pomodoro_on"]:
        parts.append(f"Pomodoro {block['pomodoro_on'] // 60}+{block['pomodoro_off'] // 60} min")
    if block["on_break"]:
        parts.append("on a break")
    if block["scheduled"]:
        parts.append("scheduled")
    if block["quota_limit"]:
        parts.append(f"quota {block['quota_used'] // 60} of {block['quota_limit'] // 60} min")
    if block["quota_open"]:
        parts.append("quota in use")
    return " · ".join(parts)


def theme_colors(widget):
    """Accent, text and faint (grid) colors from the GTK theme, as RGBA tuples,
    so charts follow the light or dark theme."""
    ctx = widget.get_style_context()
    fg = ctx.get_color(Gtk.StateFlags.NORMAL)
    found, accent = ctx.lookup_color("theme_selected_bg_color")
    if not found:
        found, accent = ctx.lookup_color("accent_bg_color")
    accent = (accent.red, accent.green, accent.blue, 1.0) if found else (0.2, 0.5, 0.8, 1.0)
    text = (fg.red, fg.green, fg.blue, 1.0)
    return accent, text, (fg.red, fg.green, fg.blue, 0.62), (fg.red, fg.green, fg.blue, 0.12)


def rounded_top_bar(cr, x, y, width, height, radius=4):
    """A bar with rounded data end (top) and a flat baseline."""
    radius = min(radius, width / 2, height)
    cr.move_to(x, y + height)
    cr.line_to(x, y + radius)
    cr.arc(x + radius, y + radius, radius, math.pi, 1.5 * math.pi)
    cr.arc(x + width - radius, y + radius, radius, 1.5 * math.pi, 2 * math.pi)
    cr.line_to(x + width, y + height)
    cr.close_path()


def rounded_end_bar(cr, x, y, width, height, radius=4):
    """A horizontal bar with a rounded data end (right) and a flat baseline."""
    radius = min(radius, height / 2, width)
    cr.move_to(x, y)
    cr.line_to(x + width - radius, y)
    cr.arc(x + width - radius, y + radius, radius, 1.5 * math.pi, 2 * math.pi)
    cr.arc(x + width - radius, y + height - radius, radius, 0, 0.5 * math.pi)
    cr.line_to(x, y + height)
    cr.close_path()


def nice_step(maximum):
    """A round gridline step in seconds for a time axis."""
    for step in (60, 300, 600, 900, 1800, 3600, 7200, 10800, 21600):
        if maximum / step <= 4:
            return step
    return 43200


# Widgets -----------------------------------------------------------------

class EditableList(Gtk.Box):
    """A titled list with an entry for adding and a button for removing."""

    def __init__(self, title, placeholder, on_add, on_remove,
                 on_import=None, on_export=None, help_text=""):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.on_add, self.on_remove = on_add, on_remove
        if title:
            self.pack_start(heading(title, "medium"), False, False, 0)
        if help_text:
            self.pack_start(dim(help_text), False, False, 0)

        self.store = Gtk.ListStore(str)
        self.view = Gtk.TreeView(model=self.store, headers_visible=False)
        self.view.append_column(
            Gtk.TreeViewColumn("", Gtk.CellRendererText(), text=0))
        scroll = Gtk.ScrolledWindow(min_content_height=120)
        scroll.set_shadow_type(Gtk.ShadowType.IN)
        scroll.add(self.view)
        self.pack_start(scroll, True, True, 0)

        row = Gtk.Box(spacing=6)
        self.entry = Gtk.Entry(placeholder_text=placeholder)
        self.entry.connect("activate", self._add)
        add = Gtk.Button(label="Add")
        add.connect("clicked", self._add)
        remove = Gtk.Button(label="Remove")
        remove.connect("clicked", self._remove)
        row.pack_start(self.entry, True, True, 0)
        row.pack_start(add, False, False, 0)
        row.pack_start(remove, False, False, 0)
        for label_text, handler in (("Import…", on_import), ("Export…", on_export)):
            if handler:
                button = Gtk.Button(label=label_text)
                button.connect("clicked", lambda _b, h=handler: h())
                row.pack_start(button, False, False, 0)
        self.pack_start(row, False, False, 0)

    def set_items(self, items):
        current = [row[0] for row in self.store]
        if current == list(items):
            return
        self.store.clear()
        for item in items:
            self.store.append([item])

    def _add(self, _widget):
        values = self.entry.get_text().split()
        if values:
            self.entry.set_text("")
            self.on_add(values)

    def _remove(self, _button):
        model, iterator = self.view.get_selection().get_selected()
        if iterator:
            self.on_remove(model[iterator][0], model.get_path(iterator)[0] + 1)


class ListCard(Gtk.Frame):
    """One website list: where it looks, its actions (blacklists), its
    thresholds for page text, and its entries. Every change runs through
    the window, which runs a kc9k command."""

    def __init__(self, window, data):
        super().__init__()
        self.window, self.data = window, data
        self.filling = False
        self.is_exceptions = data["kind"] == "exceptions"
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

        self.action_checks, self.spins = {}, {}
        self.threshold_row = None
        if not self.is_exceptions:
            box.pack_start(Gtk.Label(label="Actions:", xalign=0), False, False, 0)
            actions = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=5,
                                  column_spacing=12, row_spacing=2)
            for name, title in SITE_ACTIONS:
                check = Gtk.CheckButton(label=title)
                check.connect("toggled", self.toggle_action, name)
                self.action_checks[name] = check
                actions.add(check)
            box.pack_start(actions, False, False, 0)
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
            for row, (name, title) in enumerate(SITE_ACTIONS, start=1):
                label = Gtk.Label(label=title, xalign=0)
                distinct = Gtk.SpinButton.new_with_range(1, 1000, 1)
                hits = Gtk.SpinButton.new_with_range(1, 1000, 1)
                self.spins[name] = (label, distinct, hits)
                grid.attach(label, 0, row, 1, 1)
                grid.attach(distinct, 1, row, 1, 1)
                grid.attach(hits, 2, row, 1, 1)
            save = Gtk.Button(label="Save numbers", halign=Gtk.Align.START)
            save.connect("clicked", self.save_numbers)
            grid.attach(save, 1, len(SITE_ACTIONS) + 1, 2, 1)
            self.threshold_row.pack_start(grid, False, False, 0)
            box.pack_start(self.threshold_row, False, False, 0)

        self.entries = EditableList(
            "", "reddit.com  *casino*  nsfw", self.add_entries, self.remove_entry,
            lambda: window.import_list(["list", "import", window.selected, self.data["name"]]),
            lambda: window.export_list(["list", "export", window.selected, self.data["name"]]))
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
            for name, (row, label) in getattr(self, "param_rows", {}).items():
                row.set_no_show_all(name not in actions)
                row.set_visible(name in actions)
                if name in actions:
                    label.set_text(self.settings_text(name, actions[name]))
            for name, (label, distinct, hits) in self.spins.items():
                distinct.set_value(actions.get(name, {}).get("distinct", 1))
                hits.set_value(actions.get(name, {}).get("hits", 1))
                for widget in (label, distinct, hits):
                    widget.set_no_show_all(name not in actions)
                    widget.set_visible(name in actions)
            if self.threshold_row is not None:
                self.threshold_row.set_no_show_all("text" not in data["where"])
                self.threshold_row.set_visible("text" in data["where"])
            self.entries.set_items(data["entries"])
            words = "title" in data["where"] or "text" in data["where"]
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

    def toggle_place(self, check, name):
        if self.filling or self.window.updating:
            return
        self.window.run(self.command("where", name, "on" if check.get_active() else "off"),
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
        verb = "Logs out" if name == "logout" else "Shuts down"
        return f"{verb} for {fmt_minutes(minutes)}" if minutes else f"{verb} once"

    def ask_settings(self, name):
        """Asks for an action's address, picture or time, and turns it on with
        them (or changes them). Cancelling leaves the list as it was."""
        window, args = self.window, None
        current = {a["action"]: a for a in self.data["actions"]}.get(name, {})
        if name == "redirect":
            url = window.ask_text("Open another page", "The address to open instead, starting "
                                  "with https://.", initial=current.get("param", "https://"))
            if url:
                args = ["--url", url]
        elif name == "image":
            path = window.choose_file("Choose a picture", Gtk.FileChooserAction.OPEN, "Choose")
            if path:
                args = ["--image", path]
        else:
            details = {
                "monk": ("Monk mode", "The browser closes and every website is blocked, locked, "
                         "for this long (the block monk-mode).", 1, 60),
                "logout": ("Log out", "The browser closes and you are logged out. With a time, "
                           "you stay logged out that long; with 0, just once.", 0, 0),
                "shutdown": ("Shut down", "The browser closes and the computer shuts down. With a "
                             "time, it stays off that long; with 0, just once.", 0, 0),
            }[name]
            title, detail, minimum, initial = details
            limit = 30 * 24 * 60 if name == "monk" else 24 * 60
            minutes = window.ask_minutes(title, detail, minimum, limit, current.get("minutes", initial))
            if minutes is not None:
                args = ["--for", f"{minutes}m"] if minutes else []
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
        self.window.run_thresholds(self.window.selected, self.data["name"], changes)

    def rename(self, _button):
        name = self.window.ask_text("Rename list", initial=self.data["name"])
        if name and name != self.data["name"]:
            self.window.run(self.command("rename", name))

    def delete(self, _button):
        if self.window.confirm(f"Delete the list {self.data['name']}?",
                               "Its entries go with it. Deleting a blacklist loosens the block.",
                               "Delete", destructive=True):
            self.window.run(self.command("delete"), then=self.window.queued_note)


class StatTile(Gtk.Box):
    """A label above one large number."""

    def __init__(self, title):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        self.pack_start(dim(title, wrap=False), False, False, 0)
        self.value = Gtk.Label(xalign=0)
        self.pack_start(self.value, False, False, 0)
        self.set_value("–")

    def set_value(self, text):
        self.value.set_markup(f"<span size='xx-large' weight='bold'>{GLib.markup_escape_text(text)}</span>")


DAY_NAMES = ("mon", "tue", "wed", "thu", "fri", "sat", "sun")


class WeekCalendar(Gtk.DrawingArea):
    """A block's schedule as a week: Monday to Sunday across, 00:00 to 24:00
    down. Drag down a day to add a window (snapped to 15 minutes); click a
    window to remove it."""

    SNAP = 15

    def __init__(self, on_create, on_pick):
        super().__init__()
        self.on_create, self.on_pick = on_create, on_pick
        self.windows = []  # dicts: days (bit 0 Sunday), start, end, allow
        self.drag = None  # (column, start minute, current minute)
        self.press_y = 0
        self.set_size_request(-1, 380)
        self.add_events(Gdk.EventMask.BUTTON_PRESS_MASK | Gdk.EventMask.BUTTON_RELEASE_MASK
                        | Gdk.EventMask.POINTER_MOTION_MASK)
        self.connect("draw", self.on_draw)
        self.connect("button-press-event", self.on_press)
        self.connect("motion-notify-event", self.on_motion)
        self.connect("button-release-event", self.on_release)

    def set_windows(self, windows):
        self.windows = windows
        self.queue_draw()

    def geometry(self):
        left, top, right, bottom = 44, 22, 4, 4
        width = self.get_allocated_width() - left - right
        height = self.get_allocated_height() - top - bottom
        return left, top, width / 7, height

    def at(self, x, y):
        """The column and minute (snapped) under a point, clamped to the grid."""
        left, top, column_width, height = self.geometry()
        column = min(max(int((x - left) // column_width), 0), 6)
        minute = (y - top) / height * 1440
        minute = int(round(minute / self.SNAP) * self.SNAP)
        return column, min(max(minute, 0), 1440)

    @staticmethod
    def weekday_bit(column):
        """Monday is column 0; the day bits start at Sunday."""
        return 1 << ((column + 1) % 7)

    def window_at(self, column, minute):
        for index, w in enumerate(self.windows):
            if w["days"] & self.weekday_bit(column) and w["start"] <= minute < w["end"]:
                return index
        return None

    def on_press(self, _widget, event):
        if event.button != 1:
            return False
        column, minute = self.at(event.x, event.y)
        self.drag = (column, minute, minute)
        self.press_y = event.y
        return True

    def on_motion(self, _widget, event):
        if self.drag:
            column, start, _ = self.drag
            _, minute = self.at(event.x, event.y)
            self.drag = (column, start, minute)
            self.queue_draw()
        return False

    def on_release(self, _widget, event):
        if not self.drag:
            return False
        column, start, _ = self.drag
        _, end = self.at(event.x, event.y)
        self.drag = None
        self.queue_draw()
        start, end = min(start, end), max(start, end)
        if abs(event.y - self.press_y) >= 5:  # a drag, not a click that wobbled
            if end - start >= self.SNAP:
                self.on_create(column, start, end)
        else:
            exact = (event.y - self.geometry()[1]) / self.geometry()[3] * 1440
            index = self.window_at(column, exact)
            if index is not None:
                self.on_pick(index)
        return True

    def on_draw(self, widget, cr):
        accent, text, muted, faint = theme_colors(widget)
        left, top, column_width, height = self.geometry()
        layout = widget.create_pango_layout("")
        layout.set_font_description(Pango.FontDescription("Sans 8"))

        def y_of(minute):
            return top + height * minute / 1440

        # Hours: a faint line every 3 hours, labels every 6.
        for hour in range(0, 25, 3):
            y = round(y_of(hour * 60)) + 0.5
            cr.set_source_rgba(*faint)
            cr.set_line_width(1)
            cr.move_to(left, y)
            cr.line_to(left + 7 * column_width, y)
            cr.stroke()
            if hour % 6 == 0:
                layout.set_text(f"{hour:02d}:00", -1)
                w, h = layout.get_pixel_size()
                cr.set_source_rgba(*muted)
                cr.move_to(left - 6 - w, y - h / 2)
                PangoCairo.show_layout(cr, layout)
        for column in range(8):
            x = round(left + column * column_width) + 0.5
            cr.set_source_rgba(*faint)
            cr.move_to(x, top)
            cr.line_to(x, top + height)
            cr.stroke()
        for column, name in enumerate(DAY_NAMES):
            layout.set_text(name.capitalize(), -1)
            w, _h = layout.get_pixel_size()
            cr.set_source_rgba(*text)
            cr.move_to(left + column * column_width + column_width / 2 - w / 2, 4)
            PangoCairo.show_layout(cr, layout)

        r, g, b, _a = accent
        for w in self.windows:
            for column in range(7):
                if not w["days"] & self.weekday_bit(column):
                    continue
                x = left + column * column_width + 2
                y0, y1 = y_of(w["start"]), y_of(w["end"])
                cr.rectangle(x, y0, column_width - 4, max(y1 - y0, 2))
                if w["allow"]:
                    # Allow windows are gaps in an always-on block: outlined.
                    cr.set_source_rgba(r, g, b, 0.15)
                    cr.fill_preserve()
                    cr.set_source_rgba(r, g, b, 0.9)
                    cr.set_line_width(1.5)
                    cr.stroke()
                else:
                    cr.set_source_rgba(r, g, b, 0.8)
                    cr.fill()
        if self.drag:
            column, start, end = self.drag
            start, end = min(start, end), max(start, end)
            cr.set_source_rgba(*text[:3], 0.35)
            cr.rectangle(left + column * column_width + 2, y_of(start), column_width - 4,
                         max(y_of(end) - y_of(start), 2))
            cr.fill()
        return False


class DailyChart(Gtk.DrawingArea):
    """Time spent per day: one series of vertical bars, hover for the value."""

    def __init__(self):
        super().__init__()
        self.days = []  # [(date, seconds)]
        self.hover = None
        self.set_size_request(-1, 200)
        self.set_has_tooltip(True)
        self.add_events(POINTER_EVENTS)
        self.connect("draw", self.on_draw)
        self.connect("query-tooltip", self.on_tooltip)
        self.connect("motion-notify-event", self.on_motion)
        self.connect("leave-notify-event", self.on_leave)

    def set_days(self, days):
        self.days = days
        self.queue_draw()

    def geometry(self, width, height):
        left, right, top, bottom = 52, 8, 8, 24
        slot = (width - left - right) / max(len(self.days), 1)
        return left, top, slot, height - top - bottom

    def index_at(self, x):
        width = self.get_allocated_width()
        left, _top, slot, _plot = self.geometry(width, self.get_allocated_height())
        index = int((x - left) // slot) if slot > 0 else -1
        return index if 0 <= index < len(self.days) else None

    def on_motion(self, _widget, event):
        index = self.index_at(event.x)
        if index != self.hover:
            self.hover = index
            self.queue_draw()

    def on_leave(self, *_args):
        self.hover = None
        self.queue_draw()

    def on_tooltip(self, _widget, x, _y, _keyboard, tooltip):
        index = self.index_at(x)
        if index is None:
            return False
        day, seconds = self.days[index]
        tooltip.set_text(f"{day.strftime('%a %-d %b')}: {fmt_spent(seconds)}")
        return True

    def on_draw(self, widget, cr):
        width, height = widget.get_allocated_width(), widget.get_allocated_height()
        accent, text, muted, faint = theme_colors(widget)
        left, top, slot, plot = self.geometry(width, height)
        maximum = max([s for _d, s in self.days] + [60])
        step = nice_step(maximum)
        top_value = math.ceil(maximum / step) * step

        layout = widget.create_pango_layout("")
        layout.set_font_description(Pango.FontDescription("Sans 8"))

        # Recessive grid and axis labels.
        value = 0
        while value <= top_value:
            y = top + plot - plot * value / top_value
            cr.set_source_rgba(*faint)
            cr.set_line_width(1)
            cr.move_to(left, round(y) + 0.5)
            cr.line_to(width - 8, round(y) + 0.5)
            cr.stroke()
            layout.set_text(fmt_spent(value) if value else "0", -1)
            w, h = layout.get_pixel_size()
            cr.set_source_rgba(*muted)
            cr.move_to(left - 8 - w, y - h / 2)
            PangoCairo.show_layout(cr, layout)
            value += step

        # Bars: thin, 2px gaps, rounded data ends on the baseline.
        bar_width = max(min(slot - 2, 28), 1)
        label_every = 1 if len(self.days) <= 10 else math.ceil(len(self.days) / 8)
        for i, (day, seconds) in enumerate(self.days):
            x = left + i * slot + (slot - bar_width) / 2
            if seconds > 0:
                bar_height = max(plot * seconds / top_value, 2)
                r, g, b, _a = accent
                cr.set_source_rgba(r, g, b, 1.0 if self.hover in (None, i) else 0.45)
                rounded_top_bar(cr, x, top + plot - bar_height, bar_width, bar_height)
                cr.fill()
            if i % label_every == 0 or i == len(self.days) - 1:
                layout.set_text(day.strftime("%a" if len(self.days) <= 7 else "%-d %b"), -1)
                w, _h = layout.get_pixel_size()
                cr.set_source_rgba(*(text if self.hover == i else muted))
                cr.move_to(left + i * slot + slot / 2 - w / 2, top + plot + 6)
                PangoCairo.show_layout(cr, layout)
        return False


class TopChart(Gtk.DrawingArea):
    """The most used sites or apps: horizontal bars with name and time."""

    ROW = 26

    def __init__(self):
        super().__init__()
        self.items = []  # [(name, seconds)]
        self.hover = None
        self.set_has_tooltip(True)
        self.add_events(POINTER_EVENTS)
        self.connect("draw", self.on_draw)
        self.connect("query-tooltip", self.on_tooltip)
        self.connect("motion-notify-event", self.on_motion)
        self.connect("leave-notify-event", self.on_leave)

    def set_items(self, items):
        self.items = items
        self.set_size_request(-1, max(len(items), 1) * self.ROW + 4)
        self.queue_draw()

    def index_at(self, y):
        index = int(y // self.ROW)
        return index if 0 <= index < len(self.items) else None

    def on_motion(self, _widget, event):
        index = self.index_at(event.y)
        if index != self.hover:
            self.hover = index
            self.queue_draw()

    def on_leave(self, *_args):
        self.hover = None
        self.queue_draw()

    def on_tooltip(self, _widget, _x, y, _keyboard, tooltip):
        index = self.index_at(y)
        if index is None:
            return False
        name, seconds = self.items[index]
        total = sum(s for _n, s in self.items) or 1
        tooltip.set_text(f"{name}: {fmt_spent(seconds)} ({round(100 * seconds / total)}% of the listed time)")
        return True

    def on_draw(self, widget, cr):
        width = widget.get_allocated_width()
        accent, text, muted, _faint = theme_colors(widget)
        layout = widget.create_pango_layout("")
        layout.set_font_description(Pango.FontDescription("Sans 9"))
        longest = 0
        for name, _seconds in self.items:
            layout.set_text(name, -1)
            longest = max(longest, layout.get_pixel_size()[0])
        name_width, value_width = min(longest + 16, 260, width * 0.4), 70
        bar_space = max(width - name_width - value_width - 16, 10)
        maximum = max([s for _n, s in self.items] + [1])
        for i, (name, seconds) in enumerate(self.items):
            y = i * self.ROW
            layout.set_width(int((name_width - 8) * Pango.SCALE))
            layout.set_ellipsize(Pango.EllipsizeMode.END)
            layout.set_text(name, -1)
            _w, h = layout.get_pixel_size()
            cr.set_source_rgba(*text)
            cr.move_to(0, y + (self.ROW - h) / 2)
            PangoCairo.show_layout(cr, layout)

            bar = max(bar_space * seconds / maximum, 2)
            r, g, b, _a = accent
            cr.set_source_rgba(r, g, b, 1.0 if self.hover in (None, i) else 0.45)
            rounded_end_bar(cr, name_width, y + 7, bar, self.ROW - 14)
            cr.fill()

            layout.set_width(-1)
            layout.set_text(fmt_spent(seconds), -1)
            _w, h = layout.get_pixel_size()
            cr.set_source_rgba(*muted)
            cr.move_to(name_width + bar + 8, y + (self.ROW - h) / 2)
            PangoCairo.show_layout(cr, layout)
        return False


# The window --------------------------------------------------------------

class Window(Gtk.ApplicationWindow):
    def __init__(self, app):
        super().__init__(application=app, title="Killer Chicken 9000")
        self.set_default_size(980, 680)
        self.state_json = None
        self.state = None
        self.blocks = {}
        self.selected = None
        self.busy = False
        self.updating = False  # set while widgets are filled from the state
        self.settings_password = None  # remembered for this session once given
        self.stats_rows = []
        self.stats_on = True

        header = Gtk.HeaderBar(show_close_button=True, title="Killer Chicken 9000")
        self.set_titlebar(header)

        root = Gtk.Box()
        self.add(root)
        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE)
        sidebar = Gtk.StackSidebar(stack=self.stack)
        sidebar.set_size_request(180, -1)
        root.pack_start(sidebar, False, False, 0)
        root.pack_start(Gtk.Separator(orientation=Gtk.Orientation.VERTICAL), False, False, 0)

        content = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        root.pack_start(content, True, True, 0)
        self.banner = Gtk.InfoBar(message_type=Gtk.MessageType.WARNING, revealed=False)
        self.banner_label = Gtk.Label(xalign=0, wrap=True)
        self.banner.get_content_area().add(self.banner_label)
        content.pack_start(self.banner, False, False, 0)
        content.pack_start(self.stack, True, True, 0)

        self.stack.add_titled(self.build_dashboard(), "dashboard", "Dashboard")
        self.stack.add_titled(self.build_blocks(), "blocks", "Blocks")
        self.stack.add_titled(self.build_device(), "device", "Device Blocks")
        self.stack.add_titled(self.build_stats(), "stats", "Statistics")
        self.stack.add_titled(self.build_settings(), "settings", "Settings")
        self.stack.connect("notify::visible-child-name", self.page_changed)

        self.refresh()
        self.refresh_stats()
        GLib.timeout_add_seconds(REFRESH_SECONDS, self.refresh)
        GLib.timeout_add_seconds(STATS_REFRESH_SECONDS, self.refresh_stats)

    # Running commands ----------------------------------------------------
    def run(self, args, secret=None, then=None, timeout=120):
        """Run a command in the background, then refresh.

        With a settings password set, the password remembered for this
        session goes first on standard input; commands that don't need it
        never read it. A command that needs it and doesn't have it fails,
        and finished() asks for it and runs the command again."""
        self.set_sensitive(False)
        stdin = secret
        if self.settings_password and self.state and self.state.get("settings_password"):
            stdin = self.settings_password + "\n" + (secret or "")

        def work():
            ok, output = run_cli(args, stdin, timeout)
            GLib.idle_add(self.finished, ok, output, then, (args, secret, timeout))

        threading.Thread(target=work, daemon=True).start()

    def finished(self, ok, output, then, request=None):
        self.set_sensitive(True)
        if not ok and request and ("needs the settings password" in output
                                   or "wrong settings password" in output):
            wrong = "wrong settings password" in output and self.settings_password
            self.settings_password = None
            password = self.ask_text(
                "Settings password",
                "That password was wrong. Try again." if wrong else
                "This change needs the settings password.", password=True)
            if password:
                self.settings_password = password
                args, secret, timeout = request
                self.state = dict(self.state or {}, settings_password=True)
                self.run(args, secret, then, timeout)
            return False
        if not ok:
            self.message(output or "The command failed.")
        elif then:
            then(output)
        self.state_json = None  # repaint even if the state looks unchanged
        self.refresh()
        return False

    def message(self, text, kind=Gtk.MessageType.ERROR):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=text, message_type=kind,
            buttons=Gtk.ButtonsType.OK)
        dialog.run()
        dialog.destroy()

    def confirm(self, text, detail, ok_label="OK", destructive=False):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=text, secondary_text=detail,
            message_type=Gtk.MessageType.WARNING)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button(ok_label, Gtk.ResponseType.OK)
        if destructive:
            ok.get_style_context().add_class("destructive-action")
        answer = dialog.run()
        dialog.destroy()
        return answer == Gtk.ResponseType.OK

    def ask_text(self, title, detail="", password=False, initial=""):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=title, secondary_text=detail or None,
            message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
        entry = Gtk.Entry(activates_default=True, visibility=not password, text=initial)
        dialog.set_default_response(Gtk.ResponseType.OK)
        dialog.get_message_area().pack_start(entry, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        value = entry.get_text() if password else entry.get_text().strip()
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK else ""

    def ask_minutes(self, title, detail, minimum, maximum, initial):
        """Asks for a number of minutes; None when cancelled."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=title, secondary_text=detail,
            message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
        row = Gtk.Box(spacing=8)
        spin = Gtk.SpinButton.new_with_range(minimum, maximum, 5)
        spin.set_value(max(minimum, initial))
        row.pack_start(spin, False, False, 0)
        row.pack_start(Gtk.Label(label="minutes"), False, False, 0)
        dialog.get_message_area().pack_start(row, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        value = spin.get_value_as_int()
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK else None

    def ask_nepsis(self, title, current, allow_lock):
        """Asks for a bypass prevention setting; returns the nepsis set
        arguments, or None when cancelled."""
        current = current if current and current.get("count") else {
            "count": 10, "minutes": 5, "consequence": "monk", "action_minutes": 60}
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_buttons("Cancel", Gtk.ResponseType.CANCEL, "Save", Gtk.ResponseType.OK)
        area = dialog.get_content_area()
        area.set_spacing(10)
        area.set_border_width(16)
        area.pack_start(dim("When Killer Chicken stops this many things (pages, apps, windows, "
                            "attempts to loosen a held block) within this long, it does this. "
                            "Making it stricter is always allowed; anything else loosens it."),
                        False, False, 0)
        grid = Gtk.Grid(column_spacing=10, row_spacing=8)
        count = Gtk.SpinButton.new_with_range(1, 1000, 1)
        count.set_value(current["count"])
        within = Gtk.SpinButton.new_with_range(1, 24 * 60, 1)
        within.set_value(current["minutes"])
        then = Gtk.ComboBoxText()
        for key, label in CONSEQUENCES:
            then.append(key, label)
        then.set_active_id(current["consequence"])
        then_for = Gtk.SpinButton.new_with_range(0, 30 * 24 * 60, 5)
        then_for.set_value(current.get("action_minutes", 0))
        rows = [("Things stopped", count), ("Within (minutes)", within), ("Then", then),
                ("For (minutes; 0 for once)", then_for)]
        lock = None
        if allow_lock:
            lock = Gtk.SpinButton.new_with_range(0, 24 * 30, 1)
            rows.append(("Can't be loosened for (hours)", lock))
        for row, (label, widget) in enumerate(rows):
            grid.attach(Gtk.Label(label=label, xalign=0), 0, row, 1, 1)
            grid.attach(widget, 1, row, 1, 1)
        area.pack_start(grid, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        args = ["--count", str(count.get_value_as_int()), "--within", f"{within.get_value_as_int()}m",
                "--then", then.get_active_id() or "close-browser"]
        if then.get_active_id() != "close-browser" and then_for.get_value_as_int():
            args += ["--then-for", f"{then_for.get_value_as_int()}m"]
        if lock is not None and lock.get_value_as_int():
            args += ["--for", f"{lock.get_value_as_int()}h"]
        dialog.destroy()
        return args if answer == Gtk.ResponseType.OK else None

    def monk_mode(self, _button):
        minutes = self.ask_minutes(
            "Monk mode", "Every website is blocked, locked, for this long (the block monk-mode: add "
            "exceptions or apps to it under Blocks). A running monk mode can only be extended.",
            1, 30 * 24 * 60, 60)
        if minutes:
            self.run(["monk", "--for", f"{minutes}m"])

    def change_global_nepsis(self, _button):
        args = self.ask_nepsis("Bypass prevention", self.state.get("nepsis"), True)
        if args:
            self.run(["nepsis", "set", *args], then=self.queued_note)

    def change_block_nepsis(self, _button):
        block = self.blocks.get(self.selected)
        if block:
            args = self.ask_nepsis(f"Bypass prevention for {block['name']}", block.get("nepsis"), False)
            if args:
                self.run(["nepsis", "set", "--block", block["name"], *args], then=self.queued_note)

    def nepsis_off(self, _button, block=None):
        if self.confirm("Turn bypass prevention off?", "This loosens it, so it isn't possible while "
                        "it is locked (or, for a block, while the block is held).", "Turn off"):
            self.run(["nepsis", "off"] + (["--block", block()] if block else []), then=self.queued_note)

    def run_thresholds(self, block_name, list_name, changes):
        """Set each (threshold or hits, action, number) of a list in turn: one
        command at a time."""
        if not changes:
            return
        (command, name, value), rest = changes[0], changes[1:]
        self.run(["list", command, block_name, list_name, name, str(value)],
                 then=lambda output: (self.queued_note(output),
                                      self.run_thresholds(block_name, list_name, rest)))

    def queued_note(self, output):
        """Says so when a change was queued instead of applied."""
        if "queued" in output:
            self.message(output, Gtk.MessageType.INFO)

    def block_cmd(self, command, values):
        if self.selected:
            self.run(["block", command, self.selected, *values])

    # State ---------------------------------------------------------------
    def refresh(self):
        if self.busy:
            return True
        self.busy = True

        def work():
            ok, output = run_cli(["gui-state"])
            GLib.idle_add(self.apply_state, ok, output)

        threading.Thread(target=work, daemon=True).start()
        return True

    def apply_state(self, ok, output):
        self.busy = False
        if not ok:
            self.show_banner(f"Can't read the state: {output}", Gtk.MessageType.ERROR)
            return False
        if output == self.state_json:
            return False
        self.state_json = output
        self.state = json.loads(output)
        self.blocks = {block["name"]: block for block in self.state["blocks"]}
        self.updating = True
        try:
            frozen = self.state["freeze_seconds"]
            if frozen:
                self.show_banner(f"Frozen Chicken is on for {fmt_seconds(frozen)} more.",
                                 Gtk.MessageType.WARNING)
            else:
                self.banner.set_revealed(False)
            self.update_dashboard()
            self.update_block_list()
            self.update_device()
            self.update_settings()
        finally:
            self.updating = False
        return False

    def show_banner(self, text, kind):
        self.banner_label.set_text(text)
        self.banner.set_message_type(kind)
        self.banner.set_revealed(True)

    def page_changed(self, *_args):
        if self.stack.get_visible_child_name() in ("stats", "dashboard"):
            self.refresh_stats()

    # Dashboard -----------------------------------------------------------
    def build_dashboard(self):
        scroll, box = page_box()
        box.pack_start(heading("Dashboard", "x-large"), False, False, 0)

        tiles = Gtk.Box(spacing=40)
        self.tile_active = StatTile("Active blocks")
        self.tile_sites = StatTile("Websites today")
        self.tile_apps = StatTile("Apps today")
        for tile in (self.tile_active, self.tile_sites, self.tile_apps):
            tiles.pack_start(tile, False, False, 0)
        box.pack_start(tiles, False, False, 0)

        self.dash_locks = dim("")
        box.pack_start(section("Device", self.dash_locks), False, False, 0)

        self.dash_blocks = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.dash_blocks.get_style_context().add_class("frame")
        self.dash_blocks.connect("row-activated", self.open_block_from_dashboard)
        box.pack_start(section("Active blocks", self.dash_blocks), False, False, 0)

        self.dash_top = TopChart()
        self.dash_top_empty = dim("Nothing recorded today yet.")
        box.pack_start(section("Most used today", self.dash_top, self.dash_top_empty), False, False, 0)

        actions = Gtk.Box(spacing=6)
        new_block = Gtk.Button(label="New block…")
        new_block.connect("clicked", self.create_block)
        freeze = Gtk.Button(label="Frozen Chicken…")
        freeze.connect("clicked", lambda _b: self.stack.set_visible_child_name("device"))
        monk = Gtk.Button(label="Monk mode…")
        monk.connect("clicked", self.monk_mode)
        actions.pack_start(new_block, False, False, 0)
        actions.pack_start(monk, False, False, 0)
        actions.pack_start(freeze, False, False, 0)
        box.pack_start(actions, False, False, 0)
        return scroll

    def update_dashboard(self):
        active = [b for b in self.state["blocks"] if b["active"]]
        self.tile_active.set_value(str(len(active)))
        for row in self.dash_blocks.get_children():
            self.dash_blocks.remove(row)
        if not active:
            row = Gtk.ListBoxRow(activatable=False)
            row.add(dim("No block is on. Start one from Blocks.", wrap=False))
            row.get_child().set_margin_start(12)
            row.get_child().set_margin_top(10)
            row.get_child().set_margin_bottom(10)
            self.dash_blocks.add(row)
        for block in active:
            row = Gtk.ListBoxRow()
            row.block_name = block["name"]
            inner = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2,
                            margin_start=12, margin_end=12, margin_top=8, margin_bottom=8)
            name = Gtk.Label(xalign=0)
            name.set_markup(f"<b>{GLib.markup_escape_text(block['name'])}</b>")
            inner.pack_start(name, False, False, 0)
            inner.pack_start(dim(block_summary(block)), False, False, 0)
            row.add(inner)
            self.dash_blocks.add(row)
        self.dash_blocks.show_all()

        locks = []
        if self.state["freeze_seconds"]:
            locks.append(f"Frozen Chicken ({self.state['freeze_mode']}) for {fmt_seconds(self.state['freeze_seconds'])}")
        locks.append("strict mode on" if self.state["strict"] else "strict mode off")
        locks.append("no-escape on" if self.state["no_escape"] else "no-escape off")
        line = " · ".join(locks)
        self.dash_locks.set_text(line[:1].upper() + line[1:])

    def open_block_from_dashboard(self, _box, row):
        self.selected = row.block_name
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
        top = sorted(((r["name"], r["seconds"]) for r in rows), key=lambda item: -item[1])[:5]
        self.dash_top.set_items(top)
        self.dash_top.set_visible(bool(top))
        self.dash_top_empty.set_visible(not top)
        self.dash_top_empty.set_text(
            "Nothing recorded today yet." if self.stats_on
            else "Statistics are off. Turn them on in Settings.")

    # Blocks --------------------------------------------------------------
    def build_blocks(self):
        paned = Gtk.Paned()

        left = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        self.block_list = Gtk.ListBox()
        self.block_list.connect("row-selected", self.block_selected)
        list_scroll = Gtk.ScrolledWindow(min_content_width=210, vexpand=True)
        list_scroll.add(self.block_list)
        left.pack_start(list_scroll, True, True, 0)
        new_block = Gtk.Button(label="New block…", margin=8)
        new_block.connect("clicked", self.create_block)
        left.pack_start(new_block, False, False, 0)
        paned.pack1(left, False, False)

        self.detail = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10,
                              margin_start=20, margin_end=20, margin_top=16, margin_bottom=16)
        # The details scroll, so the window fits small and scaled screens.
        detail_scroll = Gtk.ScrolledWindow(propagate_natural_width=True)
        detail_scroll.add(self.detail)
        paned.pack2(detail_scroll, True, False)
        self.title_label = Gtk.Label(xalign=0)
        self.status_label = dim("")
        self.detail.pack_start(self.title_label, False, False, 0)
        self.detail.pack_start(self.status_label, False, False, 0)

        actions = Gtk.Box(spacing=6)
        self.start_button = Gtk.Button(label="Start…")
        self.start_button.get_style_context().add_class("suggested-action")
        self.start_button.connect("clicked", self.start_block)
        self.stop_button = Gtk.Button(label="Stop")
        self.stop_button.connect("clicked", self.stop_block)
        self.unlock_button = Gtk.Button(label="Unlock…")
        self.unlock_button.connect("clicked", self.unlock_block)
        for button in (self.start_button, self.stop_button, self.unlock_button):
            actions.pack_start(button, False, False, 0)
        self.delete_button = Gtk.Button(label="Delete…")
        self.delete_button.get_style_context().add_class("destructive-action")
        self.delete_button.connect("clicked", self.delete_block)
        rename = Gtk.Button(label="Rename…")
        rename.connect("clicked", self.rename_block)
        duplicate = Gtk.Button(label="Duplicate…")
        duplicate.connect("clicked", self.duplicate_block)
        for button in (self.delete_button, rename, duplicate):
            actions.pack_end(button, False, False, 0)
        self.detail.pack_start(actions, False, False, 0)

        tabs = Gtk.Notebook()
        self.detail.pack_start(tabs, True, True, 0)

        websites = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14, margin=12)
        websites_help = dim(
            "Each list ticks where it looks: the address, the page title (the tab title) or the "
            "page text. Addresses take domains, paths and wildcards (*.xxx, *casino*, *.* for "
            "everything); titles and text take words: a plain word counts only on its own "
            "(porn misses pornhub), * matches inside words (*porn* hits pornhub). A match in "
            "the address or title takes every action; in the text, each action needs its number "
            "of different words. Unticking or removing loosens the block, so it isn't possible "
            "while it is locked.")
        # A wrapped label asks for its whole text on one line; cap it, so the
        # window still fits small screens.
        websites_help.set_max_width_chars(70)
        websites.pack_start(websites_help, False, False, 0)
        self.list_cards = {}
        self.list_boxes = {}
        for kind, title, button_label, detail in (
                ("blacklist", "Blacklists", "New blacklist…", ""),
                ("exceptions", "Exceptions", "New exception list…",
                 "A page an exception list matches is spared from every blacklist of this block.")):
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
                      "whose programs can't run. Everything a blocked app started is closed too.")
        apps.pack_start(self.apps, True, True, 0)
        self.titles = EditableList(
            "Window titles", "YouTube  Twitch",
            lambda values: self.block_cmd("add-title", values),
            lambda value, _num: self.block_cmd("remove-title", [value]),
            lambda: self.import_list(["block", "import-list", self.selected, "titles"]),
            lambda: self.export_list(["block", "export-list", self.selected, "titles"]),
            help_text="Any app whose window title contains one of these words is closed while "
                      "the block is on; in LibreWolf, just that tab. Separate words with spaces.")
        apps.set_spacing(14)
        apps.pack_start(self.titles, True, True, 0)
        tabs.append_page(apps, Gtk.Label(label="Apps"))

        schedule = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, margin=12)
        self.calendar = WeekCalendar(self.create_window_from_calendar, self.remove_window_from_calendar)
        schedule.pack_start(dim("Drag down a day to add a window; click one to remove it. Filled: the "
                                "block is on. Outlined: an allow window."), False, False, 0)
        schedule.pack_start(self.calendar, False, False, 0)
        self.schedules = EditableList(
            "Schedule", "weekdays 09:00 17:00", self.add_schedule, self.remove_schedule,
            help_text="Days, start and end: weekdays 09:00 17:00, or mon,wed,fri 18:00 22:00. "
                      "The block turns on and off by itself, and can't be weakened from "
                      "2 hours before a window opens.")
        schedule.pack_start(self.schedules, True, True, 0)
        tabs.append_page(schedule, Gtk.Label(label="Schedule"))

        tabs.append_page(self.build_quota_tab(), Gtk.Label(label="Quota & Pomodoro"))

        nepsis = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10, margin=12)
        nepsis.pack_start(dim(
            "Bypass prevention: when this block stops many things in a short time (pages, apps, "
            "attempts to loosen it while held), likely a hunt for a loophole, it takes a "
            "consequence so you step away. It counts while the block is on; the one under "
            "Settings counts everything, and both apply."), False, False, 0)
        self.block_nepsis_label = Gtk.Label(xalign=0, wrap=True)
        nepsis.pack_start(self.block_nepsis_label, False, False, 0)
        row = Gtk.Box(spacing=6)
        change = Gtk.Button(label="Change…")
        change.connect("clicked", self.change_block_nepsis)
        off = Gtk.Button(label="Turn off…")
        off.connect("clicked", self.nepsis_off, lambda: self.selected)
        row.pack_start(change, False, False, 0)
        row.pack_start(off, False, False, 0)
        nepsis.pack_start(row, False, False, 0)
        tabs.append_page(nepsis, Gtk.Label(label="Bypass prevention"))
        return paned

    def build_quota_tab(self):
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18, margin=12)

        quota_row = Gtk.Box(spacing=6)
        quota_row.pack_start(Gtk.Label(label="Minutes per day:"), False, False, 0)
        self.quota_minutes = Gtk.SpinButton.new_with_range(1, 1440, 5)
        self.quota_minutes.set_value(30)
        quota_row.pack_start(self.quota_minutes, False, False, 0)
        self.quota_set = Gtk.Button(label="Save quota")
        self.quota_set.connect("clicked", self.set_quota)
        self.quota_start = Gtk.Button(label="Use quota now")
        self.quota_start.connect("clicked", self.start_quota)
        self.quota_end = Gtk.Button(label="End session")
        self.quota_end.connect("clicked", self.end_quota)
        self.quota_remove = Gtk.Button(label="Remove at midnight")
        self.quota_remove.connect("clicked", self.remove_quota)
        quota_row.pack_start(self.quota_set, False, False, 0)
        # The session buttons get their own row, so the page fits a narrow screen.
        session_row = Gtk.Box(spacing=6)
        for button in (self.quota_start, self.quota_end, self.quota_remove):
            session_row.pack_start(button, False, False, 0)
        self.quota_label = dim("")
        box.pack_start(section(
            "Daily quota",
            dim("The block stays on, except during sessions you open, until the day's minutes are spent."),
            quota_row, session_row, self.quota_label), False, False, 0)

        pomodoro_row = Gtk.Box(spacing=6)
        pomodoro_row.pack_start(Gtk.Label(label="Work:"), False, False, 0)
        self.work_minutes = Gtk.SpinButton.new_with_range(1, 240, 5)
        self.work_minutes.set_value(25)
        pomodoro_row.pack_start(self.work_minutes, False, False, 0)
        pomodoro_row.pack_start(Gtk.Label(label="min   Break:"), False, False, 0)
        self.break_minutes = Gtk.SpinButton.new_with_range(1, 240, 5)
        self.break_minutes.set_value(5)
        pomodoro_row.pack_start(self.break_minutes, False, False, 0)
        pomodoro_row.pack_start(Gtk.Label(label="min"), False, False, 0)
        set_pomodoro = Gtk.Button(label="Save")
        set_pomodoro.connect("clicked", self.set_pomodoro)
        clear_pomodoro = Gtk.Button(label="Turn off")
        clear_pomodoro.connect("clicked", self.clear_pomodoro)
        pomodoro_row.pack_start(set_pomodoro, False, False, 0)
        pomodoro_row.pack_start(clear_pomodoro, False, False, 0)
        box.pack_start(section(
            "Pomodoro", dim("While the block is on, it pauses for the break after each work period."),
            pomodoro_row), False, False, 0)
        return box

    def update_block_list(self):
        names = list(self.blocks)
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
            self.selected = names[0] if names else None
        for row in self.block_list.get_children():
            if row.block_name == self.selected:
                self.block_list.select_row(row)
        self.show_block()

    def block_selected(self, _box, row):
        if row is None or self.updating:
            return
        self.selected = row.block_name
        self.show_block()

    def show_block(self):
        block = self.blocks.get(self.selected)
        self.detail.set_sensitive(block is not None)
        if block is None:
            self.title_label.set_markup("<big><b>No blocks yet</b></big>")
            self.status_label.set_text("Create a block, add sites or apps, then start it.")
            for widget in (self.apps, self.titles, self.schedules):
                widget.set_items([])
            self.update_lists([])
            self.calendar.set_windows([])
            self.quota_label.set_text("")
            self.block_nepsis_label.set_text("")
            return
        self.title_label.set_markup(f"<big><b>{GLib.markup_escape_text(block['name'])}</b></big>")
        self.status_label.set_text(block_summary(block))
        self.update_lists(block.get("lists", []))
        self.apps.set_items(block["apps"])
        self.titles.set_items(block.get("titles", []))
        self.schedules.set_items(block["schedules"])
        self.calendar.set_windows(block.get("schedule_windows", []))

        self.start_button.set_label("Add locks…" if block["active"] else "Start…")
        self.stop_button.set_sensitive(
            block["active"] and (not block["held"] or (
                block["delay_seconds"] > 0 and not block["lock_seconds"]
                and not block["char_locked"] and not block["password_locked"]
                and not block["restart_locked"] and not block["quota_limit"])))
        self.unlock_button.set_sensitive(
            (block["char_locked"] or block["password_locked"])
            and not block["lock_seconds"] and not block["restart_locked"])
        self.delete_button.set_sensitive(not block["active"] and not block["held"])

        limit = block["quota_limit"]
        self.quota_start.set_sensitive(limit > 0 and not block["quota_open"] and block["quota_used"] < limit)
        self.quota_end.set_sensitive(block["quota_open"])
        self.quota_remove.set_sensitive(limit > 0)
        self.quota_label.set_text(
            f"Used today: {block['quota_used'] // 60} of {limit // 60} min"
            + (" · session open" if block["quota_open"] else "") if limit else "No quota set.")
        self.block_nepsis_label.set_text(describe_nepsis(block.get("nepsis"), block.get("nepsis_count")
                                                         if block["active"] else None))

    def update_lists(self, lists):
        """Shows the selected block's lists, keeping the cards of lists that
        are still there, so a refresh doesn't disturb what is being typed."""
        ids = [entry["id"] for entry in lists]
        for list_id in list(self.list_cards):
            if list_id not in ids:
                card = self.list_cards.pop(list_id)
                card.get_parent().remove(card)
        for entry in lists:
            card = self.list_cards.get(entry["id"])
            if card is None:
                card = ListCard(self, entry)
                self.list_cards[entry["id"]] = card
                self.list_boxes[entry["kind"]].pack_start(card, False, False, 0)
                card.show_all()
            card.update(entry)

    def create_list(self, _button, kind):
        if not self.selected:
            return
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL,
            text="New exception list" if kind == "exceptions" else "New blacklist",
            secondary_text="A name, and where the list looks.")
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        area = dialog.get_message_area()
        area.pack_start(entry, False, False, 0)
        checks = {}
        for name, title in LIST_PLACES:
            checks[name] = Gtk.CheckButton(label=title, active=name == "url")
            area.pack_start(checks[name], False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        name = entry.get_text().strip()
        places = [place for place, check in checks.items() if check.get_active()]
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or not name:
            return
        args = ["list", "create", self.selected, name] + ["--" + place for place in places]
        if kind == "exceptions":
            args.append("--exceptions")
        self.run(args)

    # Block actions -------------------------------------------------------
    def create_block(self, _button):
        name = self.ask_text("New block", "A short name, such as social-media.")
        if name:
            self.selected = name
            self.stack.set_visible_child_name("blocks")
            self.run(["block", "create", name])

    def rename_block(self, _button):
        name = self.ask_text("Rename block", initial=self.selected or "")
        if self.selected and name and name != self.selected:
            old, self.selected = self.selected, name
            self.run(["block", "rename", old, name])

    def duplicate_block(self, _button):
        name = self.ask_text("Duplicate block", "Name of the copy.")
        if self.selected and name:
            self.run(["block", "duplicate", self.selected, name])

    def start_block(self, _button):
        block = self.blocks.get(self.selected)
        if not block:
            return
        dialog = Gtk.Dialog(title=f"Start {block['name']}", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        start = dialog.add_button("Add locks" if block["active"] else "Start", Gtk.ResponseType.OK)
        start.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(16)
        box.pack_start(dim("Locks are optional. Once on, a lock can only be made stronger." if block["active"]
                           else "Start without locks to stop any time, or add locks below."), False, False, 0)

        grid = Gtk.Grid(column_spacing=10, row_spacing=8)
        minutes = Gtk.SpinButton.new_with_range(0, 10080, 5)
        delay = Gtk.SpinButton.new_with_range(0, 57600, 5)
        password = Gtk.Entry(visibility=False, placeholder_text="optional")
        typing = Gtk.CheckButton(label="Typing challenge to stop")
        chars = challenge_length(typing)
        restart = Gtk.CheckButton(label="Stay on until the computer restarts")
        rows = (
            ("Time lock (minutes)", minutes, "Can't be stopped at all until the time has passed."),
            ("Delay to stop (minutes)", delay, "Stopping only takes effect after this long."),
            ("Password", password, "Needed to stop. Give it to someone else to keep."),
        )
        for i, (label, widget, note) in enumerate(rows):
            grid.attach(Gtk.Label(label=label, xalign=0), 0, i * 2, 1, 1)
            grid.attach(widget, 1, i * 2, 1, 1)
            grid.attach(dim(note), 1, i * 2 + 1, 1, 1)
        grid.attach(typing, 0, 6, 2, 1)
        attach_challenge_length(grid, chars, 7)
        grid.attach(restart, 0, 9, 2, 1)
        box.pack_start(grid, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        values = (minutes.get_value_as_int(), delay.get_value_as_int(), password.get_text(),
                  typing.get_active(), restart.get_active())
        lock_chars = chars.get_value_as_int()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        lock_minutes, delay_minutes, secret, use_typing, use_restart = values

        args = ["start", block["name"]]
        if lock_minutes:
            args += ["--for", f"{lock_minutes}m"]
        if use_typing:
            args += ["--lock", "--lock-chars", str(lock_chars)]
        if use_restart:
            args.append("--restart")
        if delay_minutes:
            args += ["--delay", f"{delay_minutes}m"]
        if secret:
            args.append("--password-stdin")
        if lock_minutes and not self.confirm(
                f"Time-lock {block['name']} for {lock_minutes} minutes?",
                "It can't be stopped until the time has passed, not even with the typing "
                "challenge. Time the computer is off doesn't count.", "Lock"):
            return
        self.run(args, secret + "\n" if secret else None)

    def stop_block(self, _button):
        if self.selected:
            self.run(["stop", self.selected])

    def unlock_block(self, _button):
        block = self.blocks.get(self.selected)
        if not block:
            return
        if block["password_locked"] and not block["char_locked"]:
            password = self.ask_text(f"Unlock {block['name']}", "Enter the block's password.", password=True)
            if password:
                self.run(["unlock", block["name"]], password + "\n")
            return
        command = f"{CLI} unlock {shlex.quote(block['name'])}"
        try:
            subprocess.Popen(["x-terminal-emulator", "-e", command])
        except OSError as error:
            self.message(f"Can't open a terminal for the typing challenge: {error}\n\nRun: {command}")

    def delete_block(self, _button):
        if self.selected and self.confirm(
                f"Delete {self.selected}?", "The block and its lists are removed.",
                "Delete", destructive=True):
            name, self.selected = self.selected, None
            self.run(["block", "remove", name])

    def set_quota(self, _button):
        if self.selected:
            self.run(["quota", "set", self.selected, f"{self.quota_minutes.get_value_as_int()}m"])

    def start_quota(self, _button):
        if self.selected:
            self.run(["quota", "start", self.selected])

    def end_quota(self, _button):
        if self.selected:
            self.run(["quota", "end", self.selected])

    def remove_quota(self, _button):
        if self.selected:
            self.run(["quota", "remove", self.selected])

    def set_pomodoro(self, _button):
        if self.selected:
            self.run(["pomodoro", "set", self.selected,
                      f"{self.work_minutes.get_value_as_int()}m",
                      f"{self.break_minutes.get_value_as_int()}m"])

    def clear_pomodoro(self, _button):
        if self.selected:
            self.run(["pomodoro", "off", self.selected])

    def add_schedule(self, values):
        if len(values) != 3:
            self.message("Enter days, start and end, for example: weekdays 09:00 17:00")
            return
        self.run(["schedule", "add", self.selected, "--days", values[0],
                  "--from", values[1], "--to", values[2]])

    def create_window_from_calendar(self, column, start, end):
        if not self.selected:
            return
        day, frm, to = DAY_NAMES[column], f"{start // 60:02d}:{start % 60:02d}", f"{end // 60:02d}:{end % 60:02d}"
        if self.confirm(f"Add {day.capitalize()} {frm}-{to} to {self.selected}?",
                        "The block turns on by itself then. To repeat it on other days, type "
                        "them in the list below, e.g. weekdays " + frm + " " + to + ".", "Add"):
            self.run(["schedule", "add", self.selected, "--days", day, "--from", frm, "--to", to])

    def remove_window_from_calendar(self, index):
        block = self.blocks.get(self.selected)
        if not block or index >= len(block["schedules"]):
            return
        if self.confirm(f"Remove {block['schedules'][index]}?",
                        "This can't be done from 2 hours before the window opens until it closes.",
                        "Remove", destructive=True):
            self.run(["schedule", "remove", self.selected, str(index + 1)])

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
        """Runs command (an import with its block and list) with a file."""
        if not self.selected:
            return
        path = self.choose_file("Import list", Gtk.FileChooserAction.OPEN, "Import")
        if path:
            self.run(command + [path], then=self.queued_note)

    def export_list(self, command):
        """Saves what command (an export) prints to a file."""
        if not self.selected:
            return
        path = self.choose_file("Export list", Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = run_cli(command)
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n" if output else "")
        except OSError as error:
            self.message(str(error))

    # Device Blocks -------------------------------------------------------
    def build_device(self):
        scroll, box = page_box()
        box.pack_start(heading("Device Blocks", "x-large"), False, False, 0)
        box.pack_start(dim("Locks for the whole computer, not just sites and apps."), False, False, 0)

        self.freeze_status = Gtk.Label(xalign=0)
        row = Gtk.Box(spacing=8)
        self.freeze_mode = Gtk.ComboBoxText()
        self.freeze_mode.append("lock", "Lock the screen")
        self.freeze_mode.append("logout", "Log out")
        self.freeze_mode.append("shutdown", "Shut down")
        self.freeze_mode.set_active_id("lock")
        self.freeze_minutes = Gtk.SpinButton.new_with_range(1, 1440, 5)
        self.freeze_minutes.set_value(30)
        self.freeze_button = Gtk.Button(label="Start Frozen Chicken…")
        self.freeze_button.get_style_context().add_class("destructive-action")
        self.freeze_button.connect("clicked", self.freeze)
        row.pack_start(self.freeze_mode, False, False, 0)
        row.pack_start(Gtk.Label(label="for"), False, False, 0)
        row.pack_start(self.freeze_minutes, False, False, 0)
        row.pack_start(Gtk.Label(label="minutes (at most 1440)"), False, False, 0)
        row.pack_end(self.freeze_button, False, False, 0)
        self.freeze_schedules = EditableList(
            "Scheduled", "daily 23:00 24:00", self.add_freeze_schedule, self.remove_freeze_schedule,
            help_text="Freeze at set times in the mode chosen above, for example every night. "
                      "Days, start and end; for overnight add two (23:00 24:00 and 00:00 06:00). "
                      "A schedule can't be removed from 2 hours before it starts until it ends.")
        box.pack_start(section(
            "Frozen Chicken",
            dim("Keeps the computer unusable until the time is up, even across restarts: the "
                "screen is locked again right away, sessions are logged out, or the computer "
                "shuts down. It can't be cancelled from the desktop, only extended. The time "
                "keeps counting while the computer is off."),
            row, self.freeze_status, self.freeze_schedules), False, False, 0)

        self.strict_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.strict_switch.connect("state-set", self.toggle_strict)
        box.pack_start(section(
            "Strict mode",
            self.switch_row(self.strict_switch, "Take away sudo and pkexec during locks",
                            "While a block is locked or Frozen Chicken runs, no user can get "
                            "administrator rights, so the blocks can't be undone from a terminal. "
                            "Rights return when the lock ends.")), False, False, 0)

        self.no_escape_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.no_escape_switch.connect("state-set", self.toggle_no_escape)
        box.pack_start(section(
            "No-escape",
            self.switch_row(self.no_escape_switch, "Close the ways out through the boot menu",
                            "Sets a random boot menu password that nobody knows and hides the "
                            "recovery entries, so the emergency exit (kc9k.nofreeze) stops "
                            "working. Normal boot entries, Windows included, still start. The "
                            "only way past a lock is then booting a USB stick or reinstalling. "
                            "It can't be turned off during a lock.")), False, False, 0)
        return scroll

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
        if self.confirm(f"Freeze {values[0]} {values[1]}-{values[2]}?",
                        f"The computer will be frozen then ({self.freeze_mode.get_active_text().lower()}), "
                        "and the schedule can't be removed from 2 hours before it starts.", "Schedule"):
            self.run(["freeze", "schedule", "add", "--days", values[0], "--from", values[1],
                      "--to", values[2], "--mode", mode, "--yes"])

    def remove_freeze_schedule(self, _value, number):
        self.run(["freeze", "schedule", "remove", str(number)])

    def update_device(self):
        self.freeze_schedules.set_items(self.state.get("freeze_schedules", []))
        frozen = self.state["freeze_seconds"]
        modes = {"lock": "locking the screen", "logout": "logging out", "shutdown": "shutting down"}
        self.freeze_status.set_markup(
            f"<b>On</b> for {fmt_seconds(frozen)} more, {modes.get(self.state['freeze_mode'], '')}. "
            "Starting again can only extend it." if frozen else "Off.")
        self.freeze_button.set_label("Extend…" if frozen else "Start Frozen Chicken…")
        set_switch(self.strict_switch, self.state["strict"])
        set_switch(self.no_escape_switch, self.state["no_escape"])

    def freeze(self, _button):
        minutes = self.freeze_minutes.get_value_as_int()
        mode = self.freeze_mode.get_active_id()
        effect = {
            "lock": "The screen locks now and keeps locking again until the time is up.",
            "logout": "Every session is logged out now, and logging in again ends at once.",
            "shutdown": "The computer shuts down now, and again whenever it starts.",
        }[mode]
        if self.confirm(f"Freeze the computer for {minutes} minutes?",
                        f"{effect} It can't be cancelled. Save your work first.",
                        "Freeze", destructive=True):
            self.run(["freeze", "--for", f"{minutes}m", "--mode", mode, "--yes"])

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

    # Statistics ----------------------------------------------------------
    def build_stats(self):
        scroll, box = page_box()
        box.pack_start(heading("Statistics", "x-large"), False, False, 0)

        controls = Gtk.Box(spacing=12)
        self.stats_period = Gtk.ComboBoxText()
        for days, label in ((1, "Today"), (7, "Last 7 days"), (30, "Last 30 days"), (90, "Last 90 days")):
            self.stats_period.append(str(days), label)
        self.stats_period.set_active_id("7")
        self.stats_period.connect("changed", lambda _c: self.refresh_stats())
        kinds = Gtk.Box()
        kinds.get_style_context().add_class("linked")
        self.kind_sites = Gtk.RadioButton.new_with_label(None, "Websites")
        self.kind_apps = Gtk.RadioButton.new_with_label_from_widget(self.kind_sites, "Apps")
        for button in (self.kind_sites, self.kind_apps):
            button.set_mode(False)
            button.connect("toggled", lambda b: b.get_active() and self.update_stats())
            kinds.pack_start(button, False, False, 0)
        controls.pack_start(self.stats_period, False, False, 0)
        controls.pack_start(kinds, False, False, 0)
        box.pack_start(controls, False, False, 0)

        self.stats_off = Gtk.InfoBar(message_type=Gtk.MessageType.INFO)
        self.stats_off.get_content_area().add(
            Gtk.Label(label="Statistics are off, so nothing new is recorded. Turn them on in Settings.", xalign=0))
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
        self.daily_section = section("Per day", self.daily_chart)
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
            "tab in LibreWolf. Time behind a locked screen or after two "
            "minutes without input isn't counted. Everything stays on this computer."), False, False, 0)
        return scroll

    def refresh_stats(self):
        days = int(self.stats_period.get_active_id() or 7) if hasattr(self, "stats_period") else 7
        # The dashboard needs today; ask for at least that.
        def work():
            ok, output = run_cli(["stats", "--json", "--days", str(max(days, 1))])
            GLib.idle_add(self.apply_stats, ok, output)

        threading.Thread(target=work, daemon=True).start()
        return True

    def apply_stats(self, ok, output):
        if not ok:
            return False
        data = json.loads(output)
        self.stats_rows = data["rows"]
        self.stats_on = data["enabled"]
        self.update_stats()
        self.update_dashboard_stats()
        return False

    def update_stats(self):
        kind = "site" if self.kind_sites.get_active() else "app"
        days = int(self.stats_period.get_active_id() or 7)
        today = datetime.date.today()
        dates = [today - datetime.timedelta(days=n) for n in range(days - 1, -1, -1)]
        per_day = {d.isoformat(): 0 for d in dates}
        totals = {}
        for row in self.stats_rows:
            if row["kind"] != kind or row["day"] not in per_day:
                continue
            per_day[row["day"]] += row["seconds"]
            totals[row["name"]] = totals.get(row["name"], 0) + row["seconds"]
        total = sum(per_day.values())

        self.stats_off.set_visible(not self.stats_on)
        if not self.stats_on:
            self.stats_off.show_all()
        self.tile_total.set_value(fmt_spent(total))
        self.tile_average.set_value(fmt_spent(total // days))
        self.tile_average.set_visible(days > 1)
        self.tile_count.get_children()[0].set_text("Different websites" if kind == "site" else "Different apps")
        self.tile_count.set_value(str(len(totals)))
        self.daily_section.set_visible(days > 1)
        self.daily_chart.set_days([(d, per_day[d.isoformat()]) for d in dates])

        top = sorted(totals.items(), key=lambda item: (-item[1], item[0]))[:15]
        self.top_title.set_markup(
            f"<span size='medium' weight='bold'>Most used {'websites' if kind == 'site' else 'apps'}</span>")
        self.top_chart.set_items(top)
        self.top_chart.set_visible(bool(top))
        self.top_empty.set_visible(not top)

    # Settings ------------------------------------------------------------
    def choose_theme(self, combo):
        app = self.get_application()
        app.prefs["theme"] = combo.get_active_id()
        app.theme.apply(app.prefs["theme"])
        save_prefs(app.prefs)

    def build_settings(self):
        scroll, box = page_box()
        box.pack_start(heading("Settings", "x-large"), False, False, 0)

        theme = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        for key, label in THEMES:
            theme.append(key, label)
        theme.set_active_id(self.get_application().prefs.get("theme", "system"))
        theme.connect("changed", self.choose_theme)
        row = Gtk.Box(spacing=16)
        row.pack_start(Gtk.Label(label="Theme", xalign=0), True, True, 0)
        row.pack_end(theme, False, False, 0)
        box.pack_start(section("Appearance", row), False, False, 0)

        self.browser_switches = {}
        self.librewolf_status = Gtk.Label(xalign=0, wrap=True)
        self.librewolf_install = Gtk.Button(label="Install LibreWolf", halign=Gtk.Align.START)
        self.librewolf_install.get_style_context().add_class("suggested-action")
        self.librewolf_install.connect("clicked", self.install_librewolf)
        self.extension_status = dim("")
        librewolf_rows = [self.librewolf_status, self.librewolf_install, self.extension_status]
        self.setting_locks = {}
        for name, title, detail in LIBREWOLF_OPTIONS:
            librewolf_rows.append(self.setting_row(name, title, detail))
        box.pack_start(section(
            "LibreWolf",
            dim("LibreWolf is a privacy-focused Firefox that, unlike Firefox, runs the Killer "
                "Chicken extension. The extension is always on: it enforces wildcards, page titles "
                "and text, greyscale, images and web-only, and while a block has websites, browsers "
                "without it are closed."),
            *librewolf_rows), False, False, 0)

        rows = [self.setting_row(name, title, detail) for name, title, detail in BROWSER_OPTIONS]
        box.pack_start(section(
            "Browser options", dim("Each option can be locked on its own with Lock…, independently "
                                   "of your blocks. A locked option can't be turned off until its "
                                   "lock ends or is unlocked. \"Allow only LibreWolf\" and blocking "
                                   "developer tools also can't be turned off while a block is "
                                   "locked."), *rows),
            False, False, 0)

        self.stats_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.stats_switch.connect("state-set", self.toggle_stats)
        clear = Gtk.Button(label="Delete all statistics…", halign=Gtk.Align.START)
        clear.get_style_context().add_class("destructive-action")
        clear.connect("clicked", self.clear_stats)
        box.pack_start(section(
            "Statistics",
            self.switch_row(self.stats_switch, "Record time spent on websites and apps",
                            "Stored only on this computer and kept for a year."),
            clear), False, False, 0)

        self.password_status = Gtk.Label(xalign=0, wrap=True)
        password_row = Gtk.Box(spacing=6)
        self.password_set_button = Gtk.Button(label="Set password…")
        self.password_set_button.connect("clicked", self.set_settings_password)
        self.password_remove_button = Gtk.Button(label="Remove password…")
        self.password_remove_button.connect("clicked", self.remove_settings_password)
        password_row.pack_start(self.password_set_button, False, False, 0)
        password_row.pack_start(self.password_remove_button, False, False, 0)
        box.pack_start(section(
            "Settings password",
            dim("Guards Killer Chicken 9000 against anything running as you, and lets someone "
                "else decide when your blocks may loosen: loosening, stopping, unlocking, "
                "creating locks, turning browser options off, strict mode, no-escape and "
                "uninstalling all ask for it. Starting a block without a lock and adding sites "
                "don't."),
            self.password_status, password_row), False, False, 0)

        self.nepsis_label = Gtk.Label(xalign=0, wrap=True)
        nepsis_row = Gtk.Box(spacing=6)
        change = Gtk.Button(label="Change…")
        change.connect("clicked", self.change_global_nepsis)
        off = Gtk.Button(label="Turn off…")
        off.connect("clicked", self.nepsis_off)
        nepsis_row.pack_start(change, False, False, 0)
        nepsis_row.pack_start(off, False, False, 0)
        box.pack_start(section(
            "Bypass prevention",
            dim("When Killer Chicken stops many things in a short time (blocked pages, closed apps "
                "and windows, browsers without the extension, attempts to loosen or stop a held "
                "block), you are likely hunting for a loophole. It then takes the consequence you "
                "choose, so you really step away. Each block can have its own too."),
            self.nepsis_label, nepsis_row), False, False, 0)

        self.hotkey_ids = []
        self.hotkeys = EditableList(
            "Hotkeys", "ctrl+alt+s  study  1h", self.add_hotkey,
            lambda _value, number: self.run(["hotkey", "remove", str(self.hotkey_ids[number - 1])]),
            help_text="Keys, a block, and optionally how long to lock it: pressing ctrl+alt+s "
                      "then starts 'study' locked for an hour. Use ctrl, alt, shift or super with "
                      "a key; F1 to F24 also work alone. A lock needs the settings password when "
                      "you add the hotkey, not when you press it.")
        box.pack_start(self.hotkeys, False, False, 0)
        return scroll

    def add_hotkey(self, values):
        if len(values) not in (2, 3):
            self.message("Enter keys, a block and optionally a lock: ctrl+alt+s study 1h",
                         Gtk.MessageType.INFO)
            return
        args = ["hotkey", "add", values[0], values[1]]
        if len(values) == 3:
            args += ["--for", values[2]]
        self.run(args)

    def set_settings_password(self, _button):
        new = self.ask_text("New settings password", "At least 4 characters.", password=True)
        if not new:
            return
        if self.ask_text("Repeat the new password", password=True) != new:
            self.message("The two passwords differ; nothing changed.", Gtk.MessageType.INFO)
            return
        self.run(["password", "set"], new + "\n",
                 then=lambda _o: setattr(self, "settings_password", new))

    def remove_settings_password(self, _button):
        if self.confirm("Remove the settings password?",
                        "Loosening and locks won't ask for it any more.", "Remove"):
            self.run(["password", "remove"], then=lambda _o: setattr(self, "settings_password", None))

    def update_settings(self):
        lock = self.state.get("nepsis_lock", {})
        text = describe_nepsis(self.state.get("nepsis"), self.state.get("nepsis_count"))
        if lock.get("lock_seconds"):
            text += f" Locked for {fmt_seconds(lock['lock_seconds'])}."
        self.nepsis_label.set_text(text)
        for name, switch in self.browser_switches.items():
            set_switch(switch, self.state["browser"].get(name, False))
        for name, (label, unlock) in self.setting_locks.items():
            info = self.state.get("browser_locks", {}).get(name, {})
            parts = []
            if info.get("lock_seconds"):
                parts.append(f"for {fmt_seconds(info['lock_seconds'])}")
            if info.get("restart_locked"):
                parts.append("until restart")
            if info.get("char_locked"):
                parts.append("typing challenge")
            if info.get("password_locked"):
                parts.append("password")
            label.set_text("Locked: " + ", ".join(parts) if parts else "")
            label.set_visible(bool(parts))
            unlock.set_visible(info.get("char_locked", False) or info.get("password_locked", False))
            unlock.set_sensitive(not info.get("lock_seconds") and not info.get("restart_locked"))
            # A locked option is on and stays on, so its switch can't be used.
            self.browser_switches[name].set_sensitive(not parts and name != "force-librewolf")
        version = self.state.get("librewolf", "")
        if version:
            self.librewolf_status.set_markup(f"<b>Installed</b> (version {GLib.markup_escape_text(version)}).")
        else:
            self.librewolf_status.set_markup(
                "<b>Not installed.</b> Until it is, the extension can't run, so wildcards, "
                "keywords, greyscale, images and web-only aren't enforced in the browser.")
        self.librewolf_install.set_visible(not version)
        # Without LibreWolf the line above already says the extension can't run.
        self.extension_status.set_text("Extension: " + self.state.get("extension_status", ""))
        self.extension_status.set_visible(bool(version))
        # Turning it on needs LibreWolf; turning it off must stay possible.
        switch = self.browser_switches["force-librewolf"]
        locked = bool(self.setting_locks["force-librewolf"][0].get_text())
        switch.set_sensitive(not locked and (bool(version) or switch.get_active()))
        set_switch(self.stats_switch, self.state["stats"])
        has_password = self.state.get("settings_password", False)
        self.password_status.set_markup("<b>On.</b>" if has_password else "Not set.")
        self.password_set_button.set_label("Change password…" if has_password else "Set password…")
        self.password_remove_button.set_visible(has_password)
        hotkeys = self.state.get("hotkeys") or []
        self.hotkey_ids = [h["id"] for h in hotkeys]
        self.hotkeys.set_items([
            f"{h['keys']}  starts {h['block']}" + (f", locked for {h['lock_for']}" if h["lock_for"] else "")
            for h in hotkeys])

    def install_librewolf(self, _button):
        if self.confirm("Install LibreWolf?",
                        "It is installed from LibreWolf's official repository through extrepo. "
                        "This needs an internet connection and can take a few minutes.",
                        "Install"):
            self.librewolf_status.set_text("Installing LibreWolf, this can take a few minutes…")
            self.run(["browser", "install-librewolf"], timeout=900,
                     then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def setting_row(self, name, title, detail):
        """A browser option: its switch, plus its lock state and buttons."""
        switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        switch.connect("state-set", self.toggle_browser, name)
        self.browser_switches[name] = switch
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        box.pack_start(self.switch_row(switch, title, detail), False, False, 0)
        line = Gtk.Box(spacing=8)
        status = dim("", wrap=False)
        status.set_no_show_all(True)
        lock = Gtk.Button(label="Lock…")
        lock.connect("clicked", self.lock_setting, name, title)
        unlock = Gtk.Button(label="Unlock…")
        unlock.set_no_show_all(True)
        unlock.connect("clicked", self.unlock_setting, name, title)
        line.pack_start(status, False, False, 0)
        line.pack_end(unlock, False, False, 0)
        line.pack_end(lock, False, False, 0)
        box.pack_start(line, False, False, 0)
        self.setting_locks[name] = (status, unlock)
        return box

    def lock_setting(self, _button, name, title):
        dialog = Gtk.Dialog(title=f"Lock: {title}", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Turn on and lock", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(16)
        box.pack_start(dim("The option is turned on and can't be turned off until the lock ends. "
                           "Locks can be added later, but only made stronger."), False, False, 0)
        grid = Gtk.Grid(column_spacing=10, row_spacing=8)
        minutes = Gtk.SpinButton.new_with_range(0, 57600, 5)
        password = Gtk.Entry(visibility=False, placeholder_text="optional")
        typing = Gtk.CheckButton(label="Typing challenge to turn it off")
        chars = challenge_length(typing)
        restart = Gtk.CheckButton(label="Stay on until the computer restarts")
        rows = (
            ("Time lock (minutes)", minutes, "Can't be turned off at all until the time has passed."),
            ("Password", password, "Needed to turn it off. Give it to someone else to keep."),
        )
        for i, (label, widget, note) in enumerate(rows):
            grid.attach(Gtk.Label(label=label, xalign=0), 0, i * 2, 1, 1)
            grid.attach(widget, 1, i * 2, 1, 1)
            grid.attach(dim(note), 1, i * 2 + 1, 1, 1)
        grid.attach(typing, 0, 4, 2, 1)
        attach_challenge_length(grid, chars, 5)
        grid.attach(restart, 0, 7, 2, 1)
        box.pack_start(grid, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        lock_minutes, secret = minutes.get_value_as_int(), password.get_text()
        use_typing, use_restart = typing.get_active(), restart.get_active()
        lock_chars = chars.get_value_as_int()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        args = ["browser", name, "on"]
        if lock_minutes:
            args += ["--for", f"{lock_minutes}m"]
        if use_typing:
            args += ["--lock", "--lock-chars", str(lock_chars)]
        if use_restart:
            args.append("--restart")
        if secret:
            args.append("--password-stdin")
        if len(args) == 3:
            self.message("Choose at least one lock.", Gtk.MessageType.INFO)
            return
        if lock_minutes and not self.confirm(
                f"Lock {title} for {lock_minutes} minutes?",
                "It can't be turned off until the time has passed, not even with the typing "
                "challenge. Time the computer is off doesn't count.", "Lock"):
            return
        self.run(args, secret + "\n" if secret else None)

    def unlock_setting(self, _button, name, title):
        info = self.state.get("browser_locks", {}).get(name, {})
        if info.get("password_locked") and not info.get("char_locked"):
            password = self.ask_text(f"Unlock {title}", "Enter the option's password.", password=True)
            if password:
                self.run(["browser", "unlock", name], password + "\n")
            return
        command = f"{CLI} browser unlock {shlex.quote(name)}"
        try:
            subprocess.Popen(["x-terminal-emulator", "-e", command])
        except OSError as error:
            self.message(f"Can't open a terminal for the typing challenge: {error}\n\nRun: {command}")

    def toggle_browser(self, _switch, wanted, name):
        if self.updating:
            return False
        self.run(["browser", name, "on" if wanted else "off"])
        return True

    def toggle_stats(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["stats", "on" if wanted else "off"], then=lambda _o: self.refresh_stats())
        return True

    def clear_stats(self, _button):
        if self.confirm("Delete all statistics?", "Every recorded day is removed. This can't be undone.",
                        "Delete", destructive=True):
            self.run(["stats", "clear"], then=lambda _o: self.refresh_stats())


class App(Gtk.Application):
    def __init__(self):
        super().__init__(application_id="io.github.kc9k.Gui")
        self.prefs = {}
        self.theme = None

    def do_startup(self):
        Gtk.Application.do_startup(self)
        self.prefs = load_prefs()
        self.theme = Theme()
        self.theme.apply(self.prefs.get("theme", "system"))

    def do_activate(self):
        window = self.get_active_window() or Window(self)
        window.show_all()
        window.present()


if __name__ == "__main__":
    App().run()
