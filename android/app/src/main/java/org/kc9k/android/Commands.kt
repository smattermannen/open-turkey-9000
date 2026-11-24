package org.kc9k.android

import java.util.TimeZone

/**
 * The command layer (R-ANDROID-3): every change to blocks, places and schedules goes
 * through here, and here alone weakening is decided. Strengthening is always allowed;
 * weakening a locked block is refused and logged (R-WEAK-1–3). With a settings password
 * set (SPEC §3.11), every weakening also needs it: [settingsUnlocked] says it was typed
 * (Session); without it the command stops with [Outcome.needsPassword], unlogged. Pure:
 * the caller saves [Outcome.model] (refusals change the model too: the log).
 */
class Commands(
    private val now: Long,
    private val boot: Int,
    private val tz: TimeZone = TimeZone.getDefault(),
    private val settingsUnlocked: Boolean = Session.settingsUnlocked,
) {
    data class Outcome(val model: Model, val error: String? = null, val note: String? = null,
                       val needsPassword: Boolean = false)

    companion object { const val NEEDS_PASSWORD = "Needs the settings password" }

    private fun ok(m: Model, note: String? = null) = Outcome(m, null, note)

    /** Why weakening needs the settings password now, or null. */
    private fun passwordWhy(m: Model) = if (m.settingsPassword.isNotEmpty() && !settingsUnlocked) NEEDS_PASSWORD else null

    private fun refuse(m: Model, why: String, what: String): Outcome {
        if (why.contains(NEEDS_PASSWORD)) return Outcome(m, NEEDS_PASSWORD, needsPassword = true)
        val line = "${java.text.SimpleDateFormat("yyyy-MM-dd HH:mm", java.util.Locale.ROOT).apply { timeZone = tz }.format(now)} refused: $what ($why)"
        return Outcome(m.copy(refused = (m.refused + line).takeLast(Model.LOG_MAX)), why)
    }

    fun locked(m: Model, b: Block) = Enforce.locked(m, b, now, boot, tz)

    /** Why [b] can't be weakened now (its locks, else the settings password), or null. */
    private fun whyLocked(m: Model, b: Block): String? {
        if (b.lock.inForce(now, boot)) return "Locked: " + b.lock.current(now, boot).describe(now, boot)
        if (b.scheduleLock && b.active && Enforce.enabled(m, b, now, tz)) return "Locked while it is on (schedule lock)"
        return passwordWhy(m)
    }

    /**
     * Settings outside blocks (protections, browser settings): weakening refused while
     * anything is locked, if no weakening is on; else it needs the settings password if set.
     */
    fun whySettingsLocked(m: Model): String? =
        if (m.noWeakening && Enforce.anythingLocked(m, now, boot, tz)) "No weakening while anything is locked" else passwordWhy(m)

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

    // Autostart (SPEC §3.6): adding switches it on more often (free); removing weakens.
    fun addAutostart(m: Model, block: String, a: Autostart): Outcome {
        a.valid(now)?.let { return Outcome(m, it) }
        val b = m.block(block) ?: return Outcome(m, "No block $block")
        if (b.autostarts.any { it.rule() == a.rule() }) return Outcome(m, "It has that autostart")
        return edit(m, block, false, "add autostart") { it.copy(autostarts = it.autostarts + a.copy(lastDay = "")) }
    }

    fun removeAutostart(m: Model, block: String, a: Autostart) =
        edit(m, block, true, "remove autostart ${a.kind}") { b -> b.copy(autostarts = b.autostarts.filter { it.rule() != a.rule() }) }

    // Phrase sets (SPEC §3.2): adding phrases strengthens; removing, narrowing, a higher threshold or test mode weakens.
    private fun editSet(m: Model, block: String, set: String, weakens: Boolean, what: String, change: (PhraseSet) -> PhraseSet): Outcome {
        val b = m.block(block) ?: return Outcome(m, "No block $block")
        b.phraseSets.find { it.name == set } ?: return Outcome(m, "No phrase set $set in $block")
        return edit(m, block, weakens, "$what in phrase set $set") { bb ->
            bb.copy(phraseSets = bb.phraseSets.map { if (it.name == set) change(it) else it }) }
    }

    fun addPhraseSet(m: Model, block: String, name: String): Outcome {
        val n = name.trim()
        if (n.isEmpty()) return Outcome(m, "Give the phrase set a name")
        val b = m.block(block) ?: return Outcome(m, "No block $block")
        if (b.phraseSets.any { it.name == n }) return Outcome(m, "$block has a phrase set named $n")
        return edit(m, block, false, "add phrase set") { it.copy(phraseSets = it.phraseSets + PhraseSet(n)) }
    }

    fun renamePhraseSet(m: Model, block: String, name: String, to: String): Outcome {
        val n = to.trim()
        if (n.isEmpty()) return Outcome(m, "Give the phrase set a name")
        if (n != name && m.block(block)?.phraseSets?.any { it.name == n } == true) return Outcome(m, "$block has a phrase set named $n")
        return editSet(m, block, name, false, "rename") { it.copy(name = n) }
    }

    fun deletePhraseSet(m: Model, block: String, name: String): Outcome {
        val b = m.block(block) ?: return Outcome(m, "No block $block")
        b.phraseSets.find { it.name == name } ?: return Outcome(m, "No phrase set $name in $block")
        return edit(m, block, true, "delete phrase set $name") { bb -> bb.copy(phraseSets = bb.phraseSets.filter { it.name != name }) }
    }

    /** One phrase per line, each checked (R-PHRASE-1, R-PHRASE-8); phrases it has already are skipped. */
    fun addPhrases(m: Model, block: String, set: String, text: String): Outcome {
        val lines = text.lines().map { it.trim() }.filter { it.isNotEmpty() }
        if (lines.isEmpty()) return Outcome(m, "No phrases given")
        val add = mutableListOf<Phrase>()
        for (l in lines) {
            val (p, err) = Phrase.parse(l)
            if (p == null) return Outcome(m, "$l: $err")
            add += p.canonical()
        }
        return editSet(m, block, set, false, "add phrases") { s ->
            s.copy(phrases = s.phrases + add.distinctBy { it.key() }.filter { a -> s.phrases.none { it.key() == a.key() } }) }
    }

    fun removePhrase(m: Model, block: String, set: String, phrase: Phrase) =
        editSet(m, block, set, true, "remove $phrase") { s -> s.copy(phrases = s.phrases.filter { it.key() != phrase.key() }) }

    /** A change that only widens the phrase (more terms) is a strengthening (R-WEAK-5). */
    fun replacePhrase(m: Model, block: String, set: String, old: Phrase, text: String): Outcome {
        val (p, err) = Phrase.parse(text)
        if (p == null) return Outcome(m, err)
        val n = p.canonical()
        return editSet(m, block, set, !n.widens(old), "change $old") { s ->
            if (s.phrases.any { it.key() == n.key() }) s.copy(phrases = s.phrases.filter { it.key() != old.key() })
            else s.copy(phrases = s.phrases.map { if (it.key() == old.key()) n else it })
        }
    }

    fun setPhraseScope(m: Model, block: String, set: String, url: Boolean, title: Boolean, text: Boolean): Outcome {
        val s = m.block(block)?.phraseSets?.find { it.name == set } ?: return Outcome(m, "No phrase set $set in $block")
        val weakens = (s.inUrl && !url) || (s.inTitle && !title) || (s.inText && !text)
        return editSet(m, block, set, weakens, "narrow where it looks") { it.copy(inUrl = url, inTitle = title, inText = text) }
    }

    /** How many different phrases must match before it blocks (R-PHRASE-9): lower strengthens. */
    fun setPhraseThreshold(m: Model, block: String, set: String, n: Int): Outcome {
        if (n < 1) return Outcome(m, "At least 1 phrase")
        val s = m.block(block)?.phraseSets?.find { it.name == set } ?: return Outcome(m, "No phrase set $set in $block")
        return editSet(m, block, set, n > s.threshold, "raise the threshold") { it.copy(threshold = n) }
    }

    /** Test mode only reports matches: turning it on weakens. */
    fun setPhraseTest(m: Model, block: String, set: String, on: Boolean) =
        editSet(m, block, set, on, "test mode") { it.copy(test = on) }

    // Social media settings: on strengthens, off weakens.
    fun setSocial(m: Model, block: String, preset: String, on: Boolean): Outcome {
        if (!Social.valid(preset)) return Outcome(m, "No social media setting $preset")
        return edit(m, block, !on, "turn off $preset") {
            it.copy(social = if (on) (it.social + preset).distinct() else it.social - preset) }
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
    private fun badAutomation(m: Model, a: Automation): String? {
        a.windows.firstOrNull { !it.valid() }?.let { return "Invalid window ${it.describe()} (5-minute steps)" }
        for (i in a.windows.indices) for (j in i + 1 until a.windows.size)
            if (a.windows[i].overlaps(a.windows[j])) return "Windows overlap: ${a.windows[i].describe()} and ${a.windows[j].describe()}"
        a.schedules.firstOrNull { m.schedule(it) == null }?.let { return "No schedule $it" }
        a.places.firstOrNull { m.place(it) == null }?.let { return "No place $it" }
        return null
    }

    fun setAutomation(m: Model, name: String, a: Automation): Outcome {
        badAutomation(m, a)?.let { return Outcome(m, it) }
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        if (b.mode.isNotEmpty()) return Outcome(m, "$name is in the mode ${b.mode}: change the mode's automation, or take it out")
        if (b.automation == a) return ok(m)
        return edit(m, name, !Weakening.strengthens(b.automation, a), "change automation") { it.copy(automation = a) }
    }

    // Modes (SPEC §3.2, R-MODE-1–3): a group of blocks with one automation for all.
    fun addMode(m: Model, name: String): Outcome {
        val n = name.trim()
        if (n.isEmpty()) return Outcome(m, "Give the mode a name")
        if (m.mode(n) != null) return Outcome(m, "A mode named $n exists")
        return ok(m.copy(modes = m.modes + Mode(n)))
    }

    fun renameMode(m: Model, name: String, to: String): Outcome {
        val n = to.trim()
        m.mode(name) ?: return Outcome(m, "No mode $name")
        if (n.isEmpty()) return Outcome(m, "Give the mode a name")
        if (n != name && m.mode(n) != null) return Outcome(m, "A mode named $n exists")
        return ok(m.copy(modes = m.modes.map { if (it.name == name) it.copy(name = n) else it },
            blocks = m.blocks.map { if (it.mode == name) it.copy(mode = n) else it }))
    }

    /** Locked blocks for which [after] (block name to its new automation) weakens, named; null if none. */
    private fun weakenedLocked(m: Model, after: Map<String, Automation>): String? {
        val hit = after.mapNotNull { (n, a) ->
            val b = m.block(n) ?: return@mapNotNull null
            val why = whyLocked(m, b) ?: return@mapNotNull null
            if (Weakening.strengthens(Weakening.flatten(m, m.automation(b)), Weakening.flatten(m, a))) null else "$n: $why"
        }
        return if (hit.isEmpty()) null else hit.joinToString("; ")
    }

    /** One change for all the mode's blocks, refused as a whole if it weakens any locked one (R-MODE-2). */
    fun setModeAutomation(m: Model, name: String, a: Automation): Outcome {
        badAutomation(m, a)?.let { return Outcome(m, it) }
        val mode = m.mode(name) ?: return Outcome(m, "No mode $name")
        if (mode.automation == a) return ok(m)
        weakenedLocked(m, m.blocksIn(name).associate { it.name to a })?.let { return refuse(m, it, "change automation of mode $name") }
        return ok(m.copy(modes = m.modes.map { if (it.name == name) it.copy(automation = a) else it }))
    }

    /** A block gets the mode's automation (from another mode too); refused if that weakens it while locked (R-MODE-1). */
    fun putInMode(m: Model, block: String, mode: String): Outcome {
        val b = m.block(block) ?: return Outcome(m, "No block $block")
        val md = m.mode(mode) ?: return Outcome(m, "No mode $mode")
        if (b.mode == mode) return ok(m)
        weakenedLocked(m, mapOf(block to md.automation))?.let { return refuse(m, it, "put $block into mode $mode") }
        return ok(m.copy(blocks = m.blocks.map { if (it.name == block) it.copy(mode = mode) else it }))
    }

    /** The block's own automation comes back; refused if that weakens it while locked (R-MODE-1). */
    fun takeOut(m: Model, block: String): Outcome {
        val b = m.block(block) ?: return Outcome(m, "No block $block")
        if (b.mode.isEmpty()) return ok(m)
        weakenedLocked(m, mapOf(block to b.automation))?.let { return refuse(m, it, "take $block out of mode ${b.mode}") }
        return ok(m.copy(blocks = m.blocks.map { if (it.name == block) it.copy(mode = "") else it }))
    }

    /** Its blocks are taken out, each getting its own automation back; refused as a whole if that weakens a locked one. */
    fun deleteMode(m: Model, name: String): Outcome {
        val mode = m.mode(name) ?: return Outcome(m, "No mode $name")
        weakenedLocked(m, m.blocksIn(name).associate { it.name to it.automation })?.let { return refuse(m, it, "delete mode $name") }
        return ok(m.copy(modes = m.modes - mode, blocks = m.blocks.map { if (it.mode == name) it.copy(mode = "") else it }))
    }

    /** Switches the mode's blocks on or off; switching off leaves locked ones on (R-MODE-3), refused and logged. */
    fun switchMode(m: Model, name: String, on: Boolean): Outcome {
        m.mode(name) ?: return Outcome(m, "No mode $name")
        val blocks = m.blocksIn(name)
        if (blocks.isEmpty()) return Outcome(m, "The mode $name has no blocks")
        var cur = m
        val stayed = mutableListOf<String>()
        for (b in blocks) {
            val o = setActive(cur, b.name, on)
            if (o.needsPassword) return Outcome(m, o.error, needsPassword = true)
            if (o.error != null) stayed += b.name
            cur = o.model
        }
        return if (stayed.isEmpty()) ok(cur)
        else Outcome(cur, "Locked, so still on: " + stayed.joinToString(", "))
    }

    /** Locks every block of the mode at once (R-MODE-3); each gets what it can take. */
    fun lockMode(m: Model, name: String, want: SettingLock, scheduleLock: Boolean): Outcome {
        m.mode(name) ?: return Outcome(m, "No mode $name")
        val blocks = m.blocksIn(name)
        if (blocks.isEmpty()) return Outcome(m, "The mode $name has no blocks")
        if (m.lockNeedsPassword) passwordWhy(m)?.let { return Outcome(m, it, needsPassword = true) }
        var cur = m
        val errors = mutableListOf<String>()
        for (b in blocks) {
            val o = lock(cur, b.name, want, scheduleLock)
            if (o.error != null) errors += "${b.name}: ${o.error}" else cur = o.model
        }
        return when {
            errors.size == blocks.size -> Outcome(m, errors.joinToString("; "))
            errors.isNotEmpty() -> Outcome(cur, null, "Locked, except " + errors.joinToString("; "))
            else -> ok(cur)
        }
    }

    /** The mode is locked while all its blocks are (R-MODE-3). */
    fun modeLocked(m: Model, name: String): Boolean {
        val blocks = m.blocksIn(name)
        return blocks.isNotEmpty() && blocks.all { locked(m, it) }
    }

    // Locks (each confirmed by the GUI before this runs, R-LOCK-12).
    fun lock(m: Model, name: String, want: SettingLock, scheduleLock: Boolean): Outcome {
        val b = m.block(name) ?: return Outcome(m, "No block $name")
        if (m.lockNeedsPassword) passwordWhy(m)?.let { return Outcome(m, it, needsPassword = true) }
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
        return b.lock.refuseUnlock(now, boot) ?: passwordWhy(m)
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
        if (old != null && old.wifi) return Outcome(m, "$n is a Wi-Fi place")
        if (old != null) usedByLocked(m) { n in m.automation(it).places }?.let { return refuse(m, it, "move place $n") }
        val p = Place(n, lat, lon, radius)
        return ok(m.copy(places = if (old == null) m.places + p else m.places.map { if (it.name == n) p else it }))
    }

    /**
     * A Wi-Fi place: new, or [seen] networks added to it and [needed] set. A change to a place
     * a locked block uses must strengthen each such block (placeStrengthens).
     */
    fun setWifiPlace(m: Model, name: String, seen: List<Network>, needed: Int? = null): Outcome {
        val n = name.trim()
        if (n.isEmpty()) return Outcome(m, "Give the place a name")
        val old = m.place(n)
        if (old != null && !old.wifi) return Outcome(m, "$n is an area on the map")
        val nets = ((old?.networks ?: emptyList()) + seen.map { Network(it.bssid.lowercase(), it.ssid) }).distinctBy { it.bssid }
        if (nets.isEmpty()) return Outcome(m, "No Wi-Fi networks in range: switch Wi-Fi (or Wi-Fi scanning) on")
        val need = needed ?: old?.needed ?: 2
        if (need < 1 || need > nets.size) return Outcome(m, "Networks needed: 1 to ${nets.size}")
        val p = (old ?: Place(n)).copy(networks = nets, needed = need)
        if (old != null) placeWeakens(m, old, p)?.let { return refuse(m, it, "change place $n") }
        return ok(m.copy(places = if (old == null) m.places + p else m.places.map { if (it.name == n) p else it }))
    }

    fun setPlaceNeeded(m: Model, name: String, needed: Int): Outcome {
        val p = m.place(name) ?: return Outcome(m, "No place $name")
        if (!p.wifi) return Outcome(m, "$name is an area on the map")
        return setWifiPlace(m, name, emptyList(), needed)
    }

    /** Forgets one network of a Wi-Fi place (the place stays with at least one). */
    fun removeNetwork(m: Model, name: String, bssid: String): Outcome {
        val p = m.place(name) ?: return Outcome(m, "No place $name")
        val nets = p.networks.filter { it.bssid != bssid }
        if (nets.isEmpty()) return Outcome(m, "A Wi-Fi place keeps at least one network; delete the place instead")
        val np = p.copy(networks = nets, needed = minOf(p.needed, nets.size))
        placeWeakens(m, p, np)?.let { return refuse(m, it, "remove a network from $name") }
        return ok(m.copy(places = m.places.map { if (it.name == name) np else it }))
    }

    /**
     * Why changing [old] to [new] weakens a locked block using it, or null. A wider place
     * (more networks, fewer needed) is there more often: stronger for a block it turns on
     * (disabled by default), weaker for one it lifts; a narrower one the other way round.
     */
    private fun placeWeakens(m: Model, old: Place, new: Place): String? {
        val wider = new.networks.containsAll(old.networks) && new.need <= old.need
        val narrower = old.networks.containsAll(new.networks) && new.need >= old.need
        return usedByLocked(m) { b ->
            val a = m.automation(b)
            old.name in a.places && !(if (a.defaultEnabled) narrower else wider)
        }
    }

    fun deletePlace(m: Model, name: String): Outcome {
        val p = m.place(name) ?: return Outcome(m, "No place $name")
        usedByLocked(m) { name in m.automation(it).places }?.let { return refuse(m, it, "delete place $name") }
        return ok(m.copy(places = m.places - p,
            blocks = m.blocks.map { it.copy(automation = it.automation.copy(places = it.automation.places - name)) },
            modes = m.modes.map { it.copy(automation = it.automation.copy(places = it.automation.places - name)) }))
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
        for (b in m.blocks.filter { name in m.automation(it).schedules }) {
            val why = whyLocked(m, b) ?: continue
            val before = Weakening.flatten(m, m.automation(b))
            val after = Weakening.flatten(m.copy(schedules = m.schedules.map { if (it.name == name) Schedule(name, windows) else it }), m.automation(b))
            if (!Weakening.strengthens(before, after)) return refuse(m, why, "change schedule $name (used by ${b.name})")
        }
        return ok(m.copy(schedules = m.schedules.map { if (it.name == name) s.copy(windows = windows) else it }))
    }

    fun deleteSchedule(m: Model, name: String): Outcome {
        val s = m.schedule(name) ?: return Outcome(m, "No schedule $name")
        usedByLocked(m) { name in m.automation(it).schedules }?.let { return refuse(m, it, "delete schedule $name") }
        return ok(m.copy(schedules = m.schedules - s,
            blocks = m.blocks.map { it.copy(automation = it.automation.copy(schedules = it.automation.schedules - name)) },
            modes = m.modes.map { it.copy(automation = it.automation.copy(schedules = it.automation.schedules - name)) }))
    }

    // Passwords (SPEC §3.11): hashes made by the caller (Passwords.hash); "" = none.
    /** A new settings password is free; changing or removing one weakens. */
    fun setSettingsPassword(m: Model, hash: String): Outcome {
        if (m.settingsPassword.isNotEmpty()) whySettingsLocked(m)?.let {
            return refuse(m, it, if (hash.isEmpty()) "remove the settings password" else "change the settings password") }
        return ok(m.copy(settingsPassword = hash, lockNeedsPassword = m.lockNeedsPassword && hash.isNotEmpty()))
    }

    /** Whether setting a lock needs the settings password too: on free, off weakens. */
    fun setLockNeedsPassword(m: Model, on: Boolean): Outcome {
        if (on && m.settingsPassword.isEmpty()) return Outcome(m, "Set a settings password first")
        if (!on && m.lockNeedsPassword) whySettingsLocked(m)?.let { return refuse(m, it, "let locks be set without the settings password") }
        return ok(m.copy(lockNeedsPassword = on))
    }

    /** An access password for an area (Access.AREAS): a new one is free; changing or removing one needs the settings password. */
    fun setAccessPassword(m: Model, area: String, hash: String): Outcome {
        if (area !in Access.AREAS) return Outcome(m, "No area $area")
        if (m.access[area].isNullOrEmpty() && hash.isEmpty()) return ok(m)
        if (!m.access[area].isNullOrEmpty()) passwordWhy(m)?.let { return Outcome(m, it, needsPassword = true) }
        return ok(m.copy(access = if (hash.isEmpty()) m.access - area else m.access + (area to hash)))
    }

    /** A block password hides the block's contents until typed (viewing only, locks unaffected). */
    fun setBlockPassword(m: Model, block: String, hash: String) =
        edit(m, block, false, "block password") { it.copy(viewPassword = hash) }

    // Device protection (SPEC §8.4): strict mode and tamper protection, each with its own setting lock.
    private fun device(m: Model, d: Device) = ok(m.copy(device = d))

    /** Why the device setting [name] (Device.STRICT, Device.TAMPER) can't be weakened now, or null. */
    private fun whyDeviceLocked(m: Model, name: String): String? {
        val l = if (name == Device.STRICT) m.device.strictLock else m.device.tamperLock
        if (l.inForce(now, boot)) return "Locked: " + l.current(now, boot).describe(now, boot)
        // R-LOCK-9: tamper protection can't be turned off while anything is locked.
        if (name == Device.TAMPER && Enforce.anythingLocked(m, now, boot, tz)) return "Tamper protection stays on while anything is locked"
        return whySettingsLocked(m)
    }

    /** Strict mode on is free; off is refused only by its own lock and no weakening (R-STRICT-1, R-WEAK-4). */
    fun setStrict(m: Model, on: Boolean): Outcome {
        if (!on && m.device.strict) whyDeviceLocked(m, Device.STRICT)?.let { return refuse(m, it, "turn off strict mode") }
        return device(m, m.device.copy(strict = on))
    }

    /** Leaving a part of strict mode out weakens; taking it back in is free. */
    fun setStrictPart(m: Model, part: StrictPart, leftOut: Boolean): Outcome {
        if (leftOut && part.key !in m.device.leftOut) whyDeviceLocked(m, Device.STRICT)?.let { return refuse(m, it, "leave ${part.label} out of strict mode") }
        return device(m, m.device.copy(leftOut = if (leftOut) m.device.leftOut + part.key else m.device.leftOut - part.key))
    }

    /** A tamper protection item on is free (the GUI asks OK first); off weakens. */
    fun setTamper(m: Model, item: Tamper, on: Boolean): Outcome {
        if (!on && m.device.has(item)) whyDeviceLocked(m, Device.TAMPER)?.let { return refuse(m, it, "turn off ${item.label}") }
        return device(m, m.device.copy(tamper = if (on) m.device.tamper + item.key else m.device.tamper - item.key))
    }

    /** Locks strict mode (only while on) or the tamper protection, only ever stronger. */
    fun lockDevice(m: Model, name: String, want: SettingLock): Outcome {
        if (m.lockNeedsPassword) passwordWhy(m)?.let { return Outcome(m, it, needsPassword = true) }
        if (name == Device.STRICT && !m.device.strict) return Outcome(m, "Turn strict mode on first")
        if (name == Device.TAMPER && m.device.tamper.isEmpty()) return Outcome(m, "Turn some tamper protection on first")
        val cur = if (name == Device.STRICT) m.device.strictLock else m.device.tamperLock
        val r = SettingLock.strengthen(cur, want, now, boot)
        r.error?.let { return Outcome(m, it) }
        return device(m, if (name == Device.STRICT) m.device.copy(strictLock = r.lock) else m.device.copy(tamperLock = r.lock))
    }

    /** Why the device setting's lock can't be opened by challenge/password now, or null. */
    fun refuseDeviceUnlock(m: Model, name: String): String? {
        val l = if (name == Device.STRICT) m.device.strictLock else m.device.tamperLock
        // Judged with this lock set aside: it must not hold itself (no weakening counts other locks).
        val without = m.copy(device = if (name == Device.STRICT) m.device.copy(strictLock = SettingLock()) else m.device.copy(tamperLock = SettingLock()))
        return l.refuseUnlock(now, boot) ?: whySettingsLocked(without)
    }

    /** After the password and challenge: the device setting's lock goes. */
    fun deviceUnlocked(m: Model, name: String): Outcome {
        refuseDeviceUnlock(m, name)?.let { return if (it == NEEDS_PASSWORD) Outcome(m, it, needsPassword = true) else Outcome(m, it) }
        return device(m, if (name == Device.STRICT) m.device.copy(strictLock = SettingLock()) else m.device.copy(tamperLock = SettingLock()))
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
