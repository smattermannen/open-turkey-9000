// Bypass prevention (§3.10): its whole ladder in one dialog, as the GTK
// GUI's ask_nepsis: steps of "N things stopped within a time: these
// consequences", what counts, the quiet time, and (globally) a timer lock.
// It gives the arguments of `kc9k bypass set`; each step goes as JSON
// (--step-json), since what a custom action runs may hold any character.
using System.Text.Json;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public static class NepsisDialog
{
    sealed class StepCard
    {
        public required NumberField Count;
        public required DurationField Within;
        public required Dictionary<string, (CheckBox Check, DurationField? Minutes, Func<string>? Param)> Actions;
        public required Border View;
    }

    /// <summary>The 'bypass set' arguments, and the timer lock's words; null when cancelled.</summary>
    public static async Task<(List<string> Args, string LockWords)?> Ask(MainWindow window, string title, Nepsis? current, bool allowLock)
    {
        var c = window.Controller;
        current ??= new Nepsis();
        var steps = current.Steps.Count > 0 ? current.Steps
            : [new NepsisStep { Count = 10, Minutes = 5, Actions = [new NepsisAction { Action = "warning" }] }];
        var body = Ui.Column(12, Ui.Dim("A ladder of steps. The first time its number of things is stopped within its time, step 1 acts; the next time, step 2; and so on, staying on the last. Making it stricter is always allowed; anything else weakens it."));
        var stepsBox = Ui.Column(10);
        body.Children.Add(stepsBox);
        var cards = new List<StepCard>();
        var fields = new List<Func<bool>>();
        Action? refresh = null;
        void Changed() => refresh?.Invoke();

        void AddCard(NepsisStep step)
        {
            var n = cards.Count + 1;
            var count = new NumberField(step.Count > 0 ? step.Count : 3, 1, Tables.NepsisMaxCount, width: 80);
            count.Named($"Step {n}: how many things stopped", $"Step{n}_Count");
            var within = new DurationField(new DurationRule { Minimum = 60, Maximum = 86400 }, (step.Minutes > 0 ? step.Minutes : 10) * 60L);
            within.Named($"Step {n}: within how long", $"Step{n}_Within");
            count.Changed += Changed;
            within.Changed += Changed;
            var grid = Ui.Column(6, Ui.Strong($"Step {n}"), Ui.Wrap(8, count, Ui.Text("things stopped within"), within));
            var chosen = step.Actions.ToDictionary(a => a.Action);
            var actions = new Dictionary<string, (CheckBox, DurationField?, Func<string>?)>();
            foreach (var (key, label) in Tables.Consequences)
            {
                var check = new CheckBox { Content = label, IsChecked = chosen.ContainsKey(key), MinWidth = 220 }.Named($"Step {n}: {label}", $"Step{n}_{key}");
                check.Click += (_, _) => Changed();
                var row = Ui.Wrap(8, check);
                DurationField? minutes = null;
                Func<string>? param = null;
                if (Tables.Timed.TryGetValue(key, out var needsTime))
                {
                    var have = chosen.GetValueOrDefault(key)?.Minutes ?? 0;
                    minutes = new DurationField(new DurationRule
                    {
                        Minimum = 60, Maximum = key is "monk" or "mode" ? 30 * 86400 : 86400, Zero = needsTime ? null : "Just once",
                    }, (have > 0 ? have : needsTime ? 30 : 0) * 60L, width: 110);
                    minutes.Named($"Step {n}: {label}: how long", $"Step{n}_{key}_For");
                    minutes.Changed += Changed;
                    row.Children.Add(minutes);
                }
                if (key == "mode")
                {
                    var combo = new ComboBox { MinWidth = 180 }.Named($"Step {n}: which mode", $"Step{n}_Mode");
                    var names = c.ModeChoices();
                    foreach (var m in names) combo.Items.Add(m);
                    var wanted = chosen.GetValueOrDefault("mode")?.Param;
                    if (names.Count > 0) combo.SelectedIndex = Math.Max(0, names.IndexOf(wanted ?? ""));
                    row.Children.Add(combo);
                    param = () => combo.SelectedItem as string ?? "";
                }
                if (key == "custom")
                {
                    var items = CustomEditor.FromJson(chosen.GetValueOrDefault("custom")?.Param);
                    var summary = Ui.Note(string.Join("; ", items));
                    row.Children.Add(Ui.Button("Change…", async () =>
                    {
                        var got = await Asks.Custom(window, "Custom action", items);
                        if (got is null) return;
                        items = got;
                        summary.Text = string.Join("; ", got);
                        check.IsChecked = true;
                        Changed();
                    }, $"Step{n}_CustomChange"));
                    row.Children.Add(summary);
                    param = () => CustomEditor.Json(items);
                }
                actions[key] = (check, minutes, param);
                grid.Children.Add(row);
            }
            var card = Ui.Card(grid);
            stepsBox.Children.Add(card);
            cards.Add(new StepCard { Count = count, Within = within, Actions = actions, View = card });
            Changed();
        }

        foreach (var s in steps) AddCard(s);
        body.Children.Add(Ui.Row(6,
            Ui.Button("Add a step", () =>
            {
                if (cards.Count < 10) AddCard(new NepsisStep { Count = 3, Minutes = 10, Actions = [new NepsisAction { Action = "close-browser" }] });
            }, "AddStep", glyph: Glyphs.Add),
            Ui.Button("Remove the last step", () =>
            {
                if (cards.Count <= 1) return;
                stepsBox.Children.Remove(cards[^1].View);
                cards.RemoveAt(cards.Count - 1);
                Changed();
            }, "RemoveStep")));

        var quiet = new DurationField(new DurationRule { Minimum = 60, Maximum = 30 * 86400, Zero = "Never falls back" }, current.QuietMinutes * 60L, width: 120);
        quiet.Named("Fall back a step after quiet time of", "Quiet");
        quiet.Changed += Changed;
        body.Children.Add(Ui.Wrap(8, Ui.Text("Fall back a step after quiet time of"), quiet));

        body.Children.Add(Ui.Heading("What counts"));
        var detectors = new Dictionary<string, CheckBox>();
        foreach (var (key, label) in Tables.Detectors)
        {
            var check = new CheckBox { Content = label, IsChecked = current.Detectors.Count == 0 || current.Detectors.Contains(key) }.Named(label, "Detect_" + key);
            detectors[key] = check;
            body.Children.Add(check);
        }
        LockSpec? lockSpec = null;
        if (allowLock)
        {
            lockSpec = new LockSpec(["timer"], "weakened");
            var timer = new TimerLockField(lockSpec, id: "NepsisTimer");
            timer.Changed += Changed;
            body.Children.Add(Ui.Heading("Timer lock"));
            body.Children.Add(Ui.Dim("Can't be weakened until then."));
            body.Children.Add(timer);
        }

        bool Valid() =>
            cards.All(k => k.Count.IsValid && k.Within.IsValid &&
                           k.Actions.Values.All(a => a.Minutes is null || a.Check.IsChecked != true || a.Minutes.IsValid))
            && quiet.IsValid && (lockSpec?.TimerProblem() is null);
        if (!await c.Dialogs.Ask(title, body, "Save", Valid, watch => refresh = watch, width: 760)) return null;

        var args = new List<string>();
        string? problem = null;
        for (var i = 0; i < cards.Count; i++)
        {
            var card = cards[i];
            var parts = new List<Dictionary<string, object>>();
            foreach (var (key, (check, minutes, param)) in card.Actions)
            {
                if (check.IsChecked != true) continue;
                var part = new Dictionary<string, object> { ["action"] = key };
                if (minutes?.Seconds is long s && s > 0) part["minutes"] = s / 60;
                if (param is not null)
                {
                    var p = param();
                    part["param"] = p;
                    if (p.Length == 0 || p == "[]")
                        problem = key == "mode" ? $"Step {i + 1}: choose the mode it runs." : $"Step {i + 1}: say what its custom action opens or runs.";
                }
                parts.Add(part);
            }
            if (parts.Count == 0) problem = $"Step {i + 1} does nothing: tick at least one thing for it to do.";
            args.AddRange(["--step-json", JsonSerializer.Serialize(new Dictionary<string, object>
            {
                ["count"] = card.Count.Value ?? 1, ["minutes"] = card.Within.Minutes, ["actions"] = parts,
            })]);
        }
        var counted = detectors.Where(d => d.Value.IsChecked == true).Select(d => d.Key).ToList();
        if (counted.Count == 0) problem = "Nothing is counted: tick at least one thing under What counts.";
        else if (counted.Count < Tables.Detectors.Length) args.AddRange(["--detect", string.Join(",", counted)]);
        if (quiet.Seconds is long q && q > 0) args.AddRange(["--quiet", $"{q / 60}m"]);
        var lockWords = "";
        if (lockSpec is not null)
        {
            args.AddRange(lockSpec.TimerFlags());
            lockWords = lockSpec.TimerWords();
        }
        if (problem is not null)
        {
            await c.Dialogs.Message(problem, "Not saved");
            return null;
        }
        return (args, lockWords);
    }

    /// <summary>Steps with monk mode or a Mode action need the acknowledgement.</summary>
    public static async Task<List<string>?> AcknowledgeMonkSteps(MainWindow window, List<string>? args)
    {
        if (args is null) return null;
        var steps = args.Where((a, i) => i > 0 && args[i - 1] is "--step" or "--step-json").ToList();
        if (steps.Any(s => System.Text.RegularExpressions.Regex.IsMatch(s, @"""action"":""(monk|mode)""") || System.Text.RegularExpressions.Regex.IsMatch(s, @"\bmonk\b")))
        {
            if (!await window.Controller.Dialogs.AcknowledgeNoConfirmation("A bypass prevention step")) return null;
            return [.. args, "--no-confirm-ok"];
        }
        return args;
    }
}
