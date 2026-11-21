// Small builders for the UI in code (R-GUI-3: grouped on cards, one widget
// per concept everywhere). Every control that a person can reach gets an
// accessible name: Narrator reads it, and the UI Automation tests find it.
using Kc9k.Core;
using Microsoft.UI.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;
using Microsoft.UI.Xaml.Media;
using Windows.Foundation;

namespace Kc9k.Gui;

public static class Ui
{
    public static T Named<T>(this T element, string name, string? id = null) where T : DependencyObject
    {
        AutomationProperties.SetName(element, name);
        if (id is not null) AutomationProperties.SetAutomationId(element, id);
        return element;
    }

    public static T Id<T>(this T element, string id) where T : DependencyObject
    {
        AutomationProperties.SetAutomationId(element, id);
        return element;
    }

    public static T Tip<T>(this T element, string text) where T : DependencyObject
    {
        ToolTipService.SetToolTip(element, text);
        return element;
    }

    public static TextBlock Text(string text, string? style = null, bool wrap = true)
    {
        var t = new TextBlock { Text = text, TextWrapping = wrap ? TextWrapping.Wrap : TextWrapping.NoWrap, IsTextSelectionEnabled = false };
        if (style is not null) t.Style = Theme.Style(style);
        if (!wrap) t.TextTrimming = TextTrimming.CharacterEllipsis;
        return t;
    }

    public static TextBlock Dim(string text, bool wrap = true)
    {
        var t = Text(text, "Kc9kDim", wrap);
        if (!wrap) t.TextWrapping = TextWrapping.NoWrap;
        return t;
    }

    public static TextBlock Note(string text) => Text(text, "Kc9kNote");
    public static TextBlock Strong(string text) => Text(text, "BodyStrongTextBlockStyle");

    public static TextBlock Heading(string text)
    {
        var t = Text(text, "Kc9kSectionTitle");
        AutomationProperties.SetHeadingLevel(t, AutomationHeadingLevel.Level2);
        return t;
    }

    public static StackPanel Column(double spacing = 8, params UIElement[] children)
    {
        var p = new StackPanel { Orientation = Orientation.Vertical, Spacing = spacing };
        foreach (var c in children) p.Children.Add(c);
        return p;
    }

    public static StackPanel Row(double spacing = 8, params UIElement[] children)
    {
        var p = new StackPanel { Orientation = Orientation.Horizontal, Spacing = spacing };
        foreach (var c in children)
        {
            if (c is FrameworkElement f && f.VerticalAlignment == VerticalAlignment.Stretch) f.VerticalAlignment = VerticalAlignment.Center;
            p.Children.Add(c);
        }
        return p;
    }

    /// <summary>A row that wraps onto more lines when the window is narrow.</summary>
    public static WrapPanel Wrap(double spacing = 8, params UIElement[] children)
    {
        var p = new WrapPanel { HorizontalSpacing = spacing, VerticalSpacing = spacing };
        foreach (var c in children) p.Children.Add(c);
        return p;
    }

    /// <summary>A rounded, bordered card (R-GUI-3: cards, not boxes in boxes).</summary>
    public static Border Card(UIElement child, string style = "Kc9kCard") =>
        new() { Style = Theme.Style(style), Child = child };

    public static Border Card(params UIElement[] children) => Card(Column(10, children));

    /// <summary>A titled group: the title (with a ? for the long text), then a card.</summary>
    public static StackPanel Section(string title, string? help, params UIElement[] children)
    {
        var head = Row(6, Heading(title));
        if (help is not null) head.Children.Add(HelpButton(help, title));
        return Column(8, head, Card(children));
    }

    public static StackPanel Section(string title, params UIElement[] children) => Section(title, null, children);

    /// <summary>A section without the card, for things that have their own frames.</summary>
    public static StackPanel Plain(string title, params UIElement[] children)
    {
        var p = Column(8, Heading(title));
        foreach (var c in children) p.Children.Add(c);
        return p;
    }

