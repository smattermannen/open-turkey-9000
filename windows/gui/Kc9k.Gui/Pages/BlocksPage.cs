// §4.2 Blocks (owner 2026-11-16, 2026-11-17): the page opens on the list,
// one card per block, modes above them; a card opens its block (BlockView),
// a mode card its mode (ModeView), "Automated/Continuous block" the block's
// automation (AutomationView). Drag a block card onto a mode to put it in;
// from the keyboard, the card's context menu (Shift+F10) or the mode's Add
// blocks… do the same (R-GUI-3).
using Kc9k.Core;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Windows.ApplicationModel.DataTransfer;

namespace Kc9k.Gui;

public sealed class BlocksPage : IPage
{
    readonly MainWindow window;
    readonly Grid host = new();
    readonly UIElement cardsView;
    readonly StackPanel modeCards = Ui.Column(10), blockCards = Ui.Column(10);
    string cardsShown = "";
    string view = "cards";
    GuiState? state;

    public BlockView Block { get; }
    public ModeView Mode { get; }
    public AutomationView Automation { get; }
    public string Area => "blocks";
    public UIElement View => host;
    Controller C => window.Controller;

    /// <summary>The block opened, its tabs showing (for drafts, §4.3).</summary>
    public string? OpenedBlock => view == "detail" ? Block.Name : null;
    public string CurrentView => view;

    public BlocksPage(MainWindow window)
    {
        this.window = window;
        var modesHelp = "A mode is a group of blocks with one automation for all of them: one schedule and places switch them together. Drag a block onto a mode to put it in (it then has the mode's automation), or open the mode and use Add blocks…. Monk mode is a mode too: give it windows or places and switch it on, and it runs then by itself.";
        cardsView = Ui.Page(Ui.Column(20,
            Ui.PageHeader("Blocks", "What each block blocks, what switches it, its breaks and its locks. Drag a block onto a mode to automate it with the mode's blocks."),
            Section("Modes", modesHelp, modeCards),
            Section("Blocks", null, blockCards),
            Ui.Wrap(8,
                Ui.Button("New block…", () => _ = Actions.CreateBlock(window), "NewBlock", accent: true, glyph: Glyphs.Add),
                Ui.Button("New mode…", () => _ = CreateMode(), "NewMode"))));
        Block = new BlockView(window, this);
        Mode = new ModeView(window, this);
        Automation = new AutomationView(window, this);
        foreach (var v in new[] { cardsView, Block.View, Mode.View, Automation.View }) host.Children.Add(v);
        Show("cards");
    }

    static StackPanel Section(string title, string? help, UIElement content)
    {
        var head = Ui.Wrap(6, Ui.Heading(title));
        if (help is not null) head.Children.Add(Ui.HelpButton(help, title));
        return Ui.Column(10, head, content);
    }

    void Show(string which)
    {
        view = which;
        cardsView.Visibility = which == "cards" ? Visibility.Visible : Visibility.Collapsed;
        Block.View.Visibility = which == "detail" ? Visibility.Visible : Visibility.Collapsed;
        Mode.View.Visibility = which == "mode" ? Visibility.Visible : Visibility.Collapsed;
        Automation.View.Visibility = which == "automation" ? Visibility.Visible : Visibility.Collapsed;
    }

    public void ShowCards()
    {
        if (view == "detail")
        {
            _ = LeaveBlock();
            return;
        }
        Show("cards");
    }

    public void BackToBlock() => Show("detail");

    public void Update(GuiState s)
    {
        state = s;
        UpdateCards(s);
        if (view == "detail" && !Block.Update(s)) Show("cards");
        if (view == "mode" && !Mode.Update(s)) Show("cards");
        if (view == "automation" && !Automation.Update(s)) Show(Automation.ForMode ? "cards" : "detail");
    }

    /// <summary>One card per block (drafts stay with their block, blocks in a
    /// mode are inside it), modes above them.</summary>
    // Each card is kept while what it shows is the same (W54): a state read
    // (every 5 s, and when the page is shown) never replaces a card under a
    // click; only a card whose block or mode changed is built again.
    readonly Dictionary<string, (string Key, UIElement Card)> cardCache = [];
    UIElement? noBlocks, allInModes;

