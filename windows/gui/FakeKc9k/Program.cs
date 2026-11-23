// The fake kc9k (W4 tests). Replays fixtures captured from the real CLI
// (fixtures/make-fixtures.sh) and records every command:
//   KC9K_FAKE_DIR   fixtures: gui-state.json, stats.json, responses.json
//                   (default: the fixtures folder next to this program)
//   KC9K_FAKE_LOG   where each command is appended as one JSON line
//                   {"args": [...], "stdin": "..."} (default: commands.log
//                   in KC9K_FAKE_DIR)
// responses.json lists canned answers: [{"args": ["start", "Work"], "ok":
// false, "output": "..."}], matched by leading arguments, first match wins.
//   KC9K_FAKE_REAL  a real kc9k.exe: each command not answered from
//                   responses.json is logged, then run by it (with the
//                   caller's KC9K_DB: a scratch database, nothing applied)
//                   and its answer passed on (W32: the same test against
//                   the fake and the real CLI)
// An answer with "delay_ms" waits that long first (a slow kc9k); one with
// only "delay_ms" (no "output") then goes on as if it weren't there.
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace FakeKc9k;

public static class FakeCli
{
    public static int Main(string[] args)
    {
        var dir = Environment.GetEnvironmentVariable("KC9K_FAKE_DIR");
        if (string.IsNullOrEmpty(dir)) dir = Path.Combine(AppContext.BaseDirectory, "fixtures");
        var stdin = Console.IsInputRedirected ? Console.In.ReadToEnd() : "";
        var (code, output) = Run(args, stdin, dir, Environment.GetEnvironmentVariable("KC9K_FAKE_LOG"));
        var stream = Console.OpenStandardOutput();
        var bytes = new UTF8Encoding(false).GetBytes(output.Length > 0 ? output + "\n" : "");
        stream.Write(bytes);
        stream.Flush();
        return code;
    }

    public static (int Code, string Output) Run(string[] args, string stdin, string dir, string? log = null)
    {
        log ??= Path.Combine(dir, "commands.log");
        var line = new JsonObject
        {
            ["args"] = new JsonArray([.. args.Select(a => (JsonNode)JsonValue.Create(a)!)]),
            ["stdin"] = stdin,
        };
        // Two fakes at once (commands run side by side): one waits its turn.
        for (var attempt = 0; attempt < 50; attempt++)
        {
            try
            {
                File.AppendAllText(log, line.ToJsonString() + "\n");
                break;
            }
            catch (IOException) { Thread.Sleep(20); }
        }

        // The GUI's own flags before the command: --access-lines N, --settings-line.
        var rest = args.ToList();
        while (rest.Count > 0 && rest[0].StartsWith("--", StringComparison.Ordinal))
        {
            if (rest[0] == "--access-lines" && rest.Count > 1) rest.RemoveRange(0, 2);
            else rest.RemoveAt(0);
        }
        var canned = Path.Combine(dir, "responses.json");
        if (File.Exists(canned))
        {
            foreach (var r in JsonNode.Parse(File.ReadAllText(canned))!.AsArray())
            {
                var want = r!["args"]!.AsArray().Select(a => a!.GetValue<string>()).ToList();
                if (want.Count > rest.Count || !want.SequenceEqual(rest.Take(want.Count))) continue;
                if (r["delay_ms"]?.GetValue<int>() is int delay) Thread.Sleep(delay);
                if (r["output"] is null && r["ok"] is null) continue;
                return (r["ok"]?.GetValue<bool>() == false ? 1 : 0, r["output"]?.GetValue<string>() ?? "");
            }
        }
        if (Environment.GetEnvironmentVariable("KC9K_FAKE_REAL") is { Length: > 0 } real) return RunReal(real, args, stdin);
        if (rest.Count > 0 && rest[0] == "gui-state") return (0, File.ReadAllText(Path.Combine(dir, "gui-state.json")));
        if (rest.Count > 0 && rest[0] == "stats" && rest.Contains("--json")) return (0, File.ReadAllText(Path.Combine(dir, "stats.json")));
        if (rest.Count > 0 && rest[0] is "--version" or "version") return (0, "kc9k fake (windows/amd64)");
        // Other reads the GUI makes, each from its fixture.
        foreach (var (want, file) in new[] { (new[] { "circadian", "show", "--json" }, "circadian.json"),
                     (["guided-setup", "--json"], "guided-setup.json"), (["firmware-lock", "--json"], "firmware-lock.json") })
            if (rest.SequenceEqual(want) && File.Exists(Path.Combine(dir, file))) return (0, File.ReadAllText(Path.Combine(dir, file)));
        return (0, "Done (fake kc9k): kc9k " + string.Join(' ', rest));
    }

    static (int Code, string Output) RunReal(string exe, string[] args, string stdin)
    {
        var info = new System.Diagnostics.ProcessStartInfo(exe)
        {
            UseShellExecute = false, RedirectStandardInput = true, RedirectStandardOutput = true, RedirectStandardError = true,
            StandardOutputEncoding = Encoding.UTF8, StandardErrorEncoding = Encoding.UTF8,
        };
        foreach (var a in args) info.ArgumentList.Add(a);
        info.Environment.Remove("KC9K_FAKE_REAL");
        using var p = System.Diagnostics.Process.Start(info)!;
        var stdout = p.StandardOutput.ReadToEndAsync();
        var stderr = p.StandardError.ReadToEndAsync();
        p.StandardInput.Write(stdin);
        p.StandardInput.Close();
        p.WaitForExit();
        var answer = (p.ExitCode, (stdout.Result + stderr.Result).TrimEnd());
        // What the real CLI said, for a failing test's log.
        var dir = Environment.GetEnvironmentVariable("KC9K_FAKE_DIR");
        if (!string.IsNullOrEmpty(dir))
            try { File.AppendAllText(Path.Combine(dir, "real.log"), $"kc9k {string.Join(' ', args)} -> {answer.ExitCode}: {(answer.Item2.Length > 300 ? answer.Item2[..300] + "…" : answer.Item2)}\n"); }
            catch (IOException) { }
        return answer;
    }

    /// <summary>The commands recorded in a log, oldest first.</summary>
    public static List<(List<string> Args, string Stdin)> ReadLog(string log)
    {
        var output = new List<(List<string>, string)>();
        if (!File.Exists(log)) return output;
        foreach (var l in File.ReadAllLines(log))
        {
            if (l.Trim().Length == 0) continue;
            var node = JsonNode.Parse(l)!;
            output.Add((node["args"]!.AsArray().Select(a => a!.GetValue<string>()).ToList(), node["stdin"]?.GetValue<string>() ?? ""));
        }
        return output;
    }
}
