package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.TimeZone

/** The settings password (SPEC §3.11): every weakening needs it once set; strengthening never does. */
class SettingsPasswordTest {
    private val now = at(10)
    private val utc = TimeZone.getTimeZone("UTC")
    private val shut = Commands(now, 1, utc, settingsUnlocked = false)
    private val open = Commands(now, 1, utc, settingsUnlocked = true)
    private val pw = "pbkdf2\$1\$aa\$bb"
    private val p = Phrase.parse("[a] [b]").first!!
    private val m = Model(
        blocks = listOf(Block("x", sites = listOf("a.com"), apps = listOf("com.game"), wifiOff = true, social = listOf("youtube:no-shorts"),
            phraseSets = listOf(PhraseSet("P", listOf(p))), lock = SettingLock(chars = 20),
            automation = Automation(windows = listOf(Window(0, 540, 600)), places = listOf("Home")), mode = ""),
            Block("y", mode = "M", automation = Automation(windows = listOf(Window(0, 540, 570))))),
        modes = listOf(Mode("M", Automation(windows = listOf(Window(0, 540, 600))))),
        places = listOf(Place("Home", networks = listOf(Network("aa:aa"), Network("bb:bb")), needed = 1)),
        schedules = listOf(Schedule("S", listOf(Window(1, 0, 60)))),
        settingsPassword = pw, noWeakening = false,
    ).let { it.copy(blocks = it.blocks.map { b -> if (b.name == "x") b.copy(lock = SettingLock(), automation = b.automation.copy(schedules = listOf("S"))) else b }) }

    @Test fun weakeningNeedsIt() {
        val stops = listOf(
            shut.setActive(m, "x", false), shut.deleteBlock(m, "x"), shut.removeSite(m, "x", "a.com"),
            shut.addSiteExceptions(m, "x", "a.com/ok"), shut.removeApp(m, "x", "com.game"), shut.setWifiOff(m, "x", false),
            shut.pause(m, "x", now + 60_000), shut.setAutomation(m, "x", m.block("x")!!.automation.copy(windows = emptyList())),
            shut.removePhrase(m, "x", "P", p), shut.setSocial(m, "x", "youtube:no-shorts", false),
            shut.setModeAutomation(m, "M", Automation(windows = listOf(Window(0, 540, 570)))), shut.takeOut(m, "y"),
            shut.switchMode(m, "M", false), shut.setScheduleWindows(m, "S", emptyList()), shut.deleteSchedule(m, "S"),
            shut.setPlaceNeeded(m, "Home", 2), shut.deletePlace(m, "Home"),
            shut.setSettingsPassword(m, ""), shut.setSettingsPassword(m, "other"),
            shut.setNoWeakening(m.copy(noWeakening = true), false),
        )
        for ((i, o) in stops.withIndex()) {
            assertTrue("command $i: ${o.error}", o.needsPassword)
            assertEquals("command $i", Commands.NEEDS_PASSWORD, o.error)
            assertEquals("command $i", m.copy(noWeakening = o.model.noWeakening), o.model)
            assertTrue("command $i not logged", o.model.refused.isEmpty())
        }
        assertEquals(Commands.NEEDS_PASSWORD, shut.whySettingsLocked(m))
        assertEquals(Commands.NEEDS_PASSWORD, shut.refuseUnlock(m.copy(blocks = listOf(Block("z", lock = SettingLock(chars = 20)))), "z"))
        // Typed: the same commands go through.
        assertNull(open.setActive(m, "x", false).error)
        assertNull(open.deletePlace(m, "Home").error)
        assertNull(open.whySettingsLocked(m))
        assertNull(open.setSettingsPassword(m, "").error)
    }

    @Test fun strengtheningNeverNeedsIt() {
        assertNull(shut.addSites(m, "x", "b.com").error)
        assertNull(shut.addApps(m, "x", listOf("com.other")).error)
        assertNull(shut.addPhrases(m, "x", "P", "[c] [d]").error)
        assertNull(shut.setSocial(m, "x", "reddit:no-home", true).error)
        assertNull(shut.lock(m, "x", SettingLock(until = now + 60_000), false).error)
        assertNull(shut.addBlock(m, "z").error)
        assertNull(shut.setWifiPlace(m, "Home", listOf(Network("cc:cc"))).error)
    }

    @Test fun locksMayNeedIt() {
        val mm = shut.setLockNeedsPassword(m, true).model
        assertTrue(mm.lockNeedsPassword)
        assertTrue(shut.lock(mm, "x", SettingLock(until = now + 60_000), false).needsPassword)
        assertTrue(shut.lockMode(mm, "M", SettingLock(until = now + 60_000), false).needsPassword)
        assertNull(open.lock(mm, "x", SettingLock(until = now + 60_000), false).error)
        assertTrue(shut.setLockNeedsPassword(mm, false).needsPassword)
        assertFalse(shut.setLockNeedsPassword(Model(), true).error == null)
    }

    @Test fun lockedStaysRefusedWithIt() {
        val locked = m.copy(noWeakening = true, blocks = m.blocks + Block("l", lock = SettingLock(until = now + 60_000)))
        val o = open.setSettingsPassword(locked, "")
        assertFalse(o.needsPassword)
        assertEquals(1, o.model.refused.size)
        assertEquals(pw, o.model.settingsPassword)
    }

    @Test fun firstPasswordIsFree() {
        val o = shut.setSettingsPassword(Model(), pw)
        assertNull(o.error)
        assertEquals(pw, o.model.settingsPassword)
    }
}

class AccessPasswordTest {
    private val shut = Commands(at(10), 1, TimeZone.getTimeZone("UTC"), settingsUnlocked = false)

    @Test fun areas() {
        val m = shut.setAccessPassword(Model(settingsPassword = "s"), "blocks", "h").model
        assertEquals(mapOf("blocks" to "h"), m.access)
        assertTrue(shut.setAccessPassword(m, "blocks", "").needsPassword)
        assertTrue(shut.setAccessPassword(m, "blocks", "h2").needsPassword)
        assertFalse(shut.setAccessPassword(m, "nowhere", "h").error == null)
        // Without a settings password, changing one is free.
        assertNull(shut.setAccessPassword(m.copy(settingsPassword = ""), "blocks", "").error)
    }

    @Test fun blockPasswordAndCodec() {
        val m = shut.setBlockPassword(Model(blocks = listOf(Block("x"))), "x", "v").model
        assertEquals("v", m.block("x")!!.viewPassword)
        val full = m.copy(settingsPassword = "s", lockNeedsPassword = true, access = mapOf("app" to "a", "more" to "b"))
        assertEquals(full, Codec.decode(Codec.encode(full)))
    }
}
