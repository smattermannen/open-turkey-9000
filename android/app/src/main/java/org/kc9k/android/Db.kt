package org.kc9k.android

import android.content.Context
import java.io.File

/** Model to and from JSON (pure, tested). */
object Codec {
    private fun win(w: Window) = mapOf("day" to w.day, "start" to w.start, "end" to w.end)
    private fun win(o: Map<String, Any?>) = Window(o.int("day"), o.int("start"), o.int("end"))
    private fun lock(l: SettingLock) = mapOf("until" to l.until, "chars" to l.chars, "pw" to l.passwordHash, "boot" to l.restartBoot)
    private fun lock(o: Map<String, Any?>) = SettingLock(o.long("until"), o.int("chars"), o.str("pw"), o.int("boot", -1))
    private fun auto(a: Automation) = mapOf("default_enabled" to a.defaultEnabled, "windows" to a.windows.map(::win),
        "schedules" to a.schedules, "places" to a.places, "both" to a.both)
    private fun auto(o: Map<String, Any?>) = Automation(o.bool("default_enabled"), o["windows"].list().map { win(it.obj()) },
        o.strings("schedules"), o.strings("places"), o.bool("both"))

    fun encode(m: Model): String = Json.write(mapOf(
        "version" to 1,
        "no_weakening" to m.noWeakening,
        "blocks" to m.blocks.map { b -> mapOf(
            "name" to b.name, "active" to b.active, "sites" to b.sites, "site_exceptions" to b.siteExceptions,
            "apps" to b.apps, "app_exceptions" to b.appExceptions, "wifi_off" to b.wifiOff,
            "automation" to auto(b.automation), "lock" to lock(b.lock), "schedule_lock" to b.scheduleLock,
            "paused_until" to b.pausedUntil, "mode" to b.mode, "social" to b.social, "view_password" to b.viewPassword,
            "autostarts" to b.autostarts.map { a -> mapOf("kind" to a.kind, "at" to a.at, "days" to a.days,
                "minute" to a.minute, "last_day" to a.lastDay) },
            "phrase_sets" to b.phraseSets.map { s -> mapOf("name" to s.name, "url" to s.inUrl, "title" to s.inTitle,
                "text" to s.inText, "threshold" to s.threshold, "test" to s.test,
                "phrases" to s.phrases.map { p -> mapOf("length" to p.length,
                    "sets" to p.sets.map { w -> mapOf("terms" to w.terms, "necessary" to w.necessary) }) }) }) },
        "modes" to m.modes.map { md -> mapOf("name" to md.name, "automation" to auto(md.automation)) },
        "places" to m.places.map { p -> mapOf(
            "name" to p.name, "lat" to p.lat, "lon" to p.lon, "radius" to p.radius.toDouble(),
            "inside" to p.inside, "outside_since" to p.outsideSince, "last_distance" to p.lastDistance.toDouble(),
            "last_accuracy" to p.lastAccuracy.toDouble(), "last_fix" to p.lastFix, "needed" to p.needed,
            "last_seen" to p.lastSeen, "networks" to p.networks.map { mapOf("bssid" to it.bssid, "ssid" to it.ssid) }) },
        "schedules" to m.schedules.map { s -> mapOf("name" to s.name, "windows" to s.windows.map(::win)) },
        "refused" to m.refused,
        "settings_password" to m.settingsPassword,
        "lock_needs_password" to m.lockNeedsPassword,
        "access" to m.access,
        "device" to mapOf("strict" to m.device.strict, "left_out" to m.device.leftOut.sorted(),
            "strict_lock" to lock(m.device.strictLock), "tamper" to m.device.tamper.sorted(),
            "tamper_lock" to lock(m.device.tamperLock)),
    ))

