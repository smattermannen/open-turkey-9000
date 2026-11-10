package schedule

import (
	"testing"
	"time"
)

// Windows follow the wall clock, so on the days the clocks change:
//   - an hour that is skipped doesn't happen, and a window inside it never
//     opens that day;
//   - an hour that repeats is covered both times.
//
// 2026 in Europe/Stockholm: 29 March 02:00 -> 03:00, 25 October 03:00 -> 02:00.
// In America/New_York: 8 March 02:00 -> 03:00, 1 November 02:00 -> 01:00.

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("no time zone data for %s: %v", name, err)
	}
	return loc
}

// openMinutes counts the real minutes a window is open during the 30 hours
// from midnight of the given day.
func openMinutes(w Window, day time.Time) int {
	n := 0
	for i := 0; i < 30*60; i++ {
		if w.ActiveAt(day.Add(time.Duration(i) * time.Minute)) {
			n++
		}
	}
	return n
}

func TestWindowsWhenClocksChange(t *testing.T) {
	cases := []struct {
		zone                       string
		spring, autumn             time.Time
		skipped, repeated          Window // the hour that is skipped / repeated
		across                     Window // a window from before it to after it
		acrossSpring, acrossAutumn int
	}{
		{zone: "Europe/Stockholm",
			skipped: Window{Days: Sunday, Start: 2 * 60, End: 3 * 60}, repeated: Window{Days: Sunday, Start: 2 * 60, End: 3 * 60},
			across: Window{Days: Sunday, Start: 60, End: 4 * 60}, acrossSpring: 120, acrossAutumn: 240},
		{zone: "America/New_York",
			skipped: Window{Days: Sunday, Start: 2 * 60, End: 3 * 60}, repeated: Window{Days: Sunday, Start: 60, End: 2 * 60},
			across: Window{Days: Sunday, Start: 30, End: 4 * 60}, acrossSpring: 150, acrossAutumn: 270},
	}
	days := map[string][2][3]int{
		"Europe/Stockholm": {{2026, 3, 29}, {2026, 10, 25}},
		"America/New_York": {{2026, 3, 8}, {2026, 11, 1}},
	}
	for _, c := range cases {
		loc := zone(t, c.zone)
		d := days[c.zone]
		spring := time.Date(d[0][0], time.Month(d[0][1]), d[0][2], 0, 0, 0, 0, loc)
		autumn := time.Date(d[1][0], time.Month(d[1][1]), d[1][2], 0, 0, 0, 0, loc)
		if got := openMinutes(c.skipped, spring); got != 0 {
			t.Errorf("%s: a window in the skipped hour was open %d minutes; want 0", c.zone, got)
		}
		if got := openMinutes(c.repeated, autumn); got != 120 {
			t.Errorf("%s: a window in the repeated hour was open %d minutes; want 120", c.zone, got)
		}
		if got := openMinutes(c.across, spring); got != c.acrossSpring {
			t.Errorf("%s: spring, window across the change open %d minutes; want %d", c.zone, got, c.acrossSpring)
		}
		if got := openMinutes(c.across, autumn); got != c.acrossAutumn {
			t.Errorf("%s: autumn, window across the change open %d minutes; want %d", c.zone, got, c.acrossAutumn)
		}
		// An ordinary evening window is 2 hours on both days, at the same clock times.
		evening := Window{Days: Sunday, Start: 20 * 60, End: 22 * 60}
		for _, day := range []time.Time{spring, autumn} {
			if got := openMinutes(evening, day); got != 120 {
				t.Errorf("%s %s: evening window open %d minutes; want 120", c.zone, day.Format("2006-01-02"), got)
			}
			at := time.Date(day.Year(), day.Month(), day.Day(), 20, 0, 0, 0, loc)
			if !evening.ActiveAt(at) || evening.ActiveAt(at.Add(-time.Minute)) || evening.ActiveAt(at.Add(2*time.Hour)) {
				t.Errorf("%s %s: evening window isn't 20:00-22:00 on the clock", c.zone, day.Format("2006-01-02"))
			}
		}
	}
}

// The edit freeze looks ahead in real time, so it sees a window on the far
// side of a clock change exactly EditFreeze before it opens.
func TestEditFreezeAcrossClockChange(t *testing.T) {
	loc := zone(t, "Europe/Stockholm")
	morning := []Window{{Days: Sunday, Start: 4 * 60, End: 6 * 60}}
	// Spring: 04:00 is 3 real hours after 00:00 (02:00-03:00 is skipped).
	opens := time.Date(2026, 3, 29, 4, 0, 0, 0, loc)
	if EnforcedWithin(morning, opens.Add(-EditFreeze-time.Minute), EditFreeze) {
		t.Error("spring: frozen more than the lead time ahead")
	}
	if !EnforcedWithin(morning, opens.Add(-EditFreeze), EditFreeze) {
		t.Error("spring: not frozen the lead time ahead")
	}
	if got := opens.Add(-EditFreeze).Format("15:04"); got != "01:00" {
		t.Errorf("spring: the freeze starts at %s on the clock; want 01:00", got)
	}
	// Autumn: 04:00 is 5 real hours after 00:00 (02:00-03:00 happens twice).
	opens = time.Date(2026, 10, 25, 4, 0, 0, 0, loc)
	if EnforcedWithin(morning, opens.Add(-EditFreeze-time.Minute), EditFreeze) || !EnforcedWithin(morning, opens.Add(-EditFreeze), EditFreeze) {
		t.Error("autumn: the freeze doesn't start exactly the lead time ahead")
	}
	// Removing the window is a weakening inside the lead time, not before.
	if !Weakens(morning, nil, opens.Add(-time.Hour), EditFreeze) || Weakens(morning, nil, opens.Add(-3*time.Hour), EditFreeze) {
		t.Error("autumn: Weakens disagrees with the lead time")
	}
}
