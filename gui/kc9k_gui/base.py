# Constants, CLI runners, preferences, theme, card and pill helpers.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

# A15 (R-GUI-3): Enter presses a dialog's OK. Every dialog is run through
# Gtk.Dialog.run, so the default response and the entries' activates-default
# are set there, once, for all of them.
_OK_RESPONSES = (Gtk.ResponseType.OK, Gtk.ResponseType.ACCEPT, Gtk.ResponseType.YES, Gtk.ResponseType.APPLY)


def _entries(widget):
    if isinstance(widget, Gtk.Entry):
        yield widget
    if isinstance(widget, Gtk.Container):
        for child in widget.get_children():
            yield from _entries(child)


def enter_presses_ok(dialog):
    """Makes the dialog's OK (or Accept/Yes/Apply) its default and lets
    Enter in any of its entries press it."""
    for response in _OK_RESPONSES:
        button = dialog.get_widget_for_response(response)
        if button is not None:
            button.set_can_default(True)
            dialog.set_default_response(response)
            entries = list(_entries(dialog.get_content_area()))
            for entry in entries:
                entry.set_activates_default(True)
            if not entries:
                # Enter presses the focused button: with nothing to type,
                # that is OK (GTK would focus the first button, Cancel/No).
                button.grab_focus()
            return response
    return None


_dialog_run = Gtk.Dialog.run


def _run_with_enter(dialog):
    enter_presses_ok(dialog)
    return _dialog_run(dialog)


Gtk.Dialog.run = _run_with_enter

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
    # Breaks & allowance and Bypass prevention (2026-11-21): kept in the
    # draft by the CLI and run at Save edits; "bypass" names the block by
    # --block (draft_position).
    "allowance": {"set": 2, "remove": 2},
    "pomodoro": {"set": 2, "off": 2},
    "break": {"reward": 2, "text": 2, "delay": 2},
    "bypass": {"set": "--block", "off": "--block"},
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
    where = where.get(args[1]) if len(args) > 1 else None
    if isinstance(where, str):  # the block comes after this flag
        return args.index(where) + 1 if where in args else None
    return where


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
    ("safesearch-google", "SafeSearch on Google", "Also keeps Brave, Yahoo and SearXNG searches strict and blocks "
     "search engines that can't be kept strict (so do Bing's and DuckDuckGo's)."),
    ("safesearch-bing", "SafeSearch on Bing", ""),
    ("safesearch-duckduckgo", "SafeSearch on DuckDuckGo", ""),
    ("safesearch-youtube", "YouTube Restricted Mode", ""),
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
    ("guest", "Close guest profiles",
     "Guest browsing is turned off in every Chrome-like browser (Chrome and Chromium have it off "
     "always) and the login screen offers no guest session.", ("tamper", "guest")),
    ("extension-stop", "Close browsers if the extension stops",
     "A browser whose Killer Chicken extension stops checking in is closed while a block needs "
     "it. Off, only while something is locked.", ("tamper", "extension-stop")),
    ("installer", "Block the installer",
     "While anything is locked, reinstalling Killer Chicken 9000 or installing an older version "
     "is refused, by the package manager and install.sh. Removing it always is.",
     ("tamper", "installer")),
    ("inactive-tabs", "Block inactive tabs",
     "Tabs in the background are blocked at once. Off, a background tab is blocked when you "
     "switch to it.", ("tamper", "inactive-tabs")),
    ("embedded", "Block embedded content",
     "A blocked site shown inside another page (a video, a frame) gets the block page. Off, "
     "the extension leaves embedded pages alone; whole blocked domains stay blocked.",
     ("tamper", "embedded")),
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

# What a found browser waits for before it gets the extension (2.12,
# gui-state 'waiting'); until then it is closed like any unsupported one.
BROWSER_WAITING = {
    "chrome-web-store": "it gets the extension once the Chrome Web Store listing exists.",
    "mozilla-signature": "it gets the extension once Mozilla has signed it. LibreWolf runs it now.",
}

# Install notes for the supported browsers (SPEC §3.13), by ID.
BROWSER_INSTALL_NOTES = {
    "librewolf": "It is installed from LibreWolf's official repository through extrepo.",
    "chromium": "It is installed from Linux Mint's own repository.",
    "firefox": "It is installed from Linux Mint's own repository.",
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
    waiting = browser.get("waiting")
    if waiting:
        why = BROWSER_WAITING.get(waiting, "it waits for " + waiting + ".")
        if browser.get("found"):
            return "Installed, no extension yet: " + why
        return "Not installed. " + why[0].upper() + why[1:]
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
CHALLENGE_TIMEOUT = 6 * 3600  # seconds a challenge window may stay open (5000 characters: about an hour)

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
    locks = []  # (kind, pill)
    if block.get("lock_seconds"):
        locks.append(("timer", (f"Locked {fmt_seconds(block['lock_seconds'])}", "locked")))
    if block.get("char_locked"):
        locks.append(("text", ("Text lock", "locked")))
    if block.get("password_locked"):
        locks.append(("password", ("Password lock", "locked")))
    if block.get("restart_locked"):
        locks.append(("restart", ("Until restart", "locked")))
    if block.get("delay_left"):
        locks.append(("delay", (f"Unlocks in {fmt_seconds(block['delay_left'])}", "warn")))
    elif block.get("delay_seconds"):
        locks.append(("delay", (f"Delay {fmt_seconds(block['delay_seconds'])}", "locked")))
    if block.get("timer_after"):
        locks.append(("timer", (f"Then locked {fmt_seconds(block['timer_after'])}", "locked")))
    if block.get("range_lock"):
        locks.append(("range", (f"Range {block['range_lock']}", "locked" if block.get("range_on") else "off")))
    # R-LOCK-6: in the order they are unlocked; locks not in it (off now) last.
    order = block.get("lock_order") or []
    locks.sort(key=lambda kp: order.index(kp[0]) if kp[0] in order else len(order))
    out += [p for _k, p in locks]
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
                   "restart": "restart lock", "delay": "delay lock", "range": "range lock"}


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


CARD_LOCK_WORDS = {"text": "text lock", "password": "password", "restart": "until restart",
                   "range": "range lock", "delay": "delay lock", "timer": "timer lock"}


def lock_words(block):
    """A block's lock state, in short (§4.2 block list): the lock unlocked
    next (R-LOCK-6), the first of its unlock order."""
    if not block.get("held"):
        return "Unlocked"
    first = (block.get("lock_order") or [None])[0]
    if block.get("lock_seconds") and first in (None, "timer"):
        return f"Locked, {fmt_seconds(block['lock_seconds'])} left"
    if first in CARD_LOCK_WORDS:
        return f"Locked: {CARD_LOCK_WORDS[first]}"
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
