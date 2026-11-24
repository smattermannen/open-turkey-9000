package org.kc9k.android

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.os.Bundle
import android.text.InputType
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.RadioButton
import android.widget.RadioGroup
import android.widget.ScrollView
import android.widget.Switch
import android.widget.TextView
import android.widget.Toast
import java.text.DateFormat
import java.util.Date

/**
 * The app's one screen (SPEC §8.5): bottom bar Overview · Blocks · Statistics · More; More
 * holds Schedules, Places and Settings. The GUI decides nothing: every change runs a
 * [Commands] method (blocks, places, schedules), [Options] or [WebSettings] (settings).
 * The page sits in one ScrollView kept across renders, so a tap keeps the scroll position.
 */
class MainActivity : Activity() {
    companion object {
        private const val PICK_APK = 1
        private const val SAVE_XPI = 2
        private const val CHALLENGE = 3
        private const val EXPORT = 4
        private const val IMPORT = 5
        /** Larger files aren't lists. */
        private const val MAX_LIST_BYTES = 2 shl 20
    }

    private lateinit var store: Store
    private lateinit var options: Options
    private lateinit var web: WebSettings
    private lateinit var ui: Ui
    private lateinit var scroll: ScrollView
    private lateinit var bar: LinearLayout

    /** Page: overview, blocks, stats, more; [sub]: a block's name, or schedules, places, settings, log. */
    private var page = "overview"
    private var sub: String? = null
    private var tab = "websites"
    /** The page shown last, so its scroll position is kept only while it stays. */
    private var shown = ""

    /** The lock a running typing challenge opens: "block:<name>" or "setting:<name>". */
    private var unlocking: String? = null
    /** The list an export/import picker is for: block name and whether it is the exceptions. */
    private var fileList: Pair<String, Boolean>? = null

    private fun toast(s: String) = Toast.makeText(this, s, Toast.LENGTH_SHORT).show()
    private val now get() = System.currentTimeMillis()
    private fun cmd() = Commands(now, store.bootCount)
    private val model get() = Db.model(this)

