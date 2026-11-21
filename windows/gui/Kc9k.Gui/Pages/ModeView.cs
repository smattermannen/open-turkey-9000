// §4.2 a mode's view (owner 2026-11-17; gui/kc9k-gui.py build_mode_view):
// its blocks as cards with Add blocks… and Take out…, its switch, Lock…
// (R-MODE-3: every block locked at once), what it opens or runs when it
// becomes enabled (§3.9), its automation. Monk mode: what it is and Run now,
// locked for a time; Frozen Chicken: its freeze view (Device).
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace Kc9k.Gui;

public sealed class ModeView
{
    readonly MainWindow window;
    readonly BlocksPage page;
    readonly TextBlock title = Ui.Text("", "Kc9kPageTitle", wrap: false);
    readonly TextBlock status = Ui.Dim("");
    readonly ToggleSwitch toggle;
    readonly Button lockButton, menuButton, takeOutButton;
    readonly StackPanel blockCards = Ui.Column(10);
    readonly UIElement blocksSection, monkBox, customSection, automationSection, freezeSection, wallpaperSection;
    readonly WallpaperEditor wallpapers;
    readonly ToggleSwitch overrideCircadian;
    readonly TextBlock freezeLocked = Ui.Dim("Frozen Chicken is locked by an access password: open it under Settings or with its password.");
    readonly CustomEditor custom;
    readonly AutomationEditor modeEditor, monkEditor;
    readonly Device device;
    string name = "";
    string blocksShown = "";
    bool filling;
    Controller C => window.Controller;
    public UIElement View { get; }

    public ModeView(MainWindow window, BlocksPage page)
    {
        this.window = window;
        this.page = page;
        toggle = Ui.Switch("Mode on", false, on => { if (!filling && C.State?.FindMode(name) is { } m) _ = page.ToggleMode(m, on); }, "ModeViewSwitch");
        lockButton = Ui.Button("Lock…", () => _ = LockMode(), "ModeLock", glyph: Glyphs.Lock)
            .Tip("Lock every block of the mode: it then can't be switched off or weakened until they are unlocked");
        var menu = new MenuFlyout();
        void Item(string text, Func<Task> act)
        {
            var i = new MenuFlyoutItem { Text = text };
            i.Click += (_, _) => _ = act();
            menu.Items.Add(i);
        }
        Item("Rename…", Rename);
        Item("Delete…", Delete);
        Item("Save automation as preset…", () => AutomationView.SavePreset(window, "automation", name, true));
        Item("Use an automation preset…", () => AutomationView.UsePreset(window, name, true));
        menuButton = new Button { Content = new FontIcon { Glyph = Glyphs.More, FontSize = 14 }, Flyout = menu }.Named("More: rename, delete, presets", "ModeMore");
        var head = new Grid { ColumnSpacing = 10 };
        head.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        head.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        head.Children.Add(title);
        var right = Ui.Row(8, toggle, lockButton, menuButton);
        Grid.SetColumn(right, 1);
        head.Children.Add(right);

        takeOutButton = Ui.Button("Take out…", () => _ = TakeOut(), "ModeTakeOut").Tip("Take a block out of the mode");
        blocksSection = Ui.Column(10, Ui.Row(6, Ui.Heading("Its blocks"), Ui.HelpButton(
                "They share the mode's automation below: changing it changes all of them, and a change that weakens one of them while it is locked is refused for all. Each keeps its own lists, apps, breaks and locks. Take out… takes one out again.", "its blocks")),
            blockCards, Ui.Row(6, Ui.Button("Add blocks…", () => _ = AddBlocks(), "ModeAddBlocks", glyph: Glyphs.Add), takeOutButton));
        monkBox = Ui.Card(Ui.Column(10,
            Ui.Dim("Every website is blocked and the internet connection is cut. Switch it on like any mode: with windows or places below it runs in them by itself, without them until you switch it off. Started for a time it is locked: it can't be switched off until then. While its places decide, Wi-Fi stays on to see where you are; the firewall still cuts it."),
            Ui.Button("Run now, locked for a time…", () => _ = Actions.MonkMode(window), "MonkRunNow")));
        device = new Device(window);
        freezeSection = Ui.Column(10, freezeLocked, device.View);
        custom = new CustomEditor(window, id: "ModeCustom");
        custom.Changed += items => _ = SetCustom(items);
        customSection = Ui.Section("When it becomes enabled", Ui.Dim("Open sites, files and folders, or run programs, each time the mode becomes enabled (also once at sign-in, when it is enabled then)."), custom);
        // R-MODE-5: the mode's wallpapers, laid out as circadian rhythm's.
        wallpapers = new WallpaperEditor(window, () => ["mode", "wallpaper", name], "Mode");
        overrideCircadian = Ui.Switch("Override circadian rhythm wallpapers", false, on => { if (!filling) _ = C.Run(["wallpaper", "priority", on ? "modes" : "circadian"]); }, "OverrideCircadian");
        wallpaperSection = Ui.Section("Wallpapers",
            "Drag a bar to move it, or an end to change when it starts or stops (Shift: to the minute). Of several enabled modes, the one enabled last shows its wallpaper. Override circadian rhythm wallpapers is one setting for all modes, the same as Override modes on the circadian rhythm page the other way round. A mode switched by places shows its wallpaper once Killer Chicken knows where you are; until then the wallpaper default in Settings shows.",
            Ui.Dim("While the mode is enabled, each picture is the wallpaper between its two times."), wallpapers,
            Ui.SettingRow("Override circadian rhythm wallpapers", "", overrideCircadian));
        modeEditor = new AutomationEditor(window, "mode");
        monkEditor = new AutomationEditor(window, "monk");
        automationSection = Ui.Section("Automation", "Its windows (its own, and those of schedules applied to it) and its places. Disabled by default: they turn its blocks on, and they are lifted otherwise; enabled by default: they turn them off. With windows and places, either or both at once. Monk mode is always disabled by default.",
            modeEditor, monkEditor);
        View = Ui.Page(Ui.Column(18, Ui.Link("← All blocks", () => page.ShowCards(), "ModeBack"), head, status,
            blocksSection, monkBox, freezeSection, customSection, wallpaperSection, automationSection));
    }

