// §4.3 one website list or phrase set of an opened block, on its own card
// (the GTK GUI's ListCard, PhraseSetCard, PhraseEditor): where it looks, what
// it does, its numbers for page text, its entries. Every change runs a kc9k
// command on the opened block, which the controller sends to its draft.
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public abstract partial class ActionCard : UserControl
{
    protected readonly MainWindow Window;
    protected readonly Func<string> Block;
    protected bool Filling;
    protected readonly TextBlock TitleText = Ui.Strong("");
    protected readonly TextBlock TitleNote = Ui.Note("");
    protected readonly Dictionary<string, CheckBox> PlaceChecks = [];
    protected readonly Dictionary<string, CheckBox> ActionChecks = [];
    protected readonly Dictionary<string, (Grid Row, TextBlock Label)> ParamRows = [];
    protected readonly CheckBox TestCheck;
    protected readonly TextBlock ActionsSummary = Ui.Text("");
    protected readonly StackPanel Body = Ui.Column(10);
    protected readonly string Id;

    protected abstract string ItemName { get; }
    protected abstract List<ListAction> Actions { get; }
    protected abstract string Kind { get; } // list, phrase-set
    protected abstract List<string> Command(params string[] args);

    protected ActionCard(MainWindow window, Func<string> block, string id, bool exceptions)
    {
        Window = window;
        Block = block;
        Id = id;
        Content = Ui.Card(Body);
        var top = new Grid { ColumnSpacing = 8 };
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        top.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        top.Children.Add(Ui.Row(10, TitleText, TitleNote));
        var buttons = Ui.Row(4, Ui.Button("Rename…", () => _ = Rename(), id + "_Rename"), Ui.Button("Delete…", () => _ = Delete(), id + "_Delete"));
        Grid.SetColumn(buttons, 1);
        top.Children.Add(buttons);
        Body.Children.Add(top);

        // §3.2: the scope boxes say where each entry is matched.
        var places = Ui.Wrap(14, Ui.Text("Looks at:"));
        IEnumerable<(string Key, string Title)> scopes = exceptions || Kind == "phrase-set" ? Tables.ListPlaces : [.. Tables.ListPlaces, Tables.AdPlace];
        foreach (var (key, title) in scopes)
        {
            var check = new CheckBox { Content = title, MinWidth = 0 }.Named($"{title}: where {(Kind == "list" ? "the list" : "the phrase set")} looks", $"{id}_Where_{key}");
            if (key == "ads") check.Tip("The ad networks a page loads: entries are ad network domains or adult, crypto, dating, gambling");
            check.Click += (_, _) => { if (!Filling) _ = Window.Controller.Run(Command("scope", key, check.IsChecked == true ? "on" : "off")); };
            PlaceChecks[key] = check;
            places.Children.Add(check);
        }
        Body.Children.Add(places);

        TestCheck = new CheckBox { Content = "Test mode: show what it finds in pages instead of acting" }.Named("Test mode", id + "_Test");
        TestCheck.Tip("Turning it on weakens the block.");
        TestCheck.Click += (_, _) => { if (!Filling) _ = Window.Controller.Run(Command("test", TestCheck.IsChecked == true ? "on" : "off")); };
        if (!exceptions)
        {
            Body.Children.Add(TestCheck);
            var actions = Ui.Wrap(12);
            foreach (var a in Tables.SiteActions)
            {
                var check = new CheckBox { Content = a.Title, MinWidth = 150 }.Named(a.Title, $"{id}_Action_{a.Key}");
                check.Click += (_, _) => { if (!Filling) _ = ToggleAction(a.Key, check.IsChecked == true); };
                ActionChecks[a.Key] = check;
                actions.Children.Add(check);
            }
            var expander = new Expander
            {
                Header = Ui.Row(8, Ui.Text("Does:"), ActionsSummary),
                Content = actions,
                HorizontalAlignment = HorizontalAlignment.Stretch,
                HorizontalContentAlignment = HorizontalAlignment.Stretch,
            }.Named("Change actions", id + "_Actions");
            Body.Children.Add(expander);
            foreach (var name in Tables.ParamActions)
            {
                var label = Ui.Text("", wrap: false);
                var row = new Grid { ColumnSpacing = 8, Visibility = Visibility.Collapsed };
                row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
                row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                row.Children.Add(label);
                var change = Ui.Button("Change…", () => _ = AskSettings(name), $"{id}_Param_{name}");
                Grid.SetColumn(change, 1);
                row.Children.Add(change);
                ParamRows[name] = (row, label);
                Body.Children.Add(row);
            }
        }
    }

    protected void FillCommon(List<string> where, bool test)
    {
        foreach (var (key, check) in PlaceChecks) check.IsChecked = where.Contains(key);
        TestCheck.IsChecked = test;
        var actions = Actions.ToDictionary(a => a.Action);
        foreach (var (key, check) in ActionChecks) check.IsChecked = actions.ContainsKey(key);
        var titles = Tables.SiteActions.Where(a => actions.ContainsKey(a.Key)).Select(a => a.Title).ToList();
        ActionsSummary.Text = titles.Count > 0 ? string.Join(", ", titles) : "Nothing yet: it only finds (tick an action under Does)";
        foreach (var (name, (row, label)) in ParamRows)
        {
            row.Visibility = actions.ContainsKey(name) ? Visibility.Visible : Visibility.Collapsed;
            if (actions.TryGetValue(name, out var a)) label.Text = SettingsText(name, a);
        }
    }

    public static string SettingsText(string name, ListAction a) => name switch
    {
        "redirect" => $"Opens {a.Param}",
        "image" => "Shows your picture",
        "monk" => $"Monk mode for {Words.Minutes(a.Minutes)}",
        "mode" => $"Runs {a.Param} for {Words.Minutes(a.Minutes)}, locked",
        "custom" => Words.CustomWords(a.Param) is { Length: > 0 } c ? c : "Custom action",
        _ => (name == "logout" ? "Logs out" : "Shuts down") + (a.Minutes > 0 ? $" for {Words.Minutes(a.Minutes)}" : " once"),
    };

    async Task ToggleAction(string name, bool on)
    {
        if (on && Tables.ParamActions.Contains(name))
        {
            await AskSettings(name);
            return;
        }
        await Window.Controller.Run(Command("action", name, on ? "on" : "off"));
    }

    /// <summary>Asks for an action's address, picture or time and turns it on
    /// with them (or changes them). Cancelling leaves it as it was.</summary>
    protected async Task AskSettings(string name)
    {
        var c = Window.Controller;
        var current = Actions.FirstOrDefault(a => a.Action == name) ?? new ListAction();
        List<string>? args = null;
        switch (name)
        {
            case "redirect":
                var url = await c.Dialogs.AskText("Open another page", "The address to open instead, starting with https://.",
                    initial: current.Param.Length > 0 ? current.Param : "https://",
                    problem: t => System.Text.RegularExpressions.Regex.IsMatch(t, @"^https?://[^\s/?#]+\S*$") ? null : "An address such as https://example.com.");
                if (!string.IsNullOrEmpty(url)) args = ["--url", url];
                break;
            case "image":
                var path = await Files.Open(Window, Files.Pictures);
                if (path is not null) args = ["--image", path];
                break;
            case "mode":
                var chosen = await Asks.ModeRun(Window, "Run a mode", current.Param, current.Minutes > 0 ? current.Minutes : 60);
                if (chosen is { } m && await c.Dialogs.AcknowledgeNoConfirmation("This action"))
                    args = ["--target", m.Mode, "--for", $"{m.Minutes}m", "--no-confirm-ok"];
                break;
            case "custom":
                var items = await Asks.Custom(Window, "Custom action", CustomEditor.FromJson(current.Param));
                if (items is not null) args = [.. items.SelectMany(i => new[] { "--custom", i })];
                break;
            default:
                var (title, detail, minimum, initial) = name switch
                {
                    "monk" => ("Monk mode", "The browser closes and every website is blocked, locked, and the internet connection is cut, for this long.", 1, 60),
                    "logout" => ("Log out", "The browser closes and you are logged out. With a time, you stay logged out that long; with 0, just once.", 0, 0),
                    _ => ("Shut down", "The browser closes and the computer shuts down. With a time, it stays off that long; with 0, just once.", 0, 0),
                };
                var rule = new DurationRule
                {
                    Minimum = Math.Max(minimum * 60, 60),
                    Maximum = name == "monk" ? 30 * 86400 : 86400,
                    Zero = name == "monk" ? null : "Just once",
                    WhyMax = name == "monk" ? "" : "It goes through Frozen Chicken, which lasts at most a week.",
                };
                var seconds = await c.Dialogs.AskDuration(title, detail, (current.Minutes > 0 ? current.Minutes : initial) * 60L, rule,
                    name == "monk" ? [3600, 86400] : [3600, 8 * 3600]);
                if (seconds is long s)
                {
                    args = s > 0 ? ["--for", $"{s / 60}m"] : [];
                    if (name == "monk")
                        args = await c.Dialogs.AcknowledgeNoConfirmation("This action") ? [.. args, "--no-confirm-ok"] : null;
                }
                break;
        }
        if (args is null)
        {
            Refill(); // untick the box again
            return;
        }
        await c.Run(Command(["action", name, "on", .. args]));
    }

    protected abstract void Refill();

    async Task Rename()
    {
        var name = await Window.Controller.Dialogs.AskText(Kind == "list" ? "Rename list" : "Rename phrase set", initial: ItemName);
        if (!string.IsNullOrEmpty(name) && name != ItemName) await Window.Controller.Run(Command("rename", name));
    }

    async Task Delete()
    {
        var what = Kind == "list" ? "list" : "phrase set";
        var detail = Kind == "list" ? "Its entries go with it. Deleting a blacklist weakens the block." : "Its phrases go with it. Deleting one with phrases weakens the block.";
        if (await Window.Controller.Dialogs.Confirm($"Delete the {what} {ItemName}?", detail, "Delete", destructive: true))
            await Window.Controller.Run(Command("delete"));
    }

    /// <summary>Runs each change in turn, one command at a time.</summary>
    protected async Task RunAll(IEnumerable<List<string>> commands)
    {
        foreach (var command in commands)
            if (!await Window.Controller.Run(command)) return;
    }
}

