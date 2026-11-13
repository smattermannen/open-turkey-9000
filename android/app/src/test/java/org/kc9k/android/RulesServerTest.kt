package org.kc9k.android

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.net.Socket

/** Talks HTTP to RulesServer on a spare port, the way the extension does. */
class RulesServerTest {
    private val port = 47699
    private lateinit var server: RulesServer

    @Before fun up() {
        ExtensionStatus.last = null
        server = RulesServer({ "{\"lists\":[]}" }, { byteArrayOf(80, 75, 3, 4) }, port)
        server.start()
        assertTrue(server.awaitListening())
    }

    @After fun down() = server.stop()

    private fun request(raw: String): Pair<Int, String> = Socket("127.0.0.1", port).use { s ->
        s.getOutputStream().write(raw.toByteArray()); s.getOutputStream().flush()
        val text = s.getInputStream().readBytes().decodeToString()
        text.substringAfter(" ").substringBefore(" ").toInt() to text.substringAfter("\r\n\r\n")
    }

    @Test fun rulesNeedTheHeader() {
        assertEquals(403, request("GET /rules HTTP/1.1\r\nHost: x\r\n\r\n").first)
        assertEquals(null, ExtensionStatus.last)
        val (status, body) = request("GET /rules HTTP/1.1\r\nHost: x\r\nx-kc9k-extension: 1\r\n" +
            "X-KC9K-Version: 1.15.0\r\nX-KC9K-Private: 1\r\n\r\n")
        assertEquals(200, status)
        assertEquals("{\"lists\":[]}", body)
        val c = ExtensionStatus.last!!
        assertEquals("1.15.0", c.version)
        assertTrue(c.privateTabs)
    }

    @Test fun noPreflight() = assertEquals(405, request("OPTIONS /rules HTTP/1.1\r\nOrigin: https://evil.example\r\n\r\n").first)

    @Test fun servesTheExtension() {
        val (status, body) = request("GET /kc9k.xpi HTTP/1.1\r\n\r\n")
        assertEquals(200, status)
        assertTrue(body.startsWith("PK"))
    }

    @Test fun reportsAnswered() {
        val body = "{\"list\":1}"
        val (status, answer) = request("POST /hit HTTP/1.1\r\nX-KC9K-Extension: 1\r\nContent-Length: ${body.length}\r\n\r\n$body")
        assertEquals(200, status)
        assertEquals("{}", answer)
        assertEquals(404, request("GET /nothing HTTP/1.1\r\nX-KC9K-Extension: 1\r\n\r\n").first)
    }
}
