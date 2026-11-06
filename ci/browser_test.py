"""Checks the extension inside a real LibreWolf. Run on a disposable machine."""
import os
import subprocess
import sys
import time
from urllib.parse import unquote

from selenium import webdriver
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

failures = 0


def check(name, passed):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}")
    failures += 0 if passed else 1


def ot(*args):
    subprocess.run(["kc9k", *args], check=True)


def ot_quiet(*args):
    subprocess.run(["kc9k", *args], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def browser_settings():
    out = subprocess.run(["kc9k", "browser"], check=True, capture_output=True, text=True).stdout
    return dict(line.split() for line in out.splitlines() if line.strip())


def remove_test_block():
    ot_quiet("stop", "browsertest")
    ot_quiet("block", "remove", "browsertest")


def cleanup(saved):
    remove_test_block()
    for name, value in saved.items():
        ot_quiet("browser", name, value)
    subprocess.run(["pkill", "-f", "^/tmp/chromium"], stderr=subprocess.DEVNULL)
    if os.path.exists("/tmp/chromium"):
        os.remove("/tmp/chromium")


# Leave the machine as we found it so the test can run again on real hardware.
remove_test_block()
saved_settings = browser_settings()

def start(signatures_required=True):
    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    if not signatures_required:
        options.set_preference("xpinstall.signatures.required", False)
    service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"),
                      service_args=["--allow-system-access"])
    browser = webdriver.Firefox(options=options, service=service)
    browser.set_page_load_timeout(40)
    return browser


def report(browser, label):
    time.sleep(10)
    with browser.context(browser.CONTEXT_CHROME):
        policies = browser.execute_script(
            "return Services.policies.status + ' ' + Object.keys(Services.policies.getActivePolicies() || {}).join(',')")
        addons = browser.execute_async_script("""
            const done = arguments[arguments.length - 1];
            const {AddonManager} = ChromeUtils.importESModule("resource://gre/modules/AddonManager.sys.mjs");
            AddonManager.getAllAddons().then(list => done(list.map(
                a => a.id + ' active=' + a.isActive + ' sign=' + a.signedState + ' disabled=' + a.userDisabled + ' ' + (a.appDisabled ? 'appDisabled' : '')).join(' ; ')));
        """)
    print(f"::notice::{label} policies: {policies}"[:600])
    print(f"::notice::{label} addons: {addons}"[:600])


try:
    ot("block", "create", "browsertest")
    ot("block", "add", "browsertest", "*example.org/secret*")
    ot("block", "add-exception", "browsertest", "example.org/secret/allowed")
    ot("start", "browsertest")
    ot("browser", "greyscale", "on")
    ot("browser", "images", "on")
    ot("browser", "web-only", "on")
    time.sleep(8)

    for label, required in (("default-profile", True),):
        try:
            probe = start(required)
            try:
                report(probe, label)
            finally:
                probe.quit()
        except Exception as error:
            print("::notice::" + label + " failed: " + str(error)[:300].replace(chr(10), " "))

    driver = start()
    try:
        time.sleep(10)
        driver.get("https://example.org/secret/page")
        time.sleep(2)
        print("blocked url:", driver.current_url)
        check("keyword rule blocks in LibreWolf", driver.current_url.startswith("moz-extension:"))

        driver.get("https://example.org/secret/allowed")
        time.sleep(2)
        check("exception opens the page", "example.org" in driver.current_url)
        styles = driver.execute_script(
            "return getComputedStyle(document.documentElement).filter")
        print("filter:", styles)
        check("greyscale applies", "grayscale" in styles)

        driver.get("https://www.google.com/search?q=test&tbm=isch")
        time.sleep(3)
        print("search url:", driver.current_url)
        check("search is forced to web results", "udm=14" in unquote(driver.current_url))
    finally:
        driver.quit()

    ot("browser", "safesearch", "on")
    ot("browser", "force-search", "on")
    ot("browser", "force-librewolf", "on")
    time.sleep(8)
    hosts = open("/etc/hosts").read()
    check("SafeSearch entry in hosts", " www.google.com" in hosts)
    resolved = subprocess.run("getent ahostsv4 www.google.com | head -1", shell=True, capture_output=True, text=True).stdout
    safe = subprocess.run("getent ahostsv4 forcesafesearch.google.com | head -1", shell=True, capture_output=True, text=True).stdout
    print("::notice::resolved " + resolved.strip() + " safe " + safe.strip())
    check("Google resolves to SafeSearch", resolved.split()[:1] == safe.split()[:1] or "216.239.38.120" in resolved)

    subprocess.run(["cp", "/bin/sleep", "/tmp/chromium"], check=True)
    other = subprocess.Popen(["/tmp/chromium", "60"])
    time.sleep(8)
    check("other browsers are closed", other.poll() is not None)

    driver = start()
    try:
        time.sleep(10)
        check("LibreWolf survives forcing", True)
        driver.get("https://www.wikipedia.org/")
        time.sleep(4)
        loaded = driver.execute_script(
            "return [...document.images].filter(i => i.complete && i.naturalWidth > 0).length")
        total = driver.execute_script("return document.images.length")
        print(f"::notice::images loaded {loaded} of {total}")
        check("images are blocked", total > 0 and loaded == 0)
        with driver.context(driver.CONTEXT_CHROME):
            engine = driver.execute_async_script("""
                const done = arguments[arguments.length - 1];
                // Newer Firefox builds dropped Services.search for a moz-src module.
                const search = Services.search || ChromeUtils.importESModule(
                    "moz-src:///toolkit/components/search/SearchService.sys.mjs").SearchService;
                Promise.resolve(search.init && search.init())
                    .then(() => search.getDefault())
                    .then(e => done(e.name), e => done("error: " + e));
            """)
        print("::notice::default engine " + engine)
        check("Google is the forced search engine", engine == "Google Web")
    finally:
        driver.quit()
finally:
    cleanup(saved_settings)

sys.exit(failures)
