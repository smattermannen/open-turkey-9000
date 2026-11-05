"""Checks the extension inside a real LibreWolf. Run on a disposable machine."""
import os
import subprocess
import sys
import time

from selenium import webdriver
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

failures = 0


def check(name, passed):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}")
    failures += 0 if passed else 1


def ot(*args):
    subprocess.run(["open-turkey", *args], check=True)


ot("block", "create", "browsertest")
ot("block", "add", "browsertest", "*example.org/secret*")
ot("block", "add-exception", "browsertest", "example.org/secret/allowed")
ot("start", "browsertest")
ot("browser", "greyscale", "on")
ot("browser", "images", "on")
ot("browser", "web-only", "on")
time.sleep(8)

def start(signatures_required=True):
    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    options.add_argument("-remote-allow-system-access")
    if not signatures_required:
        options.set_preference("xpinstall.signatures.required", False)
    service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"))
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


for label, required in (("signed-default", True), ("signatures-off", False)):
    try:
        probe = start(required)
        try:
            report(probe, label)
        finally:
            probe.quit()
    except Exception as error:
        print("::notice::" + label + " failed: " + str(error)[:300].replace(chr(10), " "))

driver = start(False)
try:
    try:
        driver.install_addon("/usr/share/open-turkey/open-turkey.xpi", temporary=True)
    except Exception as error:
        print("::error::temporary install failed: " + str(error)[:300].replace(chr(10), " "))
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
    check("search is forced to web results", "udm=14" in driver.current_url)
finally:
    driver.quit()

sys.exit(failures)
