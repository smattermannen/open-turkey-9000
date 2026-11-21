// §4.10 one automation clip of circadian rhythm (the GTK GUI's ClipEditor):
// a 24-hour curve of points. Right-click adds a point (on a point: removes
// it); drag a point to move it (Shift: to the minute); drag the small
// handle in the middle of a stretch to bend it; double-click a point or
// handle to type exact values. The keyboard's way: Points…, a list of the
// points to add, change and remove.
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI;
using Microsoft.UI.Input;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Shapes;
using Windows.Foundation;
using Windows.System;

namespace Kc9k.Gui;

public sealed partial class ClipEditor : Grid
{
    const double L = 52, R = 12, T = 10, B = 30, Grab = 9;
    readonly Canvas canvas = new() { Background = new SolidColorBrush(Colors.Transparent) };
    readonly MainWindow window;
    Lane lane;
    public List<ClipPoint> Points { get; private set; }
    double minute;
    bool active;
    public double? Playhead { get; set; }
    (string Kind, int Index, double StartY, double StartTension)? drag;
    bool moved;

    /// <summary>After an edit: whether a point was dragged (its preview then ends).</summary>
    public event Action<ClipEditor, bool>? Changed;
    /// <summary>While a point is dragged: its value, to show on the screen.</summary>
    public event Action<ClipEditor, double>? Preview;
    public string LaneName => lane.Name;

    public ClipEditor(MainWindow window, Lane lane)
    {
        this.window = window;
        this.lane = lane;
        Points = Copy(lane.Points);
        Height = 200;
        Children.Add(canvas);
        AutomationProperties.SetName(this, $"{lane.Title}: right-click adds a point; Points… edits them by keyboard");
        AutomationProperties.SetAutomationId(this, "Clip_" + lane.Name);
        SizeChanged += (_, _) => Draw();
        ActualThemeChanged += (_, _) => Draw();
        canvas.PointerPressed += Pressed;
        canvas.PointerMoved += MovedPointer;
        canvas.PointerReleased += Released;
        canvas.DoubleTapped += OnDoubleTap;
        canvas.RightTapped += OnRightTap;
    }

    static List<ClipPoint> Copy(IEnumerable<ClipPoint> p) => [.. p.Select(x => new ClipPoint { Minute = x.Minute, Value = x.Value, Tension = x.Tension })];

    /// <summary>Takes the stored clip over, unless a drag is under way.</summary>
    public void SetData(Lane l, double now, bool on)
    {
        minute = now;
        active = on;
        if (drag is null)
        {
            lane = l;
            Points = Copy(l.Points);
        }
        Draw();
    }

    (double X, double Y, double W, double H) Plot() => (L, T, Math.Max(10, ActualWidth - L - R), Math.Max(10, ActualHeight - T - B));
    double X(double m) { var (x, _, w, _) = Plot(); return x + w * m / 1440; }
    double Y(double v) { var (_, y, _, h) = Plot(); return y + h * (1 - (v - lane.Min) / (lane.Max - lane.Min)); }

    double MinuteOf(double px, bool fine)
    {
        var (x, _, w, _) = Plot();
        var m = (px - x) / w * 1440;
        var step = fine ? 1 : 5;
        return Math.Clamp(Math.Round(m / step) * step, 0, 1439);
    }

    double ValueOf(double py)
    {
        var (_, y, _, h) = Plot();
        var v = lane.Min + (1 - (py - y) / h) * (lane.Max - lane.Min);
        var step = lane.Unit == "K" ? 50 : 1;
        return Math.Clamp(Math.Round(v / step) * step, lane.Min, lane.Max);
    }

    int? PointAt(Point p)
    {
        for (var i = 0; i < Points.Count; i++)
            if (Math.Abs(X(Points[i].Minute) - p.X) <= Grab && Math.Abs(Y(Points[i].Value) - p.Y) <= Grab) return i;
        return null;
    }

    IEnumerable<(int Index, double X, double Y)> Handles()
    {
        var n = Points.Count;
        if (n < 2) yield break;
        for (var i = 0; i < n; i++)
        {
            var start = Points[i].Minute;
            var end = Points[(i + 1) % n].Minute + (i + 1 == n ? 1440 : 0);
            var mid = (start + end) / 2 % 1440;
            yield return (i, X(mid), Y(Clips.Value(Points, mid) ?? 0));
        }
    }

    int? HandleAt(Point p)
    {
        foreach (var (i, hx, hy) in Handles())
            if (Math.Abs(hx - p.X) <= Grab && Math.Abs(hy - p.Y) <= Grab) return i;
        return null;
    }

