package org.kc9k.android

import java.text.SimpleDateFormat
import java.util.Locale
import java.util.TimeZone

/**
 * Notifications a few minutes before something happens (SPEC §4.10, as the desktop's
 * enforce/notices.go): an active block's windows switching it on or off, a timer lock
 * ending; and an app paused. The user picks the kinds and how many minutes ahead (each
 * advance once). Pure: GuardService shows each [Notice] once, by its key.
 */
object Notices {
    const val WINDOW_START = "window-start"
    const val WINDOW_END = "window-end"
    const val TIMER_END = "timer-end"
    const val APP_BLOCKED = "app-blocked"
    val KINDS = listOf(WINDOW_START, WINDOW_END, TIMER_END, APP_BLOCKED)
    val LABELS = mapOf(WINDOW_START to "A block's window starts", WINDOW_END to "A block's window ends",
        TIMER_END to "A timer lock ends", APP_BLOCKED to "Apps are paused")

    data class Options(val off: Set<String> = emptySet(), val advance: List<Int> = listOf(5)) {
        fun on(kind: String) = kind !in off
        val lead get() = maxOf(1, advance.maxOrNull() ?: 1)
        /** The smallest advance at least [m] minutes, or 0 when the event is further away than every advance. */
        fun advanceFor(m: Int) = advance.filter { it >= m }.minOrNull() ?: 0

        companion object {
            /** "2,5" to minutes (1–1440); null if not a list of them. */
            fun parseAdvance(s: String): List<Int>? {
                val parts = s.split(',', ' ').map { it.trim() }.filter { it.isNotEmpty() }
                val n = parts.map { it.toIntOrNull() ?: return null }
                if (n.isEmpty() || n.any { it !in 1..1440 }) return null
                return n.distinct().sorted()
            }
        }
    }

    data class Notice(val key: String, val title: String, val body: String)

    private fun minutes(m: Int) = if (m == 1) "1 minute" else "$m minutes"

    /** In how many minutes, within [lead], [on] (false now) becomes true, or 0. */
    private fun startsWithin(on: (Long) -> Boolean, now: Long, lead: Int): Int {
        if (on(now)) return 0
        for (m in 1..lead) if (on(now + m * 60_000L)) return m
        return 0
    }

    fun of(model: Model, o: Options, now: Long, tz: TimeZone = TimeZone.getDefault()): List<Notice> {
        val out = mutableListOf<Notice>()
        val minute = now - now % 60_000L
        val fmt = SimpleDateFormat("yyyy-MM-dd HH:mm", Locale.ROOT).apply { timeZone = tz }
        fun ahead(kind: String, name: String, m: Int, title: String, body: String) {
            if (!o.on(kind) || m <= 0) return
            val a = o.advanceFor(m)
            if (a > 0) out += Notice("$kind $name ${fmt.format(minute + m * 60_000L)} +$a", title, body)
        }
        // Windows: only of blocks that are switched on and switched by time (places can't be foreseen).
        for (b in model.blocks.filter { it.active && it.pausedUntil <= now }.sortedBy { it.name }) {
            val a = model.automation(b)
            if (a.windows.isEmpty() && a.schedules.isEmpty()) continue
            val on = { t: Long -> Enforce.enabled(model, b, t, tz) }
            startsWithin(on, now, o.lead).let { ahead(WINDOW_START, b.name, it, "'${b.name}' starts in ${minutes(it)}", "Its schedule enables the block then.") }
            startsWithin({ t -> !on(t) }, now, o.lead).let { ahead(WINDOW_END, b.name, it, "'${b.name}' ends in ${minutes(it)}", "Its window closes then.") }
        }
        for (b in model.blocks.filter { it.lock.until > now }.sortedBy { it.name }) {
            val m = ((b.lock.until - now + 59_999) / 60_000).toInt()
            ahead(TIMER_END, b.name, m, "The timer lock of '${b.name}' ends in ${minutes(m)}", "The block stays on; it can be switched off then.")
        }
        return out
    }

    /** The notice for apps newly paused (shown names), or null. */
    fun paused(o: Options, names: List<String>): Notice? {
        if (!o.on(APP_BLOCKED) || names.isEmpty()) return null
        val shown = names.sorted()
        return Notice("$APP_BLOCKED ${shown.joinToString(",")}", "Paused: " + shown.take(3).joinToString(", ") +
            if (shown.size > 3) " and ${shown.size - 3} more" else "", "A block that is on pauses them.")
    }
}
