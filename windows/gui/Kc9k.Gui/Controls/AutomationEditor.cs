// §3.6, §4.5 automation (the GTK GUI's AutomationEditor and a block's
// automation tab in one): enabled or disabled by default, both or either,
// its windows on a week (and as a list, the keyboard's way), schedules
// applied, places. Kinds: "block", "mode" (one automation for its blocks),
// "monk", "schedule" (a schedule kept outside blocks: its windows only).
// Every change runs the commands a block's does with the right name and
// flag; the CLI decides what weakens (R-MODE-2, R-MONK-3).
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;

namespace Kc9k.Gui;

public sealed partial class AutomationEditor : StackPanel
{
    readonly MainWindow window;
    readonly string kind;
    readonly TextBlock summary = Ui.Text("");
    readonly Dictionary<(string, string), ToggleButton> choices = [];
    readonly UIElement needRow;
    readonly WeekCalendar calendar = new();
    readonly TextBlock total = Ui.Note("");
    readonly EditableList windowList;
    readonly WrapPanel applied = new() { HorizontalSpacing = 6 };
    readonly PlaceRules? places;
    bool filling;
    string name = "";
    List<TimeWindow> windows = [];
    AutomationSettings settings = new();
    List<string> appliedNames = [];
    bool active;

    Controller C => window.Controller;

    public AutomationEditor(MainWindow window, string kind)
    {
        this.window = window;
        this.kind = kind;
        Spacing = 12;
        Children.Add(summary);
        if (kind is "block" or "mode")
            Children.Add(Choice("default", ("disabled", "Disabled by default"), ("enabled", "Enabled by default")));
        needRow = Ui.Wrap(8, Ui.Text("With windows and places:"), Choice("needs", ("either", "Either is enough"), ("both", "Both at once")));
        if (kind != "schedule") Children.Add(needRow);

        Children.Add(Ui.Row(6, Ui.Dim("Drag down a day to add a window; click one to change or remove it; drag it to move it."),
            Ui.HelpButton("Filled: the block is enabled. Outlined: an allow window, when it is lifted. Dashed: from a schedule, changed on the Schedules page. Other blocks' windows are greyed.", "the week")));
        calendar.Create += (column, start, end) => _ = CreateWindow(column, start, end);
        calendar.Pick += index => _ = EditWindow(index);
        calendar.Move += (index, days, start, end) => _ = MoveWindow(index, days, start, end);
        Children.Add(calendar);
        var presets = Ui.Wrap(6, Ui.Text("Add:"));
        foreach (var (label, days, from, to) in Tables.SchedulePresets)
            presets.Children.Add(Ui.Button(label, () => _ = AddWindow(days, from, to), $"{kind}_Preset_{label.Replace(' ', '_')}").Tip($"{days} {from}-{to}"));
        presets.Children.Add(total);
        Children.Add(presets);
        windowList = new EditableList("Windows", "weekdays 09:00 17:00",
            values => AddTyped(values), (_, number) => RemoveWindow(number - 1),
            help: "Days, start and end: weekdays 09:00 17:00, or mon,wed,fri 18:00 22:00.",
            split: t => t.Trim().Length > 0 ? [t.Trim()] : [], id: kind + "_Windows");
        windowList.Activated += text => _ = EditWindow(windowList.Items.ToList().IndexOf(text));
        Children.Add(windowList);
        if (kind != "schedule")
        {
            Children.Add(Ui.Wrap(6, Ui.Text("Schedules:"), applied,
                Ui.Button("Apply schedule…", () => _ = ApplySchedule(), kind + "_ApplySchedule")
                    .Tip("Evening, Weekend…: made on the Schedules page; changing it there changes this too")));
            Children.Add(Ui.Row(6, Ui.Heading("Places"), Ui.HelpButton(
                "Being at one of these places switches it as its default says: on when it is disabled by default, off when it is enabled by default. As many places as you like. A place is known by the Wi-Fi networks in range when you save it (yours and the neighbours'); the computer is there when as many of them as you choose are in range again. With no network seen (Wi-Fi off), places count as whatever keeps the block enabled, and Wi-Fi is switched back on.", "places")));
            places = new PlaceRules(window, p => AddPlace(p), p => C.Run(["place", "rule", name, p, "off", .. Flags()]),
                () => SavePlace(window, name, Flags()), kind);
            Children.Add(places);
        }
    }

