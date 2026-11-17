"""GUI smoke test: every button. Clicks each visible, usable button on every
page, every block tab and in every ⋯ menu (popovers), flips each switch, and
activates each item of the Overview's Start menu: once with dialogs answered
Cancel, then again with them answered OK (or Accept, Yes, Apply) as they
come, empty fields and no file chosen included. No click may raise
in its handler, and none may make the GUI send a command the CLI can't read
(unknown command or flag, wrong arguments). The messages each click shows
are printed for review. Builds kc9k into a scratch folder, a wrapper with
KC9K_DB = scratch file (and a fake nmcli), loads gui/kc9k-gui.py as a module
with CLI replaced, another application ID, never shows the main window.
Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_click_everything_smoke.py
"""
import importlib.util, os, subprocess, sys, tempfile, time, traceback
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
BIN = os.path.join(S, "bin")
os.makedirs(BIN)
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(os.path.join(BIN, "nmcli"), "w") as f:
    f.write('#!/bin/sh\nprintf "AA\\\\:AA\\\\:AA\\\\:AA\\\\:AA\\\\:01:home\\n"\n')
os.chmod(os.path.join(BIN, "nmcli"), 0o755)
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nPATH="{BIN}:$PATH" KC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a):
    return subprocess.run([W, *a], capture_output=True, text=True)


# A block with something in every tab, a cluster, a freeze schedule.
for a in (["block", "create", "b"], ["block", "add", "b", "reddit.com"], ["block", "add-term", "b", "casino"],
          ["block", "add-app", "b", "steam"], ["block", "add-title", "b", "YouTube"],
          ["schedule", "add", "b", "--days", "mon", "--from", "09:00", "--to", "10:00"],
          ["place", "add", "home"], ["place", "rule", "b", "home", "on"],
          ["allowance", "set", "b", "30m", "--per", "day"], ["break", "reward", "b", "10m", "--per", "1h"],
          ["phrase-set", "create", "b", "talk"], ["phrase-set", "add", "b", "talk", "[eat, nice] [hello]"],
          ["cluster", "create", "c"], ["cluster", "add", "c", "cnn.com"], ["stats", "on"]):
    r = cli(*a)
    if r.returncode:
        print("setup", a, r.stderr.strip())

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kclicksmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True

errors, msgs = [], []
sys.excepthook = lambda kind, value, tb: errors.append("".join(traceback.format_exception(kind, value, tb)))
win.message = lambda text, *a, **k: msgs.append(str(text))
# Every dialog answered Cancel (then OK in the second round); menus never
# pop up.
dialogs, answer = [], {"ok": False}


def respond(dialog):
    dialogs.append(dialog.get_title() or type(dialog).__name__)
    if answer["ok"]:
        # As a person can: only a button that isn't greyed out.
        for response in (Gtk.ResponseType.OK, Gtk.ResponseType.ACCEPT, Gtk.ResponseType.YES, Gtk.ResponseType.APPLY):
            button = dialog.get_widget_for_response(response)
            if button is not None and button.get_sensitive():
                return response
    return Gtk.ResponseType.CANCEL


Gtk.Dialog.run = respond
Gtk.Menu.popup_at_widget = Gtk.Menu.popup_at_pointer = lambda *a, **k: None
fails = []


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


def pump(cond, secs=60):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    return cond()


def spin(n=30):
    for _ in range(n):
        GLib.MainContext.default().iteration(False); time.sleep(0.005)


def settle():
    pump(lambda: win.get_sensitive() and not win.busy)
    spin()


def widgets(root):
    """root and everything in it, popovers of menu buttons included."""
    out = [root]
    if isinstance(root, Gtk.Container):
        for child in root.get_children():
            out += widgets(child)
    if isinstance(root, Gtk.MenuButton) and root.get_popover() is not None:
        out += widgets(root.get_popover())
    return out


def shown_in(widget, page):
    """Visible all the way up to the page (the window is never shown), on
    the open tab; a popover counts as shown when its button is."""
    while widget is not None and widget is not page:
        if not widget.get_visible() and not isinstance(widget, Gtk.Popover):
            return False
        parent = widget.get_relative_to() if isinstance(widget, Gtk.Popover) else widget.get_parent()
        if isinstance(parent, Gtk.Notebook) and parent.page_num(widget) != parent.get_current_page() \
                and parent.page_num(widget) >= 0:
            return False
        widget = parent
    return widget is page


def name(button):
    if isinstance(button, Gtk.Switch):
        row = button.get_parent()
        texts = [w.get_text() for w in widgets(row) if isinstance(w, Gtk.Label) and w.get_text()] if row else []
        return f"Switch({(texts[0] if texts else '')[:50]})"
    label = button.get_label() or button.get_tooltip_text() or ""
    if not label and isinstance(button.get_child(), Gtk.Label):
        label = button.get_child().get_text()
    if not label:
        for w in widgets(button):
            if isinstance(w, Gtk.Label) and w.get_text():
                label = w.get_text()
                break
            if isinstance(w, Gtk.Image):
                label = label or "icon:" + (w.get_icon_name()[0] or "")
    return f"{type(button).__name__}({label.strip()[:50]})"


