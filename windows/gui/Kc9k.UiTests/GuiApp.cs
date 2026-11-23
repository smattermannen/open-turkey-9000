// Starts the built GUI (KC9K_GUI_EXE) against the fake kc9k (KC9K_FAKE_EXE)
// in a fresh folder of fixtures, with its own preferences file, and gives
// the tests its window, the commands it ran, and screenshots
// (KC9K_SCREENSHOTS) to look at.
using System.Diagnostics;
using FakeKc9k;
using FlaUI.Core;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Capturing;
using FlaUI.Core.Conditions;
using FlaUI.Core.Definitions;
using FlaUI.Core.Input;
using FlaUI.Core.Tools;
using FlaUI.UIA3;

namespace Kc9k.UiTests;

public sealed class GuiApp : IDisposable
{
    public string Dir { get; }
    public Application App { get; }
    public UIA3Automation Automation { get; } = new();
    public Window Window { get; }
    public string Log => Path.Combine(Dir, "commands.log");
    /// <summary>How the GUI was started: a second start uses the same (W36).</summary>
    public ProcessStartInfo StartInfo { get; private set; } = null!;
    readonly string name;

    readonly bool real_;

    public GuiApp(string name, string theme = "light", Action<string>? prepare = null, bool real = false, string ready = "Headline",
        string arguments = "", bool multi = true, string? dir = null, IDictionary<string, string>? env = null,
        bool scratch = false, string[][]? setup = null)
    {
        Scratch = scratch;
        real_ = real;
        this.name = name;
        // dir: start again in an earlier start's folder (W52: what a crash left).
        Dir = dir ?? Path.Combine(Path.GetTempPath(), "kc9k-ui-" + name + "-" + Guid.NewGuid().ToString("N")[..8]);
        if (dir is null)
        {
            Directory.CreateDirectory(Dir);
            foreach (var f in Directory.GetFiles(Path.Combine(AppContext.BaseDirectory, "fixtures")))
                File.Copy(f, Path.Combine(Dir, Path.GetFileName(f)));
            File.WriteAllText(Path.Combine(Dir, "gui.json"), $$"""{"theme": "{{theme}}", "tray": false, "window_width": 1280, "window_height": 900}""");
        }
        if (scratch)
        {
            // W32: the real kc9k.exe on a scratch database (KC9K_DB: it runs
            // without the service and applies nothing), through the fake,
            // which logs each command as it does for the fixtures.
            ScratchExe = Environment.GetEnvironmentVariable("KC9K_SCRATCH_EXE") is { Length: > 0 } x && File.Exists(x) ? x
                : throw new InvalidOperationException("KC9K_SCRATCH_EXE (a built kc9k.exe) is not set");
            foreach (var args in setup ?? [])
            {
                var (ok, output) = Cli(args);
                if (!ok) throw new InvalidOperationException($"setup kc9k {string.Join(' ', args)}: {output}");
            }
        }
        prepare?.Invoke(Dir);
        var exe = Environment.GetEnvironmentVariable("KC9K_GUI_EXE") ?? throw new InvalidOperationException("KC9K_GUI_EXE is not set");
        // KC9K_REAL_EXE: the real kc9k.exe (installed, its service running) instead of the fake.
        var realExe = real_ ? Environment.GetEnvironmentVariable("KC9K_REAL_EXE") : null;
        var fake = realExe ?? Environment.GetEnvironmentVariable("KC9K_FAKE_EXE") ?? throw new InvalidOperationException("KC9K_FAKE_EXE is not set");
        var info = new ProcessStartInfo(exe, arguments) { UseShellExecute = false };
        info.Environment["KC9K_CLI"] = fake;
        info.Environment["KC9K_FAKE_DIR"] = Dir;
        info.Environment["KC9K_GUI_PREFS"] = Path.Combine(Dir, "gui.json");
        if (multi) info.Environment["KC9K_GUI_MULTI"] = "1";
        if (scratch)
        {
            info.Environment["KC9K_FAKE_REAL"] = ScratchExe;
            info.Environment["KC9K_DB"] = Db;
        }
        foreach (var (k, v) in env ?? new Dictionary<string, string>()) info.Environment[k] = v;
        StartInfo = info;
        App = Application.Launch(info);
        Window = Retry.WhileNull(() => App.GetMainWindow(Automation, TimeSpan.FromSeconds(2)),
            TimeSpan.FromSeconds(60), TimeSpan.FromMilliseconds(500), throwOnTimeout: true).Result!;
        Window.SetForeground();
        // The first state is read in the background.
        WaitFor(() => ById(ready)?.Name is { Length: > 0 } || ById("CoverUnlock_dashboard") is not null, "the first state", real_ ? 60 : 20);
        if (scratch)
        {
            // A new database has no browser checking in: the GUI asks once.
            var later = Retry.WhileNull(() => FindButtonOrNull("Later"), TimeSpan.FromSeconds(8), TimeSpan.FromMilliseconds(250)).Result;
            if (later is not null) Invoke(later);
        }
    }

