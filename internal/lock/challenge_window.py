"""The text lock's challenge window (SPEC §4.6, R-LOCK-5).

kc9k runs this itself, as the signed-in user on their desktop, when the
command has no terminal. It reads the challenge's pages as JSON on standard
input and writes one line per key to standard output; kc9k reads those keys
and decides (this window decides nothing):

    k <code> <time>   a printable ASCII key (code), X server time in ms
    b <time>          Backspace
    n <time>          Enter: the page is done (the next one is shown)
    p                 a paste or a drop was tried
    s                 a key not typed on the keyboard (a synthetic event)
    c                 cancelled (Escape or the window closed)

There is no text field: nothing can be pasted into it.
"""
import json
import sys

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
from gi.repository import Gdk, GLib, Gtk  # noqa: E402

challenge = json.loads(sys.stdin.read())
pages = challenge["pages"]
need = challenge.get("accuracy", 98)
width = challenge.get("width", 60)
state = {"page": 0, "typed": "", "done": False}


def say(line):
    sys.stdout.write(line + "\n")
    sys.stdout.flush()


def finish(line):
    if not state["done"]:
        state["done"] = True
        say(line)
    Gtk.main_quit()


def wrap(text):
    return "\n".join(text[i:i + width] for i in range(0, len(text), width)) or " "


window = Gtk.Window(title="Killer Chicken 9000: unlock challenge")
window.set_keep_above(True)
window.set_position(Gtk.WindowPosition.CENTER)
window.set_border_width(18)
box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
window.add(box)
title = Gtk.Label(xalign=0)
box.pack_start(title, False, False, 0)
shown = Gtk.Label(xalign=0, selectable=False)
typed = Gtk.Label(xalign=0, selectable=False)
for label in (shown, typed):
    label.get_style_context().add_class("monospace")
frame = Gtk.Frame(label="Typed")
frame.add(typed)
box.pack_start(shown, False, False, 0)
box.pack_start(frame, False, False, 0)
hint = Gtk.Label(xalign=0, wrap=True, max_width_chars=width)
hint.get_style_context().add_class("dim-label")
box.pack_start(hint, False, False, 0)
cancel = Gtk.Button(label="Cancel")
cancel.connect("clicked", lambda _b: finish("c"))
row = Gtk.Box()
row.pack_end(cancel, False, False, 0)
box.pack_start(row, False, False, 0)


def show():
    page = pages[state["page"]]
    title.set_markup("<b>Type the text below by hand</b>" + (
        f"  (page {state['page'] + 1} of {len(pages)})" if len(pages) > 1 else ""))
    shown.set_text(wrap(page))
    typed.set_text(wrap(state["typed"]))
    hint.set_text(f"Type all {len(page)} characters, then press Enter. Line breaks are only for display; "
                  f"don't type them. You need {need:g}% accuracy over all pages. Pasting is refused. "
                  "Backspace works; Escape cancels.")


def on_key(_widget, event):
    if event.send_event:
        finish("s")
        return True
    ctrl = event.state & Gdk.ModifierType.CONTROL_MASK
    name = Gdk.keyval_name(event.keyval) or ""
    if (ctrl and name.lower() == "v") or (event.state & Gdk.ModifierType.SHIFT_MASK and name == "Insert"):
        finish("p")
        return True
    if name == "Escape":
        finish("c")
        return True
    if name in ("Return", "KP_Enter"):
        say(f"n {event.time}")
        state["typed"] = ""
        if state["page"] + 1 >= len(pages):
            state["done"] = True
            Gtk.main_quit()
            return True
        state["page"] += 1
        show()
        return True
    if name == "BackSpace":
        say(f"b {event.time}")
        state["typed"] = state["typed"][:-1]
        typed.set_text(wrap(state["typed"]))
        return True
    if ctrl or event.state & Gdk.ModifierType.MOD1_MASK:
        return True
    code = Gdk.keyval_to_unicode(event.keyval)
    if 0x20 <= code < 0x7f:
        say(f"k {code} {event.time}")
        state["typed"] += chr(code)
        typed.set_text(wrap(state["typed"]))
    return True


def on_button(_widget, event):
    if event.button == 2:  # the middle button pastes the selection
        finish("p")
    return False


def on_drop(*_args):
    finish("p")
    return True


window.connect("key-press-event", on_key)
window.connect("button-press-event", on_button)
window.add_events(Gdk.EventMask.BUTTON_PRESS_MASK)
window.drag_dest_set(Gtk.DestDefaults.ALL, [], Gdk.DragAction.COPY | Gdk.DragAction.MOVE)
window.drag_dest_add_text_targets()
window.drag_dest_add_uri_targets()
window.connect("drag-drop", on_drop)
window.connect("delete-event", lambda *_a: finish("c") or True)
show()
window.show_all()
window.present()
GLib.idle_add(lambda: window.present_with_time(Gdk.CURRENT_TIME) or False)
Gtk.main()