/// <summary>One website list (blacklist or exceptions).</summary>
public sealed partial class ListCard : ActionCard
{
    WebList data;
    readonly EditableList entries;
    readonly StackPanel thresholds;
    readonly Grid numbers = new() { ColumnSpacing = 8, RowSpacing = 4 };
    readonly Dictionary<string, (NumberBox Distinct, NumberBox Hits, NumberBox Rate)> spins = [];

    protected override string ItemName => data.Name;
    protected override List<ListAction> Actions => data.Actions;
    protected override string Kind => "list";
    public string ListName => data.Name;

    protected override List<string> Command(params string[] args) => ["list", args[0], Block(), data.Name, .. args[1..]];

    public ListCard(MainWindow window, Func<string> block, WebList list)
        : base(window, block, "List_" + list.Name.Replace(' ', '_'), list.IsExceptions)
    {
        data = list;
        thresholds = Ui.Column(6, Ui.Dim("What a page's text needs for each action: different words, and how many times they occur in all (both must be met). Rate: the share of a page's words (pages of 200 words or more); 0 for none."));
        numbers.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        for (var i = 0; i < 3; i++) numbers.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        thresholds.Children.Add(numbers);
        thresholds.Children.Add(Ui.Button("Save numbers", () => _ = SaveNumbers(), Id + "_SaveNumbers"));
        if (!list.IsExceptions) Body.Children.Add(thresholds);
        entries = new EditableList("", "reddit.com  *casino*  nsfw",
            values => Window.Controller.Run(Command(["add", .. values])),
            (value, _) => Window.Controller.Run(Command("remove", value)),
            () => Files.ImportIntoList(Window, Block(), data.Name),
            () => Files.Export(Window, ["list", "export", Block(), data.Name], data.Name),
            searchBox: true, hidden: true, multi: true, sizeKey: "website list", id: Id + "_Entries", showTitle: false);
        Body.Children.Add(entries);
        Update(list);
    }

