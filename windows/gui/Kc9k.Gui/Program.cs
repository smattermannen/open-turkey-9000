// Entry point (no XAML, so no generated Main): one window per user; a second
// start brings the first one forward (§4.10 tray: the window may be hidden)
// with its arguments (W36: --page KEY, --do ACTION).
using Microsoft.UI.Dispatching;
using Microsoft.UI.Xaml;
using Microsoft.Windows.AppLifecycle;

namespace Kc9k.Gui;

public static class Program
{
    [STAThread]
    public static int Main(string[] args)
    {
        WinRT.ComWrappersSupport.InitializeComWrappers();
        JumpList.SetAppId();
        var key = AppInstance.FindOrRegisterForKey("kc9k-gui-" + Environment.UserName);
        if (!key.IsCurrent && Environment.GetEnvironmentVariable("KC9K_GUI_MULTI") != "1")
        {
            // Hand the start to the running one, which shows its window.
            var activation = AppInstance.GetCurrent().GetActivatedEventArgs();
            key.RedirectActivationToAsync(activation).AsTask().Wait(TimeSpan.FromSeconds(5));
            return 0;
        }
        Application.Start(p =>
        {
            var context = new DispatcherQueueSynchronizationContext(DispatcherQueue.GetForCurrentThread());
            SynchronizationContext.SetSynchronizationContext(context);
            _ = new App(args);
        });
        return 0;
    }
}
