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
        "find /usr -type d -name distribution 2>/dev/null; curl -s 127.0.0.1:47613/rules",
        shell=True, capture_output=True, text=True).stdout
    for line in diag.splitlines():
        print("::notice::diag " + line)
    driver.get("about:policies");time.sleep(1)
    print("::notice::policies " + driver.find_element("tag name", "body").text[:300].replace("
", " | "))
    driver.get("about:support");time.sleep(2)
    print("::notice::addons " + driver.execute_script(
        "return [...document.querySelectorAll('#addons-tbody tr')].map(r=>r.innerText).join(' ; ')")[:400].replace("
", " "))
    print("::notice::version " + driver.capabilities.get("browserVersion", "?"))
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
