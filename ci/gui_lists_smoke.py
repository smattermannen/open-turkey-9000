"""GUI smoke test for lists without types (SPEC §3.2): the New list dialog
has the three scope boxes URL, title, text instead of a type question, and
every list card shows them. Builds kc9k into a scratch folder, points a
wrapper at a scratch database (KC9K_DB: no layers are ever applied), loads
gui/kc9k-gui.py as a module with CLI replaced and another application ID,
and never shows the window. Needs an X display.

    xvfb-run -a python3 ci/gui_lists_smoke.py
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
r = cli("block", "create", "b"); assert r.returncode == 0, r.stderr
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib
app = gui.App(); app.set_application_id("io.test.kc9klists"); app.register(None)
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
    win.show_block()
def lists():
    return {l["name"]: l for l in json.loads(cli("gui-state").stdout)["blocks"][0]["lists"]}
win.state = None; win.refresh(); pump(lambda: win.state is not None and "b" in win.blocks)
win.selected = "b"; win.show_block()

# The dialog: a name and three scope boxes named as SPEC §3.2 names them,
# URL ticked; no type question.
check([t for _, t in gui.LIST_PLACES] == ["URL", "Title", "Text"], "scope words: %s" % [t for _, t in gui.LIST_PLACES])
dialog, entry, checks = win.list_dialog("blacklist")
check(list(checks) == ["url", "title", "text"], "three scope boxes")
check(checks["url"].get_active() and not checks["title"].get_active() and not checks["text"].get_active(), "URL ticked at first")
radios = []
def walk(w):
    if isinstance(w, Gtk.RadioButton):
        radios.append(w)
    if isinstance(w, Gtk.Container):
        for c in w.get_children():
            walk(c)
walk(dialog)
check(not radios, "no type question")
ok = dialog.get_widget_for_response(Gtk.ResponseType.OK)
check(not ok.get_sensitive(), "no name: OK off")
entry.set_text("Mixed")
check(ok.get_sensitive(), "a name: OK on")
for c in checks.values():
    c.set_active(False)
check(not ok.get_sensitive() and win.list_create_args("blacklist", entry, checks) is None, "no box: OK off")
checks["url"].set_active(True); checks["text"].set_active(True)
args = win.list_create_args("blacklist", entry, checks)
check(args == ["list", "create", "b", "Mixed", "--url", "--text"], "command: %s" % args)
dialog.destroy()
win.run(args); settle()
l = lists().get("Mixed")
check(l is not None and l["where"] == ["url", "text"] and "type" not in l, "list made: %s" % l)
# One list holds a site and words (§3.2).
r = cli("list", "add", "b", "Mixed", "reddit.com", "*casino*", "long cat"); check(r.returncode == 0, "mixed entries: " + r.stderr)

dialog, entry, checks = win.list_dialog("exceptions")
entry.set_text("Spared"); checks["url"].set_active(False); checks["title"].set_active(True)
args = win.list_create_args("exceptions", entry, checks)
check(args == ["list", "create", "b", "Spared", "--title", "--exceptions"], "exception list command: %s" % args)
dialog.destroy()
win.run(args); settle()
check(lists().get("Spared", {}).get("where") == ["title"], "exception list made")

# Every card shows the three boxes, the default lists too.
for name in ("Blocked", "Exceptions", "Mixed", "Spared"):
    card = next(c for c in win.list_cards.values() if isinstance(c, gui.ListCard) and not isinstance(c, gui.PhraseSetCard)
                and c.data["name"] == name)
    check(list(card.place_checks) == ["url", "title", "text"] and
          [n for n, c in card.place_checks.items() if c.get_active()] == card.data["where"], "%s card boxes" % name)
card = next(c for c in win.list_cards.values() if isinstance(c, gui.ListCard) and c.data.get("name") == "Blocked")
card.place_checks["title"].set_active(True); settle()
check(lists()["Blocked"]["where"] == ["url", "title"], "Blocked ticks title from its card")
# §3.9 rate from the card of a text list: saved through 'kc9k list rate'.
card = next(c for c in win.list_cards.values() if isinstance(c, gui.ListCard) and c.data.get("name") == "Mixed")
card.rates["block"].set_value(2.5); card.save_numbers(None); settle()
rate = [a for a in lists()["Mixed"]["actions"] if a["action"] == "block"][0].get("rate")
check(rate == 25, "rate saved from the card: %s" % rate)
# §3.11 block password: contents hidden until it is typed.
r = subprocess.run([W, "block", "password", "b", "set"], input="open1\n", capture_output=True, text=True)
check(r.returncode == 0, "block password set: " + r.stderr)
settle(); win.show_block()
check(win.hidden_bar.get_visible() and not any(l["entries"] for l in win.blocks["b"]["lists"]), "hidden block shows no entries")
win.block_passwords["b"] = "open1"; settle(); win.show_block()
check(not win.hidden_bar.get_visible() and any(l["entries"] for l in win.blocks["b"]["lists"]), "revealed with its password")
# §4.3: select several entries and remove them; double-click to edit one.
card = next(c for c in win.list_cards.values() if isinstance(c, gui.ListCard) and c.data.get("name") == "Mixed")
card.entries.reveal()
sel = card.entries.view.get_selection()
sel.unselect_all(); sel.select_path(Gtk.TreePath(0)); sel.select_path(Gtk.TreePath(1))
before = list(card.data["entries"])
card.entries._remove(None); settle(); win.show_block()
after = [l for l in win.blocks["b"]["lists"] if l["name"] == "Mixed"][0]["entries"]
check(len(after) == len(before) - 2, "two entries removed at once: %s -> %s" % (before, after))
card = next(c for c in win.list_cards.values() if isinstance(c, gui.ListCard) and c.data.get("name") == "Mixed")
card.entries.reveal()
if len(card.entries.store):
    first = card.entries.store[0][0]
    card.entries._edit(card.entries.view, Gtk.TreePath(0), None); settle()
    check(card.entries.entry.get_text() == first, "double-click moves the entry into the box: %r" % card.entries.entry.get_text())

# §4.3 import from KC9K: copy another block's list in (the dialog ticked).
r = cli("block", "create", "src"); r = cli("block", "add", "src", "copied.example"); settle()
real_run = Gtk.Dialog.run
def tick_all(dialog):
    for child in dialog.get_content_area().get_children():
        if isinstance(child, Gtk.CheckButton) and "src" in child.get_label():
            child.set_active(True)
    return Gtk.ResponseType.OK
Gtk.Dialog.run = tick_all
win.selected = "b"; win.import_from_kc9k("b", "Blocked"); settle()
Gtk.Dialog.run = real_run
check("copied.example" in [l for l in win.blocks["b"]["lists"] if l["name"] == "Blocked"][0]["entries"], "imported from another block")

# §3.6 autostart from the block page.
win.selected = "b"; win.autostarts.on_add(["at 08:00 mon-fri"]); settle(); win.show_block()
check(win.blocks["b"]["autostarts"] == ["at 08:00 weekdays"], "autostart added: %s" % win.blocks["b"].get("autostarts"))
win.autostarts.on_remove("at 08:00 weekdays", 1); settle()
check(win.blocks["b"]["autostarts"] == [], "autostart removed")
# §3.6 delay break from the Breaks section.
win.selected = "b"; win.delay_break_check.set_active(True); win.delay_break_minutes.set_value(10)
win.delay_wait_minutes.set_value(30); win.set_delay_break(None); settle(); win.show_block()
check(win.blocks["b"].get("delay_break") == 600 and win.blocks["b"].get("delay_wait") == 1800, "delay break saved")
# §3.6 allowance per week, refilled at 04:00, from the block page.
win.selected = "b"; win.quota_minutes.set_value(300); win.quota_per.set_active_id("week")
win.quota_refill.set_text("04:00"); win.set_quota(None); settle(); win.show_block()
blk = win.blocks["b"]
check(blk["quota_limit"] == 18000 and blk.get("quota_period") == "week" and blk.get("quota_refill") == "04:00",
      "allowance per week saved: %s %s %s" % (blk["quota_limit"], blk.get("quota_period"), blk.get("quota_refill")))
# R-LOCK-6 in the GUI: the user moves the second lock up.
real_run = Gtk.Dialog.run
def move_up(dialog):
    win.lock_order_dialog[1](1)  # first row down = second row up
    return Gtk.ResponseType.OK
Gtk.Dialog.run = move_up
got = win.choose_lock_order([(["--for", "5m"], "timer lock"), (["--lock"], "text lock")])
Gtk.Dialog.run = real_run
check(got is not None and [f[0] for f, _t in got] == ["--lock", "--for"], "unlock order chosen: %s" % got)
print("FAILED:" if fails else "ALL OK", fails)
sys.exit(1 if fails else 0)
