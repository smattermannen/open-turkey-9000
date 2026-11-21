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
        Assert.Equal("3 blocks on · 2 locked", gui.Find("Headline").Name);
        Assert.NotNull(gui.ById("OnNow_Work"));
        Assert.NotNull(gui.ById("OnNow_Social media"));
        Assert.Null(gui.ById("OnNow_Games")); // off
        gui.Find("StartBlock").Click();
        var games = gui.FindName("Games");
        gui.Shot("start-menu");
        games.Click();
        gui.WaitFor(() => gui.Ran("start", "Games"), "kc9k start Games");
    }

    [Fact]
    public void MonkModeIsConfirmedAsALock()
    {
        using var gui = new GuiApp("monk");
        gui.Find("QuickMonk").Click();
        var box = gui.Find("AskDuration").AsTextBox();
        box.Text = "2h";
        gui.Shot("monk-length");
        Keyboard.Press(FlaUI.Core.WindowsAPI.VirtualKeyShort.RETURN);
        gui.FindName("Set this lock?");
        gui.Shot("monk-confirm");
        Assert.False(gui.Ran("monk"));
        gui.FindName("Lock").Click();
        gui.WaitFor(() => gui.Ran("monk", "--for", "120m", "--yes"), "kc9k monk --for 120m --yes");
    }

    [Fact]
    public void RefusalsAreShownInTheClisWords()
    {
        using var gui = new GuiApp("refusal", prepare: dir => File.WriteAllText(Path.Combine(dir, "responses.json"),
            """[{"args": ["start", "Games"], "ok": false, "output": "Error: refused by the fake for this test"}]"""));
        gui.Find("StartBlock").Click();
        gui.FindName("Games").Click();
        var message = gui.Find("MessageText");
        Assert.Contains("refused by the fake", message.Name);
        gui.Shot("refusal");
    }
}

[CollectionDefinition("gui", DisableParallelization = true)]
public class GuiCollection { }
