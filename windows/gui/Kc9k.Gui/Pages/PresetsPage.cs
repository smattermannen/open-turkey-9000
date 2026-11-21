// §4.4 Presets (§3.2 Preset, owner 2026-11-17): saved copies by type, tidily
// grouped: use, rename, delete; list and phrase set presets edited here and
// copied into blocks; link and word recording (§3.12) into a list preset
// (gui/kc9k-gui.py build_clusters).
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class PresetsPage : IPage
{
    static readonly (string Kind, string Title, string Empty)[] Sections =
    [
        ("block", "Block presets", "Save one from a block's ⋯ menu: Save as block preset…"),
        ("lists", "Lists presets", "Save one from a block's Websites tab: Save lists as preset…"),
        ("automation", "Automation presets", "Save one from a block's or mode's Automation."),
        ("schedule", "Schedule presets", "Save one from a block's or mode's Automation, or a schedule."),
        ("hotkey", "Hotkey presets", "Save one on the Hotkeys page: Save as preset…."),
    ];

    readonly MainWindow window;
    readonly Dictionary<string, (StackPanel Rows, TextBlock Empty)> sections = [];
    readonly ComboBox clusterCombo = new ComboBox { MinWidth = 220 }.Named("List or phrase set preset", "ClusterChoice");
    readonly Dictionary<string, CheckBox> where = [];
    readonly EditableList entries;
    readonly TextBlock recordLabel = Ui.Text("");
    readonly Button recordStart, recordWords, recordStop, recordDiscard;
    readonly NumberField recordCut = new(0, 0, 95, width: 70);
    string presetsShown = "";
    string clustersShown = "";
    string? clusterWanted;
    bool filling;
    Controller C => window.Controller;
    public string Area => "blocks";
    public UIElement View { get; }
    string? Cluster => clusterCombo.SelectedItem as string;

    public PresetsPage(MainWindow window)
    {
        this.window = window;
        var page = Ui.Column(18, Ui.PageHeader("Presets", "Saved copies of blocks, lists, phrase sets, automations and schedules, to use again.",
            "A preset blocks nothing by itself and can always be changed. Using one copies it into a block or mode, and saving one copies too: changing either afterwards changes nothing on the other. Copying into a block follows that block's rules: a locked block refuses what would weaken it. Save presets from a block: its menu (the whole block), its Websites tab (its lists) and its Automation."));
        foreach (var (kind, title, empty) in Sections)
        {
            var rows = Ui.Column(6);
            var note = Ui.Dim(empty);
            page.Children.Add(Ui.Section(title, rows, note));
            sections[kind] = (rows, note);
        }
        clusterCombo.SelectionChanged += (_, _) => { if (!filling) ShowCluster(); };
        var more = new MenuFlyout();
        foreach (var (label, act) in new (string, Func<Task>)[] { ("Rename…", RenameCluster), ("Import…", ImportCluster), ("Export…", ExportCluster), ("Delete…", DeleteCluster) })
        {
            var i = new MenuFlyoutItem { Text = label };
            i.Click += (_, _) => _ = act();
            more.Items.Add(i);
        }
        var moreButton = new Button { Content = new FontIcon { Glyph = Glyphs.More, FontSize = 14 }, Flyout = more }.Named("Rename, delete, import, export", "ClusterMore");
        var wherePanel = Ui.Wrap(12, Ui.Text("Looks at:"));
        foreach (var (key, label) in Tables.ListPlaces.Append(Tables.AdPlace))
        {
            var check = new CheckBox { Content = label, MinWidth = 0 }.Named(label, "ClusterWhere_" + key);
            check.Click += (_, _) => { if (!filling && Cluster is { } n) _ = C.Run(["list-preset", "scope", n, key, check.IsChecked == true ? "on" : "off"]); };
            where[key] = check;
            wherePanel.Children.Add(check);
        }
        entries = new EditableList("Entries", "reddit.com  *casino*",
            values => Cluster is { } n ? C.Run(["list-preset", "add", n, .. values]) : Task.CompletedTask,
            (value, _) => Cluster is { } n ? C.Run(["list-preset", "remove", n, value]) : Task.CompletedTask,
            searchBox: true, multi: true, id: "ClusterEntries");
        page.Children.Add(Ui.Section("List and phrase set presets",
            Ui.Dim("One list or phrase set, edited here and copied into blocks."),
            Ui.Wrap(6, clusterCombo, Ui.Button("New…", () => _ = NewCluster(), "ClusterNew"),
                Ui.Button("Copy into a block…", () => _ = CopyCluster(), "ClusterCopy", accent: true), moreButton),
            wherePanel, entries));

        recordCut.Named("Cut the least specific words, percent", "RecordCut");
        recordStart = Ui.Button("Record into this preset", () => { if (Cluster is { } n) _ = C.Run(["record", "start", "--preset", n]); }, "RecordStart");
        recordWords = Ui.Button("Record words", () => { if (Cluster is { } n) _ = C.Run(["record", "start", "--preset", n, "--words"]); }, "RecordWords")
            .Tip("Record the words shown on the pages you open (the preset must look at page text)");
        recordStop = Ui.Button("Stop and add", () => _ = StopRecording(), "RecordStop");
        recordDiscard = Ui.Button("Stop and discard", () => _ = C.Run(["record", "stop", "--discard"]), "RecordDiscard");
        page.Children.Add(Ui.Section("Record the sites or words you open",
            Ui.Dim("While recording, every site opened in the browser (pages and the frames inside them, such as a login from another site) is noted. Stopping adds them to the list preset. Use it to collect what a task needs, then copy the preset into a block's exceptions. Recording words notes the words shown on each page instead, ranked by how much more common they are there than in your normal browsing; cut drops the least specific before adding."),
            recordLabel, Ui.Wrap(6, recordStart, recordWords, Ui.Text("cut %"), recordCut, recordStop, recordDiscard)));
        View = Ui.Page(page);
    }

    public void Update(GuiState s)
    {
        var key = string.Join("|", s.Templates) + System.Text.Json.JsonSerializer.Serialize(s.Presets, Kc9kJson.Options);
        if (key != presetsShown)
        {
            presetsShown = key;
            var items = new Dictionary<string, List<Preset>> { ["block"] = [.. s.Templates.Select(n => new Preset { Kind = "block", Name = n, What = "a whole block" })] };
            foreach (var p in s.Presets) (items.TryGetValue(p.Kind, out var l) ? l : items[p.Kind] = []).Add(p);
            foreach (var (kind, (rows, note)) in sections)
            {
                rows.Children.Clear();
                foreach (var p in items.GetValueOrDefault(kind) ?? []) rows.Children.Add(Row(p));
                note.Visibility = items.GetValueOrDefault(kind) is { Count: > 0 } ? Visibility.Collapsed : Visibility.Visible;
            }
        }
        var names = s.Clusters.Select(c => c.Name).ToList();
        if (string.Join("|", names) != clustersShown)
        {
            clustersShown = string.Join("|", names);
            var current = clusterWanted ?? Cluster;
            filling = true;
            clusterCombo.Items.Clear();
            foreach (var n in names) clusterCombo.Items.Add(n);
            if (names.Count > 0) clusterCombo.SelectedIndex = Math.Max(0, names.IndexOf(current ?? ""));
            filling = false;
            clusterWanted = null;
        }
        ShowCluster();
        var rec = s.Recording;
        if (rec is not null)
        {
            var target = rec.Cluster.Length > 0 ? $"the list preset {rec.Cluster}" : $"the list {rec.List} of {rec.Block}";
            recordLabel.Text = rec.Words
                ? $"Recording words into {target}: {rec.WordCount} words on {rec.Pages} pages so far." + (rec.Top.Count > 0 ? $"\nMost specific: {string.Join(", ", rec.Top)}" : "")
                : $"Recording into {target}: {rec.Sites} sites so far.";
        }
        else recordLabel.Text = "Not recording.";
        var shown = s.Clusters.FirstOrDefault(c => c.Name == Cluster);
        // R-PHRASE-10: phrases take no part in recording.
        recordStart.IsEnabled = rec is null && shown is { Phrases: false };
        recordWords.IsEnabled = rec is null && shown is { Phrases: false } && shown.Where.Contains("text");
        recordStop.IsEnabled = recordDiscard.IsEnabled = rec is not null;
    }

    UIElement Row(Preset p)
    {
        var g = new Grid { ColumnSpacing = 8 };
        g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var texts = Ui.Column(2, Ui.Strong(p.Name));
        if (p.What.Length > 0) texts.Children.Add(Ui.Dim(p.What));
        g.Children.Add(texts);
        var buttons = Ui.Row(6, Ui.Button(p.Kind == "block" ? "New block from it…" : "Use…", () => _ = Use(p), $"PresetUse_{p.Kind}_{p.Name}"));
        if (p.Kind != "block") buttons.Children.Add(Ui.Button("Rename…", () => _ = Rename(p), $"PresetRename_{p.Kind}_{p.Name}"));
        buttons.Children.Add(Ui.Button("Delete", () => _ = Delete(p), $"PresetDelete_{p.Kind}_{p.Name}"));
        Grid.SetColumn(buttons, 1);
        g.Children.Add(buttons);
        return Ui.Card(g);
    }

    /// <summary>Copies a preset: a block preset into a new block; lists into a
    /// block; an automation or windows into a block or mode; a hotkey again.</summary>
    async Task Use(Preset p)
    {
        if (p.Kind == "block")
        {
            var name = await C.Dialogs.AskText($"New block from {p.Name}", "A short name for the new block.", problem: t => t.Length == 0 ? "Type a name." : null, ok: "Create");
            if (!string.IsNullOrEmpty(name) && await C.Run(["block", "create", name, "--preset", p.Name])) window.OpenBlock(name);
            return;
        }
        if (p.Kind == "hotkey")
        {
            var keys = await C.Dialogs.AskText($"Add the hotkey {p.Name}", "Its keys, such as ctrl+alt+s (empty: the preset's own).",
                problem: t => t.Length == 0 ? null : Words.HotkeyProblem(t));
            if (keys is null) return;
            var locks = new[] { "locked", "with ", "Frozen Chicken", "monk mode" }.Any(p.What.Contains);
            if (locks && !(await C.Dialogs.ConfirmLock([$"a hotkey: {p.What}"])).Ok) return;
            List<string> args = ["preset", "use", p.Name, "--kind", "hotkey", "--yes"];
            if (keys.Trim().Length > 0) args.AddRange(["--keys", keys.Trim()]);
            await C.Run(args, then: o => C.Dialogs.Message(o, "Done"));
            return;
        }
        var s = C.State!;
        var blocks = s.RealBlocks.Select(b => b.Name).ToList();
        var choices = blocks.Select(b => ("b:" + b, b)).ToList();
        if (p.Kind != "lists") choices.AddRange(s.Modes.Where(m => !m.Freeze).Select(m => ("m:" + m.Name, $"mode {m.Name}")));
        if (choices.Count == 0) return;
        var target = await C.Dialogs.Choose($"Use {p.Name}", "It is copied into the one you choose; what would weaken a locked block is refused.", choices);
        if (target is null) return;
        List<string> use = ["preset", "use", p.Name, target[2..], "--kind", p.Kind];
        if (target.StartsWith("m:")) use.Add("--mode");
        await C.Run(use, then: o => C.Dialogs.Message(o, "Done"));
    }

    async Task Rename(Preset p)
    {
        var name = await C.Dialogs.AskText($"Rename {p.Name}", initial: p.Name);
        if (!string.IsNullOrEmpty(name) && name != p.Name) await C.Run(["preset", "rename", p.Name, name, "--kind", p.Kind]);
    }

    async Task Delete(Preset p)
    {
        if (await C.Dialogs.Confirm($"Delete the preset {p.Name}?", "What was copied from it stays as it is.", "Delete", destructive: true))
            await C.Run(p.Kind == "block" ? ["block-preset", "remove", p.Name] : ["preset", "delete", p.Name, "--kind", p.Kind]);
    }

    void ShowCluster()
    {
        var cluster = C.State?.Clusters.FirstOrDefault(c => c.Name == Cluster);
        filling = true;
        foreach (var (key, check) in where)
        {
            check.IsChecked = cluster?.Where.Contains(key) == true;
            check.IsEnabled = cluster is not null;
        }
        filling = false;
        entries.SetItems(cluster?.Entries ?? [], Cluster);
        // A phrase preset takes one phrase per line, as typed.
        var phrases = cluster?.Phrases == true;
        entries.Splitter = phrases ? t => t.Trim().Length > 0 ? [t.Trim()] : [] : Entries.Split;
        entries.Placeholder = phrases ? "[eat, nice] [hello, grandpa]" : "reddit.com  *casino*";
    }

    async Task NewCluster()
    {
        var entry = new TextBox().Named("Preset name", "ClusterName");
        var phrases = new CheckBox { Content = "Holds phrases (copied into blocks as a phrase set)" }.Named("Holds phrases", "ClusterPhrases");
        if (!await C.Dialogs.Ask("New list or phrase set preset", Ui.Column(10, Ui.Text("A name, such as study or shopping."), entry, phrases), "Create",
                () => entry.Text.Trim().Length > 0, w => entry.TextChanged += (_, _) => w())) return;
        clusterWanted = entry.Text.Trim();
        await C.Run(["list-preset", "create", clusterWanted, .. phrases.IsChecked == true ? new[] { "--phrases" } : []]);
    }

    async Task RenameCluster()
    {
        if (Cluster is not { } old) return;
        var name = await C.Dialogs.AskText($"Rename {old}", initial: old);
        if (string.IsNullOrEmpty(name) || name == old) return;
        clusterWanted = name;
        await C.Run(["list-preset", "rename", old, name]);
    }

    async Task DeleteCluster()
    {
        if (Cluster is { } name && await C.Dialogs.Confirm($"Delete the preset {name}?", "Lists copied from it into blocks stay as they are.", "Delete", destructive: true))
            await C.Run(["list-preset", "delete", name]);
    }

    async Task CopyCluster()
    {
        if (Cluster is not { } name || C.State is not { } s) return;
        var blocks = s.RealBlocks.Select(b => b.Name).ToList();
        if (blocks.Count == 0) return;
        var phrases = s.Clusters.FirstOrDefault(c => c.Name == name)?.Phrases == true;
        var combo = new ComboBox { MinWidth = 220 }.Named("Block", "ClusterCopyBlock");
        foreach (var b in blocks) combo.Items.Add(b);
        combo.SelectedIndex = 0;
        var exceptions = new CheckBox { Content = "As an exception list" }.Named("As an exception list", "ClusterCopyExceptions");
        var body = Ui.Column(10, Ui.Text(phrases
            ? "It becomes a new phrase set of the block, named after the preset, which makes the block stricter."
            : "It becomes a new list of the block, named after the preset. As a blacklist it makes the block stricter; as exceptions it weakens it, which a locked block refuses."), combo);
        if (!phrases) body.Children.Add(exceptions);
        if (!await C.Dialogs.Ask($"Copy {name} into a block", body, "Copy")) return;
        List<string> args = ["list-preset", "copy", name, (string)combo.SelectedItem];
        if (exceptions.IsChecked == true) args.Add("--exceptions");
        await C.Run(args);
    }

    async Task ImportCluster()
    {
        if (Cluster is not { } name) return;
        var path = await Files.Open(window, ".txt");
        if (path is not null) await C.Run(["list-preset", "import", name, path]);
    }

    async Task ExportCluster()
    {
        if (Cluster is { } name) await Files.Export(window, ["list-preset", "export", name], name);
    }

    async Task StopRecording()
    {
        if (!recordCut.IsValid)
        {
            await C.Dialogs.Message($"Cut: {recordCut.Problem}", "Not stopped");
            return;
        }
        await C.Run(["record", "stop", "--cut", (recordCut.Value ?? 0).ToString(CultureInfo.InvariantCulture)], then: o => C.Dialogs.Message(o, "Recorded"));
    }
}