    public void Update(WebList list)
    {
        data = list;
        Refill();
    }

    protected override void Refill()
    {
        Filling = true;
        try
        {
            TitleText.Text = data.Name;
            TitleNote.Text = $"{(data.IsExceptions ? "exceptions" : "blacklist")} · {Words.Plural(data.Entries.Count, "entry", "entries")}";
            FillCommon(data.Where, data.Test);
            var words = data.Where.Contains("title") || data.Where.Contains("text");
            TestCheck.Visibility = words ? Visibility.Visible : Visibility.Collapsed;
            thresholds.Visibility = data.Where.Contains("text") && !data.IsExceptions ? Visibility.Visible : Visibility.Collapsed;
            FillNumbers();
            entries.SetItems(data.Entries, Block());
        }
        finally { Filling = false; }
    }

    void FillNumbers()
    {
        var actions = data.Actions.Where(a => Tables.SiteActions.Any(s => s.Key == a.Action)).ToList();
        var key = string.Join("|", actions.Select(a => $"{a.Action}:{a.Distinct}:{a.Hits}:{a.Rate}"));
        if ((string?)numbers.Tag == key) return;
        numbers.Tag = key;
        numbers.Children.Clear();
        numbers.RowDefinitions.Clear();
        spins.Clear();
        numbers.RowDefinitions.Add(new RowDefinition());
        foreach (var (text, col) in new[] { ("Different", 1), ("In all", 2), ("Rate %", 3) })
        {
            var h = Ui.Note(text);
            Grid.SetColumn(h, col);
            numbers.Children.Add(h);
        }
        var row = 1;
        foreach (var a in actions)
        {
            numbers.RowDefinitions.Add(new RowDefinition());
            var title = Tables.SiteActions.First(s => s.Key == a.Action).Title;
            var label = Ui.Text(title);
            label.VerticalAlignment = VerticalAlignment.Center;
            Grid.SetRow(label, row);
            numbers.Children.Add(label);
            NumberBox Box(double value, double min, double max, double step, string what, int col)
            {
                var b = new NumberBox { Value = value, Minimum = min, Maximum = max, SmallChange = step, SpinButtonPlacementMode = NumberBoxSpinButtonPlacementMode.Compact, Width = 110 };
                b.Named($"{title}: {what}", $"{Id}_{what}_{a.Action}");
                Grid.SetRow(b, row);
                Grid.SetColumn(b, col);
                numbers.Children.Add(b);
                return b;
            }
            spins[a.Action] = (Box(Math.Max(a.Distinct, 1), 1, 1000, 1, "Different", 1), Box(Math.Max(a.Hits, 1), 1, 1000, 1, "In all", 2),
                Box(a.Rate / 10.0, 0, 100, 0.1, "Rate", 3));
            row++;
        }
    }

