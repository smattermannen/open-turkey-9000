package org.kc9k.android

import android.app.AlarmManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.SystemClock

/**
 * Enforces the enabled blocks (Enforce.plan: their apps suspended, Wi-Fi off), decides the
 * places (PlaceWatcher), serves the IronFox extension its rules (RulesServer) and pauses
 * IronFox while it runs without it (ExtensionGuard). The DO exemption allows background FGS
 * starts.
 */
class GuardService : Service() {
    companion object {
        private const val CHANNEL = "guard"
        private const val IRONFOX_CHANNEL = "ironfox"
        private const val NOTICE_CHANNEL = "notices"
        private const val TICK_MS = 5_000L
        private const val WATCHDOG_MS = 15 * 60_000L
        @Volatile var running = false
        @Volatile private var instance: GuardService? = null

        /**
         * Re-applies everything at once (after lifting/resuming, or when the picker is done),
         * on the running service's thread; starts the service if it isn't running.
         */
        fun resync(context: Context) {
            val s = instance ?: return start(context)
            s.handler.post { DeviceOwner(s).applyBaseline(); s.sync(force = true) }
        }

        fun start(context: Context) {
            try { context.startForegroundService(Intent(context, GuardService::class.java)) }
            catch (_: Exception) {}
            scheduleWatchdog(context)
        }

        fun scheduleWatchdog(context: Context) {
            val pi = PendingIntent.getBroadcast(context, 0,
                Intent(context, WatchdogReceiver::class.java), PendingIntent.FLAG_IMMUTABLE)
            context.getSystemService(AlarmManager::class.java).setAndAllowWhileIdle(
                AlarmManager.ELAPSED_REALTIME_WAKEUP, SystemClock.elapsedRealtime() + WATCHDOG_MS, pi)
        }
    }

    private lateinit var places: PlaceWatcher
    private lateinit var wifi: WifiWatcher
    private lateinit var enforcer: Enforcer
    private lateinit var store: Store
    private lateinit var server: RulesServer
    private lateinit var foreground: Foreground
    private lateinit var minExtension: String
    private val handler = Handler(Looper.getMainLooper())

    /** The resolved packages (`*` is costly), redone each minute or when the plan changes. */
    private var resolved: Pair<Enforce.Plan, Set<String>>? = null
    private var resolvedAt = 0L
    private var guard = ExtensionGuard.State()
    private var paused = false
    private var lastTick = 0L
    private var lastSync = 0L
    private var synced: Pair<Set<String>, Boolean>? = null
    private var browsersAt = 0L
    private var browserSet: Set<String> = emptySet()
    private var neverSet: Set<String> = emptySet()
    private var neverAt = 0L

    /** The installed browsers, looked up at most once a minute. */
    private fun cachedBrowsers(now: Long): Set<String> {
        if (now - browsersAt > 60_000L) { browserSet = enforcer.browsers(); browsersAt = now }
        return browserSet
    }

    private fun neverCached(now: Long): Set<String> {
        if (now - neverAt > 60_000L) { neverSet = enforcer.never(); neverAt = now }
        return neverSet
    }

    /** The block-paused apps at the last sync, for the "apps are paused" notice. */
    private var resolvedBefore: Set<String>? = null
    /** The device protection applied last, so a lock starting or ending (or a change) re-applies it. */
    private var device: DevicePlan? = null

