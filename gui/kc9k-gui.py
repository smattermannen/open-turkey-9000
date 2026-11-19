#!/usr/bin/env python3
"""Graphical interface for Killer Chicken 9000 (GTK 3).

All changes go through the kc9k command, so the interface follows the
same lock rules as the terminal and needs no privileges of its own.

Pages: Overview, Blocks, Clusters, Frozen Chicken (whole-computer locks),
Statistics, Circadian rhythm and Settings.
"""
import datetime
import json
import math
import os
import re
import shlex
import subprocess
import threading

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
gi.require_version("PangoCairo", "1.0")
from gi.repository import Gdk, Gio, GLib, Gtk, Pango, PangoCairo  # noqa: E402

CLI = "kc9k"
REFRESH_SECONDS = 5
STATS_REFRESH_SECONDS = 30
# Pointer events for the charts' hover.
POINTER_EVENTS = Gdk.EventMask.POINTER_MOTION_MASK | Gdk.EventMask.LEAVE_NOTIFY_MASK

# Schedules added with one click: days, from, to.
SCHEDULE_PRESETS = (
    ("Work hours", "weekdays", "09:00", "17:00"),
    ("Evenings", "daily", "18:00", "23:00"),
    ("Weekends", "sat,sun", "00:00", "24:00"),
    ("Nights", "daily", "00:00", "06:00"),
)

# §4.2 (owner 2026-11-17): a block card dragged onto a mode.
BLOCK_TARGET = Gtk.TargetEntry.new("kc9k/block", Gtk.TargetFlags.SAME_APP, 0)

# §3.8, §4.8 (owner 2026-11-17): Frozen Chicken is a mode (db.FrozenChickenName),
# up to a week (R-FREEZE-3).
FROZEN_CHICKEN = "Frozen Chicken"
FREEZE_MAX = 7 * 86400

# §4.3 (owner 2026-11-17): an opened block is always a draft. These commands
# change what its Websites and Apps tabs show; run on the block opened on the
# Blocks page they go to its draft (made at the first change), and apply
# only with Save edits. For each: where the block's name is in the command.
DRAFT_EDITS = {
    "block": {sub: 2 for sub in ("add", "add-exception", "add-term", "add-keyword", "add-pattern",
                                 "remove-site", "remove-entry", "remove-exception", "threshold", "action",
                                 "add-app", "remove-app", "add-title", "remove-title", "import-list",
                                 "trigger")},
    "list": {sub: 2 for sub in ("create", "rename", "delete", "add", "remove", "scope", "where", "action",
                                "threshold", "hits", "rate", "test", "import")} | {"copy": 4},
    "phrase-set": {sub: 2 for sub in ("create", "add", "remove", "change", "scope", "action", "threshold",
                                      "test", "rename", "delete", "import")},
    "social": 1,
}
# Reading what those tabs show: the draft's, when there is one.
DRAFT_READS = {"block": {"export-list": 2}, "list": {"export": 2, "show": 2},
               "phrase-set": {"export": 2, "show": 2}}


def draft_position(args, table):
    """Where a block's name is in args by table (DRAFT_EDITS or DRAFT_READS),
    or None."""
    # Presets of lists (owner 2026-11-17): used into, or saved from, what the
    # Websites tab shows.
    if args[:2] == ["preset", "use"] and "--kind" in args and args[args.index("--kind") + 1:][:1] == ["lists"]:
        return 3 if table is DRAFT_EDITS else None
    if args[:3] == ["preset", "save", "lists"]:
        return 3 if table is DRAFT_READS else None
    if not args or args[0] not in table:
        return None
    where = table[args[0]]
    if isinstance(where, int):
        return where
    return where.get(args[1]) if len(args) > 1 else None


def draft_of_name(block):
    """The name of a block's draft (cli draftName)."""
    return block + " (draft)"

# The switches of 'kc9k option' (internal/enforce/options.go).
OPTIONS = (
    ("polite-close", "Ask blocked apps to close first",
     "A blocked app with a window is asked to close, so it can offer to save, and is "
     "closed for good a few seconds later if it is still open. Off: closed at once."),
    ("block-page-details", "Say why on the block page",
     "The block page names the block and list that stopped the page, and when the block "
     "ends. It changes nothing about what a list does: redirects, pictures and closed tabs "
     "work as before."),
)

# Browser settings (SPEC §4.10), in the order SPEC lists them.
BROWSER_OPTIONS = (
    ("safesearch", "Force Google SafeSearch", ""),
    ("force-search", "Force Google as the search engine", ""),
    ("web-only", "Web results only",
     "No image, shopping or AI results in Google."),
    ("block-devtools", "Block developer tools",
     "In LibreWolf they can switch the extension off. Turn this off only to develop websites; "
     "it applies when LibreWolf restarts. Chrome never lets them reach the extension."),
    ("no-private", "Disable private windows",
     "No private browsing in LibreWolf; it applies when LibreWolf restarts. Chrome's incognito "
     "windows are always off: the extension can't run in them."),
)

# Tamper protection settings (SPEC §4.10) that have a command: key, title,
# detail, command prefix ("on"/"off" is appended). Block unsupported browsers
# replaced the browser setting force-librewolf (decision 2026-11-14) and has
# its own lock.
TAMPER_SETTINGS = (
    ("clock", "Clock: ignore time and date changes",
     "While a timer lock runs, changing the date or time doesn't move it, and time the "
     "computer is off isn't counted.", ("tamper", "clock")),
    ("task-managers", "Block task managers while locked",
     "While a block or setting is locked, System Monitor, htop and other task managers are closed.",
     ("tamper", "task-managers")),
    ("private", "Close private windows and Tor",
     "Private windows are turned off in every browser (open ones close when it restarts) and "
     "Tor Browser is closed.", ("tamper", "private")),
    ("file-urls", "Allow file:// blocking",
     "Blacklist entries such as file:///home/me/games/* block local files in the browsers. "
     "Each block's own file:// exceptions (file://* by default) still spare them.",
     ("tamper", "file-urls")),
    ("unsupported-browsers", "Block unsupported browsers",
     "Only LibreWolf, Google Chrome and Chromium running the Killer Chicken extension stay "
     "open, even when no block is on. Every other browser, Firefox included (it can't run the "
     "extension), is closed at once; a supported browser without the extension a minute "
     "after it starts. Needs a supported browser installed.",
     ("tamper", "unsupported-browsers")),
)

# Install notes for the supported browsers (SPEC §3.13), by ID.
BROWSER_INSTALL_NOTES = {
    "librewolf": "It is installed from LibreWolf's official repository through extrepo.",
    "chromium": "It is installed from Linux Mint's own repository.",
    "chrome": "Google's repository and its signing key are added, and Chrome is installed from it; "
              "it then updates with the rest of the system.",
}

# What a blacklist does to the pages it matches; any combination.
SITE_ACTIONS = (
    ("block", "Block page"),
    ("greyscale", "Greyscale"),
    ("no-images", "Block images"),
    ("mute", "Mute sound"),
    ("no-video", "Stop and hide video"),
    ("dark", "Dark page"),
    ("no-downloads", "Cancel downloads"),
    ("redirect", "Open another page"),
    ("image", "Show a picture"),
    ("close-tab", "Close the tab"),
    ("close-browser", "Close the browser"),
    ("logout", "Log out"),
    ("shutdown", "Shut down"),
    ("monk", "Monk mode"),
    ("mode", "Run a mode, locked"),
    ("custom", "Custom action"),
)

# Actions with settings: what to ask for when one is ticked.
PARAM_ACTIONS = ("redirect", "image", "logout", "shutdown", "monk", "mode", "custom")

# What bypass prevention can do, mildest first.
CONSEQUENCES = (
    ("warning", "Show a warning"),
    ("close-tab", "Close the tab that tripped it"),
    ("close-browser", "Close the browser"),
    ("lock-screen", "Lock the screen"),
    ("logout", "Log out"),
    ("shutdown", "Shut down"),
    ("monk", "Monk mode"),
    ("mode", "Run a mode, locked, for"),
    ("custom", "Custom action"),
)


def fmt_minutes(minutes):
    """30 min, 2 h, 1 h 30 min."""
    hours, rest = divmod(int(minutes), 60)
    if not hours:
        return f"{rest} min"
    return f"{hours} h" + (f" {rest} min" if rest else "")


# What bypass detection can count (db.Detectors).
DETECTORS = (
    ("page", "Blocked pages"),
    ("search", "Blocked pages opened from a search"),
    ("app", "Closed apps"),
    ("title", "Windows closed for their title"),
    ("browser", "Browsers closed for lacking the extension"),
    ("workaround", "Sites reached another way (proxies, mirrors, download tools)"),
    ("tamper", "Tampering (daemon restarted, clock moved)"),
    ("access", "Wrong access passwords"),
)
# Consequences that take a time, and whether they need one.
TIMED = {"lock-screen": True, "logout": False, "shutdown": False, "monk": True, "mode": True}
NEPSIS_MAX_COUNT = 1000  # db.MaxNepsisCount


def describe_step(step):
    actions = []
    for action in step["actions"]:
        if action["action"] == "mode":
            text = f"run {action.get('param', '')}, locked, for {fmt_minutes(action.get('minutes', 0))}"
        elif action["action"] == "custom":
            try:
                text = "; ".join(f"{list(i)[0]} {list(i.values())[0]}" for i in json.loads(action.get("param") or "[]"))
            except (ValueError, IndexError, AttributeError):
                text = "a custom action"
        else:
            text = dict(CONSEQUENCES).get(action["action"], action["action"]).lower()
            if action.get("minutes"):
                text += f" for {fmt_minutes(action['minutes'])}"
        actions.append(text)
    return f"{step['count']} stopped things within {fmt_minutes(step['minutes'])}: {', '.join(actions)}."


def describe_nepsis(nepsis, count=None):
    """A bypass prevention setting in words: one line per step of its ladder."""
    steps = (nepsis or {}).get("steps") or []
    if not steps:
        return "Off."
    if len(steps) == 1:
        text = describe_step(steps[0])
    else:
        text = "\n".join(f"Step {n}: {describe_step(step)}" for n, step in enumerate(steps, start=1))
    quiet = (nepsis or {}).get("quiet_minutes")
    if quiet and len(steps) > 1:
        text += f"\nFalls back a step after {fmt_minutes(quiet)} without tripping."
    if count is not None:
        text += f" {count} so far."
    return text

# Where a list looks.
# A list's scope boxes (SPEC §3.2).
LIST_PLACES = (
    ("url", "URL"),
    ("title", "Title"),
    ("text", "Text"),
)
# §3.9 ad trigger: blacklists and clusters can also look at the ad networks
# a page loads (entries: ad network domains, or adult, crypto, dating,
# gambling). Not phrase sets or exception lists.
AD_PLACE = ("ads", "Ads")


# Helpers -----------------------------------------------------------------

def describe_setting_lock(info):
    """The parts of a setting lock that hold, as words."""
    parts = []
    if info.get("lock_seconds"):
        parts.append(f"for {fmt_seconds(info['lock_seconds'])}")
    if info.get("restart_locked"):
        parts.append("until restart")
    if info.get("char_locked"):
        parts.append("text lock")
    if info.get("password_locked"):
        parts.append("password")
    if info.get("timer_after"):
        parts.append(f"timer {fmt_seconds(info['timer_after'])} after the locks before it")
    if info.get("relock") and (info.get("char_locked") or info.get("password_locked")):
        parts.append(f"re-lock {fmt_seconds(info['relock'])} after unlocking")
    if len(info.get("lock_order") or []) > 1:
        parts.append("unlock order: " + " → ".join(LOCK_KIND_NAMES.get(k, k) for k in info["lock_order"]))
    return parts


def browser_status(browser):
    """A supported browser's line in the Browsers list (gui-state 'browsers')."""
    if not browser.get("installed"):
        return "Not installed."
    seconds = browser.get("checkin_seconds", -1)
    if seconds < 0:
        return "Installed. Extension not seen yet: open it once."
    if seconds < 120:
        return "Installed. Extension working."
    if seconds < 2 * 86400:
        return f"Installed. Extension last seen {fmt_seconds(seconds)} ago."
    return f"Installed. Extension last seen {seconds // 86400} days ago."


def fmt_seconds(seconds):
    """A remaining time, rounded to minutes: '69 days 10 h', '2 h 5 min',
    '3 min' (days past a day, so a long timer lock reads as one)."""
    seconds = int(seconds)
    if seconds >= 86400:
        days, rest = divmod(seconds, 86400)
        hours = rest // 3600
        return f"{days} day{'s' if days != 1 else ''}" + (f" {hours} h" if hours else "")
    hours, rest = divmod(seconds, 3600)
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


def cli_with_access(passwords, args, secret=None, timeout=120):
    """run_cli with the access passwords given so far as the first lines of
    standard input, announced with --access-lines."""
    if not passwords:
        return run_cli(args, secret, timeout)
    return run_cli(["--access-lines", str(len(passwords)), *args],
                   "".join(p + "\n" for p in passwords) + (secret or ""), timeout)


AREA_TITLES = {"app": "Killer Chicken 9000", "dashboard": "Overview", "blocks": "Blocks",
               "device": "Frozen Chicken", "stats": "Statistics", "settings": "Settings"}
AREAS = list(AREA_TITLES)


def heading(text, size="large"):
    label = Gtk.Label(xalign=0)
    label.set_markup(f"<span size='{size}' weight='bold'>{GLib.markup_escape_text(text)}</span>")
    return label


def dim(text, wrap=True):
    label = Gtk.Label(label=text, xalign=0, wrap=wrap)
    label.get_style_context().add_class("dim-label")
    return label


CHALLENGE_MIN, CHALLENGE_MAX, CHALLENGE_DEFAULT = 1, 5000, 300

# Appearance is a preference of whoever runs the interface, so it lives in
# their own config, not in the KC9K database.
PREFS_PATH = os.path.join(os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config"),
                          "kc9k", "gui.json")
THEMES = (("system", "Same as the system"), ("light", "Light"), ("dark", "Dark"))
# §4.10 Wallpaper default (owner 2026-11-17), as `kc9k wallpaper default` takes it.
WALLPAPER_DEFAULTS = (("latest", "Latest wallpaper"), ("circadian", "Circadian rhythm"), ("custom", "Custom"))
THEME_DIRS = ("/usr/share/themes", os.path.expanduser("~/.themes"),
              os.path.expanduser("~/.local/share/themes"))


LOGIN_DESKTOP = os.path.join(os.environ.get("XDG_CONFIG_HOME") or os.path.expanduser("~/.config"),
                             "autostart", "kc9k-gui.desktop")


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

    def apply(self, choice, darkness=None):
        name, dark = self.system if choice not in ("light", "dark") else themed(self.system[0], choice == "dark")
        self.settings.props.gtk_theme_name = name
        self.settings.props.gtk_application_prefer_dark_theme = dark
        self.set_darkness(darkness if choice == "dark" else None)

    def set_darkness(self, darkness):
        """§4.10: with the dark theme, the background from dark grey (0) to
        black (100); None leaves the theme's own."""
        screen = Gdk.Screen.get_default()
        if screen is None:
            return
        if getattr(self, "css", None) is not None:
            Gtk.StyleContext.remove_provider_for_screen(screen, self.css)
            self.css = None
        if darkness is None:
            return
        level = round(48 * (100 - max(0, min(100, darkness))) / 100)
        self.css = Gtk.CssProvider()
        self.css.load_from_data(
            f"window, .background {{ background-color: rgb({level},{level},{level}); }}".encode())
        Gtk.StyleContext.add_provider_for_screen(screen, self.css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
        self.level = level  # for tests


def set_switch(switch, value):
    """Show a switch's real state, also after a refused change (set_state alone
    is a no-op when the state didn't change, leaving the knob where it was)."""
    switch.set_active(value)
    switch.set_state(value)


def section(title, *children, help=None, plain=False):
    """A titled group of widgets: the title, then the widgets on a card
    (R-GUI-3: grouped, aligned, not a wall). help puts the long explanation
    behind a ? button; plain leaves the card out (for a list that has its
    own frame)."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
    title_label = heading(title, "medium")
    if help:
        row = Gtk.Box(spacing=6)
        row.pack_start(title_label, False, False, 0)
        row.pack_start(help_button(help), False, False, 0)
        box.pack_start(row, False, False, 0)
        box.kc9k_title = title_label
    else:
        box.pack_start(title_label, False, False, 0)
    inner = box if plain else card()
    for child in children:
        inner.pack_start(child, False, False, 0)
    if not plain:
        box.pack_start(inner, False, False, 0)
    return box


def card(*children, spacing=10, kind=""):
    """A rounded, bordered box (R-GUI-3: cards, not boxes in boxes)."""
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=spacing)
    box.get_style_context().add_class("kc9k-card")
    if kind:
        box.get_style_context().add_class(kind)
    for child in children:
        box.pack_start(child, False, False, 0)
    return box


def help_button(text):
    """A small ? that shows a longer explanation in a popover (progressive
    disclosure: one line on the page, the rest on demand)."""
    button = Gtk.MenuButton(valign=Gtk.Align.CENTER, tooltip_text="More about this")
    button.add(Gtk.Label(label="?"))
    button.get_style_context().add_class("kc9k-help")
    popover = Gtk.Popover()
    label = Gtk.Label(label=text, wrap=True, max_width_chars=56, xalign=0, margin=14, selectable=True)
    popover.add(label)
    label.show()
    button.set_popover(popover)
    button.kc9k_text = text  # for tests
    return button


def pill(text, kind="off"):
    """A status pill: on, locked, lifted, off or warn."""
    label = Gtk.Label(label=text)
    label.get_style_context().add_class("kc9k-pill")
    label.get_style_context().add_class(kind)
    return label


def page_header(box, title, subtitle="", help=None):
    """A page's title and its one-line subtitle (R-GUI-3)."""
    head = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
    top = Gtk.Box(spacing=8)
    label = Gtk.Label(label=title, xalign=0)
    label.get_style_context().add_class("kc9k-page-title")
    top.pack_start(label, False, False, 0)
    if help:
        top.pack_start(help_button(help), False, False, 0)
    head.pack_start(top, False, False, 0)
    if subtitle:
        sub = Gtk.Label(label=subtitle, xalign=0, wrap=True)
        sub.get_style_context().add_class("kc9k-page-subtitle")
        head.pack_start(sub, False, False, 0)
    box.pack_start(head, False, False, 0)
    return head


def block_pills(block):
    """A block's state as pills: on/off, enabled/lifted, and each lock with
    its time (R-GUI-3: lock state shown where the block appears)."""
    out = []
    if not block["active"]:
        out.append(("Off", "off"))
    elif block.get("enabled"):
        out.append(("On", "on"))
    else:
        out.append(("Lifted now", "lifted"))
    if block.get("lock_seconds"):
        out.append((f"Locked {fmt_seconds(block['lock_seconds'])}", "locked"))
    if block.get("char_locked"):
        out.append(("Text lock", "locked"))
    if block.get("password_locked"):
        out.append(("Password lock", "locked"))
    if block.get("restart_locked"):
        out.append(("Until restart", "locked"))
    if block.get("delay_left"):
        out.append((f"Unlocks in {fmt_seconds(block['delay_left'])}", "warn"))
    elif block.get("delay_seconds"):
        out.append((f"Delay {fmt_seconds(block['delay_seconds'])}", "locked"))
    if block.get("timer_after"):
        out.append((f"Then locked {fmt_seconds(block['timer_after'])}", "locked"))
    if block.get("range_lock"):
        out.append((f"Range {block['range_lock']}", "locked" if block.get("range_on") else "off"))
    if block.get("schedule_lock"):
        out.append(("Schedule lock", "locked"))
    if block.get("paused_seconds"):
        out.append((f"Break {fmt_seconds(block['paused_seconds'])}", "lifted"))
    if block.get("quota_open"):
        out.append(("Allowance session", "lifted"))
    return out


def pill_row(pills):
    row = Gtk.Box(spacing=6)
    for text, kind in pills:
        row.pack_start(pill(text, kind), False, False, 0)
    return row


def page_box():
    box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18,
                  margin_start=24, margin_end=24, margin_top=20, margin_bottom=20)
    scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
    scroll.add(box)
    return scroll, box


LOCK_KIND_NAMES = {"timer": "timer lock", "text": "text lock", "password": "password lock",
                   "restart": "restart lock", "delay": "delay lock"}


NOTICE_KINDS = (
    ("window-start", "A window opens"),
    ("window-end", "A window closes"),
    ("timer-end", "A timer lock ends"),
    ("pomodoro", "Pomodoro work or rest begins"),
    ("allowance", "An allowance runs out"),
    ("app-blocked", "A blocked app was closed"),
)


def breaks_words(block):
    """What breaks a block allows, in short (§4.2 block list)."""
    parts = []
    if block.get("break_allowance"):
        per = block.get("break_per") or 3600
        every = "hour" if per == 3600 else f"{per // 3600} hours" if per % 3600 == 0 else fmt_seconds(per)
        parts.append(f"{fmt_seconds(block['break_allowance'])} every {every}")
    if block.get("relock"):
        parts.append("on unlocking")
    if block.get("delay_break"):
        parts.append("after a wait")
    if block.get("pomodoro_on"):
        parts.append("Pomodoro")
    if block.get("quota_limit"):
        parts.append("allowance")
    return "Breaks: " + ", ".join(parts) if parts else "No breaks"


def lock_words(block):
    """A block's lock state, in short (§4.2 block list)."""
    if not block.get("held"):
        return "Unlocked"
    if block.get("lock_seconds"):
        return f"Locked, {fmt_seconds(block['lock_seconds'])} left"
    for key, words in (("char_locked", "text lock"), ("password_locked", "password"),
                       ("restart_locked", "until restart"), ("range_lock", "range lock")):
        if block.get(key):
            return f"Locked: {words}"
    return "Locked"


def block_summary(block):
    """One line describing a block's state and locks."""
    # §3.3: active (switched on) and enabled (enforcing now) or lifted.
    parts = [("Active, enabled" if block.get("enabled") else "Active, lifted") if block["active"] else "Inactive"]
    if block["lock_seconds"]:
        parts.append(f"timer lock, {fmt_seconds(block['lock_seconds'])} left")
    if block["char_locked"]:
        parts.append("text lock")
    if block["password_locked"]:
        parts.append("password-locked")
    if block["restart_locked"]:
        parts.append("on until restart")
    if block["stop_seconds"]:
        parts.append(f"stops in {fmt_seconds(block['stop_seconds'])}")
    if block.get("delay_left"):
        parts.append(f"delay lock unlocks in {fmt_seconds(block['delay_left'])}")
    elif block["delay_seconds"]:
        parts.append(f"delay lock of {fmt_seconds(block['delay_seconds'])}")
    if block.get("timer_after"):
        parts.append(f"timer lock of {fmt_seconds(block['timer_after'])} after the locks before it")
    if block.get("range_lock"):
        parts.append(f"range lock {block['range_lock']} ({'on' if block.get('range_on') else 'off'} now)")
    if len(block.get("lock_order") or []) > 1:
        # R-LOCK-6: unlocked one at a time in this order.
        parts.append("unlock order: " + " → ".join(LOCK_KIND_NAMES.get(k, k) for k in block["lock_order"]))
    if block["pomodoro_on"]:
        parts.append(f"Pomodoro {block['pomodoro_on'] // 60}+{block['pomodoro_off'] // 60} min")
    if block.get("paused_seconds"):
        parts.append(f"on a break for {fmt_seconds(block['paused_seconds'])}")
    if block["on_break"]:
        parts.append("on a break")
    if block["scheduled"]:
        parts.append("automated")  # §3.3: windows or places switch it
    if block.get("lifted"):
        parts.append("lifted by its automation now")
    if block.get("today_windows"):
        parts.append("today: " + ", ".join(block["today_windows"]))
    if block["quota_limit"]:
        parts.append(f"allowance {block['quota_used'] // 60} of {block['quota_limit'] // 60} min")
    if block["quota_open"]:
        parts.append("allowance session open")
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


def rounded_rect(cr, x, y, width, height, radius=4):
    """A rectangle with all four corners rounded."""
    radius = max(min(radius, width / 2, height / 2), 0)
    cr.new_sub_path()
    cr.arc(x + width - radius, y + radius, radius, -0.5 * math.pi, 0)
    cr.arc(x + width - radius, y + height - radius, radius, 0, 0.5 * math.pi)
    cr.arc(x + radius, y + height - radius, radius, 0.5 * math.pi, math.pi)
    cr.arc(x + radius, y + radius, radius, math.pi, 1.5 * math.pi)
    cr.close_path()


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


