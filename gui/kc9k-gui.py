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
    ("extension", "Install the Killer Chicken extension",
     "Adds the bundled extension to LibreWolf (and other Firefox-family browsers that accept "
     "it) so it can't be removed or turned off. It enforces wildcards and keywords, and "
     "measures website time for Statistics."),
    ("require-extension", "Close browsers without the extension",
     "While a block is on, any browser that has run for 60 seconds without a working Killer "
     "Chicken extension is closed. Today that is every browser except LibreWolf: Firefox only "
     "accepts signed extensions, and Chrome-family browsers have none."),
    ("force-librewolf", "Allow only LibreWolf",
     "Closes every other browser at once, even when no block is on."),
)

BROWSER_OPTIONS = (
    ("force-search", "Force Google as the search engine", ""),
    ("safesearch", "Force Google SafeSearch", ""),
    ("web-only", "Web results only",
     "No image, shopping or AI results in Google."),
    ("images", "Block images", ""),
    ("greyscale", "Show pages in greyscale", ""),
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
    if block["mode"] == "allowlist":
        parts.append("allowlist")
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
        """Run a command in the background, then refresh."""
        self.set_sensitive(False)

        def work():
            ok, output = run_cli(args, secret, timeout)
            GLib.idle_add(self.finished, ok, output, then)

        threading.Thread(target=work, daemon=True).start()

    def finished(self, ok, output, then):
        self.set_sensitive(True)
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
        actions.pack_start(new_block, False, False, 0)
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
        paned.pack2(self.detail, True, False)
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
        self.sites = EditableList(
            "Blocked", "reddit.com  reddit.com/r/funny  *.xxx  *casino*",
            lambda values: self.block_cmd("add", values),
            lambda value, _num: self.block_cmd("remove-entry", [value]),
            lambda: self.import_list("blocked"), lambda: self.export_list("blocked"),
            "Domains, paths, wildcards (*.xxx, *.* for everything) and keywords (*casino*).")
        self.exceptions = EditableList(
            "Exceptions", "reddit.com/r/programming",
            lambda values: self.block_cmd("add-exception", values),
            lambda value, _num: self.block_cmd("remove-exception", [value]),
            lambda: self.import_list("exceptions"), lambda: self.export_list("exceptions"),
            "Allowed even when they match a blocked entry.")
        self.limited_label = dim("")
        for widget in (self.sites, self.exceptions):
            websites.pack_start(widget, True, True, 0)
        websites.pack_start(self.limited_label, False, False, 0)
        tabs.append_page(websites, Gtk.Label(label="Websites"))

        apps = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, margin=12)
        self.apps = EditableList(
            "Apps and folders", "discord  com.discordapp.Discord  /opt/games/",
            lambda values: self.block_cmd("add-app", values),
            lambda value, _num: self.block_cmd("remove-app", [value]),
            help_text="Executable names, Flatpak app IDs (see 'flatpak list --app'), or folders "
                      "whose programs can't run. Everything a blocked app started is closed too.")
        apps.pack_start(self.apps, True, True, 0)
        tabs.append_page(apps, Gtk.Label(label="Apps"))

        schedule = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, margin=12)
        self.schedules = EditableList(
            "Schedule", "weekdays 09:00 17:00", self.add_schedule, self.remove_schedule,
            help_text="Days, start and end: weekdays 09:00 17:00, or mon,wed,fri 18:00 22:00. "
                      "The block turns on and off by itself, and can't be weakened from "
                      "2 hours before a window opens.")
        schedule.pack_start(self.schedules, True, True, 0)
        tabs.append_page(schedule, Gtk.Label(label="Schedule"))

        tabs.append_page(self.build_quota_tab(), Gtk.Label(label="Quota & Pomodoro"))
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
        for button in (self.quota_set, self.quota_start, self.quota_end, self.quota_remove):
            quota_row.pack_start(button, False, False, 0)
        self.quota_label = dim("")
        box.pack_start(section(
            "Daily quota",
            dim("The block stays on, except during sessions you open, until the day's minutes are spent."),
            quota_row, self.quota_label), False, False, 0)

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
            for widget in (self.sites, self.exceptions, self.apps, self.schedules):
                widget.set_items([])
            self.limited_label.set_text("")
            self.quota_label.set_text("")
            return
        self.title_label.set_markup(f"<big><b>{GLib.markup_escape_text(block['name'])}</b></big>")
        self.status_label.set_text(block_summary(block))
        self.sites.set_items(block["sites"])
        self.exceptions.set_items(block["exceptions"])
        limited = block["firefox_only"]
        self.limited_label.set_text(
            "Only enforced in browsers with the extension: " + ", ".join(limited) if limited else "")
        self.apps.set_items(block["apps"])
        self.schedules.set_items(block["schedules"])

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
        grid.attach(restart, 0, 7, 2, 1)
        box.pack_start(grid, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        values = (minutes.get_value_as_int(), delay.get_value_as_int(), password.get_text(),
                  typing.get_active(), restart.get_active())
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        lock_minutes, delay_minutes, secret, use_typing, use_restart = values

        args = ["start", block["name"]]
        if lock_minutes:
            args += ["--for", f"{lock_minutes}m"]
        if use_typing:
            args.append("--lock")
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

    def import_list(self, kind):
        if not self.selected:
            return
        path = self.choose_file("Import list", Gtk.FileChooserAction.OPEN, "Import")
        if path:
            self.run(["block", "import-list", self.selected, kind, path])

    def export_list(self, kind):
        if not self.selected:
            return
        path = self.choose_file("Export list", Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = run_cli(["block", "export-list", self.selected, kind])
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
        box.pack_start(section(
            "Frozen Chicken",
            dim("Keeps the computer unusable until the time is up, even across restarts: the "
                "screen is locked again right away, sessions are logged out, or the computer "
                "shuts down. It can't be cancelled from the desktop, only extended. The time "
                "keeps counting while the computer is off."),
            row, self.freeze_status), False, False, 0)

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

    def update_device(self):
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
            "tab in browsers with the extension. Time behind a locked screen or after two "
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
    def build_settings(self):
        scroll, box = page_box()
        box.pack_start(heading("Settings", "x-large"), False, False, 0)

        self.browser_switches = {}
        self.librewolf_status = Gtk.Label(xalign=0)
        self.librewolf_install = Gtk.Button(label="Install LibreWolf", halign=Gtk.Align.START)
        self.librewolf_install.get_style_context().add_class("suggested-action")
        self.librewolf_install.connect("clicked", self.install_librewolf)
        librewolf_rows = [self.librewolf_status, self.librewolf_install]
        for name, title, detail in LIBREWOLF_OPTIONS:
            switch = Gtk.Switch(valign=Gtk.Align.CENTER)
            switch.connect("state-set", self.toggle_browser, name)
            self.browser_switches[name] = switch
            librewolf_rows.append(self.switch_row(switch, title, detail))
        box.pack_start(section(
            "LibreWolf",
            dim("LibreWolf is a privacy-focused Firefox that, unlike Firefox, accepts the "
                "bundled extension."),
            *librewolf_rows), False, False, 0)

        rows = []
        for name, title, detail in BROWSER_OPTIONS:
            switch = Gtk.Switch(valign=Gtk.Align.CENTER)
            switch.connect("state-set", self.toggle_browser, name)
            self.browser_switches[name] = switch
            rows.append(self.switch_row(switch, title, detail))
        box.pack_start(section(
            "Browser options", dim("These, like the LibreWolf switches, can always be turned on, "
                                   "but not off during a lock."), *rows),
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
        return scroll

    def update_settings(self):
        for name, switch in self.browser_switches.items():
            set_switch(switch, self.state["browser"].get(name, False))
        version = self.state.get("librewolf", "")
        if version:
            self.librewolf_status.set_markup(f"<b>Installed</b> (version {GLib.markup_escape_text(version)}).")
        else:
            self.librewolf_status.set_markup(
                "<b>Not installed.</b> Install it before allowing only LibreWolf, or every "
                "browser would be closed.")
        self.librewolf_install.set_visible(not version)
        # Turning it on needs LibreWolf; turning it off must stay possible.
        for name in ("force-librewolf", "require-extension"):
            switch = self.browser_switches[name]
            switch.set_sensitive(bool(version) or switch.get_active())
        set_switch(self.stats_switch, self.state["stats"])

    def install_librewolf(self, _button):
        if self.confirm("Install LibreWolf?",
                        "It is installed from LibreWolf's official repository through extrepo. "
                        "This needs an internet connection and can take a few minutes.",
                        "Install"):
            self.librewolf_status.set_text("Installing LibreWolf, this can take a few minutes…")
            self.run(["browser", "install-librewolf"], timeout=900,
                     then=lambda output: self.message(output, Gtk.MessageType.INFO))

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

    def do_activate(self):
        window = self.get_active_window() or Window(self)
        window.show_all()
        window.present()


if __name__ == "__main__":
    App().run()
