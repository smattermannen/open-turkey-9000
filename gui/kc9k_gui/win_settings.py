# Window: Settings, hotkeys, browsers, setting locks.
# Part of the GTK interface, loaded by gui/kc9k-gui.py into its one namespace
# (imports and every other part's names are there; see notes/gui.md).

class _SettingsPages:
    """Methods of Window (window.py); no state of its own."""

    # Settings ------------------------------------------------------------
    def choose_darkness(self, scale):
        app = self.get_application()
        app.prefs["darkness"] = int(scale.get_value())
        save_prefs(app.prefs)
        app.theme.apply(app.prefs.get("theme", "system"), app.prefs["darkness"])

    def toggle_tray(self, _switch, wanted):
        app = self.get_application()
        app.prefs["tray"] = bool(wanted)
        save_prefs(app.prefs)
        app.set_tray(bool(wanted))
        return False

    def choose_clock(self, combo):
        global CLOCK_24
        app = self.get_application()
        app.prefs["clock_24"] = CLOCK_24 = combo.get_active_id() == "24"
        save_prefs(app.prefs)
        self.queue_draw()
        if hasattr(self, "calendar"):
            self.calendar.queue_draw()

    def choose_first_day(self, combo):
        global FIRST_DAY
        app = self.get_application()
        app.prefs["first_day"] = combo.get_active_id()
        FIRST_DAY = 6 if app.prefs["first_day"] == "sunday" else 0
        save_prefs(app.prefs)
        if hasattr(self, "calendar"):
            self.calendar.queue_draw()

    def toggle_open_at_login(self, _switch, wanted):
        """§4.10: a desktop autostart entry for this window."""
        try:
            if wanted:
                os.makedirs(os.path.dirname(LOGIN_DESKTOP), exist_ok=True)
                with open(LOGIN_DESKTOP, "w") as f:
                    f.write("[Desktop Entry]\nType=Application\nName=Killer Chicken 9000\n"
                            "Exec=kc9k-gui\nIcon=changes-prevent\nTerminal=false\n")
            elif os.path.exists(LOGIN_DESKTOP):
                os.remove(LOGIN_DESKTOP)
        except OSError as error:
            self.message(f"Can't change opening at login: {error}")
            return True
        return False

    def choose_theme(self, combo):
        app = self.get_application()
        app.prefs["theme"] = combo.get_active_id()
        app.theme.apply(app.prefs["theme"], app.prefs.get("darkness"))
        save_prefs(app.prefs)

    def build_settings(self):
        """Groups in the order of SPEC §4.10."""
        scroll, box = page_box()
        page_header(box, "Settings", "How Killer Chicken looks, what protects it, and what it may never do.")

        theme = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        for key, label in THEMES:
            theme.append(key, label)
        theme.set_active_id(self.get_application().prefs.get("theme", "system"))
        theme.connect("changed", self.choose_theme)
        row = Gtk.Box(spacing=16)
        row.pack_start(Gtk.Label(label="Theme", xalign=0), True, True, 0)
        row.pack_end(theme, False, False, 0)
        darkness = Gtk.Scale.new_with_range(Gtk.Orientation.HORIZONTAL, 0, 100, 5)
        darkness.set_size_request(160, -1)
        darkness.set_value(self.get_application().prefs.get("darkness", 0))
        darkness.connect("value-changed", self.choose_darkness)
        dark_row = Gtk.Box(spacing=16)
        dark_row.pack_start(Gtk.Label(label="Darkness (dark theme): grey to black", xalign=0), True, True, 0)
        dark_row.pack_end(darkness, False, False, 0)
        self.darkness_scale = darkness  # for tests
        first_day = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        first_day.append("monday", "Monday")
        first_day.append("sunday", "Sunday")
        first_day.set_active_id(self.get_application().prefs.get("first_day", "monday"))
        first_day.connect("changed", self.choose_first_day)
        day_row = Gtk.Box(spacing=16)
        day_row.pack_start(Gtk.Label(label="First day of the week", xalign=0), True, True, 0)
        day_row.pack_end(first_day, False, False, 0)
        clock = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        clock.append("24", "24-hour (14:30)")
        clock.append("12", "12-hour (2:30 PM)")
        clock.set_active_id("24" if self.get_application().prefs.get("clock_24", True) else "12")
        clock.connect("changed", self.choose_clock)
        self.clock_combo = clock  # for tests
        self.tray_switch = Gtk.Switch(valign=Gtk.Align.CENTER,
                                      active=bool(self.get_application().prefs.get("tray")))
        self.tray_switch.connect("state-set", self.toggle_tray)
        clock_row = Gtk.Box(spacing=16)
        clock_row.pack_start(Gtk.Label(label="Clock", xalign=0), True, True, 0)
        clock_row.pack_end(clock, False, False, 0)
        self.login_switch = Gtk.Switch(valign=Gtk.Align.CENTER, active=os.path.exists(LOGIN_DESKTOP))
        self.login_switch.connect("state-set", self.toggle_open_at_login)
        box.pack_start(section("Preferences", row, dark_row, clock_row, day_row,
                               self.switch_row(self.login_switch, "Open at login",
                                               "Start this window when you sign in."),
                               self.switch_row(self.tray_switch, "Show in the tray",
                                               "An icon in the panel shows what is on; closing the "
                                               "window keeps it there.")), False, False, 0)

        self.no_weakening_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.no_weakening_switch.connect("state-set", self.toggle_no_weakening)
        box.pack_start(section(
            "No weakening during active locks",
            self.switch_row(self.no_weakening_switch, "Keep every setting while anything is locked",
                            "While any block or setting is locked, no setting can be weakened: "
                            "strict mode, browser settings, bypass prevention, the settings "
                            "password. Blocks without a lock stay free to edit. It can't be "
                            "turned off while anything is locked.")), False, False, 0)

        # Tamper protection: one group, changed in its own window (OK/Cancel).
        self.browser_switches = {}
        self.setting_locks = {}
        self.tamper_summary = Gtk.Label(xalign=0, wrap=True)
        tamper_change = Gtk.Button(label="Change…", halign=Gtk.Align.START)
        tamper_change.connect("clicked", self.tamper_dialog)
        tamper_rows = [
            dim("Closes the ways around your blocks from inside the computer. Once on, a tamper "
                "protection setting can't be turned off while anything is locked."),
            self.tamper_summary, tamper_change]
        # Block unsupported browsers keeps its own lock until the group has one.
        tamper_rows.append(self.setting_lock_line("unsupported-browsers", "Block unsupported browsers"))
        box.pack_start(section("Tamper protection", *tamper_rows), False, False, 0)

        # Browsers (§4.10): supported browsers found, extension status,
        # one-click install. Rows are made once the state names them.
        self.browser_rows = {}
        self.browser_list = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        self.extension_status = dim("")
        box.pack_start(section(
            "Browsers",
            dim("The Killer Chicken extension runs in the supported browsers: LibreWolf (a "
                "privacy-focused Firefox), Google Chrome and Chromium, which install it themselves. "
                "Firefox and other browsers can't run it. The extension is always on: it enforces "
                "wildcards, page titles and text, greyscale, images and web-only, and while a block "
                "has websites, browsers without it are closed."),
            self.browser_list, self.extension_status), False, False, 0)

        rows = [self.setting_row(name, title, detail) for name, title, detail in BROWSER_OPTIONS]
        box.pack_start(section(
            "Browser settings", dim("Each browser setting can be locked on its own with Lock…, "
                                    "independently of your blocks. A locked setting can't be turned "
                                    "off until its lock ends or is unlocked. Blocking developer "
                                    "tools also can't be turned off while a block is locked."), *rows),
            False, False, 0)

        # §3.7 use counting.
        self.strict_use_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.strict_use_switch.connect("state-set", self.toggle_strict_use)
        box.pack_start(section(
            "Use counting",
            self.switch_row(self.strict_use_switch, "Strict use counting",
                            "An allowance is spent while any tab of its sites is open and while "
                            "its apps run, even in the background or minimised. Turning it off "
                            "weakens your allowances.")), False, False, 0)

        # §4.10 notifications: which ones, and how many minutes ahead.
        self.notice_checks = {}
        notice_flow = Gtk.FlowBox(selection_mode=Gtk.SelectionMode.NONE, max_children_per_line=3,
                                  column_spacing=12, row_spacing=2)
        for key, title in NOTICE_KINDS:
            check = Gtk.CheckButton(label=title)
            check.connect("toggled", self.toggle_notice, key)
            self.notice_checks[key] = check
            notice_flow.add(check)
        advance_row = Gtk.Box(spacing=6)
        advance_row.pack_start(Gtk.Label(label="Minutes ahead:"), False, False, 0)
        self.notice_advance = Gtk.Entry(width_chars=10, placeholder_text="2,5")
        advance_apply = Gtk.Button(label="Apply")
        advance_apply.connect("clicked", self.apply_notice_advance)
        advance_row.pack_start(self.notice_advance, False, False, 0)
        advance_row.pack_start(advance_apply, False, False, 0)
        box.pack_start(section(
            "Notifications",
            dim("Desktop notifications before things happen. A scheduled Frozen Chicken is "
                "always announced."),
            notice_flow, advance_row), False, False, 0)

        dns_row = Gtk.Box(spacing=6)
        self.dns_choice = Gtk.ComboBoxText()
        for key, label in (("network", "The network's own server, then Quad9"),
                           ("quad9", "Quad9 only"), ("own", "My own servers")):
            self.dns_choice.append(key, label)
        self.dns_choice.connect("changed", self.dns_choice_changed)
        self.dns_servers = Gtk.Entry(placeholder_text="e.g. 192.168.1.1, 9.9.9.9", no_show_all=True)
        dns_apply = Gtk.Button(label="Apply")
        dns_apply.connect("clicked", self.apply_dns)
        dns_row.pack_start(self.dns_choice, False, False, 0)
        dns_row.pack_start(self.dns_servers, True, True, 0)
        dns_row.pack_end(dns_apply, False, False, 0)
        box.pack_start(section(
            "DNS",
            dim("The firewall needs the real addresses of blocked sites, so Killer Chicken 9000 "
                "asks a DNS server for them. By default that is the server the computer already "
                "uses, so nobody new learns which sites are blocked."),
            dns_row), False, False, 0)

        self.strict_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.strict_switch.connect("state-set", self.toggle_strict)
        self.reset_guard_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.reset_guard_switch.connect("state-set", self.toggle_reset_guard)
        self.no_escape_switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        self.no_escape_switch.connect("state-set", self.toggle_no_escape)
        box.pack_start(section(
            "Device protection",
            self.locked_switch_row(self.strict_switch, "strict", "Strict mode: take away sudo and pkexec during locks",
                                   "While a block or setting is locked or Frozen Chicken runs, no user "
                                   "can get administrator rights, so the blocks can't be undone from a "
                                   "terminal. Rights return when the lock ends. Strict mode itself can "
                                   "be turned off at any time unless you lock it."),
            self.strict_exempt_list(),
            self.switch_row(self.no_escape_switch, "No-escape: close the ways out through the boot menu",
                            "Sets a random boot menu password that nobody knows and hides the "
                            "recovery entries, so the emergency exit (kc9k.nofreeze) stops "
                            "working. Normal boot entries, Windows included, still start. The "
                            "only way past a lock is then booting a USB stick or reinstalling. "
                            "It can't be turned off during a lock."),
            self.switch_row(self.reset_guard_switch, "Factory reset protection: installer media guard",
                            "While anything is locked, the download pages of operating systems and USB "
                            "image writers are blocked and image writers (USB Image Writer, Etcher, "
                            "Disks, …) are closed. It can't be turned off during a lock."),
            self.device_setup_row()), False, False, 0)

        # §4.10, R-MODE-5 (owner 2026-11-17): what shows when no mode or
        # circadian rhythm wallpaper does, and while the place isn't known.
        self.wallpaper_default = Gtk.ComboBoxText(valign=Gtk.Align.CENTER)
        for key, label in WALLPAPER_DEFAULTS:
            self.wallpaper_default.append(key, label)
        self.wallpaper_default.connect("changed", self.wallpaper_default_changed)
        default_row = Gtk.Box(spacing=16)
        default_row.pack_start(Gtk.Label(label="Default to", xalign=0), True, True, 0)
        default_row.pack_end(self.wallpaper_default, False, False, 0)
        self.wallpaper_custom_row = Gtk.Box(spacing=10, no_show_all=True)
        self.wallpaper_custom_thumb = Gtk.Box()
        self.wallpaper_custom_name = Gtk.Label(xalign=0, ellipsize=Pango.EllipsizeMode.MIDDLE, max_width_chars=30)
        choose = Gtk.Button(label="Choose a picture…", valign=Gtk.Align.CENTER)
        choose.connect("clicked", self.choose_wallpaper_default)
        for widget in (self.wallpaper_custom_thumb, self.wallpaper_custom_name):
            self.wallpaper_custom_row.pack_start(widget, False, False, 0)
        self.wallpaper_custom_row.pack_end(choose, False, False, 0)
        self.wallpaper_default_note = dim("")
        box.pack_start(section(
            "Wallpaper default",
            dim("What the desktop shows when no mode and no circadian rhythm wallpaper does, and while "
                "Killer Chicken can't tell where you are for a mode switched by places (no Wi-Fi, the "
                "first seconds after starting or waking)."),
            default_row, self.wallpaper_custom_row, self.wallpaper_default_note), False, False, 0)
        self.wallpaper_default_shown = None

        self.password_status = Gtk.Label(xalign=0, wrap=True)
        password_row = Gtk.Box(spacing=6)
        self.password_set_button = Gtk.Button(label="Set password…")
        self.password_set_button.connect("clicked", self.set_settings_password)
        self.password_remove_button = Gtk.Button(label="Remove password…")
        self.password_remove_button.connect("clicked", self.remove_settings_password)
        self.password_partner_button = Gtk.Button(label="Partner holds it…")
        self.password_partner_button.set_tooltip_text(
            "Killer Chicken makes the password up and sends it only to your accountability partners (§4.15)")
        self.password_partner_button.connect("clicked", self.partner_settings_password)
        password_row.pack_start(self.password_set_button, False, False, 0)
        password_row.pack_start(self.password_partner_button, False, False, 0)
        password_row.pack_start(self.password_remove_button, False, False, 0)
        self.password_locks = Gtk.CheckButton(label="Also needed to set a lock, so nobody else can lock me out")
        self.password_locks.connect("toggled", self.toggle_password_locks)

        self.access_status = Gtk.Label(xalign=0, wrap=True)
        access_row = Gtk.Box(spacing=6)
        access_set = Gtk.Button(label="Set a password…")
        access_set.connect("clicked", self.set_access_password)
        self.access_remove_button = Gtk.Button(label="Remove…", no_show_all=True)
        self.access_remove_button.connect("clicked", self.remove_access_password)
        access_row.pack_start(access_set, False, False, 0)
        access_row.pack_start(self.access_remove_button, False, False, 0)
        idle_row = Gtk.Box(spacing=6)
        idle_row.pack_start(Gtk.Label(label="Lock the pages again after", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
        idle = DurationField(self.get_application().prefs.get("access_idle_minutes", 0) * 60, minimum=60,
                             maximum=7 * 86400, zero="Never", width=9)

        def save_idle(field):
            if field.is_valid():
                prefs = self.get_application().prefs
                prefs["access_idle_minutes"] = field.get_value_as_int()
                save_prefs(prefs)

        idle.on_change(save_idle)
        idle_row.pack_start(idle, False, False, 0)
        idle_row.pack_start(Gtk.Label(label="without using this window", valign=Gtk.Align.START, margin_top=6),
                            False, False, 0)
        self.idle_field = idle  # for tests
        top_align(idle_row)
        box.pack_start(section(
            "Passwords",
            Gtk.Label(label="Settings password", xalign=0, margin_top=6),
            dim("Guards Killer Chicken 9000 against anything running as you, and lets someone "
                "else decide when your blocks may be weakened: weakening, stopping, unlocking, "
                "turning browser settings off, strict mode, no-escape and uninstalling all ask "
                "for it. Starting a block and adding sites don't."),
            self.password_status, password_row, self.password_locks,
            Gtk.Label(label="Access passwords", xalign=0, margin_top=6),
            dim("Optional passwords to open Killer Chicken 9000 and each of its pages: "
                f"{', '.join(AREA_TITLES[a] for a in AREAS[1:-1])} and {AREA_TITLES[AREAS[-1]]}. Use the same one "
                "for several, or a different one for each. A locked page shows nothing until "
                "its password is given; the Lock button at the top locks them again. "
                "Changing or removing one asks for the one it has."),
            self.access_status, access_row, idle_row), False, False, 0)

        self.nepsis_label = Gtk.Label(xalign=0, wrap=True)
        nepsis_row = Gtk.Box(spacing=6)
        change = Gtk.Button(label="Change…")
        change.connect("clicked", self.change_global_nepsis)
        off = Gtk.Button(label="Turn off…")
        off.connect("clicked", self.nepsis_off)
        self.nepsis_off_button = off
        nepsis_row.pack_start(change, False, False, 0)
        nepsis_row.pack_start(off, False, False, 0)
        box.pack_start(section(
            "Bypass prevention",
            dim("When Killer Chicken stops many things in a short time (blocked pages, closed apps "
                "and windows, browsers without the extension, wrong access "
                "passwords), you are likely hunting for a loophole. It then takes the consequence you "
                "choose, so you really step away. Each block can have its own too."),
            self.nepsis_label, nepsis_row), False, False, 0)


        project_row = Gtk.Box(spacing=6)
        for label, handler in (("Back up…", self.export_project), ("Share…", self.share_project),
                               ("Import…", self.import_project)):
            button = Gtk.Button(label=label)
            button.connect("clicked", handler)
            project_row.pack_start(button, False, False, 0)
        box.pack_start(section(
            "Project",
            dim("Save every block and setting to one file. A backup also holds every preset, your "
                "places (the Wi-Fi networks they are known by), running locks, passwords (as hashes), "
                "hotkeys and statistics; a file to share leaves those out and asks about presets. "
                "Importing shows what it adds first, only adds, and gives a block or preset whose "
                "name is taken a new name."),
            project_row), False, False, 0)

        self.option_switches = {}
        option_rows = []
        for name, title, detail in OPTIONS:
            switch = Gtk.Switch(valign=Gtk.Align.CENTER)
            switch.connect("state-set", self.toggle_option, name)
            self.option_switches[name] = switch
            option_rows.append(self.switch_row(switch, title, detail))
        box.pack_start(section("Behaviour settings", *option_rows), False, False, 0)
        return scroll

    def tamper_dialog(self, _button=None):
        """The tamper protection group in its own window: nothing changes until
        OK, and turning one on is confirmed by OK (SPEC §4.10)."""
        current = self.tamper_state()
        dialog = Gtk.Dialog(title="Tamper protection", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("OK", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.CANCEL)
        box = dialog.get_content_area()
        box.set_spacing(12)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(16)
        box.pack_start(dim("Once on, a tamper protection setting can't be turned off while any "
                           "block or setting is locked. Turning one off asks for the settings "
                           "password."), False, False, 0)
        has_supported = any(b.get("installed") for b in self.state.get("browsers") or [])
        checks = {}
        for key, title, detail, _command in TAMPER_SETTINGS:
            check = Gtk.CheckButton(label=title)
            check.set_active(current[key])
            note = detail
            if key == "unsupported-browsers" and self.setting_locked(key):
                check.set_sensitive(False)
                note += " It has its own lock now."
            elif key == "unsupported-browsers" and not current[key] and not has_supported:
                check.set_sensitive(False)
                note += " Install a supported browser first (Browsers, below)."
            elif current[key] and self.state.get("anything_locked"):
                # §4.10: stuck on while anything is locked.
                check.set_sensitive(False)
                note += " Something is locked, so it stays on."
            checks[key] = check
            item = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=2)
            item.pack_start(check, False, False, 0)
            text = dim(note)
            text.set_margin_start(26)
            item.pack_start(text, False, False, 0)
            box.pack_start(item, False, False, 0)
        self.tamper_checks = checks  # for tests
        grace_row = Gtk.Box(spacing=8)
        grace_row.pack_start(Gtk.Label(label="Close browsers without the extension after",
                                       valign=Gtk.Align.START, margin_top=6), False, False, 0)
        grace = DurationField(self.state.get("tamper_grace", 60), minimum=10, maximum=600, bare=1, width=8)
        grace_row.pack_start(grace, False, False, 0)
        grace_row.pack_start(Gtk.Label(label="(longer weakens)", valign=Gtk.Align.START, margin_top=6),
                             False, False, 0)
        top_align(grace_row)
        ok_button = dialog.get_widget_for_response(Gtk.ResponseType.OK)
        if ok_button is not None:
            guard_button(ok_button, grace)
        box.pack_start(grace_row, False, False, 0)
        self.tamper_grace = grace  # for tests
        dialog.show_all()
        answer = dialog.run()
        wanted = {key: check.get_active() for key, check in checks.items()}
        new_grace = grace.get_value_as_int()
        dialog.destroy()
        if answer != Gtk.ResponseType.OK:
            return
        if new_grace != self.state.get("tamper_grace", 60):
            self.run(["tamper", "grace", str(new_grace)], then=lambda _o: self.apply_tamper(current, wanted))
            return
        self.apply_tamper(current, wanted)

    def apply_tamper(self, current, wanted):
        """Runs one command per changed tamper protection setting, in turn."""
        commands = [list(command) + ["on" if wanted[key] else "off"]
                    for key, _title, _detail, command in TAMPER_SETTINGS
                    if wanted[key] != current[key]]

        def next_command(_output=None):
            if commands:
                self.run(commands.pop(0), then=next_command)

        next_command()

    def tamper_state(self):
        return {"clock": bool(self.state.get("tamper_clock", False)),
                "file-urls": bool(self.state.get("tamper_file_urls", False)),
                "task-managers": bool(self.state.get("tamper_task_managers", False)),
                "private": bool(self.state.get("tamper_private", False)),
                "guest": bool(self.state.get("tamper_guest", False)),
                "extension-stop": bool(self.state.get("tamper_extension_stop", True)),
                "installer": bool(self.state.get("tamper_installer", False)),
                "inactive-tabs": bool(self.state.get("tamper_inactive_tabs", True)),
                "embedded": bool(self.state.get("tamper_embedded", True)),
                "unsupported-browsers": bool(self.state.get("unsupported_browsers", False))}

    def setting_lock_info(self, name):
        """A lockable setting's lock from the state: strict mode, block
        unsupported browsers or a browser setting."""
        if name == "strict":
            return self.state.get("strict_lock") or {}
        if name == "unsupported-browsers":
            return self.state.get("unsupported_browsers_lock") or {}
        return (self.state.get("browser_locks") or {}).get(name) or {}

    @staticmethod
    def setting_command(name, verb):
        """The command that turns a lockable setting on, or unlocks it."""
        if name == "strict":
            return ["strict", verb]
        if name == "unsupported-browsers":
            return ["tamper", "unlock", name] if verb == "unlock" else ["tamper", name, verb]
        return ["browser", "unlock", name] if verb == "unlock" else ["browser", name, verb]

    def setting_locked(self, name):
        return bool(describe_setting_lock(self.setting_lock_info(name)))

    def build_hotkeys_page(self):
        """§4.13 (owner 2026-11-17): hotkeys on their own page: what each
        press does (hotkey actions, §3.9), added, removed, saved as presets."""
        scroll, box = page_box()
        page_header(box, "Hotkeys", "Keys that do what you choose: start and lock blocks and modes, open your "
                    "sites, files and programs, close the browsers, lock the screen…",
                    help="A hotkey runs its actions in order when its keys are pressed. One that locks "
                         "something asks you to confirm at each press, unless you chose Run without "
                         "confirmation when adding it (its locks are then confirmed only then). One that "
                         "switches something off needs the settings password when it is added, and is "
                         "refused at the press while that thing is locked. The keys show in the desktop's "
                         "keyboard settings as KC9K: ….")
        self.hotkey_ids = []
        self.hotkeys = EditableList(
            "Hotkeys", "", None,
            lambda _value, number: self.run(["hotkey", "remove", str(self.hotkey_ids[number - 1])]),
            entry=False)
        for label, handler, tip in (("Add hotkey…", self.add_hotkey_actions, "Keys and the actions they run"),
                                    ("Start a block…", self.add_hotkey, "Keys that start a block, with locks"),
                                    ("Save as preset…", self.save_hotkey_preset, "The selected hotkey, to add again")):
            button = Gtk.Button(label=label, tooltip_text=tip)
            button.connect("clicked", lambda _b, h=handler: h(None))
            self.hotkeys.button_row.pack_start(button, False, False, 0)
        for position, child in enumerate([c for c in self.hotkeys.button_row.get_children()
                                          if isinstance(c, Gtk.Button) and c.get_label() in
                                          ("Add hotkey…", "Start a block…", "Save as preset…")]):
            self.hotkeys.button_row.reorder_child(child, position)
        box.pack_start(self.hotkeys, True, True, 0)
        return scroll

    def save_hotkey_preset(self, _button=None):
        chosen = self.hotkeys._selected()
        if not chosen:
            self.message("Pick a hotkey in the list first.", Gtk.MessageType.INFO)
            return
        number = self.hotkey_ids[chosen[-1][1] - 1]
        name = self.ask_text("Save as preset", "A name for a copy of this hotkey, to add again later.")
        if name:
            self.run(["preset", "save", "hotkey", str(number), name],
                     then=lambda output: self.message(output, Gtk.MessageType.INFO))

    HOTKEY_KINDS = (("start", "Start a block"), ("stop", "Switch a block off"), ("mode-on", "Switch a mode on"),
                    ("mode-off", "Switch a mode off"), ("mode-run", "Run a mode, locked, for"),
                    ("monk", "Monk mode for"), ("freeze", "Frozen Chicken for"),
                    ("close-browser", "Close the browsers"), ("lock-screen", "Lock the screen"),
                    ("logout", "Log out"), ("shutdown", "Shut down"), ("custom", "Custom action"))

    def ask_hotkey_action(self, answer=None):
        """One hotkey action (§3.9): its kind and what it needs. Returns
        (action dict, words, password secret or None) or None. answer: for
        tests."""
        if answer is not None:
            return answer
        blocks = sorted(n for n, b in self.blocks.items() if not b.get("draft_of"))
        modes = [m["name"] for m in self.state.get("modes") or [] if not m.get("monk") and not m.get("freeze")]
        dialog = Gtk.Dialog(title="Add an action", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Add", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        box = dialog.get_content_area()
        box.set_spacing(10)
        box.set_border_width(14)
        kind = Gtk.ComboBoxText()
        for key, label in self.HOTKEY_KINDS:
            kind.append(key, label)
        kind.set_active_id("start" if blocks else "custom")
        box.pack_start(kind, False, False, 0)
        target = Gtk.ComboBoxText()
        length = DurationField(3600, minimum=60, maximum=30 * 86400, zero="no time (once)", width=9)
        how = Gtk.ComboBoxText()
        for key, label in (("lock", "Lock the screen"), ("logout", "Log out"), ("shutdown", "Shut down"),
                           ("apps", "Allowed apps only")):
            how.append(key, label)
        how.set_active_id("lock")
        custom = CustomEditor(self)
        for widget in (target, length, how, custom):
            box.pack_start(widget, False, False, 0)

        def shown(_combo=None):
            k = kind.get_active_id()
            target.remove_all()
            names = blocks if k in ("start", "stop") else modes if k in ("mode-on", "mode-off", "mode-run") else []
            for n in names:
                target.append_text(n)
            if names:
                target.set_active(0)
            target.set_visible(bool(names) or k in ("start", "stop", "mode-on", "mode-off", "mode-run"))
            timed = k in ("mode-run", "monk", "freeze", "lock-screen", "logout", "shutdown")
            length.set_visible(timed)
            length.set_limits(60, 7 * 86400 if k in ("freeze", "lock-screen", "logout", "shutdown") else 30 * 86400)
            how.set_visible(k == "freeze")
            custom.set_visible(k == "custom")
        kind.connect("changed", shown)
        self.hotkey_action_parts = (dialog, kind, target, length, how, custom)  # for tests
        dialog.set_default_size(520, -1)
        dialog.show_all()
        shown()
        reply = dialog.run()
        k, t, minutes = kind.get_active_id(), target.get_active_text(), length.minutes if length.is_valid() else None
        freeze_how, items = how.get_active_id(), custom.items
        label = dict(self.HOTKEY_KINDS)[k]
        dialog.destroy()
        if reply != Gtk.ResponseType.OK:
            return None
        action = {"kind": k}
        secret = None
        words = label
        if k in ("start", "stop", "mode-on", "mode-off", "mode-run"):
            if not t:
                self.message("There is nothing to choose: make a block or a mode first.", Gtk.MessageType.INFO)
                return None
            action["target"] = t
            words = f"{label} {t}" if k != "mode-run" else f"Run {t}, locked,"
        if k in ("mode-run", "monk", "freeze", "lock-screen", "logout", "shutdown"):
            if minutes is None:
                self.message("How long: type a time such as 1h or 30m.", Gtk.MessageType.INFO)
                return None
            if minutes:
                action["minutes"] = minutes
                words += f" for {fmt_minutes(minutes)}" if not words.endswith(" for") else f" {fmt_minutes(minutes)}"
            elif k in ("mode-run", "monk", "freeze"):
                self.message("How long: at least a minute.", Gtk.MessageType.INFO)
                return None
        if k == "freeze":
            action["param"] = freeze_how
        if k == "custom":
            if not items:
                self.message("Add something to open or run first.", Gtk.MessageType.INFO)
                return None
            action["param"] = json.dumps([{i.split(" ", 1)[0]: i.split(" ", 1)[1]} for i in items])
            words = "; ".join(items)
        if k == "start":
            picker = LockPicker("stop", "stopped", LockPicker.ALL_KINDS, relock_any=True)
            asked = self.lock_dialog(f"Start {t}", "Locks each press sets (none: it just starts it). On a block "
                                     "that is on, a press adds its locks again (a kind it has: a second lock).", "OK", picker)
            if asked is None:
                return None
            chosen, relock_flags, secret = asked
            if len(chosen) > 1:
                chosen = self.choose_lock_order(chosen)
                if chosen is None:
                    return None
            flags = [f for flags_, _text in chosen for f in flags_] + relock_flags
            if flags:
                action["lock_args"] = flags
                words += " with " + ", then ".join(text for _f, text in chosen)
        return action, words, secret

    def add_hotkey_actions(self, _button=None, answer=None):
        """A hotkey with any actions in order (§3.9 Hotkey action, §4.13):
        its keys, its actions (Add action…, Remove), Run without
        confirmation. answer: (keys, [(action, words, secret)], no_confirm)
        for tests."""
        if answer is None:
            dialog = Gtk.Dialog(title="Add a hotkey", transient_for=self, modal=True)
            dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
            ok = dialog.add_button("Add", Gtk.ResponseType.OK)
            ok.get_style_context().add_class("suggested-action")
            box = dialog.get_content_area()
            box.set_spacing(10)
            box.set_border_width(14)
            keys = Gtk.Entry(placeholder_text="ctrl+alt+s", width_chars=18)
            note = dim("ctrl, alt, shift or super with a key; F1 to F24 also work alone.")
            box.pack_start(heading("Keys", "medium"), False, False, 0)
            box.pack_start(keys, False, False, 0)
            box.pack_start(note, False, False, 0)
            box.pack_start(heading("What a press does, in order", "medium"), False, False, 0)
            rows = Gtk.ListBox(selection_mode=Gtk.SelectionMode.NONE)
            rows.get_style_context().add_class("frame")
            box.pack_start(rows, False, False, 0)
            actions = []

            def redraw():
                for row in rows.get_children():
                    rows.remove(row)
                for number, (_a, words, _s) in enumerate(actions):
                    line = Gtk.Box(spacing=8, margin=6)
                    line.pack_start(Gtk.Label(label=f"{number + 1}. {words}", xalign=0, wrap=True), True, True, 0)
                    remove = Gtk.Button(label="Remove", relief=Gtk.ReliefStyle.NONE)
                    remove.connect("clicked", lambda _b, n=number: (actions.pop(n), redraw(), check()))
                    line.pack_start(remove, False, False, 0)
                    rows.add(line)
                rows.show_all()

            add = Gtk.Button(label="Add action…", halign=Gtk.Align.START)

            def add_action(_b):
                got = self.ask_hotkey_action()
                if got:
                    actions.append(got)
                    redraw()
                    check()
            add.connect("clicked", add_action)
            box.pack_start(add, False, False, 0)
            unasked = Gtk.CheckButton(label="Run without confirmation (a press that locks then asks nothing)")
            box.pack_start(unasked, False, False, 0)

            def check(*_args):
                problem = hotkey_problem(keys.get_text())
                typed = bool(keys.get_text().strip())
                mark_problem(keys, problem if typed else None)
                note.set_text(problem if typed and problem else
                              "ctrl, alt, shift or super with a key; F1 to F24 also work alone.")
                ok.set_sensitive(not problem and bool(actions))
            keys.connect("changed", check)
            self.hotkey_dialog = (dialog, keys, actions, unasked, add_action)  # for tests
            dialog.set_default_size(560, -1)
            dialog.show_all()
            check()
            reply = dialog.run()
            typed, chosen, no_confirm = keys.get_text().strip(), list(actions), unasked.get_active()
            dialog.destroy()
            if reply != Gtk.ResponseType.OK:
                return
        else:
            typed, chosen, no_confirm = answer
        args = ["hotkey", "add", typed]
        secrets = [s for _a, _w, s in chosen if s]
        locking = []
        for action, words, _s in chosen:
            args += ["--do-json", json.dumps(action)]
            if action.get("lock_args") or action["kind"] in ("mode-run", "monk", "freeze") or \
                    (action["kind"] in ("lock-screen", "logout", "shutdown") and action.get("minutes")):
                locking.append(f"{words}, each time {typed} is pressed")
        if locking:
            # R-LOCK-12: the locks are confirmed now (and at each press, unless
            # Run without confirmation).
            self.lock_option = False
            if not self.confirm_lock(locking, option=None if no_confirm else
                                     "Run without confirmation (pressing the hotkey then locks without asking again)"):
                return
            args.append("--yes")
            if no_confirm or self.lock_option:
                args.append("--no-confirm")
        self.run(args, secrets[0] + "\n" if secrets else None)

    def add_hotkey(self, _button=None):
        """A new hotkey: its keys, the block it starts and its locks, in the
        lock picker every start uses (§3.13, R-GUI-2). The locks are
        confirmed now (R-LOCK-12); pressing asks again unless Run without
        confirmation is ticked."""
        blocks = sorted(name for name, b in self.blocks.items() if not b.get("draft_of"))
        if not blocks:
            self.message("Make a block first: a hotkey starts one.", Gtk.MessageType.INFO)
            return
        fields = HotkeyFields(blocks, self.selected)
        picker = LockPicker("stop", "stopped", LockPicker.ALL_KINDS, relock_any=True)
        asked = self.lock_dialog(
            "Add a hotkey", "Pressing the keys starts the block with these locks (none: it just "
            "starts). On a block that is already on, a press adds its locks again (a kind it has: a second lock).",
            "Add", picker, extra=fields)
        if asked is None:
            return
        keys, block = fields.values
        chosen, relock_flags, secret = asked
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        args = ["hotkey", "add", keys, block]
        locks = []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"block '{block}': {order}{text}, set each time {keys} is pressed")
        args += relock_flags
        locks += relock_lines(f"block '{block}'", relock_flags)
        if locks:
            if not self.confirm_lock(locks, option="Run without confirmation (pressing the hotkey then "
                                                    "locks without asking again)"):
                return
            args.append("--yes")
            if self.lock_option:
                args.append("--no-confirm")
        self.run(args, secret + "\n" if secret else None)

    def set_settings_password(self, _button):
        new = self.ask_text("New settings password", "At least 4 characters.", password=True)
        if not new:
            return
        if self.ask_text("Repeat the new password", password=True) != new:
            self.message("The two passwords differ; nothing changed.", Gtk.MessageType.INFO)
            return
        self.run(["password", "set"], new + "\n",
                 then=lambda _o: setattr(self, "settings_password", new))

    def partner_settings_password(self, _button):
        """§4.15: a settings password only the accountability partners get."""
        if self.confirm("Let your accountability partner hold the settings password?",
                        "Killer Chicken makes a new password up and sends it to your partners; you never see it. "
                        "To weaken anything later, ask them (Account > Ask to unlock…).", "Send it to them"):
            self.run(["password", "set", "--partner-holds"],
                     then=lambda o: (setattr(self, "settings_password", None),
                                     self.message(o, Gtk.MessageType.INFO)))

    def remove_settings_password(self, _button):
        if self.confirm("Remove the settings password?",
                        "Weakening and locks won't ask for it any more.", "Remove"):
            self.run(["password", "remove"], then=lambda _o: setattr(self, "settings_password", None))

    def update_wallpaper_default(self):
        w = self.state.get("wallpaper") or {}
        key = json.dumps(w, sort_keys=True)
        if key == self.wallpaper_default_shown:
            return
        self.wallpaper_default_shown = key
        was, self.updating = self.updating, True
        self.wallpaper_default.set_active_id(w.get("default") or "latest")
        self.updating = was
        custom = w.get("default") == "custom" and w.get("custom_path")
        for child in self.wallpaper_custom_thumb.get_children():
            self.wallpaper_custom_thumb.remove(child)
        if custom:
            self.wallpaper_custom_thumb.add(picture_thumb(w["custom_path"]))
        self.wallpaper_custom_name.set_text(w.get("custom_name") or "")
        self.wallpaper_custom_row.set_visible(bool(custom))
        if custom:
            self.wallpaper_custom_row.show_all()
        self.wallpaper_default_note.set_text({
            "latest": "The wallpaper stays as it is. At a new place without Wi-Fi that can be the last "
                      "place's wallpaper until Killer Chicken knows where you are.",
            "circadian": "Circadian rhythm's wallpaper for the time of day, even with circadian rhythm "
                         "switched off.",
            "custom": "This picture shows until Killer Chicken knows where you are. It also goes on before "
                      "the computer sleeps, so it wakes with it, not with the last place's wallpaper.",
        }.get(w.get("default") or "latest", ""))

    def wallpaper_default_changed(self, combo):
        if self.updating:
            return
        key = combo.get_active_id()
        if key != "custom":
            self.run(["wallpaper", "default", key])
        elif (self.state.get("wallpaper") or {}).get("custom_path"):
            self.run(["wallpaper", "default", "custom"])
        else:
            self.choose_wallpaper_default(None)

    def choose_wallpaper_default(self, _button, path=None):
        """Custom: the picture shown by default. path: for tests."""
        path = path or choose_picture(self, "Choose the default wallpaper")
        self.wallpaper_default_shown = None  # cancelled: the box shows the setting again
        if path:
            self.run(["wallpaper", "default", "custom", path])
        else:
            self.update_wallpaper_default()

    def update_settings(self):
        self.update_wallpaper_default()
        lock = self.state.get("nepsis_lock", {})
        text = describe_nepsis(self.state.get("nepsis"), self.state.get("nepsis_count"))
        if lock.get("lock_seconds"):
            text += f" Locked for {fmt_seconds(lock['lock_seconds'])}."
        self.nepsis_label.set_text(text)
        self.nepsis_off_button.set_visible(bool((self.state.get("nepsis") or {}).get("steps")))
        for name, switch in self.browser_switches.items():
            set_switch(switch, self.state["browser"].get(name, False))
        notices = self.state.get("notifications") or {}
        off = set(notices.get("off") or [])
        was, self.updating = self.updating, True
        try:
            for key, check in self.notice_checks.items():
                check.set_active(key not in off)
        finally:
            self.updating = was
        if not self.notice_advance.has_focus():
            self.notice_advance.set_text(",".join(str(m) for m in notices.get("advance") or [5]))
        for name, (label, unlock) in self.setting_locks.items():
            info = self.setting_lock_info(name)
            parts = describe_setting_lock(info)
            text = "Locked: " + ", ".join(parts) if parts else ""
            if info.get("relock_in"):
                # Unlocked with re-lock: on again by itself unless turned off.
                text = (text + "; " if text else "") + \
                    f"unlocked, locks on again in {fmt_seconds(info['relock_in'])}"
            label.set_text(text)
            label.set_visible(bool(text))
            unlock.set_visible(info.get("char_locked", False) or info.get("password_locked", False))
            order = info.get("lock_order") or []
            # R-LOCK-6: only the first lock still on can be unlocked.
            unlock.set_sensitive(bool(order) and order[0] in ("text", "password"))
            # A locked setting is on and stays on, so its switch can't be used.
            if name in self.browser_switches:
                self.browser_switches[name].set_sensitive(not parts)
        set_switch(self.strict_switch, self.state["strict"])
        set_switch(self.strict_use_switch, self.state.get("strict_use", False))
        self.strict_exempt.set_items(self.state.get("strict_exempt") or [])
        self.strict_switch.set_sensitive(not self.setting_locked("strict"))
        set_switch(self.no_escape_switch, self.state["no_escape"])
        set_switch(self.reset_guard_switch, self.state.get("reset_guard", False))
        tamper = self.tamper_state()
        on = [title for key, title, _detail, _command in TAMPER_SETTINGS if tamper[key]]
        self.tamper_summary.set_markup(
            "<b>On:</b> " + GLib.markup_escape_text("; ".join(on)) if on else "<b>All off.</b>")
        self.update_browsers()
        for name, switch in self.option_switches.items():
            set_switch(switch, (self.state.get("options") or {}).get(name, False))
        has_password = self.state.get("settings_password", False)
        self.password_status.set_markup("<b>On.</b>" if has_password else "Not set.")
        self.password_set_button.set_label("Change password…" if has_password else "Set password…")
        self.password_remove_button.set_visible(has_password)
        self.password_locks.set_visible(has_password)
        self.updating, was = True, self.updating
        self.password_locks.set_active(self.state.get("password_for_locks", False))
        self.updating = was
        set_switch(self.no_weakening_switch, self.state.get("no_weakening", False))
        dns = self.state.get("dns") or "network"
        if dns in ("network", "quad9"):
            self.dns_choice.set_active_id(dns)
        else:
            self.dns_choice.set_active_id("own")
            self.dns_servers.set_text(dns)

    def update_hotkeys_page(self):
        hotkeys = self.state.get("hotkeys") or []
        self.hotkey_ids = [h["id"] for h in hotkeys]
        self.hotkeys.set_items([
            f"{hotkey_words(h['keys'])}  {h.get('does') or ''}" + (" (runs without confirmation)" if h.get("no_confirm") else "")
            for h in hotkeys])

    def update_browsers(self):
        """The Browsers list: each supported browser, installed or not, its
        extension's last check-in, and Install."""
        browsers = self.state.get("browsers") or []
        for b in browsers:
            if b["id"] not in self.browser_rows:
                row = Gtk.Box(spacing=12)
                name = Gtk.Label(xalign=0)
                name.set_markup(f"<b>{GLib.markup_escape_text(b['name'])}</b>")
                name.set_size_request(140, -1)
                status = dim("")
                install = Gtk.Button(label="Install", valign=Gtk.Align.CENTER)
                install.set_no_show_all(True)
                install.connect("clicked", self.install_browser, b["id"], b["name"])
                row.pack_start(name, False, False, 0)
                row.pack_start(status, True, True, 0)
                row.pack_end(install, False, False, 0)
                self.browser_list.pack_start(row, False, False, 0)
                row.show_all()
                self.browser_rows[b["id"]] = (status, install)
            status, install = self.browser_rows[b["id"]]
            status.set_text(browser_status(b))
            install.set_visible(not b.get("installed") and not b.get("waiting"))  # KC9K installs only ready ones
        any_installed = any(b.get("installed") for b in browsers)
        if any_installed:
            self.extension_status.set_text("Extension: " + self.state.get("extension_status", ""))
        else:
            self.extension_status.set_text(
                "No supported browser is installed, so the extension can't run: wildcards, rendered terms, "
                "greyscale, images and web-only aren't enforced in the browser, and no browser is "
                "closed. Install one above.")

    def install_browser(self, _button, browser_id, name):
        if self.confirm(f"Install {name}?",
                        BROWSER_INSTALL_NOTES.get(browser_id, "") +
                        " This needs an internet connection and can take a few minutes.",
                        "Install"):
            self.browser_rows[browser_id][0].set_text("Installing, this can take a few minutes…")
            self.run(["browsers", "install", browser_id], timeout=900,
                     then=lambda output: self.message(output, Gtk.MessageType.INFO))

    def setting_row(self, name, title, detail):
        """A browser setting: its switch, plus its lock state and buttons."""
        switch = Gtk.Switch(valign=Gtk.Align.CENTER)
        switch.connect("state-set", self.toggle_browser, name)
        self.browser_switches[name] = switch
        return self.locked_switch_row(switch, name, title, detail)

    def locked_switch_row(self, switch, name, title, detail):
        """A setting's row with its switch, Lock… and Unlock… beside it and
        the lock's state under its description (browser settings, strict
        mode)."""
        row = self.switch_row(switch, title, detail)
        status, lock, unlock = self.setting_lock_parts(name, title)
        row.get_children()[0].pack_start(status, False, False, 0)
        for button in (unlock, lock):
            button.set_valign(Gtk.Align.CENTER)
            row.pack_end(button, False, False, 0)
        return row

    def setting_lock_parts(self, name, title):
        """A setting lock's state, Lock… and Unlock…"""
        status = dim("", wrap=False)
        status.set_no_show_all(True)
        lock = Gtk.Button(label="Lock…")
        lock.connect("clicked", self.lock_setting, name, title)
        unlock = Gtk.Button(label="Unlock…")
        unlock.set_no_show_all(True)
        unlock.connect("clicked", self.unlock_setting, name, title)
        self.setting_locks[name] = (status, unlock)
        return status, lock, unlock

    def setting_lock_line(self, name, title):
        """A setting lock's state with Lock… and Unlock… on a line of their
        own (block unsupported browsers, whose switch is in the tamper
        protection window)."""
        line = Gtk.Box(spacing=8)
        status, lock, unlock = self.setting_lock_parts(name, title)
        line.pack_start(status, False, False, 0)
        line.pack_end(unlock, False, False, 0)
        line.pack_end(lock, False, False, 0)
        return line

    def lock_setting(self, _button, name, title):
        picker = LockPicker("turn it off", "turned off",
                            ("timer", "password", "text", "restart", "range", "relock"))
        asked = self.lock_dialog(
            f"Lock: {title}",
            "The setting is turned on and can't be turned off until the locks end. More locks can be "
            "added later, after these; a lock that is on is never changed.", "Turn on and lock", picker, need_lock=True)
        if asked is None:
            return
        chosen, relock_flags, secret = asked
        if not chosen:
            self.message("Choose at least one lock.", Gtk.MessageType.INFO)
            return
        # R-LOCK-6: the locks unlock one at a time in the order of their flags.
        if len(chosen) > 1:
            chosen = self.choose_lock_order(chosen)
            if chosen is None:
                return
        args = self.setting_command(name, "on")
        locks = []
        for i, (flags, text) in enumerate(chosen):
            args += flags
            order = f"{i + 1} of {len(chosen)}: " if len(chosen) > 1 else ""
            locks.append(f"{title}: {order}{text}")
        args += relock_flags
        if not self.confirm_lock(locks):
            return
        self.run(args + ["--yes"], secret + "\n" if secret else None)

    def unlock_setting(self, _button, name, title):
        info, unlock = self.setting_lock_info(name), self.setting_command(name, "unlock")
        order = info.get("lock_order") or []
        if (order[:1] == ["password"]) or (info.get("password_locked") and not info.get("char_locked")):
            password = self.ask_text(f"Unlock {title}", "Enter the setting's password.", password=True)
            if password:
                self.run(unlock, password + "\n")
            return
        self.run_challenge(unlock)

    def strict_exempt_list(self):
        """§3.8: folders programs may still start from during strict mode's
        lockdown. Adding one weakens strict mode."""
        self.strict_exempt = EditableList(
            "", "/home/me/bin", lambda dirs: [self.run(["strict", "exempt", "add", d]) for d in dirs],
            lambda d, *_: self.run(["strict", "exempt", "remove", d]),
            help_text="Exempt folders: programs in them still start while strict mode holds "
                      "(adding one weakens strict mode).",
            split=lambda text: [text.strip()] if text.strip() else [], multi=True)
        return self.strict_exempt

    def toggle_strict_use(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["use-counting", "strict" if wanted else "normal"])
        return True

    def toggle_notice(self, check, key):
        if self.updating:
            return
        self.run(["notifications", key, "on" if check.get_active() else "off"])

    def apply_notice_advance(self, _button):
        self.run(["notifications", "advance", self.notice_advance.get_text().replace(" ", "")])

    def toggle_browser(self, _switch, wanted, name):
        if self.updating:
            return False
        self.run(["browser", name, "on" if wanted else "off"])
        return True

    def toggle_option(self, _switch, wanted, name):
        if self.updating:
            return False
        self.run(["behaviour", name, "on" if wanted else "off"])
        return True

    def update_social(self, block):
        """Builds the social media boxes once, from the state's list, and ticks
        the block's."""
        if not self.social_checks:
            site = None
            for preset in self.state.get("social_presets") or []:
                if preset["site"] != site:
                    site = preset["site"]
                    self.social_box.pack_start(heading(site, "medium"), False, False, 0)
                check = Gtk.CheckButton(label=preset["title"])
                check.connect("toggled", self.toggle_social, preset["name"])
                self.social_checks[preset["name"]] = check
                self.social_box.pack_start(check, False, False, 0)
            self.social_box.show_all()
        on = block.get("social") or []
        for name, check in self.social_checks.items():
            check.set_active(name in on)
        count = sum(1 for name in self.social_checks if name in on)
        self.social_expander.set_label("<b>Social media</b>" + (
            f"  <small>{count} {'setting' if count == 1 else 'settings'} on</small>" if count else ""))

    def save_place(self, _button, target=None, flags=()):
        """§3.6: saves the Wi-Fi networks in range as a place and, ticked by
        default, adds it to the automation of target (a mode's name) or of
        the selected block."""
        dialog = Gtk.Dialog(title="Save this place", transient_for=self, modal=True)
        dialog.add_button("Cancel", Gtk.ResponseType.CANCEL)
        ok = dialog.add_button("Save", Gtk.ResponseType.OK)
        ok.get_style_context().add_class("suggested-action")
        dialog.set_default_response(Gtk.ResponseType.OK)
        box = dialog.get_content_area()
        box.set_spacing(10)
        for side in ("start", "end", "top", "bottom"):
            getattr(box, f"set_margin_{side}")(18)
        box.pack_start(dim("A name for where the computer is now, such as home or library. Saving under a "
                           "name that exists adds the networks seen now."), False, False, 0)
        entry = Gtk.Entry(activates_default=True)
        box.pack_start(entry, False, False, 0)
        rule = None
        target = target or self.selected
        if target:
            rule = Gtk.CheckButton(label=f"Add it to '{target}' (being here switches it)", active=True)
            box.pack_start(rule, False, False, 0)
        ok.set_sensitive(False)
        entry.connect("changed", lambda e: ok.set_sensitive(bool(e.get_text().strip())))
        self.place_dialog = (dialog, entry, rule)  # for tests
        dialog.show_all()
        answer = dialog.run()
        name, with_rule = entry.get_text().strip(), bool(rule and rule.get_active())
        dialog.destroy()
        if answer != Gtk.ResponseType.OK or not name:
            return
        block = target
        self.place_to_show = name

        def saved(output):
            if with_rule and block:
                self.run(["place", "rule", block, name, "on", *flags],
                         then=lambda rule_output: self.message(output + "\n" + rule_output, Gtk.MessageType.INFO))
            else:
                self.message(output, Gtk.MessageType.INFO)

        self.run(["place", "add", name], timeout=60, then=saved)

    def pick_apps(self, _button):
        if not self.selected:
            return
        dialog = AppPicker(self)
        answer = dialog.run()
        chosen = dialog.chosen()
        dialog.destroy()
        if answer == Gtk.ResponseType.OK and chosen:
            self.block_cmd("add-app", chosen)

    def pick_app_folder(self, _button):
        path = self.choose_file("Block every program in a folder", Gtk.FileChooserAction.SELECT_FOLDER, "Add")
        if path and self.selected:
            self.block_cmd("add-app", [path.rstrip("/") + "/"])

    def pick_app_file(self, _button):
        path = self.choose_file("Block a program file (and copies of it)", Gtk.FileChooserAction.OPEN, "Add")
        if path and self.selected:
            self.block_cmd("add-app", [path])

    def toggle_social(self, check, name):
        if self.updating or not self.selected:
            return
        self.run(["social", self.selected, name, "on" if check.get_active() else "off"])

    def toggle_keyword_test(self, _switch, wanted):
        if self.updating or not self.selected:
            return False
        self.run(["list", "test", self.selected, "on" if wanted else "off"])
        return True

    def toggle_stats(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["stats", "on" if wanted else "off"], then=lambda _o: self.refresh_stats())
        return True

    def toggle_stats_private(self, _switch, wanted):
        if self.updating:
            return False
        self.run(["stats", "private", "skip" if wanted else "count"])
        return True

    def clear_stats(self, _button):
        if self.confirm("Delete all statistics?", "Every recorded day is removed. This can't be undone.",
                        "Delete", destructive=True):
            self.run(["stats", "clear"], then=lambda _o: self.refresh_stats())
