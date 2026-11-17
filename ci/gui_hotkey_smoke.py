"""GUI smoke test for hotkeys (§3.13, R-GUI-2, R-LOCK-12): the Add hotkey
dialog takes keys (checked as typed), a block from a list and any lock the
lock picker offers. Builds kc9k into a scratch folder, points a wrapper at a
scratch database (KC9K_DB: no layers are applied and the desktop's
shortcuts are left alone), loads gui/kc9k-gui.py as a module with CLI
replaced and another application ID, and never shows the main window.
Needs an X display.

    xvfb-run -a python3 ci/gui_hotkey_smoke.py
"""
import datetime, importlib.util, json, os, subprocess, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")  # never the real ~/.config


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


# Keys are checked where they are typed, by the CLI's rules. ---------------
for keys in ("ctrl+alt+s", "Super+F9", "F12", "shift+ctrl+1", "alt+Tab"):
    check(gui.hotkey_problem(keys) is None, f"keys {keys!r} accepted")
for keys, why in (("s", "only F1 to F24"), ("ctrl+", "Type keys"), ("hyper+s", "isn't a modifier"),
                  ("ctrl+é", "isn't a key"), ("F25", "only F1 to F24"), ("", "Type keys")):
    problem = gui.hotkey_problem(keys) or ""
    check(why in problem, f"keys {keys!r} refused: {problem}")
check(gui.hotkey_words("<Primary><Alt>s") == "ctrl+alt+s", "accelerator shown as typed")

# The dialog, end to end --------------------------------------------------
for a in (["block", "create", "study"], ["block", "add", "study", "reddit.com"],
          ["block", "create", "work"], ["block", "add", "work", "example.org"]):
    r = cli(*a); assert r.returncode == 0, r.stderr
app = gui.App(); app.set_application_id("io.test.kc9khotkeysmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True


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
confirmed = []


def confirm(locks, option=None):
    confirmed.append((locks, option))
    win.lock_option = True  # tick "Run without confirmation"
    return True


win.confirm_lock = confirm
real_run = Gtk.Dialog.run


def answering(setup):
    def fake(dialog):
        if dialog.get_title() == "Add a hotkey":
            setup(dialog)
        return Gtk.ResponseType.OK  # the unlock order dialog: as it is
    Gtk.Dialog.run = fake


def add_dialog(dialog):
    fields, picker = win.lock_extra, win.lock_picker
    ok = dialog.get_widget_for_response(Gtk.ResponseType.OK)
    check(fields.block.get_active_id() in ("study", "work"), "a block is chosen from the list")
    check(not ok.get_sensitive(), "Add waits for keys")
    fields.keys.set_text("q")
    check(not ok.get_sensitive() and "only F1 to F24" in fields.note.get_text(),
          "wrong keys explained under the box: " + fields.note.get_text())
    fields.keys.set_text("ctrl+alt+s")
    check(ok.get_sensitive(), "Add on once the keys read right")
    fields.block.set_active_id("study")
    picker.timer.set_until(datetime.datetime(2099, 12, 24, 18, 0))
    picker.restart.set_active(True)
    picker.password.set_text("partner1"); picker.password_again.set_text("partner1")


answering(add_dialog)
win.add_hotkey(None); settle()
keys = state()["hotkeys"]
check(len(keys) == 1 and keys[0]["block"] == "study" and keys[0]["no_confirm"], "hotkey added: %s" % keys)
lock = keys[0].get("lock", "") if keys else ""
check("2099-12-24 18:00" in lock and "restart lock" in lock and "password lock" in lock,
      "its locks, in words: " + lock)
check("password_hash" not in json.dumps(keys), "no password hash in the GUI's state")
check(confirmed and any("set each time ctrl+alt+s is pressed" in line for line in confirmed[-1][0])
      and confirmed[-1][1], "locks confirmed, with Run without confirmation offered: %s" % confirmed[-1:])
shown = [row[0] for row in win.hotkeys.store]
check(shown and shown[0].startswith("ctrl+alt+s  starts 'study' with ") and "runs without confirmation" in shown[0],
      "listed as typed: %s" % shown)
check("hotkeys" in win.nav_rows, "Hotkeys has its own page in the sidebar (§4.13)")

# A hotkey without locks asks nothing.
confirmed.clear()


def plain(dialog):
    win.lock_extra.keys.set_text("F9")
    win.lock_extra.block.set_active_id("work")


answering(plain)
win.add_hotkey(None); settle()
keys = state()["hotkeys"]
check(len(keys) == 2 and keys[1]["block"] == "work" and not keys[1].get("lock") and not confirmed,
      "a plain hotkey, no confirmation: %s" % keys[1:])
Gtk.Dialog.run = real_run

# §3.9 Hotkey actions, §4.13 (owner 2026-11-17): any actions in order, from
# the Hotkeys page; locks confirmed; saved as a preset and added again.
confirmed.clear()
r = cli("mode", "create", "Evening"); assert r.returncode == 0, r.stderr
settle()
actions = [({"kind": "start", "target": "work"}, "Start a block work", None),
           ({"kind": "mode-run", "target": "Evening", "minutes": 60}, "Run Evening, locked, for 1 h", None),
           ({"kind": "custom", "param": json.dumps([{"open": "https://example.org/plan"}])}, "open https://example.org/plan", None)]
win.add_hotkey_actions(answer=("ctrl+alt+p", actions, False)); settle()
keys = state()["hotkeys"]
added = next((h for h in keys if h["keys"] == "<Primary><Alt>p"), None)
check(added is not None and len(added.get("actions") or []) == 3 and "runs the mode 'Evening', locked" in added.get("does", ""),
      "a hotkey of three actions: %s" % added)
check(confirmed and any("Run Evening, locked" in line for line in confirmed[-1][0]), "its lock confirmed: %s" % confirmed[-1:])
shown = [row[0] for row in win.hotkeys.store]
check(any(row.startswith("ctrl+alt+p  starts 'work', then runs the mode 'Evening'") for row in shown), "listed: %s" % shown)
r = cli("preset", "save", "hotkey", str(added["id"]), "Plan")
check(r.returncode == 0, "saved as a preset: %s" % r.stderr.strip())
settle()
win.use_preset("hotkey", "Plan", target="ctrl+alt+o"); settle()
check(any(h["keys"] == "<Primary><Alt>o" for h in state()["hotkeys"]), "added again from the preset on other keys")
check(not [m for m in msgs if m.startswith("Error")], "no errors shown: " + "; ".join(msgs))

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
