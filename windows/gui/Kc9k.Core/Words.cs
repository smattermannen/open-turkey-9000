// Lengths, times and states in words, as the GTK GUI words them
// (gui/kc9k-gui.py: fmt_*, duration_*, block_pills, lock_words, ...), so the
// two GUIs read alike (R-GUI-3: plain glossary words, short sentences).
using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;

namespace Kc9k.Core;

/// <summary>Preferences that change how words read (§4.10 Preferences).</summary>
public static class Clock
{
    /// <summary>24-hour clock (false: 12-hour, AM/PM).</summary>
    public static bool Use24 { get; set; } = true;

    /// <summary>First day of the week: 0 Monday ... 6 Sunday (WEEKDAYS order).</summary>
    public static int FirstDay { get; set; }

    public const int DayMinutes = 1440;

    /// <summary>HH:MM, as commands and entry fields take it.</summary>
    public static string Text(double minute)
    {
        var m = Mod((int)Math.Round(minute), DayMinutes);
        return $"{m / 60:00}:{m % 60:00}";
    }

    /// <summary>A time of day for reading; end=true shows a window end at midnight as 24:00.</summary>
    public static string Show(double minute, bool end = false)
    {
        var m = (int)Math.Round(minute);
        if (!(end && m >= DayMinutes)) m = Mod(m, DayMinutes);
        if (Use24) return $"{m / 60:00}:{m % 60:00}";
        var hour = Mod(m, DayMinutes) / 60;
        var rest = Mod(m, DayMinutes) % 60;
        return $"{(hour % 12 == 0 ? 12 : hour % 12)}:{rest:00} {(hour < 12 ? "AM" : "PM")}";
    }

    /// <summary>An hour label on a time axis.</summary>
    public static string Hour(int hour)
    {
        if (Use24) return $"{hour:00}:00";
        hour = Mod(hour, 24);
        return $"{(hour % 12 == 0 ? 12 : hour % 12)} {(hour < 12 ? "AM" : "PM")}";
    }

    /// <summary>Minutes after midnight in a typed time of day ('18:00', '18.00', '6:30 pm', '6pm', '18'), or null.</summary>
    public static int? Parse(string? text)
    {
        text = (text ?? "").Trim().ToLowerInvariant().Replace(".", ":").Replace(" ", "");
        var m = Regex.Match(text, @"^(\d{1,2})(?::(\d{2}))?(am|pm)?$");
        if (!m.Success) return null;
        int hour = int.Parse(m.Groups[1].Value), minute = m.Groups[2].Success ? int.Parse(m.Groups[2].Value) : 0;
        if (m.Groups[3].Success)
        {
            if (hour < 1 || hour > 12) return null;
            hour = hour % 12 + (m.Groups[3].Value == "pm" ? 12 : 0);
        }
        if (hour > 23 || minute > 59) return null;
        return hour * 60 + minute;
    }

    /// <summary>The days of the week from the first day: (key, short title).</summary>
    public static IReadOnlyList<(string Key, string Title)> Weekdays
    {
        get
        {
            var all = Words.Weekdays;
            return [.. all.Skip(FirstDay), .. all.Take(FirstDay)];
        }
    }

    public static int Mod(int a, int b) => ((a % b) + b) % b;
}

public static class Words
{
    public static readonly (string Key, string Title)[] Weekdays =
        [("mon", "Mon"), ("tue", "Tue"), ("wed", "Wed"), ("thu", "Thu"), ("fri", "Fri"), ("sat", "Sat"), ("sun", "Sun")];

    /// <summary>The bit of a weekday key in a window's days (bit 0 is Sunday).</summary>
    public static int DayBit(string key) => key switch
    {
        "sun" => 1, "mon" => 2, "tue" => 4, "wed" => 8, "thu" => 16, "fri" => 32, "sat" => 64, _ => 0,
    };

    /// <summary>30 min, 2 h, 1 h 30 min.</summary>
    public static string Minutes(long minutes)
    {
        long hours = minutes / 60, rest = minutes % 60;
        if (hours == 0) return $"{rest} min";
        return $"{hours} h" + (rest != 0 ? $" {rest} min" : "");
    }

