"""Checks that a quota session spends time only while the block's websites
are in use, inside a real LibreWolf: a page playing muted audio counts, a
page that doesn't match the block doesn't. (The focused tab only counts
while someone is at the computer, so it is left out: this runs unattended.)
Run on a disposable machine, like browser_test.py:

    GECKOWEBDRIVER=<dir with geckodriver> python3 ci/quota_test.py
"""
import http.server
import io
import os
import struct
import subprocess
import sys
import tempfile
import threading
import time
import wave

from selenium import webdriver
from selenium.common.exceptions import WebDriverException
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service

failures = 0
REMOTE_CONTROL_MARKER = "/var/lib/kc9k/allow-remote-control"
PORT = 47699


def check(name, passed):
    global failures
    print(f"PASS: {name}" if passed else f"::error::FAIL: {name}")
    failures += 0 if passed else 1


def ot(*args):
    subprocess.run(["kc9k", *args], check=True)


def cleanup():
    # A quota can only be removed at midnight, and keeps its block on until
    # then; the block 'quotatest' only blocks addresses with "kc9kquota", so
    # it stays (and is reused by the next run) until it can be removed.
    for args in (("quota", "end", "quotatest"), ("quota", "remove", "quotatest"),
                 ("stop", "quotatest"), ("block", "remove", "quotatest")):
        subprocess.run(["kc9k", *args], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["sudo", "rm", "-f", REMOTE_CONTROL_MARKER])


def used_seconds():
    out = subprocess.run(["kc9k", "gui-state"], capture_output=True, text=True, check=True).stdout
    import json
    for block in json.loads(out)["blocks"]:
        if block["name"] == "quotatest":
            return block["quota_used"]
    return None


def silent_wav(seconds=2):
    out = io.BytesIO()
    with wave.open(out, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(8000)
        w.writeframes(struct.pack("<h", 0) * 8000 * seconds)
    return out.getvalue()


# A local site: one page plays a muted, looping sound; another is quiet.
site = tempfile.mkdtemp()
with open(os.path.join(site, "silence.wav"), "wb") as f:
    f.write(silent_wav())
with open(os.path.join(site, "kc9kquota.html"), "w") as f:
    f.write('<!doctype html><title>quota</title><audio src="silence.wav" autoplay loop muted></audio>')
with open(os.path.join(site, "other.html"), "w") as f:
    f.write('<!doctype html><title>other</title><audio src="silence.wav" autoplay loop muted></audio>')
handler = lambda *a, **k: http.server.SimpleHTTPRequestHandler(*a, directory=site, **k)
server = http.server.ThreadingHTTPServer(("127.0.0.1", PORT), handler)
threading.Thread(target=server.serve_forever, daemon=True).start()

cleanup()
subprocess.run(["sudo", "touch", REMOTE_CONTROL_MARKER], check=True)
time.sleep(12)  # the marker changes LibreWolf's settings; let the daemon notice
try:
    if subprocess.run(["kc9k", "block", "info", "quotatest"], capture_output=True).returncode != 0:
        ot("block", "create", "quotatest")
    ot("list", "add", "quotatest", "Blocked", "*kc9kquota*")
    ot("quota", "set", "quotatest", "30m")
    ot("quota", "start", "quotatest")

    options = Options()
    options.binary_location = "/usr/bin/librewolf"
    options.add_argument("-headless")
    options.set_preference("media.autoplay.default", 0)  # LibreWolf blocks autoplay, even muted
    service = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"),
                      service_args=["--allow-system-access"])
    driver = webdriver.Firefox(options=options, service=service)
    try:
        time.sleep(15)
        driver.get(f"http://localhost:{PORT}/other.html")
        start = used_seconds()
        time.sleep(30)
        idle = used_seconds() - start
        print("spent on another page:", idle)
        check("another page spends nothing", idle == 0)

        driver.get(f"http://localhost:{PORT}/kc9kquota.html")
        playing = driver.execute_script("const a = document.querySelector('audio'); return !a.paused")
        print("audio playing:", playing)
        start = used_seconds()
        time.sleep(40)
        spent = used_seconds() - start
        print("spent while the block's page plays muted audio:", spent)
        check("a muted page of the block spends the quota", 15 <= spent <= 50)
    finally:
        try:
            driver.quit()
        except WebDriverException:
            pass
finally:
    cleanup()
    server.shutdown()

sys.exit(failures)
