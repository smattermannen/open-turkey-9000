package enforce

import (
	"testing"
	"time"
)

// W17: Windows' boot event, as wevtutil prints it.
func TestParseBootEvent(t *testing.T) {
	xml := `<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='Microsoft-Windows-Kernel-General' Guid='{a68ca8b7-004f-d7b6-a698-07e2de0f1f5d}'/><EventID>12</EventID>` +
		`<TimeCreated SystemTime='2026-11-22T07:00:01.5000000Z'/></System><EventData><Data Name='MajorVersion'>10</Data>` +
		`<Data Name='StartTime'>2026-11-22T07:00:00.4999999Z</Data></EventData></Event>`
	start, logged, ok := parseBootEvent(xml)
	if !ok || start.Unix() != time.Date(2026, 11, 22, 7, 0, 0, 0, time.UTC).Unix() || logged.Second() != 1 {
		t.Fatalf("start %v, logged %v, ok %v", start, logged, ok)
	}
	if _, _, ok := parseBootEvent("<Event></Event>"); ok {
		t.Error("an event without StartTime was read")
	}
	// Two hours after boot the hardware clock's count is 9:00, whatever
	// the wall clock was set to meanwhile.
	if got := hardwareClockAt(start, 2*time.Hour); got != time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC).Unix() {
		t.Errorf("count %d", got)
	}
}
