#!/usr/bin/env bash
# Builds the fake kc9k's fixtures from the real Linux kc9k (W4): a test DB
# filled through kc9k commands, then `gui-state` and `stats --json` saved
# as JSON. Run as a normal user, never root (CLAUDE.md: as root KC9K_DB is
# ignored). Usage: make-fixtures.sh path/to/kc9k [outdir]
set -u
KC9K=$(readlink -f "$1")
OUT=$(readlink -f "${2:-$(dirname "$0")}")
if [ "$(id -u)" = 0 ]; then echo "run as a normal user" >&2; exit 1; fi
WORK=$(mktemp -d)
export KC9K_DB="$WORK/test.db"
k() { "$KC9K" "$@" >>"$WORK/log" 2>&1 || echo "failed: kc9k $*" >&2; }

# Blocks (§4.2, §4.3)
k block create Work
k block add Work youtube.com reddit.com news.ycombinator.com
k block add-exception Work music.youtube.com
k block add-term Work "breaking news"
k block add-app Work steam.exe
k block add-title Work "Solitaire"
k list create Work Shopping
k list add Work Shopping amazon.com ebay.com
k phrase-set create Work Gossip
k phrase-set add Work Gossip "[celebrity, star] [divorce, scandal]"
k block create Evening
k block add Evening netflix.com twitch.tv
k block create Games
k block add-app Games steam.exe epicgameslauncher.exe
k block create "Social media"
k block add "Social media" facebook.com instagram.com x.com tiktok.com
k block create Reading
k block add Reading goodreads.com

# Automation (§4.5): windows, schedules
k schedule add Evening --days weekdays --from 18:00 --to 23:00
k automation "Social media" default enabled
k schedule add "Social media" --days sat,sun --from 10:00 --to 12:00 --allow
k schedule create Weekend
k schedule add Weekend --days sat,sun --from 00:00 --to 24:00
k schedule apply Games Weekend

# Modes (§4.2)
k mode create Study
k mode add Study Work Games
k schedule add Study --days mon,tue,wed,thu,fri --from 09:00 --to 17:00

# Starts and locks (§4.6), breaks (§4.7)
k allowance set Games 1h
k break reward Work 10m
k pomodoro set Evening 25m 5m
k start Work --for 2h --yes
k start Evening
k start "Social media" --for 3d --delay 1h --yes

# Hotkeys (§4.13), presets (§4.4)
k hotkey add ctrl+alt+w Work
k block-preset save Work "Work preset"
k preset save schedule Weekend "Weekend preset"

# Settings (§4.10)
k browser safesearch-google on
k browser safesearch-youtube on
k notifications window-start on
k notifications advance 2,5
k tamper clock on
k bypass set --count 5 --within 10m --then warning

"$KC9K" gui-state | python3 -m json.tool --indent 1 >"$OUT/gui-state.json"

# Statistics (§4.9): a week of use, written straight into the test DB (the
# daemon records it on a real machine).
python3 - "$KC9K_DB" <<'PY'
import sqlite3, sys, datetime, random
db = sqlite3.connect(sys.argv[1])
random.seed(9000)
today = datetime.date.today()
for back in range(14):
    day = (today - datetime.timedelta(days=back)).isoformat()
    for kind, names in (("site", ["youtube.com", "github.com", "wikipedia.org", "reddit.com", "mail.proton.me", "news.ycombinator.com"]),
                        ("app", ["firefox.exe", "code.exe", "steam.exe", "explorer.exe"])):
        for name in names:
            seconds = random.randint(60, 7200)
            db.execute("INSERT OR REPLACE INTO usage (day, kind, name, seconds) VALUES (?,?,?,?)", (day, kind, name, seconds))
            for user in ("alex", "sam"):
                db.execute("INSERT OR REPLACE INTO usage_users (day, kind, name, user, seconds) VALUES (?,?,?,?,?)", (day, kind, name, user, seconds // 2))
    for name in ("youtube.com", "reddit.com"):
        db.execute("INSERT OR REPLACE INTO usage (day, kind, name, seconds) VALUES (?,?,?,?)", (day, "blocked", name, random.randint(0, 12)))
    db.execute("INSERT OR REPLACE INTO usage (day, kind, name, seconds) VALUES (?,?,?,?)", (day, "blocked-block", "Work", random.randint(0, 20)))
db.commit()
PY
"$KC9K" stats --json --days 14 | python3 -m json.tool --indent 1 >"$OUT/stats.json" 2>/dev/null ||
  "$KC9K" stats --json | python3 -m json.tool --indent 1 >"$OUT/stats.json"
cat "$WORK/log" >"$OUT/make-fixtures.log"
rm -rf "$WORK"
echo "fixtures in $OUT"
