// The state the GUI shows, as `kc9k gui-state` prints it (internal/cli
// freeze.go guiState and its parts). Property names map to the JSON by
// snake_case (Kc9kJson.Options); a field the CLI adds later is ignored until
// a property is added here, and one it drops reads as its default.
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Kc9k.Core;

public static class Kc9kJson
{
    public static readonly JsonSerializerOptions Options = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        PropertyNameCaseInsensitive = false,
        NumberHandling = JsonNumberHandling.AllowReadingFromString,
        ReadCommentHandling = JsonCommentHandling.Skip,
        AllowTrailingCommas = true,
    };

    public static T Parse<T>(string json) where T : new() =>
        JsonSerializer.Deserialize<T>(json, Options) ?? new T();
}

public sealed class GuiState
{
    public long FreezeSeconds { get; set; }
    public string FreezeMode { get; set; } = "";
    public List<string> FreezeApps { get; set; } = [];
    public bool Strict { get; set; }
    public bool NoEscape { get; set; }
    public bool ResetGuard { get; set; }
    public bool Stats { get; set; }
    public bool StatsSkipPrivate { get; set; }
    public List<string> Accounts { get; set; } = [];
    public string Librewolf { get; set; } = "";
    public bool Chrome { get; set; }
    public string ExtensionStatus { get; set; } = "";
    public bool ExtensionMissing { get; set; }
    public List<string> ExtensionMissingIn { get; set; } = [];
    public bool SettingsPassword { get; set; }
    public List<string> FreezeSchedules { get; set; } = [];
    public List<SettingLock> FreezeScheduleLocks { get; set; } = [];
    public Dictionary<string, bool> Browser { get; set; } = [];
    public Dictionary<string, SettingLock> BrowserLocks { get; set; } = [];
    public List<Block> Blocks { get; set; } = [];
    public List<Hotkey> Hotkeys { get; set; } = [];
    public Nepsis Nepsis { get; set; } = new();
    public int NepsisCount { get; set; }
    public SettingLock NepsisLock { get; set; } = new();
    public SettingLock StrictLock { get; set; } = new();
    public bool NoWeakening { get; set; }
    public bool PasswordForLocks { get; set; }
    public string Dns { get; set; } = "";
    public bool TamperClock { get; set; }
    public bool TamperFileUrls { get; set; }
    public bool TamperTaskManagers { get; set; }
    public bool TamperPrivate { get; set; }
    public bool TamperGuest { get; set; }
    public bool TamperExtensionStop { get; set; }
    public bool TamperInstaller { get; set; }
    public bool TamperInactiveTabs { get; set; }
    public bool TamperEmbedded { get; set; }
    public int TamperGrace { get; set; }
    public bool AnythingLocked { get; set; }
    public string DesktopWarning { get; set; } = "";
    public NoticeOptions Notifications { get; set; } = new();
    public bool StrictUse { get; set; }
    public List<string> StrictExempt { get; set; } = [];
    public List<string> Templates { get; set; } = [];
    public List<Preset> Presets { get; set; } = [];
    public bool UnsupportedBrowsers { get; set; }
    public SettingLock UnsupportedBrowsersLock { get; set; } = new();
    public List<BrowserInfo> Browsers { get; set; } = [];
    public long MonkSeconds { get; set; }
    public Dictionary<string, bool> Options { get; set; } = [];
    public List<Cluster> Clusters { get; set; } = [];
    public Recording? Recording { get; set; }
    public List<Place> Places { get; set; } = [];
    public List<Mode> Modes { get; set; } = [];
    public WallpaperSettings Wallpaper { get; set; } = new();
    public List<Schedule> Schedules { get; set; } = [];
    public List<SocialPreset> SocialPresets { get; set; } = [];
    public Dictionary<string, string> Access { get; set; } = [];
    public List<int> AccessUnused { get; set; } = [];

    public Block? FindBlock(string name) => Blocks.FirstOrDefault(b => b.Name == name);
    public Mode? FindMode(string name) => Modes.FirstOrDefault(m => m.Name == name);

    /// <summary>Blocks the user sees: drafts (§4.3) stay behind their block.</summary>
    public IEnumerable<Block> RealBlocks => Blocks.Where(b => string.IsNullOrEmpty(b.DraftOf));

    public static GuiState Parse(string json) => Kc9kJson.Parse<GuiState>(json);
}

public sealed class TimeWindow
{
    public int Days { get; set; } // bit 0 is Sunday
    public int Start { get; set; } // minutes after midnight
    public int End { get; set; }
    public bool Allow { get; set; }
    public long Break { get; set; }
    public string Source { get; set; } = "";
}

public sealed class Schedule
{
    public string Name { get; set; } = "";
    public List<TimeWindow> Windows { get; set; } = [];
    public List<string> Blocks { get; set; } = [];
    public List<string> Modes { get; set; } = [];
}

public sealed class Preset
{
    public string Kind { get; set; } = "";
    public string Name { get; set; } = "";
    public string What { get; set; } = "";
}

