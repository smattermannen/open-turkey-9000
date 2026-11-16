"""Browser test for the no-downloads action (SPEC §3.9): a download from a
page whose text matches a list with no-downloads is cancelled; one from
another page is not. Uses the installed KC9K (TAGS=devmaster) and LibreWolf.

    GECKOWEBDRIVER=~/.local/share/kc9k-browsertest \
      ~/.local/share/kc9k-browsertest/venv/bin/python ci/download_test.py
"""
import subprocess
import http.server, os, shutil, sys, tempfile, threading, time
from selenium import webdriver
from selenium.webdriver.firefox.options import Options
from selenium.webdriver.firefox.service import Service
from selenium.webdriver.common.by import By

class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def do_GET(self):
        if self.path.startswith("/file"):
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("Content-Disposition", "attachment; filename=kc9k-%s.bin" % self.path[5:].strip("?") )
            self.end_headers(); self.wfile.write(b"x" * 1000); return
        word = "kcninedownload" if self.path.startswith(("/bad", "/cdn", "/blob")) else "harmless"
        tag = self.path[1:5].strip("?/")
        # /cdn: the file comes from another host (a CDN), so the browser
        # sends only the page's origin as referrer; /blob: the page makes
        # the file itself (no address of its own).
        href = (OTHER + f"/file?{tag}") if self.path.startswith("/cdn") else f"/file?{tag}"
        link = (f"<a id=l href='#' onclick=\"const a=document.createElement('a');"
                f"a.href=URL.createObjectURL(new Blob(['x'.repeat(1000)],{{type:'application/octet-stream'}}));"
                f"a.download='kc9k-{tag}.bin';document.body.appendChild(a);a.click();return false;\">get</a>"
                if self.path.startswith("/blob") else f"<a id=l href='{href}'>get</a>")
        body = f"<title>t</title><p>A page about {word}.</p>{link}".encode()
        self.send_response(200); self.send_header("Content-Type", "text/html"); self.end_headers(); self.wfile.write(body)
srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), H)
threading.Thread(target=srv.serve_forever, daemon=True).start()
base = "http://127.0.0.1:%d" % srv.server_address[1]
OTHER = "http://localhost:%d" % srv.server_address[1]
dl = tempfile.mkdtemp(prefix="kc9kdl-")
o = Options(); o.binary_location = "/usr/bin/librewolf"; o.add_argument("-headless")
for k, v in {"browser.download.folderList": 2, "browser.download.dir": dl, "browser.download.useDownloadDir": True,
             "browser.download.always_ask_before_handling_new_types": False,
             "browser.helperApps.neverAsk.saveToDisk": "application/octet-stream"}.items():
    o.set_preference(k, v)
s = Service(os.path.join(os.environ["GECKOWEBDRIVER"], "geckodriver"), service_args=["--allow-system-access"])
def kc9k(*a):
    subprocess.run(["kc9k", *a], check=True, stdout=subprocess.DEVNULL)
kc9k("block", "create", "dltest"); kc9k("list", "create", "dltest", "Words", "--text")
kc9k("list", "add", "dltest", "Words", "kcninedownload")
kc9k("list", "action", "dltest", "Words", "no-downloads", "on"); kc9k("list", "action", "dltest", "Words", "block", "off")
kc9k("start", "dltest")
subprocess.run(["sudo", "touch", "/var/lib/kc9k/allow-remote-control"], check=True)
time.sleep(12)
got = []
d = webdriver.Firefox(options=o, service=s); d.set_page_load_timeout(30)
try:
    time.sleep(10)
    for page in ("ok", "bad", "cdn", "blob"):
        d.get(base + "/" + page); time.sleep(4)
        d.find_element(By.ID, "l").click(); time.sleep(5)
    got = sorted(os.listdir(dl))
    print("downloaded:", got)
finally:
    d.quit(); shutil.rmtree(dl, ignore_errors=True)
    subprocess.run(["kc9k", "stop", "dltest"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["kc9k", "block", "remove", "dltest"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(["sudo", "rm", "-f", "/var/lib/kc9k/allow-remote-control"])
failures = 0
for name, want in (("ok", True), ("bad", False), ("cdn", False), ("blob", False)):
    passed = (f"kc9k-{name}.bin" in got) == want
    failures += 0 if passed else 1
    print(("PASS" if passed else "FAIL") + f": {name}: " + ("downloads" if want else "cancelled") +
          {"ok": " (page without the word)", "bad": " (same site)", "cdn": " (file from another host)",
           "blob": " (file made by the page)"}[name])
sys.exit(1 if failures else 0)
