"""GUI smoke test for presets (§3.2 Preset, §4.4; owner 2026-11-17): the
Presets page (it was Clusters) shows block, lists, automation and schedule
presets with Use…, Rename… and Delete; a block's Websites tab saves its
lists as a preset (blacklists, exceptions or both, with or without
actions), its Automation saves its automation; using them copies into
another block or a mode. Builds kc9k into a scratch folder, a wrapper with
KC9K_DB = scratch file, loads gui/kc9k-gui.py as a module with CLI replaced,
another application ID, never shows the main window. Needs an X display;
run as a normal user.

    xvfb-run -a python3 ci/gui_presets_smoke.py
"""
import importlib.util, json, os, subprocess, tempfile, time
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


for a in (["block", "create", "b"], ["block", "add", "b", "reddit.com"], ["block", "add-exception", "b", "x.com/ok"],
          ["schedule", "add", "b", "--days", "weekdays", "--from", "09:00", "--to", "17:00"],
          ["block", "create", "c"], ["mode", "create", "Evening"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kpresetssmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs = [], []
win.message = lambda text, *a, **k: msgs.append(text)
win.confirm = lambda *a, **k: True


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
    for _ in range(20):
        GLib.MainContext.default().iteration(False)


def widgets(root):
    out = [root]
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += widgets(child)
    return out


def state():
    return json.loads(cli("gui-state").stdout)


def block(name):
    return next(b for b in state()["blocks"] if b["name"] == name)


Gtk.Dialog.run = lambda d: Gtk.ResponseType.CANCEL
win.get_child().show_all()
settle()
check(win.nav_rows["clusters"].get_child().get_children()[1].get_text() == "Presets", "the sidebar says Presets")
win.open_block("b"); settle()
labels = [w.get_label() for w in widgets(win.block_tabs) if isinstance(w, Gtk.Button)]
check("Save lists as preset…" in labels and "Add lists from a preset…" in labels, "the Websites tab offers lists presets")
win.save_preset("lists", "b", answer=("Mine", "both", True)); settle()
win.save_preset("lists", "b", answer=("Black", "blacklists", False)); settle()
win.open_automation("b"); settle()
labels = [w.get_label() for w in widgets(win.automation_page) if isinstance(w, Gtk.Button)]
check("Save as preset…" in labels and "Use a preset…" in labels, "the Automation view offers automation presets")
win.save_preset("automation", "b", answer=("Workdays", None, False)); settle()
kinds = {(p["kind"], p["name"]) for p in state()["presets"]}
check({("lists", "Mine"), ("lists", "Black"), ("automation", "Workdays")} <= kinds, "saved: %s" % kinds)

# The Presets page lists them by type.
win.stack.set_visible_child_name("clusters"); settle()
rows, _note = win.preset_sections["lists"]
names = [w.get_text() for w in widgets(rows) if isinstance(w, Gtk.Label) and w.get_style_context().has_class("kc9k-card-name")]
check(names == ["Black", "Mine"], "lists presets on the page: %s" % names)
check(win.preset_sections["schedule"][1].get_visible(), "an empty section says how to save one")

# Used: lists into c, the automation into the mode.
win.use_preset("lists", "Mine", target="c"); settle()
c = block("c")
lists = {l["name"]: l for l in c["lists"]}
check("Blocked 2" in lists and "reddit.com" in lists["Blocked 2"]["entries"] and "x.com/ok" in lists["Exceptions 2"]["entries"],
      "lists copied into c: %s" % sorted(lists))
win.use_preset("automation", "Workdays", target="mode Evening"); settle()
evening = next(m for m in state()["modes"] if m["name"] == "Evening")
check(len(evening["windows"]) == 1, "the automation copied into the mode: %s" % evening["windows"])
# A copy: deleting b leaves the preset.
r = cli("block", "remove", "b"); check(r.returncode == 0, "b deleted: %s" % r.stderr)
win.use_preset("automation", "Workdays", target="c"); settle()
check(block("c")["schedules"] == ["weekdays 09:00-17:00"], "the automation outlives its block: %s" % block("c")["schedules"])
win.delete_preset("lists", "Black"); settle()
check(("lists", "Black") not in {(p["kind"], p["name"]) for p in state()["presets"]}, "deleted")
check(not [m for m in msgs if m.startswith("Error")], "no errors: %s" % msgs)
win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
raise SystemExit(1 if fails else 0)
