// What the GTK GUI's Window does between the widgets and kc9k: run a
// command in the background (the window never freezes, R-GUI-3), read the
// state again after it and every few seconds, ask for the settings or an
// access password when the CLI says one is needed and run the command again,
// and send changes to an opened block to its draft (§4.3).
using Kc9k.Core;
using Microsoft.UI.Dispatching;

namespace Kc9k.Gui;

public interface IPage
{
    /// <summary>The access area the page belongs to (§3.11).</summary>
    string Area { get; }
    Microsoft.UI.Xaml.UIElement View { get; }
    void Update(GuiState state);
    void UpdateStats(StatsOutput stats) { }
}

public sealed class Controller
{
    public const int RefreshSeconds = 5, StatsRefreshSeconds = 30;

    readonly DispatcherQueue ui;
    readonly DispatcherQueueTimer refreshTimer, statsTimer;
    bool reading, readAgain;
    bool extensionAsked;
    int running;
    DateTime lastInput = DateTime.Now;

    public CliSession Session { get; }
    public Dialogs Dialogs { get; }
    public GuiState? State { get; private set; }
    public string StateJson { get; private set; } = "";
    public StatsOutput? Stats { get; private set; }
    public Dictionary<string, string> Access => State?.Access ?? [];

    /// <summary>The block opened on the Blocks page, its tabs showing (never a draft).</summary>
    public Func<string?> OpenedBlock { get; set; } = () => null;

    public event Action<GuiState>? StateChanged;
    public event Action<StatsOutput>? StatsChanged;
    public event Action<bool>? BusyChanged;
    public event Action<string?>? Banner;
    public event Action? ExtensionMissing;
    /// <summary>Extra arguments for `stats --json` (the Statistics page's time frame).</summary>
    public Func<IEnumerable<string>> StatsArgs { get; set; } = () => ["--days", "14"];

    public Controller(DispatcherQueue ui, Dialogs dialogs, ICliProcess? process = null)
    {
        this.ui = ui;
        Dialogs = dialogs;
        Session = new CliSession(process ?? new ProcessCli(ProcessCli.Locate()));
        refreshTimer = ui.CreateTimer();
        refreshTimer.Interval = TimeSpan.FromSeconds(RefreshSeconds);
        refreshTimer.Tick += (_, _) => Refresh();
        statsTimer = ui.CreateTimer();
        statsTimer.Interval = TimeSpan.FromSeconds(StatsRefreshSeconds);
        statsTimer.Tick += (_, _) => _ = RefreshStats();
    }

    public void Start()
    {
        Refresh();
        _ = RefreshStats();
        refreshTimer.Start();
        statsTimer.Start();
    }

    public bool Busy => running > 0;
    bool stopped;

    /// <summary>The window closed: no more reads, nothing more shown.</summary>
    public void Stop()
    {
        stopped = true;
        refreshTimer.Stop();
        statsTimer.Stop();
    }

    public void NoteInput() => lastInput = DateTime.Now;

    /// <summary>Runs a command, then reads the state again. A refusal is shown
    /// in the CLI's words; then (when given) runs on success with the output.
    /// Returns whether it succeeded.</summary>
    public async Task<bool> Run(IReadOnlyList<string> args, string? secret = null, Func<string, Task>? then = null,
        TimeSpan? timeout = null, bool quiet = false)
    {
        var drafted = await ToDraft(args);
        if (drafted is null) return false; // cancelled (a block password not given)
        args = drafted;
        SetBusy(+1);
        CliResult result;
        try { result = await Session.RunAsync(args, secret, timeout); }
        finally { SetBusy(-1); }
        if (!result.Ok && result.AccessArea is string area)
        {
            var title = Tables.AreaTitles.GetValueOrDefault(area, area);
            var password = await Dialogs.AskText("Access password", $"This needs the password for {title}.", password: true);
            if (!string.IsNullOrEmpty(password) && await TryAccess(password)) return await Run(args, secret, then, timeout, quiet);
            return false;
        }
        if (!result.Ok && result.NeedsSettingsPassword)
        {
            var wrong = result.WrongSettingsPassword && Session.SettingsPassword is not null;
            Session.SettingsPassword = null;
            var password = await Dialogs.AskText("Settings password",
                wrong ? "That password was wrong. Try again." : "This change needs the settings password.", password: true);
            if (string.IsNullOrEmpty(password)) return false;
            Session.SettingsPassword = password;
            Session.SettingsPasswordSet = true;
            return await Run(args, secret, then, timeout, quiet);
        }
        if (!result.Ok)
        {
            if (!quiet) await Dialogs.Message(result.Output.Length > 0 ? result.Output : "The command failed.");
        }
        else if (then is not null) await then(result.Output);
        else if (!quiet) QueuedNote(result.Output);
        StateJson = ""; // show it again even if it looks unchanged
        // What runs after a command sees its effect: wait for the state read after it.
        await RefreshAsync();
        return result.Ok;
    }

    /// <summary>Says so when a change was queued instead of applied.</summary>
    public void QueuedNote(string output)
    {
        if (output.Contains("queued")) _ = Dialogs.Message(output, "Queued");
    }

    /// <summary>Runs a command and gives its output (for reads such as export);
    /// null after a refusal, which is shown.</summary>
    public async Task<string?> Read(IReadOnlyList<string> args, string? secret = null)
    {
        var block = OpenedBlock();
        if (block is not null && State?.FindBlock(block) is { } b && b.Draft.Length > 0)
            args = Drafts.ToDraft(args, block, edit: false) ?? args;
        SetBusy(+1);
        try
        {
            var result = await Session.RunAsync(args, secret);
            if (result.Ok) return result.Output;
            await Dialogs.Message(result.Output);
            return null;
        }
        finally { SetBusy(-1); }
    }

