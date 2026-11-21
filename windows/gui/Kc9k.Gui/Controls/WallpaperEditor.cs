// Wallpapers by time (§4.10; R-MODE-5, owner 2026-11-17; the GTK GUI's
// WallpaperEditor and WallpaperTimeline): a 24-hour timeline, one lane per
// picture like clips in a video editor (drag a bar to move it, an end to
// change it; 5-minute steps, Shift: to the minute), a row per picture with
// its times typed (the keyboard's way), Set, Remove, and Add a wallpaper….
// The same for circadian rhythm and for each mode: Command() gives
// ["circadian", "wallpaper"] or ["mode", "wallpaper", NAME].
using Kc9k.Core;
using Microsoft.UI;
using Microsoft.UI.Input;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;
using Microsoft.UI.Xaml.Shapes;
using Windows.System;

namespace Kc9k.Gui;

public sealed partial class WallpaperEditor : StackPanel
{
    readonly MainWindow window;
    readonly Func<List<string>> command;
    readonly Canvas timeline = new() { Background = new SolidColorBrush(Colors.Transparent), Height = 60 };
    readonly StackPanel rows = Ui.Column(6);
    List<Picture> pictures = [];
    string shown = "";
    (Picture P, string Part, double X, int From, int To)? drag;
    (int From, int To)? dragged;
    const double LeftPad = 14, RightPad = 14, Top = 22, LaneH = 40, Gap = 6;

    public WallpaperEditor(MainWindow window, Func<List<string>> command, string id)
    {
        this.window = window;
        this.command = command;
        Spacing = 8;
        AutomationProperties.SetName(timeline, "Wallpapers through the day: drag a bar to move it, an end to change it");
        Children.Add(timeline);
        Children.Add(rows);
        Children.Add(Ui.Button("Add a wallpaper…", () => _ = Add(), id + "_AddWallpaper", glyph: Glyphs.Picture));
        timeline.SizeChanged += (_, _) => Draw();
        timeline.PointerPressed += Pressed;
        timeline.PointerMoved += Moved;
        timeline.PointerReleased += Released;
    }

    Task Run(params string[] args) => window.Controller.Run([.. command(), .. args]);

    static int Minutes(string hm)
    {
        var p = hm.Split(':');
        return p.Length == 2 && int.TryParse(p[0], out var h) && int.TryParse(p[1], out var m) ? (h * 60 + m) % 1440 : 0;
    }

    public void Fill(List<Picture> list)
    {
        if (drag is not null) return;
        pictures = list;
        timeline.Visibility = list.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        timeline.Height = Top + Math.Max(list.Count, 1) * (LaneH + Gap) + 6;
        Draw();
        var key = string.Join("|", command()) + System.Text.Json.JsonSerializer.Serialize(list, Kc9kJson.Options);
        if (key == shown) return;
        shown = key;
        rows.Children.Clear();
        foreach (var p in list)
        {
            var from = new ClockField(Minutes(p.From), width: 80);
            from.Named($"{p.Name}: from", "WallpaperFrom" + p.Index);
            var to = new ClockField(Minutes(p.To), allowEnd: true, width: 80);
            to.Named($"{p.Name}: to", "WallpaperTo" + p.Index);
            var index = p.Index.ToString(System.Globalization.CultureInfo.InvariantCulture);
            var g = new Grid { ColumnSpacing = 10 };
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            g.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
            g.Children.Add(Thumb(p.Path, 96, 54));
            var name = Ui.Text(p.Name + (p.Active ? "  (now)" : ""), wrap: false);
            name.VerticalAlignment = VerticalAlignment.Center;
            Grid.SetColumn(name, 1);
            g.Children.Add(name);
            var controls = Ui.Row(6, from, Ui.Text("to"), to,
                Ui.Button("Set", () => { if (from.IsValid && to.IsValid) _ = Run("time", index, Clock.Text(from.Minute!.Value), to.Minute >= 1440 ? "24:00" : Clock.Text(to.Minute!.Value)); }, "WallpaperSet" + p.Index),
                Ui.Button("Remove", () => _ = Run("remove", index), "WallpaperRemove" + p.Index));
            Grid.SetColumn(controls, 2);
            g.Children.Add(controls);
            rows.Children.Add(g);
        }
        if (list.Count == 0) rows.Children.Add(Ui.Dim("None yet."));
    }

