# Durations, numbers, dates, clock, timer and lock fields, hotkey fields (R-GUI-1, R-GUI-2).
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

# Durations, numbers and timer locks (R-GUI-1, R-GUI-2) -------------------
#
# Every length is typed in days, hours and minutes ("2d 4h", "90m",
# "1 week"); a number or length out of range is never changed silently: the
# field says why under the box, keeps the text, and the dialog's main button
# waits until it reads right.

_DURATION_UNITS = {
    "w": 604800, "wk": 604800, "wks": 604800, "week": 604800, "weeks": 604800,
    "d": 86400, "day": 86400, "days": 86400,
    "h": 3600, "hr": 3600, "hrs": 3600, "hour": 3600, "hours": 3600,
    "m": 60, "min": 60, "mins": 60, "minute": 60, "minutes": 60,
    "s": 1, "sec": 1, "secs": 1, "second": 1, "seconds": 1,
}
_DURATION_PART = re.compile(r"\s*(\d+(?:\.\d+)?)\s*([a-z]*)\s*")
DURATION_EXAMPLES = "45m, 2h 30m, 3d or 1w"


def parse_duration(text, bare=60):
    """Seconds in a typed length, or None when it can't be read: '2d 4h',
    '2 days and 4 hours', '1w', '90m', '1.5 h', '1,5h'. A number alone
    counts in `bare` seconds (minutes by default). Empty is 0."""
    text = re.sub(r"(\d),(\d)", r"\1.\2", (text or "").strip().lower())
    text = re.sub(r"\band\b|,|\+", " ", text).strip()
    if not text:
        return 0
    total, pos, parts = 0.0, 0, 0
    while pos < len(text):
        match = _DURATION_PART.match(text, pos)
        if not match or match.end() == pos:
            return None
        number, unit = float(match.group(1)), match.group(2)
        if unit:
            if unit not in _DURATION_UNITS:
                return None
            total += number * _DURATION_UNITS[unit]
        elif pos == 0 and match.end() == len(text):
            total += number * bare  # a number alone
        else:
            return None  # "2 4h": which unit is the 2?
        pos, parts = match.end(), parts + 1
    return int(round(total))


def duration_words(seconds):
    """A length for reading: '69 days 10 hours 40 minutes', '1 hour',
    '45 seconds'."""
    seconds = max(0, int(seconds))
    if seconds < 60:
        return f"{seconds} second" + ("" if seconds == 1 else "s")
    parts = []
    for size, unit in ((86400, "day"), (3600, "hour"), (60, "minute")):
        count, seconds = divmod(seconds, size)
        if count:
            parts.append(f"{count} {unit}" + ("" if count == 1 else "s"))
    return " ".join(parts)


def duration_text(seconds):
    """A length as a field shows it: '2d 4h 30m', '45m', '30s'."""
    seconds = max(0, int(seconds))
    if seconds < 60:
        return f"{seconds}s"
    parts = []
    for size, unit in ((86400, "d"), (3600, "h"), (60, "m")):
        count, seconds = divmod(seconds, size)
        if count:
            parts.append(f"{count}{unit}")
    return " ".join(parts)


def mark_problem(widget, problem):
    """Shows a field as wrong (red) or right."""
    context = widget.get_style_context()
    if problem:
        context.add_class("error")
    else:
        context.remove_class("error")


HOTKEY_MODIFIERS = {"ctrl": "ctrl", "control": "ctrl", "primary": "ctrl", "alt": "alt", "shift": "shift",
                    "super": "super", "win": "super", "meta": "super"}


def hotkey_problem(text):
    """Why keys typed like ctrl+alt+s can't be a hotkey, or None: the rules
    of the CLI's blocker.ParseAccel, checked where they are typed."""
    parts = text.strip().split("+")
    key = parts[-1].strip()
    if not key:
        return "Type keys such as ctrl+alt+s or super+F9."
    mods = []
    for part in parts[:-1]:
        mod = HOTKEY_MODIFIERS.get(part.strip().lower())
        if not mod:
            return f"“{part.strip()}” isn't a modifier: use ctrl, alt, shift or super."
        mods.append(mod)
    if not re.fullmatch(r"[A-Za-z0-9_]+", key):
        return f"“{key}” isn't a key."
    if not mods and not re.fullmatch(r"F([1-9]|1[0-9]|2[0-4])", key.upper()):
        return "Add ctrl, alt, shift or super: only F1 to F24 work alone."
    return None


def hotkey_words(accel):
    """A GTK accelerator such as <Primary><Alt>s as ctrl+alt+s."""
    for tag, word in (("<Primary>", "ctrl+"), ("<Alt>", "alt+"), ("<Shift>", "shift+"), ("<Super>", "super+")):
        accel = accel.replace(tag, word)
    return accel