    /// <summary>Whether the GUI runs the real kc9k.exe on a scratch database.</summary>
    public bool Scratch { get; }
    public string? ScratchExe { get; }
    public string Db => Path.Combine(Dir, "scratch.db");

    /// <summary>Runs the real kc9k.exe on the scratch database (setup, checks).</summary>
    public (bool Ok, string Output) Cli(params string[] args)
    {
        var info = new ProcessStartInfo(ScratchExe!) { UseShellExecute = false, RedirectStandardOutput = true, RedirectStandardError = true, RedirectStandardInput = true };
        foreach (var a in args) info.ArgumentList.Add(a);
        info.Environment["KC9K_DB"] = Db;
        using var p = Process.Start(info)!;
        p.StandardInput.Close();
        var output = p.StandardOutput.ReadToEndAsync();
        var error = p.StandardError.ReadToEndAsync();
        p.WaitForExit();
        return (p.ExitCode == 0, (output.Result + error.Result).Trim());
    }

    /// <summary>The scratch database's state, as `kc9k gui-state` prints it
    /// (read again when caught between a draft's save and its removal).</summary>
    public System.Text.Json.Nodes.JsonObject RealState()
    {
        var (ok, output) = Cli("gui-state");
        for (var i = 0; i < 5 && !ok; i++)
        {
            Thread.Sleep(500);
            (ok, output) = Cli("gui-state");
        }
        if (!ok) throw new InvalidOperationException("kc9k gui-state: " + output);
        return System.Text.Json.Nodes.JsonNode.Parse(output[output.IndexOf('{')..])!.AsObject();
    }

    /// <summary>A block of the scratch database's state.</summary>
    public System.Text.Json.Nodes.JsonObject RealBlock(string name) =>
        RealState()["blocks"]!.AsArray().First(b => (string?)b!["name"] == name)!.AsObject();

    public AutomationElement? ById(string id, AutomationElement? root = null) =>
        (root ?? Window).FindFirstDescendant(c => c.ByAutomationId(id)) ??
        (root is null ? Automation.GetDesktop().FindFirstDescendant(c => c.ByAutomationId(id).And(c.ByProcessId(App.ProcessId))) : null);

    public AutomationElement? ByName(string text, AutomationElement? root = null) =>
        (root ?? Window).FindFirstDescendant(c => c.ByName(text));

