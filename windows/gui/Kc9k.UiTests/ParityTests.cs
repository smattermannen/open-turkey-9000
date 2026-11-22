// W32: UI Automation counterparts of the GTK GUI's smoke tests (ci/gui_*.py),
// against the fake kc9k: what each checks on Linux, checked here.
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class ParityTests
{
    // gui_cards: a card's switch stops its block; a refusal is shown and the
    // switch put back; a locked block's switch is greyed out (R-GUI-3).
    [Fact]
    public void ACardsSwitchStopsAndARefusalIsPutBack()
    {
        using var gui = new GuiApp("cards-stop", prepare: dir => GuiApp.Respond(dir,
            (["stop", "Evening"], false, "Error: 'Evening' needs its block password")));
        gui.Page("blocks");
        Assert.False(gui.Find("BlockSwitch_Social media").IsEnabled);
        gui.Invoke("BlockSwitch_Evening");
        gui.WaitFor(() => gui.Ran("stop", "Evening"), "kc9k stop Evening");
        gui.FindName("Not done");
        gui.Shot("refused");
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.Find("BlockSwitch_Evening").Patterns.Toggle.Pattern.ToggleState.Value == ToggleState.On, "the switch put back on");
    }

    // gui_extend and gui_duration: Extend timer… on a timer-locked block; a
    // duration that can't be read says why, keeps what was typed and can't
    // be used (R-GUI-1); the extension is confirmed (R-LOCK-12).
    [Fact]
    public void ExtendATimerLockWithADurationThatSaysWhy()
    {
        using var gui = new GuiApp("extend");
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.WaitFor(() => !gui.Find("BlockExtend").IsEnabled, "Extend timer… greyed out without a timer lock");
        gui.Invoke("BackToBlocks");
        gui.Invoke("BlockCard_Social media");
        gui.WaitFor(() => gui.Find("BlockExtend").IsEnabled, "Extend timer… on a timer-locked block");
        gui.Invoke("BlockExtend");
        var length = gui.Find("Extend_Length").AsTextBox();
        length.Text = "5 parsecs";
        gui.WaitFor(() => !gui.FindButton("Extend").IsEnabled, "Extend greyed out for a length it can't read");
        Assert.Equal("5 parsecs", length.Text);
        Assert.False(string.IsNullOrEmpty(length.HelpText));
        gui.Shot("duration-problem");
        length.Text = "1d 2h";
        gui.WaitFor(() => gui.FindButton("Extend").IsEnabled, "Extend for 1d 2h");
        gui.Invoke(gui.FindButton("Extend"));
        gui.FindName("Set this lock?");
        Assert.False(gui.Ran("extend"));
        gui.Invoke(gui.FindButton("Lock"));
        gui.WaitFor(() => gui.Ran("extend", "Social media", "--for", "1560m", "--yes"), "kc9k extend Social media --for 1560m --yes");
    }

    // gui_lists: a new list asks for its name and where it looks (URL, title, text).
    [Fact]
    public void ANewListAsksWhereItLooks()
    {
        using var gui = new GuiApp("new-list");
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.Invoke("New_blacklist");
        gui.Find("NewListName").AsTextBox().Text = "Shops";
        gui.Find("NewListWhere_url");
        gui.Find("NewListWhere_text");
        gui.Invoke("NewListWhere_title");
        gui.Shot("new-list");
        gui.Invoke(gui.FindButton("Create"));
        gui.WaitFor(() => gui.RanWith("list", "create", "Shops", "--url", "--title"), "kc9k list create … Shops --url --title");
    }

    // gui_presets: a block's lists saved as a list preset (blacklists only).
    [Fact]
    public void ListsAreSavedAsAPreset()
    {
        using var gui = new GuiApp("save-lists");
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.Invoke("SaveListsPreset");
        gui.Find("PresetName").AsTextBox().Text = "Study set";
        gui.Invoke(gui.FindName("Blacklists"));
        gui.Invoke(gui.FindButton("Save"));
        gui.WaitFor(() => gui.RanWith("preset", "save", "lists", "Study set", "--blacklists"), "kc9k preset save lists … --blacklists");
    }

    // gui_modes: a block put into a mode by its card's menu (the keyboard's
    // way to drag it), after a question.
    [Fact]
    public void ABlockIsPutIntoAModeByItsMenu()
    {
        using var gui = new GuiApp("into-mode");
        gui.Page("blocks");
        gui.Find("BlockCard_Reading").Focus();
        Keyboard.TypeSimultaneously(VirtualKeyShort.SHIFT, VirtualKeyShort.F10);
        gui.Invoke(gui.FindMenuItem("Put into Study"));
        gui.FindName("Put Reading into Study?");
        gui.Invoke(gui.FindButton("Put it in"));
        gui.WaitFor(() => gui.Ran("mode", "add", "Study", "Reading"), "kc9k mode add Study Reading");
    }

    // gui_click_everything: every button on every page and in a block's tabs
    // pressed once, each question answered Cancel (or closed); the window
    // never shows "Something went wrong" and never ends.
    [Fact]
    public void EveryButtonOnce()
    {
        using var gui = new GuiApp("click-everything");
        var skip = new[] { "Close", "Minimize", "Maximize", "Import", "Export", "Back up", "Share", "picture", "Picture", "Save statistics", "Install", "Lock: forget" };
        var pressed = 0;
        void PressAll(string page, Action open)
        {
            open();
            var ids = gui.Window.FindAllDescendants(c => c.ByControlType(ControlType.Button))
                .Select(e => { try { return (Id: e.Properties.AutomationId.ValueOrDefault ?? "", Name: e.Name ?? ""); } catch { return (Id: "", Name: ""); } })
                .Where(x => x.Id.Length > 0 && !x.Id.StartsWith("Nav_") && !skip.Any(s => x.Name.Contains(s)))
                .Select(x => x.Id).Distinct().ToList();
            foreach (var id in ids)
            {
                var e = gui.ById(id);
                if (e is null) { open(); e = gui.ById(id); }
                if (e is null) continue;
                try
                {
                    if (e.IsOffscreen || !e.IsEnabled) continue;
                    gui.Invoke(e);
                    pressed++;
                }
                catch (Exception) { continue; }
                Thread.Sleep(250);
                for (var i = 0; i < 4; i++)
                {
                    // A dialog's own button (never the title bar's Close).
                    var close = gui.ById("CloseButton") ?? gui.FindButtonOrNull("Cancel");
                    if (close is null) break;
                    try { gui.Invoke(close); } catch (Exception) { }
                    Thread.Sleep(200);
                }
                Keyboard.Press(VirtualKeyShort.ESCAPE);
                Assert.False(gui.App.HasExited, $"the window ended after {page}: {id}");
                Assert.Null(gui.DesktopByName("Something went wrong"));
            }
        }
        foreach (var page in new[] { "dashboard", "blocks", "schedules", "places", "hotkeys", "clusters", "stats", "circadian", "settings" })
            PressAll(page, () => gui.Page(page));
        foreach (var tab in new[] { "websites", "apps", "breaks", "bypass" })
            PressAll("block " + tab, () =>
            {
                gui.Page("blocks");
                if (gui.ById("Tab_" + tab) is null) gui.Invoke("BlockCard_Reading");
                gui.Invoke("Tab_" + tab);
            });
        Console.WriteLine($"pressed {pressed} buttons");
        Assert.True(pressed > 50, $"only {pressed} buttons pressed");
        Assert.False(gui.App.HasExited);
    }

    // gui_freeze_lock: a scheduled freeze picked in its list is locked with
    // a timer lock, confirmed before --yes (R-LOCK-12).
    [Fact]
    public void AFreezeScheduleIsLocked()
    {
        using var gui = new GuiApp("freeze-lock", prepare: dir => GuiApp.EditState(dir, s =>
        {
            s["freeze_schedules"] = new System.Text.Json.Nodes.JsonArray("daily 23:00 24:00");
            s["freeze_schedule_locks"] = new System.Text.Json.Nodes.JsonArray(new System.Text.Json.Nodes.JsonObject());
        }));
        gui.Page("blocks");
        gui.Invoke("ModeCard_Frozen Chicken");
        var item = gui.Window.FindFirstDescendant(c => c.ByControlType(ControlType.ListItem).And(c.ByName("daily 23:00 24:00")))
                   ?? gui.FindName("daily 23:00 24:00");
        if (item.Patterns.SelectionItem.IsSupported) item.Patterns.SelectionItem.Pattern.Select();
        else gui.Invoke(item);
        gui.Invoke("FreezeScheduleLock");
        gui.Invoke("FreezeLocks_Timer_For");
        gui.Find("FreezeLocks_Timer_Length").AsTextBox().Text = "2h";
        gui.Invoke(gui.FindButton("Lock"));
        gui.FindName("Set this lock?");
        Assert.False(gui.Ran("freeze", "schedule", "lock"));
        gui.Shot("freeze-lock-confirm");
        gui.Invoke(gui.FindButton("Lock"));
        gui.WaitFor(() => gui.Ran("freeze", "schedule", "lock", "1", "--for", "120m", "--yes"), "kc9k freeze schedule lock 1 --for 120m --yes");
    }

    // gui_automation: Disabled or Enabled by default runs 'kc9k automation'.
    [Fact]
    public void AutomationDefaultIsChosen()
    {
        using var gui = new GuiApp("automation-default");
        gui.Page("blocks");
        gui.Invoke("BlockAuto_Evening");
        gui.Invoke("block_default_enabled");
        gui.WaitFor(() => gui.Ran("automation", "Evening", "default", "enabled"), "kc9k automation Evening default enabled");
        gui.Invoke("block_default_disabled");
        gui.WaitFor(() => gui.Ran("automation", "Evening", "default", "disabled"), "kc9k automation Evening default disabled");
    }
}
