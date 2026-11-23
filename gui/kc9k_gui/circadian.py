# Circadian rhythm (§4.10): clips, clip editor, wallpaper timeline and editor, the page.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

# Circadian rhythm (§4.10) -------------------------------------------------
# The same curve as circadian.Shape and Lane.ValueAt in internal/circadian.
DAY_MINUTES = 24 * 60


def clip_shape(x, tension, rising):
    x = max(0.0, min(1.0, x))
    if not rising:
        tension = -tension
    middle = 0.5 + 0.49 * max(-1.0, min(1.0, tension))
    if abs(middle - 0.5) < 1e-9:
        return x
    # Below the straight line x^p, above it its mirror image: +t and -t are
    # mirror images and neither end is a corner.
    if middle < 0.5:
        return x ** (math.log(middle) / math.log(0.5))
    return 1 - (1 - x) ** (math.log(1 - middle) / math.log(0.5))


def clip_segment(points, minute):
    """The points the curve runs between at minute: (from, to, start, end),
    with start and end in minutes (before 0 or past 1440 around midnight)."""
    count = len(points)
    index = -1
    for i, point in enumerate(points):
        if point["minute"] <= minute:
            index = i
    if index < 0:
        return count - 1, 0, points[-1]["minute"] - DAY_MINUTES, points[0]["minute"]
    if index + 1 < count:
        return index, index + 1, points[index]["minute"], points[index + 1]["minute"]
    return index, 0, points[index]["minute"], points[0]["minute"] + DAY_MINUTES


def clip_value(points, minute):
    if not points:
        return None
    if len(points) == 1:
        return points[0]["value"]
    a, b, start, end = clip_segment(points, minute)
    first, second = points[a], points[b]
    if end <= start:
        return first["value"]
    way = clip_shape((minute - start) / (end - start), first.get("tension", 0),
                     second["value"] >= first["value"])
    return first["value"] + (second["value"] - first["value"]) * way


def kelvin_rgb(kelvin):
    """The white of a colour temperature (Tanner Helland's fit, 6500 K is
    white), as circadian.WhitePoint draws it."""
    def black_body(k):
        t = max(1000.0, min(40000.0, k)) / 100
        if t <= 66:
            r = 255.0
            g = 99.4708025861 * math.log(t) - 161.1195681661
        else:
            r = 329.698727446 * (t - 60) ** -0.1332047592
            g = 288.1221695283 * (t - 60) ** -0.0755148492
        if t >= 66:
            b = 255.0
        elif t <= 19:
            b = 0.0
        else:
            b = 138.5177312231 * math.log(t - 10) - 305.0447927307
        return [max(1.0, min(255.0, v)) for v in (r, g, b)]
    here, neutral = black_body(kelvin), black_body(6500)
    rgb = [h / n for h, n in zip(here, neutral)]
    top = max(rgb)
    return tuple(v / top for v in rgb)


def clock_text(minute):
    """HH:MM, as commands and entry fields take it."""
    minute = int(round(minute)) % DAY_MINUTES
    return f"{minute // 60:02d}:{minute % 60:02d}"


# §4.10 preference: show times on a 24-hour clock (False: 12-hour, AM/PM).
CLOCK_24 = True


def show_clock(minute, end=False):
    """A time of day for reading, by the clock preference; end=True shows
    a window end at midnight as 24:00 (12:00 AM)."""
    minute = int(round(minute))
    if not (end and minute >= DAY_MINUTES):
        minute %= DAY_MINUTES
    if CLOCK_24:
        return f"{minute // 60:02d}:{minute % 60:02d}"
    hour, rest = divmod(minute % DAY_MINUTES, 60)
    return f"{(hour % 12) or 12}:{rest:02d} {'AM' if hour < 12 else 'PM'}"


def show_hour(hour):
    """An hour label on a time axis, by the clock preference."""
    if CLOCK_24:
        return f"{hour:02d}:00"
    hour %= 24
    return f"{(hour % 12) or 12} {'AM' if hour < 12 else 'PM'}"


def points_text(points):
    """Points as 'kc9k circadian lane --points' reads them."""
    parts = []
    for point in points:
        text = f"{clock_text(point['minute'])}={point['value']:.1f}"
        if point.get("tension"):
            text += f"~{point['tension']:.2f}"
        parts.append(text)
    return " ".join(parts)


# The sky through the day, for the clips' time axis: (hour, red, green, blue).
SKY = ((0, .04, .06, .15), (5, .10, .16, .32), (6.5, .96, .64, .38), (8, .56, .79, .90),
       (12, .75, .90, 1.0), (17, .56, .79, .90), (18.5, .96, .64, .38), (20, .10, .16, .32),
       (24, .04, .06, .15))