    /** Runs a command on the model, saves it, re-applies, and shows the refusal if any. */
    private fun run(op: (Commands, Model) -> Commands.Outcome) {
        val o = op(cmd(), model)
        Db.save(this, o.model)
        GuardService.resync(this)
        o.error?.let(::toast)
        o.note?.let { Toast.makeText(this, it, Toast.LENGTH_LONG).show() }
        render()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        store = Store(this)
        options = Options(store)
        web = WebSettings(store)
        ui = Ui(this)
        savedInstanceState?.let {
            page = it.getString("page", page); sub = it.getString("sub"); tab = it.getString("tab", tab)
        }
        scroll = ScrollView(this).apply {
            // The first text field would take focus and scroll itself into view on every render.
            descendantFocusability = ViewGroup.FOCUS_BEFORE_DESCENDANTS
            isFocusableInTouchMode = true
            isFillViewport = true
            setBackgroundColor(ui.background)
        }
        bar = ui.row().apply { setBackgroundColor(ui.surface); setPadding(ui.dp(4), ui.dp(4), ui.dp(4), ui.dp(4)) }
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        root.addView(scroll, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f))
        root.addView(bar)
        setContentView(root)
        GuardService.start(this)
    }

    override fun onSaveInstanceState(out: Bundle) {
        super.onSaveInstanceState(out)
        out.putString("page", page); out.putString("sub", sub); out.putString("tab", tab)
    }

    // Drawn on every return, so the state (places, update result) is current.
    override fun onResume() {
        super.onResume()
        render()
    }

    @Deprecated("Deprecated in API")
    override fun onBackPressed() {
        if (sub != null) { sub = null; render() }
        else if (page != "overview") go("overview")
        else @Suppress("DEPRECATION") super.onBackPressed()
    }

    private fun go(p: String, s: String? = null) {
        page = p; sub = s
        if (p == "blocks" && s != null) tab = "websites"
        render()
    }

    private fun render() {
        renderBar()
        val content = ui.column(16)
        when (page) {
            "blocks" -> if (sub != null && model.block(sub!!) != null) blockPage(content, sub!!) else { sub = null; blocksPage(content) }
            "stats" -> statsPage(content)
            "more" -> when (sub) {
                "schedules" -> schedulesPage(content)
                "places" -> placesPage(content)
                "settings" -> settingsPage(content)
                "log" -> logPage(content)
                else -> morePage(content)
            }
            else -> overviewPage(content)
        }
        val key = "$page/$sub/$tab"
        val y = if (key == shown) scroll.scrollY else 0
        shown = key
        scroll.removeAllViews()
        scroll.addView(content)
        scroll.requestFocus()
        scroll.post { scroll.scrollTo(0, y) }
    }

    private fun renderBar() {
        bar.removeAllViews()
        for ((key, label) in listOf("overview" to "Overview", "blocks" to "Blocks", "stats" to "Statistics", "more" to "More")) {
            bar.addView(ui.grow(ui.button(label) { go(key) }.apply {
                if (key == page) { setTextColor(ui.accent); setTypeface(typeface, android.graphics.Typeface.BOLD) }
                contentDescription = label
            }))
        }
    }

    // Overview: what is on, until when, how locked.
    private fun overviewPage(root: LinearLayout) {
        root.addView(ui.title("Overview"))
        val owner = DeviceOwner(this)
        val m = model
        val c = cmd()
        val warnings = mutableListOf<String>()
        if (!owner.isOwner) warnings += "KC9K isn't Device Owner, so it can't pause apps (adb shell dpm set-device-owner org.kc9k.android/.AdminReceiver)."
        if (store.lifted) warnings += "Everything is lifted: nothing is enforced (More > Settings > Resume protection)."
        if (store.extensionRequired() && !Foreground(this).hasAccess()) warnings += "Usage access is missing: IronFox stays paused while the extension is needed."
        DeviceOwnerEnforcer.notSuspended.takeIf { it.isNotEmpty() }?.let { warnings += "Android wouldn't pause: " + it.sorted().joinToString(", ") }
        UpdateReceiver.lastResult?.let { warnings += it }
        for (w in warnings) root.addView(ui.card().apply { addView(ui.pill("Warning", Ui.Pill.WARNING)); addView(ui.text(w)) })

        val on = m.blocks.filter { Enforce.enabled(m, it, now) && !store.lifted }
        root.addView(ui.heading(if (on.isEmpty()) "Nothing is blocked now" else "On now"))
        for (b in on) root.addView(blockCard(m, b, c))
        val lifted = m.blocks.filter { it.active && !Enforce.enabled(m, it, now) }
        if (lifted.isNotEmpty()) {
            root.addView(ui.heading("Switched on, not blocking now"))
            for (b in lifted) root.addView(blockCard(m, b, c))
        }
        if (m.places.isNotEmpty()) {
            root.addView(ui.heading("Places"))
            for (p in m.places) root.addView(ui.text("${p.name}: " + placeLine(p)))
        }
    }

    private fun placeLine(p: Place): String {
        val state = if (p.inside) "here" else "not here"
        if (p.lastFix == 0L) return "$state (no location fix yet)"
        val t = DateFormat.getTimeInstance(DateFormat.SHORT).format(Date(p.lastFix))
        return "$state (at $t ${p.lastDistance.toInt()} m from its centre, ±${p.lastAccuracy.toInt()} m; radius ${p.radius.toInt()} m)"
    }

    // Blocks.
    private fun blocksPage(root: LinearLayout) {
        root.addView(ui.title("Blocks"))
        val m = model
        val c = cmd()
        if (m.blocks.isEmpty()) root.addView(ui.help("No blocks yet."))
        for (b in m.blocks) root.addView(blockCard(m, b, c))
        root.addView(ui.button("New block…") {
            ask("New block", "Name") { name -> run { c2, m2 -> c2.addBlock(m2, name) }; if (model.block(name.trim()) != null) go("blocks", name.trim()) }
        })
    }

    /** Two-line card: name and switch; then type, state and lock, opening the block. */
    private fun blockCard(m: Model, b: Block, c: Commands): View = ui.card().apply {
        val top = ui.row()
        top.addView(ui.grow(ui.text(b.name, 17f).apply { setTypeface(typeface, android.graphics.Typeface.BOLD) }))
        top.addView(Switch(this@MainActivity).apply {
            isChecked = b.active
            contentDescription = "Switch ${b.name} on or off"
            setOnCheckedChangeListener { _, on -> run { c2, m2 -> c2.setActive(m2, b.name, on) } }
        })
        addView(top)
        val pills = ui.row()
        val st = Enforce.state(m, b, now)
        pills.addView(ui.pill(st.word, when (st) {
            Enforce.State.ENABLED -> Ui.Pill.ON; Enforce.State.INACTIVE -> Ui.Pill.OFF; else -> Ui.Pill.LIFTED }))
        if (c.locked(m, b)) pills.addView(ui.pill("locked", Ui.Pill.LOCKED))
        addView(pills)
        addView(ui.help(buildString {
            append(if (b.automation.continuous) "Continuous block" else "Automated block")
            if (st == Enforce.State.PAUSED) append(" · paused until " + time(b.pausedUntil))
            val parts = mutableListOf<String>()
            if (b.sites.isNotEmpty()) parts += "${b.sites.size} sites"
            if (b.apps.isNotEmpty()) parts += "${b.apps.size} apps"
            if (b.wifiOff) parts += "Wi-Fi off"
            if (parts.isNotEmpty()) append(" · " + parts.joinToString(", "))
            lockText(m, b).takeIf { it.isNotEmpty() }?.let { append("\nLocked: $it") }
        }))
        setOnClickListener { go("blocks", b.name) }
    }

    private fun lockText(m: Model, b: Block): String {
        val parts = mutableListOf<String>()
        b.lock.current(now, store.bootCount).describe(now, store.bootCount).takeIf { it.isNotEmpty() }?.let { parts += it }
        if (b.scheduleLock) parts += "schedule lock (while it is on)"
        return parts.joinToString(", ")
    }

    private fun time(t: Long) = DateFormat.getDateTimeInstance(DateFormat.SHORT, DateFormat.SHORT).format(Date(t))

    // The block editor: tabs Websites · Apps · Automation · Lock.
    private fun blockPage(root: LinearLayout, name: String) {
        val m = model
        val b = m.block(name)!!
        val c = cmd()
        val head = ui.row()
        head.addView(ui.button("‹ Blocks") { go("blocks") })
        head.addView(ui.grow(ui.title(b.name)))
        root.addView(head)
        root.addView(blockCard(m, b, c).apply { setOnClickListener(null) })
        val tabs = ui.row()
        for ((key, label) in listOf("websites" to "Websites", "apps" to "Apps", "automation" to "Automation", "lock" to "Lock")) {
            tabs.addView(ui.grow(ui.button(label) { tab = key; render() }.apply {
                if (key == tab) setTextColor(ui.accent)
            }))
        }
        root.addView(tabs)
        when (tab) {
            "apps" -> appsTab(root, b)
            "automation" -> automationTab(root, m, b)
            "lock" -> lockTab(root, m, b, c)
            else -> websitesTab(root, b)
        }
    }

    private fun websitesTab(root: LinearLayout, b: Block) {
        root.addView(ui.help("Entries as on the computer: reddit.com (and its subdomains), reddit.com/r/funny, " +
            "*.xxx, *casino*, *.* (everything). An exception spares the pages it matches, in this block only. " +
            "Enforced by the extension in IronFox."))
        entryList(root, "Blocked", b.sites, "Add to Blocked",
            { t -> run { c, m -> c.addSites(m, b.name, t) } }, { e -> run { c, m -> c.removeSite(m, b.name, e) } })
        listFiles(root, b, false)
        entryList(root, "Exceptions", b.siteExceptions, "Add exceptions",
            { t -> run { c, m -> c.addSiteExceptions(m, b.name, t) } }, { e -> run { c, m -> c.removeSiteException(m, b.name, e) } })
        listFiles(root, b, true)
        root.addView(ui.help("Files: one entry per line, # for comments, as the computer's kc9k list export/import. " +
            "Importing adds to the list; lines that aren't addresses are skipped."))
    }

    private fun appsTab(root: LinearLayout, b: Block) {
        val enforcer = DeviceOwnerEnforcer(this)
        root.addView(ui.help("Blocked apps are paused while the block is on. All apps = every app you can open, " +
            "but KC9K, the launcher, the system and emergency calls. An app exception lets an app through this block only."))
        root.addView(ui.heading("Apps (${b.apps.size})"))
        for (a in b.apps) root.addView(removable(AppEntries.describe(a, enforcer::label)) { run { c, m -> c.removeApp(m, b.name, a) } })
        root.addView(ui.button("Add apps…") { pickApps("Block apps", true) { run { c, m -> c.addApps(m, b.name, it) } } })
        root.addView(ui.heading("App exceptions (${b.appExceptions.size})"))
        for (a in b.appExceptions) root.addView(removable(AppEntries.describe(a, enforcer::label)) { run { c, m -> c.removeAppException(m, b.name, a) } })
        root.addView(ui.button("Add app exceptions…") { pickApps("Let through", false) { run { c, m -> c.addAppExceptions(m, b.name, it) } } })
        root.addView(ui.heading("Device"))
        root.addView(CheckBox(this).apply {
            text = "Wi-Fi off while the block is on"
            isChecked = b.wifiOff
            setOnCheckedChangeListener { _, on -> run { c, m -> c.setWifiOff(m, b.name, on) } }
        })
    }

    /** A multi-choice list of All apps, the roles and the launchable apps. */
    private fun pickApps(title: String, withAll: Boolean, done: (List<String>) -> Unit) {
        val apps = DeviceOwnerEnforcer(this).apps()
        val entries = (if (withAll) listOf(AppEntries.ALL to "All apps") else emptyList()) +
            listOf("role:dialer" to "Default phone app", "role:sms" to "Default SMS app") +
            apps.map { it.first to "${it.second} (${it.first})" }
        val picked = BooleanArray(entries.size)
        AlertDialog.Builder(this).setTitle(title)
            .setMultiChoiceItems(entries.map { it.second }.toTypedArray(), picked) { _, i, on -> picked[i] = on }
            .setPositiveButton("Add") { _, _ -> done(entries.filterIndexed { i, _ -> picked[i] }.map { it.first }) }
            .setNeutralButton("By package name…") { _, _ -> ask(title, "Package name, e.g. com.example.app") { done(listOf(it)) } }
            .setNegativeButton("Cancel", null).show()
    }

    private fun automationTab(root: LinearLayout, m: Model, b: Block) {
        val a = b.automation
        fun set(n: Automation) = run { c, mm -> c.setAutomation(mm, b.name, n) }
        root.addView(ui.help("No windows, schedules or places: a continuous block, on whenever it is switched on. " +
            "Otherwise they switch it from its default while it is switched on."))
        root.addView(RadioGroup(this).apply {
            val off = RadioButton(this@MainActivity).apply { id = View.generateViewId(); text = "Disabled by default: windows and places turn it on" }
            val on = RadioButton(this@MainActivity).apply { id = View.generateViewId(); text = "Enabled by default: windows and places turn it off" }
            addView(off); addView(on)
            check(if (a.defaultEnabled) on.id else off.id)
            setOnCheckedChangeListener { _, id -> if ((id == on.id) != a.defaultEnabled) set(a.copy(defaultEnabled = id == on.id)) }
        })
        root.addView(ui.heading(if (a.defaultEnabled) "Allow windows" else "Block windows"))
        for (w in a.windows) root.addView(removable(w.describe()) { set(a.copy(windows = a.windows - w)) })
        root.addView(ui.button("Add window…") { askWindows { ws -> set(a.copy(windows = a.windows + ws)) } })
        root.addView(ui.heading("Schedules"))
        if (m.schedules.isEmpty()) root.addView(ui.help("No schedules yet (More > Schedules)."))
        for (s in m.schedules) root.addView(CheckBox(this).apply {
            text = "${s.name} (${s.windows.size} windows)"; isChecked = s.name in a.schedules
            setOnCheckedChangeListener { _, on -> set(a.copy(schedules = if (on) a.schedules + s.name else a.schedules - s.name)) }
        })
        root.addView(ui.heading("Places"))
        if (m.places.isEmpty()) root.addView(ui.help("No places yet (More > Places)."))
        for (p in m.places) root.addView(CheckBox(this).apply {
            text = "${p.name} (${if (p.inside) "here" else "not here"})"; isChecked = p.name in a.places
            setOnCheckedChangeListener { _, on -> set(a.copy(places = if (on) a.places + p.name else a.places - p.name)) }
        })
        if ((a.windows.isNotEmpty() || a.schedules.isNotEmpty()) && a.places.isNotEmpty()) {
            root.addView(ui.heading("With times and places"))
            root.addView(RadioGroup(this).apply {
                val either = RadioButton(this@MainActivity).apply { id = View.generateViewId(); text = "Either switches it" }
                val both = RadioButton(this@MainActivity).apply { id = View.generateViewId(); text = "Both needed at once" }
                addView(either); addView(both)
                check(if (a.both) both.id else either.id)
                setOnCheckedChangeListener { _, id -> if ((id == both.id) != a.both) set(a.copy(both = id == both.id)) }
            })
        }
        root.addView(ui.heading("Pause"))
        if (b.pausedUntil > now) {
            root.addView(ui.text("Paused until " + time(b.pausedUntil)))
            root.addView(ui.button("End the pause") { run { c, mm -> c.endPause(mm, b.name) } })
        } else root.addView(ui.button("Pause until…") {
            ask("Pause ${b.name} until", "HH:MM") { t ->
                val until = untilClock(t, now) ?: return@ask toast("Invalid clock time (HH:MM)")
                run { c, mm -> c.pause(mm, b.name, until) }
            }
        })
        root.addView(ui.heading("Delete"))
        root.addView(ui.button("Delete ${b.name}…") {
            confirm("Delete ${b.name}?", "Its lists, apps and automation go with it.") {
                run { c, mm -> c.deleteBlock(mm, b.name) }
                if (model.block(b.name) == null) go("blocks")
            }
        })
    }

    private fun lockTab(root: LinearLayout, m: Model, b: Block, c: Commands) {
        root.addView(ui.help("While locked, the block can't be weakened: not switched off, paused or deleted, no " +
            "entries removed, no exceptions added, no automation that lets it lift more. Strengthening stays allowed."))
        val desc = lockText(m, b)
        root.addView(ui.text(if (desc.isEmpty()) "Not locked." else "Locks: $desc"))
        if (c.locked(m, b)) root.addView(ui.pill("locked now", Ui.Pill.LOCKED))
        root.addView(ui.button("Lock…") {
            askLock("Lock ${b.name}", schedule = !b.automation.continuous) { want, sched ->
                run { c2, m2 -> c2.lock(m2, b.name, want, sched) }
            }
        })
        if (b.lock.chars > 0 || b.lock.passwordHash.isNotEmpty()) root.addView(ui.button("Unlock…") { startUnlock("block:${b.name}") })
        if (b.scheduleLock) root.addView(ui.button("Remove the schedule lock") { run { c2, m2 -> c2.removeScheduleLock(m2, b.name) } })
    }

    // Statistics.
    private fun statsPage(root: LinearLayout) {
        root.addView(ui.title("Statistics"))
        root.addView(ui.card().apply {
            addView(ui.pill("Not built yet", Ui.Pill.WARNING))
            addView(ui.text("Statistics (use per block, app and site) come with breaks and allowance on the phone."))
        })
    }

    // More.
    private fun morePage(root: LinearLayout) {
        root.addView(ui.title("More"))
        for ((key, label, help) in listOf(
            Triple("schedules", "Schedules", "Named weekly windows that blocks use"),
            Triple("places", "Places", "Areas on the map, found by GPS"),
            Triple("settings", "Settings", "Protections, browser, IronFox, update"),
            Triple("log", "Refused changes", "Weakenings refused while locked"),
        )) root.addView(ui.card().apply {
            addView(ui.text(label, 17f)); addView(ui.help(help))
            setOnClickListener { go("more", key) }
        })
    }

    private fun backTo(root: LinearLayout, title: String) {
        val head = ui.row()
        head.addView(ui.button("‹ More") { go("more") })
        head.addView(ui.grow(ui.title(title)))
        root.addView(head)
    }

    private fun schedulesPage(root: LinearLayout) {
        backTo(root, "Schedules")
        val m = model
        root.addView(ui.help("A schedule is a set of weekly windows; a block uses it in its automation. " +
            "Changing a schedule changes every block using it."))
        for (s in m.schedules) root.addView(ui.card().apply {
            addView(ui.text(s.name, 17f))
            val users = m.blocks.filter { s.name in it.automation.schedules }.map { it.name }
            if (users.isNotEmpty()) addView(ui.help("Used by " + users.joinToString(", ")))
            for (w in s.windows) addView(removable(w.describe()) { run { c, mm -> c.setScheduleWindows(mm, s.name, s.windows - w) } })
            val row = ui.row()
            row.addView(ui.button("Add window…") { askWindows { ws -> run { c, mm -> c.setScheduleWindows(mm, s.name, s.windows + ws) } } })
            row.addView(ui.button("Delete") { confirm("Delete ${s.name}?", "Blocks using it lose its windows.") { run { c, mm -> c.deleteSchedule(mm, s.name) } } })
            addView(row)
        })
        root.addView(ui.button("New schedule…") { ask("New schedule", "Name, e.g. Evening") { n -> run { c, mm -> c.addSchedule(mm, n) } } })
    }

    private fun placesPage(root: LinearLayout) {
        backTo(root, "Places")
        val m = model
        root.addView(ui.help("A place is an area on the map. The phone is there when a location fix is inside it; it " +
            "has left when the fix's accuracy circle has been fully outside for 10 minutes."))
        for (p in m.places) root.addView(ui.card().apply {
            val top = ui.row()
            top.addView(ui.grow(ui.text(p.name, 17f)))
            top.addView(ui.pill(if (p.inside) "here" else "not here", if (p.inside) Ui.Pill.ON else Ui.Pill.OFF))
            addView(top)
            addView(ui.help(placeLine(p)))
            val users = m.blocks.filter { p.name in it.automation.places }.map { it.name }
            if (users.isNotEmpty()) addView(ui.help("Used by " + users.joinToString(", ")))
            val row = ui.row()
            row.addView(ui.button("Edit on the map…") { openMap(p.name) })
            row.addView(ui.button("Delete") { confirm("Delete ${p.name}?", "Blocks using it lose it.") { run { c, mm -> c.deletePlace(mm, p.name) } } })
            addView(row)
        })
        root.addView(ui.button("New place…") {
            ask("New place", "Name, e.g. Home") { n ->
                if (n.isBlank()) return@ask toast("Give the place a name")
                if (model.place(n.trim()) != null) return@ask toast("A place named ${n.trim()} exists")
                openMap(n.trim())
            }
        })
    }

    private fun openMap(name: String) =
        startActivity(Intent(this, MapPickerActivity::class.java).putExtra(MapPickerActivity.EXTRA_NAME, name))

    private fun logPage(root: LinearLayout) {
        backTo(root, "Refused changes")
        val log = model.refused
        if (log.isEmpty()) root.addView(ui.help("Nothing refused yet."))
        for (l in log.reversed()) root.addView(ui.text(l, 13f))
    }

    // Settings: protections, no weakening, browser, IronFox, update, lift.
    private fun settingsPage(root: LinearLayout) {
        backTo(root, "Settings")
        val owner = DeviceOwner(this)
        root.addView(ui.heading("No weakening"))
        root.addView(CheckBox(this).apply {
            text = "No weakening during active locks"
            isChecked = model.noWeakening
            setOnCheckedChangeListener { _, on -> run { c, m -> c.setNoWeakening(m, on) } }
        })
        root.addView(ui.help("While any block is locked, no setting on this page can be weakened."))

        root.addView(ui.heading("Protections"))
        for (o in Option.entries) {
            root.addView(CheckBox(this).apply {
                text = o.label + if (o == Option.DEBUGGING && BuildConfig.DEBUG) " (ignored in debug builds)" else ""
                isChecked = store.isOn(o)
                setOnCheckedChangeListener { _, on ->
                    val err = options.set(o, on)
                    if (err != null) { toast(err); render() } else apply()
                }
            })
            if (options.locked(o)) root.addView(ui.help("  Locked until " + time(store.lockedUntil(o))))
            if (o == Option.DATE_TIME && !store.isOn(o) && o.effective(store))
                root.addView(ui.help("  On while anything is locked: locks trust the phone's clock."))
            root.addView(ui.button("Lock \"${o.label}\"…") { askOptionLock(o) })
        }

        ironFoxSection(root)
        browserSection(root)

        // Android only installs an APK signed with KC9K's own key, so updating is always allowed.
        root.addView(ui.heading("Update"))
        root.addView(ui.button("Install update (APK)…") {
            pick(Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "application/vnd.android.package-archive"
            }, PICK_APK)
        })

        // For now: a way out that keeps Device Owner, so a new APK can still be installed (SPEC §7).
        root.addView(ui.heading("Lift"))
        if (store.lifted) {
            root.addView(ui.help("Everything is lifted. Resuming applies the blocks, the protections and the browser " +
                "settings as set (the locks of the protections and browser settings were cleared)."))
            root.addView(ui.button("Resume protection") {
                owner.resume(); GuardService.resync(this); toast("Protection is back"); render()
            })
        } else root.addView(ui.button("Lift everything (keeps Device Owner)…") { confirmLift(owner) })

        if (BuildConfig.DEBUG) root.addView(ui.button("DEBUG: lift everything and remove Device Owner") {
            owner.releaseForDebug(); toast("Released"); render()
        })
    }

    private fun apply() { DeviceOwner(this).applyBaseline(); render() }

    private fun confirmLift(owner: DeviceOwner) = confirm("Lift everything?",
        "Every app is unpaused, Wi-Fi is allowed, every protection is lifted and the locks of the protections and " +
            "browser settings are cleared, and IronFox needs no extension. KC9K stays Device Owner and keeps your " +
            "settings and blocks; \"Resume protection\" turns them back on.") {
        owner.liftEverything(); GuardService.resync(this); toast("Everything is lifted"); render()
    }

    /** Starts the system file picker, let through while it is open (Picker). */
    private fun pick(intent: Intent, request: Int) {
        Picker.allow(this)
        try { startActivityForResult(intent, request) }
        catch (e: Exception) { Picker.done(this); toast("No file picker: ${e.message}") }
    }

    private fun result(err: String?) { err?.let(::toast); render() }

    // IronFox: the extension is installed by hand once, then checks in every few seconds.
    private fun ironFoxSection(root: LinearLayout) {
        root.addView(ui.heading("IronFox"))
        val installed = IronFox.installed(this)
        val bundled = RulesServer.bundledVersion(this)
        val last = ExtensionStatus.last
        val fmt = DateFormat.getTimeInstance(DateFormat.MEDIUM)
        root.addView(ui.text(buildString {
            append(if (installed.isEmpty()) "IronFox: not installed\n" else "IronFox: installed\n")
            append("Extension: " + ExtensionGuard.problem(last, now, bundled).text)
            if (last != null) append(" (last ${fmt.format(Date(last.at))}, version ${last.version})")
            append("\nThis app carries version $bundled\n")
            append("Needed now: " + if (store.extensionRequired()) "yes" else "no (no block with websites is on, no browser setting)")
            append("\nUsage access: " + if (Foreground(this@MainActivity).hasAccess()) "granted" else "MISSING: IronFox stays paused while the extension is needed")
        }))
        root.addView(ui.help("While it is needed, IronFox is paused if it is open for a minute without the extension. " +
            "One-time setup in IronFox: Settings > IronFox > IronFox settings > Security > Allow installation of add-ons; " +
            "about:config > xpinstall.signatures.required = false; then install the extension below and tick \"Allow in private browsing\"."))
        root.addView(ui.button("Grant usage access…") { startActivity(Intent(android.provider.Settings.ACTION_USAGE_ACCESS_SETTINGS)) })
        root.addView(ui.button("Install the extension in IronFox", installed.isNotEmpty()) {
            try {
                startActivity(Intent(Intent.ACTION_VIEW, android.net.Uri.parse(RulesServer.XPI_URL))
                    .setPackage(installed.first()).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
            } catch (e: Exception) { toast("Couldn't open IronFox: ${e.message}") }
        })
        root.addView(ui.button("Save the extension file (for Install extension from file)…") {
            pick(Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "application/x-xpinstall"
                putExtra(Intent.EXTRA_TITLE, "kc9k-$bundled.xpi")
            }, SAVE_XPI)
        })
    }

    // SafeSearch and web results, each with its own lock, as on the computer.
    private fun browserSection(root: LinearLayout) {
        root.addView(ui.heading("Browser (enforced by the extension in IronFox)"))
        for (b in BrowserSetting.entries) {
            root.addView(CheckBox(this).apply {
                text = b.label
                isChecked = store.isOn(b)
                setOnCheckedChangeListener { _, on -> result(if (on) web.turnOn(b, null) else web.turnOff(b)) }
            })
            root.addView(ui.help(b.help))
            val l = web.lock(b.key)
            if (l.inForce(now, store.bootCount)) root.addView(ui.help("Locked: " + l.describe(now, store.bootCount)))
            val row = ui.row()
            row.addView(ui.button("Lock…") { askLock("Lock \"${b.label}\"", schedule = false) { want, _ -> result(web.turnOn(b, want)) } })
            if (l.chars > 0 || l.passwordHash.isNotEmpty()) row.addView(ui.button("Unlock…") { startUnlock("setting:${b.key}") })
            root.addView(row)
        }
    }

    // Lists.
    private fun entryList(root: LinearLayout, title: String, entries: List<String>, addLabel: String,
                          add: (String) -> Unit, remove: (String) -> Unit) {
        root.addView(ui.heading("$title (${entries.size})"))
        val input = EditText(this).apply { hint = "One entry per line"; minLines = 2 }
        root.addView(input)
        root.addView(ui.button(addLabel) { add(input.text.toString()) })
        for (e in entries) root.addView(removable(e) { remove(e) })
    }

    private fun removable(label: String, remove: () -> Unit): View {
        val row = ui.row()
        row.addView(ui.grow(ui.text(label)))
        row.addView(ui.button("Remove") { remove() })
        return row
    }

    /** Export and import buttons for one of a block's lists. */
    private fun listFiles(root: LinearLayout, b: Block, exceptions: Boolean) {
        val list = if (exceptions) WebBlock.EXCEPTIONS else WebBlock.BLOCKED
        val entries = if (exceptions) b.siteExceptions else b.sites
        val row = ui.row()
        row.addView(ui.button("Export…", entries.isNotEmpty()) {
            fileList = b.name to exceptions
            pick(Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "text/plain"
                putExtra(Intent.EXTRA_TITLE, "kc9k-${b.name.lowercase().replace(' ', '-')}-${list.lowercase()}.txt")
            }, EXPORT)
        })
        row.addView(ui.button("Import…") {
            fileList = b.name to exceptions
            pick(Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
                addCategory(Intent.CATEGORY_OPENABLE); type = "text/*"
                // Some file managers call .txt/.list files octet-stream.
                putExtra(Intent.EXTRA_MIME_TYPES, arrayOf("text/*", "application/octet-stream"))
            }, IMPORT)
        })
        root.addView(row)
    }

    private fun exportList(uri: android.net.Uri, block: String, exceptions: Boolean) {
        val b = model.block(block) ?: return
        val entries = if (exceptions) b.siteExceptions else b.sites
        val list = if (exceptions) WebBlock.EXCEPTIONS else WebBlock.BLOCKED
        try {
            contentResolver.openOutputStream(uri, "wt")!!.use { it.write(ListFile.format("$block/$list", entries).toByteArray()) }
            toast("Exported ${entries.size} entries")
        } catch (e: Exception) { toast("Couldn't export: ${e.message}") }
    }

    private fun importList(uri: android.net.Uri, block: String, exceptions: Boolean) {
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
        run { c, m -> c.importSites(m, block, exceptions, text) }
    }

    // Dialogs (bottom of the screen where Android allows; plain dialogs for now).
    private fun ask(title: String, hint: String, done: (String) -> Unit) {
        val input = EditText(this).apply { this.hint = hint; setSingleLine() }
        AlertDialog.Builder(this).setTitle(title).setView(input)
            .setPositiveButton("OK") { _, _ -> done(input.text.toString()) }
            .setNegativeButton("Cancel", null).show()
    }

    private fun confirm(title: String, message: String, yes: () -> Unit) {
        AlertDialog.Builder(this).setTitle(title).setMessage(message)
            .setPositiveButton("OK") { _, _ -> yes() }.setNegativeButton("Cancel", null).show()
    }

    /** Days and a time range: one window per ticked day. */
    private fun askWindows(done: (List<Window>) -> Unit) {
        val box = ui.column(16)
        val days = Window.DAYS.map { d -> CheckBox(this).apply { text = d } }
        val dayRow = ui.row()
        days.forEach(dayRow::addView)
        box.addView(android.widget.HorizontalScrollView(this).apply { addView(dayRow) })
        val from = EditText(this).apply { hint = "From, HH:MM (5-minute steps)"; inputType = InputType.TYPE_CLASS_DATETIME }
        val to = EditText(this).apply { hint = "To, HH:MM (24:00 = midnight)"; inputType = InputType.TYPE_CLASS_DATETIME }
        box.addView(from); box.addView(to)
        AlertDialog.Builder(this).setTitle("Add window").setView(box)
            .setPositiveButton("Add") { _, _ ->
                val f = Window.parse(from.text.toString()); val t = Window.parse(to.text.toString())
                val picked = days.indices.filter { days[it].isChecked }
                when {
                    picked.isEmpty() -> toast("Tick at least one day")
                    f == null || t == null -> toast("Invalid time (HH:MM)")
                    t <= f -> toast("The end must be after the start (a window over midnight is two windows)")
                    else -> done(picked.map { Window(it, f, t) })
                }
            }
            .setNegativeButton("Cancel", null).show()
    }

    /**
     * The lock: any mix of a timer lock, the typing challenge, a password, until restart and
     * (for blocks with automation) a schedule lock. Confirmed before it is written (R-LOCK-12).
     */
    private fun askLock(title: String, schedule: Boolean, onLock: (SettingLock, Boolean) -> Unit) {
        val box = ui.column(16)
        val hours = EditText(this).apply {
            hint = "Timer lock: hours (e.g. 2, 168 for a week)"
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
        val sched = CheckBox(this).apply { text = "Schedule lock: locked while its automation has it on" }
        listOf(hours, clock, useChallenge, chars, usePassword, password, again, restart).forEach(box::addView)
        if (schedule) box.addView(sched)
        AlertDialog.Builder(this).setTitle(title).setView(ScrollView(this).apply { addView(box) })
            .setPositiveButton("Lock…") { _, _ ->
                var until = 0L
                val h = hours.text.toString().trim()
                val c = clock.text.toString().trim()
                if (h.isNotEmpty() && c.isNotEmpty()) return@setPositiveButton toast("Give hours or a clock time, not both")
                if (h.isNotEmpty()) {
                    val v = h.toDoubleOrNull()
                    if (v == null || v <= 0) return@setPositiveButton toast("Invalid hours")
                    until = now + (v * 3_600_000).toLong()
                }
                if (c.isNotEmpty()) until = untilClock(c, now) ?: return@setPositiveButton toast("Invalid clock time (HH:MM)")
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
                val s = schedule && sched.isChecked
                if (want == SettingLock() && !s) return@setPositiveButton toast("Choose at least one lock")
                val what = listOfNotNull(want.describe(now, store.bootCount).takeIf { it.isNotEmpty() },
                    if (s) "schedule lock" else null).joinToString(", ")
                confirm("$title?", "Locks: $what.\n\nWhile locked, it can't be weakened, and the lock can't be " +
                    "shortened or removed early.") { onLock(want, s) }
            }
            .setNegativeButton("Cancel", null).show()
    }

    /** The next HH:MM from now (today, or tomorrow if it has passed), in ms. */
    private fun untilClock(text: String, now: Long): Long? {
        val m = Regex("^(\\d{1,2}):(\\d{2})$").find(text.trim()) ?: return null
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
    private fun startUnlock(key: String) {
        val isBlock = key.startsWith("block:")
        val name = key.substringAfter(':')
        val refusal = if (isBlock) cmd().refuseUnlock(model, name) else web.refuseUnlock(name)
        refusal?.let { return toast(it) }
        val l = if (isBlock) model.block(name)!!.lock else store.settingLock(name)
        val challenge = {
            if (l.chars > 0) {
                unlocking = key
                startActivityForResult(Intent(this, ChallengeActivity::class.java)
                    .putExtra(ChallengeActivity.EXTRA_CHARS, l.chars), CHALLENGE)
            } else unlocked(key)
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

    private fun unlocked(key: String) {
        val name = key.substringAfter(':')
        if (key.startsWith("block:")) run { c, m -> c.unlocked(m, name) } else result(web.unlocked(name))
    }

    private fun askOptionLock(o: Option) {
        val input = EditText(this).apply {
            hint = "Hours"; inputType = InputType.TYPE_CLASS_NUMBER or InputType.TYPE_NUMBER_FLAG_DECIMAL
        }
        AlertDialog.Builder(this).setTitle("Lock \"${o.label}\" for").setView(input)
            .setPositiveButton("Lock…") { _, _ ->
                val h = input.text.toString().toDoubleOrNull()
                if (h == null || h <= 0) return@setPositiveButton toast("Invalid hours")
                confirm("Lock \"${o.label}\"?", "For $h hours it can't be turned off.") {
                    options.lock(o, (h * 3_600_000).toLong())?.let(::toast) ?: apply()
                }
            }
            .setNegativeButton("Cancel", null).show()
    }

    @Deprecated("Deprecated in API")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == CHALLENGE) {
            val key = unlocking ?: return
            unlocking = null
            if (resultCode == RESULT_OK) unlocked(key) else toast("Still locked")
            return
        }
        // Every other request was a file picker: pause it again.
        Picker.done(this)
        val uri = data?.data ?: return
        if (resultCode != RESULT_OK) return
        val list = fileList
        when (requestCode) {
            EXPORT -> return list?.let { exportList(uri, it.first, it.second) } ?: Unit
            IMPORT -> return list?.let { importList(uri, it.first, it.second) } ?: Unit
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
