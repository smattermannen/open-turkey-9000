# Statistics widgets: tiles, week calendar, daily and top charts, legend.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class StatTile(Gtk.Box):
    """A label above one large number."""

    def __init__(self, title):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        label = Gtk.Label(label=title, xalign=0)
        label.get_style_context().add_class("kc9k-stat-label")
        self.pack_start(label, False, False, 0)
        self.value = Gtk.Label(xalign=0)
        self.value.get_style_context().add_class("kc9k-stat-value")
        self.pack_start(self.value, False, False, 0)
        self.set_value("–")

    def set_value(self, text):
        self.value.set_text(text)


DAY_NAMES = ("mon", "tue", "wed", "thu", "fri", "sat", "sun")
# §4.10 preference: the week starts on Monday (0) or Sunday (6).
FIRST_DAY = 0


def column_day(column):
    """The day (0 = Monday) shown in a schedule grid column."""
    return (column + FIRST_DAY) % 7


class WeekCalendar(Gtk.DrawingArea):
    """A block's schedule as a week: the days across (from the first day of
    the week), 00:00 to 24:00 down. Drag down a day to add a window (snapped
    to 15 minutes); drag a window to move it; click a window to edit it
    (§4.5)."""

    SNAP = 15

    def __init__(self, on_create, on_pick, on_move=None):
        super().__init__()
        self.on_create, self.on_pick, self.on_move = on_create, on_pick, on_move
        self.moving = None  # (window index, column, minute) while a window is dragged
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

    def set_windows(self, windows, others=()):
        """windows: the block's; others: other blocks' block windows, drawn
        grey behind them (§4.5)."""
        self.windows = windows
        self.others = list(others)
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
        """The column's day as a schedule day bit (the bits start at Sunday)."""
        return 1 << ((column_day(column) + 1) % 7)

    def window_at(self, column, minute):
        for index, w in enumerate(self.windows):
            if w["days"] & self.weekday_bit(column) and w["start"] <= minute < w["end"]:
                return index
        return None

    def on_press(self, _widget, event):
        if event.button != 1:
            return False
        column, minute = self.at(event.x, event.y)
        exact = (event.y - self.geometry()[1]) / self.geometry()[3] * 1440
        index = self.window_at(column, exact)
        self.moving = (index, column, minute) if index is not None else None
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
        to_column, end = self.at(event.x, event.y)
        self.drag = None
        self.queue_draw()
        moving, self.moving = self.moving, None
        if moving and self.on_move and (abs(event.y - self.press_y) >= 5 or to_column != column):
            # Move the window: by the time dragged, and to another day for
            # a window of one day.
            index, from_column, from_minute = moving
            w = self.windows[index]
            delta = end - from_minute
            length = w["end"] - w["start"]
            new_start = min(max(w["start"] + delta, 0), 1440 - length)
            days = w["days"]
            if bin(days).count("1") == 1 and to_column != from_column:
                days = self.weekday_bit(to_column)
            if new_start != w["start"] or days != w["days"]:
                self.on_move(index, days, new_start, new_start + length)
            return True
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
                layout.set_text(show_hour(hour), -1)
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
        for column in range(7):
            layout.set_text(DAY_NAMES[column_day(column)].capitalize(), -1)
            w, _h = layout.get_pixel_size()
            cr.set_source_rgba(*text)
            cr.move_to(left + column * column_width + column_width / 2 - w / 2, 4)
            PangoCairo.show_layout(cr, layout)

        r, g, b, _a = accent
        for w in getattr(self, "others", []):
            for column in range(7):
                if w["days"] & self.weekday_bit(column):
                    y0, y1 = y_of(w["start"]), y_of(w["end"])
                    cr.rectangle(left + column * column_width + 2, y0, column_width - 4, max(y1 - y0, 2))
                    cr.set_source_rgba(*faint[:3], 0.6)
                    cr.fill()
        for w in self.windows:
            for column in range(7):
                if not w["days"] & self.weekday_bit(column):
                    continue
                x = left + column * column_width + 2
                y0, y1 = y_of(w["start"]), y_of(w["end"])
                cr.rectangle(x, y0, column_width - 4, max(y1 - y0, 2))
                if w.get("source"):
                    # §3.6: a copy of a schedule's window, changed on the
                    # Schedules page: lighter, with the schedule's name.
                    cr.set_source_rgba(r, g, b, 0.45 if not w["allow"] else 0.12)
                    cr.fill_preserve()
                    cr.set_source_rgba(r, g, b, 0.9)
                    cr.set_line_width(1)
                    cr.set_dash([3, 2])
                    cr.stroke()
                    cr.set_dash([])
                    if y1 - y0 > 14:
                        layout.set_text(w["source"], -1)
                        layout.set_width(int((column_width - 8) * Pango.SCALE))
                        layout.set_ellipsize(Pango.EllipsizeMode.END)
                        cr.set_source_rgba(*text)
                        cr.move_to(x + 2, y0 + 1)
                        PangoCairo.show_layout(cr, layout)
                        layout.set_width(-1)
                elif w["allow"]:
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


def fmt_count(n):
    """A count of times, for charts of times blocked."""
    return f"{n}×"


# §4.9: one colour per site or app. The categorical palette's first seven
# slots, light and dark (checked with the chart guide's palette validator:
# lightness, chroma, colour-blind and normal-vision separation, contrast; in
# light mode three are under 3:1 against the surface, so the legend and the
# list name every series), in this order; the rest fold into Other.
SERIES_LIGHT = ("#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4", "#008300", "#6250d6")
SERIES_DARK = ("#3987e5", "#d95926", "#199e70", "#c98500", "#d55181", "#0ca30c", "#9085e9")
OTHER_SERIES = "#898781"


