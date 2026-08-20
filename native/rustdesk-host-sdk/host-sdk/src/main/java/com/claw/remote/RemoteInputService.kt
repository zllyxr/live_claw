package com.claw.remote

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.GestureDescription
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.graphics.Path
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.WindowManager
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

class RemoteInputService : AccessibilityService() {
    override fun onServiceConnected() {
        super.onServiceConnected()
        CoreRuntime.attachAccessibilityService(this)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) = Unit
    override fun onInterrupt() = Unit

    internal fun tap(x: Double, y: Double): Boolean {
        val (width, height) = screenSize()
        val path = Path().apply { moveTo((x * width).toFloat(), (y * height).toFloat()) }
        return dispatch(path, 0, 80)
    }

    internal fun swipe(x1: Double, y1: Double, x2: Double, y2: Double, durationMillis: Long): Boolean {
        val (width, height) = screenSize()
        val path = Path().apply {
            moveTo((x1 * width).toFloat(), (y1 * height).toFloat())
            lineTo((x2 * width).toFloat(), (y2 * height).toFloat())
        }
        return dispatch(path, 0, durationMillis)
    }

    internal fun systemAction(action: String): Boolean {
        val globalAction = when (action) {
            "back" -> GLOBAL_ACTION_BACK
            "home" -> GLOBAL_ACTION_HOME
            "recents" -> GLOBAL_ACTION_RECENTS
            else -> return false
        }
        return performGlobalAction(globalAction)
    }

    internal fun inputText(text: String): Boolean {
        val focused = rootInActiveWindow?.findFocus(AccessibilityNodeInfo.FOCUS_INPUT)
        if (focused != null) {
            val arguments = Bundle().apply { putCharSequence(AccessibilityNodeInfo.ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, text) }
            if (focused.performAction(AccessibilityNodeInfo.ACTION_SET_TEXT, arguments)) return true
        }
        return setClipboard(text) && focused?.performAction(AccessibilityNodeInfo.ACTION_PASTE) == true
    }

    internal fun setClipboard(text: String): Boolean = try {
        (getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager)
            .setPrimaryClip(ClipData.newPlainText("远程协助", text))
        true
    } catch (_: Throwable) {
        false
    }

    private fun dispatch(path: Path, startMillis: Long, durationMillis: Long): Boolean {
        val latch = CountDownLatch(1)
        var completed = false
        val gesture = GestureDescription.Builder()
            .addStroke(GestureDescription.StrokeDescription(path, startMillis, durationMillis.coerceIn(50, 2_000)))
            .build()
        val accepted = dispatchGesture(gesture, object : GestureResultCallback() {
            override fun onCompleted(gestureDescription: GestureDescription?) { completed = true; latch.countDown() }
            override fun onCancelled(gestureDescription: GestureDescription?) { latch.countDown() }
        }, Handler(Looper.getMainLooper()))
        if (!accepted) return false
        latch.await(3, TimeUnit.SECONDS)
        return completed
    }

    private fun screenSize(): Pair<Int, Int> {
        val manager = getSystemService(Context.WINDOW_SERVICE) as WindowManager
        return if (android.os.Build.VERSION.SDK_INT >= 30) {
            val bounds = manager.currentWindowMetrics.bounds
            bounds.width() to bounds.height()
        } else {
            @Suppress("DEPRECATION")
            resources.displayMetrics.widthPixels to resources.displayMetrics.heightPixels
        }
    }

    override fun onUnbind(intent: Intent?): Boolean {
        CoreRuntime.detachAccessibilityService(this)
        return super.onUnbind(intent)
    }

    override fun onDestroy() {
        CoreRuntime.detachAccessibilityService(this)
        super.onDestroy()
    }
}
