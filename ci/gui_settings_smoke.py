"""GUI smoke test for the settings groups (SPEC §4.10): builds kc9k into a
scratch folder, points a wrapper at a scratch database (KC9K_DB: no layers
are ever applied), loads gui/kc9k-gui.py as a module with CLI replaced and
another application ID, and never shows the main window. Needs an X display.

    xvfb-run -a python3 ci/gui_settings_smoke.py

Strict mode and no-escape are only looked at, never switched.
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
def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)
def state():
    return json.loads(cli("gui-state").stdout)
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib
app = gui.App(); app.set_application_id("io.test.kc9ksettingssmoke"); app.register(None)
win = gui.Window(app)
def pump(cond, secs=15):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"
fails = []
def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what); ok or fails.append(what)
def settle():
    # Wait out a refresh already running (it may have read the state before
    # the last change), then read it afresh.
    pump(lambda: win.get_sensitive() and not win.busy)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
msgs = []
win.message = lambda text, *a, **k: msgs.append(text)
win.state = None; win.refresh(); pump(lambda: win.state is not None)

def titles(page):
    """Section headings of a page, top to bottom."""
    box = win.covers[page][0].get_child_by_name("page").get_child().get_child()
    out = []
    for child in box.get_children():
        first = child.get_children()[0] if isinstance(child, Gtk.Box) and child.get_children() else child
        if isinstance(first, Gtk.Label) and first.get_use_markup():
            out.append(first.get_text())
    return out

settings = titles("settings")
want = ["Settings", "Preferences", "No weakening during active locks", "Tamper protection", "Browsers",
        "Browser settings", "DNS", "Circadian rhythm", "Device protection", "Passwords",
        "Bypass prevention", "Project", "Behaviour settings"]
check([t for t in settings if t in want] == want, "Settings groups in §4.10 order: " + ", ".join(settings))
check("Statistics" not in settings, "no Statistics group in Settings")
device = titles("device")
check("Strict mode" not in device and "No-escape" not in device, "Device Blocks keeps only Frozen Chicken: " + ", ".join(device))
check("Recording" in titles("stats"), "statistics recording on the Statistics page")
check([n for n, *_ in gui.BROWSER_OPTIONS] == ["safesearch", "force-search", "web-only", "block-devtools", "no-private"],
      "browser settings in §4.10 order")
check("force-librewolf" not in win.browser_switches and "unsupported-browsers" not in win.browser_switches,
      "block unsupported browsers not among browser settings")
check("unsupported-browsers" in win.setting_locks and "strict" in win.setting_locks,
      "lock lines for block unsupported browsers and strict mode")

# Browsers list (§4.10): every supported browser, installed or not, Install
# shown only where missing. Never clicked: it would install for real.
ids = [b["id"] for b in win.state["browsers"]]
check(ids == ["librewolf", "chrome", "chromium"], "supported browsers listed: " + ", ".join(ids))
for b in win.state["browsers"]:
    status, install = win.browser_rows[b["id"]]
    check(status.get_text().startswith("Installed" if b["installed"] else "Not installed"),
          f"{b['name']} status: {status.get_text()}")
    check(install.get_visible() == (not b["installed"]), f"{b['name']} Install button only when missing")
check(gui.browser_status({"installed": True, "checkin_seconds": 5}) == "Installed. Extension working.", "fresh check-in")
check("not seen" in gui.browser_status({"installed": True, "checkin_seconds": -1}), "never checked in")
check(win.tamper_summary.get_text() == "All off.", "tamper summary all off: " + win.tamper_summary.get_text())

# The tamper protection window: Cancel changes nothing, OK applies.
real_run = Gtk.Dialog.run
def answer(response, tick=None):
    def fake(dialog):
        if tick:
            for key, value in tick.items():
                win.tamper_checks[key].set_active(value)
        return response
    Gtk.Dialog.run = fake
answer(Gtk.ResponseType.CANCEL, {"clock": True}); win.tamper_dialog(); settle()
def clock():
    return cli("tamper").stdout.splitlines()[0]
check(clock() == "clock off", "Cancel leaves the clock off")
answer(Gtk.ResponseType.OK, {"clock": True}); win.tamper_dialog(); settle()
check(clock() == "clock on", "OK turns the clock on")
check(win.tamper_summary.get_text().startswith("On: Clock"), "summary shows it: " + win.tamper_summary.get_text())
supported = any(b["installed"] for b in win.state["browsers"])
answer(Gtk.ResponseType.CANCEL); win.tamper_dialog()
check(win.tamper_checks["unsupported-browsers"].get_sensitive() == supported,
      f"block unsupported browsers usable only with a supported browser (installed: {supported})")
answer(Gtk.ResponseType.OK, {"clock": False}); win.tamper_dialog(); settle()
check(clock() == "clock off", "OK turns the clock off while nothing is locked")
check(not msgs, "no error so far: " + "; ".join(msgs))

# Locked: turning it off is refused by the command and the message shown.
answer(Gtk.ResponseType.OK, {"clock": True}); win.tamper_dialog(); settle()
r = cli("browser", "safesearch", "on", "--for", "1h", "--yes"); check(r.returncode == 0, "safesearch locked: " + r.stderr)
settle()
status, _unlock = win.setting_locks["safesearch"]
check(status.get_text().startswith("Locked: for"), "browser setting lock shown: " + status.get_text())
check(not win.browser_switches["safesearch"].get_sensitive(), "locked browser setting can't be switched")
answer(Gtk.ResponseType.OK, {"clock": False}); win.tamper_dialog(); settle()
check(clock() == "clock on", "clock stays on while something is locked")
check(any("locked" in m for m in msgs), "refusal shown: " + "; ".join(msgs))
# Block unsupported browsers: OK turns it on (the scratch database applies
# nothing); its lock line shows a lock; it can't be turned off while locked.
if supported:
    answer(Gtk.ResponseType.OK, {"unsupported-browsers": True}); win.tamper_dialog(); settle()
    check(cli("tamper").stdout.splitlines()[1].startswith("unsupported-browsers on"), "OK turns block unsupported browsers on")
    r = cli("tamper", "unsupported-browsers", "on", "--for", "1h", "--yes"); check(r.returncode == 0, "locked: " + r.stderr)
    settle()
    status, _unlock = win.setting_locks["unsupported-browsers"]
    check(status.get_text().startswith("Locked: for"), "its lock shown: " + status.get_text())
    answer(Gtk.ResponseType.CANCEL); win.tamper_dialog()
    check(not win.tamper_checks["unsupported-browsers"].get_sensitive(), "locked: can't be unticked")
Gtk.Dialog.run = real_run

# Statistics recording from the Statistics page.
win.stats_switch.set_active(False); settle()
check(state()["stats"] is False, "statistics off from the Statistics page")
check(not win.stats_switch.get_active(), "switch follows")
win.stats_switch.set_active(True); settle()
check(state()["stats"] is True, "statistics on again")

check(win.strict_switch.get_active() == win.state["strict"], "strict switch shows the state")
check(win.no_escape_switch.get_active() == win.state["no_escape"], "no-escape switch shows the state")

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
