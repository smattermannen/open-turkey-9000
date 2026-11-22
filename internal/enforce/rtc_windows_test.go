package enforce

import (
	"testing"
	"time"
)

// W17: the runner's own boot event gives a hardware clock near the wall
// clock.
func TestWinHardwareClockForReal(t *testing.T) {
	n, ok := readHardwareClock()
	if !ok {
		t.Skip("no boot event in the System log")
	}
	if d := time.Since(time.Unix(n, 0)); d > 10*time.Minute || d < -10*time.Minute {
		t.Errorf("hardware clock %v, %v from now", time.Unix(n, 0), d)
	}
}
