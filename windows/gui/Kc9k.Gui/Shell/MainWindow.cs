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

        nav = new NavigationView
        {
            PaneDisplayMode = NavigationViewPaneDisplayMode.Left,
            IsBackButtonVisible = NavigationViewBackButtonVisible.Collapsed,
            IsSettingsVisible = false,
            IsPaneToggleButtonVisible = true,
            OpenPaneLength = 240,
            CompactModeThresholdWidth = 720,
            ExpandedModeThresholdWidth = 1000,
        };
        nav.Named("Pages", "Nav");
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
        Controller.StatsChanged += stats =>
        {
            foreach (var (key, p) in pages)
                if (!Controller.Locked(Area(key))) p.Page.UpdateStats(stats);
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

        nav.SelectedItem = items["dashboard"];
        Controller.Start();
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
            if (!locked) p.Page.Update(state);
        }
        lockItem.Visibility = Controller.Session.AccessPasswords.Count > 0 ? Visibility.Visible : Visibility.Collapsed;
        tray?.SetStatus(state);
    }

    void Show(string key)
    {
        CurrentPage = key;
        if (items.TryGetValue(key, out var item) && !ReferenceEquals(nav.SelectedItem, item)) nav.SelectedItem = item;
        nav.Content = pages[key].Holder;
        if (Controller.State is { } s && !Controller.Locked(Area(key))) pages[key].Page.Update(s);
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

    void RestoreSize()
    {
        var p = Prefs.Current;
        var scale = root.XamlRoot?.RasterizationScale ?? 1.0;
        var display = DisplayArea.GetFromWindowId(AppWindow.Id, DisplayAreaFallback.Primary);
        int w = Math.Min(p.WindowWidth, display.WorkArea.Width), h = Math.Min(p.WindowHeight, display.WorkArea.Height);
        AppWindow.Resize(new Windows.Graphics.SizeInt32(Math.Max(w, 640), Math.Max(h, 480)));
        AppWindow.Move(new Windows.Graphics.PointInt32(
            display.WorkArea.X + (display.WorkArea.Width - Math.Max(w, 640)) / 2,
            display.WorkArea.Y + (display.WorkArea.Height - Math.Max(h, 480)) / 2));
        if (p.WindowMaximized && AppWindow.Presenter is OverlappedPresenter o) o.Maximize();
        _ = scale;
    }

    void SaveSize()
    {
        var p = Prefs.Current;
        if (AppWindow.Presenter is OverlappedPresenter { State: OverlappedPresenterState.Maximized })
            p.WindowMaximized = true;
        else
        {
            p.WindowMaximized = false;
            p.WindowWidth = AppWindow.Size.Width;
            p.WindowHeight = AppWindow.Size.Height;
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
