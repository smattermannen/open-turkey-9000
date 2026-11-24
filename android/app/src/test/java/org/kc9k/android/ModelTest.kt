package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Calendar
import java.util.TimeZone

private val UTC: TimeZone = TimeZone.getTimeZone("UTC")

/** Monday 2026-11-23 at [h]:[m] UTC, plus [days]. */
fun at(h: Int, m: Int = 0, days: Int = 0): Long = Calendar.getInstance(UTC).apply {
    clear(); set(2026, Calendar.NOVEMBER, 23 + days, h, m)
}.timeInMillis

class WeekTimeTest {
    @Test fun mondayIsZero() = assertEquals(WeekTime(0, 9 * 60 + 30), WeekTime.of(at(9, 30), UTC))
    @Test fun sundayIsSix() = assertEquals(6, WeekTime.of(at(1, days = 6), UTC).day)
    @Test fun parse() {
        assertEquals(24 * 60, Window.parse("24:00"))
        assertEquals(9 * 60 + 5, Window.parse("9:05"))
        assertNull(Window.parse("24:01"))
        assertNull(Window.parse("12:60"))
    }
}

class EnforceTest {
    private val work = Window(0, 9 * 60, 17 * 60)
    private fun m(vararg b: Block, places: List<Place> = emptyList(), schedules: List<Schedule> = emptyList()) =
        Model(blocks = b.toList(), places = places, schedules = schedules)

    @Test fun continuousIsOnWhileActive() {
        val b = Block("x", sites = listOf("a.com"))
        assertTrue(Enforce.enabled(m(b), b, at(3), UTC))
        assertFalse(Enforce.enabled(m(b), b.copy(active = false), at(3), UTC))
    }

    @Test fun blockWindows() {
        val b = Block("x", automation = Automation(windows = listOf(work)))
        assertTrue(Enforce.enabled(m(b), b, at(10), UTC))
        assertFalse(Enforce.enabled(m(b), b, at(17), UTC))
        assertFalse(Enforce.enabled(m(b), b, at(10, days = 1), UTC))
        assertEquals(Enforce.State.LIFTED, Enforce.state(m(b), b, at(18), UTC))
    }

    @Test fun allowWindowsLift() {
        val b = Block("x", automation = Automation(defaultEnabled = true, windows = listOf(work)))
        assertFalse(Enforce.enabled(m(b), b, at(10), UTC))
        assertTrue(Enforce.enabled(m(b), b, at(18), UTC))
    }

    @Test fun schedulesCount() {
        val b = Block("x", automation = Automation(schedules = listOf("Work")))
        val mm = m(b, schedules = listOf(Schedule("Work", listOf(work))))
        assertTrue(Enforce.enabled(mm, b, at(10), UTC))
    }

    @Test fun placesAndBothOrEither() {
        val home = Place("Home", 0.0, 0.0, 100f, inside = true)
        val nokia = Block("Nokia", automation = Automation(places = listOf("Home")))
        assertTrue(Enforce.enabled(m(nokia, places = listOf(home)), nokia, at(3), UTC))
        assertFalse(Enforce.enabled(m(nokia, places = listOf(home.copy(inside = false))), nokia, at(3), UTC))
        val both = Block("b", automation = Automation(windows = listOf(work), places = listOf("Home"), both = true))
        assertFalse(Enforce.enabled(m(both, places = listOf(home)), both, at(3), UTC))
        assertTrue(Enforce.enabled(m(both, places = listOf(home)), both, at(10), UTC))
        val either = both.copy(automation = both.automation.copy(both = false))
        assertTrue(Enforce.enabled(m(either, places = listOf(home)), either, at(3), UTC))
    }

    @Test fun pauseLifts() {
        val b = Block("x", pausedUntil = at(12))
        assertEquals(Enforce.State.PAUSED, Enforce.state(m(b), b, at(11), UTC))
        assertTrue(Enforce.enabled(m(b), b, at(12), UTC))
    }

    @Test fun planUnionsWithOwnExceptions() {
        val a = Block("a", apps = listOf("*"), appExceptions = listOf("com.phone"), sites = listOf("x.com"), siteExceptions = listOf("x.com/ok"))
        val b = Block("b", apps = listOf("com.phone"), wifiOff = true)
        val off = Block("off", active = false, apps = listOf("com.other"))
        val p = Enforce.plan(m(a, b, off), at(3), UTC)
        assertTrue(p.wifiOff)
        assertEquals(listOf(Triple("a", listOf("x.com"), listOf("x.com/ok"))), p.web)
        val all = setOf("com.phone", "com.game", "com.other", "org.kc9k.android")
        val resolve = { e: List<String> -> e.flatMap { if (it == "*") all else setOf(it) }.toSet() }
        // a's exception spares com.phone from a only; b blocks it.
        assertEquals(setOf("com.phone", "com.game", "com.other"), Enforce.suspend(p, resolve, setOf("org.kc9k.android")))
        val p2 = Enforce.plan(m(a), at(3), UTC)
        assertEquals(setOf("com.game", "com.other"), Enforce.suspend(p2, resolve, setOf("org.kc9k.android")))
    }

