package org.kc9k.android

import java.util.Calendar
import java.util.TimeZone

/** A weekly window: [day] 0 = Monday … 6 = Sunday, [start] and [end] minutes of that day (end ≤ 1440). */
data class Window(val day: Int, val start: Int, val end: Int) {
    fun covers(d: Int, minute: Int) = d == day && minute >= start && minute < end
    fun valid() = day in 0..6 && start in 0 until 1440 && end in 1..1440 && end > start &&
        start % 5 == 0 && end % 5 == 0

    fun overlaps(o: Window) = day == o.day && start < o.end && o.start < end

    fun describe() = "${DAYS[day]} ${hm(start)}–${hm(end)}"

    companion object {
        val DAYS = listOf("Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun")
        fun hm(m: Int) = "%02d:%02d".format(m / 60, m % 60)
        /** "HH:MM" (24:00 allowed as an end) to minutes, or null. */
        fun parse(s: String): Int? {
            val m = Regex("^(\\d{1,2}):(\\d{2})$").find(s.trim()) ?: return null
            val h = m.groupValues[1].toInt(); val mm = m.groupValues[2].toInt()
            if (mm > 59 || h > 24 || (h == 24 && mm != 0)) return null
            return h * 60 + mm
        }
    }
}

/**
 * What switches an active block by itself (SPEC §3.6): its own windows, named schedules and
 * places. Disabled by default: a window or place turns it on; enabled by default: turns it
 * off. With windows and places both, [both] = needs both at once, else either.
 */
data class Automation(
    val defaultEnabled: Boolean = false,
    val windows: List<Window> = emptyList(),
    val schedules: List<String> = emptyList(),
    val places: List<String> = emptyList(),
    val both: Boolean = false,
) {
    val continuous get() = windows.isEmpty() && schedules.isEmpty() && places.isEmpty()
}

/**
 * A block. [apps]: `*` = every launchable app (but the essentials), `role:dialer`,
 * `role:sms`, or package names; [appExceptions] spare apps from this block only.
 * [scheduleLock]: locked while its automation enables it (SPEC §3.4 Schedule lock).
 */
data class Block(
    val name: String,
    val active: Boolean = true,
    val sites: List<String> = emptyList(),
    val siteExceptions: List<String> = emptyList(),
    val apps: List<String> = emptyList(),
    val appExceptions: List<String> = emptyList(),
    val wifiOff: Boolean = false,
    val automation: Automation = Automation(),
    val lock: SettingLock = SettingLock(),
    val scheduleLock: Boolean = false,
    val pausedUntil: Long = 0,
    /** The mode it is in, or ""; in a mode its own [automation] is kept, unused (R-MODE-1). */
    val mode: String = "",
)

/** A mode (SPEC §3.2): a named group of blocks with one automation for all. */
data class Mode(val name: String, val automation: Automation = Automation())

/** An area (SPEC §8.2) with its last decided state (Area.step). */
data class Place(
    val name: String, val lat: Double, val lon: Double, val radius: Float,
    val inside: Boolean = false, val outsideSince: Long = 0,
    val lastDistance: Float = -1f, val lastAccuracy: Float = -1f, val lastFix: Long = 0,
)

data class Schedule(val name: String, val windows: List<Window> = emptyList())

data class Model(
    val blocks: List<Block> = emptyList(),
    val places: List<Place> = emptyList(),
    val schedules: List<Schedule> = emptyList(),
    val modes: List<Mode> = emptyList(),
    /** "No weakening during active locks" (SPEC §3.5), on by default. */
    val noWeakening: Boolean = true,
    /** Refused weakenings, newest last (R-WEAK-3), at most [LOG_MAX]. */
    val refused: List<String> = emptyList(),
) {
    fun block(name: String) = blocks.find { it.name == name }
    fun place(name: String) = places.find { it.name == name }
    fun schedule(name: String) = schedules.find { it.name == name }
    fun mode(name: String) = modes.find { it.name == name }
    fun blocksIn(mode: String) = blocks.filter { it.mode == mode }

    /** The automation that switches [b]: its mode's while in one, else its own (R-MODE-1). */
    fun automation(b: Block): Automation =
        if (b.mode.isEmpty()) b.automation else mode(b.mode)?.automation ?: b.automation

    companion object { const val LOG_MAX = 100 }
}

