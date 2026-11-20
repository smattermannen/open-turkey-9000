"""GUI smoke test for Extend timer… (§3.4, §4.6: extending a timer lock is
its own action): the button is greyed out without a timer lock, and on a
timer-locked block its dialog runs 'kc9k extend' after the lock
confirmation; the lock ends 30 minutes later and no lock is added. Builds
kc9k into a scratch folder, a wrapper with KC9K_DB = scratch file, loads
gui/kc9k-gui.py as a module with CLI replaced, another application ID,
never shows the main window. Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_extend_smoke.py
"""
import importlib.util, json, os, sqlite3, subprocess, tempfile, time
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


for a in (["block", "create", "plain"], ["block", "add", "plain", "reddit.com"], ["start", "plain"],
          ["block", "create", "timed"], ["block", "add", "timed", "youtube.com"],
          ["start", "timed", "--for", "1h", "--yes"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)


def lock_row(name):
    with sqlite3.connect(os.path.join(S, "t.db")) as db:
        return db.execute("""SELECT a.locked_until, a.lock_queue FROM active_blocks a JOIN blocks b
                             ON a.block_id = b.id WHERE b.name = ?""", (name,)).fetchone()


spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kextendsmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs, confirmed = [], [], []
win.message = lambda text, *a, **k: msgs.append(text)
win.confirm_lock = lambda locks, option=None: (confirmed.append(locks), True)[1]


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


settle()
win.open_block("plain"); settle()
check(not win.extend_button.get_sensitive(), "Extend timer… greyed out on a block with no timer lock")

win.open_block("timed"); settle()
check(win.extend_button.get_sensitive(), "Extend timer… offered on a timer-locked block")
before, queue = lock_row("timed")
real_run = Gtk.Dialog.run
Gtk.Dialog.run = lambda d: Gtk.ResponseType.OK  # the dialog's 30 minutes, as it opens
win.extend_button.clicked()
Gtk.Dialog.run = real_run
settle()
after, queue = lock_row("timed")
check(confirmed and "timer lock extended" in confirmed[-1][0], "the extension confirmed: %s" % confirmed)
check(after - before == 1800, "the timer lock ends 30 minutes later: %s -> %s" % (before, after))
check(queue in ("", None), "no second timer lock added: %r" % queue)
check(not any("Error" in m for m in msgs), "no errors shown: %s" % msgs)

print("all passed" if not fails else "FAILED: %d" % len(fails))
raise SystemExit(1 if fails else 0)
