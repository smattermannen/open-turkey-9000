"""GUI smoke test for YouTube playlists and social media settings (§3.2
Exception list, §4.3; owner 2026-11-17): a playlist is let through by its
address typed into an exception list (no box of its own), kept one way
however it is pasted; the social media settings are a section of the
Websites tab after the blacklists and above the exceptions, opened and
closed, with no tab of their own. Builds kc9k into a scratch folder, a
wrapper with KC9K_DB = scratch file, loads gui/kc9k-gui.py as a module with
CLI replaced, another application ID, never shows the main window. Needs an
X display; run as a normal user.

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


def block():
    return next(b for b in json.loads(cli("gui-state").stdout)["blocks"] if b["name"] == "yt")


def exceptions():
    return next(lst for lst in block()["lists"] if lst["name"] == "Exceptions")["entries"]


settle()
win.selected = "yt"; win.show_block()
card = exceptions_card()
check(not hasattr(card, "playlists") and not find(card, Gtk.Button, lambda b: b.get_label() == "Let through"),
      "the Exceptions card has no playlist box")
card.entries.reveal()
card.entries.entry.set_text("https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf&si=x")
card.entries.entry.emit("activate")
settle()
check("youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf" in exceptions() and not block().get("social"),
      "a playlist's address typed into Exceptions is kept one way: %s" % exceptions())
check(any("lets the playlist" in l.get_text() or "playlist's address" in l.get_text() for l in find(win, Gtk.Label)),
      "the Websites tab says what a playlist's address does")
r = cli("block", "remove-exception", "yt", "youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf")
check(r.returncode == 0 and not any("playlist" in e for e in exceptions()), "removed like any entry: %s" % r.stderr)

# Social media: after the blacklists, above the exceptions, opened and closed.
tabs = [win.block_tabs.get_tab_label_text(win.block_tabs.get_nth_page(n)) for n in range(win.block_tabs.get_n_pages())]
check("Social media" not in tabs, "no Social media tab: %s" % tabs)
websites = win.list_boxes["blacklist"].get_parent()
order = websites.get_children()
check(order.index(win.list_boxes["blacklist"]) < order.index(win.social_expander) < order.index(win.list_boxes["exceptions"]),
      "the Social media section sits between the blacklists and the exceptions")
check(not win.social_expander.get_expanded(), "closed at first")
win.social_expander.set_expanded(True)
check(win.social_expander.get_expanded(), "opens")
win.social_checks["youtube:no-shorts"].set_active(True)
settle()
check(block().get("social") == ["youtube:no-shorts"] and "1 setting on" in win.social_expander.get_label(),
      "a setting ticked in it, and the closed section says so: %s" % win.social_expander.get_label())
win.social_expander.set_expanded(False)
check(not win.social_expander.get_expanded(), "closes")
check(not [m for m in msgs if m.startswith("Error")], "no errors: %s" % msgs)
win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