    /// <summary>A small ? that shows a longer explanation (progressive disclosure).</summary>
    public static Button HelpButton(string text, string about = "this")
    {
        var flyout = new Flyout
        {
            Content = new TextBlock { Text = text, TextWrapping = TextWrapping.Wrap, MaxWidth = 420, IsTextSelectionEnabled = true },
        };
        var b = new Button
        {
            Content = new FontIcon { Glyph = Glyphs.Help, FontSize = 12 },
            Padding = new Thickness(6, 4, 6, 4),
            VerticalAlignment = VerticalAlignment.Center,
            Flyout = flyout,
        };
        b.Named($"More about {about}").Tip("More about this");
        return b;
    }

    /// <summary>A page's title and its one-line subtitle.</summary>
    public static StackPanel PageHeader(string title, string subtitle = "", string? help = null)
    {
        var titleText = Text(title, "Kc9kPageTitle");
        AutomationProperties.SetHeadingLevel(titleText, AutomationHeadingLevel.Level1);
        var top = Row(8, titleText);
        if (help is not null) top.Children.Add(HelpButton(help, title));
        var p = Column(2, top);
        if (subtitle.Length > 0) p.Children.Add(Dim(subtitle));
        return p;
    }

    /// <summary>A status pill: on, locked, lifted, off or warn.</summary>
    public static Border Pill(Pill pill)
    {
        var (bg, fg) = pill.Kind switch
        {
            PillKind.On => ("Kc9kPillOnBackground", "Kc9kPillOnForeground"),
            PillKind.Locked => ("Kc9kPillLockedBackground", "Kc9kPillLockedForeground"),
            PillKind.Lifted => ("Kc9kPillLiftedBackground", "Kc9kPillLiftedForeground"),
            PillKind.Warn => ("Kc9kPillWarnBackground", "Kc9kPillWarnForeground"),
            _ => ("Kc9kPillOffBackground", "Kc9kPillOffForeground"),
        };
        var text = new TextBlock { Text = pill.Text, FontSize = 12, FontWeight = FontWeights.SemiBold, Foreground = Theme.Brush(fg) };
        var border = new Border
        {
            Background = Theme.Brush(bg),
            CornerRadius = new CornerRadius(10),
            Padding = new Thickness(9, 2, 9, 3),
            Child = text,
            VerticalAlignment = VerticalAlignment.Center,
        };
        AutomationProperties.SetName(border, pill.Text);
        return border;
    }

    public static WrapPanel Pills(IEnumerable<Pill> pills)
    {
        var p = new WrapPanel { HorizontalSpacing = 6, VerticalSpacing = 6 };
        foreach (var pill in pills) p.Children.Add(Pill(pill));
        return p;
    }

    public static Button Button(string label, Action click, string? id = null, bool accent = false, string? glyph = null)
    {
        object content = label;
        if (glyph is not null)
            content = Row(8, new FontIcon { Glyph = glyph, FontSize = 14 }, new TextBlock { Text = label });
        var b = new Button { Content = content };
        if (accent) b.Style = Theme.Style("AccentButtonStyle");
        b.Click += (_, _) => click();
        b.Named(label.TrimEnd('…'), id);
        return b;
    }

    public static Button IconButton(string glyph, string name, Action click, string? id = null)
    {
        var b = new Button { Content = new FontIcon { Glyph = glyph, FontSize = 14 }, Padding = new Thickness(8, 6, 8, 6) };
        b.Click += (_, _) => click();
        b.Named(name, id).Tip(name);
        return b;
    }

    public static HyperlinkButton Link(string label, Action click, string? id = null)
    {
        var b = new HyperlinkButton { Content = label, Padding = new Thickness(0) };
        b.Click += (_, _) => click();
        b.Named(label, id);
        return b;
    }

