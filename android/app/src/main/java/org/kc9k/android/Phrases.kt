package org.kc9k.android

/** One place of a phrase: the terms that may fill it (as the desktop's db.WordSet). */
data class WordSet(val terms: List<String>, val necessary: Boolean = true)

/**
 * A phrase (SPEC §3.2, R-PHRASE-1–8), as the desktop's db.Phrase: word sets and a length
 * (0 = the default, [minLength]). Written as `[eat, nice] [hello, grandpa] [today]? :3`.
 * Held to the Go code by the shared cases in internal/db/testdata/phrase_cases.json.
 */
data class Phrase(val sets: List<WordSet>, val length: Int = 0) {
    fun necessary() = sets.count { it.necessary }
    /** The shortest length allowed: the necessary word sets, but at least two terms (R-PHRASE-1). */
    fun minLength() = maxOf(necessary(), 2)
    private fun effectiveLength() = if (length == 0) minLength() else length

    /** Trimmed, lowercased terms; empty ones (and bare `-`, `*`) dropped. */
    fun normalize() = Phrase(sets.map { s ->
        WordSet(s.terms.map { it.trim().split(Regex("\\s+")).filter(String::isNotEmpty).joinToString(" ").lowercase() }
            .filter { it.isNotEmpty() && it.trim('*', '-').isNotEmpty() }, s.necessary)
    }, length)

    /** Why the phrase can't be used (R-PHRASE-1, R-PHRASE-8), or null. */
    fun check(): String? {
        if (sets.size < 2) return "A phrase needs at least two word sets, such as [eat, nice] [hello, grandpa]"
        if (sets.size > MAX_WORD_SETS) return "A phrase has at most $MAX_WORD_SETS word sets"
        val ins = BooleanArray(sets.size); val outs = BooleanArray(sets.size)
        for ((i, s) in sets.withIndex()) {
            if (s.terms.isEmpty()) return "A word set is empty"
            for (t in s.terms) {
                if (t.length > 100) return "The term \"${t.take(20)}…\" is too long"
                val (inn, out) = bridgeEnds(t)
                ins[i] = ins[i] || inn; outs[i] = outs[i] || out
            }
        }
        fun elsewhere(has: BooleanArray, vararg skip: Int) = has.indices.any { has[it] && it !in skip }
        for ((i, s) in sets.withIndex()) for (t in s.terms) {
            val (inn, out) = bridgeEnds(t)
            when {
                inn && out -> {
                    if (sets.size < 3) return "A middle link ($t) needs at least three word sets"
                    if (sets.indices.none { a -> outs[a] && a != i && elsewhere(ins, i, a) })
                        return "The middle link $t needs a term ending in - (big-) and one starting with - (-gorilla), each in another word set"
                }
                inn && !elsewhere(outs, i) -> return "A bridge needs both ends: $t needs a term ending in - (big-) in another word set"
                out && !elsewhere(ins, i) -> return "A bridge needs both ends: $t needs a term starting with - (-gorilla) in another word set"
            }
        }
        if (length != 0 && (length < minLength() || length > sets.size))
            return "The phrase length is from ${minLength()} (its necessary word sets, at least 2) to ${sets.size} (all its word sets)"
        return null
    }

    /** As typed: `[a, b] [c]? :N` (the length only when not the default). */
    override fun toString(): String {
        val s = sets.joinToString(" ") { "[" + it.terms.joinToString(", ") + "]" + if (it.necessary) "" else "?" }
        return if (length != 0 && length != minLength()) "$s :$length" else s
    }

    /** The one stored form: normalized, the default length written as 0. */
    fun canonical(): Phrase = normalize().let { if (it.length == it.minLength()) it.copy(length = 0) else it }
    fun key() = canonical().toString()

    /**
     * Whether this matches everything [old] matches: the same word sets, each just as
     * necessary and holding at least old's terms, and the same length (not weakening, R-WEAK-5).
     */
    fun widens(old: Phrase): Boolean {
        val p = normalize(); val o = old.normalize()
        if (p.sets.size != o.sets.size || p.effectiveLength() != o.effectiveLength()) return false
        return o.sets.indices.all { i -> o.sets[i].necessary == p.sets[i].necessary && p.sets[i].terms.containsAll(o.sets[i].terms) }
    }

