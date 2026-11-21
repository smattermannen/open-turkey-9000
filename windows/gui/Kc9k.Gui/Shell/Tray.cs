// §4.10 tray: built in a later step (ROADMAP W4); until then there is none
// and closing the window ends the GUI.
using Kc9k.Core;

namespace Kc9k.Gui;

public sealed class Tray : IDisposable
{
    public static Tray? TryCreate(MainWindow window) => null;
    public void SetStatus(GuiState state) { }
    public void Dispose() { }
}
