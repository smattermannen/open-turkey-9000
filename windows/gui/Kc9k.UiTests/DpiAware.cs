// The test process is per-monitor DPI aware (laptop, 200 %): UI Automation,
// screenshots and window moves then all use physical pixels, as gui.json does.
using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;

namespace Kc9k.UiTests;

static class DpiAware
{
    [DllImport("user32.dll")]
    static extern bool SetProcessDpiAwarenessContext(IntPtr context);

#pragma warning disable CA2255 // a test assembly: it runs first, as meant
    [ModuleInitializer]
    internal static void Init()
    {
        if (!OperatingSystem.IsWindows()) return;
        try { SetProcessDpiAwarenessContext(new IntPtr(-4)); } // PER_MONITOR_AWARE_V2
        catch (Exception) { }
    }
}
