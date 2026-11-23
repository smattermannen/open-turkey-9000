"""A15 (R-GUI-3): Enter presses a dialog's OK in the GTK GUI. Loads
gui/kc9k-gui.py as a module (no window, no kc9k) and runs a dialog with an
entry through Gtk.Dialog.run, as every dialog of the GUI is run: Enter in the
entry must end it with OK. Needs an X display (Xvfb will do).
    python3 ci/gui_enter_smoke.py
"""
import importlib.util, os, sys
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
Gtk, GLib = gui.Gtk, gui.GLib

failed = 0
def check(name, ok):
    global failed
    print(("ok   " if ok else "FAIL ") + name)
    failed += 0 if ok else 1

def run_with_enter(buttons, entry=True):
    d = Gtk.Dialog(title="Enter test")
    for label, response in buttons:
        d.add_button(label, response)
    e = Gtk.Entry()
    if entry:
        d.get_content_area().add(e)
    d.show_all()
    # Enter, once the dialog runs: in the entry, or on the dialog itself.
    def press():
        if entry:
            e.grab_focus(); e.activate()
        else:
            d.activate_default()
        return False
    GLib.timeout_add(300, press)
    GLib.timeout_add(5000, lambda: d.response(Gtk.ResponseType.DELETE_EVENT) or False)
    r = d.run(); d.destroy()
    return r

check("Enter in an entry presses OK", run_with_enter([("Cancel", Gtk.ResponseType.CANCEL), ("OK", Gtk.ResponseType.OK)]) == Gtk.ResponseType.OK)
check("Enter presses Yes (no entry)", run_with_enter([("No", Gtk.ResponseType.NO), ("Yes", Gtk.ResponseType.YES)], entry=False) == Gtk.ResponseType.YES)
check("Enter presses Accept", run_with_enter([("Cancel", Gtk.ResponseType.CANCEL), ("Lock", Gtk.ResponseType.ACCEPT)]) == Gtk.ResponseType.ACCEPT)
sys.exit(1 if failed else 0)
