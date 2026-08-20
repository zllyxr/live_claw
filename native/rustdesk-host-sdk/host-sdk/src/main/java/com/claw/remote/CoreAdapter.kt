package com.claw.remote

import android.accessibilityservice.AccessibilityService
import android.content.Context
import android.media.projection.MediaProjection

/** Implemented by the source-generated adapter built from the licensed RustDesk 1.4.9 fork. */
interface CoreAdapter {
    fun initialize(context: Context, config: HostConfig, eventSink: HostEventSink)
    fun attachAccessibilityService(service: AccessibilityService)
    fun detachAccessibilityService()
    fun start(projection: MediaProjection)
    fun stop()
    fun rustDeskId(): String
    fun setTemporaryPassword(password: String, expiresAtMillis: Long)
    fun rotateParkingPassword()
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
    private const val IMPLEMENTATION = "com.claw.remote.generated.RustDesk149Adapter"

    fun load(): CoreAdapter? = try {
        Class.forName(IMPLEMENTATION).getDeclaredConstructor().newInstance() as CoreAdapter
    } catch (_: Throwable) {
        null
    }
}
