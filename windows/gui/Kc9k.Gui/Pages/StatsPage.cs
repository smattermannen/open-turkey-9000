// §4.9 Statistics (gui/kc9k-gui.py build_stats): time used and times blocked
// per site and app, a time frame (presets or custom), a bar per day in one
// colour per site or app, per user, top 5 or 10, search, filter by block,
// JSON export, delete, on/off, private windows left out. Read with `kc9k
// stats --json` for the page's frame and filters.
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;
using Microsoft.UI.Xaml.Media;

namespace Kc9k.Gui;

public sealed class StatsPage : IPage
{
    readonly MainWindow window;
    readonly ComboBox period, user, top, blockFilter;
    readonly Button rangeButton;
    readonly Dictionary<string, ToggleButton> kinds = [];
    readonly TextBox search;
    readonly InfoBar offBar;
    readonly StatTile total = new("Total", "StatsTotal"), average = new("Average per day", "StatsAverage"), count = new("Different websites", "StatsCount");
    readonly DailyChart daily = new();
    readonly WrapPanel legend = new() { HorizontalSpacing = 14 };
    readonly UIElement dailySection;
    readonly TopChart topChart = new();
    readonly TextBlock topTitle = Ui.Heading("Most used websites"), topEmpty = Ui.Dim("Nothing recorded in this period.");
    readonly ToggleSwitch statsSwitch, privateSwitch;
    readonly Series series = new();
    (DateOnly First, DateOnly Last)? range;
    string lastPeriod = "7";
    string kind = "site";
    List<UsageRow> rows = [];
    bool enabled = true, filling, reading;
    string usersShown = "", blocksShown = "";
    Controller C => window.Controller;
    public string Area => "stats";
    public UIElement View { get; }

    public StatsPage(MainWindow window)
    {
        this.window = window;
        period = new ComboBox { MinWidth = 160 }.Named("Time frame", "StatsPeriod");
        foreach (var (key, label) in new[] { ("1", "Today"), ("7", "Last 7 days"), ("30", "Last 30 days"), ("90", "Last 90 days"), ("custom", "Custom…") })
            period.Items.Add(new ComboBoxItem { Content = label, Tag = key });
        period.SelectedIndex = 1;
        period.SelectionChanged += (_, _) => { if (!filling) _ = PeriodChanged(); };
        rangeButton = Ui.Button("", () => _ = ChooseRange(true), "StatsRange", glyph: Glyphs.Calendar).Tip("Change the time frame");
        rangeButton.Visibility = Visibility.Collapsed;
        user = new ComboBox { MinWidth = 140, Visibility = Visibility.Collapsed }.Named("Whose time", "StatsUser");
        user.Items.Add(new ComboBoxItem { Content = "All users", Tag = "" });
        user.SelectedIndex = 0;
        user.SelectionChanged += (_, _) => { if (!filling) _ = Read(); };
        var kindRow = Ui.Wrap(0);
        foreach (var (key, label) in new[] { ("site", "Websites"), ("app", "Apps"), ("blocked", "Times blocked") })
        {
            var b = new ToggleButton { Content = label, IsChecked = key == "site" }.Named(label, "StatsKind_" + key);
            b.Click += (_, _) => { kind = key; foreach (var (k, t) in kinds) t.IsChecked = k == key; Show(); };
            kinds[key] = b;
            kindRow.Children.Add(b);
        }
        top = new ComboBox().Named("How many", "StatsTop");
        top.Items.Add(new ComboBoxItem { Content = "Top 5", Tag = "5" });
        top.Items.Add(new ComboBoxItem { Content = "Top 10", Tag = "10" });
        top.SelectedIndex = 0;
        top.SelectionChanged += (_, _) => Show();
        blockFilter = new ComboBox { MinWidth = 160 }.Named("Which block", "StatsBlock");
        blockFilter.Items.Add(new ComboBoxItem { Content = "All blocks", Tag = "" });
        blockFilter.SelectedIndex = 0;
        blockFilter.SelectionChanged += (_, _) => { if (!filling) _ = Read(); };
        search = new TextBox { PlaceholderText = "Find a site or app", MinWidth = 200 }.Named("Find a site or app", "StatsSearch");
        search.TextChanged += (_, _) => _ = Read();
        offBar = new InfoBar { Severity = InfoBarSeverity.Informational, IsClosable = false, Message = "Statistics are off, so nothing new is recorded. Turn them on under Recording, below." };
        var tiles = new Grid { ColumnSpacing = 12 };
        foreach (var (tile, i) in new[] { total, average, count }.Select((t, i) => (t, i)))
        {
            tiles.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            var c = Ui.Card(tile);
            Grid.SetColumn(c, i);
            tiles.Children.Add(c);
        }
        dailySection = Ui.Section("Per day", daily, legend);
        topChart.ColorOf = name => Brush(name);
        statsSwitch = Ui.Switch("Record time spent on websites and apps", true, on => _ = C.Run(["stats", on ? "on" : "off"], then: _ => C.RefreshStats()), "StatsOn");
        privateSwitch = Ui.Switch("Leave out private windows", false, on => _ = C.Run(["stats", "private", on ? "skip" : "count"]), "StatsPrivate");
        View = Ui.Page(Ui.Column(18,
            Ui.PageHeader("Statistics", "Time on sites and apps, and how often pages were blocked. Only on this computer."),
            Ui.Wrap(12, period, rangeButton, user, kindRow),
            Ui.Wrap(12, top, blockFilter, search,
                Ui.Button("Export as JSON…", () => _ = Export(), "StatsExport", glyph: Glyphs.Export)),
            offBar, tiles, dailySection,
            Ui.Column(8, topTitle, topChart, topEmpty),
            Ui.Dim("App time counts the app in the focused window; website time counts the active tab in the browser. Time behind a locked screen or after two minutes without input isn't counted. Everything stays on this computer."),
            Ui.Section("Recording",
                Ui.SettingRow("Record time spent on websites and apps", "Stored only on this computer and kept for a year.", statsSwitch),
                Ui.SettingRow("Leave out private windows", "Time on websites in private windows isn't counted.", privateSwitch),
                Ui.Button("Delete all statistics…", () => _ = Clear(), "StatsClear", glyph: Glyphs.Delete))));
        daily.Format = Words.Spent;
    }