    fun decode(text: String): Model {
        val o = Json.parse(text).obj()
        return Model(
            blocks = o["blocks"].list().map { it.obj() }.map { b -> Block(
                name = b.str("name"), active = b.bool("active", true), sites = b.strings("sites"),
                siteExceptions = b.strings("site_exceptions"), apps = b.strings("apps"),
                appExceptions = b.strings("app_exceptions"), wifiOff = b.bool("wifi_off"),
                automation = auto(b["automation"].obj()), lock = lock(b["lock"].obj()),
                scheduleLock = b.bool("schedule_lock"), pausedUntil = b.long("paused_until"), mode = b.str("mode"),
                social = b.strings("social"), viewPassword = b.str("view_password"),
                autostarts = b["autostarts"].list().map { it.obj() }.map { a -> Autostart(a.str("kind"), a.long("at"),
                    a["days"].list().mapNotNull { (it as? Number)?.toInt() }, a.int("minute"), a.str("last_day")) },
                phraseSets = b["phrase_sets"].list().map { it.obj() }.map { s -> PhraseSet(s.str("name"),
                    s["phrases"].list().map { it.obj() }.map { p -> Phrase(p["sets"].list().map { it.obj() }.map { w ->
                        WordSet(w.strings("terms"), w.bool("necessary", true)) }, p.int("length")) },
                    s.bool("url"), s.bool("title"), s.bool("text", true), s.int("threshold", 1), s.bool("test")) }) },
            modes = o["modes"].list().map { it.obj() }.map { md -> Mode(md.str("name"), auto(md["automation"].obj())) },
            places = o["places"].list().map { it.obj() }.map { p -> Place(
                p.str("name"), p.double("lat"), p.double("lon"), p.double("radius", 150.0).toFloat(),
                p.bool("inside"), p.long("outside_since"), p.double("last_distance", -1.0).toFloat(),
                p.double("last_accuracy", -1.0).toFloat(), p.long("last_fix"),
                p["networks"].list().map { it.obj() }.map { Network(it.str("bssid"), it.str("ssid")) },
                p.int("needed", 2), p.int("last_seen")) },
            schedules = o["schedules"].list().map { it.obj() }.map { s ->
                Schedule(s.str("name"), s["windows"].list().map { win(it.obj()) }) },
            noWeakening = o.bool("no_weakening", true),
            refused = o.strings("refused"),
            settingsPassword = o.str("settings_password"),
            lockNeedsPassword = o.bool("lock_needs_password"),
            access = o["access"].obj().mapValues { it.value as? String ?: "" }.filterValues { it.isNotEmpty() },
            device = o["device"].obj().let { d -> if (d.isEmpty()) Device() else Device(d.bool("strict"),
                d.strings("left_out").toSet(), lock(d["strict_lock"].obj()),
                if (d.containsKey("tamper")) d.strings("tamper").toSet() else Tamper.DEFAULTS, lock(d["tamper_lock"].obj())) },
        )
    }

    /**
     * The model from the app before blocks (0.2): Nokia mode becomes the block "Nokia mode"
     * at the place "Home" (with a schedule lock: no weakening while at home, as before), the
     * website block becomes "Websites" with its lock.
     */
    fun migrate(homeSet: Boolean, lat: Double, lon: Double, radius: Float, atHome: Boolean,
                websitesOn: Boolean, blocked: List<String>, exceptions: List<String>, websitesLock: SettingLock): Model {
        val places = if (homeSet) listOf(Place("Home", lat, lon, radius, inside = atHome)) else emptyList()
        val nokia = Block(
            name = NOKIA, active = homeSet,
            apps = listOf("*", Policies.SETTINGS), appExceptions = listOf("role:dialer", "role:sms"),
            wifiOff = true,
            automation = Automation(defaultEnabled = false, places = if (homeSet) listOf("Home") else emptyList()),
            scheduleLock = homeSet,
        )
        val web = Block(name = "Websites", active = websitesOn, sites = blocked, siteExceptions = exceptions,
            lock = websitesLock)
        return Model(blocks = listOf(nokia, web), places = places)
    }

    const val NOKIA = "Nokia mode"
}

/**
 * The model in device-protected storage (readable before the first unlock), one JSON file
 * written atomically. One process: the GUI and GuardService share [model].
 */
object Db {
    private var cached: Model? = null
    private var file: File? = null

    @Synchronized fun model(context: Context): Model {
        cached?.let { return it }
        val f = file(context)
        val m = if (f.exists()) try { Codec.decode(f.readText()) } catch (_: Exception) { null } else null
        return (m ?: migrated(context).also { write(f, it) }).also { cached = it }
    }

    @Synchronized fun save(context: Context, m: Model) {
        write(file(context), m)
        cached = m
    }

    /** Applies [change] to the model and saves it; returns the new model. */
    @Synchronized fun update(context: Context, change: (Model) -> Model): Model {
        val m = change(model(context))
        save(context, m)
        return m
    }

    private fun file(context: Context) = file ?: File(context.createDeviceProtectedStorageContext().filesDir, "kc9k.json").also { file = it }

    private fun write(f: File, m: Model) {
        f.parentFile?.mkdirs()
        val tmp = File(f.parentFile, f.name + ".tmp")
        tmp.writeText(Codec.encode(m))
        if (!tmp.renameTo(f)) { f.writeText(Codec.encode(m)); tmp.delete() }
    }

    private fun migrated(context: Context): Model {
        val s = Store(context)
        s.migrateArea()
        return Codec.migrate(s.homeSet, s.lat, s.lon, s.radiusM, s.atHome, s.websitesOn, s.blocked, s.exceptions,
            s.settingLock(WebBlock.LOCK))
    }
}
