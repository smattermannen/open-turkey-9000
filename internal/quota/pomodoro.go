package quota

// OnBreak reports whether a Pomodoro cycle is in its break at now. A cycle runs
// on seconds of blocking, then off seconds of break, repeating from anchor.
func OnBreak(on, off, anchor, now int64) bool {
	if on <= 0 || off <= 0 || now < anchor {
		return false
	}
	return (now-anchor)%(on+off) >= on
}

// BreakLeft returns how many seconds of the current break remain at now, 0
// outside a break.
func BreakLeft(on, off, anchor, now int64) int64 {
	if !OnBreak(on, off, anchor, now) {
		return 0
	}
	return (on + off) - (now-anchor)%(on+off)
}
