package org.kc9k.android

import java.text.SimpleDateFormat
import java.util.Locale
import java.util.TimeZone

/**
 * An autostart (SPEC §3.6): switches its block on. [ONCE] at [at] (ms); [AT] the time of
 * day [minute] on [days] (0 = Monday), within ten minutes after it, once a day; [UNLOCK]
 * when the phone is unlocked (sign-in/wake on a phone, §8.3); [WINDOW] when its next block
 * window opens. Once and window fire one time and go.
 */
data class Autostart(
    val kind: String,
    val at: Long = 0,
    val days: List<Int> = emptyList(),
    val minute: Int = 0,
    /** AT: the day ("yyyy-MM-dd") it last fired. */
    val lastDay: String = "",
) {
    fun describe(): String = when (kind) {
        ONCE -> "once, " + Fmt.time(at)
        AT -> "at ${Fmt.clock(minute)} on " + if (days.size == 7) "every day" else Fmt.days().filter { it in days }.joinToString(", ") { Window.DAYS[it] }
        UNLOCK -> "when the phone is unlocked"
        else -> "at its next block window"
    }

    fun valid(now: Long): String? = when (kind) {
        ONCE -> if (at > now) null else "The time must be in the future"
        AT -> if (days.isEmpty() || days.any { it !in 0..6 }) "Choose at least one day"
            else if (minute !in 0 until 1440) "Invalid time" else null
        UNLOCK, WINDOW -> null
        else -> "No such autostart"
    }

    /** The rule without its run state, for comparing. */
    fun rule() = copy(lastDay = "")

    companion object {
        const val ONCE = "once"; const val AT = "at"; const val UNLOCK = "unlock"; const val WINDOW = "window"
    }
}

object Autostarts {
    data class Step(val model: Model, val inWindow: Map<String, Boolean>, val fired: List<Pair<String, Autostart>>)

    /**
     * One look: which autostarts are due, given [unlocked] (the phone was unlocked since the
     * last look) and [before] (whether each block was in a window then; null = the first look,
     * which only records, so a restart isn't taken for an opening window). Due blocks that are
     * off are switched on; once and window rules go, an AT rule records its day.
     */
    fun step(m: Model, now: Long, unlocked: Boolean, before: Map<String, Boolean>?, tz: TimeZone = TimeZone.getDefault()): Step {
        val t = WeekTime.of(now, tz)
        val today = SimpleDateFormat("yyyy-MM-dd", Locale.ROOT).apply { timeZone = tz }.format(now)
        val inWindow = m.blocks.filter { b -> b.autostarts.any { it.kind == Autostart.WINDOW } }
            .associate { it.name to Enforce.inWindow(m, m.automation(it), t) }
        if (before == null) return Step(m, inWindow, emptyList())
        val fired = mutableListOf<Pair<String, Autostart>>()
        val blocks = m.blocks.map { b ->
            var on = b.active
            val kept = b.autostarts.mapNotNull { a ->
                val due = when (a.kind) {
                    Autostart.ONCE -> now >= a.at
                    Autostart.AT -> t.day in a.days && t.minute >= a.minute && t.minute < a.minute + 10 && a.lastDay != today
                    Autostart.UNLOCK -> unlocked
                    Autostart.WINDOW -> inWindow[b.name] == true && before[b.name] == false
                    else -> false
                }
                if (!due) return@mapNotNull a
                if (!on) { on = true; fired += b.name to a }
                when (a.kind) { Autostart.ONCE, Autostart.WINDOW -> null; Autostart.AT -> a.copy(lastDay = today); else -> a }
            }
            b.copy(active = on, autostarts = kept)
        }
        return Step(m.copy(blocks = blocks), inWindow, fired)
    }
}
