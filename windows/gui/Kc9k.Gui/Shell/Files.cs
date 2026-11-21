// Files: Windows' own open and save dialogs, and the imports and exports of
// lists (§4.3 Import From KC9K / From file / Whole internet / All searches
// containing…; Export). Reading a file to import is the CLI's (`kc9k list
// read`): it says what the file holds, and the GUI only routes it.
using Kc9k.Core;
using Microsoft.UI.Xaml.Controls;
using Windows.Storage.Pickers;

namespace Kc9k.Gui;

public static class Files
{
    static nint Hwnd(MainWindow w) => WinRT.Interop.WindowNative.GetWindowHandle(w);

    /// <summary>A file to open; null when cancelled. Kinds like ".txt"; none: any file.</summary>
    public static async Task<string?> Open(MainWindow window, params string[] kinds)
    {
        var picker = new FileOpenPicker { SuggestedStartLocation = PickerLocationId.DocumentsLibrary };
        WinRT.Interop.InitializeWithWindow.Initialize(picker, Hwnd(window));
        if (kinds.Length == 0) picker.FileTypeFilter.Add("*");
        foreach (var k in kinds) picker.FileTypeFilter.Add(k);
        try { return (await picker.PickSingleFileAsync())?.Path; }
        catch (Exception e) { Log.Write("open picker: " + e.Message); return null; }
    }

    public static async Task<string?> Folder(MainWindow window)
    {
        var picker = new FolderPicker { SuggestedStartLocation = PickerLocationId.ComputerFolder };
        WinRT.Interop.InitializeWithWindow.Initialize(picker, Hwnd(window));
        picker.FileTypeFilter.Add("*");
        try { return (await picker.PickSingleFolderAsync())?.Path; }
        catch (Exception e) { Log.Write("folder picker: " + e.Message); return null; }
    }

    /// <summary>Where to save; null when cancelled.</summary>
    public static async Task<string?> Save(MainWindow window, string name, string kind = ".txt", string what = "Text file")
    {
        var picker = new FileSavePicker { SuggestedStartLocation = PickerLocationId.DocumentsLibrary, SuggestedFileName = name };
        WinRT.Interop.InitializeWithWindow.Initialize(picker, Hwnd(window));
        picker.FileTypeChoices.Add(what, [kind]);
        try { return (await picker.PickSaveFileAsync())?.Path; }
        catch (Exception e) { Log.Write("save picker: " + e.Message); return null; }
    }

    public static readonly string[] Pictures = [".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp"];

    /// <summary>Saves what an export command prints to a file.</summary>
    public static async Task Export(MainWindow window, IReadOnlyList<string> command, string name)
    {
        var path = await Save(window, name + ".txt");
        if (path is null) return;
        var output = await window.Controller.Read(command);
        if (output is null) return;
        try { await File.WriteAllTextAsync(path, output.Length > 0 ? output + "\n" : ""); }
        catch (Exception e) { await window.Controller.Dialogs.Message(e.Message); }
    }

    /// <summary>What a file to import holds, as KC9K reads it; null after saying why it can't.</summary>
    public static async Task<ImportFound?> ReadImport(MainWindow window, string path)
    {
        var output = await window.Controller.Read(["list", "read", path]);
        if (output is null) return null;
        try { return ImportFound.Parse(output); }
        catch (System.Text.Json.JsonException)
        {
            await window.Controller.Dialogs.Message(output);
            return null;
        }
    }

    /// <summary>Runs commands one after another; then says what they did, once.</summary>
    public static async Task RunSteps(MainWindow window, List<List<string>> steps, bool routedPhrases = false)
    {
        var said = new List<string>();
        foreach (var step in steps)
        {
            var ok = await window.Controller.Run(step, then: output =>
            {
                said.AddRange(output.Split('\n').Where(l => l.Trim().Length > 0 &&
                    !(routedPhrases && l.StartsWith("Left out") && l.Contains(" phrase"))));
                return Task.CompletedTask;
            });
            if (!ok) return;
        }
        if (said.Count > 0) await window.Controller.Dialogs.Message(string.Join("\n", said), "Done");
    }

