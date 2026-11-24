package org.kc9k.android

import android.app.Activity
import android.content.Context
import android.location.Location
import android.location.LocationManager
import android.os.Bundle
import android.view.ViewGroup
import android.widget.Button
import android.widget.LinearLayout
import android.widget.SeekBar
import android.widget.TextView
import android.widget.Toast

/**
 * Picks a place's area on a map (owner, 2026-11-16: no looking up coordinates elsewhere):
 * tap the map to put the centre there, set the radius with the slider, save. Saving goes
 * through Commands.setPlace (refused while a locked block uses the place). Opened with
 * [EXTRA_NAME]: the place to edit, or a new one's name.
 */
class MapPickerActivity : Activity() {
    companion object { const val EXTRA_NAME = "name" }
    private lateinit var name: String
    private lateinit var map: MapView
    private lateinit var radiusLabel: TextView
    private var picked: Pair<Double, Double>? = null
    private var radius = 150f

    private fun toast(s: String) = Toast.makeText(this, s, Toast.LENGTH_SHORT).show()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        name = intent.getStringExtra(EXTRA_NAME) ?: "Home"
        map = MapView(this)
        val old = Db.model(this).place(name)
        if (old != null) { radius = old.radius; picked = old.lat to old.lon }
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        root.addView(TextView(this).apply {
            text = "$name: tap the map at the place's centre. Drag to move, pinch or double-tap to zoom."
            setPadding(24, 16, 24, 8)
        })
        root.addView(map, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f))
        radiusLabel = TextView(this).apply { setPadding(24, 8, 24, 0) }
        root.addView(radiusLabel)
        // 50 m to 2 km, in 10 m steps.
        root.addView(SeekBar(this).apply {
            max = (2000 - 50) / 10
            progress = ((radius.coerceIn(50f, 2000f) - 50) / 10).toInt()
            setOnSeekBarChangeListener(object : SeekBar.OnSeekBarChangeListener {
                override fun onProgressChanged(s: SeekBar, p: Int, fromUser: Boolean) { setRadius(50f + p * 10) }
                override fun onStartTrackingTouch(s: SeekBar) {}
                override fun onStopTrackingTouch(s: SeekBar) {}
            })
        })
        val buttons = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL; setPadding(16, 8, 16, 16) }
        buttons.addView(Button(this).apply { text = "−"; setOnClickListener { map.zoomBy(-1) } })
        buttons.addView(Button(this).apply { text = "+"; setOnClickListener { map.zoomBy(1) } })
        buttons.addView(Button(this).apply { text = "Where I am"; setOnClickListener { goToMe() } })
        buttons.addView(Button(this).apply { text = "Save place"; setOnClickListener { save() } })
        root.addView(buttons)
        setContentView(root)

        map.onPick = { lat, lon -> picked = lat to lon }
        setRadius(radius)
        val start = picked ?: lastLocation()?.let { it.latitude to it.longitude }
        if (start != null) {
            map.home = picked
            map.post { map.centre(start.first, start.second, MapMath.zoomFor(radius.toDouble(), start.first, map.width.coerceAtLeast(400))) }
        } else {
            map.post { map.centre(30.0, 0.0, MapMath.MIN_ZOOM) }
        }
    }

    private fun setRadius(r: Float) {
        radius = r
        map.radiusM = r.toDouble()
        radiusLabel.text = "Radius: ${r.toInt()} m"
    }

    @Suppress("MissingPermission") // the app holds location access to work at all
    private fun lastLocation(): Location? = try {
        val lm = getSystemService(Context.LOCATION_SERVICE) as LocationManager
        listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)
            .mapNotNull { lm.getLastKnownLocation(it) }.maxByOrNull { it.time }
    } catch (e: SecurityException) {
        null
    }

    private fun goToMe() {
        val here = lastLocation()
        if (here == null) { toast("No position known yet"); return }
        map.centre(here.latitude, here.longitude, MapMath.zoomFor(radius.toDouble(), here.latitude, map.width))
    }

    private fun save() {
        val p = picked
        when {
            p == null -> toast("Tap the map at the place first")
            else -> {
                val o = Commands(System.currentTimeMillis(), Store(this).bootCount)
                    .setPlace(Db.model(this), name, p.first, p.second, radius)
                Db.save(this, o.model)
                if (o.error != null) toast(o.error) else {
                    // Decide it at once from the newest fix, so the GUI shows where the phone is.
                    PlaceWatcher(this).newest()?.let { f ->
                        Db.update(this) { m -> m.copy(places = m.places.map {
                            if (it.name == name) Places.step(it, System.currentTimeMillis(), f.latitude, f.longitude, f.accuracy) else it }) }
                    }
                    GuardService.resync(this)
                    toast("$name saved")
                    finish()
                }
            }
        }
    }
}
