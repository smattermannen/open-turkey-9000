package org.kc9k.android

/**
 * What was typed this session (SPEC §3.11), in memory only: the settings password (so
 * weakening commands may run, Commands.settingsUnlocked), the access areas opened and the
 * blocks revealed. Ends when the screen turns off, the app is closed, or "Lock KC9K".
 */
object Session {
    @Volatile var settingsUnlocked = false
    val areas: MutableSet<String> = java.util.Collections.synchronizedSet(mutableSetOf())
    val revealed: MutableSet<String> = java.util.Collections.synchronizedSet(mutableSetOf())

    fun end() { settingsUnlocked = false; areas.clear(); revealed.clear() }
}

/** Access password areas (SPEC §3.11) on the phone: the whole app, and each page of the bottom bar. */
object Access {
    const val APP = "app"
    val AREAS = listOf(APP, "overview", "blocks", "stats", "more")
    val LABELS = mapOf(APP to "The app", "overview" to "Overview", "blocks" to "Blocks", "stats" to "Statistics", "more" to "More (settings, places, schedules)")
}
