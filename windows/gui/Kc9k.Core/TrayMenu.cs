// The notification-area icon's menu (W35, §4.10 tray): what is on, then
// quick actions that only strengthen: start a block or a mode, monk mode,
// Frozen Chicken, a new block. Stop, Unlock and breaks are never here, so
// the tray is never an easier way past a block than the window (By Opus).
namespace Kc9k.Core;

/// <summary>One menu line: Run is a kc9k command, Do one of the GUI's own
/// actions (open, quit, new-block, monk, freeze); Sub a submenu.</summary>
public sealed record TrayItem(string Text, bool Enabled = true, IReadOnlyList<string>? Run = null, string? Do = null,
    IReadOnlyList<TrayItem>? Sub = null, bool Separator = false)
{
    public static readonly TrayItem Line = new("", Separator: true);
}

public static class TrayMenu
{
    public static List<TrayItem> Build(GuiState? state, string status)
    {
        var items = new List<TrayItem>();
        foreach (var line in status.Split('\n').Skip(1)) items.Add(new(line, Enabled: false));
        items.Add(TrayItem.Line);
        if (state is not null)
        {
            var off = state.RealBlocks.Where(b => !b.Active).Select(b => new TrayItem(b.Name, Run: ["start", b.Name])).ToList();
            items.Add(off.Count > 0 ? new("Start a block", Sub: off) : new("Start a block (all are on)", Enabled: false));
            var modes = state.Modes.Where(m => !m.Monk && m.Blocks.Count > 0 && m.On < m.Blocks.Count)
                .Select(m => new TrayItem($"{m.Name} ({Words.Plural(m.Blocks.Count, "block")})", Run: ["mode", "on", m.Name])).ToList();
            if (modes.Count > 0) items.Add(new("Start a mode", Sub: modes));
            items.Add(new("Monk mode…", Do: "monk"));
            items.Add(new("Frozen Chicken…", Do: "freeze"));
            items.Add(new("New block…", Do: "new-block"));
            items.Add(TrayItem.Line);
        }
        items.Add(new("Open Killer Chicken 9000", Do: "open"));
        items.Add(new("Quit (blocks stay on)", Do: "quit"));
        return items;
    }
}
