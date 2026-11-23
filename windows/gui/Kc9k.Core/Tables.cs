// What the GUI offers, in the order SPEC lists it, with the words it shows.
// The keys are the CLI's; the help texts are worded for Windows (the GTK
// GUI's are Linux's: System Monitor, install.sh).
namespace Kc9k.Core;

public sealed record Choice(string Key, string Title, string Help = "");

public static class Tables
{
    /// <summary>Sidebar pages, in order (the GTK GUI's sidebar).</summary>
    public static readonly (string Key, string Title)[] Pages =
    [
        ("dashboard", "Overview"), ("blocks", "Blocks"), ("schedules", "Schedules"), ("places", "Places"),
        ("hotkeys", "Hotkeys"), ("clusters", "Presets"), ("stats", "Statistics"), ("circadian", "Circadian rhythm"),
        ("settings", "Settings"),
    ];

    /// <summary>Areas an access password can close (§3.11), as the CLI names them.</summary>
    public static readonly Dictionary<string, string> AreaTitles = new()
    {
        ["app"] = "Killer Chicken 9000", ["dashboard"] = "Overview", ["blocks"] = "Blocks",
        ["device"] = "Frozen Chicken", ["stats"] = "Statistics", ["settings"] = "Settings",
    };

    /// <summary>Schedules added with one click: title, days, from, to.</summary>
    public static readonly (string Title, string Days, string From, string To)[] SchedulePresets =
    [
        ("Work hours", "weekdays", "09:00", "17:00"), ("Evenings", "daily", "18:00", "23:00"),
        ("Weekends", "sat,sun", "00:00", "24:00"), ("Nights", "daily", "00:00", "06:00"),
    ];

    public const string FrozenChicken = "Frozen Chicken";
    public const long FreezeMax = 7 * 86400; // R-FREEZE-3
    public const int ChallengeMin = 1, ChallengeMax = 5000, ChallengeDefault = 300;
    public const int NepsisMaxCount = 1000; // db.MaxNepsisCount

    /// <summary>The switches of 'kc9k option' (internal/enforce/options.go).</summary>
    public static readonly Choice[] Options =
    [
        new("polite-close", "Ask blocked apps to close first",
            "A blocked app with a window is asked to close, so it can offer to save, and is closed for good a few seconds later if it is still open. Off: closed at once."),
        new("block-page-details", "Say why on the block page",
            "The block page names the block and list that stopped the page, and when the block ends. It changes nothing about what a list does."),
    ];

    /// <summary>Browser settings (§4.10), in the order SPEC lists them.</summary>
    public static readonly Choice[] BrowserOptions =
    [
        new("safesearch-google", "SafeSearch on Google",
            "Also keeps Brave, Yahoo and SearXNG searches strict and blocks search engines that can't be kept strict (so do Bing's and DuckDuckGo's)."),
        new("safesearch-bing", "SafeSearch on Bing"),
        new("safesearch-duckduckgo", "SafeSearch on DuckDuckGo"),
        new("safesearch-youtube", "YouTube Restricted Mode"),
        new("force-search", "Force Google as the search engine"),
        new("web-only", "Web results only", "No image, shopping or AI results in Google."),
        new("block-devtools", "Block developer tools",
            "In LibreWolf and Firefox they can switch the extension off. Turn this off only to develop websites; it applies when the browser restarts."),
        new("no-private", "Disable private windows",
            "No private browsing; it applies when the browser restarts. Chrome's and Edge's InPrivate and incognito windows can't run the extension, so they stay off."),
    ];

    /// <summary>Tamper protection settings (§4.10): key, title, help, command.</summary>
    public static readonly (string Key, string Title, string Help, string[] Command, string StateField)[] Tamper =
    [
        ("clock", "Clock: ignore time and date changes",
            "While a timer lock runs, changing the date or time doesn't move it, and time the computer is off isn't counted.",
            ["tamper", "clock"], "tamper_clock"),
        ("task-managers", "Block task managers while locked",
            "While a block or setting is locked, Task Manager, Process Explorer and other task managers are closed.",
            ["tamper", "task-managers"], "tamper_task_managers"),
        ("private", "Close private windows and Tor",
            "Private windows are turned off in every browser (open ones close when it restarts) and Tor Browser is closed.",
            ["tamper", "private"], "tamper_private"),
        ("guest", "Close guest profiles",
            "Guest browsing is turned off in every Chrome-like browser, Edge included, and Windows' Guest account stays off.",
            ["tamper", "guest"], "tamper_guest"),
        ("extension-stop", "Close browsers if the extension stops",
            "A browser whose Killer Chicken extension stops checking in is closed while a block needs it. Off, only while something is locked.",
            ["tamper", "extension-stop"], "tamper_extension_stop"),
        ("installer", "Block the installer",
            "While anything is locked, reinstalling Killer Chicken 9000 or installing an older version is refused. Removing it always is.",
            ["tamper", "installer"], "tamper_installer"),
        ("inactive-tabs", "Block inactive tabs",
            "Tabs in the background are blocked at once. Off, a background tab is blocked when you switch to it.",
            ["tamper", "inactive-tabs"], "tamper_inactive_tabs"),
        ("embedded", "Block embedded content",
            "A blocked site shown inside another page (a video, a frame) gets the block page. Off, the extension leaves embedded pages alone; whole blocked domains stay blocked.",
            ["tamper", "embedded"], "tamper_embedded"),
        ("file-urls", "Allow file:// blocking",
            "Blacklist entries such as file:///C:/Games/* block local files in the browsers. Each block's own file:// exceptions (file://* by default) still spare them.",
            ["tamper", "file-urls"], "tamper_file_urls"),
        ("unsupported-browsers", "Block unsupported browsers",
            "Only supported browsers running the Killer Chicken extension stay open, even when no block is on. Every other browser is closed at once; a supported browser without the extension a minute after it starts. Needs a supported browser installed.",
            ["tamper", "unsupported-browsers"], "unsupported_browsers"),
    ];

