package org.kc9k.android

import java.util.TimeZone

/**
 * The command layer (R-ANDROID-3): every change to blocks, places and schedules goes
 * through here, and here alone weakening is decided. Strengthening is always allowed;
 * weakening a locked block is refused and logged (R-WEAK-1–3). Pure: the caller saves
 * [Outcome.model] (refusals change the model too: the log).
 */
class Commands(
    private val now: Long,
    private val boot: Int,
    private val tz: TimeZone = TimeZone.getDefault(),
) {
    data class Outcome(val model: Model, val error: String? = null, val note: String? = null)

    private fun ok(m: Model, note: String? = null) = Outcome(m, null, note)

    private fun refuse(m: Model, why: String, what: String): Outcome {
        val line = "${java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.ROOT).apply { timeZone = tz }.format(now)} refused: $what ($why)"
        return Outcome(m.copy(refused = (m.refused + line).takeLast(Model.LOG_MAX)), why)
    }

    fun locked(m: Model, b: Block) = Enforce.locked(m, b, now, boot, tz)

    /** Why [b] can't be weakened now, or null. */
    private fun whyLocked(m: Model, b: Block): String? {
        if (b.lock.inForce(now, boot)) return "Locked: " + b.lock.current(now, boot).describe(now, boot)
        if (b.scheduleLock && b.active && Enforce.enabled(m, b, now, tz)) return "Locked while it is on (schedule lock)"
        return null
    }

    /** Settings outside blocks (protections, browser settings): refused while anything is locked, if no weakening is on. */
    fun whySettingsLocked(m: Model): String? =
        if (m.noWeakening && Enforce.anyLocked(m, now, boot, tz)) "No weakening while a block is locked" else null

    private fun edit(m: Model, name: String, weakens: Boolean, what: String, change: (Block) -> Block): Outcome {
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        if (weakens) whyLocked(m, b)?.let { return refuse(m, it, "$what in $name") }
        val nb = change(b)
        return ok(m.copy(blocks = m.blocks.map { if (it.name == name) nb else it }))
    }

    // Blocks.
    fun addBlock(m: Model, name: String): Outcome {
        val n = name.trim()
        if (n.isEmpty()) return Outcome(m, "Give the block a name")
        if (m.block(n) != null) return Outcome(m, "A block named $n exists")
        return ok(m.copy(blocks = m.blocks + Block(n, active = false)))
    }

    fun renameBlock(m: Model, name: String, to: String): Outcome {
        val n = to.trim()
        if (n.isEmpty()) return Outcome(m, "Give the block a name")
        if (n != name && m.block(n) != null) return Outcome(m, "A block named $n exists")
        return edit(m, name, false, "rename") { it.copy(name = n) }
    }

    fun deleteBlock(m: Model, name: String): Outcome {
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        whyLocked(m, b)?.let { return refuse(m, it, "delete $name") }
        return ok(m.copy(blocks = m.blocks - b))
    }

    fun setActive(m: Model, name: String, on: Boolean): Outcome =
        edit(m, name, !on, "switch off") { it.copy(active = on, pausedUntil = if (on) it.pausedUntil else 0) }

    fun pause(m: Model, name: String, until: Long): Outcome {
        if (until <= now) return Outcome(m, "The pause must end in the future")
        return edit(m, name, true, "pause") { it.copy(pausedUntil = until) }
    }

    /** Ending a pause early strengthens. */
    fun endPause(m: Model, name: String) = edit(m, name, false, "end pause") { it.copy(pausedUntil = 0) }

    // Websites.
    fun addSites(m: Model, name: String, text: String): Outcome {
        val e = entries(text) ?: return Outcome(m, badEntries(text))
        return edit(m, name, false, "add sites") { it.copy(sites = (it.sites + e).distinct()) }
    }

    fun removeSite(m: Model, name: String, entry: String) =
        edit(m, name, true, "remove $entry") { it.copy(sites = it.sites - entry) }

    fun addSiteExceptions(m: Model, name: String, text: String): Outcome {
        val e = entries(text) ?: return Outcome(m, badEntries(text))
        return edit(m, name, true, "add exceptions") { it.copy(siteExceptions = (it.siteExceptions + e).distinct()) }
    }

    fun removeSiteException(m: Model, name: String, entry: String) =
        edit(m, name, false, "remove exception") { it.copy(siteExceptions = it.siteExceptions - entry) }

    /** A list file imported into the sites or exceptions (ListFile), added to what is there. */
    fun importSites(m: Model, name: String, exceptions: Boolean, text: String): Outcome {
        val r = ListFile.read(text)
        if (r.entries.isEmpty()) return Outcome(m, if (r.skipped.isEmpty()) "The file has no entries"
            else "No addresses in the file (e.g. ${r.skipped.first()})")
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        val cur = if (exceptions) b.siteExceptions else b.sites
        val new = r.entries.filter { it !in cur }
        val list = if (exceptions) WebBlock.EXCEPTIONS else WebBlock.BLOCKED
        val o = edit(m, name, exceptions, "import exceptions") {
            if (exceptions) it.copy(siteExceptions = cur + new) else it.copy(sites = cur + new)
        }
        return if (o.error != null) o else ok(o.model, ListFile.summary(list, new.size, r.entries.size - new.size, r.skipped))
    }

    private fun entries(text: String): List<String>? {
        val raw = Entries.split(text)
        if (raw.isEmpty() || raw.any { Entries.normalize(it) == null }) return null
        return raw.mapNotNull(Entries::normalize)
    }

    private fun badEntries(text: String): String {
        val bad = Entries.split(text).filter { Entries.normalize(it) == null }
        return if (bad.isEmpty()) "No entries given" else "Not an address or pattern: " + bad.joinToString(" ")
    }

    // Apps.
    fun addApps(m: Model, name: String, apps: List<String>): Outcome {
        val a = apps.map { it.trim() }.filter { it.isNotEmpty() }
        if (a.isEmpty()) return Outcome(m, "No apps given")
        a.firstOrNull { !AppEntries.valid(it) }?.let { return Outcome(m, "Not an app: $it") }
        return edit(m, name, false, "add apps") { it.copy(apps = (it.apps + a).distinct()) }
    }

    fun removeApp(m: Model, name: String, app: String) =
        edit(m, name, true, "remove app $app") { it.copy(apps = it.apps - app) }

    fun addAppExceptions(m: Model, name: String, apps: List<String>): Outcome {
        val a = apps.map { it.trim() }.filter { it.isNotEmpty() }
        if (a.isEmpty()) return Outcome(m, "No apps given")
        a.firstOrNull { !AppEntries.valid(it) || it == "*" }?.let { return Outcome(m, "Not an app: $it") }
        return edit(m, name, true, "add app exceptions") { it.copy(appExceptions = (it.appExceptions + a).distinct()) }
    }

    fun removeAppException(m: Model, name: String, app: String) =
        edit(m, name, false, "remove app exception") { it.copy(appExceptions = it.appExceptions - app) }

    fun setWifiOff(m: Model, name: String, on: Boolean) =
        edit(m, name, !on, "allow Wi-Fi") { it.copy(wifiOff = on) }

    // Automation.
    fun setAutomation(m: Model, name: String, a: Automation): Outcome {
        a.windows.firstOrNull { !it.valid() }?.let { return Outcome(m, "Invalid window ${it.describe()} (5-minute steps)") }
        for (i in a.windows.indices) for (j in i + 1 until a.windows.size)
            if (a.windows[i].overlaps(a.windows[j])) return Outcome(m, "Windows overlap: ${a.windows[i].describe()} and ${a.windows[j].describe()}")
        a.schedules.firstOrNull { m.schedule(it) == null }?.let { return Outcome(m, "No schedule $it") }
        a.places.firstOrNull { m.place(it) == null }?.let { return Outcome(m, "No place $it") }
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        if (b.automation == a) return ok(m)
        return edit(m, name, !Weakening.strengthens(b.automation, a), "change automation") { it.copy(automation = a) }
    }

    // Locks (each confirmed by the GUI before this runs, R-LOCK-12).
    fun lock(m: Model, name: String, want: SettingLock, scheduleLock: Boolean): Outcome {
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        if (want == SettingLock() && !scheduleLock) return Outcome(m, "Choose at least one lock")
        var lock = b.lock
        if (want != SettingLock()) {
            val r = SettingLock.strengthen(b.lock, want, now, boot)
            if (r.error != null && !(scheduleLock && !b.scheduleLock)) return Outcome(m, r.error)
            if (r.error == null) lock = r.lock
        } else if (b.scheduleLock) return Outcome(m, "It already has a schedule lock")
        return edit(m, name, false, "lock") { it.copy(lock = lock, scheduleLock = it.scheduleLock || scheduleLock) }
    }

    /** Why [name]'s own lock can't be opened by challenge/password now, or null. */
    fun refuseUnlock(m: Model, name: String): String? {
        val b = m.block(name) ?: return "No block $name"
        return b.lock.refuseUnlock(now, boot)
    }

    /** After the password and challenge were passed: the lock goes. */
    fun unlocked(m: Model, name: String): Outcome {
        refuseUnlock(m, name)?.let { return Outcome(m, it) }
        return edit(m, name, false, "unlock") { it.copy(lock = SettingLock()) }
    }

    /** The schedule lock goes only while it isn't holding. */
    fun removeScheduleLock(m: Model, name: String) =
        edit(m, name, true, "remove schedule lock") { it.copy(scheduleLock = false) }

    // Places.
    fun setPlace(m: Model, name: String, lat: Double, lon: Double, radius: Float): Outcome {
        val n = name.trim()
        if (n.isEmpty()) return Outcome(m, "Give the place a name")
        if (lat !in -90.0..90.0 || lon !in -180.0..180.0) return Outcome(m, "Invalid position")
        if (radius !in 50f..2000f) return Outcome(m, "The radius must be 50 to 2000 m")
        val old = m.place(n)
        if (old != null) usedByLocked(m) { n in it.automation.places }?.let { return refuse(m, it, "move place $n") }
        val p = Place(n, lat, lon, radius)
        return ok(m.copy(places = if (old == null) m.places + p else m.places.map { if (it.name == n) p else it }))
    }

    fun deletePlace(m: Model, name: String): Outcome {
        val p = m.place(name) ?: return Outcome(m, "No place $name")
        usedByLocked(m) { name in it.automation.places }?.let { return refuse(m, it, "delete place $name") }
        return ok(m.copy(places = m.places - p,
            blocks = m.blocks.map { it.copy(automation = it.automation.copy(places = it.automation.places - name)) }))
    }

    // Schedules.
    fun addSchedule(m: Model, name: String): Outcome {
        val n = name.trim()
        if (n.isEmpty()) return Outcome(m, "Give the schedule a name")
        if (m.schedule(n) != null) return Outcome(m, "A schedule named $n exists")
        return ok(m.copy(schedules = m.schedules + Schedule(n)))
    }

    fun setScheduleWindows(m: Model, name: String, windows: List<Window>): Outcome {
        val s = m.schedule(name) ?: return Outcome(m, "No schedule $name")
        windows.firstOrNull { !it.valid() }?.let { return Outcome(m, "Invalid window ${it.describe()} (5-minute steps)") }
        // Each locked block using it must take the change as a strengthening.
        for (b in m.blocks.filter { name in it.automation.schedules }) {
            val why = whyLocked(m, b) ?: continue
            val before = Weakening.flatten(m, b.automation)
            val after = Weakening.flatten(m.copy(schedules = m.schedules.map { if (it.name == name) Schedule(name, windows) else it }), b.automation)
            if (!Weakening.strengthens(before, after)) return refuse(m, why, "change schedule $name (used by ${b.name})")
        }
        return ok(m.copy(schedules = m.schedules.map { if (it.name == name) s.copy(windows = windows) else it }))
    }

    fun deleteSchedule(m: Model, name: String): Outcome {
        val s = m.schedule(name) ?: return Outcome(m, "No schedule $name")
        usedByLocked(m) { name in it.automation.schedules }?.let { return refuse(m, it, "delete schedule $name") }
        return ok(m.copy(schedules = m.schedules - s,
            blocks = m.blocks.map { it.copy(automation = it.automation.copy(schedules = it.automation.schedules - name)) }))
    }

    fun setNoWeakening(m: Model, on: Boolean): Outcome {
        if (!on && m.noWeakening) whySettingsLocked(m)?.let { return refuse(m, it, "turn off no weakening") }
        return ok(m.copy(noWeakening = on))
    }

    private fun usedByLocked(m: Model, uses: (Block) -> Boolean): String? =
        m.blocks.filter(uses).firstNotNullOfOrNull { b -> whyLocked(m, b)?.let { "${b.name}: $it" } }
}

