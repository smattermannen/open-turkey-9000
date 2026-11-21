// Small charts built from ordinary elements (not pixels), so Narrator reads
// every row and they follow the theme and the display's scale by themselves.
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;

namespace Kc9k.Gui;

/// <summary>A label above one large number.</summary>
public sealed partial class StatTile : StackPanel
{
    readonly TextBlock value;
    readonly string title;

    public StatTile(string title, string id)
    {
        this.title = title;
        Spacing = 2;
        Children.Add(Ui.Text(title, "Kc9kNote"));
        value = Ui.Text("–", "SubtitleTextBlockStyle");
        Children.Add(value);
        this.Named(title, id);
    }

    public void Set(string text)
    {
        value.Text = text;
        AutomationProperties.SetName(this, $"{title}: {text}");
    }
}

/// <summary>The most used sites or apps: horizontal bars with name and time.</summary>
public sealed partial class TopChart : Grid
{
    public Func<long, string> Format { get; set; } = Words.Spent;
    /// <summary>A name's colour (the daily chart's series); the accent otherwise.</summary>
    public Func<string, Brush?> ColorOf { get; set; } = _ => null;

    public TopChart()
    {
        ColumnSpacing = 10;
        RowSpacing = 6;
        ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Auto), MaxWidth = 260 });
        ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
    }

    public void SetItems(IReadOnlyList<(string Name, long Value)> items)
    {
        Children.Clear();
        RowDefinitions.Clear();
        var max = Math.Max(1, items.Count > 0 ? items.Max(i => i.Value) : 1);
        var total = Math.Max(1, items.Sum(i => i.Value));
        for (var r = 0; r < items.Count; r++)
        {
            var (name, v) = items[r];
            RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
            var label = Ui.Text(name, wrap: false);
            label.MaxWidth = 250;
            Grid.SetRow(label, r);
            Children.Add(label);
            var bar = new Border
            {
                Height = 12,
                CornerRadius = new CornerRadius(0, 6, 6, 0),
                Background = ColorOf(name) ?? Theme.Brush("AccentFillColorDefaultBrush"),
                HorizontalAlignment = HorizontalAlignment.Left,
                VerticalAlignment = VerticalAlignment.Center,
            };
            var holder = new Grid();
            holder.Children.Add(bar);
            holder.SizeChanged += (_, e) => bar.Width = Math.Max(2, e.NewSize.Width * v / max);
            var text = $"{name}: {Format(v)} ({Math.Round(100.0 * v / total)}% of the listed total)";
            ToolTipService.SetToolTip(holder, text);
            AutomationProperties.SetName(holder, text);
            Grid.SetRow(holder, r);
            Grid.SetColumn(holder, 1);
            Children.Add(holder);
            var value = Ui.Note(Format(v));
            value.VerticalAlignment = VerticalAlignment.Center;
            Grid.SetRow(value, r);
            Grid.SetColumn(value, 2);
            Children.Add(value);
        }
    }
}