    async Task SaveNumbers()
    {
        var current = data.Actions.ToDictionary(a => a.Action);
        var changes = new List<List<string>>();
        foreach (var (name, (distinct, hits, rate)) in spins)
        {
            if (!current.TryGetValue(name, out var a)) continue;
            if (!double.IsNaN(distinct.Value) && (int)distinct.Value != Math.Max(a.Distinct, 1))
                changes.Add(Command("threshold", name, ((int)distinct.Value).ToString(CultureInfo.InvariantCulture)));
            if (!double.IsNaN(hits.Value) && (int)hits.Value != Math.Max(a.Hits, 1))
                changes.Add(Command("hits", name, ((int)hits.Value).ToString(CultureInfo.InvariantCulture)));
            var tenths = double.IsNaN(rate.Value) ? 0 : (int)Math.Round(rate.Value * 10);
            if (tenths != a.Rate)
                changes.Add(Command("rate", name, tenths > 0 ? (tenths / 10.0).ToString("0.#", CultureInfo.InvariantCulture) : "off"));
        }
        await RunAll(changes);
    }
}

/// <summary>One phrase set (§4.3): its phrases, edited in the phrase editor inside the card.</summary>
public sealed partial class PhraseSetCard : ActionCard
{
    PhraseSet data;
    readonly EditableList entries;
    readonly PhraseEditor editor;
    readonly TextBlock editorTitle = Ui.Strong("New phrase");
    readonly Button save, cancel, deleteButton;
    readonly Grid numbers = new() { ColumnSpacing = 8, RowSpacing = 4 };
    readonly StackPanel thresholds;
    readonly Dictionary<string, NumberBox> spins = [];
    Phrase? editing;
    string? savedAs;
    bool saving;

