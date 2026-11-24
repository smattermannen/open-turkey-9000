package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

/** The repository root (tests run in android/app). */
private fun repo(path: String) = File("../../$path")

class PhrasesTest {
    /** The phrase cases Go's TestSharedPhraseCases runs too (R-ANDROID-3). */
    @Test fun sharedCases() {
        val o = Json.parse(repo("internal/db/testdata/phrase_cases.json").readText()).obj()
        val parse = o["parse"].list().map { it.obj() }
        assertTrue(parse.size > 10)
        for (c in parse) {
            val (p, err) = Phrase.parse(c.str("text"))
            if (c.bool("error")) assertNull("${c.str("text")}: parsed as ${p?.key()}", p)
            else {
                assertNull("${c.str("text")}: $err", err)
                assertEquals(c.str("text"), c.str("key"), p!!.key())
            }
        }
        for (c in o["widens"].list().map { it.list() }) {
            val old = Phrase.parse(c[0] as String).first!!
            val new = Phrase.parse(c[1] as String).first!!
            assertEquals("${c[1]} widens ${c[0]}", c[2] as Boolean, new.widens(old))
        }
    }

    /** The social media settings are the desktop's (internal/db/social.go), in its order. */
    @Test fun socialPresetsAsOnTheDesktop() {
        val go = Regex("""\{"([a-z]+:[a-z-]+)", "([^"]+)", "([^"]+)"\}""").findAll(repo("internal/db/social.go").readText())
            .map { Social.Preset(it.groupValues[1], it.groupValues[2], it.groupValues[3]) }.toList()
        assertEquals(go, Social.PRESETS)
    }
}

class PhraseSetCommandsTest {
    private val now = at(10)
    private val c = Commands(now, 1, java.util.TimeZone.getTimeZone("UTC"))
    private val locked = SettingLock(until = now + 3_600_000)
    private val p = Phrase.parse("[a] [b]").first!!
    private val set = PhraseSet("P", listOf(p), inUrl = true, inText = true, threshold = 2)
    private fun m(lock: SettingLock = SettingLock()) =
        Model(blocks = listOf(Block("x", lock = lock, phraseSets = listOf(set), social = listOf("youtube:no-shorts"))))

    @Test fun strengtheningAllowedWhileLocked() {
        val mm = m(locked)
        assertNull(c.addPhraseSet(mm, "x", "Q").error)
        assertNull(c.addPhrases(mm, "x", "P", "[c] [d]\n[e, f] [g]").error)
        assertNull(c.replacePhrase(mm, "x", "P", p, "[a, z] [b]").error)
        assertNull(c.setPhraseScope(mm, "x", "P", true, true, true).error)
        assertNull(c.setPhraseThreshold(mm, "x", "P", 1).error)
        assertNull(c.setPhraseTest(mm, "x", "P", false).error)
        assertNull(c.renamePhraseSet(mm, "x", "P", "R").error)
        assertNull(c.setSocial(mm, "x", "reddit:no-home", true).error)
    }

    /** Every weakening phrase set and social media command, refused and logged while locked. */
    @Test fun weakeningRefusedAndLogged() {
        val mm = m(locked)
        val refused = listOf(
            c.deletePhraseSet(mm, "x", "P"), c.removePhrase(mm, "x", "P", p),
            c.replacePhrase(mm, "x", "P", p, "[a] [c]"), c.setPhraseScope(mm, "x", "P", false, false, true),
            c.setPhraseThreshold(mm, "x", "P", 3), c.setPhraseTest(mm, "x", "P", true),
            c.setSocial(mm, "x", "youtube:no-shorts", false),
        )
        for ((i, o) in refused.withIndex()) {
            assertNotNull("command $i", o.error)
            assertEquals("command $i", mm.blocks, o.model.blocks)
            assertEquals("command $i", 1, o.model.refused.size)
        }
        // Unlocked, the same weakenings go through.
        assertNull(c.deletePhraseSet(m(), "x", "P").error)
        assertNull(c.setSocial(m(), "x", "youtube:no-shorts", false).error)
    }

    @Test fun phrasesChecked() {
        val mm = m()
        assertNotNull(c.addPhrases(mm, "x", "P", "[only]").error)
        assertNotNull(c.addPhrases(mm, "x", "P", "[big-] [cat]").error)
        assertNotNull(c.setSocial(mm, "x", "myspace:no-feed", true).error)
        val added = c.addPhrases(mm, "x", "P", "[A]  [B]\n[c] [d] :2").model.block("x")!!.phraseSets[0].phrases
        assertEquals(listOf("[a] [b]", "[c] [d]"), added.map { it.key() })
    }

    @Test fun rulesCarryPhraseSetsAndSocial() {
        val mm = m()
        val web = Enforce.plan(mm, now).web
        assertEquals(1, web.size)
        val json = Rules.json(web, false, false)
        assertTrue(json, json.contains("\"id\":-1,\"block\":\"x\",\"name\":\"P\""))
        assertTrue(json, json.contains("\"phrase_scope\":[\"url\",\"text\"]"))
        assertTrue(json, json.contains("\"distinct\":2"))
        assertTrue(json, json.contains("\"social\":[\"youtube:no-shorts\"]"))
        // A set without phrases or scope isn't served; a block with nothing for the extension isn't either.
        val empty = Model(blocks = listOf(Block("y", phraseSets = listOf(PhraseSet("E"), set.copy(inUrl = false, inText = false)))))
        assertTrue(Enforce.plan(empty, now).web.isEmpty())
    }
}
