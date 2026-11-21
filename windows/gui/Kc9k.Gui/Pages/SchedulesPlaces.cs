// §4.5 (owner 2026-11-17): Schedules and Places each have a page: made and
// changed there without a block; each shows what uses it.
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class SchedulesPage : IPage
{
    readonly MainWindow window;
    readonly StackPanel cards = Ui.Column(10);
    readonly TextBlock editorTitle = Ui.Heading("");
    readonly AutomationEditor editor;
    readonly StackPanel editorBox;
    string? shown;
    string cardsShown = "";
    Controller C => window.Controller;
    public string Area => "blocks";
    public UIElement View { get; }

    public SchedulesPage(MainWindow window)
    {
        this.window = window;
        editor = new AutomationEditor(window, "schedule");
        editorBox = Ui.Column(8, editorTitle, Ui.Card(editor));
        View = Ui.Page(Ui.Column(18,
            Ui.PageHeader("Schedules", "Weekly windows kept outside blocks, such as Evening or Weekend. Apply them to blocks and modes; changing one here changes everything using it.",
                "A block or mode can use any number of schedules beside its own windows (Apply schedule… in its Automation). Where they overlap they count once. A change that weakens a locked block using the schedule is refused as a whole. Deleting a schedule leaves its windows to what used it, as its own."),
            cards,
            Ui.Button("New schedule…", () => _ = Create(), "NewSchedule", accent: true, glyph: Glyphs.Add),
            editorBox));
    }

    public void Open(string name)
    {
        shown = name;
        cardsShown = "";
        if (C.State is { } s) Update(s);
    }

    public void Update(GuiState s)
    {
        var names = s.Schedules.Select(x => x.Name).ToList();
        if (shown is null || !names.Contains(shown)) shown = names.FirstOrDefault();
        var key = System.Text.Json.JsonSerializer.Serialize(s.Schedules, Kc9kJson.Options) + shown;
        if (key != cardsShown)
        {
            cardsShown = key;
            cards.Children.Clear();
            foreach (var sc in s.Schedules) cards.Children.Add(Card(sc));
            if (s.Schedules.Count == 0) cards.Children.Add(Ui.Dim("No schedules yet. Make Evening, Weekend or Work hours once, then apply them to any block or mode."));
        }
        var chosen = s.Schedules.FirstOrDefault(x => x.Name == shown);
        editorBox.Visibility = chosen is null ? Visibility.Collapsed : Visibility.Visible;
        if (chosen is null) return;
        editorTitle.Text = chosen.Name;
        var used = chosen.Blocks.Concat(chosen.Modes.Select(m => $"mode {m}")).ToList();
        editor.Show(chosen.Name, chosen.Windows, new AutomationSettings(), chosen.Windows.Count == 0, [], [], false, "", usedBy: string.Join(", ", used));
    }

    UIElement Card(Schedule sc)
    {
        var name = sc.Name;
        var title = new HyperlinkButton { Content = Ui.Text(name, "SubtitleTextBlockStyle", wrap: false), Padding = new Thickness(0) }
            .Named($"Show the schedule {name}", "ScheduleCard_" + name).Tip("Show and change its windows");
        title.Click += (_, _) => Open(name);
        var menu = new MenuFlyout();
        var rename = new MenuFlyoutItem { Text = "Rename…" };
        rename.Click += (_, _) => _ = Rename(name);
        var delete = new MenuFlyoutItem { Text = "Delete…" };
        delete.Click += (_, _) => _ = Delete(name);
        menu.Items.Add(rename);
        menu.Items.Add(delete);
        var more = new Button { Content = new FontIcon { Glyph = Glyphs.More, FontSize = 14 }, Flyout = menu }.Named($"More: {name}", "ScheduleMore_" + name);
        var top = new Grid();
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var left = Ui.Row(10, title);
        if (name == shown) left.Children.Add(Ui.Pill(new("Shown below", PillKind.On)));
        top.Children.Add(left);
        Grid.SetColumn(more, 1);
        top.Children.Add(more);
        var used = sc.Blocks.Concat(sc.Modes.Select(m => $"mode {m}")).ToList();
        return Ui.Card(Ui.Column(4, top, Ui.Dim(Words.ScheduleSummary(sc.Windows)), Ui.Dim(used.Count > 0 ? "Used by " + string.Join(", ", used) : "Not used yet")));
    }

    async Task Create()
    {
        var name = await C.Dialogs.AskText("New schedule", "A name, such as Evening or Weekend. Then give it windows below.",
            problem: t => t.Length == 0 ? "Type a name." : null, ok: "Create");
        if (!string.IsNullOrEmpty(name) && await C.Run(["schedule", "create", name])) Open(name);
    }

    async Task Rename(string name)
    {
        var renamed = await C.Dialogs.AskText("Rename the schedule", initial: name);
        if (!string.IsNullOrEmpty(renamed) && renamed != name && await C.Run(["schedule", "rename", name, renamed])) Open(renamed);
    }

    async Task Delete(string name)
    {
        if (await C.Dialogs.Confirm($"Delete the schedule {name}?", "Blocks and modes using it keep its windows as their own: nothing they block changes.", "Delete", destructive: true))
            await C.Run(["schedule", "delete", name]);
    }
}

