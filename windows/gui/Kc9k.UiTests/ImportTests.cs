// gui_import (§4.3; owner 2026-11-16: one Import button, KC9K reads the
// file's layout), through Windows' own open dialog (W4: file dialogs): a
// phrase set's Import takes phrases, makes rendered terms one word set in
// its editor and offers a list of sites to Blocked; a list's Import reads
// hosts files and adblock rules as sites and sends phrases to a phrase set
// (or leaves them out); From KC9K, Whole internet, All searches containing.
// Against the fake (its `list read` answers are what the real CLI prints
// for these files) and the real kc9k.exe on a scratch database.
using System.Text.Json.Nodes;
using FlaUI.Core.AutomationElements;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class ImportTests
{
    static readonly (string Name, string Text, string Read)[] Files =
    [
        ("phrases.txt", "[eat, nice] [hello, grandpa]\n[big-, strong] [-gorilla, cat]\n",
            """{"phrases":["[eat, nice] [hello, grandpa]","[big-, strong] [-gorilla, cat]"],"entries":[],"sites":[],"terms":[],"skipped":[]}"""),
        ("terms.txt", "casino\npoker\nslot machine\n",
            """{"phrases":[],"entries":["casino","poker","slot machine"],"sites":[],"terms":["casino","poker","slot machine"],"skipped":[]}"""),
        ("hosts.txt", "0.0.0.0 ads.example.net\n0.0.0.0 tracker.example.com\n",
            """{"phrases":[],"entries":["ads.example.net","tracker.example.com"],"sites":["ads.example.net","tracker.example.com"],"terms":[],"skipped":[]}"""),
        ("mixed.txt", "[sunny, warm] [beach]\n||banners.example.org^\nlottery\n",
            """{"phrases":["[sunny, warm] [beach]"],"entries":["banners.example.org","lottery"],"sites":["banners.example.org"],"terms":["lottery"],"skipped":[]}"""),
    ];

    static string PathOf(GuiApp gui, string name) => Path.Combine(gui.Dir, name);

    static void Import(GuiApp gui, string filesButton, string source, string? file)
    {
        gui.Invoke(filesButton);
        gui.Invoke(gui.FindMenuItem("Import…"));
        gui.Invoke("Import_" + source);
        if (file is not null) gui.PickFile(PathOf(gui, file));
    }

    static JsonObject Work(GuiApp gui) => gui.RealBlock("Work");

    static List<string> Entries(GuiApp gui, string list) =>
        [.. Work(gui)["lists"]!.AsArray().First(l => (string?)l!["name"] == list)!["entries"]!.AsArray().Select(e => (string?)e ?? "")];

    static List<string> Phrases(GuiApp gui) =>
        [.. Work(gui)["phrase_sets"]!.AsArray().First(p => (string?)p!["name"] == "Gossip")!["phrases"]!.AsArray().Select(p => (string?)p!["text"] ?? "")];

    static void Close(GuiApp gui, string title)
    {
        gui.FindName(title, 30);
        gui.Invoke(gui.FindButton("OK"));
    }

    [Theory, InlineData(false), InlineData(true)]
    public void ImportsReadTheFilesLayout(bool real)
    {
        using var gui = new GuiApp("import" + (real ? "-real" : ""), scratch: real,
            setup: [["block", "create", "Work"], ["block", "add", "Work", "youtube.com"], ["block", "create", "Other"], ["block", "add-term", "Other", "roulette", "blackjack"],
                ["phrase-set", "create", "Work", "Gossip"], ["phrase-set", "add", "Work", "Gossip", "[celebrity, star] [divorce, scandal]"]],
            prepare: dir =>
            {
                if (!real) ParityRealTests.WorkOutOfStudy(dir);
                foreach (var (name, text, _) in Files) File.WriteAllText(Path.Combine(dir, name), text);
                if (!real) GuiApp.Respond(dir, [.. Files.Select(f => (new[] { "list", "read", Path.Combine(dir, f.Name) }, true, f.Read))]);
            });
        gui.Page("blocks");
        gui.Invoke("BlockCard_Work");

        // Phrases go into the set.
        Import(gui, "Phrases_Gossip_Phrases_Files", "file", "phrases.txt");
        gui.WaitFor(() => gui.Ran("phrase-set", "import", "Work (draft)", "Gossip", PathOf(gui, "phrases.txt")), "kc9k phrase-set import … phrases.txt");
        Close(gui, "Done");
        // Rendered terms become one word set in the editor; nothing is added to the set.
        var before = gui.Commands().Count;
        Import(gui, "Phrases_Gossip_Phrases_Files", "file", "terms.txt");
        Close(gui, "Imported");
        Assert.Contains("casino, poker, slot machine", new[] { 1, 2 }.Select(n => gui.Find($"Phrases_Gossip_Set{n}").AsTextBox().Text));
        Assert.DoesNotContain(gui.Commands().Skip(before), c => c.Contains("import"));
        // A hosts file in a phrase set: offered to Blocked.
        Import(gui, "Phrases_Gossip_Phrases_Files", "file", "hosts.txt");
        gui.FindName("The file lists sites", 30);
        gui.Invoke(gui.FindButton("Add to Blocked"));
        gui.WaitFor(() => gui.Ran("list", "import", "Work (draft)", "Blocked", PathOf(gui, "hosts.txt")), "kc9k list import … Blocked hosts.txt");
        Close(gui, "Done");

        // A list's Import: the phrase to a phrase set, the rule and the term into the list.
        Import(gui, "List_Blocked_Entries_Files", "file", "mixed.txt");
        gui.FindName("Phrases in the file", 30);
        gui.Invoke(gui.FindButton("Add them"));
        gui.WaitFor(() => gui.Ran("phrase-set", "import", "Work (draft)", "Gossip", PathOf(gui, "mixed.txt")) &&
                          gui.Ran("list", "import", "Work (draft)", "Blocked", PathOf(gui, "mixed.txt")), "the phrase to Gossip, the rest to Blocked");
        gui.FindName("Done", 30);
        Assert.DoesNotContain("Left out", gui.Find("MessageText").Name);
        gui.Invoke(gui.FindButton("OK"));
        // "Leave them out": only the entries.
        before = gui.Commands().Count;
        Import(gui, "List_Blocked_Entries_Files", "file", "mixed.txt");
        gui.FindName("Phrases in the file", 30);
        gui.Invoke(gui.FindButton("Leave them out"));
        gui.WaitFor(() => gui.Commands().Skip(before).Any(c => c.Contains("list") && c.Contains("import")), "the entries imported");
        Assert.DoesNotContain(gui.Commands().Skip(before), c => c.Contains("phrase-set"));
        Close(gui, "Done");

        // From KC9K: another list's entries become a word set.
        Import(gui, "Phrases_Gossip_Phrases_Files", "kc9k", null);
        var other = real ? "Other: Rendered terms on page" : "Work: Blocked";
        gui.Invoke(gui.FindName(other, 20));
        gui.Invoke(gui.FindButton("Import"));
        Close(gui, "Imported");
        if (real) Assert.Contains(new[] { 1, 2, 3 }.Select(n => gui.ById($"Phrases_Gossip_Set{n}")?.AsTextBox().Text ?? ""), t => t.Contains("roulette") && t.Contains("blackjack"));

        // §4.3: the whole internet, and all searches containing some words.
        Import(gui, "List_Blocked_Entries_Files", "internet", null);
        gui.Invoke(gui.FindButton("Add"));
        gui.WaitFor(() => gui.Ran("list", "add", "Work (draft)", "Blocked", "*.*"), "Whole internet adds *.*");
        Import(gui, "List_Blocked_Entries_Files", "searches", null);
        gui.Find("AskText").AsTextBox().Text = "poker, slot machine";
        gui.Invoke(gui.FindButton("OK"));
        gui.WaitFor(() => gui.Ran("list", "add", "Work (draft)", "Blocked", "poker", "slot machine"), "the words added");
        Close(gui, "Done");
        gui.Shot("imported");

        if (real)
        {
            gui.Invoke(gui.Find("SaveEdits", 30));
            gui.WaitFor(() => gui.Ran("block", "draft", "Work", "save"), "saved");
            var phrases = Phrases(gui);
            Assert.Contains("[eat, nice] [hello, grandpa]", phrases);
            Assert.Contains("[sunny, warm] [beach]", phrases);
            Assert.Equal(4, phrases.Count);
            var blocked = Entries(gui, "Blocked");
            foreach (var e in new[] { "ads.example.net", "tracker.example.com", "banners.example.org", "lottery", "*.*", "poker", "slot machine" })
                Assert.Contains(e, blocked);
            Assert.DoesNotContain(blocked, e => e.Contains('[') || e.Contains("||"));
        }
        Assert.Null(gui.DesktopByName("Something went wrong"));
    }

    // W55 with the real kc9k.exe and Windows' own open dialog: a project
    // exported by kc9k is imported back through Settings › Import project…
    // after its dry run (nothing locked, so no lock to confirm); its block
    // comes in beside the one there.
    [Fact]
    public void AProjectExportedIsImportedThroughTheOpenDialog()
    {
        using var gui = new GuiApp("project-real", scratch: true, setup: [["block", "create", "Exported"], ["block", "add", "Exported", "example.com"]]);
        var (ok, json) = gui.Cli("project", "export");
        Assert.True(ok, json);
        var file = Path.Combine(gui.Dir, "setup.json");
        File.WriteAllText(file, json[json.IndexOf('{')..]);
        gui.Page("settings");
        gui.Invoke("ProjectImport");
        gui.PickFile(file);
        gui.WaitFor(() => gui.Ran("project", "import", "--dry-run", file), "kc9k project import --dry-run FILE");
        gui.FindName("Import this setup?", 30);
        gui.Shot("dry-run");
        gui.Invoke(gui.FindButton("Import"));
        gui.WaitFor(() => gui.Ran("project", "import", file), "kc9k project import FILE");
        Close(gui, "Imported");
        // A name taken here comes in as name-2 (nothing that exists is changed).
        Assert.Contains(gui.RealState()["blocks"]!.AsArray(), b => (string?)b!["name"] == "Exported-2");
    }
}
