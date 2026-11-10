"""Checks, inside a real LibreWolf, what extension 1.13 added: the reason on
the block page (and the option that hides it), sites reached through a proxy
address or a front-end, keyword test mode, the time left for the toolbar
button, and adding a site from it. Run on a disposable machine:

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/extras_test.py
"""
import datetime
import http.server
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
BLOCKS = ("extras", "extrawords", "extrasoon", "extraadd")
REMOTE_CONTROL_MARKER = "/var/lib/kc9k/allow-remote-control"


def check(name, passed):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}")
    failures += 0 if passed else 1


def ot(*args):
    subprocess.run(["kc9k", *args], check=True, stdout=subprocess.DEVNULL)


def ot_quiet(*args):
    subprocess.run(["kc9k", *args], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def output(*args):
    return subprocess.run(["kc9k", *args], capture_output=True, text=True).stdout


def cleanup():
    for name in BLOCKS:
        ot_quiet("list", "test", name, "off")
        ot_quiet("stop", name)
        ot_quiet("block", "remove", name)
    ot_quiet("option", "block-page-details", "on")
    subprocess.run(["sudo", "rm", "-f", REMOTE_CONTROL_MARKER])


PAGES = {
    "/frontend": '<!doctype html><meta property="og:site_name" content="Invidious"><title>Videos</title><p>hello</p>',
    "/plain": "<!doctype html><title>Plain</title><p>nothing here</p>",
    "/words": "<!doctype html><title>Words</title><p>kcninewordone and kcninewordtwo, kcninewordone again</p>",
    "/go": "<!doctype html><title>Proxy</title><p>a proxy page</p>",
}


class Pages:
    def __init__(self):
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

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()


def start():
    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"),
                      service_args=["--allow-system-access"])
    browser = webdriver.Firefox(options=options, service=service)
    browser.set_page_load_timeout(40)
    time.sleep(15)
    return browser


def visit(browser, url, wait=5):
    try:
        browser.get(url)
    except WebDriverException:
        pass
    time.sleep(wait)


def on_block_page(browser):
    return browser.current_url.startswith("moz-extension:")


def text_of(browser, element_id):
    return browser.execute_script(
        "const e = document.getElementById(arguments[0]); return e ? e.textContent : null;", element_id)


def ask(browser, message):
    """Sends a message to the extension's background script from one of its pages."""
    return browser.execute_async_script(
        "const done = arguments[arguments.length - 1];"
        "const api = typeof browser !== 'undefined' ? browser : window.wrappedJSObject.browser;"
        "api.runtime.sendMessage(arguments[0]).then(done, (e) => done({error: String(e)}));", message)


cleanup()
subprocess.run(["sudo", "touch", REMOTE_CONTROL_MARKER], check=True)
time.sleep(12)
pages = Pages()
try:
    ot("block", "create", "extras")
    ot("block", "add", "extras", "example.org", "youtube.com")
    ot("start", "extras")
    ot("block", "create", "extrawords")
    ot("list", "create", "extrawords", "Words", "--text")
    ot("list", "add", "extrawords", "Words", "kcninewordone", "kcninewordtwo")
    ot("list", "action", "extrawords", "Words", "block", "on")
    ot("list", "test", "extrawords", "on")
    ot("start", "extrawords")
    time.sleep(6)

    driver = start()
    try:
        # The block page says why.
        visit(driver, "https://example.org/")
        check("a blocked page goes to the block page", on_block_page(driver))
        why = text_of(driver, "why") or ""
        check("the block page names the block and the list", "extras" in why and "Blocked" in why)

        # A proxy address carrying a blocked site.
        visit(driver, pages.base + "/plain")
        check("an unrelated local page opens", "/plain" in driver.current_url)
        visit(driver, pages.base + "/go?url=https://example.org/page")
        check("a blocked site inside a proxy address is blocked", on_block_page(driver))
        visit(driver, pages.base + "/go?url=https://example.com/page")
        check("another site inside a proxy address is not", "/go" in driver.current_url)

        # A front-end that says what it is.
        visit(driver, pages.base + "/frontend", wait=8)
        check("an alternative front-end of a blocked site is blocked",
              on_block_page(driver) and "reason=mirror" in driver.current_url)

        # Keyword test mode: the page stays, with the panel.
        visit(driver, pages.base + "/words", wait=6)
        panel = driver.execute_script("return Boolean(document.getElementById('kc9k-test-panel'));")
        check("in test mode the page stays", "/words" in driver.current_url)
        check("in test mode the panel shows", panel)
        ot("list", "test", "extrawords", "off")
        time.sleep(8)
        check("with test mode off the page is blocked", on_block_page(driver))

        # The option hides the reason; the page is blocked all the same.
        ot("option", "block-page-details", "off")
        time.sleep(5)
        visit(driver, "https://example.org/")
        check("with the option off the block page gives no reason",
              on_block_page(driver) and (text_of(driver, "why") or "") == "")
        ot("option", "block-page-details", "on")

        # Time left: a block whose schedule starts in half an hour.
        now = datetime.datetime.now()
        begin = now + datetime.timedelta(minutes=30)
        end = begin + datetime.timedelta(minutes=30)
        if begin.day == now.day and end.day == now.day:
            ot("block", "create", "extrasoon")
            ot("block", "add", "extrasoon", "wikipedia.org")
            ot("schedule", "add", "extrasoon", "--days", begin.strftime("%a").lower(),
               "--from", begin.strftime("%H:%M"), "--to", end.strftime("%H:%M"))
            ot("block", "create", "extraadd")
            time.sleep(6)
            state = ask(driver, {"type": "popup", "url": "https://en.wikipedia.org/wiki/Chicken"})
            left = (state or {}).get("left") or []
            check("the toolbar button knows the time until a schedule blocks the site",
                  len(left) == 1 and left[0]["block"] == "extrasoon" and 27 * 60 <= left[0]["seconds"] <= 30 * 60)
            state = ask(driver, {"type": "popup", "url": "https://example.com/"})
            check("no time left for a site no block covers", (state or {}).get("left") == [])
            check("the toolbar button lists the blocks", "extraadd" in ((state or {}).get("blocks") or []))
            answer = ask(driver, {"type": "add-site", "block": "extraadd", "url": "https://www.example.net/a/b"})
            time.sleep(2)
            check("a site is added to a block from the toolbar button",
                  (answer or {}).get("ok") and "example.net" in output("block", "info", "extraadd"))
        else:
            print("::notice::too close to midnight for the time-left checks")
    finally:
        try:
            driver.quit()
        except WebDriverException:
            pass
finally:
    cleanup()
    pages.server.shutdown()

sys.exit(1 if failures else 0)
