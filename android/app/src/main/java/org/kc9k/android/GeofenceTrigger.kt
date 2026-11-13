package org.kc9k.android

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Handler
import android.os.Looper

/**
 * Active while at home. Entering is immediate; leaving needs [LEAVE_MS] outside the radius;
 * no fix for [NO_FIX_MS] counts as home (fail closed). Wall-clock times are safe because
 * DISALLOW_CONFIG_DATE_TIME is always set.
 */
class GeofenceTrigger(context: Context) : Trigger, LocationListener {
    companion object {
        const val LEAVE_MS = 10 * 60_000L
        const val NO_FIX_MS = 15 * 60_000L
        private const val TICK_MS = 60_000L
        private const val INTERVAL_MS = 30_000L
    }

    private val store = Store(context)
    private val lm = context.getSystemService(LocationManager::class.java)
    private val handler = Handler(Looper.getMainLooper())
    private var onChange: ((Boolean) -> Unit)? = null
    private val tick = object : Runnable {
        override fun run() { evaluate(null); handler.postDelayed(this, TICK_MS) }
    }

    @SuppressLint("MissingPermission")
    override fun start(onChange: (active: Boolean) -> Unit) {
        this.onChange = onChange
        if (store.lastFix == 0L) store.lastFix = System.currentTimeMillis()
        for (prov in listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)) {
            try {
                lm.requestLocationUpdates(prov, INTERVAL_MS, 0f, this, Looper.getMainLooper())
            } catch (_: Exception) { /* provider missing or permission not yet granted */ }
        }
        onChange(store.atHome)
        handler.post(tick)
    }

    override fun stop() {
        handler.removeCallbacks(tick)
        try { lm.removeUpdates(this) } catch (_: Exception) {}
        onChange = null
    }

    override fun onLocationChanged(location: Location) = evaluate(location)

    @Synchronized
    private fun evaluate(fix: Location?) {
        val now = System.currentTimeMillis()
        var home = store.atHome
        if (!store.homeSet) {
            home = false
        } else if (fix != null) {
            store.lastFix = now
            val d = FloatArray(1)
            Location.distanceBetween(store.lat, store.lon, fix.latitude, fix.longitude, d)
            store.lastDistance = d[0]; store.lastAccuracy = fix.accuracy
            // Only count as outside when even the accuracy circle is outside.
            if (d[0] <= store.radiusM) {
                home = true; store.outsideSince = 0
            } else if (d[0] - fix.accuracy > store.radiusM) {
                if (store.outsideSince == 0L) store.outsideSince = now
                if (now - store.outsideSince >= LEAVE_MS) home = false
            }
        } else if (now - store.lastFix >= NO_FIX_MS) {
            home = true; store.outsideSince = 0
        }
        store.atHome = home
        onChange?.invoke(home)
    }

    override fun onProviderEnabled(provider: String) {}
    override fun onProviderDisabled(provider: String) {}
    @Deprecated("Deprecated in API") override fun onStatusChanged(p: String?, s: Int, e: android.os.Bundle?) {}
}