/** Week clock: day 0 = Monday and minute of the day, for [now] in [tz]. */
data class WeekTime(val day: Int, val minute: Int) {
    companion object {
        fun of(now: Long, tz: TimeZone = TimeZone.getDefault()): WeekTime {
            val c = Calendar.getInstance(tz).apply { timeInMillis = now }
            val d = (c.get(Calendar.DAY_OF_WEEK) + 5) % 7 // Calendar: Sunday = 1
            return WeekTime(d, c.get(Calendar.HOUR_OF_DAY) * 60 + c.get(Calendar.MINUTE))
        }
    }
}

/** What is enforced now: the single source of truth for the service and the GUI (as `enforce`). */
object Enforce {
    /** Why a block is or isn't enforcing now, for the GUI. */
    enum class State(val word: String) {
        INACTIVE("off"), ENABLED("on"), PAUSED("paused"), LIFTED("lifted")
    }

    fun inWindow(m: Model, a: Automation, t: WeekTime) =
        (a.windows + a.schedules.flatMap { m.schedule(it)?.windows ?: emptyList() }).any { it.covers(t.day, t.minute) }

    fun atPlace(m: Model, a: Automation) = a.places.any { m.place(it)?.inside == true }

    /** Whether the automation's windows/places trigger now (switching from the default). */
    fun triggered(m: Model, a: Automation, t: WeekTime): Boolean {
        val hasTime = a.windows.isNotEmpty() || a.schedules.isNotEmpty()
        val hasPlace = a.places.isNotEmpty()
        val w = hasTime && inWindow(m, a, t)
        val p = hasPlace && atPlace(m, a)
        return when {
            hasTime && hasPlace -> if (a.both) w && p else w || p
            else -> w || p
        }
    }

    fun state(m: Model, b: Block, now: Long, tz: TimeZone = TimeZone.getDefault()): State {
        val a = m.automation(b)
        return when {
            !b.active -> State.INACTIVE
            b.pausedUntil > now -> State.PAUSED
            a.continuous -> State.ENABLED
            triggered(m, a, WeekTime.of(now, tz)) != a.defaultEnabled -> State.ENABLED
            else -> State.LIFTED
        }
    }

    fun enabled(m: Model, b: Block, now: Long, tz: TimeZone = TimeZone.getDefault()) =
        state(m, b, now, tz) == State.ENABLED

    /** Locked (SPEC §3.3): its own lock in force, or its schedule lock while it is enabled. */
    fun locked(m: Model, b: Block, now: Long, boot: Int, tz: TimeZone = TimeZone.getDefault()) =
        b.lock.inForce(now, boot) || (b.scheduleLock && b.active && enabled(m, b, now, tz))

    fun anyLocked(m: Model, now: Long, boot: Int, tz: TimeZone = TimeZone.getDefault()) =
        m.blocks.any { locked(m, it, now, boot, tz) }

    /** The union of the enabled blocks' layer inputs. */
    data class Plan(
        /** Per enabled block: its apps and its own app exceptions. */
        val apps: List<Pair<List<String>, List<String>>>,
        val wifiOff: Boolean,
        /** Per enabled block with sites: name, sites, its own exceptions. */
        val web: List<Triple<String, List<String>, List<String>>>,
    ) {
        val empty get() = apps.isEmpty() && !wifiOff && web.isEmpty()
    }

    fun plan(m: Model, now: Long, tz: TimeZone = TimeZone.getDefault()): Plan {
        val on = m.blocks.filter { enabled(m, it, now, tz) }
        return Plan(
            apps = on.filter { it.apps.isNotEmpty() }.map { it.apps to it.appExceptions },
            wifiOff = on.any { it.wifiOff },
            web = on.filter { it.sites.isNotEmpty() }.map { Triple(it.name, it.sites, it.siteExceptions) },
        )
    }

    /**
     * The packages to suspend: each enabled block's apps minus its own exceptions, resolved by
     * [resolve] (`*`, roles), minus [never] (essentials, launcher, KC9K).
     */
    fun suspend(p: Plan, resolve: (List<String>) -> Set<String>, never: Set<String>): Set<String> =
        p.apps.flatMap { (apps, except) -> resolve(apps) - resolve(except) }.toSet() - never
}
