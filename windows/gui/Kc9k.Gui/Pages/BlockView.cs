// §4.3 an opened block (gui/kc9k-gui.py show_block and its tabs): the part
// from "← All blocks" to the Start/Stop/… row stays put, only the tabs below
// scroll (owner 2026-11-20, D1). The block is always a draft: what its tabs
// change goes to the draft (the controller moves those commands) and applies
// with Save edits; Start, Stop, Unlock, Extend and taking a break apply at once.
using System.Globalization;
using Kc9k.Core;
using TriggerAction = Kc9k.Core.TriggerAction;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;

namespace Kc9k.Gui;

public sealed class BlockView
{
    readonly MainWindow window;
    readonly BlocksPage page;
    readonly TextBlock title = Ui.Text("", "Kc9kPageTitle", wrap: false).Id("PageTitle");
    readonly WrapPanel pills = new() { HorizontalSpacing = 6 };
    readonly TextBlock summary = Ui.Dim("");
    readonly InfoBar modeBar, hiddenBar, draftBar;
    readonly Button startButton, stopButton, unlockButton, extendButton, automationButton;
    readonly MenuFlyoutItem usersItem, deleteItem;
    // The tabs wrap onto more lines when narrow (no sideways scroll, W54).
    readonly WrapPanel tabs = new() { HorizontalSpacing = 4, VerticalSpacing = 4 };
    string tabShown = "";
    readonly ScrollViewer tabScroll = new() { HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled, VerticalScrollBarVisibility = ScrollBarVisibility.Auto };
    readonly Dictionary<string, (ToggleButton Item, UIElement View)> tabViews = [];
    Block? block;
    GuiState? state;

    // Websites
    readonly ToggleSwitch testSwitch;
    readonly Dictionary<string, StackPanel> listBoxes = [];
    readonly Dictionary<string, ListCard> listCards = [];
    readonly Dictionary<string, PhraseSetCard> phraseCards = [];
    readonly StackPanel socialBox = Ui.Column(4);
    readonly Dictionary<string, CheckBox> socialChecks = [];
    readonly Expander socialExpander;

    // Apps
    readonly EditableList apps, titles;
    readonly TriggerEditor appTriggers, titleTriggers;

    // Breaks & allowance
    readonly TextBlock quotaKept = Ui.Text("");
    readonly DurationField quotaTime, workTime, breakTime, longTime, rewardTime, rewardPer, rewardKeepTime, relockTime, delayBreakTime, delayWaitTime;
    readonly ComboBox quotaPer, rewardKeep;
    readonly ClockField quotaRefill;
    readonly NumberField quotaSessions, longEvery;
    readonly CheckBox longCheck, relockCheck, delayBreakCheck;
    readonly Button quotaStart, quotaEnd, quotaRemove, breakStart, breakEnd, delayRequest;
    readonly TextBlock quotaLabel = Ui.Dim(""), breakLabel = Ui.Dim("");

    // Bypass prevention
    readonly TextBlock nepsisLabel = Ui.Text(""), nepsisKept = Ui.Text("");
    readonly Button nepsisOff;

    public string Name { get; private set; } = "";
    public UIElement View { get; }
    Controller C => window.Controller;

