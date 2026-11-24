package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AreaTest {
    private val r = 150f

    @Test fun noFixIsNotHome() {
        // The owner's bug: without a fix the phone counted as at home after 15 minutes.
        var s = Area.State()
        for (m in 0..120) s = Area.step(s, m * 60_000L, r, null, 0f)
        assertFalse(s.inside)
    }

    @Test fun noFixKeepsTheState() {
        val s = Area.step(Area.State(true), 0, r, null, 0f)
        assertTrue(s.inside)
        assertFalse(Area.step(Area.State(false), 0, r, null, 0f).inside)
    }

    @Test fun entersAtOnce() = assertTrue(Area.step(Area.State(), 0, r, 100f, 500f).inside)

    @Test fun leavesAfterTenMinutesFullyOutside() {
        val t = 1_700_000_000_000L
        var s = Area.step(Area.State(true), t, r, 400f, 20f)
        assertTrue(s.inside)
        s = Area.step(s, t + Area.LEAVE_MS - 1, r, 400f, 20f)
        assertTrue(s.inside)
        s = Area.step(s, t + Area.LEAVE_MS, r, 400f, 20f)
        assertFalse(s.inside)
    }

    @Test fun overlappingAccuracyDoesNotLeave() {
        var s = Area.State(true)
        for (m in 0..30) s = Area.step(s, m * 60_000L, r, 400f, 300f)
        assertTrue(s.inside)
        assertEquals(0L, s.outsideSince)
    }

    @Test fun backInsideResetsTheCount() {
        var s = Area.step(Area.State(true), 0, r, 400f, 20f)
        s = Area.step(s, 5 * 60_000L, r, 50f, 20f)
        s = Area.step(s, 9 * 60_000L, r, 400f, 20f)
        s = Area.step(s, 15 * 60_000L, r, 400f, 20f)
        assertTrue(s.inside)
    }
}