def hex_rgba(color, alpha=1.0):
    return (int(color[1:3], 16) / 255, int(color[3:5], 16) / 255, int(color[5:7], 16) / 255, alpha)


def dark_theme(widget):
    """Whether the window shows dark: its text is light."""
    _accent, text, _muted, _faint = theme_colors(widget)
    return 0.2126 * text[0] + 0.7152 * text[1] + 0.0722 * text[2] > 0.5


def series_color(slot, dark):
    """The colour of a series slot (None: Other)."""
    if slot is None:
        return hex_rgba(OTHER_SERIES)
    return hex_rgba((SERIES_DARK if dark else SERIES_LIGHT)[slot])


class DailyChart(Gtk.DrawingArea):
    """Time spent per day: vertical bars, stacked by site or app (§4.9),
    hover for the values."""

    fmt = staticmethod(fmt_spent)  # how values read; fmt_count for counts

    def __init__(self):
        super().__init__()
        self.days = []  # [(date, seconds)]
        self.stacks, self.series = None, []  # set_stacks
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
        self.stacks = None
        self.queue_draw()

    def set_stacks(self, days, stacks, series):
        """Stacked columns (§4.9): days as for set_days, stacks a {name:
        value} for each day, series [(name, slot)] bottom up (slot None:
        Other). Colour follows the name, not its rank."""
        self.days, self.stacks, self.series = days, stacks, series
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
        text = f"{day.strftime('%a %-d %b')}: {self.fmt(seconds)}"
        if getattr(self, "stacks", None):
            parts = sorted(((v, n) for n, v in self.stacks[index].items() if v), reverse=True)
            text += "".join(f"\n{n}: {self.fmt(v)}" for v, n in parts)
        tooltip.set_text(text)
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
            layout.set_text(self.fmt(value) if value else "0", -1)
            w, h = layout.get_pixel_size()
            cr.set_source_rgba(*muted)
            cr.move_to(left - 8 - w, y - h / 2)
            PangoCairo.show_layout(cr, layout)
            value += step

        # Bars: thin, 2px gaps, rounded data ends on the baseline; stacked,
        # each part in its series' colour with a 2px surface gap above it.
        bar_width = max(min(slot - 2, 24), 1)
        label_every = 1 if len(self.days) <= 10 else math.ceil(len(self.days) / 8)
        dark = dark_theme(widget)
        for i, (day, seconds) in enumerate(self.days):
            x = left + i * slot + (slot - bar_width) / 2
            alpha = 1.0 if self.hover in (None, i) else 0.45
            if seconds > 0 and getattr(self, "stacks", None):
                base = top + plot
                parts = [(color_slot, self.stacks[i].get(name, 0)) for name, color_slot in self.series]
                parts = [p for p in parts if p[1] > 0]
                for j, (color_slot, part) in enumerate(parts):
                    part_height = plot * part / top_value
                    last = j == len(parts) - 1
                    drawn = max(part_height - (0 if last else 2), 1)
                    cr.set_source_rgba(*series_color(color_slot, dark)[:3], alpha)
                    if last:
                        rounded_top_bar(cr, x, base - drawn, bar_width, drawn)
                    else:
                        cr.rectangle(x, base - drawn, bar_width, drawn)
                    cr.fill()
                    base -= part_height
            elif seconds > 0:
                bar_height = max(plot * seconds / top_value, 2)
                r, g, b, _a = accent
                cr.set_source_rgba(r, g, b, alpha)
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

    fmt = staticmethod(fmt_spent)  # how values read; fmt_count for counts

    ROW = 26

    def __init__(self):
        super().__init__()
        self.items = []  # [(name, seconds)]
        self.colors = {}  # name -> series slot (None: Other), for the daily chart's colours
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
        tooltip.set_text(f"{name}: {self.fmt(seconds)} ({round(100 * seconds / total)}% of the listed total)")
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
            r, g, b, _a = series_color(self.colors[name], dark_theme(widget)) if name in self.colors else accent
            cr.set_source_rgba(r, g, b, 1.0 if self.hover in (None, i) else 0.45)
            rounded_end_bar(cr, name_width, y + 7, bar, self.ROW - 14)
            cr.fill()

            layout.set_width(-1)
            layout.set_text(self.fmt(seconds), -1)
            _w, h = layout.get_pixel_size()
            cr.set_source_rgba(*muted)
            cr.move_to(name_width + bar + 8, y + (self.ROW - h) / 2)
            PangoCairo.show_layout(cr, layout)
        return False


class Legend(Gtk.FlowBox):
    """The daily chart's legend (§4.9): a swatch in each series' colour and
    its name in text ink; wraps on a narrow window."""

    def __init__(self):
        super().__init__(selection_mode=Gtk.SelectionMode.NONE, column_spacing=14, row_spacing=4,
                         max_children_per_line=12, homogeneous=False)
        self.entries = []

    def set_series(self, series):
        """series: [(name, slot)] (slot None: Other)."""
        if series == self.entries:
            return
        self.entries = list(series)
        for child in self.get_children():
            self.remove(child)
        for name, slot in series:
            entry = Gtk.Box(spacing=6)
            swatch = Gtk.DrawingArea()
            swatch.set_size_request(10, 10)
            swatch.set_valign(Gtk.Align.CENTER)
            swatch.connect("draw", lambda w, cr, sl=slot: (
                cr.set_source_rgba(*series_color(sl, dark_theme(w))),
                rounded_rect(cr, 0, 0, 10, 10, 2), cr.fill(), False)[-1])
            entry.pack_start(swatch, False, False, 0)
            entry.pack_start(Gtk.Label(label=name, xalign=0), False, False, 0)
            self.add(entry)
        self.set_visible(len(series) >= 2)
        self.show_all()
        self.set_visible(len(series) >= 2)
