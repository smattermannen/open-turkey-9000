// What a block's apps or window titles do when found (owner 2026-11-16: any
// trigger to any action): their own action (close the app or the window)
// and the consequences, each with its time. Ticking runs `kc9k block
// trigger`, as does Enter in a time field while its action is on.
using System.Diagnostics;
using Kc9k.Core;
using TriggerAction = Kc9k.Core.TriggerAction;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed partial class TriggerEditor : StackPanel
{
    static readonly (string Key, string Label, long? Seconds)[] Consequences =
    [
        ("close-browser", "Close the browsers", null), ("lock-screen", "Lock the screen for", 30 * 60),
        ("logout", "Sign out", 0), ("shutdown", "Shut down", 0), ("monk", "Start monk mode for", 60 * 60),
        ("mode", "Run a mode, locked, for", 60 * 60), ("custom", "Custom action", null),
    ];

    readonly MainWindow window;
    readonly string group;
    readonly Dictionary<string, CheckBox> checks = [];
    readonly Dictionary<string, DurationField> times = [];
    readonly ComboBox modeCombo;
    readonly TextBlock customLabel = Ui.Note("");
    List<string> customItems = [];
    string? block;
    bool filling;

    public TriggerEditor(MainWindow window, string group, string own, string ownLabel, string intro)
    {
        this.window = window;
        this.group = group;
        Spacing = 4;
        Children.Add(Ui.Dim(intro));
        modeCombo = new ComboBox { MinWidth = 160 }.Named("Which mode", $"Trigger_{group}_Mode");
        modeCombo.SelectionChanged += (_, _) => { if (!filling) _ = TimeChanged("mode"); };
        foreach (var (key, label, seconds) in new[] { (own, ownLabel, (long?)null) }.Concat(Consequences))
        {
            var check = new CheckBox { Content = label, MinWidth = 200 }.Named(label, $"Trigger_{group}_{key}");
            check.Click += (_, _) => { if (!filling) _ = Toggled(key, check); };
            checks[key] = check;
            var row = Ui.Wrap(6, check);
            if (seconds is long s)
            {
                var once = key is "logout" or "shutdown";
                var field = new DurationField(new DurationRule
                {
                    Minimum = 60, Zero = once ? "once" : null, Maximum = key is "monk" or "mode" ? 30 * 86400 : 86400,
                }, s, width: 110);
                field.Named($"{label}: how long", $"Trigger_{group}_{key}_For");
                field.KeyDown += (_, e) => { if (e.Key == Windows.System.VirtualKey.Enter) _ = TimeChanged(key); };
                times[key] = field;
                row.Children.Add(field);
            }
            if (key == "mode") row.Children.Add(modeCombo);
            if (key == "custom")
            {
                row.Children.Add(Ui.Button("Change…", () => _ = ChangeCustom(), $"Trigger_{group}_CustomChange"));
                row.Children.Add(customLabel);
            }
            Children.Add(row);
        }
    }

    public void SetActions(string blockName, List<TriggerAction>? actions)
    {
        block = blockName;
        filling = true;
        try
        {
            var have = (actions ?? []).GroupBy(a => a.Name).ToDictionary(g => g.Key, g => g.First());
            foreach (var (name, check) in checks)
            {
                check.IsChecked = have.ContainsKey(name);
                if (times.TryGetValue(name, out var f) && have.TryGetValue(name, out var a) && f.FocusState == FocusState.Unfocused)
                    f.SetSeconds(a.Minutes * 60L);
            }
            var choices = window.Controller.ModeChoices();
            var chosen = have.GetValueOrDefault("mode")?.Param is { Length: > 0 } p ? p : modeCombo.SelectedItem as string;
            modeCombo.Items.Clear();
            foreach (var m in choices) modeCombo.Items.Add(m);
            if (choices.Count > 0) modeCombo.SelectedIndex = Math.Max(0, choices.IndexOf(chosen ?? ""));
            if (have.TryGetValue("custom", out var custom)) customItems = CustomEditor.FromJson(custom.Param);
            customLabel.Text = string.Join("; ", customItems);
        }
        finally { filling = false; }
    }

    async Task ChangeCustom()
    {
        var items = await Asks.Custom(window, "Custom action", customItems);
        if (items is null) return;
        customItems = items;
        customLabel.Text = string.Join("; ", items);
        if (block is not null && checks["custom"].IsChecked == true && await Command("custom", true) is { } args)
            await window.Controller.Run(args);
    }

    async Task<List<string>?> Command(string name, bool on)
    {
        var c = window.Controller;
        List<string> args = ["block", "trigger", block!, group, name, on ? "on" : "off"];
        if (on && times.TryGetValue(name, out var field))
        {
            if (!field.IsValid)
            {
                await c.Dialogs.Message($"{checks[name].Content}: {field.Problem}", "Not done");
                return null;
            }
            if (field.Seconds > 0) args.AddRange(["--for", field.Arg()]);
        }
        if (on && name == "mode")
        {
            if (modeCombo.SelectedItem is not string mode)
            {
                await c.Dialogs.Message("Make a mode first: a Mode action runs one.", "Not done");
                return null;
            }
            args.AddRange(["--target", mode]);
        }
        if (on && name == "custom")
        {
            if (customItems.Count == 0)
            {
                var items = await Asks.Custom(window, "Custom action", []);
                if (items is null || items.Count == 0) return null;
                customItems = items;
            }
            args.AddRange(customItems.SelectMany(i => new[] { "--custom", i }));
        }
        if (on && name is "monk" or "mode")
        {
            if (!await c.Dialogs.AcknowledgeNoConfirmation("This trigger")) return null;
            args.Add("--no-confirm-ok");
        }
        return args;
    }

    async Task Toggled(string name, CheckBox check)
    {
        if (block is null) return;
        var args = await Command(name, check.IsChecked == true);
        if (args is null)
        {
            filling = true;
            check.IsChecked = !(check.IsChecked == true);
            filling = false;
            return;
        }
        await window.Controller.Run(args);
    }

    async Task TimeChanged(string name)
    {
        if (block is not null && checks[name].IsChecked == true && await Command(name, true) is { } args)
            await window.Controller.Run(args);
    }
}

