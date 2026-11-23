"""GUI smoke test for the Account page (2.10, SPEC §4.14): builds kc9k and
cmd/fake-cloud into a scratch folder, two wrappers on two scratch databases
(KC9K_DB: no layers are ever applied) stand for two devices, both pointed at
the one fake cloud. The page signs up, uploads, logs out and in on the first
device, then the second logs in and runs Download all (dry run shown, the
lock it sets confirmed). Every change is a kc9k command. Needs an X display.

    xvfb-run -a dbus-run-session -- python3 ci/gui_account_smoke.py
"""
import importlib.util, json, os, subprocess, sys, tempfile, time
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-account-")
env = dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"])
for out, pkg in (("kc9k-bin", "./cmd/kc9k"), ("fake-cloud", "./cmd/fake-cloud")):
    subprocess.run(["go", "build", "-o", os.path.join(S, out), pkg], cwd=ROOT, check=True, env=env)
cloud = subprocess.Popen([os.path.join(S, "fake-cloud")], stdout=subprocess.PIPE, text=True)
url, key = cloud.stdout.readline().split()

def wrapper(name):
    path = os.path.join(S, name)
    with open(path, "w") as f:
        f.write(f'#!/bin/sh\nKC9K_DB={S}/{name}.db exec {S}/kc9k-bin "$@"\n')
    os.chmod(path, 0o755)
    subprocess.run([path, "account", "server", url, key], check=True, capture_output=True)
    return path

A, B = wrapper("device-a"), wrapper("device-b")
def cli(w, *a):
    return subprocess.run([w, *a], capture_output=True, text=True)

os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")  # never the real ~/.config
spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = A
from gi.repository import Gtk, GLib
app = gui.App(); app.set_application_id("io.test.kc9kaccountsmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True  # no install dialog in a test
fails = []
def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what); ok or fails.append(what)
def pump(cond, secs=30):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    return cond()

msgs, asked, confirms, lock_asks = [], [], [], []
win.message = lambda text, *a, **k: msgs.append(text)
win.ask_text = lambda title, detail="", **k: asked.pop(0) if asked else ""
def confirm(text, detail, ok_label="OK", destructive=False):
    confirms.append((text, detail)); return True
win.confirm = confirm
win.confirm_lock = lambda locks, option=None: lock_asks.append(list(locks)) or True

def account():
    """Ask the page to refresh and wait for its answer."""
    win.account_state = None
    win.refresh_account()
    pump(lambda: win.account_state is not None)
    return win.account_state or {}
def done():
    pump(lambda: win.get_sensitive())
    return account()

win.stack.get_child_by_name("account").show_all()  # the window itself is never shown
win.stack.set_visible_child_name("account")  # showing the page asks the account
pump(lambda: "Asking" not in win.account_status.get_text())
check(win.account_status.get_text() == "Not signed in.", "not signed in: " + win.account_status.get_text())
check(win.account_buttons["signup"].get_sensitive() and not win.account_buttons["upload"].get_sensitive(),
      "Sign up on, Upload all off while signed out")

asked[:] = ["me@example.org", "secret123", "secret123"]
win.account_signup(None)
st = done()
check(st.get("email") == "me@example.org", "signed up and in: %s" % st)
check(win.account_status.get_text() == "Signed in as me@example.org.", "status says the account")
check(win.sync_note.get_text() == "Nothing uploaded yet.", "nothing uploaded: " + win.sync_note.get_text())
check(not win.account_buttons["download"].get_sensitive(), "Download all off with nothing uploaded")

cli(A, "block", "create", "work"); cli(A, "block", "add", "work", "reddit.com")
cli(A, "start", "work", "--for", "2h", "--yes")
win.sync_upload(None)
st = done()
check(bool(st.get("last_upload")), "uploaded: %s" % st)
check(win.sync_note.get_text().startswith("Last upload: "), "last upload shown: " + win.sync_note.get_text())
check(win.account_buttons["download"].get_sensitive(), "Download all on after an upload")

confirms.clear()
win.sync_upload(None)
st = done()
check(any(t == "Upload all?" for t, _ in confirms), "a second upload asks before replacing the first")

win.account_logout(None)
st = done()
check(not st.get("email") and win.account_status.get_text() == "Not signed in.", "logged out")

asked[:] = ["me@example.org", "wrong-password"]
msgs.clear()
win.account_login(None)
st = done()
check(not st.get("email") and msgs, "a wrong password is refused and said: %s" % msgs[-1:])
msgs.clear()
asked[:] = ["me@example.org", "secret123"]
win.account_login(None)
st = done()
check(st.get("email") == "me@example.org", "logged in again")

# The second device: its own setup, replaced by Download all.
gui.CLI = B
cli(B, "block", "create", "old")
asked[:] = ["me@example.org", "secret123"]
win.account_login(None)
st = done()
check(st.get("email") == "me@example.org" and st.get("last_upload"), "second device signed in, sees the upload")
confirms.clear(); lock_asks.clear()
win.sync_download(None)
done()
text = " ".join(d for t, d in confirms if t == "Download all?")
check("add block 'work'" in text and "remove this device's setup (1 blocks)" in text,
      "the dry run is shown before replacing: " + text[:200])
check(lock_asks and any("work" in l for l in lock_asks[0]), "the lock it sets is confirmed: %s" % lock_asks)
names = [b["name"] for b in json.loads(cli(B, "gui-state").stdout).get("blocks", [])]
check("work" in names and "old" not in names, "second device has the upload's setup: %s" % names)
check(not [m for m in msgs if "rror" in m or "refused" in m], "no error said: %s" % msgs)

cloud.terminate()
print("account page: " + ("all passed" if not fails else f"{len(fails)} failed"))
sys.exit(1 if fails else 0)
