// W32: the rest of the GTK GUI's smoke tests (ci/gui_*_smoke.py) through UI
// Automation, each run twice: against the fake kc9k (its fixtures; the
// commands checked) and against the real kc9k.exe on a scratch database
// (KC9K_SCRATCH_EXE, KC9K_DB: nothing applied), where what the commands
// did is checked in `kc9k gui-state` too. The same names exist in both:
// the scratch database is set up as the fixtures are (make-fixtures.sh).
using System.Text.Json.Nodes;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class ParityRealTests
{
    static GuiApp Start(string name, bool real, string[][] setup, Action<string>? prepare = null) =>
        new(name + (real ? "-real" : ""), scratch: real, setup: setup, prepare: prepare);

    /// <summary>The fixture's Work is in the mode Study (no card of its own on
    /// the Blocks page); the phrase tests open it as its own block.</summary>
    static void WorkOutOfStudy(string dir) => GuiApp.EditState(dir, s =>
    {
        s["blocks"]!.AsArray().First(b => (string?)b!["name"] == "Work")!["mode"] = "";
        var blocks = s["modes"]!.AsArray().First(m => (string?)m!["name"] == "Study")!["blocks"]!.AsArray();
        foreach (var b in blocks.Where(b => (string?)b == "Work").ToList()) blocks.Remove(b);
    });

    static bool On(GuiApp gui, string id) => gui.Find(id).Patterns.Toggle.Pattern.ToggleState.Value == ToggleState.On;

    static void Acknowledge(GuiApp gui)
    {
        gui.Invoke("AcknowledgeTick");
        gui.Invoke(gui.FindButton("Set it up"));
    }

    static JsonArray Actions(JsonObject block, string list) =>
        block["lists"]!.AsArray().First(l => (string?)l!["name"] == list)!["actions"]!.AsArray();

    static JsonNode? Named(JsonArray items, string key, string value) =>
        items.FirstOrDefault(a => (string?)a![key] == value);

    static void SaveEdits(GuiApp gui)
    {
        gui.Invoke("SaveEdits");
        gui.WaitFor(() => gui.Ran("block", "draft", "Reading", "save"), "kc9k block draft Reading save");
    }

    static readonly string[][] ReadingAndStudy =
    [
        ["block", "create", "Reading"], ["block", "add", "Reading", "goodreads.com"],
        ["block", "add-app", "Reading", "steam.exe"], ["block", "add-title", "Reading", "YouTube"],
        ["block", "create", "Work"], ["mode", "create", "Study"], ["mode", "add", "Study", "Work"],
    ];

    // gui_actions (§3.9, R-MODE-4, R-CUSTOM-1/2): a list's actions take "Run a
    // mode, locked" (which mode, how long) and "Custom action"; the apps'
    // actions too; a mode says what it opens when it becomes enabled; a mode
    // run by a Mode action shows locked, its switch greyed out.
    [Theory, InlineData(false), InlineData(true)]
    public void ActionsRunAModeLockedAndCustomActions(bool real)
    {
        using var gui = Start("actions", real, ReadingAndStudy, prepare: real ? null : dir => GuiApp.EditState(dir, s =>
        {
            var study = s["modes"]!.AsArray().First(m => (string?)m!["name"] == "Study")!;
            study["run_left"] = 3600;
        }));
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.Invoke("List_Blocked_Actions");
        gui.Invoke("List_Blocked_Action_mode");
        var modes = gui.Find("ModeRunMode").AsComboBox();
        modes.Expand();
        Assert.Equal("Monk mode", modes.Items[0].Name);
        Assert.Contains(modes.Items, i => i.Name == "Study");
        Assert.DoesNotContain(modes.Items, i => i.Name == "Frozen Chicken");
        modes.Select("Study");
        modes.Collapse();
        gui.Find("ModeRunLength").AsTextBox().Text = "1h 30m";
        gui.Shot("mode-action");
        gui.Invoke(gui.FindButton("OK"));
        Acknowledge(gui);
        gui.WaitFor(() => gui.RanWith("list", "action", "Blocked", "mode", "on", "--target", "Study", "--for", "90m", "--no-confirm-ok"),
            "kc9k list action … Blocked mode on --target Study --for 90m");

        gui.Invoke("List_Blocked_Actions");
        if (gui.ById("List_Blocked_Action_custom") is { IsOffscreen: true }) gui.Invoke("List_Blocked_Actions");
        gui.Invoke("List_Blocked_Action_custom");
        gui.Find("AskCustom_Entry").AsTextBox().Text = "https://example.org/notes";
        gui.Invoke("AskCustom_Add");
        gui.Find("AskCustom_Kind").AsComboBox().Select("Run a program or command");
        gui.Find("AskCustom_Entry").AsTextBox().Text = "notepad.exe";
        gui.Invoke("AskCustom_Add");
        gui.Shot("custom-action");
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.RanWith("list", "action", "Blocked", "custom", "on", "open https://example.org/notes", "run notepad.exe"),
            "kc9k list action … custom on --custom 'open https://example.org/notes' --custom 'run notepad.exe'");

        // The apps' actions.
        gui.Invoke("Tab_apps");
        gui.Find("Trigger_apps_Mode").AsComboBox().Select("Study");
        gui.Invoke("Trigger_apps_mode");
        Acknowledge(gui);
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "mode", "on", "--target", "Study"), "kc9k block trigger … apps mode on --target Study");
        gui.Invoke("Trigger_apps_custom");
        gui.Find("AskCustom_Entry").AsTextBox().Text = @"C:\Notes";
        gui.Invoke("AskCustom_Add");
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "custom", "on", @"open C:\Notes"), "kc9k block trigger … apps custom on");
        if (real)
        {
            SaveEdits(gui);
            var b = gui.RealBlock("Reading");
            var mode = Named(Actions(b, "Blocked"), "action", "mode");
            Assert.True(mode is not null && (string?)mode["param"] == "Study" && (long?)mode["minutes"] == 90, $"Run a mode, locked: {mode}");
            Assert.Contains("notepad.exe", (string?)Named(Actions(b, "Blocked"), "action", "custom")?["param"] ?? "");
            var apps = b["triggers"]!["apps"]!.AsArray();
            Assert.Equal("Study", (string?)Named(apps, "name", "mode")?["param"]);
            Assert.Contains("Notes", (string?)Named(apps, "name", "custom")?["param"] ?? "");
            gui.Invoke("Tab_websites");
            gui.FindName("Runs Study for 1 h 30 min, locked", 20);
            gui.FindName("open https://example.org/notes; run notepad.exe", 20);
        }

        // A mode's custom action, and a mode run by a Mode action.
        gui.Invoke("BackToBlocks");
        gui.Invoke("ModeCard_Study");
        gui.Find("ModeCustom_Entry").AsTextBox().Text = "https://example.org/work";
        gui.Invoke("ModeCustom_Add");
        gui.WaitFor(() => gui.Ran("mode", "custom", "Study", "add", "open https://example.org/work"), "kc9k mode custom Study add …");
        if (real)
        {
            var study = gui.RealState()["modes"]!.AsArray().First(m => (string?)m!["name"] == "Study")!;
            Assert.Equal("open https://example.org/work", string.Join(" ", study["custom"]!.AsArray().Select(c => (string?)c)));
            var (ok, output) = gui.Cli("mode", "run", "Study", "--for", "1h", "--yes");
            Assert.True(ok, output);
        }
        gui.Invoke("ModeBack");
        gui.WaitFor(() => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "").StartsWith("Running, locked")), "the card says it runs, locked", 20);
        gui.Shot("mode-running-locked");
        gui.Invoke("ModeCard_Study");
        gui.WaitFor(() => !gui.Find("ModeViewSwitch").IsEnabled, "its switch greyed out");
        Assert.Contains(gui.Window.FindAllDescendants(), e => (e.Properties.Name.ValueOrDefault ?? "").StartsWith("Run by a Mode action: locked"));
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    // gui_trigger (owner 2026-11-16): apps close by default, titles close the
    // window; a consequence with its time; a place rule has no actions;
    // locked, turning an action off is refused and the box goes back, and
    // turning one on is free.
    [Theory, InlineData(false), InlineData(true)]
    public void TriggersEachWithItsTime(bool real)
    {
        using var gui = Start("trigger", real, ReadingAndStudy);
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.Invoke("Tab_apps");
        gui.WaitFor(() => On(gui, "Trigger_apps_close") && !On(gui, "Trigger_apps_logout"), "apps: Close the app by default");
        Assert.True(On(gui, "Trigger_titles_close"), "titles: Close the window by default");
        gui.Find("Trigger_apps_logout_For").AsTextBox().Text = "30m";
        gui.Invoke("Trigger_apps_logout");
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "logout", "on", "--for"), "kc9k block trigger … apps logout on --for 30m");
        gui.Invoke("Trigger_apps_close");
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "close", "off"), "the app no longer closed");
        gui.Find("Trigger_apps_monk_For").AsTextBox().Text = "2h";
        gui.Invoke("Trigger_apps_monk");
        Acknowledge(gui);
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "monk", "on", "--no-confirm-ok"), "monk mode, acknowledged");
        gui.Invoke("Trigger_titles_close-browser");
        gui.WaitFor(() => gui.RanWith("block", "trigger", "titles", "close-browser", "on"), "titles close the browsers too");
        gui.Shot("triggers");
        // §3.6: a place rule has no actions.
        Assert.Null(gui.FindButtonOrNull("Actions…"));
        if (!real)
        {
            Assert.Null(gui.DesktopByName("Something went wrong"));
            return;
        }
        SaveEdits(gui);
        var apps = gui.RealBlock("Reading")["triggers"]!["apps"]!.AsArray();
        Assert.Null(Named(apps, "name", "close"));
        Assert.Equal(30, (long?)Named(apps, "name", "logout")?["minutes"]);
        Assert.Equal(120, (long?)Named(apps, "name", "monk")?["minutes"]);
        Assert.NotNull(Named(gui.RealBlock("Reading")["triggers"]!["titles"]!.AsArray(), "name", "close-browser"));
        Assert.DoesNotContain(gui.RealBlock("Reading")["triggers"]!.AsObject(), kv => kv.Key.StartsWith("place:"));

        // Locked: turning logout off is refused at Save edits, whole, and said.
        var (ok, output) = gui.Cli("start", "Reading", "--for", "1h", "--yes");
        Assert.True(ok, output);
        gui.WaitFor(() => On(gui, "Trigger_apps_logout"), "logout shown on");
        gui.Invoke("Trigger_apps_logout");
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "logout", "off"), "logout off in the draft");
        gui.Invoke("SaveEdits");
        gui.FindName("Not done", 30);
        Assert.Contains("lock", gui.Find("MessageText").Name, StringComparison.OrdinalIgnoreCase);
        gui.Shot("refused-locked");
        gui.Invoke(gui.FindButton("OK"));
        Assert.Equal(30, (long?)Named(gui.RealBlock("Reading")["triggers"]!["apps"]!.AsArray(), "name", "logout")?["minutes"]);
        // The save ended the draft (all but the weakening part): the box is back on.
        gui.WaitFor(() => On(gui, "Trigger_apps_logout"), "the box back on");
        // Turning close back on strengthens: free while locked.
        gui.Invoke("Trigger_apps_close");
        gui.WaitFor(() => gui.RanWith("block", "trigger", "apps", "close", "on"), "close back on");
        gui.Invoke("SaveEdits");
        gui.WaitFor(() => Named(gui.RealBlock("Reading")["triggers"]!["apps"]!.AsArray(), "name", "close") is not null, "close saved while locked", 30);
        Assert.Null(gui.DesktopByName("Not done"));
    }

    static double Height(GuiApp gui, string list) =>
        (gui.ById(list) ?? gui.Find(list + "_Cover")).BoundingRectangle.Height;

    static JsonObject Prefs(GuiApp gui) =>
        JsonNode.Parse(File.ReadAllText(Path.Combine(gui.Dir, "gui.json")))!.AsObject();

    static double? KeptHeight(GuiApp gui) => (double?)Prefs(gui)["list_heights"]?["website list"];

    static void Keys(AutomationElement grip, FlaUI.Core.WindowsAPI.VirtualKeyShort key, int times)
    {
        grip.Focus();
        for (var i = 0; i < times; i++) FlaUI.Core.Input.Keyboard.Press(key);
        FlaUI.Core.Input.Wait.UntilInputIsProcessed();
        Thread.Sleep(300);
    }

    // gui_list_resize (owner 2026-11-19): the grip under a list makes it
    // taller or shorter (never below the least height), by mouse or keys;
    // the height is kept in gui.json per kind of list, so the other list of
    // that kind on screen takes it at once and a new one opens as tall; a
    // double-click puts it back and forgets it.
    [Theory, InlineData(false), InlineData(true)]
    public void AListsGripResizesEveryListOfItsKind(bool real)
    {
        const string blocked = "List_Blocked_Entries", exceptions = "List_Exceptions_Entries";
        string dir;
        double tall;
        double start;
        using (var gui = Start("list-resize", real, [["block", "create", "Reading"], ["block", "add", "Reading", "goodreads.com"]]))
        {
            dir = gui.Dir;
            gui.Page("blocks");
            gui.Invoke("BlockCard_Reading");
            var grip = gui.Find(blocked + "_Grip");
            start = Height(gui, blocked);
            Keys(grip, FlaUI.Core.WindowsAPI.VirtualKeyShort.DOWN, 5);
            gui.WaitFor(() => Height(gui, blocked) >= start + 150, $"taller by keys ({start} -> {Height(gui, blocked)})");
            Assert.True(Math.Abs(Height(gui, exceptions) - Height(gui, blocked)) <= 2,
                $"the other list of that kind takes the height ({Height(gui, blocked)}, {Height(gui, exceptions)})");
            gui.WaitFor(() => KeptHeight(gui) >= 300, $"height kept in gui.json: {Prefs(gui)}");
            Keys(grip, FlaUI.Core.WindowsAPI.VirtualKeyShort.UP, 40);
            gui.WaitFor(() => KeptHeight(gui) == 80, $"far up: stops at the least height ({KeptHeight(gui)})");
            // By mouse, where a person drags it.
            grip = gui.Find(blocked + "_Grip");
            var r = grip.BoundingRectangle;
            var from = new System.Drawing.Point(r.Left + r.Width / 2, r.Top + r.Height / 2);
            FlaUI.Core.Input.Mouse.Drag(from, new System.Drawing.Point(from.X, from.Y + 200));
            Thread.Sleep(500);
            gui.WaitFor(() => KeptHeight(gui) > 150, $"dragged down 200: taller ({KeptHeight(gui)})");
            gui.Shot("tall");
            tall = Height(gui, blocked);
        }
        using (var again = new GuiApp("list-resize-again" + (real ? "-real" : ""), dir: dir, scratch: real))
        {
            again.Page("blocks");
            again.Invoke("BlockCard_Reading");
            again.WaitFor(() => Math.Abs(Height(again, blocked) - tall) <= 2, $"a new list of that kind opens as tall ({tall}, {Height(again, blocked)})");
            var grip = again.Find(blocked + "_Grip");
            var r = grip.BoundingRectangle;
            FlaUI.Core.Input.Mouse.DoubleClick(new System.Drawing.Point(r.Left + r.Width / 2, r.Top + r.Height / 2));
            again.WaitFor(() => KeptHeight(again) is null, "double-click: forgotten in gui.json");
            again.WaitFor(() => Math.Abs(Height(again, blocked) - start) <= 2 && Math.Abs(Height(again, exceptions) - start) <= 2,
                $"double-click: both back to the usual height ({start}: {Height(again, blocked)}, {Height(again, exceptions)})");
        }
    }

    static string Editor(GuiApp gui, int n) => gui.Find($"Phrases_Gossip_Set{n}").AsTextBox().Text;

    static void Type(GuiApp gui, int n, string text) => gui.Find($"Phrases_Gossip_Set{n}").AsTextBox().Text = text;

    static AutomationElement Save(GuiApp gui) => gui.Find("Phrases_Gossip_Save");

    static List<string> Phrases(GuiApp gui) =>
        [.. gui.RealBlock("Work")["phrase_sets"]!.AsArray().First(p => (string?)p!["name"] == "Gossip")!["phrases"]!.AsArray()
            .Select(p => (string?)p!["text"] ?? "")];

    /// <summary>Loads a phrase into the card's editor (Enter on it, as a double-click).</summary>
    static void Load(GuiApp gui, string text)
    {
        if (gui.ById("Phrases_Gossip_Phrases_Cover") is { IsOffscreen: false }) gui.Invoke("Phrases_Gossip_Phrases_Cover");
        var item = FlaUI.Core.Tools.Retry.WhileNull(() => gui.Find("Phrases_Gossip_Phrases").FindFirstDescendant(c => c.ByName(text)),
            TimeSpan.FromSeconds(20), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: "no phrase " + text).Result!;
        item.Patterns.SelectionItem.Pattern.Select();
        item.Focus();
        FlaUI.Core.Input.Keyboard.Press(FlaUI.Core.WindowsAPI.VirtualKeyShort.RETURN);
        FlaUI.Core.Input.Wait.UntilInputIsProcessed();
        gui.WaitFor(() => Save(gui).Name == "Save changes", "the phrase loaded: Save changes");
    }

    static readonly string[][] Gossip =
    [
        ["block", "create", "Work"], ["block", "add", "Work", "youtube.com"],
        ["phrase-set", "create", "Work", "Gossip"], ["phrase-set", "add", "Work", "Gossip", "[celebrity, star] [divorce, scandal]"],
    ];
    const string GossipPhrase = "[celebrity, star] [divorce, scandal]";

    // gui_phrase (§4.3): the phrase editor sits in the card; Enter (or a
    // double-click) loads a phrase; an unchanged one can't be saved; Cancel
    // asks before dropping changes; a phrase built from word sets, more than
    // nine of them; Delete phrase leaves the editor empty; a one-ended
    // bridge is refused; the list cards tick where they look, ads included.
    [Theory, InlineData(false), InlineData(true)]
    public void PhraseSetsInTheCard(bool real)
    {
        using var gui = Start("phrase", real, Gossip, prepare: real ? null : dir =>
        {
            WorkOutOfStudy(dir);
            GuiApp.Respond(dir, (["phrase-set", "add", "Work (draft)", "Gossip", "[-gorilla] [x]"], false, "Error: a bridge needs both ends: a term ending in - and one starting with -"));
        });
        gui.Page("blocks");
        gui.Invoke("BlockCard_Work");
        gui.Find("Phrases_Gossip_Where_title");
        Assert.Null(gui.ById("Phrases_Gossip_Where_ads"));
        Load(gui, GossipPhrase);
        Assert.Equal("celebrity, star", Editor(gui, 1));
        Assert.Equal("divorce, scandal", Editor(gui, 2));
        Assert.False(Save(gui).IsEnabled, "an unchanged phrase can't be saved");
        Assert.False(gui.FindButton("Remove word set 1").IsEnabled, "two word sets can't be removed");
        Type(gui, 2, "divorce, scandal, breakup");
        gui.WaitFor(() => Save(gui).IsEnabled, "Save on after a change");
        gui.Find("Phrases_Gossip_DeletePhrase");
        gui.Invoke("Phrases_Gossip_Cancel");
        gui.FindName("Drop the unsaved phrase?");
        gui.Invoke(gui.FindButton("Drop changes"));
        gui.WaitFor(() => Editor(gui, 1) == "" && Save(gui).Name == "Add phrase" && !Save(gui).IsEnabled, "a cleared editor can't be saved");
        Assert.True(gui.ById("Phrases_Gossip_DeletePhrase") is null or { IsOffscreen: true }, "no Delete phrase for a new phrase");

        Type(gui, 1, "red");
        Type(gui, 2, "car, truck");
        gui.Invoke("Phrases_Gossip_AddSet");
        Type(gui, 3, "big");
        gui.Invoke("Phrases_Gossip_Necessary3");
        gui.WaitFor(() => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "") == "Phrase: [red] [car, truck] [big]?"), "the phrase built");
        for (var i = 0; i < 10; i++) gui.Invoke("Phrases_Gossip_AddSet");
        gui.Find("Phrases_Gossip_Set13");
        Assert.True(gui.Find("Phrases_Gossip_AddSet").IsEnabled, "more than nine word sets");
        for (var n = 13; n > 3; n--) gui.Invoke($"Phrases_Gossip_RemoveSet{n}");
        gui.Shot("built");
        gui.Invoke("Phrases_Gossip_Save");
        gui.WaitFor(() => gui.Ran("phrase-set", "add", "Work (draft)", "Gossip", "[red] [car, truck] [big]?"), "kc9k phrase-set add … [red] [car, truck] [big]?");
        if (real)
        {
            gui.WaitFor(() => Save(gui).Name == "Save changes", "the added phrase stays loaded");
            gui.Invoke("Phrases_Gossip_DeletePhrase");
            gui.Invoke(gui.FindButton("Delete"));
            gui.WaitFor(() => gui.Ran("phrase-set", "remove", "Work (draft)", "Gossip", "[red] [car, truck] [big]?"), "Delete phrase");
            gui.WaitFor(() => Editor(gui, 1) == "" && Save(gui).Name == "Add phrase", "the editor empty after Delete phrase");
            Assert.Null(gui.DesktopByName("Drop the unsaved phrase?"));
        }

        // A one-ended bridge: refused with a message, the editor keeps it.
        gui.Invoke("Phrases_Gossip_Cancel");
        if (gui.ById("Phrases_Gossip_Set3") is not null) gui.Invoke(gui.FindButton("Drop changes"));
        Type(gui, 1, "-gorilla");
        Type(gui, 2, "x");
        gui.Invoke("Phrases_Gossip_Save");
        gui.FindName("Not done", 20);
        Assert.Contains("bridge", gui.Find("MessageText").Name);
        gui.Invoke(gui.FindButton("OK"));
        Assert.Equal("-gorilla", Editor(gui, 1));

        // §3.2, §3.9: a list's card ticks where it looks, ads included.
        gui.Invoke("List_Blocked_Where_ads");
        gui.WaitFor(() => gui.Ran("list", "scope", "Work (draft)", "Blocked", "ads", "on"), "kc9k list scope … Blocked ads on");
        if (real)
        {
            gui.Invoke("SaveEdits");
            gui.WaitFor(() => gui.Ran("block", "draft", "Work", "save"), "saved");
            Assert.Equal([GossipPhrase], Phrases(gui));
            var (_, shown) = gui.Cli("list", "show", "Work");
            Assert.Contains("ads", shown);
        }
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    // gui_phrase_edit (§4.3, R-WEAK-5): a phrase changed twice in a row stays
    // one phrase; letter case alone can't be saved; one the set has can't be
    // added again; a state read while typing keeps the text; Unicode terms;
    // locked, a change that drops terms is refused, said in kc9k's words, and
    // the phrase stays.
    [Theory, InlineData(false), InlineData(true)]
    public void PhrasesChangedTheHardWay(bool real)
    {
        using var gui = Start("phrase-edit", real, Gossip, prepare: real ? null : WorkOutOfStudy);
        gui.Page("blocks");
        gui.Invoke("BlockCard_Work");
        Load(gui, GossipPhrase);
        Type(gui, 2, "divorce, scandal, breakup");
        gui.Invoke("Phrases_Gossip_Save");
        gui.WaitFor(() => gui.Ran("phrase-set", "change", "Work (draft)", "Gossip", GossipPhrase, "[celebrity, star] [divorce, scandal, breakup]"),
            "kc9k phrase-set change … (one step)");
        if (real)
        {
            gui.WaitFor(() => Save(gui).Name == "Save changes" && !Save(gui).IsEnabled, "the changed phrase stays loaded");
            Type(gui, 1, "celebrity, star, idol");
            gui.Invoke("Phrases_Gossip_Save");
            gui.WaitFor(() => gui.Ran("phrase-set", "change", "Work (draft)", "Gossip", "[celebrity, star] [divorce, scandal, breakup]", "[celebrity, star, idol] [divorce, scandal, breakup]"),
                "the second change of the same phrase");
            Type(gui, 1, "CELEBRITY, STAR, IDOL");
            gui.WaitFor(() => !Save(gui).IsEnabled, "a case-only change can't be saved");
            gui.Invoke("Phrases_Gossip_Cancel");
            gui.Invoke(gui.FindButton("Drop changes"));
        }
        else
        {
            gui.Invoke("Phrases_Gossip_Cancel");
            if (gui.DesktopByName("Drop the unsaved phrase?") is not null) gui.Invoke(gui.FindButton("Drop changes"));
        }
        // A new phrase equal to one there: Add is off.
        Type(gui, 1, real ? "celebrity, star, idol" : "celebrity, star");
        Type(gui, 2, real ? "divorce, scandal, breakup" : "divorce, scandal");
        gui.WaitFor(() => !Save(gui).IsEnabled, "a phrase the set has can't be added again");
        // Unicode terms; a state read while typing (every 5 s) keeps the text.
        Type(gui, 1, "café, naïve");
        Type(gui, 2, "日本");
        Thread.Sleep(7000);
        Assert.Equal("café, naïve", Editor(gui, 1));
        gui.Invoke("Phrases_Gossip_Save");
        gui.WaitFor(() => gui.Ran("phrase-set", "add", "Work (draft)", "Gossip", "[café, naïve] [日本]"), "the Unicode phrase added");
        if (!real) return;
        gui.Invoke("SaveEdits");
        gui.WaitFor(() => gui.Ran("block", "draft", "Work", "save"), "saved");
        gui.WaitFor(() => Phrases(gui).Count == 2, $"two phrases: {string.Join(" | ", Phrases(gui))}");
        Assert.Contains("[café, naïve] [日本]", Phrases(gui));

        // Locked: dropping a term is refused and said; the phrase stays.
        var (ok, output) = gui.Cli("start", "Work", "--for", "1h", "--yes");
        Assert.True(ok, output);
        const string locked = "[celebrity, star, idol] [divorce, scandal, breakup]";
        Load(gui, locked);
        Type(gui, 2, "divorce, scandal");
        gui.Invoke("Phrases_Gossip_Save");
        gui.WaitFor(() => gui.Ran("phrase-set", "change", "Work (draft)", "Gossip", locked, "[celebrity, star, idol] [divorce, scandal]"), "the change in the draft");
        gui.Invoke(gui.Find("SaveEdits", 30));
        gui.FindName("Not done", 30);
        Assert.Contains("locked", gui.Find("MessageText").Name);
        gui.Invoke(gui.FindButton("OK"));
        Assert.Contains(locked, Phrases(gui));
        // Adding a term only strengthens (R-WEAK-5): free through `phrase-set
        // change`, not yet through a draft's save (ROADMAP A18), so not checked here.
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    static bool Shows(GuiApp gui, string text) => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "") == text);

    static void SavePlaceDialog(GuiApp gui, string id, string name, bool rule)
    {
        gui.Invoke(id);
        gui.Find("PlaceName").AsTextBox().Text = name;
        if (!rule) gui.Invoke("PlaceRule");
        gui.Invoke(gui.FindButton("Save"));
    }

    static void Place(string dir, string name, int networks, int need, bool here, params string[] blocks) => GuiApp.EditState(dir, s =>
    {
        var places = s["places"]!.AsArray();
        foreach (var old in places.Where(p => (string?)p!["name"] == name).ToList()) places.Remove(old);
        places.Add(new JsonObject
        {
            ["name"] = name, ["networks"] = networks, ["need"] = need, ["here"] = here,
            ["blocks"] = new JsonArray([.. blocks.Select(b => (JsonNode)JsonValue.Create(b)!)]), ["modes"] = new JsonArray(),
        });
        foreach (var b in s["blocks"]!.AsArray())
        {
            var list = b!["places"]!.AsArray();
            if (blocks.Contains((string?)b["name"]) && !list.Any(x => (string?)x == name)) list.Add(name);
            if (!blocks.Contains((string?)b["name"])) foreach (var x in list.Where(x => (string?)x == name).ToList()) list.Remove(x);
        }
    });

    // gui_places (§3.6, §4.5, R-MODE-5): a block with no place says so; Save
    // where I am now saves the networks in range and, ticked, adds the place
    // to the block; its row says it is added, what arriving does and that the
    // computer is there; enabled by default says off; networks needed checked
    // as typed; Add this place for another block, then Remove; without the
    // tick only the place is saved; with no Wi-Fi in range kc9k says why and
    // nothing is saved. windows-latest has no Wi-Fi: the real kc9k.exe is
    // checked on that last path (W10 has the rest on the laptop).
    [Theory, InlineData(false), InlineData(true)]
    public void PlacesSavedAndAddedToBlocks(bool real)
    {
        using var gui = Start("places", real, [["block", "create", "Reading"], ["block", "create", "Games"]],
            prepare: real ? null : dir => GuiApp.Respond(dir, (["place", "add", "home"], true, "Place 'home' saved: 4 networks in range (needs 2)."),
                (["place", "add", "library"], true, "Place 'library' saved: 2 networks in range (needs 2)."),
                (["place", "add", "nowhere"], false, "Error: no Wi-Fi network is in range: a place is the networks seen where you save it")));
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        gui.Invoke("BlockAutomation");
        gui.WaitFor(() => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "").StartsWith("No places saved yet")), "no places yet");
        if (real)
        {
            SavePlaceDialog(gui, "block_SaveHere", "nowhere", rule: true);
            gui.FindName("Not done", 90);
            Assert.Contains("Wi-Fi", gui.Find("MessageText").Name);
            gui.Shot("no-wifi");
            gui.Invoke(gui.FindButton("OK"));
            Assert.Empty(gui.RealState()["places"]!.AsArray());
            Assert.False(gui.Ran("place", "rule"), "no rule without a place");
            return;
        }

        // 1. Saved where the computer is, and turned on there.
        SavePlaceDialog(gui, "block_SaveHere", "home", rule: true);
        gui.WaitFor(() => gui.Ran("place", "add", "home") && gui.RanWith("place", "rule", "home", "on"), "kc9k place add home, then place rule … home on");
        Assert.True(gui.Commands().FindIndex(c => c.Contains("add") && c.Contains("home")) < gui.Commands().FindIndex(c => c.Contains("rule")), "the place first, then the rule");
        gui.FindName("Place saved");
        gui.Invoke(gui.FindButton("OK"));
        Place(gui.Dir, "home", 4, 2, true, "Reading");
        gui.WaitFor(() => Shows(gui, "Added. Arriving here turns this block on.") && Shows(gui, "You are here"), "its row: added, what arriving does, here", 20);
        Assert.False(gui.Find("block_AddPlace").IsEnabled, "nothing left to add");
        gui.Shot("place-added");
        // Enabled by default: arriving turns it off.
        GuiApp.EditState(gui.Dir, s => s["blocks"]!.AsArray().First(b => (string?)b!["name"] == "Reading")!["automation"]!["default_on"] = true);
        gui.WaitFor(() => Shows(gui, "Added. Arriving here turns this block off."), "enabled by default: off here", 20);

        // 2. Networks needed, checked as typed.
        gui.Page("places");
        var need = gui.Find("PlaceNeed_home").AsTextBox();
        Assert.Equal("2", need.Text);
        need.Text = "5";
        gui.WaitFor(() => !gui.Find("PlaceNeedSave_home").IsEnabled && (need.HelpText ?? "").Contains("At most 4"), "5 of 4 refused where typed");
        need.Text = "3";
        gui.WaitFor(() => gui.Find("PlaceNeedSave_home").IsEnabled, "3 of 4 can be saved");
        gui.Invoke("PlaceNeedSave_home");
        gui.WaitFor(() => gui.Ran("place", "need", "home", "3"), "kc9k place need home 3");

        // 3. Add this place for another block, then remove it.
        gui.Page("blocks");
        gui.Invoke("BlockCard_Games");
        gui.Invoke("BlockAutomation");
        gui.WaitFor(() => gui.Find("block_PlaceChoice").AsComboBox().SelectedItem?.Name == "home", "home offered for another block");
        gui.Invoke("block_AddPlace");
        gui.WaitFor(() => gui.RanWith("place", "rule", "Games", "home", "on"), "kc9k place rule Games home on");
        Place(gui.Dir, "home", 4, 3, true, "Reading", "Games");
        gui.Invoke(gui.Find("RemovePlace_home", 20));
        gui.WaitFor(() => gui.RanWith("place", "rule", "Games", "home", "off"), "kc9k place rule Games home off");
        Place(gui.Dir, "home", 4, 3, true, "Reading");
        gui.WaitFor(() => gui.ById("RemovePlace_home") is null, "the rule gone from the list", 20);

        // 4. Without the tick, only the place.
        var before = gui.Commands().Count;
        SavePlaceDialog(gui, "block_SaveHere", "library", rule: false);
        gui.WaitFor(() => gui.Ran("place", "add", "library"), "kc9k place add library");
        gui.FindName("Place saved");
        gui.Invoke(gui.FindButton("OK"));
        Assert.DoesNotContain(gui.Commands().Skip(before), c => c.Contains("rule"));

        // 5. No Wi-Fi in range: kc9k says why.
        SavePlaceDialog(gui, "block_SaveHere", "nowhere", rule: true);
        gui.FindName("Not done", 20);
        Assert.Contains("no Wi-Fi network is in range", gui.Find("MessageText").Name);
        gui.Invoke(gui.FindButton("OK"));
        Assert.False(gui.RanWith("place", "rule", "nowhere"), "no rule for a place not saved");
    }

    const string Playlist = "youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf";

    // gui_playlist (§3.2 Exception list, §4.3; owner 2026-11-17): a playlist is
    // let through by its address typed into the exception list (no box of its
    // own), kept one way however it is pasted; Social media is a section of
    // the Websites tab between the blacklists and the exceptions, closed at
    // first, opened and closed, saying how many settings are on.
    [Theory, InlineData(false), InlineData(true)]
    public void PlaylistsAndSocialMediaInTheWebsitesTab(bool real)
    {
        using var gui = Start("playlist", real, [["block", "create", "Reading"], ["block", "add", "Reading", "goodreads.com"]]);
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        Assert.Null(gui.FindButtonOrNull("Let through"));
        Assert.Null(gui.ById("Tab_social"));
        gui.TypeEnter("List_Exceptions_Entries_Entry", "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf&si=x");
        gui.WaitFor(() => gui.Commands().Any(c => c.Contains("Exceptions") && c.Any(a => a.Contains("playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"))),
            "the playlist added to Exceptions");
        Assert.Contains(gui.Window.FindAllDescendants(), e => (e.Properties.Name.ValueOrDefault ?? "").Contains("playlist"));
        // The section's place: after the blacklists, above the exceptions.
        double Top(string id) => gui.Find(id).BoundingRectangle.Top;
        var social = gui.Find("Social");
        Assert.True(Top("List_Blocked_Entries") < social.BoundingRectangle.Top && social.BoundingRectangle.Top < Top("List_Exceptions_Entries"),
            "Social media between the blacklists and the exceptions");
        Assert.Equal(ExpandCollapseState.Collapsed, social.Patterns.ExpandCollapse.Pattern.ExpandCollapseState.Value);
        social.Patterns.ExpandCollapse.Pattern.Expand();
        gui.WaitFor(() => gui.ById("Social_youtube_no-shorts") is { IsOffscreen: false }, "opens");
        gui.Invoke("Social_youtube_no-shorts");
        gui.WaitFor(() => gui.Ran("social", "Reading (draft)", "youtube:no-shorts", "on") || gui.Ran("social", "Reading", "youtube:no-shorts", "on"),
            "kc9k social … youtube:no-shorts on");
        if (real)
        {
            gui.Invoke(gui.Find("SaveEdits", 30));
            gui.WaitFor(() => gui.Ran("block", "draft", "Reading", "save"), "saved");
            var b = gui.RealBlock("Reading");
            var exceptions = b["lists"]!.AsArray().First(l => (string?)l!["name"] == "Exceptions")!["entries"]!.AsArray().Select(e => (string?)e).ToList();
            Assert.Contains(Playlist, exceptions);
            Assert.Equal(["youtube:no-shorts"], b["social"]!.AsArray().Select(x => (string?)x).ToList());
            gui.WaitFor(() => gui.Find("Social").Name.Contains("1 setting on") || gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "") == "Social media · 1 setting on"),
                "the section says 1 setting on");
        }
        gui.Find("Social").Patterns.ExpandCollapse.Pattern.Collapse();
        gui.WaitFor(() => gui.Find("Social").Patterns.ExpandCollapse.Pattern.ExpandCollapseState.Value == ExpandCollapseState.Collapsed, "closes");
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    static JsonObject RealMode(GuiApp gui, string name) =>
        gui.RealState()["modes"]!.AsArray().First(m => (string?)m!["name"] == name)!.AsObject();

    // gui_wallpaper (R-MODE-5, §4.10; owner 2026-11-17): a mode's view has
    // circadian rhythm's wallpaper layout (picture through Windows' own open
    // dialog, its times, a bar on the timeline dragged); Override circadian
    // rhythm wallpapers there and Override modes on the circadian rhythm page
    // are one setting seen from both sides; Frozen Chicken takes wallpapers
    // too; Remove; Settings' wallpaper default (Latest, Circadian rhythm,
    // Custom with its picture, which asks once).
    [Theory, InlineData(false), InlineData(true)]
    public void WallpapersOfModes(bool real)
    {
        using var gui = Start("wallpaper", real, [["block", "create", "Work"], ["mode", "create", "Study"], ["mode", "add", "Study", "Work"]]);
        var sky = gui.Picture("sky.png");
        gui.Page("blocks");
        gui.Invoke("ModeCard_Study");
        if (real)
        {
            gui.Invoke("Mode_AddWallpaper");
            gui.PickFile(sky);
            gui.Find("NewWallpaperFrom").AsTextBox().Text = "09:00";
            gui.Find("NewWallpaperTo").AsTextBox().Text = "17:00";
            gui.Invoke(gui.FindButton("Add"));
            gui.WaitFor(() => gui.RanWith("mode", "wallpaper", "Study", "add", sky, "09:00", "17:00"), "kc9k mode wallpaper Study add … 09:00 17:00");
            var w = RealMode(gui, "Study")["wallpapers"]!.AsArray();
            Assert.True(w.Count == 1 && (string?)w[0]!["name"] == "sky.png" && (string?)w[0]!["from"] == "09:00", $"added: {w.ToJsonString()}");
        }
        // The bar dragged an hour later on the timeline.
        var bar = FlaUI.Core.Tools.Retry.WhileNull(() => gui.Window.FindFirstDescendant(c => c.ByName("sky.png  09:00–17:00")),
            TimeSpan.FromSeconds(20), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: "no bar sky.png 09:00–17:00").Result!;
        bar.Focus();
        // The timeline's hour labels (every 3 h) stand 16 DIP left of their
        // line (CI: 100 %); from two of them, where 09:30 is and how long an
        // hour is. Start there, on the 5-minute grid a drag snaps to.
        var hours = gui.Window.FindAllDescendants()
            .Select(e => (Name: e.Properties.Name.ValueOrDefault ?? "", Rect: e.BoundingRectangle))
            .Where(x => x.Rect.Bottom <= bar.BoundingRectangle.Top + 2 && x.Rect.Bottom > bar.BoundingRectangle.Top - 60 &&
                        System.Text.RegularExpressions.Regex.IsMatch(x.Name, @"^\d\d:00$"))
            .Select(x => (Hour: int.Parse(x.Name[..2]), X: x.Rect.Left + 16.0)).Where(x => x.Hour is > 0 and < 24).OrderBy(x => x.Hour).ToList();
        Assert.True(hours.Count >= 2, "two hour labels over the bar: " + string.Join(", ", hours));
        var hour = (hours[^1].X - hours[0].X) / (hours[^1].Hour - hours[0].Hour);
        double At(double h) => hours[0].X + (h - hours[0].Hour) * hour;
        var b = bar.BoundingRectangle;
        var from = new System.Drawing.Point((int)Math.Round(At(9.5)), b.Top + b.Height / 2);
        FlaUI.Core.Input.Mouse.Drag(from, new System.Drawing.Point((int)Math.Round(At(10.5)), from.Y));
        gui.WaitFor(() => gui.RanWith("mode", "wallpaper", "Study", "time", "1", "10:00", "18:00"), "kc9k mode wallpaper Study time 1 10:00 18:00 (dragged)");
        gui.Shot("dragged");
        if (real) Assert.Equal("10:00", (string?)RealMode(gui, "Study")["wallpapers"]![0]!["from"]);

        // One setting, seen from both sides.
        Assert.True(On(gui, "OverrideCircadian"), "modes override circadian rhythm at first");
        gui.Invoke("OverrideCircadian");
        gui.WaitFor(() => gui.Ran("wallpaper", "priority", "circadian"), "kc9k wallpaper priority circadian");
        if (real)
        {
            Assert.True((bool?)gui.RealState()["wallpaper"]!["override_modes"] == true, "circadian rhythm overrides modes");
            gui.Page("circadian");
            gui.WaitFor(() => On(gui, "OverrideModes"), "the circadian rhythm page's Override modes is on");
            gui.Invoke("OverrideModes");
            gui.WaitFor(() => gui.Ran("wallpaper", "priority", "modes"), "kc9k wallpaper priority modes");
            Assert.True((bool?)gui.RealState()["wallpaper"]!["override_modes"] == false, "modes win again");
            gui.Page("blocks");
            gui.Invoke("ModeCard_Study");
            gui.WaitFor(() => On(gui, "OverrideCircadian"), "and the mode's switch shows it");
        }

        // Frozen Chicken takes wallpapers too.
        gui.Invoke("ModeBack");
        gui.Invoke("ModeCard_Frozen Chicken");
        gui.Find("Mode_AddWallpaper");
        if (real)
        {
            gui.Invoke("Mode_AddWallpaper");
            gui.PickFile(sky);
            gui.Find("NewWallpaperFrom").AsTextBox().Text = "22:00";
            gui.Find("NewWallpaperTo").AsTextBox().Text = "06:00";
            gui.Invoke(gui.FindButton("Add"));
            gui.WaitFor(() => gui.RanWith("mode", "wallpaper", "Frozen Chicken", "add"), "Frozen Chicken's wallpaper");
            Assert.Single(RealMode(gui, "Frozen Chicken")["wallpapers"]!.AsArray());
        }

        // Removing from a row.
        gui.Invoke("ModeBack");
        gui.Invoke("ModeCard_Study");
        gui.Invoke("WallpaperRemove1");
        gui.WaitFor(() => gui.Ran("mode", "wallpaper", "Study", "remove", "1"), "kc9k mode wallpaper Study remove 1");
        if (real) Assert.Empty(RealMode(gui, "Study")["wallpapers"]!.AsArray());

        // Settings: the wallpaper default.
        gui.Page("settings");
        var choice = gui.Find("WallpaperDefault").AsComboBox();
        Assert.Equal("Latest wallpaper", choice.SelectedItem?.Name);
        Assert.Contains(gui.Window.FindAllDescendants(), e => (e.Properties.Name.ValueOrDefault ?? "").Contains("last place's"));
        choice.Select("Circadian rhythm");
        gui.WaitFor(() => gui.Ran("wallpaper", "default", "circadian"), "kc9k wallpaper default circadian");
        if (!real) return;
        gui.WaitFor(() => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "").Contains("even with circadian rhythm switched off")), "what Circadian rhythm means");
        var plain = gui.Picture("plain.png");
        gui.Find("WallpaperDefault").AsComboBox().Select("Custom");
        gui.PickFile(plain);
        gui.WaitFor(() => gui.Ran("wallpaper", "default", "custom", plain), "Custom asks for a picture");
        var w2 = gui.RealState()["wallpaper"]!;
        Assert.True((string?)w2["default"] == "custom" && (string?)w2["custom_name"] == "plain.png", $"custom: {w2.ToJsonString()}");
        gui.WaitFor(() => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "").Contains("before the computer sleeps")), "and says when it shows");
        gui.Find("WallpaperDefault").AsComboBox().Select("Latest wallpaper");
        gui.WaitFor(() => gui.Ran("wallpaper", "default", "latest"), "back to Latest");
        var count = gui.Commands().Count;
        gui.Find("WallpaperDefault").AsComboBox().Select("Custom");
        gui.WaitFor(() => gui.Commands().Skip(count).Any(c => c.SequenceEqual(["wallpaper", "default", "custom"])), "Custom again, without asking");
        Assert.Equal("custom", (string?)gui.RealState()["wallpaper"]!["default"]);
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }
}
