"""GUI smoke test: editing phrases the hard way (SPEC §4.3, R-WEAK-5).
Builds kc9k into a scratch folder, points a wrapper at a scratch database
(KC9K_DB: no layers are ever applied), loads gui/kc9k-gui.py as a module
with CLI replaced and another application ID, and never shows the window.
Needs an X display.

    xvfb-run -a python3 ci/gui_phrase_edit_smoke.py
"""
import importlib.util, json, os, sqlite3, subprocess, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
DB = os.path.join(S, "t.db")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={DB} exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a, stdin=""):
    return subprocess.run([W, *a], capture_output=True, text=True, input=stdin)


for a in (["block", "create", "b"], ["block", "add", "b", "example.org"],
          ["phrase-set", "create", "b", "P", "--text"],
          ["phrase-set", "add", "b", "P", "[eat, nice] [hello, grandpa]", "[red] [car]"]):
    r = cli(*a); assert r.returncode == 0, r.stderr
# A row from before phrases were canonical: the default length written out.
con = sqlite3.connect(DB)
set_id = con.execute("SELECT id FROM phrase_sets WHERE name = 'P'").fetchone()[0]
con.execute("INSERT INTO phrases (set_id, spec) VALUES (?, ?)", (set_id, json.dumps(
    {"sets": [{"terms": ["old"], "necessary": True}, {"terms": ["row"], "necessary": True}], "length": 2})))
con.commit(); con.close()

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kphraseedit"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs, asked = [], [], []
win.message = lambda text, *a, **k: msgs.append(text)
# Cancel asks before dropping unsaved work: answer yes, and count the asks.
win.confirm = lambda *a, **k: asked.append(a[0] if a else "") or True


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


def pump(cond, secs=20):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"


def settle():
    pump(lambda: win.get_sensitive() and not win.busy)
    for _ in range(30):
        GLib.MainContext.default().iteration(False)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
    win.show_block()


def phrases():
    return [p["text"] for p in json.loads(cli("gui-state").stdout)["blocks"][0]["phrase_sets"][0]["phrases"]]


def card():
    return next(c for c in win.list_cards.values() if isinstance(c, gui.PhraseSetCard))


def fill(sets):
    """Puts word sets [(terms, necessary)] into the editor."""
    ed = card().editor
    while len(ed.rows.get_children()) > len(sets):
        ed.rows.remove(ed.rows.get_children()[-1])
    while len(ed.rows.get_children()) < len(sets):
        ed.add_row()
    for row, (terms, necessary) in zip(ed.rows.get_children(), sets):
        row.entry.set_text(terms)
        row.check.set_active(necessary)
    ed.changed()


win.state = None; win.refresh(); pump(lambda: win.state is not None and "b" in win.blocks)
win.selected = "b"; win.show_block()
check(phrases() == ["[eat, nice] [hello, grandpa]", "[red] [car]", "[old] [row]"], "listed once each: %s" % phrases())

# 1. Change a phrase twice in a row: always one phrase, the editor follows.
c = card(); c.edit_phrase("[eat, nice] [hello, grandpa]")
fill([("eat, tasty", True), ("hello, grandpa", True)])
c.commit_phrase(); settle()
check(phrases().count("[eat, tasty] [hello, grandpa]") == 1 and "[eat, nice] [hello, grandpa]" not in phrases(),
      "first change replaced it: %s" % phrases())
c = card()
check(c.save_phrase.get_label() == "Save changes" and c.editing and c.editing["text"] == "[eat, tasty] [hello, grandpa]",
      "still editing the new phrase: %s / %s" % (c.save_phrase.get_label(), c.editing and c.editing["text"]))
fill([("eat, tasty", True), ("hello, grandpa, grandma", True)])
c.commit_phrase(); settle()
check(len(phrases()) == 3 and "[eat, tasty] [hello, grandpa, grandma]" in phrases(), "second change, no copy: %s" % phrases())

# 2. Only letter case: nothing to save, nothing lost.
c = card(); c.edit_phrase("[red] [car]")
fill([("RED", True), ("Car", True)])
check(not c.save_phrase.get_sensitive(), "a case-only change can't be saved")
c.commit_phrase(); settle()
check("[red] [car]" in phrases(), "the phrase is still there: %s" % phrases())

# 3. The old row with a written-out default length: changed, not doubled,
# no "has no such phrase".
c = card(); c.edit_phrase("[old] [row]")
fill([("old", True), ("rows", True)])
c.commit_phrase(); settle()
check("[old] [rows]" in phrases() and "[old] [row]" not in phrases(), "old row changed: %s" % phrases())
check(not msgs, "no error: %s" % msgs)