    /// <summary>A remaining time, rounded to minutes: '69 days 10 h', '2 h 5 min', '3 min'.</summary>
    public static string Seconds(long seconds)
    {
        if (seconds >= 86400)
        {
            long days = seconds / 86400, hours = seconds % 86400 / 3600;
            return $"{days} day{(days != 1 ? "s" : "")}" + (hours != 0 ? $" {hours} h" : "");
        }
        long h = seconds / 3600, rest = seconds % 3600;
        return h != 0 ? $"{h} h {rest / 60} min" : $"{Math.Max(rest / 60, 1)} min";
    }

    /// <summary>Time spent: '45s', '12m', '3h 04m'.</summary>
    public static string Spent(long seconds)
    {
        if (seconds < 60) return $"{seconds}s";
        if (seconds < 3600) return $"{seconds / 60}m";
        return $"{seconds / 3600}h {seconds % 3600 / 60:00}m";
    }

    /// <summary>A length for reading: '69 days 10 hours 40 minutes', '1 hour', '45 seconds'.</summary>
    public static string Duration(long seconds)
    {
        seconds = Math.Max(0, seconds);
        if (seconds < 60) return $"{seconds} second" + (seconds == 1 ? "" : "s");
        var parts = new List<string>();
        foreach (var (size, unit) in new[] { (86400L, "day"), (3600L, "hour"), (60L, "minute") })
        {
            var count = seconds / size;
            seconds %= size;
            if (count != 0) parts.Add($"{count} {unit}" + (count == 1 ? "" : "s"));
        }
        return string.Join(" ", parts);
    }

    /// <summary>A length as a field shows it: '2d 4h 30m', '45m', '30s'.</summary>
    public static string DurationText(long seconds)
    {
        seconds = Math.Max(0, seconds);
        if (seconds < 60) return $"{seconds}s";
        var parts = new List<string>();
        foreach (var (size, unit) in new[] { (86400L, "d"), (3600L, "h"), (60L, "m") })
        {
            var count = seconds / size;
            seconds %= size;
            if (count != 0) parts.Add($"{count}{unit}");
        }
        return string.Join(" ", parts);
    }

    /// <summary>A preset's label: '15 min', '1 hour', '1 day', '1 week', '30 days'.</summary>
    public static string ShortLength(long seconds)
    {
        foreach (var (size, one, many) in new[] { (604800L, "week", "weeks"), (86400L, "day", "days"), (3600L, "hour", "hours") })
            if (seconds >= size && seconds % size == 0)
            {
                var count = seconds / size;
                return $"{count} {(count == 1 ? one : many)}";
            }
        return seconds >= 60 ? $"{seconds / 60} min" : $"{seconds} s";
    }

    static readonly Dictionary<string, long> DurationUnits = new()
    {
        ["w"] = 604800, ["wk"] = 604800, ["wks"] = 604800, ["week"] = 604800, ["weeks"] = 604800,
        ["d"] = 86400, ["day"] = 86400, ["days"] = 86400,
        ["h"] = 3600, ["hr"] = 3600, ["hrs"] = 3600, ["hour"] = 3600, ["hours"] = 3600,
        ["m"] = 60, ["min"] = 60, ["mins"] = 60, ["minute"] = 60, ["minutes"] = 60,
        ["s"] = 1, ["sec"] = 1, ["secs"] = 1, ["second"] = 1, ["seconds"] = 1,
    };
    static readonly Regex DurationPart = new(@"\G\s*(\d+(?:\.\d+)?)\s*([a-z]*)\s*");
    public const string DurationExamples = "45m, 2h 30m, 3d or 1w";

    /// <summary>R-GUI-1: seconds in a typed length, or null when it can't be read:
    /// '2d 4h', '2 days and 4 hours', '1w', '90m', '1.5 h', '1,5h'. A number alone
    /// counts in `bare` seconds (minutes by default). Empty is 0.</summary>
    public static long? ParseDuration(string? text, int bare = 60)
    {
        text = Regex.Replace((text ?? "").Trim().ToLowerInvariant(), @"(\d),(\d)", "$1.$2");
        text = Regex.Replace(text, @"\band\b|,|\+", " ").Trim();
        if (text.Length == 0) return 0;
        double total = 0;
        var pos = 0;
        while (pos < text.Length)
        {
            var m = DurationPart.Match(text, pos);
            if (!m.Success || m.Index != pos || m.Length == 0) return null;
            var number = double.Parse(m.Groups[1].Value, CultureInfo.InvariantCulture);
            var unit = m.Groups[2].Value;
            if (unit.Length > 0)
            {
                if (!DurationUnits.TryGetValue(unit, out var size)) return null;
                total += number * size;
            }
            else if (pos == 0 && m.Index + m.Length == text.Length) total += number * bare;
            else return null; // "2 4h": which unit is the 2?
            pos = m.Index + m.Length;
        }
        return (long)Math.Round(total, MidpointRounding.ToEven);
    }

