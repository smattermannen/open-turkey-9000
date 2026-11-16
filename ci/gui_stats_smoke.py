"""GUI smoke test for the Statistics page (§4.9): the daily chart stacks the
top sites in their own colours (the rest as Other), the legend names them,
Top 5 / Top 10, search and the block filter (behind a block password, §3.11)
read the rows again, a site keeps its colour while the filters change, and
the dashboard never sees the page's filters. Draws the chart on an
off-screen surface and reads its pixels. Builds kc9k into a scratch folder,
a wrapper with KC9K_DB = scratch file, loads gui/kc9k-gui.py as a module
with CLI replaced, another application ID, never shows the main window.
Needs an X display; run as a normal user.

    xvfb-run -a python3 ci/gui_stats_smoke.py
"""
import datetime, importlib.util, json, os, sqlite3, subprocess, tempfile, time, types
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
S = tempfile.mkdtemp(prefix="kc9k-gui-")
W = os.path.join(S, "kc9k")
subprocess.run(["go", "build", "-o", os.path.join(S, "kc9k-bin"), "./cmd/kc9k"], cwd=ROOT, check=True,
               env=dict(os.environ, CGO_ENABLED="0", PATH="/usr/local/go/bin:" + os.environ["PATH"]))
with open(W, "w") as f:
    f.write(f'#!/bin/sh\nKC9K_DB={S}/t.db exec {S}/kc9k-bin "$@"\n')
os.chmod(W, 0o755)
os.environ["XDG_CONFIG_HOME"] = os.path.join(S, "config")


def cli(*a, stdin=None):
    return subprocess.run([W, *a], capture_output=True, text=True, input=stdin)


for a in (["block", "create", "work"], ["block", "add", "work", "c.example"], ["block", "add", "work", "d.example"],
          ["stats", "on"]):
    r = cli(*a); assert r.returncode == 0, (a, r.stderr)

# Seven days of nine sites, an app and a blocked site.
SITES = [("a.example", 3600), ("b.example", 1800), ("c.example", 1200), ("d.example", 900), ("e.example", 600),
         ("f.example", 500), ("g.example", 400), ("h.example", 300), ("i.example", 200)]
today = datetime.date.today()
con = sqlite3.connect(os.path.join(S, "t.db"))
for n in range(7):
    day = (today - datetime.timedelta(days=n)).isoformat()
    for name, seconds in SITES:
        con.execute("INSERT INTO usage (day, kind, name, seconds) VALUES (?, 'site', ?, ?)", (day, name, seconds))
con.execute("INSERT INTO usage (day, kind, name, seconds) VALUES (?, 'app', 'steam', 1200)", (today.isoformat(),))
con.execute("INSERT INTO usage (day, kind, name, seconds) VALUES (?, 'blocked', 'c.example', 3)", (today.isoformat(),))
con.commit(); con.close()

spec = importlib.util.spec_from_file_location("gui", os.path.join(ROOT, "gui", "kc9k-gui.py"))
gui = importlib.util.module_from_spec(spec); spec.loader.exec_module(gui)
gui.CLI = W
from gi.repository import Gtk, GLib  # noqa: E402
import cairo  # noqa: E402
app = gui.App(); app.set_application_id("io.test.kc9kstatssmoke"); app.register(None)
win = gui.Window(app)
win.extension_asked = True
fails, msgs = [], []
win.message = lambda text, *a, **k: msgs.append(text)


def check(ok, what):
    print(("ok   " if ok else "FAIL ") + what)
    ok or fails.append(what)


def pump(cond, secs=30):
    end = time.time() + secs
    while time.time() < end and not cond():
        GLib.MainContext.default().iteration(False); time.sleep(0.01)
    assert cond(), "timeout"


def spin(n=60):
    for _ in range(n):
        GLib.MainContext.default().iteration(False); time.sleep(0.005)


def settle():
    pump(lambda: win.get_sensitive() and not win.busy)
    spin()
    win.state = None; win.state_json = None; win.refresh(); pump(lambda: win.state is not None)
    spin()


def page_names():
    return {r["name"] for r in win.stats_page_rows if r["kind"] == "site"}


def reread(cond):
    """Reads the rows again and waits until cond holds."""
    win.refresh_stats()
    pump(cond)
    spin()


settle()
win.stats_period.set_active_id("7")
win.kind_sites.set_active(True)
reread(lambda: len(page_names()) == 9)
dark = gui.dark_theme(win.daily_chart)
slot = lambda name: dict(win.daily_chart.series).get(name, "missing")  # noqa: E731

# Top 5: five named in their own colours, the other four as Other.
check([n for n, _s in win.top_chart.items] == [n for n, _s in SITES[:5]], "Top 5: the five most used, in order")
check([n for n, _s in win.daily_chart.series] == [n for n, _s in SITES[:5]] + ["Other"],
      "the chart stacks those five and Other: %s" % win.daily_chart.series)
slots = [s for _n, s in win.daily_chart.series]
check(len(set(slots[:-1])) == 5 and slots[-1] is None, "five colours, Other grey")
check(win.daily_legend.entries == win.daily_chart.series and win.daily_legend.get_visible(), "the legend names every series")
check(win.daily_chart.stacks[-1]["Other"] == 500 + 400 + 300 + 200, "Other adds up the rest: %s" % win.daily_chart.stacks[-1])
check(win.tile_total.value.get_text() == gui.fmt_spent(9500 * 7),
      "the total")

