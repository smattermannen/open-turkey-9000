// §4.10 tray: an icon in the notification area whose tooltip says what is
// on; click opens the window, its menu has Open and Quit. Closing the window
// keeps the GUI there (Prefs.Tray). Win32 Shell_NotifyIcon on the window's
// own handle (a subclass receives the icon's messages): WinUI has no tray.
using System.Runtime.InteropServices;
using Kc9k.Core;

namespace Kc9k.Gui;

public sealed class Tray : IDisposable
{
    const int WM_APP_TRAY = 0x8000 + 17, WM_LBUTTONUP = 0x0202, WM_RBUTTONUP = 0x0205, WM_CONTEXTMENU = 0x007B, NIN_SELECT = 0x0400, NIN_KEYSELECT = 0x0401;
    const uint NIM_ADD = 0, NIM_MODIFY = 1, NIM_DELETE = 2, NIM_SETVERSION = 4;
    const uint NIF_MESSAGE = 1, NIF_ICON = 2, NIF_TIP = 4, NIF_SHOWTIP = 0x80;
    const uint MF_STRING = 0, MF_GRAYED = 1, MF_SEPARATOR = 0x800, TPM_RETURNCMD = 0x100, TPM_RIGHTBUTTON = 2;

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct NotifyIconData
    {
        public int Size;
        public nint Window;
        public uint Id, Flags, CallbackMessage;
        public nint Icon;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 128)] public string Tip;
        public uint State, StateMask;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 256)] public string Info;
        public uint Version;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 64)] public string InfoTitle;
        public uint InfoFlags;
        public Guid Item;
        public nint BalloonIcon;
    }

    delegate nint SubclassProc(nint hwnd, uint msg, nint wParam, nint lParam, nuint id, nuint data);

    [DllImport("shell32.dll", CharSet = CharSet.Unicode, EntryPoint = "Shell_NotifyIconW")]
    static extern bool ShellNotifyIcon(uint message, ref NotifyIconData data);
    [DllImport("comctl32.dll")] static extern bool SetWindowSubclass(nint hwnd, SubclassProc proc, nuint id, nuint data);
    [DllImport("comctl32.dll")] static extern bool RemoveWindowSubclass(nint hwnd, SubclassProc proc, nuint id);
    [DllImport("comctl32.dll")] static extern nint DefSubclassProc(nint hwnd, uint msg, nint wParam, nint lParam);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern nint LoadImage(nint inst, string name, uint type, int cx, int cy, uint load);
    [DllImport("user32.dll")] static extern nint CreatePopupMenu();
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern bool AppendMenu(nint menu, uint flags, nuint id, string? text);
    [DllImport("user32.dll")] static extern int TrackPopupMenuEx(nint menu, uint flags, int x, int y, nint hwnd, nint tpm);
    [DllImport("user32.dll")] static extern bool DestroyMenu(nint menu);
    [DllImport("user32.dll")] static extern bool GetCursorPos(out Point pt);
    [DllImport("user32.dll")] static extern bool SetForegroundWindow(nint hwnd);
    [StructLayout(LayoutKind.Sequential)] struct Point { public int X, Y; }

    readonly MainWindow window;
    readonly nint hwnd;
    readonly SubclassProc proc;
    NotifyIconData data;
    string status = "Killer Chicken 9000";
    bool added;

    Tray(MainWindow window)
    {
        this.window = window;
        hwnd = WinRT.Interop.WindowNative.GetWindowHandle(window);
        proc = Proc;
        SetWindowSubclass(hwnd, proc, 9000, 0);
        var icon = LoadImage(0, Path.Combine(AppContext.BaseDirectory, "Assets", "kc9k.ico"), 1, 0, 0, 0x10 | 0x40); // IMAGE_ICON, LR_LOADFROMFILE | LR_DEFAULTSIZE
        data = new NotifyIconData
        {
            Size = Marshal.SizeOf<NotifyIconData>(), Window = hwnd, Id = 1, Flags = NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP,
            CallbackMessage = WM_APP_TRAY, Icon = icon, Tip = status, Info = "", InfoTitle = "",
        };
        added = ShellNotifyIcon(NIM_ADD, ref data);
        data.Version = 4; // NOTIFYICON_VERSION_4
        ShellNotifyIcon(NIM_SETVERSION, ref data);
    }

    /// <summary>The tray icon, unless Windows refuses it.</summary>
    public static Tray? TryCreate(MainWindow window)
    {
        try
        {
            var tray = new Tray(window);
            return tray.added ? tray : null;
        }
        catch (Exception e)
        {
            Log.Write("tray: " + e.Message);
            return null;
        }
    }

    public void SetStatus(GuiState state)
    {
        var on = state.RealBlocks.Where(b => b.Active).Select(b => b.Name).ToList();
        var text = on.Count > 0 ? "On: " + string.Join(", ", on) : "No block is on";
        if (state.FreezeSeconds > 0) text = $"Frozen Chicken {Words.Seconds(state.FreezeSeconds)} · " + text;
        text = "Killer Chicken 9000\n" + text;
        if (text.Length > 127) text = text[..126] + "…";
        if (text == status) return;
        status = text;
        data.Tip = text;
        ShellNotifyIcon(NIM_MODIFY, ref data);
    }

    nint Proc(nint h, uint msg, nint wParam, nint lParam, nuint id, nuint refData)
    {
        if (msg == WM_APP_TRAY)
        {
            var e = (int)(lParam & 0xFFFF);
            if (e is WM_LBUTTONUP or NIN_SELECT or NIN_KEYSELECT) window.DispatcherQueue.TryEnqueue(window.ShowFromTray);
            else if (e is WM_RBUTTONUP or WM_CONTEXTMENU) Menu();
            return 0;
        }
        return DefSubclassProc(h, msg, wParam, lParam);
    }

    void Menu()
    {
        var menu = CreatePopupMenu();
        foreach (var line in status.Split('\n').Skip(1)) AppendMenu(menu, MF_STRING | MF_GRAYED, 0, line);
        AppendMenu(menu, MF_SEPARATOR, 0, null);
        AppendMenu(menu, MF_STRING, 1, "Open Killer Chicken 9000");
        AppendMenu(menu, MF_STRING, 2, "Quit (blocks stay on)");
        GetCursorPos(out var p);
        SetForegroundWindow(hwnd);
        var chosen = TrackPopupMenuEx(menu, TPM_RETURNCMD | TPM_RIGHTBUTTON, p.X, p.Y, hwnd, 0);
        DestroyMenu(menu);
        if (chosen == 1) window.DispatcherQueue.TryEnqueue(window.ShowFromTray);
        else if (chosen == 2) window.DispatcherQueue.TryEnqueue(window.Quit);
    }

    public void Dispose()
    {
        if (added) ShellNotifyIcon(NIM_DELETE, ref data);
        added = false;
        RemoveWindowSubclass(hwnd, proc, 9000);
    }
}