    public static bool TamperValue(GuiState s, string key) => key switch
    {
        "clock" => s.TamperClock, "task-managers" => s.TamperTaskManagers, "private" => s.TamperPrivate,
        "guest" => s.TamperGuest, "extension-stop" => s.TamperExtensionStop, "installer" => s.TamperInstaller,
        "inactive-tabs" => s.TamperInactiveTabs, "embedded" => s.TamperEmbedded, "file-urls" => s.TamperFileUrls,
        "unsupported-browsers" => s.UnsupportedBrowsers, _ => false,
    };

    /// <summary>What a blacklist does to the pages it matches; any combination.</summary>
    public static readonly Choice[] SiteActions =
    [
        new("block", "Block page"), new("greyscale", "Greyscale"), new("no-images", "Block images"),
        new("mute", "Mute sound"), new("no-video", "Stop and hide video"), new("dark", "Dark page"),
        new("no-downloads", "Cancel downloads"), new("redirect", "Open another page"), new("image", "Show a picture"),
        new("close-tab", "Close the tab"), new("close-browser", "Close the browser"), new("logout", "Log out"),
        new("shutdown", "Shut down"), new("monk", "Monk mode"), new("mode", "Run a mode, locked"),
        new("custom", "Custom action"),
    ];

    /// <summary>Actions with settings: what to ask for when one is ticked.</summary>
    public static readonly HashSet<string> ParamActions = ["redirect", "image", "logout", "shutdown", "monk", "mode", "custom"];

    /// <summary>What bypass prevention can do, mildest first.</summary>
    public static readonly (string Key, string Title)[] Consequences =
    [
        ("warning", "Show a warning"), ("close-tab", "Close the tab that tripped it"), ("close-browser", "Close the browser"),
        ("lock-screen", "Lock the screen"), ("logout", "Log out"), ("shutdown", "Shut down"), ("monk", "Monk mode"),
        ("mode", "Run a mode, locked, for"), ("custom", "Custom action"),
    ];

    /// <summary>Consequences that take a time, and whether they need one.</summary>
    public static readonly Dictionary<string, bool> Timed = new()
    {
        ["lock-screen"] = true, ["logout"] = false, ["shutdown"] = false, ["monk"] = true, ["mode"] = true,
    };

    /// <summary>What bypass detection can count (db.Detectors).</summary>
    public static readonly (string Key, string Title)[] Detectors =
    [
        ("page", "Blocked pages"), ("search", "Blocked pages opened from a search"), ("app", "Closed apps"),
        ("title", "Windows closed for their title"), ("browser", "Browsers closed for lacking the extension"),
        ("workaround", "Sites reached another way (proxies, mirrors, download tools)"),
        ("tamper", "Tampering (service restarted, clock moved)"), ("access", "Wrong access passwords"),
    ];

    /// <summary>A list's scope boxes (§3.2); ads only for blacklists and clusters (§3.9).</summary>
    public static readonly (string Key, string Title)[] ListPlaces = [("url", "URL"), ("title", "Title"), ("text", "Text")];
    public static readonly (string Key, string Title) AdPlace = ("ads", "Ads");

    /// <summary>Notifications (§4.10).</summary>
    public static readonly (string Key, string Title)[] NoticeKinds =
    [
        ("window-start", "A window opens"), ("window-end", "A window closes"), ("timer-end", "A timer lock ends"),
        ("pomodoro", "Pomodoro work or rest begins"), ("allowance", "An allowance runs out"),
        ("app-blocked", "A blocked app was closed"),
    ];

    public static readonly (string Key, string Title)[] Themes = [("system", "Same as Windows"), ("light", "Light"), ("dark", "Dark")];

    /// <summary>§4.10 Wallpaper default, as `kc9k wallpaper default` takes it.</summary>
    public static readonly (string Key, string Title)[] WallpaperDefaults =
        [("latest", "Latest wallpaper"), ("circadian", "Circadian rhythm"), ("custom", "Custom")];

    /// <summary>Frozen Chicken's modes (§3.8, `kc9k freeze --mode`).</summary>
    public static readonly (string Key, string Title)[] FreezeModes =
        [("lock", "Lock screen"), ("logout", "Log out"), ("shutdown", "Shut down"), ("apps", "Allowed apps only")];
}
