package org.kc9k.android

import android.content.Context
import android.util.Log
import java.io.BufferedInputStream
import java.io.ByteArrayOutputStream
import java.io.InputStream
import java.io.OutputStream
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.util.concurrent.Executors

/**
 * Serves the KC9K extension in IronFox on 127.0.0.1:47613, where the desktop daemon serves
 * it, so the extension (internal/webext, packed into this app) runs unchanged: GET /rules
 * gives the rules in the daemon's format and counts as a check-in. Like the daemon, it
 * needs the X-KC9K-Extension header, which a web page can't send without a preflight that
 * is never allowed. GET /kc9k.xpi serves the extension itself, for installing it.
 */
class RulesServer(
    private val rules: () -> String,
    private val xpi: () -> ByteArray,
    private val port: Int = PORT,
) {
    constructor(context: Context) : this(
        rules = { Store(context).rulesJson() },
        xpi = { context.assets.open("kc9k.xpi").use { it.readBytes() } },
    )

    companion object {
        const val PORT = 47613
        const val XPI_URL = "http://127.0.0.1:$PORT/kc9k.xpi"
        private const val MAX_BODY = 64 * 1024

        /** The packed extension's version, from the asset Gradle writes next to it. */
        fun bundledVersion(context: Context): String = try {
            context.assets.open("kc9k-version.txt").use { it.readBytes().decodeToString().trim() }
        } catch (_: Exception) { "0" }
    }

    @Volatile private var running = false
    @Volatile private var socket: ServerSocket? = null
    private val pool = Executors.newFixedThreadPool(4)

    fun start() {
        if (running) return
        running = true
        Thread({ loop() }, "kc9k-rules").apply { isDaemon = true }.start()
    }

    fun stop() {
        running = false
        try { socket?.close() } catch (_: Exception) {}
    }

    private fun loop() {
        while (running) {
            try {
                ServerSocket(port, 50, InetAddress.getByName("127.0.0.1")).use { server ->
                    socket = server
                    while (running) {
                        val client = server.accept()
                        pool.execute { handle(client) }
                    }
                }
            } catch (e: Exception) {
                // Port taken or the socket closed: no check-ins reach us, so IronFox gets
                // paused (fail closed). Try again shortly.
                if (running) log("rules server: ${e.message}")
                try { Thread.sleep(5_000) } catch (_: InterruptedException) {}
            }
        }
    }

    private fun handle(client: Socket) {
        client.use { s ->
            try {
                s.soTimeout = 5_000
                val input = BufferedInputStream(s.getInputStream())
                val requestLine = readLine(input) ?: return
                val parts = requestLine.split(" ")
                if (parts.size < 2) return
                val method = parts[0]
                val path = parts[1].substringBefore('?')
                val headers = mutableMapOf<String, String>()
                while (true) {
                    val line = readLine(input) ?: return
                    if (line.isEmpty()) break
                    val i = line.indexOf(':')
                    if (i > 0) headers[line.substring(0, i).trim().lowercase()] = line.substring(i + 1).trim()
                }
                val length = headers["content-length"]?.toIntOrNull() ?: 0
                // The body isn't used; read it so the client doesn't see a reset.
                var left = if (length in 1..MAX_BODY) length else 0
                val skip = ByteArray(4096)
                while (left > 0) {
                    val n = input.read(skip, 0, minOf(left, skip.size))
                    if (n < 0) break
                    left -= n
                }
                respond(s.getOutputStream(), method, path, headers)
            } catch (e: Exception) {
                log("rules request: ${e.message}")
            }
        }
    }

    private fun respond(out: OutputStream, method: String, path: String, headers: Map<String, String>) {
        val fromExtension = headers["x-kc9k-extension"] == "1"
        when {
            method == "GET" && path == "/kc9k.xpi" -> {
                send(out, 200, "application/x-xpinstall", xpi())
            }
            // Web pages would need a preflight to send the header: none is ever allowed.
            method == "OPTIONS" -> send(out, 405, "text/plain", "no".toByteArray())
            !fromExtension -> send(out, 403, "text/plain", "extension only".toByteArray())
            method == "GET" && path == "/rules" -> {
                ExtensionStatus.last = CheckIn(System.currentTimeMillis(),
                    headers["x-kc9k-version"] ?: "0", headers["x-kc9k-private"] == "1")
                send(out, 200, "application/json", rules().toByteArray())
            }
            // Reports the desktop daemon acts on; nothing here acts on them yet.
            method == "POST" && (path == "/hit" || path == "/visit" || path == "/usage") ->
                send(out, 200, "application/json", "{}".toByteArray())
            else -> send(out, 404, "text/plain", "not found".toByteArray())
        }
    }

    /** Waits until the server listens (for tests). */
    fun awaitListening(timeoutMs: Long = 5_000): Boolean {
        val end = System.currentTimeMillis() + timeoutMs
        while (System.currentTimeMillis() < end) {
            if (socket?.isBound == true && socket?.isClosed == false) return true
            Thread.sleep(20)
        }
        return false
    }

    private fun log(msg: String) = try { Log.w("kc9k", msg) } catch (_: RuntimeException) { println(msg) }

    private fun send(out: OutputStream, status: Int, type: String, body: ByteArray) {
        val reason = mapOf(200 to "OK", 403 to "Forbidden", 404 to "Not Found", 405 to "Method Not Allowed")[status] ?: ""
        val head = "HTTP/1.1 $status $reason\r\nContent-Type: $type\r\nContent-Length: ${body.size}\r\n" +
            "Cache-Control: no-store\r\nConnection: close\r\n\r\n"
        out.write(head.toByteArray())
        out.write(body)
        out.flush()
    }

    private fun readLine(input: InputStream): String? {
        val buf = ByteArrayOutputStream()
        while (true) {
            val b = input.read()
            if (b < 0) return if (buf.size() == 0) null else buf.toString(Charsets.ISO_8859_1.name())
            if (b == '\n'.code) break
            if (b != '\r'.code) buf.write(b)
            if (buf.size() > 8192) return null
        }
        return buf.toString(Charsets.ISO_8859_1.name())
    }
}
