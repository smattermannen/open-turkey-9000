"""GUI smoke test for automation (§3.6, owner 2026-11-16): the Schedule &
places tab says what switches the block, the Disabled/Enabled by default
choice and Either/Both (shown with windows and places) run 'kc9k
automation', a new window follows the default, a refused change on a
locked block goes back. Clicks the real buttons. Builds kc9k into a scratch
folder, a wrapper with KC9K_DB = scratch file and a fake nmcli, loads
gui/kc9k-gui.py as a module with CLI replaced, another application ID,
never shows the main window. Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_automation_smoke.py
"""
import importlib.util, json, os, subprocess, tempfile, time
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


for a in (["block", "create", "b"], ["block", "add", "b", "reddit.com"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kautomationsmoke"); app.register(None)
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
    for _ in range(40):
        GLib.MainContext.default().iteration(False); time.sleep(0.005)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
    win.selected = "b"; win.show_block()


def state():
    return next(b for b in json.loads(cli("gui-state").stdout)["blocks"] if b["name"] == "b")


def automation():
    return cli("automation", "b").stdout.strip()


settle()
check(win.auto_summary.get_text().startswith("Continuous"), "a new block is continuous: " + win.auto_summary.get_text())
check(not win.auto_need_row.get_visible(), "no both/either without windows and places")

# A place, then Enabled by default: arriving there turns it off.
r = cli("place", "add", "home"); assert r.returncode == 0, r.stderr
r = cli("place", "rule", "b", "home", "on"); assert r.returncode == 0, r.stderr
settle()
check("its places turn it on" in win.auto_summary.get_text(), "places turn it on: " + win.auto_summary.get_text())
win.auto_buttons[("default", "enabled")].set_active(True); settle()
check(state()["automation"]["default_on"] and "turn it off" in automation(), "Enabled by default clicked: " + automation())
check("its places turn it off" in win.auto_summary.get_text(), "the summary follows: " + win.auto_summary.get_text())

# A window drawn on the grid follows the default: it turns the block off.
win.create_window_from_calendar(0, 9 * 60, 10 * 60); settle()
windows = state()["schedule_windows"]
check(len(windows) == 1 and windows[0]["allow"], "the new window turns it off too: %s; %s" % (windows, msgs))
both = win.auto_buttons[("needs", "both")]
check(all(w.get_visible() for w in (win.auto_need_row, both, both.get_parent())),
      "both/either shows with windows and places, buttons and all")
win.auto_buttons[("needs", "both")].set_active(True); settle()
check(state()["automation"]["need_both"], "Both at once clicked")

# Locked: Disabled by default would flip its times: refused, and the choice goes back.
r = cli("start", "b", "--for", "1h", "--yes"); assert r.returncode == 0, r.stderr
settle()
msgs.clear()
win.auto_buttons[("default", "disabled")].set_active(True)
pump(lambda: bool(msgs), secs=20)
settle()
check(any("locked" in m for m in msgs), "refused while locked: %s" % msgs)
check(win.auto_buttons[("default", "enabled")].get_active() and state()["automation"]["default_on"],
      "the choice went back to Enabled by default")

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
raise SystemExit(1 if fails else 0)
