// The application: Fluent resources, the theme, the main window.
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Markup;
using Microsoft.UI.Xaml.XamlTypeInfo;
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
    }

    protected override void OnLaunched(LaunchActivatedEventArgs args)
    {
        Resources.MergedDictionaries.Add(new XamlControlsResources());
        Theme.AddResources(Resources);
        Window = new MainWindow();
        AppInstance.GetCurrent().Activated += (_, _) => Window.DispatcherQueue.TryEnqueue(Window.ShowFromTray);
        if (!StartHidden || !Prefs.Current.Tray) Window.Activate();
        else Window.StartInTray();
    }

    public IXamlType GetXamlType(Type type) => provider.GetXamlType(type);
    public IXamlType GetXamlType(string fullName) => provider.GetXamlType(fullName);
    public XmlnsDefinition[] GetXmlnsDefinitions() => provider.GetXmlnsDefinitions();
}

/// <summary>A small log for what goes wrong (%LOCALAPPDATA%\kc9k\gui.log).</summary>
public static class Log
{
    static readonly object Gate = new();

    public static void Write(string text)
    {
        try
        {
            var dir = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "kc9k");
            Directory.CreateDirectory(dir);
            lock (Gate) File.AppendAllText(Path.Combine(dir, "gui.log"), $"{DateTime.Now:s} {text}\n");
        }
        catch (Exception) { }
    }
}