    /// <summary>'Thursday 24 December 2099, 18:00' (by the clock preference).</summary>
    public static string Date(DateTime moment) =>
        $"{moment.ToString("dddd", CultureInfo.InvariantCulture)} {moment.Day} {moment.ToString("MMMM yyyy", CultureInfo.InvariantCulture)}, {Clock.Show(moment.Hour * 60 + moment.Minute)}";

    /// <summary>How far a moment is: 'in 3 hours', 'in 73 years'.</summary>
    public static string Until(DateTime moment, DateTime? now = null)
    {
        var seconds = (moment - (now ?? DateTime.Now)).TotalSeconds;
        if (seconds < 0) return "already passed";
        var days = seconds / 86400;
        if (days >= 730) return $"in {(int)Math.Floor(days / 365.25)} years";
        if (days >= 60) return $"in {(int)Math.Floor(days / 30.4)} months";
        var whole = (long)seconds / 60 * 60;
        return "in " + Duration(whole == 0 ? 60 : whole);
    }

    static string D(DateOnly d, string format) => d.ToString(format, CultureInfo.InvariantCulture);

    /// <summary>A time frame in words: 2 – 16 Nov 2026, 28 Oct – 3 Nov 2026.</summary>
    public static string Range(DateOnly first, DateOnly last)
    {
        if (first == last) return D(first, "d MMM yyyy");
        if (first.Year != last.Year) return $"{D(first, "d MMM yyyy")} – {D(last, "d MMM yyyy")}";
        if (first.Month != last.Month) return $"{D(first, "d MMM")} – {D(last, "d MMM yyyy")}";
        return $"{first.Day} – {D(last, "d MMM yyyy")}";
    }

    public const int StatsKeepDays = 365; // enforce.StatsKeepDays

    /// <summary>Why a statistics time frame can't be shown, or null.</summary>
    public static string? StatsRangeProblem(DateOnly first, DateOnly last, DateOnly today)
    {
        if (first > last) return "The first day is after the last.";
        if (last > today) return "The last day is still to come: statistics reach today.";
        var oldest = today.AddDays(-(StatsKeepDays - 1));
        if (first < oldest) return $"Statistics are kept for a year: the first day can be {D(oldest, "d MMM yyyy")} at the earliest.";
        return null;
    }

    public static readonly Dictionary<string, string> LockKindNames = new()
    {
        ["timer"] = "timer lock", ["text"] = "text lock", ["password"] = "password lock",
        ["restart"] = "restart lock", ["delay"] = "delay lock", ["range"] = "range lock",
    };

    static readonly Dictionary<string, string> CardLockWords = new()
    {
        ["text"] = "text lock", ["password"] = "password", ["restart"] = "until restart",
        ["range"] = "range lock", ["delay"] = "delay lock", ["timer"] = "timer lock",
    };

    static string KindName(string k) => LockKindNames.TryGetValue(k, out var n) ? n : k;

