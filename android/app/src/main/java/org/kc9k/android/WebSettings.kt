package org.kc9k.android

/**
 * Changes to the browser settings. As on the desktop, what strengthens is always allowed;
 * turning a setting off is refused while its lock holds, and while any block is locked
 * (no weakening, Commands.whySettingsLocked). Each method returns why it refused, or null.
 * Websites are in blocks (Commands).
 */
class WebSettings(private val store: Store, private val now: () -> Long = System::currentTimeMillis) {
    private val boot get() = store.bootCount

    fun lock(name: String) = store.settingLock(name).current(now(), boot)
    fun locked(name: String) = lock(name).inForce(now(), boot)

    private fun refuseLoosen(name: String): String? {
        val l = lock(name)
        return when {
            l.inForce(now(), boot) -> "Locked: " + l.describe(now(), boot)
            store.whySettingsLocked() != null -> store.whySettingsLocked()
            else -> null
        }
    }

    /** Adds locks (only ever stronger). Time locks need the date/time protection on. */
    fun addLock(name: String, want: SettingLock): String? {
        if (want.until != 0L && !store.isOn(Option.DATE_TIME)) return "Turn on \"${Option.DATE_TIME.label}\" first"
        val r = SettingLock.strengthen(store.settingLock(name), want, now(), boot)
        r.error?.let { return it }
        store.setSettingLock(name, r.lock)
        return null
    }

    // Browser settings.
    fun turnOn(b: BrowserSetting, want: SettingLock?): String? {
        if (want != null) {
            if (want.until != 0L && !store.isOn(Option.DATE_TIME)) return "Turn on \"${Option.DATE_TIME.label}\" first"
            val r = SettingLock.strengthen(store.settingLock(b.key), want, now(), boot)
            r.error?.let { return it }
            store.setOn(b, true)
            store.setSettingLock(b.key, r.lock)
        } else {
            store.setOn(b, true)
        }
        return null
    }

    fun turnOff(b: BrowserSetting): String? {
        if (!store.isOn(b)) return null
        refuseLoosen(b.key)?.let { return it }
        store.clearSettingLock(b.key)
        store.setOn(b, false)
        return null
    }

    /** Why [name]'s lock can't be opened (time and restart locks are waited out), or null. */
    fun refuseUnlock(name: String): String? {
        store.whySettingsLocked()?.let { return it }
        return store.settingLock(name).refuseUnlock(now(), boot)
    }

    /** After the challenge and password: the lock goes; a browser setting also goes off, as on the desktop. */
    fun unlocked(name: String): String? {
        refuseUnlock(name)?.let { return it }
        store.clearSettingLock(name)
        BrowserSetting.entries.find { it.key == name }?.let { store.setOn(it, false) }
        return null
    }

    /** Whether any setting's time lock runs (the date/time protection stays on while one does). */
    fun anyTimeLock() = store.settingLockNames.any { store.settingLock(it).timeLocked(now()) }
}
