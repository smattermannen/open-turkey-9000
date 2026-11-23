// W52 (R-GUI-3): what goes wrong in the window is logged and said once;
// a crash leaves a note the next start shows. KC9K_GUI_TEST_THROW is the
// GUI's test-only hook (App.cs TestHook).
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class StabilityTests
{
    static Dictionary<string, string> Throw(string what) => new() { ["KC9K_GUI_TEST_THROW"] = what };

    [Fact]
    public void TheSameErrorTwiceIsOneDialog()
    {
        using var gui = new GuiApp("throw-window", env: Throw("window"));
        gui.FindName("Something went wrong", 20);
        Thread.Sleep(1500);
        gui.Invoke(gui.FindButton("OK"));
        Thread.Sleep(1500);
        Assert.Null(gui.DesktopByName("Something went wrong"));
        Assert.False(gui.App.HasExited);
    }

    [Fact]
    public void AnUnobservedTaskIsLoggedNotACrash()
    {
        using var gui = new GuiApp("throw-task", env: Throw("task"));
        Thread.Sleep(6000);
        Assert.False(gui.App.HasExited);
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    [Fact]
    public void ACrashIsShownOnceAtTheNextStart()
    {
        string dir;
        using (var crashing = new GuiApp("throw-crash", env: Throw("crash")))
        {
            dir = crashing.Dir;
            crashing.WaitFor(() => crashing.App.HasExited, "the crash", 90);
        }
        Assert.True(File.Exists(Path.Combine(dir, "gui-crash.txt")));
        using (var again = new GuiApp("after-crash", dir: dir))
        {
            again.FindName("Closed unexpectedly", 20);
            again.Shot("crash-note");
            again.Invoke(again.FindButton("OK"));
        }
        Assert.False(File.Exists(Path.Combine(dir, "gui-crash.txt")));
        using var third = new GuiApp("after-crash-2", dir: dir);
        Thread.Sleep(2000);
        Assert.Null(third.DesktopByName("Closed unexpectedly"));
    }

    // ROADMAP §4.3 (stability): a command kc9k is slow to answer never stops
    // the window: pages still change, the progress bar shows, and the state
    // is read again after it.
    [Fact]
    public void ASlowCommandNeverStopsTheWindow()
    {
        using var gui = new GuiApp("slow-command", prepare: dir => GuiApp.Slow(dir, 15000, "stop"));
        gui.Page("blocks");
        gui.Invoke("BlockSwitch_Evening");
        gui.WaitFor(() => gui.Ran("stop", "Evening"), "kc9k stop Evening started");
        var watch = System.Diagnostics.Stopwatch.StartNew();
        gui.Page("settings");
        gui.Find("Nav_dashboard", 3);
        gui.Page("dashboard");
        gui.Find("Headline", 3);
        Assert.True(watch.Elapsed < TimeSpan.FromSeconds(8), $"pages took {watch.Elapsed} while kc9k was slow");
        Assert.True(gui.ById("Busy") is { IsOffscreen: false }, "the progress bar shows while kc9k works");
        gui.Shot("busy");
        gui.WaitFor(() => gui.ById("Busy") is null or { IsOffscreen: true }, "the progress bar gone after the command", 40);
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    // Every read slow (a busy computer): the window opens, shows the state
    // and changes pages; reads never pile up behind each other.
    [Fact]
    public void SlowReadsDoNotPileUp()
    {
        using var gui = new GuiApp("slow-reads", prepare: dir =>
        {
            GuiApp.Slow(dir, 4000, "gui-state");
            GuiApp.Slow(dir, 4000, "stats");
        });
        foreach (var page in new[] { "blocks", "stats", "settings", "dashboard" })
        {
            var watch = System.Diagnostics.Stopwatch.StartNew();
            gui.Page(page);
            Assert.True(watch.Elapsed < TimeSpan.FromSeconds(4), $"{page} took {watch.Elapsed}");
        }
        int Reads() => gui.Commands().Count(c => c.Contains("gui-state"));
        var before = Reads();
        Thread.Sleep(15000);
        // A read every 5 s, each 4 s long: about 3 in 15 s, never piled up.
        Assert.True(Reads() - before <= 4, $"{Reads() - before} state reads in 15 s");
        Assert.False(gui.App.HasExited);
    }

    // kc9k.exe missing (removed, or a broken install): the window says so in
    // its banner, keeps running, and no dialog repeats every few seconds.
    [Fact]
    public void AMissingKc9kIsSaidOnceInTheBanner()
    {
        var missing = Path.Combine(Path.GetTempPath(), "no-kc9k-" + Guid.NewGuid().ToString("N")[..8], "kc9k.exe");
        using var gui = new GuiApp("no-kc9k", ready: "Banner", env: new Dictionary<string, string> { ["KC9K_CLI"] = missing });
        gui.WaitFor(() => (gui.Find("Banner").HelpText ?? "").Contains("could not be started"), "the banner saying kc9k could not be started", 30);
        gui.Shot("banner");
        foreach (var page in new[] { "blocks", "settings", "dashboard" }) gui.Page(page);
        Thread.Sleep(11000);
        Assert.False(gui.App.HasExited);
        Assert.Null(gui.DesktopByName("Something went wrong"));
        Assert.Null(gui.DesktopByName("Not done"));
    }

    // Two refusals at once: one dialog at a time, the second after the first
    // is closed, each in kc9k's words (WinUI allows one dialog per window).
    [Fact]
    public void TwoRefusalsAreShownOneAfterTheOther()
    {
        using var gui = new GuiApp("two-refusals", prepare: dir =>
        {
            GuiApp.Respond(dir, (["stop", "Evening"], false, "Error: first refusal"), (["start", "Reading"], false, "Error: second refusal"));
            GuiApp.Slow(dir, 2000, "stop");
            GuiApp.Slow(dir, 2000, "start");
        });
        gui.Page("blocks");
        gui.Invoke("BlockSwitch_Evening");
        gui.Invoke("BlockSwitch_Reading");
        gui.WaitFor(() => gui.Ran("start", "Reading") && gui.Ran("stop", "Evening"), "both commands");
        gui.FindName("Not done", 20);
        var text = gui.Find("MessageText").Name;
        gui.Shot("first-refusal");
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.ById("MessageText")?.Name is { } t && t != text, "the second refusal after the first", 20);
        Assert.Contains(new[] { text, gui.Find("MessageText").Name }, t => t.Contains("first refusal"));
        Assert.Contains(new[] { text, gui.Find("MessageText").Name }, t => t.Contains("second refusal"));
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.ById("MessageText") is null, "no dialog left");
    }
}
