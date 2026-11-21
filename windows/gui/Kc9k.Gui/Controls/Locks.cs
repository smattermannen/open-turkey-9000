// Locks (§3.4, §4.6, R-GUI-2): the timer lock for a length or until a date
// picked on a calendar, every other kind behind More locks, the unlock order
// (R-LOCK-6), and the one confirmation (R-LOCK-12) before --yes. What each
// lock means as flags is Kc9k.Core's LockSpec; these only show it.
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;

namespace Kc9k.Gui;

/// <summary>R-GUI-2: no timer lock, one for a length (R-GUI-1), or one until a
/// date and time picked on a calendar.</summary>
public sealed partial class TimerLockField : StackPanel
{
    readonly LockSpec spec;
    readonly Dictionary<TimerMode, ToggleButton> modes = [];
    readonly DurationField duration;
    readonly CalendarDatePicker date;
    readonly ClockField time;
    readonly TextBlock untilNote = Ui.Note("");
    readonly StackPanel forPanel, untilPanel;
    public event Action? Changed;

    public TimerLockField(LockSpec spec, bool allowNone = true, string id = "Timer")
    {
        this.spec = spec;
        Spacing = 8;
        var switcher = Ui.Row(0);
        foreach (var (mode, label) in new[] { (TimerMode.None, "No timer"), (TimerMode.For, "For a time"), (TimerMode.Until, "Until a date") })
        {
            if (mode == TimerMode.None && !allowNone) continue;
            var b = new ToggleButton { Content = label, CornerRadius = new CornerRadius(0) }.Named(label, $"{id}_{mode}");
            b.Click += (_, _) => SetMode(mode);
            modes[mode] = b;
            switcher.Children.Add(b);
        }
        if (switcher.Children.FirstOrDefault() is ToggleButton first) first.CornerRadius = new CornerRadius(4, 0, 0, 4);
        if (switcher.Children.LastOrDefault() is ToggleButton last) last.CornerRadius = new CornerRadius(0, 4, 4, 0);
        Children.Add(switcher);

        duration = new DurationField(spec.TimerRule, Words.ParseDuration(spec.TimerText) ?? 3600, [3600, 86400, 7 * 86400, 30 * 86400]);
        duration.Named("Timer lock length", id + "_Length");
        duration.Changed += () => { spec.TimerText = duration.Text; Changed?.Invoke(); };
        forPanel = Ui.Column(0, duration);
        Children.Add(forPanel);

        date = new CalendarDatePicker { Date = spec.UntilDay, MinDate = DateTimeOffset.Now.Date, IsTodayHighlighted = true }.Named("Until which day", id + "_Date");
        date.DateChanged += (_, _) =>
        {
            if (date.Date is DateTimeOffset d) spec.UntilDay = d.Date;
            Refresh();
        };
        time = new ClockField(Clock.Parse(spec.UntilTime) ?? 18 * 60);
        time.Named("At what time", id + "_Time");
        time.Changed += () => { spec.UntilTime = time.Text; Refresh(); };
        untilPanel = Ui.Column(4, Ui.Row(8, date, Ui.Text("at"), time), untilNote);
        Children.Add(untilPanel);
        SetMode(allowNone ? spec.Timer : spec.Timer == TimerMode.None ? TimerMode.For : spec.Timer);
    }

    public void SetMode(TimerMode mode)
    {
        spec.Timer = mode;
        foreach (var (m, b) in modes) b.IsChecked = m == mode;
        forPanel.Visibility = mode == TimerMode.For ? Visibility.Visible : Visibility.Collapsed;
        untilPanel.Visibility = mode == TimerMode.Until ? Visibility.Visible : Visibility.Collapsed;
        Refresh();
    }

