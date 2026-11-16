"""GUI smoke test for the circadian rhythm (SPEC §4.10): builds kc9k into a
scratch folder, points a wrapper at a scratch database (KC9K_DB: no
hardware is ever touched), loads gui/kc9k-gui.py as a module with CLI
replaced and another application ID, and never shows the window. Needs an
X display (Xvfb will do).

    python3 ci/gui_circadian_smoke.py
"""
import importlib.util, json, os, struct, subprocess, sys, tempfile, time, types, zlib
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)
def show():
    r = cli("circadian", "show", "--json")
    assert r.returncode == 0, "show failed: " + r.stderr + r.stdout
    return json.loads(r.stdout)
# A small PNG to use as a wallpaper.
def chunk(kind, data):
    return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
PNG = os.path.join(S, "dawn.png")
with open(PNG, "wb") as f:
    f.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 8, 8, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(b"".join(b"\x00" + b"\xff\x80\x20" * 8 for _ in range(8))))
            + chunk(b"IEND", b""))
for a in (["circadian", "on"], ["circadian", "lane", "kelvin", "on"],
          ["circadian", "wallpaper", "add", PNG, "06:00", "12:00"],
          ["circadian", "exempt", "add", "example.com"]):
    r = cli(*a); assert r.returncode == 0, r.stderr
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gdk, Gtk, GLib
app = gui.App(); app.set_application_id("io.test.kc9kcircadian"); app.register(None)
win = gui.Window(app)
win.extension_asked = True  # no install dialog in a test
def pump(cond, secs=15):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"
fails = []
def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what); ok or fails.append(what)
page = win.circadian
def reread():
    pump(lambda: win.get_sensitive() and not page.reading)
    page.view = None; page.refresh(force=True); pump(lambda: page.view is not None)

# The curve is the daemon's: compare with what the command computes.
check(abs(gui.clip_shape(0.5, 1, True) - 0.99) < 1e-9 and abs(gui.clip_shape(0.5, 1, False) - 0.01) < 1e-9
      and gui.clip_shape(0.3, 0, True) == 0.3, "clip_shape is circadian.Shape")
check(gui.kelvin_rgb(6500) == (1.0, 1.0, 1.0) and gui.kelvin_rgb(2000)[2] < 0.3, "kelvin colours")
reread()
check(win.stack.get_child_by_name("circadian") is not None, "the page is in the sidebar")
check(page.master.get_active(), "master switch shows on")
check(list(page.editors) == ["backlight", "software", "kelvin"], "three clips, in order")
check(page.lane_switches["kelvin"].get_active() and not page.lane_switches["backlight"].get_active(), "clip switches")
for lane in page.view["lanes"]:
    pts = lane["points"]
    ours = gui.clip_value(pts, page.view["minute"])
    check(abs(ours - lane["value"]) <= 0.06 * (50 if lane["unit"] == "K" else 1) + 0.06,
          f"{lane['name']}: value now matches the command ({ours:.1f} vs {lane['value']})")
check(len(page.wallpapers.get_children()) == 1 and isinstance(page.wallpapers.get_children()[0], Gtk.Box), "one wallpaper row")
check(len(page.exempt.get_children()) == 1, "one exempt site")
check(page.dark_radios["system"].get_active(), "dark mode: system")
check(page.preset.get_active_id() == "natural" and page.preset_detail.get_text() != "", "preset picker")

# Drawing every clip on an off-screen surface (never shown).
import cairo
for name, editor in page.editors.items():
    surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, 700, 190)
    editor.get_allocated_width = lambda: 700
    editor.get_allocated_height = lambda: 190
    try:
        editor.draw(editor, cairo.Context(surface)); ok = True
    except Exception as error:  # noqa: BLE001
        ok = False; print(error)
    check(ok, f"{name}: draws")

# Editing a clip with the pointer: right-click adds, drag moves, the curve
# bends, right-click on a point removes.
editor = page.editors["kelvin"]
def event(kind, x, y, button=1, state=0, type_=None):
    e = types.SimpleNamespace(x=x, y=y, button=button, state=Gdk.ModifierType(state), type=type_ or Gdk.EventType.BUTTON_PRESS)
    return getattr(editor, kind)(editor, e)
