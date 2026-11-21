// Clicks through every page against the fake kc9k (W4): each page opens,
// shows the fixtures' state, and the commands it runs are the CLI's.
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Input;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class SmokeTests
{
    static readonly string[] Pages = ["dashboard", "blocks", "schedules", "places", "hotkeys", "clusters", "stats", "circadian", "settings"];

    [Theory]
    [InlineData("light")]
    [InlineData("dark")]
    public void EveryPageOpens(string theme)
    {
        using var gui = new GuiApp("pages-" + theme, theme);
        foreach (var page in Pages)
        {
            gui.Page(page);
            gui.Shot(page);
        }
        Assert.True(gui.Ran("gui-state"));
        Assert.True(gui.Ran("stats", "--json"));
    }

    [Fact]
    public void OverviewShowsWhatIsOn()
    {
        using var gui = new GuiApp("overview");
        gui.WaitFor(() => gui.Find("Headline").Name.Contains("on"), "the headline");
        Assert.Equal("4 blocks on · 2 locked", gui.Find("Headline").Name);
        Assert.NotNull(gui.ById("OnNow_Work"));
        Assert.NotNull(gui.ById("OnNow_Social media"));
        Assert.Null(gui.ById("OnNow_Reading")); // off
        gui.Invoke("StartBlock");
        var reading = gui.FindMenuItem("Reading");
        gui.Shot("start-menu");
        reading.AsMenuItem().Invoke();
        gui.WaitFor(() => gui.Ran("start", "Reading"), "kc9k start Reading");
    }

    [Fact]
    public void MonkModeIsConfirmedAsALock()
    {
        using var gui = new GuiApp("monk");
        gui.Invoke("QuickMonk");
        var box = gui.Find("AskDuration").AsTextBox();
        box.Text = "2h";
        gui.Shot("monk-length");
        Keyboard.Press(FlaUI.Core.WindowsAPI.VirtualKeyShort.RETURN);
        gui.FindName("Set this lock?");
        gui.Shot("monk-confirm");
        Assert.False(gui.Ran("monk"));
        gui.Invoke(gui.FindButton("Lock"));
        gui.WaitFor(() => gui.Ran("monk", "--for", "120m", "--yes"), "kc9k monk --for 120m --yes");
    }

    [Fact]
    public void RefusalsAreShownInTheClisWords()
    {
        using var gui = new GuiApp("refusal", prepare: dir => File.WriteAllText(Path.Combine(dir, "responses.json"),
            """[{"args": ["start", "Reading"], "ok": false, "output": "Error: refused by the fake for this test"}]"""));
        gui.Invoke("StartBlock");
        gui.FindMenuItem("Reading").AsMenuItem().Invoke();
        var message = gui.Find("MessageText");
        Assert.Contains("refused by the fake", message.Name);
        gui.Shot("refusal");
    }
}

[CollectionDefinition("gui", DisableParallelization = true)]
public class GuiCollection { }
