"""GUI smoke test for phrase sets (SPEC §4.3): builds kc9k into a scratch
folder, points a wrapper at a scratch database (KC9K_DB: no layers are
ever applied), loads gui/kc9k-gui.py as a module with CLI replaced and
another application ID, and never shows the window. Needs an X display.

    python3 ci/gui_phrase_smoke.py
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
for a in (["block", "create", "b"], ["phrase-set", "create", "b", "Grandpa", "--title", "--text"],
          ["phrase-set", "add", "b", "Grandpa", "[eat, nice] [hello, grandpa]", "[big-] [-gorilla] [x]? :3"]):
    r = cli(*a); assert r.returncode == 0, r.stderr
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib
app = gui.App(); app.set_application_id("io.test.kc9ksmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True  # no install dialog in a test
asked = []
win.confirm = lambda text, *a, **k: asked.append(text) or True
def pump(cond, secs=15):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"
fails = []
def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what); ok or fails.append(what)
win.state = None; win.refresh(); pump(lambda: win.state is not None and "b" in win.blocks)
win.selected = "b"; win.show_block()
cards = {k: c for k, c in win.list_cards.items() if isinstance(c, gui.PhraseSetCard)}
check(len(cards) == 1, "one phrase set card")
card = next(iter(cards.values()))
check(card.get_parent() is win.list_boxes["phrases"], "card sits under Phrase sets")
check(card.place_checks["title"].get_active() and card.place_checks["text"].get_active() and not card.place_checks["url"].get_active(), "scope boxes")
check([r[0] for r in card.entries.store] == ["[eat, nice] [hello, grandpa]", "[big-] [-gorilla] [x]? :3"], "phrases listed")
# The phrase editor sits in the card (§4.3); double-click loads a phrase.
check(not hasattr(gui, "PhraseDialog"), "no pop-up phrase editor")
card.edit_phrase(card.data["phrases"][1]["text"])
d = card.editor
check(d.get_text() == "[big-] [-gorilla] [x]? :3", "editor reads a phrase back: " + d.get_text())
check(card.save_phrase.get_label() == "Save changes" and not card.save_phrase.get_sensitive(), "unchanged phrase can't be saved")
rows = d.rows.get_children()
check(all(r.check.get_sensitive() for r in rows), "every set can be optional")
rows[2].check.set_active(True)
check(d.get_text() == "[big-] [-gorilla] [x]", "all necessary: length is the default: " + d.get_text())
check(card.cancel_phrase.get_label() == "Cancel" and card.delete_phrase.get_visible(), "Cancel and Delete phrase while changing")
card.edit_phrase(None)
check(asked and asked[-1].startswith("Drop the unsaved"), "Cancel asks before dropping changes")
check(d.get_text() == "" and not card.save_phrase.get_sensitive() and card.save_phrase.get_label() == "Add phrase", "cleared editor can't be saved")
check(not card.delete_phrase.get_visible(), "no Delete phrase for a new phrase")
rows = d.rows.get_children(); rows[0].entry.set_text("red"); rows[1].entry.set_text("car, truck")
d.add_row(); d.rows.get_children()[2].entry.set_text("big"); d.rows.get_children()[2].check.set_active(False)
check(d.get_text() == "[red] [car, truck] [big]?", "built phrase: " + d.get_text())
d.length.set_value(3)
text = d.get_text(); check(text == "[red] [car, truck] [big]? :3", "with length: " + text)
for _ in range(10): d.add_row()
check(len(d.rows.get_children()) == 13 and d.add_set.get_sensitive(), "more than 9 word sets")
check(win.free_phrase_set_name() == "Phrase set 1", "unnamed phrase set gets a name")
# Through the card: add, threshold, action, scope, test, rename.
def settle():
    # Wait out a refresh already running (it may have read the state before
    # the last change), then read it afresh.
    pump(lambda: win.get_sensitive() and not win.busy)
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
    win.show_block()
card.edit_phrase(None); rows = card.editor.rows.get_children()
rows[0].entry.set_text("red"); rows[1].entry.set_text("car, truck")
card.editor.add_row(); card.editor.rows.get_children()[2].entry.set_text("big"); card.editor.rows.get_children()[2].check.set_active(False)
card.editor.length.set_value(3)
check(card.editor.get_text() == text, "rebuilt in the card")
card.commit_phrase(); settle()
s = json.loads(cli("gui-state").stdout)["blocks"][0]["phrase_sets"][0]
check(len(s["phrases"]) == 3, "phrase added through the card")
card = win.list_cards[-s["id"]]
card.action_checks["greyscale"].set_active(True); settle()
card = win.list_cards[-s["id"]]
card.spins["block"][1].set_value(2); card.save_numbers(None); settle()
card = win.list_cards[-s["id"]]
card.place_checks["url"].set_active(True); settle()
card = win.list_cards[-s["id"]]
card.test_check.set_active(True); settle()
out = cli("phrase-set", "show", "b", "Grandpa").stdout
check("block (2)" in out and "greyscale (1)" in out and "url, title, text" in out and "test mode" in out, "show after card edits: " + out.splitlines()[0])
# The owner's report: seven word sets, one set's terms copied into another,
# saved, then Save clicked again at once: one change, no error, no copy.
seven = "[a, b] [c, d] [e] [f] [g]? [h] [i]"
r = cli("phrase-set", "add", "b", "Grandpa", seven); assert r.returncode == 0, r.stderr
settle(); card = win.list_cards[-s["id"]]
asked.clear(); card.edit_phrase(seven)
check(not asked, "loading into an untouched editor asks nothing")
rows = card.editor.rows.get_children()
rows[1].entry.set_text(rows[1].entry.get_text() + ", " + rows[0].entry.get_text())
msgs = []
win.message = lambda text, *a, **k: msgs.append(text)
card.commit_phrase(); card.commit_phrase()
check(not card.save_phrase.get_sensitive(), "Save is off while saving")
pump(lambda: win.get_sensitive() and not win.busy)
for _ in range(50): GLib.MainContext.default().iteration(False)
card = win.list_cards[-s["id"]]
texts = [p["text"] for p in json.loads(cli("gui-state").stdout)["blocks"][0]["phrase_sets"][0]["phrases"]]
copied = "[a, b] [c, d, a, b] [e] [f] [g]? [h] [i]"
check(copied in texts and seven not in texts and len(texts) == 4, "changed in place: %s" % texts)
check(not msgs, "no error: %s" % msgs)
check(card.editing and card.editing["text"] == copied and card.save_phrase.get_label() == "Save changes"
      and not card.save_phrase.get_sensitive(), "keeps editing the saved phrase")
check([r[0] for r in card.entries.store] == texts, "list shows the change: %s" % [r[0] for r in card.entries.store])
# Delete phrase removes the loaded one.
card.remove_loaded(); settle(); card = win.list_cards[-s["id"]]
texts = [p["text"] for p in json.loads(cli("gui-state").stdout)["blocks"][0]["phrase_sets"][0]["phrases"]]
check(copied not in texts and len(texts) == 3 and card.editing is None, "Delete phrase: %s" % texts)
# A one-ended bridge from the card is refused, with a message.
msgs = []
win.message = lambda text, *a, **k: msgs.append(text)
card = win.list_cards[-s["id"]]
card.add_entries(["[-gorilla] [x]"]); pump(lambda: win.get_sensitive()); time.sleep(0.3)
for _ in range(50): GLib.MainContext.default().iteration(False)
check(len(json.loads(cli("gui-state").stdout)["blocks"][0]["phrase_sets"][0]["phrases"]) == 3, "one-ended bridge not stored")
print("messages:", msgs[-1:] if msgs else "(none captured)")
# A phrase cluster: phrases typed whole, no recording into it.
r = cli("cluster", "create", "Cats", "--phrases"); assert r.returncode == 0, r.stderr
settle(); win.update_clusters()
win.cluster_combo.set_active(win.clusters_shown.index("Cats")); win.update_clusters()
win.cluster_entries.entry.set_text("[long cat, neko] [running, sitting]")
win.cluster_entries._add(None); settle(); win.update_clusters()
c = [c for c in json.loads(cli("gui-state").stdout)["clusters"] if c["name"] == "Cats"][0]
check(c["phrases"] and c["entries"] == ["[long cat, neko] [running, sitting]"], "phrase typed into a cluster: %s" % c["entries"])
check(not win.record_start.get_sensitive(), "no recording into a phrase cluster")
check(not win.record_words.get_sensitive(), "no word recording into a phrase cluster")
# §3.12 word recording into a text cluster.
r = cli("cluster", "create", "Study", "--text"); assert r.returncode == 0, r.stderr
settle(); win.update_clusters()
win.cluster_combo.set_active(win.clusters_shown.index("Study")); win.update_clusters()
check(win.record_words.get_sensitive(), "word recording offered for a text cluster")
win.record_words.clicked(); settle(); win.update_clusters()
rec = json.loads(cli("gui-state").stdout)["recording"]
check(rec and rec["words"] and "Recording words" in win.record_label.get_label(), "word recording from the GUI: %s" % rec)
r = cli("record", "stop", "--discard"); assert r.returncode == 0, r.stderr
r = cli("record", "baseline", "off"); assert r.returncode == 0, r.stderr
settle()
# R-PHRASE-6, §3.2: a list's card ticks the URL, title and text.
r = cli("list", "create", "b", "Url words", "--url", "--terms"); assert r.returncode == 0, r.stderr
settle()
card = next(c for c in win.list_cards.values() if isinstance(c, gui.ListCard) and not isinstance(c, gui.PhraseSetCard)
            and c.data["name"] == "Url words")
check(set(card.place_checks) == {"url", "title", "text", "ads"} and card.place_checks["url"].get_active(), "list card ticks url")
card.place_checks["text"].set_active(True); settle()
out = cli("list", "show", "b").stdout
check("Url words (blacklist) looks at: url, text" in out, "list scope from the card")
# §3.9 ad trigger: the Ads box on a blacklist card.
card.place_checks["ads"].set_active(True); settle()
check("Url words (blacklist) looks at: url, text, ads" in cli("list", "show", "b").stdout, "ads scope from the card")
print("FAILED:" if fails else "ALL OK", fails)
sys.exit(1 if fails else 0)
