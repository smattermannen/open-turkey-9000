// Appearance and conveniences are preferences of whoever runs the GUI, so
// they live in their own %APPDATA%\kc9k\gui.json, not in the KC9K database
// (as the GTK GUI's ~/.config/kc9k/gui.json). KC9K_GUI_PREFS moves it (tests).
using System.Text.Json;
using System.Text.Json.Serialization;
using Kc9k.Core;

namespace Kc9k.Gui;

public sealed class Prefs
{
    public string Theme { get; set; } = "system"; // system, light, dark
    public int Darkness { get; set; } = 30;       // §4.10: dark grey (0) to black (100)
    public bool Clock24 { get; set; } = true;
    public int FirstDay { get; set; }              // 0 Monday ... 6 Sunday
    public bool Tray { get; set; } = true;
    public bool OpenAtLogin { get; set; }
    public int AccessIdleMinutes { get; set; }
    public Dictionary<string, double> ListHeights { get; set; } = [];
    public int WindowWidth { get; set; } = 1180;
    public int WindowHeight { get; set; } = 820;
    public bool WindowMaximized { get; set; }
    public string StatsRange { get; set; } = "week";

    // Before Current: static fields are set in the order written.
    static readonly JsonSerializerOptions Options = new() { PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower, WriteIndented = true };

    [JsonIgnore] public static Prefs Current { get; private set; } = Load();

    static string PathOf()
    {
        var env = Environment.GetEnvironmentVariable("KC9K_GUI_PREFS");
        if (!string.IsNullOrEmpty(env)) return env;
        return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "kc9k", "gui.json");
    }

    static Prefs Load()
    {
        Prefs prefs;
        try { prefs = JsonSerializer.Deserialize<Prefs>(File.ReadAllText(PathOf()), Options) ?? new(); }
        catch (Exception) { prefs = new(); }
        Clock.Use24 = prefs.Clock24;
        Clock.FirstDay = Math.Clamp(prefs.FirstDay, 0, 6);
        return prefs;
    }

    public void Save()
    {
        Clock.Use24 = Clock24;
        Clock.FirstDay = Math.Clamp(FirstDay, 0, 6);
        try
        {
            var path = PathOf();
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(path, JsonSerializer.Serialize(this, Options));
        }
        catch (Exception e) { Log.Write("prefs: " + e.Message); } // the choice still applies until the GUI closes
    }
}
