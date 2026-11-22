// The application: Fluent resources, the theme, the main window.
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Markup;
using Microsoft.UI.Xaml.XamlTypeInfo;
using Kc9k.Core;
using Microsoft.Windows.AppLifecycle;

namespace Kc9k.Gui;

public sealed partial class App : Application, IXamlMetadataProvider
{
    // Without XAML markup nothing generates the type information the Fluent
    // control templates need; WinUI's own provider gives it.
    readonly XamlControlsXamlMetaDataProvider provider = new();

    public static new App Current => (App)Application.Current;
    public MainWindow? Window { get; private set; }
    public string[] Args { get; }
    public bool StartHidden => Args.Contains("--hidden");
    public StartArgs StartAsked => StartArgs.Parse(Args);

    public App(string[] args)
    {
        Args = args;
        // Brushes looked up in code follow the app's theme: start in the one chosen.
        if (Prefs.Current.Theme == "dark") RequestedTheme = ApplicationTheme.Dark;
        else if (Prefs.Current.Theme == "light") RequestedTheme = ApplicationTheme.Light;
        UnhandledException += (_, e) =>
        {
            Log.Write("unhandled: " + e.Exception);
            e.Handled = true; // shown, never a crash that loses what was typed
            Window?.Controller.ShowError("Something went wrong in the window: " + e.Message);
        };
        // W52: what WinUI's handler never sees. A task nobody awaited is
        // logged and kept alive; an exception on another thread ends the
        // process, so it leaves a note the next start shows once.
        TaskScheduler.UnobservedTaskException += (_, e) =>
        {
            Log.Write("unobserved task: " + e.Exception);
            e.SetObserved();
        };
        AppDomain.CurrentDomain.UnhandledException += (_, e) =>
        {
            Log.Write("crash: " + e.ExceptionObject);
            if (e.IsTerminating) CrashNote.Write(DateTime.Now);
        };
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        Resources.MergedDictionaries.Add(new XamlControlsResources());
        Theme.AddResources(Resources);
        Window = new MainWindow();
        // A second start (W36): its window comes forward with what it asked for.
        AppInstance.GetCurrent().Activated += (_, e) =>
        {
            var asked = e.Data is Windows.ApplicationModel.Activation.ILaunchActivatedEventArgs launch ? StartArgs.Parse(launch.Arguments) : StartArgs.Parse("");
            Window.DispatcherQueue.TryEnqueue(() =>
            {
                Window.ShowFromTray();
                _ = Window.Perform(asked);
            });
        };
        if (!StartHidden || !Prefs.Current.Tray) Window.Activate();
        else Window.StartInTray();
        if (StartAsked.Any) _ = Window.Perform(StartAsked);
        if (CrashNote.Take() is { } crashed)
            Window.Controller.ShowError($"Killer Chicken 9000 closed unexpectedly at {crashed:g}. The details are in {Log.FilePath}.",
                "Closed unexpectedly");
        TestHook.Arm(Window);
        _ = Task.Run(JumpList.Write);
    }

    public IXamlType GetXamlType(Type type) => provider.GetXamlType(type);
    public IXamlType GetXamlType(string fullName) => provider.GetXamlType(fullName);
    public XmlnsDefinition[] GetXmlnsDefinitions() => provider.GetXmlnsDefinitions();
}

/// <summary>A small log for what goes wrong (%LOCALAPPDATA%\kc9k\gui.log).</summary>
public static class Log
{
    static readonly object Gate = new();

    public static string FilePath =>
        Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "kc9k", "gui.log");

    public static void Write(string text)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(FilePath)!);
            lock (Gate) File.AppendAllText(FilePath, $"{DateTime.Now:s} {text}\n");
        }
        catch (Exception) { }
    }
}

/// <summary>W52: a crash leaves the time beside gui.json; the next start
/// says so once and removes it.</summary>
public static class CrashNote
{
    static string FilePath => Path.Combine(Prefs.Folder, "gui-crash.txt");

    public static void Write(DateTime at)
    {
        try
        {
            Directory.CreateDirectory(Prefs.Folder);
            File.WriteAllText(FilePath, at.ToString("o"));
        }
        catch (Exception) { }
    }

    public static DateTime? Take()
    {
        try
        {
            if (!File.Exists(FilePath)) return null;
            var text = File.ReadAllText(FilePath).Trim();
            File.Delete(FilePath);
            return DateTime.TryParse(text, null, System.Globalization.DateTimeStyles.RoundtripKind, out var at) ? at : DateTime.Now;
        }
        catch (Exception) { return null; }
    }
}

/// <summary>Test-only (W52): KC9K_GUI_TEST_THROW makes the window fail on
/// purpose once the first state is shown: "window" throws in two ticks of
/// a XAML timer (one dialog), "task" leaves a failed task unobserved, "crash"
/// throws on another thread (the process ends). Unset, nothing happens.</summary>
public static class TestHook
{
    public static void Arm(MainWindow window)
    {
        var what = Environment.GetEnvironmentVariable("KC9K_GUI_TEST_THROW");
        if (string.IsNullOrEmpty(what)) return;
        // A XAML timer: the framework keeps it while it runs (a DispatcherQueueTimer
        // nobody holds is collected and never fires), and an exception in its Tick
        // reaches Application.UnhandledException as one from a real handler would.
        var timer = new DispatcherTimer { Interval = TimeSpan.FromSeconds(5) };
        var ticks = 0;
        timer.Tick += (_, _) =>
        {
            ticks++;
            switch (what)
            {
                case "window":
                    if (ticks >= 2) timer.Stop();
                    else timer.Interval = TimeSpan.FromMilliseconds(300); // the second while the first is shown
                    throw new InvalidOperationException("test hook");
                case "task":
                    timer.Stop();
                    _ = Task.Run(() => throw new InvalidOperationException("test hook"));
                    Thread.Sleep(200);
                    GC.Collect();
                    GC.WaitForPendingFinalizers();
                    break;
                case "crash":
                    timer.Stop();
                    new Thread(() => throw new InvalidOperationException("test hook")).Start();
                    break;
                default:
                    timer.Stop();
                    break;
            }
        };
        timer.Start();
    }
}
