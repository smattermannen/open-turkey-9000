// The GUI against the real kc9k.exe and its service (W4 with Core's W1), on
// windows-latest where the workflow installed it: KC9K_REAL_EXE names the
// installed kc9k.exe. Skipped elsewhere. The runner is a disposable
// administrator's machine; the test makes a block, starts it and stops it.
using FlaUI.Core.AutomationElements;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class RealTests
{
    static bool Ready => Environment.GetEnvironmentVariable("KC9K_REAL_EXE") is { Length: > 0 } p && File.Exists(p);

    [Fact]
    public void ABlockMadeStartedAndStoppedThroughTheService()
    {
        if (!Ready) return; // only where the workflow installed KC9K
        using var gui = new GuiApp("real", real: true);
        gui.Shot("overview");
        gui.Page("blocks");
        gui.Invoke("NewBlock");
        gui.Find("AskText").AsTextBox().Text = "CI block";
        gui.Invoke(gui.FindButton("Create"));
        gui.Find("Tab_websites", 60);
        gui.Shot("new-block");
        gui.TypeEnter("List_Blocked_Entries_Entry", "example.com");
        gui.Find("SaveEdits", 60);
        gui.Invoke("SaveEdits");
        gui.Invoke("BackToBlocks");
        gui.Invoke(gui.Find("BlockSwitch_CI block", 60));
        gui.WaitFor(() => gui.Find("BlockSwitch_CI block").Patterns.Toggle.Pattern.ToggleState.Value == FlaUI.Core.Definitions.ToggleState.On, "the block on", 60);
        gui.Shot("block-on");
        gui.Invoke("BlockSwitch_CI block");
        gui.WaitFor(() => gui.Find("BlockSwitch_CI block").Patterns.Toggle.Pattern.ToggleState.Value == FlaUI.Core.Definitions.ToggleState.Off, "the block off", 60);
        foreach (var page in new[] { "schedules", "places", "hotkeys", "clusters", "stats", "circadian", "settings" })
        {
            gui.Page(page);
            gui.Shot(page);
        }
    }
}
