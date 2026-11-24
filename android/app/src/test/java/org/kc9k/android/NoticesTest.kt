package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.TimeZone

class NoticesTest {
    private val utc = TimeZone.getTimeZone("UTC")
    private val o = Notices.Options(advance = listOf(2, 5))
    private val work = Block("Work", automation = Automation(windows = listOf(Window(0, 9 * 60, 17 * 60))))

    @Test fun windowStartAndEnd() {
        val m = Model(blocks = listOf(work))
        val start = Notices.of(m, o, at(8, 56), utc)
        assertEquals(listOf("window-start Work 2026-11-23 09:00 +5"), start.map { it.key })
        assertEquals("'Work' starts in 4 minutes", start[0].title)
        assertEquals(listOf("window-start Work 2026-11-23 09:00 +2"), Notices.of(m, o, at(8, 58), utc).map { it.key })
        assertTrue(Notices.of(m, o, at(8, 50), utc).isEmpty())
        assertEquals(listOf("window-end Work 2026-11-23 17:00 +2"), Notices.of(m, o, at(16, 59), utc).map { it.key })
        // Off blocks and kinds turned off say nothing.
        assertTrue(Notices.of(Model(blocks = listOf(work.copy(active = false))), o, at(8, 58), utc).isEmpty())
        assertTrue(Notices.of(m, o.copy(off = setOf(Notices.WINDOW_START)), at(8, 58), utc).isEmpty())
    }

    @Test fun modesAndSchedulesCount() {
        val m = Model(blocks = listOf(Block("x", mode = "M")), modes = listOf(Mode("M", Automation(schedules = listOf("S")))),
            schedules = listOf(Schedule("S", listOf(Window(0, 600, 660)))))
        assertEquals(1, Notices.of(m, o, at(9, 57), utc).size)
    }

    @Test fun timerEnd() {
        val m = Model(blocks = listOf(Block("x", lock = SettingLock(until = at(10, 3)))))
        assertEquals(listOf("timer-end x 2026-11-23 10:03 +5"), Notices.of(m, o, at(10, 0), utc).map { it.key })
    }

    @Test fun pausedAndAdvance() {
        assertEquals("Paused: a, b, c and 1 more", Notices.paused(o, listOf("d", "c", "b", "a"))!!.title)
        assertNull(Notices.paused(o.copy(off = setOf(Notices.APP_BLOCKED)), listOf("a")))
        assertEquals(listOf(1, 2, 5), Notices.Options.parseAdvance("5, 2,1,2"))
        assertNull(Notices.Options.parseAdvance("5,x"))
        assertNull(Notices.Options.parseAdvance(""))
    }
}