# Top 10: nine sites; seven colours (the first five keep theirs), the last two Other.
before = {n: s for n, s in win.daily_chart.series}
win.stats_top.set_active_id("10")
check(len(win.top_chart.items) == 9, "Top 10: all nine sites listed")
check([n for n, _s in win.daily_chart.series] == [n for n, _s in SITES[:7]] + ["Other"], "seven named in the chart")
check(all(slot(n) == before[n] for n in before if n != "Other"), "the first five keep their colours")
check(win.top_chart.colors["h.example"] is None and win.top_chart.colors["a.example"] == slot("a.example"),
      "the list's bars use the chart's colours (h.example as Other)")
check(len({s for _n, s in win.daily_chart.series if s is not None}) == 7, "no two in one colour")


# The chart draws: today's column, bottom up, in the series' colours.
def column_colors(chart, index, width=700, height=200):
    chart.get_allocated_width = lambda: width
    chart.get_allocated_height = lambda: height
    surface = cairo.ImageSurface(cairo.FORMAT_ARGB32, width, height)
    chart.on_draw(chart, cairo.Context(surface))
    surface.flush()
    data, stride = surface.get_data(), surface.get_stride()
    left, top, slot_w, plot = chart.geometry(width, height)
    x = int(left + index * slot_w + slot_w / 2)
    runs, last, length = [], None, 0
    for y in range(int(top + plot) - 1, int(top), -1):
        b, g, r, a = data[y * stride + 4 * x: y * stride + 4 * x + 4]
        pixel = (r, g, b) if a == 255 else None
        if pixel == last:
            length += 1
            continue
        if last is not None and length >= 2 and (not runs or runs[-1] != last):
            runs.append(last)
        last, length = pixel, 1
    if last is not None and length >= 2 and (not runs or runs[-1] != last):
        runs.append(last)
    if os.environ.get("KC9K_SHOT"):
        surface.write_to_png(os.environ["KC9K_SHOT"])
    return runs


def rgb(color_slot):
    r, g, b, _a = gui.series_color(color_slot, dark)
    return tuple(round(v * 255) for v in (r, g, b))


win.daily_chart.hover = None
runs = column_colors(win.daily_chart, 6)
check(runs == [rgb(s) for _n, s in win.daily_chart.series], "today's column stacks the series bottom up: %s" % runs)


class Tip:
    text = ""

    def set_text(self, text):
        Tip.text = text


left, _top, slot_w, _plot = win.daily_chart.geometry(700, 200)
win.daily_chart.on_tooltip(win.daily_chart, left + 6 * slot_w + slot_w / 2, 50, False, Tip())
check(Tip.text.startswith(today.strftime("%a %-d %b")) and "a.example: 1h" in Tip.text and "Other:" in Tip.text,
      "hovering a day names every part: %r" % Tip.text)
win.update_dashboard_stats()
check(win.dash_top.items[0][0] == "a.example", "the dashboard's most used")

# Search: only what matches, read again; colours stay.
was_b = slot("b.example")
win.stats_search.set_text("b.ex")
pump(lambda: page_names() == {"b.example"})
spin()
check([n for n, _s in win.top_chart.items] == ["b.example"], "search: only b.example listed")
check(slot("b.example") == was_b, "b.example keeps its colour")
check(not win.daily_legend.get_visible(), "one series: no legend")
win.update_dashboard_stats()
check(len(win.dash_top.items) == 5 and win.dash_top.items[0][0] == "a.example",
      "the dashboard ignores the page's search")
win.stats_search.set_text("")
pump(lambda: len(page_names()) == 9)
spin()

# The block filter: only the block's sites; colours stay.
was = {n: slot(n) for n in ("c.example", "d.example")}
check("work" in [row[0] for row in win.stats_block.get_model()], "the block filter lists the blocks")
win.stats_block.set_active_id("work")
pump(lambda: page_names() == {"c.example", "d.example"})
spin()
check({n for n, _s in win.top_chart.items} == {"c.example", "d.example"}, "block filter: its sites only")
check(all(slot(n) == was[n] for n in was), "c and d keep their colours")
win.kind_blocked.set_active(True)
check(win.top_chart.items == [("c.example", 3)] and win.top_chart.fmt(3) == "3×", "times blocked within the block")
win.kind_sites.set_active(True)
win.stats_block.set_active_id("")
pump(lambda: len(page_names()) == 9)
spin()

# Behind a block password: refused with a reason, then with the password.
r = cli("block", "password", "work", "set", stdin="view1\nview1\n"); assert r.returncode == 0, r.stderr
msgs.clear()
win.stats_block.set_active_id("work")
pump(lambda: msgs and win.stats_block.get_active_id() == "")
check("block password" in msgs[0], "a hidden block's sites need its password: %s" % msgs)
pump(lambda: len(page_names()) == 9)
spin()
check(len(win.top_chart.items) == 9, "and the page shows everything again")
win.block_passwords["work"] = "view1"
win.stats_block.set_active_id("work")
pump(lambda: page_names() == {"c.example", "d.example"})
check(True, "with the block password typed: its sites")
win.update_dashboard_stats()
check(win.dash_top.items[0][0] == "a.example", "the dashboard ignores the block filter")

# Apps: the app time and its own tile title.
win.stats_block.set_active_id("")
pump(lambda: len(page_names()) == 9)
spin()
win.kind_apps.set_active(True)
check(win.top_chart.items == [("steam", 1200)], "apps: steam")
check(win.tile_count.get_children()[0].get_text() == "Different apps", "the tile says apps")

print("FAILED: %d" % len(fails) if fails else "all passed")
raise SystemExit(1 if fails else 0)
