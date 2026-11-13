package org.kc9k.android

import android.app.Activity
import android.graphics.Typeface
import android.os.Bundle
import android.text.Editable
import android.text.InputType
import android.text.TextWatcher
import android.view.ActionMode
import android.view.Menu
import android.view.MenuItem
import android.view.WindowManager
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast

/**
 * The typing challenge that opens a challenge lock (see [Challenge]). Paste-resistant like
 * the desktop's: no copy or paste menus, an edit that adds more than a few characters at
 * once is undone, no keyboard suggestions, no screenshots, and leaving the screen ends the
 * attempt. Returns RESULT_OK when typed well enough.
 */
class ChallengeActivity : Activity() {
    companion object { const val EXTRA_CHARS = "chars" }

    private lateinit var challenge: String
    private lateinit var shown: TextView
    private lateinit var input: EditText

    private val noMenu = object : ActionMode.Callback {
        override fun onCreateActionMode(mode: ActionMode, menu: Menu) = false
        override fun onPrepareActionMode(mode: ActionMode, menu: Menu) = false
        override fun onActionItemClicked(mode: ActionMode, item: MenuItem) = false
        override fun onDestroyActionMode(mode: ActionMode) {}
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        val chars = intent.getIntExtra(EXTRA_CHARS, Challenge.DEFAULT_CHARS)
            .coerceIn(Challenge.MIN_CHARS, Challenge.MAX_CHARS)
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(48, 48, 48, 48) }
        root.addView(TextView(this).apply {
            text = "Type the characters below exactly ($chars characters, ${(Challenge.MIN_ACCURACY * 100).toInt()}% accuracy needed). The spaces are only there to help reading. Leaving this screen starts over."
        })
        shown = TextView(this).apply {
            typeface = Typeface.MONOSPACE; textSize = 18f; setTextIsSelectable(false); setPadding(0, 24, 0, 24)
        }
        root.addView(shown)
        input = EditText(this).apply {
            typeface = Typeface.MONOSPACE
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_VISIBLE_PASSWORD or
                InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS or InputType.TYPE_TEXT_FLAG_MULTI_LINE
            isLongClickable = false
            setTextIsSelectable(false)
            customSelectionActionModeCallback = noMenu
            customInsertionActionModeCallback = noMenu
            importantForAutofill = android.view.View.IMPORTANT_FOR_AUTOFILL_NO_EXCLUDE_DESCENDANTS
            addTextChangedListener(object : TextWatcher {
                private var before = ""
                private var undoing = false
                override fun beforeTextChanged(s: CharSequence, start: Int, count: Int, after: Int) {
                    if (!undoing) before = s.toString()
                }
                override fun onTextChanged(s: CharSequence, start: Int, removed: Int, added: Int) {}
                override fun afterTextChanged(s: Editable) {
                    if (undoing) return
                    if (s.length - before.length > Challenge.PASTE_BURST) {
                        undoing = true
                        s.replace(0, s.length, before)
                        undoing = false
                        Toast.makeText(this@ChallengeActivity, "Pasting doesn't work here", Toast.LENGTH_SHORT).show()
                    }
                }
            })
        }
        root.addView(input)
        root.addView(Button(this).apply {
            text = "Done"
            setOnClickListener {
                // The challenge has no spaces: those shown are only for reading.
                val typed = input.text.toString().replace("\n", "").replace(" ", "")
                if (Challenge.passed(challenge, typed)) {
                    setResult(RESULT_OK); finish()
                } else {
                    val pct = (Challenge.accuracy(challenge, typed) * 100).coerceAtLeast(0.0)
                    Toast.makeText(this@ChallengeActivity, "%.1f%% correct. Here is a new one.".format(pct), Toast.LENGTH_LONG).show()
                    fresh(chars)
                }
            }
        })
        root.addView(Button(this).apply { text = "Cancel"; setOnClickListener { finish() } })
        setContentView(ScrollView(this).apply { addView(root) })
        fresh(chars)
    }

    private fun fresh(chars: Int) {
        challenge = Challenge.generate(chars)
        shown.text = challenge.chunked(30).joinToString("\n") { it.chunked(5).joinToString(" ") }
        input.setText("")
    }

    /** Leaving the screen ends the attempt, so the text can't be read off elsewhere. */
    override fun onStop() {
        super.onStop()
        if (!isFinishing) finish()
    }
}