    Brush? Brush(string name) =>
        current.TryGetValue(name, out var slot) ? new SolidColorBrush(Parse(Series.Color(slot, View is FrameworkElement f && f.ActualTheme == ElementTheme.Dark))) : null;
    Dictionary<string, int?> current = [];

    public static Windows.UI.Color Parse(string hex) => Windows.UI.Color.FromArgb(255,
        byte.Parse(hex[1..3], NumberStyles.HexNumber), byte.Parse(hex[3..5], NumberStyles.HexNumber), byte.Parse(hex[5..7], NumberStyles.HexNumber));

    static string Key(ComboBox c) => (c.SelectedItem as ComboBoxItem)?.Tag as string ?? "";

    public void Update(GuiState s)
    {
        Ui.Set(statsSwitch, s.Stats);
        Ui.Set(privateSwitch, s.StatsSkipPrivate);
        var names = s.RealBlocks.Select(b => b.Name).OrderBy(n => n).ToList();
        if (string.Join("|", names) != blocksShown)
        {
            blocksShown = string.Join("|", names);
            var chosen = Key(blockFilter);
            filling = true;
            blockFilter.Items.Clear();
            blockFilter.Items.Add(new ComboBoxItem { Content = "All blocks", Tag = "" });
            foreach (var n in names) blockFilter.Items.Add(new ComboBoxItem { Content = n, Tag = n });
            blockFilter.SelectedIndex = Math.Max(0, names.IndexOf(chosen) + 1);
            filling = false;
        }
    }

    /// <summary>The controller read statistics (every 30 s): read the page's own.</summary>
    public void UpdateStats(StatsOutput stats)
    {
        enabled = stats.Enabled;
        _ = Read();
    }

