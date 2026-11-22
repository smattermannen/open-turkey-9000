// W33 (R-GUI-3): every control a person can use has a name Narrator reads
// and takes keyboard focus, on every page and in an opened block's tabs;
// the pages in high contrast and with text at 200 % (screenshots for a
// person to look at). Narrator itself and display scaling: a person.
using System.Runtime.InteropServices;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using Microsoft.Win32;
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class AccessibilityTests
{
    static readonly string[] Pages = ["dashboard", "blocks", "schedules", "places", "hotkeys", "clusters", "stats", "circadian", "settings"];
    static readonly ControlType[] Usable =
        [ControlType.Button, ControlType.CheckBox, ControlType.ComboBox, ControlType.Edit, ControlType.Slider, ControlType.RadioButton,
         ControlType.SplitButton, ControlType.Hyperlink, ControlType.MenuItem, ControlType.TabItem, ControlType.ListItem];

    static List<string> Problems(GuiApp gui, string where)
    {
        var problems = new List<string>();
        foreach (var e in gui.Window.FindAllDescendants())
        {
            ControlType type;
            try { type = e.ControlType; } catch { continue; }
            if (!Usable.Contains(type)) continue;
            try
            {
                // A scroll bar's own arrows and thumb are WinUI's, not ours.
                if (e.Parent?.ControlType == ControlType.ScrollBar) continue;
                if (e.IsOffscreen || !e.IsEnabled) continue;
                var id = e.Properties.AutomationId.ValueOrDefault ?? "";
                if (string.IsNullOrWhiteSpace(e.Name)) problems.Add($"{where}: {type} {id} has no name");
                else if (!e.Properties.IsKeyboardFocusable.ValueOrDefault && type != ControlType.ListItem && type != ControlType.MenuItem)
                    problems.Add($"{where}: {type} \"{e.Name}\" {id} can't take keyboard focus");
            }
            catch (Exception) { } // an element gone while read
        }
        return problems;
    }

    [Fact]
    public void EveryControlHasANameAndTakesFocus()
    {
        using var gui = new GuiApp("names");
        var problems = new List<string>();
        foreach (var page in Pages)
        {
            gui.Page(page);
            Thread.Sleep(300);
            problems.AddRange(Problems(gui, page));
        }
        gui.Page("blocks");
        gui.Invoke("BlockCard_Reading");
        foreach (var tab in new[] { "websites", "apps", "breaks", "bypass" })
        {
            gui.Invoke("Tab_" + tab);
            Thread.Sleep(300);
            problems.AddRange(Problems(gui, "block " + tab));
        }
        foreach (var p in problems.Distinct()) Console.WriteLine(p);
        Assert.True(problems.Count == 0, $"{problems.Distinct().Count()} control(s) without a name or keyboard focus:\n" + string.Join("\n", problems.Distinct()));
    }

    [StructLayout(LayoutKind.Sequential)]
    struct HighContrast { public int Size; public uint Flags; public IntPtr Scheme; }

    [DllImport("user32.dll", SetLastError = true)]
    static extern bool SystemParametersInfo(uint action, uint param, ref HighContrast value, uint winIni);

    const uint GetHighContrast = 0x0042, SetHighContrastAction = 0x0043, HighContrastOn = 0x1;

    /// <summary>Turns high contrast on and gives back how to put the
    /// person's own setting back exactly (flags and scheme, not only on/off).</summary>
    static Action SetHighContrast()
    {
        var size = Marshal.SizeOf<HighContrast>();
        var before = new HighContrast { Size = size };
        SystemParametersInfo(GetHighContrast, (uint)size, ref before, 0);
        var flags = before.Flags;
        var scheme = before.Scheme != IntPtr.Zero ? Marshal.PtrToStringUni(before.Scheme) : null;
        void Put(uint f, string? s)
        {
            var text = s is null ? IntPtr.Zero : Marshal.StringToHGlobalUni(s);
            try
            {
                var hc = new HighContrast { Size = size, Flags = f, Scheme = text };
                SystemParametersInfo(SetHighContrastAction, (uint)size, ref hc, 0x3); // update and broadcast
            }
            finally { if (text != IntPtr.Zero) Marshal.FreeHGlobal(text); }
            Thread.Sleep(1500);
        }
        Put(flags | HighContrastOn, (flags & HighContrastOn) != 0 ? scheme : "High Contrast Black");
        return () => Put(flags, scheme);
    }

    [Fact]
    public void PagesInHighContrast()
    {
        var restore = SetHighContrast();
        try
        {
            using var gui = new GuiApp("contrast");
            foreach (var page in Pages)
            {
                gui.Page(page);
                gui.Shot(page);
            }
            gui.Page("blocks");
            gui.Invoke("BlockCard_Reading");
            gui.Shot("block");
        }
        finally { restore(); }
    }

    [Fact]
    public void PagesWithLargeText()
    {
        using var key = Registry.CurrentUser.CreateSubKey(@"Software\Microsoft\Accessibility");
        var before = key.GetValue("TextScaleFactor");
        key.SetValue("TextScaleFactor", 200, RegistryValueKind.DWord);
        try
        {
            using var gui = new GuiApp("text-200");
            foreach (var page in Pages)
            {
                gui.Page(page);
                gui.Shot(page);
            }
            gui.Page("blocks");
            gui.Invoke("BlockCard_Reading");
            gui.Find("Tab_websites");
            gui.Shot("block");
        }
        finally
        {
            if (before is null) key.DeleteValue("TextScaleFactor", false);
            else key.SetValue("TextScaleFactor", before, RegistryValueKind.DWord);
        }
    }
}