    /// <summary>A setting's row: title and description on the left, the control on the right.</summary>
    public static Grid SettingRow(string title, string detail, UIElement control, UIElement? under = null)
    {
        var g = new Grid { ColumnSpacing = 16 };
        g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var texts = Column(2, Text(title));
        if (detail.Length > 0) texts.Children.Add(Note(detail));
        if (under is not null) texts.Children.Add(under);
        g.Children.Add(texts);
        if (control is FrameworkElement f) f.VerticalAlignment = VerticalAlignment.Center;
        Grid.SetColumn((FrameworkElement)control, 1);
        g.Children.Add(control);
        return g;
    }

    public static ToggleSwitch Switch(string name, bool on, Action<bool> changed, string? id = null)
    {
        var t = new ToggleSwitch { IsOn = on, OnContent = "", OffContent = "", MinWidth = 0 };
        t.Named(name, id);
        t.Toggled += (_, _) =>
        {
            if (t.Tag is "updating") return;
            changed(t.IsOn);
        };
        return t;
    }

    /// <summary>Shows a switch's real state without running its command.</summary>
    public static void Set(ToggleSwitch t, bool on)
    {
        if (t.IsOn == on) return;
        t.Tag = "updating";
        t.IsOn = on;
        t.Tag = null;
    }

    public static ScrollViewer Page(UIElement content)
    {
        var holder = new Grid { MaxWidth = 1100, HorizontalAlignment = HorizontalAlignment.Stretch, Padding = new Thickness(32, 24, 32, 32) };
        holder.Children.Add(content);
        return new ScrollViewer
        {
            Content = holder,
            HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
        };
    }

    public static void Clear(Panel p) => p.Children.Clear();

    public static FontIcon Icon(string glyph, double size = 16) => new() { Glyph = glyph, FontSize = size };
}

/// <summary>Segoe Fluent Icons glyphs used here.</summary>
public static class Glyphs
{
    public const string Home = "", Blocks = "", Calendar = "", Place = "", Keyboard = "",
        Presets = "", Chart = "", Moon = "", Settings = "", Lock = "", Unlock = "",
        Add = "", Delete = "", Edit = "", Play = "", Stop = "", Next = "",
        Back = "", Help = "", Search = "", More = "", Clock = "", Save = "",
        Cancel = "", Info = "", Warning = "", Accept = "", Copy = "", Rename = "",
        Import = "", Export = "", Globe = "", App = "", Shield = "", Snow = "",
        Monk = "", Brightness = "", Picture = "", Record = "", Filter = "",
        Refresh = "", Coffee = "", Timer = "", Sync = "", Browser = "";
}

/// <summary>Lays children out like words in a line of text: each at its own
/// width, a new line only when the next doesn't fit (the GTK GUI's WrapBox).</summary>
public sealed partial class WrapPanel : Panel
{
    public double HorizontalSpacing { get; set; } = 8;
    public double VerticalSpacing { get; set; } = 8;

    protected override Size MeasureOverride(Size available)
    {
        double x = 0, y = 0, line = 0, widest = 0;
        foreach (var child in Children)
        {
            child.Measure(new Size(available.Width, double.PositiveInfinity));
            var size = child.DesiredSize;
            if (child.Visibility == Visibility.Collapsed) continue;
            if (x > 0 && x + size.Width > available.Width)
            {
                y += line + VerticalSpacing;
                x = 0;
                line = 0;
            }
            x += size.Width + HorizontalSpacing;
            line = Math.Max(line, size.Height);
            widest = Math.Max(widest, x - HorizontalSpacing);
        }
        return new Size(double.IsInfinity(available.Width) ? widest : Math.Min(widest, available.Width), y + line);
    }

    protected override Size ArrangeOverride(Size final)
    {
        double x = 0, y = 0, line = 0;
        foreach (var child in Children)
        {
            var size = child.DesiredSize;
            if (child.Visibility == Visibility.Collapsed) continue;
            if (x > 0 && x + size.Width > final.Width)
            {
                y += line + VerticalSpacing;
                x = 0;
                line = 0;
            }
            child.Arrange(new Rect(x, y, Math.Min(size.Width, final.Width), size.Height));
            x += size.Width + HorizontalSpacing;
            line = Math.Max(line, size.Height);
        }
        return final;
    }
}