/// <summary>Picks apps to block from the installed apps (Start menu shortcuts)
/// and the running programs, with a search box. An app is blocked by its
/// program file's name, as `kc9k block add-app` takes it.</summary>
public static class AppPicker
{
    sealed record Found(string Name, string Entry, string From);

    static List<Found> Installed()
    {
        var output = new List<Found>();
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        var roots = new[]
        {
            Environment.GetFolderPath(Environment.SpecialFolder.CommonStartMenu),
            Environment.GetFolderPath(Environment.SpecialFolder.StartMenu),
        };
        dynamic? shell = null;
        try { shell = Activator.CreateInstance(Type.GetTypeFromProgID("WScript.Shell")!); }
        catch (Exception e) { Log.Write("WScript.Shell: " + e.Message); }
        foreach (var root in roots.Where(Directory.Exists))
        {
            IEnumerable<string> links;
            try { links = Directory.EnumerateFiles(root, "*.lnk", SearchOption.AllDirectories); }
            catch (Exception) { continue; }
            foreach (var link in links)
            {
                try
                {
                    string target = shell?.CreateShortcut(link).TargetPath ?? "";
                    if (!target.EndsWith(".exe", StringComparison.OrdinalIgnoreCase)) continue;
                    var exe = Path.GetFileName(target);
                    if (exe.Contains("unins", StringComparison.OrdinalIgnoreCase) || !seen.Add(exe)) continue;
                    output.Add(new(Path.GetFileNameWithoutExtension(link), exe, "Installed"));
                }
                catch (Exception) { } // one broken shortcut mustn't hide the rest
            }
        }
        return [.. output.OrderBy(f => f.Name, StringComparer.OrdinalIgnoreCase)];
    }

    static readonly HashSet<string> Skip = new(StringComparer.OrdinalIgnoreCase)
    {
        "svchost", "System", "Idle", "Registry", "smss", "csrss", "wininit", "winlogon", "services", "lsass", "dwm",
        "explorer", "conhost", "fontdrvhost", "RuntimeBroker", "sihost", "taskhostw", "ctfmon", "dllhost", "SearchHost",
        "StartMenuExperienceHost", "ShellExperienceHost", "TextInputHost", "kc9k", "kc9k-agent", "kc9k-gui", "MsMpEng",
        "spoolsv", "WmiPrvSE", "audiodg", "SecurityHealthService", "SecurityHealthSystray", "LockApp",
    };

    static List<Found> Running()
    {
        var names = new SortedSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var p in Process.GetProcesses())
        {
            try
            {
                if (p.SessionId == Process.GetCurrentProcess().SessionId && !Skip.Contains(p.ProcessName)) names.Add(p.ProcessName + ".exe");
            }
            catch (Exception) { }
            finally { p.Dispose(); }
        }
        return [.. names.Select(n => new Found(Path.GetFileNameWithoutExtension(n), n, "Running now"))];
    }

    public static async Task<List<string>> Pick(MainWindow window)
    {
        var all = await Task.Run(() =>
        {
            var installed = Installed();
            var known = installed.Select(f => f.Entry).ToHashSet(StringComparer.OrdinalIgnoreCase);
            return installed.Concat(Running().Where(r => !known.Contains(r.Entry))).ToList();
        });
        var chosen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        var search = new TextBox { PlaceholderText = "Search apps and running programs" }.Named("Search apps", "AppSearch");
        var list = new StackPanel { Spacing = 2 };
        void Fill()
        {
            list.Children.Clear();
            var words = search.Text.ToLowerInvariant().Split(' ', StringSplitOptions.RemoveEmptyEntries);
            foreach (var f in all.Where(f => words.All(w => $"{f.Name} {f.Entry}".ToLowerInvariant().Contains(w))).Take(300))
            {
                var box = new CheckBox { IsChecked = chosen.Contains(f.Entry), Content = Ui.Row(8, Ui.Text(f.Name, wrap: false), Ui.Note($"{f.Entry} · {f.From}")) }
                    .Named($"{f.Name} ({f.Entry})", "App_" + f.Entry);
                box.Click += (_, _) => { if (box.IsChecked == true) chosen.Add(f.Entry); else chosen.Remove(f.Entry); };
                list.Children.Add(box);
            }
            if (list.Children.Count == 0) list.Children.Add(Ui.Dim("Nothing found."));
        }
        search.TextChanged += (_, _) => Fill();
        Fill();
        var body = Ui.Column(8, search, new ScrollViewer { Content = list, Height = 380 });
        if (!await window.Controller.Dialogs.Ask("Choose apps to block", body, "Add", scroll: false, width: 640)) return [];
        return [.. chosen];
    }
}
