// Kc9k.Core against the GTK GUI's behaviour (gui/kc9k-gui.py) and the real
// CLI's output (fixtures captured by fixtures/make-fixtures.sh).
using FakeKc9k;
using Kc9k.Core;
using Xunit;

namespace Kc9k.Tests;

public class WordsTests
{
    [Theory]
    [InlineData("2d 4h", 60, 187200L)]
    [InlineData("2 days and 4 hours", 60, 187200L)]
    [InlineData("1w", 60, 604800L)]
    [InlineData("90m", 60, 5400L)]
    [InlineData("1.5 h", 60, 5400L)]
    [InlineData("1,5h", 60, 5400L)]
    [InlineData("45", 60, 2700L)]
    [InlineData("45", 1, 45L)]
    [InlineData("", 60, 0L)]
    [InlineData("300m", 60, 18000L)]
    public void ParsesDurations(string text, int bare, long seconds) =>
        Assert.Equal(seconds, Words.ParseDuration(text, bare));

    [Theory]
    [InlineData("2 4h")]
    [InlineData("3 parsecs")]
    [InlineData("h")]
    public void RefusesDurations(string text) => Assert.Null(Words.ParseDuration(text));

    [Fact]
    public void WordsLengths()
    {
        Assert.Equal("2 days 4 hours", Words.Duration(187200));
        Assert.Equal("1 hour", Words.Duration(3600));
        Assert.Equal("45 seconds", Words.Duration(45));
        Assert.Equal("2d 4h 30m", Words.DurationText(189000));
        Assert.Equal("30s", Words.DurationText(30));
        Assert.Equal("69 days 10 h", Words.Seconds(69 * 86400 + 10 * 3600 + 40 * 60));
        Assert.Equal("2 h 5 min", Words.Seconds(7500));
        Assert.Equal("1 min", Words.Seconds(10));
        Assert.Equal("3h 04m", Words.Spent(3 * 3600 + 4 * 60));
        Assert.Equal("1 week", Words.ShortLength(604800));
        Assert.Equal("30 days", Words.ShortLength(30 * 86400));
        Assert.Equal("1 h 30 min", Words.Minutes(90));
    }

    [Fact]
    public void ReadsClocks()
    {
        Assert.Equal(18 * 60, Clock.Parse("18:00"));
        Assert.Equal(18 * 60, Clock.Parse("18.00"));
        Assert.Equal(18 * 60 + 30, Clock.Parse("6:30 pm"));
        Assert.Equal(18 * 60, Clock.Parse("6pm"));
        Assert.Equal(0, Clock.Parse("12am"));
        Assert.Null(Clock.Parse("25:00"));
        Assert.Null(Clock.Parse("13pm"));
        Clock.Use24 = false;
        try { Assert.Equal("6:30 PM", Clock.Show(18 * 60 + 30)); }
        finally { Clock.Use24 = true; }
        Assert.Equal("24:00", Clock.Show(1440, end: true));
        Assert.Equal("00:00", Clock.Show(1440));
    }

    [Fact]
    public void WordsTimeFrames()
    {
        Assert.Equal("2 – 16 Nov 2026", Words.Range(new(2026, 11, 2), new(2026, 11, 16)));
        Assert.Equal("28 Oct – 3 Nov 2026", Words.Range(new(2026, 10, 28), new(2026, 11, 3)));
        Assert.Equal("in 73 years", Words.Until(new DateTime(2099, 12, 24, 18, 0, 0), new DateTime(2026, 11, 21)));
        Assert.NotNull(Words.StatsRangeProblem(new(2026, 11, 3), new(2026, 11, 2), new(2026, 11, 21)));
        Assert.Null(Words.StatsRangeProblem(new(2026, 11, 1), new(2026, 11, 21), new(2026, 11, 21)));
    }

