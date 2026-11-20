"""GUI smoke test for the block list (§4.2, owner 2026-11-16): the Blocks
page opens on one card per block with its name, an on/off switch,
Continuous or Automated block, its breaks and its lock state; the switch
starts (without locks) and stops the block, refused and put back while it
is locked; the parts open the block, its automation, its breaks and its
locks; Blocks in the sidebar comes back to the list. Clicks the real
widgets. Builds kc9k into a scratch folder, a wrapper with KC9K_DB =
scratch file, loads gui/kc9k-gui.py as a module with CLI replaced, another
application ID, never shows the main window. Needs an X display; run as a
normal user.

    xvfb-run -a python3 ci/gui_cards_smoke.py
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


for a in (["block", "create", "plain"], ["block", "add", "plain", "reddit.com"],
          ["block", "create", "work"], ["block", "add", "work", "youtube.com"],
          ["schedule", "add", "work", "--days", "weekdays", "--from", "09:00", "--to", "17:00"],
          ["break", "reward", "work", "10m", "--per", "1h"],
          ["block", "create", "night"], ["block", "add", "night", "netflix.com"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kcardssmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs, dialogs = [], [], []
win.message = lambda text, *a, **k: msgs.append(text)


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


def cards():
    """{name: (card, texts, buttons by text)} from the page as it is."""
    out = {}
    for c in win.block_cards.get_children():
        texts = [w.get_text() for w in widgets(c) if isinstance(w, Gtk.Label)]
        buttons = {}
        for b in (w for w in widgets(c) if isinstance(w, Gtk.Button)):
            label = b.get_label() or " ".join(w.get_text() for w in widgets(b) if isinstance(w, Gtk.Label))
            buttons[label] = b
        name = next((b.get_label() for b in (w for w in widgets(c) if isinstance(w, Gtk.Button))
                     if b.get_style_context().has_class("kc9k-card-name")), None)
        if name:
            out[name] = (c, texts, buttons)
    return out


def active(name):
    return next(b for b in json.loads(cli("gui-state").stdout)["blocks"] if b["name"] == name)["active"]


real_run = Gtk.Dialog.run
Gtk.Dialog.run = lambda d: (dialogs.append(d.get_title() or type(d).__name__), Gtk.ResponseType.CANCEL)[1]
win.get_child().show_all()  # as the app shows it (a stack only switches to shown pages); the window stays hidden
settle()
check(win.blocks_stack.get_visible_child_name() == "cards", "the Blocks page opens on the list")
shown = cards()
check(sorted(shown) == ["night", "plain", "work"], "a card per block: %s" % sorted(shown))
texts = shown["work"][1]
check("Automated block" in texts and "Continuous block" in shown["plain"][1], "continuous or automated")
check("Breaks: 10 min every hour" in texts and "No breaks" in shown["plain"][1], "breaks: %s" % texts)
check("Unlocked" in texts and "Off" in texts, "unlocked and off: %s" % texts)

# The switch starts and stops a block.
win.card_switches["plain"].set_active(True); settle()
check(active("plain"), "switched on")
win.card_switches["plain"].set_active(False); settle()
check(not active("plain"), "switched off")

# Locked: its switch is greyed out; it can't be stopped from the list.
r = cli("start", "night", "--for", "1h", "--yes"); assert r.returncode == 0, r.stderr
settle()
shown = cards()
check(any(t.startswith("Locked, ") for t in shown["night"][1]), "locked, with the time left: %s" % shown["night"][1])
check(not win.card_switches["night"].get_sensitive() and win.card_switches["night"].get_active(),
      "a locked block's switch is on and greyed out")

# Its parts open the block where they say.
shown["work"][2]["Automated block"].clicked(); settle()
check(win.blocks_stack.get_visible_child_name() == "automation" and win.selected == "work",
      "Automated block opens its automation")
tabs = [win.block_tabs.get_tab_label_text(win.block_tabs.get_nth_page(n)) for n in range(win.block_tabs.get_n_pages())]
check("Schedule & places" not in tabs and win.automation_button.get_label() == "Automation",
      "Automation is in the block's menu, not a tab: %s" % tabs)
win.nav.emit("row-activated", win.nav_rows["blocks"]); settle()
check(win.blocks_stack.get_visible_child_name() == "cards", "Blocks in the sidebar comes back to the list")
cards()["work"][2]["Breaks: 10 min every hour"].clicked(); settle()
check(win.block_tabs.get_tab_label_text(win.block_tabs.get_nth_page(win.block_tabs.get_current_page())) == "Breaks & allowance",
      "the breaks open Breaks & allowance")
win.blocks_stack.set_visible_child_name("cards")
cards()["plain"][2]["plain"].clicked(); settle()
check(win.selected == "plain" and win.blocks_stack.get_visible_child_name() == "detail", "the name opens the block")
win.blocks_stack.set_visible_child_name("cards")
dialogs.clear()
cards()["plain"][2]["Unlocked"].clicked(); settle()
check(dialogs and dialogs[-1].startswith("Start"), "Unlocked asks for locks to start it with: %s" % dialogs)
# With a draft open, the locks are still the block's own (a draft never runs).
r = cli("block", "draft", "plain"); assert r.returncode == 0, r.stderr
settle()
dialogs.clear()
cards()["plain"][2]["Unlocked"].clicked(); settle()
check(dialogs == ["Start plain with locks"], "with a draft open: the block's own locks: %s" % dialogs)
Gtk.Dialog.run = real_run

# §4.2 (owner 2026-11-17, later): nothing on a card but its parts (no ⋯
# menu); deleting an opened block comes back to the list, never to another
# block; a refused delete leaves it open.
check(not any(isinstance(w, Gtk.MenuButton) for w in widgets(win.block_cards)), "the cards have no ⋯ menu")
r = cli("block", "create", "gone"); assert r.returncode == 0, r.stderr
settle()
win.confirm = lambda *a, **k: True
win.open_block("gone"); settle()
win.delete_block(None); settle()
check("gone" not in win.blocks and win.blocks_stack.get_visible_child_name() == "cards" and win.selected != "plain",
      "deleted, the page comes back to the block list: %s %s" % (win.blocks_stack.get_visible_child_name(), win.selected))
win.open_block("night"); settle()
msgs.clear()
win.delete_block(None); settle()
check("night" in win.blocks and win.blocks_stack.get_visible_child_name() == "detail" and win.selected == "night",
      "refused (it is locked), the block stays open: %s" % msgs)

# D1 (owner 2026-11-20): the bar from "← All blocks" to the Start/Stop row
# stays put; only the tabs scroll.
def inside(widget, ancestor):
    while widget is not None:
        if widget is ancestor:
            return True
        widget = widget.get_parent()
    return False


win.open_block("plain"); settle()
check(inside(win.block_tabs, win.detail_scroll), "the tabs scroll")
check(not any(inside(w, win.detail_scroll) for w in (win.title_label, win.draft_bar, win.start_button, win.more_menu[0])),
      "the top bar (title, Save edits, Start, ⋯) stays outside the scrolled part")
check(not isinstance(win.detail.get_parent(), Gtk.Viewport), "the page itself does not scroll")

# D2 (owner 2026-11-20): Start runs at once, no lock dialog; then it is Add locks….
cli("stop", "plain"); settle()
win.open_block("plain"); settle()
check(win.start_button.get_label() == "Start", "an off block's button is Start: %s" % win.start_button.get_label())
dialogs.clear()
real_run, Gtk.Dialog.run = Gtk.Dialog.run, lambda d: (dialogs.append(d.get_title()), Gtk.ResponseType.CANCEL)[1]
win.start_button.clicked(); settle()
Gtk.Dialog.run = real_run
check(not dialogs and win.blocks["plain"]["active"], "Start starts it at once, no dialog: %s" % dialogs)
check(win.start_button.get_label() == "Add locks…", "once on: Add locks…")
check(not win.blocks["plain"].get("held"), "started without locks")

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
raise SystemExit(1 if fails else 0)