    public static UIElement Thumb(string path, double width, double height)
    {
        var holder = new Border { Width = width, Height = height, CornerRadius = new CornerRadius(4), Background = Theme.Brush("ControlFillColorSecondaryBrush") };
        try
        {
            if (File.Exists(path))
                holder.Child = new Image { Source = new BitmapImage(new Uri(path)) { DecodePixelWidth = (int)width * 2 }, Stretch = Stretch.UniformToFill };
            else holder.Child = new FontIcon { Glyph = Glyphs.Picture, FontSize = 20, Opacity = 0.6 };
        }
        catch (Exception) { holder.Child = new FontIcon { Glyph = Glyphs.Picture, FontSize = 20, Opacity = 0.6 }; }
        return holder;
    }

    double W => Math.Max(timeline.ActualWidth - LeftPad - RightPad, 1);
    double X(double m) => LeftPad + W * m / 1440;
    double MinuteAt(double x, bool fine)
    {
        var m = (x - LeftPad) / W * 1440;
        var step = fine ? 1 : 5;
        return Math.Round(m / step) * step;
    }

    void Draw()
    {
        timeline.Children.Clear();
        if (timeline.ActualWidth <= 0) return;
        var faint = Theme.Brush("DividerStrokeColorDefaultBrush", this);
        var soft = Theme.Brush("TextFillColorSecondaryBrush", this);
        var accent = (Theme.Brush("AccentFillColorDefaultBrush", this) as SolidColorBrush)?.Color ?? Colors.SteelBlue;
        for (var hour = 0; hour <= 24; hour += 3)
        {
            var x = Math.Round(X(hour * 60)) + 0.5;
            timeline.Children.Add(new Line { X1 = x, X2 = x, Y1 = Top - 4, Y2 = timeline.Height, Stroke = faint, StrokeThickness = 1 });
            var t = new TextBlock { Text = Clock.Hour(hour % 24), FontSize = 11, Foreground = soft };
            Canvas.SetLeft(t, Math.Clamp(x - 16, 0, timeline.ActualWidth - 34));
            timeline.Children.Add(t);
        }
        var now = DateTime.Now.Hour * 60 + DateTime.Now.Minute;
        for (var i = 0; i < pictures.Count; i++)
        {
            var p = pictures[i];
            var (from, to) = drag?.P == p && dragged is { } d ? d : (Minutes(p.From), Minutes(p.To));
            var y = Top + i * (LaneH + Gap);
            // Past midnight it wraps round: two pieces.
            var pieces = to > from ? new[] { (from, to) } : new[] { (from, 1440), (0, to == 0 ? 0 : to) };
            foreach (var (a, b) in pieces.Where(x => x.Item2 > x.Item1))
            {
                var r = new Rectangle
                {
                    Width = Math.Max(X(b) - X(a), 2), Height = LaneH, RadiusX = 4, RadiusY = 4,
                    Fill = new SolidColorBrush(Windows.UI.Color.FromArgb(p.Active ? (byte)220 : (byte)150, accent.R, accent.G, accent.B)),
                    IsHitTestVisible = false,
                };
                Canvas.SetLeft(r, X(a));
                Canvas.SetTop(r, y);
                timeline.Children.Add(r);
            }
            var label = new TextBlock { Text = $"{p.Name}  {Clock.Show(from)}–{Clock.Show(to == 0 ? 1440 : to, true)}", FontSize = 12, Foreground = new SolidColorBrush(Colors.White), IsHitTestVisible = false, MaxWidth = Math.Max(X(to > from ? to : 1440) - X(from) - 8, 0), TextTrimming = TextTrimming.CharacterEllipsis };
            Canvas.SetLeft(label, X(from) + 6);
            Canvas.SetTop(label, y + 11);
            timeline.Children.Add(label);
        }
        var nx = Math.Round(X(now)) + 0.5;
        timeline.Children.Add(new Line { X1 = nx, X2 = nx, Y1 = Top - 4, Y2 = timeline.Height, Stroke = Theme.Brush("TextFillColorPrimaryBrush", this), StrokeThickness = 1, Opacity = 0.7 });
    }

