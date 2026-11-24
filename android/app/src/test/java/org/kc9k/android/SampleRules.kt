package org.kc9k.android

import org.junit.Test
import java.io.File

/** Writes the rules for a sample setup, for ci/android_rules_test.js (run by Node) to check with match.js. */
class SampleRules {
    @Test fun write() {
        val blocked = listOf("https://www.Reddit.com/", "youtube.com/shorts", "*casino*", "*.xxx")
            .map { Entries.normalize(it)!! }
        val exceptions = listOf(Entries.normalize("reddit.com/r/programming")!!)
        val phrases = PhraseSet("Gambling", listOf("[online, live] [casino, poker]", "[big-] [-jackpot]").map { Phrase.parse(it).first!! })
        val json = Rules.json(listOf(
            WebRule("Websites", blocked, exceptions),
            WebRule("Words", phraseSets = listOf(phrases, PhraseSet("Unused")), social = listOf("youtube:no-shorts")),
            WebRule("More", social = listOf("youtube:no-shorts", "reddit:no-home")),
        ), true, true)
        File(System.getProperty("java.io.tmpdir"), "kc9k-android-rules.json").writeText(json)
    }
}
