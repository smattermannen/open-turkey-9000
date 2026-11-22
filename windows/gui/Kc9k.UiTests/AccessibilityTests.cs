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

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct HighContrast { public int Size; public uint Flags; public string? Scheme; }

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    static extern bool SystemParametersInfo(uint action, uint param, ref HighContrast value, uint winIni);

    static void SetHighContrast(bool on)
    {
        var hc = new HighContrast { Size = Marshal.SizeOf<HighContrast>(), Flags = on ? 1u : 0u, Scheme = on ? "High Contrast Black" : null };
        SystemParametersInfo(0x0043, (uint)hc.Size, ref hc, 0x3); // SPI_SETHIGHCONTRAST, update and broadcast
        Thread.Sleep(1500);
    }

    [Fact]
    public void PagesInHighContrast()
    {
        SetHighContrast(true);
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
        finally { SetHighContrast(false); }
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
