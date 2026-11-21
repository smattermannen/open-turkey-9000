// Frozen Chicken (§3.8, §4.8, owner 2026-11-17): a mode among the blocks;
// its view shows freeze now, its scheduled freezes with their locks, and the
// allowed apps of an "allowed apps only" freeze (gui/kc9k-gui.py
// build_device, freeze_dialog). It belongs to the "device" access area.
using System.Globalization;
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class Device
{
    readonly MainWindow window;
    readonly TextBlock status = Ui.Text("");
    readonly ComboBox mode;
    readonly DurationField minutes;
    readonly Button freezeButton;
    readonly EditableList schedules, apps;
    readonly CheckBox lockNew;
    readonly DurationField lead;
    Controller C => window.Controller;
    public UIElement View { get; }

    public Device(MainWindow window)
    {
        this.window = window;
        mode = ModeCombo("How it freezes", "FreezeMode");
        minutes = new DurationField(new DurationRule { Minimum = 60, Maximum = Tables.FreezeMax, WhyMax = "Frozen Chicken lasts at most a week." }, 1800, [1800, 3600, 4 * 3600, 86400]);
        minutes.Named("Frozen Chicken for", "FreezeFor");
        freezeButton = Ui.Button("Start Frozen Chicken…", () => _ = Freeze(), "FreezeStart");
        schedules = new EditableList("", "daily 23:00 24:00", AddSchedule, (_, number) => C.Run(["freeze", "schedule", "remove", number.ToString(CultureInfo.InvariantCulture)]),
            help: "Freeze at set times in the way chosen above, for example every night. Days, start and end; for overnight add two (23:00 24:00 and 00:00 06:00). A locked schedule can't be removed from its lead time before it starts until it ends.",
            split: t => t.Trim().Length > 0 ? [t.Trim()] : [], id: "FreezeSchedules", showTitle: false);
        apps = new EditableList("", "WINWORD.EXE  notepad.exe",
            values => C.Run(["freeze", "apps", "add", .. values]), (value, _) => C.Run(["freeze", "apps", "remove", value]),
            help: "While an 'allowed apps only' freeze runs, every other program of yours is closed; the desktop keeps working. Adding an app isn't possible while a freeze runs or anything is locked.",
            id: "FreezeApps", showTitle: false);
        lockNew = new CheckBox { Content = "Lock new schedules from", MinWidth = 0 }.Named("Lock new freeze schedules", "FreezeLockNew");
        lead = new DurationField(new DurationRule { Minimum = 60, Maximum = 7 * 86400, Zero = "No lead time" }, 7200, width: 110);
        lead.Named("Lead time before each window", "FreezeLead");
        View = Ui.Column(18,
            Ui.Dim("Lock the whole computer for a while: the screen, your session, or the power. It keeps the computer unusable until the time is up, even across restarts, and can't be cancelled, only extended. The time keeps counting while the computer is off. At most a week at a time."),
            Ui.Section("Freeze now", Ui.Wrap(8, mode, Ui.Text("for"), minutes, freezeButton), status),
            Ui.Section("Scheduled freezes", schedules, Ui.Wrap(6, lockNew, lead, Ui.Text("before each window")),
                Ui.Wrap(6, Ui.Text("The selected schedule:"), Ui.Button("Lock…", () => _ = LockSchedule(), "FreezeScheduleLock"),
                    Ui.Button("Unlock…", () => _ = UnlockSchedule(), "FreezeScheduleUnlock"))),
            Ui.Section("Allowed apps", apps));
    }

    static ComboBox ModeCombo(string name, string id)
    {
        var c = new ComboBox { MinWidth = 180 }.Named(name, id);
        foreach (var (key, title) in Tables.FreezeModes) c.Items.Add(new ComboBoxItem { Content = title, Tag = key });
        c.SelectedIndex = 0;
        return c;
    }

    static string Key(ComboBox c) => (c.SelectedItem as ComboBoxItem)?.Tag as string ?? "lock";
    static string Title(ComboBox c) => (c.SelectedItem as ComboBoxItem)?.Content as string ?? "";

    public void Update(GuiState s)
    {
        schedules.SetItems(s.FreezeSchedules);
        apps.SetItems(s.FreezeApps);
        var ways = new Dictionary<string, string> { ["lock"] = "locking the screen", ["logout"] = "signing out", ["shutdown"] = "shutting down", ["apps"] = "allowing only the allowed apps" };
        status.Text = s.FreezeSeconds > 0
            ? $"On for {Words.Seconds(s.FreezeSeconds)} more, {ways.GetValueOrDefault(s.FreezeMode, "")}. Starting again can only extend it."
            : "Off.";
        freezeButton.Content = s.FreezeSeconds > 0 ? "Extend…" : "Start Frozen Chicken…";
    }

    async Task Freeze()
    {
        if (!minutes.IsValid)
        {
            await C.Dialogs.Message($"Frozen Chicken: {minutes.Problem}", "Not started");
            return;
        }
        var effect = Key(mode) switch
        {
            "lock" => "The screen locks now and keeps locking again until the time is up.",
            "logout" => "Every session is signed out now, and signing in again ends at once.",
            "shutdown" => "The computer shuts down now, and again whenever it starts.",
            _ => "Every program of yours that isn't an allowed app is closed now, and again when it starts.",
        };
        if (await C.Dialogs.Confirm($"Freeze the computer for {Words.Duration(minutes.Seconds ?? 0)}?", $"{effect} It can't be cancelled. Save your work first.", "Freeze", destructive: true))
            await C.Run(["freeze", "--for", minutes.Arg(), "--mode", Key(mode), "--yes"]);
    }

    /// <summary>§3.8, R-FREEZE-3: Frozen Chicken switched on asks how and until
    /// when, at most a week ahead.</summary>
    public static async Task FreezeDialog(MainWindow window)
    {
        var c = window.Controller;
        var how = ModeCombo("How it freezes", "FreezeDialogMode");
        var spec = new LockSpec(["timer"], "ended") { Timer = TimerMode.For, TimerText = "1h" };
        spec.TimerRule.Maximum = Tables.FreezeMax;
        spec.TimerRule.WhyMax = "Frozen Chicken lasts at most a week.";
        var until = new TimerLockField(spec, allowNone: false, id: "FreezeDialog");
        var body = Ui.Column(10, Ui.Dim("Until the time is up the computer can't be used as chosen here, also after a restart. It can't be cancelled, only extended. Save your work first."), how, until);
        string? Problem()
        {
            if (spec.TimerProblem() is string p) return p;
            if (spec.Timer == TimerMode.Until && spec.Until() is DateTime u && (u - DateTime.Now).TotalSeconds > Tables.FreezeMax)
                return "Frozen Chicken lasts at most a week: pick an earlier date.";
            return null;
        }
        if (!await c.Dialogs.Ask("Freeze the computer", body, "Freeze", () => Problem() is null, w => until.Changed += w)) return;
        var flags = spec.TimerFlags();
        if (flags.Count == 0) return;
        await c.Run(["freeze", .. flags, "--mode", Key(how), "--yes"]);
    }

    async Task AddSchedule(IReadOnlyList<string> values)
    {
        var parts = values.Count == 1 ? values[0].Split(' ', StringSplitOptions.RemoveEmptyEntries) : [.. values];
        if (parts.Length != 3)
        {
            await C.Dialogs.Message("Type days, start and end, for example: daily 23:00 24:00", "Not added");
            return;
        }
        var locked = lockNew.IsChecked == true;
        if (locked && !lead.IsValid)
        {
            await C.Dialogs.Message($"Lead time: {lead.Problem}", "Not added");
            return;
        }
        var way = Title(mode).ToLowerInvariant();
        bool ok;
        if (locked)
            ok = (await C.Dialogs.ConfirmLock([$"freeze schedule {parts[0]} {parts[1]}-{parts[2]} ({way}): schedule lock, while each window is open" +
                                               (lead.Seconds > 0 ? $" and {Words.Duration(lead.Seconds ?? 0)} before it" : "")])).Ok;
        else
            ok = await C.Dialogs.Confirm($"Freeze {parts[0]} {parts[1]}-{parts[2]}?", $"The computer will be frozen then ({way}).", "Schedule");
        if (!ok) return;
        List<string> args = ["freeze", "schedule", "add", "--days", parts[0], "--from", parts[1], "--to", parts[2], "--mode", Key(mode), "--yes"];
        if (locked) args.AddRange(["--lock-lead", lead.Arg()]);
        await C.Run(args);
    }

    (string Text, int Number)? Chosen()
    {
        var s = C.State?.FreezeSchedules ?? [];
        var picked = schedules.Items.Count > 0 ? SelectedOf(schedules) : null;
        if (picked is null) return null;
        var i = s.IndexOf(picked);
        return i < 0 ? null : (picked, i + 1);
    }

    static string? SelectedOf(EditableList list) => list.SelectedEntry;

    async Task LockSchedule()
    {
        if (Chosen() is not var (text, number))
        {
            await C.Dialogs.Message("Pick a schedule in the list first.", "Nothing picked");
            return;
        }
        var picker = new LockPicker("remove it", "removed", ["timer", "password", "text", "restart", "range", "relock"], id: "FreezeLocks");
        var asked = await LockDialogs.Ask(window, $"Lock the freeze schedule {text}",
            "It can't be removed until the locks end. More locks can be added later, after these; a lock that is on is never changed.", "Lock", picker, needLock: true);
        if (asked is not var (locks, relock, secret) || locks.Count == 0) return;
        if (locks.Count > 1 && await LockDialogs.ChooseOrder(window, locks) is { } ordered) locks = ordered;
        else if (locks.Count > 1) return;
        var (flags, lines) = LockDialogs.Lines($"freeze schedule {text}", locks, relock);
        var (ok, _) = await C.Dialogs.ConfirmLock(lines);
        if (ok) await C.Run(["freeze", "schedule", "lock", number.ToString(CultureInfo.InvariantCulture), .. flags, "--yes"], secret.Length > 0 ? secret + "\n" : null);
    }

    async Task UnlockSchedule()
    {
        if (Chosen() is not var (text, number))
        {
            await C.Dialogs.Message("Pick a schedule in the list first.", "Nothing picked");
            return;
        }
        var locks = C.State?.FreezeScheduleLocks ?? [];
        var info = number - 1 < locks.Count ? locks[number - 1] : new SettingLock();
        List<string> unlock = ["freeze", "schedule", "unlock", number.ToString(CultureInfo.InvariantCulture)];
        if (info.LockOrder.FirstOrDefault() == "password")
        {
            var password = await C.Dialogs.AskText($"Unlock the freeze schedule {text}", "Enter its password.", password: true);
            if (!string.IsNullOrEmpty(password)) await C.Run(unlock, password + "\n");
            return;
        }
        if (!info.CharLocked)
        {
            await C.Dialogs.Message("Its first lock isn't a text or password lock: it ends by itself.", "Not unlocked");
            return;
        }
        await C.Run(unlock, then: o => C.Dialogs.Message(o, "Unlocked"), timeout: TimeSpan.FromHours(6));
    }
}