    [Fact]
    public void ChecksHotkeys()
    {
        Assert.Null(Words.HotkeyProblem("ctrl+alt+s"));
        Assert.Null(Words.HotkeyProblem("F9"));
        Assert.Null(Words.HotkeyProblem("win+F9"));
        Assert.NotNull(Words.HotkeyProblem("s"));
        Assert.NotNull(Words.HotkeyProblem("hyper+s"));
        Assert.Equal("ctrl+alt+w", Words.HotkeyKeys("<Primary><Alt>w"));
    }

    [Fact]
    public void SummarisesSchedules()
    {
        var weekdays = new TimeWindow { Days = 62, Start = 540, End = 1020 };
        var weekend = new TimeWindow { Days = 65, Start = 0, End = 1440 };
        Assert.Equal("weekdays 09:00–17:00, weekends 00:00–24:00 · 88 h a week", Words.ScheduleSummary([weekdays, weekend]));
        Assert.Equal("no windows yet", Words.ScheduleSummary([]));
        Assert.Equal("mon,tue,wed,thu,fri", Words.DaysArg(62));
    }

    [Fact]
    public void SplitsEntries()
    {
        Assert.Equal(["a.com", "b.com"], Entries.Split("a.com  b.com"));
        Assert.Equal(["cars (red, big)"], Entries.Split("cars (red, big)"));
        Assert.Equal(["(red, big) cars"], Entries.Split("(red, big) cars"));
        Assert.NotNull(Entries.ParseCombination("(red, big) (car, bike)"));
        Assert.Null(Entries.ParseCombination("(red)"));
    }
}

public class CircadianTests
{
    [Fact]
    public void ReadsTheRealView()
    {
        var v = CircadianView.Parse(File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "fixtures", "circadian.json")));
        Assert.Equal(["backlight", "software", "kelvin"], v.Lanes.Select(l => l.Name));
        Assert.Contains(v.Presets, p => p.Name == "natural");
        Assert.Equal("sky.png", Assert.Single(v.Wallpapers).Name);
        Assert.Equal("system", v.Dark.Mode);
        var kelvin = v.Lanes[2];
        Assert.Equal(6500, Clips.Value(kelvin.Points, 12 * 60));
        Assert.Equal(2300, Clips.Value(kelvin.Points, 2 * 60));
        Assert.StartsWith("04:30=2300.0 06:00=3400.0~0.50", Clips.Text(kelvin.Points));
    }
}

public class SeriesTests
{
    [Fact]
    public void AColourFollowsItsName()
    {
        var series = new Series();
        var first = series.Assign(["a", "b", "c"]);
        var second = series.Assign(["c", "a", "d"]);
        Assert.Equal(first["a"], second["a"]);
        Assert.Equal(first["c"], second["c"]);
        Assert.NotEqual(first["b"], second["d"]); // b's slot is remembered: d takes a fresh one
        var many = series.Assign([.. Enumerable.Range(0, 9).Select(i => "n" + i)]);
        Assert.Equal(2, many.Values.Count(v => v is null));
        Assert.Equal(7, many.Values.Where(v => v is not null).Distinct().Count());
    }
}

