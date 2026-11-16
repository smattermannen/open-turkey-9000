package org.kc9k.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class MapMathTest {
    @Test fun roundTrip() {
        for ((lat, lon) in listOf(59.3293 to 18.0686, -33.8688 to 151.2093, 0.0 to 0.0, 64.1466 to -21.9426)) {
            for (zoom in listOf(2, 10, 17)) {
                val (x, y) = MapMath.toPixel(lat, lon, zoom)
                val (la, lo) = MapMath.toLatLon(x, y, zoom)
                assertEquals(lat, la, 1e-7)
                assertEquals(lon, lo, 1e-7)
            }
        }
    }

    @Test fun knownPlaces() {
        // The world's centre, and Stockholm's tile at zoom 10 (x 563, y 301).
        val (x0, y0) = MapMath.toPixel(0.0, 0.0, 1)
        assertEquals(256.0, x0, 1e-9)
        assertEquals(256.0, y0, 1e-9)
        val (x, y) = MapMath.toPixel(59.3293, 18.0686, 10)
        assertEquals(563, (x / 256).toInt())
        assertEquals(301, (y / 256).toInt())
    }

    @Test fun metersPerPixel() {
        assertEquals(156543.03, MapMath.metersPerPixel(0.0, 0), 0.01)
        assertEquals(156543.03 / 2 / 1024, MapMath.metersPerPixel(60.0, 10), 0.01) // cos 60° = 0.5
    }

    @Test fun wrapsRoundTheWorld() {
        val (lat, lon) = MapMath.toLatLon(-1.0, 512.0, 2) // a pixel west of 180° W
        assertTrue(lon > 179)
        assertEquals(0.0, lat, 1e-9)
        val tiles = MapMath.visibleTiles(0.0, 512.0, 512, 256, 2)
        assertTrue(tiles.all { it.x in 0..3 && it.y in 0..3 })
        assertTrue(tiles.any { it.x == 3 } && tiles.any { it.x == 0 }) // both sides of the date line
    }

    @Test fun rowsStopAtThePoles() {
        val tiles = MapMath.visibleTiles(512.0, 0.0, 512, 512, 2)
        assertTrue(tiles.all { it.y >= 0 })
    }

    @Test fun zoomShowsTheCircle() {
        val zoom = MapMath.zoomFor(150.0, 59.0, 1000)
        assertTrue(300 / MapMath.metersPerPixel(59.0, zoom) <= 600)
        assertTrue(zoom in 14..19)
    }
}
