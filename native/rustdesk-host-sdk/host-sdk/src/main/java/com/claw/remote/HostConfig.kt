package com.claw.remote

import org.json.JSONObject

data class HostConfig(
    val backendUrl: String,
    val deviceId: String,
    val idServer: String,
    val relayServer: String,
    val apiServer: String,
    val publicKey: String,
    val notificationTitle: String,
) {
    fun toJSON() = JSONObject()
        .put("backend_url", backendUrl)
        .put("device_id", deviceId)
        .put("id_server", idServer)
        .put("relay_server", relayServer)
        .put("api_server", apiServer)
        .put("public_key", publicKey)
        .put("notification_title", notificationTitle)

    companion object {
        fun fromJSON(value: JSONObject) = HostConfig(
            backendUrl = value.optString("backend_url").trimEnd('/'),
            deviceId = value.optString("device_id"),
            idServer = value.optString("id_server"),
            relayServer = value.optString("relay_server"),
            apiServer = value.optString("api_server").trimEnd('/'),
            publicKey = value.optString("public_key"),
            notificationTitle = value.optString("notification_title", "星域远程协助正在运行"),
        )
    }
}
