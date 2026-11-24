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

    /** Last decided state (Area.step); not at home until a fix says so. */
    var atHome: Boolean
        get() = p.getBoolean("atHome", false)
        set(v) { p.edit().putBoolean("atHome", v).commit() }
    /**
     * Before 0.3 a phone with no fix for 15 minutes counted as home and the state started at
     * home, so it could stay "at home" forever; drop that once and decide from fixes.
     */
    fun migrateArea() {
        if (p.getBoolean("areaV2", false)) return
        p.edit().putBoolean("atHome", false).putLong("outsideSince", 0).putBoolean("areaV2", true).commit()
    }
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

    /** Why settings outside blocks can't be weakened now (Commands.whySettingsLocked), or null. */
    fun whySettingsLocked(): String? = if (lifted) null
        else Commands(System.currentTimeMillis(), bootCount).whySettingsLocked(Db.model(context))

    fun isOn(o: Option) = p.getBoolean("opt." + o.key, o.default)
    fun setOn(o: Option, on: Boolean) { p.edit().putBoolean("opt." + o.key, on).commit() }
    fun lockedUntil(o: Option) = p.getLong("lock." + o.key, 0)
    fun setLockedUntil(o: Option, t: Long) { p.edit().putLong("lock." + o.key, t).commit() }
    fun clearLocks() {
        Option.entries.forEach { p.edit().remove("lock." + it.key).commit() }
        settingLockNames.forEach(::clearSettingLock)
    }

    /** What the last sync suspended, so a package a block named by hand is unpaused again. */
    var suspended: Set<String>
        get() = p.getStringSet("suspended", emptySet()) ?: emptySet()
        set(v) { if (v != suspended) p.edit().putStringSet("suspended", v).commit() }

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
    val settingLockNames get() = BrowserSetting.entries.map { it.key }

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

    // While lifted the extension gets empty rules and isn't required. The website lists are
    // the enabled blocks' (Enforce.plan); the old web.* keys are only read by Db's migration.
    fun rulesJson(): String {
        val web = if (lifted) emptyList() else Enforce.plan(Db.model(context), System.currentTimeMillis()).web
        return Rules.json(web, isOn(BrowserSetting.SAFESEARCH) && !lifted, isOn(BrowserSetting.WEB_ONLY) && !lifted)
    }
    fun extensionRequired() = !lifted && (Enforce.plan(Db.model(context), System.currentTimeMillis()).web.isNotEmpty() ||
        isOn(BrowserSetting.SAFESEARCH) || isOn(BrowserSetting.WEB_ONLY))

    /**
     * Whether anything is locked (a block, a browser setting, an option): trusted time then
     * keeps date, time and time zone automatic and unchangeable (SPEC §8.3).
     */
    fun anythingLocked(now: Long = System.currentTimeMillis()): Boolean = !lifted && (
        Enforce.anyLocked(Db.model(context), now, bootCount) ||
            settingLockNames.any { settingLock(it).inForce(now, bootCount) } ||
            Option.entries.any { lockedUntil(it) > now })
}
