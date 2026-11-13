package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LockTest {
    private val now = 1_000_000L
    private val boot = 7

    @Test fun zeroIsUnlocked() = assertFalse(SettingLock().inForce(now, boot))

    @Test fun eachKindHolds() {
        assertTrue(SettingLock(until = now + 1).inForce(now, boot))
        assertFalse(SettingLock(until = now).inForce(now, boot))
        assertTrue(SettingLock(chars = 10).inForce(now, boot))
        assertTrue(SettingLock(passwordHash = "x").inForce(now, boot))
        assertTrue(SettingLock(restartBoot = boot).inForce(now, boot))
        assertFalse(SettingLock(restartBoot = boot - 1).inForce(now, boot))
    }

    @Test fun onlyStrengthens() {
        val cur = SettingLock(until = now + 100, chars = 300, passwordHash = "h")
        assertNotNull(SettingLock.strengthen(cur, SettingLock(until = now + 50), now, boot).error)
        assertNotNull(SettingLock.strengthen(cur, SettingLock(chars = 100), now, boot).error)
        assertNotNull(SettingLock.strengthen(cur, SettingLock(passwordHash = "other"), now, boot).error)
        val longer = SettingLock.strengthen(cur, SettingLock(until = now + 200, chars = 400), now, boot)
        assertNull(longer.error)
        assertEquals(now + 200, longer.lock.until)
        assertEquals(400, longer.lock.chars)
        assertEquals("h", longer.lock.passwordHash)
        val restart = SettingLock.strengthen(SettingLock(), SettingLock(restartBoot = boot), now, boot)
        assertTrue(restart.lock.restartLocked(boot))
    }

    @Test fun expiredLocksDrop() {
        val r = SettingLock.strengthen(SettingLock(until = now - 1, restartBoot = boot - 1), SettingLock(chars = 10), now, boot)
        assertEquals(SettingLock(chars = 10), r.lock)
    }

    @Test fun unlockOnlyChallengeAndPassword() {
        assertNotNull(SettingLock(until = now + 1, chars = 10).refuseUnlock(now, boot))
        assertNotNull(SettingLock(restartBoot = boot, passwordHash = "x").refuseUnlock(now, boot))
        assertNull(SettingLock(chars = 10).refuseUnlock(now, boot))
        assertNotNull(SettingLock().refuseUnlock(now, boot))
    }

    @Test fun passwords() {
        val h = Passwords.hash("correct horse")
        assertTrue(Passwords.check(h, "correct horse"))
        assertFalse(Passwords.check(h, "wrong"))
        assertFalse(Passwords.check("garbage", "x"))
    }
}

class ChallengeTest {
    @Test fun generated() {
        val c = Challenge.generate(5000)
        assertEquals(5000, c.length)
        assertFalse(c.contains(' '))
        val letters = c.count { it.isLetter() }
        assertTrue("letters $letters", letters in 2700..3300)
    }

    @Test fun accuracy() {
        val c = "a".repeat(100)
        assertTrue(Challenge.passed(c, "a".repeat(98) + "bb"))
        assertFalse(Challenge.passed(c, "a".repeat(97) + "bbb"))
        assertFalse(Challenge.passed(c, ""))
        assertEquals(3, Challenge.editDistance("kitten", "sitting"))
    }
}

class EntriesTest {
    @Test fun normalize() {
        assertEquals("reddit.com", Entries.normalize("https://www.Reddit.com/"))
        assertEquals("reddit.com", Entries.normalize("reddit.com"))
        assertEquals("reddit.com/r/funny", Entries.normalize("reddit.com/r/funny"))
        assertEquals("*casino*", Entries.normalize("*casino*"))
        assertEquals("*.*", Entries.normalize("*.*"))
        assertEquals("*.xxx", Entries.normalize("*.xxx"))
        assertNull(Entries.normalize("*./"))
        assertNull(Entries.normalize("a b"))
        assertNull(Entries.normalize(""))
        assertEquals(listOf("a.com", "b.com", "c.com"), Entries.split("a.com\nb.com, c.com "))
    }
}

class RulesTest {
    @Test fun daemonFormat() {
        val json = Rules.json(listOf("reddit.com", "a\"b"), listOf("reddit.com/r/x"), true, true, false)
        assertTrue(json, json.contains("\"lists\":[{\"id\":1,\"block\":\"phone\",\"name\":\"Blocked\",\"url\":[\"reddit.com\",\"a\\\"b\"]"))
        assertTrue(json.contains("\"actions\":[{\"action\":\"block\",\"distinct\":1,\"hits\":1}]"))
        assertTrue(json.contains("\"except\":{\"url\":[\"reddit.com/r/x\"],\"title\":[],\"text\":[]}"))
        assertTrue(json.contains("\"safesearch\":true"))
        assertTrue(json.contains("\"web_only\":false"))
        assertTrue(Rules.json(listOf("x.com"), emptyList(), false, false, false).contains("\"lists\":[]"))
    }

    @Test fun required() {
        assertFalse(Rules.extensionRequired(emptyList(), true, false, false))
        assertFalse(Rules.extensionRequired(listOf("x.com"), false, false, false))
        assertTrue(Rules.extensionRequired(listOf("x.com"), true, false, false))
        assertTrue(Rules.extensionRequired(emptyList(), false, true, false))
        assertTrue(Rules.extensionRequired(emptyList(), false, false, true))
    }

