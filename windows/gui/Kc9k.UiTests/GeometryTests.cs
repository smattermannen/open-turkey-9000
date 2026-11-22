// W53/W54 (R-GUI-3): the layout at every window width and text size, by
// geometry (CI's screenshots can't be looked at): on every sidebar page and
// block tab, every element shown lies inside the window's client area, the
// page starts right of the pinned sidebar at the same place on every page,
// nothing scrolls sideways, and a block row's switch and its locks share
// the right column. Widths are in DIPs (400 is the laptop's 800 px at 200 %);
// a width the screen can't hold is tested at the widest it can.
using System.Runtime.InteropServices;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Input;
using Microsoft.Win32;
using Xunit;
using Xunit.Abstractions;

namespace Kc9k.UiTests;

[Collection("gui")]
public class GeometryTests(ITestOutputHelper output)
{
    static readonly int[] Widths = [400, 512, 800, 1024, 1366, 1920];
    static readonly string[] Pages = ["dashboard", "blocks", "schedules", "places", "hotkeys", "clusters", "stats", "circadian", "settings"];
    static readonly string[] Tabs = ["websites", "apps", "breaks", "bypass"];

    [StructLayout(LayoutKind.Sequential)]
    struct Rect { public int Left, Top, Right, Bottom; }

    [StructLayout(LayoutKind.Sequential)]
    struct Point { public int X, Y; }

    [DllImport("user32.dll")] static extern bool GetClientRect(IntPtr hwnd, out Rect r);
    [DllImport("user32.dll")] static extern bool ClientToScreen(IntPtr hwnd, ref Point p);
    [DllImport("user32.dll")] static extern uint GetDpiForWindow(IntPtr hwnd);
    [DllImport("user32.dll")] static extern bool SetWindowPos(IntPtr hwnd, IntPtr after, int x, int y, int w, int h, uint flags);
    [DllImport("user32.dll")] static extern bool ShowWindow(IntPtr hwnd, int cmd);

    sealed class Probe(GuiApp gui)
    {
        public IntPtr Handle => gui.Window.Properties.NativeWindowHandle.Value;
        public double Scale => GetDpiForWindow(Handle) / 96.0;

        public (int Left, int Top, int Right, int Bottom) Client()
        {
            GetClientRect(Handle, out var r);
            var p = new Point();
            ClientToScreen(Handle, ref p);
            return (p.X, p.Y, p.X + r.Right, p.Y + r.Bottom);
        }

        /// <summary>Resizes the window to a width in DIPs; gives the width it got.</summary>
        public int Resize(int dips)
        {
            ShowWindow(Handle, 9); // SW_RESTORE
            var px = (int)Math.Round(dips * Scale);
            var h = (int)Math.Round(700 * Scale);
            SetWindowPos(Handle, IntPtr.Zero, 0, 0, px, h, 0x0004 | 0x0010); // no z-order, no activate
            Thread.Sleep(800);
            var (l, _, r, _) = Client();
            return (int)Math.Round((r - l) / Scale);
        }
    }

    static bool Shown(AutomationElement e, out System.Drawing.Rectangle r)
    {
        r = default;
        try
        {
            if (e.IsOffscreen) return false;
            r = e.BoundingRectangle;
            return r.Width > 0 && r.Height > 0;
        }
        catch (Exception) { return false; }
    }

