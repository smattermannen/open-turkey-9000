package org.kc9k.android

/**
 * Changes to the browser settings and the website block. As on the desktop, what
 * strengthens is always allowed; what loosens (turning a setting or the block off,
 * removing a blocked entry, adding an exception) is refused while a lock holds, and,
 * like every loosening in this app, while the phone is at home. Each method returns
 * why it refused, or null.
 */
class WebSettings(private val store: Store, private val now: () -> Long = System::currentTimeMillis) {
    private val boot get() = store.bootCount

    fun lock(name: String) = store.settingLock(name).current(now(), boot)
    fun locked(name: String) = lock(name).inForce(now(), boot)

    private fun refuseLoosen(name: String): String? {
        val l = lock(name)
        return when {
            l.inForce(now(), boot) -> "Locked: " + l.describe(now(), boot)
            store.nokiaMode -> "Not in Nokia mode"
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
        if (store.nokiaMode) return "Not in Nokia mode"
        return store.settingLock(name).refuseUnlock(now(), boot)
    }

    /** After the challenge and password: the lock goes; a browser setting also goes off, as on the desktop. */
    fun unlocked(name: String): String? {
        refuseUnlock(name)?.let { return it }
        store.clearSettingLock(name)
        BrowserSetting.entries.find { it.key == name }?.let { store.setOn(it, false) }
        return null
    }

    // The website block.
    fun setWebsitesOn(on: Boolean): String? {
        if (!on && store.websitesOn) refuseLoosen(WebBlock.LOCK)?.let { return it }
        store.websitesOn = on
        return null
    }

    /** Adds blocked entries (always allowed). Returns the entries that aren't addresses, if any. */
    fun addBlocked(text: String): String? = add(text) { store.blocked = store.blocked + it }

    fun removeBlocked(entry: String): String? {
        refuseLoosen(WebBlock.LOCK)?.let { return it }
        store.blocked = store.blocked - entry
        return null
    }

    fun addExceptions(text: String): String? {
        refuseLoosen(WebBlock.LOCK)?.let { return it }
        return add(text) { store.exceptions = store.exceptions + it }
    }

    fun removeException(entry: String): String? {
        store.exceptions = store.exceptions - entry
        return null
    }

    /**
     * Imports a list file (ListFile) into Blocked or Exceptions, added to what is there, as
     * the desktop's `kc9k list import`. Into Blocked is always allowed; into Exceptions only
     * when adding exceptions is. Lines that aren't addresses are skipped, not fatal.
     * Returns the error, or null and the summary in [done].
     */
    fun importList(list: String, text: String, done: (String) -> Unit): String? {
        val r = ListFile.read(text)
        if (r.entries.isEmpty()) return if (r.skipped.isEmpty()) "The file has no entries"
            else "No addresses in the file (e.g. ${r.skipped.first()})"
        val cur = when (list) {
            WebBlock.BLOCKED -> store.blocked
            WebBlock.EXCEPTIONS -> { refuseLoosen(WebBlock.LOCK)?.let { return it }; store.exceptions }
            else -> return "No list $list"
        }
        val new = r.entries.filter { it !in cur }
        if (list == WebBlock.BLOCKED) store.blocked = cur + new else store.exceptions = cur + new
        done(ListFile.summary(list, new.size, r.entries.size - new.size, r.skipped))
        return null
    }

    private fun add(text: String, save: (List<String>) -> Unit): String? {
        val raw = Entries.split(text)
        if (raw.isEmpty()) return "No entries given"
        val bad = raw.filter { Entries.normalize(it) == null }
        if (bad.isNotEmpty()) return "Not an address or pattern: " + bad.joinToString(" ")
        save(raw.mapNotNull(Entries::normalize))
        return null
    }

    /** Whether any setting's time lock runs (the date/time protection stays on while one does). */
    fun anyTimeLock() = store.settingLockNames.any { store.settingLock(it).timeLocked(now()) }
}
