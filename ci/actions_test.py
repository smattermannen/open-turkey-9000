"""Checks list actions and bypass prevention inside a real LibreWolf: a
picture instead of the page, close-tab, bypass prevention closing the
browser, close-browser, and monk mode from a list (which ends by itself).
Logout and shutdown are left out: they would end the session running this.
Run on a disposable machine, like browser_test.py:

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/actions_test.py
"""
import http.server
import os
import struct
import subprocess
import sys
import tempfile
import threading
import time
import zlib

from selenium import webdriver
from selenium.common.exceptions import WebDriverException
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

failures = 0
BLOCKS = ("actions", "nepsistest", "closer", "monklist")
REMOTE_CONTROL_MARKER = "/var/lib/kc9k/allow-remote-control"


def check(name, passed):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}")
    failures += 0 if passed else 1


def ot(*args):
    subprocess.run(["kc9k", *args], check=True)


def ot_quiet(*args):
    subprocess.run(["kc9k", *args], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def cleanup():
    for name in BLOCKS:
        ot_quiet("nepsis", "off", "--block", name)
        ot_quiet("stop", name)
        ot_quiet("block", "remove", name)
    subprocess.run(["sudo", "rm", "-f", REMOTE_CONTROL_MARKER])


def start():
    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"),
                      service_args=["--allow-system-access"])
    browser = webdriver.Firefox(options=options, service=service)
    browser.set_page_load_timeout(40)
    time.sleep(15)  # the extension installs (on a first start) and fetches its rules
    return browser


def visit(browser, url):
    """Opens url. A blocked page can show up as an error page to Selenium (the
    browser policy blocks it too), and a closing tab or browser cuts the
    navigation short: both are fine here."""
    try:
        browser.get(url)
    except WebDriverException:
        pass
    time.sleep(4)


def closed(browser, wait=20):
    """Waits for the browser to go away (its windows closed or killed)."""
    for _ in range(wait):
        try:
            if not browser.window_handles:
                return True
        except WebDriverException:
            return True
        time.sleep(1)
    return False


def png(path, size=40):
    """Writes a small red square."""
    row = b"\x00" + b"\xd0\x30\x30" * size
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 2, 0, 0, 0))
                + chunk(b"IDAT", zlib.compress(row * size)) + chunk(b"IEND", b""))


class SearchPage:
    """A local page shaped like a search engine's results, linking to a
    blocked site."""

    def __init__(self):
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                body = b'<!doctype html><title>results</title><a id="result" href="https://example.org/">a result</a>'
                self.send_response(200)
                self.send_header("Content-Type", "text/html")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def stop(self):
        self.server.shutdown()


