package org.kc9k.android

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager

/**
 * The system file picker (DocumentsUI), which Nokia mode pauses with everything else. While
 * KC9K itself opens it (Install update, list import/export) it is let through for at most
 * [WINDOW_MS]; GuardService pauses it again when the window closes or KC9K gets its answer.
 */
object Picker {
    const val WINDOW_MS = 5 * 60_000L
    @Volatile private var until = 0L
    @Volatile var packages: Set<String> = emptySet()
        private set

    fun open(now: Long = System.currentTimeMillis()) = until > now

    /** Lets the picker through now; call just before starting it. */
    fun allow(context: Context) {
        packages = resolve(context)
        until = System.currentTimeMillis() + WINDOW_MS
        DeviceOwnerEnforcer(context).unsuspend(packages)
    }

    /** KC9K has its answer: back to normal at GuardService's next sync. */
    fun done(context: Context) {
        until = 0
        GuardService.resync(context)
    }

    private fun resolve(context: Context): Set<String> {
        val pm = context.packageManager
        return listOf(Intent.ACTION_OPEN_DOCUMENT, Intent.ACTION_CREATE_DOCUMENT).flatMap { action ->
            val i = Intent(action).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*")
            pm.queryIntentActivities(i, PackageManager.MATCH_ALL).map { it.activityInfo.packageName }
        }.toSet()
    }
}
