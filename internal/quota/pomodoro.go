package quota

// Cycle is a Pomodoro rhythm: On seconds of blocking, then Off seconds of
// break, repeating from Anchor. With Every > 1 and Long > 0, every Every-th
// break lasts Long seconds instead.
type Cycle struct {
	On, Off, Long, Every, Anchor int64
}

// BreakLeft returns how many seconds of the current break remain at now, 0
// outside a break.
func (c Cycle) BreakLeft(now int64) int64 {
	if c.On <= 0 || c.Off <= 0 || now < c.Anchor {
		return 0
	}
	if c.Every <= 1 || c.Long <= 0 {
		pos := (now - c.Anchor) % (c.On + c.Off)
		if pos < c.On {
			return 0
		}
		return c.On + c.Off - pos
	}
	round := c.Every*c.On + (c.Every-1)*c.Off + c.Long
	pos := (now - c.Anchor) % round
	for i := int64(1); i <= c.Every; i++ {
		rest := c.Off
		if i == c.Every {
			rest = c.Long
		}
		if pos < c.On {
			return 0
		}
		if pos < c.On+rest {
			return c.On + rest - pos
		}
		pos -= c.On + rest
	}
	return 0
}

// OnBreak reports whether the cycle is in a break at now.
func (c Cycle) OnBreak(now int64) bool { return c.BreakLeft(now) > 0 }

// OnBreak reports whether a plain cycle (no long break) is in its break.
func OnBreak(on, off, anchor, now int64) bool {
	return Cycle{On: on, Off: off, Anchor: anchor}.OnBreak(now)
}

// BreakLeft is Cycle.BreakLeft for a plain cycle.
func BreakLeft(on, off, anchor, now int64) int64 {
	return Cycle{On: on, Off: off, Anchor: anchor}.BreakLeft(now)
}
