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
        gui.Front();
        Keyboard.TypeSimultaneously(VirtualKeyShort.CONTROL, VirtualKeyShort.KEY_N);
        gui.FindName("New block");
        if (gui.ById("AskText") is null)
        {
            // The preset choice: Empty is chosen; Enter takes it.
            gui.Front();
            Keyboard.Press(VirtualKeyShort.RETURN);
            Wait.UntilInputIsProcessed();
        }
        // Typed as a person types: the box has the focus when the dialog opens.
        gui.Find("AskText");
        gui.Front();
        Keyboard.Type("Enter test");
        Wait.UntilInputIsProcessed();
        gui.WaitFor(() => gui.Find("AskText").AsTextBox().Text == "Enter test", "the name typed");
        Keyboard.Press(VirtualKeyShort.RETURN);
        try { gui.WaitFor(() => gui.Ran("block", "create", "Enter test"), "kc9k block create by Enter"); }
        finally { Console.WriteLine("dialog keys: " + string.Join("\n", GuiApp.LogLines("dialog"))); }
    }
}
