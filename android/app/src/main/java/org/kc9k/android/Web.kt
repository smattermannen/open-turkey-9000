package org.kc9k.android

/**
 * Browser settings, as on the desktop (`kc9k browser`). The KC9K extension in IronFox
 * enforces them; each can be locked on its own with a [SettingLock].
 */
enum class BrowserSetting(val key: String, val label: String, val help: String) {
    SAFESEARCH("safesearch", "Enforce SafeSearch",
        "Strict SafeSearch on Google, Bing, DuckDuckGo, Brave Search, Yahoo and SearXNG, and " +
            "YouTube's Restricted Mode. Search engines where it can't be forced (Startpage, Mojeek, " +
            "Marginalia, Ecosia, Yandex) are blocked while it is on."),
    WEB_ONLY("web-only", "Force web results",
        "Google shows web results only: no images, videos, shopping, news or AI answers."),
}

/** The phone's one website block: a blacklist and its exceptions, by address. */
object WebBlock {
    const val NAME = "phone"
    const val LOCK = "websites"
    const val BLOCKED_ID = 1
    const val BLOCKED = "Blocked"
    const val EXCEPTIONS = "Exceptions"
}

/** Website list entries, normalized like the desktop's normalizeURLEntry (internal/cli/weblists.go). */
object Entries {
    private val label = Regex("^[a-z0-9]([a-z0-9-]*[a-z0-9])?$")

    /** The entry as stored, or null if it can't match an address. */
    fun normalize(raw: String): String? {
        var e = raw.trim()
        if (e.isEmpty() || e.any { it.isWhitespace() }) return null
        if (e == "*" || e == "*.*") return e
        if (e.trim('*', '.', '/').isEmpty()) return null
        val scheme = e.indexOf("://")
        var bare = if (scheme > 0) e.substring(scheme + 3) else e
        bare = bare.removeSuffix("/")
        if (bare.isNotEmpty() && !bare.contains('/') && !bare.contains('*')) {
            val host = bare.lowercase().substringAfterLast('@').substringBefore(':').removePrefix("www.")
            if (isDomain(host)) return host
        }
        if (!e.any { it.isLetterOrDigit() }) return null
        return e
    }

    fun isDomain(s: String): Boolean {
        if (s.length !in 1..253) return false
        val labels = s.split('.')
        if (labels.size < 2) return false
        if (labels.any { it.length > 63 || !label.matches(it) }) return false
        val tld = labels.last()
        return tld.length >= 2 && tld.any { it.isLetter() }
    }

    /** Splits pasted text (one entry per line, or separated by spaces or commas). */
    fun split(text: String): List<String> = text.split(Regex("[\\s,]+")).filter { it.isNotBlank() }
}

/**
 * Website list files, the desktop's format: one entry per line, `#` starts a comment. So a
 * phone export is read by `kc9k list import` and `kc9k cluster import` (the header line says
 * it looks at addresses), and a desktop `kc9k list export` or `kc9k cluster export` file
 * imports here.
 */
object ListFile {
    /** The desktop's cluster header (internal/cli/clusters.go, clusterHeader). */
    private const val CLUSTER_HEADER = "# kc9k cluster, looks at: url"

    fun format(list: String, entries: List<String>) = buildString {
        append("# KC9K phone: $list\n")
        append(CLUSTER_HEADER).append('\n')
        entries.forEach { append(it).append('\n') }
    }

    /** The file's entries: lines trimmed, blank lines and comments dropped. */
    fun parse(text: String): List<String> = text.lineSequence().map { it.trim().removePrefix("\uFEFF") }
        .filter { it.isNotEmpty() && !it.startsWith("#") }.toList()

    data class Read(val entries: List<String>, val skipped: List<String>)

    /** The file's entries normalized as when typed in (Entries.normalize); the rest skipped. */
    fun read(text: String): Read {
        val good = mutableListOf<String>()
        val bad = mutableListOf<String>()
        for (raw in parse(text)) Entries.normalize(raw)?.let(good::add) ?: bad.add(raw)
        return Read(good.distinct(), bad)
    }

    /** "Imported 12 entries into Blocked (3 already there; skipped, not addresses: a b)". */
    fun summary(list: String, added: Int, already: Int, skipped: List<String>) = buildString {
        append("Imported $added ${if (added == 1) "entry" else "entries"} into $list")
        val notes = mutableListOf<String>()
        if (already > 0) notes += "$already already there"
        if (skipped.isNotEmpty()) notes += "skipped, not addresses: " +
            skipped.take(5).joinToString(" ") + if (skipped.size > 5) " …" else ""
        if (notes.isNotEmpty()) append(" (").append(notes.joinToString("; ")).append(")")
    }
}

/** The rules the extension fetches from /rules, in the desktop daemon's format (enforce.Rules). */
object Rules {
    fun json(blocked: List<String>, exceptions: List<String>, websitesOn: Boolean,
             safeSearch: Boolean, webOnly: Boolean): String {
        val lists = if (websitesOn && blocked.isNotEmpty()) listOf(
            "{\"id\":${WebBlock.BLOCKED_ID},\"block\":${str(WebBlock.NAME)},\"name\":${str(WebBlock.BLOCKED)}," +
                "\"url\":${arr(blocked)},\"title\":[],\"text\":[]," +
                "\"actions\":[{\"action\":\"block\",\"distinct\":1,\"hits\":1}]," +
                "\"except\":{\"url\":${arr(exceptions)},\"title\":[],\"text\":[]}}"
        ) else emptyList()
        return "{\"lists\":[${lists.joinToString(",")}],\"quota\":[],\"block\":[],\"exceptions\":[]," +
            "\"web_only\":$webOnly,\"safesearch\":$safeSearch,\"titles\":[],\"recording\":false," +
            "\"details\":false,\"ends\":{},\"soon\":[],\"blocks\":[],\"social\":[]}"
    }

    /** Whether IronFox needs the extension, like the desktop's enforce.ExtensionRequired. */
    fun extensionRequired(blocked: List<String>, websitesOn: Boolean, safeSearch: Boolean, webOnly: Boolean) =
        (websitesOn && blocked.isNotEmpty()) || safeSearch || webOnly

    fun str(s: String): String = buildString {
        append('"')
        for (c in s) when {
            c == '"' -> append("\\\"")
            c == '\\' -> append("\\\\")
            c < ' ' || c == ' ' || c == ' ' -> append("\\u%04x".format(c.code))
            else -> append(c)
        }
        append('"')
    }

    private fun arr(items: List<String>) = items.joinToString(",", "[", "]") { str(it) }
}

/** Dotted version comparison: "1.15.0" vs "1.9.2". */
fun compareVersions(a: String, b: String): Int {
    val x = a.split('.').map { it.toIntOrNull() ?: 0 }
    val y = b.split('.').map { it.toIntOrNull() ?: 0 }
    for (i in 0 until maxOf(x.size, y.size)) {
        val d = x.getOrElse(i) { 0 } - y.getOrElse(i) { 0 }
        if (d != 0) return d
    }
    return 0
}
