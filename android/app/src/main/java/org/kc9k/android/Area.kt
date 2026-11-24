package org.kc9k.android

/**
 * Whether the phone is in an area (SPEC §8.2 Area): in it when a fix says so (the fix's
 * point inside the circle); left when the accuracy circle has been fully outside for
 * [LEAVE_MS]. No fix, or a fix whose accuracy circle overlaps the edge, changes nothing.
 * A phone with no fix yet is not in it (SPEC: "in it when the location says so").
 */
object Area {
    const val LEAVE_MS = 10 * 60_000L

    data class State(val inside: Boolean = false, val outsideSince: Long = 0)

    /** [distance] from the centre and [accuracy] in metres; null distance = no new fix. */
    fun step(s: State, now: Long, radius: Float, distance: Float?, accuracy: Float): State = when {
        distance == null -> s
        distance <= radius -> State(true, 0)
        distance - accuracy > radius -> {
            val since = if (s.outsideSince == 0L) now else s.outsideSince
            State(if (now - since >= LEAVE_MS) false else s.inside, since)
        }
        else -> s
    }
}