    UIElement Choice(string setting, params (string Value, string Label)[] items)
    {
        var row = Ui.Row(0);
        foreach (var (value, label) in items)
        {
            var b = new ToggleButton { Content = label }.Named(label, $"{kind}_{setting}_{value}");
            b.Click += (_, _) =>
            {
                if (filling) return;
                b.IsChecked = true;
                _ = C.Run(["automation", name, setting, value, .. Flags()]);
            };
            choices[(setting, value)] = b;
            row.Children.Add(b);
        }
        return row;
    }

    /// <summary>§4.2 Names: a mode or schedule may share a block's name, so its commands say which.</summary>
    List<string> Flags() => kind switch { "mode" or "monk" => ["--mode"], "schedule" => ["--schedule"], _ => [] };

    /// <summary>A window follows the default: enabled by default, it turns it off (an allow window).</summary>
    List<string> WindowFlags() => kind is "block" or "mode" && settings.DefaultOn ? ["--allow"] : [];

    /// <summary>R-LOCK-12: adding to monk mode's time while it is on asks first.</summary>
    async Task<List<string>?> MonkOk(string what)
    {
        if (kind != "monk" || !active) return [];
        var (ok, _) = await C.Dialogs.ConfirmLock([$"monk mode (no websites, no internet, can't be switched off while it runs): also {what}"]);
        return ok ? ["--yes"] : null;
    }

    async Task AddWindow(string days, string from, string to)
    {
        if (name.Length == 0) return;
        if (await MonkOk($"{days} {from}–{to}") is not { } yes) return;
        await C.Run(["schedule", "add", name, "--days", days, "--from", from, "--to", to, .. WindowFlags(), .. yes, .. Flags()]);
    }

    async Task AddTyped(IReadOnlyList<string> values)
    {
        foreach (var v in values)
        {
            var parts = v.Split(' ', StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length != 3)
            {
                await C.Dialogs.Message("Type days, start and end, for example: weekdays 09:00 17:00", "Not added");
                return;
            }
            await AddWindow(parts[0], parts[1], parts[2]);
        }
    }

    async Task CreateWindow(int column, int start, int end)
    {
        var day = WeekCalendar.DayKey(column);
        if (await C.Dialogs.Confirm($"Add {CultureInfo.InvariantCulture.TextInfo.ToTitleCase(day)} {Clock.Show(start)}–{Clock.Show(end, true)}?",
                "Tick more days afterwards by clicking it.", "Add"))
            await AddWindow(day, Clock.Text(start), end >= 1440 ? "24:00" : Clock.Text(end));
    }

    async Task FromSchedule(TimeWindow w)
    {
        if (await C.Dialogs.Confirm($"This window comes from the schedule {w.Source}",
                "Change it on the Schedules page (that changes everything using it), or take the schedule off here.", "Open Schedules"))
            window.OpenSchedule(w.Source);
    }

    async Task MoveWindow(int index, int days, int start, int end)
    {
        if (index >= windows.Count) return;
        if (windows[index].Source.Length > 0)
        {
            await FromSchedule(windows[index]);
            return;
        }
        await Replace(index, days, start, end, windows[index].Allow);
    }

    /// <summary>Removes window index, then adds the new one (a change that
    /// weakens is refused by the CLI, and the old one stays).</summary>
    async Task Replace(int index, int days, int start, int end, bool allow)
    {
        var names = Words.Weekdays.Where(d => (days & Words.DayBit(d.Key)) != 0).Select(d => d.Key).ToList();
        if (names.Count == 0) return;
        var from = Clock.Text(start);
        var to = end >= 1440 ? "24:00" : Clock.Text(end);
        if (await MonkOk($"{string.Join(",", names)} {from}–{to}") is not { } yes) return;
        List<string> add = ["schedule", "add", name, "--days", string.Join(",", names), "--from", from, "--to", to,
            .. (allow ? new[] { "--allow" } : []), .. yes, .. Flags()];
        if (await C.Run(["schedule", "remove", name, (index + 1).ToString(CultureInfo.InvariantCulture), .. Flags()])) await C.Run(add);
    }

