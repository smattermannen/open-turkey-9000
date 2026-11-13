package org.kc9k.android

import android.os.UserManager

/**
 * Device protections. Each is optional and can be time-locked; turning one on is always
 * allowed, turning one off needs it unlocked and the phone away from home.
 */
enum class Option(val key: String, val label: String, val restrictions: List<String>, val default: Boolean) {
    SAFE_BOOT("safe_boot", "Block safe mode", listOf(UserManager.DISALLOW_SAFE_BOOT), true),
    DEBUGGING("debugging", "Block USB debugging (and mock locations)",
        listOf(UserManager.DISALLOW_DEBUGGING_FEATURES), false),
    USERS("users", "Block adding/switching users",
        listOf(UserManager.DISALLOW_ADD_USER, UserManager.DISALLOW_USER_SWITCH), true),
    LOCATION("location", "Keep location on", listOf(UserManager.DISALLOW_CONFIG_LOCATION), true),
    DATE_TIME("date_time", "Block date/time changes", listOf(UserManager.DISALLOW_CONFIG_DATE_TIME), true),
    FACTORY_RESET("factory_reset", "Block factory reset", listOf(UserManager.DISALLOW_FACTORY_RESET), true),
    UNINSTALL("uninstall", "Block uninstalling KC9K", emptyList(), true);

    /** Debug builds never block debugging, so adb can always remove Device Owner. Lifted: nothing applies. */
    fun effective(store: Store) = store.isOn(this) && !store.lifted && !(this == DEBUGGING && BuildConfig.DEBUG)
}

class Options(private val store: Store) {
    fun locked(o: Option, now: Long = System.currentTimeMillis()) = store.lockedUntil(o) > now

    /** Why [o] can't be turned off now, or null if it can. */
    fun refuseOff(o: Option): String? = when {
        !store.isOn(o) -> null
        locked(o) -> "Locked"
        store.nokiaMode -> "Not in Nokia mode"
        // Time locks trust the clock, so the clock stays protected while any lock runs.
        o == Option.DATE_TIME && (Option.entries.any { it != o && locked(it) } ||
            WebSettings(store).anyTimeLock()) -> "Other options are locked"
        else -> null
    }

    fun set(o: Option, on: Boolean): String? {
        if (!on) refuseOff(o)?.let { return it }
        store.setOn(o, on)
        return null
    }

    /** Locks only extend, and only an option that is on. */
    fun lock(o: Option, ms: Long): String? {
        if (!store.isOn(o)) return "Turn it on first"
        // Locks trust the clock; without this they could be skipped by changing the time.
        if (!store.isOn(Option.DATE_TIME)) return "Turn on \"${Option.DATE_TIME.label}\" first"
        val until = System.currentTimeMillis() + ms
        if (until > store.lockedUntil(o)) store.setLockedUntil(o, until)
        return null
    }
}
