// §4.3 (owner 2026-11-17): an opened block is always a draft. These commands
// change what its tabs show; run on the opened block they go to its draft
// (made at the first change, `kc9k block draft`), and apply only with Save
// edits. The table says where the block's name is in each command, as the
// GTK GUI's DRAFT_EDITS and DRAFT_READS do.
namespace Kc9k.Core;

public static class Drafts
{
    /// <summary>The name of a block's draft (cli draftName).</summary>
    public static string NameOf(string block) => block + " (draft)";

    static readonly string[] BlockEdits =
    [
        "add", "add-exception", "add-term", "add-keyword", "add-pattern", "remove-site", "remove-entry",
        "remove-exception", "threshold", "action", "add-app", "remove-app", "add-title", "remove-title",
        "import-list", "trigger",
    ];
    static readonly string[] ListEdits =
        ["create", "rename", "delete", "add", "remove", "scope", "where", "action", "threshold", "hits", "rate", "test", "import"];
    static readonly string[] PhraseEdits =
        ["create", "add", "remove", "change", "scope", "action", "threshold", "test", "rename", "delete", "import"];

    /// <summary>Where the block's name is in a command that edits what an
    /// opened block shows, or null.</summary>
    public static int? EditPosition(IReadOnlyList<string> args)
    {
        if (args.Count >= 2 && args[0] == "preset" && args[1] == "use")
        {
            var k = IndexOf(args, "--kind");
            if (k >= 0 && k + 1 < args.Count && args[k + 1] == "lists") return 3;
        }
        if (args.Count == 0) return null;
        string? sub = args.Count > 1 ? args[1] : null;
        switch (args[0])
        {
            case "block": return sub is not null && BlockEdits.Contains(sub) ? 2 : null;
            case "list": return sub == "copy" ? 4 : sub is not null && ListEdits.Contains(sub) ? 2 : null;
            case "phrase-set": return sub is not null && PhraseEdits.Contains(sub) ? 2 : null;
            case "social": return 1;
            case "allowance": return sub is "set" or "remove" ? 2 : null;
            case "pomodoro": return sub is "set" or "off" ? 2 : null;
            case "break": return sub is "reward" or "text" or "delay" ? 2 : null;
            case "bypass":
                if (sub is "set" or "off")
                {
                    var b = IndexOf(args, "--block");
                    return b >= 0 ? b + 1 : null;
                }
                return null;
        }
        return null;
    }

    /// <summary>Where the block's name is in a command that reads what an
    /// opened block shows (the draft's when there is one), or null.</summary>
    public static int? ReadPosition(IReadOnlyList<string> args)
    {
        if (args.Count >= 3 && args[0] == "preset" && args[1] == "save" && args[2] == "lists") return 3;
        if (args.Count < 2) return null;
        return (args[0], args[1]) switch
        {
            ("block", "export-list") => 2,
            ("list", "export" or "show") => 2,
            ("phrase-set", "export" or "show") => 2,
            _ => null,
        };
    }

    /// <summary>The command with the block's name changed to its draft's, or
    /// null when the command isn't one that goes to the opened block's draft.</summary>
    public static List<string>? ToDraft(IReadOnlyList<string> args, string block, bool edit)
    {
        var at = edit ? EditPosition(args) : ReadPosition(args);
        if (at is not int i || i >= args.Count || args[i] != block) return null;
        var moved = new List<string>(args) { [i] = NameOf(block) };
        return moved;
    }

    static int IndexOf(IReadOnlyList<string> args, string what)
    {
        for (var i = 0; i < args.Count; i++) if (args[i] == what) return i;
        return -1;
    }
}