    void Refresh()
    {
        if (spec.Timer == TimerMode.Until)
        {
            var problem = spec.TimerProblem();
            untilNote.Text = problem ?? (spec.Until() is DateTime u ? $"Until {Words.Date(u)}, {Words.Until(u)}." : "");
            untilNote.Style = Theme.Style(problem is null ? "Kc9kNote" : "Kc9kProblem");
        }
        Changed?.Invoke();
    }
}

/// <summary>Every lock kind for a block or a setting (§3.4), the timer lock
/// first (R-GUI-2), the rest behind More locks (the GTK GUI's LockPicker).</summary>
public sealed partial class LockPicker : StackPanel
{
    public LockSpec Spec { get; }
    readonly TextBlock problem = Ui.Text("", "Kc9kProblem");
    readonly ComboBox textStyle;
    readonly NumberField chars;
    readonly TextBox customText;
    readonly PasswordBox password, again;
    readonly DurationField relock;
    readonly FrameworkElement rangeBits;
    public event Action? Changed;

    public LockPicker(string verb = "stop", string done = "stopped", IEnumerable<string>? kinds = null, bool relockAny = false, string id = "Locks")
    {
        Spec = new LockSpec(kinds, done, relockAny);
        Spacing = 12;
        var timer = new TimerLockField(Spec, id: id + "_Timer");
        timer.Changed += Fire;
        Children.Add(Field("Timer lock", timer, $"Can't be {done} at all until then. Time the computer is off doesn't count."));

        var more = Ui.Column(14);
        var others = new[] { "password", "text", "restart", "delay", "range" }.Where(Spec.Kinds.Contains).ToList();

        password = new PasswordBox { PlaceholderText = "Password", Width = 200 }.Named("Lock password", id + "_Password");
        again = new PasswordBox { PlaceholderText = "Repeat it", Width = 200 }.Named("Repeat the lock password", id + "_PasswordAgain");
        password.PasswordChanged += (_, _) => { Spec.Password = password.Password; Fire(); };
        again.PasswordChanged += (_, _) => { Spec.PasswordAgain = again.Password; Fire(); };
        if (Spec.Kinds.Contains("password"))
            more.Children.Add(Field("Password lock", Ui.Wrap(6, password, again), $"Needed to {verb} it. Give it to someone else to keep."));

        var typing = new CheckBox { Content = "Type a text to unlock" }.Named("Text lock: type a text to unlock", id + "_Typing");
        textStyle = new ComboBox { MinWidth = 180 }.Named("Text lock kind", id + "_TextStyle");
        textStyle.Items.Add(new ComboBoxItem { Content = "Random characters", Tag = "chars" });
        if (!Spec.OnlyChars)
        {
            textStyle.Items.Add(new ComboBoxItem { Content = "Random words", Tag = "words" });
            textStyle.Items.Add(new ComboBoxItem { Content = "My own text", Tag = "custom" });
        }
        textStyle.SelectedIndex = 0;
        chars = new NumberField(Tables.ChallengeDefault, Tables.ChallengeMin, Tables.ChallengeMax, width: 90);
        chars.Named("Text lock length in characters", id + "_Chars");
        customText = new TextBox { PlaceholderText = "The text to type", Width = 260 }.Named("Text lock's own text", id + "_CustomText");
        typing.Click += (_, _) => { Spec.Typing = typing.IsChecked == true; Fire(); };
        textStyle.SelectionChanged += (_, _) => { Spec.TextStyle = (textStyle.SelectedItem as ComboBoxItem)?.Tag as string ?? "chars"; Fire(); };
        chars.Changed += () => { Spec.CharsText = chars.Text; Fire(); };
        customText.TextChanged += (_, _) => { Spec.CustomText = customText.Text; Fire(); };
        if (Spec.Kinds.Contains("text"))
            more.Children.Add(Field("Text lock", Ui.Wrap(8, typing, textStyle, chars, customText),
                "Length in characters: about 300 take a few minutes to type, 5000 about an hour. Pasting is refused; the challenge opens in its own window." +
                (Spec.OnlyChars ? " A text lock already on can only get longer." : "")));

        var restart = new CheckBox { Content = "Stays on until the computer restarts" }.Named("Restart lock", id + "_Restart");
        restart.Click += (_, _) => { Spec.Restart = restart.IsChecked == true; Fire(); };
        if (Spec.Kinds.Contains("restart")) more.Children.Add(Field("Restart lock", restart));

        if (Spec.Kinds.Contains("delay"))
        {
            var delay = new DurationField(Spec.DelayRule, 0, [3600, 86400]);
            delay.Named("Delay lock", id + "_Delay");
            delay.Changed += () => { Spec.DelayText = delay.Text; Fire(); };
            more.Children.Add(Field("Delay lock", delay, "Unlocking only takes effect after this long."));
        }

        var rangeOn = new CheckBox { Content = "On at set times on set days" }.Named("Range lock", id + "_Range");
        rangeOn.Click += (_, _) => { Spec.RangeOn = rangeOn.IsChecked == true; Fire(); };
        var days = Ui.Wrap(4);
        foreach (var (key, title) in Clock.Weekdays)
        {
            var t = new ToggleButton { Content = title, IsChecked = Spec.RangeDays.Contains(key), MinWidth = 48 }.Named(title, $"{id}_RangeDay_{key}");
            t.Click += (_, _) =>
            {
                if (t.IsChecked == true) Spec.RangeDays.Add(key); else Spec.RangeDays.Remove(key);
                Fire();
            };
            days.Children.Add(t);
        }
        var from = new ClockField(9 * 60);
        from.Named("Range lock from", id + "_RangeFrom");
        var to = new ClockField(17 * 60);
        to.Named("Range lock to", id + "_RangeTo");
        from.Changed += () => { Spec.RangeFrom = from.Text; Fire(); };
        to.Changed += () => { Spec.RangeTo = to.Text; Fire(); };
        var invert = new CheckBox { Content = "outside these times instead" }.Named("Range lock outside these times", id + "_RangeInvert");
        invert.Click += (_, _) => { Spec.RangeInvert = invert.IsChecked == true; Fire(); };
        rangeBits = Ui.Column(6, days, Ui.Wrap(8, Ui.Text("from"), from, Ui.Text("to"), to, invert));
        if (Spec.Kinds.Contains("range"))
            more.Children.Add(Field("Range lock", Ui.Column(6, rangeOn, rangeBits), "Can't be unlocked at these times, only waited out."));

        relock = new DurationField(Spec.RelockRule, 0, [1800, 3600]);
        relock.Named("Re-lock", id + "_Relock");
        relock.Changed += () => { Spec.RelockText = relock.Text; Fire(); };
        if (Spec.Kinds.Contains("relock"))
            more.Children.Add(Field("Re-lock", relock, relockAny
                ? "This long after it is unlocked, all its locks come on again, in their order."
                : "With a password or text lock: unlocking lifts them only this long; then they come on again."));

        if (more.Children.Count > 0)
        {
            var expander = new Expander
            {
                Header = "More locks: " + string.Join(", ", others),
                Content = more,
                HorizontalAlignment = HorizontalAlignment.Stretch,
                HorizontalContentAlignment = HorizontalAlignment.Stretch,
            }.Named("More locks", id + "_More");
            Children.Add(expander);
        }
        Children.Add(problem);
        Fire();
    }

