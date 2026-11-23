"""GUI smoke test for the Account page (§4.14 cross-device sync, 2.10)
against the fake cloud (ci/cloudfake, internal/cloud/cloudtest): sign up
and Upload all on one computer, then log in on a second one (another
scratch database) and Download all: its dry run shown with the locks it
sets, Cancel changing nothing, OK replacing the setup; log out. Builds kc9k
into a scratch folder, wrappers with KC9K_DB = scratch files, loads
gui/kc9k-gui.py as a module with CLI replaced, another application ID,
never shows the main window. Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_account_smoke.py
"""
import importlib.util, json, os, subprocess, sys, tempfile, time, traceback
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
ENV = dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"])
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True, env=ENV)
subprocess.run(["go", "build", "-o", os.path.join(S, "cloudfake"), "./ci/cloudfake"], cwd=ROOT, check=True, env=ENV)
fake = subprocess.Popen([os.path.join(S, "cloudfake")], stdout=subprocess.PIPE, text=True)
url, key = fake.stdout.readline().split()


def wrapper(name):
    path = os.path.join(S, name)
    with open(path, "w") as f:
        f.write(f'#!/bin/sh\nKC9K_DB={S}/{name}.db exec {S}/kc9k-bin "$@"\n')
    os.chmod(path, 0o755)
    subprocess.run([path, "account", "server", url, key], check=True, capture_output=True)
    return path


A, B = wrapper("a"), wrapper("b")


def cli(w, *a):
    return subprocess.run([w, *a], capture_output=True, text=True)


os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")  # never the real ~/.config
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = A
from gi.repository import Gtk, GLib  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kaccountsmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
errors = []
sys.excepthook = lambda kind, value, tb: errors.append("".join(traceback.format_exception(kind, value, tb)))
msgs, confirms, texts, answers = [], [], [], {"confirm": True}
win.message = lambda text, *a, **k: msgs.append(str(text))
win.confirm = lambda text, detail, *a, **k: confirms.append((text, detail)) or answers["confirm"]
win.ask_text = lambda *a, **k: texts.pop(0) if texts else None
fails = []


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


def pump(cond, secs=20):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False)
        time.sleep(0.01)
    return cond()


def account():
    """The page's account state, read afresh."""
    pump(lambda: win.get_sensitive() and not getattr(win, "account_busy", False))
    win.account_state = None
    win.refresh_account()
    pump(lambda: win.account_state is not None)
    return win.account_state


try:
    win.stack.set_visible_child_name("account")
    st = account()
    check(st.get("available") and not st.get("email"), f"signed out at first: {st}")
    check(win.account_signed_out.get_visible() and not win.account_logout.get_visible(),
          "email, Log in and Sign up shown; Log out hidden")
    check(not win.sync_upload.get_sensitive() and not win.sync_download.get_sensitive(),
          "Upload all and Download all greyed out while signed out")

    # Sign up on computer A: the password asked twice.
    win.account_email.set_text("me@example.org")
    texts[:] = ["secret123", "different"]
    win.account_sign_in(new=True)
    check(msgs and "aren't the same" in msgs[-1], "different passwords refused before anything runs")
    texts[:] = ["secret123", "secret123"]
    win.account_sign_in(new=True)
    check(pump(lambda: (win.account_state or {}).get("email") == "me@example.org"), "signed up and in")
    check(not win.sync_download.get_sensitive(), "Download all greyed out: nothing uploaded yet")

    for a in (["block", "create", "work"], ["block", "add", "work", "reddit.com"],
              ["start", "work", "--for", "2h", "--yes"]):
        r = cli(A, *a)
        check(r.returncode == 0, f"setup {a}: {r.stderr.strip()}")
    win.upload_all(None)
    check(pump(lambda: (win.account_state or {}).get("last_upload") is not None), "Upload all: an upload shown")
    check(win.sync_status.get_text().startswith("Last upload:"), "last upload named: " + win.sync_status.get_text())
    confirms.clear()
    answers["confirm"] = False
    win.upload_all(None)
    check(confirms and "replaces the upload" in confirms[-1][1], "a second Upload all asks first")

    # Computer B: its own block; log in, Download all.
    gui.CLI = B
    cli(B, "block", "create", "local")
    st = account()
    check(not st.get("email"), "computer B signed out")
    win.account_email.set_text("me@example.org")
    texts[:] = ["wrong"]
    win.account_sign_in(new=False)
    check(pump(lambda: any("password" in m.lower() or "invalid" in m.lower() for m in msgs)),
          "a wrong password says so: " + (msgs[-1] if msgs else ""))
    texts[:] = ["secret123"]
    win.account_sign_in(new=False)
    check(pump(lambda: (win.account_state or {}).get("email") == "me@example.org"), "logged in on B")
    check(win.sync_download.get_sensitive(), "Download all usable once there is an upload")

    confirms.clear()
    answers["confirm"] = False
    win.download_all(None)
    check(pump(lambda: bool(confirms)), "Download all asks first")
    detail = confirms[-1][1] if confirms else ""
    check("add block 'work'" in detail and "set a lock" in detail, "its dry run and locks shown: " + detail)
    pump(lambda: win.get_sensitive())
    check(cli(B, "block", "info", "local").returncode == 0, "Cancel changed nothing")
    answers["confirm"] = True
    win.download_all(None)
    check(pump(lambda: cli(B, "block", "info", "work").returncode == 0, 30), "OK: the upload's block is here")
    pump(lambda: win.get_sensitive())
    check(cli(B, "block", "info", "local").returncode != 0, "OK: this computer's block is gone")
    work = [b for b in json.loads(cli(B, "gui-state").stdout)["blocks"] if b["name"] == "work"]
    check(work and work[0]["active"] and work[0].get("lock_seconds"), "the block came on and locked")

    answers["confirm"] = True
    win.account_log_out(None)
    check(pump(lambda: win.account_state is not None and not win.account_state.get("email")
               and not getattr(win, "account_busy", False)), "logged out")
    check(win.account_signed_out.get_visible(), "Log in shown again")
finally:
    fake.kill()

check(not errors, "no handler raised" + ("".join(errors) and ":\n" + "".join(errors)))
print("messages:", msgs)
if fails:
    print(f"{len(fails)} failed")
    sys.exit(1)
print("all passed")
