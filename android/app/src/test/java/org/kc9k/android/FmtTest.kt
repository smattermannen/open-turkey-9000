package org.kc9k.android

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Test

class FmtTest {
    @After fun reset() { Fmt.h24 = null; Fmt.systemH24 = true; Fmt.firstDay = 0 }

    @Test fun clock() {
        Fmt.h24 = true
        assertEquals("09:05", Fmt.clock(545))
        assertEquals("24:00", Fmt.clock(1440))
        assertEquals("Mon 09:00–17:00", Fmt.window(Window(0, 540, 1020)))
        Fmt.h24 = false
        assertEquals("9:05 AM", Fmt.clock(545))
        assertEquals("12:00 AM", Fmt.clock(0))
        assertEquals("12:30 PM", Fmt.clock(750))
        assertEquals("12:00 AM", Fmt.clock(1440))
        // The phone's setting when none is chosen.
        Fmt.h24 = null; Fmt.systemH24 = false
        assertEquals("5:00 PM", Fmt.clock(1020))
    }

    @Test fun firstWeekday() {
        val ws = listOf(Window(6, 0, 60), Window(0, 60, 120), Window(0, 0, 30))
        assertEquals(listOf(Window(0, 0, 30), Window(0, 60, 120), Window(6, 0, 60)), Fmt.sorted(ws))
        Fmt.firstDay = 6
        assertEquals(listOf(6, 0, 1, 2, 3, 4, 5), Fmt.days())
        assertEquals(Window(6, 0, 60), Fmt.sorted(ws).first())
    }
}