before = len(editor.points)
x, y = editor.x_of(13 * 60), editor.y_of(5000)
event("pressed", x, y, button=3)
pump(lambda: len(show()["lanes"][2]["points"]) == before + 1)
added = [p for p in show()["lanes"][2]["points"] if p["minute"] == 780]
check(len(added) == 1 and added[0]["value"] == 5000, "right-click adds a point at 13:00, 5000 K")
reread(); editor = page.editors["kelvin"]
previews = []
real_background = page.background
page.background = lambda args, then=None: (previews.append(args) if args[1] == "preview" else None, real_background(args, then))[1]
index = [p["minute"] for p in editor.points].index(780)
event("pressed", editor.x_of(780), editor.y_of(5000))
event("motion", editor.x_of(13 * 60 + 32), editor.y_of(4000))
check(editor.points[index]["minute"] == 810 and editor.points[index]["value"] == 4000, "drag snaps to 5 minutes and 50 K")
event("motion", editor.x_of(13 * 60 + 32), editor.y_of(4000), state=int(Gdk.ModifierType.SHIFT_MASK))
check(editor.points[index]["minute"] == 812, "with Shift: to the minute")
event("motion", editor.x_of(23 * 60 + 59), editor.y_of(4000))
check(editor.points[index]["minute"] == editor.points[index + 1]["minute"] - 1, "a point stays before its neighbour")
event("motion", editor.x_of(14 * 60), editor.y_of(4000))
event("released", editor.x_of(14 * 60), editor.y_of(4000))
pump(lambda: any(p["minute"] == 840 and p["value"] == 4000 for p in show()["lanes"][2]["points"]))
check(True, "the dragged point is stored")
pump(lambda: any(a[:3] == ["circadian", "preview", "kelvin"] and "--end" in a for a in previews), 10)
check(any(a[3:] == ["4000"] for a in previews), "the drag previewed 4000 K: " + str(previews[:3]))
check(any("--end" in a for a in previews), "the preview is ended after the drag")
reread(); editor = page.editors["kelvin"]
# Bend the curve after the 14:00 point.
index = [p["minute"] for p in editor.points].index(840)
mid = (editor.points[index]["minute"] + editor.points[index + 1]["minute"]) / 2
my = editor.y_of(gui.clip_value(editor.points, mid))
event("pressed", editor.x_of(mid), my)
check(editor.drag is not None and editor.drag[0] == "tension", "pressing the curve grabs its bend")
event("motion", editor.x_of(mid), my - 40)
event("released", editor.x_of(mid), my - 40)
pump(lambda: any(p["minute"] == 840 and p["tension"] == 0.5 for p in show()["lanes"][2]["points"]))
check(True, "dragging the curve up by 40 px stores tension 0.5")
reread(); editor = page.editors["kelvin"]
event("pressed", editor.x_of(840), editor.y_of(4000), button=3)
pump(lambda: not any(p["minute"] == 840 for p in show()["lanes"][2]["points"]))
check(True, "right-click on a point removes it")
check(gui.points_text([{"minute": 65, "value": 40, "tension": 0}, {"minute": 600, "value": 80.5, "tension": -0.25}])
      == "01:05=40.0 10:00=80.5~-0.25", "points_text")

# Switches and settings run their commands.
def settle():
    pump(lambda: win.get_sensitive()); reread()
page.lane_switches["backlight"].set_active(True); settle()
check(show()["lanes"][0]["on"], "clip switch turns the backlight clip on")
page.dark_radios["dark"].set_active(True); settle()
check(show()["dark"]["mode"] == "dark", "dark mode radio")
page.light_from.set_text("08:00"); page.light_to.set_text("19:30"); page.dark_radios["scheduled"].set_active(True); settle()
d = show()["dark"]
check(d["mode"] == "scheduled" and d["light_from"] == "08:00" and d["light_to"] == "19:30", "light between two times")
page.dark_pages.set_active(True); settle()
check(show()["dark_pages"], "dark pages switch")
page.exempt_entry.set_text("docs.example.org"); page.add_exempt(None); settle()
check(show()["dark_exempt"] == ["docs.example.org", "example.com"] and len(page.exempt.get_children()) == 2, "exempt site added")
win.confirm = lambda *a, **k: True
page.preset.set_active_id("candlelight"); page.use_preset(None); settle()
check(show()["lanes"][2]["points"][0]["value"] == 1900, "preset replaces the clips")
row = page.wallpapers.get_children()[0]
entries = [c for c in row.get_children() if isinstance(c, Gtk.Entry)]
entries[0].set_text("07:00"); entries[0].emit("activate"); settle()
check(show()["wallpapers"][0]["from"] == "07:00", "wallpaper times changed")
[c for c in page.wallpapers.get_children()[0].get_children() if isinstance(c, Gtk.Button) and c.get_label() == "Remove"][0].clicked(); settle()
check(show()["wallpapers"] == [], "wallpaper removed")
# A paused clip shows with Resume.
import sqlite3
db = sqlite3.connect(os.path.join(S, "t.db"), timeout=10)
db.execute("insert or replace into settings(key, value) values('circadian.paused', ?)",
           (json.dumps({"gamma": int(time.time()) + 3600}),))
