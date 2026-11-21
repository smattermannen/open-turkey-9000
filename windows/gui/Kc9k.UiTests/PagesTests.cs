// Schedules, Places, Hotkeys and Presets (§4.4, §4.5, §4.13) against the fake kc9k.
using FlaUI.Core.AutomationElements;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class PagesTests
{
    [Fact]
    public void SchedulesAreChangedOnTheirPage()
    {
        using var gui = new GuiApp("schedules");
        gui.Page("schedules");
        gui.Find("ScheduleCard_Weekend");
        gui.Shot("schedules");
        gui.Invoke("schedule_Preset_Evenings");
        gui.WaitFor(() => gui.Ran("schedule", "add", "Weekend", "--days", "daily", "--from", "18:00", "--to", "23:00", "--schedule"), "kc9k schedule add Weekend … --schedule");
        gui.Page("places");
        gui.Find("SavePlace");
        gui.Shot("places");
    }

    [Fact]
    public void AHotkeyThatStartsABlock()
    {
        using var gui = new GuiApp("hotkeys");
        gui.Page("hotkeys");
        gui.Find("Hotkeys");
        gui.Shot("hotkeys");
        gui.Invoke("AddStartHotkey");
        gui.Find("HotkeyKeys").AsTextBox().Text = "ctrl+alt+r";
        gui.Shot("hotkey-dialog");
        gui.Invoke(gui.FindButton("Add"));
        gui.WaitFor(() => gui.Ran("hotkey", "add", "ctrl+alt+r", "Evening"), "kc9k hotkey add ctrl+alt+r Evening");
    }

    [Fact]
    public void APresetIsUsedOnABlock()
    {
        using var gui = new GuiApp("presets");
        gui.Page("clusters");
        gui.Find("PresetUse_schedule_Weekend preset");
        gui.Shot("presets");
        gui.Invoke("PresetUse_schedule_Weekend preset");
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.Ran("preset", "use", "Weekend preset", "Evening", "--kind", "schedule"), "kc9k preset use");
    }
}
