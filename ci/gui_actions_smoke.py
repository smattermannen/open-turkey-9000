"""GUI smoke test for the Mode action and custom actions (§3.9, R-MODE-4,
R-CUSTOM-1/2; owner 2026-11-17): a list's actions take "Run a mode,
locked" (which mode, how long) and "Custom action" (what to open or run);
the apps' actions too; a mode's view says what it opens when it becomes
enabled; a mode run by a Mode action shows locked, its switch greyed out;
ladder steps describe both. Builds kc9k into a scratch folder, a wrapper
with KC9K_DB = scratch file, loads gui/kc9k-gui.py as a module with CLI
replaced, another application ID, never shows the main window. Needs an X
display; run as a normal user.

    xvfb-run -a python3 ci/gui_actions_smoke.py
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


for a in (["block", "create", "b"], ["block", "add", "b", "reddit.com"],
          ["block", "create", "w"], ["block", "add", "w", "w.com"], ["mode", "create", "Work"], ["mode", "add", "Work", "w"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kactionssmoke"); app.register(None)
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


def mode(name):
    return next(m for m in state()["modes"] if m["name"] == name)


Gtk.Dialog.run = lambda d: Gtk.ResponseType.CANCEL
win.acknowledge_no_confirmation = lambda what: True
win.get_child().show_all()
settle()
labels = dict(gui.SITE_ACTIONS)
check(labels.get("mode") == "Run a mode, locked" and labels.get("custom") == "Custom action", "lists offer both actions")
check(win.mode_choices()[:1] == ["Monk mode"] and "Work" in win.mode_choices() and "Frozen Chicken" not in win.mode_choices(),
      "the modes a Mode action runs: %s" % win.mode_choices())

# A list's actions (in the block's draft, then saved).
win.open_block("b"); settle()
card = next(c for c in win.list_cards.values() if c.data["name"] == "Blocked")
win.ask_mode_run = lambda *a, **k: ("Work", 90)
card.ask_settings("mode"); settle(); settle()
win.ask_custom = lambda *a, **k: ["open https://example.org/notes", "run notify-send hi"]
card = next(c for c in win.list_cards.values() if c.data["name"] == "Blocked")
card.ask_settings("custom"); settle(); settle()
win.finish_draft("save"); settle()
actions = {a["action"]: a for a in next(l for l in block("b")["lists"] if l["name"] == "Blocked")["actions"]}
check(actions.get("mode", {}).get("param") == "Work" and actions["mode"].get("minutes") == 90,
      "Run a mode, locked: %s" % actions.get("mode"))
check("custom" in actions and "notify-send hi" in actions["custom"].get("param", ""), "custom action: %s" % actions.get("custom"))
check(gui.ListCard.settings_text("mode", actions["mode"]) == "Runs Work for 1 h 30 min, locked",
      "it says what it does: %s" % gui.ListCard.settings_text("mode", actions["mode"]))
check(gui.ListCard.settings_text("custom", actions["custom"]) == "open https://example.org/notes; run notify-send hi",
      "and the custom one: %s" % gui.ListCard.settings_text("custom", actions["custom"]))

# The apps' actions.
win.open_block("b"); settle()
apps = win.app_triggers
apps.mode_combo.set_active(win.mode_choices().index("Work"))
apps.checks["mode"].set_active(True); settle(); settle()
win.ask_custom = lambda *a, **k: ["open ~/Notes"]
apps.checks["custom"].set_active(True); settle(); settle()
win.finish_draft("save"); settle()
triggers = {a["name"]: a for a in block("b")["triggers"]["apps"]}
check(triggers.get("mode", {}).get("param") == "Work" and "~/Notes" in triggers.get("custom", {}).get("param", ""),
      "the apps' actions: %s" % triggers)

# A mode's custom action, and a Mode action's run.
win.open_mode("Work"); settle()
check(win.mode_custom_section.get_visible(), "the mode's view says what it opens when it becomes enabled")
win.set_mode_custom(["open https://example.org/work"]); settle(); settle()
check(mode("Work")["custom"] == ["open https://example.org/work"], "set: %s" % mode("Work")["custom"])
r = cli("mode", "run", "Work", "--for", "1h", "--yes")
check(r.returncode == 0, "mode run: %s" % r.stderr.strip())
win.blocks_stack.set_visible_child_name("cards"); settle()
texts = [w.get_text() for w in widgets(win.mode_cards) if isinstance(w, Gtk.Label)]
check(any(t.startswith("Running, locked") for t in texts), "the card says it runs, locked: %s" % texts)
win.open_mode("Work"); settle()
check(not win.mode_switch.get_sensitive() and "Mode action" in win.mode_status.get_text(),
      "its switch is greyed out: %s" % win.mode_status.get_text())

# Ladder steps describe both.
step = {"count": 3, "minutes": 10, "actions": [{"action": "mode", "param": "Work", "minutes": 60},
                                                {"action": "custom", "param": '[{"open": "https://example.org"}]'}]}
words = gui.describe_step(step)
check("run Work, locked, for 1 h" in words and "open https://example.org" in words, "a step: %s" % words)
check(not [m for m in msgs if m.startswith("Error")], "no errors: %s" % msgs)
win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
raise SystemExit(1 if fails else 0)
