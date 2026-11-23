# W4/W54 in CI: sets the runner's display to a resolution and a scale
# (Settings › Display › Scale), live, as a person would on a laptop at 200 %,
# so the UI tests run against real DPI, not only text size. Prints what the
# display reached and exits 1 when the scale asked for can't be reached.
#   pwsh display-scale.ps1 -Percent 150 [-Width 1920 -Height 1080]
param([int]$Percent = 100, [int]$Width = 0, [int]$Height = 0)
$ErrorActionPreference = "Stop"
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class DisplayScale
{
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct DEVMODE
    {
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmDeviceName;
        public short dmSpecVersion, dmDriverVersion, dmSize, dmDriverExtra;
        public int dmFields, dmPositionX, dmPositionY, dmDisplayOrientation, dmDisplayFixedOutput;
        public short dmColor, dmDuplex, dmYResolution, dmTTOption, dmCollate;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string dmFormName;
        public short dmLogPixels;
        public int dmBitsPerPel, dmPelsWidth, dmPelsHeight, dmDisplayFlags, dmDisplayFrequency;
        public int dmICMMethod, dmICMIntent, dmMediaType, dmDitherType, dmReserved1, dmReserved2, dmPanningWidth, dmPanningHeight;
    }
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern bool EnumDisplaySettings(string device, int mode, ref DEVMODE dm);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int ChangeDisplaySettingsEx(string device, ref DEVMODE dm, IntPtr hwnd, int flags, IntPtr param);

    /// <summary>The largest mode the display offers, or the one asked for.</summary>
    public static string Resolution(int width, int height)
    {
        var best = new DEVMODE { dmSize = (short)Marshal.SizeOf<DEVMODE>() };
        var dm = new DEVMODE { dmSize = (short)Marshal.SizeOf<DEVMODE>() };
        var found = false;
        for (var i = 0; EnumDisplaySettings(null, i, ref dm); i++)
        {
            var fits = width > 0 ? dm.dmPelsWidth == width && dm.dmPelsHeight == height : (long)dm.dmPelsWidth * dm.dmPelsHeight > (long)best.dmPelsWidth * best.dmPelsHeight;
            if (fits && dm.dmBitsPerPel >= 24) { best = dm; found = true; }
        }
        if (!found) return "no such mode";
        best.dmFields = 0x80000 | 0x100000; // DM_PELSWIDTH | DM_PELSHEIGHT
        var r = ChangeDisplaySettingsEx(null, ref best, IntPtr.Zero, 0x1, IntPtr.Zero); // CDS_UPDATEREGISTRY
        return best.dmPelsWidth + "x" + best.dmPelsHeight + " (" + r + ")";
    }

    [StructLayout(LayoutKind.Sequential)] public struct LUID { public uint Low; public int High; }
    [StructLayout(LayoutKind.Sequential)] struct PathSource { public LUID adapterId; public uint id; public uint modeInfoIdx; public uint statusFlags; }
    [StructLayout(LayoutKind.Sequential)] struct PathTarget
    {
        public LUID adapterId; public uint id; public uint modeInfoIdx; public int outputTechnology; public int rotation; public int scaling;
        public uint refreshNum, refreshDen; public int scanLineOrdering; public bool targetAvailable; public uint statusFlags;
    }
    [StructLayout(LayoutKind.Sequential)] struct PathInfo { public PathSource source; public PathTarget target; public uint flags; }
    [StructLayout(LayoutKind.Sequential, Size = 64)] struct ModeInfo { public int infoType; }
    [StructLayout(LayoutKind.Sequential)] struct Header { public int type; public int size; public LUID adapterId; public uint id; }
    [StructLayout(LayoutKind.Sequential)] struct GetScale { public Header header; public int minimum, current, maximum; }
    [StructLayout(LayoutKind.Sequential)] struct SetScale { public Header header; public int scale; }
    [DllImport("user32.dll")] static extern int GetDisplayConfigBufferSizes(uint flags, out uint paths, out uint modes);
    [DllImport("user32.dll")] static extern int QueryDisplayConfig(uint flags, ref uint paths, [Out] PathInfo[] p, ref uint modes, [Out] ModeInfo[] m, IntPtr topology);
    [DllImport("user32.dll")] static extern int DisplayConfigGetDeviceInfo(ref GetScale info);
    [DllImport("user32.dll")] static extern int DisplayConfigSetDeviceInfo(ref SetScale info);

    // The steps Settings offers, relative to the recommended one.
    static readonly int[] Steps = { 100, 125, 150, 175, 200, 225, 250, 300, 350, 400, 450, 500 };

    /// <summary>Sets the first display's scale; returns "reached percent (min..max)".</summary>
    public static string Scale(int percent)
    {
        uint np, nm;
        if (GetDisplayConfigBufferSizes(2, out np, out nm) != 0) return "no display config"; // QDC_ONLY_ACTIVE_PATHS
        var paths = new PathInfo[np];
        var modes = new ModeInfo[nm];
        if (QueryDisplayConfig(2, ref np, paths, ref nm, modes, IntPtr.Zero) != 0 || np == 0) return "no display path";
        var src = paths[0].source;
        var get = new GetScale { header = new Header { type = -3, size = Marshal.SizeOf<GetScale>(), adapterId = src.adapterId, id = src.id } };
        if (DisplayConfigGetDeviceInfo(ref get) != 0) return "can't read the scale";
        var recommended = Math.Abs(get.minimum);
        var want = Array.IndexOf(Steps, percent);
        if (want < 0) return "not a scale step";
        var relative = Math.Max(get.minimum, Math.Min(get.maximum, want - recommended));
        var set = new SetScale { header = new Header { type = -4, size = Marshal.SizeOf<SetScale>(), adapterId = src.adapterId, id = src.id }, scale = relative };
        var r = DisplayConfigSetDeviceInfo(ref set);
        DisplayConfigGetDeviceInfo(ref get);
        var reached = Steps[Math.Max(0, Math.Min(Steps.Length - 1, recommended + get.current))];
        return reached + " (" + Steps[recommended + get.minimum] + ".." + Steps[Math.Min(Steps.Length - 1, recommended + get.maximum)] + ", set " + r + ")";
    }
}
"@
if ($Percent -gt 100 -or $Width -gt 0) { Write-Host "resolution: $([DisplayScale]::Resolution($Width, $Height))" }
Start-Sleep -Seconds 2
$result = [DisplayScale]::Scale($Percent)
Write-Host "scale: $result"
Start-Sleep -Seconds 3
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Screen]::AllScreens | ForEach-Object { Write-Host "screen: $($_.Bounds)" }
if (-not $result.StartsWith("$Percent ")) { Write-Host "the display can't be set to $Percent %"; exit 1 }
