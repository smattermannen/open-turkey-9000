// The taskbar's and Start menu's jump list (W36): Overview, Blocks, New
// block…, Monk mode…, Frozen Chicken…, Statistics, Settings, each starting
// kc9k-gui.exe with its arguments (Core's StartArgs), which a running GUI
// receives. Unpackaged, so through the shell's ICustomDestinationList with
// the process's own AppUserModelID.
using System.Runtime.InteropServices;
using Kc9k.Core;

namespace Kc9k.Gui;

public static class JumpList
{
    public const string AppId = "KillerChicken9000.Gui";

    [DllImport("shell32.dll", CharSet = CharSet.Unicode, PreserveSig = false)]
    static extern void SetCurrentProcessExplicitAppUserModelID(string id);

    /// <summary>Before any window: the taskbar groups by this id.</summary>
    public static void SetAppId()
    {
        try { SetCurrentProcessExplicitAppUserModelID(AppId); }
        catch (Exception e) { Log.Write("app id: " + e.Message); }
    }

    [ComImport, Guid("6332DEBF-87B5-4670-90C0-5E57B408A49E"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface ICustomDestinationList
    {
        void SetAppID([MarshalAs(UnmanagedType.LPWStr)] string id);
        void BeginList(out uint maxSlots, ref Guid riid, [MarshalAs(UnmanagedType.Interface)] out object removed);
        void AppendCategory([MarshalAs(UnmanagedType.LPWStr)] string category, [MarshalAs(UnmanagedType.Interface)] IObjectArray items);
        void AppendKnownCategory(int category);
        void AddUserTasks([MarshalAs(UnmanagedType.Interface)] IObjectArray tasks);
        void CommitList();
        void GetRemovedDestinations(ref Guid riid, [MarshalAs(UnmanagedType.Interface)] out object removed);
        void DeleteList([MarshalAs(UnmanagedType.LPWStr)] string id);
        void AbortList();
    }

    [ComImport, Guid("92CA9DCD-5622-4BBA-A805-5E9F541BD8C9"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IObjectArray
    {
        void GetCount(out uint count);
        void GetAt(uint index, ref Guid riid, [MarshalAs(UnmanagedType.Interface)] out object item);
    }

    [ComImport, Guid("5632B1A4-E38A-400A-928A-D4CD63230295"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IObjectCollection
    {
        void GetCount(out uint count);
        void GetAt(uint index, ref Guid riid, [MarshalAs(UnmanagedType.Interface)] out object item);
        void AddObject([MarshalAs(UnmanagedType.Interface)] object item);
        void AddFromArray([MarshalAs(UnmanagedType.Interface)] IObjectArray source);
        void RemoveObjectAt(uint index);
        void Clear();
    }

    [ComImport, Guid("000214F9-0000-0000-C000-000000000046"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IShellLinkW
    {
        void GetPath(nint file, int max, nint data, uint flags);
        void GetIDList(out nint list);
        void SetIDList(nint list);
        void GetDescription(nint name, int max);
        void SetDescription([MarshalAs(UnmanagedType.LPWStr)] string name);
        void GetWorkingDirectory(nint dir, int max);
        void SetWorkingDirectory([MarshalAs(UnmanagedType.LPWStr)] string dir);
        void GetArguments(nint args, int max);
        void SetArguments([MarshalAs(UnmanagedType.LPWStr)] string args);
        void GetHotkey(out short key);
        void SetHotkey(short key);
        void GetShowCmd(out int cmd);
        void SetShowCmd(int cmd);
        void GetIconLocation(nint path, int max, out int index);
        void SetIconLocation([MarshalAs(UnmanagedType.LPWStr)] string path, int index);
        void SetRelativePath([MarshalAs(UnmanagedType.LPWStr)] string path, uint reserved);
        void Resolve(nint hwnd, uint flags);
        void SetPath([MarshalAs(UnmanagedType.LPWStr)] string file);
    }

    [StructLayout(LayoutKind.Sequential, Pack = 4)]
    struct PropertyKey { public Guid Fmtid; public uint Pid; }

    [StructLayout(LayoutKind.Explicit)]
    struct PropVariant
    {
        [FieldOffset(0)] public ushort Type;
        [FieldOffset(8)] public nint Pointer;
    }

    [ComImport, Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    interface IPropertyStore
    {
        void GetCount(out uint count);
        void GetAt(uint index, out PropertyKey key);
        void GetValue(ref PropertyKey key, out PropVariant value);
        void SetValue(ref PropertyKey key, ref PropVariant value);
        void Commit();
    }

    static readonly Guid DestinationList = new("77F10CF0-3DB5-4966-B520-B7C54FD35ED6");
    static readonly Guid ObjectCollection = new("2D3468C1-36A7-43B6-AC24-D3F02FD9607A");
    static readonly Guid ShellLink = new("00021401-0000-0000-C000-000000000046");

    static T Create<T>(Guid clsid) => (T)Activator.CreateInstance(Type.GetTypeFromCLSID(clsid, true)!)!;

    /// <summary>Writes the jump list; a shell that refuses leaves none (logged).</summary>
    public static void Write()
    {
        try
        {
            var exe = Environment.ProcessPath ?? Path.Combine(AppContext.BaseDirectory, "kc9k-gui.exe");
            var list = Create<ICustomDestinationList>(DestinationList);
            list.SetAppID(AppId);
            var iid = typeof(IObjectArray).GUID;
            list.BeginList(out _, ref iid, out _);
            var tasks = Create<IObjectCollection>(ObjectCollection);
            foreach (var (title, args) in StartArgs.JumpList)
            {
                var link = Create<IShellLinkW>(ShellLink);
                link.SetPath(exe);
                link.SetArguments(args);
                link.SetIconLocation(exe, 0);
                link.SetDescription(title);
                var store = (IPropertyStore)link;
                var key = new PropertyKey { Fmtid = new Guid("F29F85E0-4FF9-1068-AB91-08002B27B3D9"), Pid = 2 }; // PKEY_Title
                var value = new PropVariant { Type = 31, Pointer = Marshal.StringToCoTaskMemUni(title) }; // VT_LPWSTR
                try
                {
                    store.SetValue(ref key, ref value);
                    store.Commit();
                }
                finally { Marshal.FreeCoTaskMem(value.Pointer); }
                tasks.AddObject(link);
            }
            list.AddUserTasks((IObjectArray)tasks);
            list.CommitList();
        }
        catch (Exception e) { Log.Write("jump list: " + e.Message); }
    }
}