    /// <summary>§4.3: a change to what the opened block's tabs show goes to its
    /// draft; the first one makes the draft. Null: cancelled.</summary>
    async Task<IReadOnlyList<string>?> ToDraft(IReadOnlyList<string> args)
    {
        var block = OpenedBlock();
        if (block is null || State?.FindBlock(block) is not { } b) return args;
        var moved = Drafts.ToDraft(args, block, edit: true);
        if (moved is null) return args;
        var draft = Drafts.NameOf(block);
        if (State.FindBlock(draft) is null && b.Draft != draft && !await StartDraft(block)) return null;
        return moved;
    }

    /// <summary>§4.3: the block's draft, made at its first edit; with a block
    /// password, that password (the draft keeps it).</summary>
    public async Task<bool> StartDraft(string name)
    {
        var block = State?.FindBlock(name);
        Session.BlockPasswords.TryGetValue(name, out var password);
        if (block?.BlockPassword == true && password is null)
        {
            password = await Dialogs.AskText($"Edit '{name}'", "Its block password.", password: true);
            if (string.IsNullOrEmpty(password)) return false;
        }
        if (password is not null) Session.BlockPasswords[name] = Session.BlockPasswords[Drafts.NameOf(name)] = password;
        SetBusy(+1);
        CliResult result;
        try { result = await Session.RunAsync(["block", "draft", name], password is null ? null : password + "\n"); }
        finally { SetBusy(-1); }
        if (!result.Ok)
        {
            await Dialogs.Message(result.Output);
            return false;
        }
        // Read at once, so the next change finds the draft.
        await ReadState();
        return true;
    }

    void SetBusy(int change)
    {
        running += change;
        BusyChanged?.Invoke(running > 0);
    }

    /// <summary>Reads the state now (after a read already running) and waits for it.</summary>
    public async Task RefreshAsync()
    {
        while (reading) await Task.Delay(40);
        await ReadState();
    }

    public void Refresh()
    {
        LockWhenIdle();
        if (reading)
        {
            // A read already running may predate the last change: read again
            // when it lands, so the last command is always shown.
            readAgain = true;
            return;
        }
        _ = ReadState();
    }

    async Task ReadState()
    {
        reading = true;
        readAgain = false;
        var passwords = Session.AccessPasswords.ToList();
        (GuiState? State, string Json, CliResult Result) read;
        try { read = await Session.ReadStateAsync(); }
        finally { reading = false; }
        if (readAgain || !passwords.SequenceEqual(Session.AccessPasswords))
        {
            Refresh();
            return;
        }
        if (read.State is null)
        {
            Banner?.Invoke($"Can't read the state: {read.Result.Output}");
            return;
        }
        if (stopped || read.Json == StateJson) return;
        StateJson = read.Json;
        State = read.State;
        Session.SettingsPasswordSet = State.SettingsPassword;
        // A password that opens nothing any more (it was changed) is dropped.
        foreach (var index in State.AccessUnused)
            if (index < passwords.Count) Session.AccessPasswords.Remove(passwords[index]);
        Banner?.Invoke(State.FreezeSeconds > 0 ? $"Frozen Chicken is on for {Words.Seconds(State.FreezeSeconds)} more." : null);
        if (State.ExtensionMissing && !extensionAsked)
        {
            extensionAsked = true; // once per session
            ui.TryEnqueue(() => ExtensionMissing?.Invoke());
        }
        try { StateChanged?.Invoke(State); }
        catch (Exception e) { Log.Write("update: " + e); }
    }

    public async Task RefreshStats()
    {
        var (stats, _) = await Session.ReadStatsAsync(StatsArgs());
        if (stats is null || stopped) return;
        Stats = stats;
        try { StatsChanged?.Invoke(stats); }
        catch (Exception e) { Log.Write("stats: " + e); }
    }

    /// <summary>Keeps a password if it opens something; false if it doesn't.</summary>
    public async Task<bool> TryAccess(string password)
    {
        if (Session.AccessPasswords.Contains(password)) return true;
        Session.AccessPasswords.Add(password);
        var (state, _, _) = await Session.ReadStateAsync();
        if (state is null || state.AccessUnused.Contains(Session.AccessPasswords.Count - 1))
        {
            Session.AccessPasswords.Remove(password);
            return false;
        }
        StateJson = "";
        Refresh();
        _ = RefreshStats();
        return true;
    }

    /// <summary>Forgets the passwords given and locks the pages again.</summary>
    public void LockAccess()
    {
        Session.AccessPasswords.Clear();
        Session.BlockPasswords.Clear();
        StateJson = "";
        Refresh();
    }

    void LockWhenIdle()
    {
        var minutes = Prefs.Current.AccessIdleMinutes;
        if (minutes > 0 && Session.AccessPasswords.Count > 0 && (DateTime.Now - lastInput).TotalMinutes >= minutes) LockAccess();
    }

    /// <summary>Whether a page's area is closed by an access password now.</summary>
    public bool Locked(string area) => Access.GetValueOrDefault(area) == "locked";

    public void ShowError(string text) => ui.TryEnqueue(() => _ = Dialogs.Message(text, "Something went wrong"));

    /// <summary>The modes a Mode action can run: monk mode first, then the others.</summary>
    public List<string> ModeChoices() =>
        [.. (State?.Modes ?? []).Where(m => !m.Freeze).Select(m => m.Name).OrderBy(n => n != "Monk mode").ThenBy(n => n.ToLowerInvariant())];
}
