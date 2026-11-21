// §4.10 circadian rhythm, its own page (gui/kc9k-gui.py CircadianPage):
// the three automation clips with presets, Play the day, wallpapers by time,
// dark mode, dark pages. Reads `kc9k circadian show --json`; every change
// runs a `kc9k circadian` command. Nothing here can be locked.
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class CircadianPage : IPage
{
    const int PlaySeconds = 15;
    readonly MainWindow window;
    readonly ToggleSwitch master, overrideModes, darkPages;
    readonly Button play;
    readonly StackPanel warnings = Ui.Column(6), clips = Ui.Column(18), exempt = Ui.Column(4);
    readonly ComboBox preset;
    readonly TextBlock presetDetail = Ui.Dim("");
    readonly Dictionary<string, (ClipEditor Editor, ToggleSwitch Switch, TextBlock Value)> lanes = [];
    readonly WallpaperEditor wallpapers;
    readonly RadioButtons dark;
    readonly ClockField lightFrom, lightTo;
    readonly TextBox exemptEntry;
    CircadianView? view;
    bool filling, reading, playing;
    string presetsShown = "", exemptShown = "", warningsShown = "";
    DateTime previewAt = DateTime.MinValue;
    (string Lane, double Value)? previewNext;
    bool previewBusy;
    Controller C => window.Controller;
    public string Area => "settings";
    public UIElement View { get; }

    public CircadianPage(MainWindow window)
    {
        this.window = window;
        master = Ui.Switch("Circadian rhythm on", false, on => { if (!filling) _ = C.Run(["circadian", on ? "on" : "off"], then: _ => Read()); }, "CircadianOn");
        play = Ui.Button("Play the day", () => _ = PlayDay(), "PlayDay", glyph: Glyphs.Play)
            .Tip($"Shows the next 24 hours on the screen in {PlaySeconds} seconds (brightness, colour, wallpaper, dark mode), then goes back to now");
        var head = new Grid();
        head.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        head.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        head.Children.Add(Ui.PageHeader("Circadian rhythm", "Brightness, colour, wallpaper and dark mode follow the time of day.",
            "The screen, the wallpaper and dark mode follow the time of day: cues for the body clock. Nothing here can be locked, and it never overrides you: change a level by hand and its clip pauses until its next point. It leaves Windows' own brightness control and night light working; off, it touches nothing."));
        var right = Ui.Row(10, play, master);
        Grid.SetColumn(right, 1);
        head.Children.Add(right);

        preset = new ComboBox { MinWidth = 200 }.Named("Preset", "CircadianPreset");
        preset.SelectionChanged += (_, _) => presetDetail.Text = view?.Presets.FirstOrDefault(p => p.Name == Key(preset))?.Detail ?? "";
        var clipsSection = Ui.Section("Automation clips",
            "Each clip is a curve over 24 hours. Right-click adds a point (on a point: removes it). Drag a point to move it (Shift: to the minute), drag the small handle on the curve to bend it, double-click a point to type exact values; Points… does the same from the keyboard. While you drag a point, the screen shows its value.",
            Ui.Dim("A curve over 24 hours for each level; right-click adds a point."),
            Ui.Wrap(8, Ui.Text("Start from a preset:"), preset, Ui.Button("Use it", () => _ = UsePreset(), "CircadianUsePreset")), presetDetail, clips);

        wallpapers = new WallpaperEditor(window, () => ["circadian", "wallpaper"], "Circadian");
        overrideModes = Ui.Switch("Override modes", false, on => { if (!filling) _ = C.Run(["wallpaper", "priority", on ? "circadian" : "modes"], then: _ => Read()); }, "OverrideModes");
        var wallpaperSection = Ui.Section("Wallpapers",
            "Drag a bar to move it, or an end to change when it starts or stops (Shift: to the minute). Where times overlap, the one that started last shows. A wallpaper you pick yourself stays until the next one starts. Modes have wallpapers too: while one is enabled its wallpaper shows, unless Override modes is on (the same setting as a mode's Override circadian rhythm wallpapers, the other way round). What shows when neither has one is the wallpaper default, in Settings.",
            Ui.Dim("Each picture is the wallpaper between its two times."), wallpapers,
            Ui.SettingRow("Override modes", "Its wallpapers over a mode's.", overrideModes));

        dark = new RadioButtons().Named("Dark mode", "DarkMode");
        foreach (var (key, label) in new[] { ("system", "Use Windows' own setting (Killer Chicken leaves it alone)"), ("light", "Always light"), ("dark", "Always dark"), ("scheduled", "Light mode between the two times below") })
            dark.Items.Add(new RadioButton { Content = label, Tag = key }.Named(label, "Dark_" + key));
        dark.SelectionChanged += (_, _) => { if (!filling) _ = DarkChanged(); };
        lightFrom = new ClockField(7 * 60, width: 80);
        lightFrom.Named("Light from", "LightFrom");
        lightTo = new ClockField(20 * 60, width: 80);
        lightTo.Named("Light until", "LightTo");
        var darkSection = Ui.Section("Dark mode", Ui.Dim("The dark mode of Windows and its apps, like a phone's."), dark,
            Ui.Wrap(8, Ui.Text("Light between"), lightFrom, Ui.Text("and"), lightTo, Ui.Button("Set", () => _ = SetDarkTimes(), "DarkTimesSet")));

        darkPages = Ui.Switch("Show light web pages dark while dark mode is on", false, on => { if (!filling) _ = C.Run(["circadian", "dark-pages", on ? "on" : "off"], then: _ => Read()); }, "DarkPages");
        exemptEntry = new TextBox { PlaceholderText = "A site to leave as it is (example.com)", Width = 300 }.Named("Site to leave as it is", "DarkExempt");
        exemptEntry.KeyDown += (_, e) => { if (e.Key == Windows.System.VirtualKey.Enter) _ = AddExempt(); };
        var pagesSection = Ui.Section("Dark pages",
            "Pages that are dark already stay as they are. With Windows' own setting, pages follow what the browser says Windows prefers. The extension's toolbar button exempts a site too.",
            Ui.SettingRow("Show light web pages dark while dark mode is on", "", darkPages), exempt,
            Ui.Row(6, exemptEntry, Ui.Button("Add", () => _ = AddExempt(), "DarkExemptAdd")));

        View = Ui.Page(Ui.Column(18, head, warnings, clipsSection, wallpaperSection, darkSection, pagesSection));
    }

    static string Key(ComboBox c) => (c.SelectedItem as ComboBoxItem)?.Tag as string ?? "";

    public void Update(GuiState s)
    {
        if (window.CurrentPage == "circadian" || view is null) _ = Read();
    }

    async Task Read()
    {
        if (reading) return;
        reading = true;
        try
        {
            var result = await C.Session.RunAsync(["circadian", "show", "--json"]);
            if (!result.Ok) return;
            try { view = CircadianView.Parse(result.Output[Math.Max(0, result.Output.IndexOf('{'))..]); }
            catch (System.Text.Json.JsonException) { return; }
            filling = true;
            try { Fill(view); }
            finally { filling = false; }
        }
        finally { reading = false; }
    }

    void Fill(CircadianView v)
    {
        Ui.Set(master, v.On);
        var presets = string.Join("|", v.Presets.Select(p => p.Name));
        if (presets != presetsShown)
        {
            presetsShown = presets;
            preset.Items.Clear();
            foreach (var p in v.Presets) preset.Items.Add(new ComboBoxItem { Content = p.Title, Tag = p.Name });
            if (v.Presets.Count > 0) preset.SelectedIndex = 0;
        }
        foreach (var lane in v.Lanes)
        {
            if (!lanes.ContainsKey(lane.Name)) BuildLane(lane);
            var (editor, sw, value) = lanes[lane.Name];
            Ui.Set(sw, lane.On);
            var now = lane.HasValue ? $"now {lane.Value.ToString("0", CultureInfo.InvariantCulture)} {lane.Unit}" : "no points";
            if (lane.PausedUntil > 0) now += $" · paused until {Clock.Show(DateTimeOffset.FromUnixTimeSeconds(lane.PausedUntil).LocalDateTime.TimeOfDay.TotalMinutes)}";
            else if (!lane.On) now += " · off";
            value.Text = now;
            editor.SetData(lane, v.Minute, v.On && lane.On);
        }
        Warnings(v);
        wallpapers.Fill(v.Wallpapers);
        Ui.Set(overrideModes, v.OverrideModes);
        dark.SelectedIndex = v.Dark.Mode switch { "light" => 1, "dark" => 2, "scheduled" => 3, _ => 0 };
        if (lightFrom.FocusState == FocusState.Unfocused && lightTo.FocusState == FocusState.Unfocused)
        {
            lightFrom.Text = Clock.Show(Clock.Parse(v.Dark.LightFrom) ?? 420);
            lightTo.Text = Clock.Show(Clock.Parse(v.Dark.LightTo) ?? 1200);
        }
        Ui.Set(darkPages, v.DarkPages);
        var ex = string.Join("|", v.DarkExempt);
        if (ex != exemptShown)
        {
            exemptShown = ex;
            exempt.Children.Clear();
            foreach (var site in v.DarkExempt)
                exempt.Children.Add(Ui.Row(8, Ui.Text(site), Ui.IconButton(Glyphs.Cancel, $"Dark pages change {site} again", () => _ = C.Run(["circadian", "exempt", "remove", site], then: _ => Read()))));
        }
    }

    void BuildLane(Lane lane)
    {
        var editor = new ClipEditor(window, lane);
        editor.Changed += (e, previewOver) => _ = Store(e, previewOver);
        editor.Preview += (e, value) => Preview(e.LaneName, value);
        var sw = Ui.Switch($"{lane.Title} on", lane.On, on => { if (!filling) _ = C.Run(["circadian", "lane", lane.Name, on ? "on" : "off"], then: _ => Read()); }, "Lane_" + lane.Name);
        var value = Ui.Dim("", wrap: false);
        var top = new Grid { ColumnSpacing = 10 };
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        top.Children.Add(Ui.Row(10, Ui.Strong(lane.Title), value));
        var right = Ui.Row(8, Ui.Button("Points…", () => _ = editor.EditPoints(), "LanePoints_" + lane.Name), sw);
        Grid.SetColumn(right, 1);
        top.Children.Add(right);
        clips.Children.Add(Ui.Column(4, top, Ui.Note(lane.Detail), editor));
        lanes[lane.Name] = (editor, sw, value);
    }

    void Warnings(CircadianView v)
    {
        var notes = new List<(string Text, string? Label, List<string>? Command)>();
        var byName = v.Lanes.ToDictionary(l => l.Name);
        foreach (var lane in v.Lanes.Where(l => l.PausedUntil > 0))
        {
            if (lane.Name == "software" && byName.TryGetValue("kelvin", out var k) && k.PausedUntil > 0) continue;
            var until = Clock.Show(DateTimeOffset.FromUnixTimeSeconds(lane.PausedUntil).LocalDateTime.TimeOfDay.TotalMinutes);
            var what = lane.Name == "kelvin" ? "Software brightness and colour temperature are" : $"{lane.Title} is";
            notes.Add(($"{what} paused until {until}, because the level was changed by hand.", "Resume", ["circadian", "resume", lane.Name]));
        }
        bool On(string n) => byName.TryGetValue(n, out var l) && l.On;
        var hw = v.Hardware;
        if (v.On && hw.Daemon)
        {
            if (hw.NightLight && (On("kelvin") || On("software")))
                notes.Add(("Windows' own night light is on. It changes the same colours as these clips, and the two would take turns.", "Turn the night light off", ["circadian", "nightlight", "off"]));
            if (On("backlight") && hw.Backlights.Count == 0 && hw.External.Count == 0)
                notes.Add(("No screen brightness to drive: this computer has no built-in screen whose brightness Windows can set, and no screen answers DDC/CI. Software brightness works on every screen.", null, null));
            if ((On("kelvin") || On("software")) && hw.Gamma == "unknown")
                notes.Add(("No screen colours to change: the screens' colour ramps can't be set here.", null, null));
        }
        var key = string.Join("|", notes.Select(n => n.Text));
        if (key == warningsShown) return;
        warningsShown = key;
        warnings.Children.Clear();
        foreach (var (text, label, command) in notes)
        {
            var bar = new InfoBar { Message = text, IsOpen = true, IsClosable = false, Severity = command is null || label!.Contains("night") ? InfoBarSeverity.Warning : InfoBarSeverity.Informational };
            if (command is not null) bar.ActionButton = Ui.Button(label!, () => _ = C.Run(command, then: _ => Read()));
            warnings.Children.Add(bar);
        }
    }

    async Task Store(ClipEditor editor, bool previewOver)
    {
        var result = await C.Session.RunAsync(["circadian", "lane", editor.LaneName, "--points", Clips.Text(editor.Points)]);
        if (!result.Ok) await C.Dialogs.Message(result.Output.Length > 0 ? result.Output : "The clip couldn't be saved.");
        if (previewOver) await C.Session.RunAsync(["circadian", "preview", editor.LaneName, "--end"]);
        await Read();
    }

    /// <summary>Shows a dragged point's value on the screen, at most every
    /// 300 ms, and only for a clip KC9K is driving.</summary>
    void Preview(string laneName, double value)
    {
        var lane = view?.Lanes.FirstOrDefault(l => l.Name == laneName);
        if (lane is null || view is not { On: true } || !lane.On || lane.PausedUntil > 0) return;
        previewNext = (laneName, value);
        if (!previewBusy) _ = SendPreview();
    }

    async Task SendPreview()
    {
        previewBusy = true;
        try
        {
            while (previewNext is { } next)
            {
                var wait = 300 - (DateTime.Now - previewAt).TotalMilliseconds;
                if (wait > 0) await Task.Delay((int)wait + 1);
                if (previewNext is not { } latest) break;
                previewNext = null;
                previewAt = DateTime.Now;
                await C.Session.RunAsync(["circadian", "preview", latest.Lane, latest.Value.ToString("0.##", CultureInfo.InvariantCulture)]);
                _ = next;
            }
        }
        finally { previewBusy = false; }
    }

    async Task UsePreset()
    {
        var name = Key(preset);
        if (name.Length > 0 && await C.Dialogs.Confirm("Replace the three clips?", "Their points are replaced by the preset's. Which clips are on stays.", "Replace"))
            await C.Run(["circadian", "preset", name], then: _ => Read());
    }

    /// <summary>The next 24 hours on the screen ('kc9k circadian play') while a
    /// playhead runs over the clips; then everything is back to now.</summary>
    async Task PlayDay()
    {
        if (view is not { On: true })
        {
            await C.Dialogs.Message("Switch the circadian rhythm on to play the day.", "Not playing");
            return;
        }
        var start = DateTime.Now.TimeOfDay.TotalMinutes;
        var started = DateTime.Now;
        playing = true;
        play.IsEnabled = false;
        var timer = window.DispatcherQueue.CreateTimer();
        timer.Interval = TimeSpan.FromMilliseconds(100);
        timer.Tick += (_, _) =>
        {
            var done = Math.Min((DateTime.Now - started).TotalSeconds / PlaySeconds, 1);
            foreach (var (editor, _, _) in lanes.Values)
            {
                editor.Playhead = (start + 1440 * done) % 1440;
                editor.SetData(view.Lanes.First(l => l.Name == editor.LaneName), view.Minute, view.On);
            }
        };
        timer.Start();
        var result = await C.Session.RunAsync(["circadian", "play", "--seconds", PlaySeconds.ToString(CultureInfo.InvariantCulture)], timeout: TimeSpan.FromSeconds(PlaySeconds + 45));
        timer.Stop();
        playing = false;
        foreach (var (editor, _, _) in lanes.Values) editor.Playhead = null;
        play.IsEnabled = true;
        if (!result.Ok) await C.Dialogs.Message(result.Output.Length > 0 ? result.Output : "The day couldn't be played.");
        await Read();
        _ = playing;
    }

    async Task DarkChanged()
    {
        var key = (dark.SelectedItem as RadioButton)?.Tag as string ?? "system";
        if (key == "scheduled") await SetDarkTimes();
        else await C.Run(["circadian", "dark", key], then: _ => Read());
    }

    async Task SetDarkTimes()
    {
        if (!lightFrom.IsValid || !lightTo.IsValid) return;
        await C.Run(["circadian", "dark", "light-between", Clock.Text(lightFrom.Minute!.Value), Clock.Text(lightTo.Minute!.Value)], then: _ => Read());
    }

    async Task AddExempt()
    {
        var site = exemptEntry.Text.Trim();
        if (site.Length > 0) await C.Run(["circadian", "exempt", "add", site], then: _ => { exemptEntry.Text = ""; return Read(); });
    }
}
