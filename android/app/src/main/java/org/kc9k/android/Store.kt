package org.kc9k.android

import android.content.Context
import android.content.SharedPreferences

/** Persisted state, in device-protected storage so it is readable before the first unlock. */
class Store(private val context: Context) {
    private val p: SharedPreferences = context.createDeviceProtectedStorageContext()
        .getSharedPreferences("kc9k", Context.MODE_PRIVATE)

    /** Counts boots, for locks that last until the phone restarts. */
    val bootCount: Int
        get() = android.provider.Settings.Global.getInt(context.contentResolver,
            android.provider.Settings.Global.BOOT_COUNT, 0)

    val homeSet get() = p.contains("lat")
    var lat: Double
        get() = java.lang.Double.longBitsToDouble(p.getLong("lat", 0))
        set(v) { p.edit().putLong("lat", java.lang.Double.doubleToRawLongBits(v)).commit() }
    var lon: Double
        get() = java.lang.Double.longBitsToDouble(p.getLong("lon", 0))
        set(v) { p.edit().putLong("lon", java.lang.Double.doubleToRawLongBits(v)).commit() }
    var radiusM: Float
        get() = p.getFloat("radius", 150f)
        set(v) { p.edit().putFloat("radius", v).commit() }

    /** Last decided state; defaults to home so a fresh or wiped state fails closed. */
    var atHome: Boolean
        get() = p.getBoolean("atHome", true)
        set(v) { p.edit().putBoolean("atHome", v).commit() }
    var outsideSince: Long
        get() = p.getLong("outsideSince", 0)
        set(v) { p.edit().putLong("outsideSince", v).commit() }
    var lastFix: Long
        get() = p.getLong("lastFix", 0)
        set(v) { p.edit().putLong("lastFix", v).commit() }
    /** The last fix's distance from home and its accuracy, in metres, for the status line. */
    var lastDistance: Float
        get() = p.getFloat("lastDistance", -1f)
        set(v) { p.edit().putFloat("lastDistance", v).commit() }
    var lastAccuracy: Float
        get() = p.getFloat("lastAccuracy", -1f)
        set(v) { p.edit().putFloat("lastAccuracy", v).commit() }

    /**
     * "Lift everything" (DeviceOwner.liftEverything): nothing is enforced and every lock is
     * gone, but KC9K stays Device Owner. Resuming is always allowed.
     */
    var lifted: Boolean
        get() = p.getBoolean("lifted", false)
        set(v) { p.edit().putBoolean("lifted", v).commit() }

    /** Nokia mode: at home, so calls and texts only. Off while no home is set or while lifted. */
    val nokiaMode get() = homeSet && atHome && !lifted

    fun isOn(o: Option) = p.getBoolean("opt." + o.key, o.default)
    fun setOn(o: Option, on: Boolean) { p.edit().putBoolean("opt." + o.key, on).commit() }
    fun lockedUntil(o: Option) = p.getLong("lock." + o.key, 0)
    fun setLockedUntil(o: Option, t: Long) { p.edit().putLong("lock." + o.key, t).commit() }
    fun clearLocks() {
        Option.entries.forEach { p.edit().remove("lock." + it.key).commit() }
        settingLockNames.forEach(::clearSettingLock)
    }

    // Browser settings (Web.kt) and their locks.
    fun isOn(b: BrowserSetting) = p.getBoolean("browser." + b.key, false)
    fun setOn(b: BrowserSetting, on: Boolean) { p.edit().putBoolean("browser." + b.key, on).commit() }

    fun settingLock(name: String) = SettingLock(
        until = p.getLong("slock.$name.until", 0),
        chars = p.getInt("slock.$name.chars", 0),
        passwordHash = p.getString("slock.$name.pw", "") ?: "",
        restartBoot = p.getInt("slock.$name.boot", -1),
    )
    fun setSettingLock(name: String, l: SettingLock) {
        p.edit().putLong("slock.$name.until", l.until).putInt("slock.$name.chars", l.chars)
            .putString("slock.$name.pw", l.passwordHash).putInt("slock.$name.boot", l.restartBoot).commit()
    }
    fun clearSettingLock(name: String) {
        p.edit().remove("slock.$name.until").remove("slock.$name.chars").remove("slock.$name.pw")
            .remove("slock.$name.boot").commit()
    }
    /** Every setting lock's name, for the date/time protection (time locks trust the clock). */
    val settingLockNames get() = BrowserSetting.entries.map { it.key } + WebBlock.LOCK

    // The website block (WebBlock).
    var websitesOn: Boolean
        get() = p.getBoolean("web.on", true)
        set(v) { p.edit().putBoolean("web.on", v).commit() }
    var blocked: List<String>
        get() = lines("web.blocked")
        set(v) { p.edit().putString("web.blocked", v.distinct().joinToString("\n")).commit() }
    var exceptions: List<String>
        get() = lines("web.exceptions")
        set(v) { p.edit().putString("web.exceptions", v.distinct().joinToString("\n")).commit() }
    private fun lines(key: String) = (p.getString(key, "") ?: "").split("\n").filter { it.isNotEmpty() }

    // While lifted the extension gets empty rules and isn't required.
    fun rulesJson() = Rules.json(blocked, exceptions, websitesOn && !lifted,
        isOn(BrowserSetting.SAFESEARCH) && !lifted, isOn(BrowserSetting.WEB_ONLY) && !lifted)
    fun extensionRequired() = !lifted && Rules.extensionRequired(blocked, websitesOn,
        isOn(BrowserSetting.SAFESEARCH), isOn(BrowserSetting.WEB_ONLY))
}