    /// <summary>From a file or from KC9K: "file", "kc9k", "internet", "searches" or null.</summary>
    public static async Task<string?> AskSource(MainWindow window, string title, string detail, bool more)
    {
        string? chosen = null;
        var body = Ui.Column(10, Ui.Text(detail));
        var buttons = Ui.Wrap(8);
        var dialog = new ContentDialog { Title = title, Content = body, CloseButtonText = "Cancel" };
        void Add(string label, string key, bool accent = false)
        {
            buttons.Children.Add(Ui.Button(label, () => { chosen = key; dialog.Hide(); }, "Import_" + key, accent));
        }
        Add("From file…", "file", accent: true);
        Add("From KC9K…", "kc9k");
        if (more)
        {
            Add("Whole internet", "internet");
            Add("All searches containing…", "searches");
        }
        body.Children.Add(buttons);
        await window.Controller.Dialogs.Show(dialog);
        return chosen;
    }

    /// <summary>Ticks among labelled choices; the chosen values, or null when cancelled.</summary>
    public static async Task<List<T>?> Tick<T>(MainWindow window, string title, string detail, IReadOnlyList<(string Label, T Value)> choices, string ok = "Import")
    {
        var body = Ui.Column(6);
        if (detail.Length > 0) body.Children.Add(Ui.Text(detail));
        var checks = choices.Select(c => (Box: new CheckBox { Content = c.Label }.Named(c.Label), c.Value)).ToList();
        foreach (var (box, _) in checks) body.Children.Add(box);
        if (!await window.Controller.Dialogs.Ask(title, body, ok)) return null;
        return [.. checks.Where(c => c.Box.IsChecked == true).Select(c => c.Value)];
    }

    /// <summary>§4.3: Import into a list: from a file (phrases in it go to a
    /// phrase set), from other lists and list presets, the whole internet, or
    /// all searches containing some words.</summary>
    public static async Task ImportIntoList(MainWindow window, string block, string list)
    {
        var c = window.Controller;
        var answer = await AskSource(window, "Import into this list",
            "From a text file with one entry per line (a hosts file and adblock rules are read too), or from other lists and list presets in Killer Chicken 9000; or the whole internet, or all searches containing some words.",
            more: true);
        switch (answer)
        {
            case "kc9k":
            {
                var sources = new List<(string, List<string>)>();
                foreach (var other in c.State?.Blocks ?? [])
                {
                    if (other.Hidden) continue;
                    foreach (var l in other.Lists)
                        if ((other.Name, l.Name) != (block, list) && l.Entries.Count > 0)
                            sources.Add(($"{other.Name}: {l.Name}", ["list", "copy", other.Name, l.Name, block, list]));
                }
                foreach (var cl in c.State?.Clusters ?? [])
                    if (!cl.Phrases) sources.Add(($"List preset: {cl.Name}", ["list-preset", "copy", cl.Name, block, "--into", list]));
                if (sources.Count == 0)
                {
                    await c.Dialogs.Message("No other lists or list presets to import from.", "Nothing to import");
                    return;
                }
                var chosen = await Tick(window, "Import from Killer Chicken 9000", "", sources);
                foreach (var command in chosen ?? [])
                    if (!await c.Run(command)) return;
                return;
            }
            case "internet":
                if (await c.Dialogs.Confirm($"Add the whole internet to {list}?",
                        "It gets *.*: every site is matched (local files, localhost and the local network stay reachable). In a blacklist, only the block's exceptions are let through.", "Add"))
                    await c.Run(["list", "add", block, list, "*.*"]);
                return;
            case "searches":
            {
                var text = await c.Dialogs.AskText("All searches containing…",
                    "Words, separated by commas. A search for one of them is matched by its address (it holds the words searched for), and so is any other address with that word in it.",
                    problem: t => t.Split(',').Any(w => w.Trim().Length > 0) ? null : "Type at least one word.");
                var words = (text ?? "").Split(',').Select(w => w.Trim()).Where(w => w.Length > 0).ToList();
                if (words.Count == 0) return;
                var where = c.State?.FindBlock(block)?.Lists.FirstOrDefault(l => l.Name == list)?.Where ?? [];
                List<List<string>> steps = [["list", "add", block, list, .. words]];
                if (!where.Contains("url")) steps.Add(["list", "scope", block, list, "url", "on"]);
                await RunSteps(window, steps);
                return;
            }
            case "file":
            {
                var path = await Open(window, ".txt", ".csv", ".hosts", ".list");
                if (path is null) return;
                var found = await ReadImport(window, path);
                if (found is null) return;
                var steps = new List<List<string>>();
                if (found.Phrases.Count > 0 && await ChoosePhraseSet(window, block, found.Phrases.Count) is { } target)
                {
                    var (name, isNew) = target;
                    if (isNew) steps.Add(["phrase-set", "create", block, name]);
                    steps.Add(["phrase-set", "import", block, name, path]);
                }
                if (found.Entries.Count > 0) steps.Add(["list", "import", block, list, path]);
                if (steps.Count == 0)
                {
                    if (found.Phrases.Count == 0)
                        await c.Dialogs.Message("Nothing in that file can go in a list: " + found.Describe() + ".", "Nothing to import");
                    return;
                }
                await RunSteps(window, steps, routedPhrases: found.Phrases.Count > 0);
                return;
            }
        }
    }

