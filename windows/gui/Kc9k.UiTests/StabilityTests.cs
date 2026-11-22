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
}