    /// <summary>A block's state as pills (on/off, enabled/lifted, each lock with its time).</summary>
    public static List<Pill> BlockPills(Block b)
    {
        var output = new List<Pill>();
        if (!b.Active) output.Add(new("Off", PillKind.Off));
        else if (b.Enabled) output.Add(new("On", PillKind.On));
        else output.Add(new("Lifted now", PillKind.Lifted));
        var locks = new List<(string Kind, Pill Pill)>();
        if (b.LockSeconds > 0) locks.Add(("timer", new($"Locked {Seconds(b.LockSeconds)}", PillKind.Locked)));
        if (b.CharLocked) locks.Add(("text", new("Text lock", PillKind.Locked)));
        if (b.PasswordLocked) locks.Add(("password", new("Password lock", PillKind.Locked)));
        if (b.RestartLocked) locks.Add(("restart", new("Until restart", PillKind.Locked)));
        if (b.DelayLeft > 0) locks.Add(("delay", new($"Unlocks in {Seconds(b.DelayLeft)}", PillKind.Warn)));
        else if (b.DelaySeconds > 0) locks.Add(("delay", new($"Delay {Seconds(b.DelaySeconds)}", PillKind.Locked)));
        if (b.TimerAfter > 0) locks.Add(("timer", new($"Then locked {Seconds(b.TimerAfter)}", PillKind.Locked)));
        if (!string.IsNullOrEmpty(b.RangeLock)) locks.Add(("range", new($"Range {b.RangeLock}", b.RangeOn ? PillKind.Locked : PillKind.Off)));
        // R-LOCK-6: in the order they are unlocked; locks not in it (off now) last.
        var order = b.LockOrder;
        output.AddRange(locks.Select((l, i) => (l, i))
            .OrderBy(x => order.IndexOf(x.l.Kind) is var n && n >= 0 ? n : order.Count).ThenBy(x => x.i)
            .Select(x => x.l.Pill));
        if (b.ScheduleLock) output.Add(new("Schedule lock", PillKind.Locked));
        if (b.PausedSeconds > 0) output.Add(new($"Break {Seconds(b.PausedSeconds)}", PillKind.Lifted));
        if (b.QuotaOpen) output.Add(new("Allowance session", PillKind.Lifted));
        return output;
    }

    /// <summary>What breaks a block allows, in short (§4.2 block list).</summary>
    public static string Breaks(Block b)
    {
        var parts = new List<string>();
        if (b.BreakAllowance > 0)
        {
            var per = b.BreakPer > 0 ? b.BreakPer : 3600;
            var every = per == 3600 ? "hour" : per % 3600 == 0 ? $"{per / 3600} hours" : Seconds(per);
            parts.Add($"{Seconds(b.BreakAllowance)} every {every}");
        }
        if (b.Relock > 0) parts.Add("on unlocking");
        if (b.DelayBreak > 0) parts.Add("after a wait");
        if (b.PomodoroOn > 0) parts.Add("Pomodoro");
        if (b.QuotaLimit > 0) parts.Add("allowance");
        return parts.Count > 0 ? "Breaks: " + string.Join(", ", parts) : "No breaks";
    }

    /// <summary>A block's lock state in short: the lock unlocked next (R-LOCK-6).</summary>
    public static string LockState(Block b)
    {
        if (!b.Held) return "Unlocked";
        var first = b.LockOrder.Count > 0 ? b.LockOrder[0] : null;
        if (b.LockSeconds > 0 && (first is null or "timer")) return $"Locked, {Seconds(b.LockSeconds)} left";
        if (first is not null && CardLockWords.TryGetValue(first, out var w)) return $"Locked: {w}";
        if (b.CharLocked) return "Locked: text lock";
        if (b.PasswordLocked) return "Locked: password";
        if (b.RestartLocked) return "Locked: until restart";
        if (!string.IsNullOrEmpty(b.RangeLock)) return "Locked: range lock";
        return "Locked";
    }

    /// <summary>One line describing a block's state and locks.</summary>
    public static string BlockSummary(Block b)
    {
        var parts = new List<string> { b.Active ? (b.Enabled ? "Active, enabled" : "Active, lifted") : "Inactive" };
        if (b.LockSeconds > 0) parts.Add($"timer lock, {Seconds(b.LockSeconds)} left");
        if (b.CharLocked) parts.Add("text lock");
        if (b.PasswordLocked) parts.Add("password-locked");
        if (b.RestartLocked) parts.Add("on until restart");
        if (b.StopSeconds > 0) parts.Add($"stops in {Seconds(b.StopSeconds)}");
        if (b.DelayLeft > 0) parts.Add($"delay lock unlocks in {Seconds(b.DelayLeft)}");
        else if (b.DelaySeconds > 0) parts.Add($"delay lock of {Seconds(b.DelaySeconds)}");
        if (b.TimerAfter > 0) parts.Add($"timer lock of {Seconds(b.TimerAfter)} after the locks before it");
        if (!string.IsNullOrEmpty(b.RangeLock)) parts.Add($"range lock {b.RangeLock} ({(b.RangeOn ? "on" : "off")} now)");
        if (b.LockOrder.Count > 1) parts.Add("unlock order: " + string.Join(" → ", b.LockOrder.Select(KindName)));
        if (b.PomodoroOn > 0) parts.Add($"Pomodoro {b.PomodoroOn / 60}+{b.PomodoroOff / 60} min");
        if (b.PausedSeconds > 0) parts.Add($"on a break for {Seconds(b.PausedSeconds)}");
        if (b.OnBreak) parts.Add("on a break");
        if (b.Scheduled) parts.Add("automated");
        if (b.Lifted) parts.Add("lifted by its automation now");
        if (b.TodayWindows.Count > 0) parts.Add("today: " + string.Join(", ", b.TodayWindows));
        if (b.QuotaLimit > 0) parts.Add($"allowance {b.QuotaUsed / 60} of {b.QuotaLimit / 60} min");
        if (b.QuotaOpen) parts.Add("allowance session open");
        return string.Join(" · ", parts);
    }