    Task RemoveWindow(int index) =>
        index < 0 || index >= windows.Count ? Task.CompletedTask : C.Run(["schedule", "remove", name, (index + 1).ToString(CultureInfo.InvariantCulture), .. Flags()]);

    /// <summary>§4.5: change a window's days (copy to days), times and, on a
    /// block, its break; or remove it.</summary>
    async Task EditWindow(int index)
    {
        if (index < 0 || index >= windows.Count) return;
        var w = windows[index];
        if (w.Source.Length > 0)
        {
            await FromSchedule(w);
            return;
        }
        var checks = new List<(int Bit, CheckBox Box)>();
        var days = Ui.Wrap(6);
        for (var column = 0; column < 7; column++)
        {
            var bit = WeekCalendar.Bit(column);
            var title = Words.Weekdays[(column + Clock.FirstDay) % 7].Title;
            var box = new CheckBox { Content = title, IsChecked = (w.Days & bit) != 0, MinWidth = 0 }.Named(title, "WindowDay_" + title);
            checks.Add((bit, box));
            days.Children.Add(box);
        }
        var from = new ClockField(w.Start);
        from.Named("From", "WindowFrom");
        var to = new ClockField(w.End, allowEnd: true);
        to.Named("To", "WindowTo");
        var body = Ui.Column(10, Ui.Dim("Tick more days to copy the window to them."), days, Ui.Wrap(8, Ui.Text("From"), from, Ui.Text("to"), to));
        DurationField? brk = null;
        if (kind == "block" && !w.Allow)
        {
            // §3.6: one break each time this window is open.
            brk = new DurationField(new DurationRule { Minimum = 60, Maximum = 4 * 3600, Zero = "No break" }, w.Break, width: 110);
            brk.Named("Break each time", "WindowBreak");
            body.Children.Add(Ui.Wrap(8, Ui.Text("Break each time"), brk));
        }
        var d = new ContentDialog
        {
            Title = "Window", Content = body, PrimaryButtonText = "Save", SecondaryButtonText = "Remove", CloseButtonText = "Cancel",
            PrimaryButtonStyle = Theme.Style("AccentButtonStyle"),
        };
        void Check() => d.IsPrimaryButtonEnabled = from.IsValid && to.IsValid && (brk?.IsValid ?? true) && from.Minute < to.Minute && checks.Any(c => c.Box.IsChecked == true);
        from.Changed += Check;
        to.Changed += Check;
        if (brk is not null) brk.Changed += Check;
        foreach (var (_, b) in checks) b.Click += (_, _) => Check();
        Check();
        var answer = await C.Dialogs.Show(d);
        if (answer == ContentDialogResult.Secondary)
        {
            if (await C.Dialogs.Confirm("Remove this window?", "A change that weakens a locked block is refused.", "Remove", destructive: true))
                await RemoveWindow(index);
            return;
        }
        if (answer != ContentDialogResult.Primary) return;
        var newDays = checks.Where(c => c.Box.IsChecked == true).Sum(c => c.Bit);
        int start = from.Minute!.Value, end = to.Minute!.Value;
        if ((newDays, start, end) != (w.Days, w.Start, w.End)) await Replace(index, newDays, start, end, w.Allow);
        else if (brk?.Seconds is long b && b != w.Break)
            await C.Run(["schedule", "break", name, (index + 1).ToString(CultureInfo.InvariantCulture), b > 0 ? brk.Arg() : "off"]);
    }

    async Task ApplySchedule()
    {
        var names = (C.State?.Schedules ?? []).Select(s => s.Name).Where(n => !appliedNames.Contains(n)).ToList();
        if (names.Count == 0)
        {
            if (await C.Dialogs.Confirm("No schedule to apply", "Make schedules such as Evening or Weekend on the Schedules page, then apply them here.", "Open Schedules"))
                window.Navigate("schedules");
            return;
        }
        var chosen = await C.Dialogs.Choose("Apply a schedule", "Its windows join these; changing it on the Schedules page changes them too.",
            [.. names.Select(n => (n, n))]);
        if (chosen is null) return;
        if (await MonkOk($"the schedule {chosen}") is { } yes) await C.Run(["schedule", "apply", name, chosen, .. yes, .. Flags()]);
    }

    async Task AddPlace(string place)
    {
        if (name.Length == 0) return;
        if (await MonkOk($"while the computer is at {place}") is { } yes) await C.Run(["place", "rule", name, place, "on", .. yes, .. Flags()]);
    }