db.commit(); db.close()
reread()
check(len(page.warnings.get_children()) == 1 and page.warnings.get_visible(), "a paused clip is announced")
check("paused until" in page.lane_values["kelvin"].get_text(), "and marked on the clip")
win.run(["circadian", "resume", "kelvin"]); settle()
check(not show()["lanes"][2]["paused_until"] and not page.warnings.get_children(), "Resume ends the pause")
# Owner, 2026-11-16: no Circadian section in Settings (it has its own page).
def labels(root):
    out = [root.get_text()] if isinstance(root, Gtk.Label) else []
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += labels(child)
    return out
check("Circadian rhythm" not in labels(win.stack.get_child_by_name("settings")), "Settings has no Circadian section")

# The wallpaper timeline: drag an end, then the whole bar (a minute a pixel).
page.background = real_background
r = cli("circadian", "wallpaper", "add", PNG, "06:00", "12:00"); assert r.returncode == 0, r.stderr
reread()
tl = page.wallpaper_timeline
tl.get_allocated_width = lambda: 1440 + tl.LEFT + tl.RIGHT
check(len(tl.items) == 1 and (tl.items[0]["from"], tl.items[0]["to"]) == (360, 720), "the bar: %s" % tl.items[:1])
y = tl.lane_top(0) + tl.LANE / 2
def mouse(x, shift=False):
    return types.SimpleNamespace(x=x, y=y, button=1, state=Gdk.ModifierType.SHIFT_MASK if shift else 0)
check(tl.hit(tl.x_of(720), y)[1] == "to" and tl.hit(tl.x_of(500), y)[1] == "move", "ends and body are grabbed")
tl.pressed(tl, mouse(tl.x_of(720))); tl.motion(tl, mouse(tl.x_of(902))); tl.released(tl, mouse(tl.x_of(902)))
settle()
check(show()["wallpapers"][0]["to"] == "15:00", "end dragged to 15:00 (5-minute steps): %s" % show()["wallpapers"][0])
tl.pressed(tl, mouse(tl.x_of(500))); tl.motion(tl, mouse(tl.x_of(1100), shift=True))
tl.released(tl, mouse(tl.x_of(1100), shift=True))
settle()
w = show()["wallpapers"][0]
check((w["from"], w["to"]) == ("16:00", "01:00"), "moved by 10 hours, past midnight: %s" % w)
check(len(tl.segments(tl.items[0])) == 2, "a bar past midnight is two pieces: %s" % tl.segments(tl.items[0]))
check(tl.hit(tl.x_of(960), y)[1] == "from" and tl.hit(tl.x_of(60), y)[1] == "to", "its ends are grabbed on both pieces")
tl.pressed(tl, mouse(tl.x_of(960))); tl.motion(tl, mouse(tl.x_of(1431))); tl.released(tl, mouse(tl.x_of(1431)))
settle()
w = show()["wallpapers"][0]
check((w["from"], w["to"]) == ("23:50", "01:00"), "start dragged to 23:50: %s" % w)
tl.pressed(tl, mouse(tl.x_of(60))); tl.motion(tl, mouse(tl.x_of(1430))); tl.released(tl, mouse(tl.x_of(1430)))
settle()
w = show()["wallpapers"][0]
check((w["from"], w["to"]) == ("23:50", "01:00"), "an end can't be dragged onto the start: %s" % w)

# The timeline draws: two pictures, one covering part of the other.
r = cli("circadian", "wallpaper", "add", PNG, "10:00", "20:00"); assert r.returncode == 0, r.stderr
reread()
tl = page.wallpaper_timeline
tl.get_allocated_width = lambda: 900
surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, 900, tl.get_size_request()[1])
cr = cairo.Context(surface)
tl.get_allocated_height = lambda: tl.get_size_request()[1]
try:
    tl.draw(tl, cr)
    drawn = True
except Exception as error:  # noqa: BLE001
    drawn = False
    print(error)
check(drawn and len(tl.items) == 2, "the timeline draws two pictures")
if os.environ.get("KC9K_SHOT"):
    surface.write_to_png(os.environ["KC9K_SHOT"])

# Play the day (owner, 2026-11-16): the button plays, a playhead runs, then all is back.
page.PLAY_SECONDS = 3
page.play.clicked()
check(not page.play.get_sensitive() and page.play.get_label() == "Playing…", "playing")
pump(lambda: any(e.playhead is not None for e in page.editors.values()), secs=5)
check(True, "a playhead runs over the clips")
pump(lambda: page.play.get_sensitive(), secs=40)
check(page.play.get_label() == "Play the day" and all(e.playhead is None for e in page.editors.values()),
      "after playing: the button and the clips are back")

page.master.set_active(False); settle()
check(not show()["on"], "master switch off")
print("\n%d failed" % len(fails) if fails else "\nall passed")
sys.exit(1 if fails else 0)
