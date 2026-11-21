// Dialogs: one at a time (WinUI allows one ContentDialog open per window),
// Escape cancels, Enter takes the main button when it can be used, and the
// main button waits until what was typed reads right (R-GUI-1).
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class Dialogs(Func<XamlRoot?> root, Func<ElementTheme> theme)
{
    readonly SemaphoreSlim one = new(1, 1);

    /// <summary>The dialog shown now (for tests and for closing on lock).</summary>
    public ContentDialog? Open { get; private set; }

    public async Task<ContentDialogResult> Show(ContentDialog dialog)
    {
        await one.WaitAsync();
        try
        {
            dialog.XamlRoot = root();
            dialog.RequestedTheme = theme();
            dialog.DefaultButton = dialog.IsPrimaryButtonEnabled && !string.IsNullOrEmpty(dialog.PrimaryButtonText)
                ? ContentDialogButton.Primary : ContentDialogButton.None;
            Open = dialog;
            return await dialog.ShowAsync();
        }
        catch (Exception e)
        {
            Log.Write("dialog: " + e.Message);
            return ContentDialogResult.None;
        }
        finally
        {
            Open = null;
            one.Release();
        }
    }

    static ContentDialog New(string title, object? content, string primary, string close = "Cancel") => new()
    {
        Title = title,
        Content = content,
        PrimaryButtonText = primary,
        CloseButtonText = close,
    };

    static ScrollViewer Scrolling(UIElement content) => new()
    {
        Content = content,
        VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
        HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
        Padding = new Thickness(0, 0, 12, 0),
    };

    /// <summary>What a command said, refusals included, in the CLI's words.</summary>
    public async Task Message(string text, string title = "Not done")
    {
        var body = new TextBlock { Text = text, TextWrapping = TextWrapping.Wrap, IsTextSelectionEnabled = true };
        body.Named(text, "MessageText");
        await Show(new ContentDialog { Title = title, Content = Scrolling(body), CloseButtonText = "OK" });
    }

    public async Task<bool> Confirm(string title, string detail, string ok = "OK", bool destructive = false, string cancel = "Cancel")
    {
        var d = New(title, Scrolling(Ui.Text(detail)), ok, cancel);
        if (!destructive) d.PrimaryButtonStyle = Theme.Style("AccentButtonStyle");
        return await Show(d) == ContentDialogResult.Primary;
    }

    /// <summary>R-LOCK-12: the one confirmation for every lock. The lines say
    /// what is locked and how, as the CLI shows them; the caller passes --yes
    /// only when this returns true. With an option, a tick box is shown too.</summary>
    public async Task<(bool Ok, bool Option)> ConfirmLock(IReadOnlyList<string> locks, string? option = null)
    {
        if (locks.Count == 0) return (true, false);
        var body = Ui.Column(10);
        foreach (var line in locks) body.Children.Add(Ui.Text("• " + line));
        body.Children.Add(Ui.Dim("While a lock is on, this can't be weakened."));
        CheckBox? tick = null;
        if (option is not null)
        {
            tick = new CheckBox { Content = option }.Named(option, "LockOption");
            body.Children.Add(tick);
        }
        var d = New(locks.Count == 1 ? "Set this lock?" : "Set these locks?", Scrolling(body), "Lock");
        d.PrimaryButtonStyle = Theme.Style("AccentButtonStyle");
        var ok = await Show(d) == ContentDialogResult.Primary;
        return (ok, tick?.IsChecked == true);
    }

    /// <summary>R-LOCK-12: monk mode started by itself asks nothing, so setting
    /// it up needs the tick "I know this will not ask for confirmation".</summary>
    public async Task<bool> AcknowledgeNoConfirmation(string what)
    {
        var tick = new CheckBox { Content = "I know this will not ask for confirmation" }.Named("I know this will not ask for confirmation", "AcknowledgeTick");
        var d = New("Monk mode will start without asking",
            Ui.Column(12, Ui.Text($"{what} starts monk mode by itself, and it will not ask for confirmation when it does."), tick),
            "Set it up");
        d.IsPrimaryButtonEnabled = false;
        tick.Checked += (_, _) => d.IsPrimaryButtonEnabled = true;
        tick.Unchecked += (_, _) => d.IsPrimaryButtonEnabled = false;
        return await Show(d) == ContentDialogResult.Primary && tick.IsChecked == true;
    }

    /// <summary>Asks for a line of text; null when cancelled. problem says what
    /// is wrong with it (null: nothing): the main button waits and the reason
    /// shows under the box.</summary>
    public async Task<string?> AskText(string title, string detail = "", bool password = false, string initial = "",
        Func<string, string?>? problem = null, string ok = "OK")
    {
        var body = Ui.Column(8);
        if (detail.Length > 0) body.Children.Add(Ui.Text(detail));
        Control box;
        Func<string> read;
        if (password)
        {
            var p = new PasswordBox { Password = initial }.Named(title, "AskPassword");
            box = p;
            read = () => p.Password;
        }
        else
        {
            var t = new TextBox { Text = initial }.Named(title, "AskText");
            t.SelectAll();
            box = t;
            read = () => t.Text.Trim();
        }
        body.Children.Add(box);
        var note = Ui.Text("", "Kc9kProblem");
        body.Children.Add(note);
        var d = New(title, body, ok);
        d.PrimaryButtonStyle = Theme.Style("AccentButtonStyle");
        void Check()
        {
            var why = problem?.Invoke(read());
            note.Text = why ?? "";
            d.IsPrimaryButtonEnabled = why is null;
        }
        if (box is TextBox tb) tb.TextChanged += (_, _) => Check();
        if (box is PasswordBox pb) pb.PasswordChanged += (_, _) => Check();
        Check();
        d.Opened += (_, _) => box.Focus(FocusState.Programmatic);
        return await Show(d) == ContentDialogResult.Primary ? read() : null;
    }

    /// <summary>R-GUI-1: a length in days, hours and minutes; its seconds, or
    /// null when cancelled. OK waits until it reads right.</summary>
    public async Task<long?> AskDuration(string title, string detail, long seconds, DurationRule rule, IEnumerable<long>? presets = null)
    {
        var field = new DurationField(rule, seconds, presets);
        field.Named(title, "AskDuration");
        var d = New(title, Ui.Column(12, Ui.Text(detail), field), "OK");
        d.PrimaryButtonStyle = Theme.Style("AccentButtonStyle");
        field.Changed += () => d.IsPrimaryButtonEnabled = field.IsValid;
        d.IsPrimaryButtonEnabled = field.IsValid;
        d.Opened += (_, _) => field.FocusBox();
        return await Show(d) == ContentDialogResult.Primary && field.Seconds is long s ? s : null;
    }

    /// <summary>A dialog around any content; valid says when the main button can be used.</summary>
    public async Task<bool> Ask(string title, UIElement content, string ok = "OK", Func<bool>? valid = null,
        Action<Action>? watch = null, double width = 0, bool scroll = true)
    {
        var d = New(title, scroll ? Scrolling(content) : content, ok);
        d.PrimaryButtonStyle = Theme.Style("AccentButtonStyle");
        if (width > 0) d.Resources["ContentDialogMaxWidth"] = width;
        if (valid is not null)
        {
            d.IsPrimaryButtonEnabled = valid();
            watch?.Invoke(() => d.IsPrimaryButtonEnabled = valid());
        }
        return await Show(d) == ContentDialogResult.Primary;
    }

    /// <summary>Picks one of several; null when cancelled.</summary>
    public async Task<string?> Choose(string title, string detail, IReadOnlyList<(string Key, string Title)> choices, string? selected = null, string ok = "OK")
    {
        var combo = new ComboBox { MinWidth = 260 }.Named(title, "AskChoice");
        foreach (var (key, text) in choices) combo.Items.Add(new ComboBoxItem { Content = text, Tag = key });
        combo.SelectedIndex = Math.Max(0, choices.ToList().FindIndex(c => c.Key == selected));
        var body = Ui.Column(10);
        if (detail.Length > 0) body.Children.Add(Ui.Text(detail));
        body.Children.Add(combo);
        if (!await Ask(title, body, ok)) return null;
        return (combo.SelectedItem as ComboBoxItem)?.Tag as string;
    }
}
