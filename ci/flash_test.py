"""Checks R-PAGE-1 (no flashing) inside a real LibreWolf: a page blocked or
given a look by its text never shows its content without it, on a load and
on an address change without loading; a page no list reads shows at once.
Run on a disposable machine (installed KC9K, TAGS=devmaster):

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/flash_test.py

Each test page records, at every animation frame (just before it is
painted), whether its content is visible (the root's opacity) and the
root's filter, in localStorage; a page of the same site reads them back
after the blocked one is gone.
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
BLOCKS = ("flashblock", "flashlooks")
REMOTE_CONTROL_MARKER = "/var/lib/kc9k/allow-remote-control"


def check(name, passed, detail=""):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name} {detail}")
    failures += 0 if passed else 1


def ot(*args):
    subprocess.run(["kc9k", *args], check=True, stdout=subprocess.DEVNULL)


def ot_quiet(*args):
    subprocess.run(["kc9k", *args], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def cleanup():
    for name in BLOCKS:
        ot_quiet("stop", name)
        ot_quiet("block", "remove", name)
    subprocess.run(["sudo", "rm", "-f", REMOTE_CONTROL_MARKER])


# Records, before each paint, "opacity|filter|has-word" for the page; the
# word is looked for in the DOM, so a frame painted with it visible shows.
RECORDER = """<script>
(function () {
  const key = "frames:" + location.pathname;
  const frames = [];
  const word = /kcnine\\w+word/;
  function tick() {
    const root = document.documentElement;
    const cs = getComputedStyle(root);
    const text = document.body ? document.body.innerText : "";
    frames.push(cs.opacity + "|" + cs.filter + "|" + (word.test(text) ? 1 : 0) + "|" + Math.round(performance.now()));
    try { localStorage.setItem(key, JSON.stringify(frames)); } catch (e) {}
    if (frames.length < 400) requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);
})();
</script>"""

PAGES = {
    # Blocked by its text, the word right there.
    "/blocked": "<!doctype html><title>Blocked</title>" + RECORDER + "<p>a page with kcnineblockword in it</p>",
    # Blocked by text written in a moment later (a script fills it in).
    "/late": "<!doctype html><title>Late</title>" + RECORDER + "<p id='p'>loading</p>"
             "<script>setTimeout(() => { document.getElementById('p').textContent = "
             "'now kcnineblockword shows'; }, 150);</script>",
    # Greyed by its text.
    "/grey": "<!doctype html><title>Grey</title>" + RECORDER + "<p>some kcninegreyword here</p>",
    # A page with nothing to find.
    "/plain": "<!doctype html><title>Plain</title>" + RECORDER + "<p>nothing here</p>",
    # A page that changes address without loading, and then shows the word.
    "/spa": "<!doctype html><title>App</title>" + RECORDER + "<p id='p'>start page</p>"
            "<script>window.go = () => { history.pushState({}, '', '/spa/next'); "
            "document.getElementById('p').textContent = 'next has kcnineblockword'; };"
            "window.grey = () => { history.pushState({}, '', '/spa/grey'); "
            "document.getElementById('p').textContent = 'next has kcninegreyword'; };</script>",
    "/reader": "<!doctype html><title>Reader</title><p>reads the frames</p>",
}


class Pages:
    def __init__(self):
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                path = self.path.split("?")[0]
                body = PAGES.get("/spa" if path.startswith("/spa") else path, "<!doctype html><title>?</title>").encode()
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


def visit(browser, url, wait=4):
    try:
        browser.get(url)
    except WebDriverException:
        pass
    time.sleep(wait)


def frames(browser, base, path):
    """The frames a page recorded, read from another page of its site."""
    visit(browser, base + "/reader", 1)
    raw = browser.execute_script("return localStorage.getItem(arguments[0]);", "frames:" + path)
    browser.execute_script("localStorage.removeItem(arguments[0]);", "frames:" + path)
    return [f.split("|") for f in json.loads(raw or "[]")]


def shown_with_word(recorded):
    """Frames painted with the word visible (opacity not 0)."""
    return [f for f in recorded if f[0] != "0" and f[2] == "1"]


cleanup()
subprocess.run(["sudo", "touch", REMOTE_CONTROL_MARKER], check=True)
pages = Pages()
browser = None
try:
    ot("block", "create", "flashblock")
    ot("list", "create", "flashblock", "Words", "--text")
    ot("list", "add", "flashblock", "Words", "kcnineblockword")
    ot("start", "flashblock")
    ot("block", "create", "flashlooks")
    ot("list", "create", "flashlooks", "Grey", "--text")
    ot("list", "add", "flashlooks", "Grey", "kcninegreyword")
    ot("list", "action", "flashlooks", "Grey", "greyscale", "on")
    ot("list", "action", "flashlooks", "Grey", "block", "off")
    ot("start", "flashlooks")
    time.sleep(8)
    browser = start()

    visit(browser, pages.base + "/blocked")
    check("a page with a blocked word goes to the block page", browser.current_url.startswith("moz-extension:"),
          browser.current_url)
    rec = frames(browser, pages.base, "/blocked")
    check("the blocked page never painted its word", rec and not shown_with_word(rec), f"{len(rec)} frames: {rec[:8]}")

    visit(browser, pages.base + "/late")
    check("a word written in later still blocks", browser.current_url.startswith("moz-extension:"), browser.current_url)
    rec = frames(browser, pages.base, "/late")
    check("the late word was never painted", rec and not shown_with_word(rec), f"{len(rec)} frames: {rec[:12]}")

    visit(browser, pages.base + "/grey")
    rec = frames(browser, pages.base, "/grey")
    ungrey = [f for f in rec if f[0] != "0" and "grayscale" not in f[1]]
    check("a greyed page is never painted in colour", rec and not ungrey, f"{len(rec)} frames: {ungrey[:5]}")
    check("the greyed page is shown in the end", any(f[0] != "0" for f in rec), str(rec[-3:]))

    visit(browser, pages.base + "/plain", 2)
    rec = frames(browser, pages.base, "/plain")
    hidden = [f for f in rec if f[0] == "0"]
    # Shown within about half a second (SETTLE_MS) plus reading: frames at
    # ~60 a second.
    check("a page without the words is shown soon", rec and any(f[0] != "0" for f in rec) and len(hidden) < 60,
          f"{len(hidden)} hidden frames of {len(rec)}")

    # An address change without loading.
    visit(browser, pages.base + "/spa", 2)
    browser.execute_script("localStorage.removeItem('frames:/spa'); window.go();")
    time.sleep(3)
    check("an address change to a blocked word goes to the block page", browser.current_url.startswith("moz-extension:"),
          browser.current_url)
    rec = frames(browser, pages.base, "/spa")
    check("after an address change the blocked word is never painted", rec and not shown_with_word(rec),
          f"{len(rec)} frames: {shown_with_word(rec)[:5]} ... {shown_with_word(rec)[-3:]}; all: {[f[0]+f[2]+'@'+f[3] for f in rec][-40:]}")
    visit(browser, pages.base + "/spa", 2)
    browser.execute_script("window.grey();")
    time.sleep(2)
    painted = browser.execute_script("const f = JSON.parse(localStorage.getItem('frames:/spa') || '[]');"
                                     "return f.filter(x => x.split('|')[2] === '1');")
    ungrey = [f for f in painted if not f.startswith("0|") and "grayscale" not in f]
    check("after an address change the word is never painted in colour", painted and not ungrey,
          f"{len(painted)} frames with the word: {ungrey[:5]}")
finally:
    if browser:
        try:
            browser.quit()
        except WebDriverException:
            pass
    cleanup()
    pages.server.shutdown()

print(f"{failures} failures")
sys.exit(1 if failures else 0)