public class StateTests
{
    static string Fixture(string name) => File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "fixtures", name));

    [Fact]
    public void ReadsTheRealGuiState()
    {
        var s = GuiState.Parse(Fixture("gui-state.json"));
        Assert.Contains(s.Blocks, b => b.Name == "Work");
        var work = s.FindBlock("Work")!;
        Assert.True(work.Active);
        Assert.True(work.Held);
        Assert.Equal(["timer"], work.LockOrder);
        Assert.Equal("Study", work.Mode);
        Assert.Contains(work.Lists, l => l.Name == "Shopping" && l.Entries.Contains("amazon.com"));
        Assert.Contains(work.Lists, l => l.IsExceptions && l.Entries.Contains("music.youtube.com"));
        var gossip = Assert.Single(work.PhraseSets);
        Assert.Equal("[celebrity, star] [divorce, scandal]", gossip.Phrases[0].Text);
        Assert.Equal(["celebrity", "star"], gossip.Phrases[0].Sets[0].Terms);
        Assert.StartsWith("Locked, ", Words.LockState(work));
        var social = s.FindBlock("Social media")!;
        Assert.Equal(["timer", "delay"], social.LockOrder);
        Assert.True(social.Automation.DefaultOn);
        Assert.True(social.ScheduleWindows[0].Allow);
        Assert.Contains(s.Modes, m => m.Monk);
        Assert.Contains(s.Modes, m => m.Freeze && m.Name == Tables.FrozenChicken);
        Assert.Equal(["Games", "Work"], s.FindMode("Study")!.Blocks);
        Assert.Equal("Weekend", Assert.Single(s.Schedules).Name);
        Assert.Equal(65, s.Schedules[0].Windows[0].Days); // Saturday and Sunday
        Assert.True(s.Browser["safesearch-google"]);
        Assert.True(s.AnythingLocked);
        Assert.Equal([2, 5], s.Notifications.Advance);
        Assert.Equal("warning", s.Nepsis.Steps[0].Actions[0].Action);
        Assert.Equal("ctrl+alt+w", Words.HotkeyKeys(s.Hotkeys[0].Keys));
        Assert.NotEmpty(s.SocialPresets);
    }

    [Fact]
    public void ReadsTheRealStats()
    {
        var stats = StatsOutput.Parse(Fixture("stats.json"));
        Assert.True(stats.Enabled);
        Assert.Contains(stats.Rows, r => r.Kind == "site" && r.Name == "youtube.com");
        Assert.Contains(stats.Rows, r => r.Kind == "blocked-block" && r.Name == "Work");
    }

    [Fact]
    public void WordsBlocksAsTheGtkGuiDoes()
    {
        var s = GuiState.Parse(Fixture("gui-state.json"));
        var evening = s.FindBlock("Evening")!;
        Assert.Equal("Breaks: Pomodoro", Words.Breaks(evening));
        Assert.Equal("Unlocked", Words.LockState(evening));
        Assert.Equal(new Pill("Lifted now", PillKind.Lifted), Words.BlockPills(evening)[0]);
        var social = s.FindBlock("Social media")!;
        var pills = Words.BlockPills(social);
        Assert.StartsWith("Locked ", pills[1].Text); // the timer lock first: R-LOCK-6 order
        Assert.StartsWith("Delay ", pills[2].Text);
        Assert.Equal("Breaks: allowance", Words.Breaks(s.FindBlock("Games")!));
    }
}

public class LockSpecTests
{
    [Fact]
    public void FlagsInTheDialogsOrder()
    {
        var spec = new LockSpec(relockAny: true)
        {
            Timer = TimerMode.For, TimerText = "2d 4h", Restart = true, Password = "pw", PasswordAgain = "pw",
            Typing = true, TextStyle = "words", CharsText = "500", DelayText = "1h", RangeOn = true, RelockText = "30m",
        };
        Assert.Null(spec.Problem());
        var chosen = spec.Chosen();
        Assert.Equal(["--for", "3120m"], chosen[0].Flags);
        Assert.Equal(["--restart"], chosen[1].Flags);
        Assert.Equal(["--password-stdin"], chosen[2].Flags);
        Assert.Equal(["--lock", "--lock-chars", "500", "--lock-words"], chosen[3].Flags);
        Assert.Equal(["--delay", "60m"], chosen[4].Flags);
        Assert.Equal(["--range", "mon,tue,wed,thu,fri 09:00-17:00"], chosen[5].Flags);
        Assert.Equal(["--relock", "30m"], spec.RelockFlags());
        Assert.Equal("pw", spec.Secret);
    }

