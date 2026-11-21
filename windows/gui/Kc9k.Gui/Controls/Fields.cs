// Typed values (R-GUI-1): a box, optional one-click presets, and a line
// under it that reads the value back or says what is wrong. Nothing is ever
// clamped: the text stays as typed until it reads right.
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public abstract partial class CheckedField : StackPanel
{
    protected readonly TextBox Box;
    protected readonly TextBlock NoteText;
    readonly StackPanel line;

    public event Action? Changed;
    public string? Problem { get; private set; }
    public bool IsValid => Problem is null;
    public string Text { get => Box.Text; set => Box.Text = value; }

    protected CheckedField(string text, double width, string placeholder)
    {
        Spacing = 4;
        Box = new TextBox { Text = text, Width = width, PlaceholderText = placeholder };
        line = Ui.Row(6, Box);
        Children.Add(line);
        NoteText = Ui.Note("");
        Children.Add(NoteText);
        Box.TextChanged += (_, _) => Check();
    }

    protected void AddPreset(string label, string value)
    {
        var chip = new Button { Content = label, Padding = new Thickness(8, 3, 8, 4), FontSize = 12 };
        chip.Click += (_, _) => Box.Text = value;
        chip.Named(label);
        line.Children.Add(chip);
    }

    public CheckedField Named(string name, string? id = null)
    {
        AutomationProperties.SetName(Box, name);
        if (id is not null) AutomationProperties.SetAutomationId(Box, id);
        return this;
    }

    public void FocusBox() => Box.Focus(FocusState.Programmatic);

    protected abstract (string? Problem, string Describe) Read(string text);

    protected void Check()
    {
        var (problem, describe) = Read(Box.Text);
        Problem = problem;
        NoteText.Text = problem ?? describe;
        NoteText.Style = Theme.Style(problem is null ? "Kc9kNote" : "Kc9kProblem");
        NoteText.Visibility = NoteText.Text.Length > 0 ? Visibility.Visible : Visibility.Collapsed;
        AutomationProperties.SetHelpText(Box, NoteText.Text);
        Changed?.Invoke();
    }

    public bool Enabled { get => Box.IsEnabled; set { Box.IsEnabled = value; foreach (var c in line.Children) if (c is Control k) k.IsEnabled = value; } }
}

/// <summary>R-GUI-1: a length in days, hours and minutes.</summary>
public sealed partial class DurationField : CheckedField
{
    public DurationRule Rule { get; }
    public long? Seconds { get; private set; }

    public DurationField(DurationRule rule, long seconds = 0, IEnumerable<long>? presets = null, double width = 140)
        : base(seconds > 0 ? Words.DurationText(seconds) : "", width, rule.Placeholder)
    {
        Rule = rule;
        foreach (var p in presets ?? []) AddPreset(Words.ShortLength(p), Words.DurationText(p));
        Check();
    }

    protected override (string?, string) Read(string text)
    {
        var r = Rule.Read(text);
        Seconds = r.Value;
        return (r.Problem, r.Value is long v ? Rule.Describe(v, text) : "");
    }

    public string Arg() => Rule.Arg(Seconds ?? 0);
    public long Minutes => (Seconds ?? 0) / 60;

    public void SetSeconds(long seconds) => Box.Text = seconds > 0 ? Words.DurationText(seconds) : "";

    /// <summary>Changes the allowed range and checks the text again.</summary>
    public void SetLimits(long minimum, long? maximum, string? whyMax = null)
    {
        Rule.Minimum = minimum;
        Rule.Maximum = maximum;
        if (whyMax is not null) Rule.WhyMax = whyMax;
        Check();
    }
}

/// <summary>A whole number between a minimum and a maximum.</summary>
public sealed partial class NumberField : CheckedField
{
    public NumberRule Rule { get; }
    public string Hint { get; }
    public long? Value { get; private set; }

    public NumberField(long value, long minimum, long maximum, string hint = "", double width = 90)
        : base(value.ToString(System.Globalization.CultureInfo.InvariantCulture), width, "")
    {
        Rule = new NumberRule(minimum, maximum);
        Hint = hint;
        Check();
    }

    protected override (string?, string) Read(string text)
    {
        var r = (Rule ?? new NumberRule(0, long.MaxValue)).Read(text);
        Value = r.Value;
        return (r.Problem, Hint ?? "");
    }

    public void SetLimits(long minimum, long maximum)
    {
        Rule.Minimum = minimum;
        Rule.Maximum = maximum;
        Check();
    }
}

/// <summary>A time of day, typed ('18:00', '6pm').</summary>
public sealed partial class ClockField : CheckedField
{
    public int? Minute { get; private set; }
    readonly bool allowEnd;

    public ClockField(int minute, bool allowEnd = false, double width = 96)
        : base(Clock.Show(minute, allowEnd), width, Clock.Use24 ? "18:00" : "6:00 PM")
    {
        this.allowEnd = allowEnd;
        Check();
    }

    protected override (string?, string) Read(string text)
    {
        if (allowEnd && text.Trim() is "24:00" or "24")
        {
            Minute = 1440;
            return (null, "");
        }
        Minute = Clock.Parse(text);
        return (Minute is null ? "Type a time of day such as 18:00." : null, "");
    }
}