    public static Windows.UI.Color Kelvin(double k)
    {
        static double[] Body(double kelvin)
        {
            var t = Math.Clamp(kelvin, 1000, 40000) / 100;
            double r, g, b;
            if (t <= 66) { r = 255; g = 99.4708025861 * Math.Log(t) - 161.1195681661; }
            else { r = 329.698727446 * Math.Pow(t - 60, -0.1332047592); g = 288.1221695283 * Math.Pow(t - 60, -0.0755148492); }
            b = t >= 66 ? 255 : t <= 19 ? 0 : 138.5177312231 * Math.Log(t - 10) - 305.0447927307;
            return [Math.Clamp(r, 1, 255), Math.Clamp(g, 1, 255), Math.Clamp(b, 1, 255)];
        }
        var here = Body(k);
        var neutral = Body(6500);
        var rgb = here.Zip(neutral, (h, n) => h / n).ToArray();
        var top = rgb.Max();
        return Windows.UI.Color.FromArgb(255, (byte)(255 * rgb[0] / top), (byte)(255 * rgb[1] / top), (byte)(255 * rgb[2] / top));
    }

    static readonly (double Hour, byte R, byte G, byte B)[] Sky =
        [(0, 10, 15, 38), (5, 26, 41, 82), (6.5, 245, 163, 97), (8, 143, 201, 230), (12, 191, 230, 255), (17, 143, 201, 230), (18.5, 245, 163, 97), (20, 26, 41, 82), (24, 10, 15, 38)];

    void Draw()
    {
        canvas.Children.Clear();
        if (ActualWidth <= 0) return;
        var (x, y, w, h) = Plot();
        var faint = Theme.Brush("DividerStrokeColorDefaultBrush", this);
        var soft = Theme.Brush("TextFillColorSecondaryBrush", this);
        var accent = (Theme.Brush("AccentFillColorDefaultBrush", this) as SolidColorBrush)?.Color ?? Colors.SteelBlue;
        var alpha = active ? 1.0 : 0.45;
        var marks = lane.Unit == "K" ? new double[] { 1000, 2500, 4000, 5500, 7000, 8500, 10000 } : [lane.Min, 25, 50, 75, 100];
        foreach (var mark in marks.Where(m => m >= lane.Min && m <= lane.Max))
        {
            var py = Math.Round(Y(mark)) + 0.5;
            canvas.Children.Add(new Line { X1 = x, X2 = x + w, Y1 = py, Y2 = py, Stroke = faint, StrokeThickness = 1 });
            Label($"{mark.ToString("0", CultureInfo.InvariantCulture)}{(lane.Unit == "K" ? "" : " %")}", 0, py - 8, soft);
        }
        var sky = new LinearGradientBrush { StartPoint = new Point(0, 0), EndPoint = new Point(1, 0) };
        foreach (var (hour, r, g, b) in Sky) sky.GradientStops.Add(new GradientStop { Color = Windows.UI.Color.FromArgb(255, r, g, b), Offset = hour / 24 });
        var band = new Rectangle { Width = w, Height = 6, Fill = sky };
        Canvas.SetLeft(band, x);
        Canvas.SetTop(band, y + h + 3);
        canvas.Children.Add(band);
        for (var hour = 0; hour <= 24; hour += 3)
        {
            var px = Math.Round(x + w * hour / 24) + 0.5;
            canvas.Children.Add(new Line { X1 = px, X2 = px, Y1 = y, Y2 = y + h, Stroke = faint, StrokeThickness = 1 });
            Label(Clock.Hour(hour), Math.Clamp(px - 16, x - 10, x + w - 30), y + h + 11, soft);
        }
        if (Points.Count == 0)
        {
            Label("No points yet: right-click to add one.", x + w / 2 - 110, y + h / 2 - 8, soft);
            return;
        }
        var samples = new List<(double X, double V)>();
        for (var step = 0.0; step <= w; step += 2) samples.Add((x + step, Clips.Value(Points, Math.Min(step / w * 1440, 1439.999)) ?? 0));
        var baseY = y + h;
        if (lane.Unit == "K")
            foreach (var (px, v) in samples)
            {
                var c = Kelvin(v);
                var r = new Rectangle { Width = 2, Height = Math.Max(0, baseY - Y(v)), Fill = new SolidColorBrush(Windows.UI.Color.FromArgb((byte)(190 * alpha), c.R, c.G, c.B)) };
                Canvas.SetLeft(r, px);
                Canvas.SetTop(r, Y(v));
                canvas.Children.Add(r);
            }
        else
        {
            var area = new Polygon { Fill = new SolidColorBrush(Windows.UI.Color.FromArgb((byte)(56 * alpha), accent.R, accent.G, accent.B)) };
            area.Points.Add(new Point(x, baseY));
            foreach (var (px, v) in samples) area.Points.Add(new Point(px, Y(v)));
            area.Points.Add(new Point(samples[^1].X, baseY));
            canvas.Children.Add(area);
        }
        var curve = new Polyline { Stroke = new SolidColorBrush(Windows.UI.Color.FromArgb((byte)(255 * alpha), accent.R, accent.G, accent.B)), StrokeThickness = 2 };
        foreach (var (px, v) in samples) curve.Points.Add(new Point(px, Y(v)));
        canvas.Children.Add(curve);
        var nowX = Math.Round(X(minute)) + 0.5;
        canvas.Children.Add(new Line { X1 = nowX, X2 = nowX, Y1 = y, Y2 = y + h + 9, Stroke = Theme.Brush("TextFillColorPrimaryBrush", this), StrokeThickness = 1, Opacity = 0.7 });
        if (Playhead is double ph)
        {
            var px = Math.Round(X(ph)) + 0.5;
            canvas.Children.Add(new Line { X1 = px, X2 = px, Y1 = y, Y2 = y + h + 9, Stroke = new SolidColorBrush(accent), StrokeThickness = 2 });
        }
        var stroke = new SolidColorBrush(Windows.UI.Color.FromArgb((byte)(255 * alpha), accent.R, accent.G, accent.B));
        foreach (var (_, hx, hy) in Handles()) Dot(hx, hy, 3, stroke, 1.5);
        foreach (var p in Points) Dot(X(p.Minute), Y(p.Value), 4.5, stroke, 2);
    }

