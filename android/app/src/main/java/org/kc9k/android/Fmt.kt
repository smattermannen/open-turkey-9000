package org.kc9k.android

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * Preferences that shape what is shown (SPEC §4.10 Preferences; interface only, never
 * locked): the clock (24-hour, 12-hour or the phone's) and the first weekday. Set from
 * Store at start (MainActivity); pure, so the command layer's texts use it too.
 */
object Fmt {
    /** true 24-hour, false 12-hour, null the phone's setting ([systemH24]). */
    @Volatile var h24: Boolean? = null
    @Volatile var systemH24 = true
    /** 0 = Monday … 6 = Sunday (Window's days). */
    @Volatile var firstDay = 0

    private val is24 get() = h24 ?: systemH24
    val DAY_NAMES = listOf("Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday")

    /** The week's days in the order shown. */
    fun days() = (0..6).map { (firstDay + it) % 7 }

    /** A minute of the day: "17:05" or "5:05 PM"; 1440 is the day's end ("24:00", "12:00 AM"). */
    fun clock(minutes: Int): String {
        val h = minutes / 60 % 24; val m = minutes % 60
        if (is24) return "%02d:%02d".format(if (minutes == 1440) 24 else h, m)
        return "%d:%02d %s".format(if (h % 12 == 0) 12 else h % 12, m, if (h < 12) "AM" else "PM")
    }

    fun window(w: Window) = "${Window.DAYS[w.day]} ${clock(w.start)}–${clock(w.end)}"

    /** Windows in the week's order (first weekday first). */
    fun sorted(ws: List<Window>) = ws.sortedWith(compareBy({ (it.day - firstDay + 7) % 7 }, { it.start }))

    /** A moment: "Mon 24 Nov 17:05". */
    fun time(t: Long): String = SimpleDateFormat(if (is24) "EEE d MMM HH:mm" else "EEE d MMM h:mm a", Locale.getDefault()).format(Date(t))

    /** A time of day: "17:05". */
    fun hm(t: Long): String = SimpleDateFormat(if (is24) "HH:mm" else "h:mm a", Locale.getDefault()).format(Date(t))
}