class ClipEditor(Gtk.DrawingArea):
    """One automation clip: a 24-hour curve of points. Right-click adds a
    point (on a point: removes it), dragging a point moves it, dragging the
    small handle in the middle of a segment bends it (owner 2026-11-20,
    D4), double-click types exact values."""

    LEFT, RIGHT, TOP, BOTTOM = 52, 12, 10, 30
    GRAB = 9

    def __init__(self, lane, on_change, on_preview):
        super().__init__()
        self.lane = lane
        self.points = [dict(p) for p in lane["points"]]
        self.minute = 0.0
        self.playhead = None  # the minute "Play the day" shows, while it plays
        self.active = False  # the clip is on, and so is the circadian rhythm
        self.on_change, self.on_preview = on_change, on_preview
        self.drag = None  # ("point", index) or ("tension", index, start y, start tension)
        self.moved = False
        self.hover = None  # a point's index
        self.hover_handle = None  # a segment's index (the point it starts at)
        self.set_size_request(-1, 190)
        self.set_has_tooltip(True)
        self.add_events(Gdk.EventMask.BUTTON_PRESS_MASK | Gdk.EventMask.BUTTON_RELEASE_MASK | POINTER_EVENTS)
        self.connect("draw", self.draw)
        self.connect("button-press-event", self.pressed)
        self.connect("button-release-event", self.released)
        self.connect("motion-notify-event", self.motion)
        self.connect("leave-notify-event", self.left)
        self.connect("query-tooltip", self.tooltip)

    # Geometry
    def plot(self):
        width, height = self.get_allocated_width(), self.get_allocated_height()
        return self.LEFT, self.TOP, max(10, width - self.LEFT - self.RIGHT), max(10, height - self.TOP - self.BOTTOM)

    def x_of(self, minute):
        x, _y, w, _h = self.plot()
        return x + w * minute / DAY_MINUTES

    def y_of(self, value):
        _x, y, _w, h = self.plot()
        low, high = self.lane["min"], self.lane["max"]
        return y + h * (1 - (value - low) / (high - low))

    def minute_of(self, px, fine=False):
        x, _y, w, _h = self.plot()
        minute = (px - x) / w * DAY_MINUTES
        step = 1 if fine else 5
        return int(max(0, min(DAY_MINUTES - 1, round(minute / step) * step)))

    def value_of(self, py):
        _x, y, _w, h = self.plot()
        low, high = self.lane["min"], self.lane["max"]
        value = low + (1 - (py - y) / h) * (high - low)
        step = 50 if self.lane["unit"] == "K" else 1
        return max(low, min(high, round(value / step) * step))

    def point_at(self, px, py):
        for index, point in enumerate(self.points):
            if abs(self.x_of(point["minute"]) - px) <= self.GRAB and abs(self.y_of(point["value"]) - py) <= self.GRAB:
                return index
        return None

    def handles(self):
        """(segment index, x, y) of each segment's bend handle: on the
        curve, halfway in time between its point and the next (the last
        segment runs past midnight to the first point)."""
        count = len(self.points)
        if count < 2:
            return []
        found = []
        for index, point in enumerate(self.points):
            start = point["minute"]
            end = self.points[(index + 1) % count]["minute"] + (DAY_MINUTES if index + 1 == count else 0)
            mid = ((start + end) / 2) % DAY_MINUTES
            found.append((index, self.x_of(mid), self.y_of(clip_value(self.points, mid))))
        return found

    def handle_at(self, px, py):
        """The segment whose bend handle is under the pointer."""
        for index, hx, hy in self.handles():
            if abs(hx - px) <= self.GRAB and abs(hy - py) <= self.GRAB:
                return index
        return None

    def set_data(self, lane, minute, active):
        """Takes the stored clip over, unless a drag is under way."""
        self.minute, self.active = minute, active
        if self.drag is None:
            self.lane = lane
            self.points = [dict(p) for p in lane["points"]]
        self.queue_draw()

    # Drawing
    def draw(self, _widget, cr):
        accent, text, soft, faint = theme_colors(self)
        x, y, w, h = self.plot()
        low, high, unit = self.lane["min"], self.lane["max"], self.lane["unit"]
        alpha = 1.0 if self.active else 0.45
        layout = self.create_pango_layout("")
        layout.set_font_description(Pango.FontDescription("Sans 8"))

        # Value grid and labels.
        cr.set_line_width(1)
        marks = (1000, 2500, 4000, 5500, 7000, 8500, 10000) if unit == "K" else (low, 25, 50, 75, 100)
        for mark in marks:
            py = round(self.y_of(mark)) + 0.5
            cr.set_source_rgba(*faint)
            cr.move_to(x, py)
            cr.line_to(x + w, py)
            cr.stroke()
            layout.set_text(f"{mark:g}{'' if unit == 'K' else ' %'}")
            width, height = layout.get_pixel_size()
            cr.set_source_rgba(*soft)
            cr.move_to(x - width - 6, py - height / 2)
            PangoCairo.show_layout(cr, layout)

        # The time axis: the sky through the day, and the hours.
        import cairo  # comes with PyGObject's drawing support
        sky = cairo.LinearGradient(x, 0, x + w, 0)
        for hour, red, green, blue in SKY:
            sky.add_color_stop_rgb(hour / 24, red, green, blue)
        cr.set_source(sky)
        cr.rectangle(x, y + h + 3, w, 6)
        cr.fill()
        for hour in range(0, 25, 3):
            px = round(x + w * hour / 24) + 0.5
            cr.set_source_rgba(*faint)
            cr.move_to(px, y)
            cr.line_to(px, y + h)
            cr.stroke()
            layout.set_text(show_hour(hour))
            width, _height = layout.get_pixel_size()
            cr.set_source_rgba(*soft)
            cr.move_to(min(max(px - width / 2, x - 10), x + w - width + 10), y + h + 12)
            PangoCairo.show_layout(cr, layout)

        if not self.points:
            layout.set_text("No points yet: right-click to add one.")
            width, height = layout.get_pixel_size()
            cr.set_source_rgba(*soft)
            cr.move_to(x + (w - width) / 2, y + (h - height) / 2)
            PangoCairo.show_layout(cr, layout)
            return False

        # The curve, sampled every other pixel, and what lies under it.
        samples = []
        for step in range(0, int(w) + 1, 2):
            minute = step / w * DAY_MINUTES
            samples.append((x + step, clip_value(self.points, min(minute, DAY_MINUTES - 0.001))))
        base = y + h
        if unit == "K":
            # Filled with the colour each temperature gives the screen.
            for px, value in samples:
                red, green, blue = kelvin_rgb(value)
                cr.set_source_rgba(red, green, blue, 0.75 * alpha)
                cr.rectangle(px, self.y_of(value), 2, base - self.y_of(value))
                cr.fill()
        else:
            cr.move_to(x, base)
            for px, value in samples:
                cr.line_to(px, self.y_of(value))
            cr.line_to(samples[-1][0], base)
            cr.close_path()
            cr.set_source_rgba(accent[0], accent[1], accent[2], 0.22 * alpha)
            cr.fill()
        cr.set_line_width(2)
        cr.set_source_rgba(accent[0], accent[1], accent[2], alpha)
        for index, (px, value) in enumerate(samples):
            (cr.move_to if index == 0 else cr.line_to)(px, self.y_of(value))
        cr.stroke()

        # Now.
        now_x = round(self.x_of(self.minute)) + 0.5
        cr.set_line_width(1)
        cr.set_source_rgba(text[0], text[1], text[2], 0.7)
        cr.move_to(now_x, y)
        cr.line_to(now_x, y + h + 9)
        cr.stroke()
        # The minute "Play the day" shows.
        if self.playhead is not None:
            play_x = round(self.x_of(self.playhead)) + 0.5
            cr.set_line_width(2)
            cr.set_source_rgba(accent[0], accent[1], accent[2], 1)
            cr.move_to(play_x, y)
            cr.line_to(play_x, y + h + 9)
            cr.stroke()

        # The bend handles, small and hollow, one mid-segment.
        for index, hx, hy in self.handles():
            grabbed = self.hover_handle == index or (self.drag and self.drag[0] == "tension" and self.drag[1] == index)
            cr.arc(hx, hy, 4.5 if grabbed else 3, 0, 2 * math.pi)
            cr.set_source_rgba(1, 1, 1, 0.85 * alpha)
            cr.fill_preserve()
            cr.set_line_width(1.5)
            cr.set_source_rgba(accent[0], accent[1], accent[2], 0.8 * alpha)
            cr.stroke()

        # The points.
        for index, point in enumerate(self.points):
            px, py = self.x_of(point["minute"]), self.y_of(point["value"])
            grabbed = self.hover == index or (self.drag and self.drag[0] == "point" and self.drag[1] == index)
            cr.arc(px, py, 6 if grabbed else 4.5, 0, 2 * math.pi)
            cr.set_source_rgba(1, 1, 1, alpha)
            cr.fill_preserve()
            cr.set_line_width(2)
            cr.set_source_rgba(accent[0], accent[1], accent[2], alpha)
            cr.stroke()
        return False

    # Pointer
    def describe(self, value):
        return f"{value:.0f} K" if self.lane["unit"] == "K" else f"{value:.0f} %"

    def tooltip(self, _widget, px, py, _keyboard, tip):
        x, _y, w, _h = self.plot()
        if px < x or px > x + w:
            return False
        index = self.point_at(px, py)
        if index is not None:
            point = self.points[index]
            tip.set_text(f"{show_clock(point['minute'])}  {self.describe(point['value'])}\n"
                         "Drag to move · right-click to remove · double-click to type")
            return True
        segment = self.handle_at(px, py)
        if segment is not None:
            tip.set_text(f"Bend {self.points[segment].get('tension', 0):+.2f}\n"
                         "Drag up or down to bend this stretch · double-click to type")
            return True
        if not self.points:
            return False
        minute = (px - x) / w * DAY_MINUTES
        tip.set_text(f"{show_clock(minute)}  {self.describe(clip_value(self.points, minute))}")
        return True

    def pressed(self, _widget, event):
        index = self.point_at(event.x, event.y)
        if event.button == 3:
            if index is not None:
                del self.points[index]
            else:
                fine = bool(event.state & Gdk.ModifierType.SHIFT_MASK)
                minute = self.minute_of(event.x, fine)
                self.points = [p for p in self.points if p["minute"] != minute]
                self.points.append({"minute": minute, "value": self.value_of(event.y), "tension": 0})
                self.points.sort(key=lambda p: p["minute"])
            self.hover = None
            self.queue_draw()
            self.on_change(self)
            return True
        if event.button != 1:
            return False
        segment = self.handle_at(event.x, event.y) if index is None else None
        if event.type == Gdk.EventType.DOUBLE_BUTTON_PRESS:
            self.drag = None
            if index is not None or segment is not None:
                self.type_point(index if index is not None else segment)
            return True
        if index is not None:
            self.drag, self.moved = ("point", index), False
        else:
            if segment is not None:
                self.drag, self.moved = ("tension", segment, event.y, self.points[segment].get("tension", 0)), False
        return True

    def motion(self, _widget, event):
        if self.drag is None:
            hover = self.point_at(event.x, event.y)
            handle = self.handle_at(event.x, event.y) if hover is None else None
            if hover != self.hover or handle != self.hover_handle:
                self.hover, self.hover_handle = hover, handle
                self.queue_draw()
            self.trigger_tooltip_query()
            return False
        self.moved = True
        if self.drag[0] == "point":
            index = self.drag[1]
            point = self.points[index]
            minute = self.minute_of(event.x, bool(event.state & Gdk.ModifierType.SHIFT_MASK))
            # A point stays between its neighbours.
            if index > 0:
                minute = max(minute, self.points[index - 1]["minute"] + 1)
            if index + 1 < len(self.points):
                minute = min(minute, self.points[index + 1]["minute"] - 1)
            point["minute"], point["value"] = minute, self.value_of(event.y)
            self.on_preview(self, point["value"])
        else:
            _kind, index, start_y, start_tension = self.drag
            # Up bulges the curve up, whichever way it runs.
            tension = start_tension + (start_y - event.y) / 80
            self.points[index]["tension"] = round(max(-1.0, min(1.0, tension)), 2)
        self.queue_draw()
        return True

    def released(self, _widget, event):
        if event.button != 1 or self.drag is None:
            return False
        was_point = self.drag[0] == "point"
        self.drag = None
        if self.moved:
            self.on_change(self, preview_over=was_point)
        self.queue_draw()
        return True

    def left(self, *_args):
        if (self.hover is not None or self.hover_handle is not None) and self.drag is None:
            self.hover = self.hover_handle = None
            self.queue_draw()

    def type_point(self, index):
        point = self.points[index]
        dialog = Gtk.Dialog(title=f"{self.lane['title']}: point", transient_for=self.get_toplevel(), modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Save", Gtk.ResponseType.OK)
        dialog.set_default_response(Gtk.ResponseType.OK)
        grid = Gtk.Grid(row_spacing=8, column_spacing=10, border_width=14)
        dialog.get_content_area().add(grid)
        time_entry = Gtk.Entry(text=clock_text(point["minute"]), width_chars=6, activates_default=True)
        unit = self.lane["unit"]
        value = Gtk.SpinButton.new_with_range(self.lane["min"], self.lane["max"], 50 if unit == "K" else 1)
        value.set_value(point["value"])
        value.set_activates_default(True)
        tension = Gtk.SpinButton.new_with_range(-1, 1, 0.05)
        tension.set_digits(2)
        tension.set_value(point.get("tension", 0))
        tension.set_tooltip_text("Bends the curve from this point to the next: 0 is straight, "
                                 "above 0 bulges it up, below 0 down.")
        for row, (label, widget) in enumerate((("Time (HH:MM)", time_entry),
                                               (f"Value ({'K' if unit == 'K' else '%'})", value),
                                               ("Bend to the next point", tension))):
            grid.attach(Gtk.Label(label=label, xalign=0), 0, row, 1, 1)
            grid.attach(widget, 1, row, 1, 1)
        dialog.show_all()
        answer = dialog.run()
        clock = re.fullmatch(r"\s*(\d{1,2}):(\d{2})\s*", time_entry.get_text())
        new_value, new_tension = value.get_value(), tension.get_value()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        if not clock or int(clock.group(1)) > 23 or int(clock.group(2)) > 59:
            return
        minute = int(clock.group(1)) * 60 + int(clock.group(2))
        self.points = [p for i, p in enumerate(self.points) if i == index or p["minute"] != minute]
        point["minute"], point["value"], point["tension"] = minute, new_value, round(new_tension, 2)
        self.points.sort(key=lambda p: p["minute"])
        self.queue_draw()
        self.on_change(self)


def hm_text(minutes):
    """Minutes after midnight as HH:MM (00:00 for midnight at either end)."""
    minutes = int(round(minutes)) % 1440
    return f"{minutes // 60:02d}:{minutes % 60:02d}"


def hm_minutes(text):
    """HH:MM as minutes after midnight, or None."""
    try:
        hours, minutes = (int(part) for part in text.split(":"))
    except (ValueError, AttributeError):
        return None
    return hours * 60 + minutes if 0 <= hours <= 24 and 0 <= minutes < 60 else None


class WallpaperTimeline(Gtk.DrawingArea):
    """Wallpapers on a 24-hour timeline (§4.10; owner, 2026-11-16), like
    clips in a video editor: one lane per picture, its bar from its start to
    its end (past midnight it wraps round). Drag a bar to move it, an edge
    to change its start or end (5-minute steps; Shift: to the minute);
    on_change(index, from, to) gets the new times, in minutes. Where
    pictures overlap the one that started last shows: what is covered is
    drawn faint."""

    LEFT, RIGHT, TOP, LANE, GAP, BOTTOM = 14, 14, 22, 44, 6, 6
    EDGE = 8

    def __init__(self, on_change):
        super().__init__()
        self.on_change = on_change
        self.items = []  # [{"index", "name", "from", "to", "pixbuf"}], minutes
        self.thumbs = {}  # path -> pixbuf or None
        self.minute_now = 0.0
        self.drag = None  # [item, part ("move", "from" or "to"), x at press, from, to]
        self.hover = None
        self.set_has_tooltip(True)
        self.add_events(Gdk.EventMask.BUTTON_PRESS_MASK | Gdk.EventMask.BUTTON_RELEASE_MASK | POINTER_EVENTS)
        self.connect("draw", self.draw)
        self.connect("button-press-event", self.pressed)
        self.connect("button-release-event", self.released)
        self.connect("motion-notify-event", self.motion)
        self.connect("query-tooltip", self.tooltip)

    def thumb(self, path):
        if path not in self.thumbs:
            from gi.repository import GdkPixbuf
            try:
                self.thumbs[path] = GdkPixbuf.Pixbuf.new_from_file_at_scale(path, 56, self.LANE - 8, True)
            except GLib.Error:
                self.thumbs[path] = None
        return self.thumbs[path]

    def set_wallpapers(self, wallpapers, minute_now):
        if self.drag is not None:
            return
        self.items = []
        for picture in wallpapers:
            start, end = hm_minutes(picture["from"]), hm_minutes(picture["to"])
            if start is None or end is None:
                continue
            self.items.append({"index": picture["index"], "name": picture["name"], "from": start % 1440,
                               "to": end % 1440, "pixbuf": self.thumb(picture.get("path", ""))})
        self.minute_now = minute_now
        self.set_size_request(-1, self.TOP + max(len(self.items), 1) * (self.LANE + self.GAP) + self.BOTTOM)
        self.queue_draw()

    # Geometry
    def span(self):
        return self.LEFT, max(self.get_allocated_width() - self.LEFT - self.RIGHT, 1)

    def x_of(self, minute):
        left, width = self.span()
        return left + width * minute / 1440

    def minute_at(self, x):
        left, width = self.span()
        return min(max((x - left) / width * 1440, 0), 1440)

    def lane_top(self, i):
        return self.TOP + i * (self.LANE + self.GAP)

    @staticmethod
    def segments(item):
        start, end = item["from"], item["to"]
        if start < end:
            return [(start, end)]
        if start > end:
            return [(start, 1440), (0, end)]
        return [(0, 1440)]

    def shown_by(self, minute):
        """The item whose picture is the wallpaper at minute: of those
        covering it, the one that started last before it."""
        best, best_age = None, None
        for item in self.items:
            if any(a <= minute < b for a, b in self.segments(item)):
                age = (minute - item["from"]) % 1440
                if best_age is None or age < best_age:
                    best, best_age = item, age
        return best

    def hit(self, x, y):
        """(item, part) under the pointer: part "from", "to" or "move"."""
        for i, item in enumerate(self.items):
            top = self.lane_top(i)
            if not top <= y <= top + self.LANE:
                continue
            for a, b in self.segments(item):
                xa, xb = self.x_of(a), self.x_of(b)
                if a == item["from"] and abs(x - xa) <= self.EDGE:
                    return item, "from"
                if b == item["to"] and abs(x - xb) <= self.EDGE:
                    return item, "to"
                if xa <= x <= xb:
                    return item, "move"
        return None, None

    # Drawing
    def draw(self, _widget, cr):
        accent, text, soft, faint = theme_colors(self)
        left, width = self.span()
        height = self.get_allocated_height()
        layout = self.create_pango_layout("")
        layout.set_font_description(Pango.FontDescription("Sans 8"))
        # Hours: a hairline each hour, a label every three.
        for hour in range(25):
            x = round(self.x_of(hour * 60)) + 0.5
            cr.set_source_rgba(*faint)
            cr.set_line_width(1)
            cr.move_to(x, self.TOP - 4)
            cr.line_to(x, height - self.BOTTOM)
            cr.stroke()
            if hour % 3 == 0 and hour < 24:
                layout.set_text(clock_text(hour * 60), -1)
                w, _h = layout.get_pixel_size()
                cr.set_source_rgba(*soft)
                cr.move_to(min(max(x - w / 2, 0), self.get_allocated_width() - w), 2)
                PangoCairo.show_layout(cr, layout)
        # The bars: solid where the picture shows, faint where a later one covers it.
        for i, item in enumerate(self.items):
            top = self.lane_top(i)
            for a, b in self.segments(item):
                xa, xb = self.x_of(a), self.x_of(b)
                rounded_rect(cr, xa + 1, top, max(xb - xa - 2, 2), self.LANE, 5)
                cr.set_source_rgba(accent[0], accent[1], accent[2], 0.28)
                cr.fill()
                minute = a
                while minute < b:
                    run_end = min(b, minute + 5)
                    if self.shown_by(minute) is item:
                        cr.rectangle(self.x_of(minute), top, self.x_of(run_end) - self.x_of(minute), self.LANE)
                    minute = run_end
                cr.save()
                rounded_rect(cr, xa + 1, top, max(xb - xa - 2, 2), self.LANE, 5)
                cr.clip()
                cr.set_source_rgba(accent[0], accent[1], accent[2], 0.9)
                for minute in range(int(a), int(b), 5):
                    if self.shown_by(minute) is item:
                        cr.rectangle(self.x_of(minute), top, self.x_of(min(minute + 5, b)) - self.x_of(minute) + 0.5,
                                     self.LANE)
                cr.fill()
                # The picture and its name inside the bar, where they fit.
                inner = xa + 6
                if item["pixbuf"] is not None and xb - xa > item["pixbuf"].get_width() + 12:
                    Gdk.cairo_set_source_pixbuf(cr, item["pixbuf"], inner, top + 4)
                    cr.paint()
                    inner += item["pixbuf"].get_width() + 6
                label = f"{item['name']}  {hm_text(item['from'])}–{hm_text(item['to'])}"
                layout.set_text(label, -1)
                w, h = layout.get_pixel_size()
                if inner + w < xb - 4:
                    cr.set_source_rgba(1, 1, 1, 1)
                    cr.move_to(inner, top + (self.LANE - h) / 2)
                    PangoCairo.show_layout(cr, layout)
                # Grips at the ends.
                for edge, where in ((a, "from"), (b, "to")):
                    if (where == "from" and edge == item["from"]) or (where == "to" and edge == item["to"]):
                        gx = self.x_of(edge) + (3 if where == "from" else -5)
                        cr.set_source_rgba(1, 1, 1, 0.8 if self.hover == (item["index"], where) else 0.45)
                        rounded_rect(cr, gx, top + self.LANE / 2 - 8, 2, 16, 1)
                        cr.fill()
                cr.restore()
        # Now.
        x = round(self.x_of(self.minute_now)) + 0.5
        cr.set_source_rgba(text[0], text[1], text[2], 0.7)
        cr.set_line_width(1)
        cr.move_to(x, self.TOP - 4)
        cr.line_to(x, height - self.BOTTOM)
        cr.stroke()
        return False

    # Pointer
    def snap(self, minute, event):
        step = 1 if event.state & Gdk.ModifierType.SHIFT_MASK else 5
        return int(round(minute / step) * step) % 1440

    def pressed(self, _widget, event):
        if event.button != 1:
            return False
        item, part = self.hit(event.x, event.y)
        if item is not None:
            self.drag = [item, part, event.x, item["from"], item["to"]]
        return True

    def motion(self, _widget, event):
        if self.drag is None:
            item, part = self.hit(event.x, event.y)
            hover = (item["index"], part) if item else None
            if hover != self.hover:
                self.hover = hover
                cursor = {"from": "ew-resize", "to": "ew-resize", "move": "grab"}.get(part, "default")
                window = self.get_window()
                if window is not None:
                    window.set_cursor(Gdk.Cursor.new_from_name(self.get_display(), cursor))
                self.queue_draw()
            return False
        item, part, x0, start, end = self.drag
        delta = self.minute_at(event.x) - self.minute_at(x0)
        if part == "move":
            shift = self.snap(delta, event) if abs(delta) >= 1 else 0
            item["from"], item["to"] = (start + shift) % 1440, (end + shift) % 1440
        else:
            minute = self.snap(self.minute_at(event.x), event)
            other = end if part == "from" else start
            if minute != other:
                item[part] = minute
        self.queue_draw()
        return True

    def released(self, _widget, event):
        if event.button != 1 or self.drag is None:
            return False
        item, _part, _x0, start, end = self.drag
        self.drag = None
        if (item["from"], item["to"]) != (start, end):
            self.on_change(item["index"], item["from"], item["to"])
        return True

    def tooltip(self, _widget, x, y, _keyboard, tooltip):
        item, part = self.hit(x, y)
        if item is None:
            return False
        what = {"from": "Drag to change when it starts", "to": "Drag to change when it ends",
                "move": "Drag to move it"}[part]
        tooltip.set_text(f"{item['name']}: {hm_text(item['from'])} to {hm_text(item['to'])}. {what} "
                         "(Shift: to the minute).")
        return True




def choose_picture(parent, title="Choose a wallpaper"):
    """A PNG, JPEG or WebP picture's path, or None."""
    chooser = Gtk.FileChooserDialog(title=title, transient_for=parent, action=Gtk.FileChooserAction.OPEN)
    chooser.add_button("Cancel", Gtk.ResponseType.CANCEL)
    chooser.add_button("Choose", Gtk.ResponseType.OK)
    pictures = Gtk.FileFilter()
    pictures.set_name("Pictures (PNG, JPEG, WebP)")
    for mime in ("image/png", "image/jpeg", "image/webp"):
        pictures.add_mime_type(mime)
    chooser.add_filter(pictures)
    for folder in ("/usr/share/backgrounds", os.path.expanduser("~/Pictures")):
        if os.path.isdir(folder):
            try:
                chooser.add_shortcut_folder(folder)
            except GLib.Error:
                pass
    answer = chooser.run()
    path = chooser.get_filename()
    chooser.destroy()
    return path if answer == Gtk.ResponseType.OK and path else None


def picture_thumb(path, width=112, height=63):
    """A thumbnail of a stored wallpaper (a placeholder when unreadable)."""
    from gi.repository import GdkPixbuf
    try:
        thumb = Gtk.Image.new_from_pixbuf(GdkPixbuf.Pixbuf.new_from_file_at_scale(path, width, height, True))
    except GLib.Error:
        thumb = Gtk.Image.new_from_icon_name("image-missing", Gtk.IconSize.DIALOG)
    thumb.set_size_request(width, height)
    return thumb


class WallpaperEditor(Gtk.Box):
    """Wallpapers by time (§4.10; R-MODE-5, owner 2026-11-17): the timeline,
    a row per picture (its times typed, Set, Remove) and Add a wallpaper…,
    the same for circadian rhythm and for each mode. command() gives the
    command's start: ["circadian", "wallpaper"] or ["mode", "wallpaper",
    <mode>]."""

    def __init__(self, window, command):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.window, self.command = window, command
        self.timeline = WallpaperTimeline(self.moved)
        self.rows = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.add_button = Gtk.Button(label="Add a wallpaper…", halign=Gtk.Align.START)
        self.add_button.connect("clicked", self.add)
        for widget in (self.timeline, self.rows, self.add_button):
            self.pack_start(widget, False, False, 0)
        self.shown = None

    def run(self, *args):
        self.window.run([*self.command(), *args])

    def moved(self, index, start, end):
        """A bar dragged on the timeline: the picture's new times."""
        self.run("time", str(index), hm_text(start), hm_text(end))

    def fill(self, wallpapers):
        now = datetime.datetime.now()
        self.timeline.set_wallpapers(wallpapers, now.hour * 60 + now.minute)
        self.timeline.set_visible(bool(wallpapers))
        # The rows only when they change: times being typed stay.
        key = (tuple(self.command()), json.dumps(wallpapers, sort_keys=True))
        if key == self.shown:
            return
        self.shown = key
        for child in self.rows.get_children():
            self.rows.remove(child)
        for picture in wallpapers:
            row = Gtk.Box(spacing=10)
            row.pack_start(picture_thumb(picture["path"]), False, False, 0)
            name = Gtk.Label(label=picture["name"] + ("  (now)" if picture["active"] else ""), xalign=0,
                             ellipsize=Pango.EllipsizeMode.MIDDLE, max_width_chars=30)
            row.pack_start(name, True, True, 0)
            start = Gtk.Entry(text=picture["from"], width_chars=6, max_length=5)
            end = Gtk.Entry(text=picture["to"], width_chars=6, max_length=5)
            save = lambda _w, p=picture, s=start, e=end: self.run(
                "time", str(p["index"]), s.get_text().strip(), e.get_text().strip())
            start.connect("activate", save)
            end.connect("activate", save)
            set_button = Gtk.Button(label="Set")
            set_button.connect("clicked", save)
            remove = Gtk.Button(label="Remove")
            remove.connect("clicked", lambda _b, p=picture: self.run("remove", str(p["index"])))
            for widget in (start, Gtk.Label(label="to"), end, set_button, remove):
                widget.set_valign(Gtk.Align.CENTER)
                row.pack_start(widget, False, False, 0)
            self.rows.pack_start(row, False, False, 0)
        if not wallpapers:
            self.rows.pack_start(dim("None yet."), False, False, 0)
        self.rows.show_all()

    def add(self, _button, answer=None):
        """A picture and its times. answer: for tests, (path, from, to)."""
        if answer is not None:
            self.run("add", *answer)
            return
        path = choose_picture(self.get_toplevel())
        if not path:
            return
        dialog = Gtk.Dialog(title="When is it the wallpaper?", transient_for=self.get_toplevel(), modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Add", Gtk.ResponseType.OK)
        dialog.set_default_response(Gtk.ResponseType.OK)
        row = Gtk.Box(spacing=8, border_width=14)
        start = Gtk.Entry(text="06:00", width_chars=6, max_length=5, activates_default=True)
        end = Gtk.Entry(text="12:00", width_chars=6, max_length=5, activates_default=True)
        for widget in (Gtk.Label(label="From"), start, Gtk.Label(label="to"), end):
            row.pack_start(widget, False, False, 0)
        dialog.get_content_area().add(row)
        dialog.show_all()
        reply = dialog.run()
        times = start.get_text().strip(), end.get_text().strip()
        dialog.destroy()
        if reply == Gtk.ResponseType.OK:
            self.run("add", path, *times)


class CircadianPage(Gtk.ScrolledWindow):
    """Settings that follow the time of day (§4.10). Reads 'kc9k circadian
    show --json'; every change runs a 'kc9k circadian' command. Nothing here
    can be locked."""

    PREVIEW_MS = 300
    PLAY_SECONDS = 15

    def __init__(self, window):
        super().__init__(hscrollbar_policy=Gtk.PolicyType.NEVER)
        self.window = window
        self.view = None
        self.shown = {}  # part of the page -> the data it was built from
        self.filling = False
        self.reading = False
        self.preview_busy = False
        self.preview_next = None
        self.preview_at = 0
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18,
                      margin_start=24, margin_end=24, margin_top=20, margin_bottom=20)
        self.add(box)

        # R-GUI-3 (2026-11-17): a page title and one line, as on the other
        # pages; the long explanations behind ?.
        top = Gtk.Box(spacing=8)
        title = Gtk.Label(label="Circadian rhythm", xalign=0)
        title.get_style_context().add_class("kc9k-page-title")
        top.pack_start(title, False, False, 0)
        top.pack_start(help_button(
            "The screen, the wallpaper and dark mode follow the time of day: cues for the body "
            "clock. Nothing here can be locked, and it never overrides you: change a level by "
            "hand and its clip pauses until its next point. It leaves the desktop's own "
            "brightness control and night light working; off, it touches nothing."), False, False, 0)
        self.master = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.master.connect("state-set", self.switch_master)
        top.pack_end(self.master, False, False, 0)
        # §4.10 preview (owner, 2026-11-16): the next 24 hours in seconds.
        self.play = Gtk.Button(label="Play the day", valign=Gtk.Align.CENTER)
        self.play.set_tooltip_text(f"Shows the next 24 hours on the screen in {self.PLAY_SECONDS} seconds "
                                   "(brightness, colour, wallpaper, dark mode), then goes back to now")
        self.play.connect("clicked", self.play_day)
        top.pack_end(self.play, False, False, 0)
        self.playing = False
        head = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
        head.pack_start(top, False, False, 0)
        subtitle = Gtk.Label(label="Brightness, colour, wallpaper and dark mode follow the time of day.",
                             xalign=0, wrap=True)
        subtitle.get_style_context().add_class("kc9k-page-subtitle")
        head.pack_start(subtitle, False, False, 0)
        box.pack_start(head, False, False, 0)
        self.warnings = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6, no_show_all=True)
        box.pack_start(self.warnings, False, False, 0)

        preset_row = Gtk.Box(spacing=8)
        preset_row.pack_start(Gtk.Label(label="Start from a preset:"), False, False, 0)
        self.preset = Gtk.ComboBoxText()
        preset_row.pack_start(self.preset, False, False, 0)
        apply_preset = Gtk.Button(label="Use it")
        apply_preset.connect("clicked", self.use_preset)
        preset_row.pack_start(apply_preset, False, False, 0)
        self.preset_detail = dim("")
        clips = section("Automation clips", dim("A curve over 24 hours for each level; right-click adds a point."),
                        preset_row, self.preset_detail, help=(
                            "Each clip is a curve over 24 hours. Right-click adds a point (on a point: "
                            "removes it). Drag a point to move it (Shift: to the minute), drag the curve "
                            "to bend it, double-click a point to type exact values. While you drag a "
                            "point, the screen shows its value."))
        self.preset.connect("changed", self.preset_changed)
        box.pack_start(clips, False, False, 0)
        self.editors, self.lane_switches, self.lane_values = {}, {}, {}
        self.clips_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=14)
        clips.pack_start(self.clips_box, False, False, 0)

        self.wallpaper_editor = WallpaperEditor(self.window, lambda: ["circadian", "wallpaper"])
        self.wallpapers, self.wallpaper_timeline = self.wallpaper_editor.rows, self.wallpaper_editor.timeline
        # R-MODE-5 (owner 2026-11-17): one setting with each mode's
        # "Override circadian rhythm wallpapers", seen from this side.
        override_row = Gtk.Box(spacing=12)
        override_row.pack_start(Gtk.Label(label="Override modes", xalign=0), False, False, 0)
        override_row.pack_start(dim("its wallpapers over a mode's"), False, False, 0)
        self.override_modes = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.override_modes.connect("state-set", self.switch_override_modes)
        override_row.pack_end(self.override_modes, False, False, 0)
        box.pack_start(section("Wallpapers", dim("Each picture is the wallpaper between its two times."),
                               self.wallpaper_editor, override_row, help=(
                                   "Drag a bar to move it, or an end to change when it starts or stops "
                                   "(Shift: to the minute). Where times overlap, the one that started "
                                   "last shows; what it covers is faint. A wallpaper you pick yourself "
                                   "stays until the next one starts. The wallpaper due is in place when "
                                   "you sign in. Modes have wallpapers too: while one is enabled its "
                                   "wallpaper shows, unless Override modes is on (the same setting as a "
                                   "mode's Override circadian rhythm wallpapers, the other way round). "
                                   "What shows when neither has one is the wallpaper default, in "
                                   "Settings.")), False, False, 0)

        self.dark_radios = {}
        dark_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        first = None
        for mode, label in (("system", "Use the system's setting (KC9K leaves it alone)"),
                            ("light", "Always light"), ("dark", "Always dark"),
                            ("scheduled", "Light mode between")):
            radio = Gtk.RadioButton.new_with_label_from_widget(first, label)
            first = first or radio
            radio.connect("toggled", self.dark_toggled, mode)
            self.dark_radios[mode] = radio
            if mode != "scheduled":
                dark_box.pack_start(radio, False, False, 0)
        times = Gtk.Box(spacing=6)
        times.pack_start(self.dark_radios["scheduled"], False, False, 0)
        self.light_from = Gtk.Entry(width_chars=6, max_length=5)
        self.light_to = Gtk.Entry(width_chars=6, max_length=5)
        times.pack_start(self.light_from, False, False, 0)
        times.pack_start(Gtk.Label(label="and"), False, False, 0)
        times.pack_start(self.light_to, False, False, 0)
        set_times = Gtk.Button(label="Set")
        set_times.connect("clicked", lambda _b: self.set_dark_times())
        self.light_from.connect("activate", lambda _e: self.set_dark_times())
        self.light_to.connect("activate", lambda _e: self.set_dark_times())
        times.pack_start(set_times, False, False, 0)
        dark_box.pack_start(times, False, False, 0)
        box.pack_start(section("Dark mode", dim(
            "The dark mode of the desktop and its apps, like a phone's."), dark_box), False, False, 0)

        pages_row = Gtk.Box(spacing=12)
        pages_row.pack_start(Gtk.Label(label="Show dark web pages while dark mode is on", xalign=0),
                             False, False, 0)
        self.dark_pages = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.dark_pages.connect("state-set", self.switch_dark_pages)
        pages_row.pack_end(self.dark_pages, False, False, 0)
        self.exempt = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        exempt_row = Gtk.Box(spacing=6)
        self.exempt_entry = Gtk.Entry(placeholder_text="A site to leave as it is (example.com)", width_chars=34)
        self.exempt_entry.connect("activate", self.add_exempt)
        exempt_add = Gtk.Button(label="Add")
        exempt_add.connect("clicked", self.add_exempt)
        exempt_row.pack_start(self.exempt_entry, False, False, 0)
        exempt_row.pack_start(exempt_add, False, False, 0)
        box.pack_start(section("Dark pages", pages_row, self.exempt, exempt_row, help=(
            "The browser's own dark mode comes on: sites with a dark design show it, and light "
            "pages without one are shown dark (LibreWolf from its next start). With the system's "
            "setting, pages follow what the browser says the system prefers. The extension's "
            "toolbar button exempts a site too.")), False, False, 0)

    # Reading the state
    def refresh(self, force=False):
        """Reads the state when the page shows (or was never read)."""
        showing = self.window.stack.get_visible_child_name() == "circadian"
        if self.reading or self.window.locked("circadian") or not (showing or force or self.view is None):
            return
        self.reading = True
        passwords = list(self.window.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, ["circadian", "show", "--json"])
            GLib.idle_add(self.apply, ok, output)

        threading.Thread(target=work, daemon=True).start()

    def apply(self, ok, output):
        self.reading = False
        if not ok:
            return False
        try:
            view = json.loads(output)
        except ValueError:
            return False
        self.view = view
        self.filling = True
        try:
            self.fill(view)
        finally:
            self.filling = False
        return False

    def changed(self, key, data):
        """Whether a part of the page must be built again."""
        text = json.dumps(data, sort_keys=True)
        if self.shown.get(key) == text:
            return False
        self.shown[key] = text
        return True

    def fill(self, view):
        set_switch(self.master, view["on"])
        if self.changed("presets", view["presets"]):
            self.preset.remove_all()
            for preset in view["presets"]:
                self.preset.append(preset["name"], preset["title"])
            self.preset.set_active(0)
        for lane in view["lanes"]:
            name = lane["name"]
            if name not in self.editors:
                self.build_lane(lane)
            set_switch(self.lane_switches[name], lane["on"])
            now = (f"now {lane['value']:.0f} {lane['unit']}" if lane["has_value"] else "no points")
            if lane["paused_until"]:
                until = datetime.datetime.fromtimestamp(lane["paused_until"]).strftime("%H:%M" if CLOCK_24 else "%-I:%M %p")
                now += f" · paused until {until}"
            elif not lane["on"]:
                now += " · off"
            self.lane_values[name].set_text(now)
            self.editors[name].set_data(lane, view["minute"], view["on"] and lane["on"])
        self.fill_warnings(view)
        self.wallpaper_editor.fill(view["wallpapers"])
        set_switch(self.override_modes, bool(view.get("override_modes")))
        dark = view["dark"]
        self.dark_radios[dark["mode"]].set_active(True)
        if self.changed("dark times", [dark["light_from"], dark["light_to"]]):
            self.light_from.set_text(dark["light_from"])
            self.light_to.set_text(dark["light_to"])
        set_switch(self.dark_pages, view["dark_pages"])
        if self.changed("exempt", view["dark_exempt"]):
            for child in self.exempt.get_children():
                self.exempt.remove(child)
            for site in view["dark_exempt"]:
                row = Gtk.Box(spacing=8)
                row.pack_start(Gtk.Label(label=site, xalign=0), False, False, 0)
                remove = Gtk.Button.new_from_icon_name("list-remove-symbolic", Gtk.IconSize.BUTTON)
                remove.set_relief(Gtk.ReliefStyle.NONE)
                remove.set_tooltip_text("Dark pages change this site again")
                remove.connect("clicked", lambda _b, s=site: self.window.run(["circadian", "exempt", "remove", s]))
                row.pack_start(remove, False, False, 0)
                self.exempt.pack_start(row, False, False, 0)
            self.exempt.show_all()

    def build_lane(self, lane):
        name = lane["name"]
        frame = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        top = Gtk.Box(spacing=10)
        title = Gtk.Label(xalign=0)
        title.set_markup(f"<b>{GLib.markup_escape_text(lane['title'])}</b>")
        top.pack_start(title, False, False, 0)
        value = dim("", wrap=False)
        top.pack_start(value, False, False, 0)
        switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        switch.connect("state-set", self.switch_lane, name)
        top.pack_end(switch, False, False, 0)
        frame.pack_start(top, False, False, 0)
        frame.pack_start(dim(lane["detail"]), False, False, 0)
        editor = ClipEditor(lane, self.clip_changed, self.clip_preview)
        frame.pack_start(editor, False, False, 0)
        self.editors[name], self.lane_switches[name], self.lane_values[name] = editor, switch, value
        self.clips_box.pack_start(frame, False, False, 0)
        frame.show_all()

    def fill_warnings(self, view):
        hardware = view["hardware"]
        lanes = {lane["name"]: lane for lane in view["lanes"]}
        notes = []  # (text, button label, command)
        paused = [lane for lane in view["lanes"] if lane["paused_until"]]
        for lane in paused:
            if lane["name"] == "software" and lanes["kelvin"]["paused_until"]:
                continue  # one pause, said once with the colour temperature
            until = datetime.datetime.fromtimestamp(lane["paused_until"]).strftime("%H:%M" if CLOCK_24 else "%-I:%M %p")
            what = ("Software brightness and colour temperature are"
                    if lane["name"] == "kelvin" else f"{lane['title']} is")
            notes.append((f"{what} paused until {until}, because the level was changed by hand.",
                          "Resume", ["circadian", "resume", lane["name"]]))
        if view["on"] and hardware["daemon"]:
            if hardware["night_light"] and (lanes["kelvin"]["on"] or lanes["software"]["on"]):
                notes.append(("The desktop's own night light is on. It changes the same colours as "
                              "these clips, and the two would take turns.",
                              "Turn the night light off", ["circadian", "nightlight", "off"]))
            if lanes["backlight"]["on"] and not hardware["backlights"] and not hardware["external"]:
                notes.append(("No screen brightness to drive: this computer has no backlight"
                              + (", and no screen answers DDC/CI." if hardware["ddcutil"] else
                                 ", and external screens need the program ddcutil (sudo apt install ddcutil).")
                              + " Software brightness works on every screen.", None, None))
            if (lanes["kelvin"]["on"] or lanes["software"]["on"]) and hardware["gamma"] == "unknown":
                notes.append(("No screen colours to change: there is no X11 session (on Wayland, "
                              "software brightness and colour temperature don't work).", None, None))
        if not self.changed("warnings", notes):
            return
        for child in self.warnings.get_children():
            self.warnings.remove(child)
        for text, label, command in notes:
            bar = Gtk.InfoBar(message_type=Gtk.MessageType.WARNING if command is None or "night" in label.lower()
                              else Gtk.MessageType.INFO)
            bar.get_content_area().add(Gtk.Label(label=text, xalign=0, wrap=True))
            if command:
                button = Gtk.Button(label=label)
                button.connect("clicked", lambda _b, c=command: self.window.run(c))
                bar.get_action_area().add(button)
            self.warnings.pack_start(bar, False, False, 0)
            bar.show_all()
        self.warnings.set_visible(bool(notes))

    # Changes
    def switch_master(self, switch, state):
        if not self.filling:
            self.window.run(["circadian", "on" if state else "off"])
        return True

    def switch_lane(self, _switch, state, name):
        if not self.filling:
            self.window.run(["circadian", "lane", name, "on" if state else "off"])
        return True

    def switch_dark_pages(self, _switch, state):
        if not self.filling:
            self.window.run(["circadian", "dark-pages", "on" if state else "off"])
        return True

    def switch_override_modes(self, _switch, state):
        if not self.filling:
            self.window.run(["wallpaper", "priority", "circadian" if state else "modes"])
        return True

    def preset_changed(self, _combo):
        chosen = self.preset.get_active_id()
        for preset in (self.view or {}).get("presets", []):
            if preset["name"] == chosen:
                self.preset_detail.set_text(preset["detail"])

    def use_preset(self, _button):
        name = self.preset.get_active_id()
        if name and self.window.confirm("Replace the three clips?",
                                        "Their points are replaced by the preset's. Which clips are on stays.",
                                        ok_label="Replace"):
            self.window.run(["circadian", "preset", name])

    def background(self, args, then=None, timeout=30):
        """Runs a command without greying the window out (a drag goes on)."""
        passwords = list(self.window.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, args, timeout=timeout)
            GLib.idle_add(done, ok, output)

        def done(ok, output):
            if then:
                then(ok, output)
            return False

        threading.Thread(target=work, daemon=True).start()

    def play_day(self, _button):
        """Plays the next 24 hours on the screen ('kc9k circadian play') while
        a playhead runs over the clips; then everything is back to now."""
        if not self.view or not self.view.get("on"):
            self.window.message("Switch the circadian rhythm on to play the day.", Gtk.MessageType.INFO)
            return
        now = datetime.datetime.now()
        start, started = now.hour * 60 + now.minute + now.second / 60, GLib.get_monotonic_time()
        self.playing = True
        self.play.set_sensitive(False)
        self.play.set_label("Playing…")

        def tick():
            if not self.playing:
                return False
            done = min((GLib.get_monotonic_time() - started) / 1e6 / self.PLAY_SECONDS, 1)
            for editor in self.editors.values():
                editor.playhead = (start + 1440 * done) % 1440
                editor.queue_draw()
            return True

        def finished(ok, output):
            self.playing = False
            for editor in self.editors.values():
                editor.playhead = None
                editor.queue_draw()
            self.play.set_sensitive(True)
            self.play.set_label("Play the day")
            if not ok:
                self.window.message(output or "The day couldn't be played.")
            self.refresh(force=True)

        GLib.timeout_add(100, tick)
        self.background(["circadian", "play", "--seconds", str(self.PLAY_SECONDS)], finished,
                        timeout=self.PLAY_SECONDS + 45)

    def clip_changed(self, editor, preview_over=False):
        """Stores a clip's points after an edit."""
        name = editor.lane["name"]

        def stored(ok, output):
            if not ok:
                self.window.message(output or "The clip couldn't be saved.")
            if preview_over:
                self.background(["circadian", "preview", name, "--end"], lambda *_a: self.refresh(force=True))
            else:
                self.refresh(force=True)

        self.background(["circadian", "lane", name, "--points", points_text(editor.points)], stored)

    def clip_preview(self, editor, value):
        """Shows a dragged point's value on the screen, at most every
        PREVIEW_MS, and only for a clip KC9K is driving."""
        lane = next((l for l in (self.view or {}).get("lanes", []) if l["name"] == editor.lane["name"]), None)
        if not lane or not self.view["on"] or not lane["on"] or lane["paused_until"]:
            return
        self.preview_next = (lane["name"], value)
        if not self.preview_busy:
            self.send_preview()

    def send_preview(self):
        if self.preview_next is None:
            return False
        wait = self.PREVIEW_MS - (GLib.get_monotonic_time() - self.preview_at) / 1000
        if wait > 0:
            self.preview_busy = True
            GLib.timeout_add(int(wait) + 1, self.send_preview_now)
            return False
        return self.send_preview_now()

    def send_preview_now(self):
        if self.preview_next is None:
            self.preview_busy = False
            return False
        name, value = self.preview_next
        self.preview_next = None
        self.preview_busy = True
        self.preview_at = GLib.get_monotonic_time()

        def sent(_ok, _output):
            self.preview_busy = False
            self.send_preview()

        self.background(["circadian", "preview", name, f"{value:g}"], sent)
        return False

    def dark_toggled(self, radio, mode):
        if self.filling or not radio.get_active():
            return
        if mode == "scheduled":
            self.set_dark_times()
        else:
            self.window.run(["circadian", "dark", mode])

    def set_dark_times(self):
        self.window.run(["circadian", "dark", "light-between",
                         self.light_from.get_text().strip(), self.light_to.get_text().strip()])

    def add_exempt(self, _widget):
        site = self.exempt_entry.get_text().strip()
        if site:
            self.window.run(["circadian", "exempt", "add", site],
                            then=lambda _o: self.exempt_entry.set_text(""))

    def add_wallpaper(self, button=None, answer=None):
        self.wallpaper_editor.add(button, answer)
