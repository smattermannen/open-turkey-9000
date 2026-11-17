"""GUI smoke test for dragging a block onto a mode (§4.2, owner 2026-11-17)
with a real pointer: xdotool presses on the block card's name, moves onto
the mode's card and lets go; the block is then in the mode. Builds kc9k
into a scratch folder, a wrapper with KC9K_DB = scratch file, loads
gui/kc9k-gui.py as a module with CLI replaced, another application ID.
Unlike the other smoke tests the window is shown (a drag needs it). Needs
an X display and xdotool; run as a normal user.

    xvfb-run -a python3 ci/gui_modes_drag_smoke.py
"""
import importlib.util, json, os, subprocess, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-dnd-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")
cli = lambda *a: subprocess.run([W, *a], capture_output=True, text=True)
for a in (["block", "create", "Social"], ["block", "add", "Social", "reddit.com"], ["mode", "create", "Evening"]):
    assert cli(*a).returncode == 0
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib, Gdk
app = gui.App(); app.set_application_id("io.test.kc9kdragsmoke"); app.register(None)
win = gui.Window(app); win.extension_asked = True
win.confirm = lambda *a, **k: True
win.set_default_size(1000, 760); win.move(0, 0); win.show_all()
def pump(secs):
    end = time.time() + secs
    while time.time() < end:
        GLib.MainContext.default().iteration(False); time.sleep(0.005)
pump(3)
win.stack.set_visible_child_name("blocks"); win.blocks_stack.set_visible_child_name("cards"); pump(1.5)
def widgets(root):
    out = [root]
    if isinstance(root, Gtk.Container):
        for c in root.get_children():
            out += widgets(c)
    return out
src = next(w for w in widgets(win.block_cards) if isinstance(w, Gtk.Button) and w.get_label() == "Social")
dst = win.mode_cards.get_children()[1]  # Evening (Monk mode first)
def screen_xy(widget):
    x, y = widget.translate_coordinates(win, widget.get_allocated_width() // 2, widget.get_allocated_height() // 2)
    wx, wy = win.get_window().get_origin()[1:]
    return wx + x, wy + y
sx, sy = screen_xy(src); dx, dy = screen_xy(dst)
def xdo(*a):
    subprocess.Popen(["xdotool", *map(str, a)])
xdo("mousemove", sx, sy); pump(0.3)
xdo("mousedown", 1); pump(0.3)
for i in range(1, 21):
    xdo("mousemove", sx + (dx - sx) * i // 20, sy + (dy - sy) * i // 20); pump(0.05)
pump(0.3)
xdo("mouseup", 1); pump(3)
state = json.loads(cli("gui-state").stdout)
mode = next(b for b in state["blocks"] if b["name"] == "Social")["mode"]
print(("ok   " if mode == "Evening" else "FAIL ") + "a block card dragged onto a mode is in it: %r" % mode)
print("all passed" if mode == "Evening" else "FAILED")
raise SystemExit(0 if mode == "Evening" else 1)