    /// <summary>Shows a block, mode or schedule from gui-state.</summary>
    public void Show(string who, List<TimeWindow> wins, AutomationSettings set, bool continuous, List<string> appliedSchedules,
        List<string> placeNames, bool isActive, string subject, IEnumerable<TimeWindow>? others = null, string usedBy = "")
    {
        name = who;
        windows = wins;
        settings = set;
        appliedNames = appliedSchedules;
        active = isActive;
        calendar.SetWindows(wins, others);
        total.Text = Words.ScheduleSummary(wins).Split(" · ")[^1];
        windowList.SetItems(wins.Select(w => Words.WindowWords(w) + (w.Allow ? " (allow)" : "") + (w.Source.Length > 0 ? $" (from {w.Source})" : "")), who);
        var hasWindows = wins.Count > 0;
        var hasPlaces = placeNames.Count > 0;
        if (kind == "schedule")
            summary.Text = usedBy.Length > 0 ? $"Used by {usedBy}: a change here changes them too; one that weakens a locked block is refused."
                : "Used by nothing yet: apply it to blocks and modes in their Automation.";
        else if (continuous)
            summary.Text = kind == "monk" ? "No windows or places yet: add some below."
                : kind == "mode" ? "Continuous: its blocks are enabled whenever they are on. Add windows or places below to switch them all by themselves."
                : "Continuous: enabled whenever it is on. Add windows or places below to switch it by itself.";
        else
        {
            var by = string.Join(" and ", new[] { (hasWindows, "its windows"), (hasPlaces, "its places") }.Where(x => x.Item1).Select(x => x.Item2));
            summary.Text = kind switch
            {
                "monk" => $"Monk mode runs while {by} have it on, once switched on.",
                "mode" => $"Automated: {by} turn its blocks {(set.DefaultOn ? "off" : "on")}.",
                _ => $"Automated: {by} turn it {(set.DefaultOn ? "off" : "on")}; it is {(set.DefaultOn ? "enabled" : "lifted")} otherwise.",
            };
        }
        filling = true;
        foreach (var ((setting, value), b) in choices)
            b.IsChecked = setting == "default" ? (value == "enabled") == set.DefaultOn : (value == "both") == set.NeedBoth;
        filling = false;
        needRow.Visibility = hasWindows && hasPlaces ? Visibility.Visible : Visibility.Collapsed;
        if (kind == "schedule") return;
        applied.Children.Clear();
        foreach (var s in appliedSchedules)
        {
            var chip = Ui.Row(2, Ui.Text(s), Ui.IconButton(Glyphs.Cancel, $"Take the schedule {s} off", () => _ = C.Run(["schedule", "take-off", name, s, .. Flags()])));
            applied.Children.Add(Ui.Card(chip));
        }
        if (appliedSchedules.Count == 0) applied.Children.Add(Ui.Dim("none"));
        places?.Show(placeNames, subject, set.DefaultOn, set.NeedBoth, hasWindows);
    }

    public void SetEnabled(bool on)
    {
        foreach (var c in Children) if (c is Control k) k.IsEnabled = on;
        calendar.IsHitTestVisible = on;
        calendar.Opacity = on ? 1 : 0.6;
        windowList.SetEnabled(on);
    }

    /// <summary>§3.6: saves the Wi-Fi networks in range as a place and,
    /// ticked by default, adds it to the automation of target.</summary>
    public static async Task SavePlace(MainWindow window, string? target, List<string>? flags = null)
    {
        var c = window.Controller;
        var entry = new TextBox().Named("Place name", "PlaceName");
        var body = Ui.Column(10, Ui.Dim("A name for where the computer is now, such as home or library. Saving under a name that exists adds the networks seen now."), entry);
        CheckBox? rule = null;
        if (!string.IsNullOrEmpty(target))
        {
            rule = new CheckBox { Content = $"Add it to '{target}' (being here switches it)", IsChecked = true }.Named("Add it here", "PlaceRule");
            body.Children.Add(rule);
        }
        if (!await c.Dialogs.Ask("Save this place", body, "Save", () => entry.Text.Trim().Length > 0, w => entry.TextChanged += (_, _) => w())) return;
        var name = entry.Text.Trim();
        var withRule = rule?.IsChecked == true;
        await c.Run(["place", "add", name], timeout: TimeSpan.FromSeconds(60), then: async output =>
        {
            if (withRule && target is not null)
                await c.Run(["place", "rule", target, name, "on", .. flags ?? []], then: o => c.Dialogs.Message(output + "\n" + o, "Place saved"));
            else
                await c.Dialogs.Message(output, "Place saved");
        });
    }
}

