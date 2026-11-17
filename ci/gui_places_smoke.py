"""GUI smoke test for places (§3.6, §4.5): saving where the computer is
(with a fake nmcli that sees chosen networks), turning a block on there
(also a block with nothing in it yet: owner, 2026-11-16), each added place
on a row saying it is added, what arriving does and whether the computer is
there (owner 2026-11-17), the networks needed (Places page), removing a
rule. Builds kc9k into a scratch folder, points
a wrapper at a scratch database (KC9K_DB: no layers are applied), loads
gui/kc9k-gui.py as a module with CLI replaced and another application ID,
and never shows the main window. Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_places_smoke.py
"""
import importlib.util, json, os, subprocess, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
BIN = os.path.join(S, "bin")
WIFI = os.path.join(S, "wifi.txt")
os.makedirs(BIN)
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
# nmcli's terse output: BSSID (colons written \:) and SSID.
with open(os.path.join(BIN, "nmcli"), "w") as f:
    f.write(f'#!/bin/sh\ncat "{WIFI}"\n')
os.chmod(os.path.join(BIN, "nmcli"), 0o755)
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nPATH="{BIN}:$PATH" KC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")  # never the real ~/.config


def wifi(*bssids):
    with open(WIFI, "w") as f:
        for i, b in enumerate(bssids):
            f.write(b.replace(":", "\\:") + f":net{i}\n")


def cli(*a, stdin=""):
    return subprocess.run([W, *a], capture_output=True, text=True, input=stdin)


def state():
    return json.loads(cli("gui-state").stdout)


spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402

fails = []


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


HOME = ["AA:AA:AA:AA:AA:01", "AA:AA:AA:AA:AA:02", "AA:AA:AA:AA:AA:03", "AA:AA:AA:AA:AA:04"]
wifi(*HOME)
for a in (["block", "create", "study"], ["block", "create", "games"], ["block", "add", "games", "example.org"]):
    r = cli(*a); assert r.returncode == 0, r.stderr

app = gui.App(); app.set_application_id("io.test.kc9kplacessmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True


def pump(cond, secs=30):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"


def settle():
    pump(lambda: win.get_sensitive() and not win.busy)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)


def find_button(root, label):
    """The button with this label under root (containers and flow boxes too)."""
    if isinstance(root, Gtk.Button) and root.get_label() == label:
        return root
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            found = find_button(child, label)
            if found is not None:
                return found
    return None


def gui_widgets(root):
    out = [root]
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += gui_widgets(child)
    return out


msgs = []
win.message = lambda text, *a, **k: msgs.append(text)
win.get_child().show_all()
settle()
real_run = Gtk.Dialog.run


def answering(setup, response=Gtk.ResponseType.OK):
    def fake(dialog):
        setup(dialog)
        return response
    Gtk.Dialog.run = fake


# 1. An empty block: save where the computer is, turned on there at once.
win.selected = "study"; win.show_block()
check(win.block_places.empty.get_text().startswith("No places saved yet"),
      "no places yet: " + win.block_places.empty.get_text())


def name_home(dialog):
    _d, entry, rule = win.place_dialog
    check(rule is not None and rule.get_active(), "'Add it to study' offered and ticked")
    entry.set_text("home")


answering(name_home)
win.save_place(None); settle(); win.show_block()
place = next((p for p in state()["places"] if p["name"] == "home"), None)
check(place is not None and place["networks"] == 4 and place["need"] == 2, "home saved: %s" % place)
check(place is not None and place["blocks"] == ["study"], "study is on at home: %s" % place)
check(win.block_places.names == ["home"], "the block shows home as added: %s" % win.block_places.names)
texts = [w.get_text() for w in gui_widgets(win.block_places.rows) if isinstance(w, Gtk.Label)]
check("Added. Arriving here turns this block on." in texts and "You are here" in texts,
      "its row says it is added, what arriving does, and that the computer is there: %s" % texts)
check(win.place_combo.get_active_text() is None, "nothing left to add: %s" % win.place_combo.get_active_text())
check(not [m for m in msgs if m.startswith("Error")], "no error: %s" % msgs)
msgs.clear()
# Enabled by default: the row says arriving turns it off.
r = cli("automation", "study", "default", "enabled"); assert r.returncode == 0, r.stderr
settle(); win.show_block()
texts = [w.get_text() for w in gui_widgets(win.block_places.rows) if isinstance(w, Gtk.Label)]
check("Added. Arriving here turns this block off." in texts, "enabled by default: off here: %s" % texts)

# 2. Networks needed: on the Places page, checked as typed, saved.
win.stack.set_visible_child_name("places"); settle()
need = next(w for w in gui_widgets(win.place_cards) if isinstance(w, gui.NumberField))
check(need.entry.get_text() == "2", "needs 2 of 4 shown: %r" % need.entry.get_text())
need.set_text("5")
check(not need.is_valid() and "At most 4" in need.problem, "5 of 4 refused where typed")
need.set_text("3")
next(w for w in gui_widgets(need.get_parent()) if isinstance(w, Gtk.Button) and w.get_label() == "Save").clicked()
settle()
check(next(p for p in state()["places"] if p["name"] == "home")["need"] == 3, "need saved as 3")
win.stack.set_visible_child_name("blocks"); settle()

# 3. "Add this place" for another block, then remove the rule.
win.selected = "games"; win.show_block()
check(win.place_combo.get_active_text() == "home", "the place list is there for another block")
button = find_button(win.block_places, "Add this place")
check(button is not None, "the Add this place button is there")
button.clicked(); settle(); win.show_block()
check(win.block_places.names == ["home"], "games: home added")
find_button(win.block_places, "Remove").clicked(); settle(); win.show_block()
check(win.block_places.names == [] and win.block_places.empty.get_visible(), "rule removed from the list")

# 4. Saving without the tick saves only the place.
wifi("BB:BB:BB:BB:BB:01", "BB:BB:BB:BB:BB:02")


def name_library(dialog):
    _d, entry, rule = win.place_dialog
    entry.set_text("library")
    rule.set_active(False)


answering(name_library)
win.save_place(None); settle(); win.show_block()
lib = next((p for p in state()["places"] if p["name"] == "library"), None)
check(lib is not None and lib["blocks"] == [], "library saved without a rule: %s" % lib)
check(win.place_combo.get_active_text() == "library", "library chosen after saving")

# 5. No Wi-Fi in range: the CLI says why, nothing is saved.
wifi()
msgs.clear()


def name_nowhere(dialog):
    win.place_dialog[1].set_text("nowhere")


answering(name_nowhere)
win.save_place(None); settle()
check(any("no Wi-Fi network is in range" in m for m in msgs), "no networks explained: %s" % msgs)
check(not any(p["name"] == "nowhere" for p in state()["places"]), "nothing saved without networks")

Gtk.Dialog.run = real_run
win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