    protected override string ItemName => data.Name;
    protected override List<ListAction> Actions => data.Actions;
    protected override string Kind => "phrase-set";
    public string SetName => data.Name;

    protected override List<string> Command(params string[] args) => ["phrase-set", args[0], Block(), data.Name, .. args[1..]];

    public PhraseSetCard(MainWindow window, Func<string> block, PhraseSet set)
        : base(window, block, "Phrases_" + set.Name.Replace(' ', '_'), false)
    {
        data = set;
        numbers.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        numbers.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        thresholds = Ui.Column(6, Ui.Dim("Threshold: how many different phrases each action needs."), numbers,
            Ui.Button("Save thresholds", () => _ = SaveNumbers(), Id + "_SaveNumbers"));
        Body.Children.Add(thresholds);
        entries = new EditableList("", "", _ => Task.CompletedTask, (value, _) => Window.Controller.Run(Command("remove", value)),
            () => ImportInto(),
            () => Files.Export(Window, ["phrase-set", "export", Block(), data.Name], data.Name),
            searchBox: true, hidden: true, split: t => t.Trim().Length > 0 ? [t.Trim()] : [], entryBox: false, multi: true,
            sizeKey: "phrase set", id: Id + "_Phrases", showTitle: false);
        entries.Activated += text => _ = EditPhrase(text);
        Body.Children.Add(entries);
        Body.Children.Add(Ui.Note("Double-click a phrase (or press Enter on it) to load it into the editor below."));
        Body.Children.Add(editorTitle);
        editor = new PhraseEditor(Id);
        editor.Changed += EditorChanged;
        Body.Children.Add(editor);
        save = Ui.Button("Add phrase", () => _ = Commit(), Id + "_Save", accent: true);
        cancel = Ui.Button("Cancel", () => _ = EditPhrase(null), Id + "_Cancel").Tip("Stop editing; the saved phrases stay as they are");
        deleteButton = Ui.Button("Delete phrase", () => _ = RemoveLoaded(), Id + "_DeletePhrase");
        Body.Children.Add(Ui.Row(6, save, cancel, deleteButton));
        Update(set);
        editor.Load(null);
        ShowMode();
    }

    public void Update(PhraseSet set)
    {
        data = set;
        Refill();
    }

    protected override void Refill()
    {
        Filling = true;
        try
        {
            TitleText.Text = data.Name;
            TitleNote.Text = $"phrase set · {Words.Plural(data.Phrases.Count, "phrase")}";
            FillCommon(data.Where, data.Test);
            FillNumbers();
            entries.SetItems(data.Phrases.Select(p => p.Text), Block());
            // A saved change replaced the phrase being edited: keep editing
            // the new one, never fall back to adding a copy.
            saving = false;
            if (editing is null && savedAs is not null)
            {
                var added = data.Phrases.FirstOrDefault(p => PhraseText.Key(p.Text) == savedAs);
                if (added is not null)
                {
                    editing = added;
                    ShowMode();
                }
            }
            else if (editing is not null)
            {
                var texts = data.Phrases.GroupBy(p => PhraseText.Key(p.Text)).ToDictionary(g => g.Key, g => g.First());
                var current = (savedAs is not null ? texts.GetValueOrDefault(savedAs) : null) ?? texts.GetValueOrDefault(PhraseText.Key(editing.Text));
                if (current is null)
                {
                    editing = null;
                    savedAs = null;
                }
                else editing = current;
                ShowMode();
            }
            EditorChanged(editor.Text);
        }
        finally { Filling = false; }
    }

