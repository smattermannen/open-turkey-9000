package org.kc9k.android

import android.Manifest
import android.app.admin.DeviceAdminReceiver
import android.app.admin.DevicePolicyManager
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.wifi.WifiManager
import android.os.Build
import android.os.UserManager
import android.provider.Telephony
import android.telecom.TelecomManager

class AdminReceiver : DeviceAdminReceiver() {
    override fun onEnabled(context: Context, intent: Intent) {
        DeviceOwner(context).applyBaseline()
        GuardService.start(context)
    }
}

class DeviceOwner(private val context: Context) {
    private val dpm = context.getSystemService(DevicePolicyManager::class.java)
    val admin = ComponentName(context, AdminReceiver::class.java)
    val isOwner get() = dpm.isDeviceOwnerApp(context.packageName)

    private val store = Store(context)

    /** Applies each option as set (and lifts the ones that are off), plus permission grants. */
    fun applyBaseline() {
        if (!isOwner) return
        for (perm in listOf(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.ACCESS_COARSE_LOCATION,
            Manifest.permission.ACCESS_BACKGROUND_LOCATION,
            "android.permission.POST_NOTIFICATIONS",
        )) try {
            dpm.setPermissionGrantState(admin, context.packageName, perm,
                DevicePolicyManager.PERMISSION_GRANT_STATE_GRANTED)
        } catch (_: Exception) {}
        // While lifted every option counts as off (Option.effective), so all are cleared.
        for (o in Option.entries) {
            val on = o.effective(store)
            o.restrictions.forEach {
                if (on) dpm.addUserRestriction(admin, it) else dpm.clearUserRestriction(admin, it)
            }
        }
        if (Option.LOCATION.effective(store) && Build.VERSION.SDK_INT >= 30)
            dpm.setLocationEnabled(admin, true)
        if (Option.DATE_TIME.effective(store) && Build.VERSION.SDK_INT >= 30)
            dpm.setAutoTimeEnabled(admin, true)
        dpm.setUninstallBlocked(admin, context.packageName, Option.UNINSTALL.effective(store))
    }

    /**
     * Lift everything but stay Device Owner: every app unpaused, Wi-Fi allowed, every
     * restriction and lock gone, nothing for the extension to enforce. KC9K keeps running,
     * so [resume] turns it all back on (with the options as they were set, minus the locks).
     */
    fun liftEverything() {
        store.lifted = true
        store.clearLocks()
        if (!isOwner) return
        DeviceOwnerEnforcer(context).sync(emptySet(), wifiOff = false)
        applyBaseline()
    }

    fun resume() {
        store.lifted = false
        applyBaseline()
    }

    /** Debug builds only: undo everything and give up Device Owner. */
    fun releaseForDebug() {
        check(BuildConfig.DEBUG)
        if (!isOwner) return
        DeviceOwnerEnforcer(context).sync(emptySet(), wifiOff = false)
        Option.entries.flatMap { it.restrictions }.forEach { dpm.clearUserRestriction(admin, it) }
        dpm.setUninstallBlocked(admin, context.packageName, false)
        store.clearLocks()
        @Suppress("DEPRECATION") dpm.clearDeviceOwnerApp(context.packageName)
    }
}

/** Applies policies with Device Owner APIs: package suspension and Wi-Fi. */
class DeviceOwnerEnforcer(private val context: Context) : Enforcer {
    private val owner = DeviceOwner(context)
    private val dpm = context.getSystemService(DevicePolicyManager::class.java)
    private val pm = context.packageManager

    /**
     * Never suspended: our app, system UI and emergency-call components. Settings isn't
     * here: Nokia mode pauses it (Policies.NOKIA).
     */
    private val essential = setOf(
        context.packageName, "com.android.systemui",
        "com.android.phone", "com.android.emergency", "com.android.server.telecom",
        "com.android.providers.telephony", "com.android.cellbroadcastreceiver",
        "com.android.permissioncontroller",
    )

    private fun resolve(entries: List<String>): Set<String> = entries.flatMap { e ->
        when (e) {
            "*.*" -> launchable()
            // Only system apps count, so making another app the default can't unblock it.
            "role:dialer" -> listOfNotNull(context.getSystemService(TelecomManager::class.java)
                ?.defaultDialerPackage?.takeIf(::isSystem))
            "role:sms" -> listOfNotNull(Telephony.Sms.getDefaultSmsPackage(context)?.takeIf(::isSystem))
            else -> listOf(e)
        }
    }.toSet()

    private fun launchable(): List<String> {
        val i = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_LAUNCHER)
        return pm.queryIntentActivities(i, PackageManager.MATCH_ALL).map { it.activityInfo.packageName }
    }

    private fun defaultHome(): String? {
        val i = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_HOME)
        return pm.resolveActivity(i, PackageManager.MATCH_DEFAULT_ONLY)?.activityInfo?.packageName
            ?.takeIf(::isSystem)
    }

    private fun isSystem(pkg: String) = try {
        pm.getApplicationInfo(pkg, 0).flags and android.content.pm.ApplicationInfo.FLAG_SYSTEM != 0
    } catch (_: PackageManager.NameNotFoundException) { false }

    override fun targets(policy: Policy): Set<String> =
        resolve(policy.blacklist).filter(::installed).toSet() - resolve(policy.whitelist) - essential -
            setOfNotNull(defaultHome())

    override fun sync(suspend: Set<String>, wifiOff: Boolean) {
        if (!owner.isOwner) return
        // Android refuses some packages (the default dialer, the installer…); say which.
        notSuspended = if (suspend.isEmpty()) emptySet()
            else dpm.setPackagesSuspended(owner.admin, suspend.toTypedArray(), true).toSet()
        // Unsuspend anything that left the set (away from home, IronFox's pause over, a new
        // default SMS app).
        val stale = (launchable() + IronFox.PACKAGES + Policies.SETTINGS).distinct().filter { it !in suspend && isSuspended(it) }
        if (stale.isNotEmpty()) dpm.setPackagesSuspended(owner.admin, stale.toTypedArray(), false)
        if (wifiOff) {
            dpm.addUserRestriction(owner.admin, UserManager.DISALLOW_CONFIG_WIFI)
            @Suppress("DEPRECATION")
            context.getSystemService(WifiManager::class.java).isWifiEnabled = false
        } else {
            dpm.clearUserRestriction(owner.admin, UserManager.DISALLOW_CONFIG_WIFI)
        }
    }

    /** Unpauses [pkgs] now (the file picker, while KC9K uses it); GuardService's next sync agrees. */
    fun unsuspend(pkgs: Collection<String>) {
        if (!owner.isOwner) return
        val on = pkgs.filter(::isSuspended)
        if (on.isNotEmpty()) dpm.setPackagesSuspended(owner.admin, on.toTypedArray(), false)
    }

    private fun installed(pkg: String) = try { pm.getApplicationInfo(pkg, 0); true }
        catch (_: PackageManager.NameNotFoundException) { false }

    private fun isSuspended(pkg: String) =
        try { dpm.isPackageSuspended(owner.admin, pkg) } catch (_: Exception) { false }

    companion object {
        /** What the last sync asked to pause but Android refused, for the status line. */
        @Volatile var notSuspended: Set<String> = emptySet()
    }
}
