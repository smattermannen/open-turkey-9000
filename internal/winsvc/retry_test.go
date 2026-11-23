package winsvc

import (
	"testing"
	"time"
)

func TestRetryPause(t *testing.T) {
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for i, w := range want {
		if got := retryPause(i + 1); got != w {
			t.Errorf("retryPause(%d) = %v, want %v", i+1, got, w)
		}
	}
	if got := retryPause(1000); got != maxRetryPause {
		t.Errorf("retryPause(1000) = %v", got)
	}
}

func TestPauseUntil(t *testing.T) {
	stop := make(chan struct{})
	if !pauseUntil(stop, time.Millisecond) {
		t.Fatal("pause ended as stopped")
	}
	close(stop)
	start := time.Now()
	if pauseUntil(stop, time.Hour) {
		t.Fatal("pause after stop said done")
	}
	if time.Since(start) > time.Second {
		t.Fatal("stop did not end the pause")
	}
}
