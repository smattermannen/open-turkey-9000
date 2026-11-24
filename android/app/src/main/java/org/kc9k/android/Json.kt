package org.kc9k.android

/**
 * A small JSON reader and writer for the app's own data (Model). org.json isn't usable in
 * JVM unit tests (android.jar stubs), and no third-party libraries are allowed.
 * Values: Map<String, Any?>, List<Any?>, String, Double/Long, Boolean, null.
 */
object Json {
    fun write(v: Any?): String = StringBuilder().also { write(it, v) }.toString()

    private fun write(b: StringBuilder, v: Any?) {
        when (v) {
            null -> b.append("null")
            is String -> b.append(Rules.str(v))
            is Boolean -> b.append(v)
            is Int, is Long -> b.append(v)
            is Float, is Double -> {
                val d = (v as Number).toDouble()
                b.append(if (d.isFinite()) d.toString() else "0")
            }
            is Map<*, *> -> {
                b.append('{')
                var first = true
                for ((k, x) in v) {
                    if (!first) b.append(',')
                    first = false
                    b.append(Rules.str(k.toString())).append(':')
                    write(b, x)
                }
                b.append('}')
            }
            is Iterable<*> -> {
                b.append('[')
                var first = true
                for (x in v) { if (!first) b.append(','); first = false; write(b, x) }
                b.append(']')
            }
            else -> b.append(Rules.str(v.toString()))
        }
    }

    fun parse(s: String): Any? = Parser(s).run {
        val v = value(); ws()
        if (i != s.length) error("trailing text at $i")
        v
    }

    private class Parser(val s: String) {
        var i = 0
        fun ws() { while (i < s.length && s[i].isWhitespace()) i++ }
        fun error(m: String): Nothing = throw IllegalArgumentException("JSON: $m")
        fun value(): Any? {
            ws()
            if (i >= s.length) error("unexpected end")
            return when (s[i]) {
                '{' -> obj()
                '[' -> arr()
                '"' -> str()
                't' -> lit("true", true)
                'f' -> lit("false", false)
                'n' -> lit("null", null)
                else -> num()
            }
        }
        fun lit(w: String, v: Any?): Any? {
            if (!s.startsWith(w, i)) error("bad literal at $i")
            i += w.length; return v
        }
        fun num(): Any {
            val start = i
            while (i < s.length && (s[i].isDigit() || s[i] in "+-.eE")) i++
            val t = s.substring(start, i)
            if (t.isEmpty()) error("bad value at $start")
            return t.toLongOrNull() ?: t.toDoubleOrNull() ?: error("bad number $t")
        }
        fun str(): String {
            i++ // "
            val b = StringBuilder()
            while (true) {
                if (i >= s.length) error("unterminated string")
                val c = s[i++]
                when (c) {
                    '"' -> return b.toString()
                    '\\' -> {
                        val e = s[i++]
                        when (e) {
                            'n' -> b.append('\n'); 't' -> b.append('\t'); 'r' -> b.append('\r')
                            'b' -> b.append('\b'); 'f' -> b.append('\u000c')
                            'u' -> { b.append(s.substring(i, i + 4).toInt(16).toChar()); i += 4 }
                            else -> b.append(e)
                        }
                    }
                    else -> b.append(c)
                }
            }
        }
        fun arr(): List<Any?> {
            i++
            val out = mutableListOf<Any?>()
            ws()
            if (s[i] == ']') { i++; return out }
            while (true) {
                out += value(); ws()
                when (s[i++]) { ',' -> continue; ']' -> return out; else -> error("expected , or ] at ${i - 1}") }
            }
        }
        fun obj(): Map<String, Any?> {
            i++
            val out = LinkedHashMap<String, Any?>()
            ws()
            if (s[i] == '}') { i++; return out }
            while (true) {
                ws()
                if (s[i] != '"') error("expected key at $i")
                val k = str(); ws()
                if (s[i++] != ':') error("expected : at ${i - 1}")
                out[k] = value(); ws()
                when (s[i++]) { ',' -> continue; '}' -> return out; else -> error("expected , or } at ${i - 1}") }
            }
        }
    }
}

// Readers for parsed values, lenient about missing fields (older files).
@Suppress("UNCHECKED_CAST")
fun Any?.obj(): Map<String, Any?> = this as? Map<String, Any?> ?: emptyMap()
fun Any?.list(): List<Any?> = this as? List<Any?> ?: emptyList()
fun Map<String, Any?>.str(k: String, d: String = "") = this[k] as? String ?: d
fun Map<String, Any?>.bool(k: String, d: Boolean = false) = this[k] as? Boolean ?: d
fun Map<String, Any?>.long(k: String, d: Long = 0) = (this[k] as? Number)?.toLong() ?: d
fun Map<String, Any?>.int(k: String, d: Int = 0) = (this[k] as? Number)?.toInt() ?: d
fun Map<String, Any?>.double(k: String, d: Double = 0.0) = (this[k] as? Number)?.toDouble() ?: d
fun Map<String, Any?>.strings(k: String) = this[k].list().mapNotNull { it as? String }
