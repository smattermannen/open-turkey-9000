// W55: Settings → Import… (the GTK GUI's import_project): the dry run is
// shown, with what is not used on Windows (W56), its lock is confirmed with
// the lock dialog (R-LOCK-12) and the import runs with --yes. The file comes
// from Files.ImportProjectHook instead of the file dialog.
using Xunit;

namespace Kc9k.UiTests;

[Collection("gui")]
public class ProjectImportTests
{
    const string File = @"C:\setups\linux-project.json";
    // Kc9k.Gui.Files.ImportProjectHook (this project doesn't reference the GUI's).
    const string Hook = "KC9K_GUI_IMPORT_FILE";

    const string DryRun = """
        Not used on Windows (made for Linux; kept as it is, so the project still works there):
          block 'linux-desk': app gnome-terminal-server (a Linux program)
        Would: set a lock: imported block 'linux-desk': schedule lock, while each window is open and 30 minutes before it
        Would: add block 'linux-desk' (2 lists, 5 apps, 1 schedules)
        """;

    static GuiApp Start(string name)
    {
        Environment.SetEnvironmentVariable(Hook, File);
        try
        {
            return new GuiApp(name, prepare: dir => GuiApp.Respond(dir,
                (["project", "import", "--dry-run"], true, DryRun),
                (["project", "import"], true, "Import finished. Imported blocks are not started.")));
        }
        finally { Environment.SetEnvironmentVariable(Hook, null); }
    }

    [Fact]
    public void ImportProjectShowsTheDryRunAndConfirmsItsLock()
    {
        using var gui = Start("project-import");
        gui.Page("settings");
        gui.Invoke("ProjectImport");
        gui.WaitFor(() => gui.Ran("project", "import", "--dry-run", File), "kc9k project import --dry-run FILE");
        gui.FindName("Import this setup?");
        gui.WaitFor(() => gui.Window.FindAllDescendants().Any(e => (e.Properties.Name.ValueOrDefault ?? "").Contains("app gnome-terminal-server (a Linux program)")),
            "the dry run with what is not used on Windows");
        gui.Shot("dry-run");
        gui.Invoke(gui.FindButton("Import"));
        gui.FindName("Set this lock?");
        gui.Shot("lock");
        gui.Invoke(gui.FindButton("Lock"));
        gui.WaitFor(() => gui.Ran("project", "import", File, "--yes"), "kc9k project import FILE --yes");
        gui.FindName("Imported");
        gui.Invoke(gui.FindButton("OK"));
    }

    // Declining the lock imports nothing (R-LOCK-12: nothing locked before
    // confirmation).
    [Fact]
    public void ImportProjectDeclinedLockImportsNothing()
    {
        using var gui = Start("project-import-declined");
        gui.Page("settings");
        gui.Invoke("ProjectImport");
        gui.FindName("Import this setup?");
        gui.Invoke(gui.FindButton("Import"));
        gui.FindName("Set this lock?");
        gui.Invoke(gui.FindButton("Cancel"));
        Thread.Sleep(1000);
        Assert.True(gui.Ran("project", "import", "--dry-run", File));
        Assert.False(gui.Ran("project", "import", File));
    }
}
