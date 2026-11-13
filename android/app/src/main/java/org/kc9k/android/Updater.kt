package org.kc9k.android

import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.net.Uri
import android.os.Build
import android.util.Log

/**
 * Installs a new KC9K APK picked by the user. As Device Owner this needs no prompt; Android
 * only accepts an APK signed with the same key, so nothing else can replace KC9K.
 */
object Updater {
    fun install(context: Context, apk: Uri) {
        val pi = context.packageManager.packageInstaller
        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL)
        params.setAppPackageName(context.packageName)
        val id = pi.createSession(params)
        pi.openSession(id).use { s ->
            context.contentResolver.openInputStream(apk)!!.use { input ->
                s.openWrite("kc9k.apk", 0, -1).use { out -> input.copyTo(out); s.fsync(out) }
            }
            var flags = PendingIntent.FLAG_UPDATE_CURRENT
            if (Build.VERSION.SDK_INT >= 31) flags = flags or PendingIntent.FLAG_MUTABLE
            val result = PendingIntent.getBroadcast(context, id,
                Intent(context, UpdateReceiver::class.java), flags)
            s.commit(result.intentSender)
        }
    }
}

class UpdateReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)
        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            @Suppress("DEPRECATION")
            (intent.getParcelableExtra<Intent>(Intent.EXTRA_INTENT))?.let {
                context.startActivity(it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            }
            return
        }
        val msg = intent.getStringExtra(PackageInstaller.EXTRA_STATUS_MESSAGE)
        Log.i("kc9k", "update status $status: $msg")
        UpdateReceiver.lastResult = if (status == PackageInstaller.STATUS_SUCCESS) "Updated" else "Update failed: $msg"
    }

    companion object { @Volatile var lastResult: String? = null }
}