    companion object {
        const val MAX_WORD_SETS = 100

        /** A bridge's end (-gorilla) and start (big-); a middle link (-strong-) is both (R-PHRASE-7). */
        fun bridgeEnds(t: String) = Pair(t.startsWith("-"), t.endsWith("-") && t.length > 1)

        /** Reads and checks a phrase; the phrase or why not. */
        fun parse(text: String): Pair<Phrase?, String?> {
            val (p, err) = parseLoose(text)
            if (p == null) return null to err
            p.check()?.let { return null to it }
            return p to null
        }

        /** Reads a phrase without checking it can be used. */
        fun parseLoose(text: String): Pair<Phrase?, String?> {
            var rest = text.trim()
            var length = 0
            val colon = rest.lastIndexOf(':')
            if (colon >= 0 && !rest.substring(colon).contains(']')) {
                val n = Regex("^[+-]?\\d+").find(rest.substring(colon + 1).trim())?.value?.toIntOrNull()
                if (n == null || n < 1) return null to "After : comes the phrase length, a number"
                length = n
                rest = rest.substring(0, colon).trim()
            }
            val sets = mutableListOf<WordSet>()
            while (rest.isNotEmpty()) {
                if (!rest.startsWith("[")) return null to "Write a phrase as word sets in brackets: [eat, nice] [hello, grandpa]"
                val end = rest.indexOf(']')
                if (end < 0) return null to "A [ has no ]"
                val terms = rest.substring(1, end).split(',').map { it.trim() }.filter { it.isNotEmpty() }
                rest = rest.substring(end + 1).trim()
                var necessary = true
                if (rest.startsWith("?")) { necessary = false; rest = rest.substring(1).trim() }
                sets += WordSet(terms, necessary)
            }
            return Phrase(sets, length).normalize() to null
        }
    }
}

/**
 * A phrase set in a block (SPEC §3.2): named phrases matched in the address, title and/or
 * text. It blocks the page once [threshold] different phrases match (R-PHRASE-9); in
 * [test] mode it only reports.
 */
data class PhraseSet(
    val name: String,
    val phrases: List<Phrase> = emptyList(),
    val inUrl: Boolean = false,
    val inTitle: Boolean = false,
    val inText: Boolean = true,
    val threshold: Int = 1,
    val test: Boolean = false,
) {
    val scope get() = listOfNotNull("url".takeIf { inUrl }, "title".takeIf { inTitle }, "text".takeIf { inText })
}

/** Social media settings (as the desktop's db.SocialPresets): things taken away from a site. */
object Social {
    data class Preset(val name: String, val site: String, val title: String)

    val PRESETS = listOf(
        Preset("youtube:no-shorts", "YouTube", "No Shorts"),
        Preset("youtube:no-home", "YouTube", "No home feed (opens Subscriptions)"),
        Preset("youtube:no-recommendations", "YouTube", "No recommendations beside and after videos"),
        Preset("youtube:no-comments", "YouTube", "No comments"),
        Preset("instagram:messages-only", "Instagram", "Messages only"),
        Preset("instagram:following-only", "Instagram", "Following only (no suggested posts)"),
        Preset("instagram:no-reels", "Instagram", "No Reels"),
        Preset("instagram:no-explore", "Instagram", "No Explore page"),
        Preset("facebook:no-feed", "Facebook", "No news feed"),
        Preset("facebook:no-reels", "Facebook", "No Reels or Watch"),
        Preset("facebook:messages-only", "Facebook", "Messages only"),
        Preset("x:no-explore", "X", "No Explore page or trends"),
        Preset("x:messages-only", "X", "Messages only"),
        Preset("reddit:no-popular", "Reddit", "No Popular or All"),
        Preset("reddit:no-home", "Reddit", "No home feed"),
        Preset("tiktok:following-only", "TikTok", "Following only (no For You)"),
        Preset("tiktok:no-explore", "TikTok", "No Explore page"),
        Preset("linkedin:no-feed", "LinkedIn", "No feed"),
        Preset("linkedin:no-news", "LinkedIn", "No news and suggestions column"),
    )

    fun valid(name: String) = PRESETS.any { it.name == name }
}
