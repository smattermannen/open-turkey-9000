package org.kc9k.android

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Handler
import android.os.Looper

/**
 * Active while at home, decided by [Area.step]: entering is immediate; leaving needs
 * [Area.LEAVE_MS] with the accuracy circle outside; no fix changes nothing. Listens to GPS,
 * network and fused fixes, takes the last known fix at start, and registers again each
 * minute while no fix comes (a permission granted later, a provider switched on).
 */
class GeofenceTrigger(context: Context) : Trigger, LocationListener {
    companion object {
        private const val TICK_MS = 60_000L
        private const val INTERVAL_MS = 30_000L
    }

    private val store = Store(context)
    private val lm = context.getSystemService(LocationManager::class.java)
    private val handler = Handler(Looper.getMainLooper())
    private var onChange: ((Boolean) -> Unit)? = null
    private val tick = object : Runnable {
        override fun run() {
            if (System.currentTimeMillis() - store.lastFix > 2 * TICK_MS) listen()
            evaluate(null); handler.postDelayed(this, TICK_MS)
        }
    }

    override fun start(onChange: (active: Boolean) -> Unit) {
        this.onChange = onChange
        store.migrateArea()
        listen()
        newest()?.let { evaluate(it) } ?: onChange(store.atHome)
        handler.post(tick)
    }

    private val providers = listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER, "fused")

    @SuppressLint("MissingPermission")
    private fun listen() {
        try { lm.removeUpdates(this) } catch (_: Exception) {}
        for (prov in providers) try {
            if (lm.allProviders.contains(prov))
                lm.requestLocationUpdates(prov, INTERVAL_MS, 0f, this, Looper.getMainLooper())
        } catch (_: Exception) { /* permission not yet granted */ }
    }

    /** The newest last-known fix, if any is younger than ten minutes. */
    @SuppressLint("MissingPermission")
    private fun newest(): Location? = (providers + LocationManager.PASSIVE_PROVIDER).mapNotNull {
        try { lm.getLastKnownLocation(it) } catch (_: Exception) { null }
    }.filter { System.currentTimeMillis() - it.time < Area.LEAVE_MS }.maxByOrNull { it.time }

    override fun stop() {
        handler.removeCallbacks(tick)
        try { lm.removeUpdates(this) } catch (_: Exception) {}
        onChange = null
    }

    override fun onLocationChanged(location: Location) = evaluate(location)

    @Synchronized
    private fun evaluate(fix: Location?) {
        val now = System.currentTimeMillis()
        if (!store.homeSet) {
            store.atHome = false; store.outsideSince = 0
            onChange?.invoke(false); return
        }
        var distance: Float? = null
        if (fix != null) {
            store.lastFix = now
            val d = FloatArray(1)
            Location.distanceBetween(store.lat, store.lon, fix.latitude, fix.longitude, d)
            distance = d[0]
            store.lastDistance = d[0]; store.lastAccuracy = fix.accuracy
        }
        val s = Area.step(Area.State(store.atHome, store.outsideSince), now, store.radiusM,
            distance, fix?.accuracy ?: 0f)
        store.atHome = s.inside; store.outsideSince = s.outsideSince
        onChange?.invoke(s.inside)
    }

    override fun onProviderEnabled(provider: String) {}
    override fun onProviderDisabled(provider: String) {}
    @Deprecated("Deprecated in API") override fun onStatusChanged(p: String?, s: Int, e: android.os.Bundle?) {}
}