public sealed class AutomationSettings
{
    public bool DefaultOn { get; set; }
    public bool NeedBoth { get; set; }
}

public sealed class Picture
{
    public int Index { get; set; }
    public string Image { get; set; } = "";
    public string Name { get; set; } = "";
    public string Path { get; set; } = "";
    public string From { get; set; } = "";
    public string To { get; set; } = "";
    public bool Active { get; set; }
}

public sealed class Mode
{
    public string Name { get; set; } = "";
    public bool Monk { get; set; }
    public bool Active { get; set; }
    public List<string> Blocks { get; set; } = [];
    public List<TimeWindow> Windows { get; set; } = [];
    public List<string> Schedules { get; set; } = [];
    public List<string> Places { get; set; } = [];
    public AutomationSettings Automation { get; set; } = new();
    public bool Continuous { get; set; }
    public bool Running { get; set; }
    public int On { get; set; }
    public int Locked { get; set; }
    public long MonkLeft { get; set; }
    public bool Freeze { get; set; }
    public long FreezeLeft { get; set; }
    public string FreezeMode { get; set; } = "";
    public List<string> Custom { get; set; } = [];
    public long RunLeft { get; set; }
    public List<string> TodayWindows { get; set; } = [];
    public List<Picture> Wallpapers { get; set; } = [];
}

public sealed class WallpaperSettings
{
    public bool OverrideModes { get; set; }
    public string Default { get; set; } = "latest";
    public string CustomName { get; set; } = "";
    public string CustomPath { get; set; } = "";
}

public sealed class ListAction
{
    public string Action { get; set; } = "";
    public int Distinct { get; set; }
    public int Hits { get; set; }
    public int Rate { get; set; }
    public string Param { get; set; } = "";
    public int Minutes { get; set; }
}

public sealed class WebList
{
    public int Id { get; set; }
    public string Name { get; set; } = "";
    public string Kind { get; set; } = ""; // blacklist, exceptions
    public List<string> Where { get; set; } = [];
    public List<ListAction> Actions { get; set; } = [];
    public List<string> Entries { get; set; } = [];
    public bool Test { get; set; }
    public bool IsExceptions => Kind == "exceptions";
}

public sealed class WordSet
{
    public List<string> Terms { get; set; } = [];
    public bool Necessary { get; set; }
}

public sealed class Phrase
{
    public List<WordSet> Sets { get; set; } = [];
    public int Length { get; set; }
    public string Text { get; set; } = "";
}

public sealed class PhraseSet
{
    public int Id { get; set; }
    public string Name { get; set; } = "";
    public List<string> Where { get; set; } = [];
    public List<ListAction> Actions { get; set; } = [];
    public bool Test { get; set; }
    public List<Phrase> Phrases { get; set; } = [];
}

public sealed class TriggerAction
{
    public string Name { get; set; } = "";
    public int Minutes { get; set; }
    public string Param { get; set; } = "";
}

public sealed class NepsisAction
{
    public string Action { get; set; } = "";
    public int Minutes { get; set; }
    public string Param { get; set; } = "";
}

public sealed class NepsisStep
{
    public int Count { get; set; }
    public int Minutes { get; set; }
    public List<NepsisAction> Actions { get; set; } = [];
}

public sealed class Nepsis
{
    public List<string> Detectors { get; set; } = [];
    public List<NepsisStep> Steps { get; set; } = [];
    public int QuietMinutes { get; set; }
    public bool IsOn => Steps.Count > 0;
}

