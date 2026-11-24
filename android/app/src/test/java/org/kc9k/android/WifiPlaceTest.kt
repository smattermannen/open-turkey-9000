package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.TimeZone

class WifiPlaceTest {
    private val now = at(10)
    private val c = Commands(now, 1, TimeZone.getTimeZone("UTC"))
    private val locked = SettingLock(until = now + 3_600_000)
    private val a = Network("aa:aa", "Home"); private val b = Network("bb:bb", "Home 5G"); private val x = Network("cc:cc", "Neighbour")
    private val home = Place("Home", networks = listOf(a, b, x), needed = 2)

    @Test fun step() {
        assertTrue(WifiPlace.step(home, now, setOf("aa:aa", "bb:bb", "zz:zz")).inside)
        val left = WifiPlace.step(home.copy(inside = true), now, setOf("aa:aa", "zz:zz"))
        assertFalse(left.inside)
        assertEquals(1, left.lastSeen)
        // No networks at all: unknown, nothing changes.
        assertEquals(home.copy(inside = true), WifiPlace.step(home.copy(inside = true), now, emptySet()))
        // Needs no more than it knows.
        assertTrue(WifiPlace.step(Place("One", networks = listOf(a), needed = 2), now, setOf("aa:aa")).inside)
        // Areas aren't decided by Wi-Fi.
        val area = Place("Park", 1.0, 2.0, 100f)
        assertEquals(area, WifiPlace.step(area, now, setOf("aa:aa")))
    }

    @Test fun createAndEdit() {
        var m = c.setWifiPlace(Model(), "Home", listOf(Network("AA:AA", "Home"), b)).model
        assertEquals(listOf("aa:aa", "bb:bb"), m.place("Home")!!.networks.map { it.bssid })
        assertEquals(2, m.place("Home")!!.needed)
        m = c.setWifiPlace(m, "Home", listOf(a, x)).model
        assertEquals(3, m.place("Home")!!.networks.size)
        assertNotNull(c.setPlaceNeeded(m, "Home", 4).error)
        assertNotNull(c.setWifiPlace(Model(), "Home", emptyList()).error)
        val area = Model(places = listOf(Place("Park", 1.0, 2.0, 100f)))
        assertNotNull(c.setWifiPlace(area, "Park", listOf(a)).error)
        assertNotNull(c.setPlace(m, "Home", 1.0, 2.0, 100f).error)
        assertNotNull(c.removeNetwork(Model(places = listOf(Place("One", networks = listOf(a)))), "One", "aa:aa").error)
    }

    /** A change to a place a locked block uses must strengthen that block. */
    @Test fun lockedUsersDecide() {
        fun m(defaultEnabled: Boolean) = Model(places = listOf(home),
            blocks = listOf(Block("x", lock = locked, automation = Automation(defaultEnabled, places = listOf("Home")))))
        // Disabled by default (the place turns it on): wider is stronger.
        val on = m(false)
        assertNull(c.setWifiPlace(on, "Home", listOf(Network("dd:dd"))).error)
        assertNull(c.setPlaceNeeded(on, "Home", 1).error)
        val refused = listOf(c.setPlaceNeeded(on, "Home", 3), c.removeNetwork(on, "Home", "aa:aa"))
        for (o in refused) { assertNotNull(o.error); assertEquals(on.places, o.model.places); assertEquals(1, o.model.refused.size) }
        // Enabled by default (the place lifts it): narrower is stronger.
        val off = m(true)
        assertNotNull(c.setWifiPlace(off, "Home", listOf(Network("dd:dd"))).error)
        assertNotNull(c.setPlaceNeeded(off, "Home", 1).error)
        assertNull(c.setPlaceNeeded(off, "Home", 3).error)
        assertNull(c.removeNetwork(off, "Home", "cc:cc").error)
    }

    @Test fun decidesBlocks() {
        val m = Model(places = listOf(home.copy(inside = true)), blocks = listOf(Block("x", automation = Automation(places = listOf("Home")))))
        assertTrue(Enforce.enabled(m, m.blocks[0], now))
    }

    @Test fun codec() {
        val m = Model(places = listOf(home.copy(inside = true, lastSeen = 2, lastFix = 5, needed = 3)))
        assertEquals(m, Codec.decode(Codec.encode(m)))
    }
}
