package org.kc9k.android

import android.app.AppOpsManager
import android.app.usage.UsageEvents
import android.app.usage.UsageStatsManager
import android.content.Context
import android.content.pm.PackageManager
import android.os.Process

/** IronFox, the one supported browser. */
object IronFox {
    val PACKAGES = listOf("org.ironfoxoss.ironfox", "org.ironfoxoss.ironfox.nightly")

    fun installed(context: Context): List<String> = PACKAGES.filter {
        try { context.packageManager.getApplicationInfo(it, 0); true }
        catch (_: PackageManager.NameNotFoundException) { false }
    }
}

/** The last time the KC9K extension fetched /rules, as RulesServer saw it. */
data class CheckIn(val at: Long, val version: String, val privateTabs: Boolean)

object ExtensionStatus {
    @Volatile var last: CheckIn? = null
}

/**
 * The extension is mandatory, like on the desktop (internal/blocker/extguard.go): while
 * something needs it, IronFox may be in front for [GRACE_MS] without a good check-in (time
 * to install it); then it is paused (suspended). IronFox can't be told to keep an extension
 * on, so switching it off in the add-ons manager must cost more each time: once the
 * extension has checked in, the grace is [SHORT_GRACE_MS], and each pause is twice the
 * last ([PAUSE_MS] first, at most [MAX_PAUSE_MS]) until it has run [FORGIVE_MS] unbroken.
 * Without usage access KC9K can't tell when IronFox is in front, so IronFox stays paused.
 */
object ExtensionGuard {
    const val GRACE_MS = 60_000L
    /** Enough to switch the extension back on after a pause, not to browse. */
    const val SHORT_GRACE_MS = 20_000L
    const val PAUSE_MS = 2 * 60_000L
    const val MAX_PAUSE_MS = 60 * 60_000L
    const val FORGIVE_MS = 60 * 60_000L
    /** The extension polls every 3 s; a check-in older than this is stale. */
    const val FRESH_MS = 15_000L

    enum class Problem(val text: String) {
        NONE("The extension is running"),
        NEVER("The extension hasn't checked in"),
        STALE("The extension hasn't checked in lately"),
        OLD("The extension in IronFox is older than this app's: install it again"),
        NO_PRIVATE("The extension isn't allowed in private tabs"),
    }

    fun problem(c: CheckIn?, now: Long, minVersion: String): Problem = when {
        c == null -> Problem.NEVER
        now - c.at > FRESH_MS || c.at > now + FRESH_MS -> Problem.STALE
        compareVersions(c.version, minVersion) < 0 -> Problem.OLD
        !c.privateTabs -> Problem.NO_PRIVATE
        else -> Problem.NONE
    }

    /**
     * [unchecked]: time IronFox spent in front since the last good check-in. [strikes]:
     * pauses since the extension last ran [FORGIVE_MS] unbroken ([goodSince]). [seenGood]:
     * the extension has checked in before (kept by GuardService across restarts).
     */
    data class State(val unchecked: Long = 0, val pausedUntil: Long = 0, val strikes: Int = 0,
                     val seenGood: Boolean = false, val goodSince: Long = 0)

    data class Step(val state: State, val pause: Boolean)

    /** How long the pause after [strikes] earlier ones lasts. */
    fun pauseFor(strikes: Int): Long =
        if (strikes >= 5) MAX_PAUSE_MS else minOf(PAUSE_MS shl strikes, MAX_PAUSE_MS)

    fun step(s: State, now: Long, elapsed: Long, required: Boolean, usageAccess: Boolean,
             foreground: Boolean, good: Boolean): Step {
        if (!required) return Step(State(seenGood = s.seenGood), false)
        if (!usageAccess) return Step(s, true)
        if (s.pausedUntil > now) return Step(s, true)
        if (good) {
            val since = if (s.goodSince == 0L) now else s.goodSince
            val strikes = if (now - since >= FORGIVE_MS) 0 else s.strikes
            return Step(State(strikes = strikes, seenGood = true, goodSince = since), false)
        }
        val bad = s.copy(pausedUntil = 0, goodSince = 0)
        if (!foreground) return Step(bad, false)
        val unchecked = s.unchecked + elapsed
        if (unchecked >= if (s.seenGood) SHORT_GRACE_MS else GRACE_MS)
            return Step(bad.copy(unchecked = 0, pausedUntil = now + pauseFor(s.strikes), strikes = s.strikes + 1), true)
        return Step(bad.copy(unchecked = unchecked), false)
    }
}

/** Which app is in front, from usage events (needs usage access). */
class Foreground(private val context: Context) {
    private val usm = context.getSystemService(UsageStatsManager::class.java)
    private var since = 0L
    private var top: String? = null
    private var topClass: String? = null

    fun hasAccess(): Boolean {
        val ops = context.getSystemService(AppOpsManager::class.java)
        return ops.unsafeCheckOpNoThrow(AppOpsManager.OPSTR_GET_USAGE_STATS, Process.myUid(),
            context.packageName) == AppOpsManager.MODE_ALLOWED
    }

    /** The package in front now, or null (screen off, or not known). */
    fun top(now: Long): String? {
        val from = maxOf(since, now - 6 * 3_600_000L)
        val events = try { usm.queryEvents(from, now) } catch (_: Exception) { return top }
        val e = UsageEvents.Event()
        while (events.hasNextEvent()) {
            events.getNextEvent(e)
            when (e.eventType) {
                UsageEvents.Event.ACTIVITY_RESUMED -> { top = e.packageName; topClass = e.className }
                UsageEvents.Event.ACTIVITY_PAUSED ->
                    if (e.packageName == top && e.className == topClass) { top = null; topClass = null }
                UsageEvents.Event.SCREEN_NON_INTERACTIVE, UsageEvents.Event.KEYGUARD_SHOWN -> { top = null; topClass = null }
            }
            since = maxOf(since, e.timeStamp + 1)
        }
        if (since == 0L) since = now
        return top
    }
}