    /// <summary>The parts of a setting lock that hold, as words.</summary>
    public static List<string> SettingLockParts(SettingLock l)
    {
        var parts = new List<string>();
        if (l.LockSeconds > 0) parts.Add($"for {Seconds(l.LockSeconds)}");
        if (l.RestartLocked) parts.Add("until restart");
        if (l.CharLocked) parts.Add("text lock");
        if (l.PasswordLocked) parts.Add("password");
        if (l.TimerAfter > 0) parts.Add($"timer {Seconds(l.TimerAfter)} after the locks before it");
        if (l.Relock > 0 && (l.CharLocked || l.PasswordLocked)) parts.Add($"re-lock {Seconds(l.Relock)} after unlocking");
        if (l.LockOrder.Count > 1) parts.Add("unlock order: " + string.Join(" → ", l.LockOrder.Select(KindName)));
        return parts;
    }

    /// <summary>A supported browser's line in the Browsers list.</summary>
    public static string BrowserStatus(BrowserInfo b)
    {
        if (!b.Installed && b.Found)
            return "Installed, but Killer Chicken 9000 can't give it the extension yet (its Chrome Web Store listing doesn't exist yet), so it counts as an unsupported browser: closed while anything is locked.";
        if (!b.Installed) return "Not installed.";
        var s = b.CheckinSeconds;
        if (s < 0) return "Installed. Extension not seen yet: open it once.";
        if (s < 120) return "Installed. Extension working.";
        if (s < 2 * 86400) return $"Installed. Extension last seen {Seconds(s)} ago.";
        return $"Installed. Extension last seen {s / 86400} days ago.";
    }

    /// <summary>A bypass prevention step in words.</summary>
    public static string Step(NepsisStep step)
    {
        var actions = step.Actions.Select(a =>
        {
            if (a.Action == "mode") return $"run {a.Param}, locked, for {Minutes(a.Minutes)}";
            if (a.Action == "custom") return CustomWords(a.Param) is { Length: > 0 } c ? c : "a custom action";
            var text = (Tables.Consequences.FirstOrDefault(c => c.Key == a.Action).Title ?? a.Action).ToLowerInvariant();
            if (a.Minutes > 0) text += $" for {Minutes(a.Minutes)}";
            return text;
        });
        return $"{step.Count} stopped things within {Minutes(step.Minutes)}: {string.Join(", ", actions)}.";
    }

    /// <summary>A custom action's JSON ([{"open": "..."}, {"run": "..."}]) as "open x; run y".</summary>
    public static string CustomWords(string? json)
    {
        try
        {
            using var doc = System.Text.Json.JsonDocument.Parse(string.IsNullOrEmpty(json) ? "[]" : json);
            return string.Join("; ", doc.RootElement.EnumerateArray()
                .SelectMany(i => i.EnumerateObject().Take(1)).Select(p => $"{p.Name} {p.Value.GetString()}"));
        }
        catch (Exception) { return ""; }
    }

