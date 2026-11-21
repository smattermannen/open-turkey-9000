// Typed values (R-GUI-1, R-GUI-2): every length is typed in days, hours and
// minutes; a value out of range is never changed silently: the field says
// why and keeps the text, and the dialog's main button waits until it reads
// right. The widgets (Kc9k.Gui Controls) only show what these rules say.
using System.Globalization;
using System.Text.RegularExpressions;

namespace Kc9k.Core;

public readonly record struct Reading<T>(T? Value, string? Problem) where T : struct
{
    public bool Ok => Problem is null;
}

/// <summary>A length in days, hours and minutes (R-GUI-1). Minimum and
/// maximum are seconds (maximum null: no limit); Zero, when given, is what an
/// empty or 0 field means ('No re-lock'); Bare is what a number alone counts
/// in (60: minutes, 1: seconds).</summary>
public sealed class DurationRule
{
    public long Minimum { get; set; } = 60;
    public long? Maximum { get; set; }
    public string? Zero { get; set; }
    public int Bare { get; set; } = 60;
    public string WhyMax { get; set; } = "";

    public Reading<long> Read(string? text)
    {
        var seconds = Words.ParseDuration(text, Bare);
        if (seconds is null)
            return new(null, Bare == 60 ? $"Type a length such as {Words.DurationExamples}." : "Type a length such as 30s, 90s or 2m.");
        if (seconds == 0 && Zero is not null) return new(0, null);
        if (seconds < Minimum) return new(null, $"At least {Words.Duration(Minimum)}.");
        if (Bare == 60 && seconds % 60 != 0) return new(null, "Use whole minutes (the commands take minutes).");
        if (Maximum is long max && seconds > max)
            return new(null, $"At most {Words.Duration(max)}." + (WhyMax.Length > 0 ? " " + WhyMax : ""));
        return new(seconds, null);
    }

    /// <summary>What a right value means, under the field.</summary>
    public string Describe(long value, string text) =>
        value == 0 && Zero is not null ? (text.Trim().Length == 0 ? "" : Zero) : Words.Duration(value);

    /// <summary>The value as a command takes it: '90m' or '45s'.</summary>
    public string Arg(long seconds) => Bare == 60 ? $"{seconds / 60}m" : $"{seconds}s";

    public string Placeholder => Zero ?? (Bare == 60 ? "e.g. 2h 30m" : "e.g. 90s");
}

/// <summary>A whole number between Minimum and Maximum, never clamped silently.</summary>
public sealed class NumberRule(long minimum, long maximum)
{
    public long Minimum { get; set; } = minimum;
    public long Maximum { get; set; } = maximum;

    public Reading<long> Read(string? text)
    {
        text = (text ?? "").Trim().Replace(" ", "");
        if (!Regex.IsMatch(text, @"^\d+$") || !long.TryParse(text, out var n))
            return new(null, $"Type a whole number from {Minimum} to {Maximum}.");
        if (n < Minimum) return new(null, $"At least {Minimum}.");
        if (n > Maximum) return new(null, $"At most {Maximum}.");
        return new(n, null);
    }
}

/// <summary>R-GUI-2: no timer lock, one for a length, or one until a moment.</summary>
public enum TimerMode { None, For, Until }

/// <summary>Every lock kind for a block or a setting (§3.4), as the lock
/// dialog sets them (the GTK GUI's LockPicker): Chosen() gives the locks as
/// (flags, words) in the dialog's order (timer, restart, password, text,
/// delay, range); RelockFlags() the re-lock.</summary>
public sealed class LockSpec
{
    public static readonly string[] AllKinds = ["timer", "password", "text", "restart", "delay", "range", "relock"];

    public LockSpec(IEnumerable<string>? kinds = null, string done = "stopped", bool relockAny = false)
    {
        Kinds = [.. kinds ?? AllKinds];
        Done = done;
        RelockAny = relockAny;
    }

    public HashSet<string> Kinds { get; }
    public string Done { get; }
    /// <summary>A block's re-lock brings back all its locks, so it goes with any
    /// lock; a setting's only with a password or text lock.</summary>
    public bool RelockAny { get; }
    /// <summary>An active block takes only a longer timer lock or a longer text
    /// lock of random characters.</summary>
    public bool OnlyChars => !Kinds.Contains("relock") && !Kinds.Contains("password");

