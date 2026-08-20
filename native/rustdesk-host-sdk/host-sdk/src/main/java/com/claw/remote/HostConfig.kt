package com.claw.remote

import org.json.JSONObject

data class HostConfig(
    val backendUrl: String,
    val deviceId: String,
    val notificationTitle: String,
) {
    fun toJSON() = JSONObject()
        .put("backend_url", backendUrl)
        .put("device_id", deviceId)
        .put("notification_title", notificationTitle)

    companion object {
        fun fromJSON(value: JSONObject) = HostConfig(
            backendUrl = value.optString("backend_url").trimEnd('/'),
            deviceId = value.optString("device_id"),
            notificationTitle = value.optString("notification_title", "星域远程协助正在运行"),
        )
    }
}
