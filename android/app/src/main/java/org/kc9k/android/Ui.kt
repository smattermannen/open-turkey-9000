package org.kc9k.android

import android.content.Context
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView

/**
 * The design layer (SPEC §8.5): cards with 12 dp corners and a thin outline, status pills
 * (on green, locked red, lifted blue, warning amber), section titles. KC9K's own colours.
 */
class Ui(private val c: Context, theme: String = SYSTEM, darkness: Int = 0) {
    companion object {
        const val SYSTEM = "system"; const val LIGHT = "light"; const val DARK = "dark"

        /** [t] of the way from [a] to [b]. */
        fun mix(a: Int, b: Int, t: Float) = Color.rgb(
            (Color.red(a) + (Color.red(b) - Color.red(a)) * t).toInt(),
            (Color.green(a) + (Color.green(b) - Color.green(a)) * t).toInt(),
            (Color.blue(a) + (Color.blue(b) - Color.blue(a)) * t).toInt())
    }

    private val density = c.resources.displayMetrics.density
    fun dp(v: Int) = (v * density).toInt()
    val dark = when (theme) {
        LIGHT -> false
        DARK -> true
        else -> (c.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES
    }
    /** The darkness slider: 0 dark grey … 1 black. */
    private val black = darkness.coerceIn(0, 100) / 100f

    val outline = if (dark) mix(Color.rgb(70, 70, 76), Color.rgb(44, 44, 48), black) else Color.rgb(214, 214, 220)
    val surface = if (dark) mix(Color.rgb(30, 30, 34), Color.rgb(10, 10, 10), black) else Color.WHITE
    val background = if (dark) mix(Color.rgb(18, 18, 20), Color.BLACK, black) else Color.rgb(246, 246, 248)
    val muted = if (dark) Color.rgb(170, 170, 178) else Color.rgb(96, 96, 104)
    val accent = Color.rgb(196, 64, 32)

    enum class Pill(val fg: Int, val bg: Int) {
        ON(Color.rgb(20, 108, 52), Color.rgb(214, 242, 222)),
        LOCKED(Color.rgb(160, 28, 28), Color.rgb(250, 220, 220)),
        LIFTED(Color.rgb(24, 84, 170), Color.rgb(218, 230, 250)),
        WARNING(Color.rgb(140, 88, 0), Color.rgb(252, 236, 200)),
        OFF(Color.rgb(90, 90, 96), Color.rgb(230, 230, 234)),
    }

    fun column(pad: Int = 0) = LinearLayout(c).apply {
        orientation = LinearLayout.VERTICAL; setPadding(dp(pad), dp(pad), dp(pad), dp(pad))
    }

    fun row() = LinearLayout(c).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL }

    fun card(): LinearLayout = column(14).apply {
        background = GradientDrawable().apply {
            cornerRadius = dp(12).toFloat(); setColor(surface); setStroke(dp(1), outline)
        }
        layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT,
            LinearLayout.LayoutParams.WRAP_CONTENT).apply { setMargins(0, dp(6), 0, dp(6)) }
    }

    fun title(s: String) = TextView(c).apply {
        text = s; textSize = 22f; setTypeface(typeface, Typeface.BOLD); setPadding(0, dp(8), 0, dp(8))
    }

    fun heading(s: String) = TextView(c).apply {
        text = s; textSize = 17f; setTypeface(typeface, Typeface.BOLD); setPadding(0, dp(12), 0, dp(4))
    }

    fun text(s: String, size: Float = 15f) = TextView(c).apply { text = s; textSize = size }

    fun help(s: String) = TextView(c).apply { text = s; textSize = 13f; setTextColor(muted); setPadding(0, dp(2), 0, dp(4)) }

    fun pill(s: String, kind: Pill) = TextView(c).apply {
        text = s; textSize = 12f; setTextColor(kind.fg); setPadding(dp(10), dp(3), dp(10), dp(3))
        background = GradientDrawable().apply { cornerRadius = dp(12).toFloat(); setColor(kind.bg) }
        layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT,
            LinearLayout.LayoutParams.WRAP_CONTENT).apply { setMargins(0, 0, dp(6), 0) }
    }

    fun button(label: String, enabled: Boolean = true, onClick: () -> Unit) = Button(c).apply {
        text = label; isEnabled = enabled; isAllCaps = false; setOnClickListener { onClick() }
    }

    /** Takes the rest of a row's width. */
    fun grow(v: View) = v.apply {
        layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
    }
}
