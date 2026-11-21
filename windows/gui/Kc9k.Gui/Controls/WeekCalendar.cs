// §4.5 a schedule as a week (the GTK GUI's WeekCalendar): the days across
// from the first day of the week, 00:00 to 24:00 down. Drag down a day to
// add a window (15-minute grid); drag a window to move it (to another day
// for a window of one day); click a window to edit it. Filled: a block
// window; outlined: an allow window; dashed: from a schedule (changed on the
// Schedules page); grey behind: other blocks' windows. The keyboard's way is
// the list of windows beside it (each editor has one).
using Kc9k.Core;
using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Shapes;

namespace Kc9k.Gui;

public sealed partial class WeekCalendar : Grid
{
    const int Snap = 15;
    const double Left = 48, Top = 24;
    readonly Canvas canvas = new() { Background = new SolidColorBrush(Colors.Transparent) };
    List<TimeWindow> windows = [];
    List<TimeWindow> others = [];
    (int Column, int Start, int Now)? drag;
    (int Index, int Column, int Minute)? moving;
    double pressY;
    Rectangle? ghost;

    /// <summary>(column, start, end): a window drawn.</summary>
    public event Action<int, int, int>? Create;
    /// <summary>A window clicked: its index.</summary>
    public event Action<int>? Pick;
    /// <summary>(index, days, start, end): a window dragged elsewhere.</summary>
    public event Action<int, int, int, int>? Move;

    public WeekCalendar()
    {
        Height = 400;
        Children.Add(canvas);
        AutomationProperties.SetName(this, "Week: drag down a day to add a window");
        AutomationProperties.SetAutomationId(this, "WeekCalendar");
        SizeChanged += (_, _) => Draw();
        ActualThemeChanged += (_, _) => Draw();
        canvas.PointerPressed += Pressed;
        canvas.PointerMoved += Moved;
        canvas.PointerReleased += Released;
    }

    public void SetWindows(IEnumerable<TimeWindow> mine, IEnumerable<TimeWindow>? theirs = null)
    {
        windows = [.. mine];
        others = [.. theirs ?? []];
        Draw();
    }

    double ColumnWidth => Math.Max((ActualWidth - Left - 4) / 7, 10);
    double GridHeight => Math.Max(ActualHeight - Top - 4, 10);
    double Y(double minute) => Top + GridHeight * minute / 1440;
    static int DayOfColumn(int column) => (column + Clock.FirstDay) % 7; // 0 = Monday

    /// <summary>The column's day as a window's day bit (bit 0 is Sunday).</summary>
    public static int Bit(int column) => 1 << ((DayOfColumn(column) + 1) % 7);
    public static string DayKey(int column) => Words.Weekdays[DayOfColumn(column)].Key;

    (int Column, int Minute, double Exact) At(Windows.Foundation.Point p)
    {
        var column = Math.Clamp((int)Math.Floor((p.X - Left) / ColumnWidth), 0, 6);
        var exact = (p.Y - Top) / GridHeight * 1440;
        var minute = (int)Math.Round(exact / Snap) * Snap;
        return (column, Math.Clamp(minute, 0, 1440), exact);
    }

    int? WindowAt(int column, double minute)
    {
        for (var i = 0; i < windows.Count; i++)
            if ((windows[i].Days & Bit(column)) != 0 && windows[i].Start <= minute && minute < windows[i].End) return i;
        return null;
    }

    Brush B(string key) => Theme.Brush(key);

    void Draw()
    {
        canvas.Children.Clear();
        if (ActualWidth <= 0) return;
        var faint = B("DividerStrokeColorDefaultBrush");
        var muted = B("TextFillColorSecondaryBrush");
        var accent = (B("AccentFillColorDefaultBrush") as SolidColorBrush)?.Color ?? Colors.SteelBlue;
        for (var hour = 0; hour <= 24; hour += 3)
        {
            var y = Math.Round(Y(hour * 60)) + 0.5;
            canvas.Children.Add(new Line { X1 = Left, X2 = Left + 7 * ColumnWidth, Y1 = y, Y2 = y, Stroke = faint, StrokeThickness = 1 });
            if (hour % 6 == 0)
            {
                var t = new TextBlock { Text = Clock.Hour(hour), FontSize = 11, Foreground = muted };
                Canvas.SetLeft(t, 0);
                Canvas.SetTop(t, y - 8);
                canvas.Children.Add(t);
            }
        }
        for (var c = 0; c <= 7; c++)
        {
            var x = Math.Round(Left + c * ColumnWidth) + 0.5;
            canvas.Children.Add(new Line { X1 = x, X2 = x, Y1 = Top, Y2 = Top + GridHeight, Stroke = faint, StrokeThickness = 1 });
        }
        for (var c = 0; c < 7; c++)
        {
            var t = new TextBlock { Text = Words.Weekdays[DayOfColumn(c)].Title, FontSize = 12 };
            t.Measure(new Windows.Foundation.Size(200, 40));
            Canvas.SetLeft(t, Left + c * ColumnWidth + ColumnWidth / 2 - t.DesiredSize.Width / 2);
            Canvas.SetTop(t, 2);
            canvas.Children.Add(t);
        }
        foreach (var w in others)
            for (var c = 0; c < 7; c++)
                if ((w.Days & Bit(c)) != 0) canvas.Children.Add(Box(c, w.Start, w.End, new SolidColorBrush(Color(accent, 0)) { Opacity = 0 }, faint, 0, null, gray: true));
        for (var i = 0; i < windows.Count; i++)
        {
            var w = windows[i];
            for (var c = 0; c < 7; c++)
            {
                if ((w.Days & Bit(c)) == 0) continue;
                Brush fill = w.Source.Length > 0 ? new SolidColorBrush(Color(accent, w.Allow ? 30 : 115))
                    : w.Allow ? new SolidColorBrush(Color(accent, 40)) : new SolidColorBrush(Color(accent, 205));
                Brush? stroke = w.Source.Length > 0 || w.Allow ? new SolidColorBrush(Color(accent, 230)) : null;
                var label = $"{Words.Weekdays[DayOfColumn(c)].Title} {Clock.Show(w.Start)}–{Clock.Show(w.End, true)}" +
                            (w.Allow ? ", allow window" : "") + (w.Source.Length > 0 ? $", from the schedule {w.Source}" : "");
                canvas.Children.Add(Box(c, w.Start, w.End, fill, stroke, w.Source.Length > 0 ? 1 : w.Allow ? 1.5 : 0, label, dashed: w.Source.Length > 0, source: w.Source));
            }
        }
    }

