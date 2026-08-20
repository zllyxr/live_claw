package com.claw.remote

import android.Manifest
import android.accessibilityservice.AccessibilityServiceInfo
import android.app.ActivityManager
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.os.PowerManager
import android.provider.Settings
import android.view.accessibility.AccessibilityManager
import androidx.core.content.ContextCompat
import org.json.JSONObject
import java.lang.ref.WeakReference

object HostSdk {
    const val VERSION = "1.0.0"

    @JvmStatic fun initialize(context: Context, optionsJSON: String): String {
        val options = JSONObject(optionsJSON)
        val token = options.optString("device_token")
        val config = HostConfig.fromJSON(options)
        SecureConfigStore.save(context.applicationContext, config, token)
        CoreRuntime.initialize(context.applicationContext)
        return statusJson(context)
    }

    @JvmStatic fun start(context: Context): String {
        if (SecureConfigStore.config(context) == null) return statusJson(context)
        context.startActivity(Intent(context, ProjectionPermissionActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        return statusJson(context)
    }

    @JvmStatic fun stop(context: Context, clearCredentials: Boolean): String {
        context.startService(Intent(context, RemoteHostService::class.java).setAction(RemoteHostService.ACTION_STOP))
        if (clearCredentials) SecureConfigStore.clear(context)
        return statusJson(context)
    }

    @JvmStatic fun statusJson(context: Context): String {
        val configured = SecureConfigStore.config(context) != null
        val available = true
        val running = isServiceRunning(context)
        return JSONObject()
            .put("available", available)
            .put("configured", configured)
            .put("running", running)
            .put("device_code", CoreRuntime.id())
            .put("service_status", if (running) "running" else "stopped")
            .put("message", JSONObject.NULL)
            .put("permissions", permissions(context))
            .toString()
    }

    @JvmStatic fun openPermissionSettings(context: Context, permission: String): String {
        val intent = when (permission) {
            "accessibility" -> Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
            "overlay" -> Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION, Uri.parse("package:${context.packageName}"))
            "all_files" -> if (Build.VERSION.SDK_INT >= 30) {
                Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION, Uri.parse("package:${context.packageName}"))
            } else {
                Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:${context.packageName}"))
            }
            "battery" -> Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:${context.packageName}"))
            "notification" -> Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
            else -> Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:${context.packageName}"))
        }
        context.startActivity(intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
        return statusJson(context)
    }

    private fun permissions(context: Context): JSONObject {
        val notification = Build.VERSION.SDK_INT < 33 || ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        val microphone = ContextCompat.checkSelfPermission(context, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED
        val accessibility = (context.getSystemService(Context.ACCESSIBILITY_SERVICE) as AccessibilityManager)
            .getEnabledAccessibilityServiceList(AccessibilityServiceInfo.FEEDBACK_ALL_MASK)
            .any { it.resolveInfo.serviceInfo.packageName == context.packageName && it.resolveInfo.serviceInfo.name == RemoteInputService::class.java.name }
        val battery = (context.getSystemService(Context.POWER_SERVICE) as PowerManager).isIgnoringBatteryOptimizations(context.packageName)
        val allFiles = if (Build.VERSION.SDK_INT >= 30) Environment.isExternalStorageManager()
            else ContextCompat.checkSelfPermission(context, Manifest.permission.READ_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED
        return JSONObject()
            .put("notification", notification)
            .put("media_projection", isServiceRunning(context))
            .put("system_audio", false)
            .put("accessibility", accessibility)
            .put("overlay", Settings.canDrawOverlays(context))
            .put("all_files", allFiles)
            .put("microphone", microphone)
            .put("battery", battery)
    }

    @Suppress("DEPRECATION")
    private fun isServiceRunning(context: Context): Boolean =
        (context.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager)
            .getRunningServices(Int.MAX_VALUE).any { it.service.className == RemoteHostService::class.java.name }
}

internal object CoreRuntime {
    private var adapter: CoreAdapter? = null
    private var accessibilityService = WeakReference<android.accessibilityservice.AccessibilityService>(null)
    fun initialize(context: Context) {
        adapter = adapter ?: CoreAdapterLoader.load()
        SecureConfigStore.config(context)?.let { config ->
            adapter?.initialize(context, config, HostEventSink { event ->
                RemoteEventReporter.report(context, event)
            })
            accessibilityService.get()?.let { adapter?.attachAccessibilityService(it) }
        }
    }
    fun attachAccessibilityService(service: android.accessibilityservice.AccessibilityService) {
        accessibilityService = WeakReference(service)
        adapter?.attachAccessibilityService(service)
    }
    fun detachAccessibilityService(service: android.accessibilityservice.AccessibilityService) {
        if (accessibilityService.get() !== service) return
        adapter?.detachAccessibilityService(service)
        accessibilityService.clear()
    }
    fun start(context: Context, projection: android.media.projection.MediaProjection) { initialize(context); adapter?.start(projection) }
    fun stop() { adapter?.stop() }
    fun id(): String = adapter?.deviceCode().orEmpty()
    fun tap(x: Double, y: Double): Boolean = adapter?.tap(x, y) == true
    fun swipe(x1: Double, y1: Double, x2: Double, y2: Double, durationMillis: Long): Boolean = adapter?.swipe(x1, y1, x2, y2, durationMillis) == true
    fun systemAction(action: String): Boolean = adapter?.systemAction(action) == true
    fun inputText(text: String): Boolean = adapter?.inputText(text) == true
    fun setClipboard(text: String): Boolean = adapter?.setClipboard(text) == true
}
