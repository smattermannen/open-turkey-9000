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
 * Picks the home on a map (owner, 2026-11-16: no looking up coordinates elsewhere): tap
 * the map to put the home there, set the radius with the slider, save. Like the fields in
 * Settings, only while away from home (at home it would loosen Nokia mode).
 */
class MapPickerActivity : Activity() {
    private lateinit var store: Store
    private lateinit var map: MapView
    private lateinit var radiusLabel: TextView
    private var picked: Pair<Double, Double>? = null
    private var radius = 150f

    private fun toast(s: String) = Toast.makeText(this, s, Toast.LENGTH_SHORT).show()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        store = Store(this)
        map = MapView(this)
        radius = store.radiusM
        if (store.homeSet) picked = store.lat to store.lon
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        root.addView(TextView(this).apply {
            text = "Tap the map where home is. Drag to move, pinch or double-tap to zoom."
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
        buttons.addView(Button(this).apply { text = "Save home"; setOnClickListener { save() } })
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
            p == null -> toast("Tap the map where home is first")
            store.nokiaMode -> toast("At home: not editable")
            radius < 50f -> toast("The radius is at least 50 m")
            else -> {
                store.lat = p.first; store.lon = p.second; store.radiusM = radius
                toast("Home saved")
                finish()
            }
        }
    }
}