    void ForceCards()
    {
        cardsShown = "";
        cardCache.Clear();
    }

    UIElement Cached(string name, string key, Func<UIElement> make)
    {
        if (cardCache.TryGetValue(name, out var c) && c.Key == key) return c.Card;
        var card = make();
        cardCache[name] = (key, card);
        return card;
    }

    static void Sync(Panel panel, List<UIElement> wanted)
    {
        if (panel.Children.Count == wanted.Count && panel.Children.Zip(wanted).All(p => ReferenceEquals(p.First, p.Second))) return;
        panel.Children.Clear();
        foreach (var e in wanted) panel.Children.Add(e);
    }

    void UpdateCards(GuiState s)
    {
        var blocks = s.RealBlocks.ToList();
        var modeNames = string.Join(",", s.Modes.Select(m => $"{m.Name}:{m.Monk}:{m.Freeze}"));
        string BlockKey(Block b) => $"{b.Name}|{b.Active}|{b.Held}|{b.Continuous}|{Words.Breaks(b)}|{Words.LockState(b)}|{b.Enabled}|{b.Mode}|{b.Draft}|{modeNames}";
        string ModeKey(Mode m) => System.Text.Json.JsonSerializer.Serialize(m, Kc9kJson.Options);
        var shown = string.Join("\n", blocks.Select(BlockKey)) + string.Join("\n", s.Modes.Select(ModeKey));
        if (shown == cardsShown) return;
        cardsShown = shown;
        Sync(modeCards, [.. s.Modes.Select(m => Cached("mode:" + m.Name, ModeKey(m), () => ModeCard(m)))]);
        var loose = blocks.Where(b => !b.InMode).ToList();
        List<UIElement> rows = [.. loose.Select(b => Cached("block:" + b.Name, BlockKey(b), () => BlockCard(b)))];
        if (blocks.Count == 0) rows.Add(noBlocks ??= Ui.Dim("No blocks yet. A block holds what to block, when, and how it holds."));
        else if (loose.Count == 0) rows.Add(allInModes ??= Ui.Dim("Every block is in a mode."));
        Sync(blockCards, rows);
        foreach (var gone in cardCache.Keys.Where(k => !blocks.Any(b => "block:" + b.Name == k) && !s.Modes.Any(m => "mode:" + m.Name == k)).ToList())
            cardCache.Remove(gone);
    }

    /// <summary>A card's switch: a refusal (shown in the CLI's words) leaves
    /// the state as it was, so the cards are drawn again to put the switch back.</summary>
    async Task SwitchBlock(string name, bool on)
    {
        if (await C.Run(on ? ["start", name] : ["stop", name])) return;
        ForceCards();
        if (state is not null) UpdateCards(state);
    }