class CheckedField(Gtk.Box):
    """A typed value with a line under it that reads it back or says what
    is wrong. Subclasses give read(); value is None while it is wrong."""

    def __init__(self, text="", width=12, placeholder="", presets=(), hint=""):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        self.callbacks = []
        line = Gtk.Box(spacing=6)
        self.entry = Gtk.Entry(width_chars=width, text=text, placeholder_text=placeholder,
                               activates_default=True)
        self.entry.connect("changed", self.changed)
        line.pack_start(self.entry, False, False, 0)
        self.presets = []
        for label, value in presets:
            button = Gtk.Button(label=label, valign=Gtk.Align.CENTER)
            button.get_style_context().add_class("kc9k-chip")
            button.connect("clicked", lambda _b, v=value: self.set_text(v))
            line.pack_start(button, False, False, 0)
            self.presets.append(button)
        self.pack_start(line, False, False, 0)
        self.note = Gtk.Label(xalign=0, wrap=True, max_width_chars=60, no_show_all=True)
        self.note.get_style_context().add_class("kc9k-note")
        self.pack_start(self.note, False, False, 0)
        self.hint = hint
        self.value, self.problem = None, None
        self.changed()

    def set_text(self, text):
        self.entry.set_text(text)

    def read(self, text):
        """(value, None) or (None, what is wrong)."""
        raise NotImplementedError

    def describe(self, value):
        return ""

    def changed(self, *_args):
        self.value, self.problem = self.read(self.entry.get_text())
        mark_problem(self.entry, self.problem)
        mark_problem(self.note, self.problem)
        self.note.set_text(self.problem or self.describe(self.value) or self.hint)
        self.note.set_visible(bool(self.note.get_text()))
        for callback in self.callbacks:
            callback(self)

    def on_change(self, callback):
        self.callbacks.append(callback)

    def is_valid(self):
        return self.problem is None

    def set_sensitive(self, sensitive):
        super().set_sensitive(sensitive)


class DurationField(CheckedField):
    """R-GUI-1: a length in days, hours and minutes. minimum and maximum
    are seconds (maximum None: no limit); zero, when given, is what an empty
    or 0 field means ('No re-lock'), otherwise the minimum applies; bare is
    what a number alone counts in (60: minutes, 1: seconds)."""

    def __init__(self, seconds=0, minimum=60, maximum=None, zero=None, bare=60,
                 presets=(), why_max="", width=12):
        self.minimum, self.maximum, self.zero, self.bare, self.why_max = minimum, maximum, zero, bare, why_max
        # A meaning too long for the box is said under it; the box says none.
        self.zero_below = zero is not None and len(zero) > width + 1
        placeholder = ("none" if self.zero_below else zero) if zero else "e.g. 2h 30m" if bare == 60 else "e.g. 90s"
        super().__init__(duration_text(seconds) if seconds else "", width, placeholder,
                         [(short_length(p), duration_text(p)) for p in presets])

    def read(self, text):
        seconds = parse_duration(text, self.bare)
        if seconds is None:
            return None, f"Type a length such as {DURATION_EXAMPLES}." if self.bare == 60 else \
                "Type a length such as 30s, 90s or 2m."
        if seconds == 0 and self.zero is not None:
            return 0, None
        if seconds < self.minimum:
            return None, f"At least {duration_words(self.minimum)}."
        if self.bare == 60 and seconds % 60:
            return None, "Use whole minutes (the commands take minutes)."
        if self.maximum is not None and seconds > self.maximum:
            return None, f"At most {duration_words(self.maximum)}." + (f" {self.why_max}" if self.why_max else "")
        return seconds, None

    def describe(self, value):
        if value == 0 and self.zero is not None:
            if self.zero_below:
                return self.zero
            # The placeholder already says it while empty.
            return "" if not self.entry.get_text().strip() else self.zero
        return duration_words(value)

    @property
    def seconds(self):
        return self.value

    @property
    def minutes(self):
        return None if self.value is None else self.value // 60

    def set_seconds(self, seconds):
        self.set_text(duration_text(seconds) if seconds else "")

    def arg(self):
        """The value as a command takes it: '90m' or '45s'."""
        return f"{self.value // 60}m" if self.bare == 60 else f"{self.value}s"

    def set_limits(self, minimum=None, maximum=None, why_max=None):
        """Changes the allowed range (e.g. an allowance per week may be
        longer than one per day) and checks the text again."""
        if minimum is not None:
            self.minimum = minimum
        self.maximum = maximum
        if why_max is not None:
            self.why_max = why_max
        self.changed()

    # As the spin buttons it replaces were used: whole minutes (seconds for
    # a field of seconds); 0 while the text is wrong.
    def get_value_as_int(self):
        if self.value is None:
            return 0
        return self.value // 60 if self.bare == 60 else self.value

    def set_value(self, amount):
        self.set_seconds(int(amount) * (60 if self.bare == 60 else 1))

    def has_focus(self):
        return self.entry.has_focus()


def short_length(seconds):
    """A preset's label: '15 min', '1 hour', '1 day', '1 week', '30 days'."""
    for size, one, many in ((604800, "week", "weeks"), (86400, "day", "days"), (3600, "hour", "hours")):
        if seconds >= size and seconds % size == 0:
            count = seconds // size
            return f"{count} {one if count == 1 else many}"
    if seconds >= 60:
        return f"{seconds // 60} min"
    return f"{seconds} s"