    (DateOnly First, DateOnly Last) Frame()
    {
        var today = DateOnly.FromDateTime(DateTime.Today);
        if (Key(period) == "custom" && range is { } r) return r;
        var days = int.TryParse(Key(period), out var d) ? d : 7;
        return (today.AddDays(-(days - 1)), today);
    }

    List<string> Args()
    {
        var (first, last) = Frame();
        List<string> args = Key(period) == "custom"
            ? ["--from", first.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture), "--to", last.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)]
            : ["--days", (last.DayNumber - first.DayNumber + 1).ToString(CultureInfo.InvariantCulture)];
        if (Key(user) is { Length: > 0 } u) args.AddRange(["--user", u]);
        if (search.Text.Trim() is { Length: > 0 } q) args.AddRange(["--search", q]);
        if (Key(blockFilter) is { Length: > 0 } b) args.AddRange(["--block", b]);
        return args;
    }

    async Task Read()
    {
        if (reading) return;
        reading = true;
        try
        {
            var block = Key(blockFilter);
            var secret = block.Length > 0 && C.Session.BlockPasswords.TryGetValue(block, out var pw) ? pw + "\n" : null;
            var result = await C.Session.RunAsync(["stats", "--json", .. Args()], secret);
            if (!result.Ok)
            {
                if (result.Output.Contains("block password"))
                {
                    await C.Dialogs.Message("That block's sites and apps are behind its block password: open the block and use Show contents… first.", "Hidden");
                    blockFilter.SelectedIndex = 0;
                }
                return;
            }
            var stats = StatsOutput.Parse(result.Output[Math.Max(0, result.Output.IndexOf('{'))..]);
            rows = stats.Rows;
            enabled = stats.Enabled;
            var users = stats.Users;
            if (string.Join("|", users) != usersShown)
            {
                usersShown = string.Join("|", users);
                var chosen = Key(user);
                filling = true;
                user.Items.Clear();
                user.Items.Add(new ComboBoxItem { Content = "All users", Tag = "" });
                foreach (var u in users) user.Items.Add(new ComboBoxItem { Content = u, Tag = u });
                user.SelectedIndex = Math.Max(0, users.IndexOf(chosen) + 1);
                filling = false;
            }
            user.Visibility = users.Count > 1 ? Visibility.Visible : Visibility.Collapsed;
            Show();
        }
        catch (System.Text.Json.JsonException e) { Log.Write("stats page: " + e.Message); }
        finally { reading = false; }
    }

    void Show()
    {
        Func<long, string> fmt = kind == "blocked" ? n => $"{n}×" : Words.Spent; // §4.9: times blocked are counts
        daily.Format = topChart.Format = fmt;
        var (first, last) = Frame();
        var dates = Enumerable.Range(0, last.DayNumber - first.DayNumber + 1).Select(first.AddDays).ToList();
        var perDay = dates.ToDictionary(d => d.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture), _ => 0L);
        var byName = perDay.Keys.ToDictionary(k => k, _ => new Dictionary<string, long>());
        var totals = new Dictionary<string, long>();
        foreach (var r in rows.Where(r => r.Kind == kind && perDay.ContainsKey(r.Day)))
        {
            perDay[r.Day] += r.Seconds;
            byName[r.Day][r.Name] = byName[r.Day].GetValueOrDefault(r.Name) + r.Seconds;
            totals[r.Name] = totals.GetValueOrDefault(r.Name) + r.Seconds;
        }
        var sum = perDay.Values.Sum();
        offBar.IsOpen = !enabled;
        total.Set(fmt(sum));
        average.Set(fmt(sum / Math.Max(dates.Count, 1)));
        average.Visibility = dates.Count > 1 ? Visibility.Visible : Visibility.Collapsed;
        count.Set(totals.Count.ToString(CultureInfo.InvariantCulture));
        if (count.Children[0] is TextBlock t) t.Text = kind switch { "site" => "Different websites", "app" => "Different apps", _ => "Different sites blocked" };
        dailySection.Visibility = dates.Count > 1 ? Visibility.Visible : Visibility.Collapsed;
        var ranked = totals.OrderByDescending(x => x.Value).ThenBy(x => x.Key).ToList();
        var topN = int.TryParse(Key(top), out var n) ? n : 5;
        var shownTop = ranked.Take(topN).ToList();
        // The chart: the top names in at most seven colours, the rest Other.
        var named = shownTop.Select(x => x.Key).Take(Series.Light.Length).ToList();
        current = series.Assign(named);
        var colored = named.Where(x => current[x] is not null).ToList();
        var dark = View is FrameworkElement fe && fe.ActualTheme == ElementTheme.Dark;
        var stacks = dates.Select(d =>
        {
            var day = byName[d.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)];
            var stack = colored.Select(x => (x, day.GetValueOrDefault(x))).ToList();
            var rest = day.Where(kv => !colored.Contains(kv.Key)).Sum(kv => kv.Value);
            if (rest > 0) stack.Add(("Other", rest));
            return (d, stack);
        }).ToList();
        var legendItems = colored.Select(x => (x, Series.Color(current[x], dark))).ToList();
        if (stacks.Any(s => s.stack.Any(x => x.Item1 == "Other"))) legendItems.Add(("Other", Series.Other));
        daily.Set(stacks, legendItems.ToDictionary(x => x.Item1, x => x.Item2));
        legend.Children.Clear();
        foreach (var (name, color) in legendItems)
            legend.Children.Add(Ui.Row(6, new Border { Width = 12, Height = 12, CornerRadius = new CornerRadius(3), Background = new SolidColorBrush(Parse(color)) }, Ui.Text(name, wrap: false)));
        topTitle.Text = kind switch { "site" => "Most used websites", "app" => "Most used apps", _ => "Most blocked sites" };
        topChart.SetItems(shownTop.Select(x => (x.Key, x.Value)).ToList());
        topChart.Visibility = shownTop.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        topEmpty.Visibility = shownTop.Count > 0 ? Visibility.Collapsed : Visibility.Visible;
    }

    async Task PeriodChanged()
    {
        if (Key(period) == "custom")
        {
            if (!await ChooseRange(false))
            {
                filling = true;
                foreach (var i in period.Items.OfType<ComboBoxItem>()) if ((string)i.Tag == lastPeriod) period.SelectedItem = i;
                filling = false;
                return;
            }
        }
        else lastPeriod = Key(period);
        rangeButton.Visibility = Key(period) == "custom" ? Visibility.Visible : Visibility.Collapsed;
        await Read();
    }

    /// <summary>§4.9 custom time frame, within the year statistics are kept.</summary>
    async Task<bool> ChooseRange(bool read)
    {
        var today = DateOnly.FromDateTime(DateTime.Today);
        var (first, last) = range ?? (today.AddDays(-29), today);
        var from = new CalendarDatePicker { Date = first.ToDateTime(TimeOnly.MinValue) }.Named("From", "StatsFrom");
        var to = new CalendarDatePicker { Date = last.ToDateTime(TimeOnly.MinValue) }.Named("To", "StatsTo");
        var note = Ui.Note("");
        DateOnly D(CalendarDatePicker p) => DateOnly.FromDateTime((p.Date ?? DateTimeOffset.Now).Date);
        string? Problem() => Words.StatsRangeProblem(D(from), D(to), today);
        void Check()
        {
            var p = Problem();
            var days = D(to).DayNumber - D(from).DayNumber + 1;
            note.Text = p ?? (days == 1 ? "1 day" : $"{days} days");
            note.Style = Theme.Style(p is null ? "Kc9kNote" : "Kc9kProblem");
        }
        Check();
        var body = Ui.Column(10, Ui.Dim("Statistics are kept for a year."), Ui.Wrap(8, Ui.Text("From"), from, Ui.Text("to"), to), note);
        if (!await C.Dialogs.Ask("Time frame", body, "Show", () => Problem() is null, w =>
            {
                from.DateChanged += (_, _) => { Check(); w(); };
                to.DateChanged += (_, _) => { Check(); w(); };
            })) return false;
        range = (D(from), D(to));
        rangeButton.Content = Ui.Row(8, Ui.Icon(Glyphs.Calendar, 14), new TextBlock { Text = Words.Range(range.Value.First, range.Value.Last) });
        AutomationProperties.SetName(rangeButton, "Time frame: " + Words.Range(range.Value.First, range.Value.Last));
        if (read) await Read();
        return true;
    }

    async Task Export()
    {
        var path = await Files.Save(window, "kc9k-statistics.json", ".json", "JSON");
        if (path is null) return;
        var output = await C.Read(["stats", "--json", .. Args()]);
        if (output is null) return;
        try { await File.WriteAllTextAsync(path, output + "\n"); }
        catch (Exception e) { await C.Dialogs.Message(e.Message); }
    }

    async Task Clear()
    {
        if (await C.Dialogs.Confirm("Delete all statistics?", "Every recorded day is removed. This can't be undone.", "Delete", destructive: true))
            await C.Run(["stats", "clear"], then: _ => C.RefreshStats());
    }
}

