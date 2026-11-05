#!/usr/bin/env python3
"""Graphical interface for Killer Chicken 9000 (GTK 3).

All changes go through the kc9k command, so the interface follows the
same lock rules as the terminal and needs no privileges of its own.
"""
import json
import shlex
import subprocess
import threading

import gi

gi.require_version("Gtk", "3.0")
from gi.repository import GLib, Gtk  # noqa: E402

CLI = "kc9k"
REFRESH_SECONDS = 5


def fmt_seconds(seconds):
    hours, rest = divmod(int(seconds), 3600)
    return f"{hours} h {rest // 60} min" if hours else f"{max(rest // 60, 1)} min"


def run_cli(args, secret=None):
    """Return (ok, output) for one command."""
    try:
        result = subprocess.run(
            [CLI, *args], input=secret if secret else "",
            capture_output=True, text=True, timeout=120,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        return False, str(error)
    return result.returncode == 0, (result.stdout + result.stderr).strip()


class EditableList(Gtk.Box):
    """A titled list with an entry for adding and a button for removing."""

    def __init__(self, title, placeholder, on_add, on_remove,
                 on_import=None, on_export=None):
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
        for label_text, handler in (("Importera …", on_import),
                                    ("Exportera …", on_export)):
            if handler:
                button = Gtk.Button(label=label_text)
                button.connect("clicked", lambda _b, h=handler: h())
                row.pack_start(button, False, False, 0)
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
        super().__init__(application=app, title="Killer Chicken 9000")
        self.set_default_size(820, 620)
        self.state_json = None
        self.blocks = {}
        self.selected = None
        self.busy = False
        self.strict = False
        self.browser = {}

        header = Gtk.HeaderBar(show_close_button=True, title="Killer Chicken 9000")
        self.set_titlebar(header)
        new_block = Gtk.Button(label="Nytt block")
        new_block.connect("clicked", self.create_block)
        header.pack_start(new_block)
        self.freeze_button = Gtk.Button(label="Frozen Chicken")
        self.freeze_button.connect("clicked", self.freeze)
        header.pack_end(self.freeze_button)
        browser_button = Gtk.Button(label="Webbläsare")
        browser_button.connect("clicked", self.browser_options)
        header.pack_end(browser_button)
        self.strict_button = Gtk.Button(label="Strikt läge")
        self.strict_button.connect("clicked", self.toggle_strict)
        header.pack_end(self.strict_button)

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
        self.restart_lock = Gtk.CheckButton(label="Till omstart")
        controls.pack_start(self.restart_lock, False, False, 0)
        controls.pack_start(Gtk.Label(label="Fördröjning (min):"), False, False, 0)
        self.delay_minutes = Gtk.SpinButton.new_with_range(0, 57600, 5)
        controls.pack_start(self.delay_minutes, False, False, 0)
        self.password_entry = Gtk.Entry(
            placeholder_text="Lösenordslås", visibility=False, width_chars=12)
        controls.pack_start(self.password_entry, False, False, 0)
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

        pomodoro_row = Gtk.Box(spacing=6)
        pomodoro_row.pack_start(Gtk.Label(label="Pomodoro, arbete:"), False, False, 0)
        self.work_minutes = Gtk.SpinButton.new_with_range(1, 240, 5)
        self.work_minutes.set_value(25)
        pomodoro_row.pack_start(self.work_minutes, False, False, 0)
        pomodoro_row.pack_start(Gtk.Label(label="paus:"), False, False, 0)
        self.break_minutes = Gtk.SpinButton.new_with_range(1, 240, 5)
        self.break_minutes.set_value(5)
        pomodoro_row.pack_start(self.break_minutes, False, False, 0)
        set_pomodoro = Gtk.Button(label="Spara")
        set_pomodoro.connect("clicked", self.set_pomodoro)
        clear_pomodoro = Gtk.Button(label="Stäng av")
        clear_pomodoro.connect("clicked", self.clear_pomodoro)
        rename = Gtk.Button(label="Byt namn")
        rename.connect("clicked", self.rename_block)
        duplicate = Gtk.Button(label="Duplicera")
        duplicate.connect("clicked", self.duplicate_block)
        for button in (set_pomodoro, clear_pomodoro):
            pomodoro_row.pack_start(button, False, False, 0)
        for button in (duplicate, rename):
            pomodoro_row.pack_end(button, False, False, 0)
        self.detail.pack_start(pomodoro_row, False, False, 0)

        self.sites = EditableList(
            "Blockerat", "reddit.com  reddit.com/r/funny  *.xxx  *.*",
            lambda values: self.block_cmd("add", values),
            lambda value, _num: self.block_cmd("remove-entry", [value]),
            lambda: self.import_list("blocked"),
            lambda: self.export_list("blocked"))
        self.exceptions = EditableList(
            "Undantag", "reddit.com/r/programming",
            lambda values: self.block_cmd("add-exception", values),
            lambda value, _num: self.block_cmd("remove-exception", [value]),
            lambda: self.import_list("exceptions"),
            lambda: self.export_list("exceptions"))
        self.apps = EditableList(
            "Program", "discord  /opt/spel/",
            lambda values: self.block_cmd("add-app", values),
            lambda value, _num: self.block_cmd("remove-app", [value]))
        self.schedules = EditableList(
            "Scheman", "weekdays 09:00 17:00",
            self.add_schedule, self.remove_schedule)
        for widget in (self.sites, self.exceptions, self.apps, self.schedules):
            self.detail.pack_start(widget, True, True, 0)
        self.limited_label = Gtk.Label(xalign=0, wrap=True)
        self.detail.pack_start(self.limited_label, False, False, 0)

        self.refresh()
        GLib.timeout_add_seconds(REFRESH_SECONDS, self.refresh)

    # Running commands ----------------------------------------------------
    def run(self, args, secret=None):
        """Run a command in the background, then refresh."""
        self.set_sensitive(False)

        def work():
            ok, output = run_cli(args, secret)
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

        self.strict = state["strict"]
        self.browser = state["browser"]
        self.strict_button.set_label(
            "Strikt läge: på" if self.strict else "Strikt läge: av")
        frozen = state["freeze_seconds"]
        self.banner.set_text(
            f"Frozen Chicken är aktivt i {fmt_seconds(frozen)} till."
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
            for widget in (self.sites, self.exceptions, self.apps,
                           self.schedules):
                widget.set_items([])
            self.limited_label.set_text("")
            return
        self.title_label.set_markup(
            f"<big><b>{GLib.markup_escape_text(block['name'])}</b></big>")
        parts = ["Aktivt" if block["active"] else "Inaktivt"]
        if block["lock_seconds"]:
            parts.append(f"tidslåst i {fmt_seconds(block['lock_seconds'])} till")
        if block["pomodoro_on"]:
            parts.append(
                f"Pomodoro {block['pomodoro_on'] // 60}+{block['pomodoro_off'] // 60} min")
        if block["on_break"]:
            parts.append("paus just nu")
        if block["char_locked"]:
            parts.append("skrivutmaning krävs")
        if block["password_locked"]:
            parts.append("lösenordslåst")
        if block["restart_locked"]:
            parts.append("låst till omstart")
        if block["stop_seconds"]:
            parts.append(f"stoppas om {fmt_seconds(block['stop_seconds'])}")
        elif block["delay_seconds"]:
            parts.append(f"fördröjning {fmt_seconds(block['delay_seconds'])}")
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
        self.exceptions.set_items(block["exceptions"])
        limited = block["firefox_only"]
        self.limited_label.set_text(
            "Fungerar bara i Firefox: " + ", ".join(limited) if limited else "")
        self.apps.set_items(block["apps"])
        self.schedules.set_items(block["schedules"])
        self.stop_button.set_sensitive(
            block["active"] and (not block["held"] or (
                block["delay_seconds"] > 0 and not block["lock_seconds"]
                and not block["char_locked"] and not block["password_locked"]
                and not block["restart_locked"] and not block["quota_limit"])))
        self.unlock_button.set_sensitive(
            (block["char_locked"] or block["password_locked"])
            and not block["lock_seconds"] and not block["restart_locked"])
        self.quota_start.set_sensitive(
            limit > 0 and not block["quota_open"]
            and block["quota_used"] < limit)
        self.quota_end.set_sensitive(block["quota_open"])
        self.quota_remove.set_sensitive(limit > 0)
        self.delete_button.set_sensitive(
            not block["active"] and not block["held"])

    # Actions -------------------------------------------------------------
    def ask_name(self):
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
        return name if answer == Gtk.ResponseType.OK else ""

    def create_block(self, _button):
        name = self.ask_name()
        if name:
            self.selected = name
            self.run(["block", "create", name])

    def rename_block(self, _button):
        name = self.ask_name()
        if self.selected and name:
            old, self.selected = self.selected, name
            self.run(["block", "rename", old, name])

    def duplicate_block(self, _button):
        name = self.ask_name()
        if self.selected and name:
            self.run(["block", "duplicate", self.selected, name])

    def set_pomodoro(self, _button):
        if self.selected:
            self.run(["pomodoro", "set", self.selected,
                      f"{self.work_minutes.get_value_as_int()}m",
                      f"{self.break_minutes.get_value_as_int()}m"])

    def clear_pomodoro(self, _button):
        if self.selected:
            self.run(["pomodoro", "off", self.selected])

    def start_block(self, _button):
        if not self.selected:
            return
        args = ["start", self.selected]
        minutes = self.minutes.get_value_as_int()
        if minutes:
            args += ["--for", f"{minutes}m"]
        if self.typing_lock.get_active():
            args.append("--lock")
        if self.restart_lock.get_active():
            args.append("--restart")
        delay = self.delay_minutes.get_value_as_int()
        if delay:
            args += ["--delay", f"{delay}m"]
        password = self.password_entry.get_text()
        if password:
            args.append("--password-stdin")
        if minutes and not self.confirm(
                f"Tidslås i {minutes} minuter?",
                "Blocket går inte att stoppa förrän tiden har gått ut. "
                "Tid då datorn är avstängd räknas inte."):
            return
        self.password_entry.set_text("")
        self.run(args, password + "\n" if password else None)

    def stop_block(self, _button):
        if self.selected:
            self.run(["stop", self.selected])

    def unlock_block(self, _button):
        block = self.blocks.get(self.selected)
        if block and block["password_locked"] and not block["char_locked"]:
            password = self.password_entry.get_text()
            if not password:
                self.message("Skriv lösenordet i rutan Lösenordslås först.")
                return
            self.password_entry.set_text("")
            self.run(["unlock", self.selected], password + "\n")
            return
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

    def choose_file(self, title, action, button):
        dialog = Gtk.FileChooserDialog(
            title=title, transient_for=self, action=action)
        dialog.add_buttons("Avbryt", Gtk.ResponseType.CANCEL,
                           button, Gtk.ResponseType.OK)
        if action == Gtk.FileChooserAction.SAVE:
            dialog.set_do_overwrite_confirmation(True)
            dialog.set_current_name(f"{self.selected}.txt")
        answer = dialog.run()
        path = dialog.get_filename()
        dialog.destroy()
        return path if answer == Gtk.ResponseType.OK else None

    def import_list(self, kind):
        if not self.selected:
            return
        path = self.choose_file(
            "Importera lista", Gtk.FileChooserAction.OPEN, "Importera")
        if path:
            self.run(["block", "import-list", self.selected, kind, path])

    def export_list(self, kind):
        if not self.selected:
            return
        path = self.choose_file(
            "Exportera lista", Gtk.FileChooserAction.SAVE, "Spara")
        if not path:
            return
        ok, output = run_cli(["block", "export-list", self.selected, kind])
        if not ok:
            self.message(output)
            return
        try:
            with open(path, "w", encoding="utf-8") as file:
                file.write(output + "\n" if output else "")
        except OSError as error:
            self.message(str(error))

    def browser_options(self, _button):
        labels = (
            ("extension", "Tvinga webbläsartillägget"),
            ("force-librewolf", "Tvinga LibreWolf (stäng andra webbläsare)"),
            ("force-search", "Tvinga Google som sökmotor"),
            ("safesearch", "Tvinga Googles SafeSearch"),
            ("web-only", "Bara webbresultat (inga bilder, köp eller AI)"),
            ("images", "Blockera bilder"),
            ("greyscale", "Gråskala"),
        )
        dialog = Gtk.Dialog(title="Webbläsare", transient_for=self, modal=True)
        dialog.add_button("Stäng", Gtk.ResponseType.CLOSE)
        box = dialog.get_content_area()
        box.set_spacing(6)
        note = Gtk.Label(
            label="Under lås går inställningarna inte att stänga av.", xalign=0)
        box.pack_start(note, False, False, 6)
        checks = {}
        for name, label in labels:
            check = Gtk.CheckButton(label=label)
            check.set_active(self.browser.get(name, False))
            box.pack_start(check, False, False, 0)
            checks[name] = check
        dialog.show_all()
        dialog.run()
        changes = [(name, check.get_active()) for name, check in checks.items()
                   if check.get_active() != self.browser.get(name, False)]
        dialog.destroy()
        for name, enabled in changes:
            ok, output = run_cli(["browser", name, "on" if enabled else "off"])
            if not ok:
                self.message(output)
        self.refresh()

    def toggle_strict(self, _button):
        if self.strict:
            self.run(["strict", "off"])
        elif self.confirm(
                "Slå på strikt läge?",
                "Under lås nekas sudo och pkexec för alla användare, och du "
                "kan inte installera eller ändra systemet. Nödutgång: lägg "
                "till kc9k.nofreeze på linux-raden i GRUB."):
            self.run(["strict", "on"])

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
            transient_for=self, modal=True, text="Frozen Chicken",
            secondary_text="Skärmen låses om och om igen tills tiden är slut, "
                           "även om du startar om. Det går inte att avbryta "
                           "från skrivbordet. Spara ditt arbete först.",
            message_type=Gtk.MessageType.WARNING,
            buttons=Gtk.ButtonsType.OK_CANCEL)
        mode = Gtk.ComboBoxText()
        mode.append("lock", "Lås skärmen")
        mode.append("logout", "Logga ut")
        mode.append("shutdown", "Stäng av datorn")
        mode.set_active_id("lock")
        dialog.get_message_area().pack_start(mode, False, False, 0)
        row = Gtk.Box(spacing=6)
        row.pack_start(Gtk.Label(label="Minuter (högst 1440):"), False, False, 0)
        minutes = Gtk.SpinButton.new_with_range(1, 1440, 5)
        minutes.set_value(30)
        row.pack_start(minutes, False, False, 0)
        dialog.get_message_area().pack_start(row, False, False, 0)
        dialog.show_all()
        answer = dialog.run()
        value = minutes.get_value_as_int()
        chosen = mode.get_active_id()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK:
            self.run(["freeze", "--for", f"{value}m", "--mode", chosen, "--yes"])


class App(Gtk.Application):
    def __init__(self):
        super().__init__(application_id="io.github.kc9k.Gui")

    def do_activate(self):
        window = self.get_active_window() or Window(self)
        window.show_all()
        window.present()


if __name__ == "__main__":
    App().run()
