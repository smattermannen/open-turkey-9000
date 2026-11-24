package org.kc9k.android

import android.os.UserManager
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.TimeZone

class DeviceRulesTest {
    @Test fun strictOnlyWhileSomethingIsLocked() {
        val d = Device(strict = true, leftOut = setOf("location"), tamper = emptySet())
        val idle = DeviceRules.plan(d, lockdown = false, anything = false, lifted = false)
        assertTrue(idle.restrictions.isEmpty())
        assertFalse(idle.uninstallBlocked)
        val locked = DeviceRules.plan(d, lockdown = true, anything = true, lifted = false)
        assertTrue(UserManager.DISALLOW_SAFE_BOOT in locked.restrictions)
        assertTrue(UserManager.DISALLOW_FACTORY_RESET in locked.restrictions)
        assertFalse("left out", UserManager.DISALLOW_CONFIG_LOCATION in locked.restrictions)
        assertFalse(locked.location)
        assertTrue(locked.uninstallBlocked && locked.autoTime && locked.closeBrowsers)
        assertTrue(UserManager.DISALLOW_CONFIG_DATE_TIME in locked.restrictions)
        // Strict mode's own lock alone doesn't start it, but counts as something locked.
        val ownLock = DeviceRules.plan(d, lockdown = false, anything = true, lifted = false)
        assertFalse(UserManager.DISALLOW_SAFE_BOOT in ownLock.restrictions)
        assertTrue(ownLock.uninstallBlocked)
        // Off: nothing, whatever is locked.
        assertTrue(DeviceRules.plan(Device(tamper = emptySet()), true, true, false).restrictions.none {
            it in StrictPart.entries.flatMap { p -> p.restrictions } && it != UserManager.DISALLOW_CONFIG_DATE_TIME })
    }

    @Test fun tamperItems() {
        val all = Device(tamper = Tamper.entries.map { it.key }.toSet())
        val idle = DeviceRules.plan(all, false, false, false)
        assertTrue(UserManager.DISALLOW_ADD_MANAGED_PROFILE in idle.restrictions)
        assertTrue(DeviceRules.NO_CLONE_PROFILE in idle.restrictions)
        assertFalse("installer only while locked", DeviceRules.NO_UNKNOWN_SOURCES in idle.restrictions)
        assertTrue(idle.appControls && idle.autoTime && idle.closeBrowsers)
        assertTrue(DeviceRules.NO_UNKNOWN_SOURCES in DeviceRules.plan(all, false, true, false).restrictions)
        // Close browsers off: IronFox paused only while anything is locked.
        val none = Device(tamper = emptySet())
        assertFalse(DeviceRules.plan(none, false, false, false).closeBrowsers)
        assertTrue(DeviceRules.plan(none, false, true, false).closeBrowsers)
        // Lifted: nothing at all.
        assertEquals(DevicePlan(emptySet(), false, false, false, false, false), DeviceRules.plan(all, true, true, true))
        assertTrue(DeviceRules.ALL.containsAll(DeviceRules.plan(all, true, true, false).restrictions))
    }
}

class DeviceCommandsTest {
    private val now = at(10)
    private val c = Commands(now, 1, TimeZone.getTimeZone("UTC"), settingsUnlocked = true)
    private val timeLock = SettingLock(until = now + 3_600_000)
    private val on = Device(strict = true, tamper = Tamper.entries.map { it.key }.toSet())

    @Test fun strengtheningFree() {
        val m = Model(device = Device(), blocks = listOf(Block("x", lock = timeLock)))
        assertTrue(c.setStrict(m, true).model.device.strict)
        assertNull(c.setStrictPart(m.copy(device = on.copy(leftOut = setOf("users"))), StrictPart.USERS, false).error)
        assertNull(c.setTamper(m, Tamper.INSTALLER, true).error)
        assertNull(c.lockDevice(m.copy(device = on), Device.STRICT, timeLock).error)
        assertNotNull(c.lockDevice(m, Device.STRICT, timeLock).error) // strict mode off
    }

    /** Every weakening device command, refused and logged while locked. */
    @Test fun weakeningRefusedAndLogged() {
        val ownLock = Model(device = on.copy(strictLock = timeLock, tamperLock = timeLock), noWeakening = false)
        val blockLocked = Model(device = on, noWeakening = true, blocks = listOf(Block("x", lock = timeLock)))
        val tamperWhileLocked = Model(device = on, noWeakening = false, blocks = listOf(Block("x", lock = timeLock)))
        val refused = listOf(
            c.setStrict(ownLock, false), c.setStrictPart(ownLock, StrictPart.SAFE_BOOT, true), c.setTamper(ownLock, Tamper.CLOCK, false),
            c.setStrict(blockLocked, false), c.setStrictPart(blockLocked, StrictPart.DEBUGGING, true),
            // R-LOCK-9: whatever no weakening says.
            c.setTamper(tamperWhileLocked, Tamper.APP_CONTROLS, false),
        )
        for ((i, o) in refused.withIndex()) {
            assertNotNull("command $i", o.error)
            assertEquals("command $i", 1, o.model.refused.size)
        }
        // With no weakening off and only a block locked, strict mode may go (R-STRICT-1).
        assertNull(c.setStrict(tamperWhileLocked, false).error)
        // Unlocked: all free.
        val free = Model(device = on)
        assertNull(c.setTamper(free, Tamper.CLOCK, false).error)
        assertFalse(c.setStrict(free, false).model.device.strict)
    }

    @Test fun locksOpenInOrder() {
        val m = Model(device = on.copy(tamperLock = SettingLock(chars = 20)))
        assertNull(c.refuseDeviceUnlock(m, Device.TAMPER))
        assertEquals(SettingLock(), c.deviceUnlocked(m, Device.TAMPER).model.device.tamperLock)
        assertNotNull(c.refuseDeviceUnlock(m.copy(device = on.copy(tamperLock = timeLock)), Device.TAMPER))
        // Locked tamper protection counts as something locked; strict mode's own lock doesn't start strict mode.
        assertTrue(Enforce.lockdown(m, now, 1))
        assertFalse(Enforce.lockdown(Model(device = on.copy(strictLock = timeLock)), now, 1))
        assertTrue(Enforce.anythingLocked(Model(device = on.copy(strictLock = timeLock)), now, 1))
    }

    @Test fun codec() {
        val m = Model(device = Device(true, setOf("users"), SettingLock(chars = 30), setOf("clock"), timeLock))
        assertEquals(m, Codec.decode(Codec.encode(m)))
        assertEquals(Tamper.DEFAULTS, Codec.decode("{}").device.tamper)
    }
}
