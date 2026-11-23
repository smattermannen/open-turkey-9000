using FlaUI.Core.AutomationElements;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Kc9k.UiTests;

// A15 (R-GUI-3): Enter presses a dialog's OK. New block goes through a
// choice (when block presets exist) and a text box, by Enter alone; the
// duration dialog is covered by SmokeTests.MonkModeIsConfirmedAsALock.
[Collection("gui")]
public class DialogKeyTests
{
    [Fact]
    public void EnterPressesOk()
    {
        using var gui = new GuiApp("enter");
        gui.Window.Focus();
        Keyboard.TypeSimultaneously(VirtualKeyShort.CONTROL, VirtualKeyShort.KEY_N);
        gui.FindName("New block");
        if (gui.ById("AskText") is null)
        {
            // The preset choice: Empty is chosen; Enter takes it.
            Keyboard.Press(VirtualKeyShort.RETURN);
            Wait.UntilInputIsProcessed();
        }
        var box = gui.Find("AskText").AsTextBox();
        box.Focus();
        box.Text = "Enter test";
        Keyboard.Press(VirtualKeyShort.RETURN);
        gui.WaitFor(() => gui.Ran("block", "create", "Enter test"), "kc9k block create by Enter");
    }
}
