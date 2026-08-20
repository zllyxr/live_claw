package com.claw.remote

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit

class RemoteHostService : Service() {
    private var projection: MediaProjection? = null
    private var heartbeat: ScheduledExecutorService? = null
    private val mainHandler = Handler(Looper.getMainLooper())

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopRemoteHost(clearCredentials = false)
            return START_NOT_STICKY
        }
        if (intent?.action != ACTION_START) return START_NOT_STICKY
        val config = SecureConfigStore.config(this) ?: run {
            stopSelf(); return START_NOT_STICKY
        }
        startVisibleForeground(config.notificationTitle)
        val resultData = projectionResult(intent) ?: run {
            stopSelf(); return START_NOT_STICKY
        }
        val resultCode = intent.getIntExtra(EXTRA_RESULT_CODE, 0)
        val manager = getSystemService(MEDIA_PROJECTION_SERVICE) as MediaProjectionManager
        projection = manager.getMediaProjection(resultCode, resultData)?.also { mediaProjection ->
            mediaProjection.registerCallback(object : MediaProjection.Callback() {
                override fun onStop() { mainHandler.post { stopRemoteHost(clearCredentials = false) } }
            }, mainHandler)
            CoreRuntime.start(applicationContext, mediaProjection)
            startHeartbeat()
        }
        if (projection == null) stopRemoteHost(clearCredentials = false)
        return START_NOT_STICKY
    }

    private fun startVisibleForeground(title: String) {
        var type = ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION
        if (Build.VERSION.SDK_INT >= 30 && ContextCompat.checkSelfPermission(this, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) {
            type = type or ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE
        }
        ServiceCompat.startForeground(this, NOTIFICATION_ID, notification(title), type)
    }

    private fun notification(title: String): Notification {
        val stopIntent = Intent(this, RemoteHostService::class.java).setAction(ACTION_STOP)
        val pendingStop = PendingIntent.getService(this, 0, stopIntent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setContentTitle(title.ifBlank { "星域远程协助正在运行" })
            .setContentText("屏幕共享已开启，点按“停止”可立即结束")
            .setOngoing(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .addAction(0, "停止", pendingStop)
            .build()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(CHANNEL_ID, "远程协助", NotificationManager.IMPORTANCE_LOW).apply {
            description = "远程协助运行状态（不可静默隐藏）"
            setShowBadge(false)
        }
        (getSystemService(NOTIFICATION_SERVICE) as NotificationManager).createNotificationChannel(channel)
    }

    private fun startHeartbeat() {
        heartbeat?.shutdownNow()
        heartbeat = Executors.newSingleThreadScheduledExecutor().also { executor ->
            executor.scheduleWithFixedDelay({
                try {
                    DeviceHeartbeatClient(applicationContext).beat(::applyCommand)
                } catch (_: Throwable) {
                    // Network loss is reflected by the backend's 20-second online window.
                }
            }, 0, 5, TimeUnit.SECONDS)
        }
    }

    private fun applyCommand(command: RemoteCommand): CommandResult {
        return when (command.type) {
            "set_temporary_password" -> {
                val password = command.payload.getString("password")
                val expires = command.expiresAtMillis
                CoreRuntime.setPassword(password, expires)
                val delay = (expires - System.currentTimeMillis()).coerceAtLeast(0)
                mainHandler.postDelayed({ CoreRuntime.rotatePassword() }, delay)
                CommandResult(true)
            }
            "stop" -> {
                mainHandler.post { stopRemoteHost(clearCredentials = true) }
                CommandResult(true)
            }
            "rotate_password" -> { CoreRuntime.rotatePassword(); CommandResult(true) }
            else -> CommandResult(false, "unsupported_command")
        }
    }

    private fun stopRemoteHost(clearCredentials: Boolean) {
        heartbeat?.shutdownNow(); heartbeat = null
        CoreRuntime.rotatePassword()
        CoreRuntime.stop()
        projection?.stop(); projection = null
        if (clearCredentials) SecureConfigStore.clear(this)
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        heartbeat?.shutdownNow()
        CoreRuntime.rotatePassword()
        CoreRuntime.stop()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    @Suppress("DEPRECATION")
    private fun projectionResult(intent: Intent): Intent? =
        if (Build.VERSION.SDK_INT >= 33) intent.getParcelableExtra(EXTRA_RESULT_DATA, Intent::class.java)
        else intent.getParcelableExtra(EXTRA_RESULT_DATA)

    companion object {
        const val ACTION_START = "com.claw.remote.START"
        const val ACTION_STOP = "com.claw.remote.STOP"
        const val EXTRA_RESULT_CODE = "projection_result_code"
        const val EXTRA_RESULT_DATA = "projection_result_data"
        private const val CHANNEL_ID = "claw_remote_assistance"
        private const val NOTIFICATION_ID = 4819
    }
}
