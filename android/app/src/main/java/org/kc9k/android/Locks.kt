package org.kc9k.android

import java.security.SecureRandom
import java.text.DateFormat
import java.util.Date
import javax.crypto.SecretKeyFactory
import javax.crypto.spec.PBEKeySpec

/**
 * The locks on one setting, as in the desktop's `kc9k browser … on [lock flags]`
 * (internal/db/settinglocks.go): a time lock, the typing challenge, a password, and
 * "until the phone restarts", in any combination. While any of them holds, the setting
 * can't be loosened. The zero value is unlocked.
 */
data class SettingLock(
    /** Wall-clock ms; 0 = no time lock. Trusted because date/time changes are blocked. */
    val until: Long = 0,
    /** Typing challenge length; 0 = none. */
    val chars: Int = 0,
    /** PBKDF2 hash (see [Passwords]); "" = none. */
    val passwordHash: String = "",
    /** The boot count the lock lasts for; -1 = none. */
    val restartBoot: Int = -1,
) {
    fun timeLocked(now: Long) = until > now
    fun restartLocked(boot: Int) = restartBoot >= 0 && restartBoot == boot
    fun inForce(now: Long, boot: Int) =
        timeLocked(now) || chars > 0 || passwordHash.isNotEmpty() || restartLocked(boot)

    /** The locks that have run out, dropped. */
    fun current(now: Long, boot: Int) = copy(
        until = if (timeLocked(now)) until else 0,
        restartBoot = if (restartLocked(boot)) restartBoot else -1,
    )

    /** "until 12 Nov 17:00, typing challenge (300 characters)", or "" when unlocked. */
    fun describe(now: Long, boot: Int): String {
        val parts = mutableListOf<String>()
        if (timeLocked(now)) parts += "until " + DateFormat.getDateTimeInstance(DateFormat.SHORT, DateFormat.SHORT).format(Date(until))
        if (restartLocked(boot)) parts += "until the phone restarts"
        if (chars > 0) parts += "typing challenge ($chars characters)"
        if (passwordHash.isNotEmpty()) parts += "password"
        return parts.joinToString(", ")
    }

    /** Why the lock can't be opened now, or null: time and restart locks can only be waited out. */
    fun refuseUnlock(now: Long, boot: Int): String? = when {
        timeLocked(now) -> "Locked " + describe(now, boot).substringBefore(",")
        restartLocked(boot) -> "Locked until the phone restarts"
        chars == 0 && passwordHash.isEmpty() -> "Not locked"
        else -> null
    }

    companion object {
        /**
         * Merges [want] into [cur], only ever making it stronger, like the desktop's
         * strengthenSettingLock: a time lock can only be extended, a challenge only
         * lengthened, and a password can't be replaced while one is set. Returns the new
         * lock and what was added, or an error.
         */
        fun strengthen(cur: SettingLock, want: SettingLock, now: Long, boot: Int): Result {
            var next = cur.current(now, boot)
            val changes = mutableListOf<String>()
            if (want.until != 0L) {
                if (want.until <= next.until) return Result(next, emptyList(), "Already locked longer; a time lock can only be extended")
                next = next.copy(until = want.until); changes += "time lock"
            }
            if (want.chars > 0) {
                when {
                    next.chars == 0 -> { next = next.copy(chars = want.chars); changes += "typing challenge" }
                    want.chars > next.chars -> { next = next.copy(chars = want.chars); changes += "longer challenge" }
                    want.chars < next.chars -> return Result(next, emptyList(), "The challenge is already ${next.chars} characters; it can't be shortened")
                }
            }
            if (want.passwordHash.isNotEmpty()) {
                if (next.passwordHash.isNotEmpty()) return Result(next, emptyList(), "It already has a password; that can't be replaced while locked")
                next = next.copy(passwordHash = want.passwordHash); changes += "password"
            }
            if (want.restartBoot >= 0 && next.restartBoot < 0) {
                next = next.copy(restartBoot = want.restartBoot); changes += "until restart"
            }
            if (changes.isEmpty()) return Result(next, changes, "It already has those locks")
            return Result(next, changes, null)
        }
    }

    data class Result(val lock: SettingLock, val changes: List<String>, val error: String?)
}

/** Salted PBKDF2 password hashes: "pbkdf2$<iterations>$<salt hex>$<hash hex>". */
object Passwords {
    private const val ITERATIONS = 120_000

    fun hash(password: String, salt: ByteArray = ByteArray(16).also { SecureRandom().nextBytes(it) }): String =
        "pbkdf2$$ITERATIONS$" + salt.hex() + "$" + derive(password, salt, ITERATIONS).hex()

    fun check(stored: String, password: String): Boolean {
        val parts = stored.split("$")
        if (parts.size != 4 || parts[0] != "pbkdf2") return false
        val iterations = parts[1].toIntOrNull() ?: return false
        val want = parts[3]
        val got = derive(password, parts[2].unhex(), iterations).hex()
        return got.length == want.length && got.indices.fold(0) { acc, i -> acc or (got[i].code xor want[i].code) } == 0
    }

    private fun derive(password: String, salt: ByteArray, iterations: Int): ByteArray =
        SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256")
            .generateSecret(PBEKeySpec(password.toCharArray(), salt, iterations, 256)).encoded

    private fun ByteArray.hex() = joinToString("") { "%02x".format(it) }
    private fun String.unhex() = chunked(2).map { it.toInt(16).toByte() }.toByteArray()
}

/**
 * The typing challenge, as on the desktop (internal/lock): random characters, 60% letters,
 * 35% digits, 5% symbols, typed with at least 98% accuracy (1 - edit distance / length).
 */
object Challenge {
    const val DEFAULT_CHARS = 300
    const val MIN_CHARS = 10
    const val MAX_CHARS = 5000
    const val MIN_ACCURACY = 0.98
    /** The most characters one edit may add before it counts as a paste. */
    const val PASTE_BURST = 4

    private const val LETTERS = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
    private const val DIGITS = "0123456789"
    private const val SYMBOLS = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

    fun checkLength(n: Int): String? =
        if (n in MIN_CHARS..MAX_CHARS) null else "The challenge must be $MIN_CHARS to $MAX_CHARS characters"

    fun generate(length: Int, random: SecureRandom = SecureRandom()): String {
        require(length > 0)
        return buildString {
            repeat(length) {
                val roll = random.nextInt(100)
                val cls = when { roll < 60 -> LETTERS; roll < 95 -> DIGITS; else -> SYMBOLS }
                append(cls[random.nextInt(cls.length)])
            }
        }
    }

    fun accuracy(challenge: String, typed: String): Double {
        if (challenge.isEmpty()) return 0.0
        return 1.0 - editDistance(challenge, typed).toDouble() / challenge.length
    }

    fun passed(challenge: String, typed: String) = accuracy(challenge, typed) >= MIN_ACCURACY

    fun editDistance(a: String, b: String): Int {
        var prev = IntArray(b.length + 1) { it }
        var cur = IntArray(b.length + 1)
        for (i in 1..a.length) {
            cur[0] = i
            for (j in 1..b.length) {
                val cost = if (a[i - 1] == b[j - 1]) 0 else 1
                cur[j] = minOf(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost)
            }
            val t = prev; prev = cur; cur = t
        }
        return prev[b.length]
    }
}