    public TimerMode Timer { get; set; } = TimerMode.None;
    public string TimerText { get; set; } = "1h";
    public DateTime UntilDay { get; set; } = DateTime.Today.AddDays(1);
    public string UntilTime { get; set; } = Clock.Show((DateTime.Now.Hour) * 60);
    public readonly DurationRule TimerRule = new() { Minimum = 60 };

    public string Password { get; set; } = "";
    public string PasswordAgain { get; set; } = "";

    public bool Typing { get; set; }
    public string TextStyle { get; set; } = "chars"; // chars, words, custom
    public string CharsText { get; set; } = Tables.ChallengeDefault.ToString(CultureInfo.InvariantCulture);
    public readonly NumberRule CharsRule = new(Tables.ChallengeMin, Tables.ChallengeMax);
    public string CustomText { get; set; } = "";

    public bool Restart { get; set; }

    public string DelayText { get; set; } = "";
    public readonly DurationRule DelayRule = new() { Minimum = 60, Maximum = 40 * 86400, Zero = "No delay lock" };

    public bool RangeOn { get; set; }
    public HashSet<string> RangeDays { get; } = ["mon", "tue", "wed", "thu", "fri"];
    public string RangeFrom { get; set; } = Clock.Show(9 * 60);
    public string RangeTo { get; set; } = Clock.Show(17 * 60);
    public bool RangeInvert { get; set; }

    public string RelockText { get; set; } = "";
    public readonly DurationRule RelockRule = new() { Minimum = 60, Maximum = 30 * 86400, Zero = "No re-lock" };

    public DateTime? Until()
    {
        var minute = Clock.Parse(UntilTime);
        if (minute is null) return null;
        return UntilDay.Date.AddMinutes(minute.Value);
    }

    public string? TimerProblem(DateTime? now = null)
    {
        if (Timer == TimerMode.For) return TimerRule.Read(TimerText).Problem;
        if (Timer == TimerMode.Until)
        {
            var moment = Until();
            if (moment is null) return "Type a time of day such as 18:00.";
            if (moment <= (now ?? DateTime.Now)) return "That moment has passed: pick a later date or time.";
        }
        return null;
    }

    public List<string> TimerFlags()
    {
        if (Timer == TimerMode.For && TimerRule.Read(TimerText) is { Ok: true, Value: long s } && s > 0)
            return ["--for", TimerRule.Arg(s)];
        if (Timer == TimerMode.Until && Until() is DateTime u)
            return ["--until", u.ToString("yyyy-MM-dd HH:mm", CultureInfo.InvariantCulture)];
        return [];
    }

    public string TimerWords()
    {
        if (Timer == TimerMode.For && TimerRule.Read(TimerText) is { Ok: true, Value: long s } && s > 0)
            return $"timer lock for {Words.Duration(s)}";
        if (Timer == TimerMode.Until && Until() is DateTime u) return $"timer lock until {Words.Date(u)}";
        return "";
    }

    /// <summary>'mon,tue,wed 09:00-17:00', or null while it can't be read.</summary>
    public string? RangeSpec()
    {
        var days = Words.Weekdays.Where(d => RangeDays.Contains(d.Key)).Select(d => d.Key).ToList();
        int? start = Clock.Parse(RangeFrom), end = Clock.Parse(RangeTo);
        if (days.Count == 0 || start is null || end is null || start == end) return null;
        return $"{string.Join(",", days)} {Clock.Text(start.Value)}-{Clock.Text(end.Value)}";
    }

    long DelaySeconds => Kinds.Contains("delay") && DelayRule.Read(DelayText) is { Ok: true, Value: long d } ? d : 0;

    public string? Problem(DateTime? now = null)
    {
        if (TimerProblem(now) is string t) return t;
        if (Password != PasswordAgain) return "The two passwords differ.";
        if (Typing)
        {
            if (TextStyle == "custom" && CustomText.Trim().Length == 0) return "Type the text for the text lock.";
            if (TextStyle != "custom" && CharsRule.Read(CharsText).Problem is string c) return "Text lock: " + c;
        }
        if (Kinds.Contains("delay") && DelayRule.Read(DelayText).Problem is string d) return "Delay lock: " + d;
        if (RangeOn && RangeSpec() is null)
        {
            if (RangeDays.Count == 0) return "Range lock: pick at least one day.";
            return "Range lock: two different times of day, such as 09:00 and 17:00.";
        }
        if (RelockRule.Read(RelockText).Problem is string r) return "Re-lock: " + r;
        return null;
    }

