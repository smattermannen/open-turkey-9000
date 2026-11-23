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
import subprocess
import threading
import weakref

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
gi.require_version("PangoCairo", "1.0")
from gi.repository import Gdk, Gio, GLib, Gtk, Pango, PangoCairo  # noqa: E402


# The code is split into parts (gui/kc9k_gui/, installed in
# /usr/local/lib/kc9k-gui/), run in order into this one namespace: every
# name is a global of this module, as before the split (tests replace CLI,
# PREFS_PATH and others here).
PARTS = ("base", "fields", "lists", "charts", "circadian", "automation",
         "win_places", "win_dashboard", "win_blocks", "win_modes", "win_block_actions",
         "win_clusters", "win_device", "win_stats", "win_settings", "win_account", "window", "app")


def _parts_dir():
    here = os.path.dirname(os.path.realpath(__file__))
    for d in (os.path.join(here, "kc9k_gui"), os.path.join(here, os.pardir, "lib", "kc9k-gui")):
        if os.path.isfile(os.path.join(d, "window.py")):
            return os.path.normpath(d)
    raise SystemExit("kc9k-gui: its parts are missing (kc9k_gui/ or ../lib/kc9k-gui/)")


for _part in PARTS:
    _path = os.path.join(_parts_dir(), _part + ".py")
    with open(_path, encoding="utf-8") as _f:
        exec(compile(_f.read(), _path, "exec"), globals())
del _part, _path, _f


if __name__ == "__main__":
    App().run()
