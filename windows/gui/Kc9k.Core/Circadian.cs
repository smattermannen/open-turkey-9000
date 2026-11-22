// `kc9k circadian show --json` (internal/cli/circadian.go): the circadian
// rhythm page's state (§4.10).
namespace Kc9k.Core;

public sealed class ClipPoint
{
    public double Minute { get; set; }
    public double Value { get; set; }
    public double Tension { get; set; }
}

public sealed class Lane
{
    public string Name { get; set; } = "";
    public string Title { get; set; } = "";
    public string Unit { get; set; } = "";
    public double Min { get; set; }
    public double Max { get; set; }
    public double Neutral { get; set; }
    public string Detail { get; set; } = "";
    public bool On { get; set; }
    public List<ClipPoint> Points { get; set; } = [];
    public double Value { get; set; }
    public bool HasValue { get; set; }
    public long PausedUntil { get; set; }
    public bool Available { get; set; } = true;
}

public sealed class CircadianPreset
{
    public string Name { get; set; } = "";
    public string Title { get; set; } = "";
    public string Detail { get; set; } = "";
}

public sealed class DarkSettings
{
    public string Mode { get; set; } = "system"; // system, light, dark, scheduled
    public string LightFrom { get; set; } = "07:00";
    public string LightTo { get; set; } = "20:00";
    public string Now { get; set; } = "";
}

public sealed class Hardware
{
    public bool Daemon { get; set; }
    public List<string> Backlights { get; set; } = [];
    public List<string> External { get; set; } = [];
    public bool Ddcutil { get; set; }
    public string Gamma { get; set; } = "";
    public bool NightLight { get; set; }
}

public sealed class CircadianView
{
    public bool On { get; set; }
    public double Minute { get; set; }
    public List<Lane> Lanes { get; set; } = [];
    public List<CircadianPreset> Presets { get; set; } = [];
    public List<Picture> Wallpapers { get; set; } = [];
    public bool OverrideModes { get; set; }
    public DarkSettings Dark { get; set; } = new();
    public bool DarkPages { get; set; }
    public List<string> DarkExempt { get; set; } = [];
    public Hardware Hardware { get; set; } = new();

    public static CircadianView Parse(string json) => Kc9kJson.Parse<CircadianView>(json);
}

/// <summary>A clip's curve (the GTK GUI's clip_shape, clip_segment, clip_value, points_text).</summary>
public static class Clips
{
    public static double Shape(double x, double tension, bool rising)
    {
        x = Math.Clamp(x, 0, 1);
        if (!rising) tension = -tension;
        var middle = 0.5 + 0.49 * Math.Clamp(tension, -1, 1);
        if (Math.Abs(middle - 0.5) < 1e-9) return x;
        // Below the straight line x^p, above it its mirror image: +t and -t
        // are mirror images and neither end is a corner (circadian.Shape).
        if (middle < 0.5) return Math.Pow(x, Math.Log(middle) / Math.Log(0.5));
        return 1 - Math.Pow(1 - x, Math.Log(1 - middle) / Math.Log(0.5));
    }

    /// <summary>The points the curve runs between at minute: (from, to, start, end).</summary>
    public static (int A, int B, double Start, double End) Segment(IReadOnlyList<ClipPoint> points, double minute)
    {
        var count = points.Count;
        var index = -1;
        for (var i = 0; i < count; i++) if (points[i].Minute <= minute) index = i;
        if (index < 0) return (count - 1, 0, points[^1].Minute - 1440, points[0].Minute);
        if (index + 1 < count) return (index, index + 1, points[index].Minute, points[index + 1].Minute);
        return (index, 0, points[index].Minute, points[0].Minute + 1440);
    }

    public static double? Value(IReadOnlyList<ClipPoint> points, double minute)
    {
        if (points.Count == 0) return null;
        if (points.Count == 1) return points[0].Value;
        var (a, b, start, end) = Segment(points, minute);
        if (end <= start) return points[a].Value;
        var way = Shape((minute - start) / (end - start), points[a].Tension, points[b].Value >= points[a].Value);
        return points[a].Value + (points[b].Value - points[a].Value) * way;
    }

    /// <summary>Points as 'kc9k circadian lane --points' reads them.</summary>
    public static string Text(IEnumerable<ClipPoint> points) => string.Join(" ", points.Select(p =>
        $"{Clock.Text(p.Minute)}={p.Value.ToString("0.0", System.Globalization.CultureInfo.InvariantCulture)}" +
        (p.Tension != 0 ? "~" + p.Tension.ToString("0.00", System.Globalization.CultureInfo.InvariantCulture) : "")));
}