    /// <summary>What is wrong with the layout now, in words.</summary>
    static List<string> Problems(GuiApp gui, Probe probe, string where, int? pageLeft)
    {
        var problems = new List<string>();
        var (left, _, right, _) = probe.Client();
        var tol = (int)Math.Ceiling(2 * probe.Scale);
        foreach (var e in gui.Window.FindAllDescendants())
        {
            if (!Shown(e, out var r)) continue;
            ControlType type;
            string id, name;
            try { type = e.ControlType; id = e.Properties.AutomationId.ValueOrDefault ?? ""; name = e.Properties.Name.ValueOrDefault ?? ""; }
            catch (Exception) { continue; }
            if (r.Left < left - tol || r.Right > right + tol)
                problems.Add($"{where}: {type} {id} \"{Short(name)}\" at {r.Left}–{r.Right}, outside the client area {left}–{right}");
            if (type == ControlType.ScrollBar)
            {
                var horizontal = false;
                try { horizontal = e.Properties.Orientation.ValueOrDefault == OrientationType.Horizontal; } catch (Exception) { }
                if (horizontal) problems.Add($"{where}: a horizontal scroll bar {id}");
            }
        }
        // The page begins right of the sidebar, at one place on every page.
        var title = gui.ById("PageTitle");
        var nav = gui.ById("Nav_dashboard");
        if (title is not null && nav is not null && Shown(title, out var t) && Shown(nav, out var n))
        {
            if (t.Left < n.Right) problems.Add($"{where}: the page ({t.Left}) starts under the sidebar (to {n.Right})");
            if (pageLeft is int expected && Math.Abs(t.Left - expected) > tol)
                problems.Add($"{where}: the page starts at {t.Left}, other pages at {expected}");
        }
        else problems.Add($"{where}: no page title or sidebar found");
        // No ☰: the sidebar stays as it is.
        if (gui.ById("TogglePaneButton") is { } toggle && Shown(toggle, out _))
            problems.Add($"{where}: the sidebar can be folded (☰ shown)");
        return problems;
    }

    static string Short(string s) => s.Length > 40 ? s[..40] + "…" : s;

    static int? TitleLeft(GuiApp gui) => gui.ById("PageTitle") is { } t && Shown(t, out var r) ? r.Left : null;

    /// <summary>A block row: its switch and its locks at the right edge, in one
    /// column. Each row is scrolled into view first (large text puts the
    /// blocks below the modes, out of the window).</summary>
    static List<string> RowProblems(GuiApp gui, Probe probe, string where)
    {
        var problems = new List<string>();
        var tol = (int)Math.Ceiling(12 * probe.Scale);
        var rights = new List<int>();
        var names = new List<string>();
        foreach (var e in gui.Window.FindAllDescendants())
        {
            try
            {
                var id = e.Properties.AutomationId.ValueOrDefault ?? "";
                if (id.StartsWith("BlockSwitch_")) names.Add(id["BlockSwitch_".Length..]);
            }
            catch (Exception) { }
        }
        foreach (var name in names.Distinct())
        {
            var sw = gui.ById("BlockSwitch_" + name);
            if (sw is null) continue;
            try
            {
                if (sw.Patterns.ScrollItem.IsSupported) sw.Patterns.ScrollItem.Pattern.ScrollIntoView();
                else gui.ById("BlockCard_" + name)?.Focus();
            }
            catch (Exception) { }
            Thread.Sleep(300);
            sw = gui.ById("BlockSwitch_" + name);
            var locks = gui.ById("BlockLocks_" + name);
            if (sw is null || locks is null || !Shown(sw, out var s) || !Shown(locks, out var l))
            {
                problems.Add($"{where}: {name}'s switch or locks not shown after scrolling to it");
                continue;
            }
            if (Math.Abs(s.Right - l.Right) > tol)
                problems.Add($"{where}: {name}'s switch ends at {s.Right}, its locks at {l.Right}: not one right column");
            var title = gui.ById("BlockCard_" + name);
            if (title is not null && Shown(title, out var tr) && s.Left <= tr.Left)
                problems.Add($"{where}: {name}'s switch is not right of its name");
            rights.Add(s.Right);
        }
        if (names.Count == 0) problems.Add($"{where}: no block row with a switch");
        else if (rights.Count > 0 && rights.Max() - rights.Min() > tol) problems.Add($"{where}: the switches end at {string.Join(", ", rights)}: not one column");
        return problems;
    }

