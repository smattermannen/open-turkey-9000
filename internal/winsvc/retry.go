package winsvc

// retry.go — how the service's pipe comes back after a failure (W55):
// shared so it is tested on Linux too. A service whose pipe failed once
// (an upgrade's old process still holding the first instance, a failed
// accept) keeps trying until it is stopped, pausing longer each time up to
// maxRetryPause, so commands reach it again as soon as they can.

import "time"

const (
	firstRetryPause = 250 * time.Millisecond
	maxRetryPause   = 5 * time.Second
)

// retryPause is the pause after the n-th failure in a row (n ≥ 1).
func retryPause(n int) time.Duration {
	d := firstRetryPause
	for i := 1; i < n && d < maxRetryPause; i++ {
		d *= 2
	}
	if d > maxRetryPause {
		d = maxRetryPause
	}
	return d
}

// pauseUntil waits d; false when stop closed first.
func pauseUntil(stop <-chan struct{}, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return false
	case <-t.C:
		return true
	}
}
