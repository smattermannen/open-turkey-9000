"""Checks, in a real LibreWolf, that "Show dark web pages while dark mode is
on" (SPEC §4.10) turns on the browser's own dark mode: a site with its own
dark design shows it, a light page without one is shown dark, and with
dark mode off the site is light again. Without KC9K installed
(ci/fake_daemon.py stands in for the daemon; the box has
ci/circadian_test.py): LibreWolf gets the preferences KC9K writes into
librewolf.cfg (blocker.darkPrefs, pinned by TestDarkPrefsAreDefaults). It
also shows why they are needed: with LibreWolf's own settings every site is
told light.

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/dark_browser_test.py

As a normal user. LIBREWOLF picks the browser (default /usr/bin/librewolf).
"""
import http.server
import time

from fake_daemon import check, finish, librewolf, pack, serve, start_daemon, state, wait_polls

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


start_daemon()
base = f"http://127.0.0.1:{serve(Site).server_address[1]}"
xpi = pack()

# Dark pages on, dark mode on, with KC9K's preferences.
state["rules"] = {"lists": [], "dark": DARK_ON}
driver = librewolf(KC9K_PREFS, xpi)
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
driver = librewolf({}, xpi)
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
driver = librewolf(dict(KC9K_PREFS, **{"ui.systemUsesDarkTheme": 1}), xpi)
try:
    check("dark desktop: LibreWolf's window is dark", browser_dark(driver))
    page = look(driver, "/own-dark")
    check("dark desktop: the site shows its own dark design", page["dark"] and page["background"] == "rgb(17, 17, 17)", page)
    check("dark desktop: a site dark by itself isn't turned over", not page["inverted"], page)
finally:
    driver.quit()

finish("dark browser test")