    static bool Shift() => InputKeyboardSource.GetKeyStateForCurrentThread(VirtualKey.Shift).HasFlag(Windows.UI.Core.CoreVirtualKeyStates.Down);

    void Pressed(object sender, PointerRoutedEventArgs e)
    {
        var pt = e.GetCurrentPoint(timeline).Position;
        var lane = (int)Math.Floor((pt.Y - Top) / (LaneH + Gap));
        if (lane < 0 || lane >= pictures.Count) return;
        var p = pictures[lane];
        int from = Minutes(p.From), to = Minutes(p.To);
        var part = Math.Abs(pt.X - X(from)) <= 8 ? "from" : Math.Abs(pt.X - X(to == 0 ? 1440 : to)) <= 8 ? "to" : "move";
        drag = (p, part, pt.X, from, to);
        dragged = (from, to);
        timeline.CapturePointer(e.Pointer);
    }

    void Moved(object sender, PointerRoutedEventArgs e)
    {
        var pt = e.GetCurrentPoint(timeline).Position;
        if (drag is not { } d)
        {
            var lane = (int)Math.Floor((pt.Y - Top) / (LaneH + Gap));
            ProtectedCursor = lane >= 0 && lane < pictures.Count && (Math.Abs(pt.X - X(Minutes(pictures[lane].From))) <= 8 || Math.Abs(pt.X - X(Minutes(pictures[lane].To) == 0 ? 1440 : Minutes(pictures[lane].To))) <= 8)
                ? InputSystemCursor.Create(InputSystemCursorShape.SizeWestEast) : null;
            return;
        }
        var delta = (int)(MinuteAt(pt.X, Shift()) - MinuteAt(d.X, Shift()));
        int Wrap(int m) => ((m % 1440) + 1440) % 1440;
        dragged = d.Part switch
        {
            "from" => (Wrap(d.From + delta), d.To),
            "to" => (d.From, Wrap(d.To + delta)),
            _ => (Wrap(d.From + delta), Wrap(d.To + delta)),
        };
        Draw();
    }

    void Released(object sender, PointerRoutedEventArgs e)
    {
        timeline.ReleasePointerCapture(e.Pointer);
        if (drag is not { } d || dragged is not { } n) return;
        drag = null;
        dragged = null;
        if ((n.From, n.To) != (d.From, d.To))
            _ = Run("time", d.P.Index.ToString(System.Globalization.CultureInfo.InvariantCulture), Clock.Text(n.From), Clock.Text(n.To));
        Draw();
    }

    /// <summary>A picture and its times.</summary>
    async Task Add()
    {
        var path = await Files.Open(window, ".png", ".jpg", ".jpeg", ".webp", ".bmp");
        if (path is null) return;
        var from = new ClockField(6 * 60);
        from.Named("From", "NewWallpaperFrom");
        var to = new ClockField(12 * 60, allowEnd: true);
        to.Named("To", "NewWallpaperTo");
        if (!await window.Controller.Dialogs.Ask("When is it the wallpaper?", Ui.Wrap(8, Ui.Text("From"), from, Ui.Text("to"), to), "Add",
                () => from.IsValid && to.IsValid, w => { from.Changed += w; to.Changed += w; })) return;
        await Run("add", path, Clock.Text(from.Minute!.Value), to.Minute >= 1440 ? "24:00" : Clock.Text(to.Minute!.Value));
    }
}