    @Test fun scheduleLockHoldsWhileOn() {
        val home = Place("Home", 0.0, 0.0, 100f, inside = true)
        val b = Block("Nokia", scheduleLock = true, automation = Automation(places = listOf("Home")))
        assertTrue(Enforce.locked(m(b, places = listOf(home)), b, at(3), 1, UTC))
        assertFalse(Enforce.locked(m(b, places = listOf(home.copy(inside = false))), b, at(3), 1, UTC))
    }
}

class CommandsTest {
    private val now = at(10)
    private val c = Commands(now, 1, UTC)
    private val locked = SettingLock(until = now + 3_600_000)

    private fun m(b: Block) = Model(blocks = listOf(b))

    @Test fun strengtheningAllowedWhileLocked() {
        val mm = m(Block("x", lock = locked))
        assertNull(c.addSites(mm, "x", "reddit.com").error)
        assertNull(c.addApps(mm, "x", listOf("com.game")).error)
        assertNull(c.setWifiOff(mm, "x", true).error)
        assertNull(c.setActive(mm, "x", true).error)
        assertNull(c.removeSiteException(mm, "x", "a.com").error)
        assertNull(c.lock(mm, "x", SettingLock(until = now + 7_200_000), false).error)
    }

    /** Every weakening command, refused and logged while the block is locked (R-WEAK-1–3). */
    @Test fun weakeningRefusedAndLogged() {
        val b = Block("x", lock = locked, sites = listOf("a.com"), apps = listOf("com.game"), wifiOff = true,
            automation = Automation(windows = listOf(Window(0, 540, 600))))
        val mm = m(b)
        val refused = listOf(
            c.setActive(mm, "x", false), c.deleteBlock(mm, "x"), c.removeSite(mm, "x", "a.com"),
            c.addSiteExceptions(mm, "x", "a.com/ok"), c.removeApp(mm, "x", "com.game"),
            c.addAppExceptions(mm, "x", listOf("com.phone")), c.setWifiOff(mm, "x", false),
            c.pause(mm, "x", now + 60_000), c.setAutomation(mm, "x", b.automation.copy(windows = emptyList(), places = emptyList()).copy(windows = listOf(Window(0, 540, 570)))),
            c.importSites(mm, "x", true, "b.com\n"),
            c.lock(mm, "x", SettingLock(until = now + 60_000), false),
        )
        for ((i, o) in refused.withIndex()) {
            assertNotNull("command $i", o.error)
            assertEquals("command $i", mm.blocks, o.model.blocks)
        }
        assertEquals(1, c.setActive(mm, "x", false).model.refused.size)
    }

    @Test fun unlockedBlocksStayEditable() {
        val mm = m(Block("x", sites = listOf("a.com")))
        assertEquals(emptyList<String>(), c.removeSite(mm, "x", "a.com").model.block("x")!!.sites)
        assertNull(c.deleteBlock(mm, "x").error)
    }

    @Test fun automationStrengthening() {
        val w = Window(0, 540, 600)
        val off = Automation(windows = listOf(w))
        assertTrue(Weakening.strengthens(off, off.copy(windows = listOf(w, Window(1, 0, 60)))))
        assertFalse(Weakening.strengthens(off, off.copy(windows = listOf(Window(0, 540, 570)))))
        assertTrue(Weakening.strengthens(off, Automation()))       // to continuous
        assertFalse(Weakening.strengthens(Automation(), off))       // from continuous
        val allow = Automation(defaultEnabled = true, windows = listOf(w))
        assertTrue(Weakening.strengthens(allow, allow.copy(windows = listOf(Window(0, 540, 570)))))
        assertFalse(Weakening.strengthens(allow, allow.copy(windows = listOf(w, Window(1, 0, 60)))))
        assertFalse(Weakening.strengthens(off, off.copy(defaultEnabled = true)))
    }

    @Test fun windowsValidated() {
        val mm = m(Block("x"))
        assertNotNull(c.setAutomation(mm, "x", Automation(windows = listOf(Window(0, 541, 600)))).error)
        assertNotNull(c.setAutomation(mm, "x", Automation(windows = listOf(Window(0, 540, 600), Window(0, 570, 660)))).error)
        assertNotNull(c.setAutomation(mm, "x", Automation(places = listOf("Nowhere"))).error)
    }

    @Test fun scheduleChangeRefusedForLockedUsers() {
        val s = Schedule("Work", listOf(Window(0, 540, 600)))
        val mm = Model(blocks = listOf(Block("x", lock = locked, automation = Automation(schedules = listOf("Work")))), schedules = listOf(s))
        assertNotNull(c.setScheduleWindows(mm, "Work", emptyList()).error)
        assertNull(c.setScheduleWindows(mm, "Work", s.windows + Window(1, 0, 60)).error)
        assertNotNull(c.deleteSchedule(mm, "Work").error)
    }

