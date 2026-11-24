package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.TimeZone

class AutostartTest {
    private val utc = TimeZone.getTimeZone("UTC")
    private fun m(vararg a: Autostart, auto: Automation = Automation()) =
        Model(blocks = listOf(Block("x", active = false, autostarts = a.toList(), automation = auto)))
    private val none = emptyMap<String, Boolean>()

    @Test fun firstLookOnlyRecords() {
        val s = Autostarts.step(m(Autostart(Autostart.UNLOCK)), at(10), unlocked = true, before = null, tz = utc)
        assertFalse(s.model.block("x")!!.active)
    }

    @Test fun once() {
        val mm = m(Autostart(Autostart.ONCE, at = at(10)))
        assertFalse(Autostarts.step(mm, at(9, 59), false, none, utc).model.block("x")!!.active)
        val s = Autostarts.step(mm, at(10), false, none, utc)
        assertTrue(s.model.block("x")!!.active)
        assertTrue("one-shot", s.model.block("x")!!.autostarts.isEmpty())
        assertEquals(1, s.fired.size)
    }

    @Test fun timeOfDayOncePerDayWithinTenMinutes() {
        val mm = m(Autostart(Autostart.AT, days = listOf(0), minute = 8 * 60))
        assertFalse(Autostarts.step(mm, at(7, 59), false, none, utc).model.block("x")!!.active)
        assertFalse("ten minutes late", Autostarts.step(mm, at(8, 10), false, none, utc).model.block("x")!!.active)
        assertFalse("not on Tuesday", Autostarts.step(mm, at(8, 0, days = 1), false, none, utc).model.block("x")!!.active)
        val s = Autostarts.step(mm, at(8, 5), false, none, utc)
        assertTrue(s.model.block("x")!!.active)
        assertEquals("2026-11-23", s.model.block("x")!!.autostarts[0].lastDay)
        // Switched off again the same day: it doesn't fire twice.
        val off = s.model.copy(blocks = s.model.blocks.map { it.copy(active = false) })
        assertFalse(Autostarts.step(off, at(8, 6), false, none, utc).model.block("x")!!.active)
    }

    @Test fun unlockAndWindow() {
        val mm = m(Autostart(Autostart.UNLOCK))
        assertFalse(Autostarts.step(mm, at(10), false, none, utc).model.block("x")!!.active)
        assertTrue(Autostarts.step(mm, at(10), true, none, utc).model.block("x")!!.active)
        val w = m(Autostart(Autostart.WINDOW), auto = Automation(windows = listOf(Window(0, 9 * 60, 10 * 60))))
        val first = Autostarts.step(w, at(8, 59), false, none, utc)
        assertEquals(mapOf("x" to false), first.inWindow)
        val opened = Autostarts.step(w, at(9), false, first.inWindow, utc)
        assertTrue(opened.model.block("x")!!.active)
        assertTrue(opened.model.block("x")!!.autostarts.isEmpty())
        // Already in the window at the first look: not an opening.
        assertFalse(Autostarts.step(w, at(9, 30), false, mapOf("x" to true), utc).model.block("x")!!.active)
    }

    @Test fun commands() {
        val c = Commands(at(10), 1, utc)
        val a = Autostart(Autostart.AT, days = listOf(0, 1), minute = 480)
        val locked = Model(blocks = listOf(Block("x", lock = SettingLock(until = at(11)), autostarts = listOf(a))))
        assertNull(c.addAutostart(locked, "x", Autostart(Autostart.UNLOCK)).error)
        val o = c.removeAutostart(locked, "x", a)
        assertNotNull(o.error)
        assertEquals(1, o.model.refused.size)
        assertNotNull(c.addAutostart(locked, "x", Autostart(Autostart.ONCE, at = at(9))).error)
        assertNotNull(c.addAutostart(locked, "x", Autostart(Autostart.AT)).error)
        assertNotNull(c.addAutostart(locked, "x", a).error)
        val free = locked.copy(blocks = locked.blocks.map { it.copy(lock = SettingLock()) })
        assertTrue(c.removeAutostart(free, "x", a.copy(lastDay = "2026-11-23")).model.block("x")!!.autostarts.isEmpty())
        val full = Model(blocks = listOf(Block("x", autostarts = listOf(a.copy(lastDay = "2026-11-23"), Autostart(Autostart.ONCE, at = 5)))))
        assertEquals(full, Codec.decode(Codec.encode(full)))
    }
}
