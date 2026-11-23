// §4.3 list editing (the GTK GUI's EditableList): type, Enter (focus stays);
// remove with the button or Delete/Backspace (the selection moves to the
// next); Ctrl/Shift-click and Ctrl+A select several; double-click moves an
// entry into the box to edit it (removed, then added again: R-WEAK-5);
// Ctrl+F finds. Lists of words for adult sites can be covered until clicked.
// A grip under the list makes it taller; the height is kept per kind of list.
using Kc9k.Core;
using Microsoft.UI.Input;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Controls.Primitives;
using Microsoft.UI.Xaml.Input;
using Windows.System;

namespace Kc9k.Gui;

public sealed partial class EditableList : StackPanel
{
    readonly Func<IReadOnlyList<string>, Task> onAdd;
    readonly Func<string, int, Task> onRemove;
    Func<string, List<string>> split;
    readonly ListView view;
    readonly TextBox? search;
    readonly TextBox entry;
    readonly Button cover;
    readonly Grid listHolder;
    readonly bool hidden, multi;
    readonly string sizeKey;
    List<string> items = [];
    string filter = "";
    string? owner;
    bool revealed;
    int? nextIndex;

    public string Title { get; }
    public IReadOnlyList<string> Items => items;
    /// <summary>The entry selected (the first of several), or null.</summary>
    /// <summary>How typed text becomes entries (a phrase preset takes one phrase per line).</summary>
    public Func<string, List<string>> Splitter { get => split; set => split = value; }
    public string Placeholder { get => entry.PlaceholderText; set => entry.PlaceholderText = value; }

    public string? SelectedEntry => view.SelectedItem as string;
    public event Action<string>? Activated;

    public EditableList(string title, string placeholder, Func<IReadOnlyList<string>, Task> onAdd, Func<string, int, Task> onRemove,
        Func<Task>? onImport = null, Func<Task>? onExport = null, string help = "", bool searchBox = false, bool hidden = false,
        Func<string, List<string>>? split = null, bool entryBox = true, bool multi = false, string? sizeKey = null,
        string? id = null, bool showTitle = true)
    {
        Spacing = 6;
        Title = title;
        this.onAdd = onAdd;
        this.onRemove = onRemove;
        this.split = split ?? Entries.Split;
        this.hidden = hidden;
        this.multi = multi;
        this.sizeKey = sizeKey ?? (title.Length > 0 ? title : placeholder);
        id ??= "List_" + new string(title.Where(char.IsLetterOrDigit).ToArray());
        if (showTitle && title.Length > 0) Children.Add(Ui.Heading(title));
        if (help.Length > 0) Children.Add(Ui.Dim(help));

        if (searchBox)
        {
            search = Dialogs.KeepsEnter(new TextBox { PlaceholderText = "Find in this list" }.Named($"Find in {title}", id + "_Find"));
            search.TextChanged += (_, _) => { filter = search.Text.Trim().ToLowerInvariant(); Fill(); };
            Children.Add(search);
        }

        view = new ListView
        {
            SelectionMode = multi ? ListViewSelectionMode.Extended : ListViewSelectionMode.Single,
            IsMultiSelectCheckBoxEnabled = false,
            BorderThickness = new Thickness(1),
            BorderBrush = Theme.Brush("CardStrokeColorDefaultBrush"),
            CornerRadius = new CornerRadius(4),
        };
        view.Named(title.Length > 0 ? title : placeholder, id);
        view.KeyDown += Key;
        view.DoubleTapped += (_, _) =>
        {
            if (view.SelectedItem is string v) _ = Edit(v);
        };
        cover = new Button { HorizontalAlignment = HorizontalAlignment.Stretch, VerticalAlignment = VerticalAlignment.Stretch };
        cover.Named($"{title}: hidden entries, click to show", id + "_Cover");
        cover.Click += (_, _) => Reveal();
        listHolder = new Grid { Height = Prefs.Current.ListHeights.GetValueOrDefault(this.sizeKey, UsualHeight) };
        live.Add(new WeakReference<EditableList>(this));
        // Shown again (another page meanwhile): the height kept for its kind.
        Loaded += (_, _) => listHolder.Height = Prefs.Current.ListHeights.GetValueOrDefault(this.sizeKey, UsualHeight);
        listHolder.Children.Add(view);
        listHolder.Children.Add(cover);
        Children.Add(listHolder);
        Children.Add(Grip());

        entry = new TextBox { PlaceholderText = placeholder, HorizontalAlignment = HorizontalAlignment.Stretch };
        entry.Named($"Add to {title}", id + "_Entry");
        Dialogs.KeepsEnter(entry);
        Dialogs.KeepsEnter(view);
        entry.KeyDown += (_, e) =>
        {
            if (e.Key == VirtualKey.Enter)
            {
                e.Handled = true;
                _ = Add();
            }
        };
        var row = new Grid { ColumnSpacing = 6 };
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        var buttons = Ui.Row(6);
        if (entryBox)
        {
            row.Children.Add(entry);
            buttons.Children.Add(Ui.Button("Add", () => _ = Add(), id + "_Add"));
        }
        buttons.Children.Add(Ui.Button("Remove", () => _ = Remove(), id + "_Remove"));
        if (onImport is not null || onExport is not null)
        {
            var menu = new MenuFlyout();
            if (onImport is not null)
            {
                var i = new MenuFlyoutItem { Text = "Import…", Icon = new FontIcon { Glyph = Glyphs.Import } };
                i.Click += (_, _) => _ = onImport();
                menu.Items.Add(i);
            }
            if (onExport is not null)
            {
                var x = new MenuFlyoutItem { Text = "Export…", Icon = new FontIcon { Glyph = Glyphs.Export } };
                x.Click += (_, _) => _ = onExport();
                menu.Items.Add(x);
            }
            var files = new Button { Content = new FontIcon { Glyph = Glyphs.More, FontSize = 14 }, Flyout = menu };
            files.Named($"Import or export {title}", id + "_Files").Tip("Import or export this list");
            buttons.Children.Add(files);
        }
        row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        Grid.SetColumn(buttons, 1);
        row.Children.Add(buttons);
        Children.Add(row);
        Conceal();
    }

