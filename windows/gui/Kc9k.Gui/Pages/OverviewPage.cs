// §4.1 Overview (R-GUI-3: tasks first): what is on, until when, how locked;
// starting a block is one click away (gui/kc9k-gui.py build_dashboard).
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class OverviewPage : IPage
{
    readonly MainWindow window;
    readonly TextBlock headline, detail, desktop, deviceLine, topEmpty;
    readonly WrapPanel devicePills;
    readonly DropDownButton start;
    readonly MenuFlyout startMenu = new();
    readonly StatTile tileActive, tileSites, tileApps;
    readonly StackPanel onNow = Ui.Column(0);
    readonly TopChart top = new();
    readonly StackPanel firstSteps = Ui.Column(8);
    readonly UIElement firstCard;
    GuiState? state;

    public string Area => "dashboard";
    public UIElement View { get; }

    public OverviewPage(MainWindow window)
    {
        this.window = window;
        headline = Ui.Text("", "SubtitleTextBlockStyle").Id("Headline");
        detail = Ui.Dim("").Id("HeadlineDetail");
        start = new DropDownButton { Content = "Start a block", Flyout = startMenu, Style = Theme.Style("AccentButtonStyle") }
            .Named("Start a block", "StartBlock");
        start.VerticalAlignment = VerticalAlignment.Center;
        devicePills = new WrapPanel { HorizontalSpacing = 6 };
        var heroGrid = new Grid { ColumnSpacing = 16 };
        heroGrid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        heroGrid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        heroGrid.Children.Add(Ui.Column(4, headline, detail, devicePills));
        Grid.SetColumn(start, 1);
        heroGrid.Children.Add(start);
        var hero = Ui.Card(heroGrid, "Kc9kHero");

        desktop = Ui.Dim("");
        desktop.Visibility = Visibility.Collapsed;

        tileActive = new StatTile("Blocks on", "TileActive");
        tileSites = new StatTile("Websites today", "TileSites");
        tileApps = new StatTile("Apps today", "TileApps");
        var tiles = new Grid { ColumnSpacing = 12 };
        foreach (var (tile, i) in new[] { tileActive, tileSites, tileApps }.Select((t, i) => (t, i)))
        {
            tiles.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            var c = Ui.Card(tile);
            Grid.SetColumn(c, i);
            tiles.Children.Add(c);
        }

        deviceLine = Ui.Dim("").Id("DeviceLine");
        topEmpty = Ui.Dim("Nothing recorded today yet.");

        var quick = Ui.Wrap(8,
            Ui.Button("New block…", () => _ = Actions.CreateBlock(window), "QuickNewBlock", glyph: Glyphs.Add),
            Ui.Button("Monk mode…", () => _ = Actions.MonkMode(window), "QuickMonk", glyph: Glyphs.Monk),
            Ui.Button("Frozen Chicken…", () => window.OpenMode(Tables.FrozenChicken), "QuickFreeze", glyph: Glyphs.Snow));

        // W38: a first view while nothing is set up. Not a wizard: a list of
        // what is left, each step opening its place; stricter options last.
        firstCard = Ui.Card(Ui.Column(10, Ui.Heading("Get started"), firstSteps,
            Ui.Dim("Stricter, when you want it: a settings password, tamper protection, strict mode and no-escape. Nothing is on until you turn it on."),
            Ui.Button("Guided setup…", () => _ = window.OpenGuidedSetup(), "FirstGuided")), "Kc9kCard");
        firstCard.Visibility = Visibility.Collapsed;
        AutomationProperties.SetName(firstCard, "Get started");
        AutomationProperties.SetAutomationId(firstCard, "FirstSteps");

        View = Ui.Page(Ui.Column(20,
            Ui.PageHeader("Overview", "What is on now, and how it is locked."),
            hero, firstCard, desktop, tiles,
            Ui.Section("On now", onNow),
            Ui.Section("Device", deviceLine),
            Ui.Section("Most used today", top, topEmpty),
            Ui.Plain("Quick actions", quick)));
    }

    public void Update(GuiState s)
    {
        state = s;
        desktop.Text = s.DesktopWarning;
        desktop.Visibility = s.DesktopWarning.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        var blocks = s.RealBlocks.ToList();
        var active = blocks.Where(b => b.Active).ToList();
        var locked = active.Where(b => b.Held || b.LockSeconds > 0 || b.CharLocked || b.PasswordLocked || b.RestartLocked).ToList();
        tileActive.Set(active.Count.ToString());

        // The headline: how many are on, how many locked, and the longest lock.
        if (active.Count == 0)
        {
            headline.Text = "Nothing is on right now";
            detail.Text = blocks.Count > 0 ? "Start a block to keep sites and apps away."
                : "Make a block first: the sites, words and apps to keep away.";
        }
        else
        {
            headline.Text = $"{Words.Plural(active.Count, "block")} on" + (locked.Count > 0 ? $" · {locked.Count} locked" : "");
            var longest = active.MaxBy(b => b.LockSeconds)!;
            if (longest.LockSeconds > 0)
                detail.Text = $"{longest.Name} is timer-locked until {Words.Date(DateTime.Now.AddSeconds(longest.LockSeconds))} ({Words.Seconds(longest.LockSeconds)}).";
            else if (locked.Count > 0)
                detail.Text = $"{string.Join(", ", locked.Select(b => b.Name))}: locked until unlocked.";
            else
                detail.Text = "Nothing is locked: any block can be stopped.";
        }

        var steps = Words.FirstSteps(s);
        firstCard.Visibility = steps.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        firstSteps.Children.Clear();
        foreach (var (key, text, done) in steps)
        {
            var mark = Ui.Icon(done ? Glyphs.Check : Glyphs.Circle, 14);
            var row = Ui.Row(10, mark, done ? Ui.Dim(text) : Ui.Text(text));
            if (!done)
                row.Children.Add(key switch
                {
                    "block" => Ui.Button("New block…", () => _ = Actions.CreateBlock(window), "First_block", accent: true),
                    "browser" => Ui.Button("Browsers…", window.OpenBrowsers, "First_browser"),
                    _ => Ui.Button("Blocks…", () => window.Navigate("blocks"), "First_start"),
                });
            AutomationProperties.SetName(row, (done ? "Done: " : "To do: ") + text);
            firstSteps.Children.Add(row);
        }

        startMenu.Items.Clear();
        var off = blocks.Where(b => !b.Active).ToList();
        foreach (var b in off)
        {
            var item = new MenuFlyoutItem { Text = b.Name };
            item.Click += (_, _) => _ = window.Controller.Run(["start", b.Name]);
            startMenu.Items.Add(item);
        }
        // §4.2 modes: a mode's blocks switched on at once.
        var modesOff = s.Modes.Where(m => !m.Monk && m.Blocks.Count > 0 && m.On < m.Blocks.Count).ToList();
        if (off.Count > 0 && modesOff.Count > 0) startMenu.Items.Add(new MenuFlyoutSeparator());
        foreach (var m in modesOff)
        {
            var item = new MenuFlyoutItem { Text = $"Mode: {m.Name} ({Words.Plural(m.Blocks.Count, "block")})" };
            item.Click += (_, _) => _ = window.Controller.Run(["mode", "on", m.Name]);
            startMenu.Items.Add(item);
        }
        if (off.Count > 0 || modesOff.Count > 0) startMenu.Items.Add(new MenuFlyoutSeparator());
        var create = new MenuFlyoutItem { Text = "New block…", Icon = new FontIcon { Glyph = Glyphs.Add } };
        create.Click += (_, _) => _ = Actions.CreateBlock(window);
        startMenu.Items.Add(create);
        start.Content = off.Count > 0 || modesOff.Count > 0 ? "Start a block" : "New block";

        devicePills.Children.Clear();
        if (s.FreezeSeconds > 0) devicePills.Children.Add(Ui.Pill(new($"Frozen Chicken {Words.Seconds(s.FreezeSeconds)}", PillKind.Locked)));
        if (s.MonkSeconds > 0) devicePills.Children.Add(Ui.Pill(new($"Monk mode {Words.Seconds(s.MonkSeconds)}", PillKind.Locked)));
        else if (s.Modes.FirstOrDefault(m => m.Monk) is { Active: true, TodayWindows.Count: > 0 } monk)
            devicePills.Children.Add(Ui.Pill(new("Monk mode today " + string.Join(", ", monk.TodayWindows), PillKind.Lifted)));

        onNow.Children.Clear();
        if (active.Count == 0) onNow.Children.Add(Ui.Dim("No block is on."));
        foreach (var b in active)
        {
            var extra = new List<string>();
            if (b.TodayWindows.Count > 0) extra.Add("today: " + string.Join(", ", b.TodayWindows));
            if (b.QuotaLimit > 0) extra.Add($"allowance {b.QuotaUsed / 60} of {b.QuotaLimit / 60} min");
            // §4.1 break status and ladder positions (W31).
            if (b.PausedSeconds > 0) extra.Add($"on a break for {Words.Seconds(b.PausedSeconds)}");
            else if (b.OnBreak) extra.Add("on a Pomodoro break");
            if (Words.LadderPosition(b.Nepsis, b.NepsisLevel, b.NepsisCount) is { } ladder) extra.Add(ladder);
            var grid = new Grid { ColumnSpacing = 12 };
            grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            grid.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            var texts = Ui.Column(2, Ui.Strong(b.Name));
            if (extra.Count > 0) texts.Children.Add(Ui.Note(string.Join(" · ", extra)));
            grid.Children.Add(texts);
            var pills = Ui.Pills(Words.BlockPills(b));
            pills.VerticalAlignment = VerticalAlignment.Center;
            Grid.SetColumn(pills, 1);
            grid.Children.Add(pills);
            var chevron = Ui.Icon(Glyphs.Next, 12);
            Grid.SetColumn(chevron, 2);
            grid.Children.Add(chevron);
            var row = new Button
            {
                Content = grid, HorizontalAlignment = HorizontalAlignment.Stretch, HorizontalContentAlignment = HorizontalAlignment.Stretch,
                Background = new Microsoft.UI.Xaml.Media.SolidColorBrush(Microsoft.UI.Colors.Transparent), BorderThickness = new Thickness(0),
                Padding = new Thickness(4, 8, 4, 8),
            };
            row.Named($"{b.Name}: {Words.BlockSummary(b)}", "OnNow_" + b.Name);
            row.Click += (_, _) => window.OpenBlock(b.Name);
            onNow.Children.Add(row);
        }

        var locks = new List<string> { s.Strict ? "Strict mode on" : "Strict mode off", s.NoEscape ? "no-escape on" : "no-escape off" };
        if (s.FreezeSeconds > 0) locks.Insert(0, $"Frozen Chicken ({s.FreezeMode}) for {Words.Seconds(s.FreezeSeconds)}");
        if (s.MonkSeconds > 0) locks.Insert(0, $"Monk mode for {Words.Seconds(s.MonkSeconds)}");
        if (Words.LadderPosition(s.Nepsis, s.NepsisLevel, s.NepsisCount) is { } global) locks.Add("global " + global);
        var line = string.Join(" · ", locks);
        deviceLine.Text = char.ToUpperInvariant(line[0]) + line[1..];
        if (window.Controller.Stats is { } stats) UpdateStats(stats);
    }

    public void UpdateStats(StatsOutput stats)
    {
        var today = DateTime.Today.ToString("yyyy-MM-dd");
        var rows = stats.Rows.Where(r => r.Day == today).ToList();
        long sites = rows.Where(r => r.Kind == "site").Sum(r => r.Seconds), apps = rows.Where(r => r.Kind == "app").Sum(r => r.Seconds);
        tileSites.Set(stats.Enabled ? Words.Spent(sites) : "off");
        tileApps.Set(stats.Enabled ? Words.Spent(apps) : "off");
        var items = rows.Where(r => r.Kind is "site" or "app").OrderByDescending(r => r.Seconds).Take(5)
            .Select(r => (r.Name, r.Seconds)).ToList();
        top.SetItems(items);
        top.Visibility = items.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        topEmpty.Visibility = items.Count > 0 ? Visibility.Collapsed : Visibility.Visible;
        topEmpty.Text = stats.Enabled ? "Nothing recorded today yet." : "Statistics are off. Turn them on in Statistics.";
    }
}