    void Label(string text, double x, double y, Brush brush)
    {
        var t = new TextBlock { Text = text, FontSize = 11, Foreground = brush, IsHitTestVisible = false };
        Canvas.SetLeft(t, x);
        Canvas.SetTop(t, y);
        canvas.Children.Add(t);
    }

    void Dot(double cx, double cy, double r, Brush stroke, double thickness)
    {
        var e = new Ellipse { Width = 2 * r, Height = 2 * r, Fill = new SolidColorBrush(Colors.White), Stroke = stroke, StrokeThickness = thickness, IsHitTestVisible = false };
        Canvas.SetLeft(e, cx - r);
        Canvas.SetTop(e, cy - r);
        canvas.Children.Add(e);
    }

    string Describe(double v) => lane.Unit == "K" ? $"{v:0} K" : $"{v:0} %";

    static bool Shift() => InputKeyboardSource.GetKeyStateForCurrentThread(VirtualKey.Shift).HasFlag(Windows.UI.Core.CoreVirtualKeyStates.Down);

    void OnRightTap(object sender, RightTappedRoutedEventArgs e)
    {
        var p = e.GetPosition(canvas);
        if (PointAt(p) is int i) Points.RemoveAt(i);
        else
        {
            var m = MinuteOf(p.X, Shift());
            Points.RemoveAll(q => q.Minute == m);
            Points.Add(new ClipPoint { Minute = m, Value = ValueOf(p.Y) });
            Points.Sort((a, b) => a.Minute.CompareTo(b.Minute));
        }
        Draw();
        Changed?.Invoke(this, false);
        e.Handled = true;
    }

    void Pressed(object sender, PointerRoutedEventArgs e)
    {
        var pt = e.GetCurrentPoint(canvas);
        if (!pt.Properties.IsLeftButtonPressed) return;
        var p = pt.Position;
        moved = false;
        if (PointAt(p) is int i) drag = ("point", i, 0, 0);
        else if (HandleAt(p) is int s) drag = ("tension", s, p.Y, Points[s].Tension);
        else return;
        canvas.CapturePointer(e.Pointer);
        e.Handled = true;
    }

    void MovedPointer(object sender, PointerRoutedEventArgs e)
    {
        var p = e.GetCurrentPoint(canvas).Position;
        if (drag is not { } d)
        {
            var (x, _, w, _) = Plot();
            string? tip = null;
            if (PointAt(p) is int i) tip = $"{Clock.Show(Points[i].Minute)}  {Describe(Points[i].Value)}\nDrag to move · right-click to remove · double-click to type";
            else if (HandleAt(p) is int s) tip = $"Bend {Points[s].Tension:+0.00;-0.00;0}\nDrag up or down to bend this stretch · double-click to type";
            else if (Points.Count > 0 && p.X >= x && p.X <= x + w) tip = $"{Clock.Show((p.X - x) / w * 1440)}  {Describe(Clips.Value(Points, (p.X - x) / w * 1440) ?? 0)}";
            ToolTipService.SetToolTip(canvas, tip);
            return;
        }
        moved = true;
        if (d.Kind == "point")
        {
            var point = Points[d.Index];
            var m = MinuteOf(p.X, Shift());
            // A point stays between its neighbours.
            if (d.Index > 0) m = Math.Max(m, Points[d.Index - 1].Minute + 1);
            if (d.Index + 1 < Points.Count) m = Math.Min(m, Points[d.Index + 1].Minute - 1);
            point.Minute = m;
            point.Value = ValueOf(p.Y);
            Preview?.Invoke(this, point.Value);
        }
        else
            // Up bulges the curve up, whichever way it runs.
            Points[d.Index].Tension = Math.Round(Math.Clamp(d.StartTension + (d.StartY - p.Y) / 80, -1, 1), 2);
        Draw();
    }

