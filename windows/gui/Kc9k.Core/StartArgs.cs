// The GUI's start arguments (W36): what a jump list entry, a shortcut or a
// second start asks the window to show. Only pages and the GUI's own
// dialogs: never a kc9k command (a shortcut that starts or stops a block
// would decide outside the window's questions and confirmations).
namespace Kc9k.Core;

public sealed record StartArgs(string? Page, string? Do, bool Hidden)
{
    public static readonly string[] Actions = ["new-block", "monk", "freeze"];

    /// <summary>The jump list's entries: title, arguments.</summary>
    public static readonly (string Title, string Args)[] JumpList =
    [
        ("Overview", "--page dashboard"), ("Blocks", "--page blocks"), ("New block…", "--do new-block"),
        ("Monk mode…", "--do monk"), ("Frozen Chicken…", "--do freeze"), ("Statistics", "--page stats"),
        ("Settings", "--page settings"),
    ];

    public bool Any => Page is not null || Do is not null;

    /// <summary>Reads --page KEY, --do ACTION and --hidden; anything else is ignored.</summary>
    public static StartArgs Parse(IReadOnlyList<string> args)
    {
        string? page = null, act = null;
        var hidden = false;
        for (var i = 0; i < args.Count; i++)
        {
            switch (args[i])
            {
                case "--page" when i + 1 < args.Count:
                    var p = args[++i];
                    if (Tables.Pages.Any(x => x.Key == p)) page = p;
                    break;
                case "--do" when i + 1 < args.Count:
                    var d = args[++i];
                    if (Actions.Contains(d)) act = d;
                    break;
                case "--hidden":
                    hidden = true;
                    break;
            }
        }
        return new StartArgs(page, act, hidden);
    }

    /// <summary>The same from one command line, as a redirected start gives it.</summary>
    public static StartArgs Parse(string? line) =>
        Parse((line ?? "").Split(' ', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries));
}
