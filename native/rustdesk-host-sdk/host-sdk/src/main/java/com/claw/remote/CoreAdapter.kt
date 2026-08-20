package com.claw.remote

import android.accessibilityservice.AccessibilityService
import android.content.Context
import android.media.projection.MediaProjection

internal interface CoreAdapter {
    fun initialize(context: Context, config: HostConfig, eventSink: HostEventSink)
    fun attachAccessibilityService(service: AccessibilityService)
    fun detachAccessibilityService(service: AccessibilityService)
    fun start(projection: MediaProjection)
    fun stop()
    fun deviceCode(): String
    fun tap(x: Double, y: Double): Boolean
    fun swipe(x1: Double, y1: Double, x2: Double, y2: Double, durationMillis: Long): Boolean
    fun systemAction(action: String): Boolean
    fun inputText(text: String): Boolean
    fun setClipboard(text: String): Boolean
}

data class HostEvent(
    val type: String,
    val sessionRef: String = "",
    val metadata: Map<String, String> = emptyMap(),
)

fun interface HostEventSink {
    fun emit(event: HostEvent)
}

internal object CoreAdapterLoader {
    fun load(): CoreAdapter = NativeCoreAdapter()
}