public sealed class PlacesPage : IPage
{
    readonly MainWindow window;
    readonly StackPanel cards = Ui.Column(10);
    string shown = "";
    Controller C => window.Controller;
    public string Area => "blocks";
    public UIElement View { get; }

    public PlacesPage(MainWindow window)
    {
        this.window = window;
        View = Ui.Page(Ui.Column(18,
            Ui.PageHeader("Places", "Where the computer is, known by the Wi-Fi networks in range. Blocks and modes are switched by being at one.",
                "A place is the set of Wi-Fi networks in range when you save it: yours and the neighbours'. The computer is there when as many of them as you choose are in range again. Add places to a block or mode in its Automation. Needing more networks makes a place harder to recognise: refused while a block using it is locked. With no network seen, places count as whatever keeps their blocks enabled."),
            Ui.Button("Save where I am now as…", () => _ = AutomationEditor.SavePlace(window, null), "SavePlace", accent: true, glyph: Glyphs.Place),
            cards));
    }

    public void Update(GuiState s)
    {
        var key = System.Text.Json.JsonSerializer.Serialize(s.Places, Kc9kJson.Options) + System.Text.Json.JsonSerializer.Serialize(s.Modes.Select(m => m.Blocks));
        if (key == shown) return;
        shown = key;
        cards.Children.Clear();
        foreach (var p in s.Places) cards.Children.Add(Card(s, p));
        if (s.Places.Count == 0) cards.Children.Add(Ui.Dim("No places yet. Save where the computer is now: home, the library, work."));
    }

    UIElement Card(GuiState s, Place p)
    {
        var name = p.Name;
        var menu = new MenuFlyout();
        var more1 = new MenuFlyoutItem { Text = "Add the networks in range now" };
        more1.Click += (_, _) => _ = C.Run(["place", "add", name], timeout: TimeSpan.FromSeconds(60));
        var remove = new MenuFlyoutItem { Text = "Remove…" };
        remove.Click += async (_, _) =>
        {
            if (await C.Dialogs.Confirm($"Forget the place {name}?", "Blocks and modes using it lose it: refused while that weakens a locked block.", "Remove", destructive: true))
                await C.Run(["place", "remove", name]);
        };
        menu.Items.Add(more1);
        menu.Items.Add(remove);
        var more = new Button { Content = new FontIcon { Glyph = Glyphs.More, FontSize = 14 }, Flyout = menu }.Named($"More: {name}", "PlaceMore_" + name);
        var top = new Grid();
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var left = Ui.Row(10, Ui.Text(name, "SubtitleTextBlockStyle", wrap: false));
        if (p.Here) left.Children.Add(Ui.Pill(new("You are here", PillKind.On)));
        top.Children.Add(left);
        Grid.SetColumn(more, 1);
        top.Children.Add(more);
        var need = new NumberField(Math.Max(p.Need, 1), 1, Math.Max(p.Networks, 1), width: 70);
        need.Named($"{name}: networks needed", "PlaceNeed_" + name);
        var save = Ui.Button("Save", () => _ = C.Run(["place", "need", name, (need.Value ?? 1).ToString(System.Globalization.CultureInfo.InvariantCulture)]), "PlaceNeedSave_" + name);
        need.Changed += () => save.IsEnabled = need.IsValid;
        var modeBlocks = s.Modes.Where(m => p.Modes.Contains(m.Name)).SelectMany(m => m.Blocks).ToHashSet();
        var used = p.Blocks.Where(b => !modeBlocks.Contains(b)).Concat(p.Modes.Select(m => $"mode {m}")).ToList();
        return Ui.Card(Ui.Column(6, top,
            Ui.Wrap(8, Ui.Text("There when"), need, Ui.Text($"of its {p.Networks} networks are in range"), save),
            Ui.Dim(used.Count > 0 ? "Used by " + string.Join(", ", used) : "Not used yet: add it to a block or mode in its Automation.")));
    }
}
