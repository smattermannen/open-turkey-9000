// W41: msiexec with its full UI, answered as a person would: the licence
// box ticked, then Install, Yes, Next, Finish or OK pressed (never Cancel,
// Back or the title bar's Close; No only to "want to cancel?"); each
// dialog's text goes to the transcript.
//   MsiUi.exe "<msiexec arguments>" <log> <transcript> [seconds]
// Exits with msiexec's exit code (9999 when it had to be stopped).
using System.Diagnostics;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.UIA3;

var (arguments, log, transcript) = (args[0], args[1], args[2]);
var seconds = args.Length > 3 ? int.Parse(args[3]) : 300;
using var automation = new UIA3Automation();
var p = Process.Start(new ProcessStartInfo("msiexec.exe", $"{arguments} /l*v \"{log}\"") { UseShellExecute = false })!;
var seen = new List<string>();
string[] press = ["Install", "Yes", "Next", "Finish", "OK"];
var deadline = DateTime.Now.AddSeconds(seconds);
while (!p.HasExited && DateTime.Now < deadline)
{
    Thread.Sleep(700);
    var pids = Process.GetProcessesByName("msiexec").Select(x => x.Id).ToHashSet();
    pids.Add(p.Id);
    foreach (var w in automation.GetDesktop().FindAllChildren())
    {
        try
        {
            if (!pids.Contains(w.Properties.ProcessId.ValueOrDefault)) continue;
            var all = w.FindAllDescendants();
            var texts = string.Join(" | ", all.Where(e => e.ControlType == ControlType.Text && !string.IsNullOrWhiteSpace(e.Name)).Select(e => e.Name));
            var line = $"[{w.Name}] {texts}";
            if (!seen.Contains(line)) { seen.Add(line); Console.WriteLine("dialog: " + line); }
            foreach (var box in all.Where(e => e.ControlType == ControlType.CheckBox && e.Name.Contains("accept", StringComparison.OrdinalIgnoreCase)))
                if (box.Patterns.Toggle.Pattern.ToggleState.Value != ToggleState.On) box.Patterns.Toggle.Pattern.Toggle();
            // Never the title bar's buttons (Close there cancels setup).
            var buttons = all.Where(e => e.ControlType == ControlType.Button && e.IsEnabled
                && e.Properties.AutomationId.ValueOrDefault is not ("Close" or "Minimize" or "Maximize")).ToList();
            // A question whether to cancel is answered No, as a person carrying on would.
            var order = texts.Contains("want to cancel", StringComparison.OrdinalIgnoreCase) ? ["No"] : press;
            foreach (var name in order)
            {
                var b = buttons.FirstOrDefault(e => e.Name.Replace("&", "") == name);
                if (b is null) continue;
                Console.WriteLine("press: " + name);
                if (b.Patterns.Invoke.IsSupported) b.Patterns.Invoke.Pattern.Invoke(); else b.Click();
                break;
            }
        }
        catch (Exception) { } // a dialog closing while read
    }
}
int code;
if (!p.HasExited) { Console.WriteLine($"msiexec still running after {seconds} s"); p.Kill(true); code = 9999; }
else code = p.ExitCode;
File.WriteAllLines(transcript, seen);
Console.WriteLine($"msiexec exit {code}");
return code;
