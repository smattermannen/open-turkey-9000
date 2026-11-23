// W4 on windows-latest: what a person does with the pointer and Windows
// itself, as far as a runner can: drag a block onto a mode, draw a window on
// the week, the notification area's menu, Open at sign-in, and the text
// lock's challenge window opened from the GUI (real kc9k.exe and service).
using System.Runtime.InteropServices;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Input;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class DesktopTests
{
    static void Drag(System.Drawing.Point from, System.Drawing.Point to) => GuiApp.Drag(from, to);

    static System.Drawing.Point Center(AutomationElement e)
    {
        var r = e.BoundingRectangle;
        return new(r.Left + r.Width / 2, r.Top + r.Height / 2);
    }

    // gui_modes_drag (§4.2): a block card dragged onto a mode card asks, then
    // puts it into the mode.
    [Fact]
    public void ABlockIsDraggedOntoAMode()
    {
        using var gui = new GuiApp("drag-mode");
        gui.Page("blocks");
        gui.Show("ModeCard_Study");
        var block = gui.Show("BlockCard_Reading");
        var mode = gui.Find("ModeCard_Study");
        Assert.False(mode.BoundingRectangle.IsEmpty, "the mode card on screen with the block");
        // From the card's empty part right of its name (the name is a link).
        var r = block.BoundingRectangle;
        gui.Front();
        Drag(new(r.Right + 120, r.Top + r.Height / 2), Center(mode));
        gui.FindName("Put Reading into Study?", 20);
        gui.Shot("dropped");
        gui.Invoke(gui.FindButton("Put it in"));
        gui.WaitFor(() => gui.Ran("mode", "add", "Study", "Reading"), "kc9k mode add Study Reading");
    }

    // §4.5: drag down a day of the week to draw a window (15-minute grid),
    // asked, then added; against the fake and the real kc9k.exe.
    [Theory, InlineData(false), InlineData(true)]
    public void AWindowIsDrawnOnTheWeek(bool real)
    {
        using var gui = new GuiApp("week" + (real ? "-real" : ""), scratch: real, setup: [["block", "create", "Reading"]]);
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.Invoke("BlockAutomation");
        AutomationElement Label(string text, Func<System.Drawing.Rectangle, bool>? where = null) =>
            FlaUI.Core.Tools.Retry.WhileNull(() => gui.Window.FindAllDescendants(c => c.ByName(text)).FirstOrDefault(e => where?.Invoke(e.BoundingRectangle) ?? true),
                TimeSpan.FromSeconds(20), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: "no label " + text).Result!;
        var mon = Label("Mon");
        var tue = Label("Tue", r => Math.Abs(r.Top - mon.BoundingRectangle.Top) < 4);
        GuiApp.Show(mon);
        mon = Label("Mon");
        // Hour labels sit 8 DIP above their line (CI: 100 %).
        // The week's own hour labels: left of Mon, below it.
        var six = Label("06:00", r => r.Top > mon.BoundingRectangle.Top && r.Right < mon.BoundingRectangle.Left).BoundingRectangle;
        var noon = Label("12:00", r => r.Top > mon.BoundingRectangle.Top && r.Right < mon.BoundingRectangle.Left).BoundingRectangle;
        double perMinute = (noon.Top - six.Top) / 360.0, sixY = six.Top + 8;
        int Y(int minute) => (int)Math.Round(sixY + (minute - 360) * perMinute);
        var x = Center(mon).X;
        Assert.True(Center(tue).X > x, "Tue right of Mon");
        gui.Front();
        Drag(new(x, Y(9 * 60 + 2)), new(x, Y(12 * 60 + 2)));
        gui.FindName("Add Mon 09:00–12:00?", 20);
        gui.Shot("drawn");
        gui.Invoke(gui.FindButton("Add"));
        gui.WaitFor(() => gui.RanWith("mon", "09:00", "12:00"), "the window added: mon 09:00–12:00");
        if (real)
            Assert.Contains(gui.RealBlock("Reading")["schedule_windows"]!.AsArray(), w => (int?)w!["start"] == 540 && (int?)w["end"] == 720);
    }

    [DllImport("user32.dll")] static extern bool PostMessage(IntPtr hwnd, uint msg, IntPtr wParam, IntPtr lParam);

    // W35 (§4.10 tray): the notification area's menu, as Windows opens it
    // (its callback message, NOTIFYICON_VERSION_4); Start a block › Reading
    // runs kc9k start Reading; Stop and Unlock are never in it.
    [Fact]
    public void TheTrayMenuOnlyStrengthens()
    {
        using var gui = new GuiApp("tray");
        const uint WmAppTray = 0x8000 + 17, WmContextMenu = 0x007B;
        PostMessage(gui.Window.Properties.NativeWindowHandle.Value, WmAppTray, IntPtr.Zero, (IntPtr)((1 << 16) | WmContextMenu));
        var start = gui.FindMenuItem("Start a block", 20);
        gui.Shot("tray-menu");
        foreach (var never in new[] { "Stop", "Unlock…", "Take a break" })
            Assert.Null(gui.Automation.GetDesktop().FindFirstDescendant(c => c.ByControlType(ControlType.MenuItem).And(c.ByName(never)).And(c.ByProcessId(gui.App.ProcessId))));
        gui.FindMenuItem("Quit (blocks stay on)");
        start.Patterns.ExpandCollapse.Pattern.Expand();
        var reading = gui.FindMenuItem("Reading");
        reading.Patterns.Invoke.Pattern.Invoke();
        gui.WaitFor(() => gui.Ran("start", "Reading"), "kc9k start Reading from the tray");
        Assert.False(gui.App.HasExited);
    }

    // §4.10 Open at sign-in: a shortcut in the Startup folder that starts the
    // GUI in the notification area (--hidden); switched off, it is gone.
    [Fact]
    public void OpenAtSignInIsAStartupShortcut()
    {
        var link = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Startup), "Killer Chicken 9000.lnk");
        var had = File.Exists(link);
        var keep = had ? File.ReadAllBytes(link) : null;
        try
        {
            if (had) File.Delete(link);
            using var gui = new GuiApp("startup");
            gui.Page("settings");
            Assert.Equal(ToggleState.Off, gui.Find("PrefLogin").Patterns.Toggle.Pattern.ToggleState.Value);
            gui.Invoke("PrefLogin");
            gui.WaitFor(() => File.Exists(link), "the shortcut in the Startup folder");
            dynamic shell = Activator.CreateInstance(Type.GetTypeFromProgID("WScript.Shell")!)!;
            var lnk = shell.CreateShortcut(link);
            Assert.Equal(Path.GetFullPath(Environment.GetEnvironmentVariable("KC9K_GUI_EXE")!), Path.GetFullPath((string)lnk.TargetPath));
            Assert.Equal("--hidden", (string)lnk.Arguments);
            gui.Invoke("PrefLogin");
            gui.WaitFor(() => !File.Exists(link), "the shortcut gone");
        }
        finally
        {
            if (keep is not null) File.WriteAllBytes(link, keep);
        }
    }

    // W4, R-LOCK-5: Unlock… on a text-locked block opens kc9k's challenge
    // window (its own, from the service's agent, never in the GUI); closing
    // it unlocks nothing and the window keeps working. Real kc9k.exe and
    // service only (the workflow installs them).
    [Fact]
    public void TheChallengeWindowOpensFromTheGui()
    {
        var exe = Environment.GetEnvironmentVariable("KC9K_REAL_EXE");
        Assert.True(exe is { Length: > 0 } && File.Exists(exe), "KC9K_REAL_EXE (the installed kc9k.exe) is not set");
        (bool, string) Kc9k(params string[] args)
        {
            var info = new System.Diagnostics.ProcessStartInfo(exe!) { UseShellExecute = false, RedirectStandardOutput = true, RedirectStandardError = true, RedirectStandardInput = true };
            foreach (var a in args) info.ArgumentList.Add(a);
            using var p = System.Diagnostics.Process.Start(info)!;
            p.StandardInput.Close();
            var o = p.StandardOutput.ReadToEndAsync();
            var e = p.StandardError.ReadToEndAsync();
            p.WaitForExit(120000);
            return (p.ExitCode == 0, o.Result + e.Result);
        }
        foreach (var args in new[] { new[] { "block", "create", "CI text" }, ["block", "add", "CI text", "example.org"], ["start", "CI text", "--lock", "--lock-chars", "5", "--yes"] })
        {
            var (ok, output) = Kc9k(args);
            Assert.True(ok, $"kc9k {string.Join(' ', args)}: {output}");
        }
        using var gui = new GuiApp("challenge", real: true);
        var later = FlaUI.Core.Tools.Retry.WhileNull(() => gui.FindButtonOrNull("Later"), TimeSpan.FromSeconds(8), TimeSpan.FromMilliseconds(250)).Result;
        if (later is not null) gui.Invoke(later);
        gui.Page("blocks");
        gui.Invoke(gui.Find("BlockCard_CI text", 60));
        gui.Invoke(gui.Find("BlockUnlock", 30));
        var challenge = FlaUI.Core.Tools.Retry.WhileNull(() => gui.Automation.GetDesktop().FindFirstChild(c => c.ByClassName("KC9KChallenge")),
            TimeSpan.FromSeconds(90), TimeSpan.FromMilliseconds(500), throwOnTimeout: true, timeoutMessage: "no challenge window").Result!;
        Assert.NotEqual(gui.App.ProcessId, challenge.Properties.ProcessId.Value);
        try { FlaUI.Core.Capturing.Capture.Element(challenge).ToFile(Path.Combine(Environment.GetEnvironmentVariable("KC9K_SCREENSHOTS") ?? Path.GetTempPath(), "challenge-window.png")); }
        catch (Exception e) { Console.WriteLine("screenshot: " + e.Message); }
        // While it is open, the GUI still changes pages.
        gui.Page("settings");
        gui.Find("PrefLogin", 10);
        challenge.AsWindow().Close();
        gui.WaitFor(() => gui.Automation.GetDesktop().FindFirstChild(c => c.ByClassName("KC9KChallenge")) is null, "the challenge window closed", 30);
        // kc9k's words: cancelled, still locked.
        gui.FindName("Not done", 60);
        Assert.Contains("remains active and locked", gui.Find("MessageText").Name);
        gui.Invoke(gui.FindButton("OK"));
        gui.Cards();
        gui.WaitFor(() => gui.Find("BlockSwitch_CI text").IsEnabled == false, "the block still locked", 60);
        Assert.False(gui.App.HasExited);
    }
}