/** Which automation changes strengthen (SPEC §3.5), so they are allowed while locked. */
object Weakening {
    /**
     * The automation with its schedules' windows written in (for comparing). The schedule
     * names stay, so a block whose schedule was emptied still isn't continuous.
     */
    fun flatten(m: Model, a: Automation) =
        a.copy(windows = a.windows + a.schedules.flatMap { m.schedule(it)?.windows ?: emptyList() })

    private fun minutes(ws: List<Window>): Set<Int> =
        ws.flatMap { w -> (w.start until w.end).map { w.day * 1440 + it } }.toSet()

    fun strengthens(old: Automation, new: Automation): Boolean {
        if (old == new) return true
        // Continuous is always on: anything else would let it lift.
        if (new.continuous) return true
        if (old.continuous) return false
        if (old.defaultEnabled != new.defaultEnabled) return false
        val ow = minutes(old.windows); val nw = minutes(new.windows)
        return if (!old.defaultEnabled) {
            // Windows and places switch it on: more of them, and "either" over "both".
            nw.containsAll(ow) && old.schedules.all { it in new.schedules } && new.places.containsAll(old.places) &&
                (old.both || !new.both)
        } else {
            // Windows and places lift it: fewer of them, and "both" over "either".
            ow.containsAll(nw) && new.schedules.all { it in old.schedules } && old.places.containsAll(new.places) &&
                (!old.both || new.both)
        }
    }
}

/** App entries: `*`, roles, package names (SPEC §8.2 App). */
object AppEntries {
    const val ALL = "*"
    val ROLES = listOf("role:dialer", "role:sms")
    private val pkg = Regex("^[A-Za-z][A-Za-z0-9_]*(\\.[A-Za-z][A-Za-z0-9_]*)+$")
    fun valid(e: String) = e == ALL || e == "*.*" || e in ROLES || pkg.matches(e)

    fun describe(e: String, label: (String) -> String?) = when (e) {
        ALL, "*.*" -> "All apps"
        "role:dialer" -> "Default phone app"
        "role:sms" -> "Default SMS app"
        else -> label(e)?.let { "$it ($e)" } ?: e
    }
}