    /// <summary>Waits until the page stops changing (the same places for 300 ms).</summary>
    static void WaitIdle(GuiApp gui, params string[] ids)
    {
        string Sig() => string.Join(";", ids.Select(id =>
        {
            try { return gui.ById(id)?.BoundingRectangle.ToString() ?? "-"; } catch (Exception) { return "?"; }
        }));
        var last = Sig();
        var since = DateTime.Now;
        var end = DateTime.Now.AddSeconds(5);
        while (DateTime.Now < end)
        {
            Thread.Sleep(100);
            var now = Sig();
            if (now != last) { last = now; since = DateTime.Now; }
            else if ((DateTime.Now - since).TotalMilliseconds >= 300) return;
        }
    }

    static bool Retry(Func<bool> ok, int seconds = 8) =>
        FlaUI.Core.Tools.Retry.WhileFalse(() => { try { return ok(); } catch (Exception) { return false; } },
            TimeSpan.FromSeconds(seconds), TimeSpan.FromMilliseconds(200)).Result;

    static void SetTextScale(int percent)
    {
        using var key = Registry.CurrentUser.CreateSubKey(@"Software\Microsoft\Accessibility");
        if (percent == 100) key.DeleteValue("TextScaleFactor", false);
        else key.SetValue("TextScaleFactor", percent, RegistryValueKind.DWord);
    }

    [Theory]
    [InlineData(100)]
    [InlineData(150)]
    [InlineData(200)]
    public void EveryPageFitsEveryWidth(int textScale)
    {
        using var key = Registry.CurrentUser.CreateSubKey(@"Software\Microsoft\Accessibility");
        var before = key.GetValue("TextScaleFactor");
        SetTextScale(textScale);
        var problems = new List<string>();
        try
        {
            using var gui = new GuiApp("geometry-" + textScale);
            var probe = new Probe(gui);
            foreach (var asked in Widths)
            {
                var got = probe.Resize(asked);
                var at = $"{asked} DIP" + (Math.Abs(got - asked) > 8 ? $" (got {got})" : "") + $", text {textScale} %";
                output.WriteLine($"{at}: scale {probe.Scale}");
                int? pageLeft = null;
                foreach (var page in Pages)
                {
                    gui.Page(page);
                    Thread.Sleep(300);
                    pageLeft ??= TitleLeft(gui);
                    problems.AddRange(Problems(gui, probe, $"{at}, {page}", pageLeft));
                    if (page == "blocks") problems.AddRange(RowProblems(gui, probe, $"{at}, blocks"));
                }
                gui.Page("blocks");
                WaitIdle(gui, "BlockCard_Reading", "PageTitle");
                gui.Invoke(gui.Find("BlockCard_Reading")); // once: a card must take a press right after the page shows
                if (!Retry(() => gui.ById("Tab_websites") is not null))
                {
                    problems.Add($"{at}: the block's tabs are not shown (page title: {gui.ById("PageTitle")?.Name})");
                    gui.Shot($"{asked}-{textScale}-no-tabs");
                    gui.Page("blocks");
                    continue;
                }
                foreach (var tab in Tabs)
                {
                    gui.Invoke("Tab_" + tab);
                    Thread.Sleep(400);
                    problems.AddRange(Problems(gui, probe, $"{at}, block tab {tab}", pageLeft));
                }
                gui.Shot($"{asked}-{textScale}");
                gui.Page("blocks"); // back to the cards
                Wait.UntilInputIsProcessed();
            }
        }
        finally
        {
            if (before is null) key.DeleteValue("TextScaleFactor", false);
            else key.SetValue("TextScaleFactor", before, RegistryValueKind.DWord);
        }
        var distinct = problems.Distinct().ToList();
        foreach (var p in distinct) output.WriteLine(p);
        Assert.True(distinct.Count == 0, $"{distinct.Count} layout problem(s):\n" + string.Join("\n", distinct.Take(60)));
    }
}
