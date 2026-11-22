// W30–W49 (ROADMAP §4.1): what the Windows GUI adds or fixes beyond the
// GTK GUI's pages, against the fake kc9k.
using System.Text.Json.Nodes;
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
}
