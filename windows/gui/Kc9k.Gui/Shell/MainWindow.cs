// The window: the sidebar of pages (the GTK GUI's), each page under a
// cover that asks for its access password while it is closed (§3.11), a
// banner while Frozen Chicken runs, and a thin bar while a command runs.
using Kc9k.Core;
using Microsoft.UI;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;

namespace Kc9k.Gui;

public sealed partial class MainWindow : Window
{
    public Controller Controller { get; }
    readonly NavigationView nav;
    readonly Grid root;
    readonly ProgressBar busy;
    readonly InfoBar banner;
    readonly Dictionary<string, (IPage Page, Grid Holder, Grid Cover, TextBlock CoverText, PasswordBox Entry)> pages = [];
    readonly Dictionary<string, NavigationViewItem> items = [];
    readonly NavigationViewItem lockItem;
    readonly Tray? tray;
    bool quitting;

    public string CurrentPage { get; private set; } = "dashboard";
    public BlocksPage Blocks { get; }

    public MainWindow()
    {
        Title = "Killer Chicken 9000";
        root = new Grid();
        root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(40) });
        root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        Content = root;

        var dialogs = new Dialogs(() => root.XamlRoot, () => root.ActualTheme);
        Controller = new Controller(DispatcherQueue, dialogs);

        // The title bar: the app's icon and name, drawn by us (Mica behind it).
        var titleBar = new Grid { Padding = new Thickness(16, 0, 0, 0) };
        var icon = new Image { Width = 18, Height = 18, Source = new Microsoft.UI.Xaml.Media.Imaging.BitmapImage(new Uri(Path.Combine(AppContext.BaseDirectory, "Assets", "kc9k.png"))) };
        titleBar.Children.Add(Ui.Row(10, icon, Ui.Text("Killer Chicken 9000", "CaptionTextBlockStyle", wrap: false)));
        root.Children.Add(titleBar);
        ExtendsContentIntoTitleBar = true;
        SetTitleBar(titleBar);
        AppWindow.SetIcon(Path.Combine(AppContext.BaseDirectory, "Assets", "kc9k.ico"));
        AppWindow.TitleBar.PreferredHeightOption = TitleBarHeightOption.Tall;

        busy = new ProgressBar { IsIndeterminate = true, Visibility = Visibility.Collapsed, Height = 3 }.Named("Working", "Busy");
        banner = new InfoBar { Severity = InfoBarSeverity.Warning, IsClosable = false, IsOpen = false, Margin = new Thickness(12, 0, 12, 4) }.Named("Banner", "Banner");
        var top = Ui.Column(0, busy, banner);
        Grid.SetRow(top, 1);
        root.Children.Add(top);

        // W53: the sidebar is pinned open like the GTK GUI's: no ☰, it never
        // moves or covers the page, and the page fills the rest. Only a window
        // too narrow for both shows the sidebar's icons alone (still pinned).
        nav = new NavigationView
        {
            PaneDisplayMode = NavigationViewPaneDisplayMode.Left,
            IsBackButtonVisible = NavigationViewBackButtonVisible.Collapsed,
            IsSettingsVisible = false,
            IsPaneToggleButtonVisible = false,
            IsPaneOpen = true,
            OpenPaneLength = SidebarWidth,
        };
        nav.Named("Pages", "Nav");
        nav.PaneClosing += (_, e) => { if (nav.PaneDisplayMode == NavigationViewPaneDisplayMode.Left) e.Cancel = true; };
        root.SizeChanged += (_, e) => PinSidebar(e.NewSize.Width);
        Grid.SetRow(nav, 2);
        root.Children.Add(nav);

        Blocks = new BlocksPage(this);
        IPage[] all =
        [
            new OverviewPage(this), Blocks, new SchedulesPage(this), new PlacesPage(this), new HotkeysPage(this),
            new PresetsPage(this), new StatsPage(this), new CircadianPage(this), new SettingsPage(this),
        ];
        var glyphs = new Dictionary<string, string>
        {
            ["dashboard"] = Glyphs.Home, ["blocks"] = Glyphs.Blocks, ["schedules"] = Glyphs.Calendar, ["places"] = Glyphs.Place,
            ["hotkeys"] = Glyphs.Keyboard, ["clusters"] = Glyphs.Presets, ["stats"] = Glyphs.Chart, ["circadian"] = Glyphs.Moon,
            ["settings"] = Glyphs.Settings,
        };
        foreach (var ((key, title), page) in Tables.Pages.Zip(all))
        {
            var item = new NavigationViewItem { Content = title, Icon = new FontIcon { Glyph = glyphs[key] }, Tag = key };
            item.Named(title, "Nav_" + key);
            if (key == "settings") nav.FooterMenuItems.Add(item);
            else nav.MenuItems.Add(item);
            items[key] = item;
            pages[key] = Guard(key, title, page);
        }
        lockItem = new NavigationViewItem
        {
            Content = "Lock", Icon = new FontIcon { Glyph = Glyphs.Lock }, SelectsOnInvoked = false,
            Visibility = Visibility.Collapsed,
        };
        lockItem.Named("Lock: forget the passwords given", "LockAccess");
        ToolTipService.SetToolTip(lockItem, "Forget the passwords given and lock the pages again");
        nav.FooterMenuItems.Insert(0, lockItem);
        nav.SelectionChanged += (_, e) =>
        {
            if (e.SelectedItem is NavigationViewItem { Tag: string key }) Show(key);
        };
        nav.ItemInvoked += (_, e) =>
        {
            if (ReferenceEquals(e.InvokedItemContainer, lockItem)) Controller.LockAccess();
            // §4.2: Blocks in the sidebar opens the block list, also from a block.
            else if (e.InvokedItemContainer is NavigationViewItem { Tag: "blocks" } && CurrentPage == "blocks") Blocks.ShowCards();
        };

        Controller.StateChanged += Apply;
        // W52: only the page shown is updated; another is when it is shown.
        Controller.StatsChanged += stats =>
        {
            if (!Controller.Locked(Area(CurrentPage)) && !Controller.Locked("app")) pages[CurrentPage].Page.UpdateStats(stats);
        };
        Controller.BusyChanged += on =>
        {
            busy.Visibility = on ? Visibility.Visible : Visibility.Collapsed;
            AutomationProperties.SetItemStatus(nav, on ? "Working" : "");
        };
        Controller.Banner += text =>
        {
            banner.Message = text ?? "";
            banner.IsOpen = text is not null;
        };
        Controller.ExtensionMissing += () => _ = ExtensionDialog();
        Controller.OpenedBlock = () => CurrentPage == "blocks" ? Blocks.OpenedBlock : null;
        root.AddHandler(UIElement.PointerPressedEvent, new PointerEventHandler((_, _) => Controller.NoteInput()), true);
        root.AddHandler(UIElement.KeyDownEvent, new KeyEventHandler((_, _) => Controller.NoteInput()), true);

        ApplyTheme();
        RestoreSize();
        tray = Tray.TryCreate(this);
        AppWindow.Closing += (_, e) =>
        {
            SaveSize();
            // §4.10 tray: closing hides the window while the tray icon is on.
            if (!quitting && tray is not null && Prefs.Current.Tray)
            {
                e.Cancel = true;
                AppWindow.Hide();
            }
        };
        Closed += (_, _) =>
        {
            Controller.Stop();
            tray?.Dispose();
        };

        AddShortcuts();
        var first = Prefs.Current.OpenOn == "last" && items.ContainsKey(Prefs.Current.LastPage) ? Prefs.Current.LastPage : "dashboard";
        nav.SelectedItem = items[first];
        Controller.Start();
    }

    /// <summary>The sidebar's width (W53), and the narrowest page beside it.</summary>
    public const double SidebarWidth = 220, NarrowestPage = 420;

    void PinSidebar(double width)
    {
        var mode = width >= SidebarWidth + NarrowestPage ? NavigationViewPaneDisplayMode.Left : NavigationViewPaneDisplayMode.LeftCompact;
        if (nav.PaneDisplayMode == mode) return;
        nav.PaneDisplayMode = mode;
        nav.IsPaneOpen = mode == NavigationViewPaneDisplayMode.Left;
    }

    /// <summary>The shortcuts (W34), listed under Settings › Keyboard.</summary>
    public static readonly (string Keys, string What)[] Shortcuts =
    [
        ("Ctrl+1 … Ctrl+9", "Open the pages of the sidebar, in order"),
        ("Ctrl+N", "New block"),
        ("F5", "Read the state again"),
        ("Alt+Left", "Back: from a block, a mode or an automation to the list"),
        ("Ctrl+F", "Find in the list that has the focus"),
        ("Enter, Delete", "Add or remove an entry in a list"),
        ("Shift+F10", "A card's menu (put a block into a mode)"),
    ];

    void AddShortcuts()
    {
        void Add(Windows.System.VirtualKey key, Windows.System.VirtualKeyModifiers mods, Action act)
        {
            var a = new KeyboardAccelerator { Key = key, Modifiers = mods };
            a.Invoked += (_, e) => { e.Handled = true; act(); };
            root.KeyboardAccelerators.Add(a);
        }
        root.KeyboardAcceleratorPlacementMode = KeyboardAcceleratorPlacementMode.Hidden;
        for (var i = 0; i < Tables.Pages.Length; i++)
        {
            var key = Tables.Pages[i].Key;
            Add(Windows.System.VirtualKey.Number1 + i, Windows.System.VirtualKeyModifiers.Control, () => Show(key));
        }
        Add(Windows.System.VirtualKey.N, Windows.System.VirtualKeyModifiers.Control, () => _ = Actions.CreateBlock(this));
        Add(Windows.System.VirtualKey.F5, Windows.System.VirtualKeyModifiers.None, () => _ = Controller.ReadAgain());
        Add(Windows.System.VirtualKey.Left, Windows.System.VirtualKeyModifiers.Menu, Back);
        Add(Windows.System.VirtualKey.GoBack, Windows.System.VirtualKeyModifiers.None, Back);
        // The mouse's back button.
        root.AddHandler(UIElement.PointerPressedEvent, new PointerEventHandler((_, e) =>
        {
            if (e.GetCurrentPoint(root).Properties.IsXButton1Pressed) Back();
        }), true);
    }

    /// <summary>Back from a block, a mode or an automation to the cards.</summary>
    public void Back()
    {
        if (CurrentPage == "blocks" && Blocks.CurrentView != "cards") Blocks.ShowCards();
    }

    static string Area(string page) => page switch
    {
        "clusters" or "schedules" or "places" => "blocks",
        "circadian" or "hotkeys" => "settings",
        _ => page,
    };

    (IPage, Grid, Grid, TextBlock, PasswordBox) Guard(string key, string title, IPage page)
    {
        var holder = new Grid();
        holder.Children.Add(page.View);
        var text = Ui.Text("", "BodyStrongTextBlockStyle");
        text.TextAlignment = TextAlignment.Center;
        var entry = new PasswordBox { Width = 260, PlaceholderText = "Password" }.Named("Access password", "CoverPassword_" + key);
        var unlock = Ui.Button("Unlock", () => _ = UnlockFrom(entry), "CoverUnlock_" + key, accent: true);
        entry.KeyDown += (_, e) =>
        {
            if (e.Key == Windows.System.VirtualKey.Enter) _ = UnlockFrom(entry);
        };
        var cover = new Grid { Visibility = Visibility.Collapsed };
        var inner = Ui.Column(14, Ui.Icon(Glyphs.Lock, 40), text, Ui.Row(8, entry, unlock));
        inner.HorizontalAlignment = HorizontalAlignment.Center;
        inner.VerticalAlignment = VerticalAlignment.Center;
        foreach (var c in inner.Children) if (c is FrameworkElement f) f.HorizontalAlignment = HorizontalAlignment.Center;
        cover.Children.Add(inner);
        holder.Children.Add(cover);
        return (page, holder, cover, text, entry);
    }

    async Task UnlockFrom(PasswordBox entry)
    {
        var password = entry.Password;
        entry.Password = "";
        if (password.Length > 0 && !await Controller.TryAccess(password))
        {
            await Controller.Dialogs.Message("That password opens nothing.", "Not opened");
            entry.Focus(FocusState.Programmatic);
        }
    }

    void Apply(GuiState state)
    {
        var appLocked = Controller.Locked("app");
        foreach (var (key, p) in pages)
        {
            var locked = Controller.Locked(Area(key)) || appLocked;
            var title = Tables.Pages.First(x => x.Key == key).Title;
            p.CoverText.Text = appLocked ? "Killer Chicken 9000 is locked.\nEnter its password to open it."
                : $"{title} is locked.\nEnter its password to open it.";
            p.Cover.Visibility = locked ? Visibility.Visible : Visibility.Collapsed;
            p.Page.View.Visibility = locked ? Visibility.Collapsed : Visibility.Visible;
            if (!locked && key == CurrentPage) p.Page.Update(state);
        }
        lockItem.Visibility = Controller.Session.AccessPasswords.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        tray?.SetStatus(state);
    }

    void Show(string key)
    {
        CurrentPage = key;
        Prefs.Current.LastPage = key;
        if (items.TryGetValue(key, out var item) && !ReferenceEquals(nav.SelectedItem, item)) nav.SelectedItem = item;
        nav.Content = pages[key].Holder;
        if (Controller.Locked(Area(key)) || Controller.Locked("app")) return;
        if (Controller.State is { } s) pages[key].Page.Update(s);
        if (Controller.Stats is { } stats) pages[key].Page.UpdateStats(stats);
    }

    /// <summary>Opens a page of the sidebar.</summary>
    public void Navigate(string key) => Show(key);

    public void OpenBlock(string name, string? tab = null)
    {
        Show("blocks");
        Blocks.OpenBlock(name, tab);
    }

    /// <summary>A schedule on the Schedules page (§3.6).</summary>
    public void OpenSchedule(string name)
    {
        Show("schedules");
        (pages["schedules"].Page as SchedulesPage)?.Open(name);
    }

    public void OpenMode(string name)
    {
        Show("blocks");
        Blocks.OpenMode(name);
    }

    /// <summary>What a start asked for (W36): a page, or one of the GUI's own dialogs.</summary>
    public async Task Perform(StartArgs asked)
    {
        if (asked.Page is { } page) Show(page);
        switch (asked.Do)
        {
            case "new-block": await Actions.CreateBlock(this); break;
            case "monk": await Actions.MonkMode(this); break;
            case "freeze": OpenMode(Tables.FrozenChicken); break;
        }
    }

    /// <summary>Settings, at its browsers (W38).</summary>
    public void OpenBrowsers()
    {
        Show("settings");
        (pages["settings"].Page as SettingsPage)?.ShowBrowsers();
    }

    /// <summary>Settings, with the guided setup's checklist open (W38).</summary>
    public async Task OpenGuidedSetup()
    {
        Show("settings");
        if (pages["settings"].Page is SettingsPage settings) await settings.ShowGuidedSetup();
    }

    async Task ExtensionDialog()
    {
        var state = Controller.State;
        if (state is null) return;
        var none = !state.Browsers.Any(b => b.Installed);
        var names = state.ExtensionMissingIn;
        var text = none
            ? "No supported browser is installed. Install Google Chrome, Microsoft Edge or LibreWolf, then open it once."
            : $"{(names.Count > 0 ? string.Join(" and ", names) : "A supported browser")} has run for a while without it checking in, and your blocks need it. Close and open the browser again, or see under Settings > Browsers which have it.";
        if (await Controller.Dialogs.Confirm("The Killer Chicken extension isn't running", text, "Show browsers", cancel: "Later"))
        {
            Show("settings");
            (pages["settings"].Page as SettingsPage)?.ShowBrowsers();
        }
    }

    public void ApplyTheme()
    {
        var prefs = Prefs.Current;
        root.RequestedTheme = Theme.Requested(prefs.Theme);
        var dark = prefs.Theme == "dark";
        if (dark)
        {
            // §4.10: the darkness slider, dark grey to black; Mica can't be that dark.
            SystemBackdrop = null;
            root.Background = Theme.Solid(Theme.DarkBackground(prefs.Darkness));
        }
        else if (Microsoft.UI.Composition.SystemBackdrops.MicaController.IsSupported())
        {
            SystemBackdrop = new MicaBackdrop();
            root.Background = new SolidColorBrush(Colors.Transparent);
        }
        else
        {
            SystemBackdrop = null;
            root.Background = Theme.Brush("SolidBackgroundFillColorBaseBrush");
        }
        AppWindow.TitleBar.ButtonForegroundColor = root.ActualTheme == ElementTheme.Dark || dark ? Colors.White : Colors.Black;
        AppWindow.TitleBar.ButtonBackgroundColor = Colors.Transparent;
        AppWindow.TitleBar.ButtonInactiveBackgroundColor = Colors.Transparent;
    }

    // W37: size, place and maximized kept per user; a place on a screen that
    // is gone (a second screen unplugged) falls back to the middle of the
    // primary screen's work area.
    void RestoreSize()
    {
        var p = Prefs.Current;
        DisplayArea? display = null;
        if (p.WindowX is int x && p.WindowY is int y)
        {
            var at = DisplayArea.GetFromPoint(new Windows.Graphics.PointInt32(x + 40, y + 20), DisplayAreaFallback.None);
            if (at is not null && Contains(at.WorkArea, x + 40, y + 20)) display = at;
        }
        var placed = display is not null;
        display ??= DisplayArea.GetFromWindowId(AppWindow.Id, DisplayAreaFallback.Primary);
        var work = display.WorkArea;
        int w = Math.Max(Math.Min(p.WindowWidth, work.Width), 640), h = Math.Max(Math.Min(p.WindowHeight, work.Height), 480);
        AppWindow.Resize(new Windows.Graphics.SizeInt32(w, h));
        if (placed)
            AppWindow.Move(new Windows.Graphics.PointInt32(
                Math.Clamp(p.WindowX!.Value, work.X, work.X + work.Width - Math.Min(w, work.Width)),
                Math.Clamp(p.WindowY!.Value, work.Y, work.Y + work.Height - Math.Min(h, work.Height))));
        else
            AppWindow.Move(new Windows.Graphics.PointInt32(work.X + (work.Width - w) / 2, work.Y + (work.Height - h) / 2));
        if (p.WindowMaximized && AppWindow.Presenter is OverlappedPresenter o) o.Maximize();
    }

    static bool Contains(Windows.Graphics.RectInt32 r, int x, int y) => x >= r.X && y >= r.Y && x < r.X + r.Width && y < r.Y + r.Height;

    void SaveSize()
    {
        var p = Prefs.Current;
        switch (AppWindow.Presenter)
        {
            case OverlappedPresenter { State: OverlappedPresenterState.Maximized }:
                p.WindowMaximized = true;
                break;
            case OverlappedPresenter { State: OverlappedPresenterState.Minimized }:
                break; // keep what was there before
            default:
                p.WindowMaximized = false;
                p.WindowWidth = AppWindow.Size.Width;
                p.WindowHeight = AppWindow.Size.Height;
                p.WindowX = AppWindow.Position.X;
                p.WindowY = AppWindow.Position.Y;
                break;
        }
        p.Save();
    }

    public void ShowFromTray()
    {
        AppWindow.Show();
        Activate();
        if (AppWindow.Presenter is OverlappedPresenter { State: OverlappedPresenterState.Minimized } o) o.Restore();
    }

    public void StartInTray() => AppWindow.Hide();

    public void Quit()
    {
        quitting = true;
        Close();
        Application.Current.Exit();
    }
}
