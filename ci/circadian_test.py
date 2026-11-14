"""Checks, inside a real LibreWolf, the circadian rhythm's dark pages
(SPEC §4.10): a light page is shown dark while dark mode is on, a dark page
is left alone, and so is everything once dark pages are off. Run on a
disposable machine with a TAGS=devmaster build (it switches the desktop to
dark mode, and WebDriver is closed by any other build):

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/circadian_test.py
"""
import http.server
import json
import os
import subprocess
import sys
import threading
import time

from selenium import webdriver
from selenium.common.exceptions import WebDriverException
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

failures = 0
REMOTE_CONTROL_MARKER = "/var/lib/kc9k/allow-remote-control"
PAGES = {
    "/light": "<!doctype html><title>Light</title><body style='background:#fff'><p>light</p></body>",
    "/dark": "<!doctype html><title>Dark</title><body style='background:#111;color:#eee'><p>dark</p></body>",
}


def check(name, passed):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}")
    failures += 0 if passed else 1


def kc9k(*args):
    return subprocess.run(["kc9k", *args], capture_output=True, text=True).stdout


class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = PAGES.get(self.path.split("?")[0], "<!doctype html><title>?</title>").encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


def style(browser, path):
    try:
        browser.get(base + path)
    except WebDriverException:
        pass
    time.sleep(5)
    return browser.execute_script(
        "const s = document.getElementById('kc9k-page-style'); return s ? s.textContent : '';")


before = json.loads(kc9k("circadian", "show", "--json"))
server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
base = f"http://127.0.0.1:{server.server_address[1]}"
threading.Thread(target=server.serve_forever, daemon=True).start()
subprocess.run(["sudo", "touch", REMOTE_CONTROL_MARKER], check=True)
kc9k("circadian", "on")
kc9k("circadian", "dark", "dark")
kc9k("circadian", "dark-pages", "on")
time.sleep(12)
driver = None
try:
    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    driver = webdriver.Firefox(options=options, service=Service(
        os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"), service_args=["--allow-system-access"]))
    driver.set_page_load_timeout(40)
    time.sleep(15)
    check("a light page is shown dark while dark mode is on", "invert(1)" in style(driver, "/light"))
    check("a page that is dark already is left alone", "invert(1)" not in style(driver, "/dark"))
    kc9k("circadian", "dark", "light")
    time.sleep(16)
    check("light mode: pages are left alone", "invert(1)" not in style(driver, "/light"))
    kc9k("circadian", "dark", "dark")
    kc9k("circadian", "dark-pages", "off")
    time.sleep(16)
    check("dark pages off: pages are left alone", "invert(1)" not in style(driver, "/light"))
finally:
    try:
        if driver:
            driver.quit()
    except WebDriverException:
        pass
    server.shutdown()
    subprocess.run(["sudo", "rm", "-f", REMOTE_CONTROL_MARKER])
    dark = before["dark"]
    if dark["mode"] == "scheduled":
        kc9k("circadian", "dark", "light-between", dark["light_from"], dark["light_to"])
    else:
        kc9k("circadian", "dark", dark["mode"])
    kc9k("circadian", "dark-pages", "on" if before["dark_pages"] else "off")
    kc9k("circadian", "on" if before["on"] else "off")

sys.exit(1 if failures else 0)