    public AutomationElement Find(string id, int seconds = 15) =>
        Retry.WhileNull(() => ById(id), TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200), throwOnTimeout: true,
            timeoutMessage: $"no element {id}").Result!;

    public AutomationElement FindName(string text, int seconds = 15) =>
        Retry.WhileNull(() => ByName(text) ?? DesktopByName(text), TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200), throwOnTimeout: true,
            timeoutMessage: $"nothing named {text}").Result!;

    /// <summary>Menus and flyouts open in their own popup windows.</summary>
    public AutomationElement? DesktopByName(string text) =>
        Automation.GetDesktop().FindFirstDescendant(c => c.ByName(text).And(c.ByProcessId(App.ProcessId)));

    /// <summary>Presses a button (or opens a drop-down) through UI Automation,
    /// wherever it is on the page: no pointer, no scrolling.</summary>
    public void Invoke(string id)
    {
        // An element found while its page is still being shown can go stale
        // (UIA_E_ELEMENTNOTAVAILABLE): find it again and press it again.
        for (var attempt = 1; ; attempt++)
        {
            try
            {
                Invoke(Find(id));
                return;
            }
            catch (Exception e) when (attempt < 3 && e is System.ComponentModel.Win32Exception or System.Runtime.InteropServices.COMException)
            {
                Thread.Sleep(500);
            }
        }
    }

    public void Invoke(AutomationElement e)
    {
        // A tick box is clicked where a person clicks it.
        if (e.ControlType == ControlType.CheckBox && !e.IsOffscreen && e.Properties.ProcessId.ValueOrDefault == App.ProcessId)
        {
            Show(e);
            var r = e.BoundingRectangle;
            if (!r.IsEmpty)
            {
                Front();
                Mouse.Click(new System.Drawing.Point(r.Left + Math.Min(12, r.Width / 2), r.Top + r.Height / 2));
                Wait.UntilInputIsProcessed();
                return;
            }
        }
        if (e.Patterns.Invoke.IsSupported) e.Patterns.Invoke.Pattern.Invoke();
        else if (e.Patterns.SelectionItem.IsSupported) e.Patterns.SelectionItem.Pattern.Select();
        else if (e.Patterns.ExpandCollapse.IsSupported) e.Patterns.ExpandCollapse.Pattern.Expand();
        else if (e.Patterns.Toggle.IsSupported) e.Patterns.Toggle.Pattern.Toggle();
        else e.Click();
        Wait.UntilInputIsProcessed();
    }

    /// <summary>A menu's item: menus open in a popup of their own.</summary>
    public AutomationElement FindMenuItem(string text, int seconds = 15) =>
        Retry.WhileNull(() =>
                Window.FindFirstDescendant(c => c.ByControlType(ControlType.MenuItem).And(c.ByName(text))) ??
                Automation.GetDesktop().FindFirstDescendant(c => c.ByControlType(ControlType.MenuItem).And(c.ByName(text)).And(c.ByProcessId(App.ProcessId))),
            TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: $"no menu item {text}").Result!;

    public AutomationElement? FindButtonOrNull(string text) =>
        Window.FindFirstDescendant(c => c.ByControlType(ControlType.Button).And(c.ByName(text))) ??
        Automation.GetDesktop().FindFirstDescendant(c => c.ByControlType(ControlType.Button).And(c.ByName(text)).And(c.ByProcessId(App.ProcessId)));

    public AutomationElement FindButton(string text, int seconds = 15) =>
        Retry.WhileNull(() =>
                Window.FindFirstDescendant(c => c.ByControlType(ControlType.Button).And(c.ByName(text))) ??
                Automation.GetDesktop().FindFirstDescendant(c => c.ByControlType(ControlType.Button).And(c.ByName(text)).And(c.ByProcessId(App.ProcessId))),
            TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: $"no button {text}").Result!;

    /// <summary>The window in front, so keys reach it (the runner's own
    /// console can come forward now and then).</summary>
    public void Front()
    {
        try { Window.SetForeground(); } catch (Exception) { }
        Thread.Sleep(150);
    }

    /// <summary>Scrolls an element into the window (one out of view has no
    /// rectangle) and gives it back, found again.</summary>
    public AutomationElement Show(string id)
    {
        Front();
        var e = Find(id);
        try
        {
            if (e.Patterns.ScrollItem.IsSupported) e.Patterns.ScrollItem.Pattern.ScrollIntoView();
            else e.Focus();
        }
        catch (Exception) { try { e.Focus(); } catch (Exception) { } }
        Thread.Sleep(400);
        return Find(id);
    }

    public static void Show(AutomationElement e)
    {
        try
        {
            if (e.Patterns.ScrollItem.IsSupported) e.Patterns.ScrollItem.Pattern.ScrollIntoView();
            else e.Focus();
        }
        catch (Exception) { }
        Thread.Sleep(400);
    }

    /// <summary>Types into a text box and presses Enter in it.</summary>
    public void TypeEnter(string id, string text)
    {
        var box = Find(id).AsTextBox();
        Front();
        box.Focus();
        box.Text = text;
        Keyboard.Press(FlaUI.Core.WindowsAPI.VirtualKeyShort.RETURN);
        Wait.UntilInputIsProcessed();
    }

    public void WaitFor(Func<bool> condition, string what, int seconds = 20)
    {
        var ok = Retry.WhileFalse(() => { try { return condition(); } catch (Exception) { return false; } },
            TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200)).Result;
        if (!ok) throw new TimeoutException("waited for " + what);
    }

    /// <summary>The Blocks page's cards (out of an opened block, mode or automation).</summary>
    public void Cards()
    {
        Page("blocks");
        foreach (var back in new[] { "BackToBlock", "ModeBack", "BackToBlocks" })
            if (ById(back) is { } b)
            {
                Invoke(b);
                Thread.Sleep(300);
            }
        if (ById("BackToBlocks") is { } again) Invoke(again);
    }

    public void Page(string key)
    {
        Front();
        var item = Find("Nav_" + key);
        // Click its icon, where a person clicks: with the sidebar icons only
        // (below 640 DIP) the item's box is wider than the pane.
        var r = item.BoundingRectangle;
        Mouse.Click(new System.Drawing.Point(r.Left + Math.Min(24, r.Width / 2), r.Top + r.Height / 2));
        Wait.UntilInputIsProcessed();
        Thread.Sleep(400);
    }

    public List<List<string>> Commands() => FakeCli.ReadLog(Log).Select(c => c.Args).ToList();

    /// <summary>Whether the GUI ran a command starting with these arguments.</summary>
    public bool Ran(params string[] args) =>
        Commands().Any(c => Strip(c).Take(args.Length).SequenceEqual(args));

    /// <summary>Whether a command ran holding all these arguments (any order).</summary>
    public bool RanWith(params string[] parts) => Commands().Any(c => parts.All(c.Contains));

    static IEnumerable<string> Strip(List<string> c)
    {
        var i = 0;
        while (i < c.Count && c[i].StartsWith("--")) i += c[i] == "--access-lines" ? 2 : 1;
        return c.Skip(i);
    }

    public void Shot(string what)
    {
        var dir = Environment.GetEnvironmentVariable("KC9K_SCREENSHOTS");
        if (string.IsNullOrEmpty(dir)) return;
        Directory.CreateDirectory(dir);
        Thread.Sleep(300);
        try { Capture.Element(Window).ToFile(Path.Combine(dir, $"{name}-{what}.png")); }
        catch (Exception e) { Console.WriteLine("screenshot failed: " + e.Message); }
    }

    /// <summary>Changes the fixture gui-state.json in a test's folder (for prepare:).</summary>
    public static void EditState(string dir, Action<System.Text.Json.Nodes.JsonObject> change)
    {
        var path = Path.Combine(dir, "gui-state.json");
        var state = System.Text.Json.Nodes.JsonNode.Parse(File.ReadAllText(path))!.AsObject();
        change(state);
        File.WriteAllText(path, state.ToJsonString());
    }

    /// <summary>Canned answers of the fake kc9k (for prepare:): leading arguments, ok, output.</summary>
    public static void Respond(string dir, params (string[] Args, bool Ok, string Output)[] answers)
    {
        var list = new System.Text.Json.Nodes.JsonArray();
        foreach (var (args, ok, output) in answers)
            list.Add(new System.Text.Json.Nodes.JsonObject
            {
                ["args"] = new System.Text.Json.Nodes.JsonArray([.. args.Select(a => (System.Text.Json.Nodes.JsonNode)System.Text.Json.Nodes.JsonValue.Create(a)!)]),
                ["ok"] = ok, ["output"] = output,
            });
        File.WriteAllText(Path.Combine(dir, "responses.json"), list.ToJsonString());
    }

    [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)]
    struct MouseInput { public int Dx, Dy; public uint Data, Flags, Time; public IntPtr Extra; }
    [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)]
    struct Input { public uint Type; public MouseInput Mouse; }
    [System.Runtime.InteropServices.DllImport("user32.dll")] static extern uint SendInput(uint count, Input[] inputs, int size);
    [System.Runtime.InteropServices.DllImport("user32.dll")] static extern int GetSystemMetrics(int index);

    static void Send(System.Drawing.Point p, uint flags)
    {
        const uint Move = 0x1, Absolute = 0x8000;
        int w = GetSystemMetrics(0), h = GetSystemMetrics(1);
        var input = new Input { Type = 0, Mouse = new MouseInput { Dx = (int)Math.Round(p.X * 65535.0 / (w - 1)), Dy = (int)Math.Round(p.Y * 65535.0 / (h - 1)), Flags = flags | Move | Absolute } };
        SendInput(1, [input], System.Runtime.InteropServices.Marshal.SizeOf<Input>());
    }

    /// <summary>A pointer drag as a hand makes it, all through SendInput (the
    /// pointer moved with SetCursorPos is no drag to WinUI): press, a slow
    /// path, release.</summary>
    public static void Drag(System.Drawing.Point from, System.Drawing.Point to, int steps = 24)
    {
        const uint Down = 0x2, Up = 0x4;
        Send(from, 0);
        Thread.Sleep(100);
        Send(from, Down);
        Thread.Sleep(150);
        for (var i = 1; i <= steps; i++)
        {
            Send(new(from.X + (to.X - from.X) * i / steps, from.Y + (to.Y - from.Y) * i / steps), 0);
            Thread.Sleep(30);
        }
        Thread.Sleep(150);
        Send(to, Up);
        Wait.UntilInputIsProcessed();
        Thread.Sleep(300);
    }

    /// <summary>The GUI's log lines (shared by the tests; for a failure's output).</summary>
    public static IEnumerable<string> LogLines(string contains)
    {
        var log = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "kc9k", "gui.log");
        try { return File.Exists(log) ? File.ReadAllLines(log).Where(l => l.Contains(contains)).TakeLast(20).ToList() : []; }
        catch (IOException) { return []; }
    }

    /// <summary>Answers Windows' own open dialog (W4: file dialogs) as a person
    /// would: the full path typed into File name, then Enter.</summary>
    public void PickFile(string path, int seconds = 30)
    {
        var dialog = Retry.WhileNull(() =>
                Window.FindFirstDescendant(c => c.ByClassName("#32770")) ??
                Automation.GetDesktop().FindFirstChild(c => c.ByClassName("#32770").And(c.ByProcessId(App.ProcessId))),
            TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(250), throwOnTimeout: true, timeoutMessage: "no file dialog").Result!;
        var name = Retry.WhileNull(() => dialog.FindFirstDescendant(c => c.ByAutomationId("1148").And(c.ByControlType(ControlType.Edit))) ??
                dialog.FindFirstDescendant(c => c.ByControlType(ControlType.Edit)),
            TimeSpan.FromSeconds(10), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: "no File name box").Result!;
        name.Focus();
        name.AsTextBox().Text = path;
        Keyboard.Press(FlaUI.Core.WindowsAPI.VirtualKeyShort.RETURN);
        Wait.UntilInputIsProcessed();
        WaitFor(() => Automation.GetDesktop().FindFirstChild(c => c.ByClassName("#32770").And(c.ByProcessId(App.ProcessId))) is null, "the file dialog closed");
    }

    /// <summary>A 1×1 PNG in the test's folder.</summary>
    public string Picture(string name)
    {
        var path = Path.Combine(Dir, name);
        File.WriteAllBytes(path, Convert.FromBase64String("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="));
        return path;
    }

    /// <summary>Makes the fake kc9k slow (for prepare:): commands starting with
    /// these arguments wait ms first; other answers stay.</summary>
    public static void Slow(string dir, int ms, params string[] args)
    {
        var path = Path.Combine(dir, "responses.json");
        var list = File.Exists(path) ? System.Text.Json.Nodes.JsonNode.Parse(File.ReadAllText(path))!.AsArray() : [];
        list.Insert(0, new System.Text.Json.Nodes.JsonObject
        {
            ["args"] = new System.Text.Json.Nodes.JsonArray([.. args.Select(a => (System.Text.Json.Nodes.JsonNode)System.Text.Json.Nodes.JsonValue.Create(a)!)]),
            ["delay_ms"] = ms,
        });
        File.WriteAllText(path, list.ToJsonString());
    }

    public void Dispose()
    {
        // What was on screen at the end (a failure's last view), and what
        // the real CLI answered.
        try { if (!App.HasExited) Shot("last"); } catch (Exception) { }
        var real = Path.Combine(Dir, "real.log");
        if (File.Exists(real))
            try { Console.WriteLine($"--- {name}: the real kc9k's answers (last 15)\n" + string.Join("\n", File.ReadAllLines(real).TakeLast(15))); }
            catch (IOException) { }
        try { App.Close(); } catch (Exception) { }
        try { if (!App.HasExited) App.Kill(); } catch (Exception) { }
        App.Dispose();
        Automation.Dispose();
    }
}