/// <summary>A bar per day, stacked by site or app in their colours; each bar
/// says its day and total (Narrator, tooltip).</summary>
public sealed partial class DailyChart : Grid
{
    public Func<long, string> Format { get; set; } = Words.Spent;
    List<(DateOnly Day, List<(string Name, long Value)> Stack)> data = [];
    Dictionary<string, string> colors = [];

    public DailyChart()
    {
        Height = 220;
        SizeChanged += (_, _) => Draw();
        AutomationProperties.SetName(this, "Per day");
    }

    public void Set(List<(DateOnly, List<(string, long)>)> stacks, Dictionary<string, string> palette)
    {
        data = stacks;
        colors = palette;
        Draw();
    }

    void Draw()
    {
        Children.Clear();
        ColumnDefinitions.Clear();
        RowDefinitions.Clear();
        if (data.Count == 0 || ActualWidth <= 0) return;
        RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        var max = Math.Max(1, data.Max(d => d.Stack.Sum(s => s.Value)));
        var height = Math.Max(ActualHeight - 24, 20);
        var labelEvery = Math.Max(1, (int)Math.Ceiling(data.Count / Math.Max(ActualWidth / 56, 1)));
        for (var i = 0; i < data.Count; i++)
        {
            ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            var (day, stack) = data[i];
            var column = new StackPanel { VerticalAlignment = VerticalAlignment.Bottom, Margin = new Thickness(2, 0, 2, 0) };
            foreach (var (name, value) in Enumerable.Reverse(stack))
            {
                if (value <= 0) continue;
                column.Children.Add(new Border
                {
                    Height = Math.Max(1, height * value / max),
                    Background = new SolidColorBrush(StatsPage.Parse(colors.GetValueOrDefault(name, Series.Other))),
                    CornerRadius = new CornerRadius(2),
                    Margin = new Thickness(0, 1, 0, 0),
                });
            }
            var sum = stack.Sum(s => s.Value);
            var words = $"{day.ToString("ddd d MMM", CultureInfo.InvariantCulture)}: {Format(sum)}" +
                        (stack.Count > 0 ? " (" + string.Join(", ", stack.Where(s => s.Value > 0).Select(s => $"{s.Name} {Format(s.Value)}")) + ")" : "");
            var holder = new Grid { Background = new SolidColorBrush(Microsoft.UI.Colors.Transparent) };
            holder.Children.Add(column);
            ToolTipService.SetToolTip(holder, words);
            AutomationProperties.SetName(holder, words);
            Grid.SetColumn(holder, i);
            Children.Add(holder);
            if (i % labelEvery == 0)
            {
                var label = Ui.Note(data.Count <= 7 ? day.ToString("ddd", CultureInfo.InvariantCulture) : day.ToString("d MMM", CultureInfo.InvariantCulture));
                label.HorizontalAlignment = HorizontalAlignment.Center;
                label.TextWrapping = TextWrapping.NoWrap;
                Grid.SetColumn(label, i);
                Grid.SetRow(label, 1);
                Children.Add(label);
            }
        }
    }
}
