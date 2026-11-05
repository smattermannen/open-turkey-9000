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

options = Options()
options.binary_location = "/usr/bin/librewolf"
options.add_argument("-headless")
service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"))
driver = webdriver.Firefox(options=options, service=service)
driver.set_page_load_timeout(40)
try:
    time.sleep(10)
    diag = subprocess.run(
        "ls -d /usr/lib/librewolf* /usr/share/librewolf* /opt/librewolf* 2>&1; "
        "readlink -f /usr/bin/librewolf; ls -l /etc/librewolf/policies /usr/share/open-turkey 2>&1; "
        "curl -s 127.0.0.1:47613/rules",
        shell=True, capture_output=True, text=True).stdout
    print("::notice::diag " + " ; ".join(diag.splitlines())[-900:])
    print("::notice::version " + driver.capabilities.get("browserVersion", "?"))
    for page, script in (
            ("about:policies", "return document.body.innerText"),
            ("about:support", "return [...document.querySelectorAll('#addons-tbody tr')].map(r=>r.innerText).join(' ; ')")):
        try:
            driver.get(page)
            time.sleep(2)
            print("::notice::" + page + " " + driver.execute_script(script)[:500].replace(chr(10), " | "))
        except Exception as error:
            print("::notice::" + page + " failed " + str(error)[:200].replace(chr(10), " "))
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
