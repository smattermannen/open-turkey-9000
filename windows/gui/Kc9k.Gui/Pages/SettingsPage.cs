// §4.10 Settings, in SPEC's order (gui/kc9k-gui.py build_settings):
// preferences, no weakening, tamper protection (one group, changed in its
// own dialog), browsers, browser settings with their locks, use counting,
// notifications, DNS, device protection (strict mode with its lock and
// exempt folders, no-escape, factory reset protection, firmware lock,
// guided setup), wallpaper default, passwords, bypass prevention, project,
// behaviour settings. Worded for Windows; what each does is the CLI's.
using System.Text.Json;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class SettingsPage : IPage
{
    readonly MainWindow window;
    readonly ToggleSwitch noWeakening, strictUse, strict, noEscape, resetGuard, tray, login;
    readonly TextBlock tamperSummary = Ui.Text(""), extensionStatus = Ui.Dim(""), nepsisLabel = Ui.Text(""), passwordStatus = Ui.Text(""), accessStatus = Ui.Text(""), wallpaperNote = Ui.Dim(""), wallpaperName = Ui.Dim("");
    readonly StackPanel browserList = Ui.Column(6);
    readonly Dictionary<string, ToggleSwitch> browserSwitches = [];
    readonly Dictionary<string, (TextBlock Status, Button Unlock)> settingLocks = [];
    readonly Dictionary<string, CheckBox> notices = [];
    readonly Dictionary<string, ToggleSwitch> options = [];
    readonly TextBox advance, dnsServers;
    readonly ComboBox dnsChoice, wallpaperDefault;
    readonly Button nepsisOff, passwordSet, passwordRemove, accessRemove;
    readonly CheckBox passwordLocks;
    readonly EditableList strictExempt;
    readonly UIElement wallpaperCustom;
    readonly ScrollViewer scroll;
    readonly FrameworkElement browsersSection;
    bool filling;
    Controller C => window.Controller;
    public string Area => "settings";
    public UIElement View => scroll;

    public SettingsPage(MainWindow window)
    {
        this.window = window;
        var prefs = Prefs.Current;
        // Preferences ---------------------------------------------------------
        var theme = Combo("Theme", "PrefTheme", Tables.Themes, prefs.Theme);
        theme.SelectionChanged += (_, _) => { prefs.Theme = Key(theme); prefs.Save(); window.ApplyTheme(); };
        var darkness = new Slider { Minimum = 0, Maximum = 100, StepFrequency = 5, Value = prefs.Darkness, Width = 200 }.Named("Darkness of the dark theme", "PrefDarkness");
        darkness.ValueChanged += (_, _) => { prefs.Darkness = (int)darkness.Value; prefs.Save(); window.ApplyTheme(); };
        var clock = Combo("Clock", "PrefClock", [("24", "24-hour (14:30)"), ("12", "12-hour (2:30 PM)")], prefs.Clock24 ? "24" : "12");
        clock.SelectionChanged += (_, _) => { prefs.Clock24 = Key(clock) == "24"; prefs.Save(); C.Refresh(); };
        var firstDay = Combo("First day of the week", "PrefFirstDay", [("monday", "Monday"), ("sunday", "Sunday")], prefs.FirstDay == 6 ? "sunday" : "monday");
        firstDay.SelectionChanged += (_, _) => { prefs.FirstDay = Key(firstDay) == "sunday" ? 6 : 0; prefs.Save(); C.Refresh(); };
        login = Ui.Switch("Open at sign-in", Startup.IsOn(), on => Startup.Set(window, on), "PrefLogin");
        tray = Ui.Switch("Show in the notification area", prefs.Tray, on => { prefs.Tray = on; prefs.Save(); }, "PrefTray");
        var openOn = Combo("Open on", "PrefOpenOn", [("dashboard", "The Overview"), ("last", "The page open last")], prefs.OpenOn);
        openOn.SelectionChanged += (_, _) => { prefs.OpenOn = Key(openOn); prefs.Save(); };
        var preferences = Ui.Section("Preferences",
            Ui.SettingRow("Theme", "", theme),
            Ui.SettingRow("Darkness (dark theme): grey to black", "", darkness),
            Ui.SettingRow("Clock", "", clock),
            Ui.SettingRow("First day of the week", "", firstDay),
            Ui.SettingRow("Open at sign-in", "Start this window, in the notification area, when you sign in.", login),
            Ui.SettingRow("Show in the notification area", "An icon by the clock shows what is on; closing the window keeps it there.", tray),
            Ui.SettingRow("Open on", "The page the window opens on. Its size and place are kept too.", openOn));

        noWeakening = Ui.Switch("Keep every setting while anything is locked", false, on => _ = NoWeakening(on), "NoWeakening");
        var noWeak = Ui.Section("No weakening during active locks",
            Ui.SettingRow("Keep every setting while anything is locked", "While any block or setting is locked, no setting can be weakened: strict mode, browser settings, bypass prevention, the settings password. Blocks without a lock stay free to edit. It can't be turned off while anything is locked.", noWeakening));

        var tamper = Ui.Section("Tamper protection",
            Ui.Dim("Closes the ways around your blocks from inside the computer. Once on, a tamper protection setting can't be turned off while anything is locked."),
            tamperSummary, Ui.Button("Change…", () => _ = TamperDialog(), "TamperChange"),
            LockLine("unsupported-browsers", "Block unsupported browsers"));

        browsersSection = Ui.Section("Browsers",
            Ui.Dim("The Killer Chicken extension runs in the supported browsers, which install it themselves. Other browsers can't run it. The extension is always on: it enforces wildcards, page titles and text, greyscale, images and web-only, and while a block has websites, browsers without it are closed."),
            browserList, extensionStatus);

        var browserRows = Ui.Column(14, Ui.Dim("Each browser setting can be locked on its own with Lock…, independently of your blocks. A locked setting can't be turned off until its lock ends or is unlocked. Blocking developer tools also can't be turned off while a block is locked."));
        foreach (var o in Tables.BrowserOptions)
        {
            var sw = Ui.Switch(o.Title, false, on => _ = C.Run(["browser", o.Key, on ? "on" : "off"]), "Browser_" + o.Key);
            browserSwitches[o.Key] = sw;
            browserRows.Children.Add(LockedRow(o.Key, o.Title, o.Help, sw));
        }
        var browserSettings = Ui.Plain("Browser settings", Ui.Card(browserRows));

        strictUse = Ui.Switch("Strict use counting", false, on => _ = C.Run(["use-counting", on ? "strict" : "normal"]), "StrictUse");
        var use = Ui.Section("Use counting", Ui.SettingRow("Strict use counting", "An allowance is spent while any tab of its sites is open and while its apps run, even in the background or minimised. Turning it off weakens your allowances.", strictUse));

        var noticeRow = Ui.Wrap(14);
        foreach (var (key, title) in Tables.NoticeKinds)
        {
            var check = new CheckBox { Content = title }.Named(title, "Notice_" + key);
            check.Click += (_, _) => { if (!filling) _ = C.Run(["notifications", key, check.IsChecked == true ? "on" : "off"]); };
            notices[key] = check;
            noticeRow.Children.Add(check);
        }
        advance = new TextBox { PlaceholderText = "2,5", Width = 100 }.Named("Minutes ahead", "NoticeAdvance");
        var notifications = Ui.Section("Notifications", Ui.Dim("Notifications before things happen. A scheduled Frozen Chicken is always announced."),
            noticeRow, Ui.Wrap(6, Ui.Text("Minutes ahead:"), advance, Ui.Button("Apply", () => _ = C.Run(["notifications", "advance", advance.Text.Replace(" ", "")]), "NoticeApply")));

        dnsChoice = Combo("DNS server", "DnsChoice", [("network", "The network's own server, then Quad9"), ("quad9", "Quad9 only"), ("own", "My own servers")], "network");
        dnsServers = new TextBox { PlaceholderText = "e.g. 192.168.1.1, 9.9.9.9", MinWidth = 200, MaxWidth = 240 }.Named("My own DNS servers", "DnsServers");
        dnsChoice.SelectionChanged += (_, _) => dnsServers.Visibility = Key(dnsChoice) == "own" ? Visibility.Visible : Visibility.Collapsed;
        var dns = Ui.Section("DNS", Ui.Dim("The firewall needs the real addresses of blocked sites, so Killer Chicken 9000 asks a DNS server for them. By default that is the server the computer already uses, so nobody new learns which sites are blocked."),
            Ui.Wrap(6, dnsChoice, dnsServers, Ui.Button("Apply", () => _ = ApplyDns(), "DnsApply")));

        strict = Ui.Switch("Strict mode", false, on => _ = Strict(on), "Strict");
        strictExempt = new EditableList("", @"C:\Tools\", async dirs => { foreach (var d in dirs) await C.Run(["strict", "exempt", "add", d]); },
            (d, _) => C.Run(["strict", "exempt", "remove", d]),
            help: "Exempt folders: programs in them still start while strict mode holds (adding one weakens strict mode).",
            split: t => t.Trim().Length > 0 ? [t.Trim()] : [], multi: true, id: "StrictExempt", showTitle: false);
        noEscape = Ui.Switch("No-escape", false, on => _ = NoEscape(on), "NoEscape");
        resetGuard = Ui.Switch("Installer media guard", false, on => _ = C.Run(["reset-guard", on ? "on" : "off"]), "ResetGuard");
        levelsNow = Ui.Strong("").Id("ProtectionNow");
        firmwareStatus = Ui.Note("").Id("FirmwareStatus");
        var levels = Ui.Column(4);
        foreach (var (title, detail, _) in Words.ProtectionLevels(false, false, false, false))
            levels.Children.Add(Ui.Text($"{title}: {detail}", "CaptionTextBlockStyle"));
        var device = Ui.Section("Device protection",
            Ui.Dim("How far Killer Chicken 9000 holds against you is your choice: nothing here is on until you turn it on. The guided setup goes through the strictest setup in order."),
            levelsNow, levels,
            LockedRow("strict", "Strict mode: deny the administrator account what would stop Killer Chicken", "While anything is locked or Frozen Chicken runs, the administrator account is denied what would stop Killer Chicken: its service, files and registry keys, and the boot and recovery settings. It holds again each minute. Strict mode itself can be turned off at any time unless you lock it.", strict),
            strictExempt,
            Ui.SettingRow("No-escape: close the ways out", "During a lock, Safe Mode and the recovery environment are off, so the only way past a lock is booting other media or reinstalling. It can't be turned off during a lock.", noEscape),
            Ui.SettingRow("Factory reset protection: installer media guard", "While anything is locked, the download pages of operating systems and USB image writers are blocked and image writers (Rufus, balenaEtcher, the Media Creation Tool, …) are closed. It can't be turned off during a lock.", resetGuard),
            Ui.SettingRow("Factory reset protection: firmware lock", "USB and network boot off, the boot order locked, and a firmware password a trusted person keeps. The guided setup lists every step of the strictest setup, in order.",
                Ui.Wrap(6, Ui.Button("Firmware lock…", () => _ = Firmware(), "FirmwareLock"), Ui.Button("Guided setup…", () => _ = Guided(), "GuidedSetup")),
                firmwareStatus));

        wallpaperDefault = Combo("Wallpaper default", "WallpaperDefault", Tables.WallpaperDefaults, "latest");
        wallpaperDefault.SelectionChanged += (_, _) => { if (!filling) _ = WallpaperDefaultChanged(); };
        wallpaperCustom = Ui.Wrap(10, wallpaperName, Ui.Button("Choose a picture…", () => _ = ChooseWallpaper(), "WallpaperChoose"));
        var wallpaper = Ui.Section("Wallpaper default",
            Ui.Dim("What the desktop shows when no mode and no circadian rhythm wallpaper does, and while Killer Chicken can't tell where you are for a mode switched by places (no Wi-Fi, the first seconds after starting or waking)."),
            Ui.SettingRow("Default to", "", wallpaperDefault), wallpaperCustom, wallpaperNote);

        passwordSet = Ui.Button("Set password…", () => _ = SetPassword(), "PasswordSet");
        passwordRemove = Ui.Button("Remove password…", () => _ = RemovePassword(), "PasswordRemove");
        passwordLocks = new CheckBox { Content = "Also needed to set a lock, so nobody else can lock me out" }.Named("Settings password also needed to lock", "PasswordLocks");
        passwordLocks.Click += (_, _) => { if (!filling) _ = C.Run(["password", "locks", passwordLocks.IsChecked == true ? "on" : "off"]); };
        accessRemove = Ui.Button("Remove…", () => _ = AccessPassword(remove: true), "AccessRemove");
        var idle = new DurationField(new DurationRule { Minimum = 60, Maximum = 7 * 86400, Zero = "Never" }, prefs.AccessIdleMinutes * 60L, width: 110);
        idle.Named("Lock the pages again after", "AccessIdle");
        idle.Changed += () => { if (idle.IsValid) { prefs.AccessIdleMinutes = (int)idle.Minutes; prefs.Save(); } };
        var passwords = Ui.Section("Passwords",
            Ui.Strong("Settings password"),
            Ui.Dim("Guards Killer Chicken 9000 against anything running as you, and lets someone else decide when your blocks may be weakened: weakening, stopping, unlocking, turning browser settings off, strict mode, no-escape and uninstalling all ask for it. Starting a block and adding sites don't."),
            passwordStatus, Ui.Wrap(6, passwordSet, passwordRemove), passwordLocks,
            Ui.Strong("Access passwords"),
            Ui.Dim("Optional passwords to open Killer Chicken 9000 and each of its pages: Overview, Blocks, Frozen Chicken, Statistics and Settings. Use the same one for several, or a different one for each. A locked page shows nothing until its password is given; Lock in the sidebar locks them again. Changing or removing one asks for the one it has."),
            accessStatus, Ui.Wrap(6, Ui.Button("Set a password…", () => _ = AccessPassword(remove: false), "AccessSet"), accessRemove),
            Ui.Wrap(8, Ui.Text("Lock the pages again after"), idle, Ui.Text("without using this window")));

        nepsisOff = Ui.Button("Turn off…", () => _ = NepsisOff(), "NepsisOff");
        var nepsis = Ui.Section("Bypass prevention",
            Ui.Dim("When Killer Chicken stops many things in a short time (blocked pages, closed apps and windows, browsers without the extension, wrong access passwords), you are likely hunting for a loophole. It then takes the consequence you choose, so you really step away. Each block can have its own too."),
            nepsisLabel, Ui.Wrap(6, Ui.Button("Change…", () => _ = ChangeNepsis(), "NepsisChange", accent: true), nepsisOff));

        var project = Ui.Section("Project",
            Ui.Dim("Save every block and setting to one file. A backup also holds every preset, your places (the Wi-Fi networks they are known by), running locks, passwords (as hashes), hotkeys and statistics; a file to share leaves those out and asks about presets. Importing shows what it adds first, only adds, and gives a block or preset whose name is taken a new name."),
            Ui.Wrap(6, Ui.Button("Back up…", () => _ = ExportProject(false), "ProjectBackup"), Ui.Button("Share…", () => _ = ExportProject(true), "ProjectShare"),
                Ui.Button("Import project…", () => _ = Files.ImportProject(window), "ProjectImport")));

        var behaviour = Ui.Column(12);
        foreach (var o in Tables.Options)
        {
            var sw = Ui.Switch(o.Title, false, on => _ = C.Run(["behaviour", o.Key, on ? "on" : "off"]), "Option_" + o.Key);
            options[o.Key] = sw;
            behaviour.Children.Add(Ui.SettingRow(o.Title, o.Help, sw));
        }

        scroll = Ui.Page(Ui.Column(20, Ui.PageHeader("Settings", "How Killer Chicken looks, what protects it, and what it may never do."),
            preferences, noWeak, tamper, browsersSection, browserSettings, use, notifications, dns, device, wallpaper, passwords, nepsis, project,
            Ui.Section("Behaviour settings", behaviour), Keyboard(), About()));
    }

    // W43: which versions are installed, so a person can tell (and say).
    TextBlock aboutCli = null!;

    StackPanel About()
    {
        var gui = typeof(SettingsPage).Assembly
            .GetCustomAttributes(typeof(System.Reflection.AssemblyInformationalVersionAttribute), false)
            .OfType<System.Reflection.AssemblyInformationalVersionAttribute>().FirstOrDefault()?.InformationalVersion ?? "?";
        gui = gui.Split('+')[0];
        aboutCli = Ui.Dim("kc9k: …").Id("AboutCli");
        return Ui.Section("About",
            Ui.Strong("Killer Chicken 9000"),
            Ui.Dim("This window: " + gui).Id("AboutGui"),
            aboutCli,
            Ui.Dim("Free forever: no accounts, no telemetry; everything stays on this computer.")).Id("About");
    }

    async Task ReadVersion()
    {
        var result = await C.Session.RunAsync(["version"]);
        aboutCli.Text = result.Ok && result.Output.Trim().Length > 0 ? "Service and command line: " + result.Output.Trim()
            : "Service and command line: an older kc9k without 'kc9k version'";
    }

    // W34: the window's shortcuts, so they can be found.
    static StackPanel Keyboard()
    {
        var rows = Ui.Column(6);
        foreach (var (keys, what) in MainWindow.Shortcuts)
        {
            var g = new Grid { ColumnSpacing = 12 };
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(180) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            g.Children.Add(Ui.Strong(keys));
            var t = Ui.Dim(what);
            Grid.SetColumn(t, 1);
            g.Children.Add(t);
            rows.Children.Add(g);
        }
        return Ui.Section("Keyboard", rows).Id("Keyboard");
    }

    static ComboBox Combo(string name, string id, IEnumerable<(string Key, string Title)> items, string selected)
    {
        var c = new ComboBox { MinWidth = 200 }.Named(name, id);
        foreach (var (k, t) in items) c.Items.Add(new ComboBoxItem { Content = t, Tag = k });
        c.SelectedItem = c.Items.OfType<ComboBoxItem>().FirstOrDefault(i => (string)i.Tag == selected) ?? c.Items[0];
        return c;
    }

    static string Key(ComboBox c) => (c.SelectedItem as ComboBoxItem)?.Tag as string ?? "";

    public void ShowBrowsers() => browsersSection.StartBringIntoView();

    // W39: the firmware lock's state on the page, read quietly (a refusal
    // here is no news) when the page is first shown and after its dialog.
    TextBlock levelsNow = null!, firmwareStatus = null!;
    bool firmwareLocked, firmwareRead;

    async Task ReadFirmware()
    {
        firmwareRead = true;
        var result = await C.Session.RunAsync(["firmware-lock", "--json"]);
        if (!result.Ok) { firmwareStatus.Text = "Firmware lock: its state can't be read now."; return; }
        try
        {
            var o = result.Output;
            var status = JsonDocument.Parse(o[Math.Max(0, o.IndexOf('{'))..]).RootElement;
            firmwareLocked = B(status, "locked");
            firmwareStatus.Text = !B(status, "available")
                ? "Firmware lock: not available from Windows on this computer; its steps can be done in the firmware's own setup."
                : $"Firmware lock: {(firmwareLocked ? "on" : "off")} · firmware password {(B(status, "password") ? "set" : "not set")}"
                  + (B(status, "pending_reboot") ? " · takes effect at the next restart" : "");
        }
        catch (JsonException) { firmwareStatus.Text = ""; }
        if (C.State is { } s) levelsNow.Text = Words.ProtectionNow(s.Strict, s.NoEscape, s.ResetGuard, firmwareLocked);
    }

    // Setting locks (browser settings, strict mode, block unsupported browsers) ---

    UIElement LockedRow(string name, string title, string detail, ToggleSwitch sw)
    {
        var status = Ui.Note("");
        status.Visibility = Visibility.Collapsed;
        var unlock = Ui.Button("Unlock…", () => _ = UnlockSetting(name, title), "Unlock_" + name);
        unlock.Visibility = Visibility.Collapsed;
        settingLocks[name] = (status, unlock);
        var texts = Ui.Column(2, Ui.Text(title));
        if (detail.Length > 0) texts.Children.Add(Ui.Note(detail));
        texts.Children.Add(status);
        // W54: Lock… and the switch in the page's right column.
        return Ui.Side(texts, Ui.Row(6, unlock, Ui.Button("Lock…", () => _ = LockSetting(name, title), "Lock_" + name), sw));
    }

    UIElement LockLine(string name, string title)
    {
        var status = Ui.Note("");
        var unlock = Ui.Button("Unlock…", () => _ = UnlockSetting(name, title), "Unlock_" + name);
        unlock.Visibility = Visibility.Collapsed;
        settingLocks[name] = (status, unlock);
        return Ui.Wrap(8, Ui.Text(title + ":"), status, Ui.Button("Lock…", () => _ = LockSetting(name, title), "Lock_" + name), unlock);
    }

    SettingLock LockInfo(string name)
    {
        var s = C.State!;
        return name switch
        {
            "strict" => s.StrictLock,
            "unsupported-browsers" => s.UnsupportedBrowsersLock,
            _ => s.BrowserLocks.GetValueOrDefault(name) ?? new SettingLock(),
        };
    }

    static List<string> SettingCommand(string name, string verb) => name switch
    {
        "strict" => ["strict", verb],
        "unsupported-browsers" => verb == "unlock" ? ["tamper", "unlock", name] : ["tamper", name, verb],
        _ => verb == "unlock" ? ["browser", "unlock", name] : ["browser", name, verb],
    };

    bool SettingLocked(string name) => Words.SettingLockParts(LockInfo(name)).Count > 0;

    public async Task LockSetting(string name, string title)
    {
        var picker = new LockPicker("turn it off", "turned off", ["timer", "password", "text", "restart", "range", "relock"], id: "SettingLocks");
        var asked = await LockDialogs.Ask(window, $"Lock: {title}", "The setting is turned on and can't be turned off until the locks end. More locks can be added later, after these; a lock that is on is never changed.",
            "Turn on and lock", picker, needLock: true);
        if (asked is not var (chosen, relock, secret) || chosen.Count == 0) return;
        if (chosen.Count > 1)
        {
            var ordered = await LockDialogs.ChooseOrder(window, chosen);
            if (ordered is null) return;
            chosen = ordered;
        }
        var (flags, lines) = LockDialogs.Lines(title, chosen, relock);
        lines.RemoveAll(l => l.Contains("re-lock, all its locks come back"));
        var (ok, _) = await C.Dialogs.ConfirmLock(lines);
        if (ok) await C.Run([.. SettingCommand(name, "on"), .. flags, "--yes"], secret.Length > 0 ? secret + "\n" : null);
    }

    async Task UnlockSetting(string name, string title)
    {
        var info = LockInfo(name);
        var unlock = SettingCommand(name, "unlock");
        if (info.LockOrder.FirstOrDefault() == "password" || (info.PasswordLocked && !info.CharLocked))
        {
            var password = await C.Dialogs.AskText($"Unlock {title}", "Enter the setting's password.", password: true);
            if (!string.IsNullOrEmpty(password)) await C.Run(unlock, password + "\n");
            return;
        }
        await C.Run(unlock, then: o => C.Dialogs.Message(o, "Unlocked"), timeout: TimeSpan.FromHours(6));
    }

    // Showing the state ---------------------------------------------------------------

    public void Update(GuiState s)
    {
        levelsNow.Text = Words.ProtectionNow(s.Strict, s.NoEscape, s.ResetGuard, firmwareLocked);
        if (!firmwareRead) { _ = ReadFirmware(); _ = ReadVersion(); }
        filling = true;
        try
        {
            Ui.Set(noWeakening, s.NoWeakening);
            Ui.Set(strictUse, s.StrictUse);
            Ui.Set(strict, s.Strict);
            Ui.Set(noEscape, s.NoEscape);
            Ui.Set(resetGuard, s.ResetGuard);
            foreach (var (key, sw) in browserSwitches) Ui.Set(sw, s.Browser.GetValueOrDefault(key));
            foreach (var (key, check) in notices) check.IsChecked = s.Notifications.IsOn(key);
            if (advance.FocusState == FocusState.Unfocused)
                advance.Text = string.Join(",", s.Notifications.Advance.Count > 0 ? s.Notifications.Advance : [5]);
            foreach (var (key, sw) in options) Ui.Set(sw, s.Options.GetValueOrDefault(key));
            foreach (var (name, (status, unlock)) in settingLocks)
            {
                var info = LockInfo(name);
                var parts = Words.SettingLockParts(info);
                var text = parts.Count > 0 ? "Locked: " + string.Join(", ", parts) : "";
                if (info.RelockIn > 0) text = (text.Length > 0 ? text + "; " : "") + $"unlocked, locks on again in {Words.Seconds(info.RelockIn)}";
                status.Text = text.Length > 0 ? text : name == "unsupported-browsers" ? "not locked" : "";
                status.Visibility = status.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
                unlock.Visibility = info.CharLocked || info.PasswordLocked ? Visibility.Visible : Visibility.Collapsed;
                // R-LOCK-6: only the first lock still on can be unlocked.
                unlock.IsEnabled = info.LockOrder.Count > 0 && info.LockOrder[0] is "text" or "password";
                if (browserSwitches.TryGetValue(name, out var sw)) sw.IsEnabled = parts.Count == 0;
            }
            strict.IsEnabled = !SettingLocked("strict");
            strictExempt.SetItems(s.StrictExempt);
            var on = Tables.Tamper.Where(t => Tables.TamperValue(s, t.Key)).Select(t => t.Title).ToList();
            tamperSummary.Text = on.Count > 0 ? "On: " + string.Join("; ", on) : "All off.";
            UpdateBrowsers(s);
            var nep = Words.Nepsis(s.Nepsis, s.NepsisCount);
            if (s.NepsisLock.LockSeconds > 0) nep += $" Locked for {Words.Seconds(s.NepsisLock.LockSeconds)}.";
            nepsisLabel.Text = nep;
            nepsisOff.Visibility = s.Nepsis.IsOn ? Visibility.Visible : Visibility.Collapsed;
            passwordStatus.Text = s.SettingsPassword ? "On." : "Not set.";
            passwordSet.Content = s.SettingsPassword ? "Change password…" : "Set password…";
            passwordRemove.Visibility = passwordLocks.Visibility = s.SettingsPassword ? Visibility.Visible : Visibility.Collapsed;
            passwordLocks.IsChecked = s.PasswordForLocks;
            var with = Tables.AreaTitles.Where(a => s.Access.GetValueOrDefault(a.Key, "none") != "none").Select(a => a.Key == "app" ? "Opening the program" : a.Value).ToList();
            accessStatus.Text = with.Count > 0 ? "On for: " + string.Join(", ", with) + "." : "Not set.";
            accessRemove.Visibility = with.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
            var dnsValue = s.Dns.Length > 0 ? s.Dns : "network";
            if (dnsChoice.FocusState == FocusState.Unfocused && dnsServers.FocusState == FocusState.Unfocused)
            {
                if (dnsValue is "network" or "quad9") dnsChoice.SelectedItem = dnsChoice.Items.OfType<ComboBoxItem>().First(i => (string)i.Tag == dnsValue);
                else
                {
                    dnsChoice.SelectedItem = dnsChoice.Items.OfType<ComboBoxItem>().First(i => (string)i.Tag == "own");
                    dnsServers.Text = dnsValue;
                }
                dnsServers.Visibility = Key(dnsChoice) == "own" ? Visibility.Visible : Visibility.Collapsed;
            }
            var w = s.Wallpaper;
            wallpaperDefault.SelectedItem = wallpaperDefault.Items.OfType<ComboBoxItem>().FirstOrDefault(i => (string)i.Tag == (w.Default.Length > 0 ? w.Default : "latest")) ?? wallpaperDefault.Items[0];
            var custom = w.Default == "custom" && w.CustomPath.Length > 0;
            wallpaperCustom.Visibility = custom ? Visibility.Visible : Visibility.Collapsed;
            wallpaperName.Text = w.CustomName;
            wallpaperNote.Text = (w.Default.Length > 0 ? w.Default : "latest") switch
            {
                "latest" => "The wallpaper stays as it is. At a new place without Wi-Fi that can be the last place's wallpaper until Killer Chicken knows where you are.",
                "circadian" => "Circadian rhythm's wallpaper for the time of day, even with circadian rhythm switched off.",
                _ => "This picture shows until Killer Chicken knows where you are. It also goes on before the computer sleeps, so it wakes with it, not with the last place's wallpaper.",
            };
            Ui.Set(login, Startup.IsOn());
        }
        finally { filling = false; }
    }

    void UpdateBrowsers(GuiState s)
    {
        browserList.Children.Clear();
        foreach (var b in s.Browsers)
        {
            var g = new Grid { ColumnSpacing = 12 };
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(160) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            g.Children.Add(Ui.Strong(b.Name));
            var status = Ui.Dim(Words.BrowserStatus(b));
            Grid.SetColumn(status, 1);
            g.Children.Add(status);
            if (!b.Installed && !b.Found)
            {
                var install = Ui.Button("Install", () => _ = InstallBrowser(b), "InstallBrowser_" + b.Id);
                Grid.SetColumn(install, 2);
                g.Children.Add(install);
            }
            browserList.Children.Add(g);
        }
        extensionStatus.Text = s.Browsers.Any(b => b.Installed) ? "Extension: " + s.ExtensionStatus
            : "No supported browser is installed, so the extension can't run: wildcards, rendered terms, greyscale, images and web-only aren't enforced in the browser, and no browser is closed. Install one above.";
    }

    // On Windows, kc9k doesn't download browsers: 'browsers install' says
    // where to get one, shown as information, not as a refusal (W30).
    async Task InstallBrowser(BrowserInfo b)
    {
        if (await C.Dialogs.Confirm($"Install {b.Name}?", "Killer Chicken 9000 installs it or says where to get it. The extension is added the next time it starts.", "Continue"))
            await C.Run(["browsers", "install", b.Id], timeout: TimeSpan.FromMinutes(15), then: o => C.Dialogs.Message(o, "Installed"),
                refusalTitle: $"Getting {b.Name}");
    }

    // Commands ---------------------------------------------------------------------------

    async Task NoWeakening(bool on)
    {
        if (on && !await C.Dialogs.Confirm("Turn on no weakening during active locks?", "While any block or setting is locked, no setting can be weakened, and this can't be turned off either until every lock has ended.", "Turn on"))
        {
            Ui.Set(noWeakening, false);
            return;
        }
        await C.Run(["no-weakening", on ? "on" : "off"]);
    }

    async Task Strict(bool on)
    {
        if (on && !await C.Dialogs.Confirm("Turn on strict mode?", "During a lock, the administrator account can't stop Killer Chicken: its service, files, registry keys and the boot and recovery settings are denied to it until the lock ends.", "Turn on"))
        {
            Ui.Set(strict, false);
            return;
        }
        await C.Run(["strict", on ? "on" : "off"]);
    }

    async Task NoEscape(bool on)
    {
        if (on)
        {
            if (!await C.Dialogs.Confirm("Turn on no-escape?", "During a lock, the only way out will be booting other media or reinstalling Windows. Make sure you know how before relying on this.", "Turn on", destructive: true))
            {
                Ui.Set(noEscape, false);
                return;
            }
            await C.Run(["no-escape", "on", "--yes"]);
        }
        else await C.Run(["no-escape", "off"]);
    }

    /// <summary>The tamper protection group in its own dialog: nothing changes
    /// until OK, and turning one on is confirmed by OK (§4.10).</summary>
    async Task TamperDialog()
    {
        var s = C.State;
        if (s is null) return;
        var body = Ui.Column(10, Ui.Dim("Once on, a tamper protection setting can't be turned off while any block or setting is locked. Turning one off asks for the settings password."));
        var hasSupported = s.Browsers.Any(b => b.Installed);
        var checks = new Dictionary<string, CheckBox>();
        foreach (var (key, title, help, _, _) in Tables.Tamper)
        {
            var current = Tables.TamperValue(s, key);
            var check = new CheckBox { Content = title, IsChecked = current }.Named(title, "Tamper_" + key);
            var note = help;
            if (key == "unsupported-browsers" && SettingLocked(key)) { check.IsEnabled = false; note += " It has its own lock now."; }
            else if (key == "unsupported-browsers" && !current && !hasSupported) { check.IsEnabled = false; note += " Install a supported browser first (Browsers, below)."; }
            else if (current && s.AnythingLocked) { check.IsEnabled = false; note += " Something is locked, so it stays on."; }
            checks[key] = check;
            var n = Ui.Note(note);
            n.Margin = new Thickness(28, 0, 0, 0);
            body.Children.Add(Ui.Column(2, check, n));
        }
        var grace = new DurationField(new DurationRule { Minimum = 10, Maximum = 600, Bare = 1 }, s.TamperGrace > 0 ? s.TamperGrace : 60, width: 100);
        grace.Named("Close browsers without the extension after", "TamperGrace");
        body.Children.Add(Ui.Wrap(8, Ui.Text("Close browsers without the extension after"), grace, Ui.Text("(longer weakens)")));
        if (!await C.Dialogs.Ask("Tamper protection", body, "OK", () => grace.IsValid, w => grace.Changed += w, width: 720)) return;
        if (grace.Seconds is long g && g != (s.TamperGrace > 0 ? s.TamperGrace : 60) && !await C.Run(["tamper", "grace", g.ToString(System.Globalization.CultureInfo.InvariantCulture)])) return;
        foreach (var (key, _, _, command, _) in Tables.Tamper)
        {
            var wanted = checks[key].IsChecked == true;
            if (wanted != Tables.TamperValue(s, key) && !await C.Run([.. command, wanted ? "on" : "off"])) return;
        }
    }

    async Task ApplyDns()
    {
        var choice = Key(dnsChoice);
        if (choice == "own")
        {
            choice = string.Join(",", dnsServers.Text.Split(',').Select(x => x.Trim()).Where(x => x.Length > 0));
            if (choice.Length == 0)
            {
                await C.Dialogs.Message("Enter one or more server addresses, separated by commas.", "Not applied");
                return;
            }
        }
        await C.Run(["dns", choice]);
    }

    async Task WallpaperDefaultChanged()
    {
        var key = Key(wallpaperDefault);
        if (key != "custom") await C.Run(["wallpaper", "default", key]);
        else if (C.State?.Wallpaper.CustomPath.Length > 0) await C.Run(["wallpaper", "default", "custom"]);
        else await ChooseWallpaper();
    }

    async Task ChooseWallpaper()
    {
        var path = await Files.Open(window, Files.Pictures);
        if (path is not null) await C.Run(["wallpaper", "default", "custom", path]);
        else C.Refresh();
    }

    async Task SetPassword()
    {
        var first = await C.Dialogs.AskText("New settings password", "At least 4 characters.", password: true, problem: t => t.Length < 4 ? "Use at least 4 characters." : null);
        if (string.IsNullOrEmpty(first)) return;
        if (await C.Dialogs.AskText("Repeat the new password", password: true) != first)
        {
            await C.Dialogs.Message("The two passwords differ; nothing changed.", "Not changed");
            return;
        }
        if (await C.Run(["password", "set"], first + "\n")) C.Session.SettingsPassword = first;
    }

    async Task RemovePassword()
    {
        if (await C.Dialogs.Confirm("Remove the settings password?", "Weakening and locks won't ask for it any more.", "Remove") && await C.Run(["password", "remove"]))
            C.Session.SettingsPassword = null;
    }

    /// <summary>Which areas (and, to set, which password).</summary>
    async Task AccessPassword(bool remove)
    {
        var access = C.State?.Access ?? [];
        var checks = new Dictionary<string, CheckBox>();
        var body = Ui.Column(6, Ui.Dim(remove ? "Remove the password of:" : "Everything ticked gets this one password. Do it again with other boxes ticked for a different one."));
        foreach (var (area, title) in Tables.AreaTitles)
        {
            var has = access.GetValueOrDefault(area, "none") != "none";
            var label = (area == "app" ? "Opening the program" : title) + (has && !remove ? " (has a password)" : "");
            var c = new CheckBox { Content = label, IsEnabled = !remove || has }.Named(label, "AccessArea_" + area);
            checks[area] = c;
            body.Children.Add(c);
        }
        var first = new PasswordBox { PlaceholderText = "Password (at least 4 characters)" }.Named("Access password", "AccessNew");
        var again = new PasswordBox { PlaceholderText = "Repeat it" }.Named("Repeat the access password", "AccessAgain");
        if (!remove)
        {
            body.Children.Add(first);
            body.Children.Add(again);
        }
        bool Valid() => checks.Values.Any(c => c.IsChecked == true) && (remove || (first.Password.Length >= 4 && first.Password == again.Password));
        if (!await C.Dialogs.Ask(remove ? "Remove access passwords" : "Set an access password", body, remove ? "Remove" : "Set", Valid, w =>
            {
                foreach (var c in checks.Values) c.Click += (_, _) => w();
                first.PasswordChanged += (_, _) => w();
                again.PasswordChanged += (_, _) => w();
            })) return;
        var areas = checks.Where(c => c.Value.IsChecked == true).Select(c => c.Key).ToList();
        if (remove) await C.Run(["access", "remove", .. areas]);
        else
        {
            var password = first.Password;
            // Remembered at once, so the page being worked on doesn't lock.
            await C.Run(["access", "set", .. areas], password + "\n", then: _ =>
            {
                if (!C.Session.AccessPasswords.Contains(password)) C.Session.AccessPasswords.Add(password);
                return Task.CompletedTask;
            });
        }
    }

    async Task ChangeNepsis()
    {
        var asked = await NepsisDialog.Ask(window, "Bypass prevention", C.State?.Nepsis, allowLock: true);
        if (asked is not var (args0, lockWords)) return;
        var args = await NepsisDialog.AcknowledgeMonkSteps(window, args0);
        if (args is null) return;
        if (args.Contains("--for") || args.Contains("--until"))
        {
            if (!(await C.Dialogs.ConfirmLock([$"bypass prevention: {lockWords}"])).Ok) return;
            args.Add("--yes");
        }
        await C.Run(["bypass", "set", .. args]);
    }

    async Task NepsisOff()
    {
        if (await C.Dialogs.Confirm("Turn bypass prevention off?", "This weakens it, so it isn't possible while it is locked.", "Turn off"))
            await C.Run(["bypass", "off"]);
    }

    async Task ExportProject(bool share)
    {
        List<string> command = ["project", "export"];
        if (share)
        {
            var d = new ContentDialog { Title = "Include your presets?", Content = Ui.Text("Block presets, list and phrase set presets, schedule, automation and hotkey presets."),
                PrimaryButtonText = "With presets", SecondaryButtonText = "Without presets", CloseButtonText = "Cancel" };
            var answer = await C.Dialogs.Show(d);
            if (answer == ContentDialogResult.None) return;
            command.AddRange(["--share", answer == ContentDialogResult.Primary ? "--presets" : "--presets=false"]);
        }
        var path = await Files.Save(window, share ? "kc9k-setup.json" : "kc9k-backup.json", ".json", "Killer Chicken setup");
        if (path is null) return;
        var output = await C.Read(command);
        if (output is null) return;
        try { await File.WriteAllTextAsync(path, output + "\n"); }
        catch (Exception e) { await C.Dialogs.Message(e.Message); }
    }

    Task ImportProject() => Files.ImportProject(window); // W55

    // Firmware lock and guided setup (§4.10 Device protection) -------------------------------

    async Task<JsonElement?> CliJson(List<string> args)
    {
        var output = await C.Read(args);
        if (output is null) return null;
        try { return JsonDocument.Parse(output[Math.Max(0, Math.Min(output.IndexOfAny(['{', '[']), output.Length))..]).RootElement.Clone(); }
        catch (JsonException)
        {
            await C.Dialogs.Message("Unexpected answer from kc9k: " + output[..Math.Min(200, output.Length)]);
            return null;
        }
    }

    static bool B(JsonElement e, string p) => e.TryGetProperty(p, out var v) && v.ValueKind == JsonValueKind.True;
    static string S(JsonElement e, string p) => e.TryGetProperty(p, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() ?? "" : "";

    async Task Firmware()
    {
        if (await CliJson(["firmware-lock", "--json"]) is not { } status) return;
        var lines = new List<string>();
        var available = B(status, "available");
        if (!available)
            lines.Add("This computer's firmware settings can't be changed from Windows here. Do it in the firmware's own setup (press F2, F1, F10, F12 or Del while the computer starts): turn off booting from USB and the network, lock the boot order, and set a firmware password that a trusted person types and keeps.");
        else
        {
            lines.Add("Firmware lock: " + (B(status, "locked") ? "on" : "off"));
            lines.Add("Firmware password: " + (B(status, "password") ? "set" : "not set"));
            if (status.TryGetProperty("settings", out var settings))
                foreach (var st in settings.EnumerateArray())
                {
                    var state = S(st, "current");
                    if (S(st, "want").Length == 0) state += " (can't be changed here)";
                    else if (!B(st, "done")) state += $" (the lock sets {S(st, "want")})";
                    lines.Add($"{S(st, "purpose")} ({S(st, "name")}): {state}");
                }
            if (status.TryGetProperty("missing", out var missing))
                foreach (var m in missing.EnumerateArray()) lines.Add($"{m.GetString()}: not offered here; set it in the firmware's own setup");
        }
        var d = new ContentDialog { Title = "Firmware lock", Content = new ScrollViewer { Content = Ui.Text(string.Join("\n", lines)), MaxHeight = 420 }, CloseButtonText = "Close" };
        if (available && !B(status, "locked")) d.PrimaryButtonText = "Lock the firmware…";
        if (available && B(status, "set_by_kc9k")) d.SecondaryButtonText = "Remove…";
        var answer = await C.Dialogs.Show(d);
        try { await FirmwareAnswer(answer, status); }
        finally { await ReadFirmware(); }
    }

    async Task FirmwareAnswer(ContentDialogResult answer, JsonElement status)
    {
        if (answer == ContentDialogResult.Primary) await LockFirmware(B(status, "password"));
        else if (answer == ContentDialogResult.Secondary)
        {
            var password = await C.Dialogs.AskText("Firmware password", "The trusted person types it. The settings the lock changed are put back and the password is cleared.", password: true);
            if (!string.IsNullOrEmpty(password)) await C.Run(["firmware-lock", "off"], password + "\n");
        }
    }

    async Task LockFirmware(bool hasPassword)
    {
        string? password;
        if (hasPassword) password = await C.Dialogs.AskText("The firmware password already set", "Needed to change the firmware's settings; it stays as it is.", password: true);
        else
        {
            password = await C.Dialogs.AskText("Firmware password", "The trusted person types it and keeps it. Killer Chicken 9000 doesn't keep it.", password: true);
            if (!string.IsNullOrEmpty(password) && await C.Dialogs.AskText("Type it again", password: true) != password)
            {
                await C.Dialogs.Message("The two passwords differ; nothing changed.", "Not changed");
                return;
            }
        }
        if (string.IsNullOrEmpty(password)) return;
        var kind = hasPassword ? "USB and network boot off, boot order locked, under the firmware password already set"
            : "USB and network boot off, boot order locked, a firmware password a trusted person keeps";
        if ((await C.Dialogs.ConfirmLock([$"the computer's firmware: firmware lock ({kind})"])).Ok)
            await C.Run(["firmware-lock", "on", "--yes"], password + "\n");
    }

    async Task Guided()
    {
        if (await CliJson(["guided-setup", "--json"]) is not { ValueKind: JsonValueKind.Array } steps) return;
        var body = Ui.Column(12, Ui.Dim("The strictest setup, step by step: what protects the setup first, no-escape last. Each step says whether it is done."));
        Func<Task>? chosen = null;
        var marks = new Dictionary<string, string> { ["done"] = "✓", ["todo"] = "○", ["by-hand"] = "○", ["info"] = "ℹ" };
        var d = new ContentDialog { Title = "Guided setup", CloseButtonText = "Close" };
        var i = 0;
        foreach (var step in steps.EnumerateArray())
        {
            i++;
            var state = S(step, "state");
            var g = new Grid { ColumnSpacing = 12 };
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            g.Children.Add(Ui.Text(marks.GetValueOrDefault(state, "?")));
            var texts = Ui.Column(2, Ui.Text($"{i}. {S(step, "title")}" + (B(step, "optional") ? " (optional)" : "")), Ui.Note(S(step, "detail")));
            var command = S(step, "command");
            if (state != "done" && command.Length > 0 && !command.StartsWith("kc9k "))
            {
                var c = Ui.Text(command);
                c.IsTextSelectionEnabled = true;
                c.FontFamily = new Microsoft.UI.Xaml.Media.FontFamily("Cascadia Mono, Consolas");
                texts.Children.Add(c);
            }
            Grid.SetColumn(texts, 1);
            g.Children.Add(texts);
            if (state is "todo" or "by-hand" && GuidedAction(S(step, "id"), command) is { } act)
            {
                var b = Ui.Button("Do it…", () => { chosen = act; d.Hide(); }, "Guided_" + S(step, "id"));
                Grid.SetColumn(b, 2);
                g.Children.Add(b);
            }
            body.Children.Add(g);
        }
        d.Content = new ScrollViewer { Content = body, MaxHeight = 520 };
        await C.Dialogs.Show(d);
        if (chosen is not null) await chosen();
    }

    /// <summary>What a guided setup step's Do it… runs: the same as its own
    /// switch or button, so it asks the same questions.</summary>
    Func<Task>? GuidedAction(string id, string command) => id switch
    {
        "settings-password" => SetPassword,
        "no-weakening" => () => NoWeakening(true),
        "tamper" => async () =>
        {
            foreach (var c in command.Split(';').Select(c => c.Trim()).Where(c => c.Length > 0))
                if (!await C.Run([.. c.Split(' ', StringSplitOptions.RemoveEmptyEntries).Skip(1)])) return;
        },
        "strict" => () => Strict(true),
        "strict-lock" => () => LockSetting("strict", "Strict mode"),
        "reset-guard" => () => C.Run(["reset-guard", "on"]),
        "firmware" => Firmware,
        "no-escape" => () => NoEscape(true),
        // Windows (Core W15): other administrators and the built-in
        // Administrator are changed in Windows' own account settings.
        "other-admins" or "builtin-admin" => OpenAccountSettings,
        _ => null,
    };

    static Task OpenAccountSettings()
    {
        try { System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo("ms-settings:otherusers") { UseShellExecute = true }); }
        catch (Exception e) { Log.Write("account settings: " + e.Message); }
        return Task.CompletedTask;
    }
}

/// <summary>§4.10 Open at sign-in: a shortcut in the user's Startup folder
/// that starts the GUI in the notification area.</summary>
public static class Startup
{
    static string LinkPath => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Startup), "Killer Chicken 9000.lnk");

    public static bool IsOn() => File.Exists(LinkPath);

    public static void Set(MainWindow window, bool on)
    {
        try
        {
            if (!on)
            {
                if (File.Exists(LinkPath)) File.Delete(LinkPath);
                return;
            }
            dynamic shell = Activator.CreateInstance(Type.GetTypeFromProgID("WScript.Shell")!)!;
            var link = shell.CreateShortcut(LinkPath);
            link.TargetPath = Environment.ProcessPath ?? Path.Combine(AppContext.BaseDirectory, "kc9k-gui.exe");
            link.Arguments = "--hidden";
            link.WorkingDirectory = AppContext.BaseDirectory;
            link.Description = "Killer Chicken 9000";
            link.Save();
        }
        catch (Exception e)
        {
            _ = window.Controller.Dialogs.Message($"Can't change opening at sign-in: {e.Message}");
        }
    }
}
