package org.kc9k.android

import kotlin.math.PI
import kotlin.math.atan
import kotlin.math.cos
import kotlin.math.exp
import kotlin.math.floor
import kotlin.math.ln
import kotlin.math.tan

/**
 * Web Mercator, as OpenStreetMap's tiles use it: the world at [zoom] is a square of
 * 256 * 2^zoom pixels, x growing east from 180° W, y growing south from 85.05° N.
 * Pure, so it is unit-tested.
 */
object MapMath {
    const val TILE = 256
    const val MAX_LAT = 85.05112878
    const val MIN_ZOOM = 2
    const val MAX_ZOOM = 19

    fun worldSize(zoom: Int): Double = TILE * (1 shl zoom).toDouble()

    /** The world pixel of a place at [zoom]. */
    fun toPixel(lat: Double, lon: Double, zoom: Int): Pair<Double, Double> {
        val size = worldSize(zoom)
        val la = lat.coerceIn(-MAX_LAT, MAX_LAT) * PI / 180
        val x = (lon + 180) / 360 * size
        val y = (1 - ln(tan(la) + 1 / cos(la)) / PI) / 2 * size
        return x to y
    }

    /** The place at a world pixel at [zoom] (x wraps round the world). */
    fun toLatLon(x: Double, y: Double, zoom: Int): Pair<Double, Double> {
        val size = worldSize(zoom)
        val wrapped = ((x % size) + size) % size
        val lon = wrapped / size * 360 - 180
        val n = PI * (1 - 2 * y.coerceIn(0.0, size) / size)
        val lat = 180 / PI * atan(0.5 * (exp(n) - exp(-n)))
        return lat to lon
    }

    /** Metres one pixel covers at [lat] and [zoom]. */
    fun metersPerPixel(lat: Double, zoom: Int): Double =
        40075016.686 * cos(lat.coerceIn(-MAX_LAT, MAX_LAT) * PI / 180) / worldSize(zoom)

    data class Tile(val x: Int, val y: Int, val zoom: Int, val left: Double, val top: Double)

    /**
     * The tiles a [width] x [height] view centred on world pixel ([cx], [cy]) shows, with
     * where each goes on the view. Columns wrap round the world; rows stop at its edges.
     */
    fun visibleTiles(cx: Double, cy: Double, width: Int, height: Int, zoom: Int): List<Tile> {
        val n = 1 shl zoom
        val left = cx - width / 2.0
        val top = cy - height / 2.0
        val firstX = floor(left / TILE).toInt()
        val firstY = floor(top / TILE).toInt()
        val lastX = floor((left + width) / TILE).toInt()
        val lastY = floor((top + height) / TILE).toInt()
        val out = mutableListOf<Tile>()
        for (ty in firstY..lastY) {
            if (ty < 0 || ty >= n) continue
            for (tx in firstX..lastX) {
                out += Tile(((tx % n) + n) % n, ty, zoom, tx * TILE - left, ty * TILE - top)
            }
        }
        return out
    }

    /** A sensible zoom to show a circle of [radiusM] metres at [lat] in [pixels]. */
    fun zoomFor(radiusM: Double, lat: Double, pixels: Int): Int {
        var zoom = MAX_ZOOM
        while (zoom > MIN_ZOOM && 2 * radiusM / metersPerPixel(lat, zoom) > pixels * 0.6) zoom--
        return zoom
    }
}
