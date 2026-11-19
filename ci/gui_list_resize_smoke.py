"""GUI smoke test for the grip under a list (owner 2026-11-19): dragging it
down makes the list taller, up shorter (never below ResizeGrip.MIN); the
height is kept in gui.json per kind of list, so a new list of the same kind
opens as tall; a double-click puts it back. Loads gui/kc9k-gui.py as a
module with a scratch XDG_CONFIG_HOME; runs no kc9k command. Needs an X
display.

    xvfb-run -a python3 ci/gui_list_resize_smoke.py
"""
import importlib.util, json, os, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
os.environ["XDG_CONFIG_HOME"] = tempfile.mkdtemp(prefix="kc9k-gui-")
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
from gi.repository import Gdk, GLib, Gtk
app = gui.App(); app.set_application_id("io.test.kc9kresize"); app.register(None)
app.prefs = {}
fails = []
def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what); ok or fails.append(what)
def pump(secs=0.3):
    end = time.time() + secs
    while time.time() < end:
        GLib.MainContext.default().iteration(False); time.sleep(0.01)

def make():
    lst = gui.EditableList("", "x", lambda v: None, lambda v: None, search=True, size_key="website list")
    lst.set_items(["entry%d" % i for i in range(200)])
    lst.reveal()
    window = Gtk.Window(); window.add(lst); window.show_all(); pump()
    return window, lst

def event(kind, y, button=1):
    e = Gdk.Event.new(kind)
    e.button.button = button
    e.button.y_root = y
    return e

def drag(grip, dy):
    grip.emit("button-press-event", event(Gdk.EventType.BUTTON_PRESS, 100))
    motion = Gdk.Event.new(Gdk.EventType.MOTION_NOTIFY)
    motion.motion.y_root = 100 + dy
    grip.emit("motion-notify-event", motion)
    grip.emit("button-release-event", event(Gdk.EventType.BUTTON_RELEASE, 100 + dy))
    pump()

window, lst = make()
start = lst.stack.get_allocated_height()
check(isinstance(lst.grip, gui.ResizeGrip) and lst.grip.get_parent() is lst, "a grip under the list")
drag(lst.grip, 300)
check(lst.stack.get_allocated_height() >= start + 290, "drag down 300: taller (%d -> %d)" % (start, lst.stack.get_allocated_height()))
saved = json.load(open(gui.PREFS_PATH)).get("list_heights", {})
check(saved.get("website list", 0) >= start + 290, "height kept in gui.json: %s" % saved)
drag(lst.grip, -5000)
check(lst.stack.get_size_request()[1] == gui.ResizeGrip.MIN, "drag far up: stops at the least height")
drag(lst.grip, 400)
tall = lst.stack.get_allocated_height()
window.destroy()
window2, again = make()
check(abs(again.stack.get_allocated_height() - tall) <= 2, "a new list of the same kind opens as tall (%d, %d)" % (tall, again.stack.get_allocated_height()))
again.grip.emit("button-press-event", event(Gdk.EventType._2BUTTON_PRESS, 100)); pump()
check(again.stack.get_size_request()[1] == -1 and abs(again.stack.get_preferred_height()[0] - start) <= 2,
      "double-click: back to its own height")
check("website list" not in json.load(open(gui.PREFS_PATH)).get("list_heights", {}), "and forgotten in gui.json")
window2.destroy()
print("FAILED: %d" % len(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
