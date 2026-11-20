"""Checks, inside a real LibreWolf, what extension 1.13 added: the reason on
the block page (and the option that hides it), sites reached through a proxy
address or a front-end, keyword test mode, the time left for the toolbar
button, and adding a site from it. Run on a disposable machine:

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/extras_test.py
"""
import http.server
import io
import math
import wave
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
BLOCKS = ("extras", "extrawords", "extrasoon", "extraadd", "extralooks", "extrasocial", "extraplay", "extraembed")
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
        ot_quiet("pomodoro", "off", name)
        ot_quiet("list", "test", name, "off")
        ot_quiet("stop", name)
        ot_quiet("block", "remove", name)
    ot_quiet("behaviour", "block-page-details", "on")
    subprocess.run(["sudo", "rm", "-f", REMOTE_CONTROL_MARKER])


PAGES = {
    "/frontend": '<!doctype html><meta property="og:site_name" content="Invidious"><title>Videos</title><p>hello</p>',
    "/plain": "<!doctype html><title>Plain</title><p>nothing here</p>",
    "/words": "<!doctype html><title>Words</title><p>kcninewordone and kcninewordtwo, kcninewordone again</p>",
    "/go": "<!doctype html><title>Proxy</title><p>a proxy page</p>",
    "/looks": "<!doctype html><title>Looks</title><body style='background:#fff'><aside id='side'>side</aside>"
              "<p>kcninelooksword here</p><video id='v'></video></body>",
    # A page with an embedded player from another site (localhost against
    # 127.0.0.1), with and without a word that takes its video away.
    "/embed": "<!doctype html><title>Embed</title><p>kcnineembedword</p>"
              "<iframe id='f' src='FRAME/frame'></iframe>",
    "/embedplain": "<!doctype html><title>Embed plain</title><p>nothing here</p>"
                   "<iframe id='f' src='FRAME/frame'></iframe>",
    "/frame": "<!doctype html><title>Player</title><video id='fv' src='/tone.wav' autoplay loop></video>",
    # no-images from page text, then a page without the word: its picture
    # shows once it is read (§3.9: looks change, don't block).
    "/pics": "<!doctype html><title>Pics</title><p>kcninepicsword</p><img id='i' src='/dot.gif'>",
    "/picsafter": "<!doctype html><title>After</title><p>nothing here</p><img id='i' src='/dot.gif?after'>",
}


def tone():
    """Two seconds of a 440 Hz tone as a WAV file."""
    out = io.BytesIO()
    with wave.open(out, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(8000)
        w.writeframes(b"".join(int(8000 * math.sin(2 * math.pi * 440 * i / 8000)).to_bytes(2, "little", signed=True)
                               for i in range(16000)))
    return out.getvalue()


# A 1x1 GIF.
DOT_GIF = bytes.fromhex("47494638396101000100800000ff000000000021f90401000000002c00000000010001000002024401003b")
FILES = {"/tone.wav": ("audio/wav", tone()), "/dot.gif": ("image/gif", DOT_GIF)}


class Pages:
    def __init__(self):
        frame_base = ""

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                path = self.path.split("?")[0]
                kind, body = FILES.get(path) or ("text/html", PAGES.get(path, "<!doctype html><title>?</title>")
                                                 .replace("FRAME", frame_base).encode())
                self.send_response(200)
                self.send_header("Content-Type", kind)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        frame_base = f"http://localhost:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()


def start():
    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    # Let embedded players start on their own, sound and all.
    options.set_preference("media.autoplay.default", 0)
    options.set_preference("media.autoplay.blocking_policy", 0)
    service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"),
                      service_args=["--allow-system-access"])
    browser = webdriver.Firefox(options=options, service=service)
    browser.set_page_load_timeout(40)
    time.sleep(15)
    return browser


# A bypass detection set for the whole computer counts the pages this test
# gets blocked. Pace them, so the test doesn't trip it (and start monk mode
# on the machine it runs on).
PACE = 20 if "step 1" in output("nepsis").split("block ")[0] else 0


