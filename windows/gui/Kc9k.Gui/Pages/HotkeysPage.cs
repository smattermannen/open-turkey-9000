// §4.13 (owner 2026-11-17): hotkeys on their own page: each with its keys,
// its hotkey actions in order (§3.9), Run without confirmation; add,
// remove, save as preset (gui/kc9k-gui.py build_hotkeys_page).
using System.Text.Json;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class HotkeysPage : IPage
{
    static readonly (string Key, string Title)[] Kinds =
    [
        ("start", "Start a block"), ("stop", "Switch a block off"), ("mode-on", "Switch a mode on"), ("mode-off", "Switch a mode off"),
        ("mode-run", "Run a mode, locked, for"), ("monk", "Monk mode for"), ("freeze", "Frozen Chicken for"),
        ("close-browser", "Close the browsers"), ("lock-screen", "Lock the screen"), ("logout", "Sign out"),
        ("shutdown", "Shut down"), ("custom", "Custom action"),
    ];

    readonly MainWindow window;
    readonly EditableList list;
    List<int> ids = [];
    Controller C => window.Controller;
    public string Area => "settings";
    public UIElement View { get; }

    public HotkeysPage(MainWindow window)
    {
        this.window = window;
        list = new EditableList("Hotkeys", "", _ => Task.CompletedTask,
            (_, number) => number - 1 < ids.Count ? C.Run(["hotkey", "remove", ids[number - 1].ToString(System.Globalization.CultureInfo.InvariantCulture)]) : Task.CompletedTask,
            entryBox: false, id: "Hotkeys", showTitle: false);
        View = Ui.Page(Ui.Column(18,
            Ui.PageHeader("Hotkeys", "Keys that do what you choose: start and lock blocks and modes, open your sites, files and programs, close the browsers, lock the screen…",
                "A hotkey runs its actions in order when its keys are pressed. One that locks something asks you to confirm at each press, unless you chose Run without confirmation when adding it (its locks are then confirmed only then). One that switches something off needs the settings password when it is added, and is refused at the press while that thing is locked."),
            Ui.Wrap(8,
                Ui.Button("Add hotkey…", () => _ = AddHotkey(), "AddHotkey", accent: true, glyph: Glyphs.Add).Tip("Keys and the actions they run"),
                Ui.Button("Start a block…", () => _ = AddStartHotkey(), "AddStartHotkey").Tip("Keys that start a block, with locks"),
                Ui.Button("Save as preset…", () => _ = SavePreset(), "HotkeyPreset").Tip("The selected hotkey, to add again")),
            list));
    }

    public void Update(GuiState s)
    {
        ids = [.. s.Hotkeys.Select(h => h.Id)];
        list.SetItems(s.Hotkeys.Select(h => $"{Words.HotkeyKeys(h.Keys)}  {h.Does}" + (h.NoConfirm ? " (runs without confirmation)" : "")));
    }

    async Task SavePreset()
    {
        var picked = list.SelectedEntry;
        var index = picked is null ? -1 : list.Items.ToList().IndexOf(picked);
        if (index < 0)
        {
            await C.Dialogs.Message("Pick a hotkey in the list first.", "Nothing picked");
            return;
        }
        var name = await C.Dialogs.AskText("Save as preset", "A name for a copy of this hotkey, to add again later.", problem: t => t.Length == 0 ? "Type a name." : null);
        if (!string.IsNullOrEmpty(name))
            await C.Run(["preset", "save", "hotkey", ids[index].ToString(System.Globalization.CultureInfo.InvariantCulture), name], then: o => C.Dialogs.Message(o, "Saved"));
    }

    static (TextBox Keys, TextBlock Note) KeysField()
    {
        var keys = new TextBox { PlaceholderText = "ctrl+alt+s", Width = 220 }.Named("Keys", "HotkeyKeys");
        var note = Ui.Note("ctrl, alt, shift or win with a key; F1 to F24 also work alone.");
        keys.TextChanged += (_, _) =>
        {
            var typed = keys.Text.Trim().Length > 0;
            var problem = typed ? Words.HotkeyProblem(keys.Text) : null;
            note.Text = problem ?? "ctrl, alt, shift or win with a key; F1 to F24 also work alone.";
            note.Style = Theme.Style(problem is null ? "Kc9kNote" : "Kc9kProblem");
        };
        return (keys, note);
    }

    sealed record Act(Dictionary<string, object> Json, string Words, string? Secret);

    /// <summary>One hotkey action (§3.9): its kind and what it needs.</summary>
    async Task<Act?> AskAction()
    {
        var s = C.State!;
        var blocks = s.RealBlocks.Select(b => b.Name).OrderBy(n => n).ToList();
        var modes = s.Modes.Where(m => !m.Monk && !m.Freeze).Select(m => m.Name).ToList();
        var kind = new ComboBox { MinWidth = 240 }.Named("What it does", "HotkeyActionKind");
        foreach (var (k, t) in Kinds) kind.Items.Add(new ComboBoxItem { Content = t, Tag = k });
        var target = new ComboBox { MinWidth = 240 }.Named("Which", "HotkeyActionTarget");
        var length = new DurationField(new DurationRule { Minimum = 60, Maximum = 30 * 86400, Zero = "no time (once)" }, 3600, width: 120);
        length.Named("How long", "HotkeyActionLength");
        var how = new ComboBox { MinWidth = 200 }.Named("How it freezes", "HotkeyActionFreeze");
        foreach (var (k, t) in Tables.FreezeModes) how.Items.Add(new ComboBoxItem { Content = t, Tag = k });
        how.SelectedIndex = 0;
        var custom = new CustomEditor(window, id: "HotkeyCustom");
        string K() => (kind.SelectedItem as ComboBoxItem)?.Tag as string ?? "start";
        void Shown()
        {
            var k = K();
            target.Items.Clear();
            var names = k is "start" or "stop" ? blocks : k is "mode-on" or "mode-off" or "mode-run" ? modes : [];
            foreach (var n in names) target.Items.Add(n);
            if (names.Count > 0) target.SelectedIndex = 0;
            target.Visibility = k is "start" or "stop" or "mode-on" or "mode-off" or "mode-run" ? Visibility.Visible : Visibility.Collapsed;
            var timed = k is "mode-run" or "monk" or "freeze" or "lock-screen" or "logout" or "shutdown";
            length.Visibility = timed ? Visibility.Visible : Visibility.Collapsed;
            length.SetLimits(60, k is "freeze" or "lock-screen" or "logout" or "shutdown" ? 7 * 86400 : 30 * 86400);
            how.Visibility = k == "freeze" ? Visibility.Visible : Visibility.Collapsed;
            custom.Visibility = k == "custom" ? Visibility.Visible : Visibility.Collapsed;
        }
        kind.SelectionChanged += (_, _) => Shown();
        kind.SelectedIndex = blocks.Count > 0 ? 0 : Kinds.Length - 1;
        Shown();
        if (!await C.Dialogs.Ask("Add an action", Ui.Column(10, kind, target, length, how, custom), "Add", width: 640)) return null;
        var key = K();
        var label = Kinds.First(x => x.Key == key).Title;
        var json = new Dictionary<string, object> { ["kind"] = key };
        string? secret = null;
        var words = label;
        if (key is "start" or "stop" or "mode-on" or "mode-off" or "mode-run")
        {
            if (target.SelectedItem is not string t)
            {
                await C.Dialogs.Message("There is nothing to choose: make a block or a mode first.", "Not added");
                return null;
            }
            json["target"] = t;
            words = key != "mode-run" ? $"{label} {t}" : $"Run {t}, locked, for";
        }
        if (key is "mode-run" or "monk" or "freeze" or "lock-screen" or "logout" or "shutdown")
        {
            if (!length.IsValid)
            {
                await C.Dialogs.Message($"How long: {length.Problem}", "Not added");
                return null;
            }
            var minutes = length.Minutes;
            if (minutes > 0)
            {
                json["minutes"] = minutes;
                words += words.EndsWith(" for") ? $" {Words.Minutes(minutes)}" : $" for {Words.Minutes(minutes)}";
            }
            else if (key is "mode-run" or "monk" or "freeze")
            {
                await C.Dialogs.Message("How long: at least a minute.", "Not added");
                return null;
            }
        }
        if (key == "freeze") json["param"] = (how.SelectedItem as ComboBoxItem)?.Tag as string ?? "lock";
        if (key == "custom")
        {
            if (custom.Items.Count == 0)
            {
                await C.Dialogs.Message("Add something to open or run first.", "Not added");
                return null;
            }
            json["param"] = CustomEditor.Json(custom.Items);
            words = string.Join("; ", custom.Items);
        }
        if (key == "start")
        {
            var t = (string)json["target"];
            var picker = new LockPicker("stop", "stopped", LockSpec.AllKinds, relockAny: true, id: "HotkeyLocks");
            var asked = await LockDialogs.Ask(window, $"Start {t}", "Locks each press sets (none: it just starts it). On a block that is on, a press adds its locks again (a kind it has: a second lock).", "OK", picker);
            if (asked is not var (chosen, relock, pw)) return null;
            if (chosen.Count > 1)
            {
                var ordered = await LockDialogs.ChooseOrder(window, chosen);
                if (ordered is null) return null;
                chosen = ordered;
            }
            List<string> flags = [.. chosen.SelectMany(c => c.Flags), .. relock];
            if (flags.Count > 0)
            {
                json["lock_args"] = flags;
                words += " with " + string.Join(", then ", chosen.Select(c => c.Words));
            }
            if (pw.Length > 0) secret = pw;
        }
        return new Act(json, words, secret);
    }

    /// <summary>A hotkey with any actions in order (§3.9 Hotkey action, §4.13).</summary>
    async Task AddHotkey()
    {
        if (C.State is null) return;
        var (keys, note) = KeysField();
        var actions = new List<Act>();
        var rows = Ui.Column(4);
        var unasked = new CheckBox { Content = "Run without confirmation (a press that locks then asks nothing)" }.Named("Run without confirmation", "HotkeyNoConfirm");
        System.Action? refresh = null;
        void Redraw()
        {
            rows.Children.Clear();
            for (var i = 0; i < actions.Count; i++)
            {
                var n = i;
                var g = new Grid { ColumnSpacing = 8 };
                g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
                g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                g.Children.Add(Ui.Text($"{i + 1}. {actions[i].Words}"));
                var rm = Ui.Button("Remove", () => { actions.RemoveAt(n); Redraw(); refresh?.Invoke(); }, $"HotkeyActionRemove{n + 1}");
                Grid.SetColumn(rm, 1);
                g.Children.Add(rm);
                rows.Children.Add(g);
            }
            if (actions.Count == 0) rows.Children.Add(Ui.Dim("No actions yet."));
        }
        Redraw();
        // The action dialog can't open over this one (one dialog at a time):
        // Add action… closes it, asks, and opens it again with what was typed.
        while (true)
        {
            var wantsAction = false;
            var body = Ui.Column(10, Ui.Heading("Keys"), keys, note, Ui.Heading("What a press does, in order"), rows,
                Ui.Button("Add action…", () => { wantsAction = true; C.Dialogs.Open?.Hide(); }, "HotkeyAddAction", glyph: Glyphs.Add), unasked);
            var ok = await C.Dialogs.Ask("Add a hotkey", body, "Add", () => Words.HotkeyProblem(keys.Text) is null && actions.Count > 0, w =>
            {
                refresh = w;
                keys.TextChanged += (_, _) => w();
            }, width: 680);
            if (wantsAction)
            {
                if (await AskAction() is { } got) actions.Add(got);
                Redraw();
                continue;
            }
            if (!ok) return;
            break;
        }
        var typed = keys.Text.Trim();
        List<string> args = ["hotkey", "add", typed];
        var locking = new List<string>();
        foreach (var a in actions)
        {
            args.AddRange(["--do-json", JsonSerializer.Serialize(a.Json)]);
            var k = (string)a.Json["kind"];
            if (a.Json.ContainsKey("lock_args") || k is "mode-run" or "monk" or "freeze" || (k is "lock-screen" or "logout" or "shutdown" && a.Json.ContainsKey("minutes")))
                locking.Add($"{a.Words}, each time {typed} is pressed");
        }
        var noConfirm = unasked.IsChecked == true;
        if (locking.Count > 0)
        {
            // R-LOCK-12: confirmed now (and at each press, unless Run without confirmation).
            var (confirmed, option) = await C.Dialogs.ConfirmLock(locking, noConfirm ? null : "Run without confirmation (pressing the hotkey then locks without asking again)");
            if (!confirmed) return;
            args.Add("--yes");
            if (noConfirm || option) args.Add("--no-confirm");
        }
        var secret = actions.Select(a => a.Secret).FirstOrDefault(x => x is not null);
        await C.Run(args, secret is null ? null : secret + "\n");
    }

    /// <summary>A hotkey that starts a block with locks (§3.13, R-GUI-2).</summary>
    async Task AddStartHotkey()
    {
        var blocks = (C.State?.RealBlocks ?? []).Select(b => b.Name).OrderBy(n => n).ToList();
        if (blocks.Count == 0)
        {
            await C.Dialogs.Message("Make a block first: a hotkey starts one.", "No blocks");
            return;
        }
        var (keys, note) = KeysField();
        var block = new ComboBox { MinWidth = 220 }.Named("Starts", "HotkeyBlock");
        foreach (var b in blocks) block.Items.Add(b);
        block.SelectedIndex = 0;
        var extra = Ui.Column(6, Ui.Strong("Keys"), keys, note, Ui.Strong("Starts"), block);
        var picker = new LockPicker("stop", "stopped", LockSpec.AllKinds, relockAny: true, id: "HotkeyStartLocks");
        var asked = await LockDialogs.Ask(window, "Add a hotkey",
            "Pressing the keys starts the block with these locks (none: it just starts). On a block that is already on, a press adds its locks again (a kind it has: a second lock).",
            "Add", picker, extra, () => Words.HotkeyProblem(keys.Text), w => keys.TextChanged += (_, _) => w());
        if (asked is not var (chosen, relock, secret)) return;
        if (chosen.Count > 1)
        {
            var ordered = await LockDialogs.ChooseOrder(window, chosen);
            if (ordered is null) return;
            chosen = ordered;
        }
        var typed = keys.Text.Trim();
        var name = (string)block.SelectedItem;
        List<string> args = ["hotkey", "add", typed, name];
        var lines = new List<string>();
        for (var i = 0; i < chosen.Count; i++)
        {
            args.AddRange(chosen[i].Flags);
            lines.Add($"block '{name}': {(chosen.Count > 1 ? $"{i + 1} of {chosen.Count}: " : "")}{chosen[i].Words}, set each time {typed} is pressed");
        }
        args.AddRange(relock);
        lines.AddRange(LockSpec.RelockLines($"block '{name}'", relock));
        if (lines.Count > 0)
        {
            var (ok, option) = await C.Dialogs.ConfirmLock(lines, "Run without confirmation (pressing the hotkey then locks without asking again)");
            if (!ok) return;
            args.Add("--yes");
            if (option) args.Add("--no-confirm");
        }
        await C.Run(args, secret.Length > 0 ? secret + "\n" : null);
    }
}