    /// <summary>A block's card: its name (opens it), switch, Continuous or
    /// Automated, its breaks, its locks; nothing else (§4.2).</summary>
    public UIElement BlockCard(Block b)
    {
        var name = b.Name;
        var title = new HyperlinkButton { Content = Ui.Text(name, "SubtitleTextBlockStyle", wrap: false), Padding = new Thickness(0) }
            .Named($"Open the block {name}", "BlockCard_" + name).Tip("Open the block");
        title.Click += (_, _) => OpenBlock(name);
        var toggle = Ui.Switch($"{name} on", b.Active, on => _ = SwitchBlock(name, on), "BlockSwitch_" + name);
        toggle.IsEnabled = !(b.Active && b.Held);
        ToolTipService.SetToolTip(toggle, b.Held ? "Locked: it can't be switched off" : b.Active ? "Switch the block off" : "Switch the block on (without locks)");
        var stateText = Ui.Dim(b.Active ? (b.Enabled ? "On" : "On, lifted now") : "Off", wrap: false);
        stateText.VerticalAlignment = VerticalAlignment.Center;
        // W54: a row as in the GTK GUI: name and what switches it, its breaks
        // on the left; the switch and its locks in the page's right column.
        var left = Ui.Wrap(10, title);
        if (b.Draft.Length > 0) left.Children.Add(Ui.Pill(new("Unsaved edits", PillKind.Warn)).Tip("Open the block to save or discard them"));
        var automated = !b.Continuous;
        var links = Ui.Wrap(16,
            CardLink(automated ? "Automated block" : "Continuous block", automated ? Glyphs.Clock : Glyphs.Sync,
                b.InMode ? $"Automated by '{b.Mode}'" : "What switches it by itself: schedule and places", () => OpenAutomation(name), "BlockAuto_" + name),
            CardLink(Words.Breaks(b), Glyphs.Coffee, "Its breaks", () => OpenBlock(name, "breaks"), "BlockBreaks_" + name));
        var locks = CardLink(Words.LockState(b), b.Held ? Glyphs.Lock : Glyphs.Unlock, b.Held ? "Unlock it" : "Add locks", () => _ = CardLocks(name), "BlockLocks_" + name);
        var body = Ui.Side(Ui.Column(6, left, links), RightColumn(Ui.Row(10, stateText, toggle), locks));
        var card = Ui.Card(body);
        if (!b.Active) card.Opacity = 0.85;

        // §4.2: dragged onto a mode it goes in.
        card.CanDrag = true;
        card.DragStarting += (_, e) =>
        {
            e.Data.SetText("kc9k-block:" + name);
            e.Data.RequestedOperation = DataPackageOperation.Move;
        };
        // The keyboard's way (R-GUI-3): Shift+F10 or the context key.
        var menu = new MenuFlyout();
        foreach (var m in (state?.Modes ?? []).Where(m => !m.Monk && !m.Freeze && m.Name != b.Mode))
        {
            var item = new MenuFlyoutItem { Text = $"Put into {m.Name}" };
            item.Click += (_, _) => _ = MoveToMode(name, m.Name);
            menu.Items.Add(item);
        }
        if (b.InMode)
        {
            var take = new MenuFlyoutItem { Text = $"Take out of {b.Mode}" };
            take.Click += (_, _) => _ = C.Run(["mode", "take-out", name]);
            menu.Items.Add(take);
        }
        var newMode = new MenuFlyoutItem { Text = "Into a new mode…" };
        newMode.Click += (_, _) => _ = CreateMode(name);
        menu.Items.Add(newMode);
        card.ContextFlyout = menu;
        return card;
    }

    /// <summary>The right column of a card (W54): each line at the right edge.</summary>
    static StackPanel RightColumn(params FrameworkElement[] lines)
    {
        foreach (var l in lines) l.HorizontalAlignment = HorizontalAlignment.Right;
        var p = Ui.Column(6, lines);
        p.Id("RightColumn");
        return p;
    }

    static HyperlinkButton CardLink(string text, string glyph, string tip, Action click, string id)
    {
        var b = new HyperlinkButton { Content = Ui.Row(6, Ui.Icon(glyph, 14), new TextBlock { Text = text }), Padding = new Thickness(0) };
        b.Click += (_, _) => click();
        return b.Named($"{text}: {tip}", id).Tip(tip);
    }