    /// <summary>Where the phrases of an imported file go: one of the block's
    /// phrase sets or a new one, as (name, new); null leaves them out.</summary>
    static async Task<(string, bool)?> ChoosePhraseSet(MainWindow window, string block, int count)
    {
        var names = window.Controller.State?.FindBlock(block)?.PhraseSets.Select(p => p.Name).ToList() ?? [];
        var combo = new ComboBox { MinWidth = 240 }.Named("Phrase set", "PhraseTarget");
        foreach (var n in names) combo.Items.Add(n);
        combo.Items.Add("A new phrase set:");
        combo.SelectedIndex = 0;
        var newName = new TextBox { Text = "Imported phrases", IsEnabled = names.Count == 0 }.Named("New phrase set's name");
        combo.SelectionChanged += (_, _) => newName.IsEnabled = combo.SelectedIndex == names.Count;
        var body = Ui.Column(8, Ui.Text($"{(count == 1 ? "1 line is a phrase" : $"{count} lines are phrases")} ([a, b] [c]): a list can't hold phrases. Add them to a phrase set of '{block}':"),
            combo, newName);
        var d = new ContentDialog { Title = "Phrases in the file", Content = body, PrimaryButtonText = "Add them", CloseButtonText = "Leave them out" };
        if (await window.Controller.Dialogs.Show(d) != ContentDialogResult.Primary) return null;
        if (combo.SelectedIndex < names.Count) return (names[combo.SelectedIndex], false);
        var typed = newName.Text.Trim();
        return typed.Length > 0 ? (typed, !names.Any(n => string.Equals(n, typed, StringComparison.OrdinalIgnoreCase))) : null;
    }

    /// <summary>Entries of lists and list presets in KC9K, ticked, for one word set.</summary>
    public static async Task<List<string>> PickTerms(MainWindow window)
    {
        var state = window.Controller.State;
        var sources = new List<(string, List<string>)>();
        foreach (var other in state?.Blocks ?? [])
        {
            if (other.Hidden) continue;
            foreach (var l in other.Lists)
                if (l.Entries.Count > 0) sources.Add(($"{other.Name}: {l.Name}", l.Entries));
        }
        foreach (var cl in state?.Clusters ?? [])
            if (!cl.Phrases && cl.Entries.Count > 0) sources.Add(($"List preset: {cl.Name}", cl.Entries));
        if (sources.Count == 0)
        {
            await window.Controller.Dialogs.Message("No lists or list presets to import from.", "Nothing to import");
            return [];
        }
        var chosen = await Tick(window, "Import a word set from Killer Chicken 9000", "", sources);
        return chosen is null ? [] : PhraseText.WordSetTerms(chosen.SelectMany(e => e));
    }
}
