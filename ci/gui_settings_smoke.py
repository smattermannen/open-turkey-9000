import json
"""GUI smoke test for the settings groups (SPEC §4.10): builds kc9k into a
scratch folder, points a wrapper at a scratch database (KC9K_DB: no layers
are ever applied), loads gui/kc9k-gui.py as a module with CLI replaced and
another application ID, and never shows the main window. Needs an X display.

    xvfb-run -a python3 ci/gui_settings_smoke.py

Strict mode and no-escape are only looked at, never switched.
"""
import importlib.util, json, os, re, subprocess, sys, tempfile, time
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
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")  # never the real ~/.config
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib
app = gui.App(); app.set_application_id("io.test.kc9ksettingssmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True  # no install dialog in a test
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
    box = win.covers[page][0].get_child_by_name("page")
    if isinstance(box, Gtk.ScrolledWindow):
        box = box.get_child().get_child()
    out = []
    for child in box.get_children():
        first = child.get_children()[0] if isinstance(child, Gtk.Box) and child.get_children() else child
        if isinstance(first, Gtk.Label) and first.get_use_markup():
            out.append(first.get_text())
    return out

settings = titles("settings")
want = ["Preferences", "No weakening during active locks", "Tamper protection", "Browsers",
        "Browser settings", "DNS", "Device protection", "Passwords",
        "Bypass prevention", "Project", "Behaviour settings"]
check([t for t in settings if t in want] == want, "Settings groups in §4.10 order: " + ", ".join(settings))
# Owner, 2026-11-16: the circadian rhythm has its own page, not a group here.
check("Circadian rhythm" not in settings, "no Circadian group in Settings")
check("Statistics" not in settings, "no Statistics group in Settings")
device = titles("device")
# §3.8: the allowed apps list of 'allowed apps only' freezes.
win.freeze_apps.entry.set_text("libreoffice")
win.freeze_apps._add(None)
settle()
check(win.state.get("freeze_apps") == ["libreoffice"], "allowed app added: %s" % win.state.get("freeze_apps"))
r = cli("freeze", "apps", "remove", "libreoffice"); settle()

check("Strict mode" not in device and "No-escape" not in device, "Device Blocks keeps only Frozen Chicken: " + ", ".join(device))
check("Recording" in titles("stats"), "statistics recording on the Statistics page")
check([n for n, *_ in gui.BROWSER_OPTIONS] == ["safesearch-google", "safesearch-bing", "safesearch-duckduckgo", "safesearch-youtube",
                                               "force-search", "web-only", "block-devtools", "no-private"],
      "browser settings in §4.10 order")
check("force-librewolf" not in win.browser_switches and "unsupported-browsers" not in win.browser_switches,
      "block unsupported browsers not among browser settings")
check("unsupported-browsers" in win.setting_locks and "strict" in win.setting_locks,
      "lock lines for block unsupported browsers and strict mode")

# Browsers list (§4.10): every supported browser, installed or not, Install
# shown only where missing. Never clicked: it would install for real.
ids = [b["id"] for b in win.state["browsers"]]
check(ids == ["librewolf", "chrome", "chromium", "edge", "firefox"], "supported browsers listed: " + ", ".join(ids))
for b in win.state["browsers"]:
    status, install = win.browser_rows[b["id"]]
    check(status.get_text().startswith("Installed" if b["installed"] or b.get("found") else "Not installed"),
          f"{b['name']} status: {status.get_text()}")
    check(install.get_visible() == (not b["installed"] and not b.get("waiting")),
          f"{b['name']} Install button only when missing and ready")
# 2.12: Edge and Firefox wait (the listing, Mozilla's signature).
waits = {b["id"]: b.get("waiting") for b in win.state["browsers"]}
check(waits.get("edge") == "chrome-web-store" and waits.get("firefox") in ("mozilla-signature", None),
      f"Edge and Firefox wait: {waits}")
check("Mozilla's signature" in gui.browser_status({"found": True, "waiting": "mozilla-signature"}),
      "a found Firefox says what it waits for")
check(gui.browser_status({"installed": True, "checkin_seconds": 5}) == "Installed. Extension working.", "fresh check-in")
check("not seen" in gui.browser_status({"installed": True, "checkin_seconds": -1}), "never checked in")
# A fresh setup: only what KC9K always did is on (§4.10, 2026-11-21 By Opus).
check(win.tamper_summary.get_text() == "On: Close browsers if the extension stops; Block inactive tabs; Block embedded content",
      "tamper summary of a fresh setup: " + win.tamper_summary.get_text())

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
r = cli("browser", "safesearch-bing", "on", "--for", "1h", "--yes"); check(r.returncode == 0, "safesearch-bing locked: " + r.stderr)
settle()
status, _unlock = win.setting_locks["safesearch-bing"]
check(status.get_text().startswith("Locked: for"), "browser setting lock shown: " + status.get_text())
check(not win.browser_switches["safesearch-bing"].get_sensitive(), "locked browser setting can't be switched")
check(win.browser_switches["safesearch-google"].get_sensitive(), "another engine's SafeSearch keeps its own switch")
answer(Gtk.ResponseType.OK, {"clock": False}); win.tamper_dialog(); settle()
check(clock() == "clock on", "clock stays on while something is locked")
check(any("locked" in m for m in msgs), "refusal shown: " + "; ".join(msgs))
# Block unsupported browsers: OK turns it on (the scratch database applies
# nothing); its lock line shows a lock; it can't be turned off while locked.
if supported:
    answer(Gtk.ResponseType.OK, {"unsupported-browsers": True}); win.tamper_dialog(); settle()
    check(any(l.startswith("unsupported-browsers on") for l in cli("tamper").stdout.splitlines()), "OK turns block unsupported browsers on")
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

# §3.4 re-lock on a setting lock: shown while locked, and after unlocking
# the line says when the locks are on again; the switch can turn it off.
r = subprocess.run([W, "browser", "web-only", "on", "--password-stdin", "--relock", "30m", "--yes"],
                   input="pw\n", capture_output=True, text=True)
check(r.returncode == 0, "web-only locked with re-lock: " + r.stderr)
settle()
status, _unlock = win.setting_locks["web-only"]
check("re-lock 30 min" in status.get_text() or "re-lock 30m" in status.get_text(), "re-lock shown: " + status.get_text())
r = subprocess.run([W, "browser", "unlock", "web-only"], input="pw\n", capture_output=True, text=True)
check(r.returncode == 0, "unlocked: " + r.stderr)
settle()
check("locks on again in" in status.get_text(), "lifted lock shown: " + status.get_text())
check(win.browser_switches["web-only"].get_sensitive(), "lifted: switch usable")

# §4.10 notifications from the Settings page.
win.notice_checks["window-end"].set_active(False); settle()
check("window-end" in (state()["notifications"].get("off") or []), "a notification turned off from its box")
win.notice_advance.set_text("2,5"); win.apply_notice_advance(None); settle()
check(state()["notifications"]["advance"] == [2, 5], "advance minutes saved")
check(not win.notice_checks["window-end"].get_active() and win.notice_checks["timer-end"].get_active(), "boxes show the state")

# §3.8 strict mode's exempt folders from Device protection.
exempt = tempfile.mkdtemp(prefix="kc9k-exempt-")
win.strict_exempt.on_add([exempt]); settle()
check(state()["strict_exempt"] == [exempt], "exempt folder added from the list")
win.strict_exempt.on_remove(exempt, 1); settle()
check(state()["strict_exempt"] == [], "exempt folder removed")

# §4.10 preferences: open at login writes an autostart entry; first day.
win.toggle_open_at_login(None, True)
check(os.path.exists(gui.LOGIN_DESKTOP) and gui.LOGIN_DESKTOP.startswith(S), "open at login entry written (in the test folder)")
win.toggle_open_at_login(None, False)
check(not os.path.exists(gui.LOGIN_DESKTOP), "open at login entry removed")
gui.FIRST_DAY = 6
check(gui.column_day(0) == 6 and win.calendar.weekday_bit(0) == 1, "Sunday first: column 0 is Sunday")
gui.FIRST_DAY = 0

# §4.10 extension install dialog: "Show browsers" goes to Settings.
real_msg_run = Gtk.Dialog.run
Gtk.Dialog.run = lambda dialog: Gtk.ResponseType.OK
win.state["browsers"] = [{"id": "librewolf", "installed": True}]
shown = []
real_switch = win.stack.set_visible_child_name
win.stack.set_visible_child_name = shown.append  # the window is never shown, so record
win.extension_dialog()
win.stack.set_visible_child_name = real_switch
check(shown == ["settings"], "the dialog shows the browsers: %s" % shown)
Gtk.Dialog.run = real_msg_run

# §4.9 times blocked on the Statistics page (counts, not time).
win.stats_rows = win.stats_page_rows = [{"day": __import__("datetime").date.today().isoformat(), "kind": "blocked", "name": "reddit.com", "seconds": 3},
                  {"day": __import__("datetime").date.today().isoformat(), "kind": "site", "name": "wiki.org", "seconds": 600}]
win.kind_blocked.set_active(True); win.update_stats()
check(win.tile_total.get_children()[1].get_text() == "3×" if len(win.tile_total.get_children()) > 1 else True, "blocked total as a count")
check(win.top_chart.fmt(3) == "3×", "top chart counts")
win.kind_sites.set_active(True); win.update_stats()
check(win.top_chart.fmt(60) != "60×", "websites as time")

# §4.10 darkness slider: with the dark theme, the background grey to black.
app.prefs["theme"] = "dark"; win.darkness_scale.set_value(100)
check(getattr(app.theme, "level", None) == 0, "darkness 100 is black")
win.darkness_scale.set_value(0)
check(getattr(app.theme, "level", None) == 48, "darkness 0 is dark grey")
app.theme.apply("system")

# §4.10 clock preference.
win.clock_combo.set_active_id("12")
check(app.prefs.get("clock_24") is False and gui.show_clock(14 * 60 + 30) == "2:30 PM"
      and gui.show_clock(0) == "12:00 AM" and gui.show_hour(12) == "12 PM", "12-hour clock")
win.clock_combo.set_active_id("24")
check(gui.show_clock(14 * 60 + 30) == "14:30" and gui.show_clock(1440, True) == "24:00", "24-hour clock")

# §4.10 tray: the icon comes and goes with the preference; closing hides.
win.tray_switch.set_active(True)
check(app.tray is not None and app.prefs.get("tray") is True, "tray on")
settle()
check(app.tray is None or app.tray.status.get_label() in ("No block is on",) or app.tray.status.get_label().startswith("On:"),
      "tray status: %s" % (app.tray and app.tray.status.get_label()))
win.tray_switch.set_active(False)
check(app.tray is None and app.prefs.get("tray") is False, "tray off")

# §4.9 private windows left out of statistics.
win.stats_private_switch.set_active(True)
settle()
check(win.state.get("stats_skip_private") is True, "stats private skip from the switch")
win.stats_private_switch.set_active(False)
settle()

# §4.9 statistics per user: the chooser lists the users with statistics.
win.apply_stats(True, json.dumps({"enabled": True, "since": "2026-11-01", "rows": [], "users": ["anna", "ben"]}))
check([r[0] for r in win.stats_user.get_model()] == ["All users", "anna", "ben"], "stats user chooser")

check(win.strict_switch.get_active() == win.state["strict"], "strict switch shows the state")
check(win.no_escape_switch.get_active() == win.state["no_escape"], "no-escape switch shows the state")
check("reset_guard" in win.state and win.reset_guard_switch.get_active() == win.state["reset_guard"], "reset guard switch shows the state")

# §3.4 in the setting lock dialog: a timer and a range lock, unlock order asked.
asked = []
def lock_dialogs(dialog):
    asked.append(dialog.get_title() or "")
    if dialog.get_title() and dialog.get_title().startswith("Lock:"):
        picker = win.lock_picker
        picker.timer.mode_buttons["for"].set_active(True)
        picker.timer.duration.set_text("5m")
        picker.range_on.set_active(True)
        for toggle in picker.range_days.values():
            toggle.set_active(True)
        picker.range_from.set_text("00:00"); picker.range_to.set_text("23:55")
    return Gtk.ResponseType.OK
Gtk.Dialog.run = lock_dialogs
win.lock_setting(None, "force-search", "Force search"); settle()
out = cli("browser").stdout
check("Unlock order" in asked and "force-search on" in out and "range" in out.lower(), "setting lock with range and order: %s %s" % (asked, out))

# §4.10 Device protection: the guided setup lists every step, in order,
# with Do it… on those not done; the firmware lock says the scratch
# database reaches no firmware (no firmware settings to change here).
seen = {}
def collect(widget, out):
    if isinstance(widget, Gtk.Label):
        out.append(widget.get_text())
    elif isinstance(widget, Gtk.Button):
        out.append("[" + widget.get_label() + "]")
    if isinstance(widget, Gtk.Container):
        for child in widget.get_children():
            collect(child, out)
def look(dialog):
    texts = []
    collect(dialog.get_content_area(), texts)
    if isinstance(dialog, Gtk.MessageDialog):
        texts.append(dialog.get_property("secondary-text") or "")
    seen[dialog.get_title() or dialog.get_property("text")] = texts
    return Gtk.ResponseType.CLOSE
Gtk.Dialog.run = look
win.show_guided_setup(None)
guided = " | ".join(seen.get("Guided setup", []))
steps = [t for t in seen.get("Guided setup", []) if re.match(r"^\d+\. ", t)]
check(steps and steps[0].startswith("1. Settings password") and steps[-1].endswith("No-escape"),
      "guided setup steps in order: " + "; ".join(steps))
check("[Do it…]" in guided and "Firmware lock" in guided, "guided setup offers Do it…")
win.show_firmware_lock(None)
check("can't be changed from Linux" in " ".join(seen.get("Firmware lock", [])), "firmware lock dialog: " + str(seen.get("Firmware lock")))
check(all(win.guided_action(i) for i in ("settings-password", "strict", "firmware", "no-escape", "tamper")), "each step has its action")
Gtk.Dialog.run = real_run

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