/// <summary>§3.6, §4.5 (owner 2026-11-17): the places in an automation, a
/// row each saying plainly that it is added, what arriving there does, and
/// whether the computer is there now; below, a saved place to add and Save
/// where I am now as….</summary>
public sealed partial class PlaceRules : StackPanel
{
    readonly MainWindow window;
    readonly Func<string, Task> add;
    readonly Func<string, Task> remove;
    readonly StackPanel rows = Ui.Column(6);
    readonly TextBlock empty = Ui.Dim("");
    readonly ComboBox combo;
    readonly Button addButton;
    string shown = "";

    public PlaceRules(MainWindow window, Func<string, Task> add, Func<string, Task> remove, Func<Task>? saveHere, string id)
    {
        this.window = window;
        this.add = add;
        this.remove = remove;
        Spacing = 8;
        Children.Add(rows);
        Children.Add(empty);
        combo = new ComboBox { MinWidth = 180 }.Named("A saved place", id + "_PlaceChoice").Tip("A saved place (Places page)");
        addButton = Ui.Button("Add this place", () => { if (combo.SelectedItem is string p) _ = add(p); }, id + "_AddPlace");
        var pick = Ui.Wrap(6, combo, addButton);
        if (saveHere is not null) pick.Children.Add(Ui.Button("Save where I am now as…", () => _ = saveHere(), id + "_SaveHere"));
        Children.Add(pick);
    }

    public static string Effect(string subject, bool defaultOn, bool needBoth, bool hasWindows)
    {
        var state = defaultOn ? "off" : "on";
        if (needBoth && hasWindows) return $"Here during one of its windows, {subject} {(subject == "its blocks" ? "are" : "is")} {state}.";
        return $"Arriving here turns {subject} {state}.";
    }

    public void Show(List<string> added, string subject, bool defaultOn, bool needBoth, bool hasWindows)
    {
        var all = window.Controller.State?.Places ?? [];
        var effect = Effect(subject, defaultOn, needBoth, hasWindows);
        var key = string.Join("|", added) + effect + string.Join("|", all.Select(p => $"{p.Name}:{p.Here}"));
        if (key != shown)
        {
            shown = key;
            rows.Children.Clear();
            foreach (var name in added)
            {
                var here = all.FirstOrDefault(p => p.Name == name)?.Here == true;
                var g = new Grid { ColumnSpacing = 10 };
                g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
                g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                g.Children.Add(Ui.Icon(Glyphs.Accept));
                var text = Ui.Column(2, Ui.Row(8, Ui.Strong(name), Ui.Pill(new(here ? "You are here" : "Not here now", here ? PillKind.On : PillKind.Off))), Ui.Dim("Added. " + effect));
                Grid.SetColumn(text, 1);
                g.Children.Add(text);
                var rm = Ui.Button("Remove", () => _ = remove(name), "RemovePlace_" + name).Tip("Take this place out of the automation");
                Grid.SetColumn(rm, 2);
                g.Children.Add(rm);
                rows.Children.Add(Ui.Card(g));
            }
        }
        empty.Text = added.Count > 0 ? "" : all.Count > 0
            ? "No places added. A place added here switches it by itself: " + effect.ToLowerInvariant().Replace("arriving here", "arriving at the place")
            : "No places saved yet: save where the computer is now.";
        empty.Visibility = added.Count > 0 ? Visibility.Collapsed : Visibility.Visible;
        var saved = all.Select(p => p.Name).Where(n => !added.Contains(n)).ToList();
        var chosen = combo.SelectedItem as string;
        combo.Items.Clear();
        foreach (var n in saved) combo.Items.Add(n);
        if (saved.Count > 0) combo.SelectedIndex = Math.Max(0, saved.IndexOf(chosen ?? ""));
        combo.IsEnabled = addButton.IsEnabled = saved.Count > 0;
    }
}