    @Test fun versions() {
        assertTrue(compareVersions("1.15.0", "1.9.2") > 0)
        assertEquals(0, compareVersions("1.15", "1.15.0"))
        assertTrue(compareVersions("1.14.0", "1.15.0") < 0)
    }
}

class ExtensionGuardTest {
    private val g = ExtensionGuard

    @Test fun problems() {
        val now = 100_000L
        assertEquals(ExtensionGuard.Problem.NEVER, g.problem(null, now, "1.15.0"))
        assertEquals(ExtensionGuard.Problem.STALE, g.problem(CheckIn(now - 20_000, "1.15.0", true), now, "1.15.0"))
        assertEquals(ExtensionGuard.Problem.OLD, g.problem(CheckIn(now, "1.14.0", true), now, "1.15.0"))
        assertEquals(ExtensionGuard.Problem.NO_PRIVATE, g.problem(CheckIn(now, "1.15.0", false), now, "1.15.0"))
        assertEquals(ExtensionGuard.Problem.NONE, g.problem(CheckIn(now - 3_000, "1.15.0", true), now, "1.15.0"))
    }

    @Test fun graceThenPause() {
        var s = ExtensionGuard.State()
        var t = 0L
        // In front without the extension: a minute of grace, then paused.
        repeat(11) {
            t += 5_000
            val step = g.step(s, t, 5_000, required = true, usageAccess = true, foreground = true, good = false)
            assertFalse("paused early at $t", step.pause); s = step.state
        }
        t += 5_000
        var step = g.step(s, t, 5_000, true, true, true, false)
        assertTrue(step.pause); s = step.state
        // Stays paused, in front or not, for the pause.
        step = g.step(s, t + g.PAUSE_MS - 1, 5_000, true, true, false, false)
        assertTrue(step.pause); s = step.state
        // Then free again, with a fresh minute.
        step = g.step(s, t + g.PAUSE_MS, 5_000, true, true, false, false)
        assertFalse(step.pause)
        assertEquals(0L, step.state.unchecked)
    }

    @Test fun goodCheckInResets() {
        val s = ExtensionGuard.State(unchecked = 55_000)
        val step = g.step(s, 1, 5_000, true, true, true, good = true)
        assertFalse(step.pause)
        assertEquals(ExtensionGuard.State(), step.state)
    }

    @Test fun background() {
        val step = g.step(ExtensionGuard.State(unchecked = 30_000), 1, 5_000, true, true, foreground = false, good = false)
        assertFalse(step.pause)
        assertEquals(30_000L, step.state.unchecked)
    }

    @Test fun failsClosedWithoutUsageAccess() =
        assertTrue(g.step(ExtensionGuard.State(), 1, 5_000, true, usageAccess = false, foreground = false, good = false).pause)

    @Test fun notRequired() =
        assertFalse(g.step(ExtensionGuard.State(pausedUntil = 999), 1, 5_000, required = false, usageAccess = false, foreground = true, good = false).pause)
}

class ListFileTest {
    @Test fun roundTrip() {
        val text = ListFile.format("Blocked", listOf("reddit.com", "*casino*"))
        assertTrue(text.startsWith("# KC9K phone: Blocked\n# kc9k cluster, looks at: url\n"))
        assertEquals(listOf("reddit.com", "*casino*"), ListFile.parse(text))
    }

    @Test fun desktopExports() {
        // `kc9k list export` (bare lines) and `kc9k cluster export` (header comment), CRLF and a BOM.
        val r = ListFile.read("﻿# kc9k cluster, looks at: url\r\nhttps://www.Reddit.com/\r\n\r\n  x.com  \nnot an address\nx.com\n")
        assertEquals(listOf("reddit.com", "x.com"), r.entries)
        assertEquals(listOf("not an address"), r.skipped)
    }

    @Test fun summary() {
        assertEquals("Imported 1 entry into Blocked", ListFile.summary("Blocked", 1, 0, emptyList()))
        assertEquals("Imported 2 entries into Exceptions (3 already there; skipped, not addresses: a b)",
            ListFile.summary("Exceptions", 2, 3, listOf("a", "b")))
    }
}

class PlanTest {
    private val targets = setOf("com.android.settings", "org.example.game", "com.android.documentsui")

    @Test fun nokiaPausesSettings() =
        assertTrue("com.android.settings" in Plan.suspend(true, targets, emptySet(), emptyList()))

    @Test fun pickerLetThrough() {
        val s = Plan.suspend(true, targets, setOf("com.android.documentsui"), emptyList())
        assertEquals(setOf("com.android.settings", "org.example.game"), s)
    }

    @Test fun awayOrLiftedOnlyIronFoxPause() {
        assertEquals(emptySet<String>(), Plan.suspend(false, targets, emptySet(), emptyList()))
        assertEquals(setOf("org.ironfoxoss.ironfox"), Plan.suspend(false, targets, emptySet(), listOf("org.ironfoxoss.ironfox")))
    }

    @Test fun nokiaPolicyBlocksSettings() {
        assertTrue(Policies.SETTINGS in Policies.NOKIA.blacklist)
        assertFalse(Policies.SETTINGS in Policies.NOKIA.whitelist)
    }
}
