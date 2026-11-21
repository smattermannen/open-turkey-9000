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
    readonly string name;

    public GuiApp(string name, string theme = "light", Action<string>? prepare = null)
    {
        this.name = name;
        Dir = Path.Combine(Path.GetTempPath(), "kc9k-ui-" + name + "-" + Guid.NewGuid().ToString("N")[..8]);
        Directory.CreateDirectory(Dir);
        foreach (var f in Directory.GetFiles(Path.Combine(AppContext.BaseDirectory, "fixtures")))
            File.Copy(f, Path.Combine(Dir, Path.GetFileName(f)));
        File.WriteAllText(Path.Combine(Dir, "gui.json"), $$"""{"theme": "{{theme}}", "tray": false, "window_width": 1280, "window_height": 900}""");
        prepare?.Invoke(Dir);
        var exe = Environment.GetEnvironmentVariable("KC9K_GUI_EXE") ?? throw new InvalidOperationException("KC9K_GUI_EXE is not set");
        var fake = Environment.GetEnvironmentVariable("KC9K_FAKE_EXE") ?? throw new InvalidOperationException("KC9K_FAKE_EXE is not set");
        var info = new ProcessStartInfo(exe) { UseShellExecute = false };
        info.Environment["KC9K_CLI"] = fake;
        info.Environment["KC9K_FAKE_DIR"] = Dir;
        info.Environment["KC9K_GUI_PREFS"] = Path.Combine(Dir, "gui.json");
        info.Environment["KC9K_GUI_MULTI"] = "1";
        App = Application.Launch(info);
        Window = Retry.WhileNull(() => App.GetMainWindow(Automation, TimeSpan.FromSeconds(2)),
            TimeSpan.FromSeconds(60), TimeSpan.FromMilliseconds(500), throwOnTimeout: true).Result!;
        Window.SetForeground();
        // The first state is read in the background.
        WaitFor(() => ById("Headline")?.Name is { Length: > 0 } || ById("CoverUnlock_dashboard") is not null, "the first state");
    }

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
    public void Invoke(string id) => Invoke(Find(id));

    public void Invoke(AutomationElement e)
    {
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

    public AutomationElement FindButton(string text, int seconds = 15) =>
        Retry.WhileNull(() =>
                Window.FindFirstDescendant(c => c.ByControlType(ControlType.Button).And(c.ByName(text))) ??
                Automation.GetDesktop().FindFirstDescendant(c => c.ByControlType(ControlType.Button).And(c.ByName(text)).And(c.ByProcessId(App.ProcessId))),
            TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200), throwOnTimeout: true, timeoutMessage: $"no button {text}").Result!;

    /// <summary>Types into a text box and presses Enter in it.</summary>
    public void TypeEnter(string id, string text)
    {
        var box = Find(id).AsTextBox();
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

    public void Page(string key)
    {
        var item = Find("Nav_" + key);
        item.Click();
        Wait.UntilInputIsProcessed();
        Thread.Sleep(400);
    }

    public List<List<string>> Commands() => FakeCli.ReadLog(Log).Select(c => c.Args).ToList();

    /// <summary>Whether the GUI ran a command starting with these arguments.</summary>
    public bool Ran(params string[] args) =>
        Commands().Any(c => Strip(c).Take(args.Length).SequenceEqual(args));

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

    public void Dispose()
    {
        try { App.Close(); } catch (Exception) { }
        try { if (!App.HasExited) App.Kill(); } catch (Exception) { }
        App.Dispose();
        Automation.Dispose();
    }
}
