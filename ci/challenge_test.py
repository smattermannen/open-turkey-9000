"""Types the unlock challenge in a real terminal, a page at a time, like a person.

    go build -o /tmp/try-challenge ./cmd/try-challenge
    python3 ci/challenge_test.py /tmp/try-challenge

A small terminal (14 rows) splits a 700-character challenge into 4 pages. The
script reads each page off the screen and types it one key at a time, then
checks three runs: typed correctly (unlocked), with too many mistakes (denied)
and pasted (refused).
"""
import fcntl
import os
import re
import select
import struct
import subprocess
import sys
import termios
import time

BINARY = sys.argv[1]
LENGTH = 700
ROWS, COLS = 14, 80
# Line ends arrive as \r\r\n: the terminal still turns each \n into \r\n.
PAGE = re.compile(rb"page (\d+) of (\d+)\r*\n\r*\n(.*?)\r*\n\r*\nType the (\d+) characters", re.S)
failures = 0


def check(name, passed):
    global failures
    print(("PASS: " if passed else "FAIL: ") + name)
    failures += 0 if passed else 1


class Terminal:
    def __init__(self):
        self.master, slave = os.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))
        self.proc = subprocess.Popen([BINARY, str(LENGTH)], stdin=slave, stdout=slave, stderr=slave,
                                     start_new_session=True)
        os.close(slave)
        self.output = b""

    def wait_for(self, pattern, start=0, timeout=10):
        deadline = time.time() + timeout
        while time.time() < deadline:
            match = re.compile(pattern, re.S).search(self.output, start)
            if match:
                return match
            ready, _, _ = select.select([self.master], [], [], 0.2)
            if ready:
                try:
                    self.output += os.read(self.master, 65536)
                except OSError:
                    break
        return None

    def type(self, text):
        for ch in text:
            os.write(self.master, ch.encode())
            time.sleep(0.003)  # a key at a time, never a burst

    def close(self):
        self.proc.kill()
        self.proc.wait()
        os.close(self.master)


def run(name, mangle_page=None, paste_page=None):
    """Types every page, mangling or pasting one if asked, and returns the screen output."""
    term = Terminal()
    pos = 0
    total = None
    page_no = 0
    try:
        while total is None or page_no < total:
            match = term.wait_for(PAGE.pattern, pos)
            if match is None:
                break
            page_no, total = int(match.group(1)), int(match.group(2))
            page = match.group(3).replace(b"\r", b"").replace(b"\n", b"").decode()
            if len(page) != int(match.group(4)):
                check(f"{name}: page {page_no} shown in full", False)
            pos = match.end()
            if paste_page == page_no:
                os.write(term.master, page.encode())
            else:
                if mangle_page == page_no:
                    page = "".join("#" if i < 20 else c for i, c in enumerate(page))
                term.type(page)
            os.write(term.master, b"\r")
            if paste_page == page_no:
                break
        term.wait_for(rb"(authorized|denied|Paste detected)", pos)
        time.sleep(0.3)
        return term.output, total
    finally:
        term.close()


out, pages = run("correct")
check("a 700-character challenge spans 4 pages", pages == 4)
check("typed correctly, page by page, it unlocks", b"unlock authorized" in out)

out, _ = run("mistakes", mangle_page=2)
check("20 wrong characters (2.9%) are denied", b"Unlock denied" in out)

out, _ = run("paste", paste_page=2)
check("pasting a page is refused", b"Paste detected" in out and b"authorized" not in out)

sys.exit(1 if failures else 0)