MISUSE = ("unknown command", "unknown flag", "unknown shorthand", "accepts ", "requires at least", "invalid argument",
          "Usage:", "flag needs an argument",
          # a draft that can't be saved or cancelled (found by this test)
          "cannot remove block")
clicked = set()


def candidates(page):
    """The page's usable buttons and switches, each with a key that stays
    the same when the page rebuilds a row: its label and how many with
    that label came before it."""
    seen, out = {}, []
    for button in widgets(page):
        if not isinstance(button, (Gtk.Button, Gtk.Switch)) or isinstance(button, Gtk.MenuButton):
            continue
        if not button.get_sensitive() or not shown_in(button, page) or \
                button.get_toplevel() is not win and not isinstance(button.get_toplevel(), Gtk.Popover):
            continue
        label = name(button)
        seen[label] = seen.get(label, 0) + 1
        out.append(((label, seen[label]), button))
    return out


def click_all(page, where):
    count = 0
    for key, _old in candidates(page):
        if (where, key) in clicked:
            continue
        # The page may have rebuilt since: the button with this key now.
        button = dict(candidates(page)).get(key)
        if button is None:
            continue
        clicked.add((where, key))
        before_err, before_msg, before_dialogs = len(errors), len(msgs), len(dialogs)
        label = name(button)
        if isinstance(button, Gtk.Switch):
            button.set_active(not button.get_active())
        else:
            button.clicked()
        settle()
        count += 1
        new_msgs = msgs[before_msg:]
        new_dialogs = dialogs[before_dialogs:]
        for e in errors[before_err:]:
            fails.append(f"{where} {label}: raised")
            print(f"FAIL {where} {label} raised:\n{e}")
        for m in new_msgs:
            if any(word in m for word in MISUSE):
                fails.append(f"{where} {label}: {m}")
                print(f"FAIL {where} {label}: the CLI couldn't read the command: {m}")
        if new_msgs or new_dialogs:
            print(f"  {where} {label}: " + "; ".join(
                [f"dialog '{d}'" for d in new_dialogs] + [m.replace(chr(10), ' ')[:160] for m in new_msgs]))
    return count


# As the app shows the window (show_all, then the state hides what it
# should), but the window itself is never shown.
win.get_child().show_all()
settle()
win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
settle()
total = 0
# Owner 2026-11-17: Frozen Chicken is a mode, its page part of the Blocks
# page's mode view; monk mode's view too.
for page_name in ("dashboard", "blocks", "clusters", "Frozen Chicken", "Monk mode", "stats", "circadian",
                  "settings") * 2:
    if page_name in ("Frozen Chicken", "Monk mode"):
        win.open_mode(page_name)
        settle()
        total += click_all(win.blocks_stack.get_child_by_name("mode"), f"[mode {page_name}]")
        win.blocks_stack.set_visible_child_name("cards")
        continue
    if page_name == "dashboard" and total:
        answer["ok"] = True  # the second round
        clicked.clear()
        print("--- every dialog answered OK ---")
    page = win.stack.get_child_by_name(page_name)
    if page is None:
        print("no page", page_name)
        continue
    win.stack.set_visible_child_name(page_name)
    settle()
    if page_name == "blocks":
        win.selected = "b"; win.show_block(); settle()
        for tab in range(win.block_tabs.get_n_pages()):
            win.block_tabs.set_current_page(tab)
            settle()
            title = win.block_tabs.get_tab_label_text(win.block_tabs.get_nth_page(tab))
            total += click_all(page, f"[blocks/{title}]")
            win.selected = "b"; win.show_block(); settle()
        # Owner 2026-11-17: the automation opens from the block's menu.
        win.open_automation("b"); settle()
        total += click_all(win.automation_page, "[blocks/Automation]")
        win.selected = "b"; win.blocks_stack.set_visible_child_name("detail"); win.show_block(); settle()
    else:
        total += click_all(page, f"[{page_name}]")
    if page_name == "dashboard":
        for item in getattr(win, "dash_start_menu", Gtk.Menu()).get_children():
            before_err, before_msg = len(errors), len(msgs)
            item.activate()
            settle()
            total += 1
            for e in errors[before_err:]:
                fails.append(f"[dashboard] Start menu {item.get_label()}: raised")
                print(f"FAIL Start menu {item.get_label()} raised:\n{e}")

check(total > 200, f"clicked {total} buttons")
check(not errors, f"no handler raised ({len(errors)} did)")
print("FAILED: %d" % len(fails) if fails else "all passed")
raise SystemExit(1 if fails else 0)
