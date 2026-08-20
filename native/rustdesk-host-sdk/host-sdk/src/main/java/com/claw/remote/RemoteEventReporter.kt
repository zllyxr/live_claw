package com.claw.remote

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.time.Instant
import java.util.concurrent.Executors

internal object RemoteEventReporter {
    private val executor = Executors.newSingleThreadExecutor()
    private val allowedTypes = setOf(
        "request", "authorized", "denied", "connected", "disconnected",
        "file_transfer_started", "file_transfer_finished", "chat_started",
        "voice_started", "voice_ended",
    )
    private val allowedMetadata = mapOf(
        "direction" to setOf("incoming", "outgoing", "upload", "download"),
        "transport" to setOf("p2p", "relay"),
        "result" to setOf("success", "failed", "cancelled", "denied"),
        "capability" to setOf("screen", "input", "clipboard", "file_transfer", "system_audio", "chat", "voice"),
    )

    fun report(context: Context, event: HostEvent) {
        if (event.type !in allowedTypes) return
        val safeMetadata = JSONObject()
        event.metadata.forEach { (key, value) ->
            val normalized = value.trim().lowercase()
            if (normalized in allowedMetadata[key].orEmpty()) safeMetadata.put(key, normalized)
        }
        val payload = JSONObject().put("events", JSONArray().put(JSONObject()
            .put("type", event.type)
            .put("session_ref", event.sessionRef.take(100))
            .put("occurred_at", Instant.now().toString())
            .put("metadata", safeMetadata)))
        executor.execute {
            try { send(context.applicationContext, payload) } catch (_: Throwable) { }
        }
    }

    private fun send(context: Context, payload: JSONObject) {
        val config = SecureConfigStore.config(context) ?: return
        val token = SecureConfigStore.token(context)
        if (token.isBlank()) return
        val endpoint = config.backendUrl + "/remote/devices/events"
        require(endpoint.startsWith("https://"))
        val connection = (URL(endpoint).openConnection() as HttpURLConnection).apply {
            requestMethod = "POST"
            connectTimeout = 5_000
            readTimeout = 8_000
            doOutput = true
            useCaches = false
            instanceFollowRedirects = false
            setRequestProperty("Authorization", "Device $token")
            setRequestProperty("Content-Type", "application/json")
            setRequestProperty("Cache-Control", "no-store")
        }
        try {
            connection.outputStream.use { it.write(payload.toString().toByteArray(Charsets.UTF_8)) }
            if (connection.responseCode !in 200..299) throw IllegalStateException("event rejected")
            connection.inputStream.close()
        } finally {
            connection.disconnect()
        }
    }
}
