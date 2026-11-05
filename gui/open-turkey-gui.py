#!/usr/bin/env python3
"""Graphical interface for Open Turkey (GTK 3).

All changes go through the open-turkey command, so the interface follows the
same lock rules as the terminal and needs no privileges of its own.
"""
import json
import shlex
import subprocess
import threading

import gi

gi.require_version("Gtk", "3.0")
from gi.repository import GLib, Gtk  # noqa: E402

CLI = "open-turkey"
REFRESH_SECONDS = 5


def fmt_seconds(seconds):
    hours, rest = divmod(int(seconds), 3600)
    return f"{hours} h {rest // 60} min" if hours else f"{max(rest // 60, 1)} min"


def run_cli(args):
    """Return (ok, output) for one command."""
    try:
        result = subprocess.run(
            [CLI, *args], stdin=subprocess.DEVNULL, capture_output=True,
            text=True, timeout=120,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        return False, str(error)
    return result.returncode == 0, (result.stdout + result.stderr).strip()


class EditableList(Gtk.Box):
    """A titled list with an entry for adding and a button for removing."""

    def __init__(self, title, placeholder, on_add, on_remove):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.on_add, self.on_remove = on_add, on_remove
        label = Gtk.Label(xalign=0)
        label.set_markup(f"<b>{title}</b>")
        self.pack_start(label, False, False, 0)

        self.store = Gtk.ListStore(str)
        self.view = Gtk.TreeView(model=self.store, headers_visible=False)
        self.view.append_column(
            Gtk.TreeViewColumn("", Gtk.CellRendererText(), text=0))
        scroll = Gtk.ScrolledWindow(min_content_height=110)
        scroll.set_shadow_type(Gtk.ShadowType.IN)
        scroll.add(self.view)
        self.pack_start(scroll, True, True, 0)

        row = Gtk.Box(spacing=6)
        self.entry = Gtk.Entry(placeholder_text=placeholder)
        self.entry.connect("activate", self._add)
        add = Gtk.Button(label="Lägg till")
        add.connect("clicked", self._add)
        remove = Gtk.Button(label="Ta bort")
        remove.connect("clicked", self._remove)
        row.pack_start(self.entry, True, True, 0)
        row.pack_start(add, False, False, 0)
        row.pack_start(remove, False, False, 0)
        self.pack_start(row, False, False, 0)

    def set_items(self, items):
        self.store.clear()
        for item in items:
            self.store.append([item])

    def _add(self, _widget):
        values = self.entry.get_text().split()
        if values:
            self.entry.set_text("")
            self.on_add(values)

    def _remove(self, _button):
        model, iterator = self.view.get_selection().get_selected()
        if iterator:
            self.on_remove(model[iterator][0], model.get_path(iterator)[0] + 1)


class Window(Gtk.ApplicationWindow):
    def __init__(self, app):
        super().__init__(application=app, title="Open Turkey")
        self.set_default_size(820, 620)
        self.state_json = None
        self.blocks = {}
        self.selected = None
        self.busy = False

        header = Gtk.HeaderBar(show_close_button=True, title="Open Turkey")
        self.set_titlebar(header)
        new_block = Gtk.Button(label="Nytt block")
        new_block.connect("clicked", self.create_block)
        header.pack_start(new_block)
        self.freeze_button = Gtk.Button(label="Frozen Turkey")
        self.freeze_button.connect("clicked", self.freeze)
        header.pack_end(self.freeze_button)

        root = Gtk.Box(orientation=Gtk.Orientation.VERTICAL)
        self.add(root)
        self.banner = Gtk.Label(xalign=0, margin=8)
        root.pack_start(self.banner, False, False, 0)

        paned = Gtk.Paned()
        root.pack_start(paned, True, True, 0)
        self.block_list = Gtk.ListBox()
        self.block_list.connect("row-selected", self.block_selected)
        scroll = Gtk.ScrolledWindow(min_content_width=220)
        scroll.add(self.block_list)
        paned.pack1(scroll, False, False)

        self.detail = Gtk.Box(orientation=Gtk.Orientation.VERTICAL,
                              spacing=10, margin=12)
        paned.pack2(self.detail, True, False)
        self.title_label = Gtk.Label(xalign=0)
        self.status_label = Gtk.Label(xalign=0)
        self.detail.pack_start(self.title_label, False, False, 0)
        self.detail.pack_start(self.status_label, False, False, 0)

        controls = Gtk.Box(spacing=6)
        controls.pack_start(Gtk.Label(label="Tidslås (minuter):"), False, False, 0)
        self.minutes = Gtk.SpinButton.new_with_range(0, 10080, 5)
        controls.pack_start(self.minutes, False, False, 0)
        self.typing_lock = Gtk.CheckButton(label="Skrivutmaning")
        controls.pack_start(self.typing_lock, False, False, 0)
        self.start_button = Gtk.Button(label="Starta")
        self.start_button.connect("clicked", self.start_block)
        self.stop_button = Gtk.Button(label="Stoppa")
        self.stop_button.connect("clicked", self.stop_block)
        self.unlock_button = Gtk.Button(label="Lås upp …")
        self.unlock_button.connect("clicked", self.unlock_block)
        self.delete_button = Gtk.Button(label="Radera")
        self.delete_button.connect("clicked", self.delete_block)
        for button in (self.start_button, self.stop_button, self.unlock_button):
            controls.pack_start(button, False, False, 0)
        controls.pack_end(self.delete_button, False, False, 0)
        quota_row = Gtk.Box(spacing=6)
        quota_row.pack_start(
            Gtk.Label(label="Dagskvot (minuter):"), False, False, 0)
        self.quota_minutes = Gtk.SpinButton.new_with_range(1, 1440, 5)
        self.quota_minutes.set_value(30)
        quota_row.pack_start(self.quota_minutes, False, False, 0)
        self.quota_set = Gtk.Button(label="Spara kvot")
        self.quota_set.connect("clicked", self.set_quota)
        self.quota_start = Gtk.Button(label="Använd kvot")
        self.quota_start.connect("clicked", self.start_quota)
        self.quota_end = Gtk.Button(label="Avsluta")
        self.quota_end.connect("clicked", self.end_quota)
        self.quota_remove = Gtk.Button(label="Ta bort vid midnatt")
        self.quota_remove.connect("clicked", self.remove_quota)
        for button in (self.quota_set, self.quota_start, self.quota_end,
                       self.quota_remove):
            quota_row.pack_start(button, False, False, 0)
        self.detail.pack_start(controls, False, False, 0)
        self.detail.pack_start(quota_row, False, False, 0)

        self.sites = EditableList(
            "Webbplatser", "youtube.com reddit.com",
            lambda values: self.block_cmd("add-site", values),
            lambda value, _num: self.block_cmd("remove-site", [value]))
        self.apps = EditableList(
            "Program", "discord",
            lambda values: self.block_cmd("add-app", values),
            lambda value, _num: self.block_cmd("remove-app", [value]))
        self.schedules = EditableList(
            "Scheman", "weekdays 09:00 17:00",
            self.add_schedule, self.remove_schedule)
        for widget in (self.sites, self.apps, self.schedules):
            self.detail.pack_start(widget, True, True, 0)

        self.refresh()
        GLib.timeout_add_seconds(REFRESH_SECONDS, self.refresh)

    # Running commands ----------------------------------------------------
    def run(self, args):
        """Run a command in the background, then refresh."""
        self.set_sensitive(False)

        def work():
            ok, output = run_cli(args)
            GLib.idle_add(self.finished, ok, output)

        threading.Thread(target=work, daemon=True).start()

    def finished(self, ok, output):
        self.set_sensitive(True)
        if not ok:
            self.message(output or "Kommandot misslyckades.")
        self.refresh()
        return False

    def message(self, text):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=text,
            message_type=Gtk.MessageType.ERROR,
            buttons=Gtk.ButtonsType.OK)
        dialog.run()
        dialog.destroy()

    def confirm(self, text, detail):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text=text, secondary_text=detail,
            message_type=Gtk.MessageType.WARNING,
            buttons=Gtk.ButtonsType.OK_CANCEL)
        answer = dialog.run()
        dialog.destroy()
        return answer == Gtk.ResponseType.OK

    def block_cmd(self, command, values):
        if self.selected:
            self.run(["block", command, self.selected, *values])

    # State ---------------------------------------------------------------
    def refresh(self):
        if self.busy:
            return True
        self.busy = True

        def work():
            ok, output = run_cli(["gui-state"])
            GLib.idle_add(self.apply_state, ok, output)

        threading.Thread(target=work, daemon=True).start()
        return True

    def apply_state(self, ok, output):
        self.busy = False
        if not ok:
            self.banner.set_text(f"Kan inte läsa läget: {output}")
            return False
        if output == self.state_json:
            return False
        self.state_json = output
        state = json.loads(output)
        self.blocks = {block["name"]: block for block in state["blocks"]}

        frozen = state["freeze_seconds"]
        self.banner.set_text(
            f"Frozen Turkey är aktivt i {fmt_seconds(frozen)} till."
            if frozen else f"{sum(b['active'] for b in state['blocks'])} block aktiva")

        for row in self.block_list.get_children():
            self.block_list.remove(row)
        selected_row = None
        for name, block in self.blocks.items():
            label = Gtk.Label(label=("● " if block["active"] else "○ ") + name,
                              xalign=0, margin=8)
            row = Gtk.ListBoxRow()
            row.block_name = name
            row.add(label)
            self.block_list.add(row)
            if name == self.selected:
                selected_row = row
        self.block_list.show_all()
        if selected_row is None and self.block_list.get_children():
            selected_row = self.block_list.get_children()[0]
        if selected_row is not None:
            self.block_list.select_row(selected_row)
        self.show_block()
        return False

    def block_selected(self, _box, row):
        self.selected = row.block_name if row else None
        self.show_block()

    def show_block(self):
        block = self.blocks.get(self.selected)
        self.detail.set_sensitive(block is not None)
        if block is None:
            self.title_label.set_text("Skapa ett block för att börja")
            self.status_label.set_text("")
            for widget in (self.sites, self.apps, self.schedules):
                widget.set_items([])
            return
        self.title_label.set_markup(
            f"<big><b>{GLib.markup_escape_text(block['name'])}</b></big>")
        parts = ["Aktivt" if block["active"] else "Inaktivt"]
        if block["lock_seconds"]:
            parts.append(f"tidslåst i {fmt_seconds(block['lock_seconds'])} till")
        if block["char_locked"]:
            parts.append("skrivutmaning krävs")
        if block["scheduled"]:
            parts.append("styrs av schema")
        limit = block["quota_limit"]
        if limit:
            parts.append(f"kvot {block['quota_used'] // 60} av {limit // 60} min")
        if block["quota_open"]:
            parts.append("kvot används nu")
        if block["mode"] == "allowlist":
            parts.append("tillåtelselista")
        self.status_label.set_text(" · ".join(parts))
        self.sites.set_items(block["sites"])
        self.apps.set_items(block["apps"])
        self.schedules.set_items(block["schedules"])
        self.stop_button.set_sensitive(block["active"] and not block["held"])
        self.unlock_button.set_sensitive(
            block["char_locked"] and not block["lock_seconds"])
        self.quota_start.set_sensitive(
            limit > 0 and not block["quota_open"]
            and block["quota_used"] < limit)
        self.quota_end.set_sensitive(block["quota_open"])
        self.quota_remove.set_sensitive(limit > 0)
        self.delete_button.set_sensitive(
            not block["active"] and not block["held"])

    # Actions -------------------------------------------------------------
    def create_block(self, _button):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text="Namn på blocket",
            message_type=Gtk.MessageType.QUESTION,
            buttons=Gtk.ButtonsType.OK_CANCEL)
        entry = Gtk.Entry(activates_default=True)
        dialog.set_default_response(Gtk.ResponseType.OK)
        dialog.get_message_area().pack_start(entry, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        name = entry.get_text().strip()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and name:
            self.selected = name
            self.run(["block", "create", name])

    def start_block(self, _button):
        if not self.selected:
            return
        args = ["start", self.selected]
        minutes = self.minutes.get_value_as_int()
        if minutes:
            args += ["--for", f"{minutes}m"]
        if self.typing_lock.get_active():
            args.append("--lock")
        if minutes and not self.confirm(
                f"Tidslås i {minutes} minuter?",
                "Blocket går inte att stoppa förrän tiden har gått ut. "
                "Tid då datorn är avstängd räknas inte."):
            return
        self.run(args)

    def stop_block(self, _button):
        if self.selected:
            self.run(["stop", self.selected])

    def unlock_block(self, _button):
        if self.selected:
            command = f"{CLI} unlock {shlex.quote(self.selected)}"
            subprocess.Popen(["x-terminal-emulator", "-e", command])

    def delete_block(self, _button):
        if self.selected and self.confirm(
                f"Radera {self.selected}?", "Blocket och dess listor tas bort."):
            name, self.selected = self.selected, None
            self.run(["block", "remove", name])

    def set_quota(self, _button):
        if self.selected:
            minutes = self.quota_minutes.get_value_as_int()
            self.run(["quota", "set", self.selected, f"{minutes}m"])

    def start_quota(self, _button):
        if self.selected:
            self.run(["quota", "start", self.selected])

    def end_quota(self, _button):
        if self.selected:
            self.run(["quota", "end", self.selected])

    def remove_quota(self, _button):
        if self.selected:
            self.run(["quota", "remove", self.selected])

    def add_schedule(self, values):
        if len(values) != 3:
            self.message("Skriv dagar, start och slut, till exempel: "
                         "weekdays 09:00 17:00")
            return
        self.run(["schedule", "add", self.selected, "--days", values[0],
                  "--from", values[1], "--to", values[2]])

    def remove_schedule(self, _value, number):
        self.run(["schedule", "remove", self.selected, str(number)])

    def freeze(self, _button):
        dialog = Gtk.MessageDialog(
            transient_for=self, modal=True, text="Frozen Turkey",
            secondary_text="Skärmen låses om och om igen tills tiden är slut, "
                           "även om du startar om. Det går inte att avbryta "
                           "från skrivbordet. Spara ditt arbete först.",
            message_type=Gtk.MessageType.WARNING,
            buttons=Gtk.ButtonsType.OK_CANCEL)
        row = Gtk.Box(spacing=6)
        row.pack_start(Gtk.Label(label="Minuter (högst 1440):"), False, False, 0)
        minutes = Gtk.SpinButton.new_with_range(1, 1440, 5)
        minutes.set_value(30)
        row.pack_start(minutes, False, False, 0)
        dialog.get_message_area().pack_start(row, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        value = minutes.get_value_as_int()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK:
            self.run(["freeze", "--for", f"{value}m", "--yes"])


class App(Gtk.Application):
    def __init__(self):
        super().__init__(application_id="io.github.openturkey.Gui")

    def do_activate(self):
        window = self.get_active_window() or Window(self)
        window.show_all()
        window.present()


if __name__ == "__main__":
    App().run()