    static UIElement Field(string title, UIElement field, string note = "")
    {
        var p = Ui.Column(4, Ui.Strong(title), field);
        if (note.Length > 0) p.Children.Add(Ui.Note(note));
        return p;
    }

    void Fire()
    {
        var custom = Spec.TextStyle == "custom";
        textStyle.IsEnabled = Spec.Typing;
        chars.Visibility = custom ? Visibility.Collapsed : Visibility.Visible;
        chars.Enabled = Spec.Typing;
        customText.Visibility = custom ? Visibility.Visible : Visibility.Collapsed;
        customText.IsEnabled = Spec.Typing;
        if (rangeBits is not null) rangeBits.Opacity = Spec.RangeOn ? 1 : 0.5;
        relock.Enabled = Spec.RelockAllowed();
        var why = Spec.Problem();
        problem.Text = why ?? "";
        problem.Visibility = why is null ? Visibility.Collapsed : Visibility.Visible;
        Changed?.Invoke();
    }

    public bool IsValid => Spec.Problem() is null;
}

public static class LockDialogs
{
    /// <summary>A dialog around a LockPicker; null when cancelled, else what
    /// it asks for: (locks as (flags, words), re-lock flags, password). The
    /// button waits until every field reads right, and with needLock until a
    /// lock is chosen.</summary>
    public static async Task<(List<(List<string> Flags, string Words)> Chosen, List<string> Relock, string Secret)?> Ask(
        MainWindow window, string title, string intro, string button, LockPicker picker, UIElement? extra = null,
        Func<string?>? extraProblem = null, Action<Action>? watchExtra = null, bool needLock = false)
    {
        var body = Ui.Column(14, Ui.Dim(intro));
        if (extra is not null) body.Children.Add(extra);
        body.Children.Add(picker);
        bool Valid() => picker.IsValid && extraProblem?.Invoke() is null && (!needLock || picker.Spec.Chosen().Count > 0);
        var ok = await window.Controller.Dialogs.Ask(title, body, button, Valid, watch =>
        {
            picker.Changed += watch;
            watchExtra?.Invoke(watch);
        }, width: 720);
        if (!ok || !picker.IsValid) return null;
        return (picker.Spec.Chosen(), picker.Spec.RelockFlags(), picker.Spec.Secret);
    }

