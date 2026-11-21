// The MSI's custom actions (W7, notes/install-windows.md). kc9k decides
// everything: install sets up the service (kc9k install), removal runs
// kc9k uninstall (refused while anything is locked), and a reinstall or an
// older version runs kc9k prepare-install first (refused while locked with
// tamper protection's "block the installer" on). When kc9k refuses, its
// own words are shown and the MSI rolls back.
using System;
using System.Diagnostics;
using System.IO;
using System.Text;
using WixToolset.Dtf.WindowsInstaller;

namespace Kc9k.Installer
{
    public static class CustomActions
    {
        /// <summary>Immediate: KC9K_GUARD=1 when this MSI would reinstall the
        /// same version or put an older one over a newer (§4.10 block the
        /// installer: a newer version always installs).</summary>
        [CustomAction]
        public static ActionResult DetectSameOrNewer(Session session)
        {
            try
            {
                var mine = new Version(session["ProductVersion"]);
                var code = session["UpgradeCode"];
                foreach (var installed in ProductInstallation.GetRelatedProducts(code))
                {
                    if (installed.ProductVersion != null && installed.ProductVersion >= mine)
                    {
                        session["KC9K_GUARD"] = "1";
                        session.Log("KC9K: installed version {0} is the same or newer than {1}", installed.ProductVersion, mine);
                    }
                }
                if (!string.IsNullOrEmpty(session["REINSTALL"])) session["KC9K_GUARD"] = "1";
            }
            catch (Exception e)
            {
                session.Log("KC9K: detecting installed versions failed: " + e);
            }
            return ActionResult.Success;
        }

        /// <summary>Deferred, as SYSTEM: CustomActionData is "path|arguments".
        /// A missing kc9k.exe is skipped when the data says "optional|".</summary>
        [CustomAction]
        public static ActionResult RunKc9k(Session session)
        {
            var data = session.CustomActionData.ToString();
            var optional = data.StartsWith("optional|", StringComparison.Ordinal);
            if (optional) data = data.Substring("optional|".Length);
            var bar = data.IndexOf('|');
            var exe = bar < 0 ? data : data.Substring(0, bar);
            var args = bar < 0 ? "" : data.Substring(bar + 1);
            if (!File.Exists(exe))
            {
                session.Log("KC9K: {0} not found{1}", exe, optional ? ", nothing to do" : "");
                return optional ? ActionResult.Success : Fail(session, "Killer Chicken 9000's program (" + exe + ") is missing.");
            }
            var info = new ProcessStartInfo(exe, args)
            {
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                RedirectStandardInput = true,
                StandardOutputEncoding = Encoding.UTF8,
                StandardErrorEncoding = Encoding.UTF8,
            };
            using (var process = Process.Start(info))
            {
                process.StandardInput.Close(); // nothing to type: a question is a refusal
                var output = process.StandardOutput.ReadToEndAsync();
                var errors = process.StandardError.ReadToEndAsync();
                if (!process.WaitForExit(10 * 60 * 1000))
                {
                    try { process.Kill(); } catch (Exception) { }
                    return Fail(session, "kc9k " + args + " took too long and was stopped.");
                }
                var text = (output.Result + errors.Result).Trim();
                session.Log("KC9K: kc9k {0} exited {1}: {2}", args, process.ExitCode, text);
                if (process.ExitCode != 0)
                    return Fail(session, text.Length > 0 ? text : "kc9k " + args + " failed.");
            }
            return ActionResult.Success;
        }

        static ActionResult Fail(Session session, string text)
        {
            var record = new Record(1);
            record.FormatString = "Killer Chicken 9000: " + text.Replace("[", "[\\[]").Replace("]", "[\\]]");
            // Shown in a full UI, written to the log when quiet.
            session.Message(InstallMessage.Error | (InstallMessage)MessageButtons.OK | (InstallMessage)MessageIcon.Error, record);
            return ActionResult.Failure;
        }
    }
}