    /// <summary>A mode on the Blocks page: its name (opens it), its switch, what
    /// switches it, its blocks; blocks dropped on it go in.</summary>
    UIElement ModeCard(Mode m)
    {
        var name = m.Name;
        var title = new HyperlinkButton { Content = Ui.Text(name, "SubtitleTextBlockStyle", wrap: false), Padding = new Thickness(0) }
            .Named($"Open the mode {name}", "ModeCard_" + name).Tip("Open the mode");
        title.Click += (_, _) => OpenMode(name);
        var icon = Ui.Icon(m.Monk ? Glyphs.Moon : m.Freeze ? Glyphs.Snow : "", 20);
        var (on, enabled, tip, stateWords) = ModeSwitchState(m);
        var toggle = Ui.Switch($"{name} on", on, wanted => _ = ToggleMode(m, wanted), "ModeSwitch_" + name);
        toggle.IsEnabled = enabled;
        ToolTipService.SetToolTip(toggle, tip);
        var stateText = Ui.Dim(stateWords, wrap: false);
        stateText.VerticalAlignment = VerticalAlignment.Center;
        var line = new HyperlinkButton { Content = ModeWhat(m), Padding = new Thickness(0) }
            .Named($"{name}: {ModeWhat(m)}", "ModeLine_" + name).Tip("Open the mode: its blocks and what switches them");
        line.Click += (_, _) => OpenMode(name);
        var top = Ui.Side(Ui.Column(6, Ui.Wrap(10, icon, title), line), RightColumn(Ui.Row(10, stateText, toggle)));
        var card = Ui.Card(top);
        if (!m.Monk && !m.Freeze)
        {
            card.AllowDrop = true;
            card.DragOver += (_, e) =>
            {
                e.AcceptedOperation = DataPackageOperation.Move;
                e.DragUIOverride.Caption = $"Put into {name}";
            };
            card.Drop += async (_, e) =>
            {
                if (!e.DataView.Contains(StandardDataFormats.Text)) return;
                var text = await e.DataView.GetTextAsync();
                if (text.StartsWith("kc9k-block:")) await MoveToMode(text["kc9k-block:".Length..], name);
            };
        }
        return card;
    }

    public static (bool On, bool Enabled, string Tip, string Words) ModeSwitchState(Mode m)
    {
        if (m.Freeze)
        {
            var left = m.FreezeLeft;
            return (m.Running, !m.Running,
                left > 0 ? $"Frozen for {Words.Seconds(left)} more: it can't end early" : "Freeze the computer until a time you choose (at most a week)",
                left > 0 ? $"On, {Words.Seconds(left)} left" : "Off");
        }
        if (m.Monk)
        {
            // R-MONK-3: switched like any mode; only a run for a set time is locked.
            var left = m.MonkLeft;
            return (m.Active || left > 0, left == 0,
                left > 0 ? $"Locked for {Words.Seconds(left)} more: it was started for a time"
                    : m.Active ? "Switch monk mode off" : "Switch monk mode on: no websites and no internet while it runs",
                left > 0 ? $"Locked, {Words.Seconds(left)} left" : m.Active ? (m.Running ? "Running" : "On, waiting for its windows or places") : "Off");
        }
        int on = m.On, total = m.Blocks.Count;
        var runLeft = m.RunLeft;
        return (total > 0 && on == total, total > 0 && runLeft == 0,
            runLeft > 0 ? $"Run by a Mode action, locked for {Words.Seconds(runLeft)} more"
                : on == total && total > 0 ? "Switch its blocks off (locked ones stay on)" : "Switch its blocks on (without locks)",
            runLeft > 0 ? $"Running, locked, {Words.Seconds(runLeft)} left" : total == 0 ? "No blocks yet" : on == total ? "On" : on == 0 ? "Off" : $"{on} of {total} on");
    }

    static string ModeWhat(Mode m)
    {
        if (m.Freeze)
            return m.Continuous ? "No scheduled freezes: switched on with an end time" : "Scheduled: " + Words.ScheduleSummary(m.Windows).Split(" · ")[0];
        if (m.Monk)
            return m.Continuous ? "No windows or places: on until switched off"
                : "Automated: " + (m.Schedules.Count > 0 ? string.Join(", ", m.Schedules) : Words.ScheduleSummary(m.Windows).Split(" · ")[0])
                  + (m.Places.Count > 0 ? " · at " + string.Join(", ", m.Places) : "");
        var what = (m.Continuous ? "Continuous" : "Automated") + " · " +
                   (m.Blocks.Count > 0 ? $"{Words.Plural(m.Blocks.Count, "block")}: {string.Join(", ", m.Blocks)}" : "drag blocks here");
        if (m.Locked > 0) what += $" · {m.Locked} locked";
        return what;
    }

    public async Task ToggleMode(Mode m, bool wanted)
    {
        ForceCards();
        if (m.Freeze)
        {
            if (wanted) await Device.FreezeDialog(window);
            Refresh();
            return;
        }
        if (m.Monk && wanted)
        {
            var when = m.Continuous ? "until you switch it off" : "while its windows and places have it on";
            var (ok, _) = await C.Dialogs.ConfirmLock([$"monk mode, switched on: no websites and no internet {when}"]);
            if (ok) await C.Run(["mode", "on", m.Name, "--yes"]);
            else Refresh();
            return;
        }
        await C.Run(["mode", wanted ? "on" : "off", m.Name]);
    }

