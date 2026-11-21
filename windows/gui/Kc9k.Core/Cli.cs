// Running kc9k (the contract, ROADMAP §4 W4): the GUI decides nothing; it
// reads `kc9k gui-state` and `kc9k stats --json`, and every change is a kc9k
// command whose output, refusals included, is shown as the CLI words it.
// kc9k.exe forwards to the service over its pipe; the GUI never opens the
// pipe, the DB or the registry.
using System.Diagnostics;
using System.Text;

namespace Kc9k.Core;

public sealed record CliResult(bool Ok, string Output)
{
    /// <summary>The access area a refusal asks a password for ("" when none).</summary>
    public string? AccessArea
    {
        get
        {
            if (Ok || !(Output.Contains("needs the access password") || Output.Contains("wrong access password"))) return null;
            int open = Output.LastIndexOf('('), close = Output.LastIndexOf(')');
            return open >= 0 && close > open ? Output[(open + 1)..close] : "";
        }
    }

    public bool NeedsSettingsPassword =>
        !Ok && (Output.Contains("needs the settings password") || Output.Contains("wrong settings password"));

    public bool WrongSettingsPassword => !Ok && Output.Contains("wrong settings password");
}

/// <summary>Runs one kc9k command: its arguments, what goes on standard input.</summary>
public interface ICliProcess
{
    Task<CliResult> RunAsync(IReadOnlyList<string> args, string? stdin = null, TimeSpan? timeout = null);
}

/// <summary>kc9k.exe as a child process, without a console window: a text
/// lock's challenge then opens in the CLI's own window (R-LOCK-5), never in
/// the GUI.</summary>
public sealed class ProcessCli(string path) : ICliProcess
{
    public string Path { get; } = path;

    /// <summary>Where kc9k is: KC9K_CLI, next to the GUI, one folder up (the
    /// installed layout), then PATH.</summary>
    public static string Locate()
    {
        var env = Environment.GetEnvironmentVariable("KC9K_CLI");
        if (!string.IsNullOrWhiteSpace(env)) return env;
        var exe = OperatingSystem.IsWindows() ? "kc9k.exe" : "kc9k";
        var here = AppContext.BaseDirectory;
        foreach (var dir in new[] { here, System.IO.Path.GetDirectoryName(here.TrimEnd('\\', '/')) ?? here })
        {
            var candidate = System.IO.Path.Combine(dir, exe);
            if (File.Exists(candidate)) return candidate;
        }
        foreach (var dir in (Environment.GetEnvironmentVariable("PATH") ?? "").Split(System.IO.Path.PathSeparator))
        {
            if (dir.Length == 0) continue;
            var candidate = System.IO.Path.Combine(dir, exe);
            if (File.Exists(candidate)) return candidate;
        }
        return exe;
    }

    public async Task<CliResult> RunAsync(IReadOnlyList<string> args, string? stdin = null, TimeSpan? timeout = null)
    {
        var info = new ProcessStartInfo(Path)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8,
        };
        foreach (var a in args) info.ArgumentList.Add(a);
        Process process;
        try
        {
            process = Process.Start(info) ?? throw new InvalidOperationException("kc9k did not start");
        }
        catch (Exception e)
        {
            return new CliResult(false, $"Killer Chicken 9000's command (kc9k) could not be started: {e.Message}");
        }
        using (process)
        {
            var stdout = process.StandardOutput.ReadToEndAsync();
            var stderr = process.StandardError.ReadToEndAsync();
            try
            {
                if (!string.IsNullOrEmpty(stdin)) await process.StandardInput.WriteAsync(stdin);
                process.StandardInput.Close();
            }
            catch (IOException) { } // it finished without reading
            using var cts = new CancellationTokenSource(timeout ?? TimeSpan.FromSeconds(120));
            try
            {
                await process.WaitForExitAsync(cts.Token);
            }
            catch (OperationCanceledException)
            {
                try { process.Kill(true); } catch (Exception) { }
                return new CliResult(false, $"kc9k {string.Join(' ', args)} took too long and was stopped.");
            }
            var output = ((await stdout) + (await stderr)).Trim();
            return new CliResult(process.ExitCode == 0, output);
        }
    }
}

/// <summary>What the GUI remembers while it runs (passwords are never written
/// anywhere): the settings password once given, the access passwords given,
/// and the block passwords typed (§3.11).</summary>
public sealed class CliSession(ICliProcess process)
{
    public ICliProcess Process { get; } = process;
    public string? SettingsPassword { get; set; }
    public List<string> AccessPasswords { get; } = [];
    public Dictionary<string, string> BlockPasswords { get; } = [];
    /// <summary>Whether the state says a settings password is set.</summary>
    public bool SettingsPasswordSet { get; set; }

    /// <summary>The arguments and standard input a command runs with: access
    /// passwords first (--access-lines), then the settings password
    /// (--settings-line), then the command's own secret.</summary>
    public (List<string> Args, string? Stdin) Wrap(IReadOnlyList<string> args, string? secret)
    {
        var cliArgs = new List<string>(args);
        var stdin = secret;
        if (SettingsPassword is not null && SettingsPasswordSet)
        {
            stdin = SettingsPassword + "\n" + (secret ?? "");
            cliArgs.Insert(0, "--settings-line");
        }
        if (AccessPasswords.Count > 0)
        {
            cliArgs.InsertRange(0, ["--access-lines", AccessPasswords.Count.ToString()]);
            stdin = string.Concat(AccessPasswords.Select(p => p + "\n")) + (stdin ?? "");
        }
        return (cliArgs, stdin);
    }

    public Task<CliResult> RunAsync(IReadOnlyList<string> args, string? secret = null, TimeSpan? timeout = null)
    {
        var (a, s) = Wrap(args, secret);
        return Process.RunAsync(a, s, timeout);
    }

    /// <summary>`kc9k gui-state`, with the block passwords typed (--reveal).</summary>
    public async Task<(GuiState? State, string Json, CliResult Result)> ReadStateAsync()
    {
        var args = new List<string> { "gui-state" };
        var secret = new StringBuilder();
        foreach (var (block, password) in BlockPasswords)
        {
            args.AddRange(["--reveal", block]);
            secret.Append(password).Append('\n');
        }
        var result = await RunAsync(args, secret.Length > 0 ? secret.ToString() : null);
        if (!result.Ok) return (null, "", result);
        try
        {
            var json = JsonStart(result.Output);
            return (GuiState.Parse(json), json, result);
        }
        catch (System.Text.Json.JsonException e)
        {
            return (null, "", new CliResult(false, "kc9k gui-state printed something unreadable: " + e.Message));
        }
    }

    public async Task<(StatsOutput? Stats, CliResult Result)> ReadStatsAsync(IEnumerable<string> extra)
    {
        var result = await RunAsync(["stats", "--json", .. extra]);
        if (!result.Ok) return (null, result);
        try { return (StatsOutput.Parse(JsonStart(result.Output)), result); }
        catch (System.Text.Json.JsonException e) { return (null, new CliResult(false, "kc9k stats printed something unreadable: " + e.Message)); }
    }

    /// <summary>The JSON in an output that may have notes before it.</summary>
    static string JsonStart(string output)
    {
        var i = output.IndexOf('{');
        return i > 0 ? output[i..] : output;
    }
}
