package org.kc9k.android

/** Something that decides whether a Policy should be on (GeofenceTrigger now; later Wi-Fi places, schedules). */
interface Trigger {
    /** Starts watching; [onChange] gets true while the trigger wants its policy applied. */
    fun start(onChange: (active: Boolean) -> Unit)
    fun stop()
}

/**
 * A block. Entries: "*.*" = every launchable app; "role:dialer", "role:sms" = the default
 * holders of those roles; anything else = a package name.
 */
data class Policy(
    val name: String,
    val blacklist: List<String>,
    val whitelist: List<String>,
    val wifiOff: Boolean = false,
)

/** Applies policies to the device. */
interface Enforcer {
    /** The packages [policy] suspends. */
    fun targets(policy: Policy): Set<String>
    /** Suspends exactly [suspend] (unsuspending anything else it suspended) and sets Wi-Fi. */
    fun sync(suspend: Set<String>, wifiOff: Boolean)
}

object Policies {
    const val SETTINGS = "com.android.settings"

    /**
     * Nokia mode: calls and texts only, on while at home. Everything launchable is paused
     * except the default (system) Phone and SMS apps, Settings included, and Wi-Fi is off.
     */
    val NOKIA = Policy(
        name = "nokia",
        blacklist = listOf("*.*", SETTINGS),
        whitelist = listOf("role:dialer", "role:sms"),
        wifiOff = true,
    )
}

/** What GuardService suspends, as a pure function so it can be tested. */
object Plan {
    /**
     * Nokia mode's targets (minus the system file picker while KC9K itself uses it: update,
     * list import/export) and IronFox while the extension guard pauses it.
     */
    fun suspend(nokia: Boolean, nokiaTargets: Set<String>, picker: Set<String>, pausedIronFox: Collection<String>) =
        (if (nokia) nokiaTargets - picker else emptySet()) + pausedIronFox
}
