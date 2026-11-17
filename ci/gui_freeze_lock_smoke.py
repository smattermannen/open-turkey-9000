"""GUI smoke test for locks on a scheduled freeze (§3.8, R-FREEZE-2,
R-LOCK-12): pick a schedule in the list, Lock… with a timer and a
password lock (confirmed), the list says it is locked, Remove is refused,
Unlock… asks for the password. Builds kc9k into a scratch folder, a wrapper
with KC9K_DB = scratch file, loads gui/kc9k-gui.py as a module with CLI
replaced, another application ID, never shows the main window. Needs an X
display; run as a normal user.

    xvfb-run -a python3 ci/gui_freeze_lock_smoke.py
"""
import datetime, importlib.util, json, os, subprocess, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a, stdin=""):
    return subprocess.run([W, *a], capture_output=True, text=True, input=stdin)


day = (datetime.date.today() + datetime.timedelta(days=3)).strftime("%a").lower()
r = cli("freeze", "schedule", "add", "--days", day, "--from", "03:00", "--to", "03:30", "--yes")
assert r.returncode == 0, r.stderr

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kfreezelock"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs, asked = [], [], []
win.message = lambda text, *a, **k: msgs.append(text)
win.confirm_lock = lambda locks, *a, **k: (asked.append(locks), True)[1]
win.ask_text = lambda *a, **k: "partner"


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
    for _ in range(20):
        GLib.MainContext.default().iteration(False); time.sleep(0.005)


Gtk.Dialog.run = lambda d: Gtk.ResponseType.CANCEL
win.get_child().show_all()
settle()
win.open_mode("Frozen Chicken"); settle()

msgs.clear()
win.lock_freeze_schedule(None)
check(msgs and "Pick a schedule" in msgs[-1], "nothing picked: says to pick one")

win.freeze_schedules.view.get_selection().select_path(Gtk.TreePath.new_first())
# The lock dialog answered: a timer lock, then a password lock.
win.lock_dialog = lambda *a, **k: ([(["--for", "2h"], "timer lock, 2 hours"),
                                    (["--password-stdin"], "password lock")], [], "partner")
win.choose_lock_order = lambda locks: locks
win.lock_freeze_schedule(None); settle()
check(asked and len(asked[-1]) == 2 and "freeze schedule" in asked[-1][0], "both locks confirmed: %s" % asked)
state = json.loads(cli("gui-state").stdout)
lock = state["freeze_schedule_locks"][0]
check(lock.get("lock_seconds", 0) > 0 and "password" in (lock.get("lock_order") or []), "locked: %s" % lock)
check("locked (" in state["freeze_schedules"][0], "the list says so: %s" % state["freeze_schedules"][0])
check(cli("freeze", "schedule", "remove", "1").returncode != 0, "a locked schedule can't be removed")

# Unlock…: the timer lock comes first, so nothing to type yet.
win.freeze_schedules.view.get_selection().select_path(Gtk.TreePath.new_first())
msgs.clear()
win.unlock_freeze_schedule(None); settle()
check(msgs and "ends by itself" in msgs[-1], "a timer lock first: it ends by itself: %s" % msgs)

print("all passed" if not fails else "FAILED: %s" % fails)
raise SystemExit(1 if fails else 0)