    /// <summary>A bypass prevention setting in words: one line per step of its ladder.</summary>
    public static string Nepsis(Nepsis? n, int? count = null)
    {
        var steps = n?.Steps ?? [];
        if (steps.Count == 0) return "Off.";
        var text = steps.Count == 1 ? Step(steps[0])
            : string.Join("\n", steps.Select((s, i) => $"Step {i + 1}: {Step(s)}"));
        if (n!.QuietMinutes > 0 && steps.Count > 1) text += $"\nFalls back a step after {Minutes(n.QuietMinutes)} without tripping.";
        if (count is not null) text += $" {count} so far.";
        return text;
    }

    /// <summary>The first view's steps (W38): what is done of a first
    /// setup. Empty once a block exists and a supported browser's extension
    /// has checked in, so the Overview stops showing them.</summary>
    public static List<(string Key, string Text, bool Done)> FirstSteps(GuiState s)
    {
        var blocks = s.RealBlocks.ToList();
        var browser = s.Browsers.Any(b => b.Installed && b.CheckinSeconds >= 0);
        if (blocks.Count > 0 && browser) return [];
        return
        [
            ("block", "Make a block: the sites, words and apps to keep away.", blocks.Count > 0),
            ("browser", "Use a supported browser with the extension, so pages are judged as well as addresses.", browser),
            ("start", "Start the block, now or by a schedule. Add locks when you want it to hold.", blocks.Any(b => b.Active)),
        ];
    }

    /// <summary>The levels of custom security (principle 3) as they hold on
    /// Windows (W39): title, what it gives, whether it is on now.</summary>
    public static (string Title, string Detail, bool On)[] ProtectionLevels(bool strict, bool noEscape, bool resetGuard, bool firmwareLocked) =>
    [
        ("Basic", "Blocks and their locks. You keep the administrator account: a lock outlasts an impulse, but where nothing below is on, the account could still remove Killer Chicken 9000.", true),
        ("Strict mode", "During a lock or a freeze the administrator account is denied what would stop Killer Chicken 9000: its service, files and registry keys, the boot and recovery settings. Programs started from Downloads, the Desktop, Documents, the temporary folder or a removable drive are closed.", strict),
        ("Boot and hardware", "The service runs in Safe Mode too, so blocks hold there; Safe Mode stays the way out of a freeze or strict mode's lockdown if something goes wrong, until no-escape closes it. No-escape and factory reset protection turn the recovery environment off (Reset this PC and its command prompt); the installer media guard closes USB image writers; the firmware lock keeps the computer from starting another system.", noEscape || resetGuard || firmwareLocked),
    ];

    /// <summary>The level on now, in a line (W39).</summary>
    public static string ProtectionNow(bool strict, bool noEscape, bool resetGuard, bool firmwareLocked)
    {
        var levels = ProtectionLevels(strict, noEscape, resetGuard, firmwareLocked);
        var on = levels.Where(l => l.On).Select(l => l.Title.ToLowerInvariant()).ToList();
        return "On now: " + string.Join(", ", on) + (on.Count < levels.Length ? ". Nothing stricter is on until you turn it on." : ".");
    }

    /// <summary>Where a ladder stands (§4.1 ladder positions): its step and
    /// how many events count towards it; null when it is off.</summary>
    public static string? LadderPosition(Nepsis? n, int level, int count)
    {
        if (n is null || n.Steps.Count == 0) return null;
        var i = Math.Clamp(level, 0, n.Steps.Count - 1);
        var step = n.Steps[i];
        var where = n.Steps.Count == 1 ? "bypass prevention" : $"bypass prevention step {i + 1} of {n.Steps.Count}";
        return $"{where}: {count} of {step.Count} in {Minutes(step.Minutes)}";
    }

    static readonly Dictionary<string, string> HotkeyModifiers = new()
    {
        ["ctrl"] = "ctrl", ["control"] = "ctrl", ["primary"] = "ctrl", ["alt"] = "alt", ["shift"] = "shift",
        ["super"] = "super", ["win"] = "super", ["meta"] = "super",
    };