    public void Open(string mode)
    {
        name = mode;
        blocksShown = "";
        title.Text = mode;
    }

    static void Shown(UIElement e, bool on) => e.Visibility = on ? Visibility.Visible : Visibility.Collapsed;

    public bool Update(GuiState s)
    {
        var m = s.FindMode(name);
        if (m is null) return false;
        title.Text = m.Name;
        var (on, enabled, tip, _) = BlocksPage.ModeSwitchState(m);
        filling = true;
        Ui.Set(toggle, on);
        filling = false;
        toggle.IsEnabled = enabled;
        ToolTipService.SetToolTip(toggle, tip);
        Shown(freezeSection, m.Freeze);
        Shown(automationSection, !m.Freeze);
        Shown(customSection, !m.Freeze);
        Shown(wallpaperSection, !m.Freeze);
        filling = true;
        Ui.Set(overrideCircadian, !s.Wallpaper.OverrideModes);
        filling = false;
        wallpapers.Fill(m.Wallpapers);
        Shown(blocksSection, !m.Monk && !m.Freeze);
        Shown(monkBox, m.Monk);
        Shown(menuButton, !m.Monk && !m.Freeze);
        Shown(lockButton, !m.Monk && !m.Freeze);
        if (m.Freeze)
        {
            var locked = C.Locked("device");
            Shown(freezeLocked, locked);
            Shown(device.View, !locked);
            if (!locked) device.Update(s);
            status.Text = m.FreezeLeft > 0 ? $"Frozen for {Words.Seconds(m.FreezeLeft)} more; it can't end early, only be extended."
                : "Off. Switching it on asks until when (at most a week).";
            return true;
        }
        if (!custom.Items.SequenceEqual(m.Custom)) custom.SetItems(m.Custom);
        if (m.Monk)
        {
            status.Text = (m.MonkLeft > 0 ? $"Locked for {Words.Seconds(m.MonkLeft)} more (started for a time). " : "") +
                          (m.Active && m.Continuous ? "Switched on: it runs until you switch it off."
                              : m.Active ? "Switched on: its windows and places run it." : "Switched off.");
        }
        else
        {
            lockButton.IsEnabled = m.Blocks.Count > 0;
            takeOutButton.IsEnabled = m.Blocks.Count > 0;
            status.Text = (m.RunLeft > 0 ? $"Run by a Mode action: locked for {Words.Seconds(m.RunLeft)} more. " : "") +
                          (m.Blocks.Count > 0 ? $"{m.On} of {Words.Plural(m.Blocks.Count, "block")} on" + (m.Locked > 0 ? $", {m.Locked} locked" : "") + "."
                              : "No blocks yet: drag blocks onto its card, or add them here.");
            var shown = m.Name + string.Join("|", m.Blocks.Select(b => s.FindBlock(b) is { } x ? $"{x.Name}{x.Active}{x.Held}{x.Enabled}{Words.Breaks(x)}{Words.LockState(x)}{x.Draft}" : b));
            if (shown != blocksShown)
            {
                blocksShown = shown;
                blockCards.Children.Clear();
                foreach (var b in m.Blocks)
                    if (s.FindBlock(b) is { } block) blockCards.Children.Add(page.BlockCard(block));
            }
        }
        Shown(modeEditor, !m.Monk);
        Shown(monkEditor, m.Monk);
        var editor = m.Monk ? monkEditor : modeEditor;
        editor.Show(m.Name, m.Windows, m.Automation, m.Continuous, m.Schedules, m.Places, m.Active, m.Monk ? "monk mode" : "its blocks");
        return true;
    }

