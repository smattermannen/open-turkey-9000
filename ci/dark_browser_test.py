"""Checks, in a real LibreWolf, that "Show dark web pages while dark mode is
on" (SPEC §4.10) turns on the browser's own dark mode: a site with its own
dark design shows it, a light page without one is shown dark, and with
dark mode off the site is light again. Without KC9K installed: this script
stands in for the daemon on the extension's port (127.0.0.1:47613, so not
where a KC9K daemon runs; the box has ci/circadian_test.py), loads the
extension from internal/webext/files and gives LibreWolf the preferences
KC9K writes into librewolf.cfg (blocker.darkPrefs, pinned by
TestDarkPrefsAreDefaults). It also shows why they are needed: with
LibreWolf's own settings every site is told light.

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/dark_browser_test.py

As a normal user. LIBREWOLF picks the browser (default /usr/bin/librewolf).
"""
import http.server
import json
import os
import socket
import sys
import tempfile
import threading
import time
import zipfile

from selenium import webdriver
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

HERE = os.path.dirname(os.path.abspath(__file__))
FILES = os.path.join(HERE, "..", "internal", "webext", "files")
DAEMON_PORT = 47613
# What KC9K writes into librewolf.cfg while dark pages are on
# (blocker.darkPrefs), as defaults there.
KC9K_PREFS = {
    "privacy.resistFingerprinting": False,
    "privacy.fingerprintingProtection": True,
    "privacy.fingerprintingProtection.overrides": "+AllTargets,-CSSPrefersColorScheme",
}
PAGES = {
    # A site with a dark design of its own.
    "/own-dark": "<!doctype html><title>Own dark</title><style>body{background:#fff;color:#000}"
                 "@media (prefers-color-scheme: dark){body{background:rgb(17,17,17);color:#eee}}</style>"
                 "<p>a site with its own dark design</p>",
    "/light": "<!doctype html><title>Light</title><body style='background:#fff'><p>light only</p></body>",
}
DARK_ON = {"on": True, "follow_system": False, "exempt": []}

failures = 0
state = {"rules": {"lists": [], "dark": DARK_ON}, "polls": 0}


def check(name, passed, detail=None):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}" + (f": {detail}" if detail is not None else ""))
    failures += 0 if passed else 1


class Daemon(http.server.BaseHTTPRequestHandler):
    """The daemon's rules server, as far as the extension needs it."""

    def answer(self, body, kind="application/json"):
        data = body.encode()
        self.send_response(200)
        self.send_header("Content-Type", kind)
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


class Site(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = PAGES.get(self.path.split("?")[0], "<!doctype html><title>?</title>").encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def serve(handler, port):
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server


def pack(path):
    with zipfile.ZipFile(path, "w") as z:
        for name in sorted(os.listdir(FILES)):
            z.write(os.path.join(FILES, name), name)


def wait_polls(more):
    """Waits until the extension fetched the rules more times."""
    want = state["polls"] + more
    deadline = time.time() + 20
    while state["polls"] < want and time.time() < deadline:
        time.sleep(0.2)
    return state["polls"] >= want


def start(prefs):
    options = Options()
    options.binary_location = os.environ.get("LIBREWOLF", "/usr/bin/librewolf")
    options.add_argument("-headless")
    for name, value in prefs.items():
        options.set_preference(name, value)
    driver = webdriver.Firefox(options=options, service=Service(
        os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"), service_args=["--allow-system-access"]))
    driver.set_page_load_timeout(30)
    # install_web_extension in newer Selenium, install_addon before.
    (getattr(driver, "install_web_extension", None) or driver.install_addon)(xpi, temporary=True)
    check("the extension fetches the rules", wait_polls(2))
    return driver


def look(driver, path):
    driver.get(base + path)
    time.sleep(2)  # the page is judged once it settles
    return driver.execute_script("""return {
        dark: matchMedia('(prefers-color-scheme: dark)').matches,
        background: getComputedStyle(document.body).backgroundColor,
        inverted: document.documentElement.hasAttribute('data-kc9k-dark')};""")


def browser_dark(driver):
    """Whether LibreWolf's own window is dark (its theme)."""
    driver.set_context(driver.CONTEXT_CHROME)
    try:
        return driver.execute_script("return matchMedia('(prefers-color-scheme: dark)').matches")
    finally:
        driver.set_context(driver.CONTEXT_CONTENT)


with socket.socket() as probe:
    if probe.connect_ex(("127.0.0.1", DAEMON_PORT)) == 0:
        sys.exit(f"Something answers on 127.0.0.1:{DAEMON_PORT} (a KC9K daemon?): run this where none runs.")
serve(Daemon, DAEMON_PORT)
site = serve(Site, 0)
base = f"http://127.0.0.1:{site.server_address[1]}"
work = tempfile.mkdtemp(prefix="kc9k-dark-")
xpi = os.path.join(work, "kc9k.xpi")
pack(xpi)

# Dark pages on, dark mode on, with KC9K's preferences.
driver = start(KC9K_PREFS)
try:
    page = look(driver, "/own-dark")
    check("dark mode on: the site is told dark mode is on", page["dark"], page)
    check("dark mode on: the site shows its own dark design", page["background"] == "rgb(17, 17, 17)", page)
    check("dark mode on: a site dark by itself isn't turned over", not page["inverted"], page)
    page = look(driver, "/light")
    check("dark mode on: a light page without a dark design is shown dark", page["inverted"], page)

    state["rules"] = {"lists": [], "dark": {"on": False, "follow_system": False, "exempt": []}}
    wait_polls(2)
    page = look(driver, "/own-dark")
    check("dark mode off: the site is told light again", not page["dark"], page)
    check("dark mode off: the site shows its light design", page["background"] == "rgb(255, 255, 255)", page)
    check("dark mode off: nothing turned over", not page["inverted"], page)

    state["rules"] = {"lists": [], "dark": None}
    wait_polls(2)
    page = look(driver, "/own-dark")
    check("dark pages off: the browser's own setting holds", not page["dark"] and not page["inverted"], page)
finally:
    driver.quit()

# LibreWolf's own settings (resistFingerprinting): the site never learns of
# dark mode; KC9K can only turn its light design over.
state["rules"] = {"lists": [], "dark": DARK_ON}
driver = start({})
try:
    page = look(driver, "/own-dark")
    check("LibreWolf's own settings: every site is told light", not page["dark"], page)
    check("LibreWolf's own settings: the light design is turned over instead", page["inverted"], page)
finally:
    driver.quit()

# Dark mode left to the desktop (or switched there by KC9K): with KC9K's
# preferences the browser follows it, its window and the sites.
# ui.systemUsesDarkTheme stands in for a dark desktop.
state["rules"] = {"lists": [], "dark": {"on": False, "follow_system": True, "exempt": []}}
driver = start(dict(KC9K_PREFS, **{"ui.systemUsesDarkTheme": 1}))
try:
    check("dark desktop: LibreWolf's window is dark", browser_dark(driver))
    page = look(driver, "/own-dark")
    check("dark desktop: the site shows its own dark design", page["dark"] and page["background"] == "rgb(17, 17, 17)", page)
    check("dark desktop: a site dark by itself isn't turned over", not page["inverted"], page)
finally:
    driver.quit()

print("dark browser test: " + (f"{failures} failed" if failures else "all passed"))
sys.exit(1 if failures else 0)
