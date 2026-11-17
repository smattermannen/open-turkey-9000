"""GUI smoke test for modes, schedules and places (§3.2 Mode, §3.6
Schedule, §3.8 Monk mode, §4.2, §4.5; owner 2026-11-17): the Blocks page
shows the modes above the blocks, monk mode always; a block put into a mode
(as a drop onto its card does) leaves the block list for the mode's view,
gets the mode's automation and can't change it on its own; the mode's
editor adds windows and applies a schedule for all its blocks; the
Schedules page makes a schedule and changes its windows; the Places page
saves a place (fake nmcli) and shows "You are here"; Apply schedule on a
block; monk mode switched on runs in its window. Builds kc9k into a scratch
folder, a wrapper with KC9K_DB = scratch file, loads gui/kc9k-gui.py as a
module with CLI replaced, another application ID, never shows the main
window. Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_modes_smoke.py
"""
import datetime, importlib.util, json, os, subprocess, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
BIN = os.path.join(S, "bin")
WIFI = os.path.join(S, "wifi.txt")
os.makedirs(BIN)
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(os.path.join(BIN, "nmcli"), "w") as f:
    f.write(f'#!/bin/sh\ncat "{WIFI}"\n')
os.chmod(os.path.join(BIN, "nmcli"), 0o755)
with open(WIFI, "w") as f:
    f.write("AA\\:AA\\:AA\\:AA\\:AA\\:01:home1\nAA\\:AA\\:AA\\:AA\\:AA\\:02:home2\n")
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nPATH="{BIN}:$PATH" KC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)


def state():
    return json.loads(cli("gui-state").stdout)


