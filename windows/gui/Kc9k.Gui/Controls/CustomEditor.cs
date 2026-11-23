// §3.9 Custom action (owner 2026-11-17): what to open or run, one row each
// with Remove, and a row to add one. Items are "open …" / "run …" as the CLI
// takes them; Changed follows each change.
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed partial class CustomEditor : StackPanel
{
    readonly MainWindow window;
    readonly string id;
    readonly StackPanel rows = Ui.Column(4);
    readonly ComboBox kind;
    readonly TextBox entry;
    public List<string> Items { get; private set; } = [];
    public event Action<List<string>>? Changed;

    public CustomEditor(MainWindow window, IEnumerable<string>? items = null, string id = "Custom")
    {
        this.window = window;
        this.id = id;
        Spacing = 8;
        Children.Add(rows);
        kind = new ComboBox { MinWidth = 220 }.Named("Kind of action", id + "_Kind");
        kind.Items.Add(new ComboBoxItem { Content = "Open a site, file or folder", Tag = "open" });
        kind.Items.Add(new ComboBoxItem { Content = "Run a program or command", Tag = "run" });
        kind.SelectedIndex = 0;
        entry = Dialogs.KeepsEnter(new TextBox { PlaceholderText = @"https://… or C:\Users\me\Documents\notes.docx", MinWidth = 280 }.Named("What to open or run", id + "_Entry"));
        kind.SelectionChanged += (_, _) => entry.PlaceholderText = Kind == "open"
            ? @"https://… or C:\Users\me\Documents\notes.docx" : "notepad.exe C:\\notes.txt";
        entry.KeyDown += (_, e) =>
        {
            if (e.Key != Windows.System.VirtualKey.Enter) return;
            e.Handled = true; // added, not the dialog's OK
            AddItem();
        };
        Children.Add(Ui.Wrap(6, kind, entry,
            Ui.Button("Choose…", () => _ = Choose(), id + "_Choose").Tip("Choose a file to open"),
            Ui.Button("Add", () => AddItem(), id + "_Add")));
        SetItems(items ?? []);
    }

    string Kind => (kind.SelectedItem as ComboBoxItem)?.Tag as string ?? "open";

    public void SetItems(IEnumerable<string> items)
    {
        Items = [.. items];
        rows.Children.Clear();
        for (var n = 0; n < Items.Count; n++)
        {
            var number = n;
            var parts = Items[n].Split(' ', 2);
            var what = parts.Length > 1 ? parts[1] : "";
            var g = new Grid { ColumnSpacing = 8 };
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(48) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            g.Children.Add(Ui.Strong(parts[0] == "open" ? "Open" : "Run"));
            var t = Ui.Text(what, wrap: false);
            t.TextTrimming = TextTrimming.CharacterEllipsis;
            t.IsTextSelectionEnabled = true;
            Grid.SetColumn(t, 1);
            g.Children.Add(t);
            var remove = Ui.Button("Remove", () => RemoveItem(number), $"{id}_Remove{number + 1}");
            Grid.SetColumn(remove, 2);
            g.Children.Add(remove);
            rows.Children.Add(g);
        }
        rows.Visibility = Items.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
    }

    void AddItem(string? text = null)
    {
        var what = (text ?? entry.Text).Trim();
        if (what.Length == 0 || what.Contains('\n')) return;
        entry.Text = "";
        SetItems([.. Items, $"{Kind} {what}"]);
        Changed?.Invoke(Items);
    }

    void RemoveItem(int number)
    {
        SetItems(Items.Where((_, i) => i != number));
        Changed?.Invoke(Items);
    }

    async Task Choose()
    {
        var path = await Files.Open(window);
        if (path is null) return;
        kind.SelectedIndex = 0;
        AddItem(path);
    }

    /// <summary>The items as the JSON a list action or a step takes: [{"open": "…"}].</summary>
    public static string Json(IEnumerable<string> items) =>
        System.Text.Json.JsonSerializer.Serialize(items.Select(i =>
        {
            var p = i.Split(' ', 2);
            return new Dictionary<string, string> { [p[0]] = p.Length > 1 ? p[1] : "" };
        }).ToList());

    public static List<string> FromJson(string? json)
    {
        try
        {
            using var doc = System.Text.Json.JsonDocument.Parse(string.IsNullOrEmpty(json) ? "[]" : json);
            return [.. doc.RootElement.EnumerateArray().SelectMany(i => i.EnumerateObject().Take(1))
                .Select(p => $"{p.Name} {p.Value.GetString()}")];
        }
        catch (Exception) { return []; }
    }
}

/// <summary>Questions more than one page asks (the GTK GUI's ask_mode_run,
/// ask_custom).</summary>
public static class Asks
{
    /// <summary>§3.9 Mode action: which mode, and for how long it runs, locked.</summary>
    public static async Task<(string Mode, long Minutes)?> ModeRun(MainWindow window, string title, string? mode = null, long minutes = 60)
    {
        var choices = window.Controller.ModeChoices();
        var combo = new ComboBox { MinWidth = 240 }.Named("Mode", "ModeRunMode");
        foreach (var n in choices) combo.Items.Add(n);
        combo.SelectedIndex = choices.Count == 0 ? -1 : Math.Max(0, choices.IndexOf(mode ?? ""));
        var length = new DurationField(new DurationRule { Minimum = 60, Maximum = 30 * 86400 }, Math.Max(minutes, 1) * 60, [1800, 3600, 4 * 3600]);
        length.Named("How long it runs", "ModeRunLength");
        var body = Ui.Column(10,
            Ui.Text("The mode runs at once for this long, whatever its schedule and places say, and is locked until then: it can't be switched off or weakened. Then it is as it was."),
            combo, length);
        if (!await window.Controller.Dialogs.Ask(title, body, "OK", () => length.IsValid && combo.SelectedItem is not null,
                watch => length.Changed += watch)) return null;
        return combo.SelectedItem is string m && length.Seconds is long s ? (m, s / 60) : null;
    }

    /// <summary>§3.9 Custom action: what it opens or runs; null when cancelled.</summary>
    public static async Task<List<string>?> Custom(MainWindow window, string title, IEnumerable<string>? items = null)
    {
        var editor = new CustomEditor(window, items, "AskCustom");
        var body = Ui.Column(10,
            Ui.Text("Opened or run as you, in your session. It can do nothing you couldn't do yourself: a KC9K command in it meets every lock and password."),
            editor);
        if (!await window.Controller.Dialogs.Ask(title, body, "OK", () => editor.Items.Count > 0, watch => editor.Changed += _ => watch()))
            return null;
        return editor.Items;
    }
}
