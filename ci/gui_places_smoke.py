"""GUI smoke test for places (§3.6): saving where the computer is (with a
fake nmcli that sees chosen networks), turning a block on there (also a
block with nothing in it yet: owner, 2026-11-16), the networks needed, the
"Its places" list and removing a rule. Builds kc9k into a scratch folder, points
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


msgs = []
win.message = lambda text, *a, **k: msgs.append(text)
settle()
real_run = Gtk.Dialog.run


def answering(setup, response=Gtk.ResponseType.OK):
    def fake(dialog):
        setup(dialog)
        return response
    Gtk.Dialog.run = fake


# 1. An empty block: save where the computer is, turned on there at once.
win.selected = "study"; win.show_block()
check(win.places_label.get_text().startswith("No places saved yet"), "no places yet: " + win.places_label.get_text())


def name_home(dialog):
    _d, entry, rule = win.place_dialog
    check(rule is not None and rule.get_active(), "'Add it to study' offered and ticked")
    entry.set_text("home")


answering(name_home)
win.save_place(None); settle(); win.show_block()
place = next((p for p in state()["places"] if p["name"] == "home"), None)
check(place is not None and place["networks"] == 4 and place["need"] == 2, "home saved: %s" % place)
check(place is not None and place["blocks"] == ["study"], "study is on at home: %s" % place)
check([row[0] for row in win.place_rules.store] == ["home"], "On at shows home: %s" % [r[0] for r in win.place_rules.store])
check(win.place_combo.get_active_text() == "home", "the new place is chosen in the list")
check(not [m for m in msgs if m.startswith("Error")], "no error: %s" % msgs)
msgs.clear()

# 2. Networks needed: shown for the chosen place, checked as typed, saved.
check(win.place_need_row.get_visible() and win.place_need.entry.get_text() == "2",
      "needs 2 of 4 shown: %r" % win.place_need.entry.get_text())
check("of its 4 networks" in win.place_need_of.get_text(), win.place_need_of.get_text())
win.place_need.set_text("5")
check(not win.place_need.is_valid() and "At most 4" in win.place_need.problem, "5 of 4 refused where typed")
win.place_need.set_text("3")
win.set_place_need(None); settle(); win.show_block()
check(next(p for p in state()["places"] if p["name"] == "home")["need"] == 3, "need saved as 3")
check("there when 3 of its 4" in win.places_label.get_text(), win.places_label.get_text())

# 3. "Add this place" for another block, then remove the rule.
win.selected = "games"; win.show_block()
check(win.place_combo.get_active_text() == "home", "the place list is there for another block")
win.place_combo.set_active(0)
button = find_button(win, "Add this place")
check(button is not None, "the Add this place button is there")
button.clicked(); settle(); win.show_block()
check([row[0] for row in win.place_rules.store] == ["home"], "games: On at home")
win.place_rules.on_remove("home", 1); settle(); win.show_block()
check(len(win.place_rules.store) == 0, "rule removed from the list")

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
