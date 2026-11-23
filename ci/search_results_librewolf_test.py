"""R-PAGE-2 in a real LibreWolf with the real extension: the results pages of
ci/search_results/ (markup modelled on each engine's own) at each engine's
address, served here through a local proxy (plain http: nothing leaves the
machine), the rules from ci/fake_daemon.py's stand-in for the daemon.
Results leading to Reddit or X are gone, the others shown; with the block
off, all are back. ci/search_results_test.js checks the same in Chromium,
frame by frame.

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/search_results_librewolf_test.py

As a normal user, where no KC9K daemon runs.
"""
import http.server
import json
import os
import time

from fake_daemon import HERE, check, finish, librewolf, pack, serve, start_daemon, state, wait_polls

FIXTURES = os.path.join(HERE, "search_results")
PAGES = {name: url.replace("https://", "http://", 1)
         for name, url in json.load(open(os.path.join(FIXTURES, "pages.json"))).items() if name != "_"}
# As ci/search_results_test.js: Reddit and X taken away, r/allowed spared,
# a site only greyed out.
LISTS = [
    {"id": 1, "block": "Social", "name": "Sites", "url": ["reddit.com", "x.com"], "title": [], "text": [],
     "actions": [{"action": "block", "distinct": 1, "hits": 1}],
     "except": {"url": ["reddit.com/r/allowed"], "title": [], "text": []}},
    {"id": 2, "block": "Looks", "name": "Grey", "url": ["looks.example"], "title": [], "text": [],
     "actions": [{"action": "greyscale", "distinct": 1, "hits": 1}], "except": {"url": [], "title": [], "text": []}},
]


class Proxy(http.server.BaseHTTPRequestHandler):
    """An HTTP proxy that knows only the engines' results pages."""

    def do_GET(self):
        name = next((n for n, url in PAGES.items() if self.path == url), None)
        body = open(os.path.join(FIXTURES, name + ".html"), "rb").read() if name else b""
        self.send_response(200 if name else 404)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


MARKS = """return [...document.querySelectorAll('[data-expect]')].map((el) => ({
    expect: el.dataset.expect, shown: el.getClientRects().length > 0, what: el.textContent.trim().slice(0, 50)}));"""

def settled(want, seconds=10):
    """The marks once want(marks) holds, or as they are after seconds: a
    slower machine (the CI runner's first pages) takes longer than fixed
    pauses allowed (A17). The end state is what is checked; that a result
    is never painted is ci/search_results_test.js's check."""
    end = time.time() + seconds
    while True:
        marks = driver.execute_script(MARKS)
        if want(marks) or time.time() > end:
            return marks
        time.sleep(0.2)


def as_expected(marks):
    return marks and all(m["shown"] == (m["expect"] == "kept") for m in marks)


start_daemon()
proxy = serve(Proxy)
state["rules"] = {"lists": LISTS}
driver = librewolf({
    "network.proxy.type": 1, "network.proxy.http": "127.0.0.1", "network.proxy.http_port": proxy.server_address[1],
    "dom.security.https_only_mode": False, "network.stricttransportsecurity.preloadlist": False,
}, pack())
try:
    for name, url in PAGES.items():
        state["rules"] = {"lists": LISTS}
        wait_polls(1)
        driver.get(url)
        time.sleep(1.5)  # DuckDuckGo adds more results after 400 ms
        marks = settled(as_expected)
        gone = [m for m in marks if m["expect"] == "gone"]
        kept = [m for m in marks if m["expect"] == "kept"]
        check(f"{name}: results leading to Reddit or X are gone ({len(gone)})",
              gone and not any(m["shown"] for m in gone), [m for m in gone if m["shown"]])
        check(f"{name}: the others are shown ({len(kept)})", kept and all(m["shown"] for m in kept),
              [m for m in kept if not m["shown"]])
        state["rules"] = {"lists": []}
        wait_polls(2)
        after = settled(lambda marks: all(m["shown"] for m in marks))
        check(f"{name}: with the block off every result is back", all(m["shown"] for m in after),
              [m for m in after if not m["shown"]])
finally:
    driver.quit()

finish("search results (LibreWolf)")