    void FillNumbers()
    {
        var actions = data.Actions.Where(a => Tables.SiteActions.Any(s => s.Key == a.Action)).ToList();
        var key = string.Join("|", actions.Select(a => $"{a.Action}:{a.Distinct}"));
        if ((string?)numbers.Tag == key) return;
        numbers.Tag = key;
        numbers.Children.Clear();
        numbers.RowDefinitions.Clear();
        spins.Clear();
        var row = 0;
        foreach (var a in actions)
        {
            numbers.RowDefinitions.Add(new RowDefinition());
            var title = Tables.SiteActions.First(s => s.Key == a.Action).Title;
            var label = Ui.Text(title);
            label.VerticalAlignment = VerticalAlignment.Center;
            Grid.SetRow(label, row);
            numbers.Children.Add(label);
            var b = new NumberBox { Value = Math.Max(a.Distinct, 1), Minimum = 1, Maximum = 1000, SpinButtonPlacementMode = NumberBoxSpinButtonPlacementMode.Compact, Width = 110 };
            b.Named($"{title}: different phrases", $"{Id}_Different_{a.Action}");
            Grid.SetRow(b, row);
            Grid.SetColumn(b, 1);
            numbers.Children.Add(b);
            spins[a.Action] = b;
            row++;
        }
        thresholds.Visibility = actions.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
    }

    async Task SaveNumbers()
    {
        var current = data.Actions.ToDictionary(a => a.Action);
        await RunAll(spins.Where(s => current.ContainsKey(s.Key) && !double.IsNaN(s.Value.Value) && (int)s.Value.Value != Math.Max(current[s.Key].Distinct, 1))
            .Select(s => Command("threshold", s.Key, ((int)s.Value.Value).ToString(CultureInfo.InvariantCulture))));
    }

    /// <summary>Whether the editor holds work that isn't saved.</summary>
    bool Unsaved() => editing is not null
        ? !string.Equals(editor.Text, editing.Text, StringComparison.OrdinalIgnoreCase)
        : editor.WordSets().Any(s => s.Terms.Count > 0);

    /// <summary>Loads a phrase into the editor to change it, or empties it for
    /// a new one (null). Unsaved work is dropped only after asking.</summary>
    async Task EditPhrase(string? text, bool ask = true)
    {
        var old = text is null ? null : data.Phrases.FirstOrDefault(p => p.Text == text);
        if (text is not null && old is null) return;
        if (ask && Unsaved() && !await Window.Controller.Dialogs.Confirm("Drop the unsaved phrase?",
                "The phrase in the editor has changes that aren't saved. They will be lost.", "Drop changes", destructive: true))
            return;
        editing = old;
        savedAs = null;
        ShowMode();
        editor.Load(old);
    }

    void ShowMode()
    {
        editorTitle.Text = editing is not null ? "Change phrase" : "New phrase";
        save.Content = editing is not null ? "Save changes" : "Add phrase";
        Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(save, (string)save.Content);
        deleteButton.Visibility = editing is not null ? Visibility.Visible : Visibility.Collapsed;
    }

    void EditorChanged(string text)
    {
        var same = editing is not null && PhraseText.Key(text) == PhraseText.Key(editing.Text);
        var known = editing is null && data.Phrases.Any(p => PhraseText.Key(p.Text) == PhraseText.Key(text));
        save.IsEnabled = text.Length > 0 && !same && !known && !saving;
        ToolTipService.SetToolTip(save, known ? "The set has this phrase already." : null);
    }

    /// <summary>Adds the editor's phrase, or changes the loaded one in one step
    /// (`phrase-set change`, R-WEAK-5). Saving stays off until the new state is read.</summary>
    async Task Commit()
    {
        var text = editor.Text;
        var old = editing;
        if (text.Length == 0 || saving || (old is not null && PhraseText.Key(text) == PhraseText.Key(old.Text))) return;
        saving = true;
        save.IsEnabled = false;
        savedAs = PhraseText.Key(text);
        var ok = old is null ? await Window.Controller.Run(Command("add", text)) : await Window.Controller.Run(Command("change", old.Text, text));
        // The command and its state read are done: Save works again even if
        // the state showed nothing new (savedAs still follows the phrase).
        saving = false;
        if (!ok) savedAs = null;
        EditorChanged(editor.Text);
    }

