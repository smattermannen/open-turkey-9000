// The Blocks page (§4.2, §4.3, §4.5, §4.6) against the fake kc9k: cards,
// an opened block's tabs and its draft, locks with their confirmation,
// modes, a block's automation.
using FlaUI.Core.AutomationElements;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class BlocksTests
{
    [Fact]
    public void CardsOpenABlockAndEditsGoToItsDraft()
    {
        using var gui = new GuiApp("blocks");
        gui.Page("blocks");
        gui.Find("BlockCard_Reading");
        gui.Find("ModeCard_Study");
        gui.Shot("cards");
        gui.Invoke("BlockCard_Reading");
        gui.Find("Tab_websites");
        gui.Shot("websites");
        gui.TypeEnter("List_Blocked_Entries_Entry", "example.com");
        gui.WaitFor(() => gui.Ran("list", "add", "Reading (draft)", "Blocked", "example.com"), "the entry added to the draft");
        Assert.True(gui.Ran("block", "draft", "Reading"));
        foreach (var tab in new[] { "apps", "breaks", "bypass" })
        {
            gui.Invoke("Tab_" + tab);
            gui.Shot(tab);
        }
    }

    [Fact]
    public void ACardsSwitchStartsTheBlock()
    {
        using var gui = new GuiApp("switch");
        gui.Page("blocks");
        gui.Invoke("BlockSwitch_Reading");
        gui.WaitFor(() => gui.Ran("start", "Reading"), "kc9k start Reading");
    }

    [Fact]
    public void LocksAreConfirmedBeforeYes()
    {
        using var gui = new GuiApp("locks");
        gui.Page("blocks");
        gui.Invoke("BlockCard_Social media");
        gui.Invoke("BlockStart"); // on already: Add locks…
        gui.Invoke("AddLocks_Timer_For");
        var length = gui.Find("AddLocks_Timer_Length").AsTextBox();
        length.Text = "2h";
        gui.Shot("lock-dialog");
        gui.Invoke(gui.FindButton("Add locks"));
        gui.FindName("Set this lock?");
        gui.Shot("lock-confirm");
        Assert.False(gui.Ran("start", "Social media", "--for"));
        gui.Invoke(gui.FindButton("Lock"));
        gui.WaitFor(() => gui.Ran("start", "Social media", "--for", "120m", "--yes"), "kc9k start Social media --for 120m --yes");
    }

    [Fact]
    public void ModesAndAutomation()
    {
        using var gui = new GuiApp("modes");
        gui.Page("blocks");
        gui.Invoke("ModeCard_Study");
        gui.Find("ModeAddBlocks");
        gui.Shot("mode");
        gui.Invoke("ModeBack");
        gui.Invoke("ModeCard_Frozen Chicken");
        gui.Find("FreezeStart");
        gui.Shot("frozen-chicken");
        gui.Invoke("ModeBack");
        gui.Invoke("ModeCard_Monk mode");
        gui.Find("MonkRunNow");
        gui.Shot("monk-mode");
        gui.Invoke("ModeBack");
        gui.Invoke("BlockAuto_Evening");
        gui.Find("WeekCalendar");
        gui.Shot("automation");
        gui.Invoke("block_Preset_Evenings");
        gui.WaitFor(() => gui.Ran("schedule", "add", "Evening", "--days", "daily", "--from", "18:00", "--to", "23:00"), "kc9k schedule add Evening");
    }
}