class LockPicker(Gtk.Box):
    """Every lock kind for a block or a setting (§3.4), the timer lock
    first (R-GUI-2), the rest behind More locks. chosen() gives the locks
    as (flags, words) in the order the dialog has always used (timer,
    restart, password, text, delay, range); relock_flags() the re-lock.
    kinds limits what is offered (an active block takes only a longer
    timer lock or a longer text lock of random characters)."""

    ALL_KINDS = ("timer", "password", "text", "restart", "delay", "range", "relock")

    def __init__(self, verb="stop", done="stopped", kinds=ALL_KINDS):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        self.callbacks, self.verb, self.done, self.kinds = [], verb, done, tuple(kinds)
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
            more.attach(dim("With a password or text lock: unlocking lifts them only this long; then they "
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
        self.relock.set_sensitive(on or bool(self.password.get_text()))
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

    def relock_seconds(self):
        if self.relock.value and (self.typing.get_active() or self.secret()):
            return self.relock.value
        return 0

    def relock_flags(self):
        seconds = self.relock_seconds()
        return ["--relock", f"{seconds // 60}m"] if seconds else []

    def chosen(self):
        """The locks asked for, as (flags, words), in the dialog's order."""
        relock = self.relock_seconds()
        relock_note = f", re-lock {duration_words(relock)} after unlocking" if relock else ""
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
    in gui.json under key, so the list opens the same next time."""

    MIN = 80

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
        self.target.set_size_request(-1, -1 if height is None else height)
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
        """Deletes the phrase loaded in the editor, after asking."""
        old = self.editing
        if old is None or not self.window.confirm(
                "Delete this phrase?", old["text"] + "\n\nDeleting a phrase weakens the block.",
                "Delete", destructive=True):
            return
        self.window.run(self.command("remove", old["text"]))

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


# The window --------------------------------------------------------------

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
    return x ** (math.log(middle) / math.log(0.5))


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
    curve bends it, double-click types exact values."""

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
        self.hover = None
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

    def on_curve(self, px, py):
        """The index of the point whose curve passes under the pointer."""
        if len(self.points) < 2:
            return None
        x, _y, w, _h = self.plot()
        if px < x or px > x + w:
            return None
        minute = (px - x) / w * DAY_MINUTES
        if abs(self.y_of(clip_value(self.points, minute)) - py) > self.GRAB:
            return None
        return clip_segment(self.points, minute)[0]

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
        if event.type == Gdk.EventType.DOUBLE_BUTTON_PRESS:
            self.drag = None
            if index is not None:
                self.type_point(index)
            return True
        if index is not None:
            self.drag, self.moved = ("point", index), False
        else:
            segment = self.on_curve(event.x, event.y)
            if segment is not None:
                self.drag, self.moved = ("tension", segment, event.y, self.points[segment].get("tension", 0)), False
        return True

    def motion(self, _widget, event):
        if self.drag is None:
            hover = self.point_at(event.x, event.y)
            if hover != self.hover:
                self.hover = hover
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
        if self.hover is not None and self.drag is None:
            self.hover = None
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


class TriggerEditor(Gtk.Box):
    """What a block's apps or window titles do when found (owner,
    2026-11-16: any trigger to any action): their own action (close the
    app or the window) and the consequences, each with its time. (A place
    rule has no actions, §3.6.) Ticking runs
    'kc9k block trigger', as does Enter in a time field while its action is
    on. set_actions shows what the state says."""

    CONSEQUENCES = (("close-browser", "Close the browsers", None),
                    ("lock-screen", "Lock the screen for", 30 * 60),
                    ("logout", "Log out", 0), ("shutdown", "Shut down", 0),
                    ("monk", "Start monk mode for", 60 * 60),
                    ("mode", "Run a mode, locked, for", 60 * 60),
                    ("custom", "Custom action", None))

    def __init__(self, window, group, own, own_label, intro):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        self.window, self.group, self.own = window, group, own
        self.block = None
        self.filling = False
        self.pack_start(dim(intro), False, False, 0)
        self.checks, self.times = {}, {}
        rows = [(own, own_label, None)] + list(self.CONSEQUENCES)
        for name, label, seconds in rows:
            row = Gtk.Box(spacing=6)
            check = Gtk.CheckButton(label=label)
            check.connect("toggled", self.toggled, name)
            row.pack_start(check, False, False, 0)
            self.checks[name] = check
            if seconds is not None:
                once = name in ("logout", "shutdown")
                field = DurationField(seconds, minimum=60, zero="once" if once else None,
                                      maximum=30 * 86400 if name in ("monk", "mode") else 86400, width=8)
                field.entry.connect("activate", lambda _e, n=name: self.time_changed(n))
                row.pack_start(field, False, False, 0)
                self.times[name] = field
            if name == "mode":
                # §3.9 Mode action: which mode (owner 2026-11-17).
                self.mode_combo = Gtk.ComboBoxText(valign=Gtk.Align.START)
                self.mode_combo.connect("changed", lambda _c: self.filling or self.time_changed("mode"))
                row.pack_start(self.mode_combo, False, False, 0)
            if name == "custom":
                # §3.9 Custom action: what it opens or runs.
                self.custom_items = []
                self.custom_label = dim("")
                change = Gtk.Button(label="Change…", valign=Gtk.Align.START)
                change.connect("clicked", lambda _b: self.change_custom())
                row.pack_start(change, False, False, 0)
                row.pack_start(self.custom_label, False, False, 0)
            self.pack_start(row, False, False, 0)

    def set_actions(self, block, actions):
        self.block = block
        self.filling = True
        try:
            have = {a["name"]: a for a in actions or []}
            for name, check in self.checks.items():
                check.set_active(name in have)
                if name in self.times and name in have and not self.times[name].has_focus():
                    self.times[name].set_seconds(have[name].get("minutes", 0) * 60)
            choices = self.window.mode_choices()
            chosen = (have.get("mode") or {}).get("param") or self.mode_combo.get_active_text()
            self.mode_combo.remove_all()
            for m in choices:
                self.mode_combo.append_text(m)
            if choices:
                self.mode_combo.set_active(choices.index(chosen) if chosen in choices else 0)
            if "custom" in have:
                try:
                    self.custom_items = [f"{list(i)[0]} {list(i.values())[0]}"
                                         for i in json.loads(have["custom"].get("param") or "[]")]
                except (ValueError, IndexError, AttributeError):
                    self.custom_items = []
            self.custom_label.set_text("; ".join(self.custom_items))
        finally:
            self.filling = False

    def change_custom(self):
        items = self.window.ask_custom("Custom action", self.custom_items)
        if items is None:
            return
        self.custom_items = items
        self.custom_label.set_text("; ".join(items))
        if self.block and self.checks["custom"].get_active():
            args = self.command("custom", True)
            if args:
                self.window.run(args)

    def command(self, name, on):
        args = ["block", "trigger", self.block, self.group, name, "on" if on else "off"]
        field = self.times.get(name)
        if on and field is not None:
            if not field.is_valid():
                self.window.message(f"{self.checks[name].get_label()}: {field.problem}", Gtk.MessageType.INFO)
                return None
            if field.value:
                args += ["--for", field.arg()]
        if on and name == "mode":
            if not self.mode_combo.get_active_text():
                self.window.message("Make a mode first: a Mode action runs one.", Gtk.MessageType.INFO)
                return None
            args += ["--target", self.mode_combo.get_active_text()]
        if on and name == "custom":
            if not self.custom_items:
                items = self.window.ask_custom("Custom action", [])
                if not items:
                    return None
                self.custom_items = items
            args += [a for item in self.custom_items for a in ("--custom", item)]
        if on and name in ("monk", "mode"):
            if not self.window.acknowledge_no_confirmation("This trigger"):
                return None
            args.append("--no-confirm-ok")
        return args

    def toggled(self, check, name):
        if self.filling or not self.block:
            return
        args = self.command(name, check.get_active())
        if args is None:
            self.filling = True
            check.set_active(not check.get_active())
            self.filling = False
            return
        self.window.run(args)
        self.reload_later()

    def reload_later(self):
        """Shows the state again once the command and the refresh are done
        (a refused change goes back)."""
        def check():
            if not self.window.get_sensitive() or getattr(self.window, "busy", False):
                return True
            block = self.window.blocks.get(self.block) or {}
            actions = (block.get("triggers") or {}).get(self.group)
            if actions is not None:
                self.set_actions(self.block, actions)
            return False
        GLib.timeout_add(150, check)

    def time_changed(self, name):
        if self.block and self.checks[name].get_active():
            args = self.command(name, True)
            if args:
                self.window.run(args)


def window_words(w):
    """A window as the CLI writes it: days, from, to."""
    names = [DAY_NAMES[day] for day in range(7) if w["days"] & (1 << ((day + 1) % 7))]  # Monday first
    return names, clock_text(w["start"]), "24:00" if w["end"] >= 1440 else clock_text(w["end"])


def schedule_summary(windows):
    """A schedule's windows in a line, and its hours a week."""
    covered = set()
    for w in windows:
        for day in range(7):
            if w["days"] & (1 << day):
                covered.update((day, minute) for minute in range(w["start"], w["end"]))
    parts = []
    for w in windows[:3]:
        names, frm, to = window_words(w)
        days = ",".join(names) if len(names) < 7 else "daily"
        parts.append(f"{days.replace('mon,tue,wed,thu,fri', 'weekdays').replace('sat,sun', 'weekends')} "
                     f"{show_clock(w['start'])}–{show_clock(w['end'], True)}")
    if len(windows) > 3:
        parts.append(f"{len(windows) - 3} more")
    hours = f"{len(covered) / 60:.0f} h a week" if covered else "no windows yet"
    return (", ".join(parts) + " · " if parts else "") + hours


class CustomEditor(Gtk.Box):
    """§3.9 Custom action (owner 2026-11-17): what to open or run, one row
    each with Remove, and a row to add one: open a site, file or folder, or
    run a program or command. Items are "open …" / "run …" as the CLI takes
    them. on_change(items), when given, runs after each change."""

    KINDS = (("open", "Open a site, file or folder"), ("run", "Run a program or command"))

    def __init__(self, window, items=(), on_change=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.window, self.on_change = window, on_change
        self.rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.rows.get_style_context().add_class("frame")
        self.rows.set_no_show_all(True)
        self.pack_start(self.rows, False, False, 0)
        add = Gtk.Box(spacing=6)
        self.kind = Gtk.ComboBoxText()
        for key, label in self.KINDS:
            self.kind.append(key, label)
        self.kind.set_active_id("open")
        self.kind.connect("changed", lambda _c: self.entry.set_placeholder_text(
            "https://… or ~/Documents/notes.odt" if self.kind.get_active_id() == "open" else "libreoffice --writer"))
        self.entry = Gtk.Entry(placeholder_text="https://… or ~/Documents/notes.odt", width_chars=28)
        self.entry.connect("activate", lambda _e: self.add_item())
        choose = Gtk.Button(label="Choose…", tooltip_text="Choose a file or folder to open")
        choose.connect("clicked", lambda _b: self.choose())
        button = Gtk.Button(label="Add")
        button.connect("clicked", lambda _b: self.add_item())
        for widget in (self.kind, self.entry, choose, button):
            add.pack_start(widget, widget is self.entry, widget is self.entry, 0)
        self.pack_start(flow_row(add), False, False, 0)
        self.items = []
        self.set_items(items)

    def set_items(self, items):
        self.items = list(items)
        for row in self.rows.get_children():
            self.rows.remove(row)
        for number, item in enumerate(self.items):
            verb, _sep, what = item.partition(" ")
            line = Gtk.Box(spacing=8, margin=6)
            line.pack_start(Gtk.Label(label="Open" if verb == "open" else "Run", xalign=0), False, False, 0)
            line.pack_start(Gtk.Label(label=what, xalign=0, ellipsize=Pango.EllipsizeMode.MIDDLE, selectable=True),
                            True, True, 0)
            remove = Gtk.Button(label="Remove", relief=Gtk.ReliefStyle.NONE)
            remove.connect("clicked", lambda _b, n=number: self.remove_item(n))
            line.pack_start(remove, False, False, 0)
            self.rows.add(line)
        self.rows.set_visible(bool(self.items))
        self.rows.show_all()

    def add_item(self, text=None):
        what = (text if text is not None else self.entry.get_text()).strip()
        if not what or "\n" in what:
            return
        self.entry.set_text("")
        self.set_items(self.items + [f"{self.kind.get_active_id()} {what}"])
        if self.on_change:
            self.on_change(self.items)

    def remove_item(self, number):
        self.set_items(self.items[:number] + self.items[number + 1:])
        if self.on_change:
            self.on_change(self.items)

    def choose(self):
        path = self.window.choose_file("Choose a file or folder to open", Gtk.FileChooserAction.OPEN, "Choose")
        if path:
            self.kind.set_active_id("open")
            self.add_item(path)


def place_effect(subject, default_on, need_both, has_windows):
    """What arriving at a place added to an automation does (§3.6), said
    plainly: subject is "this block", "its blocks" or "monk mode"."""
    state = "off" if default_on else "on"
    if need_both and has_windows:
        return f"Here during one of its windows, {subject} {'are' if subject == 'its blocks' else 'is'} {state}."
    return f"Arriving here turns {subject} {state}."


class PlaceRules(Gtk.Box):
    """§3.6, §4.5 (owner 2026-11-17): the places in an automation, a row
    each saying plainly that it is added, whether arriving there turns the
    block (a mode's blocks, monk mode) on or off, and whether the computer is
    there now; below, a saved place to add and, where given, Save where I am
    now as…. add(place) and remove(place) run the commands."""

    def __init__(self, window, add, remove, save_here=None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.window, self.add_place, self.remove_place = window, add, remove
        self.rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
        self.rows.get_style_context().add_class("frame")
        self.rows.set_no_show_all(True)
        self.pack_start(self.rows, False, False, 0)
        self.empty = dim("")
        self.pack_start(self.empty, False, False, 0)
        pick = Gtk.Box(spacing=6)
        self.combo = Gtk.ComboBoxText(tooltip_text="A saved place (Places page)")
        pick.pack_start(self.combo, False, False, 0)
        self.add_button = Gtk.Button(label="Add this place")
        self.add_button.connect("clicked", lambda _b: self.combo.get_active_text() and self.add_place(
            self.combo.get_active_text()))
        pick.pack_start(self.add_button, False, False, 0)
        if save_here:
            here = Gtk.Button(label="Save where I am now as…")
            here.connect("clicked", lambda _b: save_here())
            pick.pack_start(here, False, False, 0)
        self.pick = flow_row(pick)
        self.pack_start(self.pick, False, False, 0)
        self.names = []  # the places shown as added (for tests)
        self.shown = None

    def show_places(self, added, subject, default_on=False, need_both=False, has_windows=False):
        places = {p["name"]: p for p in (self.window.state or {}).get("places") or []}
        effect = place_effect(subject, default_on, need_both, has_windows)
        key = (list(added), effect, [(n, p.get("here")) for n, p in places.items()])
        if key != self.shown:
            self.shown = key
            self.names = list(added)
            for row in self.rows.get_children():
                self.rows.remove(row)
            for name in added:
                line = Gtk.Box(spacing=10, margin=8)
                line.pack_start(Gtk.Image.new_from_icon_name("emblem-ok-symbolic", Gtk.IconSize.MENU),
                                False, False, 0)
                text = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
                title = Gtk.Box(spacing=8)
                label = Gtk.Label(label=name, xalign=0)
                label.get_style_context().add_class("kc9k-card-name")
                title.pack_start(label, False, False, 0)
                here = (places.get(name) or {}).get("here")
                title.pack_start(pill("You are here" if here else "Not here now", "on" if here else "off"),
                                 False, False, 0)
                text.pack_start(title, False, False, 0)
                text.pack_start(dim("Added. " + effect), False, False, 0)
                line.pack_start(text, True, True, 0)
                remove = Gtk.Button(label="Remove", valign=Gtk.Align.CENTER)
                remove.set_tooltip_text("Take this place out of the automation")
                remove.connect("clicked", lambda _b, p=name: self.remove_place(p))
                line.pack_start(remove, False, False, 0)
                row = Gtk.ListBoxRow()
                row.add(line)
                row.place_name = name
                self.rows.add(row)
            self.rows.set_visible(bool(added))
            self.rows.show_all()
        self.empty.set_text("" if added else
                            "No places added. A place added here switches it by itself: " + effect.lower()
                            .replace("arriving here", "arriving at the place") if places else
                            "No places saved yet: save where the computer is now.")
        self.empty.set_visible(not added)
        saved = [n for n in places if n not in added]
        chosen = getattr(self, "to_choose", None) or self.combo.get_active_text()
        self.to_choose = None
        self.combo.remove_all()
        for n in saved:
            self.combo.append_text(n)
        if saved:
            self.combo.set_active(saved.index(chosen) if chosen in saved else 0)
        self.combo.set_sensitive(bool(saved))
        self.add_button.set_sensitive(bool(saved))


class AutomationEditor(Gtk.Box):
    """§3.6, §4.5 (owner 2026-11-17): the automation of a mode (one for all
    its blocks), of monk mode, or the windows of a schedule kept outside
    blocks. Every change runs the commands a block's does (schedule add and
    remove, schedule apply and take-off, place rule, automation) with the
    mode's or schedule's name; the CLI decides (R-MODE-2, R-MONK-3)."""

    def __init__(self, window, kind):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        self.window, self.kind, self.name, self.data = window, kind, None, {}
        self.summary = Gtk.Label(xalign=0, wrap=True)
        self.pack_start(self.summary, False, False, 0)
        self.buttons = {}
        self.updating = False

        def choice(pairs, setting):
            row, group = Gtk.Box(), None
            row.get_style_context().add_class("linked")
            for value, label in pairs:
                button = Gtk.RadioButton.new_with_label_from_widget(group, label)
                button.set_mode(False)
                group = group or button
                button.connect("toggled", self.set_setting, setting, value)
                row.pack_start(button, False, False, 0)
                self.buttons[(setting, value)] = button
            return row

        if kind == "mode":
            self.pack_start(choice((("disabled", "Disabled by default"), ("enabled", "Enabled by default")),
                                   "default"), False, False, 0)
        self.need_row = Gtk.Box(spacing=8)
        if kind != "schedule":
            self.need_row.pack_start(Gtk.Label(label="With windows and places:"), False, False, 0)
            self.need_row.pack_start(choice((("either", "Either is enough"), ("both", "Both at once")), "needs"),
                                     False, False, 0)
            for child in self.need_row.get_children():
                child.show_all()
            self.need_row.set_no_show_all(True)
            self.pack_start(self.need_row, False, False, 0)
        self.calendar = WeekCalendar(self.create_window, self.edit_window, self.move_window)
        intro = Gtk.Box(spacing=6)
        intro.pack_start(dim("Drag down a day to add a window; click one to change or remove it; drag it "
                             "to move it."), False, False, 0)
        self.pack_start(intro, False, False, 0)
        self.pack_start(self.calendar, False, False, 0)
        presets = Gtk.Box(spacing=6)
        presets.pack_start(Gtk.Label(label="Add:"), False, False, 0)
        for label, days, start, end in SCHEDULE_PRESETS:
            button = Gtk.Button(label=label, tooltip_text=f"{days} {start}-{end}")
            button.connect("clicked", lambda _b, d=days, f=start, t=end: self.add_window(d, f, t))
            presets.pack_start(button, False, False, 0)
        self.total = dim("")
        presets.pack_end(self.total, False, False, 0)
        self.pack_start(flow_row(presets), False, False, 0)
        self.applied = Gtk.Box(spacing=6)
        if kind != "schedule":
            row = Gtk.Box(spacing=6)
            row.pack_start(Gtk.Label(label="Schedules:"), False, False, 0)
            row.pack_start(self.applied, False, False, 0)
            apply = Gtk.Button(label="Apply schedule…")
            apply.set_tooltip_text("Evening, Weekend…: made on the Schedules page; changing it there changes this too")
            apply.connect("clicked", self.apply_schedule)
            row.pack_start(apply, False, False, 0)
            self.pack_start(flow_row(row), False, False, 0)
            self.pack_start(heading("Places", "medium"), False, False, 0)
            self.places = PlaceRules(window, self.add_named_place,
                                     lambda p: self.run(["place", "rule", self.name, p, "off"]),
                                     lambda: window.save_place(None, target=self.name, flags=self.flags()))
            self.place_combo = self.places.combo
            self.pack_start(self.places, False, False, 0)

    # The commands -----------------------------------------------------
    def monk_ok(self, what):
        """R-LOCK-12: adding to monk mode's time while it is on asks first;
        the command then takes --yes. Others need nothing."""
        if self.kind != "monk" or not self.data.get("active"):
            return []
        if self.window.confirm_lock([f"monk mode (no websites, no internet, can't be switched off while it "
                                     f"runs): also {what}"]):
            return ["--yes"]
        return None

    def flags(self):
        """§4.2 Names (owner 2026-11-17): a mode or schedule may share a
        block's name, so its commands say which is meant."""
        return {"mode": ["--mode"], "monk": ["--mode"], "schedule": ["--schedule"]}.get(self.kind, [])

    def run(self, args, then=None):
        self.window.run(args + self.flags(), then=then)

    def add_window(self, days, frm, to):
        if not self.name:
            return
        yes = self.monk_ok(f"{days} {frm}–{to}")
        if yes is None:
            return
        flags = ["--allow"] if self.kind == "mode" and (self.data.get("automation") or {}).get("default_on") else []
        self.run(["schedule", "add", self.name, "--days", days, "--from", frm, "--to", to, *flags, *yes])

    def create_window(self, column, start, end):
        day = DAY_NAMES[column_day(column)]
        if self.window.confirm(f"Add {day.capitalize()} {show_clock(start)}–{show_clock(end, True)}?",
                               "Tick more days afterwards by clicking it.", "Add"):
            self.add_window(day, clock_text(start), "24:00" if end >= 1440 else clock_text(end))

    def from_schedule(self, w):
        """A window from a schedule is changed on the Schedules page."""
        if self.window.confirm(f"This window comes from the schedule {w['source']}",
                               "Change it on the Schedules page (that changes everything using it), or take "
                               "the schedule off here.", "Open Schedules"):
            self.window.open_schedule(w["source"])

    def windows(self):
        return self.data.get("windows") or []

    def move_window(self, index, days, start, end):
        windows = self.windows()
        if index >= len(windows):
            return
        if windows[index].get("source"):
            self.from_schedule(windows[index])
            return
        self.replace(index, {"days": days, "start": start, "end": end})

    def replace(self, index, w):
        """Removes window index, then adds w (a change that weakens is
        refused by the CLI, and the old one stays)."""
        names, frm, to = window_words(w)
        if not names:
            return
        yes = self.monk_ok(f"{','.join(names)} {frm}–{to}")
        if yes is None:
            return
        flags = ["--allow"] if self.kind == "mode" and (self.data.get("automation") or {}).get("default_on") else []
        add = ["schedule", "add", self.name, "--days", ",".join(names), "--from", frm, "--to", to, *flags, *yes]
        self.run(["schedule", "remove", self.name, str(index + 1)], then=lambda _o: self.run(add))

    def edit_window(self, index):
        windows = self.windows()
        if index >= len(windows):
            return
        w = windows[index]
        if w.get("source"):
            self.from_schedule(w)
            return
        dialog = Gtk.Dialog(title="Window", transient_for=self.window, modal=True)
        dialog.add_button("Remove", 3)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Save", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        days_row, checks = Gtk.Box(spacing=6), {}
        for column in range(7):
            day = column_day(column)
            bit = 1 << ((day + 1) % 7)
            check = Gtk.CheckButton(label=DAY_NAMES[day].capitalize(), active=bool(w["days"] & bit))
            checks[bit] = check
            days_row.pack_start(check, False, False, 0)
        box.pack_start(dim("Tick more days to copy the window to them."), False, False, 0)
        box.pack_start(days_row, False, False, 0)
        times = Gtk.Box(spacing=6)
        start = Gtk.Entry(text=clock_text(w["start"]), width_chars=5)
        end = Gtk.Entry(text="24:00" if w["end"] >= 1440 else clock_text(w["end"]), width_chars=5)
        for widget in (Gtk.Label(label="From"), start, Gtk.Label(label="to"), end):
            times.pack_start(widget, False, False, 0)
        box.pack_start(times, False, False, 0)
        self.dialog = (checks, start, end)  # for tests
        dialog.show_all()
        answer = dialog.run()
        days = sum(bit for bit, check in checks.items() if check.get_active())
        frm, to = start.get_text().strip(), end.get_text().strip()
        dialog.destroy()
        if answer == 3:
            if self.window.confirm("Remove this window?", "A change that weakens a locked block is refused.",
                                   "Remove", destructive=True):
                self.run(["schedule", "remove", self.name, str(index + 1)])
            return
        if answer != Gtk.ResponseType.OK:
            return
        new_start, new_end = parse_clock(frm), (1440 if to == "24:00" else parse_clock(to))
        if new_start is None or new_end is None or new_start >= new_end:
            self.window.message("Times are HH:MM, the start before the end, e.g. 09:00 and 17:00.",
                                Gtk.MessageType.INFO)
            return
        if (days, new_start, new_end) != (w["days"], w["start"], w["end"]):
            self.replace(index, {"days": days, "start": new_start, "end": new_end})

    def set_setting(self, button, setting, value):
        if self.updating or not button.get_active() or not self.name:
            return
        self.run(["automation", self.name, setting, value])

    def apply_schedule(self, _button):
        have = set(self.data.get("schedules") or [])
        names = [s["name"] for s in self.window.state.get("schedules") or [] if s["name"] not in have]
        if not names:
            if self.window.confirm("No schedule to apply", "Make schedules such as Evening or Weekend on the "
                                   "Schedules page, then apply them here.", "Open Schedules"):
                self.window.stack.set_visible_child_name("schedules")
            return
        name = self.window.choose("Apply a schedule", "Its windows join these; changing it on the Schedules "
                                  "page changes them too.", names)
        if not name:
            return
        yes = self.monk_ok(f"the schedule {name}")
        if yes is not None:
            self.run(["schedule", "apply", self.name, name, *yes])

    def add_place(self, _button):
        self.add_named_place(self.place_combo.get_active_text())

    def add_named_place(self, place):
        if not place or not self.name:
            return
        yes = self.monk_ok(f"while the computer is at {place}")
        if yes is not None:
            self.run(["place", "rule", self.name, place, "on", *yes])

    def chip(self, text, tooltip, remove):
        chip = Gtk.Box(spacing=2)
        chip.get_style_context().add_class("kc9k-pill")
        chip.get_style_context().add_class("on")
        chip.pack_start(Gtk.Label(label=text), False, False, 0)
        x = Gtk.Button.new_from_icon_name("window-close-symbolic", Gtk.IconSize.MENU)
        x.set_relief(Gtk.ReliefStyle.NONE)
        x.set_tooltip_text(tooltip)
        x.connect("clicked", lambda _b: remove())
        chip.pack_start(x, False, False, 0)
        return chip

    # Showing it ---------------------------------------------------------
    def show_data(self, name, data):
        """data: a mode or schedule from gui-state."""
        self.name, self.data = name, data or {}
        windows = self.windows()
        self.calendar.set_windows(windows)
        self.total.set_text(schedule_summary(windows).split(" · ")[-1])
        settings = self.data.get("automation") or {}
        places = self.data.get("places") or []
        if self.kind == "schedule":
            used = (self.data.get("blocks") or []) + [f"mode {m}" for m in self.data.get("modes") or []]
            self.summary.set_text("Used by " + ", ".join(used) + ": a change here changes them too; one that "
                                  "weakens a locked block is refused." if used else
                                  "Used by nothing yet: apply it to blocks and modes in their Automation.")
        elif self.data.get("continuous"):
            self.summary.set_text("No windows or places yet: add some below." if self.kind == "monk" else
                                  "Continuous: its blocks are enabled whenever they are on. Add windows or "
                                  "places below to switch them all by themselves.")
        else:
            by = " and ".join(p for p, has in (("its windows", windows), ("its places", places)) if has)
            if self.kind == "monk":
                self.summary.set_text(f"Monk mode runs while {by} have it on, once switched on.")
            else:
                self.summary.set_text(f"Automated: {by} turn its blocks {'off' if settings.get('default_on') else 'on'}.")
        self.updating = True
        if ("default", "enabled") in self.buttons:
            self.buttons[("default", "enabled" if settings.get("default_on") else "disabled")].set_active(True)
        if ("needs", "both") in self.buttons:
            self.buttons[("needs", "both" if settings.get("need_both") else "either")].set_active(True)
        self.updating = False
        if self.kind == "schedule":
            return
        self.need_row.set_visible(bool(windows) and bool(places))
        for child in self.applied.get_children():
            self.applied.remove(child)
        for sched in self.data.get("schedules") or []:
            self.applied.pack_start(self.chip(sched, "Take this schedule off",
                                              lambda s=sched: self.run(["schedule", "take-off", self.name, s])),
                                    False, False, 0)
        if not self.data.get("schedules"):
            self.applied.pack_start(dim("none"), False, False, 0)
        self.applied.show_all()
        self.places.show_places(places, "monk mode" if self.kind == "monk" else "its blocks",
                                bool(settings.get("default_on")), bool(settings.get("need_both")), bool(windows))


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
        pages_row.pack_start(Gtk.Label(label="Show light web pages dark while dark mode is on", xalign=0),
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
            "Pages that are dark already stay as they are. With the system's setting, pages follow "
            "what the browser says the system prefers. The extension's toolbar button exempts a "
            "site too.")), False, False, 0)

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


class Window(Gtk.ApplicationWindow):
    def __init__(self, app):
        super().__init__(application=app, title="Killer Chicken 9000")
        self.set_default_size(980, 680)
        # §4.10 tray: closing hides the window while the tray icon is on.
        self.connect("delete-event", lambda w, _e: bool(getattr(w.get_application(), "tray", None)) and w.hide_on_delete())
        self.state_json = None
        self.state = None
        self.blocks = {}
        self.selected = None
        self.busy = False
        self.updating = False  # set while widgets are filled from the state
        self.settings_password = None  # remembered for this session once given
        self.access_passwords = []  # access passwords given, until closed or locked
        self.block_passwords = {}  # §3.11: block passwords typed, until closed
        self.card_switches = {}  # §4.2: the block list's on/off switches, by block
        self.access = {}  # area -> "none", "open" or "locked"
        self.covers = {}  # page name -> (stack, cover text, entry, title)
        self.last_input = GLib.get_monotonic_time()  # for locking again when idle
        self.connect("event-after", self.note_input)
        self.stats_rows = self.stats_page_rows = []  # the dashboard's and the Statistics page's (filtered)
        self.stats_on = True

        header = Gtk.HeaderBar(show_close_button=True, title="Killer Chicken 9000")
        self.set_titlebar(header)
        self.lock_button = Gtk.Button(label="Lock", no_show_all=True,
                                      tooltip_text="Forget the passwords given and lock the pages again")
        self.lock_button.connect("clicked", self.lock_access)
        header.pack_end(self.lock_button)

        root = Gtk.Box()
        self.add(root)
        self.stack = Gtk.Stack(transition_type=Gtk.StackTransitionType.CROSSFADE)
        # The sidebar: each page with its icon (R-GUI-3: recognised at a
        # glance), the app's name on top.
        side = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        side.get_style_context().add_class("kc9k-sidebar")
        side.set_size_request(200, -1)
        brand = Gtk.Box(spacing=8, margin_start=18, margin_top=16, margin_bottom=10)
        brand.pack_start(Gtk.Image.new_from_icon_name("security-high-symbolic", Gtk.IconSize.LARGE_TOOLBAR),
                         False, False, 0)
        brand_name = Gtk.Label(label="Killer Chicken", xalign=0)
        brand_name.get_style_context().add_class("kc9k-brand")
        brand.pack_start(brand_name, False, False, 0)
        side.pack_start(brand, False, False, 0)
        self.nav = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        self.nav.get_style_context().add_class("kc9k-sidebar")
        self.nav_rows = {}
        side.pack_start(self.nav, True, True, 0)
        root.pack_start(side, False, False, 0)
        root.pack_start(Gtk.Separator(orientation=Gtk.Orientation.VERTICAL), False, False, 0)

        content = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        root.pack_start(content, True, True, 0)
        self.banner = Gtk.InfoBar(message_type=Gtk.MessageType.WARNING, revealed=False)
        self.banner_label = Gtk.Label(xalign=0, wrap=True)
        self.banner.get_content_area().add(self.banner_label)
        content.pack_start(self.banner, False, False, 0)
        content.pack_start(self.stack, True, True, 0)

        for name, title, icon, build in (
                ("dashboard", "Overview", "go-home-symbolic", self.build_dashboard),
                ("blocks", "Blocks", "action-unavailable-symbolic", self.build_blocks),
                ("schedules", "Schedules", "x-office-calendar-symbolic", self.build_schedules_page),
                ("places", "Places", "mark-location-symbolic", self.build_places_page),
                ("hotkeys", "Hotkeys", "input-keyboard-symbolic", self.build_hotkeys_page),
                ("clusters", "Presets", "view-list-symbolic", self.build_clusters),
                ("stats", "Statistics", "x-office-presentation-symbolic", self.build_stats),
                ("circadian", "Circadian rhythm", "weather-clear-night-symbolic", self.build_circadian),
                ("settings", "Settings", "emblem-system-symbolic", self.build_settings)):
            self.stack.add_titled(self.guarded(name, title, build()), name, title)
            row = Gtk.ListBoxRow()
            row.page = name
            line = Gtk.Box(spacing=12)
            line.pack_start(Gtk.Image.new_from_icon_name(icon, Gtk.IconSize.MENU), False, False, 0)
            line.pack_start(Gtk.Label(label=title, xalign=0), True, True, 0)
            row.add(line)
            self.nav.add(row)
            self.nav_rows[name] = row
        self.nav.connect("row-selected", lambda _list, row: row and self.stack.set_visible_child_name(row.page))
        # §4.2: Blocks in the sidebar opens the block list, also from a block.
        self.nav.connect("row-activated", lambda _list, row: row.page == "blocks" and hasattr(self, "blocks_stack")
                         and self.blocks_stack.set_visible_child_name("cards"))
        self.stack.connect("notify::visible-child-name", self.page_changed)
        self.nav.select_row(self.nav_rows["dashboard"])

        self.refresh()
        self.refresh_stats()
        GLib.timeout_add_seconds(REFRESH_SECONDS, self.refresh)
        GLib.timeout_add_seconds(STATS_REFRESH_SECONDS, self.refresh_stats)

    # Running commands ----------------------------------------------------
    def opened_block(self):
        """The block opened on the Blocks page (never a draft), or None."""
        name = getattr(self, "selected", None)
        if (name and name in getattr(self, "blocks", {}) and not self.blocks[name].get("draft_of")
                and self.stack.get_visible_child_name() == "blocks"
                and self.blocks_stack.get_visible_child_name() == "detail"):
            return name
        return None

    def to_draft(self, args, secret, then, timeout):
        """§4.3 (owner 2026-11-17): a change to what the opened block's
        Websites and Apps tabs show goes to its draft; the first one makes
        the draft. Returns True when it took the command over."""
        block = self.opened_block()
        if not block:
            return False
        draft = draft_of_name(block)
        for table, makes in ((DRAFT_EDITS, True), (DRAFT_READS, False)):
            index = draft_position(args, table)
            if index is None or index >= len(args) or args[index] != block:
                continue
            moved = list(args)
            moved[index] = draft
            if draft in self.blocks or self.blocks[block].get("draft") == draft:
                self.run(moved, secret, then, timeout)
            elif makes:
                self.start_draft(None, then=lambda _o: self.run(moved, secret, then, timeout))
            else:
                return False  # a read with no draft: the block itself
            return True
        return False

    def run(self, args, secret=None, then=None, timeout=120):
        """Run a command in the background, then refresh.

        With a settings password set, the password remembered for this
        session goes first on standard input; commands that don't need it
        never read it. A command that needs it and doesn't have it fails,
        and finished() asks for it and runs the command again."""
        if self.to_draft(args, secret, then, timeout):
            return
        self.set_sensitive(False)
        stdin, cli_args = secret, args
        if self.settings_password and self.state and self.state.get("settings_password"):
            # --settings-line: the CLI reads it whether it needs it or not,
            # so it is never taken for the next password (a lock's).
            stdin = self.settings_password + "\n" + (secret or "")
            cli_args = ["--settings-line", *args]
        passwords = list(self.access_passwords)

        def work():
            ok, output = cli_with_access(passwords, cli_args, stdin, timeout)
            GLib.idle_add(self.finished, ok, output, then, (args, secret, timeout))

        threading.Thread(target=work, daemon=True).start()

    def finished(self, ok, output, then, request=None):
        self.set_sensitive(True)
        if not ok and request and ("needs the access password" in output
                                   or "wrong access password" in output):
            # A page's password that wasn't given yet (changing another
            # page's password asks for that one).
            area = output[output.rfind("(") + 1:output.rfind(")")] if "(" in output else ""
            password = self.ask_text(
                "Access password", f"This needs the password for {AREA_TITLES.get(area, area)}.",
                password=True)
            if password and self.try_access(password):
                args, secret, timeout = request
                self.run(args, secret, then, timeout)
            return False
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

    def confirm_lock(self, locks, option=None):
        """R-LOCK-12: the one confirmation for every lock. locks are lines
        "what is locked: kind, how long", as the CLI shows them. Returns True
        only when the user confirms; the caller then passes --yes. With
        option (a label), a tick box is shown too; whether it was ticked is
        left in self.lock_option."""
        self.lock_option = False
        if not locks:
            return True
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.WARNING,
            text="Set this lock?" if len(locks) == 1 else "Set these locks?",
            secondary_text="\n".join("• " + line for line in locks)
            + "\n\nWhile a lock is on, this can't be weakened.")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Lock", Gtk.ResponseType.OK)
        tick = None
        if option:
            tick = Gtk.CheckButton(label=option)
            dialog.get_message_area().pack_start(tick, False, False, 0)
            dialog.show_all()
        answer = dialog.run()
        self.lock_option = bool(tick and tick.get_active())
        dialog.destroy()
        return answer == Gtk.ResponseType.OK

    def acknowledge_no_confirmation(self, what):
        """R-LOCK-12: monk mode started by itself asks nothing, so setting it
        up needs the tick "I know this will not ask for confirmation"."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.WARNING,
            text="Monk mode will start without asking",
            secondary_text=f"{what} starts monk mode by itself, and it will not ask for confirmation "
                           "when it does.")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Set it up", Gtk.ResponseType.OK)
        tick = Gtk.CheckButton(label="I know this will not ask for confirmation")
        ok.set_sensitive(False)
        tick.connect("toggled", lambda t: ok.set_sensitive(t.get_active()))
        dialog.get_message_area().pack_start(tick, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        ticked = tick.get_active()
        dialog.destroy()
        return answer == Gtk.ResponseType.OK and ticked

    def ask_text(self, title, detail="", password=False, initial="", problem=None):
        """Asks for a line of text; "" when cancelled. problem, when given,
        says what is wrong with the text (None: nothing): OK waits and the
        reason shows under the box (R-GUI-1)."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=title, secondary_text=detail or None,
            message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
        entry = Gtk.Entry(activates_default=True, visibility=not password, text=initial)
        dialog.set_default_response(Gtk.ResponseType.OK)
        dialog.get_message_area().pack_start(entry, False, False, 0)
        if problem is not None:
            note = Gtk.Label(xalign=0, wrap=True)
            note.get_style_context().add_class("kc9k-note")
            dialog.get_message_area().pack_start(note, False, False, 0)
            ok = dialog.get_widget_for_response(Gtk.ResponseType.OK)

            def check(*_args):
                why = problem(entry.get_text().strip())
                note.set_text(why or "")
                ok.set_sensitive(why is None)

            entry.connect("changed", check)
            check()
        dialog.show_all()
        answer = dialog.run()
        value = entry.get_text() if password else entry.get_text().strip()
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK else ""

    def ask_duration(self, title, detail, seconds, minimum=60, maximum=None, zero=None,
                     presets=(), why_max=""):
        """R-GUI-1: asks for a length in days, hours and minutes; its
        seconds, or None when cancelled. OK waits until it reads right."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=title, secondary_text=detail,
            message_type=Gtk.MessageType.QUESTION)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        field = DurationField(seconds, minimum=minimum, maximum=maximum, zero=zero, presets=presets,
                              why_max=why_max)
        guard_button(ok, field)
        dialog.get_message_area().pack_start(field, False, False, 0)
        self.duration_field = field  # for tests
        dialog.show_all()
        answer = dialog.run()
        value = field.value
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK and value is not None else None

    def ask_nepsis(self, title, current, allow_lock):
        """Asks for a bypass detection setting, its whole ladder; returns the
        'nepsis set' arguments, or None when cancelled."""
        current = current or {}
        steps = [dict(step) for step in current.get("steps") or []] or [
            {"count": 10, "minutes": 5, "actions": [{"action": "warning"}]}]
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_buttons("Cancel", Gtk.ResponseType.CANCEL, "Save", Gtk.ResponseType.OK)
        dialog.set_default_size(620, 560)
        scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
        area = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10, border_width=16)
        scroll.add(area)
        dialog.get_content_area().pack_start(scroll, True, True, 0)
        intro = dim("A ladder of steps. The first time its number of things is stopped within its "
                    "time, step 1 acts; the next time, step 2; and so on, staying on the last. "
                    "Making it stricter is always allowed; anything else weakens it.")
        intro.set_max_width_chars(70)
        area.pack_start(intro, False, False, 0)
        steps_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        area.pack_start(steps_box, False, False, 0)
        cards, fields = [], []
        save = dialog.get_widget_for_response(Gtk.ResponseType.OK)
        save.get_style_context().add_class("suggested-action")

        def live_fields():
            """The fields that count: every step's numbers and the times of
            the ticked consequences, the quiet time and the lock."""
            out = []
            for _frame, count, within, actions in cards:
                out += [count, within]
                out += [minutes for check, minutes in actions.values() if minutes is not None and check.get_active()]
            return out + [f for f in fields if f is not None]

        def refresh_save(*_args):
            save.set_sensitive(all(f.is_valid() for f in live_fields()))

        def add_card(step):
            frame = Gtk.Frame(label=f"Step {len(cards) + 1}")
            grid = Gtk.Grid(column_spacing=10, row_spacing=4, border_width=8)
            count = NumberField(step.get("count", 3), 1, NEPSIS_MAX_COUNT, width=5)
            within = DurationField(step.get("minutes", 10) * 60, minimum=60, maximum=86400, width=9)
            head = Gtk.Box(spacing=6)
            head.pack_start(count, False, False, 0)
            head.pack_start(Gtk.Label(label="things stopped within", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
            head.pack_start(within, False, False, 0)
            grid.attach(head, 0, 0, 3, 1)
            count.on_change(refresh_save)
            within.on_change(refresh_save)
            chosen = {a["action"]: a.get("minutes", 0) for a in step.get("actions") or []}
            params = {a["action"]: a.get("param", "") for a in step.get("actions") or []}
            actions = {}
            for row, (key, label) in enumerate(CONSEQUENCES, start=1):
                check = Gtk.CheckButton(label=label, active=key in chosen)
                check.connect("toggled", refresh_save)
                grid.attach(check, 0, row, 1, 1)
                minutes = None
                if key in TIMED:
                    minutes = DurationField(
                        (chosen.get(key) or (30 if TIMED[key] else 0)) * 60, minimum=60,
                        maximum=30 * 86400 if key in ("monk", "mode") else 86400,
                        zero=None if TIMED[key] else "Just once", width=9)
                    grid.attach(minutes, 1, row, 1, 1)
                    minutes.on_change(refresh_save)
                if key == "mode":
                    # §3.9 Mode action (owner 2026-11-17): which mode.
                    combo = Gtk.ComboBoxText()
                    for name in self.mode_choices():
                        combo.append_text(name)
                    names = self.mode_choices()
                    if names:
                        combo.set_active(names.index(params.get("mode")) if params.get("mode") in names else 0)
                    grid.attach(combo, 2, row, 1, 1)
                    check.kc9k_param = lambda c=combo: c.get_active_text() or ""
                if key == "custom":
                    # §3.9 Custom action: what it opens or runs.
                    try:
                        items = [f"{list(i)[0]} {list(i.values())[0]}" for i in json.loads(params.get("custom") or "[]")]
                    except (ValueError, IndexError, AttributeError):
                        items = []
                    holder = {"items": items}
                    summary = dim("; ".join(items))
                    change = Gtk.Button(label="Change…")

                    def change_custom(_b, h=holder, s=summary, c=check):
                        got = self.ask_custom("Custom action", h["items"])
                        if got is not None:
                            h["items"] = got
                            s.set_text("; ".join(got))
                            c.set_active(True)
                    change.connect("clicked", change_custom)
                    grid.attach(change, 1, row, 1, 1)
                    grid.attach(summary, 2, row, 1, 1)
                    check.kc9k_param = lambda h=holder: json.dumps(
                        [{item.split(" ", 1)[0]: item.split(" ", 1)[1]} for item in h["items"]])
                actions[key] = (check, minutes)
            frame.add(grid)
            steps_box.pack_start(frame, False, False, 0)
            frame.show_all()
            cards.append((frame, count, within, actions))
            refresh_save()

        for step in steps:
            add_card(step)

        def remove_card(_button):
            if len(cards) > 1:
                frame = cards.pop()[0]
                steps_box.remove(frame)
                refresh_save()

        buttons = Gtk.Box(spacing=6)
        more = Gtk.Button(label="Add a step")
        more.connect("clicked", lambda _b: len(cards) < 10 and add_card(
            {"count": 3, "minutes": 10, "actions": [{"action": "close-browser"}]}))
        fewer = Gtk.Button(label="Remove the last step")
        fewer.connect("clicked", remove_card)
        buttons.pack_start(more, False, False, 0)
        buttons.pack_start(fewer, False, False, 0)
        area.pack_start(buttons, False, False, 0)

        quiet_row = Gtk.Box(spacing=6)
        quiet = DurationField(current.get("quiet_minutes", 0) * 60, minimum=60, maximum=30 * 86400,
                              zero="Never falls back", width=9)
        fields.append(quiet)
        quiet.on_change(refresh_save)
        quiet_row.pack_start(Gtk.Label(label="Fall back a step after quiet time of",
                                       valign=Gtk.Align.START, margin_top=6), False, False, 0)
        quiet_row.pack_start(quiet, False, False, 0)
        area.pack_start(quiet_row, False, False, 0)

        area.pack_start(heading("What counts", "medium"), False, False, 0)
        counted = current.get("detectors") or []
        detector_checks = {}
        for key, label in DETECTORS:
            check = Gtk.CheckButton(label=label, active=not counted or key in counted)
            detector_checks[key] = check
            area.pack_start(check, False, False, 0)
        lock = None
        if allow_lock:
            area.pack_start(heading("Timer lock", "medium"), False, False, 0)
            area.pack_start(dim("Can't be weakened until then."), False, False, 0)
            lock = TimerLockField()
            area.pack_start(lock, False, False, 0)
            fields.append(lock)
            lock.on_change(refresh_save)
        self.nepsis_fields = (cards, quiet, lock)  # for tests
        self.nepsis_lock_words = ""
        refresh_save()
        dialog.show_all()
        answer = dialog.run()
        args, problem = [], ""
        for number, (_frame, count, within, actions) in enumerate(cards, start=1):
            parts = []
            for key, (check, minutes) in actions.items():
                if check.get_active():
                    part = {"action": key}
                    if minutes is not None and minutes.minutes:
                        part["minutes"] = minutes.minutes
                    if hasattr(check, "kc9k_param"):
                        part["param"] = check.kc9k_param()
                        if not part["param"] or part["param"] == "[]":
                            problem = (f"Step {number}: choose the mode it runs." if key == "mode" else
                                       f"Step {number}: say what its custom action opens or runs.")
                    parts.append(part)
            if not parts:
                problem = f"Step {number} does nothing: tick at least one thing for it to do."
            # As JSON (--step-json): what a custom action runs may hold any character.
            args += ["--step-json", json.dumps({"count": count.value, "minutes": within.minutes, "actions": parts})]
        detectors = [key for key, check in detector_checks.items() if check.get_active()]
        if not detectors:
            problem = "Nothing is counted: tick at least one thing under What counts."
        elif len(detectors) < len(DETECTORS):
            args += ["--detect", ",".join(detectors)]
        if quiet.minutes:
            args += ["--quiet", f"{quiet.minutes}m"]
        if lock is not None:
            args += lock.flags()
            self.nepsis_lock_words = lock.describe()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return None
        if problem:
            self.message(problem, Gtk.MessageType.INFO)
            return None
        return args

    def mode_choices(self):
        """The modes a Mode action can run: monk mode, then the modes of
        blocks (Frozen Chicken is Lock the screen / Log out / Shut down)."""
        modes = [m["name"] for m in (self.state or {}).get("modes") or [] if not m.get("freeze")]
        return sorted(modes, key=lambda n: (n != "Monk mode", n.lower()))

    def ask_mode_run(self, title, mode=None, minutes=60, answer=None):
        """§3.9 Mode action: which mode, and for how long it runs, locked.
        Returns (mode, minutes) or None. answer: for tests."""
        if answer is not None:
            return answer
        choices = self.mode_choices()
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        box.set_border_width(14)
        box.pack_start(dim("The mode runs at once for this long, whatever its schedule and places say, and is "
                           "locked until then: it can't be switched off or weakened. Then it is as it was."),
                       False, False, 0)
        combo = Gtk.ComboBoxText()
        for name in choices:
            combo.append_text(name)
        if choices:
            combo.set_active(choices.index(mode) if mode in choices else 0)
        box.pack_start(combo, False, False, 0)
        length = DurationField(max(minutes, 1) * 60, minimum=60, maximum=30 * 86400, presets=(1800, 3600, 4 * 3600),
                               width=9)
        box.pack_start(length, False, False, 0)
        dialog.show_all()
        reply = dialog.run()
        chosen, valid, value = combo.get_active_text(), length.is_valid(), length.minutes
        dialog.destroy()
        if reply != Gtk.ResponseType.OK or not chosen:
            return None
        if not valid:
            self.message(f"How long: {length.problem}", Gtk.MessageType.INFO)
            return None
        return chosen, value

    def ask_custom(self, title, items=(), answer=None):
        """§3.9 Custom action: what it opens or runs. Returns the items
        ("open …"/"run …") or None. answer: for tests."""
        if answer is not None:
            return answer
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        box.set_border_width(14)
        box.pack_start(dim("Opened or run as you, in your desktop session. It can do nothing you couldn't do "
                           "yourself: a KC9K command in it meets every lock and password."), False, False, 0)
        editor = CustomEditor(self, items)
        box.pack_start(editor, False, False, 0)
        dialog.set_default_size(560, -1)
        dialog.show_all()
        reply = dialog.run()
        chosen = editor.items
        dialog.destroy()
        if reply != Gtk.ResponseType.OK:
            return None
        if not chosen:
            self.message("Add something to open or run first.", Gtk.MessageType.INFO)
            return None
        return chosen

    def monk_mode(self, _button):
        seconds = self.ask_duration(
            "Monk mode", "Every website is blocked and the internet connection is cut (Wi-Fi off, "
            "Ethernet down, firewall closed), for this long. It can't be ended early; a running "
            "monk mode can only be extended.",
            3600, minimum=60, maximum=30 * 86400, presets=(1800, 3600, 4 * 3600, 86400))
        if not seconds:
            return
        until = datetime.datetime.now() + datetime.timedelta(seconds=seconds)
        # R-LOCK-12: monk mode is a lock like any other; the command needs --yes.
        if self.confirm_lock([f"monk mode (no websites, no internet): {duration_words(seconds)}, "
                              f"until {date_words(until)}"]):
            self.run(["monk", "--for", f"{seconds // 60}m", "--yes"])

    def change_global_nepsis(self, _button):
        args = self.ask_nepsis("Bypass prevention", self.state.get("nepsis"), True)
        args = self.acknowledge_monk_steps(args)
        if args and ("--for" in args or "--until" in args):
            if not self.confirm_lock([f"bypass prevention: {self.nepsis_lock_words}"]):
                return
            args.append("--yes")
        if args:
            self.run(["bypass", "set", *args], then=self.queued_note)

    def acknowledge_monk_steps(self, args):
        """Bypass prevention steps with monk mode or a Mode action need the
        acknowledgement."""
        steps = [a for i, a in enumerate(args or []) if i and args[i - 1] in ("--step", "--step-json")]
        if any(re.search(r"\bmonk\b", step) or '"action": "mode"' in step for step in steps):
            if not self.acknowledge_no_confirmation("A bypass prevention step"):
                return None
            return args + ["--no-confirm-ok"]
        return args

    def change_block_nepsis(self, _button):
        block = self.blocks.get(self.selected)
        if block:
            args = self.acknowledge_monk_steps(
                self.ask_nepsis(f"Bypass prevention for {block['name']}", block.get("nepsis"), False))
            if args:
                self.run(["bypass", "set", "--block", block["name"], *args], then=self.queued_note)

    def nepsis_off(self, _button, block=None):
        if self.confirm("Turn bypass prevention off?", "This weakens it, so it isn't possible while "
                        "it is locked (or, for a block, while the block is locked).", "Turn off"):
            self.run(["bypass", "off"] + (["--block", block()] if block else []), then=self.queued_note)

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
        self.lock_when_idle()
        if hasattr(self, "circadian"):
            self.circadian.refresh()
        if self.busy:
            # A read already running may predate the last change: read
            # again when it lands, so the last command is always shown.
            self.refresh_again = True
            return True
        self.busy = True
        self.refresh_again = False

        passwords = list(self.access_passwords)

        reveal = dict(self.block_passwords)

        def work():
            args = ["gui-state"]
            for name in reveal:
                args += ["--reveal", name]
            ok, output = cli_with_access(passwords, args, "".join(p + "\n" for p in reveal.values()) or None)
            GLib.idle_add(self.apply_state, ok, output, passwords)

        threading.Thread(target=work, daemon=True).start()
        return True

    def extension_dialog(self):
        """§4.10: the Killer Chicken extension isn't running: offer to
        install a supported browser, or show the Browsers list."""
        none = not any(b.get("installed") for b in (self.state or {}).get("browsers") or [])
        names = (self.state or {}).get("extension_missing_in") or []
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.WARNING, buttons=Gtk.ButtonsType.NONE,
            text="The Killer Chicken extension isn't running",
            secondary_text=("No supported browser is installed. LibreWolf runs the extension; installing it "
                            "takes a few minutes." if none else
                            f"{' and '.join(names) or 'A supported browser'} has run for a while without it "
                            "checking in, and your blocks need it. Close and open the browser again, or see "
                            "under Settings > Browsers which have it."))
        dialog.add_button("Later", Gtk.ResponseType.CANCEL)
        dialog.add_button("Install LibreWolf" if none else "Show browsers", Gtk.ResponseType.OK)
        answer = dialog.run()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK:
            if none:
                self.run(["browsers", "install", "librewolf"], timeout=900)
            else:
                self.stack.set_visible_child_name("settings")
        return False

    def apply_state(self, ok, output, passwords=()):
        self.busy = False
        if getattr(self, "refresh_again", False):
            # Read before the last change: show the newer state instead.
            self.refresh()
            return False
        if list(passwords) != self.access_passwords:
            # Locked or unlocked while this was being read: read again
            # rather than show what the old passwords opened.
            self.refresh()
            return False
        if not ok:
            self.show_banner(f"Can't read the state: {output}", Gtk.MessageType.ERROR)
            return False
        if output == self.state_json:
            return False
        self.state_json = output
        self.state = json.loads(output)
        tray = getattr(self.get_application(), "tray", None)
        if tray is not None:
            on = [b["name"] for b in self.state.get("blocks", []) if b.get("active")]
            tray.set_status(f"On: {', '.join(on)}" if on else "No block is on")
        # A password that opens nothing any more (it was changed) is dropped.
        for index in self.state.get("access_unused") or []:
            if index < len(passwords) and passwords[index] in self.access_passwords:
                self.access_passwords.remove(passwords[index])
        self.access = self.state.get("access") or {}
        self.update_covers()
        if self.state.get("extension_missing") and not getattr(self, "extension_asked", False):
            self.extension_asked = True  # once per session
            GLib.idle_add(self.extension_dialog)
        self.blocks = {block["name"]: block for block in self.state["blocks"]}
        self.updating = True
        try:
            frozen = self.state["freeze_seconds"]
            if frozen:
                self.show_banner(f"Frozen Chicken is on for {fmt_seconds(frozen)} more.",
                                 Gtk.MessageType.WARNING)
            else:
                self.banner.set_revealed(False)
            for area, update in (("dashboard", self.update_dashboard),
                                 ("blocks", self.update_block_list),
                                 ("clusters", self.update_clusters),
                                 ("schedules", self.update_schedules_page),
                                 ("places", self.update_places_page),
                                 ("device", self.update_device),
                                 ("stats", self.update_stats_setting),
                                 ("settings", self.update_settings),
                                 ("hotkeys", self.update_hotkeys_page)):
                if not self.locked(area):
                    update()
        finally:
            self.updating = False
        return False

    def show_banner(self, text, kind):
        self.banner_label.set_text(text)
        self.banner.set_message_type(kind)
        self.banner.set_revealed(True)

    def build_circadian(self):
        self.circadian = CircadianPage(self)
        return self.circadian

    # Schedules and places pages (§4.5; owner 2026-11-17) ---------------
    def build_schedules_page(self):
        scroll, box = page_box()
        page_header(box, "Schedules", "Weekly windows kept outside blocks, such as Evening or Weekend. Apply "
                    "them to blocks and modes; changing one here changes everything using it.",
                    help="A block or mode can use any number of schedules beside its own windows (Apply "
                         "schedule… in its Automation). Where they overlap they count once. A change "
                         "that weakens a locked block using the schedule is refused as a whole. Deleting a "
                         "schedule leaves its windows to what used it, as its own.")
        self.schedule_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.pack_start(self.schedule_cards, False, False, 0)
        new = Gtk.Button(label="New schedule…", halign=Gtk.Align.START)
        new.get_style_context().add_class("suggested-action")
        new.connect("clicked", self.create_schedule)
        box.pack_start(new, False, False, 0)
        self.schedule_editor = AutomationEditor(self, "schedule")
        self.schedule_editor_title = heading("", "medium")
        self.schedule_editor_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8, no_show_all=True)
        self.schedule_editor_box.pack_start(self.schedule_editor_title, False, False, 0)
        self.schedule_editor_box.pack_start(card(self.schedule_editor), False, False, 0)
        box.pack_start(self.schedule_editor_box, False, False, 0)
        self.shown_schedule, self.schedule_cards_shown = None, None
        return scroll

    def update_schedules_page(self):
        schedules = self.state.get("schedules") or []
        names = [sc["name"] for sc in schedules]
        if self.shown_schedule not in names:
            self.shown_schedule = names[0] if names else None
        key = json.dumps(schedules, sort_keys=True) + str(self.shown_schedule)
        if key != self.schedule_cards_shown:
            self.schedule_cards_shown = key
            for child in self.schedule_cards.get_children():
                self.schedule_cards.remove(child)
            for sc in schedules:
                self.schedule_cards.pack_start(self.schedule_card(sc), False, False, 0)
            if not schedules:
                self.schedule_cards.pack_start(dim("No schedules yet. Make Evening, Weekend or Work hours "
                                                   "once, then apply them to any block or mode."), False, False, 0)
            self.schedule_cards.show_all()
        chosen = next((sc for sc in schedules if sc["name"] == self.shown_schedule), None)
        self.schedule_editor_box.set_visible(chosen is not None)
        if chosen is not None:
            self.schedule_editor_title.set_text(chosen["name"])
            self.schedule_editor.show_all()
            self.schedule_editor_title.show()
            self.schedule_editor_box.get_children()[1].show()
            self.schedule_editor.show_data(chosen["name"], chosen)

    def schedule_card(self, sc):
        name = sc["name"]
        title = Gtk.Button(label=name, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                           tooltip_text="Show and change its windows")
        title.get_style_context().add_class("kc9k-card-name")
        title.connect("clicked", lambda _b: self.open_schedule(name))
        top = Gtk.Box(spacing=10)
        top.pack_start(title, False, False, 0)
        more = Gtk.MenuButton(relief=Gtk.ReliefStyle.NONE, valign=Gtk.Align.CENTER, tooltip_text="More")
        more.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.MENU))
        menu = Gtk.Menu()
        for label, action in (("Rename…", lambda: self.rename_schedule(name)),
                              ("Delete…", lambda: self.delete_schedule(name))):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", lambda _i, a=action: a())
            menu.append(item)
        menu.show_all()
        more.set_popup(menu)
        top.pack_end(more, False, False, 0)
        if name == self.shown_schedule:
            top.pack_end(pill("Shown below", "on"), False, False, 0)
        used = (sc.get("blocks") or []) + [f"mode {m}" for m in sc.get("modes") or []]
        lines = [dim(schedule_summary(sc.get("windows") or [])),
                 dim("Used by " + ", ".join(used) if used else "Not used yet")]
        return card(top, *lines, spacing=4)

    def open_schedule(self, name):
        self.shown_schedule = name
        self.stack.set_visible_child_name("schedules")
        self.schedule_cards_shown = None
        self.update_schedules_page()

    def create_schedule(self, _button):
        name = self.ask_text("New schedule", "A name, such as Evening or Weekend. Then give it windows below.")
        if name:
            self.run(["schedule", "create", name], then=lambda _o: self.open_schedule(name))

    def rename_schedule(self, name):
        new = self.ask_text("Rename the schedule", initial=name)
        if new and new != name:
            self.run(["schedule", "rename", name, new], then=lambda _o: self.open_schedule(new))

    def delete_schedule(self, name):
        if self.confirm(f"Delete the schedule {name}?", "Blocks and modes using it keep its windows as their "
                        "own: nothing they block changes.", "Delete", destructive=True):
            self.run(["schedule", "delete", name])

    def build_places_page(self):
        scroll, box = page_box()
        page_header(box, "Places", "Where the computer is, known by the Wi-Fi networks in range. Blocks and "
                    "modes are switched by being at one.",
                    help="A place is the set of Wi-Fi networks in range when you save it: yours and the "
                         "neighbours'. The computer is there when as many of them as you choose are in range "
                         "again. Add places to a block or mode in its Automation. Needing more networks "
                         "makes a place harder to recognise: refused while a block using it is locked. With "
                         "no network seen, places count as whatever keeps their blocks enabled.")
        save = Gtk.Button(label="Save where I am now as…", halign=Gtk.Align.START)
        save.get_style_context().add_class("suggested-action")
        save.connect("clicked", lambda _b: self.save_place_alone())
        box.pack_start(save, False, False, 0)
        self.place_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.pack_start(self.place_cards, False, False, 0)
        self.place_cards_shown = None
        return scroll

    def update_places_page(self):
        places = self.state.get("places") or []
        key = json.dumps(places, sort_keys=True)
        if key == self.place_cards_shown:
            return
        self.place_cards_shown = key
        for child in self.place_cards.get_children():
            self.place_cards.remove(child)
        for p in places:
            self.place_cards.pack_start(self.place_card(p), False, False, 0)
        if not places:
            self.place_cards.pack_start(dim("No places yet. Save where the computer is now: home, the "
                                            "library, work."), False, False, 0)
        self.place_cards.show_all()

    def place_card(self, p):
        name = p["name"]
        top = Gtk.Box(spacing=10)
        title = Gtk.Label(label=name, xalign=0)
        title.get_style_context().add_class("kc9k-card-name")
        top.pack_start(title, False, False, 0)
        if p.get("here"):
            top.pack_start(pill("You are here", "on"), False, False, 0)
        more = Gtk.MenuButton(relief=Gtk.ReliefStyle.NONE, valign=Gtk.Align.CENTER, tooltip_text="More")
        more.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.MENU))
        menu = Gtk.Menu()
        for label, action in (("Add the networks in range now", lambda: self.run(["place", "add", name], timeout=60)),
                              ("Remove…", lambda: self.remove_place(name))):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", lambda _i, a=action: a())
            menu.append(item)
        menu.show_all()
        more.set_popup(menu)
        top.pack_end(more, False, False, 0)
        need_row = Gtk.Box(spacing=6)
        need_row.pack_start(Gtk.Label(label="There when", valign=Gtk.Align.START, margin_top=6), False, False, 0)
        need = NumberField(p.get("need", 2), 1, max(p.get("networks", 1), 1), width=4)
        need_row.pack_start(need, False, False, 0)
        need_row.pack_start(Gtk.Label(label=f"of its {p.get('networks', 0)} networks are in range",
                                      valign=Gtk.Align.START, margin_top=6), False, False, 0)
        need_save = Gtk.Button(label="Save", valign=Gtk.Align.START)
        need_save.connect("clicked", lambda _b: self.run(["place", "need", name, str(need.get_value_as_int())]))
        guard_button(need_save, need)
        need_row.pack_start(need_save, False, False, 0)
        modes = p.get("modes") or []
        mode_blocks = {b for m in self.state.get("modes") or [] if m["name"] in modes for b in m.get("blocks") or []}
        used = [b for b in p.get("blocks") or [] if b not in mode_blocks] + [f"mode {m}" for m in modes]
        return card(top, flow_row(need_row), dim("Used by " + ", ".join(used) if used else
                                                 "Not used yet: add it to a block or mode in its Automation."),
                    spacing=6)

    def remove_place(self, name):
        if self.confirm(f"Forget the place {name}?", "Blocks and modes using it lose it: refused while that "
                        "weakens a locked block.", "Remove", destructive=True):
            self.run(["place", "remove", name])

    def save_place_alone(self):
        """Saves the networks in range as a place, for no block."""
        name = self.ask_text("Save this place", "A name for where the computer is now, such as home or library. "
                             "Saving under a name that exists adds the networks seen now.")
        if name:
            self.run(["place", "add", name], timeout=60, then=lambda out: self.message(out, Gtk.MessageType.INFO))

    def page_changed(self, *_args):
        row = getattr(self, "nav_rows", {}).get(self.stack.get_visible_child_name())
        if row is not None and self.nav.get_selected_row() is not row:
            self.nav.select_row(row)
        if hasattr(self, "circadian"):
            self.circadian.refresh()
        if self.stack.get_visible_child_name() in ("stats", "dashboard"):
            self.refresh_stats()

    # Access passwords ----------------------------------------------------
    def guarded(self, name, title, page):
        """The page under a cover that asks for its password while locked."""
        stack = Gtk.Stack()
        stack.add_named(page, "page")
        cover = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12,
                        halign=Gtk.Align.CENTER, valign=Gtk.Align.CENTER)
        cover.pack_start(Gtk.Image.new_from_icon_name("changes-prevent-symbolic", Gtk.IconSize.DIALOG),
                         False, False, 0)
        text = Gtk.Label(justify=Gtk.Justification.CENTER)
        cover.pack_start(text, False, False, 0)
        row = Gtk.Box(spacing=6)
        entry = Gtk.Entry(visibility=False, placeholder_text="Password", width_chars=24,
                          input_purpose=Gtk.InputPurpose.PASSWORD)
        button = Gtk.Button(label="Unlock")
        button.get_style_context().add_class("suggested-action")
        unlock = lambda *_args: self.unlock_from(entry)
        entry.connect("activate", unlock)
        button.connect("clicked", unlock)
        row.pack_start(entry, True, True, 0)
        row.pack_start(button, False, False, 0)
        cover.pack_start(row, False, False, 0)
        stack.add_named(cover, "cover")
        self.covers[name] = (stack, text, entry, title)
        return stack

    def locked(self, area):
        # The Clusters page belongs to the Blocks area.
        return self.access.get({"clusters": "blocks", "schedules": "blocks", "places": "blocks",
                                "circadian": "settings", "hotkeys": "settings"}.get(area, area)) == "locked"

    def update_covers(self):
        app_locked = self.access.get("app") == "locked"
        for name, (stack, text, _entry, title) in self.covers.items():
            if app_locked:
                text.set_markup("<b>Killer Chicken 9000 is locked.</b>\nEnter its password to open it.")
            else:
                text.set_markup(f"<b>{title} is locked.</b>\nEnter its password to open it.")
            stack.set_visible_child_name("cover" if self.locked(name) else "page")
        self.lock_button.set_visible(bool(self.access_passwords))
        if hasattr(self, "access_status"):
            self.update_access_section()

    def unlock_from(self, entry):
        password = entry.get_text()
        entry.set_text("")
        if password and not self.try_access(password):
            self.message("That password opens nothing.")
            entry.grab_focus()

    def try_access(self, password):
        """Keeps a password if it opens something; False if it doesn't."""
        if password in self.access_passwords:
            return True
        passwords = self.access_passwords + [password]
        ok, output = cli_with_access(passwords, ["gui-state"])
        if not ok:
            return False
        if len(passwords) - 1 in (json.loads(output).get("access_unused") or []):
            return False
        self.access_passwords = passwords
        self.state_json = None
        self.apply_state(True, output, passwords)
        self.refresh_stats()
        return True

    def note_input(self, _widget, event):
        if event.type in (Gdk.EventType.KEY_PRESS, Gdk.EventType.BUTTON_PRESS, Gdk.EventType.SCROLL,
                          Gdk.EventType.MOTION_NOTIFY):
            self.last_input = GLib.get_monotonic_time()

    def lock_when_idle(self):
        """Forgets the access passwords after the idle minutes chosen under
        Settings (0: never)."""
        minutes = self.get_application().prefs.get("access_idle_minutes", 0)
        idle = (GLib.get_monotonic_time() - self.last_input) / 60e6
        if minutes and self.access_passwords and idle >= minutes:
            self.lock_access()

    def lock_access(self, _button=None):
        self.access_passwords = []
        self.block_passwords = {}
        self.state_json = None
        self.refresh()

    def access_dialog(self, remove):
        """Asks which areas (and, to set, which password). Returns (areas, password)."""
        dialog = Gtk.Dialog(title="Remove access passwords" if remove else "Set an access password",
                            transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Remove" if remove else "Set", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("destructive-action" if remove else "suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        for side in ("margin_start", "margin_end", "margin_top", "margin_bottom"):
            box.set_property(side, 16)
        box.pack_start(dim("Remove the password of:" if remove else
                           "Everything ticked gets this one password. Do it again with "
                           "other boxes ticked for a different one."), False, False, 0)
        checks = {}
        for area in AREAS:
            has = self.access.get(area, "none") != "none"
            label = ("Opening the program" if area == "app" else AREA_TITLES[area])
            check = Gtk.CheckButton(label=label + (" (has a password)" if has and not remove else ""))
            if remove:
                check.set_sensitive(has)
            checks[area] = check
            box.pack_start(check, False, False, 0)
        first = Gtk.Entry(visibility=False, placeholder_text="Password (at least 4 characters)",
                          activates_default=True)
        again = Gtk.Entry(visibility=False, placeholder_text="Repeat it", activates_default=True)
        if not remove:
            box.pack_start(first, False, False, 0)
            box.pack_start(again, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        areas = [area for area, check in checks.items() if check.get_active()]
        password, repeat = first.get_text(), again.get_text()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or not areas:
            return None, None
        if not remove and password != repeat:
            self.message("The two passwords differ; nothing changed.", Gtk.MessageType.INFO)
            return None, None
        if not remove and len(password) < 4:
            self.message("Use at least 4 characters.", Gtk.MessageType.INFO)
            return None, None
        return areas, password

    def set_access_password(self, _button):
        areas, password = self.access_dialog(remove=False)
        if areas:
            # Remembered at once, so the page being worked on doesn't lock.
            self.run(["access", "set", *areas], password + "\n",
                     then=lambda _o: self.access_passwords.append(password)
                     if password not in self.access_passwords else None)

    def remove_access_password(self, _button):
        areas, _password = self.access_dialog(remove=True)
        if areas:
            self.run(["access", "remove", *areas])

    def update_access_section(self):
        with_password = [("Opening the program" if a == "app" else AREA_TITLES[a])
                         for a in AREAS if self.access.get(a, "none") != "none"]
        if with_password:
            self.access_status.set_markup("<b>On for:</b> " + GLib.markup_escape_text(", ".join(with_password)) + ".")
        else:
            self.access_status.set_text("Not set.")
        self.access_remove_button.set_visible(bool(with_password))

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
        # The details scroll, so the window fits small and scaled screens.
        # Never wider than the window (R-GUI-3: nothing cut off at the
        # right): the content wraps and reflows instead.
        detail_scroll = Gtk.ScrolledWindow(hscrollbar_policy=Gtk.PolicyType.NEVER)
        detail_scroll.add(self.detail)
        paned.pack2(detail_scroll, True, False)
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
        self.start_button = Gtk.Button(label="Start…")
        self.start_button.get_style_context().add_class("suggested-action")
        self.start_button.connect("clicked", self.start_block)
        self.stop_button = Gtk.Button(label="Stop")
        self.stop_button.connect("clicked", self.stop_block)
        self.unlock_button = Gtk.Button(label="Unlock…")
        self.unlock_button.connect("clicked", self.unlock_block)
        # §4.3 (owner 2026-11-17): Automation in the block's menu; greyed
        # out in a mode, which automates it.
        self.automation_button = Gtk.Button(label="Automation")
        self.automation_button.connect("clicked", lambda _b: self.open_automation())
        for button in (self.start_button, self.stop_button, self.unlock_button, self.automation_button):
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
        self.detail.pack_start(tabs, True, True, 0)

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

    # Modes (§3.2, §3.8, §4.2; owner 2026-11-17) -------------------------
    def mode_card(self, mode):
        """A mode on the Blocks page: its name (opens it), its switch, what
        switches it, its blocks; blocks dropped on it go in."""
        name = mode["name"]
        title = Gtk.Button(label=name, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                           tooltip_text="Open the mode")
        title.get_style_context().add_class("kc9k-card-name")
        title.connect("clicked", lambda _b: self.open_mode(name))
        icon = Gtk.Image.new_from_icon_name("weather-clear-night-symbolic" if mode.get("monk") else
                                            "weather-snow-symbolic" if mode.get("freeze") else
                                            "view-grid-symbolic", Gtk.IconSize.LARGE_TOOLBAR)
        top = Gtk.Box(spacing=10)
        top.pack_start(icon, False, False, 0)
        top.pack_start(title, False, False, 0)
        switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        blocks = mode.get("blocks") or []
        if mode.get("freeze"):
            left = mode.get("freeze_left") or 0
            switch.set_active(bool(mode.get("running")))
            switch.set_sensitive(not mode.get("running"))
            switch.set_tooltip_text(f"Frozen for {fmt_seconds(left)} more: it can't end early" if left else
                                    "Freeze the computer until a time you choose (at most a week)")
            state_text = f"On, {fmt_seconds(left)} left" if left else "Off"
        elif mode.get("monk"):
            # R-MONK-3 (owner 2026-11-17, later): switched like any mode;
            # only a run for a set time is locked.
            left = mode.get("monk_left") or 0
            switch.set_active(bool(mode.get("active")) or bool(left))
            switch.set_sensitive(not left)
            switch.set_tooltip_text(f"Locked for {fmt_seconds(left)} more: it was started for a time" if left else
                                    "Switch monk mode off" if mode.get("active") else
                                    "Switch monk mode on: no websites and no internet while it runs")
            state_text = (f"Locked, {fmt_seconds(left)} left" if left else
                          ("Running" if mode.get("running") else "On, waiting for its windows or places")
                          if mode.get("active") else "Off")
        else:
            on, total = mode.get("on", 0), len(blocks)
            run_left = mode.get("run_left") or 0
            switch.set_active(total > 0 and on == total)
            switch.set_sensitive(total > 0 and not run_left)
            switch.set_tooltip_text(f"Run by a Mode action, locked for {fmt_seconds(run_left)} more" if run_left else
                                    "Switch its blocks off (locked ones stay on)" if on == total and total else
                                    "Switch its blocks on (without locks)")
            state_text = (f"Running, locked, {fmt_seconds(run_left)} left" if run_left else
                          "No blocks yet" if not total else "On" if on == total else "Off" if not on else
                          f"{on} of {total} on")
        switch.connect("state-set", self.toggle_mode_card, mode)
        state = Gtk.Label(label=state_text)
        state.get_style_context().add_class("dim-label")
        top.pack_end(switch, False, False, 0)
        top.pack_end(state, False, False, 0)
        if mode.get("freeze"):
            what = ("No scheduled freezes: switched on with an end time" if mode.get("continuous") else
                    "Scheduled: " + schedule_summary(mode.get("windows") or []).split(" · ")[0])
        elif mode.get("monk"):
            what = ("No windows or places: on until switched off" if mode.get("continuous") else
                    "Automated: " + (", ".join(mode.get("schedules") or []) or schedule_summary(mode.get("windows") or [])
                                     .split(" · ")[0] or "") + (" · at " + ", ".join(mode["places"]) if mode.get("places") else ""))
        else:
            what = ("Continuous" if mode.get("continuous") else "Automated") + " · " + (
                f"{len(blocks)} {'block' if len(blocks) == 1 else 'blocks'}: {', '.join(blocks)}" if blocks else
                "drag blocks here")
            if mode.get("locked"):
                what += f" · {mode['locked']} locked"
        line = Gtk.Button(label=what, relief=Gtk.ReliefStyle.NONE, halign=Gtk.Align.START,
                          tooltip_text="Open the mode: its blocks and what switches them")
        line.get_style_context().add_class("kc9k-link")
        line.connect("clicked", lambda _b: self.open_mode(name))
        box = card(top, line, spacing=6)
        if not mode.get("monk") and not mode.get("freeze"):
            box.drag_dest_set(Gtk.DestDefaults.ALL, [BLOCK_TARGET], Gdk.DragAction.MOVE)
            box.connect("drag-data-received",
                        lambda _w, _c, _x, _y, data, _i, _t: self.move_to_mode(data.get_data().decode(), name))
        return box

    def create_mode(self, _button, block=None):
        name = self.ask_text("New mode", "A name, such as Evening or Work. Then drag blocks onto it; one "
                             "schedule and places switch them together.")
        if not name:
            return
        if block:
            self.run(["mode", "create", name], then=lambda _o: self.move_to_mode(block, name))
        else:
            self.run(["mode", "create", name], then=lambda _o: self.open_mode(name))

    def move_to_mode(self, block, mode):
        """R-MODE-1: the block gets the mode's automation (refused if that
        weakens it while it is locked)."""
        if not block or block not in self.blocks:
            return
        if self.blocks[block].get("mode") == mode:
            return
        if self.confirm(f"Put {block} into {mode}?",
                        f"It then has the mode's automation: its own windows and places give way to "
                        f"{mode}'s. Taken out later, it keeps the mode's.", "Put it in"):
            self.block_cards_shown = None
            self.run(["mode", "add", mode, block])

    def take_out_of_mode(self, block):
        self.block_cards_shown = None
        self.run(["mode", "take-out", block])

    def toggle_mode_card(self, switch, wanted, mode):
        if self.updating:
            return False
        self.block_cards_shown = None
        if mode.get("freeze"):
            if wanted:
                self.freeze_dialog()
            return True
        if mode.get("monk") and wanted:
            when = ("while its windows and places have it on" if not mode.get("continuous") else
                    "until you switch it off")
            if not self.confirm_lock([f"monk mode, switched on: no websites and no internet {when}"]):
                return True
            self.run(["mode", "on", mode["name"], "--yes"])
        else:
            self.run(["mode", "on" if wanted else "off", mode["name"]])
        return True

    def find_mode(self, name):
        for m in (self.state or {}).get("modes") or []:
            if m["name"] == name:
                return m
        return None

    def open_mode(self, name):
        self.shown_mode = name
        # Made a moment ago: the state that has it is on its way.
        self.mode_wait = None if self.find_mode(name) else name
        self.stack.set_visible_child_name("blocks")
        self.blocks_stack.set_visible_child_name("mode")
        self.update_mode_view()

    def build_mode_view(self):
        scroll, box = page_box()
        back = Gtk.Button(label="← All blocks", halign=Gtk.Align.START, relief=Gtk.ReliefStyle.NONE)
        back.get_style_context().add_class("kc9k-link")
        back.connect("clicked", lambda _b: self.blocks_stack.set_visible_child_name("cards"))
        box.pack_start(back, False, False, 0)
        head = Gtk.Box(spacing=10)
        self.mode_title = Gtk.Label(xalign=0)
        self.mode_title.get_style_context().add_class("kc9k-page-title")
        head.pack_start(self.mode_title, False, False, 0)
        self.mode_menu = Gtk.MenuButton(relief=Gtk.ReliefStyle.NONE, valign=Gtk.Align.CENTER, tooltip_text="More")
        self.mode_menu.add(Gtk.Image.new_from_icon_name("view-more-symbolic", Gtk.IconSize.MENU))
        menu = Gtk.Menu()
        for label, action in (("Rename…", self.rename_mode), ("Delete…", self.delete_mode),
                              ("Save automation as preset…", lambda: self.save_preset(
                                  "automation", getattr(self, "shown_mode", None), mode=True)),
                              ("Use an automation preset…", lambda: self.add_from_preset(
                                  "automation", mode=getattr(self, "shown_mode", None)))):
            item = Gtk.MenuItem(label=label)
            item.connect("activate", lambda _i, a=action: a())
            menu.append(item)
        menu.show_all()
        self.mode_menu.set_popup(menu)
        head.pack_end(self.mode_menu, False, False, 0)
        # R-MODE-3 (owner 2026-11-17, later): a mode of blocks is locked by
        # locking all its blocks at once.
        self.mode_lock_button = Gtk.Button(label="Lock…", valign=Gtk.Align.CENTER)
        self.mode_lock_button.set_tooltip_text("Lock every block of the mode: it then can't be switched off or "
                                               "weakened until they are unlocked")
        self.mode_lock_button.connect("clicked", lambda _b: self.lock_mode())
        head.pack_end(self.mode_lock_button, False, False, 0)
        self.mode_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.mode_switch.connect("state-set", lambda sw, wanted: self.toggle_mode_card(
            sw, wanted, self.find_mode(getattr(self, "shown_mode", None)) or {"name": ""}))
        head.pack_end(self.mode_switch, False, False, 0)
        box.pack_start(head, False, False, 0)
        self.mode_status = dim("")
        box.pack_start(self.mode_status, False, False, 0)
        # A mode of blocks: its blocks as cards.
        self.mode_block_cards = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        add = Gtk.Button(label="Add blocks…")
        add.connect("clicked", self.add_blocks_to_mode)
        self.mode_take_out_button = take_out = Gtk.Button(label="Take out…")
        take_out.set_tooltip_text("Take a block out of the mode")
        take_out.connect("clicked", self.take_blocks_out_of_mode)
        mode_buttons = Gtk.Box(spacing=6)
        mode_buttons.pack_start(add, False, False, 0)
        mode_buttons.pack_start(take_out, False, False, 0)
        self.mode_blocks_section = section(
            "Its blocks", self.mode_block_cards, mode_buttons, plain=True,
            help="They share the mode's automation below: changing it changes all of them, and a change "
                 "that weakens one of them while it is locked is refused for all. Each keeps its own lists, "
                 "apps, breaks and locks. Take out… takes one out again.")
        box.pack_start(self.mode_blocks_section, False, False, 0)
        # Monk mode: what it is, and starting it for a time.
        self.mode_monk_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        self.mode_monk_box.pack_start(dim(
            "Every website is blocked and the internet connection is cut (Wi-Fi off, Ethernet down, firewall "
            "closed). Switch it on like any mode: with windows or places below it runs in them by itself, "
            "without them until you switch it off. Started for a time it is locked: it can't be switched off "
            "until then. While its places decide, Wi-Fi stays on to see where you are; the firewall still "
            "cuts it."), False, False, 0)
        monk_now = Gtk.Button(label="Run now, locked for a time…", halign=Gtk.Align.START)
        monk_now.connect("clicked", self.monk_mode)
        self.mode_monk_box.pack_start(monk_now, False, False, 0)
        box.pack_start(self.mode_monk_box, False, False, 0)
        self.mode_freeze_box = self.guarded("device", FROZEN_CHICKEN, self.build_device())
        box.pack_start(self.mode_freeze_box, False, False, 0)
        self.mode_editors = Gtk.Stack()
        self.mode_editor = AutomationEditor(self, "mode")
        self.monk_editor = AutomationEditor(self, "monk")
        self.mode_editors.add_named(self.mode_editor, "mode")
        self.mode_editors.add_named(self.monk_editor, "monk")
        # §3.9 Custom action (owner 2026-11-17): what the mode opens or runs
        # each time it becomes enabled.
        self.mode_custom = CustomEditor(self, on_change=self.set_mode_custom)
        self.mode_custom_section = section(
            "When it becomes enabled", dim("Open sites, files and folders, or run programs, each time the mode "
                                           "becomes enabled (also once at sign-in, when it is enabled then)."),
            self.mode_custom)
        box.pack_start(self.mode_custom_section, False, False, 0)
        # R-MODE-5 (owner 2026-11-17): the mode's wallpapers, laid out as
        # circadian rhythm's, and which win (one setting with circadian
        # rhythm's Override modes).
        self.mode_wallpapers = WallpaperEditor(self, lambda: ["mode", "wallpaper", getattr(self, "shown_mode", None) or ""])
        override_row = Gtk.Box(spacing=12)
        override_row.pack_start(Gtk.Label(label="Override circadian rhythm wallpapers", xalign=0), False, False, 0)
        self.mode_override = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.mode_override.connect("state-set", self.toggle_mode_override)
        override_row.pack_end(self.mode_override, False, False, 0)
        self.mode_wallpaper_section = section(
            "Wallpapers", dim("While the mode is enabled, each picture is the wallpaper between its two times."),
            self.mode_wallpapers, override_row,
            help="Drag a bar to move it, or an end to change when it starts or stops (Shift: to the minute). "
                 "Of several enabled modes, the one enabled last shows its wallpaper. Override circadian "
                 "rhythm wallpapers is one setting for all modes, the same as Override modes on the "
                 "circadian rhythm page the other way round. A mode switched by places shows its wallpaper "
                 "once KC9K knows where you are; until then (no Wi-Fi, the first seconds after starting or "
                 "waking) the wallpaper default in Settings shows.")
        box.pack_start(self.mode_wallpaper_section, False, False, 0)
        self.mode_automation_section = section(
            "Automation", self.mode_editors,
            help="Its windows (its own, and those of schedules applied to it) and its places. Disabled by "
                 "default: they turn its blocks on, and they are lifted otherwise; enabled by default: they "
                 "turn them off. With windows and places, either or both at once. Monk mode is always "
                 "disabled by default.")
        box.pack_start(self.mode_automation_section, False, False, 0)
        self.shown_mode = None
        return scroll

    def update_mode_view(self):
        mode = self.find_mode(getattr(self, "shown_mode", None))
        if mode is None and getattr(self, "mode_wait", None) == getattr(self, "shown_mode", None):
            return
        if mode is None:
            self.blocks_stack.set_visible_child_name("cards")
            return
        self.mode_wait = None
        monk = bool(mode.get("monk"))
        frozen = bool(mode.get("freeze"))
        self.mode_title.set_text(mode["name"])
        self.mode_wallpapers.fill(mode.get("wallpapers") or [])
        was, self.updating = self.updating, True
        self.mode_override.set_active(not (self.state.get("wallpaper") or {}).get("override_modes"))
        self.updating = was
        self.mode_freeze_box.set_visible(frozen)
        self.mode_automation_section.set_visible(not frozen)
        self.mode_custom_section.set_visible(not frozen)
        if not frozen and (mode.get("custom") or []) != self.mode_custom.items:
            self.mode_custom.set_items(mode.get("custom") or [])
        if frozen:
            # §3.8 (owner 2026-11-17): on only with an end time; a running
            # freeze can't end.
            was, self.updating = self.updating, True
            self.mode_switch.set_active(bool(mode.get("running")))
            self.mode_switch.set_sensitive(not mode.get("running"))
            self.updating = was
            for widget in (self.mode_menu, self.mode_lock_button, self.mode_blocks_section, self.mode_monk_box):
                widget.set_visible(False)
            left = mode.get("freeze_left") or 0
            self.mode_status.set_text(f"Frozen for {fmt_seconds(left)} more; it can't end early, only be extended."
                                      if left else "Off. Switching it on asks until when (at most a week).")
            return
        # The switch, as on the mode's card (set without running anything).
        was, self.updating = self.updating, True
        blocks = mode.get("blocks") or []
        if monk:
            self.mode_switch.set_active(bool(mode.get("active")) or bool(mode.get("monk_left")))
            self.mode_switch.set_sensitive(not mode.get("monk_left"))
        else:
            self.mode_switch.set_active(bool(blocks) and mode.get("on", 0) == len(blocks))
            self.mode_switch.set_sensitive(bool(blocks) and not mode.get("run_left"))
        self.updating = was
        self.mode_menu.set_visible(not monk)
        self.mode_lock_button.set_visible(not monk)
        self.mode_lock_button.set_sensitive(bool(mode.get("blocks")))
        self.mode_blocks_section.set_visible(not monk)
        self.mode_monk_box.set_visible(monk)
        if monk:
            left = mode.get("monk_left") or 0
            self.mode_status.set_text(
                (f"Locked for {fmt_seconds(left)} more (started for a time). " if left else "") +
                ("Switched on: it runs until you switch it off." if mode.get("active") and mode.get("continuous") else
                 "Switched on: its windows and places run it." if mode.get("active") else
                 "Switched off."))
        else:
            blocks = mode.get("blocks") or []
            self.mode_take_out_button.set_sensitive(bool(blocks))
            self.mode_status.set_text((f"Run by a Mode action: locked for {fmt_seconds(mode['run_left'])} more. "
                                       if mode.get("run_left") else "") +
                                      (f"{mode.get('on', 0)} of {len(blocks)} blocks on" +
                                       (f", {mode['locked']} locked" if mode.get("locked") else "") + "."
                                       if blocks else "No blocks yet: drag blocks onto its card, or add them here."))
            shown = json.dumps([self.blocks.get(b) for b in blocks], sort_keys=True)
            if getattr(self, "mode_blocks_shown", None) != (mode["name"], shown):
                self.mode_blocks_shown = (mode["name"], shown)
                for child in self.mode_block_cards.get_children():
                    self.mode_block_cards.remove(child)
                for b in blocks:
                    if b in self.blocks:
                        self.mode_block_cards.pack_start(self.block_card(self.blocks[b]), False, False, 0)
                self.mode_block_cards.show_all()
        editor = self.monk_editor if monk else self.mode_editor
        self.mode_editors.set_visible_child(editor)
        editor.show_data(mode["name"], mode)

    CREATE_NEW = 1  # the Add blocks… dialog's Create new block… answer

    def add_blocks_to_mode(self, _button, answer=None):
        """Ticks blocks to put into the shown mode, or Create new block…
        (§4.2, owner 2026-11-17), which makes one there. answer: for tests,
        CREATE_NEW and the new block's name."""
        mode = getattr(self, "shown_mode", None)
        if not mode:
            return
        names = [n for n, b in self.blocks.items() if not b.get("draft_of") and b.get("mode") != mode]
        dialog = Gtk.Dialog(title=f"Add blocks to {mode}", transient_for=self, modal=True)
        create = dialog.add_button("Create new block…", self.CREATE_NEW)
        create.set_tooltip_text(f"A new, empty block in {mode}")
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Add", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim("They get the mode's automation; their own windows and places give way to it."
                           if names else "Every block is in this mode already."), False, False, 0)
        checks = {}
        for n in names:
            other = self.blocks[n].get("mode")
            checks[n] = Gtk.CheckButton(label=n + (f" (now in {other})" if other else ""))
            box.pack_start(checks[n], False, False, 0)
        ok.set_sensitive(False)
        ok.set_visible(bool(names))
        for check in checks.values():
            check.connect("toggled", lambda _c: ok.set_sensitive(any(c.get_active() for c in checks.values())))
        self.mode_add_dialog = (dialog, checks)  # for tests
        if answer is None:
            dialog.show_all()
            ok.set_visible(bool(names))
            reply = dialog.run()
        else:
            reply = answer[0]
        chosen = [n for n, c in checks.items() if c.get_active()]
        dialog.destroy()
        if reply == self.CREATE_NEW:
            name = answer[1] if answer is not None else self.ask_text(
                f"New block in {mode}", "A short name, such as social-media. It gets the mode's automation.")
            if name:
                self.mode_blocks_shown = None
                self.run(["block", "create", name],
                         then=lambda _o: self.run(["mode", "add", mode, name]))
        elif reply == Gtk.ResponseType.OK and chosen:
            self.mode_blocks_shown = None
            self.run(["mode", "add", mode, *chosen])

    def take_blocks_out_of_mode(self, _button):
        """The keyboard's way to take a block out of the shown mode (R-GUI-3;
        the cards have no ⋯ menu)."""
        mode = self.find_mode(getattr(self, "shown_mode", None)) or {}
        name = self.choose("Take a block out", f"It leaves {mode.get('name')} and keeps the automation it has now.",
                           mode.get("blocks") or [])
        if name:
            self.mode_blocks_shown = None
            self.take_out_of_mode(name)

    def toggle_mode_override(self, _switch, wanted):
        """R-MODE-5: modes' wallpapers over circadian rhythm's, or the other
        way round, for all modes."""
        if not self.updating:
            self.run(["wallpaper", "priority", "modes" if wanted else "circadian"])
        return False

    def set_mode_custom(self, items):
        """What the shown mode opens or runs when it becomes enabled: the
        list as it is now (clear, then each added)."""
        mode = getattr(self, "shown_mode", None)
        if not mode:
            return
        commands = [["mode", "custom", mode, "clear"]] + [["mode", "custom", mode, "add", item] for item in items]

        def step(rest):
            if rest:
                self.run(rest[0], then=lambda _o: step(rest[1:]))
        step(commands)

    def lock_mode(self):
        """R-MODE-3: locks every block of the shown mode with the chosen
        locks (any kind), confirmed once (R-LOCK-12)."""
        mode = self.find_mode(getattr(self, "shown_mode", None)) or {}
        if not mode.get("blocks") or mode.get("monk"):
            return
        picker = LockPicker("switch the mode off", "switched off", LockPicker.ALL_KINDS)
        asked = self.lock_dialog(
            f"Lock {mode['name']}",
            f"Every block in it ({', '.join(mode['blocks'])}) gets these locks, after the ones it has. While "
            "they are on, the mode can't be switched off or weakened.", "Lock", picker, need_lock=True)
        if asked is None:
            return
        chosen, relock_flags, secret = asked
        if not chosen:
            return
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        args, locks = ["mode", "lock", mode["name"]], []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"mode '{mode['name']}' (each of its blocks): {order}{text}")
        args += relock_flags
        if self.confirm_lock(locks):
            self.run(args + ["--yes"], secret + "\n" if secret else None)

    def add_from_preset(self, kind, mode=None):
        """Uses a preset on the opened block (or a mode): lists, or an
        automation or schedule preset."""
        kinds = ("lists",) if kind == "lists" else ("automation", "schedule")
        names = [f"{p['name']} ({p['kind']})" for p in self.state.get("presets") or [] if p["kind"] in kinds]
        if not names:
            self.message("No such presets yet: save one first.", Gtk.MessageType.INFO)
            return
        chosen = self.choose("Use a preset", "It is copied in; what would weaken a locked block is refused.", names)
        if not chosen:
            return
        name, kind = chosen.rsplit(" (", 1)
        target = mode or self.selected
        args = ["preset", "use", name, target, "--kind", kind.rstrip(")")] + (["--mode"] if mode else [])
        self.run(args, then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def rename_mode(self):
        mode = getattr(self, "shown_mode", None)
        name = mode and self.ask_text("Rename the mode", initial=mode)
        if name and name != mode:
            self.run(["mode", "rename", mode, name], then=lambda _o: setattr(self, "shown_mode", name))

    def delete_mode(self):
        mode = getattr(self, "shown_mode", None)
        if mode and self.confirm(f"Delete the mode {mode}?", "Its blocks stay, with the automation they "
                                 "have now.", "Delete", destructive=True):
            self.blocks_stack.set_visible_child_name("cards")
            self.run(["mode", "delete", mode])

    def choose(self, title, detail, names):
        """One of names, picked in a dialog; None when cancelled."""
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        if detail:
            box.pack_start(dim(detail), False, False, 0)
        combo = Gtk.ComboBoxText()
        for n in names:
            combo.append_text(n)
        combo.set_active(0)
        box.pack_start(combo, False, False, 0)
        self.choose_dialog = (dialog, combo)  # for tests
        dialog.show_all()
        answer = dialog.run()
        value = combo.get_active_text()
        dialog.destroy()
        return value if answer == Gtk.ResponseType.OK else None

    def open_automation(self, name=None):
        """The automation view of a block (the opened one): from its menu
        (§4.3); a block in a mode opens the mode instead (R-MODE-1)."""
        name = name or self.selected
        block = self.blocks.get(name) or {}
        if not block:
            return
        if block.get("mode"):
            self.open_mode(block["mode"])
            return
        if name != self.selected or self.blocks_stack.get_visible_child_name() not in ("detail", "automation"):
            self.open_block(name)
        if self.selected != name:
            return  # stayed with unsaved edits elsewhere
        self.automation_title.set_text(f"Automation of {name}")
        self.blocks_stack.set_visible_child_name("automation")

    def open_block(self, name, tab=None):
        """Opens a block on the Blocks page, at a tab when given; another
        block's unsaved edits are asked about first (§4.3)."""
        if tab in ("Automation", "Schedule & places"):
            self.open_automation(name)
            return
        if (self.selected != name and self.blocks_stack.get_visible_child_name() == "detail"
                and not self.leave_block(to_cards=False)):
            return
        self.selected = self.blocks.get(name, {}).get("draft_of") or name
        self.blocks_stack.set_visible_child_name("detail")
        self.stack.set_visible_child_name("blocks")
        self.update_block_list()
        if tab is not None:
            for number in range(self.block_tabs.get_n_pages()):
                if self.block_tabs.get_tab_label_text(self.block_tabs.get_nth_page(number)) == tab:
                    self.block_tabs.set_current_page(number)

    def toggle_block_card(self, switch, wanted, name):
        """On: switched on without locks; off: switched off (refused while
        locked, and the switch goes back)."""
        if self.updating:
            return False
        self.selected = name
        self.block_cards_shown = None  # redrawn from the state after it, refused or not
        self.run(["start", name] if wanted else ["stop", name])
        return True  # the switch follows the state after the command

    def card_locks(self, name):
        """Locked: the unlock flow; else the start/add-locks dialog. On the
        block itself, never its draft (§4.3: a draft never runs)."""
        self.selected = name
        block = self.blocks.get(name) or {}
        (self.unlock_block if block.get("held") else self.start_block)(None)

    def build_quota_tab(self):
        """Breaks & allowance (R-GUI-3, redesigned 2026-11-17): a card each,
        a line saying what it is (the rest behind ?), rows of a label and its
        fields, one primary button."""
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18, margin=12)
        labels = Gtk.SizeGroup(mode=Gtk.SizeGroupMode.HORIZONTAL)

        def form_row(text, *widgets):
            row = Gtk.Box(spacing=8)
            label = Gtk.Label(label=text, xalign=0, valign=Gtk.Align.START, margin_top=6)
            labels.add_widget(label)
            row.pack_start(label, False, False, 0)
            for widget in widgets:
                row.pack_start(widget, False, False, 0)
            return row

        def buttons(primary, *others):
            row = Gtk.Box(spacing=6)
            primary.get_style_context().add_class("suggested-action")
            for button in (primary, *others):
                row.pack_start(button, False, False, 0)
            return row

        # §3.6 allowance: per day, week, month, rolling 24 hours or window,
        # refilled at a set time; sessions.
        self.quota_minutes = DurationField(1800, minimum=60, maximum=86400, width=9)
        self.quota_per = Gtk.ComboBoxText()
        for key, label in (("day", "per day"), ("week", "per week"), ("month", "per month"),
                           ("rolling", "per 24 hours, rolling"), ("window", "per window")):
            self.quota_per.append(key, label)
        self.quota_per.set_active_id("day")
        self.quota_per.connect("changed", lambda combo: self.quota_minutes.set_limits(
            maximum={"week": 7 * 86400, "month": 31 * 86400}.get(combo.get_active_id(), 86400)))
        self.quota_refill = Gtk.Entry(text="00:00", width_chars=5, max_length=5)
        quota_row = form_row("Time", self.quota_minutes, self.quota_per,
                             Gtk.Label(label="refill at", valign=Gtk.Align.START, margin_top=6), self.quota_refill)
        self.quota_sessions = NumberField(0, 0, 100, width=4, hint="0: any number")
        sessions_row = form_row("Sessions", self.quota_sessions)
        self.quota_set = Gtk.Button(label="Save allowance")
        self.quota_set.connect("clicked", self.set_quota)
        self.quota_start = Gtk.Button(label="Use allowance now")
        self.quota_start.connect("clicked", self.start_quota)
        self.quota_end = Gtk.Button(label="End session")
        self.quota_end.connect("clicked", self.end_quota)
        self.quota_remove = Gtk.Button(label="Remove")
        self.quota_remove.connect("clicked", self.remove_quota)
        self.quota_label = dim("")
        box.pack_start(section(
            "Allowance", dim("Time to use what the block blocks, in sessions you open."),
            quota_row, sessions_row, buttons(self.quota_set, self.quota_start, self.quota_end, self.quota_remove),
            self.quota_label,
            help="Sessions you open lift the block until the period's time is spent. A session only "
                 "spends time while the block is in use: its app in front, its site in the focused "
                 "tab, or playing sound or video. An allowance protects nothing by itself: changes "
                 "apply at once until the block is locked; then it only takes less time or fewer "
                 "sessions. Per window: each opening of a block window is a period."), False, False, 0)

        self.work_minutes = DurationField(25 * 60, minimum=60, maximum=86400, width=8)
        self.break_minutes = DurationField(5 * 60, minimum=60, maximum=86400, width=8)
        pomodoro_row = form_row("Work", self.work_minutes,
                                Gtk.Label(label="then a break of", valign=Gtk.Align.START, margin_top=6),
                                self.break_minutes)
        set_pomodoro = Gtk.Button(label="Save")
        set_pomodoro.connect("clicked", self.set_pomodoro)
        clear_pomodoro = Gtk.Button(label="Turn off")
        clear_pomodoro.connect("clicked", self.clear_pomodoro)
        self.long_check = Gtk.CheckButton(valign=Gtk.Align.START, margin_top=6,
                                          tooltip_text="A long break now and then, instead of the short one")
        self.long_minutes = DurationField(15 * 60, minimum=60, maximum=86400, width=8)
        self.long_every = NumberField(4, 2, 20, width=4)
        long_row = form_row("Long break", self.long_check, self.long_minutes,
                            Gtk.Label(label="every", valign=Gtk.Align.START, margin_top=6), self.long_every,
                            Gtk.Label(label="work periods", valign=Gtk.Align.START, margin_top=6))
        box.pack_start(section(
            "Pomodoro", dim("Work and break take turns while the block is on, work first."),
            pomodoro_row, long_row, buttons(set_pomodoro, clear_pomodoro),
            help="While the block is on, it pauses for the break after each work period; its rhythm "
                 "starts again each time the block is enabled (R-SCHED-3). During a work period the "
                 "block's allowance can't be used. Locks stay on through the breaks."), False, False, 0)

        # §3.6 reward: saved up to one break, accumulating until a period
        # ends, or each part kept for a time the user picks (no upper limit).
        self.allowance_minutes = DurationField(600, minimum=60, maximum=4 * 3600, width=8)
        self.allowance_per = DurationField(3600, minimum=300, maximum=86400, width=8)
        allowance_row = form_row("Earn", self.allowance_minutes,
                                 Gtk.Label(label="of break for every", valign=Gtk.Align.START, margin_top=6),
                                 self.allowance_per,
                                 Gtk.Label(label="of blocking", valign=Gtk.Align.START, margin_top=6))
        self.reward_keep = Gtk.ComboBoxText()
        for key, label in (("one", "up to one break"), ("window", "until the window ends"),
                           ("day", "until the day ends"), ("week", "until the week ends"),
                           ("month", "until the month ends"), ("time", "for a time after it is earned:")):
            self.reward_keep.append(key, label)
        self.reward_keep.set_active_id("one")
        self.reward_keep_time = DurationField(7 * 86400, minimum=3600, width=8)
        self.reward_keep_time.set_no_show_all(True)
        self.reward_keep.connect("changed", lambda combo: self.reward_keep_time.set_visible(
            combo.get_active_id() == "time"))
        keep_row = form_row("Keep it", self.reward_keep, self.reward_keep_time)
        save_reward = Gtk.Button(label="Save reward")
        save_reward.connect("clicked", self.set_allowance)
        remove_reward = Gtk.Button(label="Remove")
        remove_reward.connect("clicked", self.clear_allowance)
        self.break_start = Gtk.Button(label="Take a break now")
        self.break_start.connect("clicked", self.start_break)
        self.break_end = Gtk.Button(label="End break")
        self.break_end.connect("clicked", self.end_break)
        self.break_label = dim("")
        box.pack_start(section(
            "Reward", dim("Break time earned while the block works, to take when you choose."),
            allowance_row, keep_row, buttons(save_reward, remove_reward, self.break_start, self.break_end),
            self.break_label,
            help="Break time is earned while the block is on and the computer is in use, and kept as "
                 "chosen: up to one break, until the window, day, week or month ends, or each part "
                 "for as long as you choose. Take it when you choose, also while the block is locked; "
                 "the block comes back by itself. Allowing more breaks weakens the block, so decide "
                 "before locking it."), False, False, 0)

        self.relock_check = Gtk.CheckButton(label="Unlocking gives a break of")
        self.relock_minutes = DurationField(5 * 60, minimum=60, maximum=4 * 3600, width=8)
        relock_save = Gtk.Button(label="Save")
        relock_save.connect("clicked", self.set_relock)
        relock_row = form_row("Text break", self.relock_check, self.relock_minutes,
                              Gtk.Label(label="then the block is back with its locks", valign=Gtk.Align.START,
                                        margin_top=6), relock_save)
        # §3.6 delay break: asked for, it begins after a countdown.
        self.delay_break_check = Gtk.CheckButton(label="Delay break of")
        self.delay_break_minutes = DurationField(600, minimum=60, maximum=4 * 3600, width=8)
        self.delay_wait_minutes = DurationField(1800, minimum=60, maximum=86400, width=8)
        delay_save = Gtk.Button(label="Save")
        delay_save.connect("clicked", self.set_delay_break)
        self.delay_request = Gtk.Button(label="Request break")
        self.delay_request.connect("clicked", lambda _b: self.selected and self.run(["break", "request", self.selected]))
        delay_row = form_row("Delay break", self.delay_break_check, self.delay_break_minutes,
                             Gtk.Label(label="after waiting", valign=Gtk.Align.START, margin_top=6),
                             self.delay_wait_minutes, delay_save, self.delay_request)
        box.pack_start(section(
            "Breaks that keep the locks", dim("A break after a challenge or a countdown; the block stays locked."),
            relock_row, delay_row,
            help="Text break: passing the typing challenge or giving the password doesn't end the "
                 "block's lock, it lifts the block for a while; unlocking again during that break ends "
                 "it. Delay break: asked for, it begins after the wait you chose."), False, False, 0)
        for row in (quota_row, sessions_row, pomodoro_row, long_row, allowance_row, keep_row, relock_row, delay_row):
            top_align(row)
            flow_row(row)
        return box

    def update_block_list(self):
        # §4.3: a draft shows in its block's place, never in the list; the
        # page keeps the block itself selected.
        names = [n for n in self.blocks if not self.blocks[n].get("draft_of")]
        self.selected = self.blocks.get(self.selected, {}).get("draft_of") or self.selected
        shown = self.selected
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
            if self.selected is not None and self.blocks_stack.get_visible_child_name() == "detail":
                # The opened block is gone (deleted elsewhere): back to the
                # list rather than opening another block (§4.2).
                self.blocks_stack.set_visible_child_name("cards")
            self.selected = shown = names[0] if names else None
        for row in self.block_list.get_children():
            if row.block_name == shown:
                self.block_list.select_row(row)
        self.update_block_cards()
        self.show_block()

    def block_selected(self, _box, row):
        if row is None or self.updating:
            return
        self.selected = row.block_name
        self.show_block()

    def start_draft(self, _button, then=None):
        """§4.3: the block's draft, made at its first edit; with a block
        password, that password (the draft keeps it)."""
        name = self.selected
        block = self.blocks.get(name) or {}
        if not name:
            return
        password = self.block_passwords.get(name)
        if block.get("block_password") and not password:
            password = self.ask_text(f"Edit '{name}'", "Its block password.", password=True)
            if not password:
                return
        if password:
            self.block_passwords[name] = self.block_passwords[draft_of_name(name)] = password
        self.run(["block", "draft", name], password + "\n" if password else None, then=then)

    def has_edits(self, name=None):
        """Whether a block (the opened one) has unsaved edits: a draft."""
        name = name or self.selected
        return bool(name and (self.blocks.get(name) or {}).get("draft"))

    def finish_draft(self, how, name=None):
        """§4.3: Save edits applies the draft to its block (weakening parts
        refused on a locked block, and named); Discard drops it."""
        name = name or self.selected
        if self.has_edits(name):
            self.run(["block", "draft", name, how])

    def ask_unsaved(self, name):
        """Leaving a block with unsaved edits: "save", "cancel" (discard)
        or None (stay)."""
        dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                   text=f"Save the edits to {name}?",
                                   secondary_text="Its websites and apps change only when you save them.")
        dialog.add_button("Discard", 2)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        save = dialog.add_button("Save edits", Gtk.ResponseType.OK)
        save.get_style_context().add_class("suggested-action")
        answer = dialog.run()
        dialog.destroy()
        return {Gtk.ResponseType.OK: "save", 2: "cancel"}.get(answer)

    def leave_block(self, to_cards=True):
        """Leaves the opened block (back to the list unless to_cards is
        False); unsaved edits are asked about first. False: stay."""
        name = self.selected
        if self.has_edits(name) and self.blocks_stack.get_visible_child_name() == "detail":
            answer = self.ask_unsaved(name)
            if answer is None:
                return False
            self.finish_draft(answer, name)
        if to_cards:
            self.blocks_stack.set_visible_child_name("cards")
        return True

    def show_block(self):
        block = self.blocks.get(self.selected)
        self.detail.set_sensitive(block is not None)
        if block is None:
            self.title_label.set_text("No blocks yet")
            for child in self.status_pills.get_children():
                self.status_pills.remove(child)
            self.status_label.set_text("Create a block, add sites or apps, then start it.")
            for widget in (self.apps, self.titles, self.schedules, self.autostarts):
                widget.set_items([])
            self.update_lists([])
            self.calendar.set_windows([])
            self.quota_label.set_text("")
            self.block_nepsis_label.set_text("")
            return
        self.title_label.set_text(block["name"])
        for child in self.status_pills.get_children():
            self.status_pills.remove(child)
        for text, kind in block_pills(block):
            self.status_pills.pack_start(pill(text, kind), False, False, 0)
        self.status_pills.show_all()
        self.status_label.set_text(block_summary(block))
        users = block.get("users") or []
        self.users_button.set_label("Users: " + (", ".join(users) if users else "all"))
        self.users_button.set_visible(len(self.state.get("accounts") or []) > 1 or bool(users))
        self.hidden_bar.set_visible(bool(block.get("hidden")))
        # §4.3 (owner 2026-11-17): the block is always a draft: its Websites
        # and Apps tabs show the draft once there is one (made at the first
        # edit); Save edits applies it, Discard drops it.
        content = self.blocks.get(block.get("draft") or "") or block
        editing = content is not block
        self.draft_bar.set_visible(editing)
        self.draft_label.set_text("Unsaved edits to its websites and apps: they apply when you save them."
                                  + (" What would weaken it while it is locked is refused then, and named."
                                     if block.get("held") else "") if editing else "")
        self.update_lists(content.get("lists", []), content.get("phrase_sets", []))
        self.apps.set_items(content["apps"], owner=block["name"])
        self.titles.set_items(content.get("titles", []), owner=block["name"])
        triggers = content.get("triggers") or {}
        self.app_triggers.set_actions(block["name"], triggers.get("apps", [{"name": "close"}]))
        self.title_triggers.set_actions(block["name"], triggers.get("titles", [{"name": "close"}]))
        self.schedules.set_items(block["schedules"])
        self.update_automation(block)
        in_mode = block.get("mode")
        self.block_mode_bar.set_visible(bool(in_mode))
        self.block_mode_label.set_text(f"In the mode {in_mode}: it has the mode's automation, changed on the "
                                       f"mode for all its blocks. Its own is kept for when it is taken out."
                                       if in_mode else "")
        self.automation_button.set_sensitive(not in_mode)
        self.automation_button.set_tooltip_text(f"Automated by '{in_mode}'" if in_mode else
                                                "What switches it by itself: its windows and places")
        self.automation_title.set_text(f"Automation of {block['name']}")
        if in_mode and self.blocks_stack.get_visible_child_name() == "automation":
            self.blocks_stack.set_visible_child_name("detail")
        for widget in self.block_auto_widgets:
            widget.set_sensitive(not in_mode)
        for child in self.block_applied.get_children():
            self.block_applied.remove(child)
        for sched in block.get("applied_schedules") or []:
            chip = Gtk.Box(spacing=2)
            chip.get_style_context().add_class("kc9k-pill")
            chip.get_style_context().add_class("on")
            chip.pack_start(Gtk.Label(label=sched), False, False, 0)
            x = Gtk.Button.new_from_icon_name("window-close-symbolic", Gtk.IconSize.MENU)
            x.set_relief(Gtk.ReliefStyle.NONE)
            x.set_tooltip_text("Take this schedule off the block")
            x.connect("clicked", lambda _b, sc=sched, b=block["name"]: self.run(["schedule", "take-off", b, sc]))
            chip.pack_start(x, False, False, 0)
            self.block_applied.pack_start(chip, False, False, 0)
        if not block.get("applied_schedules"):
            self.block_applied.pack_start(dim("none"), False, False, 0)
        self.block_applied.show_all()
        self.autostarts.set_items(block.get("autostarts") or [])
        others = [w for other in self.state.get("blocks", []) if other["name"] != block["name"]
                  for w in other.get("schedule_windows") or [] if not w.get("allow")]
        self.calendar.set_windows(block.get("schedule_windows", []), others)
        set_switch(self.keyword_test_switch, content.get("keyword_test", False))
        self.update_social(content)
        covered = set()
        for window in block.get("schedule_windows", []):
            if not window.get("allow"):
                for day in range(7):
                    if window["days"] & (1 << day):
                        covered.update((day, minute) for minute in range(window["start"], window["end"]))
        self.schedule_total.set_text(f"{len(covered) / 60:.0f} of 168 hours a week" if covered else "")
        settings = block.get("automation") or {}
        self.block_places.to_choose = getattr(self, "place_to_show", None)
        self.place_to_show = None
        self.block_places.show_places(block.get("places") or [], "this block", bool(settings.get("default_on")),
                                      bool(settings.get("need_both")), bool(block.get("schedules")))
        if not self.schedule_lead.has_focus():
            self.schedule_lock.set_active(block.get("schedule_lock", False))
            self.schedule_lead.set_value(block.get("schedule_lead", 0))
        allowance, balance = block.get("break_allowance", 0), block.get("break_balance", 0)
        if allowance and not self.reward_keep.has_focus() and not self.reward_keep_time.entry.has_focus():
            keep = block.get("break_keep") or "one"
            if keep.isdigit():
                self.reward_keep_time.set_seconds(int(keep))
                keep = "time"
            self.reward_keep.set_active_id(keep)
        paused = block.get("paused_seconds", 0)
        self.break_start.set_sensitive(block["active"] and allowance > 0 and balance >= 60 and not paused)
        self.break_end.set_sensitive(bool(paused))
        if paused:
            text = f"On a break for {fmt_seconds(paused)} more."
        elif allowance:
            text = (f"Reward: {allowance // 60} min per {block.get('break_per', 0) // 60} min of blocking. "
                    f"Earned: {balance // 60} min.")
        else:
            text = "No reward."
        if block.get("relock"):
            text += f" Unlocking gives a break of {block['relock'] // 60} min."
        if block.get("delay_break_in"):
            text += f" Its delay break begins in {fmt_seconds(block['delay_break_in'])}."
        self.break_label.set_text(text)
        if not self.delay_break_minutes.has_focus() and not self.delay_wait_minutes.has_focus():
            self.delay_break_check.set_active(bool(block.get("delay_break")))
            if block.get("delay_break"):
                self.delay_break_minutes.set_value(block["delay_break"] // 60)
                self.delay_wait_minutes.set_value(block.get("delay_wait", 0) // 60)
        self.delay_request.set_sensitive(bool(block.get("delay_break")) and not block.get("delay_break_in") and not paused)

        self.start_button.set_label("Add locks…" if block["active"] else "Start…")
        self.start_button.set_sensitive(True)
        # §3.4: unlocking lifts locks (a delay lock starts counting down) and
        # leaves the block on; stopping needs every lock lifted.
        self.stop_button.set_sensitive(block["active"] and not block["held"])
        self.unlock_button.set_sensitive(
            (block["char_locked"] or block["password_locked"]
             or (block["delay_seconds"] > 0 and not block.get("delay_left")))
            and not block["lock_seconds"] and not block["restart_locked"])
        self.delete_button.set_sensitive(not block["active"] and not block["held"])

        limit = block["quota_limit"]
        self.quota_start.set_sensitive(limit > 0 and not block["quota_open"] and block["quota_used"] < limit)
        self.quota_end.set_sensitive(block["quota_open"])
        self.quota_remove.set_sensitive(limit > 0)
        if limit and not self.quota_refill.has_focus() and not self.quota_minutes.has_focus():
            self.quota_per.set_active_id(block.get("quota_period") or "day")
            self.quota_refill.set_text(block.get("quota_refill") or "00:00")
            self.quota_sessions.set_value(block.get("quota_sessions", 0))
            self.quota_minutes.set_value(limit // 60)
        self.quota_label.set_text(
            f"Used this {block.get('quota_period') or 'day'}: {block['quota_used'] // 60} of {limit // 60} min"
            + (f" · sessions {block.get('quota_sessions_used', 0)} of {block['quota_sessions']}"
               if block.get("quota_sessions") else "")
            + (" · session open" if block["quota_open"] else "") if limit else "No allowance set.")
        self.block_nepsis_label.set_text(describe_nepsis(block.get("nepsis"), block.get("nepsis_count")
                                                         if block["active"] else None))
        self.block_nepsis_off_button.set_visible(bool((block.get("nepsis") or {}).get("steps")))

    def update_lists(self, lists, phrase_sets=()):
        """Shows the selected block's lists and phrase sets, keeping the cards
        of those still there, so a refresh doesn't disturb what is being
        typed. Phrase sets are keyed by minus their ID, as in the rules."""
        lists = list(lists) + [dict(p, id=-p["id"], kind="phrases") for p in phrase_sets]
        # A list keeps its card when the block's draft takes over (other
        # IDs, same names): what is being typed stays (§4.3).
        by_name = {(c.data.get("kind") == "phrases", c.data["name"].lower()): i for i, c in self.list_cards.items()}
        for entry in lists:
            old = by_name.get((entry["kind"] == "phrases", entry["name"].lower()))
            if old is not None and old != entry["id"] and entry["id"] not in self.list_cards:
                self.list_cards[entry["id"]] = self.list_cards.pop(old)
        ids = [entry["id"] for entry in lists]
        for list_id in list(self.list_cards):
            if list_id not in ids:
                card = self.list_cards.pop(list_id)
                card.get_parent().remove(card)
        for entry in lists:
            card = self.list_cards.get(entry["id"])
            if card is None:
                card = (PhraseSetCard if entry["kind"] == "phrases" else ListCard)(self, entry)
                self.list_cards[entry["id"]] = card
                self.list_boxes[entry["kind"]].pack_start(card, False, False, 0)
                card.show_all()
            card.update(entry)

    def create_list(self, _button, kind):
        if not self.selected:
            return
        if kind == "phrases":
            self.create_phrase_set()
            return
        dialog, entry, checks = self.list_dialog(kind)
        dialog.show_all()
        answer = dialog.run()
        args = self.list_create_args(kind, entry, checks)
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and args:
            self.run(args)

    def list_dialog(self, kind):
        """The New list dialog: a name and the three scope boxes (§3.2).
        Returns the dialog, the name entry and the boxes by place."""
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL,
            text="New exception list" if kind == "exceptions" else "New blacklist",
            secondary_text="A name, and where it looks. Sites (reddit.com), URL entries and patterns "
                           "(*casino*) match the URL; words (long cat) match words in the URL, title "
                           "or text. One list can hold both.")
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        area = dialog.get_message_area()
        area.pack_start(entry, False, False, 0)
        checks = {}
        boxes = Gtk.Box(spacing=12)
        for name, title in LIST_PLACES + (() if kind == "exceptions" else (AD_PLACE,)):
            checks[name] = Gtk.CheckButton(label=title, active=name == "url")
            boxes.pack_start(checks[name], False, False, 0)
        area.pack_start(boxes, False, False, 0)
        ok = dialog.get_widget_for_response(Gtk.ResponseType.OK)
        def sync(*_):
            ok.set_sensitive(bool(entry.get_text().strip()) and any(c.get_active() for c in checks.values()))
        entry.connect("changed", sync)
        for check in checks.values():
            check.connect("toggled", sync)
        sync()
        return dialog, entry, checks

    def list_create_args(self, kind, entry, checks):
        """The kc9k command the New list dialog runs, or None without a
        name or a scope box."""
        name = entry.get_text().strip()
        places = [place for place, check in checks.items() if check.get_active()]
        if not self.selected or not name or not places:
            return None
        args = ["list", "create", self.selected, name] + ["--" + place for place in places]
        if kind == "exceptions":
            args.append("--exceptions")
        return args

    def create_phrase_set(self):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL, text="New phrase set",
            secondary_text="A name, and where its phrases are looked for.")
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        area = dialog.get_message_area()
        area.pack_start(entry, False, False, 0)
        checks = {}
        for name, title in LIST_PLACES:
            # Titles too by default: a phrase in a page's title should count.
            checks[name] = Gtk.CheckButton(label=title, active=name in ("title", "text"))
            area.pack_start(checks[name], False, False, 0)
        entry.set_placeholder_text("optional")
        dialog.show_all()
        answer = dialog.run()
        name = entry.get_text().strip()
        places = [place for place, check in checks.items() if check.get_active()]
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        if not name:
            # §4.3: an unnamed phrase set gets a name.
            name = self.free_phrase_set_name()
        self.run(["phrase-set", "create", self.selected, name] + ["--" + p for p in places])

    def free_phrase_set_name(self):
        """'Phrase set N', the first N no phrase set of the block has
        (names compare case-insensitively)."""
        block = self.blocks.get(self.selected) or {}
        taken = {p["name"].lower() for p in block.get("phrase_sets") or []}
        n = 1
        while f"phrase set {n}" in taken:
            n += 1
        return f"Phrase set {n}"

    # Block actions -------------------------------------------------------
    def create_block(self, _button):
        templates = (self.state or {}).get("templates") or []
        template = None
        if templates:
            # §3.2: start from one of your templates, or empty.
            dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                       buttons=Gtk.ButtonsType.OK_CANCEL, text="New block",
                                       secondary_text="Start empty or from a template.")
            combo = Gtk.ComboBoxText()
            combo.append("", "Empty")
            for name in templates:
                combo.append(name, "Template: " + name)
            combo.set_active(0)
            dialog.get_message_area().pack_start(combo, False, False, 0)
            dialog.show_all()
            answer = dialog.run()
            template = combo.get_active_id()
            dialog.destroy()
            if answer != Gtk.ResponseType.OK:
                return
        name = self.ask_text("New block", "A short name, such as social-media.")
        if name:
            self.selected = name
            self.blocks_stack.set_visible_child_name("detail")
            self.stack.set_visible_child_name("blocks")
            self.run(["block", "create", name] + (["--template", template] if template else []))

    def save_template(self, _button):
        if not self.selected:
            return
        name = self.ask_text("Save as block preset", "A name for a copy of this block, to make new blocks from.",
                             initial=self.selected)
        if name:
            self.run(["template", "save", self.selected, name])

    def choose_block_users(self, _button, answer=None):
        """§3.2: tick the accounts the block applies to; all ticked (or
        none) means every account. answer: the ticked names, for tests."""
        block = self.blocks.get(self.selected)
        if not block:
            return
        accounts = self.state.get("accounts") or []
        current = block.get("users") or []
        if answer is None:
            dialog = Gtk.MessageDialog(
                transient_for=self, modal=True, text=f"Who does {block['name']} apply to?",
                secondary_text="For some accounts only, its websites are blocked in their browsers by the "
                               "extension and its apps closed in their sessions. Fewer accounts weakens the "
                               "block, so it isn't possible while it is locked.",
                message_type=Gtk.MessageType.QUESTION, buttons=Gtk.ButtonsType.OK_CANCEL)
            checks = {}
            for name in accounts:
                checks[name] = Gtk.CheckButton(label=name, active=not current or name in current)
                dialog.get_message_area().pack_start(checks[name], False, False, 0)
            dialog.show_all()
            ok = dialog.run() == Gtk.ResponseType.OK
            answer = [name for name, check in checks.items() if check.get_active()]
            dialog.destroy()
            if not ok:
                return
        if not answer or set(answer) >= set(accounts):
            self.run(["block", "users", block["name"], "all"])
        else:
            self.run(["block", "users", block["name"], *answer])

    def rename_block(self, _button):
        name = self.ask_text("Rename block", initial=self.selected or "")
        if self.selected and name and name != self.selected:
            old, self.selected = self.selected, name
            self.run(["block", "rename", old, name])

    def duplicate_block(self, _button):
        name = self.ask_text("Duplicate block", "Name of the copy.")
        if self.selected and name:
            self.run(["block", "duplicate", self.selected, name])

    def lock_dialog(self, title, intro, button, picker, extra=None, need_lock=False):
        """A dialog around a LockPicker; None when cancelled, else what it
        asks for, read before the dialog goes: (locks as (flags, words),
        re-lock flags, password). extra, when given, is more fields above
        the locks (on_change, problem()). The button waits until every
        field reads right, and with need_lock until a lock is chosen."""
        dialog = Gtk.Dialog(title=title, transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button(button, Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(12)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(18)
        box.pack_start(dim(intro), False, False, 0)
        if extra is not None:
            box.pack_start(extra, False, False, 0)
        box.pack_start(picker, False, False, 0)

        def update(*_args):
            ok.set_sensitive(picker.is_valid() and (extra is None or extra.problem() is None)
                             and (not need_lock or bool(picker.chosen())))

        picker.on_change(update)
        if extra is not None:
            extra.on_change(update)
        update()
        self.lock_picker, self.lock_extra = picker, extra  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen = (picker.chosen(), picker.relock_flags(), picker.secret()) if picker.is_valid() else None
        dialog.destroy()
        return chosen if answer == Gtk.ResponseType.OK else None

    def start_block(self, _button):
        block = self.blocks.get(self.selected)
        if not block:
            return
        active = block["active"]
        # R-LOCK-7: a running block takes any kind of lock, added after the
        # ones already on; those only get stronger.
        picker = LockPicker("stop", "stopped", LockPicker.ALL_KINDS)
        asked = self.lock_dialog(
            f"Add locks to {block['name']}" if active else f"Start {block['name']}",
            "New locks come after the ones already on, unlocked last. A lock that is on can only "
            "be made stronger: a later timer, a longer text or delay lock." if active else
            "Start without locks to stop any time, or add locks below. Once on, a lock can only "
            "be made stronger.",
            "Add locks" if active else "Start", picker, need_lock=active)
        if asked is None:
            return
        chosen, relock_flags, secret = asked
        args = ["start", block["name"]]
        target = f"block '{block['name']}'"
        if active and not chosen:
            return  # nothing to add
        # R-LOCK-6: locks unlock one at a time in the order of their flags.
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        locks = []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"{target}: {order}{text}")
        args += relock_flags
        if not self.confirm_lock(locks):
            return
        if locks:
            args.append("--yes")
        self.run(args, secret + "\n" if secret else None)

    def choose_lock_order(self, chosen):
        """R-LOCK-6: the user puts the locks in the order they are unlocked
        (first at the top). None when cancelled."""
        dialog = Gtk.Dialog(title="Unlock order", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("OK", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim("The locks are unlocked one at a time, top first. A timer lock below "
                           "another counts from when the one above is unlocked."), False, False, 0)
        order = list(chosen)
        listbox = Gtk.ListBox(selection_mode=Gtk.SelectionMode.SINGLE)
        box.pack_start(listbox, False, False, 0)

        def fill(select=0):
            for row in listbox.get_children():
                listbox.remove(row)
            for _flags, text in order:
                label = Gtk.Label(label=text.split(" (")[0], xalign=0, margin=4)
                listbox.add(label)
            listbox.show_all()
            listbox.select_row(listbox.get_row_at_index(select))

        def move(step):
            row = listbox.get_selected_row()
            if row is None:
                return
            i, j = row.get_index(), row.get_index() + step
            if 0 <= j < len(order):
                order[i], order[j] = order[j], order[i]
                fill(j)

        buttons = Gtk.Box(spacing=6)
        for label, step in (("Up", -1), ("Down", 1)):
            button = Gtk.Button(label=label)
            button.connect("clicked", lambda _b, s=step: move(s))
            buttons.pack_start(button, False, False, 0)
        box.pack_start(buttons, False, False, 0)
        fill()
        self.lock_order_dialog = (dialog, move)  # for tests
        dialog.show_all()
        answer = dialog.run()
        dialog.destroy()
        return order if answer == Gtk.ResponseType.OK else None

    def reveal_block(self, _button):
        """Asks a block's password and shows its contents until the window closes."""
        name = self.selected
        password = self.ask_text(f"Show '{name}'", "Its block password.", password=True)
        if not password:
            return
        args = ["--reveal", name]
        ok, output = cli_with_access(list(self.access_passwords), ["gui-state", *args], password + "\n")
        try:
            hidden = ok and next(b for b in json.loads(output)["blocks"] if b["name"] == name).get("hidden")
        except (ValueError, StopIteration, KeyError):
            hidden = True
        if not ok or hidden:
            self.message("Wrong block password.", Gtk.MessageType.INFO)
            return
        self.block_passwords[name] = password
        self.state = None
        self.refresh()

    def stop_block(self, _button):
        if self.selected:
            self.run(["stop", self.selected])

    def save_schedule_lock(self, _button):
        if not self.selected:
            return
        if self.schedule_lock.get_active():
            if not self.fields_ok(("Schedule lock", self.schedule_lead)):
                return
            lead = self.schedule_lead.get_value_as_int()
            if not self.confirm_lock([f"block '{self.selected}': schedule lock, while each window is open"
                                      + (f" and {duration_words(lead * 60)} before it" if lead else "")]):
                return
            self.run(["schedule", "lock", self.selected, "on", "--lead", f"{lead}m", "--yes"])
        elif self.blocks.get(self.selected, {}).get("schedule_lock"):
            self.run(["schedule", "lock", self.selected, "off"])

    def unlock_block(self, _button):
        block = self.blocks.get(self.selected)
        if not block:
            return
        if not block["char_locked"]:
            if block["password_locked"]:
                password = self.ask_text(f"Unlock {block['name']}", "Enter the block's password.", password=True)
                if password:
                    self.run(["unlock", block["name"]], password + "\n",
                             then=lambda output: self.message(output, Gtk.MessageType.INFO))
            else:  # only a delay lock: unlocking starts its countdown
                self.run(["unlock", block["name"]], then=lambda output: self.message(output, Gtk.MessageType.INFO))
            return
        command = f"{CLI} unlock {shlex.quote(block['name'])}"
        try:
            subprocess.Popen(["x-terminal-emulator", "-e", command])
        except OSError as error:
            self.message(f"Can't open a terminal for the typing challenge: {error}\n\nRun: {command}")

    def delete_block(self, _button):
        """§4.2 (owner 2026-11-17): deleted, the page comes back to the block
        list; refused, the block stays open."""
        name = self.selected
        if name and self.confirm(f"Delete {name}?", "The block and its lists are removed.",
                                 "Delete", destructive=True):
            def gone(_output):
                if self.selected == name:
                    self.selected = None
                self.blocks_stack.set_visible_child_name("cards")
            self.run(["block", "remove", name], then=gone)

    def fields_ok(self, *named):
        """R-GUI-1: before a command, every (name, field) must read right;
        else the first problem is shown and nothing runs."""
        for name, field in named:
            if field is not None and not field.is_valid():
                self.message(f"{name}: {field.problem}", Gtk.MessageType.INFO)
                return False
        return True

    def set_quota(self, _button):
        if self.selected and self.fields_ok(("Allowance", self.quota_minutes), ("Sessions", self.quota_sessions)):
            self.run(["allowance", "set", self.selected, f"{self.quota_minutes.get_value_as_int()}m",
                      "--per", self.quota_per.get_active_id() or "day",
                      "--refill", self.quota_refill.get_text().strip() or "00:00",
                      "--sessions", str(self.quota_sessions.get_value_as_int())])

    def start_quota(self, _button):
        if self.selected:
            self.run(["allowance", "start", self.selected])

    def end_quota(self, _button):
        if self.selected:
            self.run(["allowance", "end", self.selected])

    def remove_quota(self, _button):
        if self.selected:
            self.run(["allowance", "remove", self.selected])

    def set_pomodoro(self, _button):
        long = self.long_check.get_active()
        if self.selected and self.fields_ok(("Work", self.work_minutes), ("Break", self.break_minutes),
                                            ("Long break", self.long_minutes if long else None),
                                            ("Work periods", self.long_every if long else None)):
            args = ["pomodoro", "set", self.selected,
                    f"{self.work_minutes.get_value_as_int()}m",
                    f"{self.break_minutes.get_value_as_int()}m"]
            if self.long_check.get_active():
                args += ["--long", f"{self.long_minutes.get_value_as_int()}m",
                         "--every", str(self.long_every.get_value_as_int())]
            self.run(args)

    def set_allowance(self, _button):
        keep = self.reward_keep.get_active_id() or "one"
        kept = self.reward_keep_time if keep == "time" else None
        if self.selected and self.fields_ok(("Break", self.allowance_minutes), ("Blocking", self.allowance_per),
                                            ("Kept for", kept)):
            self.run(["break", "reward", self.selected,
                      f"{self.allowance_minutes.get_value_as_int()}m",
                      "--per", f"{self.allowance_per.get_value_as_int()}m",
                      "--keep", kept.arg() if kept else keep])

    def clear_allowance(self, _button):
        if self.selected:
            self.run(["break", "reward", self.selected, "off"])

    def start_break(self, _button):
        if self.selected:
            self.run(["break", "start", self.selected])

    def end_break(self, _button):
        if self.selected:
            self.run(["break", "end", self.selected])

    def set_relock(self, _button):
        if self.selected and self.fields_ok(
                ("Text break", self.relock_minutes if self.relock_check.get_active() else None)):
            value = f"{self.relock_minutes.get_value_as_int()}m" if self.relock_check.get_active() else "off"
            self.run(["break", "text", self.selected, value])

    def set_delay_break(self, _button):
        if not self.selected:
            return
        if self.delay_break_check.get_active():
            if not self.fields_ok(("Delay break", self.delay_break_minutes), ("Waiting", self.delay_wait_minutes)):
                return
            self.run(["break", "delay", self.selected, f"{self.delay_break_minutes.get_value_as_int()}m",
                      f"{self.delay_wait_minutes.get_value_as_int()}m"])
        else:
            self.run(["break", "delay", self.selected, "off"])

    def clear_pomodoro(self, _button):
        if self.selected:
            self.run(["pomodoro", "off", self.selected])

    def add_schedule(self, values):
        if len(values) != 3:
            self.message("Enter days, start and end, for example: weekdays 09:00 17:00")
            return
        self.run(["schedule", "add", self.selected, "--days", values[0],
                  "--from", values[1], "--to", values[2], *self.window_flags()])

    def create_window_from_calendar(self, column, start, end):
        if not self.selected:
            return
        day, frm, to = DAY_NAMES[column_day(column)], f"{start // 60:02d}:{start % 60:02d}", f"{end // 60:02d}:{end % 60:02d}"
        if self.confirm(f"Add {day.capitalize()} {show_clock(start)}–{show_clock(end, True)} to {self.selected}?",
                        "While the block is on, it is enabled then. To repeat it on other days, type "
                        "them in the list below, e.g. weekdays " + frm + " " + to + ".", "Add"):
            self.run(["schedule", "add", self.selected, "--days", day, "--from", frm, "--to", to, *self.window_flags()])

    def remove_window_from_calendar(self, index):
        block = self.blocks.get(self.selected)
        if not block or index >= len(block["schedules"]):
            return
        if self.confirm(f"Remove {block['schedules'][index]}?",
                        "With a schedule lock, this can't be done during the window or its lead time.",
                        "Remove", destructive=True):
            self.run(["schedule", "remove", self.selected, str(index + 1)])

    def set_automation(self, button, setting, value):
        """§3.6: changes the block's default or its both/either; a refused
        change goes back (update_automation shows the state)."""
        if self.updating or not button.get_active() or not self.selected:
            return
        self.run(["automation", self.selected, setting, value])

    def update_automation(self, block):
        """Shows what switches the block (§3.6)."""
        settings = block.get("automation") or {}
        windows, places = bool(block.get("schedules")), bool(block.get("places"))
        if block.get("continuous", not (windows or places)):
            text = ("Continuous: enabled whenever it is on. Add windows or places below to switch it "
                    "by itself.")
        else:
            by = " and ".join(part for part, has in (("its windows", windows), ("its places", places)) if has)
            text = (f"Automated: {by} turn it {'off' if settings.get('default_on') else 'on'}; "
                    f"it is {'enabled' if settings.get('default_on') else 'lifted'} otherwise.")
        self.auto_summary.set_text(text)
        was, self.updating = self.updating, True
        self.auto_buttons[("default", "enabled" if settings.get("default_on") else "disabled")].set_active(True)
        self.auto_buttons[("needs", "both" if settings.get("need_both") else "either")].set_active(True)
        self.updating = was
        self.auto_need_row.set_visible(windows and places)

    def window_flags(self):
        """§3.6: a window follows its block's default: on a block enabled by
        default it turns the block off (an allow window)."""
        return ["--allow"] if ((self.blocks.get(self.selected) or {}).get("automation") or {}).get("default_on") else []

    def replace_window(self, index, days, start, end, allow=False):
        """Changes a window: removes it, then adds the new one (§4.5; a
        change that weakens is refused while the block is locked)."""
        names = [DAY_NAMES[(bit - 1) % 7] for bit in range(7) if days & (1 << bit)]
        if not names:
            return
        args = ["schedule", "add", self.selected, "--days", ",".join(names),
                "--from", clock_text(start), "--to", "24:00" if end >= 1440 else clock_text(end)]
        if allow:
            args.append("--allow")
        self.run(["schedule", "remove", self.selected, str(index + 1)], then=lambda _o: self.run(args))

    def move_window_from_calendar(self, index, days, start, end):
        windows = (self.blocks.get(self.selected) or {}).get("schedule_windows") or []
        if index < len(windows) and windows[index].get("source"):
            self.window_from_schedule(windows[index])
        elif index < len(windows):
            self.replace_window(index, days, start, end, windows[index].get("allow", False))

    def window_from_schedule(self, w):
        """§3.6: a window from a schedule is changed on the Schedules page."""
        if self.confirm(f"This window comes from the schedule {w['source']}",
                        "Change it on the Schedules page (that changes everything using it), or take the "
                        "schedule off this block under Automation.", "Open Schedules"):
            self.open_schedule(w["source"])

    def apply_schedule_to_block(self, _button):
        block = self.blocks.get(self.selected) or {}
        have = set(block.get("applied_schedules") or [])
        names = [sc["name"] for sc in self.state.get("schedules") or [] if sc["name"] not in have]
        if not names:
            if self.confirm("No schedule to apply", "Make schedules such as Evening or Weekend on the "
                            "Schedules page, then apply them here.", "Open Schedules"):
                self.stack.set_visible_child_name("schedules")
            return
        name = self.choose("Apply a schedule", "Its windows join the block's; changing the schedule on the "
                           "Schedules page changes them too.", names)
        if name and self.selected:
            self.run(["schedule", "apply", self.selected, name])

    def edit_window_from_calendar(self, index):
        """§4.5: click a window to change its days (copy to days) and times,
        or remove it."""
        block = self.blocks.get(self.selected)
        windows = (block or {}).get("schedule_windows") or []
        if not block or index >= len(windows):
            return
        w = windows[index]
        if w.get("source"):
            self.window_from_schedule(w)
            return
        dialog = Gtk.Dialog(title="Window", transient_for=self, modal=True)
        dialog.add_button("Remove", 3)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Save", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        days_row = Gtk.Box(spacing=6)
        checks = {}
        for column in range(7):
            day = column_day(column)
            bit = 1 << ((day + 1) % 7)
            check = Gtk.CheckButton(label=DAY_NAMES[day].capitalize(), active=bool(w["days"] & bit))
            checks[bit] = check
            days_row.pack_start(check, False, False, 0)
        box.pack_start(dim("Tick more days to copy the window to them."), False, False, 0)
        box.pack_start(days_row, False, False, 0)
        times = Gtk.Box(spacing=6)
        start = Gtk.Entry(text=clock_text(w["start"]), width_chars=5)
        end = Gtk.Entry(text="24:00" if w["end"] >= 1440 else clock_text(w["end"]), width_chars=5)
        for widget in (Gtk.Label(label="From"), start, Gtk.Label(label="to"), end):
            times.pack_start(widget, False, False, 0)
        box.pack_start(times, False, False, 0)
        # §3.6: one break each time this window is open.
        break_row = Gtk.Box(spacing=6)
        break_minutes = DurationField(w.get("break", 0), minimum=60, maximum=4 * 3600, zero="No break", width=8)
        for widget in (Gtk.Label(label="Break each time"), break_minutes):
            break_row.pack_start(widget, False, False, 0)
        if not w.get("allow"):
            box.pack_start(break_row, False, False, 0)
        self.window_dialog = (checks, start, end, break_minutes)  # for tests
        dialog.show_all()
        answer = dialog.run()
        days = sum(bit for bit, check in checks.items() if check.get_active())
        frm, to = start.get_text().strip(), end.get_text().strip()
        new_break = break_minutes.get_value_as_int()
        break_problem = break_minutes.problem
        dialog.destroy()
        if answer == 3:
            self.remove_window_from_calendar(index)
            return
        if answer != Gtk.ResponseType.OK:
            return
        if break_problem:
            self.message(f"Break: {break_problem}", Gtk.MessageType.INFO)
            return
        try:
            new_start = int(frm[:2]) * 60 + int(frm[3:5])
            new_end = 1440 if to == "24:00" else int(to[:2]) * 60 + int(to[3:5])
        except (ValueError, IndexError):
            self.message("Times are HH:MM, e.g. 09:00 and 17:00.", Gtk.MessageType.INFO)
            return
        if (days, new_start, new_end) != (w["days"], w["start"], w["end"]):
            self.replace_window(index, days, new_start, new_end, w.get("allow", False))
        elif new_break != w.get("break", 0) // 60:
            self.run(["schedule", "break", self.selected, str(index + 1), f"{new_break}m" if new_break else "off"])

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
        """Runs command (an import with its block and list) with a file, or
        for a block's list, copies other lists and clusters into it (§4.3
        import from KC9K)."""
        if not self.selected:
            return
        if command[:2] == ["list", "import"]:
            answer = self.ask_import_source(
                "Import into this list", "From a text file with one entry per line (a hosts file and "
                "adblock rules are read too), or from other lists and clusters in Killer Chicken 9000; "
                "or the whole internet, or all searches containing some words.", more=True)
            if answer == "kc9k":
                self.import_from_kc9k(command[2], command[3])
                return
            if answer == "internet":
                self.import_internet(command[2], command[3])
                return
            if answer == "searches":
                self.import_searches(command[2], command[3])
                return
            if answer != "file":
                return
        path = self.choose_file("Import list", Gtk.FileChooserAction.OPEN, "Import")
        if not path:
            return
        if command[:2] != ["list", "import"]:
            self.run(command + [path], then=self.queued_note)
            return
        # §4.3: KC9K reads the file's layout; phrases in it go to a phrase set.
        found = self.read_import_file(path)
        if found is None:
            return
        block, steps = command[2], []
        if found["phrases"]:
            target = self.choose_phrase_set(block, len(found["phrases"]))
            if target:
                name, new = target
                if new:
                    steps.append(["phrase-set", "create", block, name])
                steps.append(["phrase-set", "import", block, name, path])
        if found["entries"]:
            steps.append(command + [path])
        if not steps:
            if not found["phrases"]:
                self.message("Nothing in that file can go in a list: " + describe_found(found) + ".",
                             Gtk.MessageType.INFO)
            return
        self.run_steps(steps, routed_phrases=bool(found["phrases"]))

    def import_from_kc9k(self, block, list_name):
        """Ticks other blocks' lists and clusters to copy into a list."""
        sources = []
        for other in (self.state or {}).get("blocks") or []:
            if other.get("hidden"):
                continue
            for lst in other.get("lists") or []:
                if (other["name"], lst["name"]) != (block, list_name) and lst.get("entries"):
                    sources.append((f"{other['name']}: {lst['name']}", ["list", "copy", other["name"], lst["name"], block, list_name]))
        for cluster in (self.state or {}).get("clusters") or []:
            if not cluster.get("phrases"):
                sources.append((f"Cluster: {cluster['name']}", ["cluster", "copy", cluster["name"], block, "--into", list_name]))
        if not sources:
            self.message("No other lists or clusters to import from.", Gtk.MessageType.INFO)
            return
        dialog = Gtk.Dialog(title="Import from Killer Chicken 9000", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Import", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(4)
        box.set_border_width(12)
        checks = []
        for label, command in sources:
            check = Gtk.CheckButton(label=label)
            box.pack_start(check, False, False, 0)
            checks.append((check, command))
        dialog.show_all()
        answer = dialog.run()
        chosen = [command for check, command in checks if check.get_active()]
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return

        def next_copy(_output=None):
            if chosen:
                self.run(chosen.pop(0), then=next_copy)

        next_copy()

    def ask_import_source(self, title, detail, more=False):
        """From a file or from KC9K: "file", "kc9k" or None; with more (a
        list, §4.3), also "internet" (the whole internet) and "searches"
        (all searches containing some words)."""
        dialog = Gtk.MessageDialog(transient_for=self, modal=True, message_type=Gtk.MessageType.QUESTION,
                                   buttons=Gtk.ButtonsType.NONE, text=title, secondary_text=detail)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        if more:
            dialog.add_button("Whole internet", 3)
            dialog.add_button("All searches containing…", 4)
        dialog.add_button("From KC9K…", 2)
        dialog.add_button("From file…", Gtk.ResponseType.OK)
        answer = dialog.run()
        dialog.destroy()
        return {Gtk.ResponseType.OK: "file", 2: "kc9k", 3: "internet", 4: "searches"}.get(answer)

    def import_internet(self, block, name):
        """§4.3 Whole internet: *.* into the list."""
        if self.confirm(f"Add the whole internet to {name}?",
                        "It gets *.*: every site is matched (local files, localhost and the local network "
                        "stay reachable). In a blacklist, only the block's exceptions are let through.", "Add"):
            self.run(["list", "add", block, name, "*.*"])

    def import_searches(self, block, name):
        """§4.3 All searches containing…: the words as rendered terms that
        look at the address, where a search's words are (R-PHRASE-6)."""
        text = self.ask_text(
            "All searches containing…",
            "Words, separated by commas. A search for one of them is matched by its address (it holds "
            "the words searched for), and so is any other address with that word in it.",
            problem=lambda t: None if [w for w in t.split(",") if w.strip()] else "Type at least one word.")
        words = [w.strip() for w in (text or "").split(",") if w.strip()]
        if not words:
            return
        list_data = next((l for l in (self.blocks.get(block) or {}).get("lists", []) if l["name"] == name), {})
        steps = [["list", "add", block, name, *words]]
        if "url" not in (list_data.get("where") or []):
            steps.append(["list", "scope", block, name, "url", "on"])
        self.run_steps(steps)

    def read_import_file(self, path):
        """What a file to import holds, as KC9K reads it (§4.3): phrases,
        entries (sites and rendered terms), what it can't use. None after
        saying why it can't be read."""
        ok, output = cli_with_access(self.access_passwords, ["list", "read", path])
        if not ok:
            self.message(output)
            return None
        try:
            return json.loads(output)
        except ValueError:
            self.message(output)
            return None

    def run_steps(self, steps, routed_phrases=False):
        """Runs commands one after another; then says what they did, once."""
        said = []

        def next_step(output=None):
            if output:
                said.extend(line for line in output.splitlines()
                            if not (routed_phrases and line.startswith("Left out") and " phrase" in line))
            if steps:
                self.run(steps.pop(0), then=next_step)
            elif said:
                self.message("\n".join(said), Gtk.MessageType.INFO)

        next_step()

    def choose_phrase_set(self, block, count):
        """Where the phrases of an imported file go: one of the block's
        phrase sets or a new one, as (name, new); None leaves them out."""
        names = [p["name"] for p in self.blocks.get(block, {}).get("phrase_sets") or []]
        dialog = Gtk.Dialog(title="Phrases in the file", transient_for=self, modal=True)
        dialog.add_button("Leave them out", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Add them", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(8)
        box.set_border_width(12)
        box.pack_start(dim(f"{count} {'line is a phrase' if count == 1 else 'lines are phrases'} "
                           "([a, b] [c]): a list can't hold phrases. Add them to a phrase set of "
                           f"'{block}':"), False, False, 0)
        combo = Gtk.ComboBoxText()
        for name in names:
            combo.append(name, name)
        combo.append("", "A new phrase set:")
        combo.set_active(0)
        new_name = Gtk.Entry(text="Imported phrases", sensitive=not names)
        combo.connect("changed", lambda c: new_name.set_sensitive(c.get_active_id() == ""))
        box.pack_start(combo, False, False, 0)
        box.pack_start(new_name, False, False, 0)
        self.phrase_target = (dialog, combo, new_name)  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen, typed = combo.get_active_id(), new_name.get_text().strip()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return None
        if chosen:
            return chosen, False
        return (typed, typed.lower() not in (n.lower() for n in names)) if typed else None

    def import_into_phrase_set(self, card):
        """§4.3, one Import for a phrase set (owner, 2026-11-16): KC9K reads
        the file's layout. Phrases ([eat, nice] [hello, grandpa]) go into
        the set; rendered terms, one per line, become one word set in the
        phrase editor below ([term 1, term 2, …]); a list of sites can go
        to the block's Blocked list. From KC9K: other lists' and clusters'
        entries as one word set."""
        if not self.selected:
            return
        block, name = self.selected, card.data["name"]
        answer = self.ask_import_source(
            "Import into this phrase set",
            "From a text file, one per line: phrases such as [eat, nice] [hello, grandpa] are added to "
            "the set; rendered terms become one word set in the phrase editor. Or from lists and "
            "clusters in Killer Chicken 9000: their entries become one word set.")
        if answer == "kc9k":
            terms = self.pick_kc9k_terms()
            if terms:
                card.editor.add_imported(terms)
                self.message(f"{len(terms)} rendered terms are now a word set in the phrase editor of "
                             f"'{name}'. Fill the other word set and click Add phrase.", Gtk.MessageType.INFO)
            return
        if answer != "file":
            return
        path = self.choose_file("Import into a phrase set", Gtk.FileChooserAction.OPEN, "Import")
        if not path:
            return
        found = self.read_import_file(path)
        if found is None:
            return
        if found["phrases"]:
            self.run_steps([["phrase-set", "import", block, name, path]])
        elif found["terms"]:
            card.editor.add_imported(word_set_terms(found["terms"]))
            note = f" ({len(found['sites'])} sites in the file were left out)" if found["sites"] else ""
            self.message(f"{len(found['terms'])} rendered terms are now a word set in the phrase editor of "
                         f"'{name}'{note}. Fill the other word set and click Add phrase.", Gtk.MessageType.INFO)
        elif found["sites"]:
            if self.confirm("The file lists sites",
                            f"It lists {len(found['sites'])} sites, which belong in a list, not in a phrase "
                            f"set. Add them to the list 'Blocked' of '{block}'?", "Add to Blocked"):
                self.run_steps([["list", "import", block, "Blocked", path]])
        else:
            self.message("Nothing in that file can be imported: " + describe_found(found) + ".",
                         Gtk.MessageType.INFO)

    def pick_kc9k_terms(self):
        """Entries of lists and clusters in KC9K, ticked, for one word set."""
        sources = []
        for other in (self.state or {}).get("blocks") or []:
            if other.get("hidden"):
                continue
            for lst in other.get("lists") or []:
                if lst.get("entries"):
                    sources.append((f"{other['name']}: {lst['name']}", lst["entries"]))
        for cluster in (self.state or {}).get("clusters") or []:
            if not cluster.get("phrases") and cluster.get("entries"):
                sources.append((f"Cluster: {cluster['name']}", cluster["entries"]))
        if not sources:
            self.message("No lists or clusters to import from.", Gtk.MessageType.INFO)
            return []
        dialog = Gtk.Dialog(title="Import a word set from Killer Chicken 9000", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        dialog.add_button("Import", Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(4)
        box.set_border_width(12)
        checks = []
        for label, entries in sources:
            check = Gtk.CheckButton(label=label)
            box.pack_start(check, False, False, 0)
            checks.append((check, entries))
        self.term_sources = checks  # for tests
        dialog.show_all()
        answer = dialog.run()
        chosen = [e for check, entries in checks if check.get_active() for e in entries]
        dialog.destroy()
        return word_set_terms(chosen) if answer == Gtk.ResponseType.OK else []

    def export_list(self, command):
        """Saves what command (an export) prints to a file."""
        if not self.selected:
            return
        path = self.choose_file("Export list", Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, command)
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n" if output else "")
        except OSError as error:
            self.message(str(error))

    def share_project(self, button):
        self.export_project(button, share=True)

    def export_project(self, _button, share=False):
        path = self.choose_file("Share the setup (without places)" if share else "Back up the whole setup",
                                Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, ["project", "export"] + (["--share"] if share else []))
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n")
        except OSError as error:
            self.message(str(error))

    def import_project(self, _button):
        path = self.choose_file("Import a setup", Gtk.FileChooserAction.OPEN, "Import")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, ["project", "import", "--dry-run", path])
        if not ok:
            self.message(output)
            return
        if "Would:" not in output:
            self.message(output or "The file adds nothing new.", Gtk.MessageType.INFO)
            return
        locks = [line.split("Would: set a lock: ", 1)[1] for line in output.splitlines()
                 if "Would: set a lock: " in line]
        # R-CUSTOM-1: what its custom actions open or run is in the list above.
        customs = "Would: add a custom action that can " in output
        if self.confirm("Import this setup?", output.replace("Would: ", "• "), "Import") \
                and self.confirm_lock(locks):
            self.run(["project", "import", path] + (["--yes"] if locks or customs else []),
                     then=lambda result: self.message(result, Gtk.MessageType.INFO))

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
            lambda values: self.cluster_name() and self.run(["cluster", "add", self.cluster_name(), *values]),
            lambda value, _n: self.cluster_name() and self.run(["cluster", "remove", self.cluster_name(), value]),
            search=True, multi=True)
        box.pack_start(self.cluster_entries, True, True, 0)

        self.record_label = Gtk.Label(xalign=0, wrap=True)
        record_row = Gtk.Box(spacing=6)
        self.record_start = Gtk.Button(label="Record into this preset")
        self.record_start.connect("clicked", lambda _b: self.cluster_name() and self.run(
            ["record", "start", "--cluster", self.cluster_name()]))
        # §3.12 word recording: the rendered words of the pages, ranked by
        # how specific they are; the least specific cut before adding.
        self.record_words = Gtk.Button(label="Record words")
        self.record_words.set_tooltip_text("Record the words shown on the pages you open (the preset must look at page text)")
        self.record_words.connect("clicked", lambda _b: self.cluster_name() and self.run(
            ["record", "start", "--cluster", self.cluster_name(), "--words"]))
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
                self.run(["block", "create", new, "--template", name])
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
            self.run(["template", "remove", name] if kind == "block" else ["preset", "delete", name, "--kind", kind])

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
            target = (f"the cluster {recording['cluster']}" if recording.get("cluster")
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
        self.run(["cluster", "scope", self.cluster_name(), place, "on" if check.get_active() else "off"])

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
            self.run(["cluster", "create", name] + (["--phrases"] if as_phrases else []))

    def rename_cluster(self, _button):
        old = self.cluster_name()
        name = old and self.ask_text(f"Rename {old}", initial=old)
        if name and name != old:
            self.cluster_wanted = name
            self.run(["cluster", "rename", old, name])

    def delete_cluster(self, _button):
        name = self.cluster_name()
        if name and self.confirm(f"Delete the preset {name}?",
                                 "Lists copied from it into blocks stay as they are.", "Delete", destructive=True):
            self.run(["cluster", "delete", name])

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
            self.run(["cluster", "copy", name, block] + (["--exceptions"] if as_exceptions else []),
                     then=self.queued_note)

    def import_cluster(self, _button):
        name = self.cluster_name()
        path = name and self.choose_file("Import entries into the preset", Gtk.FileChooserAction.OPEN, "Import")
        if path:
            self.run(["cluster", "import", name, path])

    def export_cluster(self, _button):
        name = self.cluster_name()
        path = name and self.choose_file("Export the preset", Gtk.FileChooserAction.SAVE, "Save")
        if not path:
            return
        ok, output = cli_with_access(self.access_passwords, ["cluster", "export", name])
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n" if output else "")
        except OSError as error:
            self.message(str(error))

    # Frozen Chicken ------------------------------------------------------
    def build_device(self):
        """Frozen Chicken's part of its mode's view (§3.8, §4.8, owner
        2026-11-17): freeze now, its scheduled freezes, its allowed apps."""
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=18)
        box.pack_start(dim(
            "Lock the whole computer for a while: the screen, your session, or the power. It keeps the "
            "computer unusable until the time is up, even across restarts, and can't be cancelled, only "
            "extended. The time keeps counting while the computer is off. At most a week at a time."),
            False, False, 0)

        self.freeze_status = Gtk.Label(xalign=0)
        row = Gtk.Box(spacing=8)
        self.freeze_mode = Gtk.ComboBoxText()
        self.freeze_mode.append("lock", "Lock the screen")
        self.freeze_mode.append("logout", "Log out")
        self.freeze_mode.append("shutdown", "Shut down")
        self.freeze_mode.append("apps", "Allowed apps only")  # §3.8: kc9k freeze apps
        self.freeze_mode.set_active_id("lock")
        self.freeze_minutes = DurationField(1800, minimum=60, maximum=FREEZE_MAX, presets=(1800, 3600, 4 * 3600, 86400),
                                            why_max="Frozen Chicken lasts at most a week.", width=9)
        self.freeze_button = Gtk.Button(label="Start Frozen Chicken…")
        self.freeze_button.get_style_context().add_class("destructive-action")
        self.freeze_button.connect("clicked", self.freeze)
        row.pack_start(self.freeze_mode, False, False, 0)
        row.pack_start(Gtk.Label(label="for"), False, False, 0)
        row.pack_start(self.freeze_minutes, False, False, 0)
        row.pack_end(self.freeze_button, False, False, 0)
        self.freeze_schedules = EditableList(
            "", "daily 23:00 24:00", self.add_freeze_schedule, self.remove_freeze_schedule,
            help_text="Freeze at set times in the mode chosen above, for example every night. "
                      "Days, start and end; for overnight add two (23:00 24:00 and 00:00 06:00). "
                      "A locked schedule can't be removed from its lead time before it starts until it ends.")
        # §3.8: the apps an "allowed apps only" freeze lets run.
        self.freeze_apps = EditableList(
            "", "libreoffice  /opt/notes/",
            lambda values: self.run(["freeze", "apps", "add", *values]),
            lambda value, _num: self.run(["freeze", "apps", "remove", value]),
            help_text="While an 'allowed apps only' freeze runs, every other program of yours is closed; "
                      "the desktop keeps working. Adding an app isn't possible while a freeze runs or "
                      "anything is locked.")
        self.freeze_lock = Gtk.CheckButton(label="Lock new schedules from")
        self.freeze_lead = DurationField(7200, minimum=60, maximum=7 * 86400, zero="No lead time", width=9)
        freeze_lock_row = Gtk.Box(spacing=6)
        freeze_lock_row.pack_start(self.freeze_lock, False, False, 0)
        freeze_lock_row.pack_start(self.freeze_lead, False, False, 0)
        freeze_lock_row.pack_start(Gtk.Label(label="before each window", valign=Gtk.Align.START, margin_top=6),
                                   False, False, 0)
        top_align(freeze_lock_row)
        top_align(row)
        box.pack_start(section("Freeze now", row, self.freeze_status), False, False, 0)
        flow_row(row)
        # §3.8: every lock kind on a scheduled freeze (not removable then).
        freeze_locks_row = Gtk.Box(spacing=6)
        freeze_locks_row.pack_start(Gtk.Label(label="The selected schedule:"), False, False, 0)
        for label, handler in (("Lock…", self.lock_freeze_schedule), ("Unlock…", self.unlock_freeze_schedule)):
            button = Gtk.Button(label=label)
            button.connect("clicked", handler)
            freeze_locks_row.pack_start(button, False, False, 0)
        box.pack_start(section("Scheduled freezes", self.freeze_schedules, freeze_lock_row, freeze_locks_row),
                       False, False, 0)
        flow_row(freeze_lock_row)
        box.pack_start(section("Allowed apps", self.freeze_apps), False, False, 0)
        return box

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
        lock = self.freeze_lock.get_active()
        lead = self.freeze_lead.get_value_as_int()
        if lock and not self.fields_ok(("Lead time", self.freeze_lead)):
            return
        if lock:
            confirmed = self.confirm_lock([
                f"freeze schedule {values[0]} {values[1]}-{values[2]} "
                f"({self.freeze_mode.get_active_text().lower()}): schedule lock, while each window is open"
                + (f" and {duration_words(lead * 60)} before it" if lead else "")])
        else:
            confirmed = self.confirm(f"Freeze {values[0]} {values[1]}-{values[2]}?",
                                     f"The computer will be frozen then "
                                     f"({self.freeze_mode.get_active_text().lower()}).", "Schedule")
        if confirmed:
            self.run(["freeze", "schedule", "add", "--days", values[0], "--from", values[1],
                      "--to", values[2], "--mode", mode, "--yes"] + (["--lock-lead", f"{lead}m"] if lock else []))

    def remove_freeze_schedule(self, _value, number):
        self.run(["freeze", "schedule", "remove", str(number)])

    def chosen_freeze_schedule(self):
        """The schedule picked in the list, as (its text, its number)."""
        chosen = self.freeze_schedules._selected()
        if not chosen:
            self.message("Pick a schedule in the list first.", Gtk.MessageType.INFO)
            return None
        return chosen[-1]

    def lock_freeze_schedule(self, _button):
        """§3.8: timer, text, password, restart, range locks and re-lock on
        a scheduled freeze; while on it can't be removed (R-FREEZE-2)."""
        chosen = self.chosen_freeze_schedule()
        if not chosen:
            return
        text, number = chosen
        picker = LockPicker("remove it", "removed", ("timer", "password", "text", "restart", "range", "relock"))
        asked = self.lock_dialog(f"Lock the freeze schedule {text}",
                                 "It can't be removed until the locks end. Locks can be added later, but "
                                 "only made stronger.", "Lock", picker, need_lock=True)
        if asked is None:
            return
        locks, relock_flags, secret = asked
        if not locks:
            self.message("Choose at least one lock.", Gtk.MessageType.INFO)
            return
        if len(locks) > 1:
            locks = self.choose_lock_order(locks)
            if locks is None:
                return
        args = ["freeze", "schedule", "lock", str(number)]
        lines = []
        for i, (flags, words) in enumerate(locks):
            args += flags
            lines.append(f"freeze schedule {text}: " + (f"{i + 1} of {len(locks)}: " if len(locks) > 1 else "") + words)
        if self.confirm_lock(lines):
            self.run(args + relock_flags + ["--yes"], secret + "\n" if secret else None)

    def unlock_freeze_schedule(self, _button):
        chosen = self.chosen_freeze_schedule()
        if not chosen:
            return
        text, number = chosen
        locks = self.state.get("freeze_schedule_locks") or []
        info = locks[number - 1] if number - 1 < len(locks) else {}
        unlock = ["freeze", "schedule", "unlock", str(number)]
        if (info.get("lock_order") or [])[:1] == ["password"]:
            password = self.ask_text(f"Unlock the freeze schedule {text}", "Enter its password.", password=True)
            if password:
                self.run(unlock, password + "\n")
            return
        if not info.get("char_locked"):
            self.message("Its first lock isn't a text or password lock: it ends by itself.", Gtk.MessageType.INFO)
            return
        command = " ".join([CLI] + [shlex.quote(a) for a in unlock])
        try:
            subprocess.Popen(["x-terminal-emulator", "-e", command])
        except OSError as error:
            self.message(f"Can't open a terminal for the typing challenge: {error}\n\nRun: {command}")

    def update_device(self):
        self.freeze_schedules.set_items(self.state.get("freeze_schedules", []))
        self.freeze_apps.set_items(self.state.get("freeze_apps", []))
        frozen = self.state["freeze_seconds"]
        modes = {"lock": "locking the screen", "logout": "logging out", "shutdown": "shutting down"}
        self.freeze_status.set_markup(
            f"<b>On</b> for {fmt_seconds(frozen)} more, {modes.get(self.state['freeze_mode'], '')}. "
            "Starting again can only extend it." if frozen else "Off.")
        self.freeze_button.set_label("Extend…" if frozen else "Start Frozen Chicken…")

    def freeze_dialog(self, answer=None):
        """§3.8, R-FREEZE-3: Frozen Chicken switched on asks how (lock,
        log out, shut down, allowed apps) and until when, at most a week
        ahead. answer: (mode, flags) for tests."""
        if answer is None:
            dialog = Gtk.Dialog(title="Freeze the computer", transient_for=self, modal=True)
            dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
            ok = dialog.add_button("Freeze", Gtk.ResponseType.OK)
            ok.get_style_context().add_class("destructive-action")
            box = dialog.get_content_area()
            box.set_spacing(10)
            box.set_border_width(14)
            box.pack_start(dim("Until the time is up the computer can't be used as chosen here, also after a "
                               "restart. It can't be cancelled, only extended. Save your work first."), False, False, 0)
            how = Gtk.ComboBoxText()
            for key, label in (("lock", "Lock the screen"), ("logout", "Log out"), ("shutdown", "Shut down"),
                               ("apps", "Allowed apps only")):
                how.append(key, label)
            how.set_active_id(self.freeze_mode.get_active_id() or "lock")
            box.pack_start(how, False, False, 0)
            until = TimerLockField(allow_none=False, seconds=3600)
            until.duration.set_limits(60, FREEZE_MAX, why_max="Frozen Chicken lasts at most a week.")
            box.pack_start(until, False, False, 0)
            self.freeze_dialog_parts = (dialog, how, until)  # for tests
            dialog.show_all()
            reply = dialog.run()
            mode, flags, problem = how.get_active_id(), until.flags(), until.problem()
            moment = until.until() if until.mode == "until" else None
            dialog.destroy()
            if reply != Gtk.ResponseType.OK:
                return
            if problem or not flags:
                self.message(problem or "Choose how long.", Gtk.MessageType.INFO)
                return
            if moment and (moment - datetime.datetime.now()).total_seconds() > FREEZE_MAX:
                self.message("Frozen Chicken lasts at most a week: pick an earlier date.", Gtk.MessageType.INFO)
                return
        else:
            mode, flags = answer
        self.run(["freeze", *flags, "--mode", mode, "--yes"])

    def freeze(self, _button):
        if not self.fields_ok(("Frozen Chicken", self.freeze_minutes)):
            return
        minutes = self.freeze_minutes.get_value_as_int()
        mode = self.freeze_mode.get_active_id()
        effect = {
            "lock": "The screen locks now and keeps locking again until the time is up.",
            "logout": "Every session is logged out now, and logging in again ends at once.",
            "shutdown": "The computer shuts down now, and again whenever it starts.",
        }[mode]
        if self.confirm(f"Freeze the computer for {duration_words(minutes * 60)}?",
                        f"{effect} It can't be cancelled. Save your work first.",
                        "Freeze", destructive=True):
            self.run(["freeze", "--for", f"{minutes}m", "--mode", mode, "--yes"])

    def toggle_password_locks(self, check):
        if self.updating:
            return
        self.run(["password", "locks", "on" if check.get_active() else "off"])

    def dns_choice_changed(self, combo):
        self.dns_servers.set_visible(combo.get_active_id() == "own")

    def apply_dns(self, _button):
        choice = self.dns_choice.get_active_id()
        if choice == "own":
            choice = ",".join(s.strip() for s in self.dns_servers.get_text().split(",") if s.strip())
            if not choice:
                self.message("Enter one or more server addresses, separated by commas.")
                return
        self.run(["dns", choice])

    def toggle_reset_guard(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["reset-guard", "on" if wanted else "off"])
        return True

    def toggle_no_weakening(self, _switch, wanted):
        if self.updating:
            return False
        if wanted and not self.confirm(
                "Turn on no weakening during active locks?",
                "While any block or setting is locked, no setting can be weakened, and this "
                "can't be turned off either until every lock has ended.", "Turn on"):
            return True
        self.run(["no-weakening", "on" if wanted else "off"])
        return True

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

    # Settings ------------------------------------------------------------
    def choose_darkness(self, scale):
        app = self.get_application()
        app.prefs["darkness"] = int(scale.get_value())
        save_prefs(app.prefs)
        app.theme.apply(app.prefs.get("theme", "system"), app.prefs["darkness"])

    def toggle_tray(self, _switch, wanted):
        app = self.get_application()
        app.prefs["tray"] = bool(wanted)
        save_prefs(app.prefs)
        app.set_tray(bool(wanted))
        return False

    def choose_clock(self, combo):
        global CLOCK_24
        app = self.get_application()
        app.prefs["clock_24"] = CLOCK_24 = combo.get_active_id() == "24"
        save_prefs(app.prefs)
        self.queue_draw()
        if hasattr(self, "calendar"):
            self.calendar.queue_draw()

    def choose_first_day(self, combo):
        global FIRST_DAY
        app = self.get_application()
        app.prefs["first_day"] = combo.get_active_id()
        FIRST_DAY = 6 if app.prefs["first_day"] == "sunday" else 0
        save_prefs(app.prefs)
        if hasattr(self, "calendar"):
            self.calendar.queue_draw()

    def toggle_open_at_login(self, _switch, wanted):
        """§4.10: a desktop autostart entry for this window."""
        try:
            if wanted:
                os.makedirs(os.path.dirname(LOGIN_DESKTOP), exist_ok=True)
                with open(LOGIN_DESKTOP, "w") as f:
                    f.write("[Desktop Entry]\nType=Application\nName=Killer Chicken 9000\n"
                            "Exec=kc9k-gui\nIcon=changes-prevent\nTerminal=false\n")
            elif os.path.exists(LOGIN_DESKTOP):
                os.remove(LOGIN_DESKTOP)
        except OSError as error:
            self.message(f"Can't change opening at login: {error}")
            return True
        return False

    def choose_theme(self, combo):
        app = self.get_application()
        app.prefs["theme"] = combo.get_active_id()
        app.theme.apply(app.prefs["theme"], app.prefs.get("darkness"))
        save_prefs(app.prefs)

    def build_settings(self):
        """Groups in the order of SPEC §4.10."""
        scroll, box = page_box()
        page_header(box, "Settings", "How Killer Chicken looks, what protects it, and what it may never do.")

        theme = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        for key, label in THEMES:
            theme.append(key, label)
        theme.set_active_id(self.get_application().prefs.get("theme", "system"))
        theme.connect("changed", self.choose_theme)
        row = Gtk.Box(spacing=16)
        row.pack_start(Gtk.Label(label="Theme", xalign=0), True, True, 0)
        row.pack_end(theme, False, False, 0)
        darkness = Gtk.Scale.new_with_range(Gtk.Orientation.HORIZONTAL, 0, 100, 5)
        darkness.set_size_request(160, -1)
        darkness.set_value(self.get_application().prefs.get("darkness", 0))
        darkness.connect("value-changed", self.choose_darkness)
        dark_row = Gtk.Box(spacing=16)
        dark_row.pack_start(Gtk.Label(label="Darkness (dark theme): grey to black", xalign=0), True, True, 0)
        dark_row.pack_end(darkness, False, False, 0)
        self.darkness_scale = darkness  # for tests
        first_day = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        first_day.append("monday", "Monday")
        first_day.append("sunday", "Sunday")
        first_day.set_active_id(self.get_application().prefs.get("first_day", "monday"))
        first_day.connect("changed", self.choose_first_day)
        day_row = Gtk.Box(spacing=16)
        day_row.pack_start(Gtk.Label(label="First day of the week", xalign=0), True, True, 0)
        day_row.pack_end(first_day, False, False, 0)
        clock = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        clock.append("24", "24-hour (14:30)")
        clock.append("12", "12-hour (2:30 PM)")
        clock.set_active_id("24" if self.get_application().prefs.get("clock_24", True) else "12")
        clock.connect("changed", self.choose_clock)
        self.clock_combo = clock  # for tests
        self.tray_switch = Gtk.Switch(valign=Gtk.Align.CENTER,
                                      active=bool(self.get_application().prefs.get("tray")))
        self.tray_switch.connect("state-set", self.toggle_tray)
        clock_row = Gtk.Box(spacing=16)
        clock_row.pack_start(Gtk.Label(label="Clock", xalign=0), True, True, 0)
        clock_row.pack_end(clock, False, False, 0)
        self.login_switch = Gtk.Switch(valign=Gtk.Align.CENTER, active=os.path.exists(LOGIN_DESKTOP))
        self.login_switch.connect("state-set", self.toggle_open_at_login)
        box.pack_start(section("Preferences", row, dark_row, clock_row, day_row,
                               self.switch_row(self.login_switch, "Open at login",
                                               "Start this window when you sign in."),
                               self.switch_row(self.tray_switch, "Show in the tray",
                                               "An icon in the panel shows what is on; closing the "
                                               "window keeps it there.")), False, False, 0)

        self.no_weakening_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.no_weakening_switch.connect("state-set", self.toggle_no_weakening)
        box.pack_start(section(
            "No weakening during active locks",
            self.switch_row(self.no_weakening_switch, "Keep every setting while anything is locked",
                            "While any block or setting is locked, no setting can be weakened: "
                            "strict mode, browser settings, bypass prevention, the settings "
                            "password. Blocks without a lock stay free to edit. It can't be "
                            "turned off while anything is locked.")), False, False, 0)

        # Tamper protection: one group, changed in its own window (OK/Cancel).
        self.browser_switches = {}
        self.setting_locks = {}
        self.tamper_summary = Gtk.Label(xalign=0, wrap=True)
        tamper_change = Gtk.Button(label="Change…", halign=Gtk.Align.START)
        tamper_change.connect("clicked", self.tamper_dialog)
        tamper_rows = [
            dim("Closes the ways around your blocks from inside the computer. Once on, a tamper "
                "protection setting can't be turned off while anything is locked."),
            self.tamper_summary, tamper_change]
        # Block unsupported browsers keeps its own lock until the group has one.
        tamper_rows.append(self.setting_lock_line("unsupported-browsers", "Block unsupported browsers"))
        box.pack_start(section("Tamper protection", *tamper_rows), False, False, 0)

        # Browsers (§4.10): supported browsers found, extension status,
        # one-click install. Rows are made once the state names them.
        self.browser_rows = {}
        self.browser_list = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.extension_status = dim("")
        box.pack_start(section(
            "Browsers",
            dim("The Killer Chicken extension runs in the supported browsers: LibreWolf (a "
                "privacy-focused Firefox), Google Chrome and Chromium, which install it themselves. "
                "Firefox and other browsers can't run it. The extension is always on: it enforces "
                "wildcards, page titles and text, greyscale, images and web-only, and while a block "
                "has websites, browsers without it are closed."),
            self.browser_list, self.extension_status), False, False, 0)

        rows = [self.setting_row(name, title, detail) for name, title, detail in BROWSER_OPTIONS]
        box.pack_start(section(
            "Browser settings", dim("Each browser setting can be locked on its own with Lock…, "
                                    "independently of your blocks. A locked setting can't be turned "
                                    "off until its lock ends or is unlocked. Blocking developer "
                                    "tools also can't be turned off while a block is locked."), *rows),
            False, False, 0)

        # §3.7 use counting.
        self.strict_use_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.strict_use_switch.connect("state-set", self.toggle_strict_use)
        box.pack_start(section(
            "Use counting",
            self.switch_row(self.strict_use_switch, "Strict use counting",
                            "An allowance is spent while any tab of its sites is open and while "
                            "its apps run, even in the background or minimised. Turning it off "
                            "weakens your allowances.")), False, False, 0)

        # §4.10 notifications: which ones, and how many minutes ahead.
        self.notice_checks = {}
        notice_flow = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=3,
                                  column_spacing=12, row_spacing=2)
        for key, title in NOTICE_KINDS:
            check = Gtk.CheckButton(label=title)
            check.connect("toggled", self.toggle_notice, key)
            self.notice_checks[key] = check
            notice_flow.add(check)
        advance_row = Gtk.Box(spacing=6)
        advance_row.pack_start(Gtk.Label(label="Minutes ahead:"), False, False, 0)
        self.notice_advance = Gtk.Entry(width_chars=10, placeholder_text="2,5")
        advance_apply = Gtk.Button(label="Apply")
        advance_apply.connect("clicked", self.apply_notice_advance)
        advance_row.pack_start(self.notice_advance, False, False, 0)
        advance_row.pack_start(advance_apply, False, False, 0)
        box.pack_start(section(
            "Notifications",
            dim("Desktop notifications before things happen. A scheduled Frozen Chicken is "
                "always announced."),
            notice_flow, advance_row), False, False, 0)

        dns_row = Gtk.Box(spacing=6)
        self.dns_choice = Gtk.ComboBoxText()
        for key, label in (("network", "The network's own server, then Quad9"),
                           ("quad9", "Quad9 only"), ("own", "My own servers")):
            self.dns_choice.append(key, label)
        self.dns_choice.connect("changed", self.dns_choice_changed)
        self.dns_servers = Gtk.Entry(placeholder_text="e.g. 192.168.1.1, 9.9.9.9", no_show_all=True)
        dns_apply = Gtk.Button(label="Apply")
        dns_apply.connect("clicked", self.apply_dns)
        dns_row.pack_start(self.dns_choice, False, False, 0)
        dns_row.pack_start(self.dns_servers, True, True, 0)
        dns_row.pack_end(dns_apply, False, False, 0)
        box.pack_start(section(
            "DNS",
            dim("The firewall needs the real addresses of blocked sites, so Killer Chicken 9000 "
                "asks a DNS server for them. By default that is the server the computer already "
                "uses, so nobody new learns which sites are blocked."),
            dns_row), False, False, 0)

        self.strict_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.strict_switch.connect("state-set", self.toggle_strict)
        self.reset_guard_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.reset_guard_switch.connect("state-set", self.toggle_reset_guard)
        self.no_escape_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.no_escape_switch.connect("state-set", self.toggle_no_escape)
        box.pack_start(section(
            "Device protection",
            self.locked_switch_row(self.strict_switch, "strict", "Strict mode: take away sudo and pkexec during locks",
                                   "While a block or setting is locked or Frozen Chicken runs, no user "
                                   "can get administrator rights, so the blocks can't be undone from a "
                                   "terminal. Rights return when the lock ends. Strict mode itself can "
                                   "be turned off at any time unless you lock it."),
            self.strict_exempt_list(),
            self.switch_row(self.no_escape_switch, "No-escape: close the ways out through the boot menu",
                            "Sets a random boot menu password that nobody knows and hides the "
                            "recovery entries, so the emergency exit (kc9k.nofreeze) stops "
                            "working. Normal boot entries, Windows included, still start. The "
                            "only way past a lock is then booting a USB stick or reinstalling. "
                            "It can't be turned off during a lock."),
            self.switch_row(self.reset_guard_switch, "Factory reset protection: installer media guard",
                            "While anything is locked, the download pages of operating systems and USB "
                            "image writers are blocked and image writers (USB Image Writer, Etcher, "
                            "Disks, …) are closed. It can't be turned off during a lock.")), False, False, 0)

        # §4.10, R-MODE-5 (owner 2026-11-17): what shows when no mode or
        # circadian rhythm wallpaper does, and while the place isn't known.
        self.wallpaper_default = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        for key, label in WALLPAPER_DEFAULTS:
            self.wallpaper_default.append(key, label)
        self.wallpaper_default.connect("changed", self.wallpaper_default_changed)
        default_row = Gtk.Box(spacing=16)
        default_row.pack_start(Gtk.Label(label="Default to", xalign=0), True, True, 0)
        default_row.pack_end(self.wallpaper_default, False, False, 0)
        self.wallpaper_custom_row = Gtk.Box(spacing=10, no_show_all=True)
        self.wallpaper_custom_thumb = Gtk.Box()
        self.wallpaper_custom_name = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.MIDDLE, max_width_chars=30)
        choose = Gtk.Button(label="Choose a picture…", valign=Gtk.Align.CENTER)
        choose.connect("clicked", self.choose_wallpaper_default)
        for widget in (self.wallpaper_custom_thumb, self.wallpaper_custom_name):
            self.wallpaper_custom_row.pack_start(widget, False, False, 0)
        self.wallpaper_custom_row.pack_end(choose, False, False, 0)
        self.wallpaper_default_note = dim("")
        box.pack_start(section(
            "Wallpaper default",
            dim("What the desktop shows when no mode and no circadian rhythm wallpaper does, and while "
                "Killer Chicken can't tell where you are for a mode switched by places (no Wi-Fi, the "
                "first seconds after starting or waking)."),
            default_row, self.wallpaper_custom_row, self.wallpaper_default_note), False, False, 0)
        self.wallpaper_default_shown = None

        self.password_status = Gtk.Label(xalign=0, wrap=True)
        password_row = Gtk.Box(spacing=6)
        self.password_set_button = Gtk.Button(label="Set password…")
        self.password_set_button.connect("clicked", self.set_settings_password)
        self.password_remove_button = Gtk.Button(label="Remove password…")
        self.password_remove_button.connect("clicked", self.remove_settings_password)
        password_row.pack_start(self.password_set_button, False, False, 0)
        password_row.pack_start(self.password_remove_button, False, False, 0)
        self.password_locks = Gtk.CheckButton(label="Also needed to set a lock, so nobody else can lock me out")
        self.password_locks.connect("toggled", self.toggle_password_locks)

        self.access_status = Gtk.Label(xalign=0, wrap=True)
        access_row = Gtk.Box(spacing=6)
        access_set = Gtk.Button(label="Set a password…")
        access_set.connect("clicked", self.set_access_password)
        self.access_remove_button = Gtk.Button(label="Remove…", no_show_all=True)
        self.access_remove_button.connect("clicked", self.remove_access_password)
        access_row.pack_start(access_set, False, False, 0)
        access_row.pack_start(self.access_remove_button, False, False, 0)
        idle_row = Gtk.Box(spacing=6)
        idle_row.pack_start(Gtk.Label(label="Lock the pages again after", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
        idle = DurationField(self.get_application().prefs.get("access_idle_minutes", 0) * 60, minimum=60,
                             maximum=7 * 86400, zero="Never", width=9)

        def save_idle(field):
            if field.is_valid():
                prefs = self.get_application().prefs
                prefs["access_idle_minutes"] = field.get_value_as_int()
                save_prefs(prefs)

        idle.on_change(save_idle)
        idle_row.pack_start(idle, False, False, 0)
        idle_row.pack_start(Gtk.Label(label="without using this window", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
        self.idle_field = idle  # for tests
        top_align(idle_row)
        box.pack_start(section(
            "Passwords",
            Gtk.Label(label="Settings password", xalign=0, margin_top=6),
            dim("Guards Killer Chicken 9000 against anything running as you, and lets someone "
                "else decide when your blocks may be weakened: weakening, stopping, unlocking, "
                "turning browser settings off, strict mode, no-escape and uninstalling all ask "
                "for it. Starting a block and adding sites don't."),
            self.password_status, password_row, self.password_locks,
            Gtk.Label(label="Access passwords", xalign=0, margin_top=6),
            dim("Optional passwords to open Killer Chicken 9000 and each of its pages: "
                f"{', '.join(AREA_TITLES[a] for a in AREAS[1:-1])} and {AREA_TITLES[AREAS[-1]]}. Use the same one "
                "for several, or a different one for each. A locked page shows nothing until "
                "its password is given; the Lock button at the top locks them again. "
                "Changing or removing one asks for the one it has."),
            self.access_status, access_row, idle_row), False, False, 0)

        self.nepsis_label = Gtk.Label(xalign=0, wrap=True)
        nepsis_row = Gtk.Box(spacing=6)
        change = Gtk.Button(label="Change…")
        change.connect("clicked", self.change_global_nepsis)
        off = Gtk.Button(label="Turn off…")
        off.connect("clicked", self.nepsis_off)
        self.nepsis_off_button = off
        nepsis_row.pack_start(change, False, False, 0)
        nepsis_row.pack_start(off, False, False, 0)
        box.pack_start(section(
            "Bypass prevention",
            dim("When Killer Chicken stops many things in a short time (blocked pages, closed apps "
                "and windows, browsers without the extension, wrong access "
                "passwords), you are likely hunting for a loophole. It then takes the consequence you "
                "choose, so you really step away. Each block can have its own too."),
            self.nepsis_label, nepsis_row), False, False, 0)


        project_row = Gtk.Box(spacing=6)
        for label, handler in (("Back up…", self.export_project), ("Share…", self.share_project),
                               ("Import…", self.import_project)):
            button = Gtk.Button(label=label)
            button.connect("clicked", handler)
            project_row.pack_start(button, False, False, 0)
        box.pack_start(section(
            "Project",
            dim("Save every block, cluster and setting to one file. A backup also holds your "
                "places (the Wi-Fi networks they are known by), running locks, passwords (as hashes), "
                "hotkeys and statistics; a file to share leaves them out. Importing shows what it "
                "adds first, only adds, and gives a block whose name is taken a new name."),
            project_row), False, False, 0)

        self.option_switches = {}
        option_rows = []
        for name, title, detail in OPTIONS:
            switch = Gtk.Switch(valign=Gtk.Align.CENTER)
            switch.connect("state-set", self.toggle_option, name)
            self.option_switches[name] = switch
            option_rows.append(self.switch_row(switch, title, detail))
        box.pack_start(section("Behaviour settings", *option_rows), False, False, 0)
        return scroll

    def tamper_dialog(self, _button=None):
        """The tamper protection group in its own window: nothing changes until
        OK, and turning one on is confirmed by OK (SPEC §4.10)."""
        current = self.tamper_state()
        dialog = Gtk.Dialog(title="Tamper protection", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.CANCEL)
        box = dialog.get_content_area()
        box.set_spacing(12)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(16)
        box.pack_start(dim("Once on, a tamper protection setting can't be turned off while any "
                           "block or setting is locked. Turning one off asks for the settings "
                           "password."), False, False, 0)
        has_supported = any(b.get("installed") for b in self.state.get("browsers") or [])
        checks = {}
        for key, title, detail, _command in TAMPER_SETTINGS:
            check = Gtk.CheckButton(label=title)
            check.set_active(current[key])
            note = detail
            if key == "unsupported-browsers" and self.setting_locked(key):
                check.set_sensitive(False)
                note += " It has its own lock now."
            elif key == "unsupported-browsers" and not current[key] and not has_supported:
                check.set_sensitive(False)
                note += " Install a supported browser first (Browsers, below)."
            elif current[key] and self.state.get("anything_locked"):
                # §4.10: stuck on while anything is locked.
                check.set_sensitive(False)
                note += " Something is locked, so it stays on."
            checks[key] = check
            item = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
            item.pack_start(check, False, False, 0)
            text = dim(note)
            text.set_margin_start(26)
            item.pack_start(text, False, False, 0)
            box.pack_start(item, False, False, 0)
        self.tamper_checks = checks  # for tests
        grace_row = Gtk.Box(spacing=8)
        grace_row.pack_start(Gtk.Label(label="Close browsers without the extension after",
                                       valign=Gtk.Align.START, margin_top=6), False, False, 0)
        grace = DurationField(self.state.get("tamper_grace", 60), minimum=10, maximum=600, bare=1, width=8)
        grace_row.pack_start(grace, False, False, 0)
        grace_row.pack_start(Gtk.Label(label="(longer weakens)", valign=Gtk.Align.START, margin_top=6),
                             False, False, 0)
        top_align(grace_row)
        ok_button = dialog.get_widget_for_response(Gtk.ResponseType.OK)
        if ok_button is not None:
            guard_button(ok_button, grace)
        box.pack_start(grace_row, False, False, 0)
        self.tamper_grace = grace  # for tests
        dialog.show_all()
        answer = dialog.run()
        wanted = {key: check.get_active() for key, check in checks.items()}
        new_grace = grace.get_value_as_int()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        if new_grace != self.state.get("tamper_grace", 60):
            self.run(["tamper", "grace", str(new_grace)], then=lambda _o: self.apply_tamper(current, wanted))
            return
        self.apply_tamper(current, wanted)

    def apply_tamper(self, current, wanted):
        """Runs one command per changed tamper protection setting, in turn."""
        commands = [list(command) + ["on" if wanted[key] else "off"]
                    for key, _title, _detail, command in TAMPER_SETTINGS
                    if wanted[key] != current[key]]

        def next_command(_output=None):
            if commands:
                self.run(commands.pop(0), then=next_command)

        next_command()

    def tamper_state(self):
        return {"clock": bool(self.state.get("tamper_clock", False)),
                "file-urls": bool(self.state.get("tamper_file_urls", False)),
                "task-managers": bool(self.state.get("tamper_task_managers", False)),
                "private": bool(self.state.get("tamper_private", False)),
                "unsupported-browsers": bool(self.state.get("unsupported_browsers", False))}

    def setting_lock_info(self, name):
        """A lockable setting's lock from the state: strict mode, block
        unsupported browsers or a browser setting."""
        if name == "strict":
            return self.state.get("strict_lock") or {}
        if name == "unsupported-browsers":
            return self.state.get("unsupported_browsers_lock") or {}
        return (self.state.get("browser_locks") or {}).get(name) or {}

    @staticmethod
    def setting_command(name, verb):
        """The command that turns a lockable setting on, or unlocks it."""
        if name == "strict":
            return ["strict", verb]
        if name == "unsupported-browsers":
            return ["tamper", "unlock", name] if verb == "unlock" else ["tamper", name, verb]
        return ["browser", "unlock", name] if verb == "unlock" else ["browser", name, verb]

    def setting_locked(self, name):
        return bool(describe_setting_lock(self.setting_lock_info(name)))

    def build_hotkeys_page(self):
        """§4.13 (owner 2026-11-17): hotkeys on their own page: what each
        press does (hotkey actions, §3.9), added, removed, saved as presets."""
        scroll, box = page_box()
        page_header(box, "Hotkeys", "Keys that do what you choose: start and lock blocks and modes, open your "
                    "sites, files and programs, close the browsers, lock the screen…",
                    help="A hotkey runs its actions in order when its keys are pressed. One that locks "
                         "something asks you to confirm at each press, unless you chose Run without "
                         "confirmation when adding it (its locks are then confirmed only then). One that "
                         "switches something off needs the settings password when it is added, and is "
                         "refused at the press while that thing is locked. The keys show in the desktop's "
                         "keyboard settings as KC9K: ….")
        self.hotkey_ids = []
        self.hotkeys = EditableList(
            "Hotkeys", "", None,
            lambda _value, number: self.run(["hotkey", "remove", str(self.hotkey_ids[number - 1])]),
            entry=False)
        for label, handler, tip in (("Add hotkey…", self.add_hotkey_actions, "Keys and the actions they run"),
                                    ("Start a block…", self.add_hotkey, "Keys that start a block, with locks"),
                                    ("Save as preset…", self.save_hotkey_preset, "The selected hotkey, to add again")):
            button = Gtk.Button(label=label, tooltip_text=tip)
            button.connect("clicked", lambda _b, h=handler: h(None))
            self.hotkeys.button_row.pack_start(button, False, False, 0)
        for position, child in enumerate([c for c in self.hotkeys.button_row.get_children()
                                          if isinstance(c, Gtk.Button) and c.get_label() in
                                          ("Add hotkey…", "Start a block…", "Save as preset…")]):
            self.hotkeys.button_row.reorder_child(child, position)
        box.pack_start(self.hotkeys, True, True, 0)
        return scroll

    def save_hotkey_preset(self, _button=None):
        chosen = self.hotkeys._selected()
        if not chosen:
            self.message("Pick a hotkey in the list first.", Gtk.MessageType.INFO)
            return
        number = self.hotkey_ids[chosen[-1][1] - 1]
        name = self.ask_text("Save as preset", "A name for a copy of this hotkey, to add again later.")
        if name:
            self.run(["preset", "save", "hotkey", str(number), name],
                     then=lambda output: self.message(output, Gtk.MessageType.INFO))

    HOTKEY_KINDS = (("start", "Start a block"), ("stop", "Switch a block off"), ("mode-on", "Switch a mode on"),
                    ("mode-off", "Switch a mode off"), ("mode-run", "Run a mode, locked, for"),
                    ("monk", "Monk mode for"), ("freeze", "Frozen Chicken for"),
                    ("close-browser", "Close the browsers"), ("lock-screen", "Lock the screen"),
                    ("logout", "Log out"), ("shutdown", "Shut down"), ("custom", "Custom action"))

    def ask_hotkey_action(self, answer=None):
        """One hotkey action (§3.9): its kind and what it needs. Returns
        (action dict, words, password secret or None) or None. answer: for
        tests."""
        if answer is not None:
            return answer
        blocks = sorted(n for n, b in self.blocks.items() if not b.get("draft_of"))
        modes = [m["name"] for m in self.state.get("modes") or [] if not m.get("monk") and not m.get("freeze")]
        dialog = Gtk.Dialog(title="Add an action", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Add", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        box.set_border_width(14)
        kind = Gtk.ComboBoxText()
        for key, label in self.HOTKEY_KINDS:
            kind.append(key, label)
        kind.set_active_id("start" if blocks else "custom")
        box.pack_start(kind, False, False, 0)
        target = Gtk.ComboBoxText()
        length = DurationField(3600, minimum=60, maximum=30 * 86400, zero="no time (once)", width=9)
        how = Gtk.ComboBoxText()
        for key, label in (("lock", "Lock the screen"), ("logout", "Log out"), ("shutdown", "Shut down"),
                           ("apps", "Allowed apps only")):
            how.append(key, label)
        how.set_active_id("lock")
        custom = CustomEditor(self)
        for widget in (target, length, how, custom):
            box.pack_start(widget, False, False, 0)

        def shown(_combo=None):
            k = kind.get_active_id()
            target.remove_all()
            names = blocks if k in ("start", "stop") else modes if k in ("mode-on", "mode-off", "mode-run") else []
            for n in names:
                target.append_text(n)
            if names:
                target.set_active(0)
            target.set_visible(bool(names) or k in ("start", "stop", "mode-on", "mode-off", "mode-run"))
            timed = k in ("mode-run", "monk", "freeze", "lock-screen", "logout", "shutdown")
            length.set_visible(timed)
            length.set_limits(60, 7 * 86400 if k in ("freeze", "lock-screen", "logout", "shutdown") else 30 * 86400)
            how.set_visible(k == "freeze")
            custom.set_visible(k == "custom")
        kind.connect("changed", shown)
        self.hotkey_action_parts = (dialog, kind, target, length, how, custom)  # for tests
        dialog.set_default_size(520, -1)
        dialog.show_all()
        shown()
        reply = dialog.run()
        k, t, minutes = kind.get_active_id(), target.get_active_text(), length.minutes if length.is_valid() else None
        freeze_how, items = how.get_active_id(), custom.items
        label = dict(self.HOTKEY_KINDS)[k]
        dialog.destroy()
        if reply != Gtk.ResponseType.OK:
            return None
        action = {"kind": k}
        secret = None
        words = label
        if k in ("start", "stop", "mode-on", "mode-off", "mode-run"):
            if not t:
                self.message("There is nothing to choose: make a block or a mode first.", Gtk.MessageType.INFO)
                return None
            action["target"] = t
            words = f"{label} {t}" if k != "mode-run" else f"Run {t}, locked,"
        if k in ("mode-run", "monk", "freeze", "lock-screen", "logout", "shutdown"):
            if minutes is None:
                self.message("How long: type a time such as 1h or 30m.", Gtk.MessageType.INFO)
                return None
            if minutes:
                action["minutes"] = minutes
                words += f" for {fmt_minutes(minutes)}" if not words.endswith(" for") else f" {fmt_minutes(minutes)}"
            elif k in ("mode-run", "monk", "freeze"):
                self.message("How long: at least a minute.", Gtk.MessageType.INFO)
                return None
        if k == "freeze":
            action["param"] = freeze_how
        if k == "custom":
            if not items:
                self.message("Add something to open or run first.", Gtk.MessageType.INFO)
                return None
            action["param"] = json.dumps([{i.split(" ", 1)[0]: i.split(" ", 1)[1]} for i in items])
            words = "; ".join(items)
        if k == "start":
            picker = LockPicker("stop", "stopped", LockPicker.ALL_KINDS)
            asked = self.lock_dialog(f"Start {t}", "Locks each press sets (none: it just starts it). On a block "
                                     "that is on, a press only adds or extends locks.", "OK", picker)
            if asked is None:
                return None
            chosen, relock_flags, secret = asked
            if len(chosen) > 1:
                chosen = self.choose_lock_order(chosen)
                if chosen is None:
                    return None
            flags = [f for flags_, _text in chosen for f in flags_] + relock_flags
            if flags:
                action["lock_args"] = flags
                words += " with " + ", then ".join(text for _f, text in chosen)
        return action, words, secret

    def add_hotkey_actions(self, _button=None, answer=None):
        """A hotkey with any actions in order (§3.9 Hotkey action, §4.13):
        its keys, its actions (Add action…, Remove), Run without
        confirmation. answer: (keys, [(action, words, secret)], no_confirm)
        for tests."""
        if answer is None:
            dialog = Gtk.Dialog(title="Add a hotkey", transient_for=self, modal=True)
            dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
            ok = dialog.add_button("Add", Gtk.ResponseType.OK)
            ok.get_style_context().add_class("suggested-action")
            box = dialog.get_content_area()
            box.set_spacing(10)
            box.set_border_width(14)
            keys = Gtk.Entry(placeholder_text="ctrl+alt+s", width_chars=18)
            note = dim("ctrl, alt, shift or super with a key; F1 to F24 also work alone.")
            box.pack_start(heading("Keys", "medium"), False, False, 0)
            box.pack_start(keys, False, False, 0)
            box.pack_start(note, False, False, 0)
            box.pack_start(heading("What a press does, in order", "medium"), False, False, 0)
            rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
            rows.get_style_context().add_class("frame")
            box.pack_start(rows, False, False, 0)
            actions = []

            def redraw():
                for row in rows.get_children():
                    rows.remove(row)
                for number, (_a, words, _s) in enumerate(actions):
                    line = Gtk.Box(spacing=8, margin=6)
                    line.pack_start(Gtk.Label(label=f"{number + 1}. {words}", xalign=0, wrap=True), True, True, 0)
                    remove = Gtk.Button(label="Remove", relief=Gtk.ReliefStyle.NONE)
                    remove.connect("clicked", lambda _b, n=number: (actions.pop(n), redraw(), check()))
                    line.pack_start(remove, False, False, 0)
                    rows.add(line)
                rows.show_all()

            add = Gtk.Button(label="Add action…", halign=Gtk.Align.START)

            def add_action(_b):
                got = self.ask_hotkey_action()
                if got:
                    actions.append(got)
                    redraw()
                    check()
            add.connect("clicked", add_action)
            box.pack_start(add, False, False, 0)
            unasked = Gtk.CheckButton(label="Run without confirmation (a press that locks then asks nothing)")
            box.pack_start(unasked, False, False, 0)

            def check(*_args):
                problem = hotkey_problem(keys.get_text())
                typed = bool(keys.get_text().strip())
                mark_problem(keys, problem if typed else None)
                note.set_text(problem if typed and problem else
                              "ctrl, alt, shift or super with a key; F1 to F24 also work alone.")
                ok.set_sensitive(not problem and bool(actions))
            keys.connect("changed", check)
            self.hotkey_dialog = (dialog, keys, actions, unasked, add_action)  # for tests
            dialog.set_default_size(560, -1)
            dialog.show_all()
            check()
            reply = dialog.run()
            typed, chosen, no_confirm = keys.get_text().strip(), list(actions), unasked.get_active()
            dialog.destroy()
            if reply != Gtk.ResponseType.OK:
                return
        else:
            typed, chosen, no_confirm = answer
        args = ["hotkey", "add", typed]
        secrets = [s for _a, _w, s in chosen if s]
        locking = []
        for action, words, _s in chosen:
            args += ["--do-json", json.dumps(action)]
            if action.get("lock_args") or action["kind"] in ("mode-run", "monk", "freeze") or \
                    (action["kind"] in ("lock-screen", "logout", "shutdown") and action.get("minutes")):
                locking.append(f"{words}, each time {typed} is pressed")
        if locking:
            # R-LOCK-12: the locks are confirmed now (and at each press, unless
            # Run without confirmation).
            self.lock_option = False
            if not self.confirm_lock(locking, option=None if no_confirm else
                                     "Run without confirmation (pressing the hotkey then locks without asking again)"):
                return
            args.append("--yes")
            if no_confirm or self.lock_option:
                args.append("--no-confirm")
        self.run(args, secrets[0] + "\n" if secrets else None)

    def add_hotkey(self, _button=None):
        """A new hotkey: its keys, the block it starts and its locks, in the
        lock picker every start uses (§3.13, R-GUI-2). The locks are
        confirmed now (R-LOCK-12); pressing asks again unless Run without
        confirmation is ticked."""
        blocks = sorted(name for name, b in self.blocks.items() if not b.get("draft_of"))
        if not blocks:
            self.message("Make a block first: a hotkey starts one.", Gtk.MessageType.INFO)
            return
        fields = HotkeyFields(blocks, self.selected)
        picker = LockPicker("stop", "stopped", LockPicker.ALL_KINDS)
        asked = self.lock_dialog(
            "Add a hotkey", "Pressing the keys starts the block with these locks (none: it just "
            "starts). On a block that is already on, a press only adds or extends locks.",
            "Add", picker, extra=fields)
        if asked is None:
            return
        keys, block = fields.values
        chosen, relock_flags, secret = asked
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        args = ["hotkey", "add", keys, block]
        locks = []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"block '{block}': {order}{text}, set each time {keys} is pressed")
        args += relock_flags
        if locks:
            if not self.confirm_lock(locks, option="Run without confirmation (pressing the hotkey then "
                                                    "locks without asking again)"):
                return
            args.append("--yes")
            if self.lock_option:
                args.append("--no-confirm")
        self.run(args, secret + "\n" if secret else None)

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
                        "Weakening and locks won't ask for it any more.", "Remove"):
            self.run(["password", "remove"], then=lambda _o: setattr(self, "settings_password", None))

    def update_wallpaper_default(self):
        w = self.state.get("wallpaper") or {}
        key = json.dumps(w, sort_keys=True)
        if key == self.wallpaper_default_shown:
            return
        self.wallpaper_default_shown = key
        was, self.updating = self.updating, True
        self.wallpaper_default.set_active_id(w.get("default") or "latest")
        self.updating = was
        custom = w.get("default") == "custom" and w.get("custom_path")
        for child in self.wallpaper_custom_thumb.get_children():
            self.wallpaper_custom_thumb.remove(child)
        if custom:
            self.wallpaper_custom_thumb.add(picture_thumb(w["custom_path"]))
        self.wallpaper_custom_name.set_text(w.get("custom_name") or "")
        self.wallpaper_custom_row.set_visible(bool(custom))
        if custom:
            self.wallpaper_custom_row.show_all()
        self.wallpaper_default_note.set_text({
            "latest": "The wallpaper stays as it is. At a new place without Wi-Fi that can be the last "
                      "place's wallpaper until Killer Chicken knows where you are.",
            "circadian": "Circadian rhythm's wallpaper for the time of day, even with circadian rhythm "
                         "switched off.",
            "custom": "This picture shows until Killer Chicken knows where you are. It also goes on before "
                      "the computer sleeps, so it wakes with it, not with the last place's wallpaper.",
        }.get(w.get("default") or "latest", ""))

    def wallpaper_default_changed(self, combo):
        if self.updating:
            return
        key = combo.get_active_id()
        if key != "custom":
            self.run(["wallpaper", "default", key])
        elif (self.state.get("wallpaper") or {}).get("custom_path"):
            self.run(["wallpaper", "default", "custom"])
        else:
            self.choose_wallpaper_default(None)

    def choose_wallpaper_default(self, _button, path=None):
        """Custom: the picture shown by default. path: for tests."""
        path = path or choose_picture(self, "Choose the default wallpaper")
        self.wallpaper_default_shown = None  # cancelled: the box shows the setting again
        if path:
            self.run(["wallpaper", "default", "custom", path])
        else:
            self.update_wallpaper_default()

    def update_settings(self):
        self.update_wallpaper_default()
        lock = self.state.get("nepsis_lock", {})
        text = describe_nepsis(self.state.get("nepsis"), self.state.get("nepsis_count"))
        if lock.get("lock_seconds"):
            text += f" Locked for {fmt_seconds(lock['lock_seconds'])}."
        self.nepsis_label.set_text(text)
        self.nepsis_off_button.set_visible(bool((self.state.get("nepsis") or {}).get("steps")))
        for name, switch in self.browser_switches.items():
            set_switch(switch, self.state["browser"].get(name, False))
        notices = self.state.get("notifications") or {}
        off = set(notices.get("off") or [])
        was, self.updating = self.updating, True
        try:
            for key, check in self.notice_checks.items():
                check.set_active(key not in off)
        finally:
            self.updating = was
        if not self.notice_advance.has_focus():
            self.notice_advance.set_text(",".join(str(m) for m in notices.get("advance") or [5]))
        for name, (label, unlock) in self.setting_locks.items():
            info = self.setting_lock_info(name)
            parts = describe_setting_lock(info)
            text = "Locked: " + ", ".join(parts) if parts else ""
            if info.get("relock_in"):
                # Unlocked with re-lock: on again by itself unless turned off.
                text = (text + "; " if text else "") + \
                    f"unlocked, locks on again in {fmt_seconds(info['relock_in'])}"
            label.set_text(text)
            label.set_visible(bool(text))
            unlock.set_visible(info.get("char_locked", False) or info.get("password_locked", False))
            order = info.get("lock_order") or []
            # R-LOCK-6: only the first lock still on can be unlocked.
            unlock.set_sensitive(bool(order) and order[0] in ("text", "password"))
            # A locked setting is on and stays on, so its switch can't be used.
            if name in self.browser_switches:
                self.browser_switches[name].set_sensitive(not parts)
        set_switch(self.strict_switch, self.state["strict"])
        set_switch(self.strict_use_switch, self.state.get("strict_use", False))
        self.strict_exempt.set_items(self.state.get("strict_exempt") or [])
        self.strict_switch.set_sensitive(not self.setting_locked("strict"))
        set_switch(self.no_escape_switch, self.state["no_escape"])
        set_switch(self.reset_guard_switch, self.state.get("reset_guard", False))
        tamper = self.tamper_state()
        on = [title for key, title, _detail, _command in TAMPER_SETTINGS if tamper[key]]
        self.tamper_summary.set_markup(
            "<b>On:</b> " + GLib.markup_escape_text("; ".join(on)) if on else "<b>All off.</b>")
        self.update_browsers()
        for name, switch in self.option_switches.items():
            set_switch(switch, (self.state.get("options") or {}).get(name, False))
        has_password = self.state.get("settings_password", False)
        self.password_status.set_markup("<b>On.</b>" if has_password else "Not set.")
        self.password_set_button.set_label("Change password…" if has_password else "Set password…")
        self.password_remove_button.set_visible(has_password)
        self.password_locks.set_visible(has_password)
        self.updating, was = True, self.updating
        self.password_locks.set_active(self.state.get("password_for_locks", False))
        self.updating = was
        set_switch(self.no_weakening_switch, self.state.get("no_weakening", False))
        dns = self.state.get("dns") or "network"
        if dns in ("network", "quad9"):
            self.dns_choice.set_active_id(dns)
        else:
            self.dns_choice.set_active_id("own")
            self.dns_servers.set_text(dns)

    def update_hotkeys_page(self):
        hotkeys = self.state.get("hotkeys") or []
        self.hotkey_ids = [h["id"] for h in hotkeys]
        self.hotkeys.set_items([
            f"{hotkey_words(h['keys'])}  {h.get('does') or ''}" + (" (runs without confirmation)" if h.get("no_confirm") else "")
            for h in hotkeys])

    def update_browsers(self):
        """The Browsers list: each supported browser, installed or not, its
        extension's last check-in, and Install."""
        browsers = self.state.get("browsers") or []
        for b in browsers:
            if b["id"] not in self.browser_rows:
                row = Gtk.Box(spacing=12)
                name = Gtk.Label(xalign=0)
                name.set_markup(f"<b>{GLib.markup_escape_text(b['name'])}</b>")
                name.set_size_request(140, -1)
                status = dim("")
                install = Gtk.Button(label="Install", valign=Gtk.Align.CENTER)
                install.set_no_show_all(True)
                install.connect("clicked", self.install_browser, b["id"], b["name"])
                row.pack_start(name, False, False, 0)
                row.pack_start(status, True, True, 0)
                row.pack_end(install, False, False, 0)
                self.browser_list.pack_start(row, False, False, 0)
                row.show_all()
                self.browser_rows[b["id"]] = (status, install)
            status, install = self.browser_rows[b["id"]]
            status.set_text(browser_status(b))
            install.set_visible(not b.get("installed"))
        any_installed = any(b.get("installed") for b in browsers)
        if any_installed:
            self.extension_status.set_text("Extension: " + self.state.get("extension_status", ""))
        else:
            self.extension_status.set_text(
                "No supported browser is installed, so the extension can't run: wildcards, rendered terms, "
                "greyscale, images and web-only aren't enforced in the browser, and no browser is "
                "closed. Install one above.")

    def install_browser(self, _button, browser_id, name):
        if self.confirm(f"Install {name}?",
                        BROWSER_INSTALL_NOTES.get(browser_id, "") +
                        " This needs an internet connection and can take a few minutes.",
                        "Install"):
            self.browser_rows[browser_id][0].set_text("Installing, this can take a few minutes…")
            self.run(["browsers", "install", browser_id], timeout=900,
                     then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def setting_row(self, name, title, detail):
        """A browser setting: its switch, plus its lock state and buttons."""
        switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        switch.connect("state-set", self.toggle_browser, name)
        self.browser_switches[name] = switch
        return self.locked_switch_row(switch, name, title, detail)

    def locked_switch_row(self, switch, name, title, detail):
        """A setting's row with its switch, Lock… and Unlock… beside it and
        the lock's state under its description (browser settings, strict
        mode)."""
        row = self.switch_row(switch, title, detail)
        status, lock, unlock = self.setting_lock_parts(name, title)
        row.get_children()[0].pack_start(status, False, False, 0)
        for button in (unlock, lock):
            button.set_valign(Gtk.Align.CENTER)
            row.pack_end(button, False, False, 0)
        return row

    def setting_lock_parts(self, name, title):
        """A setting lock's state, Lock… and Unlock…"""
        status = dim("", wrap=False)
        status.set_no_show_all(True)
        lock = Gtk.Button(label="Lock…")
        lock.connect("clicked", self.lock_setting, name, title)
        unlock = Gtk.Button(label="Unlock…")
        unlock.set_no_show_all(True)
        unlock.connect("clicked", self.unlock_setting, name, title)
        self.setting_locks[name] = (status, unlock)
        return status, lock, unlock

    def setting_lock_line(self, name, title):
        """A setting lock's state with Lock… and Unlock… on a line of their
        own (block unsupported browsers, whose switch is in the tamper
        protection window)."""
        line = Gtk.Box(spacing=8)
        status, lock, unlock = self.setting_lock_parts(name, title)
        line.pack_start(status, False, False, 0)
        line.pack_end(unlock, False, False, 0)
        line.pack_end(lock, False, False, 0)
        return line

    def lock_setting(self, _button, name, title):
        picker = LockPicker("turn it off", "turned off",
                            ("timer", "password", "text", "restart", "range", "relock"))
        asked = self.lock_dialog(
            f"Lock: {title}",
            "The setting is turned on and can't be turned off until the lock ends. Locks can be "
            "added later, but only made stronger.", "Turn on and lock", picker, need_lock=True)
        if asked is None:
            return
        chosen, relock_flags, secret = asked
        if not chosen:
            self.message("Choose at least one lock.", Gtk.MessageType.INFO)
            return
        # R-LOCK-6: the locks unlock one at a time in the order of their flags.
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        args = self.setting_command(name, "on")
        locks = []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"{title}: {order}{text}")
        args += relock_flags
        if not self.confirm_lock(locks):
            return
        self.run(args + ["--yes"], secret + "\n" if secret else None)

    def unlock_setting(self, _button, name, title):
        info, unlock = self.setting_lock_info(name), self.setting_command(name, "unlock")
        order = info.get("lock_order") or []
        if (order[:1] == ["password"]) or (info.get("password_locked") and not info.get("char_locked")):
            password = self.ask_text(f"Unlock {title}", "Enter the setting's password.", password=True)
            if password:
                self.run(unlock, password + "\n")
            return
        command = " ".join([CLI] + [shlex.quote(a) for a in unlock])
        try:
            subprocess.Popen(["x-terminal-emulator", "-e", command])
        except OSError as error:
            self.message(f"Can't open a terminal for the typing challenge: {error}\n\nRun: {command}")

    def strict_exempt_list(self):
        """§3.8: folders programs may still start from during strict mode's
        lockdown. Adding one weakens strict mode."""
        self.strict_exempt = EditableList(
            "", "/home/me/bin", lambda dirs: [self.run(["strict", "exempt", "add", d]) for d in dirs],
            lambda d, *_: self.run(["strict", "exempt", "remove", d]),
            help_text="Exempt folders: programs in them still start while strict mode holds "
                      "(adding one weakens strict mode).",
            split=lambda text: [text.strip()] if text.strip() else [], multi=True)
        return self.strict_exempt

    def toggle_strict_use(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["use-counting", "strict" if wanted else "normal"])
        return True

    def toggle_notice(self, check, key):
        if self.updating:
            return
        self.run(["notifications", key, "on" if check.get_active() else "off"])

    def apply_notice_advance(self, _button):
        self.run(["notifications", "advance", self.notice_advance.get_text().replace(" ", "")])

    def toggle_browser(self, _switch, wanted, name):
        if self.updating:
            return False
        self.run(["browser", name, "on" if wanted else "off"])
        return True

    def toggle_option(self, _switch, wanted, name):
        if self.updating:
            return False
        self.run(["behaviour", name, "on" if wanted else "off"])
        return True

    def update_social(self, block):
        """Builds the social media boxes once, from the state's list, and ticks
        the block's."""
        if not self.social_checks:
            site = None
            for preset in self.state.get("social_presets") or []:
                if preset["site"] != site:
                    site = preset["site"]
                    self.social_box.pack_start(heading(site, "medium"), False, False, 0)
                check = Gtk.CheckButton(label=preset["title"])
                check.connect("toggled", self.toggle_social, preset["name"])
                self.social_checks[preset["name"]] = check
                self.social_box.pack_start(check, False, False, 0)
            self.social_box.show_all()
        on = block.get("social") or []
        for name, check in self.social_checks.items():
            check.set_active(name in on)
        count = sum(1 for name in self.social_checks if name in on)
        self.social_expander.set_label("<b>Social media</b>" + (
            f"  <small>{count} {'setting' if count == 1 else 'settings'} on</small>" if count else ""))

    def save_place(self, _button, target=None, flags=()):
        """§3.6: saves the Wi-Fi networks in range as a place and, ticked by
        default, adds it to the automation of target (a mode's name) or of
        the selected block."""
        dialog = Gtk.Dialog(title="Save this place", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Save", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(10)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(18)
        box.pack_start(dim("A name for where the computer is now, such as home or library. Saving under a "
                           "name that exists adds the networks seen now."), False, False, 0)
        entry = Gtk.Entry(activates_default=True)
        box.pack_start(entry, False, False, 0)
        rule = None
        target = target or self.selected
        if target:
            rule = Gtk.CheckButton(label=f"Add it to '{target}' (being here switches it)", active=True)
            box.pack_start(rule, False, False, 0)
        ok.set_sensitive(False)
        entry.connect("changed", lambda e: ok.set_sensitive(bool(e.get_text().strip())))
        self.place_dialog = (dialog, entry, rule)  # for tests
        dialog.show_all()
        answer = dialog.run()
        name, with_rule = entry.get_text().strip(), bool(rule and rule.get_active())
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or not name:
            return
        block = target
        self.place_to_show = name

        def saved(output):
            if with_rule and block:
                self.run(["place", "rule", block, name, "on", *flags],
                         then=lambda rule_output: self.message(output + "\n" + rule_output, Gtk.MessageType.INFO))
            else:
                self.message(output, Gtk.MessageType.INFO)

        self.run(["place", "add", name], timeout=60, then=saved)

    def pick_apps(self, _button):
        if not self.selected:
            return
        dialog = AppPicker(self)
        answer = dialog.run()
        chosen = dialog.chosen()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and chosen:
            self.block_cmd("add-app", chosen)

    def pick_app_folder(self, _button):
        path = self.choose_file("Block every program in a folder", Gtk.FileChooserAction.SELECT_FOLDER, "Add")
        if path and self.selected:
            self.block_cmd("add-app", [path.rstrip("/") + "/"])

    def pick_app_file(self, _button):
        path = self.choose_file("Block a program file (and copies of it)", Gtk.FileChooserAction.OPEN, "Add")
        if path and self.selected:
            self.block_cmd("add-app", [path])

    def toggle_social(self, check, name):
        if self.updating or not self.selected:
            return
        self.run(["social", self.selected, name, "on" if check.get_active() else "off"])

    def toggle_keyword_test(self, _switch, wanted):
        if self.updating or not self.selected:
            return False
        self.run(["list", "test", self.selected, "on" if wanted else "off"])
        return True

    def toggle_stats(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["stats", "on" if wanted else "off"], then=lambda _o: self.refresh_stats())
        return True

    def toggle_stats_private(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["stats", "private", "skip" if wanted else "count"])
        return True

    def clear_stats(self, _button):
        if self.confirm("Delete all statistics?", "Every recorded day is removed. This can't be undone.",
                        "Delete", destructive=True):
            self.run(["stats", "clear"], then=lambda _o: self.refresh_stats())


class Tray:
    """§4.10 tray: an icon with a menu (open, what is on, quit), through
    Ayatana AppIndicator or XApp's status icon, whichever the desktop has."""

    def __init__(self, app):
        self.app = app
        self.menu = Gtk.Menu()
        self.status = Gtk.MenuItem(label="Killer Chicken 9000", sensitive=False)
        open_item = Gtk.MenuItem(label="Open Killer Chicken 9000")
        open_item.connect("activate", lambda _i: app.activate())
        quit_item = Gtk.MenuItem(label="Quit the window")
        quit_item.connect("activate", lambda _i: app.quit())
        for item in (self.status, Gtk.SeparatorMenuItem(), open_item, quit_item):
            self.menu.append(item)
        self.menu.show_all()
        self.icon = None
        try:
            gi.require_version("AyatanaAppIndicator3", "0.1")
            from gi.repository import AyatanaAppIndicator3 as Indicator
            self.icon = Indicator.Indicator.new("kc9k", "security-high",
                                                Indicator.IndicatorCategory.APPLICATION_STATUS)
            self.icon.set_status(Indicator.IndicatorStatus.ACTIVE)
            self.icon.set_menu(self.menu)
            self.kind = "indicator"
        except (ImportError, ValueError):
            try:
                gi.require_version("XApp", "1.0")
                from gi.repository import XApp
                self.icon = XApp.StatusIcon()
                self.icon.set_icon_name("security-high")
                self.icon.set_primary_menu(self.menu)
                self.icon.set_secondary_menu(self.menu)
                self.kind = "xapp"
            except (ImportError, ValueError):
                self.kind = None

    def set_status(self, text):
        self.status.set_label(text)
        if self.kind == "xapp":
            self.icon.set_tooltip_text(text)
        elif self.kind == "indicator":
            self.icon.set_title(text)

    def remove(self):
        if self.kind == "indicator":
            from gi.repository import AyatanaAppIndicator3 as Indicator
            self.icon.set_status(Indicator.IndicatorStatus.PASSIVE)
        elif self.kind == "xapp":
            self.icon.set_visible(False)
        self.icon = None


# The interface's own look, on top of the GTK theme (R-GUI-3): notes under
# fields, red for what is wrong, small rounded preset buttons.
APP_CSS = b"""
.kc9k-note { font-size: 0.9em; opacity: 0.8; }
label.error, .kc9k-note.error { color: @error_color; opacity: 1; }
entry.error { border-color: @error_color; }
button.kc9k-chip { padding: 2px 10px; min-height: 0; border-radius: 999px; font-size: 0.9em; }
.kc9k-field-label { font-weight: bold; }

/* Pages: a title, a one-line subtitle, then cards. */
.kc9k-page-title { font-size: 1.7em; font-weight: 800; }
.kc9k-page-subtitle { opacity: 0.7; }
.kc9k-section-title { font-weight: bold; font-size: 1.05em; }
.kc9k-card {
  background-color: @theme_base_color;
  border: 1px solid alpha(@theme_fg_color, 0.13);
  border-radius: 12px;
  padding: 14px 16px;
}
.kc9k-card.kc9k-tight { padding: 4px 0; }
.kc9k-card.kc9k-hero { padding: 18px 20px; }
.kc9k-card.kc9k-clickable:hover { border-color: alpha(@theme_selected_bg_color, 0.7); }

/* The block list (SPEC 4.2): one card per block, its parts as links. */
button.kc9k-link { padding: 2px 6px; min-height: 0; border-radius: 6px; font-weight: 600;
  color: @theme_selected_bg_color; }
button.kc9k-link:hover { background-color: alpha(@theme_selected_bg_color, 0.10); }
button.kc9k-card-name { padding: 0 4px; min-height: 0; font-size: 1.3em; font-weight: 800; }
label.kc9k-card-name { font-size: 1.3em; font-weight: 800; }
.kc9k-card.kc9k-block-off { background-color: alpha(@theme_base_color, 0.6); }

/* Status pills: what is on, what is locked. */
.kc9k-pill { border-radius: 999px; padding: 2px 10px; font-weight: bold; font-size: 0.85em; }
.kc9k-pill.on { background-color: alpha(#2ec27e, 0.18); color: #1a7f4f; }
.kc9k-pill.locked { background-color: alpha(#e01b24, 0.15); color: #c01c28; }
.kc9k-pill.lifted { background-color: alpha(#3584e4, 0.16); color: #1c63b7; }
.kc9k-pill.off { background-color: alpha(@theme_fg_color, 0.09); color: alpha(@theme_fg_color, 0.75); }
.kc9k-pill.warn { background-color: alpha(#e5a50a, 0.2); color: #9c6e03; }

/* Big numbers on the overview. */
.kc9k-stat-value { font-size: 1.9em; font-weight: 800; }
.kc9k-stat-label { opacity: 0.7; font-size: 0.9em; }
.kc9k-hero-title { font-size: 1.35em; font-weight: bold; }

/* The sidebar. */
.kc9k-sidebar { background-color: alpha(@theme_fg_color, 0.035); }
.kc9k-sidebar row { padding: 9px 14px; border-radius: 8px; margin: 2px 8px; }
.kc9k-sidebar row:selected { background-color: alpha(@theme_selected_bg_color, 0.95); }
.kc9k-brand { font-weight: 800; font-size: 1.05em; }
frame.kc9k-listcard > border { border-radius: 12px; border: 1px solid alpha(@theme_fg_color, 0.13);
  background-color: @theme_base_color; }
button.kc9k-help { padding: 0 6px; min-height: 0; min-width: 0; border-radius: 999px; }

/* The grip under a list: drag to make it taller. */
.kc9k-grip { min-height: 10px; margin-top: -4px; }
.kc9k-grip-line { min-width: 36px; min-height: 3px; border-radius: 2px;
  background-color: alpha(@theme_fg_color, 0.25); }
.kc9k-grip:hover .kc9k-grip-line { background-color: alpha(@theme_selected_bg_color, 0.9); }
"""


class App(Gtk.Application):
    def __init__(self):
        super().__init__(application_id="io.github.kc9k.Gui")
        self.prefs = {}
        self.theme = None
        self.tray = None

    def set_tray(self, on):
        """§4.10: with the tray on, closing the window keeps the app (and
        its icon) running."""
        if on and self.tray is None:
            self.tray = Tray(self)
            self.hold()
        elif not on and self.tray is not None:
            self.tray.remove()
            self.tray = None
            self.release()

    def do_startup(self):
        Gtk.Application.do_startup(self)
        self.prefs = load_prefs()
        global FIRST_DAY
        FIRST_DAY = 6 if self.prefs.get("first_day") == "sunday" else 0
        global CLOCK_24
        CLOCK_24 = self.prefs.get("clock_24", True)
        self.theme = Theme()
        self.theme.apply(self.prefs.get("theme", "system"), self.prefs.get("darkness"))
        screen = Gdk.Screen.get_default()
        if screen is not None:
            self.css = Gtk.CssProvider()
            self.css.load_from_data(APP_CSS)
            Gtk.StyleContext.add_provider_for_screen(screen, self.css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
        if self.prefs.get("tray"):
            self.set_tray(True)

    def do_activate(self):
        window = self.get_active_window() or Window(self)
        window.show_all()
        window.present()


if __name__ == "__main__":
    App().run()
