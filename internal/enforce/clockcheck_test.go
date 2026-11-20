package enforce

import (
	"testing"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
)

// §4.10 clock check: two sources must agree; a cache's old Date never wins
// over two current ones, and the earlier of the pair is taken.
func TestAgreedTime(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	stale := now.Add(-11 * time.Hour)
	if got, ok := agreedTime([]time.Time{now, stale, now.Add(time.Second)}); !ok || !got.Equal(now) {
		t.Errorf("got %v %v; want %v", got, ok, now)
	}
	if _, ok := agreedTime([]time.Time{now, stale}); ok {
		t.Error("two sources an hour apart counted as agreeing")
	}
	if _, ok := agreedTime([]time.Time{now}); ok {
		t.Error("one source was enough")
	}
}

// The internet is asked only while "block time and date changes" is on and
// a timer lock runs, and then once per boot.
func TestCheckClockAsksOnlyWhenDue(t *testing.T) {
	d := testDB(t)
	asked := 0
	savedNet, savedRTC := internetTime, hardwareClock
	t.Cleanup(func() { internetTime, hardwareClock = savedNet, savedRTC })
	internetTime = func() (time.Time, error) { asked++; return time.Now(), nil }
	hardwareClock = func() (int64, bool) { return time.Now().Unix(), true }

	now := time.Now()
	CheckClock(d, now)
	if err := d.SetSetting(db.SettingTamperClock, "on"); err != nil {
		t.Fatal(err)
	}
	CheckClock(d, now)
	if asked != 0 {
		t.Fatalf("asked the internet %d times without a timer lock", asked)
	}
	trusted, err := d.TrustedNow(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetSettingLock(db.SettingLock{Name: "web-only", LockedUntil: trusted + 3600}); err != nil {
		t.Fatal(err)
	}
	CheckClock(d, now)
	CheckClock(d, now.Add(5*time.Minute))
	if asked != 1 {
		t.Errorf("asked the internet %d times in one boot with a timer lock; want 1", asked)
	}
}