# 4. A new phrase equal to one there: Add is off.
c = card(); c.edit_phrase(None)
fill([("Red", True), ("car", True)])
check(not c.save_phrase.get_sensitive(), "a phrase the set has can't be added again")
# And a new one: added, and it stays loaded for more changes.
fill([("blue", True), ("car", True)])
c.commit_phrase(); settle()
check("[blue] [car]" in phrases() and card().editing and card().editing["text"] == "[blue] [car]"
      and card().save_phrase.get_label() == "Save changes",
      "added, then loaded: %s" % (card().editing and card().editing["text"]))

# 5. A refresh while typing keeps the card, the loaded phrase and the text.
c = card(); c.edit_phrase("[blue] [car]")
fill([("blue, green", True), ("car", True)])
before = c
settle()
check(card() is before and card().editor.get_text() == "[blue, green] [car]" and card().editing["text"] == "[blue] [car]",
      "refresh mid-edit keeps everything")
# Renaming the set mid-edit keeps the card and the edit too.
r = cli("phrase-set", "rename", "b", "P", "Grandpa words"); check(r.returncode == 0, "renamed: " + r.stderr)
settle()
check(card() is before and card().editor.get_text() == "[blue, green] [car]", "rename mid-edit keeps the edit")
card().commit_phrase(); settle()
check("[blue, green] [car]" in phrases() and "[blue] [car]" not in phrases(), "saved after the rename: %s" % phrases())

# 6. Unicode terms, and a bridge.
c = card(); c.edit_phrase(None)
fill([("café, naïve", True), ("日本", True)])
c.commit_phrase(); settle()
check("[café, naïve] [日本]" in phrases(), "unicode phrase: %s" % phrases())
c = card(); c.edit_phrase(None)
fill([("big-", True), ("-gorilla", True)])
c.commit_phrase(); settle()
check("[big-] [-gorilla]" in phrases(), "bridge phrase")
# A one-ended bridge: refused with a message, nothing stored, the editor keeps it.
msgs.clear()
c = card(); c.edit_phrase(None)
fill([("-gorilla", True), ("x", True)])
c.commit_phrase(); settle()
check(msgs and "bridge" in msgs[-1] and "[-gorilla] [x]" not in phrases(), "one-ended bridge refused: %s" % msgs[-1:])
check(card().editor.get_text() == "[-gorilla] [x]", "the refused phrase stays in the editor to fix")
msgs.clear()

# 7. Word sets: never fewer than two; up to 100; the length follows.
c = card(); c.edit_phrase(None)
ed = c.editor
check(all(not r.remove.get_sensitive() for r in ed.rows.get_children()), "two word sets can't be removed")
for _ in range(98):
    ed.add_row()
check(len(ed.rows.get_children()) == 100 and not ed.add_set.get_sensitive(), "at most 100 word sets")
fill([("a", True), ("b", True), ("c", False)])
check(ed.length.get_value_as_int() == 2 and ed.get_text() == "[a] [b] [c]?", "length follows the necessary sets")
ed.rows.get_children()[2].check.set_active(True)
check(ed.length.get_value_as_int() == 3 and ed.get_text() == "[a] [b] [c]", "and follows when one becomes necessary")
fill([("a", True), ("", True)])
check(ed.get_text() == "" and "at least one term" in ed.preview.get_text(), "an empty word set explained: " + ed.preview.get_text())

# 8. Locked block: a change that drops terms is refused whole and said;
# one that only adds terms is free (R-WEAK-5).
r = cli("start", "b", "--lock", "--lock-chars", "5", "--yes"); check(r.returncode == 0, "locked: " + r.stderr)
settle()
msgs.clear()
c = card(); c.edit_phrase("[red] [car]")
fill([("red", True), ("truck", True)])
c.commit_phrase(); settle()
check("[red] [car]" in phrases() and "[red] [truck]" not in phrases(), "locked: refused whole: %s" % phrases())
check(msgs and "locked" in msgs[-1], "locked: the refusal is shown: %s" % msgs[-1:])
check(card().editing and card().editing["text"] == "[red] [car]" and card().editor.get_text() == "[red] [truck]",
      "locked: the edit stays in the editor")
fill([("red", True), ("car, truck", True)])
card().commit_phrase(); settle()
check("[red] [car, truck]" in phrases() and "[red] [car]" not in phrases(), "locked: adding terms is free: %s" % phrases())

# 9. Export and import: the same phrases come back.
out = cli("phrase-set", "export", "b", "Grandpa words").stdout
path = os.path.join(S, "p.txt"); open(path, "w").write(out)
r = cli("phrase-set", "create", "b", "Copy", "--text"); check(r.returncode == 0, r.stderr)
r = cli("phrase-set", "import", "b", "Copy", path); check(r.returncode == 0, "import: " + r.stderr)
copy = cli("phrase-set", "export", "b", "Copy").stdout
check(sorted(copy.splitlines()) == sorted(out.splitlines()) and out.strip(), "round trip: %d phrases" % len(out.splitlines()))

win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
