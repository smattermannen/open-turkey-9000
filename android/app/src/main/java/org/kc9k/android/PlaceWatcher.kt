package org.kc9k.android

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Handler
import android.os.Looper

/**
 * Decides each place (area) with [Area.step] from location fixes and saves it in the model:
 * entering is immediate; leaving needs [Area.LEAVE_MS] with the accuracy circle outside; no
 * fix changes nothing. Listens to GPS, network and fused fixes, takes the last known fix at
 * start, and registers again each minute while no fix comes (a permission granted later, a
 * provider switched on). [onChange] runs after each fix.
 */
class PlaceWatcher(private val context: Context) : LocationListener {
    companion object {
        private const val TICK_MS = 60_000L
        private const val INTERVAL_MS = 30_000L
        @Volatile var lastFixAt = 0L
    }

    private val lm = context.getSystemService(LocationManager::class.java)
    private val handler = Handler(Looper.getMainLooper())
    private var onChange: (() -> Unit)? = null
    private val tick = object : Runnable {
        override fun run() {
            if (System.currentTimeMillis() - lastFixAt > 2 * TICK_MS) listen()
            handler.postDelayed(this, TICK_MS)
        }
    }
    private val providers = listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER, "fused")

    fun start(onChange: () -> Unit) {
        this.onChange = onChange
        listen()
        newest()?.let { evaluate(it) }
        handler.post(tick)
    }

    fun stop() {
        handler.removeCallbacks(tick)
        try { lm.removeUpdates(this) } catch (_: Exception) {}
        onChange = null
    }

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
    fun newest(): Location? = (providers + LocationManager.PASSIVE_PROVIDER).mapNotNull {
        try { lm.getLastKnownLocation(it) } catch (_: Exception) { null }
    }.filter { System.currentTimeMillis() - it.time < Area.LEAVE_MS }.maxByOrNull { it.time }

    override fun onLocationChanged(location: Location) = evaluate(location)

    private fun evaluate(fix: Location) {
        val now = System.currentTimeMillis()
        lastFixAt = now
        Db.update(context) { m -> m.copy(places = m.places.map { p -> Places.step(p, now, fix.latitude, fix.longitude, fix.accuracy) }) }
        onChange?.invoke()
    }

    override fun onProviderEnabled(provider: String) {}
    override fun onProviderDisabled(provider: String) {}
    @Deprecated("Deprecated in API") override fun onStatusChanged(p: String?, s: Int, e: android.os.Bundle?) {}
}

object Places {
    /** One place decided from a fix (Area.step), with the distance kept for the GUI. */
    fun step(p: Place, now: Long, lat: Double, lon: Double, accuracy: Float): Place {
        val d = MapMath.distance(p.lat, p.lon, lat, lon).toFloat()
        val s = Area.step(Area.State(p.inside, p.outsideSince), now, p.radius, d, accuracy)
        return p.copy(inside = s.inside, outsideSince = s.outsideSince, lastDistance = d, lastAccuracy = accuracy, lastFix = now)
    }
}
