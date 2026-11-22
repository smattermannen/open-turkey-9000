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
}