    async Task RemoveLoaded()
    {
        var old = editing;
        if (old is null || !await Window.Controller.Dialogs.Confirm("Delete this phrase?", old.Text + "\n\nDeleting a phrase weakens the block.", "Delete", destructive: true))
            return;
        if (await Window.Controller.Run(Command("remove", old.Text)))
            if (editing == old || (editing is null && PhraseText.Key(editor.Text) == PhraseText.Key(old.Text)))
                await EditPhrase(null, ask: false);
    }

    /// <summary>§4.3: one Import for a phrase set: KC9K reads the file's layout.
    /// Phrases go into the set; rendered terms become one word set in the
    /// editor; a list of sites can go to the block's Blocked list.</summary>
    async Task ImportInto()
    {
        var c = Window.Controller;
        var block = Block();
        var answer = await Files.AskSource(Window, "Import into this phrase set",
            "From a text file, one per line: phrases such as [eat, nice] [hello, grandpa] are added to the set; rendered terms become one word set in the phrase editor. Or from lists and list presets in Killer Chicken 9000: their entries become one word set.", more: false);
        if (answer == "kc9k")
        {
            var terms = await Files.PickTerms(Window);
            if (terms.Count == 0) return;
            editor.AddImported(terms);
            await c.Dialogs.Message($"{terms.Count} rendered terms are now a word set in the phrase editor of '{data.Name}'. Fill the other word set and click Add phrase.", "Imported");
            return;
        }
        if (answer != "file") return;
        var path = await Files.Open(Window, ".txt");
        if (path is null) return;
        var found = await Files.ReadImport(Window, path);
        if (found is null) return;
        if (found.Phrases.Count > 0) await Files.RunSteps(Window, [["phrase-set", "import", block, data.Name, path]]);
        else if (found.Terms.Count > 0)
        {
            editor.AddImported(PhraseText.WordSetTerms(found.Terms));
            var note = found.Sites.Count > 0 ? $" ({found.Sites.Count} sites in the file were left out)" : "";
            await c.Dialogs.Message($"{found.Terms.Count} rendered terms are now a word set in the phrase editor of '{data.Name}'{note}. Fill the other word set and click Add phrase.", "Imported");
        }
        else if (found.Sites.Count > 0)
        {
            if (await c.Dialogs.Confirm("The file lists sites", $"It lists {found.Sites.Count} sites, which belong in a list, not in a phrase set. Add them to the list 'Blocked' of '{block}'?", "Add to Blocked"))
                await Files.RunSteps(Window, [["list", "import", block, "Blocked", path]]);
        }
        else await c.Dialogs.Message("Nothing in that file can be imported: " + found.Describe() + ".", "Nothing to import");
    }
}

/// <summary>Builds a phrase (§4.3, inside its set's card, not a pop-up): its
/// word sets, each necessary or optional, and the phrase length.</summary>
public sealed partial class PhraseEditor : StackPanel
{
    public const int MaxWordSets = 100;
    readonly StackPanel rows = Ui.Column(6);
    readonly Button addSet;
    readonly NumberBox length;
    readonly TextBlock preview = Ui.Dim("");
    readonly string id;
    bool loading;
    int lastNecessary = 2;

    public event Action<string>? Changed;

    public PhraseEditor(string id)
    {
        this.id = id;
        Spacing = 8;
        Children.Add(Ui.Dim("A page counts when it has one term of each necessary word set, directly after each other in any order, in one sentence (no punctuation between). Separate a set's terms with commas; * matches inside a word (porn*). A term ending in - (big-) and one starting with - (-gorilla) in another set make a bridge: up to 20 words may stand between them. Any word set may be necessary or optional; the phrase length (at least 2) says how many terms a match needs."));
        Children.Add(rows);
        addSet = Ui.Button("Add word set", () => AddRow([], true), id + "_AddSet", glyph: Glyphs.Add);
        Children.Add(addSet);
        length = new NumberBox { Minimum = 2, Maximum = MaxWordSets, Value = 2, SpinButtonPlacementMode = NumberBoxSpinButtonPlacementMode.Inline, Width = 130 };
        length.Named("Phrase length (terms a match needs)", id + "_Length");
        length.ValueChanged += (_, _) => Change();
        Children.Add(Ui.Row(8, Ui.Text("Phrase length (terms a match needs):"), length));
        preview.IsTextSelectionEnabled = true;
        Children.Add(preview);
    }

