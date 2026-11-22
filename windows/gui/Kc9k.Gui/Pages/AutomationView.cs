// §4.3, §4.5 a block's automation, its own view opened from the block's menu
// or its card (owner 2026-11-17): what switches it (AutomationEditor), its
// autostarts (§3.6) and its schedule lock (§3.4); presets of it.
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class AutomationView
{
    readonly MainWindow window;
    readonly TextBlock title = Ui.Text("", "Kc9kPageTitle", wrap: false);
    readonly AutomationEditor editor;
    readonly EditableList autostarts;
    readonly CheckBox scheduleLock;
    readonly DurationField lead;
    string name = "";
    Controller C => window.Controller;

    public bool ForMode => false;
    public UIElement View { get; }

    public AutomationView(MainWindow window, BlocksPage page)
    {
        this.window = window;
        editor = new AutomationEditor(window, "block");
        autostarts = new EditableList("Autostart", "at 08:00 mon-fri",
            async specs => { foreach (var s in specs) await C.Run(["autostart", name, "add", .. s.Split(' ', StringSplitOptions.RemoveEmptyEntries)]); },
            (_, number) => C.Run(["autostart", name, "remove", number.ToString(CultureInfo.InvariantCulture)]),
            help: "Switches the block on by itself, without locks: at 08:00 [days], once 2026-11-20 08:00, sign-in (also waking up), or next-window.",
            split: t => t.Trim().Length > 0 ? [t.Trim()] : [], id: "Autostarts");
        scheduleLock = new CheckBox { Content = "Schedule lock, from", MinWidth = 0 }.Named("Schedule lock", "ScheduleLock");
        lead = new DurationField(new DurationRule { Minimum = 60, Maximum = 7 * 86400, Zero = "Only while each window is open" }, 0, [900, 3600, 7200], width: 110);
        lead.Named("Schedule lock lead time", "ScheduleLead");
        var lockRow = Ui.Wrap(8, scheduleLock, lead, Ui.Text("before each window"), Ui.Button("Save", () => _ = SaveScheduleLock(), "ScheduleLockSave"),
            Ui.HelpButton("While the lock is on (during the windows, at the block's places, for the lead time before each window, and outside allow windows) the block can't be weakened or switched off. A longer lead time is always allowed; a shorter one or off is refused while the lock is on.", "the schedule lock"));
        var presets = Ui.Wrap(6, Ui.Button("Save as preset…", () => _ = SavePreset(window, "automation", name, false), "AutomationSavePreset"),
            Ui.Button("Use a preset…", () => _ = UsePreset(window, name, false), "AutomationUsePreset"));
        var head = Ui.Side(title, presets);
        View = Ui.Page(Ui.Column(16,
            Ui.Link("← Back to the block", () => page.BackToBlock(), "BackToBlock"),
            head,
            Ui.Section("Automation", "While the block is on, its windows and its places switch it by themselves. Disabled by default: they turn it on, and it is lifted otherwise. Enabled by default: they turn it off, and it is enabled otherwise. With both windows and places, switching needs either one, or both at once. A block with neither is continuous: enabled whenever it is on. A change that lifts a locked block when it is enabled now is refused; one that only adds enabled time is always allowed.", editor),
            Ui.Section("Schedule lock", lockRow),
            Ui.Section("Autostart", autostarts)));
    }

    public void OpenBlock(string block)
    {
        name = block;
        title.Text = $"Automation of {block}";
    }

    public bool Update(GuiState s)
    {
        var b = s.FindBlock(name);
        if (b is null || b.InMode) return false;
        var others = s.RealBlocks.Where(o => o.Name != name).SelectMany(o => o.ScheduleWindows).Where(w => !w.Allow);
        editor.Show(name, b.ScheduleWindows, b.Automation, b.Continuous, b.AppliedSchedules, b.Places, b.Active, "this block", others);
        autostarts.SetItems(b.Autostarts, name);
        if (lead.FocusState == FocusState.Unfocused)
        {
            scheduleLock.IsChecked = b.ScheduleLock;
            lead.SetSeconds(b.ScheduleLead * 60L);
        }
        return true;
    }

    async Task SaveScheduleLock()
    {
        var b = C.State?.FindBlock(name);
        if (scheduleLock.IsChecked == true)
        {
            if (!lead.IsValid)
            {
                await C.Dialogs.Message($"Schedule lock: {lead.Problem}", "Not saved");
                return;
            }
            var seconds = lead.Seconds ?? 0;
            var (ok, _) = await C.Dialogs.ConfirmLock([$"block '{name}': schedule lock, while each window is open" + (seconds > 0 ? $" and {Words.Duration(seconds)} before it" : "")]);
            if (ok) await C.Run(["schedule", "lock", name, "on", "--lead", $"{seconds / 60}m", "--yes"]);
        }
        else if (b?.ScheduleLock == true) await C.Run(["schedule", "lock", name, "off"]);
    }

    /// <summary>Saves an automation (block or mode) as a preset.</summary>
    public static async Task SavePreset(MainWindow window, string kind, string source, bool mode)
    {
        var c = window.Controller;
        var preset = await c.Dialogs.AskText("Save as preset", $"A copy of the automation of {source}: its windows, schedules, places and settings.",
            problem: t => t.Length == 0 ? "Type a name." : null, ok: "Save");
        if (string.IsNullOrEmpty(preset)) return;
        List<string> args = ["preset", "save", kind, source, preset];
        if (mode) args.Add("--mode");
        await c.Run(args, then: o => c.Dialogs.Message(o, "Saved"));
    }

    /// <summary>Uses an automation or schedule preset on a block or mode.</summary>
    public static async Task UsePreset(MainWindow window, string target, bool mode)
    {
        var c = window.Controller;
        var presets = (c.State?.Presets ?? []).Where(p => p.Kind is "automation" or "schedule").ToList();
        if (presets.Count == 0)
        {
            await c.Dialogs.Message("No such presets yet: save one first.", "No presets");
            return;
        }
        var chosen = await c.Dialogs.Choose("Use a preset", "It is copied in; what would weaken a locked block is refused.",
            [.. presets.Select(p => ($"{p.Kind}\n{p.Name}", $"{p.Name} ({p.Kind})"))]);
        if (chosen is null) return;
        var parts = chosen.Split('\n', 2);
        List<string> args = ["preset", "use", parts[1], target, "--kind", parts[0]];
        if (mode) args.Add("--mode");
        await c.Run(args, then: o => c.Dialogs.Message(o, "Done"));
    }
}
