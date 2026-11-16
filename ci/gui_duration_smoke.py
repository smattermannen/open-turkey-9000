"""GUI smoke test for durations and timer locks (SPEC R-GUI-1, R-GUI-2):
builds kc9k into a scratch folder, points a wrapper at a scratch database
(KC9K_DB: no layers are ever applied, monk mode cuts nothing), loads
gui/kc9k-gui.py as a module with CLI replaced and another application ID,
and never shows the main window. Needs an X display.

    xvfb-run -a python3 ci/gui_duration_smoke.py
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


# Typed lengths ------------------------------------------------------------
P = gui.parse_duration
for text, want in (("2d 4h", 52 * 3600), ("2 days and 4 hours", 52 * 3600), ("1w", 604800),
                   ("90m", 5400), ("1.5h", 5400), ("1,5h", 5400), ("300", 18000), ("100000m", 6000000),
                   ("100000", 6000000), ("1h30m", 5400), ("", 0), ("0", 0), ("45 min", 2700),
                   ("2 weeks 3 days", 17 * 86400), ("10s", 10), ("  3d  ", 3 * 86400)):
    check(P(text) == want, f"parse {text!r} = {P(text)} (want {want})")
for text in ("abc", "2 4h", "5x", "-5m", "h", "1e5m", "3d -2h", "5 m 3", "1..5h"):
    check(P(text) is None, f"refuse {text!r}: {P(text)}")
check(P("90", bare=1) == 90, "bare seconds")
check(gui.duration_words(6000000) == "69 days 10 hours 40 minutes", gui.duration_words(6000000))
check(gui.duration_words(3600) == "1 hour" and gui.duration_words(86400 + 60) == "1 day 1 minute",
      "singular words")
check(gui.duration_text(52 * 3600 + 1800) == "2d 4h 30m", gui.duration_text(52 * 3600 + 1800))
check(gui.parse_clock("6:30 pm") == 18 * 60 + 30 and gui.parse_clock("18.00") == 1080
      and gui.parse_clock("24:00") is None and gui.parse_clock("13pm") is None, "clock parsing")

# R-GUI-1: never changed silently. ---------------------------------------
f = gui.DurationField(3600, minimum=60, maximum=40 * 86400)
f.set_text("100000m")
check(f.entry.get_text() == "100000m", "the text stays as typed")
check(f.value is None and "At most 40 days" in f.problem, "out of range says why: %s" % f.problem)
check("error" in f.entry.get_style_context().list_classes(), "the field shows red")
f.set_text("2d 4h")
check(f.value == 52 * 3600 and f.note.get_text() == "2 days 4 hours", "reads back: " + f.note.get_text())
check("error" not in f.entry.get_style_context().list_classes(), "red goes away")
f.set_text("junk")
check(f.value is None and "Type a length" in f.problem, "junk explained: %s" % f.problem)
u = gui.DurationField(0, minimum=60, zero="No re-lock")
check(u.value == 0 and u.entry.get_placeholder_text() == "No re-lock" and not u.note.get_visible(),
      "empty means the zero text, shown in the box")
u.set_text("30s")
check(u.value is None and "At least 1 minute" in u.problem, "below the minimum: %s" % u.problem)
n = gui.NumberField(300, 1, 5000)
n.set_text("6000")
check(n.value is None and n.entry.get_text() == "6000" and "At most 5000" in n.problem, "number not clamped")
b = Gtk.Button()
gui.guard_button(b, f)
f.set_text("x")
check(not b.get_sensitive(), "the button waits while a field is wrong")
f.set_text("1h")
check(b.get_sensitive(), "and comes back")

# R-GUI-2: the calendar. --------------------------------------------------
t = gui.TimerLockField()
check(t.flags() == [] and t.is_valid(), "no timer lock by default")
t.mode_buttons["for"].set_active(True)
t.duration.set_text("100000m")
check(t.flags() == ["--for", "100000m"], "for: %s" % t.flags())
t.set_until(datetime.datetime(2099, 12, 24, 18, 0))
check(t.flags() == ["--until", "2099-12-24 18:00"], "until: %s" % t.flags())
check("24 December 2099" in t.until_note.get_text(), "until read back: " + t.until_note.get_text())
t.year.set_value(2100)
check(t.flags() == ["--until", "2100-12-24 18:00"], "year jump: %s" % t.flags())
t.set_until(datetime.datetime(2020, 1, 1, 10, 0))
check(not t.is_valid() and "passed" in t.problem(), "a passed date is refused")
t.set_until(datetime.datetime(2099, 12, 24, 18, 0))
t.time_entry.set_text("25:00")
check(not t.is_valid(), "a wrong time is refused")

# The dialogs, end to end -------------------------------------------------
for a in (["block", "create", "b"], ["block", "add", "b", "reddit.com"],
          ["block", "create", "c"], ["block", "add", "c", "example.org"]):
    r = cli(*a); assert r.returncode == 0, r.stderr
app = gui.App(); app.set_application_id("io.test.kc9kdurationsmoke"); app.register(None)
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
win.confirm_lock = lambda locks, option=None: confirmed.append(locks) or True
real_run = Gtk.Dialog.run


def answering(setup):
    def fake(dialog):
        setup(dialog)
        return Gtk.ResponseType.OK
    Gtk.Dialog.run = fake


def start_for(dialog):
    win.lock_picker.timer.mode_buttons["for"].set_active(True)
    win.lock_picker.timer.duration.set_text("100000m")


answering(start_for)
win.selected = "b"; win.start_block(None); settle()
blk = next(x for x in state()["blocks"] if x["name"] == "b")
check(blk["active"] and abs(blk["lock_seconds"] - 6000000) < 300, "block b timer-locked 100000 min: %s" % blk["lock_seconds"])
check(confirmed and "69 days 10 hours 40 minutes" in confirmed[-1][0], "confirmation in days: %s" % confirmed[-1:])


def start_until(dialog):
    win.lock_picker.timer.set_until(datetime.datetime(2099, 12, 24, 18, 0))


answering(start_until)
win.selected = "c"; win.start_block(None); settle()
out = cli("status").stdout
check("2099-12-24" in out, "block c locked until 2099: " + " ".join(l for l in out.splitlines() if "c" in l)[:200])


def setting_until(dialog):
    if dialog.get_title().startswith("Lock:"):
        win.lock_picker.timer.set_until(datetime.datetime(2099, 12, 24, 18, 0))


answering(setting_until)
win.lock_setting(None, "web-only", "Web results only"); settle()
check("2099-12-24" in cli("browser").stdout, "setting locked until 2099")

# A wrong field keeps the dialog's button off: the picker reports it.
def wrong_password(dialog):
    p = win.lock_picker
    p.password.set_text("one"); p.password_again.set_text("two")
    check(not p.is_valid() and "differ" in p.problem(), "two different passwords refused")
    p.password_again.set_text("one")
    p.relock.set_text("45 days")
    check(not p.is_valid() and "At most 30 days" in p.problem(), "re-lock limit explained: %s" % p.problem())
    p.relock.set_text("30m")


answering(wrong_password)
cli("block", "create", "d"); cli("block", "add", "d", "example.net")
win.selected = "d"; win.start_block(None); settle()
check(cli("block", "info", "d").returncode == 0, "d started with a password lock")

# Monk mode from the button: confirmed (R-LOCK-12), --yes passed, runs.
Gtk.Dialog.run = real_run
win.ask_duration = lambda *a, **k: 3600
confirmed.clear()
win.monk_mode(None); settle()
check(confirmed and "1 hour" in confirmed[-1][0], "monk mode confirmed: %s" % confirmed[-1:])
check("on" in cli("monk").stdout.lower(), "monk mode on: " + cli("monk").stdout)

# The ladder's timer lock until a date.
def ladder(dialog):
    cards, quiet, lock = win.nepsis_fields
    quiet.set_text("100000m")
    check(not quiet.is_valid(), "quiet time over 30 days refused")
    quiet.set_text("2h")
    lock.set_until(datetime.datetime(2099, 12, 24, 18, 0))


answering(ladder)
win.change_global_nepsis(None); settle()
check("2099" in cli("bypass").stdout, "global ladder locked until 2099: " + cli("bypass").stdout[:300])
Gtk.Dialog.run = real_run

# §3.6 reward kept for a time the user picks, with no upper limit.
cli("block", "create", "e"); cli("block", "add", "e", "example.com")
settle()
win.selected = "e"
win.reward_keep.set_active_id("time")
check(win.reward_keep_time.get_visible(), "the kept-for field shows with 'for a time'")
win.reward_keep_time.set_text("400d")
check(win.reward_keep_time.is_valid(), "400 days is fine: no upper limit")
win.set_allowance(None); settle()
e = next(x for x in state()["blocks"] if x["name"] == "e")
check(e.get("break_keep") == str(400 * 86400), "reward kept 400 days: %s" % e.get("break_keep"))
win.reward_keep_time.set_text("30m")
check(not win.reward_keep_time.is_valid(), "under an hour is refused where it is typed")
check(not msgs, "no errors shown: " + "; ".join(msgs))

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
