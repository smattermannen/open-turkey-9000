"""GUI smoke test for trigger actions (owner, 2026-11-16: any trigger to
any action): a block's apps and window titles each have a set of actions,
ticked in the GUI; a refused change goes back. A place rule has none (§3.6:
it only enables or lifts its block). Clicks the real
checkboxes. Builds kc9k into a scratch folder, a wrapper with KC9K_DB =
scratch file (and a fake nmcli), loads gui/kc9k-gui.py as a module with
CLI replaced, another application ID, never shows the main window. Needs an
X display; run as a normal user.

    xvfb-run -a python3 ci/gui_trigger_smoke.py
"""
import importlib.util, json, os, subprocess, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
BIN = os.path.join(S, "bin")
os.makedirs(BIN)
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(os.path.join(BIN, "nmcli"), "w") as f:
    f.write('#!/bin/sh\nprintf "AA\\\\:AA\\\\:AA\\\\:AA\\\\:AA\\\\:01:home\\nAA\\\\:AA\\\\:AA\\\\:AA\\\\:AA\\\\:02:next\\n"\n')
os.chmod(os.path.join(BIN, "nmcli"), 0o755)
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nPATH="{BIN}:$PATH" KC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)


for a in (["block", "create", "b"], ["block", "add-app", "b", "steam"], ["block", "add-title", "b", "YouTube"],
          ["place", "add", "home"], ["place", "rule", "b", "home", "on"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9ktriggersmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs = [], []
win.message = lambda text, *a, **k: msgs.append(text)
win.acknowledge_no_confirmation = lambda what: True


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
    for _ in range(40):
        GLib.MainContext.default().iteration(False); time.sleep(0.005)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
    win.show_block()


def triggers():
    return next(b for b in json.loads(cli("gui-state").stdout)["blocks"] if b["name"] == "b")["triggers"]


def gui_widgets(root):
    out = [root]
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += gui_widgets(child)
    return out


def names(group):
    return sorted((a["name"], a.get("minutes", 0)) for a in triggers()[group])


settle()
win.selected = "b"; win.show_block()
apps = win.app_triggers
check(apps.checks["close"].get_active() and not apps.checks["logout"].get_active(), "apps: Close the app by default")
check(win.title_triggers.checks["close"].get_active(), "titles: Close the window by default")

apps.times["logout"].set_text("30m")
apps.checks["logout"].set_active(True); settle()
check(names("apps") == [("close", 0), ("logout", 30)], "logout for 30 minutes added: %s" % names("apps"))
apps.checks["close"].set_active(False); settle()
check(names("apps") == [("logout", 30)], "the app is no longer closed: %s" % names("apps"))
apps.times["monk"].set_text("2h")
apps.checks["monk"].set_active(True); settle()
check(("monk", 120) in names("apps"), "monk mode for 2 hours, acknowledged: %s" % names("apps"))
win.title_triggers.checks["close-browser"].set_active(True); settle()
check(names("titles") == [("close", 0), ("close-browser", 0)], "titles close the browsers too: %s" % names("titles"))

# §3.6: a place rule has no actions: nothing to tick beside On at, and
# gui-state sends none.
labels = [w.get_label() for w in gui_widgets(win.block_places) if isinstance(w, Gtk.Button)]
check("Actions…" not in labels and not hasattr(win, "place_rule_actions"), "no actions for a place rule: %s" % labels)
check(not any(k.startswith("place:") for k in triggers()), "gui-state has no place actions: %s" % list(triggers()))

# Locked: turning an action off is refused and the box goes back.
r = cli("start", "b", "--for", "1h", "--yes"); assert r.returncode == 0, r.stderr
settle()
msgs.clear()
apps = win.app_triggers
apps.checks["logout"].set_active(False)
pump(lambda: bool(msgs), secs=20)
settle()
check(any("locked" in m for m in msgs), "refused while locked: %s" % msgs)
pump(lambda: apps.checks["logout"].get_active(), secs=10)
check(("logout", 30) in names("apps"), "logout stayed: %s" % names("apps"))
msgs.clear()
apps.checks["close"].set_active(True); settle()
check(("close", 0) in names("apps") and not msgs, "turning close back on is free while locked: %s" % msgs)

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