    [Fact]
    public void SaysWhatIsWrong()
    {
        var spec = new LockSpec { Password = "a", PasswordAgain = "b" };
        Assert.Equal("The two passwords differ.", spec.Problem());
        spec = new LockSpec { Timer = TimerMode.For, TimerText = "30s" };
        Assert.Equal("At least 1 minute.", spec.Problem());
        spec = new LockSpec { Timer = TimerMode.Until, UntilDay = new DateTime(2000, 1, 1), UntilTime = "18:00" };
        Assert.Equal("That moment has passed: pick a later date or time.", spec.Problem());
        spec = new LockSpec { Timer = TimerMode.Until, UntilDay = new DateTime(2099, 12, 24), UntilTime = "18:00" };
        Assert.Null(spec.Problem());
        Assert.Equal(["--until", "2099-12-24 18:00"], spec.TimerFlags());
        Assert.Equal("timer lock until Thursday 24 December 2099, 18:00", spec.TimerWords());
        spec = new LockSpec { RangeOn = true };
        spec.RangeDays.Clear();
        Assert.Equal("Range lock: pick at least one day.", spec.Problem());
    }

    [Fact]
    public void SettingRelockOnlyWithPasswordOrText()
    {
        var spec = new LockSpec(relockAny: false) { Timer = TimerMode.For, TimerText = "1h", RelockText = "30m" };
        Assert.Empty(spec.RelockFlags());
        spec.Typing = true;
        Assert.Equal(["--relock", "30m"], spec.RelockFlags());
        Assert.EndsWith(", re-lock 30 minutes after unlocking", spec.Chosen()[1].Words);
    }

    [Fact]
    public void TimerBehindOtherLocksWaits()
    {
        var spec = new LockSpec { Timer = TimerMode.For, TimerText = "1h" };
        var waiting = LockSpec.Waiting(spec.Chosen(), ["password"]);
        Assert.Equal("timer lock for 1 hour, counting from when the locks before it are unlocked (time the computer is off doesn't count)", waiting[0].Words);
        Assert.Equal(spec.Chosen()[0].Words, LockSpec.Waiting(spec.Chosen(), ["timer"])[0].Words);
    }

    [Fact]
    public void DurationRuleNeverClamps()
    {
        var rule = new DurationRule { Minimum = 60, Maximum = 86400, WhyMax = "A day at most." };
        Assert.Equal("At most 1 day. A day at most.", rule.Read("2d").Problem);
        Assert.Equal("Use whole minutes (the commands take minutes).", rule.Read("90s").Problem);
        Assert.Equal(5400, rule.Read("90m").Value);
        var zero = new DurationRule { Zero = "No re-lock" };
        Assert.Equal(0, zero.Read("").Value);
        Assert.Equal("90m", zero.Arg(5400));
        Assert.Equal("At most 5000.", new NumberRule(1, 5000).Read("6000").Problem);
    }
}

public class CliTests
{
    sealed class Recorder : ICliProcess
    {
        public List<(IReadOnlyList<string> Args, string? Stdin)> Calls { get; } = [];
        public Task<CliResult> RunAsync(IReadOnlyList<string> args, string? stdin = null, TimeSpan? timeout = null)
        {
            Calls.Add((args, stdin));
            return Task.FromResult(new CliResult(true, ""));
        }
    }

    [Fact]
    public async Task PasswordsGoFirstOnStandardInput()
    {
        var r = new Recorder();
        var session = new CliSession(r) { SettingsPassword = "set", SettingsPasswordSet = true };
        session.AccessPasswords.Add("acc");
        await session.RunAsync(["start", "Work", "--password-stdin"], "lockpw\n");
        Assert.Equal(["--access-lines", "1", "--settings-line", "start", "Work", "--password-stdin"], r.Calls[0].Args);
        Assert.Equal("acc\nset\nlockpw\n", r.Calls[0].Stdin);
    }

    [Fact]
    public void RecognisesPasswordRefusals()
    {
        Assert.Equal("settings", new CliResult(false, "Error: this needs the access password (settings)").AccessArea);
        Assert.True(new CliResult(false, "Error: this change needs the settings password").NeedsSettingsPassword);
        Assert.Null(new CliResult(true, "needs the access password (x)").AccessArea);
    }

