package org.kc9k.android

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.os.Handler
import android.os.Looper
import android.util.LruCache
import android.view.GestureDetector
import android.view.MotionEvent
import android.view.ScaleGestureDetector
import android.view.View
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executors

/**
 * A map of OpenStreetMap tiles drawn by the app itself (no library, no Play Services):
 * drag to move, pinch or double-tap to zoom, tap to put the home there. The home's radius
 * shows as a circle. Tiles are fetched while picking only, two at a time, kept in memory.
 * "© OpenStreetMap contributors" stays in the corner, as the tiles' licence asks.
 */
class MapView(context: Context) : View(context) {
    var zoom = MapMath.MIN_ZOOM + 1
        private set
    private var cx = 0.0 // the view's centre, in world pixels at [zoom]
    private var cy = 0.0
    var home: Pair<Double, Double>? = null
        set(v) { field = v; invalidate() }
    var radiusM = 150.0
        set(v) { field = v; invalidate() }
    /** Told when a tap puts the home somewhere. */
    var onPick: ((Double, Double) -> Unit)? = null

    private val cache = LruCache<String, Bitmap>(96)
    private val loading = HashSet<String>()
    private val failed = HashSet<String>()
    private val loader = Executors.newFixedThreadPool(2)
    private val main = Handler(Looper.getMainLooper())
    private val tilePaint = Paint(Paint.FILTER_BITMAP_FLAG)
    private val emptyPaint = Paint().apply { color = Color.rgb(230, 228, 224) }
    private val ring = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.STROKE; strokeWidth = 4f; color = Color.rgb(42, 120, 214) }
    private val fill = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.argb(48, 42, 120, 214) }
    private val dot = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.rgb(42, 120, 214) }
    private val dotRing = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.WHITE }
    private val credit = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = Color.rgb(40, 40, 40); textSize = 28f }
    private val creditBack = Paint().apply { color = Color.argb(200, 255, 255, 255) }

    /** Shows the place at [lat], [lon] in the middle, at [zoom]. */
    fun centre(lat: Double, lon: Double, zoom: Int = this.zoom) {
        this.zoom = zoom.coerceIn(MapMath.MIN_ZOOM, MapMath.MAX_ZOOM)
        val (x, y) = MapMath.toPixel(lat, lon, this.zoom)
        cx = x; cy = y
        invalidate()
    }

    fun zoomBy(step: Int) {
        val (lat, lon) = MapMath.toLatLon(cx, cy, zoom)
        centre(lat, lon, zoom + step)
    }

    private val gestures = GestureDetector(context, object : GestureDetector.SimpleOnGestureListener() {
        override fun onDown(e: MotionEvent) = true
        override fun onScroll(e1: MotionEvent?, e2: MotionEvent, dx: Float, dy: Float): Boolean {
            cx += dx; cy = (cy + dy).coerceIn(0.0, MapMath.worldSize(zoom))
            invalidate()
            return true
        }
        override fun onSingleTapConfirmed(e: MotionEvent): Boolean {
            val (lat, lon) = MapMath.toLatLon(cx - width / 2.0 + e.x, cy - height / 2.0 + e.y, zoom)
            home = lat to lon
            onPick?.invoke(lat, lon)
            return true
        }
        override fun onDoubleTap(e: MotionEvent): Boolean {
            // Zoom in on the tapped spot.
            val (lat, lon) = MapMath.toLatLon(cx - width / 2.0 + e.x, cy - height / 2.0 + e.y, zoom)
            centre(lat, lon, zoom + 1)
            return true
        }
    })

    private var pinch = 1f
    private val scaler = ScaleGestureDetector(context, object : ScaleGestureDetector.SimpleOnScaleGestureListener() {
        override fun onScaleBegin(d: ScaleGestureDetector): Boolean { pinch = 1f; return true }
        override fun onScale(d: ScaleGestureDetector): Boolean {
            pinch *= d.scaleFactor
            if (pinch > 1.6f) { zoomBy(1); pinch = 1f }
            if (pinch < 0.6f) { zoomBy(-1); pinch = 1f }
            return true
        }
    })

    override fun onTouchEvent(event: MotionEvent): Boolean {
        scaler.onTouchEvent(event)
        if (!scaler.isInProgress) gestures.onTouchEvent(event)
        return true
    }

    override fun onDraw(canvas: Canvas) {
        canvas.drawRect(0f, 0f, width.toFloat(), height.toFloat(), emptyPaint)
        for (t in MapMath.visibleTiles(cx, cy, width, height, zoom)) {
            val bitmap = tile(t)
            if (bitmap != null) canvas.drawBitmap(bitmap, t.left.toFloat(), t.top.toFloat(), tilePaint)
        }
        home?.let { (lat, lon) ->
            val (x, y) = MapMath.toPixel(lat, lon, zoom)
            val sx = (x - cx + width / 2.0).toFloat()
            val sy = (y - cy + height / 2.0).toFloat()
            val r = (radiusM / MapMath.metersPerPixel(lat, zoom)).toFloat()
            canvas.drawCircle(sx, sy, r, fill)
            canvas.drawCircle(sx, sy, r, ring)
            canvas.drawCircle(sx, sy, 14f, dotRing)
            canvas.drawCircle(sx, sy, 10f, dot)
        }
        val text = "© OpenStreetMap contributors"
        val w = credit.measureText(text)
        canvas.drawRect(width - w - 16, height - 44f, width.toFloat(), height.toFloat(), creditBack)
        canvas.drawText(text, width - w - 8, height - 12f, credit)
    }

    private fun tile(t: MapMath.Tile): Bitmap? {
        val key = "${t.zoom}/${t.x}/${t.y}"
        cache.get(key)?.let { return it }
        if (key in loading || key in failed) return null
        loading += key
        loader.execute {
            val bitmap = try {
                val c = URL("https://tile.openstreetmap.org/$key.png").openConnection() as HttpURLConnection
                // The tile servers' usage policy asks for an identifying User-Agent.
                c.setRequestProperty("User-Agent", "KC9K-Android/${BuildConfig.VERSION_NAME} (home picker)")
                c.connectTimeout = 10_000; c.readTimeout = 10_000
                c.inputStream.use { BitmapFactory.decodeStream(it) }
            } catch (e: Exception) {
                null
            }
            main.post {
                loading -= key
                if (bitmap != null) cache.put(key, bitmap) else failed += key
                invalidate()
            }
        }
        return null
    }

    override fun onDetachedFromWindow() {
        super.onDetachedFromWindow()
        loader.shutdownNow()
    }
}
