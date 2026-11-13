package org.kc9k.android

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.os.Bundle
import android.text.InputType
import android.widget.Button
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import java.text.DateFormat
import java.util.Date

/** Home location, options with locks, website lists, updater, lift. Loosening never in Nokia mode. */
class SettingsActivity : Activity() {
    companion object {
        private const val PICK_APK = 1
        private const val SAVE_XPI = 2
        private const val CHALLENGE = 3
        private const val EXPORT_BLOCKED = 4
        private const val EXPORT_EXCEPTIONS = 5
        private const val IMPORT_BLOCKED = 6
        private const val IMPORT_EXCEPTIONS = 7
        /** Larger files aren't lists. */
        private const val MAX_LIST_BYTES = 2 shl 20
    }

    private lateinit var store: Store
    private lateinit var options: Options
    private lateinit var web: WebSettings
    /** The lock a running typing challenge opens. */
    private var unlocking: String? = null

    private fun toast(s: String) = Toast.makeText(this, s, Toast.LENGTH_SHORT).show()
    private val away get() = !store.nokiaMode

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        store = Store(this)
        options = Options(store)
        web = WebSettings(store)
        GuardService.start(this)
    }

    // Drawn on every return, so the state (at home or away, update result) is current.
    override fun onResume() {
        super.onResume()
        render()
    }

    private fun render() {
        val owner = DeviceOwner(this)
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(48, 48, 48, 48) }
        fun text(s: String) = root.addView(TextView(this).apply { text = s; setPadding(0, 24, 0, 8) })

        text(buildString {
            append(if (owner.isOwner) "Device Owner: yes\n" else "Device Owner: NO (see README)\n")
            append("State: " + when {
                store.lifted -> "LIFTED: nothing is enforced (Resume protection below)"
                !store.homeSet -> "no home set, so never in Nokia mode"
                store.atHome -> "Nokia mode (at home): calls and texts only"
                else -> "away"
            })
            if (store.homeSet && !store.lifted) append("\n" + locationLine())
            if (store.nokiaMode) append("\nEvery other app, Settings included, is paused and Wi-Fi is off. " +
                "Loosening waits until you're away.")
            val refused = DeviceOwnerEnforcer.notSuspended
            if (refused.isNotEmpty()) append("\nAndroid wouldn't pause: " + refused.sorted().joinToString(", "))
            UpdateReceiver.lastResult?.let { append("\n$it") }
        })

        // Home geofence.
        text("Home")
        fun field(hint: String, value: String) = EditText(this).apply {
            this.hint = hint; setText(value); isEnabled = away
            inputType = InputType.TYPE_CLASS_NUMBER or InputType.TYPE_NUMBER_FLAG_DECIMAL or
                InputType.TYPE_NUMBER_FLAG_SIGNED
        }
        val lat = field("Home latitude", if (store.homeSet) store.lat.toString() else "")
        val lon = field("Home longitude", if (store.homeSet) store.lon.toString() else "")
        val rad = field("Radius (m)", store.radiusM.toString())
        listOf(lat, lon, rad).forEach(root::addView)
        root.addView(Button(this).apply {
            text = "Save home"; isEnabled = away
            setOnClickListener {
                val la = lat.text.toString().toDoubleOrNull()
                val lo = lon.text.toString().toDoubleOrNull()
                val r = rad.text.toString().toFloatOrNull()
                when {
                    la == null || lo == null || r == null || la !in -90.0..90.0 ||
                        lo !in -180.0..180.0 || r < 50f -> toast("Invalid values (radius ≥ 50 m)")
                    !away -> toast("At home: not editable")
                    else -> { store.lat = la; store.lon = lo; store.radiusM = r; toast("Saved") }
                }
            }
        })

        // Options, each optional and lockable.
        text("Protections")
        val fmt = DateFormat.getDateTimeInstance(DateFormat.SHORT, DateFormat.SHORT)
        for (o in Option.entries) {
            val row = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
            row.addView(CheckBox(this).apply {
                text = o.label + if (o == Option.DEBUGGING && BuildConfig.DEBUG) " (ignored in debug builds)" else ""
                isChecked = store.isOn(o)
                setOnCheckedChangeListener { b, on ->
                    val err = options.set(o, on)
                    if (err != null) { toast(err); render() } else apply()
                }
            })
            val until = store.lockedUntil(o)
            if (options.locked(o)) row.addView(TextView(this).apply { text = "  Locked until " + fmt.format(Date(until)) })
            row.addView(Button(this).apply {
                text = "Lock…"
                setOnClickListener { askLock(o) }
            })
            root.addView(row)
        }

        ironFoxSection(root, ::text)
        browserSection(root, ::text)
        websitesSection(root, ::text)

        // Updater. Android only installs an APK signed with KC9K's own key, so it is allowed
        // in Nokia mode too (the file picker is let through while it is open).
        text("Update")
        root.addView(button("Install update (APK)…") {
            pick(Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "application/vnd.android.package-archive"
            }, PICK_APK)
        })

        // For now: a way out that keeps Device Owner, so a new APK can still be installed.
        text("Lift")
        if (store.lifted) {
            root.addView(TextView(this).apply {
                text = "Everything is lifted. Resuming applies Nokia mode, the protections and the websites as set " +
                    "(the locks were cleared)."; textSize = 12f
            })
            root.addView(button("Resume protection") {
                owner.resume(); GuardService.resync(this); toast("Protection is back"); render()
            })
        } else root.addView(button("Lift everything (keeps Device Owner)…") { confirmLift(owner) })

        if (BuildConfig.DEBUG) root.addView(Button(this).apply {
            text = "DEBUG: lift everything and remove Device Owner"
            setOnClickListener { owner.releaseForDebug(); toast("Released"); render() }
        })
        setContentView(ScrollView(this).apply { addView(root) })
    }

    private fun apply() { DeviceOwner(this).applyBaseline(); render() }

    /** "Last location 12:03: 230 m from home (±20 m)", or why there is none. */
    private fun locationLine(): String {
        if (store.lastDistance < 0) return "No location fix yet (15 minutes without one counts as home)"
        val t = DateFormat.getTimeInstance(DateFormat.SHORT).format(Date(store.lastFix))
        return "Last location $t: ${store.lastDistance.toInt()} m from home (±${store.lastAccuracy.toInt()} m, " +
            "radius ${store.radiusM.toInt()} m)"
    }

    private fun confirmLift(owner: DeviceOwner) {
        AlertDialog.Builder(this).setTitle("Lift everything?")
            .setMessage("Every app is unpaused, Wi-Fi is allowed, every protection is lifted and every lock is " +
                "cleared, and IronFox needs no extension. KC9K stays Device Owner and keeps your settings; " +
                "\"Resume protection\" turns them back on.")
            .setPositiveButton("Lift everything") { _, _ ->
                owner.liftEverything(); GuardService.resync(this); toast("Everything is lifted"); render()
            }
            .setNegativeButton("Cancel", null).show()
    }

    /** Starts the system file picker, let through Nokia mode while it is open (Picker). */
    private fun pick(intent: Intent, request: Int) {
        Picker.allow(this)
        try { startActivityForResult(intent, request) }
        catch (e: Exception) { Picker.done(this); toast("No file picker: ${e.message}") }
    }

    private fun button(label: String, enabled: Boolean = true, onClick: () -> Unit) = Button(this).apply {
        text = label; isEnabled = enabled; setOnClickListener { onClick() }
    }

    private fun result(err: String?) { err?.let(::toast); render() }

    // IronFox: the extension is installed by hand once, then checks in every few seconds.
    private fun ironFoxSection(root: LinearLayout, text: (String) -> Unit) {
        text("IronFox")
        val installed = IronFox.installed(this)
        val now = System.currentTimeMillis()
        val bundled = RulesServer.bundledVersion(this)
        val last = ExtensionStatus.last
        val fmt = DateFormat.getTimeInstance(DateFormat.MEDIUM)
        root.addView(TextView(this).apply {
            this.text = buildString {
                append(if (installed.isEmpty()) "IronFox: not installed\n" else "IronFox: installed\n")
                append("Extension: " + ExtensionGuard.problem(last, now, bundled).text)
                if (last != null) append(" (last ${fmt.format(Date(last.at))}, version ${last.version})")
                append("\nThis app carries version $bundled\n")
                append("Needed now: " + if (store.extensionRequired()) "yes" else "no (nothing below is on)")
                append("\nUsage access: " + if (Foreground(this@SettingsActivity).hasAccess()) "granted" else "MISSING: IronFox stays paused while the extension is needed")
                append("\n\nWhile it is needed, IronFox is paused if it is open for a minute without the extension.")
                append("\nOne-time setup in IronFox: Settings > IronFox > IronFox settings > Security > Allow installation of add-ons; ")
                append("about:config > xpinstall.signatures.required = false; then install the extension below ")
                append("and tick \"Allow in private browsing\".")
            }
        })
        root.addView(button("Grant usage access…") {
            startActivity(Intent(android.provider.Settings.ACTION_USAGE_ACCESS_SETTINGS))
        })
        root.addView(button("Install the extension in IronFox", installed.isNotEmpty()) {
            try {
                startActivity(Intent(Intent.ACTION_VIEW, android.net.Uri.parse(RulesServer.XPI_URL))
                    .setPackage(installed.first()).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            } catch (e: Exception) { toast("Couldn't open IronFox: ${e.message}") }
        })
        root.addView(button("Save the extension file (for Install extension from file)…") {
            pick(Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "application/x-xpinstall"
                putExtra(Intent.EXTRA_TITLE, "kc9k-$bundled.xpi")
            }, SAVE_XPI)
        })
    }

    private fun lockLine(root: LinearLayout, name: String) {
        val l = web.lock(name)
        val now = System.currentTimeMillis()
        if (l.inForce(now, store.bootCount)) root.addView(TextView(this).apply {
            text = "  Locked: " + l.describe(now, store.bootCount)
        })
    }

    private fun lockButtons(root: LinearLayout, name: String, onLock: (SettingLock) -> String?) {
        val row = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        row.addView(button("Lock…") { askSettingLock(name, onLock) })
        val l = web.lock(name)
        if (l.chars > 0 || l.passwordHash.isNotEmpty()) row.addView(button("Unlock…") { startUnlock(name) })
        root.addView(row)
    }

    // SafeSearch and web results, each with its own lock, as on the desktop.
    private fun browserSection(root: LinearLayout, text: (String) -> Unit) {
        text("Browser (enforced by the extension in IronFox)")
        for (b in BrowserSetting.entries) {
            root.addView(CheckBox(this).apply {
                this.text = b.label
                isChecked = store.isOn(b)
                setOnCheckedChangeListener { _, on -> result(if (on) web.turnOn(b, null) else web.turnOff(b)) }
            })
            root.addView(TextView(this).apply { this.text = "  " + b.help; textSize = 12f })
            lockLine(root, b.key)
            lockButtons(root, b.key) { want -> web.turnOn(b, want) }
        }
    }

    // The website block: Blocked and Exceptions, matched like the desktop's url lists.
    private fun websitesSection(root: LinearLayout, text: (String) -> Unit) {
        text("Websites")
        root.addView(CheckBox(this).apply {
            this.text = "Block the websites below"
            isChecked = store.websitesOn
            setOnCheckedChangeListener { _, on -> result(web.setWebsitesOn(on)) }
        })
        root.addView(TextView(this).apply {
            this.text = "  Entries as on the desktop: reddit.com (and its subdomains), reddit.com/r/funny, " +
                "*.xxx, *casino*, *.* (everything). An exception spares the pages it matches."
            textSize = 12f
        })
        lockLine(root, WebBlock.LOCK)
        lockButtons(root, WebBlock.LOCK) { want -> web.setWebsitesOn(true) ?: web.addLock(WebBlock.LOCK, want) }
        entryList(root, WebBlock.BLOCKED, store.blocked, "Add to Blocked", web::addBlocked, web::removeBlocked)
        listFiles(root, WebBlock.BLOCKED, store.blocked, EXPORT_BLOCKED, IMPORT_BLOCKED)
        entryList(root, WebBlock.EXCEPTIONS, store.exceptions, "Add exceptions", web::addExceptions, web::removeException)
        listFiles(root, WebBlock.EXCEPTIONS, store.exceptions, EXPORT_EXCEPTIONS, IMPORT_EXCEPTIONS)
        root.addView(TextView(this).apply {
            this.text = "  Files: one entry per line, # for comments, as the desktop's kc9k list export/import " +
                "and kc9k cluster export/import. Importing adds to the list; lines that aren't addresses are skipped."
            textSize = 12f
        })
    }

    /** Export and import buttons for one list. */
    private fun listFiles(root: LinearLayout, list: String, entries: List<String>, export: Int, import: Int) {
        val row = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        row.addView(button("Export $list…", entries.isNotEmpty()) {
            pick(Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "text/plain"
                putExtra(Intent.EXTRA_TITLE, "kc9k-phone-${list.lowercase()}.txt")
            }, export)
        })
        row.addView(button("Import into $list…") {
            pick(Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "text/*"
                // Some file managers call .txt/.list files octet-stream.
                putExtra(Intent.EXTRA_MIME_TYPES, arrayOf("text/*", "application/octet-stream"))
            }, import)
        })
        root.addView(row)
    }

    private fun exportList(uri: android.net.Uri, list: String) {
        val entries = if (list == WebBlock.BLOCKED) store.blocked else store.exceptions
        try {
            contentResolver.openOutputStream(uri, "wt")!!.use { it.write(ListFile.format(list, entries).toByteArray()) }
            toast("Exported ${entries.size} entries from $list")
        } catch (e: Exception) { toast("Couldn't export: ${e.message}") }
    }

    private fun importList(uri: android.net.Uri, list: String) {
        val text = try {
            contentResolver.openInputStream(uri)!!.use { input ->
                val out = java.io.ByteArrayOutputStream()
                val buf = ByteArray(8192)
                while (true) {
                    val n = input.read(buf)
                    if (n < 0) break
                    out.write(buf, 0, n)
                    if (out.size() > MAX_LIST_BYTES) return toast("Too big for a list (over 2 MB)")
                }
                out.toString("UTF-8")
            }
        } catch (e: Exception) { return toast("Couldn't read: ${e.message}") }
        var summary: String? = null
        val err = web.importList(list, text) { summary = it }
        if (err != null) toast(err) else summary?.let {
            Toast.makeText(this, it, Toast.LENGTH_LONG).show()
        }
        render()
    }

    private fun entryList(root: LinearLayout, title: String, entries: List<String>, addLabel: String,
                          add: (String) -> String?, remove: (String) -> String?) {
        root.addView(TextView(this).apply { text = "$title (${entries.size})"; setPadding(0, 16, 0, 0) })
        for (e in entries) {
            val row = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
            row.addView(TextView(this).apply {
                text = e; layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            })
            row.addView(button("Remove") { result(remove(e)) })
            root.addView(row)
        }
        val input = EditText(this).apply { hint = "One entry per line"; minLines = 2 }
        root.addView(input)
        root.addView(button(addLabel) { result(add(input.text.toString())) })
    }

    /** The custom lock: any mix of a time lock, the typing challenge, a password and until restart. */
    private fun askSettingLock(name: String, onLock: (SettingLock) -> String?) {
        val box = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(48, 16, 48, 0) }
        val hours = EditText(this).apply {
            hint = "Time lock: hours (e.g. 2, 168 for a week)"
            inputType = InputType.TYPE_CLASS_NUMBER or InputType.TYPE_NUMBER_FLAG_DECIMAL
        }
        val clock = EditText(this).apply { hint = "…or until a clock time, HH:MM"; inputType = InputType.TYPE_CLASS_DATETIME }
        val useChallenge = CheckBox(this).apply { text = "Typing challenge to unlock" }
        val chars = EditText(this).apply {
            hint = "Challenge length (${Challenge.MIN_CHARS}–${Challenge.MAX_CHARS})"; setText(Challenge.DEFAULT_CHARS.toString())
            inputType = InputType.TYPE_CLASS_NUMBER
        }
        val usePassword = CheckBox(this).apply { text = "Password to unlock" }
        val pw = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
        val password = EditText(this).apply { hint = "Password"; inputType = pw }
        val again = EditText(this).apply { hint = "Password again"; inputType = pw }
        val restart = CheckBox(this).apply { text = "Until the phone restarts" }
        listOf(hours, clock, useChallenge, chars, usePassword, password, again, restart).forEach(box::addView)
        AlertDialog.Builder(this).setTitle("Lock").setView(ScrollView(this).apply { addView(box) })
            .setPositiveButton("Lock") { _, _ ->
                val now = System.currentTimeMillis()
                var until = 0L
                val h = hours.text.toString().trim()
                val c = clock.text.toString().trim()
                if (h.isNotEmpty() && c.isNotEmpty()) return@setPositiveButton toast("Give hours or a clock time, not both")
                if (h.isNotEmpty()) {
                    val v = h.toDoubleOrNull()
                    if (v == null || v <= 0) return@setPositiveButton toast("Invalid hours")
                    until = now + (v * 3_600_000).toLong()
                }
                if (c.isNotEmpty()) {
                    until = untilClock(c, now) ?: return@setPositiveButton toast("Invalid clock time (HH:MM)")
                }
                var n = 0
                if (useChallenge.isChecked) {
                    n = chars.text.toString().toIntOrNull() ?: -1
                    Challenge.checkLength(n)?.let { return@setPositiveButton toast(it) }
                }
                var hash = ""
                if (usePassword.isChecked) {
                    val p = password.text.toString()
                    if (p.isEmpty()) return@setPositiveButton toast("Empty password")
                    if (p != again.text.toString()) return@setPositiveButton toast("The passwords differ")
                    hash = Passwords.hash(p)
                }
                val want = SettingLock(until, n, hash, if (restart.isChecked) store.bootCount else -1)
                if (want == SettingLock()) return@setPositiveButton toast("Choose at least one lock")
                result(onLock(want))
            }
            .setNegativeButton("Cancel", null).show()
    }

    /** The next HH:MM from now (today, or tomorrow if it has passed), in ms. */
    private fun untilClock(text: String, now: Long): Long? {
        val m = Regex("^(\\d{1,2}):(\\d{2})$").find(text) ?: return null
        val (hh, mm) = m.destructured
        if (hh.toInt() > 23 || mm.toInt() > 59) return null
        val cal = java.util.Calendar.getInstance().apply {
            timeInMillis = now; set(java.util.Calendar.HOUR_OF_DAY, hh.toInt()); set(java.util.Calendar.MINUTE, mm.toInt())
            set(java.util.Calendar.SECOND, 0); set(java.util.Calendar.MILLISECOND, 0)
        }
        if (cal.timeInMillis <= now) cal.add(java.util.Calendar.DAY_OF_MONTH, 1)
        return cal.timeInMillis
    }

    /** Password first (if any), then the typing challenge (if any), then the lock goes. */
    private fun startUnlock(name: String) {
        web.refuseUnlock(name)?.let { return toast(it) }
        val l = store.settingLock(name)
        val challenge = {
            if (l.chars > 0) {
                unlocking = name
                startActivityForResult(Intent(this, ChallengeActivity::class.java)
                    .putExtra(ChallengeActivity.EXTRA_CHARS, l.chars), CHALLENGE)
            } else result(web.unlocked(name))
        }
        if (l.passwordHash.isEmpty()) return challenge()
        val input = EditText(this).apply {
            hint = "Password"; inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
        }
        AlertDialog.Builder(this).setTitle("Unlock").setView(input)
            .setPositiveButton("OK") { _, _ ->
                if (Passwords.check(l.passwordHash, input.text.toString())) challenge() else toast("Wrong password")
            }
            .setNegativeButton("Cancel", null).show()
    }

    private fun askLock(o: Option) {
        val input = EditText(this).apply {
            hint = "Hours"; inputType = InputType.TYPE_CLASS_NUMBER or InputType.TYPE_NUMBER_FLAG_DECIMAL
        }
        AlertDialog.Builder(this).setTitle("Lock \"${o.label}\" for").setView(input)
            .setPositiveButton("Lock") { _, _ ->
                val h = input.text.toString().toDoubleOrNull()
                if (h == null || h <= 0) toast("Invalid hours")
                else options.lock(o, (h * 3_600_000).toLong())?.let(::toast) ?: apply()
            }
            .setNegativeButton("Cancel", null).show()
    }

    @Deprecated("Deprecated in API")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == CHALLENGE) {
            val name = unlocking ?: return
            unlocking = null
            if (resultCode == RESULT_OK) result(web.unlocked(name)) else toast("Still locked")
            return
        }
        // Every other request was a file picker: pause it again.
        Picker.done(this)
        val uri = data?.data ?: return
        if (resultCode != RESULT_OK) return
        when (requestCode) {
            EXPORT_BLOCKED -> return exportList(uri, WebBlock.BLOCKED)
            EXPORT_EXCEPTIONS -> return exportList(uri, WebBlock.EXCEPTIONS)
            IMPORT_BLOCKED -> return importList(uri, WebBlock.BLOCKED)
            IMPORT_EXCEPTIONS -> return importList(uri, WebBlock.EXCEPTIONS)
        }
        if (requestCode == SAVE_XPI) {
            try {
                contentResolver.openOutputStream(uri)!!.use { out -> assets.open("kc9k.xpi").use { it.copyTo(out) } }
                toast("Saved. In IronFox: Settings > Install extension from file.")
            } catch (e: Exception) { toast("Couldn't save: ${e.message}") }
            return
        }
        if (requestCode != PICK_APK) return
        toast("Installing…")
        Thread {
            try { Updater.install(this, uri) }
            catch (e: Exception) { runOnUiThread { toast("Update failed: ${e.message}") } }
        }.start()
    }
}