cleanup()
subprocess.run(["sudo", "touch", REMOTE_CONTROL_MARKER], check=True)
# The marker changes LibreWolf's settings (developer tools); the daemon closes
# browsers started before it notices, so let it notice first.
time.sleep(12)
try:
    picture = os.path.join(tempfile.mkdtemp(), "calm.png")
    png(picture)
    ot("block", "create", "actions")
    ot("list", "add", "actions", "Blocked", "example.net")
    ot("list", "action", "actions", "Blocked", "image", "on", "--image", picture)
    ot("list", "create", "actions", "Tabs")
    ot("list", "add", "actions", "Tabs", "example.org")
    ot("list", "action", "actions", "Tabs", "close-tab", "on")
    ot("start", "actions")

    driver = start()
    try:
        visit(driver, "https://example.net/")
        print("picture url:", driver.current_url)
        shown = driver.execute_script(
            "const p = document.getElementById('picture');"
            "return p && !p.hidden && p.complete && p.naturalWidth > 0")
        check("a list's picture shows instead of the page", "image=" in driver.current_url and shown)

        driver.switch_to.new_window("tab")
        before = len(driver.window_handles)
        visit(driver, "https://example.org/")
        check("close-tab closes the tab", len(driver.window_handles) == before - 1)
    finally:
        try:
            driver.quit()
        except WebDriverException:
            pass
    ot("stop", "actions")

    # Bypass prevention: three blocked pages within 5 minutes close the browser.
    ot("block", "create", "nepsistest")
    ot("list", "add", "nepsistest", "Blocked", "example.net", "example.org", "example.edu")
    ot("start", "nepsistest")
    ot("nepsis", "set", "--block", "nepsistest", "--count", "3", "--within", "5m", "--then", "close-browser")
    driver = start()
    for url in ("https://example.net/", "https://example.org/", "https://example.edu/"):
        visit(driver, url)
    check("bypass prevention closes the browser after three blocked pages", closed(driver, 30))
    ot("nepsis", "off", "--block", "nepsistest")

    # A ladder: two blocked pages close the tab of the second; two more close
    # the browser.
    ot("nepsis", "set", "--block", "nepsistest",
       "--step", "2 in 5m: close-tab", "--step", "2 in 5m: close-browser")
    driver = start()
    try:
        visit(driver, "https://example.net/")
        driver.switch_to.new_window("tab")
        before = len(driver.window_handles)
        visit(driver, "https://example.org/")
        check("a ladder's first step closes the tab", len(driver.window_handles) == before - 1)
        driver.switch_to.window(driver.window_handles[0])
        time.sleep(1)  # only events after the step fired count for the next
        visit(driver, "https://example.edu/")
        check("the browser stays open below the second step", not closed(driver, 3))
        visit(driver, "https://example.org/")
        check("a ladder's second step closes the browser", closed(driver, 30))
    finally:
        try:
            driver.quit()
        except WebDriverException:
            pass
    ot("nepsis", "off", "--block", "nepsistest")

    # The search detector: a blocked page opened by hand doesn't count, one
    # opened from a search engine's results does (any /search?q= page is one).
    ot("nepsis", "set", "--block", "nepsistest", "--detect", "search",
       "--step", "1 in 5m: close-browser")
    results = SearchPage()
    driver = start()
    try:
        visit(driver, "https://example.net/")
        check("a blocked page opened by hand isn't a search hit", not closed(driver, 8))
        driver.switch_to.new_window("tab")
        driver.get(f"http://127.0.0.1:{results.port}/search?q=mirror")
        try:
            driver.find_element("id", "result").click()
        except WebDriverException:
            pass  # the blocked page shows up as an error page, or the browser is gone
        check("a blocked page opened from search results is", closed(driver, 30))
    finally:
        results.stop()
        try:
            driver.quit()
        except WebDriverException:
            pass
    ot("nepsis", "off", "--block", "nepsistest")
    ot("stop", "nepsistest")

    # close-browser from a list: the extension closes every window itself.
    ot("block", "create", "closer")
    ot("list", "add", "closer", "Blocked", "example.net")
    ot("list", "action", "closer", "Blocked", "close-browser", "on")
    ot("start", "closer")
    driver = start()
    visit(driver, "https://example.net/")
    check("close-browser closes the browser", closed(driver))
    ot("stop", "closer")

    # monk mode from a list, for a minute: the browser closes and monk-mode
    # starts, then ends by itself.
    ot("block", "create", "monklist")
    ot("list", "add", "monklist", "Blocked", "example.net")
    ot("list", "action", "monklist", "Blocked", "monk", "on", "--for", "1m")
    ot("start", "monklist")
    driver = start()
    visit(driver, "https://example.net/")
    check("monk mode closes the browser", closed(driver))
    time.sleep(8)  # the daemon starts it after the extension has closed the browser
    info = subprocess.run(["kc9k", "block", "info", "monk-mode"], capture_output=True, text=True).stdout
    check("monk mode starts from a list", "Status: Active" in info and "Time-locked" in info)
    ot("stop", "monklist")
    for _ in range(100):
        info = subprocess.run(["kc9k", "block", "info", "monk-mode"], capture_output=True, text=True).stdout
        if "Status: Inactive" in info:
            break
        time.sleep(2)
    check("monk mode ends by itself", "Status: Inactive" in info)
finally:
    cleanup()

sys.exit(failures)