    void Refresh()
    {
        ForceCards();
        if (state is not null) Update(state);
    }

    public async Task CreateMode(string? block = null)
    {
        var name = await C.Dialogs.AskText("New mode", "A name, such as Evening or Work. Then drag blocks onto it; one schedule and places switch them together.",
            problem: t => t.Length == 0 ? "Type a name." : state?.FindMode(t) is not null ? "A mode has that name already." : null, ok: "Create");
        if (string.IsNullOrEmpty(name)) return;
        if (await C.Run(["mode", "create", name]))
        {
            if (block is not null) await MoveToMode(block, name);
            else OpenMode(name);
        }
    }

    /// <summary>R-MODE-1: the block gets the mode's automation (refused if that
    /// weakens it while it is locked).</summary>
    public async Task MoveToMode(string block, string mode)
    {
        var b = state?.FindBlock(block);
        if (b is null || b.Mode == mode) return;
        if (await C.Dialogs.Confirm($"Put {block} into {mode}?",
                $"It then has the mode's automation: its own windows and places give way to {mode}'s. Taken out later, it keeps the mode's.", "Put it in"))
        {
            ForceCards();
            await C.Run(["mode", "add", mode, block]);
        }
    }

    /// <summary>Locked: the unlock flow; else the start/add-locks dialog.</summary>
    async Task CardLocks(string name)
    {
        var b = state?.FindBlock(name);
        if (b is null) return;
        if (b.Held) await BlockView.Unlock(window, b);
        else await BlockView.AddLocks(window, b);
    }

    public void OpenBlock(string name, string? tab = null)
    {
        if (tab is "automation")
        {
            OpenAutomation(name);
            return;
        }
        _ = OpenBlockAsync(name, tab);
    }

    async Task OpenBlockAsync(string name, string? tab)
    {
        if (view == "detail" && Block.Name != name && !await LeaveBlock(toCards: false)) return;
        var real = state?.FindBlock(name)?.DraftOf is { Length: > 0 } of ? of : name;
        Block.Open(real, tab);
        Show("detail");
        if (state is not null && !Block.Update(state))
        {
            // Made a moment ago: shown once the state that has it is read.
            C.Refresh();
        }
    }

    /// <summary>Leaves the opened block; unsaved edits are asked about first. False: stay.</summary>
    public async Task<bool> LeaveBlock(bool toCards = true)
    {
        var b = state?.FindBlock(Block.Name);
        if (view == "detail" && b is { Draft.Length: > 0 })
        {
            var choice = await AskUnsaved(b.Name);
            if (choice is null) return false;
            await C.Run(["block", "draft", b.Name, choice]);
        }
        if (toCards) Show("cards");
        return true;
    }

    async Task<string?> AskUnsaved(string name)
    {
        var d = new ContentDialog
        {
            Title = $"Save the edits to {name}?",
            Content = Ui.Text("Its websites, apps, breaks and bypass prevention change only when you save them."),
            PrimaryButtonText = "Save edits", SecondaryButtonText = "Discard", CloseButtonText = "Cancel",
            PrimaryButtonStyle = Theme.Style("AccentButtonStyle"),
        };
        return await C.Dialogs.Show(d) switch
        {
            ContentDialogResult.Primary => "save",
            ContentDialogResult.Secondary => "cancel",
            _ => null,
        };
    }

    public void OpenMode(string name)
    {
        Mode.Open(name);
        Show("mode");
        if (state is not null) Mode.Update(state);
    }

    /// <summary>The automation view of a block; a block in a mode opens the mode instead (R-MODE-1).</summary>
    public void OpenAutomation(string name)
    {
        var b = state?.FindBlock(name);
        if (b is null) return;
        if (b.InMode)
        {
            OpenMode(b.Mode);
            return;
        }
        Automation.OpenBlock(name);
        Show("automation");
        if (state is not null) Automation.Update(state);
    }
}
