package org.kc9k.android

import org.junit.Test
import java.io.File

/** Writes the rules for a sample setup, for ci/android_rules_test.js (run by Node) to check with match.js. */
class SampleRules {
    @Test fun write() {
        val blocked = listOf("https://www.Reddit.com/", "youtube.com/shorts", "*casino*", "*.xxx")
            .map { Entries.normalize(it)!! }
        val json = Rules.json(blocked, listOf(Entries.normalize("reddit.com/r/programming")!!), true, true, true)
        File(System.getProperty("java.io.tmpdir"), "kc9k-android-rules.json").writeText(json)
    }
}