    /// <summary>The usual height of a list, and the least a drag leaves (GTK ResizeGrip.MIN).</summary>
    public const double UsualHeight = 150, LeastHeight = 80, MostHeight = 1400;

    /// <summary>Lists made, by kind: dragging one sizes those on screen at
    /// once (GTK ResizeGrip.live), the others when shown; a list let go is dropped.</summary>
    static readonly List<WeakReference<EditableList>> live = [];
    string GripId => AutomationProperties.GetAutomationId(view) + "_Grip";

    void SetHeight(double height, bool keep)
    {
        height = Math.Clamp(height, LeastHeight, MostHeight);
        live.RemoveAll(w => !w.TryGetTarget(out _));
        foreach (var w in live)
            if (w.TryGetTarget(out var l) && l.sizeKey == sizeKey) l.listHolder.Height = height;
        listHolder.Height = height;
        if (keep) Keep();
    }

    /// <summary>Drag down to make the list taller, up shorter; Up/Down keys
    /// on it too; double-click: back to the usual height, forgotten.</summary>
    UIElement Grip()
    {
        var grip = new Button
        {
            Height = 12, Width = 56, Padding = new Thickness(0), MinHeight = 0, MinWidth = 0,
            HorizontalAlignment = HorizontalAlignment.Center, Margin = new Thickness(0, 2, 0, 2),
            Content = new Border { Height = 4, Width = 40, CornerRadius = new CornerRadius(2), Background = Theme.Brush("ControlStrongFillColorDisabledBrush") },
        };
        grip.Named($"Resize {(Title.Length > 0 ? Title : AutomationProperties.GetName(view))}", GripId)
            .Tip("Drag to make the list taller; double-click for the usual height");
        AutomationProperties.SetHelpText(grip, "Up and Down arrows change the height; Enter puts the usual height back");
        double? startY = null, startHeight = null;
        grip.AddHandler(PointerPressedEvent, new PointerEventHandler((_, e) =>
        {
            startY = e.GetCurrentPoint(this).Position.Y;
            startHeight = listHolder.Height;
            grip.CapturePointer(e.Pointer);
        }), true);
        grip.AddHandler(PointerMovedEvent, new PointerEventHandler((_, e) =>
        {
            if (startY is null) return;
            SetHeight(startHeight!.Value + e.GetCurrentPoint(this).Position.Y - startY.Value, keep: false);
        }), true);
        grip.AddHandler(PointerReleasedEvent, new PointerEventHandler((_, e) =>
        {
            if (startY is null) return;
            startY = null;
            grip.ReleasePointerCapture(e.Pointer);
            // Only a drag that changed the height is kept: a click (or a
            // double-click's second release) leaves gui.json as it is.
            if (Math.Abs(listHolder.Height - startHeight!.Value) >= 1) Keep();
        }), true);
        // A button takes the clicks for itself: the double-click is heard anyway.
        grip.AddHandler(DoubleTappedEvent, new DoubleTappedEventHandler((_, _) => Usual()), true);
        grip.Click += (_, _) => { if (startY is null && grip.FocusState == FocusState.Keyboard) Usual(); };
        grip.KeyDown += (_, e) =>
        {
            if (e.Key is VirtualKey.Up or VirtualKey.Down)
            {
                SetHeight(listHolder.Height + (e.Key == VirtualKey.Down ? 40 : -40), keep: true);
                e.Handled = true;
            }
        };
        grip.PointerEntered += (_, _) => ProtectedCursor = InputSystemCursor.Create(InputSystemCursorShape.SizeNorthSouth);
        grip.PointerExited += (_, _) => ProtectedCursor = null;
        return grip;
    }

