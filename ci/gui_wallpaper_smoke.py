"""GUI smoke test for wallpapers of modes (R-MODE-5, §4.10; owner
2026-11-17): a mode's view has circadian rhythm's wallpaper layout and
"Override circadian rhythm wallpapers", the circadian rhythm page "Override
modes" (one setting, seen from both sides), and Settings the wallpaper
default (Latest wallpaper, Circadian rhythm, Custom with its picture).
Builds kc9k into a scratch folder, points a wrapper at a scratch database
(KC9K_DB: nothing reaches the desktop), loads gui/kc9k-gui.py as a module
with CLI replaced and another application ID, and never shows the window.
Needs an X display.

    xvfb-run -a python3 ci/gui_wallpaper_smoke.py
"""
import importlib.util, json, os, struct, subprocess, sys, tempfile, time, zlib
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")  # never the real ~/.config


def picture(name, shade):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    path = os.path.join(S, name)
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 8, 8, 8, 2, 0, 0, 0))
                + chunk(b"IDAT", zlib.compress(b"".join(b"\x00" + bytes([shade, 0x80, 0x20]) * 8 for _ in range(8))))
                + chunk(b"IEND", b""))
    return path


SCHOOL, PLAIN = picture("school.png", 0x10), picture("plain.png", 0x90)


def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)


def state():
    return json.loads(cli("gui-state").stdout)


for a in (["block", "create", "lessons"], ["mode", "create", "School"], ["mode", "add", "School", "lessons"]):
    r = cli(*a); assert r.returncode == 0, r.stderr
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402

app = gui.App(); app.set_application_id("io.test.kc9kwallpapersmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails = []


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


def pump(cond, secs=20):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"


def settle():
    pump(lambda: win.get_sensitive() and not win.busy)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)


msgs = []
win.message = lambda text, *a, **k: msgs.append(text)
settle()
win.get_child().show_all()

# The mode's view: circadian rhythm's layout. -------------------------------
win.open_mode("School"); settle(); win.update_mode_view()
check(win.mode_wallpaper_section.get_visible() and isinstance(win.mode_wallpapers, gui.WallpaperEditor),
      "the mode's view has a Wallpapers section with the same editor as circadian rhythm")
win.mode_wallpapers.add(None, answer=(SCHOOL, "07:00", "16:00")); settle(); win.update_mode_view()
mode = next(m for m in state()["modes"] if m["name"] == "School")
check([w["name"] for w in mode["wallpapers"]] == ["school.png"] and mode["wallpapers"][0]["from"] == "07:00",
      "added through the editor: %s" % mode["wallpapers"])
rows = win.mode_wallpapers.rows.get_children()
check(len(rows) == 1 and win.mode_wallpapers.timeline.get_visible(), "one row, and the timeline shows it")
win.mode_wallpapers.moved(1, 8 * 60, 15 * 60); settle(); win.update_mode_view()
mode = next(m for m in state()["modes"] if m["name"] == "School")
check(mode["wallpapers"][0]["from"] == "08:00" and mode["wallpapers"][0]["to"] == "15:00", "dragged on the timeline")

# One setting, seen from both sides. ------------------------------------------
check(win.mode_override.get_active(), "modes override circadian rhythm at first")
win.mode_override.set_active(False); settle(); win.update_mode_view()
check(state()["wallpaper"]["override_modes"] and not win.mode_override.get_active(),
      "Override circadian rhythm wallpapers off: circadian rhythm overrides modes")
page = win.circadian
pump(lambda: win.get_sensitive() and not page.reading)
page.view = None; page.refresh(force=True); pump(lambda: page.view is not None)
check(page.override_modes.get_active(), "the circadian rhythm page's Override modes is on")
page.override_modes.emit("state-set", False); settle()
check(not state()["wallpaper"]["override_modes"], "Override modes off there: modes win again")
win.update_mode_view()
check(win.mode_override.get_active(), "and the mode's switch shows it")

# Frozen Chicken and monk mode take wallpapers too. -------------------------
win.open_mode("Frozen Chicken"); settle(); win.update_mode_view()
check(win.mode_wallpaper_section.get_visible(), "Frozen Chicken's view has Wallpapers")
win.mode_wallpapers.add(None, answer=(SCHOOL, "22:00", "06:00")); settle()
fc = next(m for m in state()["modes"] if m.get("freeze"))
check(len(fc["wallpapers"]) == 1, "Frozen Chicken's wallpaper: %s" % fc["wallpapers"])

# Removing from a row. -----------------------------------------------------
win.open_mode("School"); settle(); win.update_mode_view()
remove = [c for c in win.mode_wallpapers.rows.get_children()[0].get_children()
          if isinstance(c, Gtk.Button) and c.get_label() == "Remove"]
remove[0].clicked(); settle(); win.update_mode_view()
mode = next(m for m in state()["modes"] if m["name"] == "School")
check(not mode["wallpapers"] and not win.mode_wallpapers.timeline.get_visible(), "removed")

# Settings: the wallpaper default. -------------------------------------------
win.update_settings()
check(win.wallpaper_default.get_active_id() == "latest" and "last place's" in win.wallpaper_default_note.get_text(),
      "Latest wallpaper by default, with what that means: " + win.wallpaper_default_note.get_text())
win.wallpaper_default.set_active_id("circadian"); settle(); win.update_settings()
check(state()["wallpaper"]["default"] == "circadian" and "switched off" in win.wallpaper_default_note.get_text(),
      "Circadian rhythm")
chosen = []
gui.choose_picture = lambda *a, **k: chosen.append(1) or PLAIN
win.wallpaper_default.set_active_id("custom"); settle(); win.update_settings()
w = state()["wallpaper"]
check(chosen and w["default"] == "custom" and w["custom_name"] == "plain.png" and win.wallpaper_custom_row.get_visible(),
      "Custom asks for a picture and shows it: %s" % w)
check("before the computer sleeps" in win.wallpaper_default_note.get_text(), "and says when it shows")
gui.choose_picture = lambda *a, **k: None
win.wallpaper_default.set_active_id("latest"); settle(); win.update_settings()
win.wallpaper_default.set_active_id("custom"); settle(); win.update_settings()
check(state()["wallpaper"]["default"] == "custom", "Custom again takes the picture chosen before, without asking")
check(not [m for m in msgs if m.startswith("Error")], "no errors shown: " + "; ".join(msgs))

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
