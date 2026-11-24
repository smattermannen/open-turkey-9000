package org.kc9k.android

/** Applies the plan to the device (DeviceOwnerEnforcer). */
interface Enforcer {
    /** Every package `entries` names (`*`, roles, packages), installed ones only. */
    fun resolve(entries: List<String>): Set<String>
    /** Never suspended: KC9K, system UI, emergency calls, the default launcher. */
    fun never(): Set<String>
    /** Every installed web browser (opens any https address), but the supported ones. */
    fun browsers(): Set<String>
    /** Suspends exactly [suspend] (unsuspending anything else it suspended) and sets Wi-Fi. */
    fun sync(suspend: Set<String>, wifiOff: Boolean)
}

object Policies {
    const val SETTINGS = "com.android.settings"
}

/** What GuardService suspends, as a pure function so it can be tested. */
object Plan {
    /**
     * The enabled blocks' apps (Enforce.suspend), minus the system file picker while KC9K
     * itself uses it (update, list import/export), plus IronFox while the extension guard
     * pauses it, plus [unsupportedBrowsers] (tamper protection "block unsupported browsers"
     * while the extension is needed), never [never].
     */
    fun suspend(blockTargets: Set<String>, picker: Set<String>, pausedIronFox: Collection<String>,
                unsupportedBrowsers: Set<String> = emptySet(), never: Set<String> = emptySet()) =
        (blockTargets - picker) + pausedIronFox + (unsupportedBrowsers - picker - never - IronFox.PACKAGES.toSet())
}