    public void Load(Phrase? phrase)
    {
        rows.Children.Clear();
        var sets = phrase?.Sets is { Count: > 0 } s ? s : [new WordSet { Necessary = true }, new WordSet { Necessary = true }];
        loading = true;
        foreach (var ws in sets) AddRow(ws.Terms, ws.Necessary);
        loading = false;
        lastNecessary = Math.Max(Necessary(), 2);
        length.Maximum = Math.Max(sets.Count, 2);
        length.Value = phrase?.Length > 0 ? phrase.Length : lastNecessary;
        Change();
    }

    void AddRow(IEnumerable<string> terms, bool necessary)
    {
        var n = rows.Children.Count + 1;
        var entry = new TextBox { PlaceholderText = "eat, nice", Text = string.Join(", ", terms) }.Named($"Word set {n}", $"{id}_Set{n}");
        var check = new CheckBox { Content = "Necessary", IsChecked = necessary, MinWidth = 0 }.Named($"Word set {n} is necessary", $"{id}_Necessary{n}");
        var row = new Grid { ColumnSpacing = 8 };
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        row.Children.Add(entry);
        Grid.SetColumn(check, 1);
        row.Children.Add(check);
        var remove = Ui.IconButton(Glyphs.Delete, $"Remove word set {n}", () => { rows.Children.Remove(row); Change(); }, $"{id}_RemoveSet{n}");
        Grid.SetColumn(remove, 2);
        row.Children.Add(remove);
        row.Tag = (entry, check, remove);
        entry.TextChanged += (_, _) => Change();
        check.Click += (_, _) => Change();
        rows.Children.Add(row);
        Change();
    }

    /// <summary>Fills the first empty word set with terms, or adds a new one.</summary>
    public void AddImported(IReadOnlyList<string> terms)
    {
        if (terms.Count == 0) return;
        foreach (var r in rows.Children)
            if (r is Grid { Tag: (TextBox e, CheckBox, Button) } && e.Text.Trim().Length == 0)
            {
                e.Text = string.Join(", ", terms);
                return;
            }
        if (rows.Children.Count < MaxWordSets) AddRow(terms, true);
    }

    public List<(List<string> Terms, bool Necessary)> WordSets()
    {
        var output = new List<(List<string>, bool)>();
        foreach (var r in rows.Children)
        {
            if (r is not Grid { Tag: (TextBox e, CheckBox c, Button) }) continue;
            var terms = e.Text.Replace("[", " ").Replace("]", " ").Split(',')
                .Select(t => string.Join(" ", t.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries)))
                .Where(t => t.Trim('*', '-', ' ').Length > 0).ToList();
            output.Add((terms, c.IsChecked == true));
        }
        return output;
    }

    int Necessary() => WordSets().Count(s => s.Necessary);

    public string Text
    {
        get
        {
            var sets = WordSets();
            if (sets.Count < 2 || sets.Any(s => s.Terms.Count == 0)) return "";
            return PhraseText.Of(sets.Select(s => ((IReadOnlyList<string>)s.Terms, s.Necessary)), double.IsNaN(length.Value) ? 0 : (int)length.Value);
        }
    }

    void Change()
    {
        if (loading) return;
        var count = rows.Children.Count;
        foreach (var r in rows.Children)
            if (r is Grid { Tag: (TextBox, CheckBox, Button remove) }) remove.IsEnabled = count > 2;
        // The length follows the necessary word sets until it is set apart.
        int low = Math.Max(Necessary(), 2), high = Math.Max(count, 2);
        var follow = double.IsNaN(length.Value) || (int)length.Value == lastNecessary;
        lastNecessary = low;
        length.Minimum = low;
        length.Maximum = high;
        if (follow || length.Value < low) length.Value = low;
        if (length.Value > high) length.Value = high;
        addSet.IsEnabled = count < MaxWordSets;
        var text = Text;
        preview.Text = text.Length > 0 ? $"Phrase: {text}" : WordSets().Count < 2 ? "A phrase needs at least two word sets." : "Every word set needs at least one term.";
        Changed?.Invoke(text);
    }
}