    public bool RelockAllowed()
    {
        if (Typing || Password.Length > 0) return true;
        return RelockAny && (TimerFlags().Count > 0 || Restart || DelaySeconds > 0 || (RangeOn && RangeSpec() is not null));
    }

    public long RelockSeconds() =>
        RelockRule.Read(RelockText) is { Ok: true, Value: long s } && s > 0 && RelockAllowed() ? s : 0;

    public List<string> RelockFlags()
    {
        var s = RelockSeconds();
        return s > 0 ? ["--relock", $"{s / 60}m"] : [];
    }

    public List<string> TextFlags()
    {
        if (TextStyle == "custom") return ["--lock-text", CustomText];
        var chars = CharsRule.Read(CharsText).Value ?? Tables.ChallengeDefault;
        List<string> args = ["--lock", "--lock-chars", chars.ToString(CultureInfo.InvariantCulture)];
        if (TextStyle == "words") args.Add("--lock-words");
        return args;
    }

    /// <summary>The password, for standard input with --password-stdin.</summary>
    public string Secret => Password;

    /// <summary>The locks asked for, as (flags, words), in the dialog's order.</summary>
    public List<(List<string> Flags, string Words)> Chosen()
    {
        var relock = RelockSeconds();
        var note = relock > 0 && !RelockAny ? $", re-lock {Words.Duration(relock)} after unlocking" : "";
        var output = new List<(List<string>, string)>();
        if (TimerFlags() is { Count: > 0 } tf)
            output.Add((tf, TimerWords() + $" (it can't be {Done} until then, not even with a text lock; time the computer is off doesn't count)"));
        if (Restart && Kinds.Contains("restart")) output.Add((["--restart"], "restart lock, until the computer restarts"));
        if (Password.Length > 0 && Kinds.Contains("password")) output.Add((["--password-stdin"], $"password lock{note}"));
        if (Typing && Kinds.Contains("text"))
        {
            var chars = CharsRule.Read(CharsText).Value ?? Tables.ChallengeDefault;
            var what = TextStyle switch
            {
                "words" => $"{chars} characters of words",
                "custom" => "your own text",
                _ => $"{chars} random characters",
            };
            output.Add((TextFlags(), $"text lock ({what}){note}"));
        }
        if (DelaySeconds > 0) output.Add((["--delay", DelayRule.Arg(DelaySeconds)], $"delay lock, unlocking takes {Words.Duration(DelaySeconds)}"));
        if (RangeOn && Kinds.Contains("range") && RangeSpec() is string spec)
        {
            List<string> f = ["--range", spec];
            if (RangeInvert) f.Add("--range-invert");
            output.Add((f, $"range lock, {(RangeInvert ? "outside " : "")}{spec}"));
        }
        return output;
    }

    /// <summary>R-LOCK-6, R-LOCK-12: behind a lock other than a timer lock (one
    /// chosen above it, or one of before, the kinds already on), a timer lock for
    /// a time counts from when the locks before it are unlocked and a restart
    /// lock needs a restart after them; their confirmation lines say so.</summary>
    public static List<(List<string> Flags, string Words)> Waiting(
        IEnumerable<(List<string> Flags, string Words)> chosen, IEnumerable<string>? before = null)
    {
        var others = (before ?? []).Any(k => k != "timer");
        var output = new List<(List<string>, string)>();
        foreach (var (flags, text) in chosen)
        {
            var words = text;
            var first = flags.Count > 0 ? flags[0] : "";
            if (others && first == "--for")
                words = text.Split(" (")[0] + ", counting from when the locks before it are unlocked (time the computer is off doesn't count)";
            else if (others && first == "--restart")
                words = "restart lock, until a restart after the locks before it are unlocked";
            others = others || (first != "--for" && first != "--until");
            output.Add((flags, words));
        }
        return output;
    }

    /// <summary>A block's re-lock as the lock confirmation shows it (R-LOCK-12).</summary>
    public static List<string> RelockLines(string target, List<string> relockFlags) =>
        relockFlags.Count == 0 ? [] : [$"{target}: re-lock, all its locks come back, in their order, {relockFlags[1]} after it is unlocked"];

    /// <summary>All flags of the chosen locks, re-lock last.</summary>
    public List<string> AllFlags() => [.. Chosen().SelectMany(c => c.Flags), .. RelockFlags()];
}
