# Tray, the interface's CSS, the application.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

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