    [Fact]
    public void ChangesToAnOpenedBlockGoToItsDraft()
    {
        Assert.Equal(["block", "add", "Work (draft)", "x.com"], Drafts.ToDraft(["block", "add", "Work", "x.com"], "Work", true));
        Assert.Equal(["list", "copy", "Other", "Blocked", "Work (draft)", "Blocked"],
            Drafts.ToDraft(["list", "copy", "Other", "Blocked", "Work", "Blocked"], "Work", true));
        Assert.Equal(["bypass", "set", "--block", "Work (draft)"], Drafts.ToDraft(["bypass", "set", "--block", "Work"], "Work", true));
        Assert.Equal(["preset", "use", "Mine", "Work (draft)", "--kind", "lists"],
            Drafts.ToDraft(["preset", "use", "Mine", "Work", "--kind", "lists"], "Work", true));
        Assert.Null(Drafts.ToDraft(["start", "Work"], "Work", true)); // the block's menu applies at once
        Assert.Null(Drafts.ToDraft(["block", "add", "Other", "x.com"], "Work", true));
        Assert.Equal(["list", "show", "Work (draft)"], Drafts.ToDraft(["list", "show", "Work"], "Work", false));
    }

    [Fact]
    public async Task TheFakeReplaysAndRecords()
    {
        var dir = Path.Combine(Path.GetTempPath(), "kc9k-fake-" + Guid.NewGuid());
        Directory.CreateDirectory(dir);
        try
        {
            foreach (var f in Directory.GetFiles(Path.Combine(AppContext.BaseDirectory, "fixtures")))
                File.Copy(f, Path.Combine(dir, Path.GetFileName(f)));
            File.WriteAllText(Path.Combine(dir, "responses.json"),
                """[{"args": ["stop", "Work"], "ok": false, "output": "Error: block 'Work' is locked"}]""");
            var (code, output) = FakeCli.Run(["--access-lines", "1", "gui-state"], "pw\n", dir);
            Assert.Equal(0, code);
            Assert.NotNull(GuiState.Parse(output).FindBlock("Work"));
            (code, output) = FakeCli.Run(["stop", "Work"], "", dir);
            Assert.Equal(1, code);
            Assert.Contains("locked", output);
            var log = FakeCli.ReadLog(Path.Combine(dir, "commands.log"));
            Assert.Equal(["stop", "Work"], log[1].Args);
            Assert.Equal("pw\n", log[0].Stdin);
            await Task.CompletedTask;
        }
        finally { Directory.Delete(dir, true); }
    }

    // W30: a browser on the computer that KC9K can't give the extension yet.
    [Fact]
    public void BrowserFoundButNotSupported()
    {
        Assert.Equal("Not installed.", Words.BrowserStatus(new BrowserInfo { Id = "edge" }));
        Assert.Contains("can't give it the extension yet", Words.BrowserStatus(new BrowserInfo { Id = "edge", Found = true }));
        Assert.StartsWith("Installed. Extension working", Words.BrowserStatus(new BrowserInfo { Id = "chrome", Installed = true, Found = true, CheckinSeconds = 5 }));
    }

    // W31: a ladder's position in words.
    [Fact]
    public void LadderPositionInWords()
    {
        var n = new Nepsis { Steps = [new() { Count = 3, Minutes = 10 }, new() { Count = 2, Minutes = 30 }] };
        Assert.Null(Words.LadderPosition(new Nepsis(), 0, 0));
        Assert.Equal("bypass prevention step 2 of 2: 1 of 2 in 30 min", Words.LadderPosition(n, 1, 1));
        Assert.Equal("bypass prevention step 2 of 2: 0 of 2 in 30 min", Words.LadderPosition(n, 7, 0));
        Assert.Equal("bypass prevention: 2 of 5 in 10 min", Words.LadderPosition(new Nepsis { Steps = [new() { Count = 5, Minutes = 10 }] }, 0, 2));
    }
}