    /// <summary>Back to the usual height for every list of the kind, and
    /// forgotten in gui.json (GTK: double-click).</summary>
    void Usual()
    {
        SetHeight(UsualHeight, keep: false);
        if (Prefs.Current.ListHeights.Remove(sizeKey)) Prefs.Current.Save();
    }

    void Keep()
    {
        Prefs.Current.ListHeights[sizeKey] = listHolder.Height;
        Prefs.Current.Save();
    }

    /// <summary>Covers the entries again (when the list hides them and has any).</summary>
    public void Conceal()
    {
        revealed = !hidden;
        ShowCover();
    }

    public void Reveal()
    {
        revealed = true;
        ShowCover();
        view.Focus(FocusState.Programmatic);
    }

    void ShowCover()
    {
        var covered = !revealed && items.Count > 0;
        cover.Content = Ui.Column(2, Ui.Strong($"{Words.Plural(items.Count, "entry", "entries")} hidden"), Ui.Note("Click to show"));
        if (cover.Content is StackPanel p) foreach (var c in p.Children) if (c is FrameworkElement f) f.HorizontalAlignment = HorizontalAlignment.Center;
        cover.Visibility = covered ? Visibility.Visible : Visibility.Collapsed;
        view.Visibility = covered ? Visibility.Collapsed : Visibility.Visible;
        if (search is not null) search.IsEnabled = !covered;
    }

    /// <summary>Shows items. A different owner (another block) covers them again.</summary>
    public void SetItems(IEnumerable<string> newItems, string? newOwner = null)
    {
        if (newOwner != owner)
        {
            owner = newOwner;
            if (search is not null) search.Text = "";
            Conceal();
        }
        var list = newItems.ToList();
        if (!list.SequenceEqual(items))
        {
            items = list;
            Fill();
        }
        ShowCover();
    }

    void Fill()
    {
        var selected = view.SelectedItems.OfType<string>().ToHashSet();
        view.Items.Clear();
        foreach (var item in items.Where(i => filter.Length == 0 || i.ToLowerInvariant().Contains(filter)))
            view.Items.Add(item);
        if (nextIndex is int n && view.Items.Count > 0)
        {
            nextIndex = null;
            view.SelectedIndex = Math.Min(n, view.Items.Count - 1);
        }
        else
            foreach (var item in view.Items.OfType<string>().Where(selected.Contains)) view.SelectedItems.Add(item);
        AutomationProperties.SetItemStatus(view, Words.Plural(items.Count, "entry", "entries"));
    }

    async Task Add()
    {
        var values = split(entry.Text);
        if (values.Count == 0) return;
        entry.Text = "";
        await onAdd(values);
        entry.Focus(FocusState.Programmatic); // §4.3: focus stays
    }

    /// <summary>The selected rows as (value, 1-based position in the whole list), last first.</summary>
    List<(string Value, int Index)> Selected() =>
        [.. view.SelectedItems.OfType<string>().Select(v => (v, items.IndexOf(v) + 1)).Where(x => x.Item2 > 0).OrderByDescending(x => x.Item2)];

    async Task Remove()
    {
        var chosen = Selected();
        if (chosen.Count == 0) return;
        nextIndex = chosen.Min(c => view.Items.IndexOf(c.Value));
        foreach (var (value, index) in multi ? chosen : chosen.Take(1)) await onRemove(value, index);
    }

    async Task Edit(string value)
    {
        if (Activated is not null)
        {
            Activated(value);
            return;
        }
        entry.Text = value;
        entry.Focus(FocusState.Programmatic);
        entry.SelectionStart = value.Length;
        await onRemove(value, items.IndexOf(value) + 1);
    }

    void Key(object sender, KeyRoutedEventArgs e)
    {
        var ctrl = InputKeyboardSource.GetKeyStateForCurrentThread(VirtualKey.Control).HasFlag(Windows.UI.Core.CoreVirtualKeyStates.Down);
        if (e.Key is VirtualKey.Delete or VirtualKey.Back)
        {
            e.Handled = true;
            _ = Remove();
        }
        else if (ctrl && e.Key == VirtualKey.F && search is not null)
        {
            e.Handled = true;
            search.Focus(FocusState.Keyboard); // §4.3 Find
        }
        else if (ctrl && e.Key == VirtualKey.A && multi)
        {
            e.Handled = true;
            view.SelectAll();
        }
        else if (e.Key == VirtualKey.Enter && view.SelectedItem is string v)
        {
            e.Handled = true;
            _ = Edit(v);
        }
    }

    public void SetEnabled(bool on)
    {
        entry.IsEnabled = on;
        view.IsEnabled = on;
    }
}
