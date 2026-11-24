package org.kc9k.android

import android.os.UserManager

/** The parts of strict mode (SPEC §8.4): each closes one way out while something is locked. */
enum class StrictPart(val key: String, val label: String, val restrictions: List<String>) {
    SAFE_BOOT("safe-boot", "Safe mode", listOf(UserManager.DISALLOW_SAFE_BOOT)),
    DEBUGGING("debugging", "USB debugging (and mock locations)", listOf(UserManager.DISALLOW_DEBUGGING_FEATURES)),
    USERS("users", "Adding and switching users", listOf(UserManager.DISALLOW_ADD_USER, UserManager.DISALLOW_USER_SWITCH)),
    LOCATION("location", "Turning location off", listOf(UserManager.DISALLOW_CONFIG_LOCATION)),
    FACTORY_RESET("factory-reset", "Factory reset from Settings", listOf(UserManager.DISALLOW_FACTORY_RESET)),
}

/** Tamper protection on a phone (SPEC §8.4, §4.10); [default] as the desktop's defaults. */
enum class Tamper(val key: String, val label: String, val help: String, val default: Boolean) {
    CLOSE_BROWSERS("close-browsers", "Close IronFox if the extension stops",
        "Off: IronFox is paused without the extension only while anything is locked.", true),
    OTHER_USERS("other-users", "No other users, guests, work profiles or app clones",
        "A clone or another user would escape app pausing.", false),
    INSTALLER("installer", "Block the installer",
        "While anything is locked, no apps from unknown sources; an older KC9K never installs.", false),
    BROWSERS("browsers", "Block unsupported browsers",
        "While the extension is needed, every browser but IronFox is paused. Apps that show web pages inside themselves aren't browsers.", false),
    APP_CONTROLS("app-controls", "App controls",
        "KC9K can't be force-stopped or its data cleared.", false),
    CLOCK("clock", "Clock: block date, time and time zone changes",
        "Always while this is on; while anything is locked they are blocked anyway (trusted time).", false);

    companion object {
        val DEFAULTS = entries.filter { it.default }.map { it.key }.toSet()
        fun of(key: String) = entries.find { it.key == key }
    }
}

/**
 * Device protection in the model: strict mode (off by default, R-STRICT-1; parts left out
 * by key), tamper protection (the items on), and each one's own setting lock.
 */
data class Device(
    val strict: Boolean = false,
    val leftOut: Set<String> = emptySet(),
    val strictLock: SettingLock = SettingLock(),
    val tamper: Set<String> = Tamper.DEFAULTS,
    val tamperLock: SettingLock = SettingLock(),
) {
    fun has(t: Tamper) = t.key in tamper

    companion object {
        const val STRICT = "strict"
        const val TAMPER = "tamper"
    }
}

/** What Device Owner applies for the device protection now (DeviceOwner.applyBaseline). */
data class DevicePlan(
    val restrictions: Set<String>,
    /** Location kept on (strict mode's location part). */
    val location: Boolean,
    /** Automatic date, time and time zone (trusted time or the clock item). */
    val autoTime: Boolean,
    val uninstallBlocked: Boolean,
    /** KC9K can't be force-stopped or cleared. */
    val appControls: Boolean,
    /** IronFox is paused while it runs without the extension. */
    val closeBrowsers: Boolean,
    /** Other browsers paused while the extension is needed. */
    val blockBrowsers: Boolean = false,
)

object DeviceRules {
    const val NO_CLONE_PROFILE = "no_add_clone_profile"
    const val NO_UNKNOWN_SOURCES = "no_install_unknown_sources_globally"

    /**
     * [lockdown]: something locked other than strict mode's own lock (blocks, browser
     * settings, tamper protection), which starts strict mode; [anything]: any lock at all.
     */
    fun plan(d: Device, lockdown: Boolean, anything: Boolean, lifted: Boolean): DevicePlan {
        if (lifted) return DevicePlan(emptySet(), false, false, false, false, false)
        val r = mutableSetOf<String>()
        val strictOn = d.strict && lockdown
        val parts = StrictPart.entries.filter { strictOn && it.key !in d.leftOut }
        parts.forEach { r += it.restrictions }
        if (d.has(Tamper.OTHER_USERS)) r += listOf(UserManager.DISALLOW_ADD_USER, UserManager.DISALLOW_ADD_MANAGED_PROFILE, NO_CLONE_PROFILE)
        if (d.has(Tamper.INSTALLER) && anything) r += NO_UNKNOWN_SOURCES
        val autoTime = d.has(Tamper.CLOCK) || anything
        if (autoTime) r += UserManager.DISALLOW_CONFIG_DATE_TIME
        return DevicePlan(r, StrictPart.LOCATION in parts, autoTime, anything, d.has(Tamper.APP_CONTROLS),
            d.has(Tamper.CLOSE_BROWSERS) || anything, d.has(Tamper.BROWSERS))
    }

    /** Every restriction KC9K may set, so the ones not in a plan are cleared. */
    val ALL: Set<String> = StrictPart.entries.flatMap { it.restrictions }.toSet() + setOf(UserManager.DISALLOW_ADD_USER,
        UserManager.DISALLOW_ADD_MANAGED_PROFILE, NO_CLONE_PROFILE, NO_UNKNOWN_SOURCES, UserManager.DISALLOW_CONFIG_DATE_TIME)
}
