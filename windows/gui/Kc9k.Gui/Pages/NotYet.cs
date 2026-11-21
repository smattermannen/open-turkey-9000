// Pages of the GTK GUI not yet built here (ROADMAP W4 says which); each says so.
using Kc9k.Core;
using Microsoft.UI.Xaml;

namespace Kc9k.Gui;

#pragma warning disable CS9113 // the window: used once the page is built
public abstract class NotYetPage(string area, string title, string what) : IPage
{
    public string Area { get; } = area;
    public UIElement View { get; } = Ui.Page(Ui.Column(16, Ui.PageHeader(title, what),
        Ui.Card(Ui.Dim("This page isn't built in the Windows GUI yet. Everything on it can be done with kc9k.exe in a terminal meanwhile."))));
    public virtual void Update(GuiState state) { }
}

public sealed class SchedulesPage(MainWindow window) : NotYetPage("blocks", "Schedules", "Schedules kept outside blocks, for blocks and modes to apply.")
{
    public void Open(string name) { }
}
public sealed class PlacesPage(MainWindow window) : NotYetPage("blocks", "Places", "Places, known by their Wi-Fi networks, that switch blocks.");
public sealed class HotkeysPage(MainWindow window) : NotYetPage("settings", "Hotkeys", "Keys that start blocks, optionally locked.");
public sealed class PresetsPage(MainWindow window) : NotYetPage("blocks", "Presets", "Saved copies of lists, phrase sets, blocks, automations, schedules and hotkeys.");
public sealed class StatsPage(MainWindow window) : NotYetPage("stats", "Statistics", "Time spent on websites and apps, and times blocked.");
public sealed class CircadianPage(MainWindow window) : NotYetPage("settings", "Circadian rhythm", "Screen, wallpaper and dark mode by time of day.");
public sealed class SettingsPage(MainWindow window) : NotYetPage("settings", "Settings", "Preferences, protection, browsers and passwords.")
{
    public void ShowBrowsers() { }
}

