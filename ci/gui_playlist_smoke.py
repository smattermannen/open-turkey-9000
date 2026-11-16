"""GUI smoke test for YouTube playlists let through (§3.2; owner,
2026-11-16: they belong with the exceptions, in a box as tall as its
playlists): the Exceptions card has the box, "Let through" adds one, its
row's Remove takes it away, the Social media tab no longer has a playlist
box. Builds kc9k into a scratch folder, a wrapper with KC9K_DB = scratch
file, loads gui/kc9k-gui.py as a module with CLI replaced, another
application ID, never shows the main window. Needs an X display; run as a
normal user.

    xvfb-run -a python3 ci/gui_playlist_smoke.py
"""
import importlib.util, json, os, subprocess, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)


for a in (["block", "create", "yt"], ["block", "add", "yt", "youtube.com"]):
    r = cli(*a); assert r.returncode == 0, r.stderr

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kplaylistsmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs = [], []
win.message = lambda text, *a, **k: msgs.append(text)


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


def pump(cond, secs=30):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"


def settle():
    pump(lambda: win.get_sensitive() and not win.busy)
    for _ in range(30):
        GLib.MainContext.default().iteration(False)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
    win.show_block()


def find(root, kind, test=lambda w: True):
    out = []
    if isinstance(root, kind) and test(root):
        out.append(root)
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += find(child, kind, test)
    return out


def exceptions_card():
    return next(c for c in win.list_cards.values() if getattr(c, "data", {}).get("name") == "Exceptions")


def social():
    return next(b for b in json.loads(cli("gui-state").stdout)["blocks"] if b["name"] == "yt").get("social") or []


settle()
win.selected = "yt"; win.show_block()
card = exceptions_card()
check(card.playlists is not None, "the Exceptions card has the playlists box")
blocked = next(c for c in win.list_cards.values() if getattr(c, "data", {}).get("name") == "Blocked")
check(getattr(blocked, "playlists", None) is None, "a blacklist has none")
check(not card.playlists.rows.get_visible() or not card.playlists.rows.get_children(), "no rows without playlists")
check(not find(card.playlists, Gtk.ScrolledWindow), "no fixed-height scroll area in the box")

card.playlists.entry.set_text("https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf")
find(card.playlists, Gtk.Button, lambda b: b.get_label() == "Let through")[0].clicked()
settle()
card = exceptions_card()
check("youtube:playlist=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf" in social(), "playlist let through: %s" % social())
check(len(card.playlists.rows.get_children()) == 1, "one row for one playlist")
card.playlists.entry.set_text("PLx0sYbCqOb8TBPRdmBHs5Iftvv9TPboYG")
card.playlists.add()
settle()
card = exceptions_card()
check(len(card.playlists.rows.get_children()) == 2, "two rows for two playlists")
entries = next(lst for lst in next(b for b in json.loads(cli("gui-state").stdout)["blocks"])["lists"]
               if lst["name"] == "Exceptions")["entries"]
check("youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf" in entries, "its page is an exception: %s" % entries)

remove = find(card.playlists, Gtk.Button, lambda b: b.get_label() == "Remove")
remove[0].clicked()
settle()
card = exceptions_card()
check("youtube:playlist=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf" not in social() and len(card.playlists.rows.get_children()) == 1,
      "the row's Remove took it away: %s" % social())

# The Social media tab has no playlist box any more.
social_texts = [l.get_text() for l in find(win, Gtk.Label) if "playlist" in l.get_text().lower()]
check(not any(t == "YouTube playlists" for t in social_texts), "no playlists heading on the Social media tab")
check(not hasattr(win, "playlist_list"), "the old playlist list is gone")
check(not [m for m in msgs if m.startswith("Error")], "no errors: %s" % msgs)
win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