public sealed class Block
{
    public Dictionary<string, List<TriggerAction>> Triggers { get; set; } = [];
    public AutomationSettings Automation { get; set; } = new();
    public bool Continuous { get; set; }
    public string Mode { get; set; } = "";
    public List<string> AppliedSchedules { get; set; } = [];
    public string Name { get; set; } = "";
    public List<WebList> Lists { get; set; } = [];
    public List<PhraseSet> PhraseSets { get; set; } = [];
    public List<string> Apps { get; set; } = [];
    public List<string> Titles { get; set; } = [];
    public Nepsis Nepsis { get; set; } = new();
    public int NepsisCount { get; set; }
    public List<string> Schedules { get; set; } = [];
    public List<TimeWindow> ScheduleWindows { get; set; } = [];
    public List<string> Users { get; set; } = [];
    public bool Active { get; set; }
    public bool Held { get; set; }
    public bool CharLocked { get; set; }
    public long PomodoroOn { get; set; }
    public long PomodoroOff { get; set; }
    public long PomodoroLong { get; set; }
    public long PomodoroEvery { get; set; }
    public bool OnBreak { get; set; }
    public bool PasswordLocked { get; set; }
    public bool RestartLocked { get; set; }
    public long DelaySeconds { get; set; }
    public long StopSeconds { get; set; }
    public long DelayLeft { get; set; }
    public List<string> LockOrder { get; set; } = [];
    public long TimerAfter { get; set; }
    public string RangeLock { get; set; } = "";
    public bool RangeOn { get; set; }
    public bool Scheduled { get; set; }
    public bool BlockPassword { get; set; }
    public string DraftOf { get; set; } = "";
    public string Draft { get; set; } = "";
    public List<string> DraftKept { get; set; } = [];
    public List<string> TodayWindows { get; set; } = [];
    public List<string> Autostarts { get; set; } = [];
    public bool Hidden { get; set; }
    public bool Lifted { get; set; }
    public bool Enabled { get; set; }
    public long LockSeconds { get; set; }
    public long QuotaLimit { get; set; }
    public long QuotaUsed { get; set; }
    public bool QuotaOpen { get; set; }
    public long QuotaNext { get; set; }
    public string QuotaPeriod { get; set; } = "";
    public string QuotaRefill { get; set; } = "";
    public int QuotaSessions { get; set; }
    public int QuotaSessionsUsed { get; set; }
    public bool KeywordTest { get; set; }
    public List<string> Places { get; set; } = [];
    public long LoosenDelay { get; set; }
    public bool ScheduleLock { get; set; }
    public int ScheduleLead { get; set; }
    public List<string> Social { get; set; } = [];
    public long BreakAllowance { get; set; }
    public long BreakPer { get; set; }
    public long BreakBalance { get; set; }
    public long Relock { get; set; }
    public long DelayBreak { get; set; }
    public string BreakKeep { get; set; } = "";
    public long DelayWait { get; set; }
    public long DelayBreakIn { get; set; }
    public long PausedSeconds { get; set; }

    public bool InMode => !string.IsNullOrEmpty(Mode);
    public bool HasLocks => Held;
}

public sealed class HotkeyAction
{
    public string Kind { get; set; } = "";
    public string Target { get; set; } = "";
    public int Minutes { get; set; }
    public string Param { get; set; } = "";
    public List<string> LockArgs { get; set; } = [];
}

public sealed class Hotkey
{
    public int Id { get; set; }
    public string Keys { get; set; } = "";
    public string Block { get; set; } = "";
    public string LockFor { get; set; } = "";
    public List<string> LockArgs { get; set; } = [];
    public bool NoConfirm { get; set; }
    public List<HotkeyAction> Actions { get; set; } = [];
    public string Lock { get; set; } = "";
    public string Kinds { get; set; } = "";
    public string Does { get; set; } = "";
}

public sealed class SettingLock
{
    public long LockSeconds { get; set; }
    public bool CharLocked { get; set; }
    public bool PasswordLocked { get; set; }
    public bool RestartLocked { get; set; }
    public bool GuardsBlocks { get; set; }
    public long Relock { get; set; }
    public long RelockIn { get; set; }
    public long TimerAfter { get; set; }
    public List<string> LockOrder { get; set; } = [];

    public bool IsLocked => LockSeconds > 0 || CharLocked || PasswordLocked || RestartLocked || TimerAfter > 0;
}

public sealed class Cluster
{
    public string Name { get; set; } = "";
    public List<string> Where { get; set; } = [];
    public List<string> Entries { get; set; } = [];
    public bool Phrases { get; set; }
}

public sealed class Recording
{
    public string Cluster { get; set; } = "";
    public string Block { get; set; } = "";
    public string List { get; set; } = "";
    public int Sites { get; set; }
    public bool Words { get; set; }
    public int WordCount { get; set; }
    public long Pages { get; set; }
    public List<string> Top { get; set; } = [];
}

public sealed class Place
{
    public string Name { get; set; } = "";
    public int Networks { get; set; }
    public int Need { get; set; }
    public List<string> Blocks { get; set; } = [];
    public List<string> Modes { get; set; } = [];
    public bool Here { get; set; }
}

public sealed class NoticeOptions
{
    public List<string> Off { get; set; } = [];
    public List<int> Advance { get; set; } = [];
    public bool IsOn(string kind) => !Off.Contains(kind);
}

public sealed class BrowserInfo
{
    public string Id { get; set; } = "";
    public string Name { get; set; } = "";
    public bool Installed { get; set; }
    public long CheckinSeconds { get; set; } = -1;
}

public sealed class SocialPreset
{
    public string Name { get; set; } = "";
    public string Site { get; set; } = "";
    public string Title { get; set; } = "";
}

/// <summary>`kc9k stats --json` (internal/cli/stats.go statsOutput).</summary>
public sealed class StatsOutput
{
    public bool Enabled { get; set; }
    public string Since { get; set; } = "";
    public string Until { get; set; } = "";
    public List<UsageRow> Rows { get; set; } = [];
    public List<string> Users { get; set; } = [];

    public static StatsOutput Parse(string json) => Kc9kJson.Parse<StatsOutput>(json);
}

public sealed class UsageRow
{
    public string Day { get; set; } = "";
    public string Kind { get; set; } = ""; // site, app, blocked, blocked-block
    public string Name { get; set; } = "";
    public long Seconds { get; set; }
}