    /// <summary>What the mode opens or runs: the list as it is now (clear, then each added).</summary>
    async Task SetCustom(List<string> items)
    {
        if (!await C.Run(["mode", "custom", name, "clear"])) return;
        foreach (var item in items)
            if (!await C.Run(["mode", "custom", name, "add", item])) return;
    }

    /// <summary>R-MODE-3: locks every block of the mode with the chosen locks, confirmed once.</summary>
    async Task LockMode()
    {
        var m = C.State?.FindMode(name);
        if (m is null || m.Blocks.Count == 0 || m.Monk) return;
        var picker = new LockPicker("switch the mode off", "switched off", LockSpec.AllKinds, relockAny: true, id: "ModeLocks");
        var asked = await LockDialogs.Ask(window, $"Lock {m.Name}",
            $"Every block in it ({string.Join(", ", m.Blocks)}) gets these locks, after the ones it has. While they are on, the mode can't be switched off or weakened.",
            "Lock", picker, needLock: true);
        if (asked is not var (chosen, relock, secret) || chosen.Count == 0) return;
        if (chosen.Count > 1)
        {
            var ordered = await LockDialogs.ChooseOrder(window, chosen);
            if (ordered is null) return;
            chosen = ordered;
        }
        var (flags, lines) = LockDialogs.Lines($"mode '{m.Name}' (each of its blocks)", chosen, relock);
        var (ok, _) = await C.Dialogs.ConfirmLock(lines);
        if (ok) await C.Run(["mode", "lock", m.Name, .. flags, "--yes"], secret.Length > 0 ? secret + "\n" : null);
    }

    /// <summary>Ticks blocks to put into the mode, or makes a new one there (§4.2).</summary>
    async Task AddBlocks()
    {
        var s = C.State;
        if (s is null) return;
        var names = s.RealBlocks.Where(b => b.Mode != name).ToList();
        var checks = names.Select(b => new CheckBox { Content = b.Name + (b.InMode ? $" (now in {b.Mode})" : "") }.Named(b.Name, "AddToMode_" + b.Name)).ToList();
        var body = Ui.Column(6, Ui.Dim(names.Count > 0 ? "They get the mode's automation; their own windows and places give way to it." : "Every block is in this mode already."));
        foreach (var c in checks) body.Children.Add(c);
        var d = new ContentDialog
        {
            Title = $"Add blocks to {name}", Content = new ScrollViewer { Content = body, MaxHeight = 420 },
            PrimaryButtonText = names.Count > 0 ? "Add" : "", SecondaryButtonText = "Create new block…", CloseButtonText = "Cancel",
            PrimaryButtonStyle = Theme.Style("AccentButtonStyle"),
        };
        var answer = await C.Dialogs.Show(d);
        if (answer == ContentDialogResult.Secondary)
        {
            var created = await C.Dialogs.AskText($"New block in {name}", "A short name, such as Social media. It gets the mode's automation.",
                problem: t => t.Length == 0 ? "Type a name." : s.FindBlock(t) is not null ? "A block has that name already." : null, ok: "Create");
            if (!string.IsNullOrEmpty(created) && await C.Run(["block", "create", created])) await C.Run(["mode", "add", name, created]);
            return;
        }
        var chosen = names.Zip(checks).Where(x => x.Second.IsChecked == true).Select(x => x.First.Name).ToList();
        if (answer == ContentDialogResult.Primary && chosen.Count > 0) await C.Run(["mode", "add", name, .. chosen]);
    }

    /// <summary>The keyboard's way to take a block out (the cards have no menu button).</summary>
    async Task TakeOut()
    {
        var m = C.State?.FindMode(name);
        if (m is null || m.Blocks.Count == 0) return;
        var chosen = await C.Dialogs.Choose("Take a block out", $"It leaves {m.Name} and keeps the automation it has now.", [.. m.Blocks.Select(b => (b, b))]);
        if (chosen is not null) await C.Run(["mode", "take-out", chosen]);
    }

    async Task Rename()
    {
        var renamed = await C.Dialogs.AskText("Rename the mode", initial: name);
        if (!string.IsNullOrEmpty(renamed) && renamed != name && await C.Run(["mode", "rename", name, renamed])) Open(renamed);
    }

    async Task Delete()
    {
        if (await C.Dialogs.Confirm($"Delete the mode {name}?", "Its blocks stay, with the automation they have now.", "Delete", destructive: true))
        {
            page.ShowCards();
            await C.Run(["mode", "delete", name]);
        }
    }
}