    private val screenOff = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) = Session.end()
    }

    /** The phone was unlocked since the last autostart look (sign-in/wake on a phone, SPEC §8.3). */
    @Volatile private var unlocked = false
    private var inWindow: Map<String, Boolean>? = null
    private val userPresent = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) { unlocked = true; handler.post { autostarts() } }
    }

    /** Switches on blocks whose autostart is due (Autostarts.step); the first look only records. */
    private fun autostarts() {
        if (store.lifted) return
        val now = System.currentTimeMillis()
        val was = unlocked
        unlocked = false
        var fired: List<Pair<String, Autostart>> = emptyList()
        Db.update(this) { m -> Autostarts.step(m, now, was, inWindow).also { inWindow = it.inWindow; fired = it.fired }.model }
        for ((block, a) in fired) notice(Notices.Notice("autostart $block $now", "'$block' is on", "Its autostart (${a.describe()}) switched it on."))
        if (fired.isNotEmpty()) sync(force = true)
    }

    /** Notices shown (key to when), so each shows once (Notices). */
    private val shown = mutableMapOf<String, Long>()
    private var noticeMinute = 0L

    private fun notices() {
        val now = System.currentTimeMillis()
        if (now / 60_000 == noticeMinute || store.lifted) return
        noticeMinute = now / 60_000
        autostarts()
        shown.entries.removeAll { now - it.value > 2 * 86_400_000L }
        Notices.of(Db.model(this), store.notices, now).forEach(::notice)
    }

    private fun notice(n: Notices.Notice) {
        if (shown.containsKey(n.key)) return
        shown[n.key] = System.currentTimeMillis()
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        getSystemService(NotificationManager::class.java).notify(n.key.hashCode(), Notification.Builder(this, NOTICE_CHANNEL)
            .setContentTitle(n.title).setContentText(n.body).setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
            .setContentIntent(open).setAutoCancel(true).build())
    }

    private val tick = object : Runnable {
        override fun run() {
            notices()
            guardStep()
            val d = store.devicePlan()
            if (d != device) { device = d; DeviceOwner(this@GuardService).applyBaseline() }
            sync(force = false)
            handler.postDelayed(this, TICK_MS)
        }
    }

    override fun onCreate() {
        super.onCreate()
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(NotificationChannel(CHANNEL, "KC9K", NotificationManager.IMPORTANCE_LOW))
        nm.createNotificationChannel(NotificationChannel(IRONFOX_CHANNEL, "IronFox paused", NotificationManager.IMPORTANCE_DEFAULT))
        nm.createNotificationChannel(NotificationChannel(NOTICE_CHANNEL, "Coming up", NotificationManager.IMPORTANCE_DEFAULT))
        val n = Notification.Builder(this, CHANNEL)
            .setContentTitle("KC9K is guarding").setSmallIcon(android.R.drawable.ic_lock_lock)
            .setOngoing(true).build()
        if (Build.VERSION.SDK_INT >= 29) startForeground(1, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION)
        else startForeground(1, n)
        DeviceOwner(this).applyBaseline()
        store = Store(this)
        enforcer = DeviceOwnerEnforcer(this)
        server = RulesServer(this).also { it.start() }
        foreground = Foreground(this)
        minExtension = RulesServer.bundledVersion(this)
        Db.model(this) // migrates the 0.2 settings into blocks on first start
        // Re-apply the persisted state at once, before any fix arrives.
        sync(force = true)
        places = PlaceWatcher(this)
        places.start { sync(force = false) }
        wifi = WifiWatcher(this)
        wifi.start { sync(force = false) }
        handler.post(tick)
        // The passwords typed in KC9K (Session) hold until the screen turns off.
        registerReceiver(screenOff, android.content.IntentFilter(Intent.ACTION_SCREEN_OFF))
        registerReceiver(userPresent, android.content.IntentFilter(Intent.ACTION_USER_PRESENT))
        running = true
        instance = this
    }

    /** The extension guard (ExtensionGuard): pauses IronFox while it runs without the extension. */
    private fun guardStep() {
        val now = System.currentTimeMillis()
        val elapsed = if (lastTick == 0L) 0L else (now - lastTick).coerceIn(0L, 2 * TICK_MS)
        lastTick = now
        val required = store.extensionRequired() && IronFox.installed(this).isNotEmpty() &&
            (device ?: store.devicePlan()).closeBrowsers
        val access = !required || foreground.hasAccess()
        val inFront = required && access && foreground.top(now) in IronFox.PACKAGES
        val problem = ExtensionGuard.problem(ExtensionStatus.last, now, minExtension)
        val step = ExtensionGuard.step(guard, now, elapsed, required, access, inFront,
            problem == ExtensionGuard.Problem.NONE)
        guard = step.state
        if (step.pause != paused) {
            paused = step.pause
            notifyPause(if (!paused) null else if (!access)
                "KC9K needs usage access to see when IronFox is open. Settings > Apps > Special app access > Usage access > KC9K."
            else problem.text + ". IronFox is paused for ${ExtensionGuard.PAUSE_MS / 60_000} minutes; see KC9K.")
        }
    }

    private fun notifyPause(text: String?) {
        val nm = getSystemService(NotificationManager::class.java)
        if (text == null) { nm.cancel(2); return }
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE)
        nm.notify(2, Notification.Builder(this, IRONFOX_CHANNEL)
            .setContentTitle("IronFox is paused").setContentText(text)
            .setStyle(Notification.BigTextStyle().bigText(text))
            .setSmallIcon(android.R.drawable.ic_lock_lock).setContentIntent(open).build())
    }

    private fun sync(force: Boolean) {
        val now = System.currentTimeMillis()
        val plan = if (store.lifted) Enforce.Plan(emptyList(), false, emptyList()) else Enforce.plan(Db.model(this), now)
        val cached = resolved
        val targets = if (cached != null && cached.first == plan && now - resolvedAt < 60_000L) cached.second
            else Enforce.suspend(plan, enforcer::resolve, enforcer.never()).also { resolved = plan to it; resolvedAt = now }
        val browsers = if ((device ?: store.devicePlan()).blockBrowsers && store.extensionRequired()) cachedBrowsers(now) else emptySet()
        val wanted = Plan.suspend(targets, if (Picker.open(now)) Picker.packages else emptySet(),
            if (paused) IronFox.installed(this) else emptyList(), browsers, neverCached(now))
        val state = wanted to plan.wifiOff
        if (!force && state == synced && now - lastSync < 60_000L) return
        // Apps a block newly pauses (not at start, not IronFox's or the picker's own pauses).
        val before = resolvedBefore
        if (before != null) Notices.paused(store.notices, (targets - before).map { (enforcer as? DeviceOwnerEnforcer)?.label(it) ?: it })?.let(::notice)
        resolvedBefore = targets
        enforcer.sync(state.first, state.second)
        synced = state
        lastSync = now
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        scheduleWatchdog(this)
        return START_STICKY
    }

    override fun onDestroy() {
        running = false
        instance = null
        handler.removeCallbacks(tick)
        try { unregisterReceiver(screenOff) } catch (_: Exception) {}
        try { unregisterReceiver(userPresent) } catch (_: Exception) {}
        server.stop()
        places.stop()
        wifi.stop()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null
}

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) = GuardService.start(context)
}

class WatchdogReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) = GuardService.start(context)
}