    /// <summary>R-LOCK-6: the user puts the locks in the order they are
    /// unlocked (first at the top). Null when cancelled.</summary>
    public static async Task<List<(List<string> Flags, string Words)>?> ChooseOrder(MainWindow window, List<(List<string> Flags, string Words)> chosen)
    {
        var order = chosen.ToList();
        var list = new ListView { SelectionMode = ListViewSelectionMode.Single, MinWidth = 380 }.Named("Unlock order", "LockOrder");
        void Fill(int select)
        {
            list.Items.Clear();
            foreach (var (_, text) in order) list.Items.Add(text.Split(" (")[0]);
            list.SelectedIndex = select;
        }
        void Move(int step)
        {
            int i = list.SelectedIndex, j = i + step;
            if (i < 0 || j < 0 || j >= order.Count) return;
            (order[i], order[j]) = (order[j], order[i]);
            Fill(j);
        }
        Fill(0);
        var body = Ui.Column(10,
            Ui.Dim("The locks are unlocked one at a time, top first. A timer lock below another counts from when the one above is unlocked."),
            list,
            Ui.Row(6, Ui.Button("Up", () => Move(-1), "LockOrderUp", glyph: ""), Ui.Button("Down", () => Move(1), "LockOrderDown", glyph: "")));
        if (!await window.Controller.Dialogs.Ask("Unlock order", body)) return null;
        return LockSpec.Waiting(order);
    }

    /// <summary>The lines a confirmation shows for locks on a target, and the
    /// command's flags in order (R-LOCK-12).</summary>
    public static (List<string> Args, List<string> Lines) Lines(string target, List<(List<string> Flags, string Words)> chosen, List<string> relock)
    {
        var args = new List<string>();
        var lines = new List<string>();
        for (var i = 0; i < chosen.Count; i++)
        {
            args.AddRange(chosen[i].Flags);
            var order = chosen.Count > 1 ? $"{i + 1} of {chosen.Count}: " : "";
            lines.Add($"{target}: {order}{chosen[i].Words}");
        }
        args.AddRange(relock);
        lines.AddRange(LockSpec.RelockLines(target, relock));
        return (args, lines);
    }
}
