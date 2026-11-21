// Things more than one page starts (the GTK GUI's create_block, monk_mode).
using Kc9k.Core;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public static class Actions
{
    /// <summary>§4.2: a new block, empty or from a block preset (§3.2).</summary>
    public static async Task CreateBlock(MainWindow window)
    {
        var c = window.Controller;
        var templates = c.State?.Templates ?? [];
        string? template = null;
        if (templates.Count > 0)
        {
            var choice = await c.Dialogs.Choose("New block", "Start empty or from a block preset.",
                [("", "Empty"), .. templates.Select(t => (t, "Block preset: " + t))], "");
            if (choice is null) return;
            template = choice.Length > 0 ? choice : null;
        }
        var name = await c.Dialogs.AskText("New block", "A short name, such as Social media.",
            problem: t => t.Length == 0 ? "Type a name." : c.State?.FindBlock(t) is not null ? "A block has that name already." : null,
            ok: "Create");
        if (string.IsNullOrEmpty(name)) return;
        List<string> args = ["block", "create", name];
        if (template is not null) args.AddRange(["--preset", template]);
        if (await c.Run(args)) window.OpenBlock(name);
    }

    /// <summary>§3.8 Monk mode for a while: a lock like any other (R-LOCK-12).</summary>
    public static async Task MonkMode(MainWindow window)
    {
        var c = window.Controller;
        var seconds = await c.Dialogs.AskDuration("Monk mode",
            "Every website is blocked and the internet connection is cut, for this long. It can't be ended early; a running monk mode can only be extended.",
            3600, new DurationRule { Minimum = 60, Maximum = 30 * 86400 }, [1800, 3600, 4 * 3600, 86400]);
        if (seconds is not long s) return;
        var until = DateTime.Now.AddSeconds(s);
        var (ok, _) = await c.Dialogs.ConfirmLock([$"monk mode (no websites, no internet): {Words.Duration(s)}, until {Words.Date(until)}"]);
        if (ok) await c.Run(["monk", "--for", $"{s / 60}m", "--yes"]);
    }
}