    /// <summary>Why keys typed like ctrl+alt+s can't be a hotkey, or null (blocker.ParseAccel's rules).</summary>
    public static string? HotkeyProblem(string text)
    {
        var parts = text.Trim().Split('+');
        var key = parts[^1].Trim();
        if (key.Length == 0) return "Type keys such as ctrl+alt+s or win+F9.";
        var mods = new List<string>();
        foreach (var part in parts[..^1])
        {
            if (!HotkeyModifiers.TryGetValue(part.Trim().ToLowerInvariant(), out var mod))
                return $"“{part.Trim()}” isn't a modifier: use ctrl, alt, shift or win.";
            mods.Add(mod);
        }
        if (!Regex.IsMatch(key, "^[A-Za-z0-9_]+$")) return $"“{key}” isn't a key.";
        if (mods.Count == 0 && !Regex.IsMatch(key.ToUpperInvariant(), "^F([1-9]|1[0-9]|2[0-4])$"))
            return "Add ctrl, alt, shift or win: only F1 to F24 work alone.";
        return null;
    }

    /// <summary>A GTK accelerator such as &lt;Primary&gt;&lt;Alt&gt;s as ctrl+alt+s (the CLI stores them so).</summary>
    public static string HotkeyKeys(string accel)
    {
        foreach (var (tag, word) in new[] { ("<Primary>", "ctrl+"), ("<Control>", "ctrl+"), ("<Alt>", "alt+"), ("<Shift>", "shift+"), ("<Super>", "win+") })
            accel = accel.Replace(tag, word);
        return accel;
    }

    /// <summary>A window in words: 'Mon, Tue 09:00–17:00'.</summary>
    public static string WindowWords(TimeWindow w)
    {
        var days = Clock.Weekdays.Where(d => (w.Days & DayBit(d.Key)) != 0).Select(d => d.Title).ToList();
        var dayText = days.Count == 7 ? "Every day" : days.Count == 0 ? "No day" : string.Join(", ", days);
        return $"{dayText} {Clock.Show(w.Start)}–{Clock.Show(w.End, true)}";
    }

    /// <summary>Days of a window for a command's --days: "mon,tue".</summary>
    public static string DaysArg(int days) =>
        string.Join(",", Weekdays.Where(d => (days & DayBit(d.Key)) != 0).Select(d => d.Key));

    /// <summary>A window as the CLI writes it: day keys Monday first, from, to.</summary>
    public static (List<string> Days, string From, string To) WindowParts(TimeWindow w)
    {
        var days = Weekdays.Where(d => (w.Days & DayBit(d.Key)) != 0).Select(d => d.Key).ToList();
        return (days, Clock.Text(w.Start), w.End >= Clock.DayMinutes ? "24:00" : Clock.Text(w.End));
    }

    /// <summary>Hours a week the windows (not allow windows) cover.</summary>
    public static double HoursAWeek(IEnumerable<TimeWindow> windows)
    {
        var covered = new HashSet<(int, int)>();
        foreach (var w in windows.Where(w => !w.Allow))
            for (var day = 0; day < 7; day++)
                if ((w.Days & (1 << day)) != 0)
                    for (var m = w.Start; m < w.End; m++) covered.Add((day, m));
        return covered.Count / 60.0;
    }

    /// <summary>A schedule's windows in a line, and its hours a week.</summary>
    public static string ScheduleSummary(IReadOnlyList<TimeWindow> windows)
    {
        var parts = new List<string>();
        foreach (var w in windows.Take(3))
        {
            var (names, _, _) = WindowParts(w);
            var days = names.Count < 7 ? string.Join(",", names) : "daily";
            days = days.Replace("mon,tue,wed,thu,fri", "weekdays").Replace("sat,sun", "weekends");
            parts.Add($"{days} {Clock.Show(w.Start)}–{Clock.Show(w.End, true)}");
        }
        if (windows.Count > 3) parts.Add($"{windows.Count - 3} more");
        var hours = HoursAWeek(windows);
        var h = hours > 0 ? $"{Math.Round(hours, MidpointRounding.ToEven):0} h a week" : "no windows yet";
        return (parts.Count > 0 ? string.Join(", ", parts) + " · " : "") + h;
    }

    public static string Plural(int n, string one, string? many = null) => $"{n} {(n == 1 ? one : many ?? one + "s")}";

    /// <summary>"a", "a and b", "a, b and c".</summary>
    public static string And(IEnumerable<string> items)
    {
        var list = items.ToList();
        return list.Count switch
        {
            0 => "",
            1 => list[0],
            _ => string.Join(", ", list.Take(list.Count - 1)) + " and " + list[^1],
        };
    }
}

public enum PillKind { On, Locked, Lifted, Off, Warn }

public sealed record Pill(string Text, PillKind Kind);