class NumberField(CheckedField):
    """A whole number between minimum and maximum, never clamped silently."""

    def __init__(self, value, minimum, maximum, unit="", width=8, hint=""):
        self.minimum, self.maximum, self.unit = minimum, maximum, unit
        super().__init__(str(value), width, hint=hint)

    def read(self, text):
        text = text.strip().replace(" ", "")
        if not re.fullmatch(r"\d+", text):
            return None, f"Type a whole number from {self.minimum} to {self.maximum}."
        number = int(text)
        if number < self.minimum:
            return None, f"At least {self.minimum}."
        if number > self.maximum:
            return None, f"At most {self.maximum}."
        return number, None

    def describe(self, value):
        return self.hint

    def get_value_as_int(self):
        return self.value or 0

    def set_value(self, number):
        self.set_text(str(int(number)))

    def set_limits(self, minimum, maximum):
        """Changes the allowed range and checks the text again."""
        self.minimum, self.maximum = minimum, maximum
        self.changed()

    def has_focus(self):
        return self.entry.has_focus()


def parse_clock(text):
    """Minutes after midnight in a typed time of day ('18:00', '18.00',
    '6:30 pm', '6pm', '18'), or None."""
    text = (text or "").strip().lower().replace(".", ":").replace(" ", "")
    match = re.fullmatch(r"(\d{1,2})(?::(\d{2}))?(am|pm)?", text)
    if not match:
        return None
    hour, minute = int(match.group(1)), int(match.group(2) or 0)
    if match.group(3):
        if not 1 <= hour <= 12:
            return None
        hour = hour % 12 + (12 if match.group(3) == "pm" else 0)
    if hour > 23 or minute > 59:
        return None
    return hour * 60 + minute


def top_align(box):
    """Keeps a row's widgets at the top, so a note under one field doesn't
    stretch the buttons beside it."""
    for child in box.get_children():
        child.set_valign(Gtk.Align.START)
    return box


class WrapBox(Gtk.Fixed):
    """Lays widgets out like words in a line of text (R-GUI-3): each at its
    natural width, a new line only when the next doesn't fit, none
    stretched sideways. (GTK 3's FlowBox lines its children up in columns
    and asks only for its widest child's width.) Hidden widgets take no
    place. Gtk.Fixed keeps the children, in C: a container written in
    Python (do_forall) hangs GTK when a destroyed one is finalized."""

    spacing = row_spacing = 6

    def __init__(self, spacing=6, row_spacing=6):
        super().__init__()
        self.spacing, self.row_spacing = spacing, row_spacing

    def do_get_request_mode(self):
        return Gtk.SizeRequestMode.HEIGHT_FOR_WIDTH

    def shown(self):
        return [widget for widget in self.get_children() if widget.get_visible()]

    def do_get_preferred_width(self):
        sizes = [widget.get_preferred_width() for widget in self.shown()]
        minimum = max((m for m, _n in sizes), default=0)
        natural = sum(n for _m, n in sizes) + self.spacing * max(len(sizes) - 1, 0)
        return minimum, max(minimum, natural)

    def do_get_preferred_width_for_height(self, _height):
        return self.do_get_preferred_width()

    def lines(self, width):
        """The lines at this width: [[widget, width, height]]."""
        lines, line, used = [], [], 0
        for widget in self.shown():
            minimum, natural = widget.get_preferred_width()
            cell = min(natural, max(width, minimum))
            if line and used + self.spacing + cell > width:
                lines.append(line)
                line, used = [], 0
            used += (self.spacing if line else 0) + cell
            line.append([widget, cell, 0])
        if line:
            lines.append(line)
        for line in lines:
            for item in line:
                item[2] = item[0].get_preferred_height_for_width(item[1])[1]
        return lines

    def height_for(self, width):
        lines = self.lines(width)
        return sum(max(h for _w, _c, h in line) for line in lines) + self.row_spacing * max(len(lines) - 1, 0)

    def do_get_preferred_height_for_width(self, width):
        height = self.height_for(width)
        return height, height

    def do_get_preferred_height(self):
        height = self.height_for(self.do_get_preferred_width()[1])
        return height, height

    def do_size_allocate(self, allocation):
        self.set_allocation(allocation)
        y = allocation.y
        for line in self.lines(allocation.width):
            line_height = max(h for _w, _c, h in line)
            x = allocation.x
            for widget, cell, _h in line:
                rect = Gdk.Rectangle()
                rect.x, rect.y, rect.width, rect.height = x, y, cell, line_height
                widget.size_allocate(rect)
                x += cell + self.spacing
            y += line_height + self.row_spacing


def flow_row(row):
    """Turns a row of widgets into one that wraps onto more lines when the
    window is narrow (R-GUI-3: works at 1024 pixels wide), in place, with
    the row's own spacing."""
    flow = WrapBox(spacing=row.get_spacing() or 6)
    flow.set_valign(Gtk.Align.START)
    for child in row.get_children():
        row.remove(child)
        flow.add(child)
    parent = row.get_parent()
    if parent is not None:
        index = parent.get_children().index(row)
        expand, fill, padding, pack = parent.query_child_packing(row) if isinstance(parent, Gtk.Box) else (False, False, 0, 0)
        parent.remove(row)
        if isinstance(parent, Gtk.Box):
            parent.pack_start(flow, expand, fill, padding)
            parent.reorder_child(flow, index)
        else:
            parent.add(flow)
    return flow


