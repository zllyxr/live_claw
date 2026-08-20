package com.claw.remote

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.time.Instant

internal data class RemoteCommand(
    val id: String,
    val type: String,
    val payload: JSONObject,
    val expiresAtMillis: Long,
)

internal data class CommandResult(val applied: Boolean, val code: String = "")

internal class DeviceHeartbeatClient(private val context: Context) {
    fun beat(apply: (RemoteCommand) -> CommandResult) {
        val config = SecureConfigStore.config(context) ?: return
        val token = SecureConfigStore.token(context)
        if (token.isBlank()) return
        val body = JSONObject()
            .put("device_code", CoreRuntime.id())
            .put("service_status", "running")
            .put("permission_status", JSONObject(HostSdk.statusJson(context)).getJSONObject("permissions"))
            .put("capabilities", JSONObject()
                .put("screen", true).put("input", true).put("clipboard", true)
                .put("file_transfer", false).put("system_audio", false)
                .put("chat", false).put("voice", false))
            .put("device_name", "${Build.MANUFACTURER} ${Build.MODEL}".trim())
            .put("manufacturer", Build.MANUFACTURER)
            .put("model", Build.MODEL)
            .put("android_version", Build.VERSION.RELEASE)
            .put("android_sdk", Build.VERSION.SDK_INT)
            .put("app_version", appVersion())
            .put("app_native_code", appVersionCode())
            .put("plugin_version", HostSdk.VERSION)
        val response = request(config.backendUrl + "/remote/devices/heartbeat", token, body)
        val commands = response.optJSONObject("data")?.optJSONArray("commands") ?: JSONArray()
        for (index in 0 until commands.length()) {
            val value = commands.getJSONObject(index)
            val command = RemoteCommand(
                id = value.getString("id"),
                type = value.getString("type"),
                payload = value.optJSONObject("payload") ?: JSONObject(),
                expiresAtMillis = Instant.parse(value.getString("expires_at")).toEpochMilli(),
            )
            val result = try { apply(command) } catch (_: Throwable) { CommandResult(false, "apply_failed") }
            acknowledge(config.backendUrl, token, command.id, result)
        }
    }

    @Suppress("DEPRECATION")
    private fun appVersion(): String = try {
        context.packageManager.getPackageInfo(context.packageName, 0).versionName.orEmpty()
    } catch (_: PackageManager.NameNotFoundException) { "" }

    @Suppress("DEPRECATION")
    private fun appVersionCode(): Long = try {
        val info = context.packageManager.getPackageInfo(context.packageName, 0)
        if (Build.VERSION.SDK_INT >= 28) info.longVersionCode else info.versionCode.toLong()
    } catch (_: PackageManager.NameNotFoundException) { 0 }

    private fun acknowledge(base: String, token: String, commandID: String, result: CommandResult) {
        val body = JSONObject()
            .put("status", if (result.applied) "applied" else "failed")
            .put("result", JSONObject().put("code", result.code))
        request(base + "/remote/devices/commands/" + commandID + "/ack", token, body)
    }

    private fun request(endpoint: String, token: String, body: JSONObject): JSONObject {
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
            connection.outputStream.use { it.write(body.toString().toByteArray(Charsets.UTF_8)) }
            if (connection.responseCode !in 200..299) throw IllegalStateException("heartbeat rejected")
            val response = connection.inputStream.bufferedReader().use { it.readText() }
            return JSONObject(response)
        } finally {
            connection.disconnect()
        }
    }
}
