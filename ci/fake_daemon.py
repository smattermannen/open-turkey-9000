"""What the cloud LibreWolf tests (ci/dark_browser_test.py,
ci/search_results_librewolf_test.py) share: a stand-in for the daemon on
the extension's port, the extension packed from internal/webext/files, and
a headless LibreWolf with it installed. Without KC9K installed, as a
normal user, where no KC9K daemon runs."""
import http.server
import json
import os
import socket
import sys
import tempfile
import threading
import time
import warnings
import zipfile

from selenium import webdriver
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

HERE = os.path.dirname(os.path.abspath(__file__))
FILES = os.path.join(HERE, "..", "internal", "webext", "files")
DAEMON_PORT = 47613

failures = 0
# rules: what /rules answers; polls: how often the extension asked.
state = {"rules": {"lists": []}, "polls": 0}


def check(name, passed, detail=None):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}" + (f": {detail}" if detail is not None else ""))
    failures += 0 if passed else 1


class Daemon(http.server.BaseHTTPRequestHandler):
    """The daemon's rules server, as far as the extension needs it."""

    def answer(self, body):
        data = body.encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path.startswith("/rules"):
            state["polls"] += 1
            self.answer(json.dumps(state["rules"]))
        else:
            self.answer("{}")

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length") or 0))
        self.answer("{}")

    def log_message(self, *args):
        pass


def serve(handler, port=0):
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server


def start_daemon():
    with socket.socket() as probe:
        if probe.connect_ex(("127.0.0.1", DAEMON_PORT)) == 0:
            sys.exit(f"Something answers on 127.0.0.1:{DAEMON_PORT} (a KC9K daemon?): run this where none runs.")
    serve(Daemon, DAEMON_PORT)


def pack():
    """The extension as an .xpi, as webext.Pack makes it."""
    path = os.path.join(tempfile.mkdtemp(prefix="kc9k-xpi-"), "kc9k.xpi")
    with zipfile.ZipFile(path, "w") as z:
        for name in sorted(os.listdir(FILES)):
            z.write(os.path.join(FILES, name), name)
    return path


def wait_polls(more):
    """Waits until the extension fetched the rules more times."""
    want = state["polls"] + more
    deadline = time.time() + 20
    while state["polls"] < want and time.time() < deadline:
        time.sleep(0.2)
    return state["polls"] >= want


def librewolf(prefs, xpi):
    """A headless LibreWolf (LIBREWOLF, default /usr/bin/librewolf) with the
    extension installed and its first rules fetched."""
    options = Options()
    options.binary_location = os.environ.get("LIBREWOLF", "/usr/bin/librewolf")
    options.add_argument("-headless")
    for name, value in prefs.items():
        options.set_preference(name, value)
    driver = webdriver.Firefox(options=options, service=Service(
        os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"), service_args=["--allow-system-access"]))
    driver.set_page_load_timeout(30)
    with warnings.catch_warnings():
        # Deprecated for install_web_extension (WebDriver BiDi), which can't
        # install temporarily, as an unsigned extension needs.
        warnings.simplefilter("ignore", DeprecationWarning)
        driver.install_addon(xpi, temporary=True)
    check("the extension fetches the rules", wait_polls(2))
    return driver


def finish(name):
    print(f"{name}: " + (f"{failures} failed" if failures else "all passed"))
    sys.exit(1 if failures else 0)
