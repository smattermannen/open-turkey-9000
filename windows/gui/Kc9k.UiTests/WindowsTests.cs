// W30–W49 (ROADMAP §4.1): what the Windows GUI adds or fixes beyond the
// GTK GUI's pages, against the fake kc9k.
using System.Text.Json.Nodes;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class WindowsTests
{
    // W30: Edge found but not a supported browser yet is explained, with no
    // Install button; LibreWolf's Install shows where to get it as information.
    [Fact]
    public void BrowsersWaitingForTheStoreAreExplained()
    {
        using var gui = new GuiApp("browsers", prepare: dir =>
        {
            GuiApp.EditState(dir, s => s["browsers"] = new JsonArray(
                new JsonObject { ["id"] = "librewolf", ["name"] = "LibreWolf", ["installed"] = false, ["found"] = false, ["checkin_seconds"] = -1 },
                new JsonObject { ["id"] = "edge", ["name"] = "Microsoft Edge", ["installed"] = false, ["found"] = true, ["checkin_seconds"] = -1 }));
            GuiApp.Respond(dir, (["browsers", "install", "librewolf"], false,
                "Error: install LibreWolf from librewolf.net, or in a console: winget install LibreWolf.LibreWolf"));
        });
        gui.Page("settings");
        gui.Find("InstallBrowser_librewolf");
        gui.FindName(Kc9k.Core.Words.BrowserStatus(new() { Id = "edge", Found = true }));
        Assert.Null(gui.ById("InstallBrowser_edge"));
        gui.Shot("browsers");
        gui.Invoke("InstallBrowser_librewolf");
        gui.Invoke(gui.FindButton("Continue"));
        gui.WaitFor(() => gui.Ran("browsers", "install", "librewolf"), "kc9k browsers install librewolf");
        gui.FindName("Getting LibreWolf");
        gui.Shot("browser-install");
        gui.Invoke(gui.FindButton("OK"));
    }

    // W31 (§4.1): break status and ladder positions on the Overview.
    [Fact]
    public void OverviewShowsBreaksAndLadderPositions()
    {
        using var gui = new GuiApp("ladders", prepare: dir => GuiApp.EditState(dir, s =>
        {
            s["nepsis_level"] = 0;
            s["nepsis_count"] = 2;
            foreach (var b in s["blocks"]!.AsArray())
            {
                if ((string?)b!["name"] == "Games") b["paused_seconds"] = 600;
                if ((string?)b!["name"] != "Work") continue;
                b["nepsis"] = JsonNode.Parse("""{"steps": [{"count": 3, "minutes": 10, "actions": [{"action": "warning"}]}, {"count": 2, "minutes": 30, "actions": [{"action": "close-browser"}]}]}""");
                b["nepsis_level"] = 1;
                b["nepsis_count"] = 1;
            }
        }));
        gui.Page("dashboard");
        gui.WaitFor(() => gui.Find("DeviceLine").Name.Contains("global bypass prevention: 2 of 5 in 10 min"), "the global ladder's position");
        gui.WaitFor(() => gui.Find("OnNow_Work").FindAllDescendants().Any(e => e.Name.Contains("bypass prevention step 2 of 2: 1 of 2 in 30 min")),
            "Work's ladder position");
        gui.WaitFor(() => gui.Find("OnNow_Games").FindAllDescendants().Any(e => e.Name.Contains("on a break for 10 min")), "Games' break");
        gui.Shot("overview");
    }

    // W34: the window's shortcuts.
    [Fact]
    public void KeyboardShortcuts()
    {
        using var gui = new GuiApp("keys");
        gui.Window.Focus();
        Keyboard.TypeSimultaneously(VirtualKeyShort.CONTROL, VirtualKeyShort.KEY_3);
        gui.Find("ScheduleCard_Weekend");
        Keyboard.TypeSimultaneously(VirtualKeyShort.CONTROL, VirtualKeyShort.KEY_2);
        gui.Invoke("BlockCard_Reading");
        gui.Find("Tab_websites");
        gui.Window.Focus();
        Keyboard.TypeSimultaneously(VirtualKeyShort.ALT, VirtualKeyShort.LEFT);
        gui.WaitFor(() => gui.ById("Tab_websites") is null || gui.ById("Tab_websites")!.IsOffscreen, "back to the cards");
        Keyboard.TypeSimultaneously(VirtualKeyShort.CONTROL, VirtualKeyShort.KEY_N);
        gui.FindName("New block");
        gui.Shot("new-block");
        gui.Invoke(gui.FindButton("Cancel"));
        Keyboard.TypeSimultaneously(VirtualKeyShort.CONTROL, VirtualKeyShort.KEY_9);
        gui.FindName("Ctrl+1 … Ctrl+9");
        gui.Shot("keyboard");
    }

    // W37: the window opens where it was, as large, on the page open last,
    // and keeps that when it closes.
    [Fact]
    public void WindowStateIsKept()
    {
        string? prefs = null;
        using (var gui = new GuiApp("window", prepare: dir =>
               {
                   prefs = Path.Combine(dir, "gui.json");
                   File.WriteAllText(prefs, """{"theme": "light", "tray": false, "window_width": 800, "window_height": 600, "window_x": 60, "window_y": 50, "open_on": "last", "last_page": "stats"}""");
               }, ready: "StatsTotal"))
        {
            gui.Find("StatsTotal");
            var r = gui.Window.BoundingRectangle;
            Assert.InRange(r.X, 40, 80);
            Assert.InRange(r.Y, 30, 70);
            Assert.InRange(r.Width, 780, 820);
            gui.Page("hotkeys");
            gui.Window.Move(120, 90);
            Thread.Sleep(300);
        }
        Thread.Sleep(1000);
        var kept = JsonNode.Parse(File.ReadAllText(prefs!))!;
        Assert.Equal("hotkeys", (string?)kept["last_page"]);
        Assert.InRange((int)kept["window_x"]!, 100, 140);
    }

    // A place on a screen that is gone: the window comes back on the primary screen.
    [Fact]
    public void AWindowOffEveryScreenComesBack()
    {
        using var gui = new GuiApp("offscreen", prepare: dir => File.WriteAllText(Path.Combine(dir, "gui.json"),
            """{"theme": "light", "tray": false, "window_width": 1000, "window_height": 700, "window_x": 30000, "window_y": 30000}"""));
        var r = gui.Window.BoundingRectangle;
        Assert.InRange(r.X, 0, 4000);
        Assert.InRange(r.Y, 0, 3000);
    }

    // W38: someone with nothing set up sees what is left, each step opening its place.
    [Fact]
    public void AFirstViewForNothingSetUp()
    {
        using var gui = new GuiApp("first", prepare: dir => GuiApp.EditState(dir, s =>
        {
            s["blocks"] = new JsonArray();
            s["modes"] = new JsonArray();
            foreach (var b in s["browsers"]!.AsArray()) b!["checkin_seconds"] = -1;
        }));
        gui.Find("FirstStepsTitle");
        gui.Shot("first-view");
        gui.Invoke("First_block");
        gui.FindName("New block");
        gui.Invoke(gui.FindButton("Cancel"));
        gui.Invoke("First_browser");
        gui.Find("InstallBrowser_librewolf");
    }

    [Fact]
    public void NoFirstViewOnceSetUp()
    {
        using var gui = new GuiApp("set-up", prepare: dir => GuiApp.EditState(dir, s =>
        {
            foreach (var b in s["browsers"]!.AsArray()) if ((bool)b!["installed"]!) b["checkin_seconds"] = 5;
        }));
        gui.Find("Headline");
        Thread.Sleep(500);
        Assert.True(gui.ById("FirstStepsTitle") is null || gui.ById("FirstStepsTitle")!.IsOffscreen);
    }

    // W39: device protection says what holds on Windows, and the firmware lock's state.
    [Fact]
    public void DeviceProtectionExplainedForWindows()
    {
        using var gui = new GuiApp("device");
        gui.Page("settings");
        gui.WaitFor(() => gui.Find("ProtectionNow").Name.StartsWith("On now: basic"), "the level on now");
        gui.WaitFor(() => gui.Find("FirmwareStatus").Name.StartsWith("Firmware lock: not available from Windows"), "the firmware lock's state");
        Assert.True(gui.Ran("firmware-lock", "--json"));
        gui.Find("FirmwareStatus").Focus();
        gui.Shot("device-protection");
    }

    // W43: the versions under Settings › About.
    [Fact]
    public void AboutShowsTheVersions()
    {
        using var gui = new GuiApp("about");
        gui.Page("settings");
        gui.WaitFor(() => gui.Find("AboutCli").Name == "Service and command line: kc9k fake (windows/amd64)", "kc9k's version");
        Assert.StartsWith("This window: ", gui.Find("AboutGui").Name);
        Assert.True(gui.Ran("version"));
    }

    // W36: the jump list's arguments open their page; a second start hands
    // its arguments to the running window instead of opening another.
    [Fact]
    public void StartArgumentsAndASecondStart()
    {
        using var gui = new GuiApp("start-args", arguments: "--page stats", ready: "StatsTotal", multi: false);
        gui.Find("StatsTotal");
        var second = new System.Diagnostics.ProcessStartInfo(gui.StartInfo.FileName, "--page settings") { UseShellExecute = false };
        foreach (var (k, v) in gui.StartInfo.Environment) second.Environment[k] = v;
        using var p = System.Diagnostics.Process.Start(second)!;
        Assert.True(p.WaitForExit(30000), "the second start ended");
        gui.Find("ProtectionNow");
        Assert.Single(System.Diagnostics.Process.GetProcessesByName("kc9k-gui"));
        gui.Shot("second-start");
    }

    // W49 (Core W15): Windows' guided setup steps; the account steps open
    // Windows' own settings, Safe Mode is information only.
    [Fact]
    public void WindowsGuidedSetupSteps()
    {
        using var gui = new GuiApp("guided-windows", prepare: dir => File.WriteAllText(Path.Combine(dir, "guided-setup.json"), """
            [{"id": "settings-password", "title": "Settings password", "state": "done", "detail": "", "command": "kc9k password set"},
             {"id": "other-admins", "title": "Other administrator accounts", "state": "todo", "detail": "Make them standard accounts.", "command": ""},
             {"id": "builtin-admin", "title": "The built-in Administrator", "state": "todo", "detail": "Keep it off.", "command": ""},
             {"id": "safe-mode", "title": "Safe Mode", "state": "info", "detail": "The service runs there too.", "command": ""},
             {"id": "no-escape", "title": "No-escape", "state": "todo", "detail": "Last.", "command": "kc9k no-escape on"}]
            """));
        gui.Page("settings");
        gui.Invoke("GuidedSetup");
        gui.Find("Guided_other-admins");
        gui.Find("Guided_builtin-admin");
        gui.Find("Guided_no-escape");
        Assert.Null(gui.ById("Guided_safe-mode"));
        Assert.Null(gui.ById("Guided_settings-password"));
        gui.Shot("guided-windows");
        gui.Invoke("CloseButton"); // the dialog's, not the title bar's
    }
}
