package com.claw.remote

import android.accessibilityservice.AccessibilityService
import android.content.Intent
import android.view.accessibility.AccessibilityEvent

class RemoteInputService : AccessibilityService() {
    override fun onServiceConnected() {
        super.onServiceConnected()
        CoreRuntime.attachAccessibilityService(this)
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) = Unit
    override fun onInterrupt() = Unit

    override fun onUnbind(intent: Intent?): Boolean {
        CoreRuntime.detachAccessibilityService(this)
        return super.onUnbind(intent)
    }

    override fun onDestroy() {
        CoreRuntime.detachAccessibilityService(this)
        super.onDestroy()
    }
}