def visit(browser, url, wait=5):
    try:
        browser.get(url)
    except WebDriverException:
        pass
    time.sleep(wait)
    try:
        if PACE and browser.current_url.startswith("moz-extension:"):
            time.sleep(PACE)
    except WebDriverException:
        pass


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
        extension = driver.current_url.split("/blocked.html")[0]
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

        # A single-page site (YouTube) changes its address without loading:
        # a URL list still catches it.
        ot("block", "add", "extras", "example.net/kcninespa")
        time.sleep(6)
        visit(driver, "https://example.net/")
        check("a page outside the URL entry opens", "example.net" in driver.current_url and not on_block_page(driver))
        driver.execute_script("history.pushState({}, '', '/kcninespa/watch');"
                              "document.body.appendChild(document.createElement('p')).textContent = 'next video';")
        time.sleep(6)
        check("an address changed without loading is caught by a URL list", on_block_page(driver))

        # Keyword test mode: the page stays, with the panel.
        visit(driver, pages.base + "/words", wait=6)
        panel = driver.execute_script("return Boolean(document.getElementById('kc9k-test-panel'));")
        check("in test mode the page stays", "/words" in driver.current_url)
        check("in test mode the panel shows", panel)
        ot("list", "test", "extrawords", "off")
        time.sleep(8)
        check("with test mode off the page is blocked", on_block_page(driver))

        # The option hides the reason; the page is blocked all the same.
        ot("behaviour", "block-page-details", "off")
        time.sleep(5)
        visit(driver, "https://example.org/")
        check("with the option off the block page gives no reason",
              on_block_page(driver) and (text_of(driver, "why") or "") == "")
        ot("behaviour", "block-page-details", "on")

        # Actions that change the page: dark, no video.
        ot("block", "create", "extralooks")
        ot("list", "create", "extralooks", "Looks", "--text")
        ot("list", "add", "extralooks", "Looks", "kcninelooksword")
        for action in ("dark", "no-video", "mute"):
            ot("list", "action", "extralooks", "Looks", action, "on")
        ot("list", "action", "extralooks", "Looks", "block", "off")
        ot("start", "extralooks")
        time.sleep(6)
        visit(driver, pages.base + "/looks", wait=6)
        css = driver.execute_script(
            "const s = document.getElementById('kc9k-page-style'); return s ? s.textContent : '';")
        check("the page stays for actions that only change it", "/looks" in driver.current_url)
        check("dark turns a light page over", "invert(1)" in css)
        check("no-video hides videos",
              driver.execute_script("return getComputedStyle(document.getElementById('v')).display") == "none")
        ot("stop", "extralooks")

        # no-images from page text is carried to the next page until it is
        # read; a next page without the word then shows its pictures.
        ot("list", "create", "extralooks", "Pics", "--text")
        ot("list", "add", "extralooks", "Pics", "kcninepicsword")
        ot("list", "action", "extralooks", "Pics", "no-images", "on")
        ot("list", "action", "extralooks", "Pics", "block", "off")
        ot("start", "extralooks")
        time.sleep(6)
        picture = ("const i = document.getElementById('i');"
                   "return [i.complete && i.naturalWidth > 0, getComputedStyle(i).visibility];")
        visit(driver, pages.base + "/pics", wait=6)
        hidden = driver.execute_script(picture)
        print("picture on the page with the word:", hidden)
        check("no-images from page text hides a picture", hidden[1] == "hidden")
        driver.execute_script("location.href = arguments[0];", pages.base + "/picsafter")
        time.sleep(8)
        shown = driver.execute_script(picture)
        print("picture on the next page:", shown)
        check("the next page without the word shows its picture once read", shown == [True, "visible"])
        ot("stop", "extralooks")

        # An embedded video plays on through no-video and mute from a word in
        # the page around it unless every frame of the tab is told.
        ot("block", "create", "extraembed")
        ot("list", "create", "extraembed", "Embed", "--text")
        ot("list", "add", "extraembed", "Embed", "kcnineembedword")
        for action in ("no-video", "mute"):
            ot("list", "action", "extraembed", "Embed", action, "on")
        ot("list", "action", "extraembed", "Embed", "block", "off")
        ot("start", "extraembed")
        time.sleep(6)

        def frame_video(frame_id="f"):
            """The embedded video's display and whether it is paused, or None."""
            try:
                driver.switch_to.frame(driver.find_element("id", frame_id))
                return driver.execute_script(
                    "const v = document.getElementById('fv');"
                    "return v ? [getComputedStyle(v).display, v.paused] : null;")
            except WebDriverException as error:
                print("frame:", str(error)[:120])
                return None
            finally:
                driver.switch_to.default_content()

        def tab_muted(part):
            """Whether the extension sees the tab showing part muted (asked from a second tab)."""
            main = driver.current_window_handle
            driver.switch_to.new_window("tab")
            try:
                visit(driver, extension + "/popup.html", wait=2)
                tabs = driver.execute_async_script(
                    "const done = arguments[arguments.length - 1];"
                    "const api = typeof browser !== 'undefined' ? browser : window.wrappedJSObject.browser;"
                    "api.tabs.query({}).then((t) => done(t.map((x) => [x.url, Boolean(x.mutedInfo && x.mutedInfo.muted)])),"
                    " (e) => done([]));")
            finally:
                driver.close()
                driver.switch_to.window(main)
            return any(part in url and muted for url, muted in tabs or [])

        visit(driver, pages.base + "/embedplain", wait=6)
        plain = frame_video()
        print("embedded video without the word:", plain)
        check("an embedded video plays on a page without the word",
              plain is not None and plain[0] != "none" and plain[1] is False)
        visit(driver, pages.base + "/embed", wait=8)
        hit = frame_video()
        print("embedded video with the word:", hit)
        check("no-video from page text stops a video in a frame from another site",
              hit is not None and hit[0] == "none" and hit[1] is True)
        check("mute from page text mutes the whole tab", tab_muted("/embed"))
        driver.execute_script(
            "const f = document.createElement('iframe'); f.id = 'late';"
            "f.src = arguments[0] + '/frame'; document.body.appendChild(f);",
            pages.base.replace("127.0.0.1", "localhost"))
        time.sleep(5)
        late = frame_video("late")
        print("frame added later:", late)
        check("a frame added after the match asks and takes no-video",
              late is not None and late[0] == "none" and late[1] is True)
        ot("stop", "extraembed")

        # Social media settings: Shorts are sent to the front page.
        ot("stop", "extras")  # it stops youtube.com, which the next checks need
        ot("block", "create", "extrasocial")
        ot("social", "extrasocial", "youtube:no-shorts", "on")
        ot("start", "extrasocial")
        time.sleep(6)
        visit(driver, "https://www.youtube.com/shorts/kc9ktest", wait=8)
        print("shorts went to:", driver.current_url[:80])
        check("YouTube Shorts are sent elsewhere", "/shorts/" not in driver.current_url
              and not on_block_page(driver))
        ot("stop", "extrasocial")

        # A playlist let through a block that stops YouTube.
        ot("block", "create", "extraplay")
        ot("block", "add", "extraplay", "youtube.com")
        ot("social", "extraplay", "playlist", "PLkc9ktestplaylist00", "on")
        ot("start", "extraplay")
        time.sleep(6)
        visit(driver, "https://www.youtube.com/", wait=6)
        check("YouTube itself is blocked", on_block_page(driver))
        visit(driver, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", wait=6)
        check("a video outside the playlist is blocked",
              on_block_page(driver) and "reason=playlist" in driver.current_url)
        visit(driver, "https://www.youtube.com/playlist?list=PLkc9ktestplaylist00", wait=8)
        print("playlist page:", driver.current_url[:80])
        check("the playlist's own page opens", not on_block_page(driver))
        ot("stop", "extraplay")

        # Time left: a block in its Pomodoro break comes back when the break
        # ends. (A schedule would do too, but it holds its block for two
        # hours before it starts, and the test couldn't remove it again.)
        if True:
            ot("block", "create", "extrasoon")
            ot("block", "add", "extrasoon", "wikipedia.org")
            ot("pomodoro", "set", "extrasoon", "10s", "10m")
            ot("start", "extrasoon")
            ot("block", "create", "extraadd")
            time.sleep(16)
            visit(driver, extension + "/popup.html", wait=2)  # messages go from one of its pages
            state = ask(driver, {"type": "popup", "url": "https://en.wikipedia.org/wiki/Chicken"})
            left = (state or {}).get("left") or []
            check("the toolbar button knows the time until a break ends and the site is blocked",
                  len(left) == 1 and left[0]["block"] == "extrasoon" and left[0]["kind"] == "break"
                  and 8 * 60 <= left[0]["seconds"] <= 10 * 60)
            state = ask(driver, {"type": "popup", "url": "https://example.com/"})
            check("no time left for a site no block covers", (state or {}).get("left") == [])
            check("the toolbar button lists the blocks", "extraadd" in ((state or {}).get("blocks") or []))
            answer = ask(driver, {"type": "add-site", "block": "extraadd", "url": "https://www.example.net/a/b"})
            time.sleep(2)
            check("a site is added to a block from the toolbar button",
                  (answer or {}).get("ok") and "example.net" in output("block", "info", "extraadd"))
    finally:
        try:
            driver.quit()
        except WebDriverException:
            pass
finally:
    cleanup()
    pages.server.shutdown()

sys.exit(1 if failures else 0)