    @Test fun placeChangeRefusedForLockedUsers() {
        val mm = Model(blocks = listOf(Block("x", lock = locked, automation = Automation(places = listOf("Home")))),
            places = listOf(Place("Home", 1.0, 2.0, 100f)))
        assertNotNull(c.setPlace(mm, "Home", 3.0, 4.0, 100f).error)
        assertNotNull(c.deletePlace(mm, "Home").error)
        assertNull(c.setPlace(mm, "Work", 3.0, 4.0, 100f).error)
        assertNotNull(c.setPlace(mm, "Far", 3.0, 4.0, 3000f).error)
    }

    @Test fun settingsLockedWhileABlockIs() {
        val mm = m(Block("x", lock = locked))
        assertNotNull(c.whySettingsLocked(mm))
        assertNull(c.whySettingsLocked(mm.copy(noWeakening = false)))
        assertNotNull(c.setNoWeakening(mm, false).error)
        assertNull(c.whySettingsLocked(m(Block("x"))))
    }

    @Test fun unlockOrder() {
        val mm = m(Block("x", lock = SettingLock(chars = 50)))
        assertNull(c.refuseUnlock(mm, "x"))
        assertEquals(SettingLock(), c.unlocked(mm, "x").model.block("x")!!.lock)
        assertNotNull(c.refuseUnlock(m(Block("x", lock = SettingLock(until = now + 1, chars = 50))), "x"))
    }

    @Test fun appEntries() {
        assertTrue(AppEntries.valid("*"))
        assertTrue(AppEntries.valid("role:sms"))
        assertTrue(AppEntries.valid("com.example.app"))
        assertFalse(AppEntries.valid("not a package"))
        assertNotNull(c.addAppExceptions(m(Block("x")), "x", listOf("*")).error)
    }
}

class CodecTest {
    @Test fun roundTrip() {
        val m = Model(
            blocks = listOf(Block("Nokia \"mode\"", sites = listOf("a.com"), apps = listOf("*"), appExceptions = listOf("role:sms"),
                wifiOff = true, automation = Automation(true, listOf(Window(2, 60, 120)), listOf("S"), listOf("Home"), true),
                lock = SettingLock(5, 300, "pbkdf2\$1\$aa\$bb", 3), scheduleLock = true, pausedUntil = 9)),
            places = listOf(Place("Home", 52.1, -4.25, 150f, true, 7, 12.5f, 20f, 8)),
            schedules = listOf(Schedule("S", listOf(Window(6, 0, 1440)))),
            noWeakening = false, refused = listOf("x\ny"),
        )
        assertEquals(m, Codec.decode(Codec.encode(m)))
    }

    @Test fun lenient() = assertEquals(Model(), Codec.decode("{}"))

    @Test fun migration() {
        val m = Codec.migrate(true, 1.0, 2.0, 200f, true, true, listOf("a.com"), listOf("a.com/ok"), SettingLock(chars = 20))
        val nokia = m.block(Codec.NOKIA)!!
        assertTrue(Policies.SETTINGS in nokia.apps)
        assertTrue("*" in nokia.apps)
        assertEquals(listOf("role:dialer", "role:sms"), nokia.appExceptions)
        assertTrue(nokia.wifiOff && nokia.scheduleLock)
        assertEquals(listOf("Home"), nokia.automation.places)
        // At home: on and locked, as Nokia mode was.
        assertTrue(Enforce.enabled(m, nokia, at(3), UTC))
        assertTrue(Enforce.locked(m, nokia, at(3), 1, UTC))
        assertEquals(SettingLock(chars = 20), m.block("Websites")!!.lock)
        val none = Codec.migrate(false, 0.0, 0.0, 150f, false, false, emptyList(), emptyList(), SettingLock())
        assertFalse(none.block(Codec.NOKIA)!!.active)
        assertTrue(none.places.isEmpty())
    }
}

class JsonTest {
    @Test fun values() {
        val v = Json.parse("""{"a":[1,2.5,"xé\n",true,null],"b":{}}""").obj()
        assertEquals(listOf(1L, 2.5, "xé\n", true, null), v["a"])
        assertEquals(emptyMap<String, Any?>(), v["b"])
    }
}

class PlacesTest {
    @Test fun distanceAndStep() {
        // ~111 km per degree of latitude.
        val d = MapMath.distance(0.0, 0.0, 1.0, 0.0)
        assertTrue(d in 110_000.0..112_500.0)
        val p = Place("Home", 52.0, 4.0, 150f)
        assertTrue(Places.step(p, at(3), 52.0005, 4.0, 20f).inside)
        assertFalse(Places.step(p, at(3), 52.01, 4.0, 20f).inside)
    }
}