for a in (["block", "create", "social"], ["block", "add", "social", "reddit.com"],
          ["block", "create", "games"], ["block", "add", "games", "steam.com"],
          ["block", "create", "news"], ["block", "add", "news", "cnn.com"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kmodessmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs, asked = [], [], []
win.message = lambda text, *a, **k: msgs.append(text)
win.confirm = lambda text, *a, **k: (asked.append(text), True)[1]
win.confirm_lock = lambda locks, *a, **k: (asked.append(" ".join(locks)), True)[1]
answers = []
win.ask_text = lambda *a, **k: answers.pop(0) if answers else ""


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


def widgets(root):
    out = [root]
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += widgets(child)
    return out


def names(box):
    return [b.get_label() for b in widgets(box) if isinstance(b, Gtk.Button)
            and b.get_style_context().has_class("kc9k-card-name")]


def block(name):
    return next(b for b in state()["blocks"] if b["name"] == name)


def mode(name):
    return next(m for m in state()["modes"] if m["name"] == name)


Gtk.Dialog.run = lambda d: Gtk.ResponseType.CANCEL
win.get_child().show_all()
settle()
win.stack.set_visible_child_name("blocks"); settle()
check(names(win.mode_cards) == ["Monk mode"], "monk mode is always among the modes: %s" % names(win.mode_cards))
check(sorted(names(win.block_cards)) == ["games", "news", "social"], "the blocks below")

# New mode…, then a block dropped onto it.
answers[:] = ["Evening"]
win.create_mode(None); settle()
check(names(win.mode_cards) == ["Monk mode", "Evening"], "New mode… makes one: %s" % names(win.mode_cards))
check(win.blocks_stack.get_visible_child_name() == "mode" and win.mode_title.get_text() == "Evening",
      "and opens it")
win.move_to_mode("social", "Evening"); settle()
win.move_to_mode("games", "Evening"); settle()
check(block("social")["mode"] == "Evening" and block("games")["mode"] == "Evening", "dropped blocks are in it")
win.blocks_stack.set_visible_child_name("cards"); settle()
check(names(win.block_cards) == ["news"], "blocks in a mode leave the list: %s" % names(win.block_cards))
card_text = [w.get_label() for w in widgets(win.mode_cards) if isinstance(w, Gtk.Button)]
check(any("2 blocks: games, social" in (t or "") for t in card_text), "the mode's card names its blocks: %s" % card_text)

# The Overview's Start menu offers the mode too, and starts its blocks.
win.stack.set_visible_child_name("dashboard"); settle()
items = [i.get_label() for i in win.dash_start_menu.get_children() if isinstance(i, Gtk.MenuItem) and i.get_label()]
check("Mode: Evening (2 blocks)" in items, "the Overview offers the mode: %s" % items)
next(i for i in win.dash_start_menu.get_children() if i.get_label() == "Mode: Evening (2 blocks)").activate(); settle()
check(block("social")["active"] and block("games")["active"], "and starts its blocks")
cli("mode", "off", "Evening")
settle()

# The mode's view: its blocks, its automation for all of them.
win.open_mode("Evening"); settle()
check(sorted(names(win.mode_block_cards)) == ["games", "social"], "its view shows its blocks")
# Owner 2026-11-17 (later): no ⋯ menu on the cards; Add blocks… makes a new
# block in the mode; Take out… takes one out from the keyboard.
check(not any(isinstance(w, Gtk.MenuButton) for w in widgets(win.mode_block_cards) + widgets(win.block_cards)),
      "block cards have no ⋯ menu")
win.add_blocks_to_mode(None, answer=(win.CREATE_NEW, "reading")); settle()
check(block("reading")["mode"] == "Evening" and "reading" in names(win.mode_block_cards),
      "Create new block… makes one in the mode")
real_choose = win.choose
win.choose = lambda title, detail, names_: "reading" if "reading" in names_ else None
win.take_blocks_out_of_mode(None); settle()
win.choose = real_choose
check(not block("reading")["mode"], "Take out… takes it out")
cli("block", "remove", "reading")
win.mode_editor.add_window("daily", "20:00", "23:00"); settle()
check(all(len(block(b)["schedule_windows"]) == 1 for b in ("social", "games")), "a window for all its blocks")
check(len(win.mode_editor.calendar.windows) == 1, "drawn on the mode's grid")
r = cli("schedule", "add", "social", "--days", "mon", "--from", "08:00", "--to", "09:00")
check(r.returncode != 0 and "in the mode" in r.stderr, "a block in a mode isn't changed on its own: %s" % r.stderr.strip())

# The Schedules page: a schedule made and given a window, applied to the mode.
win.stack.set_visible_child_name("schedules"); settle()
answers[:] = ["Weekend"]
win.create_schedule(None); settle()
check([sc["name"] for sc in state()["schedules"]] == ["Weekend"], "New schedule… makes one")
check(win.shown_schedule == "Weekend" and win.schedule_editor_box.get_visible(), "and shows its editor")
win.schedule_editor.add_window("sat,sun", "00:00", "24:00"); settle()
check(state()["schedules"][0]["windows"][0]["days"] == 0b1000001, "a window on the schedule")
win.choose = lambda *a: "Weekend"
win.open_mode("Evening"); settle()
win.mode_editor.apply_schedule(None); settle()
check(mode("Evening")["schedules"] == ["Weekend"], "Apply schedule… on the mode")
check(len(block("games")["schedule_windows"]) == 3 and
      any(w.get("source") == "Weekend" for w in block("games")["schedule_windows"]),
      "its blocks get the schedule's windows, clipped: %s" % block("games")["schedule_windows"])
win.stack.set_visible_child_name("schedules"); settle()
used = [w.get_text() for w in widgets(win.schedule_cards) if isinstance(w, Gtk.Label)]
check(any("Used by mode Evening" in t for t in used), "the schedule says what uses it: %s" % used)

# A block in a mode: its own automation greyed out, the mode named.
win.open_block("social", "Schedule & places"); settle()
check(win.block_mode_bar.get_visible() and "Evening" in win.block_mode_label.get_text(), "the block names its mode")
check(not any(w.get_sensitive() for w in win.block_auto_widgets), "and its automation is greyed out")
win.take_out_of_mode("social"); settle()
check(not block("social")["mode"] and block("social")["applied_schedules"] == ["Weekend"],
      "taken out, it keeps the mode's automation")
win.open_block("social", "Schedule & places"); settle()
check(all(w.get_sensitive() for w in win.block_auto_widgets), "and is changed on itself again")

# Apply schedule on a block of its own.
win.open_block("news", "Schedule & places"); settle()
win.apply_schedule_to_block(None); settle()
check(block("news")["applied_schedules"] == ["Weekend"], "Apply schedule… on a block")
chips = [w.get_text() for w in widgets(win.block_applied) if isinstance(w, Gtk.Label)]
check(chips == ["Weekend"], "its schedules shown: %s" % chips)

# The Places page: save where I am now, "You are here".
win.stack.set_visible_child_name("places"); settle()
answers[:] = ["home"]
win.save_place_alone(); settle()
texts = [w.get_text() for w in widgets(win.place_cards) if isinstance(w, Gtk.Label)]
check("home" in texts and "You are here" in texts, "a saved place, and you are here: %s" % texts)
win.open_mode("Evening"); settle()
win.mode_editor.place_combo.set_active(0)
win.mode_editor.add_place(None); settle()
check(mode("Evening")["places"] == ["home"] and "home" in block("games")["places"], "a place for the mode's blocks")

# Monk mode: no switch without windows; a window covering today, switched
# on (confirmed), it runs.
win.blocks_stack.set_visible_child_name("cards"); settle()
monk = mode("Monk mode")
check(monk["continuous"] and not monk["active"], "monk mode starts with no automation")
win.open_mode("Monk mode"); settle()
check(win.mode_editors.get_visible_child() is win.monk_editor and not win.mode_blocks_section.get_visible(),
      "monk mode's view: no blocks, its own editor")
today = datetime.date.today().strftime("%a").lower()
win.monk_editor.add_window(today, "00:00", "24:00"); settle()
asked.clear()
win.toggle_mode_card(None, True, mode("Monk mode")); settle()
check(asked and "monk mode" in asked[-1], "switching it on asks first: %s" % asked)
monk = mode("Monk mode")
check(monk["active"] and monk["running"], "on, its window runs it: %s" % {k: monk[k] for k in ("active", "running")})
r = cli("mode", "off", "monk")
check(r.returncode != 0, "running, it can't be switched off")

print("messages:", msgs)
print("all passed" if not fails else "FAILED: %s" % fails)
raise SystemExit(1 if fails else 0)