    public BlockView(MainWindow window, BlocksPage page)
    {
        this.window = window;
        this.page = page;
        var back = Ui.Link("← All blocks", () => page.ShowCards(), "BackToBlocks");
        modeBar = new InfoBar { Severity = InfoBarSeverity.Informational, IsClosable = false, IsOpen = false }.Named("Mode", "BlockModeBar");
        modeBar.ActionButton = Ui.Button("Open the mode", () => { if (block is not null) page.OpenMode(block.Mode); }, "OpenItsMode");
        modeBar.Content = Ui.Button("Take it out", () => { if (block is not null) _ = C.Run(["mode", "take-out", block.Name]); }, "TakeOutOfMode").Tip("It gets its own automation back");
        hiddenBar = new InfoBar { Severity = InfoBarSeverity.Informational, IsClosable = false, IsOpen = false, Message = "Its lists, apps and window titles are hidden by its block password." }.Named("Hidden contents", "HiddenBar");
        hiddenBar.ActionButton = Ui.Button("Show contents…", () => _ = Reveal(), "RevealBlock");
        draftBar = new InfoBar { Severity = InfoBarSeverity.Warning, IsClosable = false, IsOpen = false, Title = "Unsaved edits" }.Named("Unsaved edits", "DraftBar");
        draftBar.ActionButton = Ui.Button("Save edits", () => _ = FinishDraft("save"), "SaveEdits", accent: true);
        draftBar.Content = Ui.Button("Discard", () => _ = FinishDraft("cancel"), "DiscardEdits").Tip("Drop the edits; the block stays as it is");

        startButton = Ui.Button("Start", () => _ = Start(), "BlockStart", accent: true, glyph: Glyphs.Play);
        stopButton = Ui.Button("Stop", () => { if (block is not null) _ = C.Run(["stop", block.Name]); }, "BlockStop", glyph: Glyphs.Stop);
        unlockButton = Ui.Button("Unlock…", () => { if (block is not null) _ = Unlock(window, block); }, "BlockUnlock", glyph: Glyphs.Unlock);
        extendButton = Ui.Button("Extend timer…", () => _ = ExtendTimer(), "BlockExtend", glyph: Glyphs.Timer);
        automationButton = Ui.Button("Automation", () => page.OpenAutomation(Name), "BlockAutomation", glyph: Glyphs.Clock);
        var more = new MenuFlyout();
        MenuFlyoutItem Item(string text, string glyph, Func<Task> act)
        {
            var i = new MenuFlyoutItem { Text = text, Icon = new FontIcon { Glyph = glyph } };
            i.Click += (_, _) => _ = act();
            more.Items.Add(i);
            return i;
        }
        Item("Rename…", Glyphs.Rename, Rename);
        Item("Duplicate…", Glyphs.Copy, Duplicate);
        Item("Save as block preset…", Glyphs.Save, SaveTemplate);
        usersItem = Item("Users: all", "\uE716", ChooseUsers);
        more.Items.Add(new MenuFlyoutSeparator());
        deleteItem = Item("Delete…", Glyphs.Delete, Delete);
        var moreButton = new Button { Content = new FontIcon { Glyph = Glyphs.More, FontSize = 14 }, Flyout = more }
            .Named("More: rename, duplicate, block preset, users, delete", "BlockMore").Tip("More: rename, duplicate, block preset, users, delete");
        var actions = Ui.Side(Ui.Wrap(6, startButton, stopButton, unlockButton, extendButton, automationButton), moreButton);
        actions.MainMin = 120;
        actions.SideAtTop = true;

        // Websites tab ---------------------------------------------------
        testSwitch = Ui.Switch("Test mode", false, on => { if (block is not null) _ = C.Run(["list", "test", block.Name, on ? "on" : "off"]); }, "KeywordTest");
        var websites = Ui.Column(16,
            Ui.Wrap(6, Ui.Dim("Lists of sites and words, each with what it does to the pages it matches."), Ui.HelpButton(
                "Each list ticks where it looks: the address, the page title (the tab title) or the page text. Addresses take domains, paths and wildcards (*.xxx, *casino*, *.* for everything); titles and text take words: a plain word counts only on its own (porn misses pornhub), and * matches inside words (*porn* hits pornhub). Words that count only together are phrases, in phrase sets below. A match in the address or title takes every action; in the text, each action needs its number of different words. Unticking or removing weakens the block, so it isn't possible while it is locked.", "lists")),
            Ui.Wrap(6,
                Ui.Button("Save lists as preset…", () => _ = SavePreset("lists"), "SaveListsPreset"),
                Ui.Button("Add lists from a preset…", () => _ = FromPreset("lists"), "UseListsPreset")),
            Ui.Card(Ui.SettingRow("Test mode", "Lists that read pages only show what they find. Turning it on weakens the block.", testSwitch)));
        socialExpander = new Expander
        {
            Header = "Social media", HorizontalAlignment = HorizontalAlignment.Stretch, HorizontalContentAlignment = HorizontalAlignment.Stretch,
            Content = Ui.Column(8, Ui.Dim("Keep a site, take a part of it away while this block is on. Don't put the site on the blacklist too. Ticking is always allowed; unticking weakens the block. Sites change their pages now and then, and a setting that hides something may then miss it until an update."), socialBox),
        }.Named("Social media", "Social");
        foreach (var (kind, heading, label, detail) in new[]
                 {
                     ("blacklist", "Blacklists", "New blacklist…", ""),
                     ("exceptions", "Exceptions", "New exception list…", "A page an exception list matches is spared from every blacklist of this block. A YouTube playlist's address (youtube.com/playlist?list=…) lets that playlist's videos through, and only them."),
                     ("phrases", "Phrase sets", "New phrase set…", "Phrases are words that count only together, such as [eat, nice] [hello, grandpa]. The block's exceptions spare pages from its phrase sets too."),
                 })
        {
            if (kind == "exceptions") websites.Children.Add(socialExpander);
            var create = Ui.Button(label, () => _ = CreateList(kind), "New_" + kind, glyph: Glyphs.Add);
            websites.Children.Add(Ui.Side(Ui.Heading(heading), create));
            if (detail.Length > 0) websites.Children.Add(Ui.Dim(detail));
            listBoxes[kind] = Ui.Column(12);
            websites.Children.Add(listBoxes[kind]);
        }

        // Apps tab -------------------------------------------------------
        apps = new EditableList("Apps and folders", "Discord.exe  steam.exe  C:\\Games\\",
            values => Cmd("add-app", values), (value, _) => Cmd("remove-app", [value]),
            () => ImportFile(["block", "import-list", Name, "apps"]),
            () => Files.Export(window, ["block", "export-list", Name, "apps"], Name + " apps"),
            help: "Program file names, or folders whose programs can't run. Everything a blocked app started is closed too.",
            searchBox: true, multi: true, id: "Apps");
        titles = new EditableList("Window titles", "YouTube  Twitch",
            values => Cmd("add-title", values), (value, _) => Cmd("remove-title", [value]),
            () => ImportFile(["block", "import-list", Name, "titles"]),
            () => Files.Export(window, ["block", "export-list", Name, "titles"], Name + " titles"),
            help: "Any app whose window title contains one of these words is found while the block is on. Browsers are never matched by title: their tabs are judged by the lists (R-TITLE-1). Separate words with spaces.",
            searchBox: true, hidden: true, multi: true, id: "Titles");
        appTriggers = new TriggerEditor(window, "apps", "close", "Close the app",
            "When one of these apps runs (logging out, shutting down and locking the screen come with a minute's warning):");
        titleTriggers = new TriggerEditor(window, "titles", "close", "Close the window", "When a window with one of these titles is open:");
        var appsTab = Ui.Column(16,
            Ui.Wrap(6,
                Ui.Button("Choose from installed apps…", () => _ = PickApps(), "PickApps", glyph: Glyphs.App),
                Ui.Button("Add a folder…", () => _ = PickFolder(), "PickAppFolder"),
                Ui.Button("Add a program file…", () => _ = PickFile(), "PickAppFile")),
            apps, Ui.Card(appTriggers), titles, Ui.Card(titleTriggers));

        // Breaks & allowance tab ------------------------------------------
        quotaKept.Visibility = Visibility.Collapsed;
        quotaTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 86400 }, 1800, width: 110);
        quotaTime.Named("Allowance time", "QuotaTime");
        quotaPer = Combo("Allowance period", "QuotaPer", ("day", "per day"), ("week", "per week"), ("month", "per month"), ("rolling", "per 24 hours, rolling"), ("window", "per window"));
        quotaPer.SelectionChanged += (_, _) => quotaTime.SetLimits(60, Key(quotaPer) switch { "week" => 7 * 86400, "month" => 31 * 86400, _ => 86400 });
        quotaRefill = new ClockField(0);
        quotaRefill.Named("Allowance refill time", "QuotaRefill");
        quotaSessions = new NumberField(0, 0, 100, "0: any number", width: 70);
        quotaSessions.Named("Allowance sessions", "QuotaSessions");
        quotaStart = Ui.Button("Use allowance now", () => Simple(["allowance", "start"]), "QuotaStart");
        quotaEnd = Ui.Button("End session", () => Simple(["allowance", "end"]), "QuotaEnd");
        quotaRemove = Ui.Button("Remove", () => Simple(["allowance", "remove"]), "QuotaRemove");
        var allowance = Ui.Section("Allowance",
            "Sessions you open lift the block until the period's time is spent. A session only spends time while the block is in use: its app in front, its site in the focused tab, or playing sound or video. An allowance protects nothing by itself: changes apply at once until the block is locked; then it only takes less time or fewer sessions. Per window: each opening of a block window is a period.",
            Ui.Dim("Time to use what the block blocks, in sessions you open."),
            Form("Time", quotaTime, quotaPer, Ui.Text("refill at"), quotaRefill),
            Form("Sessions", quotaSessions),
            Ui.Wrap(6, Ui.Button("Save allowance", () => _ = SetQuota(), "QuotaSave", accent: true), quotaStart, quotaEnd, quotaRemove),
            quotaLabel);

        workTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 86400 }, 25 * 60, width: 100);
        workTime.Named("Pomodoro work", "PomodoroWork");
        breakTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 86400 }, 5 * 60, width: 100);
        breakTime.Named("Pomodoro break", "PomodoroBreak");
        longCheck = new CheckBox { MinWidth = 0 }.Named("Long break", "PomodoroLongOn").Tip("A long break now and then, instead of the short one");
        longTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 86400 }, 15 * 60, width: 100);
        longTime.Named("Long break length", "PomodoroLong");
        longEvery = new NumberField(4, 2, 20, width: 70);
        longEvery.Named("Long break every so many work periods", "PomodoroEvery");
        var pomodoro = Ui.Section("Pomodoro",
            "While the block is on, it pauses for the break after each work period; its rhythm starts again each time the block is enabled (R-SCHED-3). During a work period the block's allowance can't be used. Locks stay on through the breaks.",
            Ui.Dim("Work and break take turns while the block is on, work first."),
            Form("Work", workTime, Ui.Text("then a break of"), breakTime),
            Form("Long break", longCheck, longTime, Ui.Text("every"), longEvery, Ui.Text("work periods")),
            Ui.Wrap(6, Ui.Button("Save", () => _ = SetPomodoro(), "PomodoroSave", accent: true), Ui.Button("Turn off", () => Simple(["pomodoro", "off"]), "PomodoroOff")));

        rewardTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 4 * 3600 }, 600, width: 100);
        rewardTime.Named("Reward: break earned", "RewardTime");
        rewardPer = new DurationField(new DurationRule { Minimum = 300, Maximum = 86400 }, 3600, width: 100);
        rewardPer.Named("Reward: for every so much blocking", "RewardPer");
        rewardKeep = Combo("Keep the reward", "RewardKeep", ("one", "up to one break"), ("window", "until the window ends"), ("day", "until the day ends"),
            ("week", "until the week ends"), ("month", "until the month ends"), ("time", "for a time after it is earned:"));
        rewardKeepTime = new DurationField(new DurationRule { Minimum = 3600 }, 7 * 86400, width: 100);
        rewardKeepTime.Named("Keep each part for", "RewardKeepTime");
        rewardKeep.SelectionChanged += (_, _) => rewardKeepTime.Visibility = Key(rewardKeep) == "time" ? Visibility.Visible : Visibility.Collapsed;
        rewardKeepTime.Visibility = Visibility.Collapsed;
        breakStart = Ui.Button("Take a break now", () => Simple(["break", "start"]), "BreakStart");
        breakEnd = Ui.Button("End break", () => Simple(["break", "end"]), "BreakEnd");
        var reward = Ui.Section("Reward",
            "Break time is earned while the block is on and the computer is in use, and kept as chosen: up to one break, until the window, day, week or month ends, or each part for as long as you choose. Take it when you choose, also while the block is locked; the block comes back by itself. Allowing more breaks weakens the block, so decide before locking it.",
            Ui.Dim("Break time earned while the block works, to take when you choose."),
            Form("Earn", rewardTime, Ui.Text("of break for every"), rewardPer, Ui.Text("of blocking")),
            Form("Keep it", rewardKeep, rewardKeepTime),
            Ui.Wrap(6, Ui.Button("Save reward", () => _ = SetReward(), "RewardSave", accent: true), Ui.Button("Remove", () => Simple(["break", "reward", "", "off"]), "RewardRemove"), breakStart, breakEnd),
            breakLabel);

        relockCheck = new CheckBox { Content = "Unlocking gives a break of", MinWidth = 0 }.Named("Text break", "RelockOn");
        relockTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 4 * 3600 }, 300, width: 100);
        relockTime.Named("Text break length", "RelockTime");
        delayBreakCheck = new CheckBox { Content = "Delay break of", MinWidth = 0 }.Named("Delay break", "DelayBreakOn");
        delayBreakTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 4 * 3600 }, 600, width: 100);
        delayBreakTime.Named("Delay break length", "DelayBreakTime");
        delayWaitTime = new DurationField(new DurationRule { Minimum = 60, Maximum = 86400 }, 1800, width: 100);
        delayWaitTime.Named("Delay break wait", "DelayWaitTime");
        delayRequest = Ui.Button("Request break", () => Simple(["break", "request"]), "DelayRequest");
        var keepLocks = Ui.Section("Breaks that keep the locks",
            "Text break: passing the text lock's challenge or giving the password doesn't end the block's lock, it lifts the block for a while; unlocking again during that break ends it. Delay break: asked for, it begins after the wait you chose.",
            Ui.Dim("A break after a challenge or a countdown; the block stays locked."),
            Form("Text break", relockCheck, relockTime, Ui.Text("then the block is back with its locks"), Ui.Button("Save", () => _ = SetRelock(), "RelockSave")),
            Form("Delay break", delayBreakCheck, delayBreakTime, Ui.Text("after waiting"), delayWaitTime, Ui.Button("Save", () => _ = SetDelayBreak(), "DelayBreakSave"), delayRequest));
        var breaksTab = Ui.Column(20, quotaKept, allowance, pomodoro, reward, keepLocks);

        // Bypass prevention tab -----------------------------------------
        nepsisKept.Visibility = Visibility.Collapsed;
        nepsisOff = Ui.Button("Turn off…", () => _ = NepsisOff(), "BlockNepsisOff");
        var bypassTab = Ui.Column(12,
            Ui.Dim("Bypass prevention: when this block stops many things in a short time (pages, apps, windows), likely a hunt for a loophole, it takes a consequence so you step away. It counts while the block is on; the one under Settings counts everything, and both apply."),
            Ui.Card(nepsisLabel), nepsisKept,
            Ui.Wrap(6, Ui.Button("Change…", () => _ = ChangeNepsis(), "BlockNepsisChange", accent: true), nepsisOff));

        foreach (var (key, label, content) in new (string, string, UIElement)[]
                 {
                     ("websites", "Websites", websites), ("apps", "Apps", appsTab),
                     ("breaks", "Breaks & allowance", breaksTab), ("bypass", "Bypass prevention", bypassTab),
                 })
        {
            var item = new ToggleButton { Content = label, Tag = key }.Named(label, "Tab_" + key);
            item.Click += (_, _) => SelectTab(key);
            tabs.Children.Add(item);
            tabViews[key] = (item, content);
        }
        SelectTab("websites");
        tabs.Named("Tabs of the block", "BlockTabs");

        var top = Ui.Column(8, back, title, pills, summary, modeBar, hiddenBar, draftBar, actions);
        top.Padding = Ui.PagePadding(double.PositiveInfinity, 20, 0);
        // W54: the full width beside the sidebar, as every page.
        // The head stays put while it takes at most half the height; taller
        // (a short window, large text) it scrolls by itself, so the tabs and
        // their content are always shown under it.
        var head = new ScrollViewer
        {
            Content = top,
            HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled,
            VerticalScrollBarVisibility = ScrollBarVisibility.Auto,
        };
        var grid = new Grid();
        var headRow = new RowDefinition { Height = GridLength.Auto };
        grid.SizeChanged += (_, e) =>
        {
            top.Padding = Ui.PagePadding(e.NewSize.Width, 20, 0);
            var pad = Ui.PagePadding(e.NewSize.Width, 8, 0);
            tabs.Margin = new Thickness(pad.Left, pad.Top, pad.Right, 0);
            headRow.MaxHeight = Math.Max(120, e.NewSize.Height / 2);
        };
        grid.RowDefinitions.Add(headRow);
        grid.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        grid.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        grid.Children.Add(head);
        tabs.Margin = new Thickness(32, 8, 32, 0);
        Grid.SetRow(tabs, 1);
        grid.Children.Add(tabs);
        Grid.SetRow(tabScroll, 2);
        grid.Children.Add(tabScroll);
        View = grid;
    }

    Grid? pad;

    /// <summary>The tab area: one padded grid whose only child is the tab shown.
    /// A view is detached from any parent first (WinUI throws 0x800F1000 when an
    /// element still has one).</summary>
    UIElement Pad(UIElement content)
    {
        if (pad is null)
        {
            pad = new Grid { Padding = Ui.PagePadding(tabScroll.ActualWidth > 0 ? tabScroll.ActualWidth : double.PositiveInfinity, 12) };
            pad.SizeChanged += (_, e) => pad.Padding = Ui.PagePadding(e.NewSize.Width, 12);
        }
        pad.Children.Clear();
        Detach(content);
        pad.Children.Add(content);
        return pad;
    }

    static void Detach(UIElement e)
    {
        switch ((e as FrameworkElement)?.Parent)
        {
            case Panel p: p.Children.Remove(e); break;
            case Border b: b.Child = null; break;
            case ContentControl c: c.Content = null; break;
        }
    }

    static ComboBox Combo(string name, string id, params (string Key, string Title)[] items)
    {
        var c = new ComboBox { MinWidth = 180 }.Named(name, id);
        foreach (var (k, t) in items) c.Items.Add(new ComboBoxItem { Content = t, Tag = k });
        c.SelectedIndex = 0;
        return c;
    }

    static string Key(ComboBox c) => (c.SelectedItem as ComboBoxItem)?.Tag as string ?? "";

    static void Select(ComboBox c, string key)
    {
        foreach (var i in c.Items.OfType<ComboBoxItem>())
            if ((string)i.Tag == key) c.SelectedItem = i;
    }

    static UIElement Form(string label, params UIElement[] fields)
    {
        var g = new Grid { ColumnSpacing = 12 };
        g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(110) });
        g.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        var l = Ui.Text(label);
        l.Margin = new Thickness(0, 6, 0, 0);
        g.Children.Add(l);
        var w = Ui.Wrap(8, fields);
        foreach (var f in fields) if (f is FrameworkElement fe) fe.VerticalAlignment = VerticalAlignment.Top;
        foreach (var f in fields) if (f is TextBlock t) t.Margin = new Thickness(0, 6, 0, 0);
        Grid.SetColumn(w, 1);
        g.Children.Add(w);
        return g;
    }

    public void Open(string name, string? tab)
    {
        Name = name;
        SelectTab(tab is not null && tabViews.ContainsKey(tab) ? tab : "websites");
    }

    /// <summary>Shows a tab; its button stays pressed (a second press keeps it).</summary>
    void SelectTab(string key)
    {
        foreach (var (k, (item, _)) in tabViews) item.IsChecked = k == key;
        if (tabShown == key) return;
        tabShown = key;
        var padded = Pad(tabViews[key].View);
        if (!ReferenceEquals(tabScroll.Content, padded)) tabScroll.Content = padded;
    }

    Task Cmd(string command, IReadOnlyList<string> values) => C.Run(["block", command, Name, .. values]);

    void Simple(List<string> args)
    {
        // Name the block: second place, or after the subcommand.
        if (args.Count >= 3 && args[2] == "") args[2] = Name;
        else args.Insert(2, Name);
        _ = C.Run(args);
    }

    /// <summary>Shows the opened block; false when it is gone (deleted elsewhere).</summary>
    public bool Update(GuiState s)
    {
        state = s;
        var b = s.FindBlock(Name);
        if (b is null) return false;
        block = b;
        title.Text = b.Name;
        pills.Children.Clear();
        foreach (var p in Words.BlockPills(b)) pills.Children.Add(Ui.Pill(p));
        summary.Text = Words.BlockSummary(b);
        usersItem.Text = "Users: " + (b.Users.Count > 0 ? string.Join(", ", b.Users) : "all");
        usersItem.Visibility = s.Accounts.Count > 1 || b.Users.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        hiddenBar.IsOpen = b.Hidden;
        var content = (b.Draft.Length > 0 ? s.FindBlock(b.Draft) : null) ?? b;
        var editing = !ReferenceEquals(content, b);
        draftBar.IsOpen = editing;
        draftBar.Message = editing ? "Its websites, apps, breaks and bypass prevention change only when you save them." +
                                     (b.Held ? " What would weaken it while it is locked is refused then, and named." : "") : "";
        modeBar.IsOpen = b.InMode;
        modeBar.Message = b.InMode ? $"In the mode {b.Mode}: it has the mode's automation, changed on the mode for all its blocks. Its own is kept for when it is taken out." : "";
        automationButton.IsEnabled = !b.InMode;
        ToolTipService.SetToolTip(automationButton, b.InMode ? $"Automated by '{b.Mode}'" : "What switches it by itself: its windows and places");

        UpdateLists(content);
        apps.SetItems(content.Apps, b.Name);
        titles.SetItems(content.Titles, b.Name);
        appTriggers.SetActions(b.Name, content.Triggers.GetValueOrDefault("apps") ?? [new TriggerAction { Name = "close" }]);
        titleTriggers.SetActions(b.Name, content.Triggers.GetValueOrDefault("titles") ?? [new TriggerAction { Name = "close" }]);
        Ui.Set(testSwitch, content.KeywordTest);
        UpdateSocial(s, content);
        UpdateBreaks(b, content);

        startButton.Content = Ui.Row(8, Ui.Icon(b.Active ? Glyphs.Lock : Glyphs.Play, 14), new TextBlock { Text = b.Active ? "Add locks…" : "Start" });
        Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(startButton, b.Active ? "Add locks" : "Start");
        // §3.4: unlocking lifts locks (a delay lock starts counting down) and
        // leaves the block on; stopping needs every lock lifted.
        stopButton.IsEnabled = b.Active && !b.Held;
        unlockButton.IsEnabled = (b.CharLocked || b.PasswordLocked || (b.DelaySeconds > 0 && b.DelayLeft == 0)) && b.LockSeconds == 0 && !b.RestartLocked;
        extendButton.IsEnabled = b.Active && b.LockOrder.Contains("timer");
        deleteItem.IsEnabled = !b.Active && !b.Held;

        nepsisLabel.Text = Words.Nepsis(content.Nepsis, b.Active ? content.NepsisCount : null);
        nepsisOff.Visibility = content.Nepsis.IsOn ? Visibility.Visible : Visibility.Collapsed;
        // §4.3: settings of these two tabs kept in the draft until Save edits.
        var kept = b.DraftKept.Concat(content.DraftKept).Distinct().ToList();
        Kept(quotaKept, kept.Where(k => !k.StartsWith("Bypass prevention")).ToList());
        Kept(nepsisKept, kept.Where(k => k.StartsWith("Bypass prevention")).ToList());
        return true;
    }

    static void Kept(TextBlock label, List<string> items)
    {
        label.Text = items.Count > 0 ? "Not saved yet (applies with Save edits): " + string.Join("; ", items) : "";
        label.Visibility = items.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
    }

    /// <summary>The lists and phrase sets of what the tabs show, keeping the
    /// cards of those still there (by name, so the draft taking over keeps
    /// what is being typed).</summary>
    void UpdateLists(Block content)
    {
        var wanted = content.Lists.ToDictionary(l => (l.IsExceptions ? "e:" : "b:") + l.Name.ToLowerInvariant());
        foreach (var key in listCards.Keys.Where(k => !wanted.ContainsKey(k)).ToList())
        {
            var card = listCards[key];
            ((Panel)card.Parent).Children.Remove(card);
            listCards.Remove(key);
        }
        foreach (var l in content.Lists)
        {
            var key = (l.IsExceptions ? "e:" : "b:") + l.Name.ToLowerInvariant();
            if (listCards.TryGetValue(key, out var card)) card.Update(l);
            else
            {
                card = new ListCard(window, () => Name, l);
                listCards[key] = card;
                listBoxes[l.IsExceptions ? "exceptions" : "blacklist"].Children.Add(card);
            }
        }
        var sets = content.PhraseSets.ToDictionary(p => p.Name.ToLowerInvariant());
        foreach (var key in phraseCards.Keys.Where(k => !sets.ContainsKey(k)).ToList())
        {
            var card = phraseCards[key];
            ((Panel)card.Parent).Children.Remove(card);
            phraseCards.Remove(key);
        }
        foreach (var p in content.PhraseSets)
        {
            var key = p.Name.ToLowerInvariant();
            if (phraseCards.TryGetValue(key, out var card)) card.Update(p);
            else
            {
                card = new PhraseSetCard(window, () => Name, p);
                phraseCards[key] = card;
                listBoxes["phrases"].Children.Add(card);
            }
        }
        if (listBoxes["phrases"].Children.Count == 0) listBoxes["phrases"].Children.Add(Ui.Dim("No phrase sets."));
        else if (listBoxes["phrases"].Children.Count > 1 && listBoxes["phrases"].Children[0] is TextBlock) listBoxes["phrases"].Children.RemoveAt(0);
    }

    void UpdateSocial(GuiState s, Block content)
    {
        if (socialChecks.Count == 0)
        {
            string? site = null;
            foreach (var p in s.SocialPresets)
            {
                if (p.Site != site)
                {
                    site = p.Site;
                    socialBox.Children.Add(Ui.Strong(site));
                }
                var check = new CheckBox { Content = p.Title }.Named($"{p.Site}: {p.Title}", "Social_" + p.Name.Replace(':', '_'));
                var name = p.Name;
                check.Click += (_, _) => _ = C.Run(["social", Name, name, check.IsChecked == true ? "on" : "off"]);
                socialChecks[p.Name] = check;
                socialBox.Children.Add(check);
            }
        }
        foreach (var (name, check) in socialChecks) check.IsChecked = content.Social.Contains(name);
        var count = socialChecks.Keys.Count(content.Social.Contains);
        socialExpander.Header = "Social media" + (count > 0 ? $" · {Words.Plural(count, "setting")} on" : "");
    }

    void UpdateBreaks(Block b, Block content)
    {
        var focus = Microsoft.UI.Xaml.Input.FocusManager.GetFocusedElement(window.Content.XamlRoot) as DependencyObject;
        bool Focused(FrameworkElement f) => focus is not null && IsInside(focus, f);
        if (content.BreakAllowance > 0 && !Focused(rewardKeep) && !Focused(rewardKeepTime))
        {
            var keep = content.BreakKeep.Length > 0 ? content.BreakKeep : "one";
            if (keep.All(char.IsDigit))
            {
                rewardKeepTime.SetSeconds(long.Parse(keep, CultureInfo.InvariantCulture));
                keep = "time";
            }
            Select(rewardKeep, keep);
        }
        var paused = b.PausedSeconds;
        breakStart.IsEnabled = b.Active && content.BreakAllowance > 0 && b.BreakBalance >= 60 && paused == 0;
        breakEnd.IsEnabled = paused > 0;
        var text = paused > 0 ? $"On a break for {Words.Seconds(paused)} more."
            : content.BreakAllowance > 0 ? $"Reward: {content.BreakAllowance / 60} min per {content.BreakPer / 60} min of blocking. Earned: {b.BreakBalance / 60} min."
            : "No reward.";
        if (content.Relock > 0) text += $" Unlocking gives a break of {content.Relock / 60} min.";
        if (b.DelayBreakIn > 0) text += $" Its delay break begins in {Words.Seconds(b.DelayBreakIn)}.";
        breakLabel.Text = text;
        if (!Focused(delayBreakTime) && !Focused(delayWaitTime))
        {
            delayBreakCheck.IsChecked = content.DelayBreak > 0;
            if (content.DelayBreak > 0)
            {
                delayBreakTime.SetSeconds(content.DelayBreak);
                delayWaitTime.SetSeconds(content.DelayWait);
            }
        }
        if (!Focused(relockTime))
        {
            relockCheck.IsChecked = content.Relock > 0;
            if (content.Relock > 0) relockTime.SetSeconds(content.Relock);
        }
        delayRequest.IsEnabled = content.DelayBreak > 0 && b.DelayBreakIn == 0 && paused == 0;
        if (content.PomodoroOn > 0 && !Focused(workTime) && !Focused(breakTime))
        {
            workTime.SetSeconds(content.PomodoroOn);
            breakTime.SetSeconds(content.PomodoroOff);
            longCheck.IsChecked = content.PomodoroLong > 0;
            if (content.PomodoroLong > 0)
            {
                longTime.SetSeconds(content.PomodoroLong);
                longEvery.Text = content.PomodoroEvery.ToString(CultureInfo.InvariantCulture);
            }
        }
        var limit = content.QuotaLimit;
        quotaStart.IsEnabled = limit > 0 && !b.QuotaOpen && b.QuotaUsed < limit;
        quotaEnd.IsEnabled = b.QuotaOpen;
        quotaRemove.IsEnabled = limit > 0;
        if (limit > 0 && !Focused(quotaRefill) && !Focused(quotaTime) && !Focused(quotaSessions))
        {
            Select(quotaPer, content.QuotaPeriod.Length > 0 ? content.QuotaPeriod : "day");
            quotaRefill.Text = content.QuotaRefill.Length > 0 ? content.QuotaRefill : "00:00";
            quotaSessions.Text = content.QuotaSessions.ToString(CultureInfo.InvariantCulture);
            quotaTime.SetSeconds(limit);
        }
        quotaLabel.Text = limit > 0
            ? $"Used this {(content.QuotaPeriod.Length > 0 ? content.QuotaPeriod : "day")}: {b.QuotaUsed / 60} of {limit / 60} min"
              + (content.QuotaSessions > 0 ? $" · sessions {b.QuotaSessionsUsed} of {content.QuotaSessions}" : "")
              + (b.QuotaOpen ? " · session open" : "")
            : "No allowance set.";
    }

    static bool IsInside(DependencyObject focus, DependencyObject container)
    {
        for (var d = focus; d is not null; d = Microsoft.UI.Xaml.Media.VisualTreeHelper.GetParent(d))
            if (ReferenceEquals(d, container)) return true;
        return false;
    }

    // Commands of the block's menu (apply at once) ------------------------

    async Task Start()
    {
        if (block is null) return;
        // §4.3 (owner 2026-11-20, D2): Start applies at once, without locks;
        // once on, the same button is Add locks….
        if (block.Active) await AddLocks(window, block);
        else await C.Run(["start", block.Name]);
    }

    /// <summary>The lock dialog: locks for a block, which starts it if it is off.</summary>
    public static async Task AddLocks(MainWindow window, Block b)
    {
        // R-LOCK-7: a running block takes any kind of lock, after the ones on.
        var picker = new LockPicker("stop", "stopped", LockSpec.AllKinds, relockAny: true, id: "AddLocks");
        var asked = await LockDialogs.Ask(window, b.Active ? $"Add locks to {b.Name}" : $"Start {b.Name} with locks",
            b.Active ? "New locks come after the ones already on, unlocked last. A kind it already has is a second lock of that kind, after the others. A lock that is on is never changed."
                : "It starts with the locks below. Once on, a lock is never changed; more can be added.",
            "Add locks", picker, needLock: true);
        if (asked is not var (chosen, relock, secret) || chosen.Count == 0) return;
        // R-LOCK-6: locks unlock one at a time in the order of their flags.
        if (chosen.Count > 1)
        {
            var ordered = await LockDialogs.ChooseOrder(window, chosen);
            if (ordered is null) return;
            chosen = ordered;
        }
        chosen = LockSpec.Waiting(chosen, b.LockOrder); // behind the locks it has
        var (flags, lines) = LockDialogs.Lines($"block '{b.Name}'", chosen, relock);
        var (ok, _) = await window.Controller.Dialogs.ConfirmLock(lines);
        if (!ok) return;
        await window.Controller.Run(["start", b.Name, .. flags, "--yes"], secret.Length > 0 ? secret + "\n" : null);
    }

    public static async Task Unlock(MainWindow window, Block b)
    {
        var c = window.Controller;
        if (!b.CharLocked)
        {
            if (b.PasswordLocked)
            {
                var password = await c.Dialogs.AskText($"Unlock {b.Name}", "Enter the block's password.", password: true);
                if (!string.IsNullOrEmpty(password))
                    await c.Run(["unlock", b.Name], password + "\n", then: o => c.Dialogs.Message(o, "Unlocked"));
            }
            else if (b.DelaySeconds > 0 && b.DelayLeft == 0) // only a delay lock: unlocking starts its countdown
                await c.Run(["unlock", b.Name], then: o => c.Dialogs.Message(o, "Unlocking"));
            else if (b.LockSeconds > 0)
                await c.Dialogs.Message($"{b.Name} is timer-locked for {Words.Seconds(b.LockSeconds)} more: it can't be unlocked before then.", "Locked");
            return;
        }
        // R-LOCK-5: the command opens its own challenge window and reads the
        // keys typed there; the GUI only runs it.
        await c.Run(["unlock", b.Name], then: o => c.Dialogs.Message(o, "Unlocked"), timeout: TimeSpan.FromHours(6));
    }

    /// <summary>§3.4, §4.6: extends the block's timer lock (kc9k extend).</summary>
    async Task ExtendTimer()
    {
        if (block is null) return;
        var spec = new LockSpec(["timer"], "stopped") { Timer = TimerMode.For, TimerText = "30m" };
        var field = new TimerLockField(spec, allowNone: false, id: "Extend");
        var body = Ui.Column(10, Ui.Dim("For a time adds that much to the timer lock; Until a date moves its end later. A timer lock is never shortened. To add a second timer lock instead, use Add locks…."), field);
        if (!await C.Dialogs.Ask($"Extend the timer lock of {block.Name}", body, "Extend", () => spec.TimerProblem() is null, w => field.Changed += w)) return;
        var flags = spec.TimerFlags();
        if (flags.Count == 0) return;
        var what = flags[0] == "--for" ? $"{Words.Duration(Words.ParseDuration(spec.TimerText) ?? 0)} more" : $"until {Words.Date(spec.Until()!.Value)}";
        var (ok, _) = await C.Dialogs.ConfirmLock([$"block '{block.Name}': timer lock extended, {what}"]);
        if (ok) await C.Run(["extend", block.Name, .. flags, "--yes"]);
    }

    async Task FinishDraft(string how)
    {
        if (block?.Draft.Length > 0) await C.Run(["block", "draft", block.Name, how]);
    }

    async Task Reveal()
    {
        var password = await C.Dialogs.AskText($"Show '{Name}'", "Its block password.", password: true);
        if (string.IsNullOrEmpty(password)) return;
        C.Session.BlockPasswords[Name] = password;
        var (s, _, _) = await C.Session.ReadStateAsync();
        if (s?.FindBlock(Name) is not { Hidden: false })
        {
            C.Session.BlockPasswords.Remove(Name);
            await C.Dialogs.Message("Wrong block password.", "Not shown");
            return;
        }
        C.Refresh();
    }

    async Task Rename()
    {
        var name = await C.Dialogs.AskText("Rename block", initial: Name);
        if (string.IsNullOrEmpty(name) || name == Name) return;
        var old = Name;
        if (await C.Run(["block", "rename", old, name])) Name = name;
    }

    async Task Duplicate()
    {
        var name = await C.Dialogs.AskText("Duplicate block", "Name of the copy.");
        if (!string.IsNullOrEmpty(name)) await C.Run(["block", "duplicate", Name, name]);
    }

    async Task SaveTemplate()
    {
        var name = await C.Dialogs.AskText("Save as block preset", "A name for a copy of this block, to make new blocks from.", initial: Name);
        if (!string.IsNullOrEmpty(name)) await C.Run(["block-preset", "save", Name, name]);
    }

    /// <summary>§3.2: tick the accounts the block applies to; all (or none) means every account.</summary>
    async Task ChooseUsers()
    {
        if (block is null || state is null) return;
        var current = block.Users;
        var checks = state.Accounts.Select(a => new CheckBox { Content = a, IsChecked = current.Count == 0 || current.Contains(a) }.Named(a, "User_" + a)).ToList();
        var body = Ui.Column(6, Ui.Text("For some accounts only, its websites are blocked in their browsers by the extension and its apps closed in their sessions. Fewer accounts weakens the block, so it isn't possible while it is locked."));
        foreach (var c in checks) body.Children.Add(c);
        if (!await C.Dialogs.Ask($"Who does {block.Name} apply to?", body)) return;
        var chosen = checks.Where(c => c.IsChecked == true).Select(c => (string)c.Content).ToList();
        if (chosen.Count == 0 || state.Accounts.All(chosen.Contains)) await C.Run(["block", "users", block.Name, "all"]);
        else await C.Run(["block", "users", block.Name, .. chosen]);
    }

    /// <summary>§4.2: deleted, the page comes back to the block list; refused, the block stays open.</summary>
    async Task Delete()
    {
        var name = Name;
        if (await C.Dialogs.Confirm($"Delete {name}?", "The block and its lists are removed.", "Delete", destructive: true))
            await C.Run(["block", "remove", name], then: _ => { page.ShowCards(); return Task.CompletedTask; });
    }

    // Websites tab -----------------------------------------------------------

    async Task CreateList(string kind)
    {
        if (kind == "phrases")
        {
            await CreatePhraseSet();
            return;
        }
        var entry = new TextBox().Named("List name", "NewListName");
        var checks = new Dictionary<string, CheckBox>();
        var boxes = Ui.Wrap(12);
        IEnumerable<(string Key, string Title)> places = kind == "exceptions" ? Tables.ListPlaces : [.. Tables.ListPlaces, Tables.AdPlace];
        foreach (var (key, label) in places)
        {
            checks[key] = new CheckBox { Content = label, IsChecked = key == "url", MinWidth = 0 }.Named(label, "NewListWhere_" + key);
            boxes.Children.Add(checks[key]);
        }
        var body = Ui.Column(10, Ui.Text("A name, and where it looks. Sites (reddit.com), URL entries and patterns (*casino*) match the URL; words (long cat) match words in the URL, title or text. One list can hold both."), entry, boxes);
        bool Valid() => entry.Text.Trim().Length > 0 && checks.Values.Any(c => c.IsChecked == true);
        if (!await C.Dialogs.Ask(kind == "exceptions" ? "New exception list" : "New blacklist", body, "Create", Valid, w =>
            {
                entry.TextChanged += (_, _) => w();
                foreach (var c in checks.Values) c.Click += (_, _) => w();
            })) return;
        List<string> args = ["list", "create", Name, entry.Text.Trim(), .. checks.Where(c => c.Value.IsChecked == true).Select(c => "--" + c.Key)];
        if (kind == "exceptions") args.Add("--exceptions");
        await C.Run(args);
    }

    async Task CreatePhraseSet()
    {
        var entry = new TextBox { PlaceholderText = "optional" }.Named("Phrase set name", "NewPhraseSetName");
        var checks = Tables.ListPlaces.ToDictionary(p => p.Key, p => new CheckBox { Content = p.Title, IsChecked = p.Key is "title" or "text", MinWidth = 0 }.Named(p.Title, "NewPhraseWhere_" + p.Key));
        var body = Ui.Column(10, Ui.Text("A name, and where its phrases are looked for."), entry, Ui.Wrap(12, [.. checks.Values]));
        if (!await C.Dialogs.Ask("New phrase set", body, "Create")) return;
        var name = entry.Text.Trim();
        if (name.Length == 0)
        {
            // §4.3: an unnamed phrase set gets a name.
            var taken = (state?.FindBlock(Name)?.PhraseSets ?? []).Select(p => p.Name.ToLowerInvariant()).ToHashSet();
            var n = 1;
            while (taken.Contains($"phrase set {n}")) n++;
            name = $"Phrase set {n}";
        }
        await C.Run(["phrase-set", "create", Name, name, .. checks.Where(c => c.Value.IsChecked == true).Select(c => "--" + c.Key)]);
    }

    /// <summary>Saves a copy as a preset: lists (which, with actions or not), or automation.</summary>
    public async Task SavePreset(string kind)
    {
        var entry = new TextBox { PlaceholderText = "A name" }.Named("Preset name", "PresetName");
        var body = Ui.Column(10, Ui.Text($"A copy of the lists of {Name}, to copy into other blocks."), entry);
        var which = new RadioButtons { Header = "Which lists", SelectedIndex = 0 }.Named("Which lists", "PresetWhich");
        which.Items.Add("Both");
        which.Items.Add("Blacklists");
        which.Items.Add("Exceptions");
        var withActions = new CheckBox { Content = "Include actions?" }.Named("Include actions", "PresetActions");
        body.Children.Add(which);
        body.Children.Add(withActions);
        if (!await C.Dialogs.Ask("Save as preset", body, "Save", () => entry.Text.Trim().Length > 0, w => entry.TextChanged += (_, _) => w())) return;
        List<string> args = ["preset", "save", kind, Name, entry.Text.Trim()];
        if (which.SelectedIndex == 1) args.Add("--blacklists");
        if (which.SelectedIndex == 2) args.Add("--exceptions");
        if (withActions.IsChecked == true) args.Add("--actions");
        await C.Run(args, then: o => C.Dialogs.Message(o, "Saved"));
    }

    /// <summary>Uses a list preset on the opened block (copied in; what would
    /// weaken a locked block is refused).</summary>
    async Task FromPreset(string kind)
    {
        var names = (state?.Presets ?? []).Where(p => p.Kind == kind).Select(p => (p.Name, $"{p.Name} ({p.Kind})")).ToList();
        if (names.Count == 0)
        {
            await C.Dialogs.Message("No such presets yet: save one first.", "No presets");
            return;
        }
        var chosen = await C.Dialogs.Choose("Use a preset", "It is copied in; what would weaken a locked block is refused.", names);
        if (chosen is not null) await C.Run(["preset", "use", chosen, Name, "--kind", kind], then: o => C.Dialogs.Message(o, "Done"));
    }

    // Apps tab -----------------------------------------------------------------

    async Task PickApps()
    {
        var chosen = await AppPicker.Pick(window);
        if (chosen.Count > 0) await Cmd("add-app", chosen);
    }

    async Task PickFolder()
    {
        var path = await Files.Folder(window);
        if (path is not null) await Cmd("add-app", [path.TrimEnd('\\') + "\\"]);
    }

    async Task PickFile()
    {
        var path = await Files.Open(window, ".exe");
        if (path is not null) await Cmd("add-app", [path]);
    }

    async Task ImportFile(List<string> command)
    {
        var path = await Files.Open(window, ".txt");
        if (path is not null) await C.Run([.. command, path]);
    }

    // Breaks & allowance tab -------------------------------------------------

    async Task<bool> FieldsOk(params (string Name, CheckedField? Field)[] fields)
    {
        foreach (var (name, f) in fields)
            if (f is not null && !f.IsValid)
            {
                await C.Dialogs.Message($"{name}: {f.Problem}", "Not saved");
                return false;
            }
        return true;
    }

    async Task SetQuota()
    {
        if (!await FieldsOk(("Allowance", quotaTime), ("Sessions", quotaSessions), ("Refill", quotaRefill))) return;
        await C.Run(["allowance", "set", Name, quotaTime.Arg(), "--per", Key(quotaPer), "--refill", Clock.Text(quotaRefill.Minute ?? 0),
            "--sessions", (quotaSessions.Value ?? 0).ToString(CultureInfo.InvariantCulture)]);
    }

    async Task SetPomodoro()
    {
        var longOn = longCheck.IsChecked == true;
        if (!await FieldsOk(("Work", workTime), ("Break", breakTime), ("Long break", longOn ? longTime : null), ("Work periods", longOn ? longEvery : null))) return;
        List<string> args = ["pomodoro", "set", Name, workTime.Arg(), breakTime.Arg()];
        if (longOn) args.AddRange(["--long", longTime.Arg(), "--every", (longEvery.Value ?? 4).ToString(CultureInfo.InvariantCulture)]);
        await C.Run(args);
    }

    async Task SetReward()
    {
        var keep = Key(rewardKeep);
        var kept = keep == "time" ? rewardKeepTime : null;
        if (!await FieldsOk(("Break", rewardTime), ("Blocking", rewardPer), ("Kept for", kept))) return;
        await C.Run(["break", "reward", Name, rewardTime.Arg(), "--per", rewardPer.Arg(), "--keep", kept is not null ? kept.Arg() : keep]);
    }

    async Task SetRelock()
    {
        var on = relockCheck.IsChecked == true;
        if (!await FieldsOk(("Text break", on ? relockTime : null))) return;
        await C.Run(["break", "text", Name, on ? relockTime.Arg() : "off"]);
    }

    async Task SetDelayBreak()
    {
        if (delayBreakCheck.IsChecked == true)
        {
            if (!await FieldsOk(("Delay break", delayBreakTime), ("Waiting", delayWaitTime))) return;
            await C.Run(["break", "delay", Name, delayBreakTime.Arg(), delayWaitTime.Arg()]);
        }
        else await C.Run(["break", "delay", Name, "off"]);
    }

    // Bypass prevention tab ---------------------------------------------------

    async Task ChangeNepsis()
    {
        if (block is null || state is null) return;
        var content = (block.Draft.Length > 0 ? state.FindBlock(block.Draft) : null) ?? block;
        var asked = await NepsisDialog.Ask(window, $"Bypass prevention for {block.Name}", content.Nepsis, allowLock: false);
        if (asked is null) return;
        var args = await NepsisDialog.AcknowledgeMonkSteps(window, asked.Value.Args);
        if (args is not null) await C.Run(["bypass", "set", "--block", block.Name, .. args]);
    }

    async Task NepsisOff()
    {
        if (await C.Dialogs.Confirm("Turn bypass prevention off?", "This weakens it, so it isn't possible while the block is locked.", "Turn off"))
            await C.Run(["bypass", "off", "--block", Name]);
    }
}