def guard_button(button, *fields):
    """Keeps button usable only while every field reads right."""
    def update(*_args):
        button.set_sensitive(all(field.is_valid() for field in fields))
    for field in fields:
        field.on_change(update)
    update()
    return update


def date_words(moment):
    """'Thursday 24 December 2099, 18:00' (by the clock preference)."""
    return f"{moment.strftime('%A')} {moment.day} {moment.strftime('%B %Y')}, " \
           f"{show_clock(moment.hour * 60 + moment.minute)}"


def until_words(moment, now=None):
    """How far a moment is: 'in 3 hours', 'in 73 years'."""
    now = now or datetime.datetime.now()
    seconds = (moment - now).total_seconds()
    if seconds < 0:
        return "already passed"
    days = seconds / 86400
    if days >= 730:
        return f"in {int(days // 365.25)} years"
    if days >= 60:
        return f"in {int(days // 30.4)} months"
    return "in " + duration_words(int(seconds) // 60 * 60 or 60)


class DateButton(Gtk.MenuButton):
    """A day: a button showing it that opens a calendar (§4.9 time frame)."""

    def __init__(self, day):
        super().__init__()
        self.callbacks = []
        inner = Gtk.Box(spacing=6)
        inner.pack_start(Gtk.Image.new_from_icon_name("x-office-calendar-symbolic", Gtk.IconSize.BUTTON),
                         False, False, 0)
        self.label = Gtk.Label()
        inner.pack_start(self.label, False, False, 0)
        self.add(inner)
        popover = Gtk.Popover()
        self.calendar = Gtk.Calendar(margin=8)
        self.calendar.show()
        popover.add(self.calendar)
        self.set_popover(popover)
        self.day = day
        self.set_day(day)
        self.calendar.connect("day-selected", self.picked)
        self.calendar.connect("day-selected-double-click", lambda *_a: popover.popdown())

    def set_day(self, day):
        self.calendar.select_month(day.month - 1, day.year)
        self.calendar.select_day(day.day)
        self.picked(self.calendar)

    def picked(self, calendar):
        year, month, day = calendar.get_date()
        self.day = datetime.date(year, month + 1, day)
        self.label.set_text(self.day.strftime("%-d %b %Y"))
        for callback in self.callbacks:
            callback()

    def on_change(self, callback):
        self.callbacks.append(callback)


def stats_range_problem(first, last, today):
    """Why a time frame can't be shown (R-GUI-1: say why), or None."""
    if first > last:
        return "The first day is after the last."
    if last > today:
        return "The last day is still to come: statistics reach today."
    oldest = today - datetime.timedelta(days=STATS_KEEP_DAYS - 1)
    if first < oldest:
        return f"Statistics are kept for a year: the first day can be {oldest.strftime('%-d %b %Y')} at the earliest."
    return None


def range_words(first, last):
    """A time frame in words: 2 – 16 Nov 2026, 28 Oct – 3 Nov 2026."""
    if first == last:
        return first.strftime("%-d %b %Y")
    if first.year != last.year:
        return f"{first.strftime('%-d %b %Y')} – {last.strftime('%-d %b %Y')}"
    if first.month != last.month:
        return f"{first.strftime('%-d %b')} – {last.strftime('%-d %b %Y')}"
    return f"{first.day} – {last.strftime('%-d %b %Y')}"


STATS_KEEP_DAYS = 365  # enforce.StatsKeepDays


class TimerLockField(Gtk.Box):
    """R-GUI-2: no timer lock, one for a length (R-GUI-1), or one until a
    date and time picked on a calendar. flags() gives --for or --until."""

    MODES = (("none", "No timer"), ("for", "For a time"), ("until", "Until a date"))

    def __init__(self, allow_none=True, mode=None, seconds=3600):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.callbacks = []
        switcher = Gtk.Box()
        switcher.get_style_context().add_class("linked")
        self.mode_buttons, group = {}, None
        for key, label in self.MODES:
            if key == "none" and not allow_none:
                continue
            button = Gtk.RadioButton.new_with_label_from_widget(group, label)
            button.set_mode(False)  # drawn as a toggle button
            group = group or button
            button.connect("toggled", self.mode_toggled, key)
            switcher.pack_start(button, False, False, 0)
            self.mode_buttons[key] = button
        self.pack_start(switcher, False, False, 0)
        self.stack = Gtk.Stack(vhomogeneous=False, hhomogeneous=False,
                               transition_type=Gtk.StackTransitionType.NONE)
        self.pack_start(self.stack, False, False, 0)
        self.stack.add_named(Gtk.Box(), "none")

        self.duration = DurationField(seconds, minimum=60, presets=(3600, 86400, 7 * 86400, 30 * 86400))
        self.duration.on_change(self.changed)
        self.stack.add_named(self.duration, "for")

        now = datetime.datetime.now()
        start = (now + datetime.timedelta(days=1)).replace(second=0, microsecond=0)
        self.day = start.date()
        until = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        line = Gtk.Box(spacing=6)
        self.date_button = Gtk.MenuButton()
        self.date_label = Gtk.Label()
        inner = Gtk.Box(spacing=6)
        inner.pack_start(Gtk.Image.new_from_icon_name("x-office-calendar-symbolic", Gtk.IconSize.BUTTON),
                         False, False, 0)
        inner.pack_start(self.date_label, False, False, 0)
        self.date_button.add(inner)
        popover = Gtk.Popover()
        pop = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6, border_width=8)
        self.calendar = Gtk.Calendar()
        self.calendar.select_month(self.day.month - 1, self.day.year)
        self.calendar.select_day(self.day.day)
        jump = Gtk.Box(spacing=6)
        jump.pack_start(Gtk.Label(label="Year"), False, False, 0)
        self.year = Gtk.SpinButton.new_with_range(now.year, 9999, 1)
        self.year.set_value(self.day.year)
        jump.pack_start(self.year, False, False, 0)
        pop.pack_start(self.calendar, False, False, 0)
        pop.pack_start(jump, False, False, 0)
        pop.show_all()
        popover.add(pop)
        self.date_button.set_popover(popover)
        self.calendar.connect("day-selected", self.day_picked)
        self.calendar.connect("day-selected-double-click", lambda *_a: popover.popdown())
        self.year.connect("value-changed", self.year_picked)
        line.pack_start(self.date_button, False, False, 0)
        line.pack_start(Gtk.Label(label="at"), False, False, 0)
        self.time_entry = Gtk.Entry(width_chars=8, text=show_clock(start.hour * 60), activates_default=True)
        self.time_entry.connect("changed", self.changed)
        line.pack_start(self.time_entry, False, False, 0)
        until.pack_start(line, False, False, 0)
        self.until_note = Gtk.Label(xalign=0, wrap=True)
        self.until_note.get_style_context().add_class("kc9k-note")
        until.pack_start(self.until_note, False, False, 0)
        self.stack.add_named(until, "until")

        first = mode or ("none" if allow_none else "for")
        self.mode = first
        self.mode_buttons[first].set_active(True)
        self.stack.set_visible_child_name(first)
        self.changed()

    # The date ------------------------------------------------------------
    def day_picked(self, calendar):
        year, month, day = calendar.get_date()
        self.day = datetime.date(year, month + 1, day)
        if int(self.year.get_value()) != year and year >= self.year.get_adjustment().get_lower():
            self.year.set_value(year)
        self.changed()

    def year_picked(self, spin):
        year = int(spin.get_value())
        month = self.day.month
        day = min(self.day.day, 28 if month == 2 else 30 if month in (4, 6, 9, 11) else 31)
        if month == 2 and day == 28 and (year % 4 == 0 and (year % 100 or year % 400 == 0)) and self.day.day == 29:
            day = 29
        self.calendar.select_month(month - 1, year)
        self.calendar.select_day(day)

    def set_until(self, moment):
        """Picks a date and time (for tests and presets)."""
        self.mode_buttons["until"].set_active(True)
        self.calendar.select_month(moment.month - 1, moment.year)
        self.calendar.select_day(moment.day)
        self.day = moment.date()
        self.time_entry.set_text(show_clock(moment.hour * 60 + moment.minute))
        self.changed()

    def until(self):
        """The picked moment, or None while the time can't be read."""
        minute = parse_clock(self.time_entry.get_text())
        if minute is None:
            return None
        return datetime.datetime(self.day.year, self.day.month, self.day.day, minute // 60, minute % 60)

    # State ---------------------------------------------------------------
    def mode_toggled(self, button, key):
        if button.get_active():
            self.mode = key
            self.stack.set_visible_child_name(key)
            self.changed()

    def on_change(self, callback):
        self.callbacks.append(callback)

    def problem(self):
        if self.mode == "for":
            return self.duration.problem
        if self.mode == "until":
            moment = self.until()
            if moment is None:
                return "Type a time of day such as 18:00."
            if moment <= datetime.datetime.now():
                return "That moment has passed: pick a later date or time."
        return None

    def is_valid(self):
        return self.problem() is None

    def changed(self, *_args):
        moment = self.until()
        self.date_label.set_text(f"{self.day.strftime('%a')} {self.day.day} {self.day.strftime('%b %Y')}")
        problem = self.problem() if self.mode == "until" else None
        mark_problem(self.time_entry, problem if moment is None else None)
        mark_problem(self.until_note, problem)
        if problem:
            self.until_note.set_text(problem)
        elif moment:
            self.until_note.set_text(f"Until {date_words(moment)}, {until_words(moment)}.")
        for callback in self.callbacks:
            callback(self)

    def flags(self):
        if self.mode == "for" and self.duration.value:
            return ["--for", self.duration.arg()]
        if self.mode == "until" and self.until():
            return ["--until", self.until().strftime("%Y-%m-%d %H:%M")]
        return []

    def describe(self):
        """'timer lock for 2 days 4 hours' or 'timer lock until Thursday
        24 December 2099, 18:00'."""
        if self.mode == "for" and self.duration.value:
            return f"timer lock for {duration_words(self.duration.value)}"
        if self.mode == "until" and self.until():
            return f"timer lock until {date_words(self.until())}"
        return ""


WEEKDAYS = (("mon", "Mon"), ("tue", "Tue"), ("wed", "Wed"), ("thu", "Thu"), ("fri", "Fri"),
            ("sat", "Sat"), ("sun", "Sun"))


def relock_lines(target, relock_flags):
    """A block's re-lock as the lock confirmation shows it (R-LOCK-12)."""
    if not relock_flags:
        return []
    return [f"{target}: re-lock, all its locks come back, in their order, {relock_flags[1]} after it is unlocked"]


class LockPicker(Gtk.Box):
    """Every lock kind for a block or a setting (§3.4), the timer lock
    first (R-GUI-2), the rest behind More locks. chosen() gives the locks
    as (flags, words) in the order the dialog has always used (timer,
    restart, password, text, delay, range); relock_flags() the re-lock.
    kinds limits what is offered (an active block takes only a longer
    timer lock or a longer text lock of random characters)."""

    ALL_KINDS = ("timer", "password", "text", "restart", "delay", "range", "relock")

    def __init__(self, verb="stop", done="stopped", kinds=ALL_KINDS, relock_any=False):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        self.callbacks, self.verb, self.done, self.kinds = [], verb, done, tuple(kinds)
        # §3.4: a block's re-lock brings back all its locks, so it goes with
        # any lock; a setting's re-lock only with a password or text lock.
        self.relock_any = relock_any
        self.only_chars = "relock" not in self.kinds and "password" not in self.kinds
        grid = Gtk.Grid(column_spacing=16, row_spacing=6)
        self.pack_start(grid, False, False, 0)
        titles = Gtk.SizeGroup(mode=Gtk.SizeGroupMode.HORIZONTAL)  # one label column for both grids

        def label(text, row, target):
            title = Gtk.Label(label=text, xalign=0, yalign=0, margin_top=6)
            title.get_style_context().add_class("kc9k-field-label")
            titles.add_widget(title)
            target.attach(title, 0, row, 1, 1)

        label("Timer lock", 0, grid)
        self.timer = TimerLockField()
        self.timer.on_change(self.changed)
        grid.attach(self.timer, 1, 0, 1, 1)
        grid.attach(dim(f"Can't be {done} at all until then. Time the computer is off doesn't count."),
                    1, 1, 1, 1)

        others = [k for k in ("password", "text", "restart", "delay", "range") if k in self.kinds]
        self.more = Gtk.Expander(label="More locks: " + ", ".join(others) if others else "")
        more = Gtk.Grid(column_spacing=16, row_spacing=6, margin_top=8, margin_start=0)
        self.more.add(more)
        if others:
            self.pack_start(self.more, False, False, 0)
        row = 0

        self.password = Gtk.Entry(visibility=False, placeholder_text="Password", width_chars=16)
        self.password_again = Gtk.Entry(visibility=False, placeholder_text="Repeat it", width_chars=16)
        for entry in (self.password, self.password_again):
            entry.connect("changed", self.changed)
        if "password" in self.kinds:
            label("Password lock", row, more)
            pw = Gtk.Box(spacing=6)
            pw.pack_start(self.password, False, False, 0)
            pw.pack_start(self.password_again, False, False, 0)
            more.attach(pw, 1, row, 1, 1)
            more.attach(dim(f"Needed to {verb} it. Give it to someone else to keep."), 1, row + 1, 1, 1)
            row += 2

        self.typing = Gtk.CheckButton(label="Type a text to unlock")
        self.text_style = Gtk.ComboBoxText()
        styles = (("chars", "Random characters"),) if self.only_chars else \
            (("chars", "Random characters"), ("words", "Random words"), ("custom", "My own text"))
        for key, title in styles:
            self.text_style.append(key, title)
        self.text_style.set_active_id("chars")
        self.chars = NumberField(CHALLENGE_DEFAULT, CHALLENGE_MIN, CHALLENGE_MAX, width=6)
        self.custom_text = Gtk.Entry(placeholder_text="The text to type", width_chars=24, no_show_all=True)
        self.typing.connect("toggled", self.changed)
        self.text_style.connect("changed", self.changed)
        self.chars.on_change(self.changed)
        self.custom_text.connect("changed", self.changed)
        if "text" in self.kinds:
            label("Text lock", row, more)
            text_line = Gtk.Box(spacing=6)
            for widget in (self.typing, self.text_style, self.chars, self.custom_text):
                widget.set_valign(Gtk.Align.START)
                text_line.pack_start(widget, False, False, 0)
            more.attach(text_line, 1, row, 1, 1)
            more.attach(dim("Length in characters: about 300 take a few minutes to type, 5000 about an "
                            "hour. Pasting is refused." + (" A text lock already on can only get longer."
                                                           if self.only_chars else "")), 1, row + 1, 1, 1)
            row += 2

        self.restart = Gtk.CheckButton(label="Stays on until the computer restarts")
        self.restart.connect("toggled", self.changed)
        if "restart" in self.kinds:
            label("Restart lock", row, more)
            more.attach(self.restart, 1, row, 1, 1)
            row += 1

        self.delay = None
        if "delay" in self.kinds:
            label("Delay lock", row, more)
            self.delay = DurationField(0, minimum=60, maximum=40 * 86400, zero="No delay lock",
                                       presets=(3600, 86400))
            self.delay.on_change(self.changed)
            more.attach(self.delay, 1, row, 1, 1)
            more.attach(dim("Unlocking only takes effect after this long."), 1, row + 1, 1, 1)
            row += 2

        self.range_on = Gtk.CheckButton(label="On at set times on set days")
        self.range_on.connect("toggled", self.changed)
        days = Gtk.Box()
        days.get_style_context().add_class("linked")
        self.range_days = {}
        order = WEEKDAYS[FIRST_DAY:] + WEEKDAYS[:FIRST_DAY] if FIRST_DAY else WEEKDAYS
        for key, title in order:
            toggle = Gtk.ToggleButton(label=title, active=key not in ("sat", "sun"))
            toggle.connect("toggled", self.changed)
            days.pack_start(toggle, False, False, 0)
            self.range_days[key] = toggle
        times = Gtk.Box(spacing=6)
        self.range_from = Gtk.Entry(width_chars=7, text=show_clock(9 * 60))
        self.range_to = Gtk.Entry(width_chars=7, text=show_clock(17 * 60))
        self.range_invert = Gtk.CheckButton(label="outside these times instead")
        for widget in (Gtk.Label(label="from"), self.range_from, Gtk.Label(label="to"), self.range_to,
                       self.range_invert):
            times.pack_start(widget, False, False, 0)
        for widget in (self.range_from, self.range_to):
            widget.connect("changed", self.changed)
        self.range_invert.connect("toggled", self.changed)
        self.range_widgets = (days, times)
        if "range" in self.kinds:
            label("Range lock", row, more)
            range_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
            range_box.pack_start(self.range_on, False, False, 0)
            range_box.pack_start(days, False, False, 0)
            range_box.pack_start(times, False, False, 0)
            more.attach(range_box, 1, row, 1, 1)
            more.attach(dim("Can't be unlocked at these times, only waited out."), 1, row + 1, 1, 1)
            row += 2

        self.relock = DurationField(0, minimum=60, maximum=30 * 86400, zero="No re-lock",
                                    presets=(1800, 3600))
        self.relock.on_change(self.changed)
        if "relock" in self.kinds:
            label("Re-lock", row, more)
            more.attach(self.relock, 1, row, 1, 1)
            more.attach(dim("This long after it is unlocked, all its locks come on again, in their order."
                            if relock_any else
                            "With a password or text lock: unlocking lifts them only this long; then they "
                            "come on again."), 1, row + 1, 1, 1)

        self.problem_label = Gtk.Label(xalign=0, wrap=True, no_show_all=True)
        self.problem_label.get_style_context().add_class("kc9k-note")
        self.problem_label.get_style_context().add_class("error")
        self.pack_start(self.problem_label, False, False, 0)
        self.changed()

    # Reading -------------------------------------------------------------
    def on_change(self, callback):
        self.callbacks.append(callback)

    def range_spec(self):
        """'mon,tue,wed 09:00-17:00', or None while it can't be read."""
        days = [key for key, _title in WEEKDAYS if self.range_days[key].get_active()]
        start, end = parse_clock(self.range_from.get_text()), parse_clock(self.range_to.get_text())
        if not days or start is None or end is None or start == end:
            return None
        return f"{','.join(days)} {clock_text(start)}-{clock_text(end)}"

    def problem(self):
        if not self.timer.is_valid():
            return self.timer.problem()
        if self.password.get_text() != self.password_again.get_text():
            return "The two passwords differ."
        if self.typing.get_active():
            if self.text_style.get_active_id() == "custom" and not self.custom_text.get_text().strip():
                return "Type the text for the text lock."
            if self.text_style.get_active_id() != "custom" and not self.chars.is_valid():
                return "Text lock: " + self.chars.problem
        if self.delay is not None and not self.delay.is_valid():
            return "Delay lock: " + self.delay.problem
        if self.range_on.get_active() and self.range_spec() is None:
            if not any(t.get_active() for t in self.range_days.values()):
                return "Range lock: pick at least one day."
            return "Range lock: two different times of day, such as 09:00 and 17:00."
        if not self.relock.is_valid():
            return "Re-lock: " + self.relock.problem
        return None

    def is_valid(self):
        return self.problem() is None

    def changed(self, *_args):
        custom = self.text_style.get_active_id() == "custom"
        on = self.typing.get_active()
        self.text_style.set_sensitive(on)
        self.chars.set_visible(not custom)
        self.chars.set_sensitive(on)
        self.custom_text.set_visible(custom)
        self.custom_text.set_sensitive(on)
        for widget in self.range_widgets:
            widget.set_sensitive(self.range_on.get_active())
        self.relock.set_sensitive(self.relock_allowed())
        mark_problem(self.password_again, self.password.get_text() != self.password_again.get_text())
        problem = self.problem()
        self.problem_label.set_text(problem or "")
        self.problem_label.set_visible(bool(problem))
        for callback in self.callbacks:
            callback(self)

    def secret(self):
        return self.password.get_text()

    def text_flags(self):
        style = self.text_style.get_active_id()
        if style == "custom":
            return ["--lock-text", self.custom_text.get_text()]
        args = ["--lock", "--lock-chars", str(self.chars.value)]
        return args + ["--lock-words"] if style == "words" else args

    def relock_allowed(self):
        """Whether re-lock goes with the locks chosen."""
        if self.typing.get_active() or self.secret():
            return True
        return self.relock_any and bool(
            self.timer.flags() or self.restart.get_active() or (self.delay is not None and self.delay.value)
            or (self.range_on.get_active() and self.range_spec()))

    def relock_seconds(self):
        return self.relock.value if self.relock.value and self.relock_allowed() else 0

    def relock_flags(self):
        seconds = self.relock_seconds()
        return ["--relock", f"{seconds // 60}m"] if seconds else []

    def chosen(self):
        """The locks asked for, as (flags, words), in the dialog's order."""
        relock = self.relock_seconds()
        relock_note = f", re-lock {duration_words(relock)} after unlocking" if relock and not self.relock_any else ""
        out = []
        if self.timer.flags():
            out.append((self.timer.flags(), self.timer.describe() +
                        f" (it can't be {self.done} until then, "
                        "not even with a text lock; time the computer is off doesn't count)"))
        if self.restart.get_active():
            out.append((["--restart"], "restart lock, until the computer restarts"))
        if self.secret():
            out.append((["--password-stdin"], f"password lock{relock_note}"))
        if self.typing.get_active():
            style = self.text_style.get_active_id()
            what = {"chars": f"{self.chars.value} random characters", "words": f"{self.chars.value} characters of words",
                    "custom": "your own text"}[style]
            out.append((self.text_flags(), f"text lock ({what}){relock_note}"))
        if self.delay is not None and self.delay.value:
            out.append((["--delay", self.delay.arg()], f"delay lock, unlocking takes {duration_words(self.delay.value)}"))
        if self.range_on.get_active() and self.range_spec():
            spec, outside = self.range_spec(), self.range_invert.get_active()
            out.append((["--range", spec] + (["--range-invert"] if outside else []),
                        f"range lock, {'outside ' if outside else ''}{spec}"))
        return out


def waiting_words(chosen, before=()):
    """R-LOCK-6, R-LOCK-12: behind a lock other than a timer lock (one
    chosen above it, or one of before, the kinds already on), a timer lock
    for a time counts from when the locks before it are unlocked and a
    restart lock needs a restart after them; their confirmation lines say
    so, as the CLI's do."""
    others = any(kind != "timer" for kind in before)
    out = []
    for flags, text in chosen:
        if others and flags[:1] == ["--for"]:
            text = (text.split(" (")[0] + ", counting from when the locks before it are unlocked "
                    "(time the computer is off doesn't count)")
        elif others and flags[:1] == ["--restart"]:
            text = "restart lock, until a restart after the locks before it are unlocked"
        others = others or flags[:1] not in (["--for"], ["--until"])
        out.append((flags, text))
    return out


class HotkeyFields(Gtk.Grid):
    """A new hotkey's keys and the block it starts, checked as they are
    typed (R-GUI-3). values is (keys, block), kept current, so it can be
    read after the dialog is gone."""

    def __init__(self, blocks, selected=None):
        super().__init__(column_spacing=16, row_spacing=6)
        self.callbacks = []
        for row, text in enumerate(("Keys", "Starts")):
            title = Gtk.Label(label=text, xalign=0)
            title.get_style_context().add_class("kc9k-field-label")
            self.attach(title, 0, row * 2, 1, 1)
        self.keys = Gtk.Entry(placeholder_text="ctrl+alt+s", width_chars=18, activates_default=True)
        self.keys.connect("changed", self.changed)
        self.attach(self.keys, 1, 0, 1, 1)
        self.note = Gtk.Label(xalign=0, wrap=True)
        self.note.get_style_context().add_class("kc9k-note")
        self.attach(self.note, 1, 1, 1, 1)
        self.block = Gtk.ComboBoxText()
        for name in blocks:
            self.block.append(name, name)
        self.block.set_active_id(selected if selected in blocks else blocks[0])
        self.block.connect("changed", self.changed)
        self.attach(self.block, 1, 2, 1, 1)
        self.values = ("", "")
        self.changed()

    def on_change(self, callback):
        self.callbacks.append(callback)

    def problem(self):
        return hotkey_problem(self.keys.get_text())

    def changed(self, *_args):
        self.values = (self.keys.get_text().strip(), self.block.get_active_id() or "")
        typed = bool(self.keys.get_text().strip())
        problem = self.problem()
        mark_problem(self.keys, problem if typed else None)
        mark_problem(self.note, problem if typed else None)
        self.note.set_text(problem if typed and problem else
                           "ctrl, alt, shift or super with a key; F1 to F24 also work alone.")
        for callback in self.callbacks:
            callback(self)