    void Released(object sender, PointerRoutedEventArgs e)
    {
        canvas.ReleasePointerCapture(e.Pointer);
        if (drag is not { } d) return;
        drag = null;
        if (moved) Changed?.Invoke(this, d.Kind == "point");
        Draw();
    }

    void OnDoubleTap(object sender, DoubleTappedRoutedEventArgs e)
    {
        var p = e.GetPosition(canvas);
        drag = null;
        if ((PointAt(p) ?? HandleAt(p)) is int i) _ = TypePoint(i);
    }

    /// <summary>A point's exact time, value and bend to the next.</summary>
    public async Task TypePoint(int index, bool isNew = false)
    {
        var point = isNew ? new ClipPoint { Minute = 12 * 60, Value = lane.Neutral } : Points[index];
        var time = new ClockField((int)point.Minute);
        time.Named("Time", "PointTime");
        var value = new NumberBox { Minimum = lane.Min, Maximum = lane.Max, Value = point.Value, SmallChange = lane.Unit == "K" ? 50 : 1, SpinButtonPlacementMode = NumberBoxSpinButtonPlacementMode.Inline }
            .Named($"Value ({(lane.Unit == "K" ? "K" : "%")})", "PointValue");
        var bend = new NumberBox { Minimum = -1, Maximum = 1, Value = point.Tension, SmallChange = 0.05, SpinButtonPlacementMode = NumberBoxSpinButtonPlacementMode.Inline }
            .Named("Bend to the next point", "PointBend").Tip("0 is straight, above 0 bulges it up, below 0 down.");
        var body = Ui.Column(8, Ui.Text("Time"), time, Ui.Text($"Value ({(lane.Unit == "K" ? "K" : "%")})"), value, Ui.Text("Bend to the next point"), bend);
        if (!await window.Controller.Dialogs.Ask($"{lane.Title}: point", body, "Save", () => time.IsValid, w => time.Changed += w)) return;
        var m = time.Minute ?? 0;
        if (isNew) Points.Add(point);
        Points.RemoveAll(q => !ReferenceEquals(q, point) && q.Minute == m);
        point.Minute = m;
        point.Value = double.IsNaN(value.Value) ? point.Value : value.Value;
        point.Tension = double.IsNaN(bend.Value) ? 0 : Math.Round(bend.Value, 2);
        Points.Sort((a, b) => a.Minute.CompareTo(b.Minute));
        Draw();
        Changed?.Invoke(this, false);
    }

    /// <summary>The keyboard's way: the points as a list to add, change and remove.</summary>
    public async Task EditPoints()
    {
        var list = new ListView { SelectionMode = ListViewSelectionMode.Single, MinWidth = 320, MaxHeight = 300 }.Named($"{lane.Title}: points", "ClipPoints");
        void Fill()
        {
            list.Items.Clear();
            foreach (var p in Points) list.Items.Add($"{Clock.Show(p.Minute)}  {Describe(p.Value)}" + (p.Tension != 0 ? $"  bend {p.Tension:+0.00;-0.00}" : ""));
        }
        Fill();
        var body = Ui.Column(8, list, Ui.Row(6,
            Ui.Button("Add…", () => { window.Controller.Dialogs.Open?.Hide(); _pending = () => TypePoint(0, isNew: true); }, "ClipPointAdd"),
            Ui.Button("Change…", () => { if (list.SelectedIndex >= 0) { var i = list.SelectedIndex; window.Controller.Dialogs.Open?.Hide(); _pending = () => TypePoint(i); } }, "ClipPointChange"),
            Ui.Button("Remove", () =>
            {
                if (list.SelectedIndex < 0) return;
                Points.RemoveAt(list.SelectedIndex);
                Fill();
                Draw();
                Changed?.Invoke(this, false);
            }, "ClipPointRemove")));
        _pending = null;
        await window.Controller.Dialogs.Ask($"{lane.Title}: points", body, "Done");
        if (_pending is { } next)
        {
            _pending = null;
            await next();
            await EditPoints();
        }
    }

    Func<Task>? _pending;
}