    static Windows.UI.Color Color(Windows.UI.Color c, int alpha) => Windows.UI.Color.FromArgb((byte)alpha, c.R, c.G, c.B);

    UIElement Box(int column, int start, int end, Brush fill, Brush? stroke, double thickness, string? label, bool gray = false, bool dashed = false, string source = "")
    {
        var height = Math.Max(Y(end) - Y(start), 2);
        var r = new Rectangle
        {
            Width = Math.Max(ColumnWidth - 4, 2), Height = height, RadiusX = 3, RadiusY = 3,
            Fill = gray ? Theme.Brush("ControlStrongFillColorDisabledBrush") : fill,
            Stroke = stroke, StrokeThickness = thickness, IsHitTestVisible = false,
        };
        if (gray) r.Opacity = 0.35;
        if (dashed) r.StrokeDashArray = [3, 2];
        if (label is not null) AutomationProperties.SetName(r, label);
        var holder = new Grid { IsHitTestVisible = false };
        holder.Children.Add(r);
        if (source.Length > 0 && height > 16)
            holder.Children.Add(new TextBlock { Text = source, FontSize = 11, Margin = new Thickness(4, 1, 2, 0), TextTrimming = TextTrimming.CharacterEllipsis, MaxWidth = ColumnWidth - 8 });
        Canvas.SetLeft(holder, Left + column * ColumnWidth + 2);
        Canvas.SetTop(holder, Y(start));
        return holder;
    }

    void Pressed(object sender, PointerRoutedEventArgs e)
    {
        var p = e.GetCurrentPoint(canvas);
        if (!p.Properties.IsLeftButtonPressed) return;
        var (column, minute, exact) = At(p.Position);
        var index = WindowAt(column, exact);
        moving = index is int i ? (i, column, minute) : null;
        drag = (column, minute, minute);
        pressY = p.Position.Y;
        canvas.CapturePointer(e.Pointer);
        e.Handled = true;
    }

    void Moved(object sender, PointerRoutedEventArgs e)
    {
        if (drag is not { } d) return;
        var (_, minute, _) = At(e.GetCurrentPoint(canvas).Position);
        drag = (d.Column, d.Start, minute);
        if (ghost is not null) canvas.Children.Remove(ghost);
        int s = Math.Min(d.Start, minute), t = Math.Max(d.Start, minute);
        ghost = new Rectangle
        {
            Width = ColumnWidth - 4, Height = Math.Max(Y(t) - Y(s), 2),
            Fill = new SolidColorBrush(Windows.UI.Color.FromArgb(90, 128, 128, 128)), IsHitTestVisible = false,
        };
        Canvas.SetLeft(ghost, Left + d.Column * ColumnWidth + 2);
        Canvas.SetTop(ghost, Y(s));
        canvas.Children.Add(ghost);
    }

    void Released(object sender, PointerRoutedEventArgs e)
    {
        canvas.ReleasePointerCapture(e.Pointer);
        if (ghost is not null)
        {
            canvas.Children.Remove(ghost);
            ghost = null;
        }
        if (drag is not { } d) return;
        drag = null;
        var p = e.GetCurrentPoint(canvas).Position;
        var (toColumn, end, exact) = At(p);
        var moved = Math.Abs(p.Y - pressY) >= 5 || toColumn != d.Column;
        if (moving is { } m && moved)
        {
            // Move the window: by the time dragged, and to another day for a
            // window of one day.
            moving = null;
            var w = windows[m.Index];
            var length = w.End - w.Start;
            var newStart = Math.Clamp(w.Start + end - m.Minute, 0, 1440 - length);
            var days = w.Days;
            if (System.Numerics.BitOperations.PopCount((uint)days) == 1 && toColumn != m.Column) days = Bit(toColumn);
            if (newStart != w.Start || days != w.Days) Move?.Invoke(m.Index, days, newStart, newStart + length);
            return;
        }
        moving = null;
        int start = Math.Min(d.Start, end), stop = Math.Max(d.Start, end);
        if (moved)
        {
            if (stop - start >= Snap) Create?.Invoke(d.Column, start, stop);
        }
        else if (WindowAt(d.Column, exact) is int index) Pick?.Invoke(index);
    }
}
