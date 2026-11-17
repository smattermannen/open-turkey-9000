"""GUI smoke test for importing (§4.3; owner, 2026-11-16: one Import button,
KC9K reads the file's layout): a phrase set's Import takes phrases, turns a
file of rendered terms into a word set in its editor, and offers a list of
sites to Blocked; a list's Import reads hosts files and adblock rules as
sites and sends phrases to a phrase set. Clicks the real Import buttons.
Builds kc9k into a scratch folder, a wrapper with KC9K_DB = scratch file,
loads gui/kc9k-gui.py as a module with CLI replaced, another application
ID, never shows the main window. Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_import_smoke.py
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
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)


def write(name, text):
    path = os.path.join(S, name)
    with open(path, "w") as f:
        f.write(text)
    return path


PHRASES = write("phrases.txt", "# my phrases\n[eat, nice] [hello, grandpa]\n[big-, strong] [-gorilla, cat]\n")
TERMS = write("terms.txt", "casino\npoker\n# a comment\nslot machine\n")
HOSTS = write("hosts.txt", "0.0.0.0 tracker.example.com\n127.0.0.1 localhost\n0.0.0.0 ads.example.net\n")
MIXED = write("mixed.txt", "[sunny, warm] [beach]\n||banners.example.org^\nlottery\n")
for a in (["block", "create", "b"], ["phrase-set", "create", "b", "Words"]):
    r = cli(*a); assert r.returncode == 0, r.stderr

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kimportsmoke"); app.register(None)
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
    win.show_block()


def block():
    return next(b for b in json.loads(cli("gui-state").stdout)["blocks"] if b["name"] == "b")


def phrases():
    return [p["text"] for p in block()["phrase_sets"][0]["phrases"]]


def entries(name):
    return next(lst for lst in block()["lists"] if lst["name"] == name)["entries"]


def phrase_card():
    return next(c for c in win.list_cards.values() if isinstance(c, gui.PhraseSetCard))


def list_card(name):
    return next(c for c in win.list_cards.values()
                if not isinstance(c, gui.PhraseSetCard) and c.data["name"] == name)


real_run = Gtk.Dialog.run
answers = []  # what each dialog answers, in turn: a response, or a function of the dialog


def scripted(dialog):
    answer = answers.pop(0) if answers else Gtk.ResponseType.CANCEL
    return answer(dialog) if callable(answer) else answer


Gtk.Dialog.run = scripted
chosen_file = []
win.choose_file = lambda *a, **k: chosen_file.pop(0) if chosen_file else None


def import_with(card, path, *more):
    """Clicks the card's Import…, answers "From file…" and picks path."""
    answers[:] = [Gtk.ResponseType.OK, *more]
    chosen_file[:] = [path]
    msgs.clear()
    card.entries.file_buttons["Import…"].clicked()
    settle()


settle()
win.selected = "b"; win.show_block()
card = phrase_card()
check("Import…" in card.entries.file_buttons, "the phrase set has one Import…")

# Phrases go into the set.
import_with(card, PHRASES)
check(sorted(phrases()) == sorted(["[eat, nice] [hello, grandpa]", "[big-, strong] [-gorilla, cat]"]),
      "phrases imported: %s" % phrases())

# Rendered terms become one word set in the editor.
card = phrase_card()
import_with(card, TERMS)
rows = card.editor.rows.get_children()
texts = [r.entry.get_text() for r in rows]
check("casino, poker, slot machine" in texts, "terms are one word set in the editor: %s" % texts)
check(msgs and "word set in the phrase editor" in msgs[-1], "said so: %s" % msgs)
check(len(phrases()) == 2, "nothing added to the set by a term list")

# A hosts file in a phrase set: offered to Blocked (confirm answers yes).
card = phrase_card()
import_with(card, HOSTS)
check(sorted(entries("Blocked")) == ["ads.example.net", "tracker.example.com"],
      "hosts file read as sites, into Blocked: %s" % entries("Blocked"))

# A list's Import: phrases go to a phrase set (the first one offered),
# the adblock rule and the term into the list.
lcard = list_card("Blocked")
import_with(lcard, MIXED, Gtk.ResponseType.OK)  # "Add them" to Words
check("[sunny, warm] [beach]" in phrases(), "the list's import sent the phrase to Words: %s" % phrases())
check("banners.example.org" in entries("Blocked") and "lottery" in entries("Blocked"),
      "adblock rule and term into Blocked: %s" % entries("Blocked"))
check(not any("[" in e or "||" in e for e in entries("Blocked")), "no raw phrase or rule stored: %s" % entries("Blocked"))
check(not any("Left out 1 phrases" in m for m in msgs), "no 'left out' for phrases that were routed: %s" % msgs)

# The same with "Leave them out": only the entries go in.
cli("list", "remove", "b", "Blocked", "lottery")
import_with(list_card("Blocked"), MIXED, Gtk.ResponseType.CANCEL)
check("lottery" in entries("Blocked"), "entries imported when phrases are left out")

# From KC9K: another list's entries become a word set.
for a in (["block", "create", "other"], ["block", "add-term", "other", "roulette", "blackjack"]):
    cli(*a)
settle(); win.selected = "b"; win.show_block()
card = phrase_card()


def tick_other(dialog):
    for check_button, entries_ in win.term_sources:
        if check_button.get_label() == "other: Rendered terms on page":
            check_button.set_active(True)
    return Gtk.ResponseType.OK


answers[:] = [2, tick_other]
msgs.clear()
card.entries.file_buttons["Import…"].clicked()
for _ in range(20):
    GLib.MainContext.default().iteration(False)
texts = [r.entry.get_text() for r in card.editor.rows.get_children()]
check(any("roulette" in t and "blackjack" in t for t in texts), "From KC9K: a list's entries as a word set: %s" % texts)

# §4.3: the whole internet, and all searches containing some words.
settle(); win.selected = "b"; win.show_block()
answers[:] = [3]  # Whole internet (confirm is answered yes)
list_card("Blocked").entries.file_buttons["Import…"].clicked()
settle()
check("*.*" in entries("Blocked"), "Whole internet adds *.*: %s" % entries("Blocked"))
cli("list", "create", "b", "Searches", "--text")
settle(); win.selected = "b"; win.show_block()
win.ask_text = lambda *a, **k: "poker, slot machine"
answers[:] = [4]  # All searches containing…
list_card("Searches").entries.file_buttons["Import…"].clicked()
settle()
searches = next(l for l in block()["lists"] if l["name"] == "Searches")
check("poker" in searches["entries"] and "slot machine" in searches["entries"] and "url" in searches["where"],
      "All searches containing… adds the words and looks at the address: %s" % searches)

Gtk.Dialog.run = real_run
check(not [m for m in msgs if m.startswith("Error")], "no errors: %s" % msgs)
win.destroy()
print("FAILED: " + ", ".join(fails) if fails else "all passed")
sys.exit(1 if fails else 0)
